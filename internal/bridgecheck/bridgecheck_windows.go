//go:build windows

package bridgecheck

import (
	"os"
	"path/filepath"
)

// profilePaths returns the PowerShell profile files scrub-setup may have written
// to — both Windows PowerShell 5.1 and PowerShell 7, per-user, current-host and
// all-hosts variants.
func profilePaths() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	docs := filepath.Join(home, "Documents")
	return []string{
		filepath.Join(docs, "PowerShell", "profile.ps1"),
		filepath.Join(docs, "PowerShell", "Microsoft.PowerShell_profile.ps1"),
		filepath.Join(docs, "WindowsPowerShell", "profile.ps1"),
		filepath.Join(docs, "WindowsPowerShell", "Microsoft.PowerShell_profile.ps1"),
	}
}
