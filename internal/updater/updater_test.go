package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestIsNewer(t *testing.T) {
	cases := []struct {
		current, latest string
		want            bool
	}{
		{"v1.0.0", "v1.0.1", true},
		{"1.0.0", "1.1.0", true}, // missing leading v is tolerated
		{"v1.2.0", "v1.2.0", false},
		{"v2.0.0", "v1.9.9", false},
		{"dev", "v1.0.0", false},     // dev builds never nag
		{"v1.0.0", "garbage", false}, // invalid latest
	}
	for _, c := range cases {
		if got := IsNewer(c.current, c.latest); got != c.want {
			t.Errorf("IsNewer(%q,%q)=%v want %v", c.current, c.latest, got, c.want)
		}
	}
}

func TestParseChecksums(t *testing.T) {
	in := []byte("ABCDEF  hub_windows_amd64.exe\n123456 *scrub-setup_darwin_arm64\n\n# comment line ignored maybe\n")
	m := ParseChecksums(in)
	if m["hub_windows_amd64.exe"] != "abcdef" {
		t.Errorf("got %q", m["hub_windows_amd64.exe"])
	}
	if m["scrub-setup_darwin_arm64"] != "123456" {
		t.Errorf("binary-marker line not parsed: %q", m["scrub-setup_darwin_arm64"])
	}
}

func TestParseChecksumsBaseName(t *testing.T) {
	// Tolerate dir-prefixed entries (e.g. "dist/" or "./") by keying on base name.
	m := ParseChecksums([]byte("aa  ./dist/hub_windows_amd64.exe\nbb  hub_darwin_arm64\n"))
	if m["hub_windows_amd64.exe"] != "aa" {
		t.Errorf("base-name parse failed: %v", m)
	}
	if m["hub_darwin_arm64"] != "bb" {
		t.Errorf("plain name parse failed: %v", m)
	}
}

func TestLatestReleaseAndCheckApp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"tag_name":"v1.5.0","html_url":"https://example/r","assets":[{"name":"a","browser_download_url":"u"}]}`)
	}))
	defer srv.Close()

	c := &Client{HTTP: srv.Client(), APIBase: srv.URL}
	u, err := c.CheckApp(context.Background(), "v1.0.0")
	if err != nil {
		t.Fatalf("CheckApp: %v", err)
	}
	if !u.Available || u.Version != "v1.5.0" {
		t.Errorf("unexpected update: %+v", u)
	}
	if u2, _ := c.CheckApp(context.Background(), "v1.5.0"); u2.Available {
		t.Errorf("should not report update when equal")
	}
}

func TestDownloadAndVerify(t *testing.T) {
	content := []byte("new-binary-bytes")
	sum := sha256.Sum256(content)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(content)
	}))
	defer srv.Close()

	var buf []byte
	w := &byteSink{&buf}
	if err := DownloadAndVerify(context.Background(), srv.Client(), srv.URL, hex.EncodeToString(sum[:]), w); err != nil {
		t.Fatalf("verify ok download failed: %v", err)
	}
	if string(buf) != string(content) {
		t.Errorf("content mismatch")
	}
	if err := DownloadAndVerify(context.Background(), srv.Client(), srv.URL, "deadbeef", &byteSink{new([]byte)}); err == nil {
		t.Errorf("expected checksum mismatch error")
	}
}

func TestUpdateBinarySwaps(t *testing.T) {
	newContent := []byte("the-new-hub-binary")
	sum := sha256.Sum256(newContent)
	assetName := "hub_test"

	mux := http.NewServeMux()
	mux.HandleFunc("/bin", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(newContent) })
	mux.HandleFunc("/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%s  %s\n", hex.EncodeToString(sum[:]), assetName)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	rel := Release{
		TagName: "v9.9.9",
		Assets: []Asset{
			{Name: assetName, URL: srv.URL + "/bin"},
			// scrubadubber-hub publishes its checksums as SHA256SUMS.
			{Name: "SHA256SUMS", URL: srv.URL + "/checksums.txt"},
		},
	}

	dir := t.TempDir()
	dest := filepath.Join(dir, "hub.exe")
	if err := os.WriteFile(dest, []byte("OLD"), 0o644); err != nil {
		t.Fatal(err)
	}

	c := &Client{HTTP: srv.Client(), APIBase: srv.URL}
	if err := c.UpdateBinary(context.Background(), rel, assetName, dest); err != nil {
		t.Fatalf("UpdateBinary: %v", err)
	}

	got, _ := os.ReadFile(dest)
	if string(got) != string(newContent) {
		t.Errorf("dest not updated: %q", got)
	}
	if old, _ := os.ReadFile(dest + ".old"); string(old) != "OLD" {
		t.Errorf("backup not kept: %q", old)
	}
	if _, err := os.Stat(dest + ".new"); !os.IsNotExist(err) {
		t.Errorf(".new temp file should be gone")
	}
}

func TestUpdateBinaryRejectsBadChecksum(t *testing.T) {
	assetName := "hub_test"
	mux := http.NewServeMux()
	mux.HandleFunc("/bin", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("tampered")) })
	mux.HandleFunc("/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%s  %s\n", "0000000000000000000000000000000000000000000000000000000000000000", assetName)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	rel := Release{TagName: "v9", Assets: []Asset{
		{Name: assetName, URL: srv.URL + "/bin"},
		{Name: "checksums.txt", URL: srv.URL + "/checksums.txt"},
	}}
	dir := t.TempDir()
	dest := filepath.Join(dir, "hub.exe")
	_ = os.WriteFile(dest, []byte("OLD"), 0o644)

	c := &Client{HTTP: srv.Client(), APIBase: srv.URL}
	if err := c.UpdateBinary(context.Background(), rel, assetName, dest); err == nil {
		t.Fatal("expected checksum mismatch to abort the update")
	}
	if got, _ := os.ReadFile(dest); string(got) != "OLD" {
		t.Errorf("dest must be untouched on failed verify, got %q", got)
	}
	if _, err := os.Stat(dest + ".new"); !os.IsNotExist(err) {
		t.Errorf(".new temp file should be cleaned up on failure")
	}
}

// byteSink is a tiny io.Writer that appends to a *[]byte.
type byteSink struct{ b *[]byte }

func (s *byteSink) Write(p []byte) (int, error) {
	*s.b = append(*s.b, p...)
	return len(p), nil
}
