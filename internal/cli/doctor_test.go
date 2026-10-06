package cli

import (
	"bytes"
	"os"
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
	doctorReport(&b, rep, 12, dir, release{})
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
	doctorReport(&b, rep, 12, dir, release{})
	if out := b.String(); !strings.Contains(out, "but kiroku archive keeps a copy") || strings.Contains(out, "! Claude Code") {
		t.Errorf("archive がオンなのに消えると出ている:\n%s", out)
	}
	b.Reset()
	doctorReport(&b, []source.Report{{Name: "Codex", Where: "/x"}}, 0, dir, release{})
	if out := b.String(); !strings.Contains(out, "No history found") {
		t.Errorf("履歴がないときの案内がない:\n%s", out)
	}
}

// 新しい版が出ていれば「!」と kiroku update を案内し、最新なら ✓、確かめられなければそう書く。手元でビルドしたものは確かめない。
func TestDoctorVersion(t *testing.T) {
	fakeAutostart(t, "darwin", noAnswer)
	old := version
	t.Cleanup(func() { version = old })
	version = "0.11.0"
	rep := []source.Report{{Name: "Codex", N: 1, Where: "/x"}}
	for _, c := range []struct {
		rel        release
		want, deny string
	}{
		{release{checked: true, latest: "v0.12.0"}, "! version: v0.11.0, and v0.12.0 is available", ""},
		{release{checked: true, latest: "v0.11.0"}, "✓ version: v0.11.0, the latest", "kiroku update"},
		{release{checked: true, err: os.ErrDeadlineExceeded}, "could not check for a newer release", "kiroku update  "},
		{release{}, "not checked for a newer release", "kiroku update  "},
	} {
		var b bytes.Buffer
		doctorReport(&b, rep, 1, t.TempDir(), c.rel)
		out := b.String()
		if !strings.Contains(out, c.want) || c.deny != "" && strings.Contains(out, c.deny) {
			t.Errorf("%+v:\n%s", c.rel, out)
		}
		if c.rel.latest == "v0.12.0" && !strings.Contains(out, "kiroku update        install v0.12.0") {
			t.Errorf("Next に kiroku update がない:\n%s", out)
		}
	}

	// kiroku doctor は、履歴を読むのと同時に最新の版を確かめる。--no-update-check と手元のビルドでは確かめない
	oldLatest := doctorLatest
	t.Cleanup(func() { doctorLatest = oldLatest })
	asked := 0
	doctorLatest = func() (string, error) { asked++; return "v0.12.0", nil }
	quietStdout(t)
	for _, c := range []struct {
		v    string
		args []string
		want int
	}{{"0.11.0", nil, 1}, {"0.11.0", []string{"--no-update-check"}, 0}, {"dev", nil, 0}} {
		version, asked = c.v, 0
		if err := dispatch(append([]string{"doctor", "--root", t.TempDir(), "--sources", "claude"}, c.args...)); err != nil {
			t.Fatal(err)
		}
		if asked != c.want {
			t.Errorf("%s %v: %d 回確かめた", c.v, c.args, asked)
		}
	}
}

// quietStdout は、テストのあいだ標準出力を捨てる。
func quietStdout(t *testing.T) {
	t.Helper()
	null, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = null
	t.Cleanup(func() { os.Stdout = old; null.Close() })
}
