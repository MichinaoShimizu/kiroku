package cli

import (
	"database/sql"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MichinaoShimizu/kiroku/internal/core"
	"github.com/MichinaoShimizu/kiroku/internal/source"
	_ "modernc.org/sqlite"
)

// kiroDB は、会話 ids（依頼 1 つずつ）を入れた Kiro CLI の SQLite を path に作る。
func kiroDB(t *testing.T, path string, ids ...string) string {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE conversations_v2 (key TEXT, conversation_id TEXT, value TEXT, created_at INTEGER, updated_at INTEGER)`); err != nil {
		t.Fatal(err)
	}
	for _, id := range ids {
		conv := `{"conversation_id": "` + id + `", "history": [{"user": {"timestamp": "2026-09-29T10:00:30Z", "content": {"Prompt": {"prompt": "sqlite-` + id + `"}}}, "assistant": {"Response": {"content": "ok"}}}]}`
		if _, err := db.Exec(`INSERT INTO conversations_v2 VALUES (?, ?, ?, ?, ?)`, "/Users/me/app", id, conv, 1790676000000, 1790676300000); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

// ids は data のセッションの「Source ID」の数。
func ids(data []*core.Session) map[string]int {
	n := map[string]int{}
	for _, s := range data {
		n[s.Source+" "+s.ID]++
	}
	return n
}

// 同じ会話の 3.0 の写しが 2 つの bucket（作業場所）にあっても、カレンダーには 1 つだけ出す。最後に書いた写しを使う。
func TestKiroV3SameIDInTwoBucketsShownOnce(t *testing.T) {
	for _, newer := range []string{"b1", "b2"} {
		home := t.TempDir()
		writeTree(t, home, map[string]string{
			"sessions/b1/sess_u1/session.json":   `{"id": "sess_u1"}`,
			"sessions/b1/sess_u1/messages.jsonl": v3Prompt("2026-09-29T11:00:00Z", "一"),
			"sessions/b2/sess_u1/session.json":   `{"id": "sess_u1"}`,
			"sessions/b2/sess_u1/messages.jsonl": v3Prompt("2026-09-29T11:00:00Z", "一") + v3Prompt("2026-09-29T11:05:00Z", "二"),
		})
		old := time.Now().Add(-time.Hour)
		for _, b := range []string{"b1", "b2"} {
			if b != newer {
				if err := os.Chtimes(filepath.Join(home, "sessions", b, "sess_u1", "messages.jsonl"), old, old); err != nil {
					t.Fatal(err)
				}
			}
		}
		data := loadKiro(t, home)
		if n := ids(data); len(data) != 1 || n["Kiro IDE / CLI (V3) sess_u1"] != 1 {
			t.Fatalf("newer=%s: sessions = %v, want sess_u1 once", newer, n)
		}
		want := map[string]int{"b1": 1, "b2": 2}[newer]
		if got := len(data[0].Prompts); got != want {
			t.Errorf("newer=%s: prompts = %d, want %d (the copy written last)", newer, got, want)
		}
	}
}

// Kiro CLI の SQLite → V2 → 3.0 と移した会話は、3.0 の写しだけを出す（SQLite の写しを重ねて出さない）。
func TestKiroSQLiteCopyOfConvertedConversationShownOnce(t *testing.T) {
	old := logw
	logw = io.Discard
	defer func() { logw = old }()
	home := t.TempDir()
	writeTree(t, home, map[string]string{
		"sessions/cli/k1.json":               classicMeta("k1", "2026-09-29T10:01:00Z", 1),
		"sessions/b1/sess_k1/session.json":   `{"id": "sess_k1"}`,
		"sessions/b1/sess_k1/messages.jsonl": v3Prompt("2026-09-29T11:00:00Z", "続き"),
	})
	dbPath := kiroDB(t, filepath.Join(home, "data.sqlite3"), "k1")
	all := source.All(source.Options{KiroHome: home, CrewHome: filepath.Join(home, "crew"), KiroCLIDB: dbPath})
	data, _ := collect(all, map[string]bool{"kiro": true}, 15)
	if n := ids(data); len(n) != 1 || n["Kiro IDE / CLI (V3) sess_k1"] != 1 {
		t.Errorf("sessions = %v, want only the 3.0 copy sess_k1", n)
	}
}

// 最後に書いた写しが空・壊れている・会話の行がないときは、行を読めるほうの写しを出す（本物の会話を隠さない）。
func TestKiroV3NewestBrokenCopyDoesNotHideReal(t *testing.T) {
	for name, newer := range map[string]map[string]string{
		"empty":       {"sessions/b2/sess_u/session.json": `{"id": "sess_u"}`, "sessions/b2/sess_u/messages.jsonl": ""},
		"unparseable": {"sessions/b2/sess_u/session.json": `{"id": "sess_u"}`, "sessions/b2/sess_u/messages.jsonl": "{not json\n"},
		"no messages": {"sessions/b2/sess_u/session.json": `{"id": "sess_u", "createdAt": "2026-09-29T12:00:00Z"}`},
		"dir":         {"sessions/b2/sess_u/session.json": `{"id": "sess_u"}`, "sessions/b2/sess_u/messages.jsonl/x": ""},
	} {
		home := t.TempDir()
		writeTree(t, home, map[string]string{
			"sessions/b1/sess_u/session.json":   `{"id": "sess_u"}`,
			"sessions/b1/sess_u/messages.jsonl": v3Prompt("2026-09-29T11:00:00Z", "本物"),
		})
		old := time.Now().Add(-time.Hour)
		if err := os.Chtimes(filepath.Join(home, "sessions", "b1", "sess_u", "messages.jsonl"), old, old); err != nil {
			t.Fatal(err)
		}
		writeTree(t, home, newer)
		data := loadKiro(t, home)
		if len(data) != 1 || len(data[0].Prompts) != 1 || data[0].Prompts[0].Text != "本物" {
			t.Errorf("%s: sessions = %v, want the real copy once", name, ids(data))
		}
	}
}

// session.json の id が読めない 3.0 の写しも、フォルダ名の ID で V2 の会話を引き継ぐ（1 回だけ数え、V2 のクレジットを足す）。
func TestKiroClassicCopyWithoutIDCountsOnce(t *testing.T) {
	home := t.TempDir()
	writeTree(t, home, map[string]string{
		"sessions/cli/u1.json":               classicMeta("u1", "2026-09-29T10:01:00Z", 10),
		"sessions/b1/sess_u1/session.json":   `{}`,
		"sessions/b1/sess_u1/messages.jsonl": v3Prompt("2026-09-29T11:00:00Z", "続き"),
	})
	data := loadKiro(t, home)
	if n := ids(data); len(n) != 1 || n["Kiro IDE / CLI (V3) sess_u1"] != 1 {
		t.Fatalf("sessions = %v, want only the 3.0 copy sess_u1", n)
	}
	if got := totalCredits(data); got != 10 {
		t.Errorf("credits = %v, want 10", got)
	}
}

// 重ねて数えない判定（Key）は、エージェントの家族（Family）ごと。V2 が 3.0 の写しに譲っても、同じ形の Key を使う別の家族の会話は落とさない。
func TestSameKeyInAnotherFamilyKept(t *testing.T) {
	old := logw
	logw = io.Discard
	defer func() { logw = old }()
	dir := t.TempDir()
	home := filepath.Join(dir, "kiro")
	writeTree(t, home, map[string]string{
		"sessions/cli/k1.json":               classicMeta("k1", "2026-09-29T10:01:00Z", 1),
		"sessions/b1/sess_k1/session.json":   `{"id": "sess_k1"}`,
		"sessions/b1/sess_k1/messages.jsonl": v3Prompt("2026-09-29T11:00:00Z", "v3"),
	})
	all := []source.Source{
		&source.KiroIDE{Home: home},
		&source.KiroCLI{Home: home, CrewHome: filepath.Join(dir, "crew")},
		&source.QStore{Label: "Kiro CLI (SQLite)", Fam: "kiro", DB: kiroDB(t, filepath.Join(dir, "k.sqlite3"), "k1")},
		&source.QStore{Label: "Other (SQLite)", Fam: "other", DB: kiroDB(t, filepath.Join(dir, "o.sqlite3"), "k1")},
	}
	data, _ := collect(all, map[string]bool{"kiro": true, "other": true}, 15)
	n := ids(data)
	if len(n) != 2 || n["Kiro IDE / CLI (V3) sess_k1"] != 1 || n["Other (SQLite) k1"] != 1 {
		t.Errorf("sessions = %v, want the 3.0 copy and the other family's k1", n)
	}
}

// 3.0 の写しは、フォルダ名（sess_<id>）で V2 の会話を引き継ぐ。ほかの会話の id を session.json に書いた写しは、その会話を引き継がず、
// その会話の V2・SQLite の写しも隠さない。
func TestKiroV3CopyWithAnotherIDDoesNotTakeOver(t *testing.T) {
	old := logw
	logw = io.Discard
	defer func() { logw = old }()
	dir := t.TempDir()
	home := filepath.Join(dir, "kiro")
	writeTree(t, home, map[string]string{
		"sessions/cli/k1.json": classicMeta("k1", "2026-09-29T10:01:00Z", 1),
		// sess_k1 のフォルダはあるが、会話の行がない（V2 の k1 は譲る相手を探す）
		"sessions/b1/sess_k1/session.json":      `{"id": "sess_zzz"}`,
		"sessions/b2/sess_other/session.json":   `{"id": "sess_k1"}`,
		"sessions/b2/sess_other/messages.jsonl": v3Prompt("2026-09-29T11:00:00Z", "other"),
	})
	all := source.All(source.Options{KiroHome: home, CrewHome: filepath.Join(dir, "crew"), KiroCLIDB: kiroDB(t, filepath.Join(dir, "k.sqlite3"), "k1")})
	data, _ := collect(all, map[string]bool{"kiro": true}, 15)
	if n := ids(data); n["Kiro CLI (V2) k1"] != 1 {
		t.Errorf("sessions = %v, want the V2 conversation k1 kept", n)
	}
}

// ほかの会話の id を session.json に書いた写し（壊れた・まねたファイル）が新しくても、本物の sess_k1 を隠さない
// （同じ会話の写しは、Claim と同じくフォルダ名で 1 つにまとめる）
func TestKiroV3CopyWithAnotherIDDoesNotHideTheReal(t *testing.T) {
	home := t.TempDir()
	writeTree(t, home, map[string]string{
		"sessions/b1/sess_k1/session.json":      `{"id": "sess_k1"}`,
		"sessions/b1/sess_k1/messages.jsonl":    v3Prompt("2026-09-29T10:00:00Z", "real one") + v3Prompt("2026-09-29T10:05:00Z", "real two"),
		"sessions/b2/sess_other/session.json":   `{"id": "sess_k1"}`,
		"sessions/b2/sess_other/messages.jsonl": v3Prompt("2026-09-29T11:00:00Z", "other"),
	})
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(filepath.Join(home, "sessions", "b1", "sess_k1", "messages.jsonl"), old, old); err != nil {
		t.Fatal(err)
	}
	var texts []string
	data := loadKiro(t, home)
	if n := ids(data); n["Kiro IDE / CLI (V3) sess_k1"] != 1 || n["Kiro IDE / CLI (V3) sess_other"] != 1 {
		t.Errorf("sessions = %v, want sess_k1 and sess_other once each", n)
	}
	for _, s := range data {
		for _, p := range s.Prompts {
			texts = append(texts, p.Text)
		}
	}
	got := strings.Join(texts, "|")
	if !strings.Contains(got, "real one") || !strings.Contains(got, "real two") {
		t.Errorf("prompts = %q, want the real sess_k1's prompts kept", got)
	}
}

// 2 つの写しが同じ（どのフォルダのものでもない）id を書いていても、同じ ID の会話を 2 つ出さない（画面は ID で会話を引くので、混ざる）
func TestKiroV3SharedForeignIDStaysUnique(t *testing.T) {
	for name, tree := range map[string]map[string]string{
		"no owner": {
			"sessions/b1/sess_a/session.json": `{"id": "sess_z"}`, "sessions/b1/sess_a/messages.jsonl": v3Prompt("2026-09-29T10:00:00Z", "a"),
			"sessions/b2/sess_b/session.json": `{"id": "sess_z"}`, "sessions/b2/sess_b/messages.jsonl": v3Prompt("2026-09-29T11:00:00Z", "b"),
		},
		"chain": {
			"sessions/b1/sess_b/session.json": `{"id": "sess_c"}`, "sessions/b1/sess_b/messages.jsonl": v3Prompt("2026-09-29T10:00:00Z", "b"),
			"sessions/b2/sess_e/session.json": `{"id": "sess_c"}`, "sessions/b2/sess_e/messages.jsonl": v3Prompt("2026-09-29T11:00:00Z", "e"),
		},
	} {
		home := t.TempDir()
		writeTree(t, home, tree)
		data := loadKiro(t, home)
		if len(data) != 2 {
			t.Errorf("%s: %d sessions, want 2", name, len(data))
		}
		for id, n := range ids(data) {
			if n > 1 {
				t.Errorf("%s: %q appears %d times", name, id, n)
			}
		}
	}
}
