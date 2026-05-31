//go:build darwin

package startup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/salehkreiner/scrubadubber/internal/config"
)

type darwinManager struct {
	execPath string
	args     []string
}

// New returns a macOS login-item manager backed by a per-user LaunchAgent.
func New(execPath string, args ...string) Manager {
	return &darwinManager{execPath: execPath, args: args}
}

func (d *darwinManager) plistPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", config.AppID+".plist"), nil
}

func (d *darwinManager) plistContents() string {
	var args strings.Builder
	args.WriteString("    <string>" + xmlEscape(d.execPath) + "</string>\n")
	for _, a := range d.args {
		args.WriteString("    <string>" + xmlEscape(a) + "</string>\n")
	}
	return `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>` + config.AppID + `</string>
  <key>ProgramArguments</key>
  <array>
` + args.String() + `  </array>
  <key>RunAtLoad</key>
  <true/>
  <key>ProcessType</key>
  <string>Interactive</string>
</dict>
</plist>
`
}

func (d *darwinManager) Enable() error {
	p, err := d.plistPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(p, []byte(d.plistContents()), 0o644); err != nil {
		return err
	}
	// Reload so changes take effect immediately. bootout may fail if not yet
	// loaded — that's fine.
	domain := fmt.Sprintf("gui/%d", os.Getuid())
	_ = exec.Command("launchctl", "bootout", domain, p).Run()
	if err := exec.Command("launchctl", "bootstrap", domain, p).Run(); err != nil {
		// Older macOS: fall back to the legacy load verb.
		return exec.Command("launchctl", "load", "-w", p).Run()
	}
	return nil
}

func (d *darwinManager) Disable() error {
	p, err := d.plistPath()
	if err != nil {
		return err
	}
	domain := fmt.Sprintf("gui/%d", os.Getuid())
	_ = exec.Command("launchctl", "bootout", domain, p).Run()
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (d *darwinManager) IsEnabled() (bool, error) {
	p, err := d.plistPath()
	if err != nil {
		return false, err
	}
	if _, err := os.Stat(p); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// xmlEscape escapes the characters that are significant inside an XML/plist
// <string> element.
func xmlEscape(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&apos;",
	)
	return r.Replace(s)
}
