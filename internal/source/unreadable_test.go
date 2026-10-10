package source

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/MichinaoShimizu/kiroku/internal/archive"
	"github.com/MichinaoShimizu/kiroku/internal/core"
	"github.com/klauspost/compress/zstd"
)

// 読めなかった履歴は、黙って捨てずにエラーとして返す（「読めなかったファイル」に出す）。読めた分とほかの会話はふつうに出す。

// zstFile は lines を .zst に圧縮して書く。cut なら後ろ半分を切る（途中で切れたコピー）。
func zstFile(t *testing.T, path string, lines []string, cut bool) {
	t.Helper()
	var buf bytes.Buffer
	enc, err := zstd.NewWriter(&buf)
	if err != nil {
		t.Fatal(err)
	}
	enc.Write([]byte(strings.Join(lines, "\n") + "\n"))
	enc.Close()
	b := buf.Bytes()
	if cut {
		b = b[:len(b)/2]
	}
	os.MkdirAll(filepath.Dir(path), 0o755)
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

// noise は圧縮しにくい文字列（.zst が複数のブロックになり、途中で切っても前のほうは読めるように）。
func noise(i int) string {
	var sb strings.Builder
	x := uint32(i*2654435761 + 1)
	for j := 0; j < 300; j++ {
		x ^= x << 13
		x ^= x >> 17
		x ^= x << 5
		fmt.Fprintf(&sb, "%02x", x&0xff)
	}
	return sb.String()
}

func loadAllErr(s Source) ([]string, error) {
	var ids []string
	err := s.Load(func(b *core.Builder) { ids = append(ids, b.ID) })
	return ids, err
}

func TestKiroIDEReportsUnreadableFiles(t *testing.T) {
	home := t.TempDir()
	writeFiles(t, home, map[string]string{
		"sessions/abc/sess_ok/session.json":    `{"id": "sess_ok", "createdAt": "2026-09-29T01:00:00Z"}`,
		"sessions/abc/sess_ok/messages.jsonl":  `{"timestamp": "2026-09-29T01:01:00Z", "payload": {"type": "user", "content": "依頼"}}` + "\n",
		"sessions/abc/sess_bad/session.json":   `{"id": "sess_bad", "createdAt": `, // 書きかけで切れた
		"sessions/abc/sess_bad/messages.jsonl": `{"timestamp": "2026-09-29T02:01:00Z", "payload": {"type": "user", "content": "依頼"}}` + "\n",
		"sessions/abc/sess_nomsg/session.json": `{"id": "sess_nomsg", "createdAt": "2026-09-29T03:00:00Z"}`, // messages.jsonl がないのはエラーにしない
	})
	ids, err := loadAllErr(&KiroIDE{Home: home})
	if err == nil || !strings.Contains(err.Error(), filepath.Join("sess_bad", "session.json")) || strings.Contains(err.Error(), "more") {
		t.Errorf("Load のエラー = %v, want 壊れた session.json だけ", err)
	}
	if !slices.Contains(ids, "sess_ok") || !slices.Contains(ids, "sess_bad") || !slices.Contains(ids, "sess_nomsg") {
		t.Errorf("読めた会話 = %v, want 3 つとも（壊れたものも、読めた分は出す）", ids)
	}
}

func TestKiroIDELegacyReportsUnreadableFiles(t *testing.T) {
	gs := t.TempDir()
	writeFiles(t, gs, map[string]string{
		"workspace-sessions/d3M=/sessions.json": `[{"sessionId": "a", "dateCreated": 1759100000000}, {"sessionId": "gone", "dateCreated": 1759100000000}]`,
		"workspace-sessions/d3M=/a.json":        `{"history": [`,
		"workspace-sessions/eA==/sessions.json": `[{"sessionId": `,
	})
	ids, err := loadAllErr(&KiroIDELegacy{Storages: []string{gs}})
	if err == nil || !strings.Contains(err.Error(), "a.json") || !strings.Contains(err.Error(), "and 1 more") {
		t.Errorf("Load のエラー = %v, want a.json と、ほかに 1 つ（消えた gone.json は数えない）", err)
	}
	if !slices.Contains(ids, "a") {
		t.Errorf("読めた会話 = %v", ids)
	}
}

// Kiro CLI の壊れたメタデータと、Crew の途中で切れた kiroku archive のコピー。
func TestKiroCLIAndCrewReportUnreadableFiles(t *testing.T) {
	kh, ch, arch := t.TempDir(), t.TempDir(), t.TempDir()
	row := func(i int) string {
		return fmt.Sprintf(`{"role": "user", "content": "依頼 %d %s", "ts": "2026-09-30T10:%02d:00+09:00"}`, i, noise(i), i%60)
	}
	var rows []string
	for i := 0; i < 400; i++ {
		rows = append(rows, row(i))
	}
	writeFiles(t, kh, map[string]string{
		"sessions/cli/ok.json":   `{"session_id": "ok", "created_at": "2026-09-30T01:00:00Z"}`,
		"sessions/cli/ok.jsonl":  `{"kind": "Prompt", "timestamp": "2026-09-30T01:00:01Z", "data": {"content": "依頼"}}` + "\n",
		"sessions/cli/bad.json":  `{"session_id": `,
		"sessions/cli/bad.jsonl": `{"kind": "Prompt", "timestamp": "2026-09-30T02:00:01Z", "data": {"content": "依頼"}}` + "\n",
	})
	writeFiles(t, ch, map[string]string{
		"sessions/slack_C1_1.jsonl":                          `{"_type": "metadata", "title": "相談"}` + "\n" + row(500) + "\n",
		"sessions/archive/slack_C1_1__20260930-100000.jsonl": strings.Join(rows, "\n") + "\n",
		"usage/tokens/2026-09-30.jsonl":                      `{"_type": "tokens", "ts": "2026-09-30T13:00:00+09:00", "slot": "slack_C1_1", "credits": 1}` + "\n",
	})
	k := &KiroCLI{Home: kh, CrewHome: ch, CrewArchive: arch}
	for _, x := range k.Keep() {
		if _, err := archive.Sync(x.Src, x.Dst); err != nil {
			t.Fatal(err)
		}
	}
	// Crew が退避の記録を消し、コピーは途中で切れている
	os.Remove(filepath.Join(ch, "sessions", "archive", "slack_C1_1__20260930-100000.jsonl"))
	zstFile(t, filepath.Join(arch, "sessions", "archive", "slack_C1_1__20260930-100000.jsonl.zst"), rows, true)

	var bs []*core.Builder
	err := k.Load(func(b *core.Builder) { bs = append(bs, b) })
	if err == nil || !strings.Contains(err.Error(), "bad.json") || !strings.Contains(err.Error(), "and 1 more") {
		t.Errorf("Load のエラー = %v, want bad.json と、切れたコピーの 2 つ", err)
	}
	var crew *core.Builder
	var ids []string
	for _, b := range bs {
		ids = append(ids, b.ID)
		if b.ID == "crew:slack_C1_1" {
			crew = b
		}
	}
	if !slices.Contains(ids, "ok") || !slices.Contains(ids, "bad") || crew == nil {
		t.Fatalf("読めた会話 = %v", ids)
	}
	if n := len(crew.Prompts); n <= 1 || n >= 401 {
		t.Errorf("Crew の会話の依頼 = %d, want 切れたコピーの読めた分 + 今の記録の 1 つ", n)
	}

	// 切れたコピーだけでも、エラーを返す（ほかに読めないファイルがないとき）
	os.WriteFile(filepath.Join(kh, "sessions", "cli", "bad.json"), []byte(`{"session_id": "bad"}`), 0o644)
	if _, err := loadAllErr(k); err == nil || !strings.Contains(err.Error(), "slack_C1_1__20260930-100000.jsonl.zst") || strings.Contains(err.Error(), "more") {
		t.Errorf("Load のエラー = %v, want 切れたコピーだけ", err)
	}
}

func TestCrewReportsBrokenSessionMap(t *testing.T) {
	ch := t.TempDir()
	writeFiles(t, ch, map[string]string{"session_map.json": `{"a": {"sid": `})
	var errs fileErrs
	if out := loadCrew(ch, &errs, nil); len(out) != 0 || errs.err() == nil || !strings.Contains(errs.err().Error(), "session_map.json") {
		t.Errorf("loadCrew = %v, err = %v", out, errs.err())
	}
	// session_map.json がない（Crew の作りが違う）のはエラーにしない
	os.Remove(filepath.Join(ch, "session_map.json"))
	errs = fileErrs{}
	loadCrew(ch, &errs, nil)
	if errs.err() != nil {
		t.Errorf("session_map.json がないとき err = %v", errs.err())
	}
}

func TestCodexReportsUnreadableFiles(t *testing.T) {
	home := codexHome(t)
	dir := filepath.Join(home, "sessions", "2026", "09", "30")
	// 先頭は読めて、途中で切れたもの（読めた分は出す）
	lines := []string{`{"timestamp": "2026-09-30T06:00:00.000Z", "type": "session_meta", "payload": {"id": "thr-long", "timestamp": "2026-09-30T06:00:00.000Z", "cwd": "/Users/me/api"}}`}
	for i := 0; i < 400; i++ {
		lines = append(lines, fmt.Sprintf(`{"timestamp": "2026-09-30T06:%02d:%02d.000Z", "type": "event_msg", "payload": {"type": "user_message", "message": "依頼 %d %s"}}`, i/60, i%60, i, noise(i)))
	}
	zstFile(t, filepath.Join(dir, "rollout-2026-09-30T15-00-00-thr-long.jsonl.zst"), lines, true)
	// 先頭すら読めないもの
	zstFile(t, filepath.Join(dir, "rollout-2026-09-30T16-00-00-thr-head.jsonl.zst"), lines[:3], true)

	c := &Codex{Home: home}
	var bs []*core.Builder
	err := c.Load(func(b *core.Builder) { bs = append(bs, b) })
	if err == nil || !strings.Contains(err.Error(), "thr-") || !strings.Contains(err.Error(), "and 1 more") {
		t.Errorf("Load のエラー = %v, want 切れた 2 つ", err)
	}
	var ids []string
	var long *core.Builder
	for _, b := range bs {
		ids = append(ids, b.ID)
		if b.ID == "thr-long" {
			long = b
		}
	}
	if !slices.Contains(ids, "thr-main") || !slices.Contains(ids, "thr-new") || long == nil {
		t.Fatalf("読めた会話 = %v", ids)
	}
	if n := len(long.Prompts); n == 0 || n >= 400 {
		t.Errorf("切れたファイルの依頼 = %d, want 読めた分だけ", n)
	}
	// serve の読み直し（Units / LoadUnit）でも、切れたファイルのまとまりだけがエラーになる
	bad := 0
	for _, u := range c.Units() {
		err := c.LoadUnit(u, func(*core.Builder) {})
		isBad := strings.Contains(u.Key, "thr-long") || strings.Contains(u.Key, "thr-head")
		if isBad != (err != nil) {
			t.Errorf("LoadUnit(%s) のエラー = %v", filepath.Base(u.Key), err)
		}
		if isBad {
			bad++
		}
	}
	if bad != 2 {
		t.Errorf("切れたファイルのまとまり = %d, want 2", bad)
	}
}

// 一覧を作ってから読むまでの間に消えたファイルは、エラーにしない。
func TestCodexVanishedFileIsNotAnError(t *testing.T) {
	c := &Codex{Home: codexHome(t)}
	us := c.Units()
	for _, f := range us[0].Files {
		os.Remove(f)
	}
	if err := c.LoadUnit(us[0], func(*core.Builder) {}); err != nil {
		t.Errorf("消えたファイルのエラー = %v", err)
	}
}
