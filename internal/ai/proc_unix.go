//go:build !windows

package ai

import (
	"os/exec"
	"syscall"
)

// ownGroup runs the agent in a process group of its own and, when the run is stopped, kills the whole group,
// so processes the agent started (a shell, a tool) don't keep running and holding its output open.
func ownGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
