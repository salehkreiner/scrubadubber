//go:build windows

package provision

import (
	"errors"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const envKey = `Environment`

// addToUserPath appends dir to the per-user PATH (HKCU\Environment) — no admin
// required — and broadcasts the change so new shells pick it up.
func addToUserPath(dir string) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, envKey, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()

	cur, _, err := k.GetStringValue("Path")
	if err != nil && !errors.Is(err, registry.ErrNotExist) {
		return err
	}
	if pathHas(cur, dir) {
		return nil
	}
	updated := cur
	if updated != "" && !strings.HasSuffix(updated, ";") {
		updated += ";"
	}
	updated += dir
	// Write as REG_EXPAND_SZ so any existing %VAR% entries keep expanding.
	if err := k.SetExpandStringValue("Path", updated); err != nil {
		return err
	}
	broadcastEnv()
	return nil
}

// removeFromUserPath removes dir from the per-user PATH.
func removeFromUserPath(dir string) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, envKey, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()

	cur, _, err := k.GetStringValue("Path")
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return nil
		}
		return err
	}
	parts := strings.Split(cur, ";")
	kept := parts[:0]
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t == "" || strings.EqualFold(t, dir) {
			continue
		}
		kept = append(kept, p)
	}
	updated := strings.Join(kept, ";")
	if updated == cur {
		return nil
	}
	if err := k.SetExpandStringValue("Path", updated); err != nil {
		return err
	}
	broadcastEnv()
	return nil
}

func pathHas(list, dir string) bool {
	for _, p := range strings.Split(list, ";") {
		if strings.EqualFold(strings.TrimSpace(p), dir) {
			return true
		}
	}
	return false
}

// broadcastEnv notifies running processes that the environment changed.
func broadcastEnv() {
	const (
		hwndBroadcast   = 0xffff
		wmSettingChange = 0x001A
		smtoAbortIfHung = 0x0002
	)
	user32 := windows.NewLazySystemDLL("user32.dll")
	send := user32.NewProc("SendMessageTimeoutW")
	env, err := windows.UTF16PtrFromString("Environment")
	if err != nil {
		return
	}
	_, _, _ = send.Call(
		uintptr(hwndBroadcast),
		uintptr(wmSettingChange),
		0,
		uintptr(unsafe.Pointer(env)),
		uintptr(smtoAbortIfHung),
		5000,
		0,
	)
}

// detachedSysProcAttr launches the child with no inherited handles and no
// console, so a long-lived child (the tray) never holds open the installer's
// stdio — which may be a pipe (e.g. `irm … | iex`). NoInheritHandles forces
// CreateProcess(bInheritHandles=FALSE) — not even the standard handles are
// inherited; DETACHED_PROCESS drops the console.
func detachedSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		CreationFlags:    windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP,
		NoInheritHandles: true,
	}
}
