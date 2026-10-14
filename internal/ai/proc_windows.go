//go:build windows

package ai

import "os/exec"

// ownGroup: on Windows the agent alone is stopped; Stream closes its output after a delay, so a process
// it started can't keep the run going.
func ownGroup(cmd *exec.Cmd) {}
