//go:build darwin

package sysopen

import "os/exec"

// command opens target with the macOS `open` tool, which handles URLs and file
// paths alike.
func command(target string) *exec.Cmd {
	return exec.Command("open", target)
}
