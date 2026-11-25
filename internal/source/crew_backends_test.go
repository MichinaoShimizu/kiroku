package source

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/MichinaoShimizu/kiroku/internal/core"
)

// dedupeYield は、読み込む側（internal/cli の load.go）と同じく、Yield の鍵を Claim したセッションがあれば、その Yield のセッションを落とす。
func dedupeYield(bs []*core.Builder) []*core.Builder {
	claimed := map[string]bool{}
	for _, b := range bs {
		if b.Claim != "" {
			claimed[b.Claim] = true
		}
	}
	var out []*core.Builder
	for _, b := range bs {
		if b.Yield != "" && claimed[b.Yield] {
			continue
		}
		out = append(out, b)
	}
	return out
}

// kasSession は KAS の engine が書く会話（Kiro IDE / CLI (V3) と同じ形の session.json と messages.jsonl）。credits は 1 ターンのクレジット。
func kasSession(sid, cwd, prompt string, credits float64) map[string]string {
	dir := "sessions/0f1e2d3c/" + sid + "/"
	return map[string]string{
		dir + "session.json": fmt.Sprintf(`{"id": %q, "title": "kas", "workspacePaths": [%q], "createdAt": "2026-11-24T10:00:00Z", "modelId": "claude-sonnet-4.5"}`, sid, cwd),
		dir + "messages.jsonl": strings.Join([]string{
			fmt.Sprintf(`{"timestamp": "2026-11-24T10:00:01Z", "payload": {"type": "user", "content": %q}}`, prompt),
			`{"timestamp": "2026-11-24T10:00:30Z", "payload": {"type": "assistant", "content": "done"}}`,
			fmt.Sprintf(`{"timestamp": "2026-11-24T10:00:31Z", "payload": {"type": "usage_summary", "promptTurnSummaries": [{"usage": %g, "unit": "credit"}]}}`, credits),
		}, "\n") + "\n",
	}
}

// Kiro Crew 0.14.0 は Kiro Agent を KAS の engine で動かす（session_map の provider "kas"、sid は sess_<UUID>）。
// KAS は会話を <Kiro の場所>/sessions/<bucket>/sess_<UUID>/ に書き、kiroku は Kiro IDE / CLI (V3) としてクレジットごと読む。
// Crew も同じターンのクレジットを使用量の行（provider "acp"、harness "kas"）に書くので、Crew のセッションは V3 のセッションに譲る（2 度数えない）。
// 中身を残さない KAS の会話は、V3 のセッションも隠す（WithheldClaims に "kiro-ide:" + sid。"kiro-ide:sess_sess_…" ではない）。
func TestKiroCrewKASYieldsToV3(t *testing.T) {
	kiro, crew := t.TempDir(), t.TempDir()
	const (
		pub  = "sess_0b7c1e2a-1111-4c1d-9a0e-00000000a001"
		priv = "sess_0b7c1e2a-2222-4c1d-9a0e-00000000a002"
	)
	writeFiles(t, kiro, mergeFiles(map[string]string{"sessions/cli/.keep": ""},
		kasSession(pub, "/w", "Fix the parser", 2), kasSession(priv, "/w", "SECRET prompt", 3)))
	row := func(slot, sid string, credits float64) string {
		return fmt.Sprintf(`{"_type": "tokens", "ts": "2026-11-24T10:00:31+00:00", "slot": %q, "provider": "acp", "harness": "kas", "acp_session_id": %q, "model": "claude-sonnet-4.5", "credits": %g, "duration_ms": 30000}`, slot, sid, credits)
	}
	writeFiles(t, crew, map[string]string{
		"session_map.json": `{"dashboard:chat-1-100": {"sid": "` + pub + `", "provider": "kas", "cwd": "/w"},
 "dashboard:chat-2-200": {"sid": "` + priv + `", "provider": "kas", "cwd": "/w", "flags": {"incognito": true}}}`,
		"usage/tokens/2026-11-24.jsonl": row("dashboard:chat-1-100", pub, 2) + "\n" + row("dashboard:chat-2-200", priv, 3) + "\n",
		"sessions/dashboard_chat-1-100.jsonl": `{"_type": "metadata", "title": "KAS で直す"}` + "\n" +
			`{"role": "user", "content": "Fix the parser", "ts": "2026-11-24T10:00:01+00:00", "meta": {"human": true}}` + "\n",
	})
	v3 := load(t, &KiroIDE{Home: kiro})
	if len(v3) != 2 {
		t.Fatalf("V3 sessions = %d, want 2", len(v3))
	}
	k := &KiroCLI{Home: kiro, CrewHome: crew}
	cb := load(t, k)
	by := map[string]*core.Builder{}
	for _, b := range cb {
		by[b.ID] = b
	}
	for id, want := range map[string]string{
		"crew:dashboard:chat-1-100": "kiro-ide:" + pub,
		"crew:dashboard:chat-2-200": "kiro-ide:" + priv,
	} {
		if b := by[id]; b == nil || b.Yield != want {
			t.Errorf("%s = %+v, want Yield %q", id, b, want)
		}
	}
	credits := 0.0
	for _, b := range dedupeYield(append(v3, cb...)) {
		credits += sumCredits(b.Credits)
	}
	if math.Abs(credits-5) > 1e-9 {
		t.Errorf("credits = %g, want 5 (each KAS turn once)", credits)
	}
	if !slices.Contains(k.WithheldClaims(), "kiro-ide:"+priv) {
		t.Errorf("WithheldClaims = %v, want the V3 copy of the private KAS conversation", k.WithheldClaims())
	}
	if slices.Contains(k.WithheldClaims(), "kiro-ide:"+pub) {
		t.Errorf("WithheldClaims hides the persistent KAS conversation: %v", k.WithheldClaims())
	}
	claims, err := k.ScanWithheldClaims()
	if err != nil || !slices.Contains(claims, "kiro-ide:"+priv) {
		t.Errorf("ScanWithheldClaims = %v, %v", claims, err)
	}
	for _, b := range v3 {
		if b.Claim == "kiro-ide:"+priv && !slices.Contains(k.WithheldClaims(), b.Claim) {
			t.Errorf("the private KAS conversation's V3 session %q is not withheld", b.Claim)
		}
	}
}

// 1 つの会話キーで kiro-cli の会話が 2 つ重なって動いた（前の会話 k-a のあいだに今の会話 k-b が始まった）とき、
// 時刻では分けられないので、acp_session_id（新しい Crew が書く、そのターンの ACP の会話 ID）があればそれで分ける。
// acp_session_id のない行（それを書く前の Crew）は、これまでどおり時刻で分ける。
func TestKiroCrewUsageByACPSession(t *testing.T) {
	kiro, crew := t.TempDir(), t.TempDir()
	writeFiles(t, kiro, map[string]string{
		"sessions/cli/k-a.json": kiroCLISession("k-a", "/w", "2026-11-24T10:00:00Z", "2026-11-24T12:00:00Z", [2]string{"2026-11-24T10:05:00Z", ""}, [2]string{"2026-11-24T12:00:00Z", ""}),
		"sessions/cli/k-b.json": kiroCLISession("k-b", "/w", "2026-11-24T10:30:00Z", "2026-11-24T11:30:00Z", [2]string{"2026-11-24T10:35:00Z", ""}, [2]string{"2026-11-24T11:30:00Z", ""}),
	})
	row := func(ts, acp string, credits float64) string {
		a := ""
		if acp != "" {
			a = fmt.Sprintf(`, "acp_session_id": %q`, acp)
		}
		return fmt.Sprintf(`{"_type": "tokens", "ts": %q, "slot": "dashboard:chat-3-300", "provider": "acp", "harness": "kiro"%s, "model": "auto", "credits": %g}`, ts, a, credits)
	}
	writeFiles(t, crew, map[string]string{
		"session_map.json": `{"dashboard:chat-3-300": {"sid": "k-b", "discarded_sid": "k-a", "cwd": "/w"}}`,
		"usage/tokens/2026-11-24.jsonl": strings.Join([]string{
			row("2026-11-24T11:00:00+00:00", "k-a", 1),
			row("2026-11-24T11:10:00+00:00", "k-b", 2),
			row("2026-11-24T11:20:00+00:00", "k-a", 4),
			row("2026-11-24T11:25:00+00:00", "", 8), // 時刻で今の会話（k-b）
		}, "\n") + "\n",
	})
	got := map[string]float64{}
	for _, b := range load(t, &KiroCLI{Home: kiro, CrewHome: crew}) {
		got[b.ID] += sumCredits(b.Credits)
	}
	want := map[string]float64{"k-a": 5, "k-b": 10}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("credits by session = %v, want %v", got, want)
	}
}

// acp_session_id は、session_map から消えた前の kiro-cli の会話（Crew の目印のない会話）を見つけるのにも使う。
// 時刻と作業場所では 2 つの会話のどちらか決められなくても、会話 ID が同じ会話に結びつける。
func TestKiroCrewFormerByACPSession(t *testing.T) {
	kiro, crew := t.TempDir(), t.TempDir()
	writeFiles(t, kiro, map[string]string{
		"sessions/cli/k-x.json":   kiroCLISession("k-x", "/w", "2026-11-24T09:00:00Z", "2026-11-24T09:30:00Z", [2]string{"2026-11-24T09:30:00Z", ""}),
		"sessions/cli/k-y.json":   kiroCLISession("k-y", "/w", "2026-11-24T09:00:00Z", "2026-11-24T09:30:00Z", [2]string{"2026-11-24T09:30:00Z", ""}),
		"sessions/cli/k-now.json": kiroCLISession("k-now", "/w", "2026-11-24T10:00:00Z", "2026-11-24T10:30:00Z", [2]string{"2026-11-24T10:30:00Z", ""}),
	})
	writeFiles(t, crew, map[string]string{
		"session_map.json":              `{"dashboard:chat-4-400": {"sid": "k-now", "cwd": "/w"}}`,
		"usage/tokens/2026-11-24.jsonl": `{"_type": "tokens", "ts": "2026-11-24T09:20:00+00:00", "slot": "dashboard:chat-4-400", "provider": "acp", "acp_session_id": "k-y", "model": "auto", "credits": 3}` + "\n",
	})
	got := map[string]float64{}
	crewOf := map[string]bool{}
	for _, b := range load(t, &KiroCLI{Home: kiro, CrewHome: crew}) {
		got[b.ID] += sumCredits(b.Credits)
		crewOf[b.ID] = b.Source == "Kiro Crew"
	}
	if got["k-y"] != 3 || got["k-x"] != 0 || !crewOf["k-y"] || crewOf["k-x"] {
		t.Errorf("credits = %v crew = %v, want the row on k-y (a former conversation of the key) only", got, crewOf)
	}
	if _, ok := got["crew:dashboard:chat-4-400"]; ok {
		t.Errorf("the row became a Crew session: %v", got)
	}
}

// Claude Code の backend のサブエージェント（行は provider "claude_code"。subagent_manager/usage_row.py）も、Crew が state.json を消したあと、
// crew-log の child_sid があれば、その Claude Code のセッション（claude-agent-acp は ACP の会話 ID を Claude Code のセッション ID にする）に譲る。
// harness を書く前の行（provider だけ）でも、harness "claude_code" の行でも同じ。
func TestKiroCrewPrunedClaudeSubagentYieldsByChildSid(t *testing.T) {
	kiro, crew := t.TempDir(), t.TempDir()
	const (
		child1 = "7c1d0e9a-0000-4000-8000-00000000cc01"
		child2 = "7c1d0e9a-0000-4000-8000-00000000cc02"
	)
	writeFiles(t, kiro, map[string]string{"sessions/cli/.keep": ""})
	writeFiles(t, crew, map[string]string{
		"session_map.json": `{"dashboard:chat-9-900": {"sid": "", "cwd": "/w"}}`,
		"sessions/dashboard_chat-9-900.jsonl": `{"_type": "metadata", "title": "親の会話"}` + "\n" +
			`{"role": "user", "content": "任せて", "ts": "2026-11-24T00:00:00+00:00", "meta": {"human": true}}` + "\n",
		"crew-log/sessions/c1/log.jsonl": strings.Join([]string{
			`{"type": "session", "id": "parent-acp", "slot": "dashboard:chat-9-900", "cwd": "/w"}`,
			`{"type": "subagent/spawned", "data": {"agent_id": "cc1", "agent": "coder", "task": "直す"}}`,
			`{"type": "subagent/spawned", "data": {"agent_id": "cc2", "agent": "coder", "task": "調べる"}}`,
			`{"type": "subagent/completed", "data": {"agent_id": "cc1", "child_sid": "` + child1 + `"}}`,
			`{"type": "subagent/completed", "data": {"agent_id": "cc2", "child_sid": "` + child2 + `"}}`,
		}, "\n") + "\n",
		"usage/tokens/2026-11-24.jsonl": strings.Join([]string{
			`{"_type": "tokens", "ts": "2026-11-24T00:01:00+00:00", "slot": "subagent:cc1", "provider": "claude_code", "model": "claude-sonnet-4-5", "input": 1000, "output": 100, "cost": 0.5, "credits": 0}`,
			`{"_type": "tokens", "ts": "2026-11-24T00:02:00+00:00", "slot": "subagent:cc2", "provider": "claude_code", "harness": "claude_code", "model": "claude-sonnet-4-5", "input": 1000, "output": 100, "cost": 0.5, "credits": 0}`,
		}, "\n") + "\n",
	})
	by := map[string]*core.Builder{}
	for _, b := range load(t, &KiroCLI{Home: kiro, CrewHome: crew}) {
		by[b.ID] = b
	}
	for id, child := range map[string]string{"crew:subagent:cc1": child1, "crew:subagent:cc2": child2} {
		sub := by[id]
		if sub == nil {
			t.Errorf("%s was folded into its parent (sessions = %v)", id, keysOf(by))
			continue
		}
		if sub.Yield != claudeClaim(child) {
			t.Errorf("%s Yield = %q, want %q", id, sub.Yield, claudeClaim(child))
		}
	}
}

// 行の harness（新しい Crew）があれば、トークンとドル額を別の履歴が記録しているかはそれで決める。
func TestCrewElsewhereHarness(t *testing.T) {
	for _, c := range []struct {
		harness, label string
		want           bool
	}{
		{"codex", "", true},
		{"claude", "", true},
		{"claude_code", "", true},
		{"kiro", "codex", false}, // そのターンは kiro-cli（会話キーはあとで Codex に切りかえた）
		{"goose", "", false},
		{"kas", "", false}, // V3 の記録にはトークンがない
		{"unknown", "codex", true},
	} {
		x := crewTurn{provider: "acp", harness: c.harness, model: "glm-5", u: core.Tokens{In: 10}}
		if got := crewElsewhere(x, c.label); got != c.want {
			t.Errorf("crewElsewhere(harness %q, label %q) = %v, want %v", c.harness, c.label, got, c.want)
		}
	}
}
