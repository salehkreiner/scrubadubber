//go:build windows

package hubmanager

import "os/exec"

// terminate stops the Hub process. Windows has no POSIX signals for console
// apps that we can rely on the Hub handling, so we kill directly. (A graceful
// HTTP-shutdown call could be added here if the Hub exposes one — open item.)
func terminate(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}
