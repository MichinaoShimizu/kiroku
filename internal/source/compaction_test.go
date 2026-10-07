package source

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MichinaoShimizu/kiroku/internal/core"
)

// Claude Code: コンパクションは compact_boundary の行と、そのすぐあとの要約の行（isCompactSummary）の両方に残る。
// 同じ 1 回を 2 回数えない。compact_boundary のない古い版は要約の行で数え、サブエージェントの行は数えない。
func TestClaudeCompactions(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "-Users-me-app")
	os.MkdirAll(dir, 0o755)
	lines := []string{
		`{"type":"user","timestamp":"2026-09-30T01:00:00Z","cwd":"/Users/me/app","message":{"role":"user","content":"直して"}}`,
		`{"type":"assistant","timestamp":"2026-09-30T01:01:00Z","message":{"id":"m1","model":"claude-sonnet-5-5","content":[{"type":"text","text":"ok"}]}}`,
		// 自動のコンパクション
		`{"type":"system","subtype":"compact_boundary","timestamp":"2026-09-30T02:00:00Z","content":"Conversation compacted","compactMetadata":{"trigger":"auto","preTokens":180000}}`,
		`{"type":"user","timestamp":"2026-09-30T02:00:05Z","isCompactSummary":true,"message":{"role":"user","content":"This session is being continued from a previous conversation..."}}`,
		// /compact（手動）
		`{"type":"user","timestamp":"2026-09-30T03:00:00Z","message":{"role":"user","content":"<command-name>/compact</command-name>"}}`,
		`{"type":"system","subtype":"compact_boundary","timestamp":"2026-09-30T03:00:30Z","compactMetadata":{"trigger":"manual","preTokens":90000}}`,
		`{"type":"user","timestamp":"2026-09-30T03:00:31Z","isCompactSummary":true,"message":{"role":"user","content":"This session is being continued..."}}`,
		// compact_boundary のない古い版の要約
		`{"type":"user","timestamp":"2026-09-30T04:00:00Z","isCompactSummary":true,"message":{"role":"user","content":"This session is being continued..."}}`,
		// trigger のほかの値は使わない
		`{"type":"system","subtype":"compact_boundary","timestamp":"2026-09-30T05:00:00Z","compactMetadata":{"trigger":"<img src=x>"}}`,
		// サブエージェント（古い版は同じファイルに isSidechain つきで混ざる）
		`{"type":"system","subtype":"compact_boundary","timestamp":"2026-09-30T06:00:00Z","isSidechain":true,"compactMetadata":{"trigger":"auto"}}`,
		`{"type":"system","subtype":"turn_duration","timestamp":"2026-09-30T07:00:00Z","durationMs":1000}`,
	}
	os.WriteFile(filepath.Join(dir, "s1.jsonl"), []byte(strings.Join(lines, "\n")+"\n"), 0o644)
	bs := load(t, &Claude{Root: root})
	if len(bs) != 1 {
		t.Fatalf("セッション数 = %d, want 1", len(bs))
	}
	s := bs[0].Finish(15)
	at := func(v string) float64 { f, _ := core.ParseTS(v); return f }
	want := []float64{at("2026-09-30T02:00:00Z"), at("2026-09-30T03:00:30Z"), at("2026-09-30T04:00:00Z"), at("2026-09-30T05:00:00Z")}
	if len(s.Compactions) != len(want) {
		t.Fatalf("コンパクション = %v, want %v", s.Compactions, want)
	}
	for i := range want {
		if s.Compactions[i] != want[i] {
			t.Errorf("コンパクション[%d] = %v, want %v", i, s.Compactions[i], want[i])
		}
	}
	if got := strings.Join(s.CompactKinds, "|"); got != "auto|manual||" {
		t.Errorf("種類 = %q, want %q（auto・manual のほかは空）", got, "auto|manual||")
	}
	if s.NPrompts != 2 {
		t.Errorf("プロンプト = %d, want 2（要約の行は依頼ではない）", s.NPrompts)
	}
}

// Codex: コンパクションは compacted の行で 1 回と数える（同じ 1 回の context_compacted や item_completed の ContextCompaction は数えない）。
// フォークが写した親の compacted は数えず、サブエージェント自身のコンパクションは親のセッションに入れない。
func TestCodexCompactions(t *testing.T) {
	home := t.TempDir()
	writeCodex(t, home, "thr-cmp",
		`{"timestamp":"2026-10-06T00:00:00.000Z","type":"session_meta","payload":{"id":"thr-cmp","timestamp":"2026-10-06T00:00:00.000Z","cwd":"/Users/me/web"}}`,
		`{"timestamp":"2026-10-06T00:00:10.000Z","type":"event_msg","payload":{"type":"user_message","message":"直して"}}`,
		`{"timestamp":"2026-10-06T00:30:00.000Z","type":"compacted","payload":{"message":"","replacement_history":[],"window_number":1}}`,
		`{"timestamp":"2026-10-06T00:30:00.100Z","type":"event_msg","payload":{"type":"context_compacted"}}`,
		`{"timestamp":"2026-10-06T00:30:00.200Z","type":"event_msg","payload":{"type":"item_completed","item":{"type":"ContextCompaction","id":"c1"}}}`,
		`{"timestamp":"2026-10-06T01:00:00.000Z","type":"compacted","payload":{"message":"summary"}}`,
	)
	writeCodex(t, home, "thr-cmp-sub",
		`{"timestamp":"2026-10-06T00:40:00.000Z","type":"session_meta","payload":{"id":"thr-cmp-sub","timestamp":"2026-10-06T00:40:00.000Z","cwd":"/Users/me/web","source":{"subagent":{"thread_spawn":{"parent_thread_id":"thr-cmp","agent_role":"explorer"}}}}}`,
		`{"timestamp":"2026-10-06T00:45:00.000Z","type":"compacted","payload":{"message":""}}`,
	)
	// フォーク: 親の行（compacted を含む）を写し、自分の thread_settings_applied から後が自分の行
	writeCodex(t, home, "thr-fork",
		`{"timestamp":"2026-10-06T02:00:00.000Z","type":"session_meta","payload":{"id":"thr-fork","forked_from_id":"thr-cmp","timestamp":"2026-10-06T02:00:00.000Z","cwd":"/Users/me/web"}}`,
		`{"timestamp":"2026-10-06T02:00:00.000Z","type":"session_meta","payload":{"id":"thr-cmp","timestamp":"2026-10-06T00:00:00.000Z","cwd":"/Users/me/web"}}`,
		`{"timestamp":"2026-10-06T02:00:00.000Z","type":"compacted","payload":{"message":"","window_number":1}}`,
		`{"timestamp":"2026-10-06T02:00:00.000Z","type":"event_msg","payload":{"type":"thread_settings_applied","thread_id":"thr-fork"}}`,
		`{"timestamp":"2026-10-06T02:01:00.000Z","type":"event_msg","payload":{"type":"user_message","message":"続けて"}}`,
	)
	bs := load(t, &Codex{Home: home})
	got := map[string][]float64{}
	for _, b := range bs {
		got[b.ID] = b.Finish(15).Compactions
	}
	base := float64(1791244800) // 2026-10-06T00:00:00Z
	if c := got["thr-cmp"]; len(c) != 2 || c[0] != base+1800 || c[1] != base+3600 {
		t.Errorf("thr-cmp のコンパクション = %v, want [00:30 01:00]（サブエージェントの分は入れない）", c)
	}
	if c, ok := got["thr-fork"]; !ok || len(c) != 0 {
		t.Errorf("thr-fork のコンパクション = %v（ok=%v）, want なし（写した親の行は数えない）", c, ok)
	}
}

// Amazon Q / Kiro CLI（SQLite）: コンパクションは latest_summary（[要約, request_metadata]）の 1 回だけ。
// 前の要約は上書きされるので、残る最後の 1 回を数える。message_meta_tags に Compact がないもの、時刻のない古い形（文字列）は数えない。
func TestQStoreCompaction(t *testing.T) {
	t0 := time.Date(2026, 9, 29, 1, 0, 0, 0, time.UTC)
	history := []any{map[string]any{
		"user":      map[string]any{"timestamp": t0.Format(time.RFC3339), "content": map[string]any{"Prompt": map[string]any{"prompt": "直して"}}},
		"assistant": map[string]any{"Response": map[string]any{"message_id": "m1", "content": "ok"}},
	}}
	end := t0.Add(-time.Hour).UnixMilli()
	cases := []struct {
		name    string
		summary any
		want    int
	}{
		{"Compact", []any{"summary", map[string]any{"request_start_timestamp_ms": end - 5000, "stream_end_timestamp_ms": end, "message_meta_tags": []any{"Compact"}}}, 1},
		{"タグのない版", []any{"summary", map[string]any{"stream_end_timestamp_ms": end}}, 1},
		{"ほかのタグ", []any{"summary", map[string]any{"stream_end_timestamp_ms": end, "message_meta_tags": []any{"TangentMode"}}}, 0},
		{"古い形", "summary", 0},
		{"時刻なし", []any{"summary", map[string]any{"message_meta_tags": []any{"Compact"}}}, 0},
	}
	for _, c := range cases {
		conv := map[string]any{"conversation_id": "conv-x", "history": history, "latest_summary": c.summary}
		s := load(t, &QStore{Label: "Amazon Q", DB: qConvDB(t, conv)})[0].Finish(15)
		if len(s.Compactions) != c.want {
			t.Errorf("%s: コンパクション = %v, want %d 回", c.name, s.Compactions, c.want)
		} else if c.want == 1 && s.Compactions[0] != float64(end)/1000 {
			t.Errorf("%s: 時刻 = %v, want 要約を返し終わった時刻 %v", c.name, s.Compactions[0], float64(end)/1000)
		}
	}
}
