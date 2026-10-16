//go:build !windows

package ai

import (
	"os/exec"
	"syscall"
)

// ownGroup runs the agent in a process group of its own and, when the run is stopped, kills the whole group,
// so processes the agent started (a shell, a tool) don't keep running and holding its output open.
// started and done have nothing to do here (Windows uses them for its job object).
func ownGroup(cmd *exec.Cmd) (started, done func()) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	return func() {}, func() {}
}
