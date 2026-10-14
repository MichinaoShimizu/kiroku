package cli

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MichinaoShimizu/kiroku/internal/core"
	"github.com/MichinaoShimizu/kiroku/internal/source"
)

// crewPrivateFixture は、中身を残さない（incognito の）Kiro Crew の会話の写しが Claude Code・Codex の履歴に残る場所を作る（合成のデータ）。
//
//	dashboard:chat-1-100（incognito）: 今の sid は Claude Code の backend（c-priv）、前の sid（discarded_sid）は Codex（x-old）。
//	                                   サブエージェント 2 つ（Claude Code の s-claude、Codex の s-codex）と、crew-log の会話（c-log）。
//	dashboard:chat-2-200（persistent）: Codex（x-pub）
//	dashboard:chat-3-300（persistent）: Claude Code（c-pub）
//	dashboard:chat-4-400（incognito）: Codex（x-priv）
//
// 中身を出してはいけない写しには SECRET を、出す写しには VISIBLE を書く。
func crewPrivateFixture(t *testing.T) (kiro, crew, claude, codex string) {
	t.Helper()
	dir := t.TempDir()
	kiro, crew, claude, codex = filepath.Join(dir, "kiro"), filepath.Join(dir, "crew"), filepath.Join(dir, "claude"), filepath.Join(dir, "codex")
	transcript := func(mode string) string {
		return `{"_type": "metadata", "title": "t", "memory_mode": "` + mode + `"}` + "\n" +
			`{"role": "user", "content": "crew prompt", "ts": "2026-10-06T00:00:01+00:00", "meta": {"human": true}}` + "\n"
	}
	claudeLog := func(prompt string) string {
		return `{"type":"user","timestamp":"2026-10-06T00:00:01Z","cwd":"/w","message":{"role":"user","content":"` + prompt + `"}}` + "\n" +
			`{"type":"assistant","timestamp":"2026-10-06T00:00:09Z","cwd":"/w","message":{"role":"assistant","model":"claude-sonnet-4-5","content":[{"type":"text","text":"` + prompt + `-REPLY"}],"usage":{"input_tokens":10,"output_tokens":5}}}` + "\n"
	}
	rollout := func(id, prompt string) string {
		return `{"timestamp":"2026-10-06T00:00:00.000Z","type":"session_meta","payload":{"id":"` + id + `","timestamp":"2026-10-06T00:00:00.000Z","cwd":"/w"}}` + "\n" +
			`{"timestamp":"2026-10-06T00:00:01.000Z","type":"event_msg","payload":{"type":"user_message","message":"` + prompt + `"}}` + "\n" +
			`{"timestamp":"2026-10-06T00:00:09.000Z","type":"event_msg","payload":{"type":"agent_message","message":"` + prompt + `-REPLY"}}` + "\n"
	}
	rolloutPath := func(id string) string {
		return filepath.Join(codex, "sessions", "2026", "10", "06", "rollout-2026-10-06T00-00-00-"+id+".jsonl")
	}
	files := map[string]string{
		filepath.Join(kiro, "sessions", "cli", ".keep"): "",
		filepath.Join(crew, "session_map.json"): `{"dashboard:chat-1-100": {"sid": "c-priv", "discarded_sid": "x-old", "provider": "claude_code", "cwd": "/w"},
 "dashboard:chat-2-200": {"sid": "x-pub", "provider": "codex", "cwd": "/w"},
 "dashboard:chat-3-300": {"sid": "c-pub", "provider": "claude_code", "cwd": "/w"},
 "dashboard:chat-4-400": {"sid": "x-priv", "provider": "codex", "cwd": "/w"}}`,
		filepath.Join(crew, "sessions", "dashboard_chat-1-100.jsonl"): transcript("incognito"),
		filepath.Join(crew, "sessions", "dashboard_chat-2-200.jsonl"): transcript("persistent"),
		filepath.Join(crew, "sessions", "dashboard_chat-3-300.jsonl"): transcript("persistent"),
		filepath.Join(crew, "sessions", "dashboard_chat-4-400.jsonl"): transcript("incognito"),
		// あとから incognito にしたサブエージェントは state.json が残る（memory_mode は親の印でも決まる）
		filepath.Join(crew, "subagents", "a1", "state.json"):              `{"parent_session": "dashboard:chat-1-100", "session_id": "s-claude", "provider": "claude_code", "agent": "dev", "task": "SECRET-TASK"}`,
		filepath.Join(crew, "subagents", "a2", "state.json"):              `{"parent_session": "dashboard:chat-1-100", "session_id": "s-codex", "provider": "codex", "agent": "dev", "task": "SECRET-TASK"}`,
		filepath.Join(crew, "crew-log", "sessions", "c-log", "log.jsonl"): `{"type": "session", "id": "c-log", "slot": "dashboard:chat-1-100", "cwd": "/w"}` + "\n",
		filepath.Join(claude, "-w", "c-priv.jsonl"):                       claudeLog("SECRET-CLAUDE-CURRENT"),
		filepath.Join(claude, "-w", "s-claude.jsonl"):                     claudeLog("SECRET-CLAUDE-SUBAGENT"),
		filepath.Join(claude, "-w", "c-log.jsonl"):                        claudeLog("SECRET-CLAUDE-CREWLOG"),
		filepath.Join(claude, "-w", "c-pub.jsonl"):                        claudeLog("VISIBLE-CLAUDE"),
		rolloutPath("x-old"):                                              rollout("x-old", "SECRET-CODEX-FORMER"),
		rolloutPath("s-codex"):                                            rollout("s-codex", "SECRET-CODEX-SUBAGENT"),
		rolloutPath("x-priv"):                                             rollout("x-priv", "SECRET-CODEX-CURRENT"),
		rolloutPath("x-pub"):                                              rollout("x-pub", "VISIBLE-CODEX"),
	}
	for p, c := range files {
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(c), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return kiro, crew, claude, codex
}

func crewPrivateSources(t *testing.T) []source.Source {
	kiro, crew, claude, codex := crewPrivateFixture(t)
	return []source.Source{&source.Claude{Root: claude}, &source.KiroCLI{Home: kiro, CrewHome: crew}, &source.Codex{Home: codex}}
}

// assertPrivate は、data に secret の文字が残らず、id の写しが数と時刻だけ（"(private)"）で出ていることを確かめる。
func assertPrivate(t *testing.T, data []*core.Session, src string, ids ...string) {
	t.Helper()
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "SECRET") {
		t.Errorf("中身を残さない Crew の会話の写しに中身が残っている: %s", b)
	}
	for _, id := range ids {
		s := sessionBy(data, src, id)
		if s == nil || len(s.Prompts) != 1 || s.Prompts[0].Text != "(private)" || s.Title != "Kiro Crew private conversation" {
			t.Errorf("%s の %s = %+v, want 数と時刻だけの写し", src, id, s)
		}
	}
}

// assertVisible は、中身を残す会話の写しが、そのまま出ていることを確かめる。
func assertVisible(t *testing.T, data []*core.Session, src, id, prompt string) {
	t.Helper()
	s := sessionBy(data, src, id)
	if s == nil || len(s.Prompts) != 1 || s.Prompts[0].Text != prompt || s.Title == "Kiro Crew private conversation" {
		t.Errorf("中身を残す会話 %s の %s = %+v, want 依頼 %q のまま", src, id, s, prompt)
	}
}

func quietLog(t *testing.T) {
	old := logw
	logw = io.Discard
	t.Cleanup(func() { logw = old })
}

// S1: Claude Code の backend で動かした incognito の Crew の会話は、Claude Code の記録（ACP の会話 ID = セッション ID）も
// 数と時刻だけにする。中身を残す会話の Claude Code の記録はそのまま出す。
func TestCollectCrewPrivateClaudeBackend(t *testing.T) {
	quietLog(t)
	all := crewPrivateSources(t)
	for _, cache := range []*loadCache{nil, newLoadCache()} {
		for range 2 {
			data, _ := collectCached(all, map[string]bool{"kiro": true, "claude": true}, 15, cache)
			assertPrivate(t, data, "Claude Code", "c-priv")
			assertVisible(t, data, "Claude Code", "c-pub", "VISIBLE-CLAUDE")
		}
	}
}

// S2: Kiro を選ばない（--sources codex・claude）ときも、Crew の記録を調べて、中身を残さない会話の写しを隠す。
func TestCollectCrewPrivateWithoutKiroSource(t *testing.T) {
	quietLog(t)
	all := crewPrivateSources(t)
	for _, want := range []map[string]bool{{"codex": true}, {"claude": true}, {"codex": true, "claude": true}} {
		for _, cache := range []*loadCache{nil, newLoadCache()} {
			data, rep := collectCached(all, want, 15, cache)
			if want["codex"] {
				assertPrivate(t, data, "Codex", "x-priv", "x-old", "s-codex")
				assertVisible(t, data, "Codex", "x-pub", "VISIBLE-CODEX")
			}
			if want["claude"] {
				assertPrivate(t, data, "Claude Code", "c-priv", "s-claude", "c-log")
				assertVisible(t, data, "Claude Code", "c-pub", "VISIBLE-CLAUDE")
			}
			for _, r := range rep {
				if r.Name == "Kiro CLI (Classic)" {
					t.Errorf("選んでいない Kiro CLI を読んだ: %+v", rep)
				}
			}
			for _, s := range data {
				if s.Source == "Kiro Crew" || s.Source == "Kiro CLI (Classic)" {
					t.Errorf("選んでいない Kiro のセッションを出した: %+v", s)
				}
			}
		}
	}
}

// S3: 中身を残さない会話の、今の sid だけでなく、前の sid（discarded_sid。backend はわからない）・サブエージェントの
// session_id・crew-log の会話の id も、provider によらず Codex・Claude Code の両方で隠す。
func TestCollectCrewPrivateFormerAndSubagentSids(t *testing.T) {
	quietLog(t)
	all := crewPrivateSources(t)
	for _, cache := range []*loadCache{nil, newLoadCache()} {
		for range 2 {
			data, _ := collectCached(all, map[string]bool{"kiro": true, "codex": true, "claude": true}, 15, cache)
			assertPrivate(t, data, "Codex", "x-priv", "x-old", "s-codex")
			assertPrivate(t, data, "Claude Code", "c-priv", "s-claude", "c-log")
		}
	}
}

// 中身を残す会話は変わらない: Codex・Claude Code の記録はそのまま出し、Codex の backend の Crew の会話は
// Codex の記録のセッションだけを数える（Claude Code のセッションに Claim がついても、数え方は変わらない）。
func TestCollectCrewNonPrivateUnchanged(t *testing.T) {
	quietLog(t)
	all := crewPrivateSources(t)
	data, rep := collect(all, map[string]bool{"kiro": true, "codex": true, "claude": true}, 15)
	assertVisible(t, data, "Codex", "x-pub", "VISIBLE-CODEX")
	assertVisible(t, data, "Claude Code", "c-pub", "VISIBLE-CLAUDE")
	if sessionBy(data, "Kiro Crew", "crew:dashboard:chat-2-200") != nil {
		t.Error("Codex のセッションがある会話を、Crew のセッションとしても出している")
	}
	claudeN := 0
	for _, s := range data {
		if s.Source == "Claude Code" {
			claudeN++
		}
	}
	if claudeN != 4 {
		t.Errorf("Claude Code のセッション = %d, want 4（隠しても数からは落とさない）", claudeN)
	}
	for _, r := range rep {
		if r.Name == "Claude Code" && r.Dup != 0 {
			t.Errorf("Claude Code の会話を重なりとして落とした: %+v", r)
		}
	}
	// 中身を残さない会話がなければ、何も隠さない
	if err := os.Remove(filepath.Join(all[1].(*source.KiroCLI).CrewHome, "session_map.json")); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"dashboard_chat-1-100.jsonl", "dashboard_chat-4-400.jsonl"} {
		if err := os.WriteFile(filepath.Join(all[1].(*source.KiroCLI).CrewHome, "sessions", p), []byte(`{"_type": "metadata", "title": "t", "memory_mode": "persistent"}`+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	data, _ = collect(all, map[string]bool{"codex": true, "claude": true}, 15)
	for _, s := range data {
		if s.Title == "Kiro Crew private conversation" {
			t.Errorf("中身を残す会話まで隠した: %s %s", s.Source, s.ID)
		}
	}
	assertVisible(t, data, "Claude Code", "c-priv", "SECRET-CLAUDE-CURRENT")
	assertVisible(t, data, "Codex", "x-priv", "SECRET-CODEX-CURRENT")
}
