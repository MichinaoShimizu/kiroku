package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MichinaoShimizu/kiroku/internal/report"
)

// 期間の選び方と、比べる前の期間。進行中の期間は、前の期間も同じ日数までにそろえる。
func TestParseStatsPeriod(t *testing.T) {
	setup(t)
	now := time.Date(2026, 10, 7, 10, 0, 0, 0, time.Local) // 水曜
	day := func(t time.Time) string { return t.Format("2006-01-02") }
	cases := []struct {
		day, week, month           string
		mode, from, to, pfrom, pto string
		partial                    bool
	}{
		{"", "", "", "week", "2026-10-05", "2026-10-12", "2026-09-28", "2026-10-01", true}, // 何もなければ今週。先週の水曜まで
		{"", "last", "", "week", "2026-09-28", "2026-10-05", "2026-09-21", "2026-09-28", false},
		{"", "", "this", "month", "2026-10-01", "2026-11-01", "2026-09-01", "2026-09-08", true}, // 9 月の 7 日まで
		{"", "", "2026-03", "month", "2026-03-01", "2026-04-01", "2026-02-01", "2026-03-01", false},
		{"today", "", "", "day", "2026-10-07", "2026-10-08", "2026-10-06", "2026-10-07", true},
		{"yesterday", "", "", "day", "2026-10-06", "2026-10-07", "2026-10-05", "2026-10-06", false},
		{"2026-01-01", "", "", "day", "2026-01-01", "2026-01-02", "2025-12-31", "2026-01-01", false},
	}
	for _, c := range cases {
		sp, err := parseStatsPeriod(c.day, c.week, c.month, now)
		if err != nil {
			t.Fatalf("%+v: %v", c, err)
		}
		got := []string{sp.mode, day(sp.from), day(sp.to), day(sp.prevFrom), day(sp.prevTo)}
		want := []string{c.mode, c.from, c.to, c.pfrom, c.pto}
		if strings.Join(got, " ") != strings.Join(want, " ") || sp.partial != c.partial {
			t.Errorf("%q %q %q: got %v partial=%v, want %v partial=%v", c.day, c.week, c.month, got, sp.partial, want, c.partial)
		}
	}
	// 3 月 31 日に今月を見ると、前の月（2 月）は月末で止まる
	sp, err := parseStatsPeriod("", "", "this", time.Date(2026, 3, 31, 9, 0, 0, 0, time.Local))
	if err != nil || day(sp.prevTo) != "2026-03-01" {
		t.Errorf("prevTo = %v, %v", sp, err)
	}
	for _, bad := range [][3]string{{"today", "this", ""}, {"", "this", "this"}, {"tomorrow", "", ""}, {"", "", "2026-13"}, {"", "x", ""}} {
		if _, err := parseStatsPeriod(bad[0], bad[1], bad[2], now); err == nil {
			t.Errorf("%q: エラーにならない", bad)
		}
	}
}

// 履歴や git から来た文字列のエスケープシーケンス・制御文字・向きを変える文字は、端末にそのまま出さない。
func TestStatsTerminalInjection(t *testing.T) {
	setup(t)
	evil := "x\x1b]52;c;cHduZWQ=\x07\x1b[2J\u009b31m\u202eevil\r\nnext"
	sp, _ := parseStatsPeriod("", "last", "", time.Date(2026, 10, 7, 10, 0, 0, 0, time.Local))
	cur := &report.Summary{
		Active: 90, Sessions: 1, Prompts: 3, Days: make([]report.Day, 7),
		ProjectStats: []report.ProjectStat{{Project: evil, Minutes: 90, Sessions: 1, Prompts: 3,
			Top: []report.TopSession{{Title: evil, Source: evil, Start: float64(sp.from.Unix()), Minutes: 90}}}},
		Friction: []report.Friction{{Title: evil, Project: evil, WhyEn: []string{"2 corrections"}}},
	}
	cur.Usage.Models = [][4]any{{evil, 1.5, 1000.0, 2.0}}
	cur.Usage.Unpriced, cur.Usage.UnpricedM = 10, []string{evil}
	cur.Days[0].Active = 90
	var b bytes.Buffer
	writeStats(&b, style{}, sp, cur, nil, 5)
	out := b.String()
	for _, bad := range []string{"\x1b", "\x07", "\u009b", "\u202e", "\r"} {
		if strings.Contains(out, bad) {
			t.Errorf("出力に %q が残っている:\n%q", bad, out)
		}
	}
	if strings.Contains(out, "\nnext") {
		t.Errorf("履歴の改行で行が増えている:\n%s", out)
	}
	if !strings.Contains(out, "x?]52;c;cHduZWQ=??[2J?31m?evil  next") {
		t.Errorf("中身が読めなくなっている:\n%s", out)
	}
}

// 幅をそろえる（全角は 2 文字ぶん、長ければ「…」）。
func TestFit(t *testing.T) {
	cases := map[string]string{
		"abc":        "abc   ",
		"abcdefghij": "abcde…",
		"日本語":        "日本語",
		"日本語です":      "日本… ",
		"a\tb":       "a b   ",
	}
	for in, want := range cases {
		if got := fit(in, 6); got != want {
			t.Errorf("fit(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestStatsFormat(t *testing.T) {
	for _, c := range [][2]string{{dur(5), "5m"}, {dur(65), "1h 05m"}, {count(1234567), "1,234,567"}, {tokens(45_200_000), "45.2M"},
		{tokens(999), "999"}, {money(1.5), "$1.50"}, {money(12345), "$12,345"},
		{deltaNum(10, 4, true, count), "+6"}, {deltaNum(4, 10, true, count), "−6"}, {deltaNum(3, 3, true, count), "±0"}, {deltaNum(3, 0, false, count), ""},
		{ordinal(1), "1st"}, {ordinal(12), "12th"}, {ordinal(22), "22nd"}, {ordinal(23), "23rd"}} {
		if c[0] != c[1] {
			t.Errorf("got %q, want %q", c[0], c[1])
		}
	}
	if b := bar(50, 100, 4); b != "██" {
		t.Errorf("bar = %q", b)
	}
}

// kiroku stats をテストの履歴で動かす。テキストと --json、--project。
func TestStatsSubcommand(t *testing.T) {
	setup(t)
	h := filepath.Join("testdata", "home")
	base := []string{"stats", "--claude-root", filepath.Join(h, ".claude", "projects"), "--kiro-home", filepath.Join(h, ".kiro"), "--sources", "claude,kiro"}
	run := func(args ...string) string {
		t.Helper()
		f, err := os.CreateTemp(t.TempDir(), "out")
		if err != nil {
			t.Fatal(err)
		}
		old, oldLog := os.Stdout, logw
		os.Stdout = f
		t.Cleanup(func() { logw = oldLog })
		err = dispatch(append(append([]string{}, base...), args...))
		os.Stdout = old
		if err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		b, _ := os.ReadFile(f.Name())
		return string(b)
	}
	out := run("--month", "2026-09", "--top", "2")
	for _, want := range []string{"September 2026", "vs August", "Active time", "Projects", "Agents", "Models", "Claude Code", "--top shows more"} {
		if !strings.Contains(out, want) {
			t.Errorf("%q がない:\n%s", want, out)
		}
	}
	var j struct {
		SchemaVersion int `json:"schemaVersion"`
		Period        struct {
			Mode, From string
		} `json:"period"`
		Current  *report.Summary `json:"current"`
		Previous *report.Summary `json:"previous"`
	}
	if err := json.Unmarshal([]byte(run("--month", "2026-09", "--project", "APP", "--json")), &j); err != nil {
		t.Fatal(err)
	}
	if j.SchemaVersion != statsJSONSchema || j.Period.Mode != "month" || j.Period.From != "2026-09-01" || j.Current == nil || j.Previous != nil {
		t.Fatalf("JSON: %+v", j)
	}
	for _, ps := range j.Current.ProjectStats {
		if ps.Project != "app" {
			t.Errorf("--project app なのに %s が入っている", ps.Project)
		}
	}
	err := dispatch(append(append([]string{}, base...), "--project", "nope"))
	if err == nil || !strings.Contains(err.Error(), "recent projects: app") {
		t.Errorf("知らないプロジェクト: %v", err)
	}
	if err := dispatch(append(append([]string{}, base...), "--top", "0")); err == nil {
		t.Error("--top 0 がエラーにならない")
	}
}
