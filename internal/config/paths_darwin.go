//go:build darwin

package config

import (
	"os"
	"path/filepath"
)

// dataDir returns ~/Library/Application Support/scrubadubber.
//
// On macOS, os.UserConfigDir() resolves to ~/Library/Application Support,
// the per-user location CLAUDE.md specifies for the install.
func dataDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "scrubadubber"), nil
}
