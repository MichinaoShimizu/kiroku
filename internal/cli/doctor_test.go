package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/MichinaoShimizu/kiroku/internal/source"
)

// kiroku doctor：消える設定のままなら「!」で設定の場所と kiroku archive on を案内し、archive がオンなら安心と出す。
func TestDoctorReport(t *testing.T) {
	fakeAutostart(t, "darwin", noAnswer)
	old := lookGit
	t.Cleanup(func() { lookGit = old })
	lookGit = func() bool { return false }
	dir := t.TempDir()
	rep := []source.Report{
		{Name: "Claude Code", N: 12, Oldest: 1.78e9, Where: "/x/projects", Keep: &source.Retention{Days: 30, Setting: "cleanupPeriodDays", Docs: "https://docs", File: "/x/settings.json"}},
		{Name: "Codex", Where: "/x/.codex"},
	}
	var b bytes.Buffer
	doctorReport(&b, rep, 12, dir)
	out := b.String()
	for _, want := range []string{"✓ Claude Code", "12 sessions, since", "· Codex", "none",
		`! Claude Code deletes history older than 30 days`, `add "cleanupPeriodDays": 3650 to /x/settings.json`, `Run "kiroku archive on"`,
		"git: not found", "autostart: off", "kiroku archive on    keep copies", "kiroku serve"} {
		if !strings.Contains(out, want) {
			t.Errorf("%q がない:\n%s", want, out)
		}
	}
	if err := dispatch([]string{"archive", "on", "--archive-dir", dir, "--root", t.TempDir(), "--sources", "claude"}); err != nil {
		t.Fatal(err)
	}
	b.Reset()
	doctorReport(&b, rep, 12, dir)
	if out := b.String(); !strings.Contains(out, "but kiroku archive keeps a copy") || strings.Contains(out, "! Claude Code") {
		t.Errorf("archive がオンなのに消えると出ている:\n%s", out)
	}
	b.Reset()
	doctorReport(&b, []source.Report{{Name: "Codex", Where: "/x"}}, 0, dir)
	if out := b.String(); !strings.Contains(out, "No history found") {
		t.Errorf("履歴がないときの案内がない:\n%s", out)
	}
}
