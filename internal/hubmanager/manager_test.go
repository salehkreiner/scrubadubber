package hubmanager

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// TestHelperHub is re-executed as the "Hub" child process. It serves /healthz
// on $HUB_PORT, optionally records each startup to $HUB_COUNT_FILE, and
// optionally self-exits after $HUB_TTL_MS to simulate a crash.
func TestHelperHub(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_HUB") != "1" {
		return
	}
	if f := os.Getenv("HUB_COUNT_FILE"); f != "" {
		if fh, err := os.OpenFile(f, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
			_, _ = fh.Write([]byte("x"))
			_ = fh.Close()
		}
	}
	if rel := os.Getenv("HUB_REL_FILE"); rel != "" {
		// Relative path: lands in the process's working directory (cmd.Dir).
		_ = os.WriteFile(rel, []byte("x"), 0o644)
	}
	if ttl := os.Getenv("HUB_TTL_MS"); ttl != "" {
		if ms, err := strconv.Atoi(ttl); err == nil {
			go func() {
				time.Sleep(time.Duration(ms) * time.Millisecond)
				os.Exit(1)
			}()
		}
	}
	status := os.Getenv("HUB_HEALTH")
	if status == "" {
		status = "ok"
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		if status == "down" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":%q}`, status)
	})
	_ = http.ListenAndServe("127.0.0.1:"+os.Getenv("HUB_PORT"), mux)
	os.Exit(0)
}

func freePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	_, port, _ := net.SplitHostPort(l.Addr().String())
	return port
}

func waitForState(t *testing.T, m *Manager, want State, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if m.State() == want {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("state %v not reached within %v (now %v)", want, timeout, m.State())
}

// helperConfig builds a Manager Config that launches TestHelperHub as the Hub.
func helperConfig(port string, env []string) Config {
	return Config{
		HubPath:       os.Args[0],
		Args:          []string{"-test.run=TestHelperHub"},
		Env:           append([]string{"GO_WANT_HELPER_HUB=1", "HUB_PORT=" + port}, env...),
		HealthURL:     "http://127.0.0.1:" + port + "/healthz",
		HealthEvery:   150 * time.Millisecond,
		HealthTimeout: 400 * time.Millisecond,
		StartupGrace:  3 * time.Second,
		StopGrace:     2 * time.Second,
		BaseBackoff:   100 * time.Millisecond,
		MaxBackoff:    1 * time.Second,
	}
}

func TestStartBecomesHealthy(t *testing.T) {
	port := freePort(t)
	m := New(helperConfig(port, nil))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.Run(ctx)
	m.Start()
	defer m.Close()

	waitForState(t, m, Healthy, 6*time.Second)
}

func TestStopReportsDown(t *testing.T) {
	port := freePort(t)
	m := New(helperConfig(port, nil))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.Run(ctx)
	m.Start()
	defer m.Close()

	waitForState(t, m, Healthy, 6*time.Second)
	m.Stop()
	waitForState(t, m, Down, 4*time.Second)
}

func TestDegradedWhenHealthNotOK(t *testing.T) {
	// Hub process is alive and returns 200, but with {"status":"degraded"} —
	// the manager must report Degraded (yellow), not Healthy.
	port := freePort(t)
	cfg := helperConfig(port, []string{"HUB_HEALTH=degraded"})
	cfg.StartupGrace = 400 * time.Millisecond
	m := New(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.Run(ctx)
	m.Start()
	defer m.Close()

	waitForState(t, m, Degraded, 6*time.Second)
}

func TestAdoptsExternalHub(t *testing.T) {
	// An external Hub already answers health checks; the manager must adopt it
	// and never try to spawn (HubPath is intentionally invalid).
	ext := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ext.Close()

	m := New(Config{
		HubPath:       filepath.Join(t.TempDir(), "does-not-exist"),
		Args:          []string{"serve"},
		HealthURL:     ext.URL + "/healthz",
		HealthEvery:   150 * time.Millisecond,
		HealthTimeout: 400 * time.Millisecond,
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.Run(ctx)
	m.Start()
	defer m.Close()

	waitForState(t, m, Healthy, 4*time.Second)
}

func TestAutoRestartOnCrash(t *testing.T) {
	port := freePort(t)
	countFile := filepath.Join(t.TempDir(), "starts")
	m := New(helperConfig(port, []string{
		"HUB_COUNT_FILE=" + countFile,
		"HUB_TTL_MS=250", // crash ~250ms after each start
	}))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.Run(ctx)
	m.Start()

	time.Sleep(3 * time.Second)
	m.Close()

	data, err := os.ReadFile(countFile)
	if err != nil {
		t.Fatalf("read count file: %v", err)
	}
	if len(data) < 2 {
		t.Fatalf("expected the Hub to be (re)started at least twice, got %d starts", len(data))
	}
}

func TestWorkDirAnchorsRelativePaths(t *testing.T) {
	// The Hub writes relative paths (sqlite state, ./ca/...) against its CWD.
	// Setting WorkDir must redirect those into the data dir, not the caller's CWD.
	port := freePort(t)
	work := t.TempDir()
	cfg := helperConfig(port, []string{"HUB_REL_FILE=hubstate.marker"})
	cfg.WorkDir = work
	m := New(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.Run(ctx)
	m.Start()
	defer m.Close()

	waitForState(t, m, Healthy, 6*time.Second)
	if _, err := os.Stat(filepath.Join(work, "hubstate.marker")); err != nil {
		t.Errorf("relative file should be created under WorkDir: %v", err)
	}
}

func TestStateString(t *testing.T) {
	cases := map[State]string{Starting: "starting", Healthy: "healthy", Degraded: "degraded", Down: "down"}
	for s, want := range cases {
		if got := s.String(); got != want {
			t.Errorf("State(%d).String() = %q, want %q", s, got, want)
		}
	}
}
