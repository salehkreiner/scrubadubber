//go:build darwin

package bridgecheck

import (
	"os"
	"path/filepath"
	"strings"
)

// Configured reports whether the user's ~/.zshrc references
// ANTHROPIC_BASE_URL, which is what scrub-setup writes on macOS. We inspect the
// persisted profile rather than the live environment because the menubar
// LaunchAgent does not inherit interactive-shell exports.
func Configured() bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	data, err := os.ReadFile(filepath.Join(home, ".zshrc"))
	if err != nil {
		return false
	}
	return strings.Contains(string(data), envVar)
}
