package procs

import (
	"slices"
	"testing"
	"time"
)

func pids(g Group) []int {
	var out []int
	for _, p := range g.Members {
		out = append(out, p.PID)
	}
	return out
}

func TestClassify(t *testing.T) {
	ps := []Proc{
		{PID: 1, PPID: 0, Name: "systemd"},
		{PID: 10, PPID: 1, Name: "systemd"}, // systemd --user, a subreaper
		{PID: 20, PPID: 10, Name: "bash", TTY: true},
		{PID: 21, PPID: 20, Name: "claude", TTY: true, Start: 1000, Cwd: "/w/app"},
		{PID: 22, PPID: 21, Name: "bash", Marker: "claude"},
		{PID: 23, PPID: 22, Name: "node", Marker: "claude"},             // a dev server of a running agent
		{PID: 24, PPID: 21, Name: "claude", Marker: "claude"},           // an agent inside the agent: belongs to 21
		{PID: 30, PPID: 10, Name: "node", Marker: "claude", Start: 900}, // left running: its agent ended
		{PID: 31, PPID: 30, Name: "esbuild", Marker: "claude"},
		{PID: 32, PPID: 31, Name: "watcher"},                       // no marker, but under a process left running
		{PID: 40, PPID: 1, Name: "codex", Start: 800, Cwd: "/w/x"}, // agent whose terminal is gone
		{PID: 50, PPID: 20, Name: "vim", TTY: true},                // nothing to do with agents
		{PID: 60, PPID: 20, Name: "kiroku"},                        // kiroku itself
		{PID: 61, PPID: 60, Name: "claude", Marker: ""},            // kiroku's "Analyze with AI" run
		{PID: 62, PPID: 61, Name: "rg", Marker: "claude"},
		{PID: 70, PPID: 0, Name: "zsh", Marker: "claude"}, // parent not listed (another user's): still left running
	}
	gs := Classify(ps, 60)
	type want struct {
		root   int
		kind   string
		fam    string
		orphan bool
		pids   []int
	}
	exp := []want{
		{30, KindLeft, "claude", true, []int{30, 31, 32}}, // newest first
		{70, KindLeft, "claude", true, []int{70}},
		{40, KindAgent, "codex", true, []int{40}},
		{21, KindAgent, "claude", false, []int{21, 22, 24, 23}},
	}
	if len(gs) != len(exp) {
		t.Fatalf("got %d groups, want %d: %+v", len(gs), len(exp), gs)
	}
	for i, e := range exp {
		g := gs[i]
		if g.Root.PID != e.root || g.Kind != e.kind || g.Family != e.fam || g.Orphan != e.orphan || !slices.Equal(pids(g), e.pids) {
			t.Errorf("group %d = root %d %s %s orphan=%v %v, want %+v", i, g.Root.PID, g.Kind, g.Family, g.Orphan, pids(g), e)
		}
		if g.Self {
			t.Errorf("group %d is marked self", i)
		}
	}
	if gs[3].Parent != "bash" {
		t.Errorf("parent = %q, want bash", gs[3].Parent)
	}
}

// kiroku started from an agent's shell (while developing it, or kiroku serve run by the agent) is inside that
// agent's group: the group is shown, but marked Self so it can't be stopped from kiroku.
func TestClassifySelf(t *testing.T) {
	ps := []Proc{
		{PID: 1, Name: "launchd"},
		{PID: 5, PPID: 1, Name: "claude", TTY: true},
		{PID: 6, PPID: 5, Name: "zsh", Marker: "claude"},
		{PID: 7, PPID: 6, Name: "kiroku", Marker: "claude"},
		{PID: 8, PPID: 7, Name: "codex"}, // kiroku's own agent run: never reported
	}
	gs := Classify(ps, 7)
	if len(gs) != 1 || gs[0].Root.PID != 5 || !gs[0].Self || !slices.Equal(pids(gs[0]), []int{5, 6}) {
		t.Fatalf("got %+v", gs)
	}
	// kiroku itself left running by an ended agent is not offered either
	ps = []Proc{{PID: 1, Name: "launchd"}, {PID: 7, PPID: 1, Name: "kiroku", Marker: "claude"}}
	if gs := Classify(ps, 7); len(gs) != 0 {
		t.Fatalf("got %+v", gs)
	}
}

func TestAgentFamily(t *testing.T) {
	for name, fam := range map[string]string{"claude": "claude", "codex": "codex", "codex-x86_64-un": "codex", "kiro-cli": "kiro",
		"kiro-cli-chat": "kiro", "qchat": "amazonq", "node": "", "claude-helper": "", "q": "", "codexx": ""} {
		if got := agentFamily(name); got != fam {
			t.Errorf("agentFamily(%q) = %q, want %q", name, got, fam)
		}
	}
}

func TestMarkerOf(t *testing.T) {
	for _, c := range []struct {
		env  []string
		want string
	}{
		{[]string{"PATH=/bin", "CLAUDECODE=1"}, "claude"},
		{[]string{"CLAUDECODE=0"}, ""},
		{[]string{"CLAUDECODE"}, ""},
		{[]string{"CODEX_SANDBOX=seatbelt"}, "codex"},
		{[]string{"CODEX_SANDBOX_NETWORK_DISABLED=1"}, "codex"},
		{[]string{"XCLAUDECODE=1", "CLAUDE_CODE=1"}, ""},
		{nil, ""},
	} {
		if got := markerOf(c.env); got != c.want {
			t.Errorf("markerOf(%q) = %q, want %q", c.env, got, c.want)
		}
	}
}

func TestLink(t *testing.T) {
	refs := []Ref{
		{ID: "old", Family: "claude", Cwd: "/w/app", Start: 100, End: 200},
		{ID: "cur", Family: "claude", Cwd: "/w/app", Start: 1000, End: 5000},
		{ID: "after", Family: "claude", Cwd: "/w/app", Start: 5100, End: 6000}, // /clear in the same process
		{ID: "other", Family: "claude", Cwd: "/w/other", Start: 1000, End: 9000},
		{ID: "cx", Family: "codex", Cwd: "/w/app", Start: 1000, End: 9000},
		{ID: "nocwd", Family: "claude", Start: 1000, End: 9000},
	}
	gs := []Group{
		{Kind: KindAgent, Family: "claude", Root: Proc{Start: 900, Cwd: "/w/app"}},
		{Kind: KindLeft, Family: "claude", Root: Proc{Start: 1500, Cwd: "/w/app/web"}}, // cd'd into a subfolder
		{Kind: KindLeft, Family: "claude", Root: Proc{Start: 150, Cwd: "/w/app"}},
		{Kind: KindLeft, Family: "claude", Root: Proc{Start: 7000, Cwd: "/w/app"}}, // no session going on then
		{Kind: KindAgent, Family: "claude", Root: Proc{Start: 900, Cwd: "/w/apple"}},
		{Kind: KindAgent, Family: "claude", Root: Proc{Start: 900}},
		{Kind: KindAgent, Family: "codex", Root: Proc{Start: 900, Cwd: "/w/app"}},
	}
	Link(gs, refs)
	want := []string{"after", "cur", "old", "", "", "", "cx"}
	for i, g := range gs {
		if g.Session != want[i] {
			t.Errorf("group %d linked to %q, want %q", i, g.Session, want[i])
		}
	}
}

func TestParsePS(t *testing.T) {
	out := `    1     0     0 ??         5000   1:02.50 Mon Oct 12 09:00:00 2026     /sbin/launchd
  501     1   501 ttys001    20480   0:03.25 Thu Oct 15 10:04:05 2026     /Users/me/.local/bin/claude
  502   501   501 ??         10240 1-02:03:04 Thu Oct 15 10:05:00 2026     /Applications/My App.app/Contents/MacOS/My App
  503     1   502 ??           10   0:00.00 Thu Oct 15 10:05:00 2026     /usr/bin/other-user
garbage line
`
	ps := parsePS(out, 501, time.UTC)
	if len(ps) != 2 {
		t.Fatalf("got %+v", ps)
	}
	start := float64(time.Date(2026, 10, 15, 10, 4, 5, 0, time.UTC).Unix())
	if p := ps[0]; p.PID != 501 || p.PPID != 1 || !p.TTY || p.RSS != 20480*1024 || p.CPU != 3.25 || p.Start != start || p.Name != "claude" {
		t.Errorf("got %+v", p)
	}
	if p := ps[1]; p.TTY || p.Name != "My App" || p.CPU != 86400+2*3600+3*60+4 {
		t.Errorf("got %+v", p)
	}
}

func TestCPUTime(t *testing.T) {
	for s, want := range map[string]float64{"0:00.00": 0, "1:02.50": 62.5, "1:00:00": 3600, "2-00:00:01": 172801, "x": 0, "1-x": 0} {
		if got := cpuTime(s); got != want {
			t.Errorf("cpuTime(%q) = %v, want %v", s, got, want)
		}
	}
}

// The macOS commands: fixed arguments, the name last (it can hold spaces), and the PIDs as one comma-separated list.
func TestPSArgs(t *testing.T) {
	if !slices.Equal(psArgs, []string{"-axww", "-o", "pid=,ppid=,uid=,tty=,rss=,time=,lstart=,comm="}) {
		t.Errorf("psArgs = %q", psArgs)
	}
	if got := psEnvArgs([]int{7, 8}); !slices.Equal(got, []string{"-E", "-ww", "-o", "pid=,command=", "-p", "7,8"}) {
		t.Errorf("psEnvArgs = %q", got)
	}
}

func TestParsePSEnv(t *testing.T) {
	out := "  700 node server.js PATH=/usr/bin CLAUDECODE=1 HOME=/Users/me\n  701 sleep 100 NOTCLAUDECODE=1\n  702 sh -c echo CLAUDECODE=1x\n x y\n"
	got := parsePSEnv(out)
	if len(got) != 1 || got[700] != "claude" {
		t.Fatalf("got %v", got)
	}
}

func TestParseLsof(t *testing.T) {
	got := parseLsof("p501\nfcwd\nn/Users/me/app\np502\nfcwd\nn/Users/me/with space\n")
	if got[501] != "/Users/me/app" || got[502] != "/Users/me/with space" || len(got) != 2 {
		t.Fatalf("got %v", got)
	}
	if args := lsofArgs([]int{1, 2}); !slices.Equal(args, []string{"-a", "-d", "cwd", "-Fn", "-p", "1,2"}) {
		t.Fatalf("args %v", args)
	}
}

func TestStopChecksIdentity(t *testing.T) {
	g := Group{Members: []Proc{{PID: 1, Start: 5}, {PID: 999999, Start: 10}}}
	// PID 999999 has been reused by another process (another start time): it isn't signaled; PID 1 never is
	now := []Proc{{PID: 1, Start: 5}, {PID: 999999, Start: 50}}
	if n := Stop(g, now, 0, false); n != 0 {
		t.Fatalf("signaled %d processes", n)
	}
	if n := Running(g, now); n != 1 {
		t.Fatalf("running = %d, want 1 (only PID 1 is the same process)", n)
	}
}

func TestWithin(t *testing.T) {
	for _, c := range []struct {
		root, p string
		want    bool
	}{{"/a", "/a", true}, {"/a", "/a/b", true}, {"/a", "/ab", false}, {"/a/b", "/a", false}, {"/a", "/a/../b", false}, {"/a/", "/a/b/", true}} {
		if got := within(c.root, c.p); got != c.want {
			t.Errorf("within(%q, %q) = %v", c.root, c.p, got)
		}
	}
}
