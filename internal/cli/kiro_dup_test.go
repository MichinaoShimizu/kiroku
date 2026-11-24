package cli

import (
	"database/sql"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MichinaoShimizu/kiroku/internal/core"
	"github.com/MichinaoShimizu/kiroku/internal/source"
	_ "modernc.org/sqlite"
)

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
	dbPath := filepath.Join(home, "data.sqlite3")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE conversations_v2 (key TEXT, conversation_id TEXT, value TEXT, created_at INTEGER, updated_at INTEGER)`); err != nil {
		t.Fatal(err)
	}
	conv := `{"conversation_id": "k1", "history": [{"user": {"timestamp": "2026-09-29T10:00:30Z", "content": {"Prompt": {"prompt": "表を作って"}}}, "assistant": {"Response": {"content": "ok"}}}]}`
	if _, err := db.Exec(`INSERT INTO conversations_v2 VALUES (?, ?, ?, ?, ?)`, "/Users/me/app", "k1", conv, 1790676000000, 1790676300000); err != nil {
		t.Fatal(err)
	}
	db.Close()
	all := source.All(source.Options{KiroHome: home, CrewHome: filepath.Join(home, "crew"), KiroCLIDB: dbPath})
	data, _ := collect(all, map[string]bool{"kiro": true}, 15)
	if n := ids(data); len(n) != 1 || n["Kiro IDE / CLI (V3) sess_k1"] != 1 {
		t.Errorf("sessions = %v, want only the 3.0 copy sess_k1", n)
	}
}
