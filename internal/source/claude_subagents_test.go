package source

import (
	"fmt"
	"math/rand/v2"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
)

// subLine は、サブエージェントのファイルの 1 行（応答 1 つ。入力トークンは in）。名前ごとに別の応答にする。
func subLine(name string, in int) string {
	return fmt.Sprintf(`{"type":"assistant","uuid":"s-%s","timestamp":"2026-09-30T01:02:00Z","requestId":"r-%s","message":{"id":"m-%s","model":"claude-haiku-5-5","usage":{"input_tokens":%d,"output_tokens":10},"content":[{"type":"text","text":"done"}]}}`, name, name, name, in)
}

// unitFiles は、Units の会話のファイル（キーは root か arch からの相対パス）ごとの、読むファイルの名前（会話のファイルの次から）。
func unitFiles(t *testing.T, c *Claude) (map[string][]string, map[string]string) {
	t.Helper()
	rel := func(p string) string {
		for _, base := range []string{c.Root, c.Archive} {
			if base == "" {
				continue
			}
			if r, err := filepath.Rel(base, p); err == nil && !strings.HasPrefix(r, "..") {
				return filepath.ToSlash(r)
			}
		}
		t.Fatalf("root にも arch にもないファイル: %s", p)
		return ""
	}
	files, tags := map[string][]string{}, map[string]string{}
	for _, u := range c.Units() {
		k := rel(u.Key)
		if u.Files[0] != u.Key {
			t.Errorf("%s: 最初のファイルが会話のファイルでない: %v", k, u.Files)
		}
		var subs []string
		for _, f := range u.Files[1:] {
			subs = append(subs, filepath.Base(f))
		}
		slices.Sort(subs)
		files[k], tags[k] = subs, u.Tag
	}
	return files, tags
}

// 会話を別のプロジェクトのフォルダへ移したあと（/cd や worktree）、同じ会話のファイルが 2 つのフォルダにある。
// 先に数えるファイル（before）のフォルダに同じ名前のサブエージェントがあれば、あとのファイルからは除く（dropSeenSubagents）。
// 名前の違うサブエージェント（kiroku archive のコピーにだけ残った古いものを含む）は残す。
func TestClaudeMovedSessionDropsSeenSubagents(t *testing.T) {
	t.Run("元のファイルが先", func(t *testing.T) {
		root, arch := t.TempDir(), t.TempDir()
		// 移った先（-Users-me-app-wt）。パスの順で、こちらが先に数えるファイル
		writeLines(t, filepath.Join(root, "-Users-me-app-wt", "orig.jsonl"), origLines)
		writeLines(t, filepath.Join(root, "-Users-me-app-wt", "orig", "subagents", "agent-a.jsonl"), []string{subLine("a", 300)})
		writeLines(t, filepath.Join(root, "-Users-me-app-wt", "orig", "subagents", "agent-b.jsonl"), []string{subLine("b", 50)})
		// 移る前（-Users-me-app）。agent-a は移った先と同じもの。agent-c は元が消え、kiroku archive のコピーにだけある
		writeLines(t, filepath.Join(root, "-Users-me-app", "orig.jsonl"), origLines[:3])
		writeLines(t, filepath.Join(root, "-Users-me-app", "orig", "subagents", "agent-a.jsonl"), []string{subLine("a", 300)})
		zstFile(t, filepath.Join(arch, "-Users-me-app", "orig", "subagents", "agent-c.jsonl.zst"), []string{subLine("c", 7)}, false)

		files, tags := unitFiles(t, &Claude{Root: root, Archive: arch})
		want := map[string][]string{
			"-Users-me-app-wt/orig.jsonl": {"agent-a.jsonl", "agent-b.jsonl"},
			"-Users-me-app/orig.jsonl":    {"agent-c.jsonl.zst"}, // agent-a は先のファイルで数える
		}
		for k, w := range want {
			if !slices.Equal(files[k], w) {
				t.Errorf("%s のサブエージェント = %v, want %v", k, files[k], w)
			}
		}
		if len(files) != len(want) {
			t.Errorf("Units = %v, want %d 個", files, len(want))
		}
		if !strings.Contains(tags["-Users-me-app/orig.jsonl"], " before:") || strings.Contains(tags["-Users-me-app-wt/orig.jsonl"], " before:") {
			t.Errorf("Tag = %q / %q, want あとのファイルにだけ before:", tags["-Users-me-app/orig.jsonl"], tags["-Users-me-app-wt/orig.jsonl"])
		}
	})

	t.Run("kiroku archive のコピー（.jsonl.zst）が先", func(t *testing.T) {
		// 元の会話はどちらのフォルダからも消え、kiroku archive のコピーだけがある。先に数えるのもコピーなので、
		// そのフォルダのサブエージェントは .jsonl.zst の名前で探す
		root, arch := t.TempDir(), t.TempDir()
		zstFile(t, filepath.Join(arch, "-Users-me-app-wt", "orig.jsonl.zst"), origLines, false)
		zstFile(t, filepath.Join(arch, "-Users-me-app-wt", "orig", "subagents", "agent-a.jsonl.zst"), []string{subLine("a", 300)}, false)
		zstFile(t, filepath.Join(arch, "-Users-me-app-wt", "orig", "subagents", "agent-b.jsonl.zst"), []string{subLine("b", 50)}, false)
		// 移る前のコピーには、移った先にない依頼も 1 つある（そのファイルも会話として出て、agent-c はそこに付く）
		zstFile(t, filepath.Join(arch, "-Users-me-app", "orig.jsonl.zst"), append(slices.Clip(origLines[:3]), userLine("orig", "only-old", "古い依頼")), false)
		zstFile(t, filepath.Join(arch, "-Users-me-app", "orig", "subagents", "agent-a.jsonl.zst"), []string{subLine("a", 300)}, false)
		zstFile(t, filepath.Join(arch, "-Users-me-app", "orig", "subagents", "agent-c.jsonl.zst"), []string{subLine("c", 7)}, false)

		c := &Claude{Root: root, Archive: arch}
		files, tags := unitFiles(t, c)
		want := map[string][]string{
			"-Users-me-app-wt/orig.jsonl.zst": {"agent-a.jsonl.zst", "agent-b.jsonl.zst"},
			"-Users-me-app/orig.jsonl.zst":    {"agent-c.jsonl.zst"},
		}
		for k, w := range want {
			if !slices.Equal(files[k], w) {
				t.Errorf("%s のサブエージェント = %v, want %v", k, files[k], w)
			}
		}
		if len(files) != len(want) {
			t.Errorf("Units = %v, want %d 個", files, len(want))
		}
		if !strings.Contains(tags["-Users-me-app/orig.jsonl.zst"], " before:") {
			t.Errorf("Tag = %q, want before:", tags["-Users-me-app/orig.jsonl.zst"])
		}
		// 読んだ結果でも、同じサブエージェント（agent-a）を 2 度数えない
		var in float64
		for _, b := range load(t, c) {
			if s := b.Finish(15); s != nil {
				for _, a := range s.Subagents {
					in += a.Usage.In
				}
			}
		}
		if in != 357 {
			t.Errorf("サブエージェントの入力トークン = %v, want 357（agent-a 300 + agent-b 50 + agent-c 7）", in)
		}
	})

	t.Run("移っていない会話はそのまま", func(t *testing.T) {
		root := t.TempDir()
		writeLines(t, filepath.Join(root, "-Users-me-app", "orig.jsonl"), origLines)
		writeLines(t, filepath.Join(root, "-Users-me-app", "orig", "subagents", "agent-a.jsonl"), []string{subLine("a", 300)})
		files, tags := unitFiles(t, &Claude{Root: root})
		if w := []string{"agent-a.jsonl"}; !slices.Equal(files["-Users-me-app/orig.jsonl"], w) || strings.Contains(tags["-Users-me-app/orig.jsonl"], " before:") {
			t.Errorf("サブエージェント = %v, Tag = %q; want %v, before: なし", files, tags["-Users-me-app/orig.jsonl"], w)
		}
	})
}

// 移った会話のあとのファイルが、先に数えるファイルにある行だけでできている（ふつうは kiroku archive に残った移る前のコピー）と、
// そのファイルからはセッションが出ない。そのフォルダにだけあるサブエージェントは、先に数えるファイルのセッションで 1 回数える。
func TestClaudeMovedSessionAdoptsSubagentsOfDuplicateFile(t *testing.T) {
	// 呼び出し（Agent）と結果（agentId）は、移る前から会話にあった（どちらのファイルにもある）
	call := []string{
		`{"type":"assistant","sessionId":"orig","uuid":"c1","timestamp":"2026-09-30T01:30:00Z","requestId":"rc","message":{"id":"mc","model":"claude-opus-5-5","usage":{"input_tokens":10,"output_tokens":5},"content":[{"type":"tool_use","id":"tc","name":"Agent","input":{"description":"調べる","subagent_type":"Explore"}}]}}`,
		`{"type":"user","sessionId":"orig","uuid":"c2","timestamp":"2026-09-30T01:31:00Z","toolUseResult":{"agentId":"c"},"message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"tc","content":"done"}]}}`,
	}
	old := append(slices.Clip(origLines[:3]), call...)
	moved := append(slices.Clip(old), origLines[3:]...)

	count := func(t *testing.T, c *Claude) (sessions int, in float64, byDesc map[string]float64) {
		t.Helper()
		byDesc = map[string]float64{}
		for _, b := range load(t, c) {
			s := b.Finish(15)
			if s == nil {
				continue
			}
			sessions++
			for _, a := range s.Subagents {
				in += a.Usage.In
				byDesc[a.Desc] += a.Usage.In
			}
		}
		return sessions, in, byDesc
	}

	t.Run("呼び出しのある会話のファイルへ", func(t *testing.T) {
		// 会話は移った先（-Users-me-app-wt）へ移り、移る前のフォルダのものは kiroku archive のコピーにだけある。
		// agent-c は移る前のフォルダにしかない
		root, arch := t.TempDir(), t.TempDir()
		writeLines(t, filepath.Join(root, "-Users-me-app-wt", "orig.jsonl"), moved)
		writeLines(t, filepath.Join(root, "-Users-me-app-wt", "orig", "subagents", "agent-b.jsonl"), []string{subLine("b", 50)})
		zstFile(t, filepath.Join(arch, "-Users-me-app", "orig.jsonl.zst"), old, false)
		zstFile(t, filepath.Join(arch, "-Users-me-app", "orig", "subagents", "agent-c.jsonl.zst"), []string{subLine("c", 7)}, false)

		n, in, byDesc := count(t, &Claude{Root: root, Archive: arch})
		if n != 1 || in != 57 || byDesc["調べる"] != 7 {
			t.Errorf("セッション %d 個・サブエージェントの入力トークン %v（呼び出しごと %v）, want 1 個・57（agent-b 50 + 呼び出し「調べる」の agent-c 7）", n, in, byDesc)
		}
	})

	t.Run("呼び出しのないサブエージェントも", func(t *testing.T) {
		// TestClaudeMovedSessionDropsSeenSubagents の kiroku archive の場合で、移る前のコピーに自身の行がないとき
		root, arch := t.TempDir(), t.TempDir()
		zstFile(t, filepath.Join(arch, "-Users-me-app-wt", "orig.jsonl.zst"), origLines, false)
		zstFile(t, filepath.Join(arch, "-Users-me-app-wt", "orig", "subagents", "agent-a.jsonl.zst"), []string{subLine("a", 300)}, false)
		zstFile(t, filepath.Join(arch, "-Users-me-app-wt", "orig", "subagents", "agent-b.jsonl.zst"), []string{subLine("b", 50)}, false)
		zstFile(t, filepath.Join(arch, "-Users-me-app", "orig.jsonl.zst"), origLines[:3], false)
		zstFile(t, filepath.Join(arch, "-Users-me-app", "orig", "subagents", "agent-a.jsonl.zst"), []string{subLine("a", 300)}, false)
		zstFile(t, filepath.Join(arch, "-Users-me-app", "orig", "subagents", "agent-c.jsonl.zst"), []string{subLine("c", 7)}, false)

		if n, in, _ := count(t, &Claude{Root: root, Archive: arch}); n != 1 || in != 357 {
			t.Errorf("セッション %d 個・サブエージェントの入力トークン %v, want 1 個・357（agent-a 300 + agent-b 50 + agent-c 7）", n, in)
		}
	})
}

// 移った会話のあとのファイルに、自身の行としてコンパクションの行だけがある（/cd のあと、人が離れて会話が要約された）と、
// そのファイルからはセッションが出ない。そのフォルダのサブエージェントとコンパクションは、先に数えるファイルのセッションで 1 回数える。
// 写した行は先に数えるファイルの行として 2 度数えない。
func TestClaudeMovedSessionAdoptsCompactionOnlyFile(t *testing.T) {
	idle := []string{
		`{"type":"system","subtype":"compact_boundary","sessionId":"orig","uuid":"k1","timestamp":"2026-09-30T02:30:00Z","compactMetadata":{"trigger":"auto","preTokens":5000}}`,
		`{"type":"user","sessionId":"orig","uuid":"k2","timestamp":"2026-09-30T02:30:05Z","isCompactSummary":true,"message":{"role":"user","content":"This session is being continued from a previous conversation."}}`,
	}
	type result struct {
		sessions    int
		in          float64
		compactions []float64
		kinds       []string
	}
	count := func(t *testing.T, c *Claude) result {
		t.Helper()
		var r result
		for _, b := range load(t, c) {
			s := b.Finish(15)
			if s == nil {
				continue
			}
			r.sessions++
			for _, a := range s.Subagents {
				r.in += a.Usage.In
			}
			r.compactions = append(r.compactions, s.Compactions...)
			r.kinds = append(r.kinds, s.CompactKinds...)
		}
		sort.Float64s(r.compactions)
		return r
	}
	setup := func(t *testing.T, later []string, subagent bool) *Claude {
		root := t.TempDir()
		// 先に数えるファイル（パスの順で -Users-me-app-wt が先）
		writeLines(t, filepath.Join(root, "-Users-me-app-wt", "orig.jsonl"), origLines)
		writeLines(t, filepath.Join(root, "-Users-me-app-wt", "orig", "subagents", "agent-a.jsonl"), []string{subLine("a", 300)})
		writeLines(t, filepath.Join(root, "-Users-me-app", "orig.jsonl"), later)
		if subagent {
			writeLines(t, filepath.Join(root, "-Users-me-app", "orig", "subagents", "agent-c.jsonl"), []string{subLine("c", 7)})
		}
		return &Claude{Root: root}
	}
	manual, auto := *ts(`2026-09-30T01:10:00Z`), *ts(`2026-09-30T02:30:00Z`)

	t.Run("サブエージェントのあるファイル", func(t *testing.T) {
		r := count(t, setup(t, append(slices.Clip(origLines), idle...), true))
		if r.sessions != 1 || r.in != 307 {
			t.Errorf("セッション %d 個・サブエージェントの入力トークン %v, want 1 個・307（agent-a 300 + agent-c 7）", r.sessions, r.in)
		}
		if !slices.Equal(r.compactions, []float64{manual, auto}) || !slices.Equal(r.kinds, []string{"manual", "auto"}) {
			t.Errorf("コンパクション = %v %v, want [%v %v] [manual auto]（写した manual は 1 回、あとのファイルの auto も数える）", r.compactions, r.kinds, manual, auto)
		}
	})

	t.Run("サブエージェントのないファイル", func(t *testing.T) {
		c := setup(t, append(slices.Clip(origLines), idle...), false)
		r := count(t, c)
		if r.sessions != 1 || r.in != 300 || !slices.Equal(r.compactions, []float64{manual, auto}) {
			t.Errorf("セッション %d 個・入力 %v・コンパクション %v, want 1 個・300・[%v %v]", r.sessions, r.in, r.compactions, manual, auto)
		}
		// kiroku serve: あとのファイルが変わったら、最初に数えるファイルを読み直す
		_, tags := unitFiles(t, c)
		writeLines(t, filepath.Join(c.Root, "-Users-me-app", "orig.jsonl"), append(slices.Clip(origLines), idle[0]))
		if _, now := unitFiles(t, c); now["-Users-me-app-wt/orig.jsonl"] == tags["-Users-me-app-wt/orig.jsonl"] {
			t.Errorf("最初に数えるファイルの Tag が変わらない: %q", now["-Users-me-app-wt/orig.jsonl"])
		}
	})

	t.Run("自身の行がほかにもあれば、あとのファイルで数える", func(t *testing.T) {
		own := strings.Replace(userLine("orig", "k3", "続き"), "2026-09-30T01:00:00Z", "2026-09-30T03:00:00Z", 1)
		r := count(t, setup(t, append(append(slices.Clip(origLines), idle...), own), true))
		if r.sessions != 2 || r.in != 307 || !slices.Equal(r.compactions, []float64{manual, auto}) {
			t.Errorf("セッション %d 個・入力 %v・コンパクション %v, want 2 個・307・[%v %v]（auto はあとのファイルのセッションで 1 回）", r.sessions, r.in, r.compactions, manual, auto)
		}
	})
}

// サブエージェントのファイルは、まず agentId でつなぎ、残りを時刻でつなぐ。時刻でつないだファイルが、
// あとで agentId の合う別のファイルに同じ呼び出しを取られて消えることはない（片方は呼び出しのない 1 つとして数える）。
func TestClaudeSubagentAgentIDBeforeTime(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "-Users-me-app")
	writeLines(t, filepath.Join(dir, "s1.jsonl"), []string{
		`{"type":"user","timestamp":"2026-09-30T01:00:00Z","cwd":"/Users/me/app","message":{"role":"user","content":"サブエージェントに任せて"}}`,
		`{"type":"assistant","timestamp":"2026-09-30T01:01:00Z","message":{"id":"m1","model":"claude-sonnet-5-5","content":[{"type":"tool_use","id":"t1","name":"Agent","input":{"subagent_type":"general-purpose","description":"実装"}}]}}`,
		`{"type":"user","timestamp":"2026-09-30T01:20:00Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":"done"}]},"toolUseResult":{"agentId":"ag1"}}`,
	})
	// agent-a0 は名前の順で先に読み、時刻だけが Agent の呼び出しに合う。agent-ag1 はその呼び出しの agentId
	writeLines(t, filepath.Join(dir, "s1", "subagents", "agent-a0.jsonl"), []string{subLine("a0", 300)})
	writeLines(t, filepath.Join(dir, "s1", "subagents", "agent-ag1.jsonl"), []string{subLine("ag1", 50)})
	bs := load(t, &Claude{Root: root})
	if len(bs) != 1 {
		t.Fatalf("セッション数 = %d, want 1", len(bs))
	}
	subs := bs[0].Subagents
	var in float64
	byType := map[string]float64{}
	for _, a := range subs {
		in += a.Usage.In
		byType[a.Type] += a.Usage.In
	}
	if len(subs) != 2 || in != 350 {
		t.Fatalf("サブエージェント = %d 個、入力 %v, want 2 個、350（どちらのファイルも数える）", len(subs), in)
	}
	if byType["general-purpose"] != 50 || byType["subagent"] != 300 {
		t.Errorf("入力 = %v, want Agent の呼び出しに agentId の合う agent-ag1（50）、呼び出しのない 1 つに agent-a0（300）", byType)
	}
}

// 同じ agentId を返す呼び出しが 2 つ（2 つめは再開）で、ファイルが両方の分を持つときは、ファイルの分だけを数える
// （2 つめの呼び出しの結果にある usage で、もう 1 度数えない）。
func TestClaudeResumedSubagentCountedOnce(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "-Users-me-app")
	use := func(id, ts string) string {
		return `{"type":"assistant","timestamp":"` + ts + `","message":{"id":"m` + id + `","model":"claude-sonnet-5-5","content":[{"type":"tool_use","id":"` + id + `","name":"Agent","input":{"subagent_type":"general-purpose","description":"実装"}}]}}`
	}
	result := func(id, ts string, in int) string {
		return fmt.Sprintf(`{"type":"user","timestamp":"%s","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"%s","content":"done"}]},"toolUseResult":{"agentId":"ag","usage":{"input_tokens":%d,"output_tokens":1}}}`, ts, id, in)
	}
	writeLines(t, filepath.Join(dir, "s1.jsonl"), []string{
		`{"type":"user","timestamp":"2026-09-30T01:00:00Z","cwd":"/Users/me/app","message":{"role":"user","content":"任せて"}}`,
		use("t1", "2026-09-30T01:01:00Z"), result("t1", "2026-09-30T01:11:00Z", 5),
		use("t2", "2026-09-30T02:00:00Z"), result("t2", "2026-09-30T02:10:00Z", 7),
	})
	writeLines(t, filepath.Join(dir, "s1", "subagents", "agent-ag.jsonl"), []string{
		`{"type":"assistant","timestamp":"2026-09-30T01:02:00Z","requestId":"r-a","message":{"id":"m-a","model":"claude-haiku-5-5","usage":{"input_tokens":5,"output_tokens":1},"content":[{"type":"text","text":"a"}]}}`,
		`{"type":"assistant","timestamp":"2026-09-30T02:01:00Z","requestId":"r-b","message":{"id":"m-b","model":"claude-haiku-5-5","usage":{"input_tokens":7,"output_tokens":1},"content":[{"type":"text","text":"b"}]}}`,
	})
	bs := load(t, &Claude{Root: root})
	if len(bs) != 1 {
		t.Fatalf("セッション数 = %d, want 1", len(bs))
	}
	var in float64
	for _, a := range bs[0].Subagents {
		in += a.Usage.In
	}
	if in != 12 {
		t.Errorf("サブエージェントの入力 = %v, want 12（ファイルの 5 + 7。再開の結果の usage は数えない）", in)
	}
}

// 同じ Agent の呼び出しの行（同じ id）が 2 度あっても、サブエージェントは 1 つ。
func TestClaudeDuplicateAgentCallLine(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "-Users-me-app")
	call := `{"type":"assistant","timestamp":"2026-09-30T01:01:00Z","message":{"id":"m1","model":"claude-sonnet-5-5","content":[{"type":"tool_use","id":"t1","name":"Agent","input":{"subagent_type":"general-purpose","description":"実装"}}]}}`
	writeLines(t, filepath.Join(dir, "s1.jsonl"), []string{
		`{"type":"user","timestamp":"2026-09-30T01:00:00Z","cwd":"/Users/me/app","message":{"role":"user","content":"任せて"}}`,
		call, call,
		`{"type":"user","timestamp":"2026-09-30T01:20:00Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":"done"}]},"toolUseResult":{"agentId":"ag1"}}`,
	})
	writeLines(t, filepath.Join(dir, "s1", "subagents", "agent-ag1.jsonl"), []string{subLine("ag1", 5)})
	bs := load(t, &Claude{Root: root})
	if len(bs) != 1 {
		t.Fatalf("セッション数 = %d, want 1", len(bs))
	}
	if subs := bs[0].Subagents; len(subs) != 1 || subs[0].Usage.In != 5 {
		t.Errorf("サブエージェント = %d 個（%+v）, want 1 個、入力 5", len(subs), subs)
	}
}

// 名前が agentId として重なるファイル（agent-foo と foo、agent- と .jsonl）も、それぞれ 1 度ずつ数える。
func TestClaudeSubagentFilesWithCollidingIDs(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "-Users-me-app")
	writeLines(t, filepath.Join(dir, "s1.jsonl"), []string{
		`{"type":"user","timestamp":"2026-09-30T01:00:00Z","cwd":"/Users/me/app","message":{"role":"user","content":"任せて"}}`,
		`{"type":"assistant","timestamp":"2026-09-30T01:01:00Z","message":{"id":"m1","model":"claude-sonnet-5-5","content":[{"type":"tool_use","id":"t1","name":"Agent","input":{"description":"a"}},{"type":"tool_use","id":"t2","name":"Agent","input":{"description":"b"}}]}}`,
		`{"type":"user","timestamp":"2026-09-30T01:20:00Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":"done"}]},"toolUseResult":{"agentId":"foo"}}`,
		`{"type":"user","timestamp":"2026-09-30T01:21:00Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t2","content":"done"}]}}`,
	})
	sub := filepath.Join(dir, "s1", "subagents")
	for name, in := range map[string]int{"agent-foo": 1, "foo": 10, "agent-": 100, "": 1000, "agent-agent-foo": 10000} {
		writeLines(t, filepath.Join(sub, name+".jsonl"), []string{subLine("x"+name, in)})
	}
	bs := load(t, &Claude{Root: root})
	if len(bs) != 1 {
		t.Fatalf("セッション数 = %d, want 1", len(bs))
	}
	var in float64
	for _, a := range bs[0].Subagents {
		in += a.Usage.In
	}
	if n := len(bs[0].Subagents); n != 5 || in != 11111 {
		t.Errorf("サブエージェント = %d 個、入力 %v, want 5 個、11111", n, in)
	}
}

// endTree で探した呼び出しは、始まりの順に全部見て最初に合うもの（前の書き方）と同じ。
func TestEndTreeMatchesLinearScan(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	for round := 0; round < 200; round++ {
		n := rng.IntN(40)
		cs := make([]*call, n)
		for i := range cs {
			st := float64(rng.IntN(100))
			cs[i] = &call{start: &st}
			if rng.IntN(4) > 0 {
				e := st + float64(rng.IntN(30))
				cs[i].end = &e
			}
		}
		sort.SliceStable(cs, func(i, j int) bool { return *cs[i].start < *cs[j].start })
		tree, used := newEndTree(cs), make([]bool, n)
		for q := 0; q < 30; q++ {
			fs := float64(rng.IntN(140) - 10)
			want := -1
			for j, x := range cs {
				end := fs
				if x.end != nil {
					end = *x.end
				}
				if !used[j] && *x.start-5 <= fs && fs <= end+5 {
					want = j
					break
				}
			}
			k := sort.Search(n, func(j int) bool { return *cs[j].start-5 > fs })
			got := tree.first(k, fs-5)
			if got != want {
				t.Fatalf("round %d: file at %v: tree %d, linear %d", round, fs, got, want)
			}
			if got >= 0 {
				used[got] = true
				tree.take(got)
			}
		}
	}
}

// バックグラウンドのサブエージェントが親の最後の行のあとまで動いても、その時刻は親のセッションで AI が動いた時刻に入る
// （カレンダーのバーと活動時間から外れない。Kiro Crew と同じ）。コンパクションの行は入れない。
func TestClaudeBackgroundSubagentExtendsParentActivity(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "-Users-me-app")
	writeLines(t, filepath.Join(dir, "s1.jsonl"), []string{
		`{"type":"user","timestamp":"2026-09-30T01:00:00Z","cwd":"/Users/me/app","message":{"role":"user","content":"裏で調べて"}}`,
		`{"type":"assistant","timestamp":"2026-09-30T01:00:30Z","message":{"id":"m1","model":"claude-sonnet-5-5","content":[{"type":"tool_use","id":"t1","name":"Agent","input":{"subagent_type":"Explore","description":"調査","run_in_background":true}}]}}`,
		`{"type":"user","timestamp":"2026-09-30T01:00:31Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":"started"}]},"toolUseResult":{"agentId":"bg1"}}`,
	})
	writeLines(t, filepath.Join(dir, "s1", "subagents", "agent-bg1.jsonl"), []string{
		`{"type":"user","timestamp":"2026-09-30T01:00:32Z","message":{"role":"user","content":"調査"}}`,
		`{"type":"assistant","timestamp":"2026-09-30T01:10:00Z","requestId":"r1","message":{"id":"sm1","model":"claude-haiku-5-5","usage":{"input_tokens":10,"output_tokens":10},"content":[{"type":"text","text":"途中"}]}}`,
		`{"type":"assistant","timestamp":"2026-09-30T01:20:00Z","requestId":"r2","message":{"id":"sm2","model":"claude-haiku-5-5","usage":{"input_tokens":10,"output_tokens":10},"content":[{"type":"text","text":"done"}]}}`,
		`{"type":"system","subtype":"compact_boundary","timestamp":"2026-09-30T02:30:00Z"}`,
	})
	bs := load(t, &Claude{Root: root})
	if len(bs) != 1 {
		t.Fatalf("セッション数 = %d, want 1", len(bs))
	}
	s := bs[0].Finish(15)
	if want := ts(`2026-09-30T01:20:00Z`); s.End != *want {
		t.Errorf("End = %v, want %v（サブエージェントの最後の応答。コンパクションの行は入れない）", s.End, *want)
	}
	if len(s.Segs) != 1 {
		t.Errorf("Segs = %v, want 1 つ（10 分おきのサブエージェントの作業で、親の活動がつながる）", s.Segs)
	}
}

// サブエージェントの時刻のうち、親の会話の最初の行より前のもの（流用されたファイル・壊れた時刻）と、会話の要約を受け持つ
// サブエージェント（agent-acompact-*）の時刻は、親の活動に入れない。親に時刻がなければ、サブエージェントだけでセッションを作らない。
func TestClaudeSubagentActivityBounds(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "-Users-me-app")
	writeLines(t, filepath.Join(dir, "s1.jsonl"), []string{
		`{"type":"user","timestamp":"2026-09-30T05:00:00Z","cwd":"/Users/me/app","message":{"role":"user","content":"調べて"}}`,
		`{"type":"assistant","timestamp":"2026-09-30T05:00:30Z","message":{"id":"m1","model":"claude-sonnet-5-5","content":[{"type":"text","text":"はい"}]}}`,
	})
	writeLines(t, filepath.Join(dir, "s1", "subagents", "agent-old.jsonl"), []string{
		`{"type":"assistant","timestamp":"2000-01-01T00:00:00Z","requestId":"r1","message":{"id":"o1","model":"claude-haiku-5-5","content":[{"type":"text","text":"x"}]}}`,
		`{"type":"assistant","timestamp":"2026-09-30T01:00:00Z","requestId":"r2","message":{"id":"o2","model":"claude-haiku-5-5","content":[{"type":"text","text":"x"}]}}`,
	})
	writeLines(t, filepath.Join(dir, "s1", "subagents", "agent-acompact-1234.jsonl"), []string{
		`{"type":"user","timestamp":"2026-09-30T05:55:00Z","message":{"role":"user","content":"要約して"}}`,
		`{"type":"assistant","timestamp":"2026-09-30T05:55:30Z","requestId":"r3","message":{"id":"c1","model":"claude-haiku-5-5","content":[{"type":"text","text":"要約"}]}}`,
	})
	// 時刻のある行がない会話に、サブエージェントだけがある
	writeLines(t, filepath.Join(dir, "s2.jsonl"), []string{`{"type":"summary","summary":"x"}`})
	writeLines(t, filepath.Join(dir, "s2", "subagents", "agent-x.jsonl"), []string{subLine("x", 10)})
	var got []string
	for _, b := range load(t, &Claude{Root: root}) {
		if s := b.Finish(15); s != nil {
			got = append(got, b.ID)
			if want := ts(`2026-09-30T05:00:00Z`); s.Start != *want {
				t.Errorf("Start = %v, want %v（親の最初の行より前のサブエージェントの時刻は入れない）", s.Start, *want)
			}
			if want := ts(`2026-09-30T05:00:30Z`); s.End != *want {
				t.Errorf("End = %v, want %v（agent-acompact-* の時刻は入れない）", s.End, *want)
			}
		}
	}
	if !slices.Equal(got, []string{"s1"}) {
		t.Errorf("セッション = %v, want [s1]（時刻のない会話は、サブエージェントだけではセッションにしない）", got)
	}
}
