// Package startup registers (or removes) the tray/menubar app as a login item
// so it launches automatically when the user signs in. The platform
// implementations live in startup_windows.go (registry Run key) and
// startup_darwin.go (LaunchAgent plist).
//
// It registers the app — not the Hub — because the app supervises the Hub.
package startup

import (
	"os"
	"path/filepath"
)

// Manager enables/disables launch-at-login for the app.
type Manager interface {
	// Enable registers the app to start on login.
	Enable() error
	// Disable removes the login registration (no-op if absent).
	Disable() error
	// IsEnabled reports whether the login registration is present.
	IsEnabled() (bool, error)
}

// ExecPath returns the absolute path to the running executable, with symlinks
// resolved. Useful as the default target for New.
func ExecPath() (string, error) {
	p, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(p); err == nil {
		p = resolved
	}
	return p, nil
}
