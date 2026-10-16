package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/MichinaoShimizu/kiroku/internal/core"
	"github.com/MichinaoShimizu/kiroku/internal/procs"
)

// fakeProcs replaces the process list for a test.
func fakeProcs(t *testing.T, ps []procs.Proc) {
	t.Helper()
	old := listProcs
	listProcs = func() ([]procs.Proc, error) { return append([]procs.Proc(nil), ps...), nil }
	t.Cleanup(func() { listProcs = old })
}

func procsLive(data ...*core.Session) *live {
	l := &live{ready: true, print: &bytes.Buffer{}}
	l.snap.data = data
	l.procs = l.procsHandler()
	return l
}

// With the list off (--no-procs, an address that isn't a loopback IP, an unsupported OS), both paths are 404.
func TestProcsNotFoundWhenOff(t *testing.T) {
	l := &live{ready: true}
	for _, r := range []*http.Request{
		httptest.NewRequest(http.MethodGet, "http://localhost:8485/procs", nil),
		httptest.NewRequest(http.MethodPost, "http://localhost:8485/procs/stop", strings.NewReader(`{"pid":1234,"start":1}`)),
	} {
		r.Header.Set("X-Kiroku", "1")
		w := httptest.NewRecorder()
		l.handler().ServeHTTP(w, r)
		if w.Code != http.StatusNotFound {
			t.Errorf("%s %s: want 404, got %d", r.Method, r.URL.Path, w.Code)
		}
	}
}

// /procs lists the groups with the session each belongs to, and never a command line or the environment.
func TestProcsList(t *testing.T) {
	fakeProcs(t, []procs.Proc{
		{PID: 1, Name: "systemd"},
		{PID: 4242, PPID: 1, Name: "node", Marker: "claude", Start: 1500, Cwd: "/w/app", RSS: 2 << 20, CPU: 61},
		{PID: 4243, PPID: 4242, Name: "esbuild", Marker: "claude", RSS: 1 << 20, CPU: 10},
	})
	l := procsLive(&core.Session{ID: "s1", Source: "Claude Code", ProjectPath: "/w/app", Start: 1000, End: 2000})
	w := httptest.NewRecorder()
	l.handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "http://localhost:8485/procs", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body)
	}
	var j struct {
		Procs []map[string]any `json:"procs"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &j); err != nil {
		t.Fatal(err)
	}
	if len(j.Procs) != 1 {
		t.Fatalf("got %s", w.Body)
	}
	p := j.Procs[0]
	if p["pid"] != 4242.0 || p["kind"] != "left" || p["family"] != "claude" || p["session"] != "s1" || p["n"] != 2.0 || p["rss"] != float64(3<<20) || p["cpu"] != 71.0 || p["cwd"] != "/w/app" {
		t.Fatalf("got %v", p)
	}
	for _, k := range []string{"cmd", "args", "command", "env", "environ"} {
		if _, ok := p[k]; ok {
			t.Errorf("/procs carries %q", k)
		}
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Error("/procs can be cached")
	}
	// POST is not a way to list
	w = httptest.NewRecorder()
	l.handler().ServeHTTP(w, httptest.NewRequest(http.MethodPost, "http://localhost:8485/procs", nil))
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /procs: %d", w.Code)
	}
}

// /procs/stop refuses everything but the view's own request for a process kiroku reports now.
func TestProcsStopRefusals(t *testing.T) {
	self := os.Getpid()
	fakeProcs(t, []procs.Proc{
		{PID: 1, Name: "launchd"},
		{PID: 4242, PPID: 1, Name: "node", Marker: "claude", Start: 1500},
		{PID: 5000, PPID: 1, Name: "claude", TTY: true, Start: 1000},
		{PID: self, PPID: 5000, Name: "kiroku", Marker: "claude", Start: 1100}, // kiroku inside an agent: Self
		{PID: 6000, PPID: 1, Name: "vim", Start: 1000},                         // not an agent's
	})
	l := procsLive()
	for _, c := range []struct {
		name, method, body, origin string
		header                     bool
		want                       int
	}{
		{"GET", http.MethodGet, `{"pid":4242,"start":1500}`, "", true, http.StatusForbidden},
		{"no X-Kiroku", http.MethodPost, `{"pid":4242,"start":1500}`, "", false, http.StatusForbidden},
		{"another origin", http.MethodPost, `{"pid":4242,"start":1500}`, "http://evil.example", true, http.StatusForbidden},
		{"bad body", http.MethodPost, `pid=4242`, "", true, http.StatusBadRequest},
		{"PID 1", http.MethodPost, `{"pid":1,"start":0}`, "", true, http.StatusBadRequest},
		{"negative PID", http.MethodPost, `{"pid":-1,"start":0}`, "", true, http.StatusBadRequest},
		{"too long", http.MethodPost, `{"pid":4242,"start":1500,"x":"` + strings.Repeat("a", 2000) + `"}`, "", true, http.StatusBadRequest},
		{"reused PID", http.MethodPost, `{"pid":4242,"start":1400}`, "", true, http.StatusNotFound},
		{"not reported", http.MethodPost, `{"pid":6000,"start":1000}`, "", true, http.StatusNotFound},
		{"a member, not a root", http.MethodPost, `{"pid":` + itoa(self) + `,"start":1100}`, "", true, http.StatusNotFound},
		{"kiroku inside", http.MethodPost, `{"pid":5000,"start":1000}`, "", true, http.StatusForbidden},
	} {
		r := httptest.NewRequest(c.method, "http://localhost:8485/procs/stop", strings.NewReader(c.body))
		if c.header {
			r.Header.Set("X-Kiroku", "1")
		}
		if c.origin != "" {
			r.Header.Set("Origin", c.origin)
		}
		w := httptest.NewRecorder()
		l.handler().ServeHTTP(w, r)
		if w.Code != c.want {
			t.Errorf("%s: want %d, got %d (%s)", c.name, c.want, w.Code, strings.TrimSpace(w.Body.String()))
		}
	}
}

func itoa(n int) string { b, _ := json.Marshal(n); return string(b) }

// The view learns from meta.procs whether to ask /procs; the key-less shell carries it too (it is not history).
func TestProcsFlagInMeta(t *testing.T) {
	for _, on := range []bool{false, true} {
		l := &live{load: func() snapshot { return snapshot{meta: map[string]any{"x": 1}} }}
		if on {
			l.procs = l.procsHandler()
		}
		if err := l.refresh(); err != nil {
			t.Fatal(err)
		}
		var j struct {
			Meta map[string]any `json:"meta"`
		}
		if err := json.Unmarshal(l.json, &j); err != nil {
			t.Fatal(err)
		}
		if got := j.Meta["procs"] == true; got != on {
			t.Errorf("procs on=%v: meta.procs = %v", on, j.Meta["procs"])
		}
		if got := strings.Contains(string(l.html), `"procs":true`); got != on {
			t.Errorf("procs on=%v: shell carries procs = %v", on, got)
		}
	}
}

func TestProcsStartLine(t *testing.T) {
	if s := procsStartLine(true, true, true); !strings.Contains(s, "--no-procs") || !strings.Contains(s, "after you confirm") {
		t.Errorf("on: %q", s)
	}
	if s := procsStartLine(false, true, true); !strings.Contains(s, "off (--no-procs)") {
		t.Errorf("--no-procs: %q", s)
	}
	if s := procsStartLine(true, false, true); !strings.Contains(s, "off") || !strings.Contains(s, "loopback") {
		t.Errorf("not loopback: %q", s)
	}
	if s := procsStartLine(true, true, false); s != "" {
		t.Errorf("unsupported OS: %q", s)
	}
}

func TestNoProcsEnv(t *testing.T) {
	for v, want := range map[string]bool{"": false, "0": false, "false": false, " FALSE ": false, "1": true, "yes": true, "off": true} {
		t.Setenv("KIROKU_NO_PROCS", v)
		if got := noProcsEnv(); got != want {
			t.Errorf("KIROKU_NO_PROCS=%q: %v, want %v", v, got, want)
		}
	}
}

// kiroku ps prints what it found with the stop command, strips terminal control characters from process names and
// titles, and gives no stop command for the group kiroku runs in.
func TestWritePS(t *testing.T) {
	now := time.Unix(10000, 0)
	gs := []procs.Group{
		{Kind: procs.KindLeft, Family: "claude", Session: "s1", Orphan: true,
			Root:    procs.Proc{PID: 4242, Name: "no\x1b[31mde", Start: 10000 - 3*3600, Cwd: "/w/app"},
			Members: []procs.Proc{{PID: 4242, RSS: 3 << 30, CPU: 600}, {PID: 4243}}},
		{Kind: procs.KindAgent, Family: "codex", Self: true, Root: procs.Proc{PID: 5000, Name: "codex", TTY: true, Start: 9000},
			Members: []procs.Proc{{PID: 5000}}},
	}
	data := []*core.Session{{ID: "s1", Title: "fix\x07 login", Source: "Claude Code", Start: 1000, End: 2000}}
	var b bytes.Buffer
	if err := writePS(&b, gs, data, now); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	for _, s := range []string{"Left running", "no[31mde", "PID 4242", "2 processes", "started by Claude Code, now running on its own",
		"running for 3h 0m", "3.0 GB memory", "10m CPU", "/w/app", "session: fix login", "kill 4242 4243", "Running agents", "open in a terminal", "kiroku runs inside it"} {
		if !strings.Contains(out, s) {
			t.Errorf("output lacks %q:\n%s", s, out)
		}
	}
	if strings.Contains(out, "\x1b[31m") || strings.Contains(out, "\x07") || strings.Contains(out, "kill 5000") {
		t.Errorf("unsafe output:\n%q", out)
	}
	b.Reset()
	_ = writePS(&b, nil, nil, now)
	if !strings.Contains(b.String(), "No AI agent is running") {
		t.Errorf("empty: %s", b.String())
	}
}

// The log line of a stop carries no terminal control characters from the process name (a process sets its own name).
func TestStopLineIsTerminalSafe(t *testing.T) {
	got := stopLine("\x1b[2J\x1b]0;X\a\nZ", 42, 3, true)
	if strings.ContainsAny(got[:len(got)-1], "\x1b\a\n") || !strings.Contains(got, "PID 42") || !strings.Contains(got, "signaled 3 (SIGKILL)") || !strings.HasSuffix(got, "\n") {
		t.Fatalf("got %q", got)
	}
}
