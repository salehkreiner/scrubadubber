// Command installer is the Scrubadubber setup program (shipped as
// scrubadubber-setup.exe on Windows). It downloads and installs the app, Hub,
// and bridge binaries, configures PATH and the shell, registers start-on-login,
// and launches the tray app — all per-user, with no elevation.
//
// Run with --uninstall to remove the installation.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/salehkreiner/scrubadubber/internal/config"
	"github.com/salehkreiner/scrubadubber/internal/provision"
	"github.com/salehkreiner/scrubadubber/internal/version"
)

func main() {
	uninstall := flag.Bool("uninstall", false, "remove Scrubadubber")
	flag.Parse()

	if *uninstall {
		runUninstall()
		return
	}
	runInstall()
}

func logf(format string, args ...any) { fmt.Printf(format+"\n", args...) }

func runInstall() {
	fmt.Printf("Installing %s %s\n\n", config.AppName, version.Version)

	binDir, err := config.BinDir()
	if err != nil {
		fatal(err)
	}
	appExe := filepath.Join(binDir, config.AppBinaryName())

	opts := provision.Options{
		AppVersion:    appVersionToInstall(),
		HubVersion:    config.PinnedHubVersion,
		BridgeVersion: config.PinnedBridgeVersion,
		StartupTarget: appExe,
		LaunchTarget:  appExe,
		Log:           logf,
	}
	if err := provision.Install(context.Background(), opts); err != nil {
		fatal(err)
	}

	// Platform finishing: keep a copy of this installer for uninstall, create a
	// Start Menu shortcut, and register the uninstaller (Windows only).
	if err := finishInstall(binDir, appExe); err != nil {
		logf("warning: %v", err)
	}

	fmt.Printf("\n%s is installed and running in your tray.\n", config.AppName)
	fmt.Println("Open a new PowerShell window and run `claude` as usual — traffic is now protected.")
	pause()
}

func runUninstall() {
	binDir, _ := config.BinDir()
	appExe := filepath.Join(binDir, config.AppBinaryName())

	if err := provision.Uninstall(provision.Options{StartupTarget: appExe, Log: logf}); err != nil {
		logf("warning: %v", err)
	}
	if err := finishUninstall(binDir); err != nil {
		logf("warning: %v", err)
	}
	fmt.Printf("%s has been removed.\n", config.AppName)
	pause()
}

// appVersionToInstall installs the app release matching this installer's version
// tag, or the latest release for untagged (dev) builds.
func appVersionToInstall() string {
	if version.Version == "dev" {
		return "latest"
	}
	return version.Version
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	pause()
	os.Exit(1)
}
