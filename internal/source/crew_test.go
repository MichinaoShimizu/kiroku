package source

import (
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
