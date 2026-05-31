// Package settings defines the app's persisted preferences and a minimal
// loopback web UI to edit them (see server.go). Persistence is a plain JSON
// file written atomically; there is intentionally no GUI-framework dependency.
package settings

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/salehkreiner/scrubadubber/internal/config"
)

// Mode is the Hub scrubbing mode.
type Mode string

const (
	ModeMask   Mode = "mask"
	ModeRedact Mode = "redact"
	ModeOff    Mode = "off"
)

// ToolClaudeCode is the identifier for the Claude Code protected tool.
const ToolClaudeCode = "claude-code"

// Settings holds the user-editable preferences.
type Settings struct {
	HubURL         string   `json:"hub_url"`
	Mode           Mode     `json:"mode"`
	StartOnLogin   bool     `json:"start_on_login"`
	ProtectedTools []string `json:"protected_tools"`
}

// Default returns the out-of-the-box settings.
func Default() Settings {
	return Settings{
		HubURL:         config.DefaultHubURL(),
		Mode:           ModeMask,
		StartOnLogin:   true,
		ProtectedTools: []string{ToolClaudeCode},
	}
}

// Load reads settings from path. A missing file yields Default() (not an
// error); malformed JSON returns Default() plus the parse error.
func Load(path string) (Settings, error) {
	s := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return s, err
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return Default(), fmt.Errorf("parse settings: %w", err)
	}
	s.normalize()
	return s, nil
}

// Save validates and atomically writes settings to path (temp file + rename).
func Save(path string, s Settings) error {
	s.normalize()
	if err := s.Validate(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".settings-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // harmless no-op once the rename succeeds
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// normalize fills empty fields with defaults so partial files stay valid.
func (s *Settings) normalize() {
	if s.HubURL == "" {
		s.HubURL = config.DefaultHubURL()
	}
	if s.Mode == "" {
		s.Mode = ModeMask
	}
	if s.ProtectedTools == nil {
		s.ProtectedTools = []string{ToolClaudeCode}
	}
}

// Validate reports whether the settings are well-formed.
func (s Settings) Validate() error {
	switch s.Mode {
	case ModeMask, ModeRedact, ModeOff:
	default:
		return fmt.Errorf("invalid scrubbing mode %q", s.Mode)
	}
	u, err := url.Parse(s.HubURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("invalid hub URL %q", s.HubURL)
	}
	return nil
}
