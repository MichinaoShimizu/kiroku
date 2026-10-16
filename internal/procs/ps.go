package procs

import (
	"bufio"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// The macOS reader runs ps and lsof (list_darwin.go). Their output is parsed here, outside a build tag,
// so that the parsers are tested on every OS.

// psArgs lists every process: PID, parent, user ID, terminal, memory (KiB), CPU time, start time and name.
// The name is last because it can hold spaces. LC_ALL=C keeps lstart in English.
var psArgs = []string{"-axww", "-o", "pid=,ppid=,uid=,tty=,rss=,time=,lstart=,comm="}

// parsePS reads the output of ps with psArgs and keeps the processes of uid.
func parsePS(out string, uid int, loc *time.Location) []Proc {
	var ps []Proc
	sc := bufio.NewScanner(strings.NewReader(out))
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) < 12 { // pid ppid uid tty rss time + lstart (5 words) + comm
			continue
		}
		pid, e1 := strconv.Atoi(f[0])
		ppid, e2 := strconv.Atoi(f[1])
		u, e3 := strconv.Atoi(f[2])
		rss, e4 := strconv.ParseInt(f[4], 10, 64)
		if e1 != nil || e2 != nil || e3 != nil || e4 != nil || u != uid || pid <= 0 {
			continue
		}
		start, err := time.ParseInLocation("Mon Jan _2 15:04:05 2006", strings.Join(f[6:11], " "), loc)
		if err != nil {
			continue
		}
		name := filepath.Base(strings.Join(f[11:], " ")) // comm is the executable's path on macOS
		ps = append(ps, Proc{PID: pid, PPID: ppid, TTY: f[3] != "??" && f[3] != "-", RSS: rss * 1024,
			CPU: cpuTime(f[5]), Start: float64(start.Unix()), Name: name})
	}
	return ps
}

// cpuTime reads the TIME column of ps: [[dd-]hh:]mm:ss[.cc].
func cpuTime(s string) float64 {
	days := 0.0
	if d, rest, ok := strings.Cut(s, "-"); ok {
		n, err := strconv.ParseFloat(d, 64)
		if err != nil {
			return 0
		}
		days, s = n, rest
	}
	var t float64
	for _, part := range strings.Split(s, ":") {
		n, err := strconv.ParseFloat(part, 64)
		if err != nil {
			return 0
		}
		t = t*60 + n
	}
	return days*86400 + t
}

// psEnvArgs prints each listed process's command line followed by its environment (ps -E), for markers.
func psEnvArgs(pids []int) []string {
	return []string{"-E", "-ww", "-o", "pid=,command=", "-p", joinPIDs(pids)}
}

// parsePSEnv finds the markers in the output of ps with psEnvArgs. ps puts the environment after the command
// line with spaces between, so a marker counts only as a whole word; the rest of the line is not kept.
// Only the PIDs asked for count: a newline in a process's arguments can't mark another process.
func parsePSEnv(out string, asked []int) map[int]string {
	want := make(map[int]bool, len(asked))
	for _, p := range asked {
		want[p] = true
	}
	res := map[int]string{}
	sc := bufio.NewScanner(strings.NewReader(out))
	sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) < 2 {
			continue
		}
		pid, err := strconv.Atoi(f[0])
		if err != nil || !want[pid] {
			continue
		}
		if m := markerOf(f[1:]); m != "" && res[pid] == "" {
			res[pid] = m
		}
	}
	return res
}

// lsofArgs prints the working folder of each listed process, in lsof's field format.
func lsofArgs(pids []int) []string {
	return []string{"-a", "-d", "cwd", "-Fn", "-p", joinPIDs(pids)}
}

// parseLsof reads lsof -Fn: a "p<pid>" line, then "n<path>" for its working folder.
func parseLsof(out string) map[int]string {
	res, pid := map[int]string{}, 0
	for _, l := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(l, "p"):
			pid, _ = strconv.Atoi(l[1:])
		case strings.HasPrefix(l, "n") && pid > 0:
			res[pid] = l[1:]
		}
	}
	return res
}

func joinPIDs(pids []int) string {
	s := make([]string, len(pids))
	for i, p := range pids {
		s[i] = strconv.Itoa(p)
	}
	return strings.Join(s, ",")
}
