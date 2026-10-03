package source

import (
	"io"
	"os"
	"path/filepath"
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
