//go:build windows

package ai

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// TestHelperProcess is not a test: the tests below run the test binary as a fake agent.
//   - KIROKU_AI_HELPER=parent: starts itself again as "child", prints the child's PID and waits
//   - KIROKU_AI_HELPER=parent-exit: the same, but exits right after printing (the child is left running)
//   - KIROKU_AI_HELPER=child: waits
func TestHelperProcess(t *testing.T) {
	mode := os.Getenv("KIROKU_AI_HELPER")
	if mode == "" {
		return
	}
	if mode == "child" {
		time.Sleep(time.Minute)
		os.Exit(0)
	}
	child := exec.Command(os.Args[0], "-test.run=^TestHelperProcess$")
	child.Env = append(os.Environ(), "KIROKU_AI_HELPER=child")
	if err := child.Start(); err != nil { // no stdout: the child doesn't hold the parent's output open
		os.Exit(2)
	}
	os.Stdout.WriteString(strconv.Itoa(child.Process.Pid) + "\n")
	if mode == "parent-exit" {
		os.Exit(0)
	}
	time.Sleep(time.Minute)
	os.Exit(0)
}

// startHelper runs the fake agent through ownGroup and returns the PID of the process it started.
func startHelper(t *testing.T, ctx context.Context, mode string) (*exec.Cmd, func(), int) {
	t.Helper()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestHelperProcess$")
	cmd.Env = append(os.Environ(), "KIROKU_AI_HELPER="+mode)
	cmd.WaitDelay = 2 * time.Second
	started, done := ownGroup(cmd)
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	started()
	line, err := bufio.NewReader(out).ReadString('\n')
	if err != nil {
		t.Fatalf("reading the child's PID: %v", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil {
		t.Fatalf("child PID %q: %v", line, err)
	}
	return cmd, done, pid
}

// ended waits up to 10 seconds for the process to end.
func ended(t *testing.T, pid int) bool {
	t.Helper()
	h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return true // already gone
	}
	defer windows.CloseHandle(h)
	ev, err := windows.WaitForSingleObject(h, 10_000)
	return err == nil && ev == windows.WAIT_OBJECT_0
}

// Stopping a run also stops what the agent started (node behind a .cmd shim), not only the agent.
func TestStopEndsAgentsChildren(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd, done, pid := startHelper(t, ctx, "parent")
	defer done()
	cancel()
	_ = cmd.Wait()
	if !ended(t, pid) {
		t.Fatal("the process the agent started is still running after the run was stopped")
	}
}

// A process the agent left behind when it exited is ended when the run is over.
func TestRunEndEndsLeftoverChildren(t *testing.T) {
	cmd, done, pid := startHelper(t, context.Background(), "parent-exit")
	if err := cmd.Wait(); err != nil {
		t.Fatalf("agent: %v", err)
	}
	done()
	if !ended(t, pid) {
		t.Fatal("the process the agent left behind is still running after the run")
	}
}
