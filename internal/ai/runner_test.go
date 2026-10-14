package ai

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestDisabled(t *testing.T) {
	_, err := (Runner{}).Run(context.Background(), "claude", "hello")
	if !errors.Is(err, ErrDisabled) {
		t.Fatalf("expected disabled, got %v", err)
	}
}

func TestUnsupportedAgent(t *testing.T) {
	_, err := (Runner{Enabled: true}).Run(context.Background(), "shell", "hello")
	if !errors.Is(err, ErrUnsupportedAgent) {
		t.Fatalf("expected unsupported, got %v", err)
	}
}

func TestEmptyPrompt(t *testing.T) {
	_, err := (Runner{Enabled: true}).Run(context.Background(), "claude", " ")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestOversizedPrompt(t *testing.T) {
	_, err := (Runner{Enabled: true}).Run(context.Background(), "claude", strings.Repeat("x", 256*1024+1))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLimitedWriter(t *testing.T) {
	var b bytes.Buffer
	w := &limitedWriter{w: &b, max: 3}
	n, err := w.Write([]byte("abcdef"))
	if err == nil || n != 3 || b.String() != "abc" {
		t.Fatalf("n=%d err=%v output=%q", n, err, b.String())
	}
	n, err = w.Write([]byte("z"))
	if err == nil || n != 0 {
		t.Fatalf("expected full writer error, n=%d err=%v", n, err)
	}
}

func TestCancellationBeforeExecution(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := (Runner{Enabled: true}).Run(ctx, "claude", "hello")
	if err == nil {
		t.Fatal("expected canceled or missing binary error")
	}
}

func TestKiroCancellationBeforeExecution(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := (Runner{Enabled: true}).Run(ctx, "kiro", "hello")
	if err == nil {
		t.Fatal("expected canceled or missing binary error")
	}
	if errors.Is(err, ErrUnsupportedAgent) {
		t.Fatal("Kiro must be recognized as a supported agent")
	}
}

// stubAgent puts a fake agent on PATH that prints its arguments and working folder, or fails with a message.
func stubAgent(t *testing.T, name, script string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell stubs need a Unix shell")
	}
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\n"+script+"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestClaudeReadsOnlyHistoryDirs(t *testing.T) {
	stubAgent(t, "claude", `cat >/dev/null; printf '{"type":"result","result":"%s"}\n' "$(pwd)"`)
	hist := t.TempDir()
	file := filepath.Join(hist, "data.sqlite3")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	wd, _ := os.Getwd()
	var argv []string
	res, err := (Runner{Enabled: true, Dirs: []string{hist, file, filepath.Join(hist, "missing"), "-x"}}).Stream(context.Background(), "claude", "hi", func(e Event) {
		if e.T == "cmd" {
			argv = e.Argv
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(argv, " "), "claude -p --output-format stream-json --verbose --include-partial-messages --strict-mcp-config --tools Read --add-dir "+hist; got != want {
		t.Errorf("command = %q, want %q (only existing folders, no files or option-like names)", got, want)
	}
	if dir := res.Output; dir == wd || !strings.Contains(filepath.Base(dir), "kiroku-ai-") {
		t.Errorf("agent ran in %q, want an empty temporary folder", dir)
	} else if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("temporary folder %q was not removed", dir)
	}
}

// Claude Code's stream-json: the reply as it is written, each file it reads, and the final reply as the result.
// Other events and lines that aren't JSON are skipped.
func TestClaudeStreamsReplyAndTools(t *testing.T) {
	stubAgent(t, "claude", `cat >/dev/null
echo '{"type":"system","subtype":"init"}'
echo 'not json'
echo '{"type":"stream_event","event":{"type":"message_start"}}'
echo '{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"text_delta","text":"Reading "}}}'
echo '{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"text_delta","text":"the \u001b[31mfile"}}}'
echo '{"type":"assistant","message":{"content":[{"type":"text","text":"Reading the file"},{"type":"tool_use","name":"Read","input":{"file_path":"/h/a.jsonl"}}]}}'
echo '{"type":"user","message":{"content":[{"type":"tool_result","content":"secret history"}]}}'
echo '{"type":"assistant","message":{"content":[{"type":"text","text":"# Report"}]}}'
echo '{"type":"result","result":"# Report"}'`)
	var got []string
	res, err := (Runner{Enabled: true}).Stream(context.Background(), "claude", "hi", func(e Event) {
		if e.T != "cmd" {
			got = append(got, e.T+":"+e.S)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"text:Reading ", "text:the file", "tool:Read /h/a.jsonl", "text:\n\n", "text:# Report", "text:\n\n"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("events = %q, want %q (escape sequences removed, tool results not shown)", got, want)
	}
	if res.Output != "# Report" {
		t.Errorf("result = %q, want the final reply", res.Output)
	}
}

// An error result from Claude Code fails the run with its message.
func TestClaudeErrorResult(t *testing.T) {
	stubAgent(t, "claude", `cat >/dev/null; echo '{"type":"result","is_error":true,"result":"Not logged in"}'`)
	_, err := (Runner{Enabled: true}).Run(context.Background(), "claude", "hi")
	if err == nil || !strings.Contains(err.Error(), "Not logged in") {
		t.Fatalf("err = %v, want the agent's message", err)
	}
}

// Codex and Kiro CLI: stderr (progress) and stdout (the reply) are sent as they arrive.
func TestStreamsStdoutAndStderr(t *testing.T) {
	stubAgent(t, "codex", `cat >/dev/null; echo "exec ls" >&2; sleep 0.2; printf 'line 1\n'; sleep 0.2; printf 'line 2\n'`)
	var got []string
	res, err := (Runner{Enabled: true}).Stream(context.Background(), "codex", "hi", func(e Event) {
		if e.T != "cmd" {
			got = append(got, e.T+":"+e.S)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, "|") != "log:exec ls\n|text:line 1\n|text:line 2\n" {
		t.Errorf("events = %q", got)
	}
	if res.Output != "line 1\nline 2" {
		t.Errorf("result = %q", res.Output)
	}
}

// Output past the limit stops the agent and fails the run.
func TestOutputLimit(t *testing.T) {
	stubAgent(t, "codex", `cat >/dev/null; head -c 2000000 /dev/zero | tr '\0' 'x'; echo`)
	_, err := (Runner{Enabled: true}).Run(context.Background(), "codex", "hi")
	if err == nil || !strings.Contains(err.Error(), "size limit") {
		t.Fatalf("err = %v, want the size limit", err)
	}
}

func TestClean(t *testing.T) {
	if got := clean("a\x1b[1mb\x1b]0;title\x07c\rd\te\u0085f\n"); got != "abcd\tef\n" {
		t.Errorf("clean = %q", got)
	}
}

func TestCodexSkipsGitRepoCheck(t *testing.T) {
	stubAgent(t, "codex", `cat >/dev/null; echo "$@"`)
	res, err := (Runner{Enabled: true}).Run(context.Background(), "codex", "hi")
	if err != nil {
		t.Fatal(err)
	}
	if got := res.Output; got != "exec --sandbox read-only --skip-git-repo-check -" {
		t.Errorf("args = %q", got)
	}
}

func TestAgentErrorIncludesStderr(t *testing.T) {
	stubAgent(t, "codex", `cat >/dev/null; echo "Not logged in" >&2; exit 3`)
	_, err := (Runner{Enabled: true}).Run(context.Background(), "codex", "hi")
	if err == nil || !strings.Contains(err.Error(), "Not logged in") {
		t.Fatalf("err = %v, want the agent's message", err)
	}
}

func TestTailWriter(t *testing.T) {
	var b bytes.Buffer
	w := &tailWriter{w: &b, max: 5}
	for _, s := range []string{"abc", "defg", "0123456789"} {
		if n, err := w.Write([]byte(s)); err != nil || n != len(s) {
			t.Fatalf("Write(%q) = %d, %v", s, n, err)
		}
	}
	if b.String() != "56789" {
		t.Errorf("kept %q, want the last 5 bytes", b.String())
	}
	b.Reset()
	w.Write([]byte("ab"))
	w.Write([]byte("cdef"))
	if b.String() != "bcdef" {
		t.Errorf("kept %q", b.String())
	}
}

// A process the agent started that keeps running (and holds the agent's output open) doesn't keep the run
// going past its limit: the run ends shortly after it is stopped, and on Unix the process is killed too.
func TestStoppedRunEndsWithChildren(t *testing.T) {
	stubAgent(t, "codex", `cat >/dev/null; echo start; sleep 30 & sleep 31`)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	began := time.Now()
	if _, err := (Runner{Enabled: true}).Run(ctx, "codex", "hi"); err == nil {
		t.Fatal("want an error for the stopped run")
	}
	if d := time.Since(began); d > 8*time.Second {
		t.Errorf("run took %v after being stopped at 1s", d)
	}
}

// Claude's final result is held to the same limit as the rest of the output.
func TestClaudeResultLimit(t *testing.T) {
	stubAgent(t, "claude", `cat >/dev/null; printf '{"type":"result","result":"'; head -c 1100000 /dev/zero | tr '\0' 'x'; printf '"}\n'`)
	if _, err := (Runner{Enabled: true}).Run(context.Background(), "claude", "hi"); err == nil || !strings.Contains(err.Error(), "size limit") {
		t.Fatalf("err = %v, want the size limit", err)
	}
}

// A character cut at the 4 KiB piece boundary is kept whole; direction overrides and invisible characters
// are removed.
func TestForwardKeepsCharactersWhole(t *testing.T) {
	var got []string
	forward(strings.NewReader(strings.Repeat("あ", 2000)+"\n"), func(s string) { got = append(got, s) })
	all := strings.Join(got, "")
	if all != strings.Repeat("あ", 2000)+"\n" || len(got) < 2 {
		t.Fatalf("pieces %d, joined ok=%v", len(got), all == strings.Repeat("あ", 2000)+"\n")
	}
	for _, p := range got {
		if !utf8.ValidString(p) {
			t.Errorf("piece with a split character: %q…", p[:10])
		}
	}
	if s := clean("a\u202eb\u2066c\u200bd\ufeffe\u200df"); s != "abcde\u200df" {
		t.Errorf("clean = %q", s)
	}
	if c := cut("aあ", 2); c != "a" {
		t.Errorf("cut = %q", c)
	}
}
