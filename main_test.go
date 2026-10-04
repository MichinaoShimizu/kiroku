package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	_ "time/tzdata" // Windows などでも Asia/Tokyo を読めるように

	"github.com/MichinaoShimizu/kiroku/internal/core"
	"github.com/MichinaoShimizu/kiroku/internal/report"
	"github.com/MichinaoShimizu/kiroku/internal/source"
)

// testdata/ は合成データ（個人の履歴は入っていない）。golden.json は、
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
	// Python 版にあった 4 つの履歴だけで比べる（あとから足したアダプターは internal/source のテストで確かめる）
	python := map[string]bool{"Claude Code": true, "Kiro IDE": true, "Kiro CLI": true, "Kiro IDE (旧)": true}
	var picked []source.Source
	for _, s := range all {
		if python[s.Name()] {
			picked = append(picked, s)
		}
	}
	sessions, rep := collect(picked, map[string]bool{"claude": true, "kiro": true}, 15)
	weeks = report.AllWeeks(sessions)
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

// 月の集計は、日数ぶんの日を持ち、日ごとの合計が月の作業時間と一致する。
func TestMonthlySummary(t *testing.T) {
	sessions, weeks, _ := setup(t)
	data := sessions.([]*core.Session)
	for k, m := range report.AllMonths(data) {
		first, _ := time.ParseInLocation("2006-01", k, time.Local)
		if want := first.AddDate(0, 1, -1).Day(); len(m.Days) != want {
			t.Errorf("%s: 日数 %d, want %d", k, len(m.Days), want)
		}
		sum := 0
		for _, d := range m.Days {
			sum += d.Active
		}
		if sum != m.Active {
			t.Errorf("%s: 日ごとの合計 %d と月の作業時間 %d が違う", k, sum, m.Active)
		}
	}
	all := map[string]*report.Summary{}
	for k, w := range weeks {
		if len(w.Days) != 7 {
			t.Errorf("週 %s の日数 = %d", k, len(w.Days))
		}
		all["週 "+k] = w
	}
	for k, m := range report.AllMonths(data) {
		all["月 "+k] = m
	}
	// 日ごとの使用量を足すと、期間の使用量になる
	for k, x := range all {
		var tok, cost, cr float64
		for _, d := range x.Days {
			tok, cost, cr = tok+d.Tokens, cost+d.Cost, cr+d.Credits
		}
		if tok != x.Usage.Tokens || math.Abs(cost-x.Usage.Cost) > 0.01 || math.Abs(cr-x.Usage.Credits) > 0.01 {
			t.Errorf("%s: 日ごとの合計 tokens=%v cost=%v credits=%v、期間 %v %v %v", k, tok, cost, cr, x.Usage.Tokens, x.Usage.Cost, x.Usage.Credits)
		}
	}
}

func TestSameConversationCountedOnce(t *testing.T) {
	h := filepath.Join("testdata", "home")
	srcs := []source.Source{
		&source.KiroCLI{Home: filepath.Join(h, ".kiro")},
		&source.QStore{Label: "Kiro CLI (SQLite)", Fam: "kiro", DB: filepath.Join("testdata", "sqlite", "kiro-cli.sqlite3")},
	}
	data, rep := collect(srcs, map[string]bool{"kiro": true}, 15)
	if rep[1].N != 2 || rep[1].Dup != 1 {
		t.Fatalf("SQLite: n=%d dup=%d, want 2 と 1（新しい形式にもある会話は外す）", rep[1].N, rep[1].Dup)
	}
	if len(data) != rep[0].N+2 {
		t.Fatalf("セッション数 = %d", len(data))
	}
}

func TestNormalizeArgs(t *testing.T) {
	cases := map[string][]string{
		"--weekly":                        {"--weekly=latest"},
		"--weekly 2026-09-30":             {"--weekly=2026-09-30"},
		"--weekly --no-open":              {"--weekly=latest", "--no-open"},
		"--monthly":                       {"--monthly=latest"},
		"--monthly 2026-09":               {"--monthly=2026-09"},
		"--monthly --no-open":             {"--monthly=latest", "--no-open"},
		"--serve":                         {"--serve=127.0.0.1:8484"},
		"--serve :9000":                   {"--serve=:9000"},
		"--serve --no-open":               {"--serve=127.0.0.1:8484", "--no-open"},
		"--serve localhost:8485 --gap 20": {"--serve=localhost:8485", "--gap", "20"},
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

// compare は数値を誤差つきで比べる。Go 版で増やした項目（週の usage.unpriced・unpricedModels、native）は Python 版になかったので比べない。
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
			inDay := strings.Contains(path, ".days[") && !strings.Contains(path[strings.LastIndex(path, ".days[")+1:], ".")
			if (k == "unpriced" || k == "unpricedModels") && strings.HasSuffix(path, ".usage") || k == "native" ||
				inDay && (k == "tokens" || k == "cost" || k == "credits") || // Go 版で足した日ごとの使用量
				k == "projectStats" || k == "shares" || k == "limits" || k == "ctx" || k == "outputs" || k == "file" || k == "prs" || k == "outSessions" || k == "costPerCommit" || k == "git" || inDay && k == "commits" || // Go 版で足したプロジェクト別のまとめ・成果
				k == "whyEn" || k == "labelEn" || k == "detailEn" { // Go 版で足した英語表示の文言
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
