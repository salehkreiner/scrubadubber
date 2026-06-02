//go:build darwin

package bridgecheck

import (
	"os"
	"path/filepath"
)

// profilePaths returns the shell profile files scrub-setup may have written to
// on macOS (zsh is the default; bash variants included for completeness).
func profilePaths() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	return []string{
		filepath.Join(home, ".zshrc"),
		filepath.Join(home, ".zprofile"),
		filepath.Join(home, ".bashrc"),
		filepath.Join(home, ".bash_profile"),
	}
}
