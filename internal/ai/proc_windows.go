//go:build windows

package ai

import (
	"os/exec"
	"sync/atomic"
	"unsafe"

	"golang.org/x/sys/windows"
)

// ownGroup puts the agent in a job object of its own once it has started (started) and, when the run is
// stopped, ends the whole job, so processes the agent started don't keep running: node behind a .cmd shim
// (npm installs claude and codex that way, and stopping only cmd.exe left node running), a shell, a tool.
// done closes the job after the run, which also ends what is still left in it; so does kiroku exiting.
// If the job can't be made or the agent can't be put in it, only the agent is stopped, as before.
func ownGroup(cmd *exec.Cmd) (started, done func()) {
	job, err := newJob()
	if err != nil {
		return func() {}, func() {}
	}
	var inJob atomic.Bool
	cmd.Cancel = func() error {
		if inJob.Load() && windows.TerminateJobObject(job, 1) == nil {
			return nil
		}
		return cmd.Process.Kill()
	}
	started = func() {
		if cmd.Process == nil {
			return
		}
		h, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(cmd.Process.Pid))
		if err != nil {
			return
		}
		defer windows.CloseHandle(h)
		if windows.AssignProcessToJobObject(job, h) == nil {
			inJob.Store(true)
		}
	}
	return started, func() { windows.CloseHandle(job) }
}

// newJob makes an unnamed job whose processes all end when its last handle is closed. The handle isn't
// inherited, so only kiroku holds it.
func newJob() (windows.Handle, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return 0, err
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE},
	}
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		windows.CloseHandle(job)
		return 0, err
	}
	return job, nil
}
