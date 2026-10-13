// Package ai provides an explicitly opted-in execution boundary for local AI agents.
package ai

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

var ErrDisabled = errors.New("AI execution is disabled")
var ErrUnsupportedAgent = errors.New("unsupported AI agent")

type Runner struct {
	Enabled bool
	Timeout time.Duration
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
		args = []string{"-p", "--output-format", "text", "--tools", ""}
	case "codex":
		binary = "codex"
		args = []string{"exec", "--sandbox", "read-only", "-"}
	default:
		return Result{}, ErrUnsupportedAgent
	}
	timeout := r.Timeout
	if timeout <= 0 || timeout > 5*time.Minute {
		timeout = 2 * time.Minute
	}
	bounded, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(bounded, binary, args...)
	cmd.Stdin = strings.NewReader(prompt)
	cmd.WaitDelay = 2 * time.Second
	var buf bytes.Buffer
	cmd.Stdout = &limitedWriter{w: &buf, max: 1024 * 1024}
	err := cmd.Run()
	output := buf.Bytes()
	if len(output) >= 1024*1024 {
		return Result{}, errors.New("agent output exceeds size limit")
	}
	if err != nil {
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
