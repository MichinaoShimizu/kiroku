package source

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/MichinaoShimizu/kiroku/internal/archive"
	"github.com/MichinaoShimizu/kiroku/internal/core"
)

// Claude Code が消した会話は、kiroku archive のコピーから読む。元があればそちらを読む。
func TestClaudeReadsArchivedCopy(t *testing.T) {
	root, arch := t.TempDir(), t.TempDir()
	dir := filepath.Join(root, "-Users-me-app")
	os.MkdirAll(filepath.Join(dir, "gone", "subagents"), 0o755)
	write := func(p, prompt string) {
		os.WriteFile(p, []byte(`{"type":"user","timestamp":"2026-09-30T01:00:00Z","cwd":"/Users/me/app","message":{"role":"user","content":"`+prompt+`"}}`+"\n"+
			`{"type":"assistant","timestamp":"2026-09-30T01:01:00Z","message":{"id":"m1","model":"claude-sonnet-5-5","content":[{"type":"text","text":"ok"}]}}`+"\n"), 0o644)
	}
	write(filepath.Join(dir, "gone.jsonl"), "消える会話")
	write(filepath.Join(dir, "gone", "subagents", "agent-a.jsonl"), "サブエージェント")
	write(filepath.Join(dir, "kept.jsonl"), "残る会話")
	c := &Claude{Root: root, Archive: arch}
	for _, k := range c.Keep() {
		if _, err := archive.Sync(k.Src, k.Dst); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(dir, "kept.jsonl"), "残る会話（新しい）") // 元があれば、古いコピーではなく元を読む
	os.Remove(filepath.Join(dir, "gone.jsonl"))
	os.RemoveAll(filepath.Join(dir, "gone"))

	units := c.Units()
	if len(units) != 2 {
		t.Fatalf("Unit = %d, want 2", len(units))
	}
	by := map[string]string{}
	for _, b := range load(t, c) {
		s := b.Finish(15)
		by[s.ID] = s.Prompts[0].Text
		if s.ID == "gone" {
			if !strings.HasSuffix(s.File, ".jsonl.zst") || s.Resume != nil || s.Project != "app" {
				t.Errorf("コピーから読んだ会話: file=%q resume=%v project=%q", s.File, s.Resume, s.Project)
			}
		}
	}
	if by["gone"] != "消える会話" || by["kept"] != "残る会話（新しい）" {
		t.Errorf("読んだ会話 = %v", by)
	}
	for _, u := range units {
		if strings.HasSuffix(u.Key, "gone.jsonl.zst") && len(u.Files) != 2 {
			t.Errorf("サブエージェントのコピーを読んでいない: %v", u.Files)
		}
	}
	if (&Claude{Root: root}).Keep() != nil {
		t.Error("保存場所がないのに残そうとする")
	}
}

// 壊れたコピー（途中で切れた .zst）は、黙って短い会話にせず、エラーとして返す（「読めなかったファイル」に出す）。
// ほかの会話はふつうに読む。
func TestClaudeReportsUnreadableFiles(t *testing.T) {
	root, arch := t.TempDir(), t.TempDir()
	dir := filepath.Join(root, "-Users-me-app")
	os.MkdirAll(dir, 0o755)
	var lines []string
	for i := 0; i < 200; i++ { // 圧縮しても途中で切れるくらいの長さ
		lines = append(lines, `{"type":"user","timestamp":"2026-09-30T01:00:00Z","cwd":"/Users/me/app","message":{"role":"user","content":"依頼 `+strings.Repeat("x", i)+`"}}`)
	}
	for _, id := range []string{"gone1", "gone2"} {
		os.WriteFile(filepath.Join(dir, id+".jsonl"), []byte(strings.Join(lines, "\n")+"\n"), 0o644)
	}
	os.WriteFile(filepath.Join(dir, "kept.jsonl"), []byte(lines[0]+"\n"), 0o644)
	c := &Claude{Root: root, Archive: arch}
	for _, k := range c.Keep() {
		if _, err := archive.Sync(k.Src, k.Dst); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []string{"gone1", "gone2"} { // 元が消えて、コピーは途中で切れている
		os.Remove(filepath.Join(dir, id+".jsonl"))
		cp := filepath.Join(arch, "-Users-me-app", id+".jsonl.zst")
		b, err := os.ReadFile(cp)
		if err != nil {
			t.Fatal(err)
		}
		os.WriteFile(cp, b[:len(b)/2], 0o644)
	}

	var ids []string
	err := c.Load(func(b *core.Builder) { ids = append(ids, b.ID) })
	if err == nil || !strings.Contains(err.Error(), "gone1.jsonl.zst") || !strings.Contains(err.Error(), "and 1 more") {
		t.Errorf("Load のエラー = %v, want 切れたコピーの名前と、ほかに 1 つ", err)
	}
	if !slices.Contains(ids, "kept") {
		t.Errorf("読めた会話 = %v, want kept も読む", ids)
	}
	for _, u := range c.Units() {
		err := c.LoadUnit(u, func(*core.Builder) {})
		if bad := strings.HasSuffix(u.Key, ".zst"); bad != (err != nil) {
			t.Errorf("LoadUnit(%s) のエラー = %v", filepath.Base(u.Key), err)
		}
	}
}

// 再開した会話で、古いサブエージェントのファイルだけが消えたときは、そのコピーを読む。元が残っているものは元を読む。
func TestClaudeReadsArchivedSubagentOfLiveSession(t *testing.T) {
	root, arch := t.TempDir(), t.TempDir()
	dir := filepath.Join(root, "-Users-me-app")
	sub := filepath.Join(dir, "s1", "subagents")
	os.MkdirAll(sub, 0o755)
	line := `{"type":"user","timestamp":"2026-09-30T01:00:00Z","cwd":"/Users/me/app","message":{"role":"user","content":"依頼"}}` + "\n"
	os.WriteFile(filepath.Join(dir, "s1.jsonl"), []byte(line), 0o644)
	os.WriteFile(filepath.Join(sub, "agent-old.jsonl"), []byte(line), 0o644)
	os.WriteFile(filepath.Join(sub, "agent-new.jsonl"), []byte(line), 0o644)
	c := &Claude{Root: root, Archive: arch}
	for _, k := range c.Keep() {
		if _, err := archive.Sync(k.Src, k.Dst); err != nil {
			t.Fatal(err)
		}
	}
	os.Remove(filepath.Join(sub, "agent-old.jsonl"))
	units := c.Units()
	if len(units) != 1 {
		t.Fatalf("Unit = %d, want 1", len(units))
	}
	var got []string
	for _, f := range units[0].Files[1:] {
		got = append(got, filepath.Base(f))
	}
	if strings.Join(got, ",") != "agent-new.jsonl,agent-old.jsonl.zst" {
		t.Errorf("サブエージェントのファイル = %v", got)
	}
}

// Kiro Crew が消した退避の記録（sessions/archive）は、kiroku archive のコピーから読む。
func TestCrewReadsArchivedCopy(t *testing.T) {
	ch, arch := t.TempDir(), t.TempDir()
	os.MkdirAll(filepath.Join(ch, "sessions", "archive"), 0o755)
	os.WriteFile(filepath.Join(ch, "sessions", "slack_C1_1.jsonl"), []byte(`{"_type": "metadata", "title": "相談"}
{"role": "user", "content": "続き", "ts": "2026-09-30T13:00:00+09:00"}
`), 0o644)
	seg := filepath.Join(ch, "sessions", "archive", "slack_C1_1__20260930-100000.jsonl")
	os.WriteFile(seg, []byte(`{"role": "user", "content": "最初の依頼", "ts": "2026-09-30T10:00:00+09:00"}
`), 0o644)
	k := &KiroCLI{Home: t.TempDir(), CrewHome: ch, CrewArchive: arch}
	for _, x := range k.Keep() {
		if _, err := archive.Sync(x.Src, x.Dst); err != nil {
			t.Fatal(err)
		}
	}
	os.Remove(seg)
	_, rows := readCrewKey(ch, arch, "slack_C1_1")
	if len(rows) != 2 || rows[0].text != "最初の依頼" || rows[1].text != "続き" {
		t.Errorf("rows = %+v", rows)
	}
	if _, rows := readCrewKey(ch, "", "slack_C1_1"); len(rows) != 1 {
		t.Errorf("コピーを読まないとき rows = %d", len(rows))
	}
}
