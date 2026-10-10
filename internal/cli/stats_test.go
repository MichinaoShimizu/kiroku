package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/MichinaoShimizu/kiroku/internal/core"
	"github.com/MichinaoShimizu/kiroku/internal/gitlog"
)

// kiroku stats は、期間の上の数字・エージェント・モデル・プロジェクトを 1 画面に出す。数字は画面と同じ集計から来る。
func TestWriteStats(t *testing.T) {
	setup(t)
	at := func(d, h, m int) float64 { return float64(time.Date(2026, 10, d, h, m, 0, 0, time.Local).Unix()) }
	fp := func(v float64) *float64 { return &v }
	sess := func(id, src, project string, a, b float64) *core.Session {
		return &core.Session{ID: id, Source: src, Project: project, Start: a, End: b, Segs: [][3]float64{{a, b, 10}}}
	}
	cc := sess("a", "Claude Code", "web-app", at(5, 10, 0), at(5, 11, 30))
	cc.UEv = []core.Event{{T: fp(at(5, 10, 30)), Model: "claude-sonnet-5-5", U: core.Tokens{In: 1000, Out: 500, CR: 1_500_000}, Cost: fp(1.25)}}
	cc.Prompts = []core.Prompt{{T: fp(at(5, 10, 0))}, {T: fp(at(5, 10, 40))}}
	data := []*core.Session{
		cc,
		sess("b", "Codex", "docs", at(7, 14, 0), at(7, 14, 45)),
		sess("old", "Claude Code", "web-app", at(1, 10, 0), at(1, 11, 0)), // 前の週（入らない）
	}
	commits := []gitlog.Commit{{Hash: "x", T: at(5, 11, 0), Project: "web-app"}}
	p, err := parsePeriod("this", "", time.Date(2026, 10, 7, 18, 0, 0, 0, time.Local))
	if err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	if err := writeStats(&b, data, commits, p, time.Date(2026, 10, 7, 18, 0, 0, 0, time.Local)); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	for _, want := range []string{
		"kiroku · Oct 5 – 11, 2026 (this week)",
		"Active time     2h 15m    2 of 3 days so far",
		"Sessions        2         2 prompts",
		"Tokens          1.5M",
		"Estimated cost  $1.25",
		"Git commits     1",
		"Claude Code   1h 30m          1     1.5M   $1.25",
		"Codex            45m          1        —       —",
		"claude-sonnet-5-5",
		"web-app",
		"docs",
		"check before sharing",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("%q がない:\n%s", want, out)
		}
	}
	if strings.Contains(out, "\x1b") {
		t.Error("端末でないのに色がついている")
	}
	if strings.Contains(out, "Kiro credits") {
		t.Error("クレジットのない期間に Kiro credits が出ている")
	}

	// 記録のない期間は、そう書くだけ（エラーにしない）
	b.Reset()
	p, _ = parsePeriod("2025-01-01", "", time.Now())
	if err := writeStats(&b, data, nil, p, time.Now()); err != nil || !strings.Contains(b.String(), "No activity in the week of 2024-12-30") ||
		!strings.Contains(b.String(), "Dec 30, 2024 – Jan 5, 2025") {
		t.Errorf("記録のない期間: %v\n%s", err, b.String())
	}
}

// 履歴から来た名前（プロジェクト・モデル）に端末の制御文字があっても、端末にはそのまま出さない（セキュリティ）。
// ESC で始まる列は、色を変えるだけでなく、ウィンドウの題を書きかえたり、端末によってはクリップボードに書いたりできる。
func TestStatsStripsTerminalControls(t *testing.T) {
	setup(t)
	a := float64(time.Date(2026, 10, 6, 10, 0, 0, 0, time.Local).Unix())
	evil := "app\x1b]0;pwned\x07\x1b[2J\x1b]52;c;ZWNobyBoaQ==\x07\u202egnp.exe\r\n"
	data := []*core.Session{{ID: "s", Source: "Claude\x1b[31m Code", Project: evil, Start: a, End: a + 3600, Segs: [][3]float64{{a, a + 3600, 10}}}}
	p, _ := parsePeriod("2026-10-06", "", time.Now())
	var b bytes.Buffer
	if err := writeStats(&b, data, nil, p, time.Now()); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	for _, bad := range []string{"\x1b", "\x07", "\r", "\u202e"} {
		if strings.Contains(out, bad) {
			t.Errorf("制御文字 %q が出ている:\n%q", bad, out)
		}
	}
	if !strings.Contains(out, "app]0;pwned[2J]52;c;ZWNobyB…") || !strings.Contains(out, "Claude[31m Code") {
		t.Errorf("制御文字のほかは残す:\n%s", out)
	}
	if got := termSafe("a\u202egnp\u200b.exe\u2028\r\n", 20); got != "agnp.exe" {
		t.Errorf("向きを変える文字と改行: %q", got)
	}
	if got := termSafe(strings.Repeat("a", 40), 10); got != "aaaaaaaaa…" {
		t.Errorf("長い名前を切る: %q", got)
	}
	if got := termSafe("\x1b", 10); got != "—" {
		t.Errorf("制御文字だけの名前: %q", got)
	}
}

// stats は --week と --month のどちらか。どちらもなければ今週。
func TestStatsSubcommand(t *testing.T) {
	if err := dispatch([]string{"stats", "--week", "this", "--month", "this"}); err == nil || !strings.Contains(err.Error(), "either --week or --month") {
		t.Errorf("両方: %v", err)
	}
	if err := dispatch([]string{"stats", "--week", "10/5"}); err == nil {
		t.Error("読めない週がエラーにならない")
	}
	if !strings.Contains(helpText, "\n  stats ") {
		t.Error("help に stats がない")
	}
}
