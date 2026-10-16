package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/MichinaoShimizu/kiroku/internal/core"
	"github.com/MichinaoShimizu/kiroku/internal/procs"
)

// Agents running on this computer and processes they left running (ADR 0035): "kiroku ps", and /procs and
// /procs/stop in kiroku serve.

// listProcs lists this user's processes. Tests replace it.
var listProcs = procs.List

// stopWait is how long /procs/stop waits for the processes to end before answering.
var stopWait = 3 * time.Second

// noProcsEnv is $KIROKU_NO_PROCS (the default of --no-procs), read like $KIROKU_NO_AI.
func noProcsEnv() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("KIROKU_NO_PROCS"))) {
	case "", "0", "false":
		return false
	}
	return true
}

// findGroups lists the processes, groups them and links each group to its session in data.
func findGroups(data []*core.Session) ([]procs.Group, []procs.Proc, error) {
	ps, err := listProcs()
	if err != nil {
		return nil, nil, err
	}
	gs := procs.Classify(ps, os.Getpid())
	refs := make([]procs.Ref, 0, len(data))
	for _, s := range data {
		if a, ok := core.AgentOf(s.Source); ok && s.ProjectPath != "" {
			refs = append(refs, procs.Ref{ID: s.ID, Family: a.Family, Cwd: s.ProjectPath, Start: s.Start, End: s.End})
		}
	}
	procs.Link(gs, refs)
	return gs, ps, nil
}

// procJSON is one group in /procs. It carries no command line and no environment, only the short process name.
type procJSON struct {
	PID     int     `json:"pid"`
	Start   float64 `json:"start"`
	Kind    string  `json:"kind"`
	Family  string  `json:"family"`
	Agent   string  `json:"agent"` // the agent's name to show
	Name    string  `json:"name"`
	Parent  string  `json:"parent,omitempty"`
	TTY     bool    `json:"tty,omitempty"`
	Orphan  bool    `json:"orphan,omitempty"`
	Self    bool    `json:"self,omitempty"`
	N       int     `json:"n"`
	RSS     int64   `json:"rss"`
	CPU     float64 `json:"cpu"`
	Cwd     string  `json:"cwd,omitempty"`
	Session string  `json:"session,omitempty"`
}

func procsJSON(gs []procs.Group) []procJSON {
	out := make([]procJSON, 0, len(gs))
	for _, g := range gs {
		out = append(out, procJSON{PID: g.Root.PID, Start: g.Root.Start, Kind: g.Kind, Family: g.Family, Agent: agentName(g.Family), Name: g.Root.Name,
			Parent: g.Parent, TTY: g.Root.TTY, Orphan: g.Orphan, Self: g.Self, N: len(g.Members), RSS: g.RSS(),
			CPU: math.Round(g.CPU()), Cwd: g.Root.Cwd, Session: g.Session})
	}
	return out
}

// procsHandler serves GET /procs and POST /procs/stop. It is mounted only when kiroku serve listens on a loopback
// address, was not started with --no-procs, and the OS is supported; otherwise both paths answer 404.
func (l *live) procsHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/procs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !l.loaded(w) {
			return
		}
		l.mu.RLock()
		data := l.snap.data
		l.mu.RUnlock()
		gs, _, err := findGroups(data)
		if err != nil {
			l.logf("could not list processes: %v\n", err)
			http.Error(w, "could not list processes; see the terminal where kiroku serve runs for details", http.StatusInternalServerError)
			return
		}
		body, _ := json.Marshal(map[string]any{"procs": procsJSON(gs)})
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(body)
	})
	// POST /procs/stop {"pid":…, "start":…, "force":…} stops a group that GET /procs reported. Like /archive, it needs
	// the X-Kiroku header (another site's page can't send it without a preflight, which is never answered) and,
	// when the browser sends Origin, this server's own origin. The PID and start time must still name the root of a
	// group kiroku finds now: a PID that was reused, or any other process, is refused.
	mux.HandleFunc("/procs/stop", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("X-Kiroku") != "1" || (r.Header.Get("Origin") != "" && r.Header.Get("Origin") != "http://"+r.Host) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		var req struct {
			PID   int     `json:"pid"`
			Start float64 `json:"start"`
			Force bool    `json:"force"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1024)).Decode(&req); err != nil || req.PID <= 1 {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		gs, _, err := findGroups(nil)
		if err != nil {
			l.logf("could not list processes: %v\n", err)
			http.Error(w, "could not list processes; see the terminal where kiroku serve runs for details", http.StatusInternalServerError)
			return
		}
		var g *procs.Group
		for i := range gs {
			if gs[i].Root.PID == req.PID && math.Abs(gs[i].Root.Start-req.Start) <= 1 {
				g = &gs[i]
				break
			}
		}
		if g == nil {
			http.Error(w, "that process has already ended or is not one kiroku reports", http.StatusNotFound)
			return
		}
		if g.Self {
			http.Error(w, "kiroku runs inside this process; stop it from its terminal", http.StatusForbidden)
			return
		}
		ps, err := listProcs()
		if err != nil {
			http.Error(w, "could not list processes", http.StatusInternalServerError)
			return
		}
		sent := procs.Stop(*g, ps, os.Getpid(), req.Force)
		l.logf("stopped %s (PID %d) and the processes under it at the view's request: signaled %d (%s)\n", g.Root.Name, g.Root.PID, sent, map[bool]string{false: "SIGTERM", true: "SIGKILL"}[req.Force])
		running := 0
		for end := time.Now().Add(stopWait); ; time.Sleep(100 * time.Millisecond) {
			if ps, err = listProcs(); err == nil {
				running = procs.Running(*g, ps)
			}
			if running == 0 || time.Now().After(end) {
				break
			}
		}
		body, _ := json.Marshal(map[string]int{"signaled": sent, "running": running})
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(body)
	})
	return mux
}

func cmdPS(args []string) error {
	fs := newFS("ps", "ps [flags]\n\nLists the AI agents running on this computer and the processes they left running after they ended\n(such as a dev server started by the agent's shell), with the session each belongs to.\nOnly reads; it prints the command that stops each one. In kiroku serve, the view can stop them after you confirm.")
	c := addCommon(fs)
	asJSON := fs.Bool("json", false, "print JSON")
	if _, err := parse(fs, args, 0); err != nil {
		return quiet(err)
	}
	if !procs.Supported() {
		return fmt.Errorf("kiroku ps works on macOS and Linux only")
	}
	_, load, err := c.loader()
	if err != nil {
		return err
	}
	snap := load()
	gs, _, err := findGroups(snap.data)
	if err != nil {
		return fmt.Errorf("could not list processes: %w", err)
	}
	if *asJSON {
		b, err := json.MarshalIndent(procsJSON(gs), "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(os.Stdout, "%s\n", b)
		return err
	}
	return writePS(os.Stdout, gs, snap.data, time.Now())
}

// writePS prints the groups: processes left running first, then running agents.
func writePS(w io.Writer, gs []procs.Group, data []*core.Session, now time.Time) error {
	st := styleFor(w)
	byID := make(map[string]*core.Session, len(data))
	for _, s := range data {
		byID[s.ID] = s
	}
	var left, agents []procs.Group
	for _, g := range gs {
		if g.Kind == procs.KindLeft || g.Orphan && !g.Root.TTY { // an agent whose terminal is gone is left running too
			left = append(left, g)
		} else {
			agents = append(agents, g)
		}
	}
	fmt.Fprintf(w, "%s · %s\n", st.bold("kiroku ps"), "AI agents and what they left running")
	if len(gs) == 0 {
		fmt.Fprintln(w, "\n  No AI agent is running, and nothing started by one is left running.")
		return nil
	}
	row := func(g procs.Group, status string) {
		n := ""
		if len(g.Members) > 1 {
			n = fmt.Sprintf(" · %d processes", len(g.Members))
		}
		fmt.Fprintf(w, "\n  %s %s%s · %s\n", st.bold(termSafe(g.Root.Name, 200)), st.dim("PID "+strconv.Itoa(g.Root.PID)), n, status)
		fmt.Fprintf(w, "    %s · %s memory · %s CPU", since(g.Root.Start, now), bytesH(g.RSS()), hm(g.CPU()/60))
		if g.Root.Cwd != "" {
			fmt.Fprintf(w, " · %s", termSafe(tilde(g.Root.Cwd), 500))
		}
		fmt.Fprintln(w)
		if s := byID[g.Session]; s != nil {
			fmt.Fprintf(w, "    session: %s %s\n", termSafe(s.Title, 200), st.dim(fmt.Sprintf("(%s, %s–%s)", s.Source, time.Unix(int64(s.Start), 0).Format("1/2 15:04"), time.Unix(int64(s.End), 0).Format("15:04"))))
		}
		if g.Self {
			fmt.Fprintln(w, st.dim("    kiroku runs inside it"))
			return
		}
		var ids []string
		for _, m := range g.Members {
			ids = append(ids, strconv.Itoa(m.PID))
		}
		fmt.Fprintf(w, "    %s %s\n", st.dim("stop:"), "kill "+strings.Join(ids, " "))
	}
	if len(left) > 0 {
		fmt.Fprintf(w, "\n%s\n", st.bold("Left running"))
		for _, g := range left {
			if g.Kind == procs.KindLeft {
				row(g, "left running after "+agentName(g.Family)+" ended")
			} else {
				row(g, agentStatus(g))
			}
		}
	}
	if len(agents) > 0 {
		fmt.Fprintf(w, "\n%s\n", st.bold("Running agents"))
		for _, g := range agents {
			row(g, agentStatus(g))
		}
	}
	return nil
}

// agentStatus says where an agent runs: in a terminal, with its terminal gone, or under another program.
func agentStatus(g procs.Group) string {
	switch {
	case g.Root.TTY:
		return "open in a terminal"
	case g.Orphan:
		return "its terminal or program is gone"
	case g.Parent != "":
		return "run by " + termSafe(g.Parent, 200)
	}
	return "running"
}

func agentName(family string) string {
	for _, a := range core.Agents {
		if a.Family == family {
			if family == "kiro" {
				return "Kiro CLI"
			}
			return a.Name
		}
	}
	return family
}

func since(start float64, now time.Time) string {
	d := now.Sub(time.Unix(int64(start), 0))
	if d < 0 {
		d = 0
	}
	return "running for " + hm(d.Minutes())
}

func bytesH(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%d MB", n>>20)
	}
	return fmt.Sprintf("%d KB", n>>10)
}
