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
		// 3.0 へ移した写し（kiro-cli chat --v3 --resume-id k1）も中身を出さない
		filepath.Join(kiro, "sessions", "b1", "sess_k1", "session.json"):   `{"id": "sess_k1", "title": "SECRET-V3-TITLE"}`,
		filepath.Join(kiro, "sessions", "b1", "sess_k1", "messages.jsonl"): `{"timestamp": "2026-09-29T10:02:00Z", "payload": {"type": "user", "content": "SECRET-V3-PROMPT"}}` + "\n",
		filepath.Join(crew, "session_map.json"):                            `{"dashboard:chat-1-100": {"sid": "k1", "cwd": "/w"}}`,
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
		&source.KiroIDE{Home: kiro},
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

// Kiro Crew の Codex の backend の会話は、Codex 自身の記録（CODEX_HOME。ACP の会話 ID がスレッド ID）にも残る。
// 1 回だけ数える: Codex のセッションがあれば Crew の記録から作るセッションは出さない（依頼・時刻を 2 度数えない）。
// Codex の読み取りを選ばないとき（--sources kiro）や、Codex のスレッドがないときは、Crew のセッションを出す。
// 中身を残さない（incognito の）会話なら、Codex のセッションも数と時刻だけにする。
func TestCollectCrewCodexBackendCountedOnce(t *testing.T) {
	old := logw
	logw = io.Discard
	defer func() { logw = old }()
	dir := t.TempDir()
	kiro, crew, codex := filepath.Join(dir, "kiro"), filepath.Join(dir, "crew"), filepath.Join(dir, "codex")
	rollout := func(id, prompt string) string {
		return `{"timestamp":"2026-10-06T00:00:00.000Z","type":"session_meta","payload":{"id":"` + id + `","timestamp":"2026-10-06T00:00:00.000Z","cwd":"/w"}}` + "\n" +
			`{"timestamp":"2026-10-06T00:00:01.000Z","type":"event_msg","payload":{"type":"user_message","message":"` + prompt + `"}}` + "\n" +
			`{"timestamp":"2026-10-06T00:00:09.000Z","type":"event_msg","payload":{"type":"agent_message","message":"done"}}` + "\n"
	}
	transcript := func(title, mode, prompt string) string {
		return `{"_type": "metadata", "title": "` + title + `", "memory_mode": "` + mode + `"}` + "\n" +
			`{"role": "user", "content": "` + prompt + `", "ts": "2026-10-06T00:00:01+00:00", "meta": {"human": true}}` + "\n"
	}
	usage := func(slot string) string {
		return `{"_type": "tokens", "ts": "2026-10-06T00:00:09+00:00", "slot": "` + slot + `", "provider": "acp", "model": "gpt-6-sol", "input": 10, "output": 5, "credits": 0.0, "turns": 0, "duration_ms": 8000}` + "\n"
	}
	for p, c := range map[string]string{
		filepath.Join(kiro, "sessions", "cli", ".keep"): "",
		filepath.Join(crew, "session_map.json"): `{"dashboard:chat-1-100": {"sid": "thr-a", "provider": "codex", "cwd": "/w"},
 "dashboard:chat-2-200": {"sid": "thr-b", "provider": "codex", "cwd": "/w"},
 "dashboard:chat-3-300": {"sid": "thr-gone", "provider": "codex", "cwd": "/w"}}`,
		filepath.Join(crew, "usage", "tokens", "2026-10-06.jsonl"):                                      usage("chat-1-100") + usage("chat-2-200") + usage("chat-3-300"),
		filepath.Join(crew, "sessions", "dashboard_chat-1-100.jsonl"):                                   transcript("a", "persistent", "prompt a"),
		filepath.Join(crew, "sessions", "dashboard_chat-2-200.jsonl"):                                   transcript("SECRET-TITLE", "incognito", "SECRET-CREW-PROMPT"),
		filepath.Join(crew, "sessions", "dashboard_chat-3-300.jsonl"):                                   transcript("gone", "persistent", "prompt gone"),
		filepath.Join(codex, "sessions", "2026", "10", "06", "rollout-2026-10-06T00-00-00-thr-a.jsonl"): rollout("thr-a", "prompt a"),
		filepath.Join(codex, "sessions", "2026", "10", "06", "rollout-2026-10-06T00-00-00-thr-b.jsonl"): rollout("thr-b", "SECRET-CODEX-PROMPT"),
	} {
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(c), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	all := []source.Source{&source.KiroCLI{Home: kiro, CrewHome: crew}, &source.Codex{Home: codex}}
	for _, cache := range []*loadCache{nil, newLoadCache()} {
		for range 2 { // キャッシュから出しても同じ
			data, rep := collectCached(all, map[string]bool{"kiro": true, "codex": true}, 15, cache)
			prompts := 0
			for _, s := range data {
				prompts += len(s.Prompts)
			}
			if len(data) != 3 || prompts != 3 {
				t.Fatalf("セッション = %d、依頼 = %d, want 3 と 3（Codex の 2 件と、Codex のスレッドがない Crew の 1 件）", len(data), prompts)
			}
			if sessionBy(data, "Codex", "thr-a") == nil || sessionBy(data, "Kiro Crew", "crew:dashboard:chat-3-300") == nil {
				t.Errorf("Codex のセッションと、スレッドのない Crew のセッションを出す: %+v", data)
			}
			if sessionBy(data, "Kiro Crew", "crew:dashboard:chat-1-100") != nil {
				t.Error("Codex のセッションがある会話を、Crew のセッションとしても出している")
			}
			if r := rep[0]; r.Name != "Kiro CLI (Classic)" || r.Dup != 2 {
				t.Errorf("Kiro CLI の report = %+v, want dup 2", r)
			}
			b, err := json.Marshal(data)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(b), "SECRET") {
				t.Fatalf("incognito の会話の中身が Codex のセッションに残っている: %s", b)
			}
			if s := sessionBy(data, "Codex", "thr-b"); s == nil || len(s.Prompts) != 1 || s.Prompts[0].Text != "(private)" {
				t.Errorf("incognito の会話の Codex のセッション = %+v（数と時刻は残す）", s)
			}
		}
	}
	// Codex を読まないときは、Crew のセッションを出す
	data, _ := collect(all, map[string]bool{"kiro": true}, 15)
	if len(data) != 3 || sessionBy(data, "Kiro Crew", "crew:dashboard:chat-1-100") == nil {
		t.Errorf("Codex を読まないときの Crew のセッション = %d 件", len(data))
	}
}
