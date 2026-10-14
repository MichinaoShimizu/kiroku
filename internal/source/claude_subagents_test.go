package source

import (
	"fmt"
	"path/filepath"
	"slices"
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
