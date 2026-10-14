package cli

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

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

// writeUnder は root の下に files（相対パス → 中身）を書く。
func writeUnder(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for p, c := range files {
		p = filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(c), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// claudeTranscript は Claude Code の会話のファイル（依頼 1 つと応答 1 つ）。
func claudeTranscript(prompt string) string {
	return `{"type":"user","timestamp":"2026-10-06T00:00:01Z","cwd":"/w","message":{"role":"user","content":"` + prompt + `"}}` + "\n" +
		`{"type":"assistant","timestamp":"2026-10-06T00:00:09Z","cwd":"/w","message":{"role":"assistant","model":"claude-sonnet-4-5","content":[{"type":"text","text":"` + prompt + `-REPLY"}],"usage":{"input_tokens":10,"output_tokens":5}}}` + "\n"
}

// kiroku serve で Kiro を選んでいない（--sources claude）とき、Crew の側だけが変わって会話が中身を残さないものになっても
// （ここでは session_map の flags）、ほかの履歴が変わるのを待たずに読み直して、Claude Code の写しを隠す。
func TestServeCrewPrivateWithoutKiroReloads(t *testing.T) {
	quietLog(t)
	all := crewPrivateSources(t)
	want := map[string]bool{"claude": true}
	crew := all[1].(*source.KiroCLI).CrewHome
	var picked []source.Source
	for _, s := range all {
		if want[s.Family()] {
			picked = append(picked, s)
		}
	}
	cache := newLoadCache()
	load := func() snapshot {
		data, rep := collectCached(all, want, 15, cache)
		return snapshot{data: data, meta: map[string]any{}, rep: rep}
	}
	var paths []string
	for _, s := range picked {
		paths = append(paths, source.WatchPaths(s)...)
	}
	l := &live{load: load, paths: append(paths, scanWatchPaths(all, want)...), print: io.Discard}
	if err := l.refresh(); err != nil {
		t.Fatal(err)
	}
	l.markReady()
	snapData := func() []*core.Session {
		l.mu.RLock()
		defer l.mu.RUnlock()
		return l.snap.data
	}
	assertVisible(t, snapData(), "Claude Code", "c-pub", "VISIBLE-CLAUDE")
	stop, done := make(chan struct{}), make(chan struct{})
	defer func() { close(stop); <-done }()
	go func() { l.watch(l.fingerprint(), 20*time.Millisecond, stop); close(done) }()
	time.Sleep(60 * time.Millisecond)
	writeUnder(t, crew, map[string]string{"session_map.json": `{"dashboard:chat-1-100": {"sid": "c-priv", "discarded_sid": "x-old", "provider": "claude_code", "cwd": "/w"},
 "dashboard:chat-3-300": {"sid": "c-pub", "provider": "claude_code", "cwd": "/w", "flags": {"incognito": true}}}`})
	deadline := time.Now().Add(5 * time.Second)
	for {
		if s := sessionBy(snapData(), "Claude Code", "c-pub"); s != nil && s.Title == "Kiro Crew private conversation" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("Crew の側だけが変わったとき、読み直さず、あとから中身を残さないものになった会話の Claude Code の写しを出したまま")
		}
		time.Sleep(20 * time.Millisecond)
	}
	b, _ := json.Marshal(snapData())
	if strings.Contains(string(b), "VISIBLE-CLAUDE") {
		t.Errorf("隠した写しの中身が残っている: %s", b)
	}
}

// Kiro を選んでいるときは、Crew の場所はもう見張っている（WatchPaths）ので、足さない。
func TestScanWatchPathsOnlyUnpicked(t *testing.T) {
	all := crewPrivateSources(t)
	crew := all[1].(*source.KiroCLI).CrewHome
	if got := scanWatchPaths(all, map[string]bool{"kiro": true, "claude": true}); len(got) != 0 {
		t.Errorf("Kiro を選んでいるのに足した: %v", got)
	}
	if got := scanWatchPaths(all, map[string]bool{"codex": true}); len(got) == 0 || got[0] != crew {
		t.Errorf("Kiro を選んでいないのに Crew の場所を見張らない: %v", got)
	}
}

// crew-log の 1 行目に slot がない（null）ときは、readCrewLog と同じく session/opened の data.slot で会話キーを決める。
// その会話キーが中身を残さないものなら、crew-log の会話 ID の写しを隠す。
func TestCollectCrewPrivateCrewLogOpenedSlot(t *testing.T) {
	quietLog(t)
	all := crewPrivateSources(t)
	crew, claude := all[1].(*source.KiroCLI).CrewHome, all[0].(*source.Claude).Root
	writeUnder(t, crew, map[string]string{
		"crew-log/sessions/c-log2/log.jsonl": `{"type": "session", "id": "c-log2", "owner": "o", "agent": "a", "slot": null}` + "\n" +
			`{"type": "session/opened", "data": {"slot": "dashboard:chat-1-100", "agent": "a", "cwd": "/w"}}` + "\n",
	})
	writeUnder(t, claude, map[string]string{"-w/c-log2.jsonl": claudeTranscript("SECRET-CLAUDE-OPENED-SLOT")})
	for _, want := range []map[string]bool{{"claude": true}, {"kiro": true, "claude": true}} {
		data, _ := collect(all, want, 15)
		assertPrivate(t, data, "Claude Code", "c-log2")
	}
}

// crew-log の session/opened の data.class.memory（と、あとからの session/class の data.memory）が persistent でない会話は、
// 会話の記録と session_map の行が消えていても、crew-log の会話 ID（と data.previous.sid）の写しを隠す。
// class のない古い crew-log と、persistent のままの crew-log は、そのまま出す。
func TestCollectCrewPrivateCrewLogClass(t *testing.T) {
	quietLog(t)
	all := crewPrivateSources(t)
	crew, claude := all[1].(*source.KiroCLI).CrewHome, all[0].(*source.Claude).Root
	head := func(id, slot string) string {
		return `{"type": "session", "id": "` + id + `", "owner": "o", "agent": "a", "slot": "` + slot + `"}` + "\n"
	}
	opened := func(slot, class string) string {
		return `{"type": "session/opened", "data": {"slot": "` + slot + `", "agent": "a"` + class + `}}` + "\n"
	}
	writeUnder(t, crew, map[string]string{
		// 会話の記録も session_map の行もない会話キー（閉じて消えた）
		"crew-log/sessions/g1/log.jsonl": head("g-incog", "dashboard:chat-9-900") +
			`{"type": "session/opened", "data": {"slot": "dashboard:chat-9-900", "previous": {"sid": "g-prev"}, "class": {"memory": "incognito"}}}` + "\n",
		// persistent で開き、あとで temporary に変わった（後ろのファイルに書かれた）
		"crew-log/sessions/g2/log.jsonl":   head("g-later", "dashboard:chat-8-800") + opened("dashboard:chat-8-800", `, "class": {"memory": "persistent"}`),
		"crew-log/sessions/g2/log.1.jsonl": `{"type": "session/class", "data": {"memory": "temporary"}}` + "\n",
		// 読めない class（object でない）は中身を残さない側に倒す
		"crew-log/sessions/g3/log.jsonl": head("g-bad", "dashboard:chat-7-700") + opened("dashboard:chat-7-700", `, "class": "x"`),
		// persistent のまま、class のない古い crew-log は、そのまま
		"crew-log/sessions/g4/log.jsonl": head("g-pers", "dashboard:chat-6-600") + opened("dashboard:chat-6-600", `, "class": {"memory": " Persistent "}`),
		"crew-log/sessions/g5/log.jsonl": head("g-old", "dashboard:chat-5-500") + opened("dashboard:chat-5-500", ""),
	})
	writeUnder(t, claude, map[string]string{
		"-w/g-incog.jsonl": claudeTranscript("SECRET-CLASS-INCOGNITO"),
		"-w/g-prev.jsonl":  claudeTranscript("SECRET-CLASS-PREVIOUS"),
		"-w/g-later.jsonl": claudeTranscript("SECRET-CLASS-LATER"),
		"-w/g-bad.jsonl":   claudeTranscript("SECRET-CLASS-BAD"),
		"-w/g-pers.jsonl":  claudeTranscript("VISIBLE-CLASS-PERSISTENT"),
		"-w/g-old.jsonl":   claudeTranscript("VISIBLE-CLASS-OLD"),
	})
	for _, want := range []map[string]bool{{"claude": true}, {"kiro": true, "claude": true}} {
		data, _ := collect(all, want, 15)
		assertPrivate(t, data, "Claude Code", "g-incog", "g-prev", "g-later", "g-bad")
		assertVisible(t, data, "Claude Code", "g-pers", "VISIBLE-CLASS-PERSISTENT")
		assertVisible(t, data, "Claude Code", "g-old", "VISIBLE-CLASS-OLD")
	}
}

// Kiro を選んでいないとき、session_map.json が読めない（壊れている）と、どの会話が中身を残さないかを調べきれない。
// 黙って出さず「Data sources」と端末に知らせ、厳しい側に倒して、state.json・crew-log からわかる会話 ID の写しは全部隠す。
// 読めるときは知らせない。
func TestCollectCrewPrivateScanReportsUnreadableMap(t *testing.T) {
	quietLog(t)
	all := crewPrivateSources(t)
	crew, claude := all[1].(*source.KiroCLI).CrewHome, all[0].(*source.Claude).Root
	writeUnder(t, crew, map[string]string{
		// 中身を残す会話のサブエージェント（session_map の flags で中身を残さないことになっていたかもしれない）
		"subagents/a3/state.json": `{"parent_session": "dashboard:chat-3-300", "session_id": "s-pub", "provider": "claude_code"}`,
	})
	writeUnder(t, claude, map[string]string{"-w/s-pub.jsonl": claudeTranscript("SECRET-SUBAGENT-OF-PERSISTENT")})
	want := map[string]bool{"claude": true}
	data, rep := collect(all, want, 15)
	for _, r := range rep {
		if r.Name == crewScanName {
			t.Errorf("読める session_map.json で知らせた: %+v", r)
		}
	}
	assertVisible(t, data, "Claude Code", "s-pub", "SECRET-SUBAGENT-OF-PERSISTENT")

	writeUnder(t, crew, map[string]string{"session_map.json": `{"dashboard:chat-1-100": {"sid": "c-priv"`})
	for _, cache := range []*loadCache{nil, newLoadCache()} {
		data, rep = collectCached(all, want, 15, cache)
		var found *source.Report
		for i, r := range rep {
			if r.Name == crewScanName {
				found = &rep[i]
			}
		}
		if found == nil || found.Error == nil || !strings.Contains(*found.Error, "session_map.json") || found.DetailEn == "" {
			t.Fatalf("読めない session_map.json を知らせない: %+v", rep)
		}
		for _, id := range []string{"s-pub", "s-claude", "c-log"} {
			if s := sessionBy(data, "Claude Code", id); s == nil || s.Title != "Kiro Crew private conversation" {
				t.Errorf("session_map.json が読めないのに、Crew からわかる会話 %s の写しを出した: %+v", id, s)
			}
		}
		assertVisible(t, data, "Claude Code", "c-pub", "VISIBLE-CLAUDE") // Crew の記録からわからない会話は、そのまま
	}
}

// kiroku serve の読み込みのキャッシュは、Crew の場所や session_map.json がシンボリックリンクでも、その先が変わったことに気づく。
// あとから中身を残さないものになった会話（ここでは session_map の flags）の写しを、次の読み直しで隠す
// （Kiro を選んでいるときの Kiro の読み込みのキャッシュも、選んでいないときの調べ直しのキャッシュも）。
func TestCollectCachedCrewPrivateBehindSymlink(t *testing.T) {
	quietLog(t)
	const flagged = `{"dashboard:chat-1-100": {"sid": "c-priv", "discarded_sid": "x-old", "provider": "claude_code", "cwd": "/w"},
 "dashboard:chat-3-300": {"sid": "c-pub", "provider": "claude_code", "cwd": "/w", "flags": {"incognito": true}}}`
	for _, link := range []string{"home", "session_map.json"} {
		for _, want := range []map[string]bool{{"claude": true}, {"kiro": true, "claude": true}} {
			all := crewPrivateSources(t)
			k := all[1].(*source.KiroCLI)
			real := k.CrewHome
			mapFile := filepath.Join(real, "session_map.json")
			switch link {
			case "home": // Crew の場所がリンク
				l := filepath.Join(t.TempDir(), "crew-link")
				if err := os.Symlink(real, l); err != nil {
					t.Fatal(err)
				}
				k.CrewHome = l
			case "session_map.json": // session_map.json だけがリンク（ほかの場所にある）
				other := filepath.Join(t.TempDir(), "map.json")
				if err := os.Rename(mapFile, other); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(other, mapFile); err != nil {
					t.Fatal(err)
				}
				mapFile = other
			}
			cache := newLoadCache()
			data, _ := collectCached(all, want, 15, cache)
			assertVisible(t, data, "Claude Code", "c-pub", "VISIBLE-CLAUDE")
			if err := os.WriteFile(mapFile, []byte(flagged), 0o600); err != nil {
				t.Fatal(err)
			}
			data, _ = collectCached(all, want, 15, cache)
			if s := sessionBy(data, "Claude Code", "c-pub"); s == nil || s.Title != "Kiro Crew private conversation" {
				t.Errorf("%s がリンク（%v）: あとから中身を残さないものになった会話の写しを、読み直しても出したまま: %+v", link, want, s)
			}
		}
	}
}

// 読めなかったファイルのエラー（履歴から来たファイルの場所を含む）を端末に出すときは、制御文字・書式文字を取り除く。
func TestUnreadableWarningIsTerminalSafe(t *testing.T) {
	all := crewPrivateSources(t)
	k := all[1].(*source.KiroCLI)
	// Windows のファイル名には制御文字（ESC）が使えないので、書式文字（U+202E）だけで試す
	name := "crew\x1b[2J\u202e"
	if runtime.GOOS == "windows" {
		name = "crew\u202e"
	}
	evil := filepath.Join(filepath.Dir(k.CrewHome), name)
	if err := os.Rename(k.CrewHome, evil); err != nil {
		t.Fatal(err)
	}
	k.CrewHome = evil
	if err := os.WriteFile(filepath.Join(evil, "session_map.json"), []byte(`[]`), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, want := range []map[string]bool{{"claude": true}, {"kiro": true, "claude": true}} {
		var b strings.Builder
		old := logw
		logw = &b
		_, rep := collect(all, want, 15)
		logw = old
		out := b.String()
		if !strings.Contains(out, "not a JSON object") {
			t.Errorf("%v: 読めなかったことを知らせていない:\n%q", want, out)
		}
		if strings.ContainsAny(out, "\x1b\u202e") {
			t.Errorf("%v: 端末の制御文字をそのまま出した:\n%q", want, out)
		}
		var d strings.Builder
		doctorReport(&d, rep, 1, t.TempDir(), release{}, false)
		// エラーの行だけを見る（場所の行は、利用者が指定した Crew の場所そのもの）
		for _, line := range strings.Split(d.String(), "\n") {
			if strings.Contains(line, "couldn't be read") && strings.ContainsAny(line, "\x1b\u202e") {
				t.Errorf("%v: doctor が読めなかったファイルのエラーの制御文字をそのまま出した:\n%q", want, line)
			}
		}
	}
}
