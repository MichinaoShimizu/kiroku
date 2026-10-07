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
	if len(bs) != 6 {
		t.Fatalf("会話の数 = %d, want 6（kiro-cli の 3 件 + Crew の記録だけにある 3 件）", len(bs))
	}
	crew := 0
	for _, b := range bs {
		if b.Source == "Kiro Crew" {
			crew++
		}
	}
	if crew != 6 || k.Detail() != "うち Kiro Crew から 3 件・Crew の使用量の記録でクレジットを補った会話 1 件・Crew の使用量の記録だけにある 3 件（2.00 クレジット）" {
		t.Fatalf("Crew の目印 = %d, detail = %q", crew, k.Detail())
	}
	if b := bs[1]; b.Title != "夜間のデプロイ見張り" {
		t.Errorf("Crew のタイトル = %q", b.Title)
	}
	if b := bs[0]; !strings.HasPrefix(b.Title, "Subagent reviewer: PR #12") {
		t.Errorf("サブエージェントのタイトル = %q", b.Title)
	}
	if b := bs[2]; b.Title != "CLIでデプロイ確認" { // 古い形（文字列だけ）の対応表。タイトルは kiro-cli のまま
		t.Errorf("古い対応表のタイトル = %q", b.Title)
	}
	if n := nativeOf(bs[0].Finish(15)); n["Crew から動かした会話"].V != 1 || n["うちサブエージェント"].V != 1 || n["ターン"].V == 0 {
		t.Errorf("Crew の参考指標 = %+v", n)
	}
	if len(bs[0].Credits) == 0 {
		t.Error("クレジットは kiro-cli の履歴のものを使う")
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
	// dashboard の会話: kiro-cli の記録 3.69 より Crew の記録 4.5 が多いので、Crew のほうを使う
	if v := credits("1cb4ad2f-90ba-4c5f-970b-5767003804b9"); v != 4.5 {
		t.Errorf("Crew の記録で補った会話のクレジット = %v, want 4.5", v)
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
			row("chat-4-400", "acp", "gpt-5", 300, 30, 0, 0, 0, ""),
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
