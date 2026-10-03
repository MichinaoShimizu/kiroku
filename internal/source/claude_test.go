package source

import (
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
