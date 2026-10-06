package source

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MichinaoShimizu/kiroku/internal/core"
	"github.com/klauspost/compress/zstd"
)

// testdata/codex は Codex CLI の作り（rollout-*.jsonl、session_index.jsonl）に合わせた合成データ。
// 新しい版の圧縮ファイル（.jsonl.zst）はテストの中で作る。
func codexHome(t *testing.T) string {
	t.Helper()
	src := filepath.Join("..", "..", "testdata", "codex")
	dst := t.TempDir()
	if err := os.CopyFS(dst, os.DirFS(src)); err != nil {
		t.Fatal(err)
	}
	in, err := os.Open(filepath.Join(dst, "thr-new.jsonl.src"))
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	dir := filepath.Join(dst, "sessions", "2026", "09", "30")
	os.MkdirAll(dir, 0o755)
	out, _ := os.Create(filepath.Join(dir, "rollout-2026-09-30T14-00-00-thr-new.jsonl.zst"))
	enc, _ := zstd.NewWriter(out)
	io.Copy(enc, in)
	enc.Close()
	out.Close()
	return dst
}

func TestCodex(t *testing.T) {
	bs := load(t, &Codex{Home: codexHome(t)})
	if len(bs) != 2 {
		t.Fatalf("セッション数 = %d, want 2（サブエージェントは親にまとめる）", len(bs))
	}
	m := find(bs, "thr-main")
	if m == nil {
		t.Fatal("thr-main がない")
	}
	if m.Title != "ヘッダーのボタンの色" || m.Project != "/Users/me/web" || m.Branch != "feat/button" {
		t.Errorf("title/project/branch = %q %q %q", m.Title, m.Project, m.Branch)
	}
	if len(m.Prompts) != 2 || len(m.FixTS) != 1 {
		t.Errorf("依頼 = %d（環境の説明は数えない）, 言い直し = %d", len(m.Prompts), len(m.FixTS))
	}
	f := m.Finish(15)
	if f.Usage.In != 200+300 || f.Usage.CR != 800+1200 || f.Usage.Out != 200+150 {
		t.Errorf("トークン = %+v（同じ値の書き直しは 1 回、cached は入力から分ける）", f.Usage.Tokens)
	}
	if f.Usage.Unpriced == 0 || f.Usage.Cost != 0 {
		t.Errorf("料金表にないモデルは目安コストに入れない: %+v", f.Usage)
	}
	if f.Models[0][0] != "gpt-5-codex" || f.Resume == nil || *f.Resume != "cd /Users/me/web && codex resume thr-main" {
		t.Errorf("model/resume = %v %v", f.Models, f.Resume)
	}
	if len(f.Subagents) != 1 {
		t.Fatalf("サブエージェント = %d", len(f.Subagents))
	}
	sa := f.Subagents[0]
	if sa.Type != "explorer" || sa.Desc != "ボタンのスタイルがどこで決まっているか調べる" || *sa.Model != "gpt-5-codex-mini" || sa.Tools != 1 {
		t.Errorf("サブエージェント = %+v", sa)
	}
	if sa.Usage.In != 300 || sa.Usage.CR != 100 { // 親の写しの分は数えない
		t.Errorf("サブエージェントのトークン = %+v", sa.Usage.Tokens)
	}
	n := find(bs, "thr-new") // 圧縮ファイル、token_usage_record だけを使う
	if n == nil {
		t.Fatal("圧縮ファイル（.jsonl.zst）が読めていない")
	}
	if fn := n.Finish(15); fn.Usage.In != 2000+100 || fn.Usage.Out != 600 || len(fn.Prompts) != 1 {
		t.Errorf("新しい版のトークン = %+v", fn.Usage.Tokens)
	}
}

func nativeOf(f *core.Session) map[string]core.NativeValue {
	out := map[string]core.NativeValue{}
	for _, v := range f.Native {
		out[v.Label] = v
	}
	return out
}

func TestCodexNativeMetrics(t *testing.T) {
	bs := load(t, &Codex{Home: codexHome(t)})
	n := nativeOf(find(bs, "thr-main").Finish(15))
	// 親の 2 回 + サブエージェントの 1 回（同じ値の書き直しは数えない）
	if n["応答の数"].V != 3 || n["ツール呼び出し"].V != 3 {
		t.Errorf("応答/ツール = %+v %+v", n["応答の数"], n["ツール呼び出し"])
	}
	if v := n["コンテキストの最大使用率"]; v.N == 0 || v.V < 0.5 || v.V > 0.6 { // 1500 / 272000
		t.Errorf("コンテキストの使用率 = %+v", v)
	}
	if _, ok := n["レート制限の最大使用率"]; ok {
		t.Error("記録がなければ出さない")
	}
}

func TestCodexPaginatedUserMessage(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "sessions", "2026", "10", "06")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "rollout-2026-10-06T00-00-00-thr-paginated.jsonl")
	data := "" +
		"{\"timestamp\":\"2026-10-06T00:00:00Z\",\"ordinal\":0,\"type\":\"session_meta\",\"payload\":{\"id\":\"thr-paginated\",\"timestamp\":\"2026-10-06T00:00:00Z\",\"cwd\":\"/tmp/project\",\"history_mode\":\"paginated\"}}\n" +
		"{\"timestamp\":\"2026-10-06T00:00:01Z\",\"ordinal\":1,\"type\":\"event_msg\",\"payload\":{\"type\":\"item_completed\",\"item\":{\"type\":\"user_message\",\"content\":[{\"type\":\"input_text\",\"text\":\"Fix the parser\"}]}}}\n" +
		"{\"timestamp\":\"2026-10-06T00:00:02Z\",\"ordinal\":2,\"type\":\"turn_context\",\"payload\":{\"model\":\"gpt-6-sol\"}}\n"
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	bs := load(t, &Codex{Home: home})
	if len(bs) != 1 {
		t.Fatalf("sessions = %d, want 1", len(bs))
	}
	if len(bs[0].Prompts) != 1 || bs[0].Prompts[0].Text != "Fix the parser" {
		t.Fatalf("paginated user prompt = %+v", bs[0].Prompts)
	}
}

// Units は、サブエージェントのファイルを親と同じまとまりにし、スレッド名を Tag に入れる。
// 見張るのは会話が入っている場所だけ（ログなどは見ない）。
func TestCodexUnits(t *testing.T) {
	c := &Codex{Home: codexHome(t)}
	us := c.Units()
	if len(us) != 2 {
		t.Fatalf("まとまり = %d, want 2（thr-main と thr-sub は一緒）", len(us))
	}
	if len(us[0].Files) != 2 || us[0].Tag != "ヘッダーのボタンの色" {
		t.Errorf("thr-main のまとまり = %+v", us[0])
	}
	if len(us[1].Files) != 1 || !strings.HasSuffix(us[1].Key, ".zst") {
		t.Errorf("thr-new のまとまり = %+v", us[1])
	}
	for _, p := range WatchPaths(c) {
		if p == c.Home || strings.HasPrefix(filepath.Base(p), "log") {
			t.Errorf("見張る場所に %s が入っている", p)
		}
	}
	// 親のファイルがなければ、子が自分のまとまりになる
	os.Remove(us[0].Files[0])
	us = c.Units()
	if len(us) != 2 || len(us[0].Files) != 1 || !strings.Contains(us[0].Key, "thr-sub") {
		t.Errorf("親がないとき = %+v", us)
	}
}

// Codex の応答: event_msg の agent_message と、新しい形（item_completed）・古い形（response_item）の
// どれからも、1 つの依頼につき最後の文を拾う。
func TestCodexReplies(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "sessions", "2026", "09", "30")
	os.MkdirAll(dir, 0o755)
	lines := []string{
		`{"timestamp":"2026-09-30T01:00:00.000Z","type":"session_meta","payload":{"id":"thr-r","timestamp":"2026-09-30T01:00:00.000Z","cwd":"/Users/me/web"}}`,
		`{"timestamp":"2026-09-30T01:00:10.000Z","type":"event_msg","payload":{"type":"user_message","message":"ボタンの色を直して"}}`,
		`{"timestamp":"2026-09-30T01:00:20.000Z","type":"event_msg","payload":{"type":"agent_message","message":"見てみます"}}`,
		`{"timestamp":"2026-09-30T01:00:40.000Z","type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"直しました"}]}}`,
		`{"timestamp":"2026-09-30T01:01:00.000Z","type":"event_msg","payload":{"type":"item_completed","item":{"type":"user_message","message":"テストも足して"}}}`,
		`{"timestamp":"2026-09-30T01:01:30.000Z","type":"event_msg","payload":{"type":"item_completed","item":{"type":"agent_message","content":[{"type":"output_text","text":"足して通しました"}]}}}`,
	}
	os.WriteFile(filepath.Join(dir, "rollout-2026-09-30T10-00-00-thr-r.jsonl"), []byte(strings.Join(lines, "\n")+"\n"), 0o644)
	bs := load(t, &Codex{Home: home})
	if len(bs) != 1 {
		t.Fatalf("セッション数 = %d, want 1", len(bs))
	}
	s := bs[0].Finish(15)
	if len(s.Prompts) != 2 {
		t.Fatalf("依頼 = %d, want 2", len(s.Prompts))
	}
	if r := s.Prompts[0].Reply; r == nil || r.Text != "直しました" {
		t.Errorf("1 件目の応答 = %+v, want 直しました（同じターンの前の文は残さない）", r)
	}
	if r := s.Prompts[1].Reply; r == nil || r.Text != "足して通しました" {
		t.Errorf("2 件目の応答 = %+v", r)
	}
}
