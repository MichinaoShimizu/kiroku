package source

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MichinaoShimizu/kiroku/internal/core"
)

func load(t *testing.T, s Source) []*core.Builder {
	t.Helper()
	var out []*core.Builder
	if err := s.Load(func(b *core.Builder) { out = append(out, b) }); err != nil {
		t.Fatal(err)
	}
	return out
}

func find(bs []*core.Builder, id string) *core.Builder {
	for _, b := range bs {
		if b.ID == id {
			return b
		}
	}
	return nil
}

// testdata/sqlite/*.sqlite3 は amazon-q-developer-cli の作りに合わせた合成データ。
func TestKiroCLISQLite(t *testing.T) {
	time.Local = time.UTC
	db := filepath.Join("..", "..", "testdata", "sqlite", "kiro-cli.sqlite3")
	bs := load(t, &QStore{Label: "Kiro CLI (SQLite)", Fam: "kiro", DB: db, Command: "kiro-cli chat --resume"})
	// v2 の 2 件 + v1 の 1 件（v1 の conv-a は v2 と同じ会話なので読まない）
	if len(bs) != 3 {
		t.Fatalf("会話の数 = %d, want 3", len(bs))
	}
	a := find(bs, "conv-a")
	if a == nil {
		t.Fatal("conv-a がない")
	}
	if a.Project != "/Users/me/app" || a.Key != "kiro-cli:conv-a" {
		t.Errorf("project/key = %q %q", a.Project, a.Key)
	}
	if len(a.Prompts) != 2 || !strings.HasPrefix(a.Prompts[1].Text, "違う") {
		t.Errorf("依頼 = %+v（ツールの結果は依頼に数えない）", a.Prompts)
	}
	if len(a.FixTS) != 1 {
		t.Errorf("言い直し = %d, want 1", len(a.FixTS))
	}
	f := a.Finish(15)
	if f.NFiles != 1 || f.Files[0] != "/Users/me/app/test/login.spec.ts" {
		t.Errorf("変更したファイル = %v", f.Files)
	}
	if len(f.Models) != 1 || f.Models[0][0] != "claude-sonnet-4.5" {
		t.Errorf("モデル = %v", f.Models)
	}
	if want := float64(time.Date(2026, 9, 29, 0, 59, 0, 0, time.UTC).Unix()); f.Start != want {
		t.Errorf("開始 = %v, want %v（created_at はミリ秒）", f.Start, want)
	}
	if len(f.Waits) == 0 {
		t.Error("待たせ時間が取れていない（request_metadata の時刻）")
	}
	if n := nativeOf(f); n["応答にかかった時間（中央値）"].V != 20 || n["ツール呼び出し"].V != 1 {
		t.Errorf("参考指標 = %+v", f.Native)
	}
	b := find(bs, "conv-b")
	if b == nil || b.Project != "/Users/me/ci" || b.Resume != "cd /Users/me/ci && kiro-cli chat --resume" {
		t.Fatalf("conv-b = %+v", b)
	}
	if fb := b.Finish(15); fb.Models[0][0] != "claude-opus-4.5" || fb.Tools[0][0] != "execute_bash" {
		t.Errorf("conv-b のモデル/ツール = %v %v", fb.Models, fb.Tools)
	}
}

func TestAmazonQ(t *testing.T) {
	time.Local = time.UTC
	db := filepath.Join("..", "..", "testdata", "sqlite", "amazon-q.sqlite3")
	bs := load(t, &QStore{Label: "Amazon Q", Fam: "amazonq", DB: db, Command: "q chat --resume"})
	if len(bs) != 2 {
		t.Fatalf("会話の数 = %d, want 2", len(bs))
	}
	if q := find(bs, "q-1"); q == nil || len(q.Prompts) != 1 || q.Finish(15).Models[0][0] != "claude-3-7-sonnet" {
		t.Errorf("q-1 = %+v", q)
	}
	old := find(bs, "q-old") // 古い [user, assistant] の形
	if old == nil || len(old.Prompts) != 1 || old.Prompts[0].Text != "古い形の会話" {
		t.Errorf("古い形の会話が読めていない: %+v", old)
	}
}

func TestSQLiteDSN(t *testing.T) {
	dsn := sqliteDSN(filepath.Join("..", "..", "testdata", "sqlite", "kiro-cli.sqlite3"))
	if !strings.HasPrefix(dsn, "file:///") || !strings.Contains(dsn, "/testdata/sqlite/kiro-cli.sqlite3?mode=ro") {
		t.Errorf("DSN = %q（絶対パスにして file:/// で始める）", dsn)
	}
}

// macOS の既定の場所（Application Support）のように、空白などが入っていても開ける。
func TestSQLitePathWithSpaces(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "Application Support", "kiro-cli #1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join("..", "..", "testdata", "sqlite", "kiro-cli.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	db := filepath.Join(dir, "data.sqlite3")
	os.WriteFile(db, b, 0o644)
	if bs := load(t, &QStore{Label: "Kiro CLI (SQLite)", DB: db}); len(bs) != 3 {
		t.Fatalf("会話の数 = %d, want 3", len(bs))
	}
}

func TestMissingDatabaseIsNotAnError(t *testing.T) {
	bs := load(t, &QStore{Label: "Amazon Q", DB: filepath.Join(t.TempDir(), "none.sqlite3")})
	if len(bs) != 0 {
		t.Fatal("ないファイルは 0 件")
	}
}
