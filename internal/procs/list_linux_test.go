//go:build linux

package procs

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestHelperLeaveRunning is not a test. Run by leaveRunning with KIROKU_LEAVE_RUNNING=1, it starts sleep with
// Claude Code's marker and exits without waiting, so sleep is left running like a dev server whose agent ended.
func TestHelperLeaveRunning(t *testing.T) {
	if os.Getenv("KIROKU_LEAVE_RUNNING") != "1" {
		t.Skip("helper for leaveRunning")
	}
	cmd := exec.Command("sleep", "60")
	cmd.Env = append(os.Environ(), "CLAUDECODE=1")
	if err := cmd.Start(); err != nil {
		os.Exit(1)
	}
	fmt.Println(cmd.Process.Pid)
	os.Exit(0)
}

// leaveRunning starts a process in dir that is left running after its parent ended, and returns its PID.
func leaveRunning(t *testing.T, dir string) int {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestHelperLeaveRunning$")
	cmd.Env = append(os.Environ(), "KIROKU_LEAVE_RUNNING=1")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0]))
	if err != nil {
		t.Fatalf("helper printed %q", out)
	}
	t.Cleanup(func() {
		if p, err := os.FindProcess(pid); err == nil {
			_ = p.Kill()
		}
	})
	return pid
}

// A process started by an agent's shell that outlives the shell is found as left running, with its folder,
// and stopping it ends it.
func TestListFindsAndStopsLeftRunning(t *testing.T) {
	dir := t.TempDir()
	pid := leaveRunning(t, dir)
	find := func() (Group, []Proc, bool) {
		ps, err := List()
		if err != nil {
			t.Fatal(err)
		}
		for _, g := range Classify(ps, os.Getpid()) {
			if g.Root.PID == pid {
				return g, ps, true
			}
		}
		return Group{}, ps, false
	}
	var g Group
	var ok bool
	for range 50 { // the shell may not have exited yet
		if g, _, ok = find(); ok {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !ok {
		t.Fatal("the process left running was not found")
	}
	if g.Kind != KindLeft || g.Family != "claude" || g.Root.Name != "sleep" || g.Root.Cwd != dir || g.Root.Start <= 0 || g.Self {
		t.Fatalf("got %+v", g)
	}
	if d := time.Since(time.Unix(int64(g.Root.Start), 0)); d < -5*time.Second || d > time.Minute {
		t.Fatalf("start time is off by %v", d)
	}
	ps, _ := List()
	if n := Stop(g, ps, os.Getpid(), false); n != 1 {
		t.Fatalf("signaled %d processes", n)
	}
	for range 100 {
		if _, _, ok = find(); !ok {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("the process is still running after Stop")
}

// Processes of other users are not listed (here: PID 1, when another user owns it).
func TestListOnlyThisUser(t *testing.T) {
	ps, err := List()
	if err != nil {
		t.Fatal(err)
	}
	self := false
	for _, p := range ps {
		if p.PID == os.Getpid() {
			self = true
		}
	}
	if !self {
		t.Fatal("this test's own process is not listed")
	}
	fi, err := os.Stat("/proc/1")
	if err != nil {
		t.Skip(err)
	}
	if st, ok := fi.Sys().(*syscall.Stat_t); !ok || int(st.Uid) == os.Getuid() {
		t.Skip("PID 1 belongs to this user")
	}
	for _, p := range ps {
		if p.PID == 1 {
			t.Fatalf("PID 1 is listed: %+v", p)
		}
	}
}
