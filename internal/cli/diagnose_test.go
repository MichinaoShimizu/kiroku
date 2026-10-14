package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/MichinaoShimizu/kiroku/internal/core"
	"github.com/MichinaoShimizu/kiroku/internal/source"
)

// kiroku doctor --report は、種類と数と版だけを書く。履歴にあるパス・プロジェクト名・セッション ID・発言・応答・ホームのフォルダは書かない
// （名前の形でない種類も other にまとめる）。ADR 0032。
func TestDiagnosticsHaveNoHistory(t *testing.T) {
	old := logw
	t.Cleanup(func() { logw = old })
	logw = &bytes.Buffer{}
	tmp := t.TempDir()
	claude := filepath.Join(tmp, "claude")
	proj := filepath.Join(claude, "-Users-alice-acme-secret")
	os.MkdirAll(proj, 0o700)
	claudeLines := []string{
		`{"type":"user","timestamp":"2026-09-30T01:00:00Z","version":"2.1.300","sessionId":"5f0c1d2e-aaaa-bbbb-cccc-0123456789ab","cwd":"/Users/alice/acme-secret","gitBranch":"feature/top-secret","message":{"role":"user","content":"please rotate the password hunter2"}}`,
		`{"type":"assistant","timestamp":"2026-09-30T01:01:00Z","message":{"id":"msg_private","model":"claude-sonnet-5-5","content":[{"type":"text","text":"rotated the confidential key"}]}}`,
		`{"type":"acme-secret","timestamp":"2026-09-30T01:02:00Z"}`,
		`{"type":"/Users/alice/acme-secret/notes.txt","timestamp":"2026-09-30T01:03:00Z"}`,
		`{"type":"please rotate the password hunter2","timestamp":"2026-09-30T01:04:00Z"}`,
		`{"type":"brand-new-kind","timestamp":"2026-09-30T01:05:00Z","version":"/Users/alice"}`,
		`{"half": "line with /Users/alice/acme-secret`,
	}
	os.WriteFile(filepath.Join(proj, "5f0c1d2e-aaaa-bbbb-cccc-0123456789ab.jsonl"), []byte(strings.Join(claudeLines, "\n")+"\n"), 0o600)
	codex := filepath.Join(tmp, "codex")
	day := filepath.Join(codex, "sessions", "2026", "10", "06")
	os.MkdirAll(day, 0o700)
	codexLines := []string{
		`{"timestamp":"2026-10-06T00:00:00.000Z","type":"session_meta","payload":{"id":"thr-acme-secret-42","timestamp":"2026-10-06T00:00:00.000Z","cwd":"/Users/alice/acme-secret","cli_version":"0.50.0"}}`,
		`{"timestamp":"2026-10-06T00:00:01.000Z","type":"event_msg","payload":{"type":"user_message","message":"please rotate the password hunter2"}}`,
		`{"timestamp":"2026-10-06T00:00:02.000Z","type":"event_msg","payload":{"type":"thr-acme-secret-42"}}`,
	}
	os.WriteFile(filepath.Join(day, "rollout-2026-10-06T00-00-00-thr-acme-secret-42.jsonl"), []byte(strings.Join(codexLines, "\n")+"\n"), 0o600)

	all := source.All(source.Options{ClaudeRoot: claude, CodexHome: codex, KiroHome: filepath.Join(tmp, "kiro"), KiroStorages: []string{},
		CrewHome: filepath.Join(tmp, "crew"), KiroCLIDB: filepath.Join(tmp, "none.sqlite3"), AmazonQDB: filepath.Join(tmp, "none2.sqlite3")})
	want := map[string]bool{}
	for _, s := range all {
		want[s.Family()] = true
	}
	_, rep := collect(all, want, 15)
	var b bytes.Buffer
	diagnostics(&b, rep)
	out := b.String()
	for _, w := range []string{"kiroku: ", "os: " + runtime.GOOS + "/" + runtime.GOARCH,
		"Claude Code: sessions 1; version 2.1.300; unreadable lines 1; unknown kinds: other 2, acme-secret 1, brand-new-kind 1",
		"Codex: sessions 1; version 0.50.0; unreadable lines 0; unknown kinds: event_msg:thr-acme-secret-42 1"} {
		if !strings.Contains(out, w) {
			t.Errorf("%q がない:\n%s", w, out)
		}
	}
	// 種類の名前の形をしていれば、プロジェクト名と同じ文字でも名前として出る（上の acme-secret・thr-acme-secret-42）。
	// それ以外の、パス・ID・発言・応答・ブランチ・ホームは出さない
	for _, bad := range []string{"/Users", "alice", "5f0c1d2e", "hunter2", "password", "confidential", "top-secret", "feature/", "msg_private", tmp, claude, codex} {
		if strings.Contains(out, bad) {
			t.Errorf("--report に履歴の中身 %q が出ている:\n%s", bad, out)
		}
	}
	if h, err := os.UserHomeDir(); err == nil && h != "" && strings.Contains(out, h) {
		t.Errorf("--report にホームのフォルダが出ている:\n%s", out)
	}
	// 「Format check」も同じものを出し、知らない種類があれば --report を案内する
	b.Reset()
	formatCheck(&b, rep)
	if out := b.String(); !strings.Contains(out, "! Claude Code") || !strings.Contains(out, "newest version seen 2.1.300") ||
		!strings.Contains(out, "1 unreadable line") || !strings.Contains(out, `kiroku doctor --report`) {
		t.Errorf("Format check:\n%s", out)
	}
}

// 「Format check」: 知らない種類がなければ ✓、数えていないエージェントはそう書き、--report を案内しない。
func TestFormatCheckQuiet(t *testing.T) {
	rep := []source.Report{
		{Name: "Claude Code", N: 3, AgentVersion: "2.1.300", Checked: true},
		{Name: "Kiro IDE / CLI", N: 2},
		{Name: "Amazon Q"},
	}
	var b bytes.Buffer
	formatCheck(&b, rep)
	out := b.String()
	if !strings.Contains(out, "✓ Claude Code") || !strings.Contains(out, "no unknown kinds") || !strings.Contains(out, "format not checked") ||
		strings.Contains(out, "Amazon Q") || strings.Contains(out, "--report") {
		t.Errorf("Format check:\n%s", out)
	}
	b.Reset()
	diagnostics(&b, rep)
	if out := b.String(); !strings.Contains(out, "Claude Code: sessions 3; version 2.1.300; unreadable lines 0; unknown kinds: none") ||
		!strings.Contains(out, "Kiro IDE / CLI: sessions 2; format not checked") || strings.Contains(out, "Amazon Q") {
		t.Errorf("--report:\n%s", out)
	}
	// 万一、名前の形でない種類や版の形でないものが Report に入っていても、書くときにもう一度確かめる
	b.Reset()
	diagnostics(&b, []source.Report{{Name: "Codex", N: 1, AgentVersion: "/home/x", Unknown: map[string]int{"/home/x": 2, core.OtherKind: 1}}})
	if out := b.String(); strings.Contains(out, "/home/x") || !strings.Contains(out, "unknown kinds: other 3") {
		t.Errorf("--report:\n%s", out)
	}
}
