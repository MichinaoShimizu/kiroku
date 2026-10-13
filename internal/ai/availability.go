package ai

import "os/exec"

// Availability reports which supported agent executables are installed.
// It does not invoke an agent or verify authentication or CLI compatibility.
type Availability struct {
	Agent     string
	Installed bool
	Path      string
}

// Adapter describes a verified, explicitly supported noninteractive execution target.
// Additional providers (including Kiro CLI and Kiro Crew) must not be advertised
// until their invocation, permissions and output contracts are verified.
type Adapter struct {
	ID     string
	Binary string
	Mode   string
}

var supportedAdapters = []Adapter{
	{ID: "claude", Binary: "claude", Mode: "single"},
	{ID: "codex", Binary: "codex", Mode: "single"},
	{ID: "kiro", Binary: "kiro-cli", Mode: "single"},
}

// Detect checks only the known executables and never executes them.
func Detect() []Availability {
	out := make([]Availability, 0, len(supportedAdapters))
	for _, adapter := range supportedAdapters {
		path, err := exec.LookPath(adapter.Binary)
		out = append(out, Availability{Agent: adapter.ID, Installed: err == nil, Path: path})
	}
	return out
}
