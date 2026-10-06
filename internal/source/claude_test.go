package source

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MichinaoShimizu/kiroku/internal/core"
)

// アウトプット（コミット・PR・変更した行）は、ツールの結果が成功したものだけを数える。
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
	if s := bs[0].Finish(15); len(s.PRs) != 1 || s.PRs[0] != "https://github.com/o/r/pull/1" || s.File == "" {
		t.Errorf("PR の URL・履歴ファイル = %v %q", s.PRs, s.File)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %v, want %v（失敗・結果のない呼び出しは数えない）", k, got[k], v)
		}
	}
}

// ありえない時刻の行（"timestamp": 100 や 0001 年）は時刻として使わない。セッションが 1970 年から始まらないように。
func TestClaudeIgnoresBogusTimestamps(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "-Users-me-app")
	os.MkdirAll(dir, 0o755)
	lines := []string{
		`{"type":"user","timestamp":100,"cwd":"/Users/me/app","message":{"role":"user","content":"x"}}`,
		`{"type":"user","timestamp":"2026-09-30T01:00:00Z","cwd":"/Users/me/app","message":{"role":"user","content":"直して"}}`,
		`{"type":"assistant","timestamp":"0001-01-01T00:00:00Z","message":{"id":"m0","model":"claude-sonnet-5-5","content":[{"type":"text","text":"?"}]}}`,
		`{"type":"assistant","timestamp":"2026-09-30T01:05:00Z","message":{"id":"m1","model":"claude-sonnet-5-5","content":[{"type":"text","text":"ok"}]}}`,
	}
	os.WriteFile(filepath.Join(dir, "s1.jsonl"), []byte(strings.Join(lines, "\n")+"\n"), 0o644)
	bs := load(t, &Claude{Root: root})
	if len(bs) != 1 {
		t.Fatalf("セッション数 = %d, want 1", len(bs))
	}
	start, _ := core.ParseTS("2026-09-30T01:00:00Z")
	end, _ := core.ParseTS("2026-09-30T01:05:00Z")
	if s := bs[0].Finish(15); s == nil || s.Start != start || s.End != end {
		t.Errorf("開始・終了 = %+v, want 2026-09-30 01:00〜01:05", s)
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

// サブスクリプションで使っていると、Claude Code は cost-state の costUSD に 0 を書く（API の請求がないため）。
// それに合わせてしまうと、トークンを何十万使っても目安コストが $0.00 になるので、kiroku の料金表の見積もりを使う。
func TestClaudeCostStateZeroOnSubscription(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "-Users-me-app")
	os.MkdirAll(dir, 0o755)
	start := `"startTime": 1790730000000` // 2026-09-30T00:20:00Z
	lines := []string{
		`{"type":"user","timestamp":"2026-09-30T01:00:00Z","cwd":"/Users/me/app","message":{"role":"user","content":"直して"}}`,
		`{"type":"assistant","timestamp":"2026-09-30T01:01:00Z","message":{"id":"m1","model":"claude-opus-5-5","usage":{"input_tokens":100000,"output_tokens":5000},"content":[{"type":"text","text":"ok"}]}}`,
		`{"type":"cost-state",` + start + `,"totalCostUSD":0,"modelUsage":{"claude-opus-5-5":{"costUSD":0,"inputTokens":100000,"outputTokens":5000},"claude-haiku-4-5-20251001":{"costUSD":0,"inputTokens":1000}}}`,
	}
	os.WriteFile(filepath.Join(dir, "s1.jsonl"), []byte(strings.Join(lines, "\n")+"\n"), 0o644)
	bs := load(t, &Claude{Root: root})
	if len(bs) != 1 {
		t.Fatalf("セッション数 = %d, want 1", len(bs))
	}
	s := bs[0].Finish(15)
	// opus-5-5: 入力 100,000 × $4 + 出力 5,000 × $20 = $0.5、履歴に応答のない haiku-4-5 の 1,000 入力 = $0.001
	if math.Abs(s.Cost-0.501) > 1e-9 {
		t.Errorf("目安コスト = %v, want 料金表の見積もり 0.501", s.Cost)
	}
	if s.CostReported {
		t.Error("Claude Code の記録は使っていないのに costReported = true")
	}
}

// 利用上限のエラー（Claude Code が作る発言）を時刻つきで拾う。続けて出たものは 1 回、ふつうの発言は数えない。
func TestClaudeLimits(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "-Users-me-app")
	os.MkdirAll(dir, 0o755)
	lines := []string{
		`{"type":"user","timestamp":"2026-09-30T01:00:00Z","cwd":"/Users/me/app","message":{"role":"user","content":"rate limit の処理を直して"}}`,
		`{"type":"assistant","timestamp":"2026-09-30T01:01:00Z","message":{"id":"m1","model":"claude-sonnet-5-5","content":[{"type":"text","text":"rate limit の処理を直しました"}]}}`,
		`{"type":"assistant","timestamp":"2026-09-30T01:02:00Z","isApiErrorMessage":true,"message":{"id":"e1","model":"<synthetic>","content":[{"type":"text","text":"Claude AI usage limit reached|1790000000"}]}}`,
		`{"type":"assistant","timestamp":"2026-09-30T01:02:20Z","isApiErrorMessage":true,"message":{"id":"e2","model":"<synthetic>","content":[{"type":"text","text":"5-hour limit reached ∙ resets 3pm"}]}}`,
		`{"type":"user","timestamp":"2026-09-30T06:00:00Z","message":{"role":"user","content":"続けて"}}`,
		`{"type":"assistant","timestamp":"2026-09-30T06:01:00Z","isApiErrorMessage":true,"message":{"id":"e3","model":"<synthetic>","content":[{"type":"text","text":"API Error: 429 {\"type\":\"error\",\"error\":{\"type\":\"rate_limit_error\"}}"}]}}`,
		`{"type":"assistant","timestamp":"2026-09-30T06:02:00Z","isApiErrorMessage":true,"message":{"id":"e4","model":"<synthetic>","content":[{"type":"text","text":"API Error: Connection error."}]}}`,
	}
	os.WriteFile(filepath.Join(dir, "s1.jsonl"), []byte(strings.Join(lines, "\n")+"\n"), 0o644)
	bs := load(t, &Claude{Root: root})
	if len(bs) != 1 {
		t.Fatalf("セッション数 = %d, want 1", len(bs))
	}
	if got := bs[0].Finish(15).Limits; len(got) != 2 {
		t.Errorf("利用上限 = %v, want 2 回（01:02 と 06:01）", got)
	}
}

// 保存期間：settings.json に cleanupPeriodDays がなければ既定の 30 日（未設定）、あればその値。
func TestClaudeRetention(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, "projects")
	os.MkdirAll(root, 0o755)
	c := &Claude{Root: root}
	if r := c.Retention(); r.Days != 30 || r.Set || r.Setting != "cleanupPeriodDays" || r.Docs == "" {
		t.Errorf("未設定 = %+v, want 30 日・未設定", r)
	}
	os.WriteFile(filepath.Join(home, "settings.json"), []byte(`{"cleanupPeriodDays": 3650}`), 0o644)
	if r := c.Retention(); r.Days != 3650 || !r.Set {
		t.Errorf("設定あり = %+v, want 3650 日・設定済み", r)
	}
}

// 依頼の流れ：作業中に送った依頼（queued_command の添付だけに残る）を拾い、
// 自動要約の「This session is being continued…」や、ほかのエージェント・通知からの割り込みは依頼に数えない。
func TestClaudePromptFlow(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "-Users-me-app")
	os.MkdirAll(dir, 0o755)
	lines := []string{
		`{"type":"user","timestamp":"2026-09-30T01:00:00Z","cwd":"/Users/me/app","message":{"role":"user","content":"一覧を作って"}}`,
		`{"type":"assistant","timestamp":"2026-09-30T01:01:00Z","message":{"id":"m1","model":"claude-sonnet-5-5","content":[{"type":"text","text":"作ります"}]}}`,
		`{"type":"queue-operation","operation":"enqueue","timestamp":"2026-09-30T01:01:30Z","content":"ブランチ単位でも"}`,
		`{"type":"attachment","timestamp":"2026-09-30T01:01:30Z","attachment":{"type":"queued_command","prompt":"ブランチ単位でも","commandMode":"prompt","origin":{"kind":"human"},"humanTurn":true}}`,
		`{"type":"attachment","timestamp":"2026-09-30T01:01:40Z","attachment":{"type":"queued_command","prompt":"別のエージェントからの連絡","commandMode":"prompt","origin":{"kind":"peer"}}}`,
		`{"type":"attachment","timestamp":"2026-09-30T01:01:50Z","attachment":{"type":"queued_command","prompt":"タスクが終わりました","commandMode":"task-notification"}}`,
		`{"type":"user","timestamp":"2026-09-30T02:00:00Z","isCompactSummary":true,"isVisibleInTranscriptOnly":true,"message":{"role":"user","content":"This session is being continued from a previous conversation that ran out of context. The user said that was wrong."}}`,
		`{"type":"user","timestamp":"2026-09-30T02:01:00Z","message":{"role":"user","content":"続けて"}}`,
	}
	os.WriteFile(filepath.Join(dir, "s1.jsonl"), []byte(strings.Join(lines, "\n")+"\n"), 0o644)
	bs := load(t, &Claude{Root: root})
	if len(bs) != 1 {
		t.Fatalf("セッション数 = %d, want 1", len(bs))
	}
	var got []string
	for _, p := range bs[0].Prompts {
		got = append(got, p.Text)
	}
	if want := []string{"一覧を作って", "ブランチ単位でも", "続けて"}; strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("依頼 = %q, want %q", got, want)
	}
	if n := len(bs[0].FixTS); n != 0 {
		t.Errorf("言い直し = %d, want 0（要約の文面を言い直しと数えない）", n)
	}
}

// サブエージェント（subagents/*.jsonl）が成功させた編集・コミット・PR も、そのセッションのアウトプットに入る。
// アウトプットの時刻は結果の時刻（許可の確認待ちなどで、呼び出しから遅れて実行されるため）。
func TestClaudeSubagentOutputsAndResultTime(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "-Users-me-app")
	os.MkdirAll(filepath.Join(dir, "s1", "subagents"), 0o755)
	main := []string{
		`{"type":"user","timestamp":"2026-09-30T01:00:00Z","cwd":"/Users/me/app","message":{"role":"user","content":"サブエージェントに任せて"}}`,
		`{"type":"assistant","timestamp":"2026-09-30T01:01:00Z","message":{"id":"m1","model":"claude-sonnet-5-5","content":[{"type":"tool_use","id":"t1","name":"Agent","input":{"subagent_type":"general-purpose","description":"実装"}}]}}`,
		`{"type":"user","timestamp":"2026-09-30T01:20:00Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":"done"}]},"toolUseResult":{"agentId":"ag1"}}`,
		`{"type":"assistant","timestamp":"2026-09-30T01:21:00Z","message":{"id":"m2","model":"claude-sonnet-5-5","content":[{"type":"tool_use","id":"t2","name":"Bash","input":{"command":"git commit -m main"}}]}}`,
		`{"type":"user","timestamp":"2026-09-30T01:27:00Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t2","content":"[main abc] main"}]}}`,
	}
	sub := []string{
		`{"type":"assistant","timestamp":"2026-09-30T01:02:00Z","message":{"id":"s1","model":"claude-sonnet-5-5","content":[{"type":"tool_use","id":"u1","name":"Write","input":{"file_path":"/Users/me/app/b.go","content":"a\nb\nc\n"}}]}}`,
		`{"type":"user","timestamp":"2026-09-30T01:02:05Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"u1","content":"ok"}]}}`,
		`{"type":"assistant","timestamp":"2026-09-30T01:03:00Z","message":{"id":"s2","model":"claude-sonnet-5-5","content":[{"type":"tool_use","id":"u2","name":"Bash","input":{"command":"git commit -am sub && gh pr create --fill"}}]}}`,
		`{"type":"user","timestamp":"2026-09-30T01:03:30Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"u2","content":"https://github.com/o/r/pull/2"}]}}`,
		`{"type":"assistant","timestamp":"2026-09-30T01:04:00Z","message":{"id":"s3","model":"claude-sonnet-5-5","content":[{"type":"tool_use","id":"u3","name":"Bash","input":{"command":"git commit -m fails"}}]}}`,
		`{"type":"user","timestamp":"2026-09-30T01:04:10Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"u3","is_error":true,"content":"hook failed"}]}}`,
	}
	os.WriteFile(filepath.Join(dir, "s1.jsonl"), []byte(strings.Join(main, "\n")+"\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "s1", "subagents", "agent-ag1.jsonl"), []byte(strings.Join(sub, "\n")+"\n"), 0o644)
	bs := load(t, &Claude{Root: root})
	if len(bs) != 1 {
		t.Fatalf("セッション数 = %d, want 1", len(bs))
	}
	got := map[string]float64{}
	var mainCommit float64
	for _, o := range bs[0].Outputs {
		got[o.Kind] += o.V
		if o.Kind == "commit" && o.T != nil && *o.T > mainCommit {
			mainCommit = *o.T
		}
	}
	want := map[string]float64{"commit": 2, "pr": 1, "added": 3}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %v, want %v（サブエージェントの成功した分も数え、失敗は数えない）", k, got[k], v)
		}
	}
	if r, _ := core.ParseTS("2026-09-30T01:27:00Z"); mainCommit != r {
		t.Errorf("コミットの時刻 = %v, want %v（結果の時刻）", mainCommit, r)
	}
	if s := bs[0].Finish(15); len(s.PRs) != 1 || s.PRs[0] != "https://github.com/o/r/pull/2" {
		t.Errorf("サブエージェントの PR の URL = %v", s.PRs)
	}
}
