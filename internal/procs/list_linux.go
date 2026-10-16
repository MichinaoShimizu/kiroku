//go:build linux

package procs

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// Supported reports whether this OS can list processes.
func Supported() bool { return true }

// clockTicks is USER_HZ, the unit of the times in /proc/<pid>/stat. It is 100 on every Linux architecture Go
// supports (sysconf(_SC_CLK_TCK) needs cgo).
const clockTicks = 100

// List returns this user's processes from /proc.
func List() ([]Proc, error) {
	return listProc("/proc", os.Getuid())
}

func listProc(root string, uid int) ([]Proc, error) {
	boot, err := bootTime(filepath.Join(root, "stat"))
	if err != nil {
		return nil, err
	}
	ents, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	page := int64(os.Getpagesize())
	var out []Proc
	for _, e := range ents {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid <= 0 {
			continue
		}
		dir := filepath.Join(root, e.Name())
		fi, err := os.Stat(dir)
		if err != nil {
			continue // ended while listing
		}
		if st, ok := fi.Sys().(*syscall.Stat_t); !ok || int(st.Uid) != uid {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, "stat"))
		if err != nil {
			continue
		}
		p, ok := parseStat(b, boot, page)
		if !ok || p.PID != pid {
			continue
		}
		p.Cwd, _ = os.Readlink(filepath.Join(dir, "cwd"))
		p.Marker = environMarker(filepath.Join(dir, "environ"))
		out = append(out, p)
	}
	return out, nil
}

// bootTime is when the computer started (btime in /proc/stat), in UNIX seconds.
func bootTime(path string) (float64, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	for _, l := range strings.Split(string(b), "\n") {
		if v, ok := strings.CutPrefix(l, "btime "); ok {
			return strconv.ParseFloat(strings.TrimSpace(v), 64)
		}
	}
	return 0, os.ErrNotExist
}

// parseStat reads /proc/<pid>/stat. The name is between the first "(" and the last ")", since it can hold
// spaces and parentheses. Zombies (state Z) are skipped: they have already ended and can't be stopped.
func parseStat(b []byte, boot float64, page int64) (Proc, bool) {
	open, end := bytes.IndexByte(b, '('), bytes.LastIndexByte(b, ')')
	if open < 0 || end < open {
		return Proc{}, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b[:open])))
	if err != nil {
		return Proc{}, false
	}
	f := strings.Fields(string(b[end+1:])) // f[0] is field 3 (state) of proc(5)
	if len(f) < 22 || f[0] == "Z" || f[0] == "X" {
		return Proc{}, false
	}
	num := func(i int) int64 { n, _ := strconv.ParseInt(f[i], 10, 64); return n }
	ppid, err := strconv.Atoi(f[1])
	if err != nil {
		return Proc{}, false
	}
	return Proc{
		PID:   pid,
		PPID:  ppid,
		Name:  string(b[open+1 : end]),
		TTY:   num(4) != 0,
		CPU:   float64(num(11)+num(12)) / clockTicks,
		Start: boot + float64(num(19))/clockTicks,
		RSS:   num(21) * page,
	}, true
}

// Limits for reading /proc/<pid>/environ. A process can make its environment area as large as it likes
// (prctl PR_SET_MM_MAP needs no privilege), so it is read in pieces and only up to envMax: an environment set by
// execve is at most a few MiB, and an entry longer than envEntryMax (the kernel's MAX_ARG_STRLEN) can't be a marker.
const (
	envMax      = 4 << 20
	envEntryMax = 128 << 10
)

// environMarker is the agent family of the first marker in the environ file at path, "" when none or unreadable.
// Memory stays bounded whatever the file holds, and reading stops at the first marker.
func environMarker(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(io.LimitReader(f, envMax))
	sc.Buffer(make([]byte, 4096), envEntryMax)
	sc.Split(func(data []byte, atEOF bool) (int, []byte, error) {
		if i := bytes.IndexByte(data, 0); i >= 0 {
			return i + 1, data[:i], nil
		}
		if atEOF && len(data) > 0 {
			return len(data), data, nil
		}
		return 0, nil, nil
	})
	entry := make([]string, 1)
	for sc.Scan() {
		entry[0] = sc.Text()
		if m := markerOf(entry); m != "" {
			return m
		}
	}
	return "" // an entry too long (bufio.ErrTooLong) ends the reading too
}
