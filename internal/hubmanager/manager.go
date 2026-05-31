// Package hubmanager owns the Scrubadubber Hub as a managed child process. It
// starts/stops/restarts the Hub, supervises it (auto-restart on unexpected exit
// with exponential backoff), health-checks it, and reports a coarse status the
// tray app turns into an icon color.
//
// It deliberately knows nothing about scrubbing — only how to run the Hub binary
// and probe its health endpoint.
package hubmanager

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// State is the Hub's observed status.
type State int

const (
	// Starting: the Hub process is launching (or warming up before healthy).
	Starting State = iota
	// Healthy: the Hub answers the health check.
	Healthy
	// Degraded: the process is alive but the health check is failing.
	Degraded
	// Down: no Hub is running or reachable.
	Down
)

func (s State) String() string {
	switch s {
	case Starting:
		return "starting"
	case Healthy:
		return "healthy"
	case Degraded:
		return "degraded"
	case Down:
		return "down"
	default:
		return "unknown"
	}
}

type desired int

const (
	desiredStopped desired = iota
	desiredRunning
)

// Config configures a Manager. Zero-value durations fall back to sensible
// defaults.
type Config struct {
	HubPath   string   // absolute path to the Hub binary
	Args      []string // arguments, e.g. ["serve"]
	Env       []string // extra environment, appended to os.Environ()
	HealthURL string   // e.g. http://127.0.0.1:8384/healthz
	LogWriter io.Writer

	HealthEvery   time.Duration // health poll interval (default 10s)
	HealthTimeout time.Duration // per-probe timeout (default 2s)
	StartupGrace  time.Duration // running-but-unhealthy window treated as Starting (default 15s)
	StopGrace     time.Duration // graceful-stop timeout before force kill (default 5s)
	BaseBackoff   time.Duration // initial restart backoff (default 1s)
	MaxBackoff    time.Duration // backoff cap (default 60s)
}

type proc struct {
	cmd  *exec.Cmd
	done chan struct{}
}

// Manager supervises the Hub process. Create with New, start the monitor with
// Run, then drive it with Start/Stop/Restart.
type Manager struct {
	hubPath       string
	args          []string
	env           []string
	healthURL     string
	logw          io.Writer
	httpClient    *http.Client
	healthEvery   time.Duration
	healthTimeout time.Duration
	startupGrace  time.Duration
	stopGrace     time.Duration
	baseBackoff   time.Duration
	maxBackoff    time.Duration

	mu              sync.Mutex
	state           State
	desired         desired
	current         *proc
	startedAt       time.Time
	lastAttempt     time.Time
	backoff         time.Duration
	intentionalExit bool
	onState         func(State)
	cancel          context.CancelFunc
	loopRunning     bool

	exited  chan struct{}
	kickCh  chan struct{}
	stateCh chan struct{}
}

// New builds a Manager. It does not start anything until Run is called.
func New(cfg Config) *Manager {
	def := func(v, d time.Duration) time.Duration {
		if v <= 0 {
			return d
		}
		return v
	}
	m := &Manager{
		hubPath:       cfg.HubPath,
		args:          cfg.Args,
		env:           cfg.Env,
		healthURL:     cfg.HealthURL,
		logw:          cfg.LogWriter,
		healthEvery:   def(cfg.HealthEvery, 10*time.Second),
		healthTimeout: def(cfg.HealthTimeout, 2*time.Second),
		startupGrace:  def(cfg.StartupGrace, 15*time.Second),
		stopGrace:     def(cfg.StopGrace, 5*time.Second),
		baseBackoff:   def(cfg.BaseBackoff, 1*time.Second),
		maxBackoff:    def(cfg.MaxBackoff, 60*time.Second),
		state:         Starting,
		desired:       desiredStopped,
		exited:        make(chan struct{}, 1),
		kickCh:        make(chan struct{}, 1),
		stateCh:       make(chan struct{}, 1),
	}
	m.backoff = m.baseBackoff
	if m.logw == nil {
		m.logw = io.Discard
	}
	m.httpClient = &http.Client{Timeout: m.healthTimeout}
	return m
}

// SetOnState registers a callback invoked (on the monitor goroutine) whenever
// the reported state changes. It always reflects the latest state.
func (m *Manager) SetOnState(fn func(State)) {
	m.mu.Lock()
	m.onState = fn
	m.mu.Unlock()
}

// State returns the current reported state.
func (m *Manager) State() State {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state
}

// Run starts the monitor loop (idempotent). It returns immediately; the loop
// runs until ctx is cancelled or Close is called.
func (m *Manager) Run(ctx context.Context) {
	m.mu.Lock()
	if m.loopRunning {
		m.mu.Unlock()
		return
	}
	m.loopRunning = true
	lctx, cancel := context.WithCancel(ctx)
	m.cancel = cancel
	m.mu.Unlock()
	go m.loop(lctx)
}

// Start requests that the Hub be running and triggers an immediate check.
func (m *Manager) Start() {
	m.mu.Lock()
	m.desired = desiredRunning
	m.backoff = m.baseBackoff
	m.lastAttempt = time.Time{}
	m.mu.Unlock()
	m.kick()
}

// Stop requests that the Hub be stopped and terminates any managed process.
// The monitor keeps running and will report Down (or Healthy if an external Hub
// is reachable at HealthURL).
func (m *Manager) Stop() {
	m.mu.Lock()
	m.desired = desiredStopped
	m.mu.Unlock()
	m.stopProcess()
	m.kick()
}

// Restart stops then starts the Hub.
func (m *Manager) Restart() {
	m.Stop()
	m.Start()
}

// Close stops the Hub and shuts down the monitor.
func (m *Manager) Close() {
	m.Stop()
	m.mu.Lock()
	cancel := m.cancel
	m.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (m *Manager) loop(ctx context.Context) {
	ticker := time.NewTicker(m.healthEvery)
	defer ticker.Stop()
	m.tick()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.tick()
		case <-m.exited:
			m.tick()
		case <-m.kickCh:
			m.tick()
		case <-m.stateCh:
			m.mu.Lock()
			st, cb := m.state, m.onState
			m.mu.Unlock()
			if cb != nil {
				cb(st)
			}
		}
	}
}

// tick evaluates health + process liveness and drives the state machine.
func (m *Manager) tick() {
	healthy := m.probeHealth() // network call — do not hold the lock here

	m.mu.Lock()
	defer m.mu.Unlock()

	running := m.current != nil

	switch {
	case healthy:
		// A Hub (ours or external) is healthy: reset backoff and report it.
		m.backoff = m.baseBackoff
		m.setStateLocked(Healthy)

	case m.desired == desiredStopped:
		m.setStateLocked(Down)

	case running:
		if time.Since(m.startedAt) < m.startupGrace {
			m.setStateLocked(Starting)
		} else {
			m.setStateLocked(Degraded)
		}

	default:
		// Desired running, nothing healthy, nothing alive: (re)start, honoring
		// the backoff window.
		if time.Since(m.lastAttempt) >= m.backoff {
			m.lastAttempt = time.Now()
			m.backoff = minDur(m.backoff*2, m.maxBackoff)
			m.spawnLocked()
		} else {
			m.setStateLocked(Down)
		}
	}
}

// spawnLocked launches the Hub. Caller must hold m.mu; it never blocks on the
// process (a separate goroutine reaps it).
func (m *Manager) spawnLocked() {
	cmd := exec.Command(m.hubPath, m.args...)
	cmd.Stdout = m.logw
	cmd.Stderr = m.logw
	cmd.Env = append(os.Environ(), m.env...)

	if err := cmd.Start(); err != nil {
		m.logf("hub start failed: %v", err)
		m.setStateLocked(Down)
		return
	}
	p := &proc{cmd: cmd, done: make(chan struct{})}
	m.current = p
	m.startedAt = time.Now()
	m.intentionalExit = false
	m.logf("hub started (pid %d)", cmd.Process.Pid)
	m.setStateLocked(Starting)
	go m.reap(p)
}

// reap waits for a process to exit and signals the loop if it was unexpected.
func (m *Manager) reap(p *proc) {
	_ = p.cmd.Wait()
	m.mu.Lock()
	intentional := m.intentionalExit
	if m.current == p {
		m.current = nil
	}
	m.mu.Unlock()
	close(p.done)
	if !intentional {
		m.logf("hub exited unexpectedly; will restart")
		select {
		case m.exited <- struct{}{}:
		default:
		}
	}
}

// stopProcess terminates the managed process (intentionally) and waits for it
// to exit, escalating to a force kill after StopGrace.
func (m *Manager) stopProcess() {
	m.mu.Lock()
	m.intentionalExit = true
	p := m.current
	grace := m.stopGrace
	m.mu.Unlock()
	if p == nil {
		return
	}
	_ = terminate(p.cmd) // platform-specific: SIGTERM on macOS, Kill on Windows
	select {
	case <-p.done:
	case <-time.After(grace):
		_ = p.cmd.Process.Kill()
		<-p.done
	}
}

func (m *Manager) probeHealth() bool {
	if m.healthURL == "" {
		return false
	}
	req, err := http.NewRequest(http.MethodGet, m.healthURL, nil)
	if err != nil {
		return false
	}
	resp, err := m.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		return false
	}
	// The Hub returns {"status":"ok"} when healthy. Treat an explicit non-"ok"
	// status as unhealthy (→ Degraded); tolerate a missing/unparseable body so a
	// bare 200 still counts as healthy.
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	var h struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(body, &h); err == nil && h.Status != "" && !strings.EqualFold(h.Status, "ok") {
		return false
	}
	return true
}

// setStateLocked updates state and signals the loop to deliver the callback.
// Caller must hold m.mu.
func (m *Manager) setStateLocked(s State) {
	if m.state == s {
		return
	}
	m.state = s
	select {
	case m.stateCh <- struct{}{}:
	default:
	}
}

func (m *Manager) kick() {
	select {
	case m.kickCh <- struct{}{}:
	default:
	}
}

func (m *Manager) logf(format string, args ...any) {
	fmt.Fprintf(m.logw, "[scrubadubber] "+format+"\n", args...)
}

func minDur(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
