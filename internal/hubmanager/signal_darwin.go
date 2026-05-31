//go:build darwin

package hubmanager

import (
	"os/exec"
	"syscall"
)

// terminate asks the Hub to stop gracefully via SIGTERM. The caller escalates
// to Kill if the process does not exit within the stop grace period.
func terminate(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	return cmd.Process.Signal(syscall.SIGTERM)
}
