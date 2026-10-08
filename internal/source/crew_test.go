package source

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	_ "time/tzdata"

	"github.com/MichinaoShimizu/kiroku/internal/core"
)

// testdata/crew は Kiro Crew の作り（session_map.json・sessions/*.jsonl・subagents/*/state.json）に合わせた合成データ。
func TestKiroCrewTagsKiroCLISessions(t *testing.T) {
	td := filepath.Join("..", "..", "testdata")
	k := &KiroCLI{Home: filepath.Join(td, "home", ".kiro"), CrewHome: filepath.Join(td, "crew")}
	bs := load(t, k)
	if len(bs) != 5 {
		t.Fatalf("会話の数 = %d, want 5（kiro-cli の 3 件のうちサブエージェントは親にまとめて 2 件 + Crew の記録だけにある 3 件）", len(bs))
	}
	crew := 0
	for _, b := range bs {
		if b.Source == "Kiro Crew" {
			crew++
		}
	}
	if crew != 5 || k.Detail() != "うち Kiro Crew から 3 件・Crew の使用量の記録でクレジットを補った会話 1 件・Crew の使用量の記録だけにある 3 件（2.00 クレジット）" {
		t.Fatalf("Crew の目印 = %d, detail = %q", crew, k.Detail())
	}
	parent := find(bs, "1cb4ad2f-90ba-4c5f-970b-5767003804b9")
	if parent == nil || parent.Title != "夜間のデプロイ見張り" {
		t.Fatalf("Crew のタイトル = %+v", parent)
	}
	if find(bs, "0d612aac-99c4-4a54-bda2-76ad60b6ddf3") != nil {
		t.Error("サブエージェントの会話は、別のセッションにしない")
	}
	// サブエージェントは親のセッションの中に出す（Claude Code・Codex と同じ）
	if len(parent.Subagents) != 1 || parent.Subagents[0].Type != "reviewer" || !strings.HasPrefix(parent.Subagents[0].Desc, "PR #12") || parent.Subagents[0].Start == nil {
		t.Errorf("親のサブエージェント = %+v", parent.Subagents)
	}
	if b := find(bs, "1e13c3c1-d7ae-41c2-a324-e6a440665d9a"); b == nil || b.Title != "CLIでデプロイ確認" { // 古い形（文字列だけ）の対応表。タイトルは kiro-cli のまま
		t.Errorf("古い対応表のタイトル = %+v", b)
	}
	// まとめても数字は同じ: Crew から動かした会話・うちサブエージェントは親の会話の参考指標に入る
	if n := nativeOf(parent.Finish(15)); n["Crew から動かした会話"].V != 2 || n["うちサブエージェント"].V != 1 || n["ターン"].V == 0 {
		t.Errorf("Crew の参考指標 = %+v", n)
	}
}

// Crew の使用量の記録（usage/tokens）: 同じ会話は多いほうを使い、kiro-cli にない記録は Crew のセッションにする。
func TestKiroCrewUsage(t *testing.T) {
	tokyo, err := time.LoadLocation("Asia/Tokyo") // 裏方の処理は、このマシンの時刻の日ごとにまとめる
	if err != nil {
		t.Skip(err)
	}
	defer func(l *time.Location) { time.Local = l }(time.Local)
	time.Local = tokyo
	td := filepath.Join("..", "..", "testdata")
	k := &KiroCLI{Home: filepath.Join(td, "home", ".kiro"), CrewHome: filepath.Join(td, "crew")}
	by := map[string]*core.Builder{}
	for _, b := range load(t, k) {
		by[b.ID] = b
	}
	credits := func(id string) float64 { return math.Round(sumCredits(by[id].Credits)*100) / 100 }
	// dashboard の会話: kiro-cli の記録 3.69 より Crew の記録 4.5 が多いので、Crew のほうを使う。
	// サブエージェント（kiro-cli の記録 2.28）は親の会話にまとめるので、そのクレジットも足される
	if v := credits("1cb4ad2f-90ba-4c5f-970b-5767003804b9"); v != 6.78 {
		t.Errorf("Crew の記録で補った会話のクレジット = %v, want 6.78（4.5 + サブエージェントの 2.28）", v)
	}
	// legacy-key の会話: Crew の記録 0.01 のほうが少ないので、kiro-cli の記録のまま
	if v := credits("1e13c3c1-d7ae-41c2-a324-e6a440665d9a"); v <= 0.01 {
		t.Errorf("kiro-cli の記録が多いときは置きかえない: %v", v)
	}
	// kiro-cli にない記録: 裏方の処理は日ごと、チャットは会話ごと。壊れた行（credits が負）と tokens 以外の行は数えない
	for id, want := range map[string]float64{"crew:_bg:2026-09-29": 0.25, "crew:_bg:2026-09-30": 0.5, "crew:dashboard:chat-9-1790000000": 1.25} {
		b := by[id]
		if b == nil {
			t.Errorf("%s がない", id)
			continue
		}
		if v := credits(id); v != want {
			t.Errorf("%s のクレジット = %v, want %v", id, v, want)
		}
		if b.Source != "Kiro Crew" || b.Key != "kiro-crew:"+id[len("crew:"):] {
			t.Errorf("%s: source=%q key=%q", id, b.Source, b.Key)
		}
	}
	if s := by["crew:dashboard:chat-9-1790000000"].Finish(15); s == nil || s.Models[0][0] != "deepseek-3.2" {
		t.Errorf("Crew のセッションのモデル = %+v", s)
	}
}

func TestNoCrew(t *testing.T) {
	td := filepath.Join("..", "..", "testdata")
	k := &KiroCLI{Home: filepath.Join(td, "home", ".kiro"), CrewHome: filepath.Join(t.TempDir(), "none")}
	for _, b := range load(t, k) {
		if b.Source != "Kiro CLI" {
			t.Fatal("Crew がなければ目印はつけない")
		}
	}
	if k.Detail() != "" {
		t.Fatal(k.Detail())
	}
}

// Crew の会話の記録（sessions/<キー>.jsonl）から依頼の流れとツールを補う。
func TestKiroCrewTranscript(t *testing.T) {
	td := filepath.Join("..", "..", "testdata")
	kh, ch := t.TempDir(), t.TempDir()
	if err := os.CopyFS(kh, os.DirFS(filepath.Join(td, "home", ".kiro"))); err != nil {
		t.Fatal(err)
	}
	if err := os.CopyFS(ch, os.DirFS(filepath.Join(td, "crew"))); err != nil {
		t.Fatal(err)
	}
	// Crew のダッシュボードから動かした会話: kiro-cli の履歴に依頼が残っていない
	os.WriteFile(filepath.Join(kh, "sessions", "cli", "1cb4ad2f-90ba-4c5f-970b-5767003804b9.jsonl"), nil, 0o644)
	// kiro-cli の会話に結びつかない記録（使用量だけ）と、会話の記録だけの会話
	os.WriteFile(filepath.Join(ch, "sessions", "dashboard_chat-9-1790000000.jsonl"), []byte(`{"_type": "metadata", "title": "API の調査"}
{"role": "user", "content": "API の遅さを調べて", "ts": "2026-09-30T11:00:00+09:00"}
{"role": "assistant", "content": "調べます", "ts": "2026-09-30T11:02:00+09:00", "tools": ["fs_read", "execute_bash"]}
`), 0o644)
	// 退避された古い行（sessions/archive/<名前>__<日時>.jsonl）も読む
	os.MkdirAll(filepath.Join(ch, "sessions", "archive"), 0o755)
	os.WriteFile(filepath.Join(ch, "sessions", "archive", "dashboard_chat-9-1790000000__20260930-100000.jsonl"), []byte(`{"_type": "archive", "reason": "rotate"}
{"role": "user", "content": "まず現状を教えて", "ts": "2026-09-30T10:55:00+09:00"}
`), 0o644)
	os.WriteFile(filepath.Join(ch, "sessions", "slack_C1_123.jsonl"), []byte(`{"_type": "metadata", "title": "Slack の相談"}
{"role": "user", "content": "リリースノートを書いて", "ts": "2026-09-30T13:00:00+09:00"}
{"role": "assistant", "content": "書きました", "ts": "2026-09-30T13:05:00+09:00"}
`), 0o644)
	k := &KiroCLI{Home: kh, CrewHome: ch}
	by := map[string]*core.Builder{}
	for _, b := range load(t, k) {
		by[b.ID] = b
	}
	prompts := func(id string) []string {
		var out []string
		if b := by[id]; b != nil {
			for _, p := range b.Prompts {
				out = append(out, p.Text)
			}
		}
		return out
	}
	if got := prompts("1cb4ad2f-90ba-4c5f-970b-5767003804b9"); len(got) != 1 || got[0] != "デプロイを見ておいて" {
		t.Errorf("kiro-cli に依頼がない会話 = %v, want Crew の記録から補う", got)
	}
	if got := prompts("crew:dashboard:chat-9-1790000000"); len(got) != 2 || got[0] != "まず現状を教えて" || got[1] != "API の遅さを調べて" {
		t.Errorf("使用量だけの会話の依頼 = %v", got)
	}
	if b := by["crew:dashboard:chat-9-1790000000"]; b.Title != "API の調査" || b.ToolCounts()["execute_bash"] != 1 {
		t.Errorf("使用量だけの会話: title=%q tools=%v", b.Title, b.ToolCounts())
	}
	if b := by["crew:slack_C1_123"]; b == nil || b.Title != "Slack の相談" || len(b.Prompts) != 1 || b.Source != "Kiro Crew" {
		t.Errorf("会話の記録だけの会話 = %+v", b)
	} else if r := b.Finish(15).Prompts[0].Reply; r == nil || r.Text != "書きました" {
		t.Errorf("Crew の会話の記録からの応答 = %+v, want 書きました", r)
	}
	if !strings.Contains(k.Detail(), "Crew の会話の記録だけにある 1 件") {
		t.Errorf("detail = %q", k.Detail())
	}
	// 記録が kiro-cli にも Crew にもある会話は二重に出さない（dashboard の記録は kiro-cli の会話に使った）
	if by["crew:dashboard_main_1759400000.123"] != nil {
		t.Error("kiro-cli の会話に結びついた記録を、別の会話として出している")
	}
}

// 使用量の記録のダッシュボードの会話キーは、会話のキー（dashboard:chat-…）にそろえる。
func TestCrewSpendKey(t *testing.T) {
	for in, want := range map[string]string{
		"chat-89-1788925480":            "dashboard:chat-89-1788925480",
		"dashboard:chat-89-1788925480":  "dashboard:chat-89-1788925480",
		"dashboard:main/1759400000.123": "dashboard:main/1759400000.123",
		"_bg":                           "_bg",
		"slack:123.456":                 "slack:123.456",
	} {
		if got := spendKey(in); got != want {
			t.Errorf("spendKey(%q) = %q, want %q", in, got, want)
		}
	}
}

// クレジットの単位は表記が揺れても数える。クレジット以外の単位は数えない。
func TestCreditsOfUnit(t *testing.T) {
	list := []any{
		map[string]any{"unit": "credit", "value": 1.0},
		map[string]any{"unit": "Credits", "value": 0.5},
		map[string]any{"unit": " CREDIT ", "value": 0.25},
		map[string]any{"value": 2.0}, // 単位がなければクレジット
		map[string]any{"unit": "token", "value": 100.0},
	}
	if got := creditsOf(list, "unit", "value"); got != 3.75 {
		t.Errorf("creditsOf = %v, want 3.75", got)
	}
}

// 保存期間：Crew の config.json に config.local.json を重ねた session.archive_retention_days を読む。
// 既定は 30 日（未設定扱い）、null か負の数は消さない（nil）、型が違う値や壊れたファイルは既定値。
func TestCrewRetention(t *testing.T) {
	cases := []struct {
		name, base, local string
		days              int
		never             bool
	}{
		{"設定ファイルなし", "", "", 30, false},
		{"キーなし", `{"session": {"pool_size": 0}}`, "", 30, false},
		{"既定値を書き出したまま", `{"session": {"archive_retention_days": 30}}`, "", 30, false},
		{"config.json で設定", `{"session": {"archive_retention_days": 90}}`, "", 90, false},
		{"小数点つきの整数", `{"session": {"archive_retention_days": 45.0}}`, "", 45, false},
		{"整数の文字列", `{"session": {"archive_retention_days": " 60 "}}`, "", 60, false},
		{"-1 は消さない", `{"session": {"archive_retention_days": -1}}`, "", 0, true},
		{"null は消さない", `{"session": {"archive_retention_days": null}}`, "", 0, true},
		{"とても長い日数は消さないのと同じ", `{"session": {"archive_retention_days": 1e12}}`, "", 0, true},
		{"0 日", `{"session": {"archive_retention_days": 0}}`, "", 0, false},
		{"整数でない数は既定値", `{"session": {"archive_retention_days": 7.5}}`, "", 30, false},
		{"数でない文字列は既定値", `{"session": {"archive_retention_days": "forever"}}`, "", 30, false},
		{"真偽値は既定値", `{"session": {"archive_retention_days": true}}`, "", 30, false},
		{"入れ子でない書き方は読まない", `{"session.archive_retention_days": 90}`, "", 30, false},
		{"session が object でない", `{"session": [90]}`, "", 30, false},
		{"壊れた config.json", `{"session": {"archive_retention_days": 90`, "", 30, false},
		{"object でない config.json", `[{"session": {"archive_retention_days": 90}}]`, "", 30, false},
		{"BOM つき", "\xef\xbb\xbf" + `{"session": {"archive_retention_days": 14}}`, "", 14, false},
		{"config.local.json が勝つ", `{"session": {"archive_retention_days": 90}}`, `{"session": {"archive_retention_days": 365}}`, 365, false},
		{"config.local.json で消さない", `{"session": {"archive_retention_days": 90}}`, `{"session": {"archive_retention_days": -1}}`, 0, true},
		{"config.local.json にキーがなければ config.json", `{"session": {"archive_retention_days": 90}}`, `{"session": {"pool_size": 2}}`, 90, false},
		{"config.local.json だけ", "", `{"session": {"archive_retention_days": 120}}`, 120, false},
		{"壊れた config.local.json は無視", `{"session": {"archive_retention_days": 90}}`, `{`, 90, false},
		{"config.local.json の session が object でなければ上書き", `{"session": {"archive_retention_days": 90}}`, `{"session": null}`, 30, false},
		{"壊れた config.json に config.local.json", `oops`, `{"session": {"archive_retention_days": 21}}`, 21, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			home := t.TempDir()
			if c.base != "" {
				os.WriteFile(filepath.Join(home, "config.json"), []byte(c.base), 0o600)
			}
			if c.local != "" {
				os.WriteFile(filepath.Join(home, "config.local.json"), []byte(c.local), 0o600)
			}
			days, never := crewRetentionDays(home)
			if days != c.days || never != c.never {
				t.Fatalf("crewRetentionDays = %d, %v, want %d, %v", days, never, c.days, c.never)
			}
			r := (&KiroCLI{Home: t.TempDir(), CrewHome: home}).Retention()
			if c.never {
				if r != nil {
					t.Fatalf("消さない設定なのに Retention = %+v", r)
				}
				return
			}
			// 0 日は Now で表す（Days の 0 は「わからない」）
			if r == nil || r.Days != c.days || r.Now != (c.days == 0) || r.Set != (c.days != 30) || r.Who != "Kiro Crew" ||
				r.Setting != "session.archive_retention_days" || r.Docs == "" || r.Snippet == "" ||
				r.File != filepath.Join(home, "config.local.json") {
				t.Fatalf("Retention = %+v, want %d 日", r, c.days)
			}
		})
	}
}

// Crew を使っていなければ（Crew の場所がなければ）保存期間は出さない。設定ファイルがディレクトリでも読まない。
func TestCrewRetentionNoCrew(t *testing.T) {
	if r := (&KiroCLI{Home: t.TempDir(), CrewHome: filepath.Join(t.TempDir(), "none")}).Retention(); r != nil {
		t.Errorf("Crew がないのに Retention = %+v", r)
	}
	home := t.TempDir()
	os.Mkdir(filepath.Join(home, "config.json"), 0o700)
	if days, never := crewRetentionDays(home); days != 30 || never {
		t.Errorf("config.json がディレクトリ = %d, %v, want 既定の 30 日", days, never)
	}
}

// Crew の使用量の記録のうち、kiro-cli 以外の backend の行（トークン・USD の cost）を読む。
// Claude Code の seam（行の provider か session_map の provider が claude_code）と Codex（session_map の provider が codex）は、
// それぞれの履歴が同じトークンを記録しているので、トークンとドル額を足さない。コンテキストの使用率と stop_reason はどの行からも読む。
func TestKiroCrewNonKiroBackends(t *testing.T) {
	kiro, crew := t.TempDir(), t.TempDir()
	n := 0
	row := func(slot, provider, model string, in, out, cw, cr, cost float64, extra string) string {
		n++
		return fmt.Sprintf(`{"_type": "tokens", "ts": "2026-09-29T10:%02d:00+00:00", "slot": %q, "app": "", "provider": %q, "model": %q, "input": %v, "output": %v, "cache_create": %v, "cache_read": %v, "cost": %v, "credits": 0.0, "turns": 1, "duration_ms": 1000, "surface": "dashboard", "agent": ""%s}`,
			n, slot, provider, model, in, out, cw, cr, cost, extra)
	}
	writeFiles(t, kiro, map[string]string{"sessions/cli/.keep": ""})
	writeFiles(t, crew, map[string]string{
		"session_map.json": `{"dashboard:chat-1-100": {"sid": "cc-sid", "provider": "claude_code", "cwd": "/Users/me/app"},
 "dashboard:chat-2-200": {"sid": "gs-sid", "provider": "goose", "cwd": "/Users/me/lib"},
 "dashboard:chat-3-300": {"sid": "cx-sid", "provider": "codex"}}`,
		"usage/tokens/2026-09-29.jsonl": strings.Join([]string{
			// Claude Code の seam: Claude Code の履歴が同じ分を記録している
			row("chat-1-100", "claude_code", "claude-sonnet-4-5", 100, 50, 10, 1000, 0.5, `, "context_used": 50000, "context_window": 200000, "stop_reason": "end_turn"`),
			// goose（行の provider は acp）: トークンと、会話の累計の差分の cost（最初のターンは 0）
			row("chat-2-200", "acp", "claude-sonnet-4-5", 1000, 200, 0, 0, 0, `, "context_used": 20000, "context_window": 100000, "stop_reason": "end_turn"`),
			row("chat-2-200", "acp", "claude-sonnet-4-5", 500, 100, 0, 0, 0.75, `, "context_used": 150000, "context_window": 100000, "stop_reason": "cancelled"`),
			// Codex（session_map の provider で見分ける）
			row("chat-3-300", "acp", "gpt-5", 10, 10, 0, 0, 0, `, "context_used": 1, "context_window": 0`),
			// session_map にない会話: cost がなければ料金表で見積もる（料金表にないモデルは見積もらない）
			row("chat-4-400", "acp", "glm-5", 300, 30, 0, 0, 0, ""),
			row("chat-5-500", "acp", "claude-haiku-4-5", 1000, 100, 0, 0, 0, `, "context_used": -5, "context_window": 1000`),
			// 壊れた値は 0 と読む
			row("chat-6-600", "acp", "claude-haiku-4-5", -1, 0, 0, 0, -3, `, "output": "x"`),
		}, "\n") + "\n",
	})
	k := &KiroCLI{Home: kiro, CrewHome: crew}
	by := map[string]*core.Session{}
	for _, b := range load(t, k) {
		by[b.ID] = b.Finish(15)
	}
	get := func(slot string) *core.Session {
		t.Helper()
		f := by["crew:dashboard:"+slot]
		if f == nil {
			t.Fatalf("%s がない: %v", slot, by)
		}
		return f
	}
	cc := get("chat-1-100")
	if cc.Usage.Total() != 0 || cc.Cost != 0 || len(cc.Models) != 1 || cc.Models[0][0] != "claude-sonnet-4-5" {
		t.Errorf("Claude Code の seam: usage = %+v, cost = %v, models = %v（トークンとコストは Claude Code の履歴で数える）", cc.Usage, cc.Cost, cc.Models)
	}
	if n := nativeOf(cc); n["コンテキストの最大使用率"].V != 25 || n["正常に終わらなかったターン"].V != 0 || n["正常に終わらなかったターン"].N != 1 {
		t.Errorf("Claude Code の seam の参考指標 = %+v", cc.Native)
	}
	gs := get("chat-2-200")
	if gs.Usage.In != 1500 || gs.Usage.Out != 300 || gs.Usage.CW != 0 || gs.Usage.CR != 0 || math.Abs(gs.Cost-0.75) > 1e-9 || gs.Usage.Unpriced != 0 {
		t.Errorf("goose: usage = %+v, cost = %v, want 1500/300 と $0.75（cost のある会話は 0 の行を見積もらない）", gs.Usage, gs.Cost)
	}
	if gs.ProjectPath != "/Users/me/lib" || len(gs.Models) != 1 || gs.Models[0][1] != 2 {
		t.Errorf("goose: project = %q, models = %v", gs.ProjectPath, gs.Models)
	}
	if n := nativeOf(gs); n["コンテキストの最大使用率"].V != 100 || n["正常に終わらなかったターン"].V != 1 || n["ターン"].V != 2 {
		t.Errorf("goose の参考指標 = %+v（上限を超えた使用率は 100%%、cancelled は正常に終わらなかったターン）", gs.Native)
	}
	if cx := get("chat-3-300"); cx.Usage.Total() != 0 || nativeOf(cx)["コンテキストの最大使用率"].N != 0 {
		t.Errorf("Codex: usage = %+v, native = %+v（窓が 0 の使用率は使わない）", cx.Usage, cx.Native)
	}
	if f := get("chat-4-400"); f.Usage.Unpriced != 330 || f.Cost != 0 {
		t.Errorf("料金表にないモデル: usage = %+v, cost = %v", f.Usage, f.Cost)
	}
	if f := get("chat-5-500"); math.Abs(f.Cost-0.0015) > 1e-12 || nativeOf(f)["コンテキストの最大使用率"].N != 0 {
		t.Errorf("料金表で見積もる: cost = %v, native = %+v（負の使用量は使わない）", f.Cost, f.Native)
	}
	if f := get("chat-6-600"); f.Usage.Total() != 0 || f.Cost != 0 {
		t.Errorf("壊れた値: usage = %+v, cost = %v", f.Usage, f.Cost)
	}
	if d := k.DetailEn(); !strings.Contains(d, "tokens and cost of 2 turns run on Claude Code or Codex are counted from their own history") {
		t.Errorf("detail = %q", d)
	}
}

// kiro-cli の会話に結びついた Crew の記録からも、コンテキストの使用率と stop_reason を読む（クレジットの置きかえとは別）。
func TestKiroCrewNativeOnKiroCLISession(t *testing.T) {
	kiro, crew := t.TempDir(), t.TempDir()
	writeFiles(t, kiro, map[string]string{
		"sessions/cli/k1.json":  `{"session_id": "k1", "cwd": "/Users/me/app", "created_at": "2026-09-29T10:00:00Z", "session_state": {"conversation_metadata": {"user_turn_metadatas": [{"end_timestamp": "2026-09-29T10:01:00Z", "metering_usage": [{"value": 5, "unit": "credit"}]}]}}}`,
		"sessions/cli/k1.jsonl": "",
	})
	writeFiles(t, crew, map[string]string{
		"session_map.json":              `{"dashboard:chat-1-100": {"sid": "k1"}}`,
		"usage/tokens/2026-09-29.jsonl": `{"_type": "tokens", "ts": "2026-09-29T10:01:00+00:00", "slot": "chat-1-100", "provider": "acp", "model": "auto", "input": 0, "output": 0, "cost": 0.0, "credits": 1.0, "context_used": 40000, "context_window": 200000, "stop_reason": "refusal"}` + "\n",
	})
	bs := load(t, &KiroCLI{Home: kiro, CrewHome: crew})
	if len(bs) != 1 {
		t.Fatalf("会話の数 = %d", len(bs))
	}
	f := bs[0].Finish(15)
	if f.Credits != 5 {
		t.Errorf("クレジット = %v, want 5（kiro-cli の記録のほうが多い）", f.Credits)
	}
	if n := nativeOf(f); n["コンテキストの最大使用率"].V != 20 || n["正常に終わらなかったターン"].V != 1 {
		t.Errorf("参考指標 = %+v", f.Native)
	}
}

// promptTexts は Builder の依頼の文。
func promptTexts(b *core.Builder) []string {
	var out []string
	if b != nil {
		for _, p := range b.Prompts {
			out = append(out, p.Text)
		}
	}
	return out
}

// role が user でも、meta.human: true のない行は人が書いたとは限らない（予定の実行・Issue Radar など）。
// 目印を書く Crew の記録では、目印のある行だけを依頼に数え、ほかは仕組みが入れたものとして残す。
func TestKiroCrewHumanMarker(t *testing.T) {
	kiro, crew := t.TempDir(), t.TempDir()
	writeFiles(t, kiro, map[string]string{"sessions/cli/.keep": ""})
	writeFiles(t, crew, map[string]string{
		// 目印ができる前の行（見分けられないので依頼に数える）のあとに、目印のある Crew の行が続く
		"sessions/dashboard_chat-1-100.jsonl": `{"_type": "metadata", "title": "混在"}
{"role": "user", "content": "古い依頼", "ts": "2026-09-29T09:00:00+00:00"}
{"role": "user", "content": "人が打った依頼", "ts": "2026-09-29T10:00:00+00:00", "meta": {"mid": "m1", "human": true}}
{"role": "assistant", "content": "はい", "ts": "2026-09-29T10:01:00+00:00"}
{"role": "user", "content": "# Cron Run: nightly\n\n夜の見張り", "ts": "2026-09-29T11:00:00+00:00", "meta": {"mid": "m2"}}
{"role": "user", "content": "Issue Radar wake", "ts": "2026-09-29T12:00:00+00:00", "meta": {"human": "true"}}
`,
		// 目印のない古い記録は、今までどおり user の行を全部依頼に数える
		"sessions/dashboard_chat-2-200.jsonl": `{"_type": "metadata", "title": "古い記録"}
{"role": "user", "content": "一つ目", "ts": "2026-09-29T09:00:00+00:00"}
{"role": "user", "content": "二つ目", "ts": "2026-09-29T09:10:00+00:00"}
`,
	})
	by := map[string]*core.Builder{}
	for _, b := range load(t, &KiroCLI{Home: kiro, CrewHome: crew}) {
		by[b.ID] = b
	}
	b := by["crew:dashboard_chat-1-100"]
	if got := strings.Join(promptTexts(b), "|"); got != "古い依頼|人が打った依頼" {
		t.Errorf("依頼 = %q, want 古い依頼|人が打った依頼（目印のない cron・Issue Radar の行は数えない）", got)
	}
	if b == nil || len(b.Notes) != 2 || b.Notes[0].Kind != "agent" || !strings.HasPrefix(b.Notes[0].Text, "# Cron Run: nightly") {
		t.Errorf("仕組みが入れた行 = %+v", b.Notes)
	}
	if got := strings.Join(promptTexts(by["crew:dashboard_chat-2-200"]), "|"); got != "一つ目|二つ目" {
		t.Errorf("目印のない記録の依頼 = %q", got)
	}
}

// fork した会話は、元の会話の行を元の ts のまま写している。写した行（created_at より前）は数えない。
func TestKiroCrewFork(t *testing.T) {
	kiro, crew := t.TempDir(), t.TempDir()
	writeFiles(t, kiro, map[string]string{"sessions/cli/.keep": ""})
	parent := `{"role": "user", "content": "親の依頼 1", "ts": "2026-09-29T09:00:00+00:00", "meta": {"mid": "a1"}}
{"role": "assistant", "content": "親の応答", "ts": "2026-09-29T09:01:00+00:00", "meta": {"mid": "a2"}}
{"role": "user", "content": "親の依頼 2", "ts": "2026-09-29T09:30:00+00:00"}
`
	writeFiles(t, crew, map[string]string{
		"sessions/dashboard_chat-1-100.jsonl": `{"_type": "metadata", "title": "親", "created_at": "2026-09-29T08:59:00+00:00"}
` + parent,
		// 使用量の記録のある fork（crewOnly → readCrewKey）
		"sessions/dashboard_chat-2-200.jsonl": `{"_type": "metadata", "title": "[fork] Fork of 親", "created_at": "2026-09-29T10:00:00.123456+00:00", "forked_from": "dashboard:chat-1-100"}
` + parent + `{"role": "user", "content": "fork での依頼", "ts": "2026-09-29T10:05:00+00:00", "meta": {"mid": "b1", "human": true}}
{"role": "assistant", "content": "fork での応答", "ts": "2026-09-29T10:06:00+00:00"}
`,
		"usage/tokens/2026-09-29.jsonl": `{"_type": "tokens", "ts": "2026-09-29T10:06:00+00:00", "slot": "chat-2-200", "provider": "acp", "model": "auto", "credits": 0.5}` + "\n",
		// 会話の記録だけの fork（写した行しかない）: 何もしていないので会話にしない
		"sessions/dashboard_chat-3-300.jsonl": `{"_type": "metadata", "created_at": "2026-09-29T11:00:00+00:00", "forked_from": "dashboard:chat-1-100"}
` + parent,
		// created_at のない fork は見分けられないので全部の行を使う
		"sessions/dashboard_chat-4-400.jsonl": `{"_type": "metadata", "forked_from": "dashboard:chat-1-100"}
` + parent,
	})
	by := map[string]*core.Builder{}
	for _, b := range load(t, &KiroCLI{Home: kiro, CrewHome: crew}) {
		by[b.ID] = b
	}
	if got := strings.Join(promptTexts(by["crew:dashboard_chat-1-100"]), "|"); got != "親の依頼 1|親の依頼 2" {
		t.Errorf("親の依頼 = %q", got)
	}
	f := by["crew:dashboard:chat-2-200"]
	if got := strings.Join(promptTexts(f), "|"); got != "fork での依頼" {
		t.Errorf("fork の依頼 = %q, want fork での依頼（親から写した行は数えない）", got)
	}
	if b := by["crew:dashboard_chat-3-300"]; b != nil {
		t.Errorf("写した行しかない fork は会話にしない: %v", promptTexts(b))
	}
	if got := strings.Join(promptTexts(by["crew:dashboard_chat-4-400"]), "|"); got != "親の依頼 1|親の依頼 2" {
		t.Errorf("created_at のない fork の依頼 = %q", got)
	}
}

// Crew は KIRO_HOME を見ず、いつも ~/.kiro/crew を使う。KIROCREW_HOME の先頭の ~ はホームにする。
func TestDefaultCrewHome(t *testing.T) {
	h := t.TempDir()
	t.Setenv("HOME", h)
	t.Setenv("USERPROFILE", h)
	t.Setenv("KIRO_HOME", filepath.Join(h, "elsewhere"))
	def := filepath.Join(h, ".kiro", "crew")
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"":                      def,
		"~":                     h,
		"~/crew":                filepath.Join(h, "crew"),
		"rel/crew":              filepath.Join(wd, "rel", "crew"),
		filepath.Join(h, "own"): filepath.Join(h, "own"),
	}
	if filepath.Separator == '/' {
		// Crew はルートやシステムの場所を無視して既定の場所を使う
		for _, p := range []string{"/", "/etc/crew", "/usr/local", "/private/etc/x"} {
			cases[p] = def
		}
	}
	for env, want := range cases {
		t.Setenv("KIROCREW_HOME", env)
		if got := DefaultCrewHome(); got != want {
			t.Errorf("KIROCREW_HOME=%q: %q, want %q", env, got, want)
		}
	}
}

// 記憶の整理は回ごとに slot（memory-consolidation:<場所>:<uuid>）が変わる。_bg と同じく、場所ごとに 1 日ごとにまとめる。
func TestKiroCrewMemoryConsolidation(t *testing.T) {
	defer func(l *time.Location) { time.Local = l }(time.Local)
	time.Local = time.UTC
	kiro, crew := t.TempDir(), t.TempDir()
	writeFiles(t, kiro, map[string]string{"sessions/cli/.keep": ""})
	row := func(ts, slot string) string {
		return fmt.Sprintf(`{"_type": "tokens", "ts": %q, "slot": %q, "provider": "acp", "model": "auto", "credits": 0.25}`, ts, slot)
	}
	writeFiles(t, crew, map[string]string{
		"usage/tokens/2026-09-29.jsonl": strings.Join([]string{
			row("2026-09-29T01:00:00+00:00", "memory-consolidation:alice:0123456789abcdef0123456789abcdef"),
			row("2026-09-29T02:00:00+00:00", "memory-consolidation:alice:fedcba9876543210fedcba9876543210"),
			row("2026-09-29T03:00:00+00:00", "memory-consolidation:bob:00000000000000000000000000000000"),
		}, "\n") + "\n",
		"usage/tokens/2026-09-30.jsonl": row("2026-09-30T01:00:00+00:00", "memory-consolidation:alice:11111111111111111111111111111111") + "\n",
	})
	bs := load(t, &KiroCLI{Home: kiro, CrewHome: crew})
	by := map[string]*core.Builder{}
	for _, b := range bs {
		by[b.ID] = b
	}
	if len(bs) != 3 {
		t.Fatalf("会話の数 = %d, want 3: %v", len(bs), by)
	}
	for id, want := range map[string]float64{
		"crew:memory-consolidation:alice:2026-09-29": 0.5,
		"crew:memory-consolidation:alice:2026-09-30": 0.25,
		"crew:memory-consolidation:bob:2026-09-29":   0.25,
	} {
		b := by[id]
		if b == nil {
			t.Errorf("%s がない", id)
			continue
		}
		if v := sumCredits(b.Credits); math.Abs(v-want) > 1e-9 {
			t.Errorf("%s のクレジット = %v, want %v", id, v, want)
		}
	}
	if b := by["crew:memory-consolidation:bob:2026-09-29"]; b != nil && b.Title != "Kiro Crew memory consolidation (bob)" {
		t.Errorf("タイトル = %q", b.Title)
	}
}

// サブエージェントの使用量の記録（slot は state.json の conversation_key か "subagent:<id>"）は、
// そのサブエージェントの kiro-cli の会話に結びつける。別の Crew のセッションにはしない。
func TestKiroCrewSubagentUsage(t *testing.T) {
	kiro, crew := t.TempDir(), t.TempDir()
	writeFiles(t, kiro, map[string]string{
		"sessions/cli/sa1.json":  `{"session_id": "sa1", "cwd": "/Users/me/app", "created_at": "2026-09-29T10:00:00Z", "session_state": {"conversation_metadata": {"user_turn_metadatas": [{"end_timestamp": "2026-09-29T10:01:00Z", "metering_usage": [{"value": 1, "unit": "credit"}]}]}}}`,
		"sessions/cli/sa1.jsonl": "",
	})
	row := func(ts, slot string) string {
		return fmt.Sprintf(`{"_type": "tokens", "ts": %q, "slot": %q, "provider": "acp", "model": "auto", "credits": 2.0, "surface": "subagent"}`, ts, slot)
	}
	writeFiles(t, crew, map[string]string{
		// 残す（keep）サブエージェント: kiro-cli の会話があり、使用量は conversation_key で記録される
		"subagents/keep1/state.json": `{"id": "keep1", "agent": "reviewer", "task": "PR を見る", "parent_session": "dashboard:chat-1-100", "session_id": "sa1", "keep": true, "conversation_key": "subagent:keep1"}`,
		// 残さないサブエージェント: Crew が kiro-cli の会話を消している。使用量は "subagent:<id>" で記録される
		"subagents/tmp2/state.json": `{"id": "tmp2", "agent": "tester", "task": "テストを流す", "parent_session": "dashboard:chat-1-100", "session_id": "gone", "keep": false, "conversation_key": "", "cwd": "/Users/me/lib"}`,
		"usage/tokens/2026-09-29.jsonl": strings.Join([]string{
			row("2026-09-29T10:00:30+00:00", "subagent:keep1"),
			row("2026-09-29T10:01:00+00:00", "subagent:keep1"),
			row("2026-09-29T11:00:00+00:00", "subagent:tmp2"),
		}, "\n") + "\n",
	})
	k := &KiroCLI{Home: kiro, CrewHome: crew}
	bs := load(t, k)
	by := map[string]*core.Builder{}
	for _, b := range bs {
		by[b.ID] = b
	}
	if len(bs) != 2 {
		t.Errorf("会話の数 = %d, want 2: %v", len(bs), by)
	}
	// (a) kiro-cli の会話に結びつける: Crew の記録 4 のほうが多いので置きかえる（1 回だけ）
	sa := by["sa1"]
	if sa == nil {
		t.Fatal("sa1 がない")
	}
	f := sa.Finish(15)
	if f.Credits != 4 || nativeOf(f)["ターン"].V != 2 || nativeOf(f)["うちサブエージェント"].V != 1 {
		t.Errorf("残すサブエージェント: credits = %v, native = %+v", f.Credits, f.Native)
	}
	if by["crew:subagent:keep1"] != nil {
		t.Error("kiro-cli の会話に結びついた使用量を、別の Crew のセッションにしない")
	}
	// (b) kiro-cli の会話がない: Crew のセッションにし、サブエージェントとして見せる
	tmp := by["crew:subagent:tmp2"]
	if tmp == nil {
		t.Fatal("crew:subagent:tmp2 がない")
	}
	if tmp.Title != "Subagent tester: テストを流す" || tmp.Project != "/Users/me/lib" {
		t.Errorf("残さないサブエージェント: title = %q, project = %q", tmp.Title, tmp.Project)
	}
	if n := nativeOf(tmp.Finish(15)); n["うちサブエージェント"].V != 1 || n["Crew から動かした会話"].V != 1 {
		t.Errorf("残さないサブエージェントの参考指標 = %+v", n)
	}
	if d := k.Detail(); !strings.Contains(d, "クレジットを補った会話 1 件") {
		t.Errorf("detail = %q", d)
	}
}

// サブエージェントは、親の会話（parent_session）が見つかれば、親のセッションのサブエージェントにまとめる。
// 親が kiro-cli の会話でも、Crew の記録だけにある会話でも同じ。クレジットと参考指標の合計は、まとめる前と変わらない。
func TestKiroCrewSubagentsFoldIntoParent(t *testing.T) {
	kiro, crew := t.TempDir(), t.TempDir()
	turn := func(end string, credits int) string {
		return fmt.Sprintf(`{"end_timestamp": %q, "metering_usage": [{"value": %d, "unit": "credit"}]}`, end, credits)
	}
	writeFiles(t, kiro, map[string]string{
		"sessions/cli/p1.json":  `{"session_id": "p1", "cwd": "/Users/me/app", "created_at": "2026-09-29T10:00:00Z", "session_state": {"conversation_metadata": {"user_turn_metadatas": [` + turn("2026-09-29T10:05:00Z", 1) + `]}}}`,
		"sessions/cli/p1.jsonl": "",
		"sessions/cli/sa1.json": `{"session_id": "sa1", "cwd": "/Users/me/app", "created_at": "2026-09-29T10:01:00Z", "session_state": {"conversation_metadata": {"user_turn_metadatas": [` + turn("2026-09-29T10:03:00Z", 2) + `]}}}`,
		"sessions/cli/sa1.jsonl": `{"version": "v1", "kind": "Prompt", "data": {"content": [{"kind": "text", "data": "PR を見て"}], "meta": {"timestamp": 1790676060}}}` + "\n" +
			`{"version": "v1", "kind": "AssistantMessage", "data": {"content": [{"kind": "toolUse", "data": {"name": "fs_write", "input": {"path": "/Users/me/app/a.go"}}}]}}` + "\n",
	})
	usage := func(ts, slot string, credits float64) string {
		return fmt.Sprintf(`{"_type": "tokens", "ts": %q, "slot": %q, "provider": "acp", "model": "auto", "credits": %v}`, ts, slot, credits)
	}
	writeFiles(t, crew, map[string]string{
		"session_map.json": `{"dashboard:chat-1-100": {"sid": "p1", "cwd": "/Users/me/app"}}`,
		// 親が kiro-cli の会話のサブエージェント（残すもの）と、kiro-cli の会話を Crew が消したもの
		"subagents/keep1/state.json": `{"id": "keep1", "agent": "reviewer", "task": "PR を見る", "parent_session": "dashboard:chat-1-100", "session_id": "sa1", "conversation_key": "subagent:keep1"}`,
		"subagents/tmp2/state.json":  `{"id": "tmp2", "agent": "tester", "task": "テストを流す", "parent_session": "dashboard:chat-1-100", "session_id": "gone"}`,
		// 親が Crew の記録だけにある会話（kiro-cli の会話がない）のサブエージェント
		"subagents/tmp3/state.json": `{"id": "tmp3", "agent": "writer", "task": "文を書く", "parent_session": "dashboard:chat-2-200", "session_id": "gone3"}`,
		"sessions/dashboard_chat-2-200.jsonl": `{"_type": "metadata", "title": "ドキュメント"}` + "\n" +
			`{"role": "user", "content": "書いて", "ts": "2026-09-29T12:00:00+00:00", "meta": {"human": true}}` + "\n",
		"usage/tokens/2026-09-29.jsonl": strings.Join([]string{
			usage("2026-09-29T11:00:00+00:00", "subagent:tmp2", 0.5),
			usage("2026-09-29T12:01:00+00:00", "subagent:tmp3", 0.25),
			usage("2026-09-29T12:00:30+00:00", "dashboard:chat-2-200", 1),
		}, "\n") + "\n",
	})
	k := &KiroCLI{Home: kiro, CrewHome: crew}
	bs := load(t, k)
	ids := []string{}
	total, subs := 0.0, 0.0
	for _, b := range bs {
		ids = append(ids, b.ID)
		f := b.Finish(15)
		total += f.Credits
		subs += nativeOf(f)["うちサブエージェント"].V
	}
	if len(bs) != 2 {
		t.Fatalf("会話 = %v, want 親の 2 件だけ", ids)
	}
	if math.Abs(total-4.75) > 1e-9 || subs != 3 {
		t.Errorf("クレジットの合計 = %v（want 4.75: p1 の 3.5 + Crew の記録だけの親の 1.25）, うちサブエージェント = %v（want 3）", total, subs)
	}
	p1 := find(bs, "p1")
	if p1 == nil || len(p1.Subagents) != 2 {
		t.Fatalf("p1 のサブエージェント = %+v", p1)
	}
	f := p1.Finish(15)
	if a := f.Subagents[0]; a.Type != "reviewer" || a.Desc != "PR を見る" || a.Start == nil || a.End == nil {
		t.Errorf("1 つ目（時刻の順）= %+v", a)
	}
	if a := f.Subagents[1]; a.Type != "tester" || a.Desc != "テストを流す" {
		t.Errorf("2 つ目 = %+v", a)
	}
	if f.NPrompts != 0 {
		t.Errorf("サブエージェントへの依頼は、人の依頼として数えない: %d", f.NPrompts)
	}
	if want := float64(time.Date(2026, 9, 29, 11, 0, 0, 0, time.UTC).Unix()); f.End != want { // tester は親の最後の記録より後に動いた
		t.Errorf("親の終わり = %v, want %v（サブエージェントが動いた時刻も親の作業に入れる）", f.End, want)
	}
	if f.Credits != 3.5 {
		t.Errorf("p1 のクレジット = %v, want 3.5（自分の 1 + reviewer の 2 + tester の 0.5）", f.Credits)
	}
	doc := find(bs, "crew:dashboard:chat-2-200")
	if doc == nil || len(doc.Subagents) != 1 || doc.Subagents[0].Type != "writer" {
		t.Fatalf("Crew の記録だけにある親 = %+v", doc)
	}
}

// kiroCLISession は kiro-cli の会話のメタデータ（sessions/cli/<id>.json）。turns は {終わりの時刻, クレジット（空なら記録なし）} の並び。
func kiroCLISession(id, cwd, created, updated string, turns ...[2]string) string {
	ts := make([]string, 0, len(turns))
	for _, x := range turns {
		m := ""
		if x[1] != "" {
			m = fmt.Sprintf(`, "metering_usage": [{"value": %s, "unit": "credit"}]`, x[1])
		}
		ts = append(ts, fmt.Sprintf(`{"end_timestamp": %q%s}`, x[0], m))
	}
	return fmt.Sprintf(`{"session_id": %q, "cwd": %q, "created_at": %q, "updated_at": %q, "session_state": {"conversation_metadata": {"user_turn_metadatas": [%s]}}}`,
		id, cwd, created, updated, strings.Join(ts, ", "))
}

// Crew は会話キーの sid を差しかえる（容量がいっぱいになった会話の作り直し・backend の切りかえ・seed）。使用量の記録は会話キーで残り、
// どの kiro-cli の会話のものかは書かれない。差しかえる前の行を今の会話に足すと、前の会話（session_map から消えて、自分のクレジットを持つ）と 2 度数える。
// 行は時刻で、その行のころに動いていた会話に結びつける。
func TestKiroCrewRebind(t *testing.T) {
	kiro, crew := t.TempDir(), t.TempDir()
	writeFiles(t, kiro, map[string]string{
		// (a) 作り直し: k-old（自分のクレジット 3）のあとを k-new（kiro-cli にクレジットが残っていない）が継いだ
		"sessions/cli/k-old.json":  kiroCLISession("k-old", "/Users/me/app", "2026-09-29T10:00:00Z", "2026-09-29T10:30:00Z", [2]string{"2026-09-29T10:10:00Z", "1.5"}, [2]string{"2026-09-29T10:30:00Z", "1.5"}),
		"sessions/cli/k-old.jsonl": "",
		"sessions/cli/k-new.json":  kiroCLISession("k-new", "/Users/me/app", "2026-09-29T11:00:00Z", "2026-09-29T11:10:00Z", [2]string{"2026-09-29T11:10:00Z", ""}),
		"sessions/cli/k-new.jsonl": "",
		// 同じころに別の場所で直接動かした kiro-cli の会話は、Crew の会話にしない
		"sessions/cli/k-user.json":  kiroCLISession("k-user", "/Users/me/other", "2026-09-29T10:00:00Z", "2026-09-29T10:40:00Z", [2]string{"2026-09-29T10:20:00Z", "7"}),
		"sessions/cli/k-user.jsonl": "",
		// (b) backend の切りかえ: kiro-cli の k2 から Claude Code に。Crew は k2 を discarded_sid に残す
		"sessions/cli/k2.json":  kiroCLISession("k2", "/Users/me/lib", "2026-09-29T12:00:00Z", "2026-09-29T12:10:00Z", [2]string{"2026-09-29T12:10:00Z", "1"}),
		"sessions/cli/k2.jsonl": "",
		// (c) 前の会話の kiro-cli の記録が消えている: その行は Crew のセッションにする（クレジットはなくさない）
		"sessions/cli/k3.json":  kiroCLISession("k3", "/Users/me/c3", "2026-09-29T15:00:00Z", "2026-09-29T15:10:00Z", [2]string{"2026-09-29T15:10:00Z", ""}),
		"sessions/cli/k3.jsonl": "",
	})
	row := func(ts, slot, provider string, credits float64, in, out int) string {
		return fmt.Sprintf(`{"_type": "tokens", "ts": %q, "slot": %q, "provider": %q, "model": "auto", "input": %d, "output": %d, "credits": %v}`, ts, slot, provider, in, out, credits)
	}
	writeFiles(t, crew, map[string]string{
		"session_map.json": `{"dashboard:chat-1-100": {"sid": "k-new", "cwd": "/Users/me/app"},
 "dashboard:chat-2-200": {"sid": "cc-sid", "provider": "claude_code", "discarded_sid": "k2", "cwd": "/Users/me/lib"},
 "dashboard:chat-3-300": {"sid": "k3", "cwd": "/Users/me/c3"}}`,
		"sessions/dashboard_chat-1-100.jsonl": `{"_type": "metadata", "title": "作り直した会話"}
{"role": "user", "content": "前の会話での依頼", "ts": "2026-09-29T10:05:00+00:00", "meta": {"human": true}}
{"role": "user", "content": "今の会話での依頼", "ts": "2026-09-29T11:05:00+00:00", "meta": {"human": true}}
`,
		"usage/tokens/2026-09-29.jsonl": strings.Join([]string{
			row("2026-09-29T10:10:01+00:00", "chat-1-100", "acp", 1.5, 0, 0),
			row("2026-09-29T10:30:01+00:00", "chat-1-100", "acp", 1.5, 0, 0),
			row("2026-09-29T11:10:01+00:00", "chat-1-100", "acp", 2, 0, 0),
			row("2026-09-29T12:10:01+00:00", "chat-2-200", "acp", 1, 0, 0),
			row("2026-09-29T13:00:00+00:00", "chat-2-200", "claude_code", 0, 100, 50),
			row("2026-09-29T14:00:00+00:00", "chat-3-300", "acp", 0.5, 0, 0),
			row("2026-09-29T15:10:01+00:00", "chat-3-300", "acp", 0.25, 0, 0),
		}, "\n") + "\n",
	})
	k := &KiroCLI{Home: kiro, CrewHome: crew}
	bs := load(t, k)
	by := map[string]*core.Session{}
	total := 0.0
	for _, b := range bs {
		f := b.Finish(15)
		by[b.ID] = f
		total += f.Credits
	}
	get := func(id string) *core.Session {
		t.Helper()
		f := by[id]
		if f == nil {
			t.Fatalf("%s がない: %v", id, by)
		}
		return f
	}
	// (a)
	if f := get("k-new"); f.Credits != 2 {
		t.Errorf("k-new のクレジット = %v, want 2（作り直す前の行は前の会話のもの）", f.Credits)
	}
	if f := get("k-old"); f.Credits != 3 || f.Source != "Kiro Crew" || f.Title != "作り直した会話" {
		t.Errorf("k-old: credits = %v, source = %q, title = %q, want 3・Kiro Crew・作り直した会話", f.Credits, f.Source, f.Title)
	}
	if got := strings.Join(promptTexts(find(bs, "k-old")), "|"); got != "前の会話での依頼" {
		t.Errorf("k-old の依頼 = %q", got)
	}
	if got := strings.Join(promptTexts(find(bs, "k-new")), "|"); got != "今の会話での依頼" {
		t.Errorf("k-new の依頼 = %q", got)
	}
	if f := get("k-user"); f.Credits != 7 || f.Source != "Kiro CLI" {
		t.Errorf("別の場所の kiro-cli の会話: credits = %v, source = %q", f.Credits, f.Source)
	}
	if by["crew:dashboard:chat-1-100"] != nil {
		t.Error("前の会話に結びついた行を、別の Crew のセッションにしない")
	}
	// (b)
	if f := get("k2"); f.Credits != 1 || f.Source != "Kiro Crew" {
		t.Errorf("切りかえ前の kiro-cli の会話: credits = %v, source = %q", f.Credits, f.Source)
	}
	if f := get("crew:dashboard:chat-2-200"); f.Credits != 0 || nativeOf(f)["ターン"].V != 1 {
		t.Errorf("切りかえ後の Crew のセッション: credits = %v, native = %+v（kiro-cli のころのクレジットは k2 で数える）", f.Credits, f.Native)
	}
	// (c)
	if f := get("k3"); f.Credits != 0.25 {
		t.Errorf("k3 のクレジット = %v, want 0.25", f.Credits)
	}
	if f := get("crew:dashboard:chat-3-300"); f.Credits != 0.5 {
		t.Errorf("前の会話が消えた行のクレジット = %v, want 0.5", f.Credits)
	}
	if math.Abs(total-13.75) > 1e-9 {
		t.Errorf("クレジットの合計 = %v, want 13.75（3 + 2 + 7 + 1 + 0.25 + 0.5）", total)
	}
}

// サブエージェントと裏方の処理の行の provider は、Claude Code 以外の backend でも "acp" か provider_label の値（"codex" など）。
// サブエージェントの本当の backend は state.json の "provider" にある。Codex で動いた行のトークンは Codex の履歴で数える。
func TestKiroCrewCodexRows(t *testing.T) {
	kiro, crew := t.TempDir(), t.TempDir()
	writeFiles(t, kiro, map[string]string{"sessions/cli/.keep": ""})
	row := func(ts, slot, provider string) string {
		return fmt.Sprintf(`{"_type": "tokens", "ts": %q, "slot": %q, "provider": %q, "model": "gpt-5", "input": 1000, "output": 100, "cost": 0.5, "credits": 0}`, ts, slot, provider)
	}
	writeFiles(t, crew, map[string]string{
		"subagents/cx1/state.json": `{"id": "cx1", "agent": "coder", "task": "直す", "parent_session": "dashboard:chat-9-900", "session_id": "codex-thread", "provider": "codex"}`,
		"subagents/gs1/state.json": `{"id": "gs1", "agent": "helper", "task": "調べる", "parent_session": "dashboard:chat-9-900", "session_id": "goose-sid", "provider": "goose"}`,
		"usage/tokens/2026-09-29.jsonl": strings.Join([]string{
			row("2026-09-29T10:00:00+00:00", "subagent:cx1", "acp"), // サブエージェントの行は claude_code か acp
			row("2026-09-29T10:05:00+00:00", "subagent:gs1", "acp"), // goose は kiroku が別に読まないので数える
			row("2026-09-29T11:00:00+00:00", "_bg", "codex"),        // 裏方の処理は provider_label（codex）
			row("2026-09-29T11:05:00+00:00", "_bg", "kas"),          // kas は kiroku が別に読まないので数える
		}, "\n") + "\n",
	})
	k := &KiroCLI{Home: kiro, CrewHome: crew}
	by := map[string]*core.Session{}
	for _, b := range load(t, k) {
		by[b.ID] = b.Finish(15)
	}
	if f := by["crew:subagent:cx1"]; f == nil || f.Usage.Total() != 0 || f.Cost != 0 {
		t.Errorf("Codex のサブエージェント = %+v（トークンとコストは Codex の履歴で数える）", f)
	}
	if f := by["crew:subagent:gs1"]; f == nil || f.Usage.In != 1000 || math.Abs(f.Cost-0.5) > 1e-9 {
		t.Errorf("goose のサブエージェント = %+v", f)
	}
	var bg *core.Session
	for id, f := range by {
		if strings.HasPrefix(id, "crew:_bg:") {
			bg = f
		}
	}
	if bg == nil || bg.Usage.In != 1000 || math.Abs(bg.Cost-0.5) > 1e-9 {
		t.Errorf("裏方の処理 = %+v, want kas の 1 行だけ（codex の行は Codex の履歴で数える）", bg)
	}
	if d := k.DetailEn(); !strings.Contains(d, "tokens and cost of 2 turns") {
		t.Errorf("detail = %q", d)
	}
}

// 会話の記録だけがある会話も、退避した古い行（sessions/archive/）を読む。
func TestKiroCrewLogOnlyArchive(t *testing.T) {
	kiro, crew := t.TempDir(), t.TempDir()
	writeFiles(t, kiro, map[string]string{"sessions/cli/.keep": ""})
	writeFiles(t, crew, map[string]string{
		"sessions/slack_C1_123.jsonl": `{"_type": "metadata", "title": "Slack の相談"}
{"role": "user", "content": "続きの依頼", "ts": "2026-09-30T13:00:00+00:00", "meta": {"human": true}}
`,
		"sessions/archive/slack_C1_123__20260930-120000.jsonl": `{"_type": "archive", "reason": "rotate"}
{"role": "user", "content": "最初の依頼", "ts": "2026-09-30T11:00:00+00:00", "meta": {"human": true}}
{"role": "assistant", "content": "はい", "ts": "2026-09-30T11:01:00+00:00"}
`,
	})
	bs := load(t, &KiroCLI{Home: kiro, CrewHome: crew})
	if got := strings.Join(promptTexts(find(bs, "crew:slack_C1_123")), "|"); got != "最初の依頼|続きの依頼" {
		t.Errorf("依頼 = %q, want 最初の依頼|続きの依頼（退避した行も読む）", got)
	}
}

// タスクの実行（taskrunner.py の _log_task）は "[Task: <spec>] Task N: <題>" を目印のない user の行として書く。人の依頼に数えない。
// 人が "[" で始めた文は、目印のない記録では依頼のまま。
func TestKiroCrewTaskRunnerRows(t *testing.T) {
	kiro, crew := t.TempDir(), t.TempDir()
	writeFiles(t, kiro, map[string]string{"sessions/cli/.keep": ""})
	writeFiles(t, crew, map[string]string{
		"sessions/taskrunner_run_t1.jsonl": `{"_type": "metadata", "title": "タスク"}
{"role": "user", "content": "[Task: tasks.md] Task 1: API を足す", "ts": "2026-09-29T09:00:00+00:00"}
{"role": "assistant", "content": "Task completed.", "ts": "2026-09-29T09:10:00+00:00"}
{"role": "user", "content": "[WIP] この関数を見て", "ts": "2026-09-29T09:20:00+00:00"}
`,
	})
	b := find(load(t, &KiroCLI{Home: kiro, CrewHome: crew}), "crew:taskrunner_run_t1")
	if got := strings.Join(promptTexts(b), "|"); got != "[WIP] この関数を見て" {
		t.Errorf("依頼 = %q, want [WIP] この関数を見て（タスクの実行の行は数えない）", got)
	}
	if b == nil || len(b.Notes) != 1 || b.Notes[0].Kind != "agent" || !strings.HasPrefix(b.Notes[0].Text, "[Task: tasks.md]") {
		t.Errorf("仕組みが入れた行 = %+v", b)
	}
}

// 会話キーのファイル名は Crew の _safe_key（Python の re.sub(r"[^\w\-.]", "_", key)。\w は Unicode の文字・数字と _）と同じにする。
// どんなキーでも sessions/ の外を指さず、glob の特別な文字も残さない。
func TestCrewSafeKey(t *testing.T) {
	for key, want := range map[string]string{
		"slack:123.456":        "slack_123.456",
		"dashboard:chat-1-100": "dashboard_chat-1-100",
		"telegram:日本語の会話":      "telegram_日本語の会話",
		"x:Ünïcödé_١٢٣":        "x_Ünïcödé_١٢٣",
		"a b\tc":               "a_b_c",
		"../../etc/passwd":     ".._.._etc_passwd",
		`a\b`:                  "a_b",
		"a／b":                  "a_b", // 全角の斜線も文字ではない
		"*?[]{}":               "______",
		"é":                   "e_", // 結合文字は Python の \w に入らない
	} {
		if got := safeKey(key); got != want {
			t.Errorf("safeKey(%q) = %q, want %q", key, got, want)
		}
	}
	home := t.TempDir()
	for _, key := range []string{"..", "../x", "a/../../b", "/abs", `..\..\x`, "*", "[a-z]", "", ".", "a\x00b"} {
		p := crewTranscriptPath(home, key)
		if filepath.Dir(p) != filepath.Join(home, "sessions") {
			t.Errorf("%q → %q は sessions/ の外", key, p)
		}
		if strings.ContainsAny(filepath.Base(p), `*?[\/`) {
			t.Errorf("%q → %q に glob の特別な文字が残る", key, p)
		}
	}
}

// 退避した行のファイル名が同じ秒に重なると、Crew は <名前>__<日時>-1.jsonl のように番号をつける。書いた順（番号なし・-1・-2…・-10）に読む。
// 名前が <キー>__ で始まる別の会話の退避ファイルは読まない。
func TestKiroCrewArchiveOrder(t *testing.T) {
	crew := t.TempDir()
	seg := func(text string) string {
		return `{"_type": "archive"}` + "\n" + `{"role": "user", "content": "` + text + `"}` + "\n"
	}
	writeFiles(t, crew, map[string]string{
		"sessions/archive/slack_C1__20260930-100000.jsonl":    seg("1"),
		"sessions/archive/slack_C1__20260930-100000-1.jsonl":  seg("2"),
		"sessions/archive/slack_C1__20260930-100000-2.jsonl":  seg("3"),
		"sessions/archive/slack_C1__20260930-100000-10.jsonl": seg("4"),
		"sessions/archive/slack_C1__20260930-100001.jsonl":    seg("5"),
		"sessions/archive/slack_C1__x__20260930-090000.jsonl": seg("別の会話"),
		"sessions/slack_C1.jsonl":                             `{"_type": "metadata"}` + "\n" + `{"role": "user", "content": "6"}` + "\n",
	})
	_, rows := readCrewKey(crew, "", "slack:C1", nil)
	var got []string
	for _, r := range rows {
		got = append(got, r.text)
	}
	if strings.Join(got, ",") != "1,2,3,4,5,6" {
		t.Errorf("読んだ順 = %v, want 1,2,3,4,5,6", got)
	}
}
