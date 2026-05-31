//go:build windows

package startup

import (
	"errors"
	"strings"

	"golang.org/x/sys/windows/registry"

	"github.com/salehkreiner/scrubadubber/internal/config"
)

// runKeyPath is the per-user "run at login" key. Writing here needs no admin
// rights, matching the elevation-free install model.
const runKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`

type windowsManager struct {
	execPath string
	args     []string
}

// New returns a Windows login-item manager that registers execPath (with the
// given args) under HKCU\...\Run.
func New(execPath string, args ...string) Manager {
	return &windowsManager{execPath: execPath, args: args}
}

// command renders the registry value: a quoted exe path followed by args.
func (w *windowsManager) command() string {
	var b strings.Builder
	b.WriteString(`"`)
	b.WriteString(w.execPath)
	b.WriteString(`"`)
	for _, a := range w.args {
		b.WriteString(" ")
		b.WriteString(a)
	}
	return b.String()
}

func (w *windowsManager) Enable() error {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	return k.SetStringValue(config.AppName, w.command())
}

func (w *windowsManager) Disable() error {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if err := k.DeleteValue(config.AppName); err != nil && !errors.Is(err, registry.ErrNotExist) {
		return err
	}
	return nil
}

func (w *windowsManager) IsEnabled() (bool, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return false, err
	}
	defer k.Close()
	if _, _, err := k.GetStringValue(config.AppName); err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
