// Package procs finds the AI agents running on this computer and the processes they left running
// after they ended, such as a dev server started by the agent's shell tool (ADR 0035).
//
// It reads only this user's processes, and from them only what it needs: the PID and parent PID, the start time,
// the short process name, the working folder, whether a terminal is attached, memory and CPU time, and whether
// the environment carries a marker that an agent sets for the commands it runs. It never keeps or returns the
// command line or the environment.
package procs

import (
	"errors"
	"math"
	"path/filepath"
	"slices"
	"strings"
)

// ErrUnsupported means this OS is not supported (Windows and others).
var ErrUnsupported = errors.New("listing processes is not supported on this OS")

// Proc is one process of this user.
type Proc struct {
	PID    int
	PPID   int
	Start  float64 // UNIX seconds
	Name   string  // short process name (Linux comm, the base name of the executable on macOS)
	Cwd    string  // working folder ("" when it can't be read)
	TTY    bool    // a terminal is attached
	RSS    int64   // resident memory in bytes
	CPU    float64 // CPU seconds used so far
	Marker string  // agent family from an environment marker (markers), "" when none or unreadable
}

// Group is a process kiroku reports, with every process under it.
type Group struct {
	Root    Proc
	Kind    string // KindAgent or KindLeft
	Family  string // agent family: claude, codex, kiro, amazonq
	Parent  string // name of the root's parent process ("" when it is gone)
	Orphan  bool   // the root's parent ended and the root was handed to init, launchd or a subreaper
	Self    bool   // kiroku itself runs inside this group, so it is never stopped from kiroku
	Members []Proc // the root first, then every process under it
	Session string // the session it belongs to (Link), "" when none matched
}

const (
	// KindAgent is an agent process (claude, codex, kiro-cli, qchat) with the processes it started.
	KindAgent = "agent"
	// KindLeft is a process an agent started that is still running after the agent ended.
	KindLeft = "left"
)

// RSS is the memory of the whole group.
func (g Group) RSS() int64 {
	var n int64
	for _, p := range g.Members {
		n += p.RSS
	}
	return n
}

// CPU is the CPU time of the whole group.
func (g Group) CPU() float64 {
	var n float64
	for _, p := range g.Members {
		n += p.CPU
	}
	return n
}

// agentFamily is the agent family of a process name, "" when it is not an agent.
// Codex installed from npm runs a native binary named after the platform (codex-x86_64-unknown-linux-musl,
// which Linux shortens to 15 characters).
func agentFamily(name string) string {
	switch {
	case name == "claude":
		return "claude"
	case name == "codex" || strings.HasPrefix(name, "codex-"):
		return "codex"
	case name == "kiro-cli" || name == "kiro-cli-chat":
		return "kiro"
	case name == "qchat":
		return "amazonq"
	}
	return ""
}

// markers are environment variables an agent sets for the commands it runs. A process carrying one was started,
// directly or through a shell, by that agent. Only their presence is read; values are never kept.
// An empty val means any value.
var markers = []struct{ key, val, family string }{
	{"CLAUDECODE", "1", "claude"},                    // Claude Code: every command of the Bash tool
	{"CODEX_SANDBOX", "", "codex"},                   // Codex: commands in the macOS sandbox (seatbelt)
	{"CODEX_SANDBOX_NETWORK_DISABLED", "1", "codex"}, // Codex: commands in a sandbox without network
}

// markerOf is the agent family of the first marker in env ("KEY=value" entries), "" when none.
func markerOf(env []string) string {
	for _, e := range env {
		k, v, ok := strings.Cut(e, "=")
		if !ok {
			continue
		}
		for _, m := range markers {
			if k == m.key && (m.val == "" || v == m.val) {
				return m.family
			}
		}
	}
	return ""
}

// reapers are processes that take over orphans: init and launchd as PID 1, and systemd --user as a subreaper.
var reapers = map[string]bool{"systemd": true, "init": true, "launchd": true}

// Classify groups the processes into running agents and processes left running by agents that ended.
// self is kiroku's own PID: processes kiroku started (its "Analyze with AI" runs) are left out, and a group that
// kiroku itself runs in is marked Self.
func Classify(ps []Proc, self int) []Group {
	by := make(map[int]*Proc, len(ps))
	kids := map[int][]int{}
	for i := range ps {
		by[ps[i].PID] = &ps[i]
	}
	for _, p := range ps {
		if p.PPID != p.PID && by[p.PPID] != nil {
			kids[p.PPID] = append(kids[p.PPID], p.PID)
		}
	}
	for _, k := range kids {
		slices.Sort(k)
	}
	under := func(root int, skip map[int]bool) []int { // root and everything under it, root first
		out, todo := []int{}, []int{root}
		for len(todo) > 0 {
			pid := todo[0]
			todo = todo[1:]
			if skip[pid] {
				continue
			}
			skip[pid] = true
			out = append(out, pid)
			todo = append(todo, kids[pid]...)
		}
		return out
	}
	// kiroku's own children (agent runs of "Analyze with AI") and kiroku itself are never reported on their own
	mine := map[int]bool{}
	if by[self] != nil {
		for _, pid := range under(self, map[int]bool{}) {
			mine[pid] = true
		}
	}
	above := func(pid int, ok func(*Proc) bool) bool { // a process above pid matches ok
		seen := map[int]bool{pid: true}
		for p := by[pid]; p != nil; {
			q := by[p.PPID]
			if q == nil || seen[q.PID] {
				return false
			}
			if ok(q) {
				return true
			}
			seen[q.PID] = true
			p = q
		}
		return false
	}
	isAgent := func(p *Proc) bool { return agentFamily(p.Name) != "" && !mine[p.PID] }
	parentOf := func(p Proc) (string, bool) {
		q := by[p.PPID]
		if q == nil {
			return "", true
		}
		return q.Name, p.PPID == 1 || reapers[q.Name]
	}
	var out []Group
	taken := map[int]bool{}
	for k := range mine {
		taken[k] = true
	}
	group := func(root Proc, kind, fam string) Group {
		g := Group{Root: root, Kind: kind, Family: fam}
		g.Parent, g.Orphan = parentOf(root)
		for _, pid := range under(root.PID, taken) {
			g.Members = append(g.Members, *by[pid])
		}
		g.Self = by[self] != nil && (root.PID == self || above(self, func(q *Proc) bool { return q.PID == root.PID }))
		return g
	}
	// 1. agents that no other agent started (an agent run inside another agent belongs to the outer one)
	for i := range ps {
		p := &ps[i]
		if isAgent(p) && !taken[p.PID] && !above(p.PID, isAgent) {
			out = append(out, group(*p, KindAgent, agentFamily(p.Name)))
		}
	}
	// 2. processes an agent started (they carry its marker) with no agent above them any more
	for i := range ps {
		p := &ps[i]
		if taken[p.PID] || p.Marker == "" || p.PID == self {
			continue
		}
		if q := by[p.PPID]; q != nil && q.Marker != "" && !taken[q.PID] {
			continue // its parent is reported, and it goes in the parent's group
		}
		out = append(out, group(*p, KindLeft, p.Marker))
	}
	// left running first (they are what the user is looking for), then orphaned agents, then the newest
	rank := func(g Group) int {
		switch {
		case g.Kind == KindLeft:
			return 0
		case g.Orphan && !g.Root.TTY:
			return 1
		}
		return 2
	}
	slices.SortStableFunc(out, func(a, b Group) int {
		if ra, rb := rank(a), rank(b); ra != rb {
			return ra - rb
		}
		switch {
		case a.Root.Start > b.Root.Start:
			return -1
		case a.Root.Start < b.Root.Start:
			return 1
		}
		return a.Root.PID - b.Root.PID
	})
	return out
}

// Ref is a session of the history, for Link.
type Ref struct {
	ID     string
	Family string
	Cwd    string // the session's project folder
	Start  float64
	End    float64 // the session's last record
}

// slack is how far a process start may fall outside a session (clock steps, a record written just after a command).
const slack = 60

// Link sets each group's Session to the session it belongs to: one of the same agent family in the same folder
// (or a folder above the process's), that
//   - for an agent, was active while the process ran (its last record is after the process started); the session
//     with the latest record wins, since /clear or a resume can start a new session in the same process
//   - for a process left running, was going on when the process started; the latest such session wins
func Link(gs []Group, refs []Ref) {
	for i := range gs {
		g := &gs[i]
		best, bestKey := "", 0.0
		for _, r := range refs {
			if r.Family != g.Family || r.Cwd == "" || g.Root.Cwd == "" || !within(r.Cwd, g.Root.Cwd) {
				continue
			}
			var key float64
			switch g.Kind {
			case KindAgent:
				if r.End < g.Root.Start-slack {
					continue
				}
				key = r.End
			default:
				if g.Root.Start < r.Start-slack || g.Root.Start > r.End+slack {
					continue
				}
				key = r.Start
			}
			if best == "" || key > bestKey || key == bestKey && r.ID < best {
				best, bestKey = r.ID, key
			}
		}
		g.Session = best
	}
}

// within reports whether p is root or a folder under it.
func within(root, p string) bool {
	root, p = filepath.Clean(root), filepath.Clean(p)
	if root == p {
		return true
	}
	rel, err := filepath.Rel(root, p)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

// same reports whether q, from a later List, is still the process p (a PID can be reused after a process ends).
func same(p, q Proc) bool { return p.PID == q.PID && math.Abs(p.Start-q.Start) <= 1 }

// Stop asks every process of g that is still running (in now, a fresh List) to end: SIGTERM, or SIGKILL with force.
// The root goes first, so a process that restarts its children can't start them again. PID 1 and self are never
// signaled. It returns how many processes were signaled.
func Stop(g Group, now []Proc, self int, force bool) int {
	cur := make(map[int]Proc, len(now))
	for _, p := range now {
		cur[p.PID] = p
	}
	n := 0
	for _, m := range g.Members {
		c, ok := cur[m.PID]
		if !ok || m.PID <= 1 || m.PID == self || !same(m, c) {
			continue
		}
		if signal(m.PID, force) == nil {
			n++
		}
	}
	return n
}

// Running counts the processes of g that are still running in now.
func Running(g Group, now []Proc) int {
	cur := make(map[int]Proc, len(now))
	for _, p := range now {
		cur[p.PID] = p
	}
	n := 0
	for _, m := range g.Members {
		if c, ok := cur[m.PID]; ok && same(m, c) {
			n++
		}
	}
	return n
}
