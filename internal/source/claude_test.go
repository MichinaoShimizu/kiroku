package source

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 成果の印（コミット・PR・変更した行）は、ツールの結果が成功したものだけを数える。
func TestClaudeOutputs(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "-Users-me-app")
	os.MkdirAll(dir, 0o755)
	lines := []string{
		`{"type":"user","timestamp":"2026-09-30T01:00:00Z","cwd":"/Users/me/app","message":{"role":"user","content":"直してコミットして"}}`,
		`{"type":"assistant","timestamp":"2026-09-30T01:01:00Z","message":{"id":"m1","model":"claude-sonnet-5-5","content":[{"type":"tool_use","id":"t1","name":"Edit","input":{"file_path":"/Users/me/app/a.go","old_string":"a\nb","new_string":"a\nc\nd"}}]}}`,
		`{"type":"user","timestamp":"2026-09-30T01:01:10Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":"ok"}]}}`,
		`{"type":"assistant","timestamp":"2026-09-30T01:02:00Z","message":{"id":"m2","model":"claude-sonnet-5-5","content":[{"type":"tool_use","id":"t2","name":"Bash","input":{"command":"git commit -m x"}}]}}`,
		`{"type":"user","timestamp":"2026-09-30T01:02:10Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t2","is_error":true,"content":"nothing to commit"}]}}`,
		`{"type":"assistant","timestamp":"2026-09-30T01:03:00Z","message":{"id":"m3","model":"claude-sonnet-5-5","content":[{"type":"tool_use","id":"t3","name":"Bash","input":{"command":"git commit -am x && gh pr create --fill"}}]}}`,
		`{"type":"user","timestamp":"2026-09-30T01:03:10Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t3","content":"https://github.com/o/r/pull/1"}]}}`,
		`{"type":"assistant","timestamp":"2026-09-30T01:04:00Z","message":{"id":"m4","model":"claude-sonnet-5-5","content":[{"type":"tool_use","id":"t4","name":"Bash","input":{"command":"git commit -m never-answered"}}]}}`,
	}
	os.WriteFile(filepath.Join(dir, "s1.jsonl"), []byte(strings.Join(lines, "\n")+"\n"), 0o644)
	bs := load(t, &Claude{Root: root})
	if len(bs) != 1 {
		t.Fatalf("セッション数 = %d, want 1", len(bs))
	}
	got := map[string]float64{}
	for _, o := range bs[0].Outputs {
		got[o.Kind] += o.V
	}
	want := map[string]float64{"commit": 1, "pr": 1, "added": 2, "removed": 1}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %v, want %v（失敗・結果のない呼び出しは数えない）", k, got[k], v)
		}
	}
}

// Claude Code 自身の使用料（cost-state）: プロセスごとの最新の累計を、目安コストに使う。
func TestClaudeCostState(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "-Users-me-app")
	os.MkdirAll(dir, 0o755)
	start := `"startTime": 1790730000000` // 2026-09-30T00:20:00Z
	lines := []string{
		`{"type":"user","timestamp":"2026-09-30T01:00:00Z","cwd":"/Users/me/app","message":{"role":"user","content":"直して"}}`,
		`{"type":"assistant","timestamp":"2026-09-30T01:01:00Z","message":{"id":"m1","model":"claude-opus-5-5","usage":{"input_tokens":1000,"output_tokens":1000},"content":[{"type":"text","text":"ok"}]}}`,
		`{"type":"cost-state",` + start + `,"totalCostUSD":0.01,"modelUsage":{"claude-opus-5-5":{"costUSD":0.01}}}`,
		`{"type":"assistant","timestamp":"2026-09-30T01:02:00Z","message":{"id":"m2","model":"claude-opus-5-5","usage":{"input_tokens":1000,"output_tokens":1000},"content":[{"type":"text","text":"ok"}]}}`,
		`{"type":"cost-state",` + start + `,"totalCostUSD":0.5,"modelUsage":{"claude-opus-5-5":{"costUSD":0.4},"claude-haiku-4-5-20251001":{"costUSD":0.1,"inputTokens":500}}}`,
	}
	os.WriteFile(filepath.Join(dir, "s1.jsonl"), []byte(strings.Join(lines, "\n")+"\n"), 0o644)
	bs := load(t, &Claude{Root: root})
	if len(bs) != 1 || len(bs[0].Reported) != 1 {
		t.Fatalf("cost-state はプロセスごとに最新の 1 件: %+v", bs[0].Reported)
	}
	s := bs[0].Finish(15)
	if math.Abs(s.Cost-0.5) > 1e-9 || !s.CostReported {
		t.Errorf("目安コスト = %v（reported=%v）, want Claude Code の記録 0.5", s.Cost, s.CostReported)
	}
}
