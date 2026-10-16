//go:build darwin

package procs

import (
	"context"
	"os"
	"os/exec"
	"time"
)

// Supported reports whether this OS can list processes.
func Supported() bool { return true }

// List returns this user's processes, read with ps and lsof (fixed paths and arguments, no shell).
// macOS gives the environment of another process only through ps -E, so it is read only for processes whose
// parent is launchd (where a process left running by an ended agent ends up) and for their children.
func List() ([]Proc, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := run(ctx, "/bin/ps", psArgs...)
	if err != nil {
		return nil, err
	}
	ps := parsePS(out, os.Getuid(), time.Local)
	parentOf := map[int]int{}
	for _, p := range ps {
		parentOf[p.PID] = p.PPID
	}
	var envPIDs, cwdPIDs []int
	for _, p := range ps {
		if p.PPID == 1 || parentOf[p.PPID] == 1 {
			envPIDs = append(envPIDs, p.PID)
		}
		if agentFamily(p.Name) != "" {
			cwdPIDs = append(cwdPIDs, p.PID)
		}
	}
	marks := map[int]string{}
	if len(envPIDs) > 0 {
		if out, err := run(ctx, "/bin/ps", psEnvArgs(envPIDs)...); err == nil || out != "" {
			marks = parsePSEnv(out)
		}
	}
	for i := range ps {
		if m := marks[ps[i].PID]; m != "" {
			ps[i].Marker = m
			cwdPIDs = append(cwdPIDs, ps[i].PID)
		}
	}
	if len(cwdPIDs) > 0 {
		// lsof exits with 1 when one of the processes has ended; what it printed for the others is still good
		if out, err := run(ctx, "/usr/sbin/lsof", lsofArgs(cwdPIDs)...); err == nil || out != "" {
			cwd := parseLsof(out)
			for i := range ps {
				ps[i].Cwd = cwd[ps[i].PID]
			}
		}
	}
	return ps, nil
}

func run(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = []string{"LC_ALL=C", "PATH=/usr/bin:/bin:/usr/sbin:/sbin"}
	if tz, ok := os.LookupEnv("TZ"); ok { // lstart in the same time zone as time.Local
		cmd.Env = append(cmd.Env, "TZ="+tz)
	}
	out, err := cmd.Output()
	return string(out), err
}
