package cli

import (
	"database/sql"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MichinaoShimizu/kiroku/internal/source"
	_ "modernc.org/sqlite"
)

// Kiro Crew の incognito の会話は、Kiro CLI の SQLite に残る写し（同じ会話 ID）も中身を出さない。
// Crew は会話を閉じるときに sessions/cli/<id>.json を消すので、SQLite の写しだけが残る。
func TestCollectHidesWithheldCopies(t *testing.T) {
	old := logw
	logw = io.Discard
	defer func() { logw = old }()
	dir := t.TempDir()
	kiro, crew := filepath.Join(dir, "kiro"), filepath.Join(dir, "crew")
	for p, c := range map[string]string{
		filepath.Join(kiro, "sessions", "cli", ".keep"): "",
		filepath.Join(crew, "session_map.json"):         `{"dashboard:chat-1-100": {"sid": "k1", "cwd": "/w"}}`,
		filepath.Join(crew, "sessions", "dashboard_chat-1-100.jsonl"): `{"_type": "metadata", "title": "SECRET-TITLE", "memory_mode": "incognito"}` + "\n" +
			`{"role": "user", "content": "SECRET-CREW-PROMPT", "ts": "2026-09-29T10:01:00+00:00"}` + "\n",
	} {
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(c), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	dbPath := filepath.Join(dir, "data.sqlite3")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	conv := func(id, prompt string) string {
		return `{"conversation_id": "` + id + `", "history": [{"user": {"timestamp": "2026-09-29T10:01:00Z", "content": {"Prompt": {"prompt": "` + prompt + `"}}},
 "assistant": {"Response": {"content": "` + prompt + `-REPLY"}}, "request_metadata": {"model_id": "m"}}]}`
	}
	for _, q := range []string{
		`CREATE TABLE conversations_v2 (key TEXT, conversation_id TEXT, value TEXT, created_at INTEGER, updated_at INTEGER)`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	for id, prompt := range map[string]string{"k1": "SECRET-SQLITE-PROMPT", "k2": "visible prompt"} {
		if _, err := db.Exec(`INSERT INTO conversations_v2 VALUES (?, ?, ?, ?, ?)`, "/w", id, conv(id, prompt), 1790676000000, 1790676300000); err != nil {
			t.Fatal(err)
		}
	}
	db.Close()
	all := []source.Source{
		&source.KiroCLI{Home: kiro, CrewHome: crew},
		&source.QStore{Label: "Kiro CLI (SQLite)", Fam: "kiro", DB: dbPath, Command: "kiro-cli chat --resume"},
	}
	for _, cache := range []*loadCache{nil, newLoadCache()} {
		for range 2 { // キャッシュから出しても同じ
			data, _ := collectCached(all, map[string]bool{"kiro": true}, 15, cache)
			b, err := json.Marshal(data)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(b), "SECRET") {
				t.Fatalf("SQLite の写しに中身が残っている: %s", b)
			}
			if !strings.Contains(string(b), "visible prompt") || !strings.Contains(string(b), "Kiro Crew private conversation") {
				t.Fatalf("ほかの会話まで隠したか、写しが出ていない: %s", b)
			}
		}
	}
}
