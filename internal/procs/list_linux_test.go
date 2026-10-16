//go:build linux

package procs

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

func TestParseStat(t *testing.T) {
	// a name with spaces and parentheses, as a process can set for itself
	b := []byte("4242 (we (ird) name) S 1 4242 4242 34817 4242 4194560 1 0 0 0 250 50 0 0 20 0 1 0 12345 1000000 300 18446744073709551615")
	p, ok := parseStat(b, 1_700_000_000, 4096)
	if !ok {
		t.Fatal("not parsed")
	}
	want := Proc{PID: 4242, PPID: 1, Name: "we (ird) name", TTY: true, CPU: 3, Start: 1_700_000_000 + 123.45, RSS: 300 * 4096}
	if p != want {
		t.Fatalf("got %+v, want %+v", p, want)
	}
	if _, ok := parseStat([]byte("9 (z) Z 1 9 9 0 -1 0 0 0 0 0 0 0 0 0 20 0 1 0 1 0 0 0"), 0, 4096); ok {
		t.Error("a zombie was kept")
	}
	for _, bad := range []string{"", "12", "x (a) S 1", "12 (a) S 1 2 3", "12 (a) S 99999999999999999999 1 1 0 -1 0 0 0 0 0 0 0 0 0 20 0 1 0 1 0 0"} {
		if _, ok := parseStat([]byte(bad), 0, 4096); ok {
			t.Errorf("parsed %q", bad)
		}
	}
}

// A process can give itself an environment of any size (prctl PR_SET_MM_MAP, no privilege needed). kiroku reads it
// in pieces and only up to envMax, so memory stays small, a marker within the limit is still found, and one past it
// is not read.
func TestEnvironMarkerBounded(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, parts ...[]byte) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, bytes.Join(parts, nil), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	nuls := func(n int) []byte { return make([]byte, n) }
	long := bytes.Repeat([]byte("A"), envEntryMax+10)
	for _, c := range []struct {
		name string
		path string
		want string
	}{
		{"plain", write("plain", []byte("PATH=/bin\x00CLAUDECODE=1\x00HOME=/h\x00")), "claude"},
		{"last, no NUL", write("last", []byte("PATH=/bin\x00CODEX_SANDBOX=seatbelt")), "codex"},
		{"after many NULs", write("nuls", nuls(1<<20), []byte("CLAUDECODE=1\x00")), "claude"},
		{"past the limit", write("past", nuls(envMax), []byte("CLAUDECODE=1\x00")), ""},
		{"after a too long entry", write("long", long, []byte("\x00CLAUDECODE=1\x00")), ""},
		{"missing", filepath.Join(dir, "none"), ""},
	} {
		var before, after runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&before)
		got := environMarker(c.path)
		runtime.ReadMemStats(&after)
		if got != c.want {
			t.Errorf("%s: marker %q, want %q", c.name, got, c.want)
		}
		if d := after.TotalAlloc - before.TotalAlloc; d > 1<<20 {
			t.Errorf("%s: allocated %d bytes", c.name, d)
		}
	}
}
