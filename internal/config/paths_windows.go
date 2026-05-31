//go:build windows

package config

import (
	"os"
	"path/filepath"
)

// dataDir returns %LOCALAPPDATA%\scrubadubber.
//
// On Windows, os.UserCacheDir() resolves to %LocalAppData%, which is the
// per-user, non-roaming location CLAUDE.md specifies for the install.
func dataDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "scrubadubber"), nil
}
