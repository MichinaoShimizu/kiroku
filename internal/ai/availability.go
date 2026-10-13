package ai

import "os/exec"

// Availability reports which supported agent executables are installed.
// It does not invoke an agent or verify authentication or CLI compatibility.
type Availability struct {
	Agent     string
	Installed bool
	Path      string
}

// Detect checks only the known executables and never executes them.
func Detect() []Availability {
	out := make([]Availability, 0, 2)
	for _, agent := range []string{"claude", "codex"} {
		path, err := exec.LookPath(agent)
		out = append(out, Availability{Agent: agent, Installed: err == nil, Path: path})
	}
	return out
}
