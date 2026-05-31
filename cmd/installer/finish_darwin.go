//go:build darwin

package main

// On macOS the .app is the installer (the app calls provision on first run) and
// distribution is a drag-to-Applications .dmg, so there is no Start Menu
// shortcut or Add/Remove Programs entry to manage.

func finishInstall(_, _ string) error { return nil }

func finishUninstall(_ string) error { return nil }

func pause() {}
