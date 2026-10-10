package cli

import (
	"database/sql"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MichinaoShimizu/kiroku/internal/core"
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

// withheldFixture は Kiro CLI（JSON は空）・Crew・SQLite の場所を作り、Kiro CLI・Kiro CLI (SQLite)・Amazon Q の Source を返す。
// SQLite には k1（Crew の dashboard:chat-1-100 の会話）と k2 の 2 件。Amazon Q の DB には k3（Crew の incognito の
// dashboard:chat-2-200 と同じ会話 ID。Kiro CLI の SQLite にはない）を入れる。
func withheldFixture(t *testing.T) (dir, transcript string, all []source.Source) {
	t.Helper()
	dir = t.TempDir()
	kiro, crew := filepath.Join(dir, "kiro"), filepath.Join(dir, "crew")
	transcript = filepath.Join(crew, "sessions", "dashboard_chat-1-100.jsonl")
	for p, c := range map[string]string{
		filepath.Join(kiro, "sessions", "cli", ".keep"):               "",
		filepath.Join(crew, "session_map.json"):                       `{"dashboard:chat-1-100": {"sid": "k1", "cwd": "/w"}, "dashboard:chat-2-200": {"sid": "k3", "cwd": "/w"}}`,
		transcript:                                                    `{"_type": "metadata", "title": "t", "memory_mode": "persistent"}` + "\n",
		filepath.Join(crew, "sessions", "dashboard_chat-2-200.jsonl"): `{"_type": "metadata", "title": "t", "memory_mode": "incognito"}` + "\n",
	} {
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(c), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	mkdb := func(path string, convs map[string]string) {
		db, err := sql.Open("sqlite", path)
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		if _, err := db.Exec(`CREATE TABLE conversations_v2 (key TEXT, conversation_id TEXT, value TEXT, created_at INTEGER, updated_at INTEGER)`); err != nil {
			t.Fatal(err)
		}
		for id, prompt := range convs {
			v := `{"conversation_id": "` + id + `", "history": [{"user": {"timestamp": "2026-09-29T10:01:00Z", "content": {"Prompt": {"prompt": "` + prompt + `"}}},
 "assistant": {"Response": {"content": "` + prompt + `-REPLY"}}, "request_metadata": {"model_id": "m"}}]}`
			if _, err := db.Exec(`INSERT INTO conversations_v2 VALUES (?, ?, ?, ?, ?)`, "/w", id, v, 1790676000000, 1790676300000); err != nil {
				t.Fatal(err)
			}
		}
	}
	kdb, qdb := filepath.Join(dir, "kiro.sqlite3"), filepath.Join(dir, "q.sqlite3")
	mkdb(kdb, map[string]string{"k1": "k1-sqlite-prompt", "k2": "visible prompt"})
	mkdb(qdb, map[string]string{"k3": "amazon-q-prompt"})
	all = []source.Source{
		&source.KiroCLI{Home: kiro, CrewHome: crew},
		&source.QStore{Label: "Kiro CLI (SQLite)", Fam: "kiro", DB: kdb, Command: "kiro-cli chat --resume"},
		&source.QStore{Label: "Amazon Q", Fam: "amazonq", DB: qdb, Command: "q chat --resume"},
	}
	return dir, transcript, all
}

// sessionBy は data のうち、source と ID が合うセッション。
func sessionBy(data []*core.Session, src, id string) *core.Session {
	for _, s := range data {
		if s.Source == src && s.ID == id {
			return s
		}
	}
	return nil
}

// kiroku serve はキャッシュのセッションを使い回し、前の画面のデータとして同時に読む。会話があとから中身を出さないものになっても、
// キャッシュのセッション（前の画面のデータ）は変えずに、隠した写しを出す。persistent に戻れば、また中身を出す。
// Amazon Q の会話は、同じ会話 ID でも隠さない。go test -race で、キャッシュのセッションを書きかえていないことも確かめる。
func TestCollectWithheldDoesNotMutateCache(t *testing.T) {
	old := logw
	logw = io.Discard
	defer func() { logw = old }()
	_, transcript, all := withheldFixture(t)
	want := map[string]bool{"kiro": true, "amazonq": true}
	cache := newLoadCache()
	first, _ := collectCached(all, want, 15, cache)
	k1 := sessionBy(first, "Kiro CLI (SQLite)", "k1")
	if k1 == nil || k1.Prompts[0].Text != "k1-sqlite-prompt" {
		t.Fatalf("persistent の会話の写し = %+v", k1)
	}
	// 前の画面のデータを読み続けながら、会話を incognito にして読み直す
	stop, done := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			default:
			}
			for _, s := range first {
				for _, p := range s.Prompts {
					_ = p.Text + p.Full()
				}
				_ = s.Title + s.File
			}
		}
	}()
	if err := os.WriteFile(transcript, []byte(`{"_type": "metadata", "title": "t", "memory_mode": "incognito"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	second, _ := collectCached(all, want, 15, cache)
	close(stop)
	<-done
	if s := sessionBy(second, "Kiro CLI (SQLite)", "k1"); s == nil || s.Prompts[0].Text != "(private)" || s.Title != "Kiro Crew private conversation" {
		t.Errorf("incognito にしたあとの写し = %+v", s)
	}
	if k1.Prompts[0].Text != "k1-sqlite-prompt" || k1.Title == "Kiro Crew private conversation" {
		t.Errorf("キャッシュのセッションを書きかえた: %+v", k1)
	}
	if s := sessionBy(second, "Amazon Q", "k3"); s == nil || s.Prompts[0].Text != "amazon-q-prompt" {
		t.Errorf("Amazon Q の会話まで隠した: %+v", s)
	}
	// persistent に戻せば、また中身を出す（前に隠したものが残らない）
	if err := os.WriteFile(transcript, []byte(`{"_type": "metadata", "title": "back", "memory_mode": "persistent"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	third, _ := collectCached(all, want, 15, cache)
	if s := sessionBy(third, "Kiro CLI (SQLite)", "k1"); s == nil || s.Prompts[0].Text != "k1-sqlite-prompt" {
		t.Errorf("persistent に戻したあとの写し = %+v", s)
	}
}
