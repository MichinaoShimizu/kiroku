// Package ai provides an explicitly opted-in execution boundary for local AI agents.
package ai

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

var ErrDisabled = errors.New("AI execution is disabled")
var ErrUnsupportedAgent = errors.New("unsupported AI agent")

type Runner struct {
	Enabled bool
	Timeout time.Duration
	// Dirs are the history folders the prompts point to. Claude Code may read only these
	// (--add-dir with only the Read tool); the agent itself runs in an empty temporary folder.
	Dirs []string
}

type Result struct {
	Output string
	Agent  string
}

func (r Runner) Run(ctx context.Context, agent, prompt string) (Result, error) {
	if !r.Enabled {
		return Result{}, ErrDisabled
	}
	if strings.TrimSpace(prompt) == "" {
		return Result{}, errors.New("empty prompt")
	}
	if len(prompt) > 256*1024 {
		return Result{}, errors.New("prompt exceeds size limit")
	}
	var binary string
	var args []string
	switch agent {
	case "claude":
		binary = "claude"
		// Only Read, and only inside the history folders: reads elsewhere need approval, which -p denies.
		args = []string{"-p", "--output-format", "text", "--strict-mcp-config", "--tools", "Read"}
		for _, d := range r.Dirs {
			if fi, err := os.Stat(d); err == nil && fi.IsDir() && !strings.HasPrefix(d, "-") {
				args = append(args, "--add-dir", d)
			}
		}
	case "codex":
		binary = "codex"
		// The temporary folder is not a git repository, which codex exec otherwise refuses.
		args = []string{"exec", "--sandbox", "read-only", "--skip-git-repo-check", "-"}
	case "kiro":
		binary = "kiro-cli"
		args = []string{"chat", "--no-interactive"}
	default:
		return Result{}, ErrUnsupportedAgent
	}
	timeout := r.Timeout
	if timeout <= 0 || timeout > 5*time.Minute {
		timeout = 2 * time.Minute
	}
	bounded, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	// An empty folder of its own, so the agent doesn't pick up the project instructions, settings or
	// repository of wherever kiroku serve was started.
	dir, err := os.MkdirTemp("", "kiroku-ai-")
	if err != nil {
		return Result{}, err
	}
	defer os.RemoveAll(dir)
	cmd := exec.CommandContext(bounded, binary, args...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(prompt)
	cmd.WaitDelay = 2 * time.Second
	var buf, ebuf bytes.Buffer
	cmd.Stdout = &limitedWriter{w: &buf, max: 1024 * 1024}
	cmd.Stderr = &tailWriter{w: &ebuf, max: 2048}
	err = cmd.Run()
	output := buf.Bytes()
	if len(output) >= 1024*1024 {
		return Result{}, errors.New("agent output exceeds size limit")
	}
	if err != nil {
		if msg := strings.TrimSpace(ebuf.String()); msg != "" {
			return Result{}, fmt.Errorf("agent %s failed: %w: %s", agent, err, msg)
		}
		return Result{}, fmt.Errorf("agent %s failed: %w", agent, err)
	}
	return Result{Output: string(output), Agent: agent}, nil
}

type limitedWriter struct {
	w   *bytes.Buffer
	max int
}

func (w *limitedWriter) Write(p []byte) (int, error) {
	remaining := w.max - w.w.Len()
	if remaining <= 0 {
		return 0, errors.New("agent output limit reached")
	}
	if len(p) > remaining {
		_, _ = w.w.Write(p[:remaining])
		return remaining, errors.New("agent output limit reached")
	}
	return w.w.Write(p)
}

// tailWriter keeps the last max bytes, so a failing agent's final error message is kept
// without holding all of its output.
type tailWriter struct {
	w   *bytes.Buffer
	max int
}

func (t *tailWriter) Write(p []byte) (int, error) {
	n := len(p)
	if len(p) > t.max {
		p = p[len(p)-t.max:]
	}
	if over := t.w.Len() + len(p) - t.max; over > 0 {
		rest := append([]byte(nil), t.w.Bytes()[over:]...)
		t.w.Reset()
		t.w.Write(rest)
	}
	t.w.Write(p)
	return n, nil
}
