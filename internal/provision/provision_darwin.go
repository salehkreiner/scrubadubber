//go:build darwin

package provision

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

const pathMarker = "# added by Scrubadubber"

func zshrcPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".zshrc"), nil
}

// addToUserPath ensures ~/.zshrc prepends dir to PATH (idempotent). scrub-setup
// handles ANTHROPIC_BASE_URL; this only guarantees the bin dir is on PATH.
func addToUserPath(dir string) error {
	p, err := zshrcPath()
	if err != nil {
		return err
	}
	data, _ := os.ReadFile(p)
	if strings.Contains(string(data), dir) {
		return nil
	}
	line := "\n" + pathMarker + "\nexport PATH=\"" + dir + ":$PATH\"\n"
	f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(line)
	return err
}

// removeFromUserPath strips the lines this installer added to ~/.zshrc.
func removeFromUserPath(dir string) error {
	p, err := zshrcPath()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	lines := strings.Split(string(data), "\n")
	kept := make([]string, 0, len(lines))
	for _, l := range lines {
		if strings.TrimSpace(l) == pathMarker || strings.Contains(l, dir) {
			continue
		}
		kept = append(kept, l)
	}
	return os.WriteFile(p, []byte(strings.Join(kept, "\n")), 0o644)
}

// detachedSysProcAttr needs no special attributes on macOS (the installer is
// not run under a captured pipe there). Returning nil keeps default behavior.
func detachedSysProcAttr() *syscall.SysProcAttr { return nil }
