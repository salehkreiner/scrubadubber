//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"golang.org/x/sys/windows/registry"

	"github.com/salehkreiner/scrubadubber/internal/config"
	"github.com/salehkreiner/scrubadubber/internal/provision"
	"github.com/salehkreiner/scrubadubber/internal/version"
)

const (
	uninstallKey      = `Software\Microsoft\Windows\CurrentVersion\Uninstall\Scrubadubber`
	installerCopyName = "scrubadubber-setup.exe"
)

// finishInstall keeps a copy of this installer (for uninstall), creates a Start
// Menu shortcut to the tray app, and registers an Add/Remove Programs entry.
func finishInstall(binDir, appExe string) error {
	uninstaller := filepath.Join(binDir, installerCopyName)
	if self, err := os.Executable(); err == nil {
		_ = provision.CopyFile(self, uninstaller)
	}
	if err := createStartMenuShortcut(appExe); err != nil {
		logf("warning: Start Menu shortcut: %v", err)
	}
	return registerUninstaller(uninstaller, binDir)
}

func finishUninstall(_ string) error {
	_ = os.Remove(startMenuShortcutPath())
	return registry.DeleteKey(registry.CURRENT_USER, uninstallKey)
}

func startMenuShortcutPath() string {
	return filepath.Join(os.Getenv("APPDATA"), "Microsoft", "Windows", "Start Menu", "Programs", config.AppName+".lnk")
}

// createStartMenuShortcut writes a .lnk via the WScript.Shell COM object,
// avoiding a Go COM dependency in this trust repo.
func createStartMenuShortcut(target string) error {
	lnk := startMenuShortcutPath()
	if err := os.MkdirAll(filepath.Dir(lnk), 0o755); err != nil {
		return err
	}
	script := fmt.Sprintf(
		`$s=(New-Object -ComObject WScript.Shell).CreateShortcut(%q);$s.TargetPath=%q;$s.Save()`,
		lnk, target,
	)
	return exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script).Run()
}

func registerUninstaller(uninstaller, binDir string) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, uninstallKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	_ = k.SetStringValue("DisplayName", config.AppName)
	_ = k.SetStringValue("DisplayVersion", version.Version)
	_ = k.SetStringValue("Publisher", config.AppName)
	_ = k.SetStringValue("InstallLocation", binDir)
	_ = k.SetStringValue("UninstallString", fmt.Sprintf(`"%s" --uninstall`, uninstaller))
	_ = k.SetDWordValue("NoModify", 1)
	_ = k.SetDWordValue("NoRepair", 1)
	return nil
}

func pause() {
	fmt.Print("\nPress Enter to close…")
	_, _ = fmt.Scanln()
}
