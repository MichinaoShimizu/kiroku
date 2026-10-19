package source

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/MichinaoShimizu/kiroku/internal/core"
)

// formatOf は、読んだ Builder の形の手がかりを 1 つにまとめる。
func formatOf(bs []*core.Builder) core.Format {
	var f core.Format
	for _, b := range bs {
		f.Merge(b.Format)
	}
	return f
}

// Claude Code: 知らない行の type と読めない行を数え、いちばん新しい version を覚える。わざと読まない type は数えない。
// 名前の形でない type（パスや発言）は other にまとめる。
func TestClaudeFormat(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "-Users-alice-secret-project")
	os.MkdirAll(dir, 0o755)
	lines := []string{
		`{"type":"user","timestamp":"2026-09-30T01:00:00Z","version":"2.1.9","cwd":"/Users/alice/secret-project","message":{"role":"user","content":"fix it"}}`,
		`{"type":"file-history-snapshot","messageId":"m1","snapshot":{}}`,
		`{"type":"queue-operation","operation":"enqueue","timestamp":"2026-09-30T01:00:01Z"}`,
		`{"type":"brand-new-kind","timestamp":"2026-09-30T01:00:02Z","version":"2.1.10"}`,
		`{"type":"brand-new-kind","timestamp":"2026-09-30T01:00:03Z"}`,
		`{"type":"/Users/alice/secret-project","timestamp":"2026-09-30T01:00:04Z"}`,
		`this line is not JSON`,
		`{"type":"assistant","timestamp":"2026-09-30T01:01:00Z","version":"2.1.2","message":{"id":"m1","model":"claude-sonnet-5-5","content":[{"type":"text","text":"ok"}]}}`,
	}
	os.WriteFile(filepath.Join(dir, "s1.jsonl"), []byte(strings.Join(lines, "\n")+"\n"), 0o644)
	f := formatOf(load(t, &Claude{Root: root}))
	if want := map[string]int{"brand-new-kind": 2, core.OtherKind: 1}; !reflect.DeepEqual(f.Unknown, want) {
		t.Errorf("Unknown = %v, want %v", f.Unknown, want)
	}
	if f.Bad != 1 || f.Version != "2.1.10" {
		t.Errorf("Bad = %d, Version = %q", f.Bad, f.Version)
	}
}

// Codex: 知らない行の type と、event_msg・response_item の知らない payload.type を数える。session_meta の cli_version を覚える。
// わざと読まないもの（reasoning・exec_command_end など）は数えない。
func TestCodexFormat(t *testing.T) {
	home := t.TempDir()
	writeCodex(t, home, "thr-fmt",
		`{"timestamp":"2026-10-06T00:00:00.000Z","type":"session_meta","payload":{"id":"thr-fmt","timestamp":"2026-10-06T00:00:00.000Z","cwd":"/Users/alice/secret-project","cli_version":"0.50.0-alpha.2"}}`,
		`{"timestamp":"2026-10-06T00:00:01.000Z","type":"event_msg","payload":{"type":"user_message","message":"my private prompt"}}`,
		`{"timestamp":"2026-10-06T00:00:02.000Z","type":"event_msg","payload":{"type":"exec_command_end","exit_code":0}}`,
		`{"timestamp":"2026-10-06T00:00:03.000Z","type":"event_msg","payload":{"type":"user_message_v2","message":"x"}}`,
		`{"timestamp":"2026-10-06T00:00:04.000Z","type":"response_item","payload":{"type":"reasoning","summary":[]}}`,
		`{"timestamp":"2026-10-06T00:00:05.000Z","type":"response_item","payload":{"type":"brand_new_call","name":"x"}}`,
		`{"timestamp":"2026-10-06T00:00:06.000Z","type":"rollout_extra","payload":{}}`,
		`{"timestamp":"2026-10-06T00:00:07.000Z","type":"event_msg","payload":{"type":"agent_message","message":"done"}}`,
		// 0.162 から Codex が残す種類は、知っていて読まないもの（知らない種類に数えない）
		`{"timestamp":"2026-10-06T00:00:08.000Z","type":"world_state","payload":{}}`,
		`{"timestamp":"2026-10-06T00:00:08.000Z","type":"retained_context","payload":{}}`,
		`{"timestamp":"2026-10-06T00:00:08.000Z","type":"security_risk_score","payload":{}}`,
		`{"timestamp":"2026-10-06T00:00:08.000Z","type":"inter_agent_communication","payload":{}}`,
		`{"timestamp":"2026-10-06T00:00:08.000Z","type":"inter_agent_communication_metadata","payload":{}}`,
		`{"timestamp":"2026-10-06T00:00:08.000Z","type":"realtime_item","payload":{}}`,
		`{"timestamp":"2026-10-06T00:00:08.000Z","type":"event_msg","payload":{"type":"thread_goal_updated"}}`,
		`{"timestamp":"2026-10-06T00:00:08.000Z","type":"event_msg","payload":{"type":"sub_agent_activity"}}`,
		`{"timestamp":"2026-10-06T00:00:08.000Z","type":"response_item","payload":{"type":"agent_message","author":"a","recipient":"b","content":[]}}`,
		`{"timestamp":"2026-10-06T00:00:08.000Z","type":"response_item","payload":{"type":"additional_tools"}}`,
		`{"timestamp":"2026-10-06T00:00:08.000Z","type":"response_item","payload":{"type":"configuration_update"}}`,
		`{"timestamp":"2026-10-06T00:00:08.000Z","type":"response_item","payload":{"type":"context_compaction"}}`,
		`{broken`,
	)
	bs := load(t, &Codex{Home: home})
	if len(bs) != 1 {
		t.Fatalf("sessions = %d", len(bs))
	}
	f := formatOf(bs)
	want := map[string]int{"event_msg:user_message_v2": 1, "response_item:brand_new_call": 1, "rollout_extra": 1}
	if !reflect.DeepEqual(f.Unknown, want) || f.Bad != 1 || f.Version != "0.50.0" {
		t.Errorf("Format = %+v, want unknown %v, 1 bad line, version 0.50.0", f, want)
	}
}

// 合成のテストデータ（testdata）のどの行も、知らない種類に数えない（ふだんの履歴で「Format check」が騒がないように）。
func TestFormatTestdataClean(t *testing.T) {
	for _, s := range []Source{&Claude{Root: "../../testdata/home/.claude/projects"}, &Codex{Home: "../../testdata/codex"}} {
		var bs []*core.Builder
		s.Load(func(b *core.Builder) { bs = append(bs, b) })
		if f := formatOf(bs); len(f.Unknown) > 0 || f.Bad > 0 {
			t.Errorf("%s: %+v", s.Name(), f)
		}
	}
}
