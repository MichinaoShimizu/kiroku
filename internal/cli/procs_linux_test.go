//go:build linux

package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/MichinaoShimizu/kiroku/internal/procs"
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

// The view's Stop ends a real process left running by an agent's shell, and a second request finds it gone.
func TestProcsStopEndsProcess(t *testing.T) {
	old := stopWait
	stopWait = 5 * time.Second
	t.Cleanup(func() { stopWait = old })
	pid := leaveRunning(t, t.TempDir())
	var start float64
	for range 50 {
		ps, err := procs.List()
		if err != nil {
			t.Fatal(err)
		}
		for _, g := range procs.Classify(ps, os.Getpid()) {
			if g.Root.PID == pid && g.Kind == procs.KindLeft {
				start = g.Root.Start
			}
		}
		if start > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if start == 0 {
		t.Fatal("the process left running was not found")
	}
	l := procsLive()
	stop := func() *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "http://localhost:8485/procs/stop", strings.NewReader(`{"pid":`+strconv.Itoa(pid)+`,"start":`+strconv.FormatFloat(start, 'f', -1, 64)+`}`))
		r.Header.Set("X-Kiroku", "1")
		r.Header.Set("Origin", "http://localhost:8485")
		w := httptest.NewRecorder()
		l.handler().ServeHTTP(w, r)
		return w
	}
	w := stop()
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body)
	}
	var res map[string]int
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil || res["signaled"] != 1 || res["running"] != 0 {
		t.Fatalf("got %s (%v)", w.Body, err)
	}
	if w := stop(); w.Code != http.StatusNotFound {
		t.Fatalf("second stop: %d", w.Code)
	}
}
