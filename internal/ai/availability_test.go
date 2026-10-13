package ai

import "testing"

func TestDetectSupportedAgents(t *testing.T) {
	got := Detect()
	if len(got) != 2 {
		t.Fatalf("want 2 adapters, got %d", len(got))
	}
	if got[0].Agent != "claude" || got[1].Agent != "codex" {
		t.Fatalf("unexpected adapters: %+v", got)
	}
	for _, a := range got {
		if a.Installed && a.Path == "" {
			t.Errorf("installed %s without path", a.Agent)
		}
	}
}
