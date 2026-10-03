package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
	_ "time/tzdata" // Windows などでも Asia/Tokyo を読めるように

	"github.com/MichinaoShimizu/kiroku/internal/report"
	"github.com/MichinaoShimizu/kiroku/internal/source"
)

// testdata/ は合成データ（個人の履歴は入っていない）。golden.json と golden-week.md は、
// Go に移す前の Python 版 kiroku が同じデータから出した結果。Go 版がそれと同じ数字を出すことを確かめる。

func setup(t *testing.T) (data any, weeks map[string]*report.Week, rep []source.Report) {
	t.Helper()
	tokyo, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Skip("タイムゾーンのデータがない:", err)
	}
	time.Local = tokyo
	// 古い Kiro IDE は終了時刻にファイルの更新時刻を使う。git はそれを保存しないので、ここで戻す。
	var mt map[string]float64
	b, err := os.ReadFile("testdata/mtimes.json")
	if err != nil {
		t.Fatal(err)
	}
	json.Unmarshal(b, &mt)
	for rel, ts := range mt {
		tm := time.Unix(0, int64(ts*1e9))
		if err := os.Chtimes(filepath.Join("testdata", "home", filepath.FromSlash(rel)), tm, tm); err != nil {
			t.Fatal(err)
		}
	}
	h := filepath.Join("testdata", "home")
	all := source.All(source.Options{
		ClaudeRoot:   filepath.Join(h, ".claude", "projects"),
		KiroHome:     filepath.Join(h, ".kiro"),
		KiroStorages: []string{filepath.Join(h, ".config", "Kiro", "User", "globalStorage", "kiro.kiroagent")},
	})
	sessions, rep := collect(all, map[string]bool{"claude": true, "kiro": true}, 15)
	weeks = report.AllWeeks(sessions)
	report.Annotate(weeks, report.ReadJournal(filepath.Join("testdata", "journal")))
	return sessions, weeks, rep
}

func TestMatchesPythonVersion(t *testing.T) {
	sessions, weeks, _ := setup(t)
	got := roundTrip(t, map[string]any{"sessions": sessions, "weeks": weeks})
	var want any
	b, err := os.ReadFile("testdata/golden.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &want); err != nil {
		t.Fatal(err)
	}
	var diffs []string
	compare(want, got, "", &diffs)
	for i, d := range diffs {
		if i == 30 {
			t.Errorf("…ほか %d 件", len(diffs)-30)
			break
		}
		t.Error(d)
	}
}

func TestWeeklyMarkdownMatchesPythonVersion(t *testing.T) {
	_, weeks, rep := setup(t)
	keys := make([]string, 0, len(weeks))
	for k := range weeks {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	got := report.WeeklyMarkdown(weeks[keys[len(keys)-1]], rep, "")
	want, err := os.ReadFile("testdata/golden-week.md")
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		os.WriteFile(filepath.Join(t.TempDir(), "got.md"), []byte(got), 0o644)
		t.Errorf("Markdown が違う:\n--- want\n%s\n--- got\n%s", want, got)
	}
}

func TestWeeklyKeepsHandWrittenSection(t *testing.T) {
	_, weeks, rep := setup(t)
	st := weeks["2026-09-21"]
	first := report.WeeklyMarkdown(st, rep, "")
	edited := first + "\n- 自分のメモ\n"
	again := report.WeeklyMarkdown(st, rep, edited)
	if !contains(again, "- 自分のメモ") {
		t.Fatal("作り直したら自分で書いた欄が消えた")
	}
}

func TestNormalizeArgs(t *testing.T) {
	cases := map[string][]string{
		"--weekly":            {"--weekly=latest"},
		"--weekly 2026-09-30": {"--weekly=2026-09-30"},
		"--weekly --no-open":  {"--weekly=latest", "--no-open"},
	}
	for in, want := range cases {
		got := normalizeArgs(splitArgs(in))
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("%q → %v, want %v", in, got, want)
		}
	}
}

func splitArgs(s string) []string {
	var out []string
	cur := ""
	for _, r := range s + " " {
		if r == ' ' {
			if cur != "" {
				out = append(out, cur)
			}
			cur = ""
			continue
		}
		cur += string(r)
	}
	return out
}

func contains(s, sub string) bool { return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0) }

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func roundTrip(t *testing.T, v any) any {
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out any
	json.Unmarshal(b, &out)
	return out
}

// compare は数値を誤差つきで比べる。Go 版で増やした週の usage.unpriced は Python 版になかったので比べない。
func compare(want, got any, path string, diffs *[]string) {
	switch w := want.(type) {
	case map[string]any:
		g, ok := got.(map[string]any)
		if !ok {
			*diffs = append(*diffs, fmt.Sprintf("%s: 型が違う", path))
			return
		}
		keys := map[string]bool{}
		for k := range w {
			keys[k] = true
		}
		for k := range g {
			keys[k] = true
		}
		for k := range keys {
			if k == "unpriced" && filepath.Base(filepath.ToSlash(path)) == "usage" {
				continue
			}
			wv, wok := w[k]
			gv, gok := g[k]
			if !wok || !gok {
				if empty(wv) && empty(gv) {
					continue
				}
				*diffs = append(*diffs, fmt.Sprintf("%s.%s: want %v, got %v", path, k, wv, gv))
				continue
			}
			compare(wv, gv, path+"."+k, diffs)
		}
	case []any:
		g, ok := got.([]any)
		if !ok || len(g) != len(w) {
			*diffs = append(*diffs, fmt.Sprintf("%s: 長さが違う want %d got %v", path, len(w), lenOf(got)))
			return
		}
		for i := range w {
			compare(w[i], g[i], fmt.Sprintf("%s[%d]", path, i), diffs)
		}
	case float64:
		g, ok := got.(float64)
		if !ok || math.Abs(w-g) > 1e-6+1e-9*math.Abs(w) {
			*diffs = append(*diffs, fmt.Sprintf("%s: want %v got %v", path, w, got))
		}
	default:
		if fmt.Sprint(want) != fmt.Sprint(got) {
			*diffs = append(*diffs, fmt.Sprintf("%s: want %v got %v", path, want, got))
		}
	}
}

func empty(v any) bool {
	switch x := v.(type) {
	case nil:
		return true
	case string:
		return x == ""
	case []any:
		return len(x) == 0
	}
	return false
}

func lenOf(v any) any {
	if l, ok := v.([]any); ok {
		return len(l)
	}
	return v
}
