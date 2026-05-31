// Package bridgecheck reports whether the bridge has configured the shell
// environment (ANTHROPIC_BASE_URL) to route LLM traffic through the Hub.
//
// The tray process's own environment is an unreliable signal — a macOS
// LaunchAgent does not inherit ~/.zshrc exports — so each platform inspects
// what scrub-setup persisted instead (see bridgecheck_windows.go /
// bridgecheck_darwin.go).
package bridgecheck

// envVar is the variable the bridge sets to point tools at the Hub.
const envVar = "ANTHROPIC_BASE_URL"
