// Package ai provides an explicitly opted-in execution boundary for local AI agents.
package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"
)

var ErrDisabled = errors.New("AI execution is disabled")
var ErrUnsupportedAgent = errors.New("unsupported AI agent")

const (
	maxPrompt = 256 * 1024
	maxOutput = 1024 * 1024 // the result, and the progress sent to the view, each
	maxRead   = 64 << 20    // what kiroku reads from the agent's stdout at most (Claude's events include the files it reads)
	maxLine   = 16 << 20    // one line of Claude's events; longer lines (a huge file it read) are skipped
)

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

// Event is one step of a run, sent to the view as it happens:
//   - "cmd": the command kiroku runs (Argv)
//   - "text": text the agent writes back
//   - "tool": something the agent does, such as reading a file (Claude Code)
//   - "log": the agent's own progress or error output (stderr)
//
// All text is the agent's output, which may quote history: the view shows it as text, never as HTML.
type Event struct {
	T    string   `json:"t"`
	S    string   `json:"s,omitempty"`
	Argv []string `json:"argv,omitempty"`
}

// command is the fixed command line for an agent. The request never supplies any of it.
func (r Runner) command(agent string) (string, []string, error) {
	switch agent {
	case "claude":
		// Only Read, and only inside the history folders: reads elsewhere need approval, which -p denies.
		// stream-json (which needs --verbose with -p) shows the reply and each file read as they happen.
		args := []string{"-p", "--output-format", "stream-json", "--verbose", "--include-partial-messages", "--strict-mcp-config", "--tools", "Read"}
		for _, d := range r.Dirs {
			if fi, err := os.Stat(d); err == nil && fi.IsDir() && !strings.HasPrefix(d, "-") {
				args = append(args, "--add-dir", d)
			}
		}
		return "claude", args, nil
	case "codex":
		// The temporary folder is not a git repository, which codex exec otherwise refuses.
		return "codex", []string{"exec", "--sandbox", "read-only", "--skip-git-repo-check", "-"}, nil
	case "kiro":
		return "kiro-cli", []string{"chat", "--no-interactive"}, nil
	}
	return "", nil, ErrUnsupportedAgent
}

// Run runs the prompt and returns the whole result.
func (r Runner) Run(ctx context.Context, agent, prompt string) (Result, error) {
	return r.Stream(ctx, agent, prompt, func(Event) {})
}

// Stream runs the prompt with the agent and calls emit for each step as it happens (never from two
// goroutines at once). The returned Result holds the agent's final reply.
func (r Runner) Stream(ctx context.Context, agent, prompt string, emit func(Event)) (Result, error) {
	if !r.Enabled {
		return Result{}, ErrDisabled
	}
	if strings.TrimSpace(prompt) == "" {
		return Result{}, errors.New("empty prompt")
	}
	if len(prompt) > maxPrompt {
		return Result{}, errors.New("prompt exceeds size limit")
	}
	binary, args, err := r.command(agent)
	if err != nil {
		return Result{}, err
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
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return Result{}, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return Result{}, err
	}
	var mu sync.Mutex
	shown := 0 // bytes of progress sent to the view
	send := func(e Event) {
		mu.Lock()
		defer mu.Unlock()
		if e.T != "cmd" {
			e.S = clean(e.S)
			if e.S == "" || shown >= maxOutput {
				return
			}
			if shown += len(e.S); shown > maxOutput {
				e.S = e.S[:len(e.S)-(shown-maxOutput)]
			}
		}
		emit(e)
	}
	send(Event{T: "cmd", Argv: append([]string{binary}, args...)})
	if err := cmd.Start(); err != nil {
		return Result{}, fmt.Errorf("agent %s failed: %w", agent, err)
	}
	var tail bytes.Buffer
	tw := &tailWriter{w: &tail, max: 2048}
	errDone := make(chan struct{})
	go func() { // the agent's own progress and errors, as they arrive
		defer close(errDone)
		forward(stderr, func(s string) {
			mu.Lock()
			tw.Write([]byte(s))
			mu.Unlock()
			send(Event{T: "log", S: s})
		})
	}()
	var out strings.Builder
	var final *string // Claude's result event: the final reply only
	var failed string // Claude's error result
	tooLong := false
	add := func(e Event) {
		if tooLong {
			return
		}
		if e.T == "text" {
			if out.Len()+len(e.S) > maxOutput {
				tooLong = true
				cancel() // stop the agent rather than read without end
				return
			}
			out.WriteString(e.S)
		}
		send(e)
	}
	limited := &io.LimitedReader{R: stdout, N: maxRead}
	if agent == "claude" {
		readClaude(limited, add, func(text string, isErr bool) {
			if isErr {
				failed = text
			} else {
				final = &text
			}
		})
	} else {
		forward(limited, func(s string) { add(Event{T: "text", S: s}) })
	}
	if limited.N <= 0 && !tooLong {
		tooLong = true
		cancel()
	}
	_, _ = io.Copy(io.Discard, stdout)
	<-errDone
	err = cmd.Wait()
	if tooLong {
		return Result{}, errors.New("agent output exceeds size limit")
	}
	if err == nil && failed != "" {
		err = errors.New("the agent reported an error")
	}
	if err != nil {
		mu.Lock()
		msg := strings.TrimSpace(clean(firstNonEmpty(failed, tail.String())))
		mu.Unlock()
		if msg != "" {
			return Result{}, fmt.Errorf("agent %s failed: %w: %s", agent, err, msg)
		}
		return Result{}, fmt.Errorf("agent %s failed: %w", agent, err)
	}
	text := out.String()
	if final != nil {
		text = *final
	}
	return Result{Output: strings.TrimSpace(clean(text)), Agent: agent}, nil
}

// forward reads r and calls each with what arrived, a line (or 4 KiB of a long line) at a time.
func forward(r io.Reader, each func(string)) {
	br := bufio.NewReaderSize(r, 4096)
	for {
		line, err := br.ReadSlice('\n')
		if len(line) > 0 {
			each(string(line))
		}
		if err != nil && !errors.Is(err, bufio.ErrBufferFull) {
			return
		}
	}
}

// claudeLine is the part of a Claude Code stream-json event that kiroku reads.
type claudeLine struct {
	Type  string `json:"type"`
	Event struct {
		Type  string `json:"type"`
		Delta struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"delta"`
	} `json:"event"`
	Message struct {
		Content []struct {
			Type  string          `json:"type"`
			Text  string          `json:"text"`
			Name  string          `json:"name"`
			Input json.RawMessage `json:"input"`
		} `json:"content"`
	} `json:"message"`
	Result  *string `json:"result"`
	IsError bool    `json:"is_error"`
}

// readClaude reads Claude Code's stream-json events: the reply as it is written (text deltas), the tools it
// uses, and the final result. Other events and lines too long to hold (a large file it read) are skipped.
func readClaude(r io.Reader, emit func(Event), result func(text string, isErr bool)) {
	br := bufio.NewReaderSize(r, 64<<10)
	streamed := false // the current message's text came as deltas
	for {
		line, err := readLine(br, maxLine)
		var e claudeLine
		if len(line) > 0 && json.Unmarshal(line, &e) == nil {
			switch e.Type {
			case "stream_event":
				switch {
				case e.Event.Type == "message_start":
					streamed = false
				case e.Event.Type == "content_block_delta" && e.Event.Delta.Type == "text_delta":
					streamed = true
					emit(Event{T: "text", S: e.Event.Delta.Text})
				}
			case "assistant":
				wrote := streamed
				for _, c := range e.Message.Content {
					switch c.Type {
					case "text":
						if !streamed { // a version without partial messages
							emit(Event{T: "text", S: c.Text})
							wrote = true
						}
					case "tool_use":
						emit(Event{T: "tool", S: toolLine(c.Name, c.Input)})
					}
				}
				if wrote {
					emit(Event{T: "text", S: "\n\n"})
				}
				streamed = false
			case "result":
				if e.Result != nil {
					result(*e.Result, e.IsError)
				}
			}
		}
		if err != nil {
			return
		}
	}
}

// readLine reads one line; a line longer than max is read through and returned empty.
func readLine(br *bufio.Reader, max int) ([]byte, error) {
	var buf []byte
	over := false
	for {
		part, err := br.ReadSlice('\n')
		if !over {
			if len(buf)+len(part) > max {
				over, buf = true, nil
			} else {
				buf = append(buf, part...)
			}
		}
		if errors.Is(err, bufio.ErrBufferFull) {
			continue
		}
		return buf, err
	}
}

// toolLine describes a tool call in one line, such as "Read /path/to/file". Only a few known fields are
// shown, cut short.
func toolLine(name string, input json.RawMessage) string {
	var in map[string]any
	_ = json.Unmarshal(input, &in)
	for _, k := range []string{"file_path", "path", "pattern", "command"} {
		if v, ok := in[k].(string); ok && v != "" {
			if len(v) > 300 {
				v = v[:300] + "…"
			}
			return name + " " + strings.Join(strings.Fields(v), " ")
		}
	}
	return name
}

var ansi = regexp.MustCompile(`\x1b(\[[0-9;?]*[ -/]*[@-~]|\][^\x07\x1b]*(\x07|\x1b\\)|[@-_])`)

// clean removes terminal escape sequences and control characters other than newline and tab, so the
// agent's output can't change how the view shows it. Invalid UTF-8 becomes U+FFFD.
func clean(s string) string {
	s = ansi.ReplaceAllString(s, "")
	s = strings.ToValidUTF8(s, "�")
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return r
		}
		if r < 0x20 || r == 0x7f || (r >= 0x80 && r < 0xa0) {
			return -1
		}
		return r
	}, s)
}

func firstNonEmpty(s ...string) string {
	for _, v := range s {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
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
