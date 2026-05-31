//go:build windows

package sysopen

import "os/exec"

// command opens target via the shell file/URL protocol handler. rundll32 is a
// GUI process, so there is no console-window flash, and FileProtocolHandler
// handles both http(s) URLs and local file paths.
func command(target string) *exec.Cmd {
	return exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
}
