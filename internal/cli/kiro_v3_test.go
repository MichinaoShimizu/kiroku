package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MichinaoShimizu/kiroku/internal/core"
	"github.com/MichinaoShimizu/kiroku/internal/source"
)

// Kiro CLI V2（Classic）の会話を 3.0 へ移した写し（sessions/<hash>/sess_<id>）の、まわりの場合。

func classicMeta(id string, end string, credits float64) string {
	return fmt.Sprintf(`{"session_id": %q, "cwd": "/Users/me/app", "created_at": "2026-09-29T10:00:00Z", "session_state": {"conversation_metadata": {"user_turn_metadatas": [{"end_timestamp": %q, "metering_usage": [{"value": %g, "unit": "credit"}]}]}}}`, id, end, credits)
}

func v3Prompt(at, text string) string {
	return `{"timestamp": "` + at + `", "payload": {"type": "user", "content": "` + text + `"}}` + "\n"
}

func loadKiro(t *testing.T, home string) []*core.Session {
	t.Helper()
	all := source.All(source.Options{KiroHome: home, CrewHome: filepath.Join(home, "crew"), KiroCLIDB: filepath.Join(home, "none.sqlite3")})
	data, _ := collect(all, map[string]bool{"kiro": true}, 15)
	return data
}

func totalCredits(data []*core.Session) float64 {
	sum := 0.0
	for _, s := range data {
		sum += s.Credits
	}
	return sum
}

// Crew のサブエージェントを持つ会話を 3.0 へ移しても、サブエージェントのクレジットは消えない
// （親は写しに譲るので、サブエージェントは親にまとめずに出す）。
func TestKiroClassicConvertedKeepsCrewSubagents(t *testing.T) {
	home := t.TempDir()
	writeTree(t, home, map[string]string{
		"sessions/cli/p1.json":               classicMeta("p1", "2026-09-29T10:05:00Z", 1),
		"sessions/cli/p1.jsonl":              "",
		"sessions/cli/sa1.json":              classicMeta("sa1", "2026-09-29T10:03:00Z", 2),
		"sessions/cli/sa1.jsonl":             `{"version": "v1", "kind": "Prompt", "data": {"content": [{"kind": "text", "data": "PR を見て"}], "meta": {"timestamp": 1790676060}}}` + "\n",
		"crew/session_map.json":              `{"dashboard:chat-1-100": {"sid": "p1", "cwd": "/Users/me/app"}}`,
		"crew/subagents/keep1/state.json":    `{"id": "keep1", "agent": "reviewer", "task": "PR を見る", "parent_session": "dashboard:chat-1-100", "session_id": "sa1", "conversation_key": "subagent:keep1"}`,
		"sessions/b1/sess_p1/session.json":   `{"id": "sess_p1"}`,
		"sessions/b1/sess_p1/messages.jsonl": v3Prompt("2026-09-29T12:00:00Z", "続き"),
	})
	if got := totalCredits(loadKiro(t, home)); got != 3 {
		t.Errorf("クレジットの合計 = %v, want 3（p1 の 1 は 3.0 の写しへ、sa1 の 2 はそのまま）", got)
	}
}

// 同じ id の写しが 2 つ（別の作業場所の bucket）あっても、V2 のクレジットは 1 回だけ足す。
func TestKiroClassicCreditsAddedOnce(t *testing.T) {
	home := t.TempDir()
	writeTree(t, home, map[string]string{
		"sessions/cli/u1.json":               classicMeta("u1", "2026-09-29T10:01:00Z", 10),
		"sessions/b1/sess_u1/session.json":   `{"id": "sess_u1"}`,
		"sessions/b1/sess_u1/messages.jsonl": v3Prompt("2026-09-29T11:00:00Z", "一"),
		"sessions/b2/sess_u1/session.json":   `{"id": "sess_u1"}`,
		"sessions/b2/sess_u1/messages.jsonl": v3Prompt("2026-09-29T11:00:00Z", "二"),
	})
	if got := totalCredits(loadKiro(t, home)); got != 10 {
		t.Errorf("クレジットの合計 = %v, want 10", got)
	}
}

// Crew の incognito の会話は、V2 の会話が消えたあとも、3.0 の写しの中身を出さない（sessions/cli がない場合も）。
func TestKiroClassicPrivateCopyWithheldWithoutClassic(t *testing.T) {
	for _, withCLI := range []bool{true, false} {
		home := t.TempDir()
		files := map[string]string{
			"crew/session_map.json":              `{"dashboard:chat-1-100": {"sid": "p1", "cwd": "/Users/me/app", "flags": {"incognito": true}}}`,
			"sessions/b1/sess_p1/session.json":   `{"id": "sess_p1", "title": "SECRET-TITLE"}`,
			"sessions/b1/sess_p1/messages.jsonl": v3Prompt("2026-09-29T10:00:30Z", "SECRET-PROMPT"),
		}
		if withCLI {
			files["sessions/cli/other.json"] = classicMeta("other", "2026-09-29T10:05:00Z", 1)
		}
		writeTree(t, home, files)
		b, err := json.Marshal(loadKiro(t, home))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(b), "SECRET") {
			t.Errorf("sessions/cli あり=%v: 3.0 の写しの中身が出ている", withCLI)
		}
	}
}

// 写しの id から V2 の会話を探すとき、ほかの場所のファイルは読まない。
func TestKiroClassicCopyIDTraversal(t *testing.T) {
	for _, id := range []string{"sess_../secret", "sess_../../secret", "sess_..", "sess_", `sess_..\\secret`, "sess_C:secret", `sess_secret\u0000`} {
		home := t.TempDir()
		meta, _ := json.Marshal(map[string]string{"id": id})
		writeTree(t, home, map[string]string{
			"secret.json":                  classicMeta("secret", "2026-09-29T10:01:00Z", 7),
			"sessions/secret.json":         classicMeta("secret", "2026-09-29T10:01:00Z", 7),
			"sessions/b1/x/session.json":   string(meta),
			"sessions/b1/x/messages.jsonl": v3Prompt("2026-09-29T11:00:00Z", "依頼"),
		})
		if got := totalCredits(loadKiro(t, home)); got != 0 {
			t.Errorf("id %q: クレジット = %v, want 0（ほかの場所のファイルを読んだ）", id, got)
		}
	}
}
