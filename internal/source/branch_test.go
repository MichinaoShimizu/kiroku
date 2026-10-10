package source

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MichinaoShimizu/kiroku/internal/archive"
	"github.com/MichinaoShimizu/kiroku/internal/core"
)

// 元の会話の行（sessionId は orig）。/branch や --fork-session は、これを新しい会話のファイルにそのまま写す。
var origLines = []string{
	`{"type":"user","sessionId":"orig","uuid":"u1","timestamp":"2026-09-30T01:00:00Z","cwd":"/Users/me/app","message":{"role":"user","content":"一覧を作って"}}`,
	`{"type":"assistant","sessionId":"orig","uuid":"u2","timestamp":"2026-09-30T01:01:00Z","requestId":"r1","message":{"id":"m1","model":"claude-opus-5-5","usage":{"input_tokens":1000,"output_tokens":100},"content":[{"type":"tool_use","id":"t1","name":"Bash","input":{"command":"ls"}}]}}`,
	`{"type":"user","sessionId":"orig","uuid":"u3","timestamp":"2026-09-30T01:01:10Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":"ok"}]}}`,
	`{"type":"system","subtype":"compact_boundary","sessionId":"orig","uuid":"u4","timestamp":"2026-09-30T01:10:00Z","compactMetadata":{"trigger":"manual","preTokens":1000}}`,
	`{"type":"assistant","sessionId":"orig","uuid":"u5","timestamp":"2026-09-30T01:20:00Z","isApiErrorMessage":true,"message":{"id":"e1","model":"<synthetic>","content":[{"type":"text","text":"You've hit your session limit · resets 3:45pm"}]}}`,
}

// 分けたあとに足した行（sessionId は branch）。
var branchLines = []string{
	`{"type":"user","sessionId":"branch","uuid":"b1","timestamp":"2026-09-30T02:00:00Z","cwd":"/Users/me/app","message":{"role":"user","content":"別のやり方で"}}`,
	`{"type":"assistant","sessionId":"branch","uuid":"b2","timestamp":"2026-09-30T02:01:00Z","requestId":"r2","message":{"id":"m2","model":"claude-opus-5-5","usage":{"input_tokens":2000,"output_tokens":200},"content":[{"type":"text","text":"はい"}]}}`,
}

func writeLines(t *testing.T, path string, lines []string) {
	t.Helper()
	os.MkdirAll(filepath.Dir(path), 0o755)
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// forkedFrom は、写した行の sessionId を新しい会話に書き換え、元の会話を forkedFrom に残す形。
func forkedFrom(lines []string) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = strings.Replace(l, `"sessionId":"orig"`, `"sessionId":"branch","forkedFrom":{"sessionId":"orig","messageUuid":"x"}`, 1)
	}
	return out
}

// /branch・--fork-session で写した行は、元の会話で数え、写した先では数えない（トークン・応答・依頼・ツール・
// コンパクション・利用上限を 2 度数えない）。元の会話がもうなければ（kiroku archive にもなければ）、写した先で数える。
func TestClaudeBranchCountsCopiesOnce(t *testing.T) {
	for _, tc := range []struct {
		name   string
		copied []string
	}{
		{"sessionId のまま", origLines},
		{"forkedFrom", forkedFrom(origLines)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, "-Users-me-app")
			writeLines(t, filepath.Join(dir, "orig.jsonl"), origLines)
			writeLines(t, filepath.Join(dir, "branch.jsonl"), append(append([]string{}, tc.copied...), branchLines...))
			c := &Claude{Root: root}
			by := map[string]int{}
			bs := load(t, c)
			for i, b := range bs {
				by[b.ID] = i
			}
			o, br := bs[by["orig"]].Finish(15), bs[by["branch"]].Finish(15)
			if o.Usage.In != 1000 || len(o.Prompts) != 1 || len(o.Compactions) != 1 || len(o.Limits) != 1 || len(o.Tools) != 1 {
				t.Errorf("元の会話は自分の行を全部数える: in=%v prompts=%d compactions=%d limits=%d tools=%v", o.Usage.In, len(o.Prompts), len(o.Compactions), len(o.Limits), o.Tools)
			}
			start, _ := core.ParseTS("2026-09-30T02:00:00Z")
			if br.Usage.In != 2000 || br.Usage.Out != 200 || len(br.Prompts) != 1 || br.Prompts[0].Text != "別のやり方で" ||
				len(br.Compactions) != 0 || len(br.Limits) != 0 || len(br.Tools) != 0 || br.Start != start || br.Project != "app" {
				t.Errorf("写した先は分けたあとの分だけ: in=%v out=%v prompts=%v compactions=%d limits=%d tools=%v start=%v project=%q",
					br.Usage.In, br.Usage.Out, br.Prompts, len(br.Compactions), len(br.Limits), br.Tools, br.Start, br.Project)
			}
			if math.Abs(o.Cost+br.Cost-(1000*4+100*20+2000*4+200*20)/1e6) > 1e-9 {
				t.Errorf("目安コストの合計 = %v, want 両方の応答を 1 回ずつ", o.Cost+br.Cost)
			}

			// 元の会話が消えたら、写した行も写した先で数える（黙って消えない）。kiroku serve でも読み直すよう、Tag が変わる
			before := c.Units()
			os.Remove(filepath.Join(dir, "orig.jsonl"))
			after := c.Units()
			if len(before) != 2 || len(after) != 1 || after[0].Tag == "" || after[0].Tag == before[0].Tag {
				t.Errorf("元の会話が消えたら Tag が変わる: before=%+v after=%+v", before, after)
			}
			bs = load(t, c)
			if len(bs) != 1 {
				t.Fatalf("セッション数 = %d, want 1", len(bs))
			}
			br = bs[0].Finish(15)
			if br.Usage.In != 3000 || len(br.Prompts) != 2 || len(br.Compactions) != 1 || len(br.Limits) != 1 || len(br.Tools) != 1 {
				t.Errorf("元がなければ写した行も数える: in=%v prompts=%d compactions=%d limits=%d tools=%v", br.Usage.In, len(br.Prompts), len(br.Compactions), len(br.Limits), br.Tools)
			}
		})
	}
}

// 元の会話が kiroku archive のコピーにだけ残っていれば、コピーで数えるので、写した先では数えない。
func TestClaudeBranchOriginInArchive(t *testing.T) {
	root, arch := t.TempDir(), t.TempDir()
	dir := filepath.Join(root, "-Users-me-app")
	writeLines(t, filepath.Join(dir, "orig.jsonl"), origLines)
	c := &Claude{Root: root, Archive: arch}
	for _, k := range c.Keep() {
		if _, err := archive.Sync(k.Src, k.Dst); err != nil {
			t.Fatal(err)
		}
	}
	os.Remove(filepath.Join(dir, "orig.jsonl"))
	writeLines(t, filepath.Join(dir, "branch.jsonl"), append(append([]string{}, origLines...), branchLines...))
	in := 0.0
	for _, b := range load(t, c) {
		in += b.Finish(15).Usage.In
	}
	if in != 3000 {
		t.Errorf("入力トークンの合計 = %v, want 3000（コピーの元の会話 1000 + 分けたあと 2000）", in)
	}
}

// 会話を分けた（/branch）プロセスの cost-state には、分ける前の分も入っている（元の会話の cost-state にもある）。
// 分けた時刻より前に起動したプロセスの記録は使わず、分けたあとの応答を料金表で見積もる。
func TestClaudeBranchSkipsInheritedCostState(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "-Users-me-app")
	start := `"startTime": 1790730000000` // 2026-09-30T00:20:00Z（分ける前に起動した）
	writeLines(t, filepath.Join(dir, "orig.jsonl"), append(append([]string{}, origLines...),
		`{"type":"cost-state",`+start+`,"modelUsage":{"claude-opus-5-5":{"costUSD":0.006}}}`))
	writeLines(t, filepath.Join(dir, "branch.jsonl"), append(append(append([]string{}, origLines...), branchLines...),
		`{"type":"cost-state",`+start+`,"modelUsage":{"claude-opus-5-5":{"costUSD":0.018}}}`))
	total := 0.0
	for _, b := range load(t, &Claude{Root: root}) {
		total += b.Finish(15).Cost
	}
	// 元の会話は記録の 0.006、分けたあとは料金表で 2000 × $4 + 200 × $20 = $0.012
	if math.Abs(total-0.018) > 1e-9 {
		t.Errorf("目安コストの合計 = %v, want 0.018（分ける前の分を 2 度数えない）", total)
	}
}

// 行の sessionId がファイルの名前と同じなら、ふつうの行。パスや glob の記号を含む sessionId は、元の会話として探さない。
func TestCopiedFrom(t *testing.T) {
	for _, tc := range []struct {
		line, want string
	}{
		{`{"sessionId":"s1"}`, ""},
		{`{}`, ""},
		{`{"sessionId":"orig"}`, "orig"},
		{`{"sessionId":"s1","forkedFrom":{"sessionId":"orig"}}`, "orig"},
		{`{"sessionId":"../../etc/passwd"}`, ""},
		{`{"sessionId":"*"}`, ""},
	} {
		var e core.Obj
		if err := json.Unmarshal([]byte(tc.line), &e); err != nil {
			t.Fatal(err)
		}
		if got := copiedFrom(e, "s1"); got != tc.want {
			t.Errorf("copiedFrom(%s) = %q, want %q", tc.line, got, tc.want)
		}
	}
}
