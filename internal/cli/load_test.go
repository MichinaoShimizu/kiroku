package cli

import (
	"bytes"
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
	python := map[string]bool{"Claude Code": true, "Kiro IDE": true, "Kiro CLI": true, "Kiro IDE (legacy)": true}
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
	// golden.json は、Sonnet 5.5 のキャッシュ読み込みを $0.20 としていたころの料金表で作った。比べるのは計算の仕方なので、そのときの料金に戻す
	old := core.Prices["claude-sonnet-5-5"]
	core.Prices["claude-sonnet-5-5"] = [5]float64{2, 10, 2.5, 4, 0.20}
	t.Cleanup(func() { core.Prices["claude-sonnet-5-5"] = old })
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

// Kiro IDE を v1.0 へ移すと、v1.0 より前の形式の会話も残る（kiro.dev の whats-new-v1）。同じ ID の会話は v1.0 のほうだけを数える。
func TestKiroIDEMigratedCountedOnce(t *testing.T) {
	home, gs := t.TempDir(), t.TempDir()
	files := map[string]string{
		filepath.Join(home, "sessions", "h", "sess_1", "session.json"):   `{"id": "conv-a", "createdAt": "2026-09-29T01:00:00Z", "modelId": "claude-sonnet-4.5"}`,
		filepath.Join(home, "sessions", "h", "sess_1", "messages.jsonl"): `{"timestamp": "2026-09-29T01:01:00Z", "payload": {"type": "user", "content": "画面を作って"}}` + "\n",
		filepath.Join(gs, "workspace-sessions", "d3M=", "sessions.json"): `[{"sessionId": "conv-a", "dateCreated": 1759100000000}, {"sessionId": "conv-b", "dateCreated": 1759100000000}]`,
		filepath.Join(gs, "workspace-sessions", "d3M=", "conv-a.json"):   `{"history": [{"message": {"role": "user", "content": "画面を作って"}}]}`,
		filepath.Join(gs, "workspace-sessions", "d3M=", "conv-b.json"):   `{"history": [{"message": {"role": "user", "content": "表を作って"}}]}`,
	}
	for p, body := range files {
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	all := source.All(source.Options{KiroHome: home, KiroStorages: []string{gs}, CrewHome: filepath.Join(home, "crew"),
		KiroCLIDB: filepath.Join(home, "none.sqlite3")})
	data, rep := collect(all, map[string]bool{"kiro": true}, 15)
	n := map[string]source.Report{}
	for _, r := range rep {
		n[r.Name] = r
	}
	if r := n["Kiro IDE"]; r.N != 1 || r.Error != nil {
		t.Errorf("Kiro IDE: n=%d err=%v, want 1", r.N, r.Error)
	}
	if r := n["Kiro IDE (legacy)"]; r.N != 1 || r.Dup != 1 || r.Error != nil {
		t.Errorf("Kiro IDE (legacy): n=%d dup=%d err=%v, want 1 と 1（v1.0 にもある会話は外す。読めないファイルではない）", r.N, r.Dup, r.Error)
	}
	if len(data) != 2 {
		t.Fatalf("セッション数 = %d, want 2", len(data))
	}
	for _, s := range data {
		if s.ID == "conv-a" && s.Source != "Kiro IDE" {
			t.Errorf("conv-a は v1.0 のほうを使う: %s", s.Source)
		}
	}
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

// compare は数値を誤差つきで比べる。Go 版で増やした項目（週の usage.unpriced・unpricedModels・prefixPriced、native）は Python 版になかったので比べない。
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
			if (k == "unpriced" || k == "unpricedModels" || k == "prefixPriced") && strings.HasSuffix(path, ".usage") || k == "native" ||
				inDay && (k == "tokens" || k == "cost" || k == "credits") || // Go 版で足した日ごとの使用量
				k == "projectStats" || k == "shares" || k == "limits" || k == "limitResets" || k == "ctx" || k == "ctxWindow" || k == "outputs" || k == "file" || k == "prs" || k == "outSessions" || k == "outBase" || k == "costPerCommit" || k == "git" || inDay && k == "commits" || // Go 版で足したプロジェクト別のまとめ・アウトプット
				k == "costPerAsk" || k == "costPrompts" || k == "outPrompts" || // Go 版で、トークンやアウトプットを記録するセッションの依頼だけで割るようにした
				k == "whyEn" || k == "labelEn" || k == "detailEn" || // Go 版で足した英語表示の文言
				k == "interruptsAt" || k == "prAt" || strings.Contains(path, ".prompts[") && (k == "work" || k == "wait" || k == "len") { // Go 版で足した依頼の流れの出来事と時間
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

// serve・html の読み込みの行は、履歴があったエージェントだけを並べ、残りは1行にまとめる
// （どれも 0 件のときは、どこを探したのか分かるように全部出す）。
func TestLogSources(t *testing.T) {
	old := logw
	t.Cleanup(func() { logw = old })
	var b bytes.Buffer
	logw = &b
	rep := []source.Report{
		{Name: "Claude Code", N: 12, Where: "/x/projects", Took: 1520 * time.Millisecond},
		{Name: "Codex", Where: "/x/.codex"},
		{Name: "Kiro IDE (legacy)", Where: "none"},
	}
	logSources(rep, true)
	out := b.String()
	if !strings.Contains(out, "Claude Code: 12 sessions (/x/projects) in 1.5s") {
		t.Errorf("見つかったものが出ていない:\n%s", out)
	}
	if !strings.Contains(out, "no history yet: Codex, Kiro IDE (legacy)") {
		t.Errorf("0 件のまとめがない:\n%s", out)
	}
	if strings.Contains(out, "/x/.codex") {
		t.Errorf("0 件の場所まで出ている:\n%s", out)
	}
	// どれも 0 件なら、探した場所を全部出す。場所が分からないものはそう書く
	b.Reset()
	logSources(rep[1:], false)
	out = b.String()
	if !strings.Contains(out, "Codex: 0 sessions (/x/.codex)") || !strings.Contains(out, "(not installed)") {
		t.Errorf("どれも 0 件のときに場所が出ていない:\n%s", out)
	}
	if strings.Contains(out, "no history yet") {
		t.Errorf("どれも 0 件なのにまとめている:\n%s", out)
	}
}

// --prices で Haiku 5.5 を上書きすると、足した料金をプロンプトの長さによらず使う（100K 超えの料金は使わない）。
func TestLoadPricesOverridesLongPrices(t *testing.T) {
	oldP, oldL := core.Prices["claude-haiku-5-5"], core.LongPrices["claude-haiku-5-5"]
	t.Cleanup(func() {
		core.Prices["claude-haiku-5-5"] = oldP
		core.LongPrices["claude-haiku-5-5"] = oldL
	})
	path := filepath.Join(t.TempDir(), "prices.json")
	if err := os.WriteFile(path, []byte(`{"Claude-Haiku-5-5": {"input": 1, "output": 2}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := loadPrices(path); err != nil {
		t.Fatal(err)
	}
	c, ok := core.CostOfRequest("claude-haiku-5-5", core.Tokens{In: 200_000, Out: 1_000_000})
	if want := 0.2 + 2.0; !ok || math.Abs(c-want) > 1e-9 {
		t.Errorf("cost = %v, want %v", c, want)
	}
}
