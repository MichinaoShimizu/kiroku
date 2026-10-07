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
		{Name: "Claude Code", N: 12, Oldest: 1.78e9, Where: "/x/projects", Keep: &source.Retention{Days: 30, Setting: "cleanupPeriodDays", Snippet: `"cleanupPeriodDays": 3650`, Docs: "https://docs", File: "/x/settings.json"}},
		{Name: "Kiro CLI", N: 3, Where: "/x/.kiro/sessions/cli", Keep: &source.Retention{Days: 30, Who: "Kiro Crew", Setting: "session.archive_retention_days", Snippet: `"session": {"archive_retention_days": 3650}`, Docs: "https://docs", File: "/x/crew/config.local.json"}},
		{Name: "Codex", Where: "/x/.codex"},
	}
	fakeKirokuPath(t, "/x/bin/kiroku", "/x/bin/kiroku")
	var b bytes.Buffer
	doctorReport(&b, rep, 12, dir, release{}, false)
	out := b.String()
	for _, want := range []string{"✓ Claude Code", "12 sessions, since", "/x/projects",
		`! Claude Code deletes history older than 30 days`, `add "cleanupPeriodDays": 3650 to /x/settings.json`,
		`! Kiro Crew deletes history older than 30 days (session.archive_retention_days is at its default)`, `add "session": {"archive_retention_days": 3650} to /x/crew/config.local.json`, `Run "kiroku archive on"`,
		"git: not found", "autostart: off", "kiroku archive on    keep copies", "kiroku serve"} {
		if !strings.Contains(out, want) {
			t.Errorf("%q がない:\n%s", want, out)
		}
	}
	// 履歴がなかったエージェントは1行にまとめ、場所は出さない（読んでほしい行を押し出さないため）
	if !strings.Contains(out, "no history yet") || !strings.Contains(out, "Codex") {
		t.Errorf("履歴がないエージェントのまとめがない:\n%s", out)
	}
	if strings.Contains(out, "/x/.codex") || strings.Contains(out, "· Codex") {
		t.Errorf("履歴がないエージェントの場所まで出ている:\n%s", out)
	}
	// --all なら、どこを探したのかまで出す
	b.Reset()
	doctorReport(&b, rep, 12, dir, release{}, true)
	if out := b.String(); !strings.Contains(out, "· Codex") || !strings.Contains(out, "/x/.codex") || strings.Contains(out, "no history yet") {
		t.Errorf("--all なのに場所が出ていない:\n%s", out)
	}
	if err := dispatch([]string{"archive", "on", "--archive-dir", dir, "--root", t.TempDir(), "--sources", "claude"}); err != nil {
		t.Fatal(err)
	}
	b.Reset()
	doctorReport(&b, rep, 12, dir, release{}, false)
	if out := b.String(); !strings.Contains(out, "but kiroku archive keeps a copy") || strings.Contains(out, "! Claude Code") {
		t.Errorf("archive がオンなのに消えると出ている:\n%s", out)
	}
	b.Reset()
	doctorReport(&b, []source.Report{{Name: "Codex", Where: "/x"}, {Name: "Kiro IDE (legacy)", Where: "none"}}, 0, dir, release{}, false)
	// どれも 0 件なら、どこを探したのかを全部出す。場所が分からないエージェントは、そう書く
	if out := b.String(); !strings.Contains(out, "No history found") || !strings.Contains(out, "/x") || !strings.Contains(out, "not installed") {
		t.Errorf("履歴がないときの案内がない:\n%s", out)
	}
}

// 読めないファイルがあったエージェントは、0 件でもまとめずに出す（問題を隠さない）。
func TestDoctorKeepsErrors(t *testing.T) {
	fakeAutostart(t, "darwin", noAnswer)
	fakeKirokuPath(t, "/x/bin/kiroku", "/x/bin/kiroku")
	msg := "permission denied"
	rep := []source.Report{
		{Name: "Claude Code", N: 3, Where: "/x/projects"},
		{Name: "Codex", Where: "/x/.codex", Error: &msg},
		{Name: "Amazon Q", Where: "/x/q.sqlite3"},
	}
	var b bytes.Buffer
	doctorReport(&b, rep, 3, t.TempDir(), release{}, false)
	out := b.String()
	if !strings.Contains(out, "! Codex") || !strings.Contains(out, msg) {
		t.Errorf("読めなかったエージェントが隠れている:\n%s", out)
	}
	if !strings.Contains(out, "no history yet") || !strings.Contains(out, "Amazon Q") {
		t.Errorf("ふつうの 0 件はまとめるはず:\n%s", out)
	}
}

// Kiro Crew の 0 日（Now）は「0 日残す」ではなく、次の片付けで消えると「!」で出す。archive がオンなら安心と出す。
func TestDoctorZeroDayRetention(t *testing.T) {
	fakeAutostart(t, "darwin", noAnswer)
	fakeKirokuPath(t, "/x/bin/kiroku", "/x/bin/kiroku")
	dir := t.TempDir()
	rep := []source.Report{{Name: "Kiro CLI", N: 3, Where: "/x/.kiro/sessions/cli", Keep: &source.Retention{Days: 0, Now: true, Set: true, Who: "Kiro Crew",
		Setting: "session.archive_retention_days", Snippet: `"session": {"archive_retention_days": 3650}`, Docs: "https://docs", File: "/x/crew/config.local.json"}}}
	var b bytes.Buffer
	doctorReport(&b, rep, 3, dir, release{}, false)
	out := b.String()
	for _, want := range []string{"! Kiro Crew deletes old history at its next cleanup, within an hour (session.archive_retention_days is 0)",
		`add "session": {"archive_retention_days": 3650} to /x/crew/config.local.json`, `Run "kiroku archive on"`} {
		if !strings.Contains(out, want) {
			t.Errorf("%q がない:\n%s", want, out)
		}
	}
	if strings.Contains(out, "keeps history for 0 days") {
		t.Errorf("0 日を「0 日残す」と出している:\n%s", out)
	}
	if err := dispatch([]string{"archive", "on", "--archive-dir", dir, "--root", t.TempDir(), "--sources", "claude"}); err != nil {
		t.Fatal(err)
	}
	b.Reset()
	doctorReport(&b, rep, 3, dir, release{}, false)
	if out := b.String(); !strings.Contains(out, "✓ Kiro Crew deletes old history at its next cleanup (session.archive_retention_days is 0), but kiroku archive keeps a copy") {
		t.Errorf("archive がオンなのに安心と出ていない:\n%s", out)
	}
}

// fakeKirokuPath は、動いている kiroku と PATH で先に見つかる kiroku を差しかえる。
func fakeKirokuPath(t *testing.T, exe, first string) {
	t.Helper()
	oldRun, oldPath := runningExe, pathExe
	t.Cleanup(func() { runningExe, pathExe = oldRun, oldPath })
	runningExe = func() string { return exe }
	pathExe = func() string { return first }
}

// PATH の前のほうに別の kiroku があれば、doctor もそれを知らせる（install.sh と同じ）。
func TestDoctorShadowedExe(t *testing.T) {
	fakeAutostart(t, "darwin", noAnswer)
	old := version
	t.Cleanup(func() { version = old })
	version = "0.16.0"
	rep := []source.Report{{Name: "Codex", N: 1, Where: "/x"}}
	fakeKirokuPath(t, "/home/me/.local/bin/kiroku", "/usr/local/bin/kiroku")
	var b bytes.Buffer
	doctorReport(&b, rep, 1, t.TempDir(), release{}, false)
	out := b.String()
	if !strings.Contains(out, "another kiroku comes first in your PATH") || !strings.Contains(out, "/usr/local/bin/kiroku") {
		t.Errorf("PATH の重複を知らせていない:\n%s", out)
	}
	if !strings.Contains(out, "/home/me/.local/bin/kiroku") {
		t.Errorf("動いている kiroku の場所がない:\n%s", out)
	}
	// 同じものなら知らせない。手元でビルドしたものは、場所が入れ替わるので比べない
	b.Reset()
	fakeKirokuPath(t, "/usr/local/bin/kiroku", "/usr/local/bin/kiroku")
	doctorReport(&b, rep, 1, t.TempDir(), release{}, false)
	if strings.Contains(b.String(), "comes first in your PATH") {
		t.Errorf("同じ kiroku なのに知らせている:\n%s", b.String())
	}
	b.Reset()
	version = "dev"
	fakeKirokuPath(t, "/tmp/go-build/kiroku", "/usr/local/bin/kiroku")
	doctorReport(&b, rep, 1, t.TempDir(), release{}, false)
	if strings.Contains(b.String(), "comes first in your PATH") {
		t.Errorf("dev ビルドで知らせている:\n%s", b.String())
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
		doctorReport(&b, rep, 1, t.TempDir(), c.rel, false)
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
