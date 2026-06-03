package settings

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadMissingReturnsDefaults(t *testing.T) {
	got, err := Load(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Mode != ModeMask || !got.StartOnLogin || len(got.ProtectedTools) != 1 {
		t.Errorf("missing file did not yield defaults: %+v", got)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	in := Settings{HubURL: "http://127.0.0.1:9000", Mode: ModeRedact, StartOnLogin: false, ProtectedTools: []string{"claude-code"}}
	if err := Save(path, in); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.HubURL != in.HubURL || got.Mode != in.Mode || got.StartOnLogin != in.StartOnLogin {
		t.Errorf("round trip mismatch: got %+v want %+v", got, in)
	}
	if len(got.ProtectedTools) != 1 || got.ProtectedTools[0] != "claude-code" {
		t.Errorf("protected tools not preserved: %+v", got.ProtectedTools)
	}
}

func TestLoadFillsDefaultsForPartial(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(`{"mode":"redact"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Mode != ModeRedact {
		t.Errorf("mode not preserved: %q", got.Mode)
	}
	if got.HubURL == "" || len(got.ProtectedTools) == 0 {
		t.Errorf("defaults not applied for missing fields: %+v", got)
	}
}

func TestLoadStripsUTF8BOM(t *testing.T) {
	// PowerShell's Set-Content -Encoding utf8 (and some editors) prepend a BOM;
	// encoding/json rejects it, so Load must strip it rather than silently
	// reverting to defaults.
	path := filepath.Join(t.TempDir(), "settings.json")
	body := append([]byte{0xEF, 0xBB, 0xBF}, []byte(`{"mode":"redact"}`)...)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load with BOM: %v", err)
	}
	if got.Mode != ModeRedact {
		t.Errorf("BOM not stripped; mode reverted to default: %q", got.Mode)
	}
}

func TestValidate(t *testing.T) {
	bad := []Settings{
		{HubURL: "http://127.0.0.1:8383", Mode: "bogus"},
		{HubURL: "not a url", Mode: ModeMask},
		{HubURL: "", Mode: ModeMask}, // empty host
	}
	for i, s := range bad {
		if err := s.Validate(); err == nil {
			t.Errorf("case %d: expected validation error for %+v", i, s)
		}
	}
	good := Settings{HubURL: "http://127.0.0.1:8383", Mode: ModeMask}
	if err := good.Validate(); err != nil {
		t.Errorf("unexpected error for valid settings: %v", err)
	}
}

func TestSaveLeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	if err := Save(path, Default()); err != nil {
		t.Fatalf("Save: %v", err)
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("leftover temp file: %s", e.Name())
		}
	}
}

func TestServerAuthAndRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	var applied *Settings
	srv := NewServer(Options{
		Path:   path,
		Apply:  func(s Settings) error { applied = &s; return nil },
		Status: func() Status { return Status{HubState: "healthy", Protected: true, Version: "test"} },
	})
	rawURL, err := srv.EnsureRunning()
	if err != nil {
		t.Fatalf("EnsureRunning: %v", err)
	}
	defer srv.Close()

	u, _ := url.Parse(rawURL)
	token := u.Query().Get("t")
	base := "http://" + u.Host

	// Page load with token succeeds and embeds the token.
	resp := mustGet(t, rawURL, "")
	body := readBody(t, resp)
	if !strings.Contains(body, token) || !strings.Contains(body, "Scrubadubber") {
		t.Errorf("index page missing token or title")
	}

	// API without token is forbidden.
	if r := mustGet(t, base+"/api/settings", ""); r.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 without token, got %d", r.StatusCode)
	}

	// API with token returns defaults.
	r := mustGet(t, base+"/api/settings", token)
	if r.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 with token, got %d", r.StatusCode)
	}

	// POST a change; Apply should fire and the file should be written.
	newS := Settings{HubURL: "http://127.0.0.1:9999", Mode: ModeOff, StartOnLogin: false, ProtectedTools: []string{}}
	payload, _ := json.Marshal(newS)
	req, _ := http.NewRequest(http.MethodPost, base+"/api/settings", bytes.NewReader(payload))
	req.Header.Set("X-Scrub-Token", token)
	req.Header.Set("Content-Type", "application/json")
	postResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer postResp.Body.Close()
	if postResp.StatusCode != http.StatusNoContent {
		t.Fatalf("POST expected 204, got %d", postResp.StatusCode)
	}
	if applied == nil || applied.Mode != ModeOff {
		t.Errorf("Apply not invoked with new settings: %+v", applied)
	}
	saved, _ := Load(path)
	if saved.Mode != ModeOff || saved.HubURL != "http://127.0.0.1:9999" {
		t.Errorf("settings not persisted: %+v", saved)
	}
}

func mustGet(t *testing.T, rawurl, token string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, rawurl, nil)
	if token != "" {
		req.Header.Set("X-Scrub-Token", token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", rawurl, err)
	}
	return resp
}

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	var b bytes.Buffer
	_, _ = b.ReadFrom(resp.Body)
	return b.String()
}
