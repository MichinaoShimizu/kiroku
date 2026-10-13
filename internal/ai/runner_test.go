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
	stubAgent(t, "claude", `cat >/dev/null; echo "$@"; pwd`)
	hist := t.TempDir()
	file := filepath.Join(hist, "data.sqlite3")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	wd, _ := os.Getwd()
	res, err := (Runner{Enabled: true, Dirs: []string{hist, file, filepath.Join(hist, "missing"), "-x"}}).Run(context.Background(), "claude", "hi")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(res.Output), "\n")
	if len(lines) != 2 {
		t.Fatalf("output %q", res.Output)
	}
	if want := "-p --output-format text --strict-mcp-config --tools Read --add-dir " + hist; lines[0] != want {
		t.Errorf("args = %q, want %q (only existing folders, no files or option-like names)", lines[0], want)
	}
	if dir := lines[1]; dir == wd || !strings.Contains(filepath.Base(dir), "kiroku-ai-") {
		t.Errorf("agent ran in %q, want an empty temporary folder", dir)
	} else if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("temporary folder %q was not removed", dir)
	}
}

func TestCodexSkipsGitRepoCheck(t *testing.T) {
	stubAgent(t, "codex", `cat >/dev/null; echo "$@"`)
	res, err := (Runner{Enabled: true}).Run(context.Background(), "codex", "hi")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(res.Output); got != "exec --sandbox read-only --skip-git-repo-check -" {
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
