package ai

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectSupportedAgents(t *testing.T) {
	got := Detect()
	if len(got) != 3 {
		t.Fatalf("want 3 adapters, got %d", len(got))
	}
	if got[0].Agent != "claude" || got[1].Agent != "codex" || got[2].Agent != "kiro" {
		t.Fatalf("unexpected adapters: %+v", got)
	}
	for _, a := range got {
		if a.Installed && a.Path == "" {
			t.Errorf("installed %s without path", a.Agent)
		}
	}
}

// Installed lists only the agents whose command is on PATH, in the fixed order.
func TestInstalled(t *testing.T) {
	stubAgent(t, "kiro-cli", "true")
	t.Setenv("PATH", filepath.Dir(mustLook(t, "kiro-cli")))
	if got := strings.Join(Installed(), ","); got != "kiro" {
		t.Errorf("Installed = %q, want kiro only", got)
	}
}

func mustLook(t *testing.T, name string) string {
	t.Helper()
	p, err := exec.LookPath(name)
	if err != nil {
		t.Fatal(err)
	}
	return p
}
