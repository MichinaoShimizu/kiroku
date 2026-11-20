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
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"
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
	// Private returns the history files of conversations whose content kiroku keeps out of its view and
	// prompts (Kiro Crew incognito and temporary conversations, and their copies in other agents' history).
	// Claude Code gets a Read deny rule for each, in a settings file in the run's temporary folder, so it
	// can't read them although they are inside Dirs. Deny rules win over --add-dir and the user's own rules.
	Private func() []string
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
// binaryOf is the program each agent runs as.
func binaryOf(agent string) (string, error) {
	switch agent {
	case "claude", "codex":
		return agent, nil
	case "kiro":
		return "kiro-cli", nil
	}
	return "", ErrUnsupportedAgent
}

// ctx bounds reading the agent's help (flagsOf).
func (r Runner) command(ctx context.Context, agent string) (string, []string, error) {
	switch agent {
	case "claude":
		// Only Read, and only inside the history folders: reads elsewhere need approval, which -p denies.
		// stream-json (which needs --verbose with -p) shows the reply and each file read as they happen.
		args := []string{"-p", "--output-format", "stream-json", "--verbose", "--include-partial-messages", "--strict-mcp-config", "--tools", "Read"}
		// The user's own settings could widen that (allow rules, extra folders, bypassPermissions, hooks), and
		// history could ask the agent to use them. --restricted ignores user, project and local settings and keeps
		// the file tools inside the working folders; older versions get --setting-sources instead (only the
		// empty temporary folder's project settings). --no-session-persistence keeps the run, which holds the
		// prompt and the files it read, out of Claude Code's history. Each is added only when this claude has it.
		fl := flagsOf(ctx, "claude", "--help")
		switch {
		case fl["--restricted"]:
			args = append(args, "--restricted")
		case fl["--setting-sources"]:
			args = append(args, "--setting-sources", "project")
		}
		if fl["--no-session-persistence"] {
			args = append(args, "--no-session-persistence")
		}
		for _, d := range r.Dirs {
			if fi, err := os.Stat(d); err == nil && fi.IsDir() && !strings.HasPrefix(d, "-") {
				args = append(args, "--add-dir", d)
			}
		}
		return "claude", args, nil
	case "codex":
		// The temporary folder is not a git repository, which codex exec otherwise refuses.
		args := []string{"exec", "--sandbox", "read-only", "--skip-git-repo-check"}
		fl := flagsOf(ctx, "codex", "exec", "--help")
		if fl["--ephemeral"] {
			args = append(args, "--ephemeral") // don't keep the run (the prompt and what it read) as a Codex session
		}
		if fl["--config"] {
			// No web search and none of the user's MCP servers: ways out of the machine for what it read.
			// Both spellings of web search (newer and older Codex); a key a version doesn't know is ignored.
			args = append(args, "-c", `web_search="disabled"`, "-c", "tools.web_search=false", "-c", "mcp_servers={}")
		}
		return "codex", append(args, "-"), nil
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
	timeout := r.Timeout
	if timeout <= 0 || timeout > 5*time.Minute {
		timeout = 2 * time.Minute
	}
	bounded, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	binary, args, err := r.command(bounded, agent)
	if err != nil {
		return Result{}, err
	}
	if err := bounded.Err(); err != nil {
		return Result{}, err // stopped while reading the agent's help
	}
	// An empty folder of its own, so the agent doesn't pick up the project instructions, settings or
	// repository of wherever kiroku serve was started.
	dir, err := os.MkdirTemp("", "kiroku-ai-")
	if err != nil {
		return Result{}, err
	}
	defer os.RemoveAll(dir)
	if agent == "claude" && r.Private != nil {
		if rules := denyRules(r.Private()); len(rules) > 0 {
			b, err := json.Marshal(map[string]any{"permissions": map[string]any{"deny": rules}})
			if err != nil {
				return Result{}, err
			}
			p := filepath.Join(dir, "kiroku-private.json")
			if err := os.WriteFile(p, b, 0o600); err != nil {
				return Result{}, err
			}
			args = append(args, "--settings", p)
		}
	}
	cmd := exec.CommandContext(bounded, binary, args...)
	cmd.Dir = dir
	cmd.Env = agentEnv(os.Environ())
	cmd.Stdin = strings.NewReader(prompt)
	cmd.WaitDelay = 2 * time.Second
	started, done := ownGroup(cmd)
	defer done()
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
				e.S = cut(e.S, len(e.S)-(shown-maxOutput))
			}
		}
		emit(e)
	}
	send(Event{T: "cmd", Argv: append([]string{binary}, args...)})
	if err := cmd.Start(); err != nil {
		return Result{}, fmt.Errorf("agent %s failed: %w", agent, err)
	}
	started()
	// Once the run is stopped (timeout, the browser gave up, too much output), stop reading after a short
	// delay even if something the agent started still holds its output open, so the run always ends.
	stopReading := context.AfterFunc(bounded, func() {
		time.Sleep(cmd.WaitDelay)
		stdout.Close()
		stderr.Close()
	})
	defer stopReading()
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
	// Claude's result event ends the run. If something the agent started still holds its output open
	// after that, stop it after a short delay instead of waiting for the time limit and losing the result.
	var linger *time.Timer
	var lingered atomic.Bool
	if agent == "claude" {
		readClaude(limited, add, func(text string, isErr bool) {
			if isErr {
				failed = text
			} else {
				final = &text
			}
			if linger == nil {
				linger = time.AfterFunc(cmd.WaitDelay, func() { lingered.Store(true); cancel() })
			}
		})
		if linger != nil {
			linger.Stop()
		}
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
	if lingered.Load() && ctx.Err() == nil && (final != nil || failed != "") {
		err = nil // stopped by us after the result (above), not by the time limit or the browser
	}
	if tooLong {
		return Result{}, errors.New("agent output exceeds size limit")
	}
	if err == nil && failed != "" {
		err = errors.New("the agent reported an error")
	}
	if err != nil {
		mu.Lock()
		msg := strings.TrimSpace(clean(cut(firstNonEmpty(failed, tail.String()), 2048)))
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
	if len(text) > maxOutput {
		return Result{}, errors.New("agent output exceeds size limit")
	}
	return Result{Output: strings.TrimSpace(clean(text)), Agent: agent}, nil
}

// forward reads r and calls each with what arrived, a line (or about 4 KiB of a long line) at a time.
// A character split at the 4 KiB cut is kept whole for the next piece.
func forward(r io.Reader, each func(string)) {
	br := bufio.NewReaderSize(r, 4096)
	var carry []byte
	for {
		line, err := br.ReadSlice('\n')
		buf := append(carry, line...)
		carry = nil
		if errors.Is(err, bufio.ErrBufferFull) {
			if k := partialRune(buf); k > 0 {
				carry = append([]byte(nil), buf[len(buf)-k:]...)
				buf = buf[:len(buf)-k]
			}
		}
		if len(buf) > 0 {
			each(string(buf))
		}
		if err != nil && !errors.Is(err, bufio.ErrBufferFull) {
			if len(carry) > 0 {
				each(string(carry))
			}
			return
		}
	}
}

// partialRune is how many bytes at the end of b start a UTF-8 character that isn't complete yet.
func partialRune(b []byte) int {
	for k := 1; k <= 3 && k <= len(b); k++ {
		c := b[len(b)-k]
		if c&0xC0 != 0x80 { // the first byte of a character: is it longer than what arrived?
			need := 1
			switch {
			case c >= 0xF0:
				need = 4
			case c >= 0xE0:
				need = 3
			case c >= 0xC0:
				need = 2
			}
			if need > k {
				return k
			}
			return 0
		}
	}
	return 0
}

// cut shortens s to at most n bytes without splitting a character.
func cut(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
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
		// Characters that change the direction of the text or are invisible (but not the joiners emoji and
		// some scripts need), so a file name can't be shown reversed or disguised.
		if r == 0x200B || r == 0x200E || r == 0x200F || (r >= 0x202A && r <= 0x202E) || (r >= 0x2066 && r <= 0x2069) || r == 0xFEFF {
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

// denyRules turns history files into Claude Code Read deny rules (gitignore patterns anchored at the
// filesystem root with //). Each file also covers the folder of the same name beside it (Claude Code's
// subagents/ folder) and, for a Kiro Crew conversation log in sessions/, the lines Crew moved to
// sessions/archive/<name>__<time>.jsonl. A path the patterns can't express literally (with parentheses
// or a line break) denies its whole folder instead.
func denyRules(files []string) []string {
	var out []string
	seen := map[string]bool{}
	add := func(pattern string) {
		if r := "Read(//" + pattern + ")"; !seen[r] {
			seen[r] = true
			out = append(out, r)
		}
	}
	for _, f := range files {
		if f == "" || !filepath.IsAbs(f) {
			continue
		}
		paths := []string{f}
		if real, err := filepath.EvalSymlinks(f); err == nil && real != f {
			paths = append(paths, real)
		}
		for _, p := range paths {
			p = posixPath(p)
			dir, base := pathDir(p), pathBase(p)
			if strings.ContainsAny(p, unruled) {
				// the nearest folder whose path the rule can hold; at worst the whole filesystem
				up := dir
				for strings.ContainsAny(up, unruled) {
					up = pathDir(up)
				}
				if up == "" {
					add("**")
				} else {
					add(globEscape(up) + "/**")
				}
				continue
			}
			add(globEscape(p))
			stem := strings.TrimSuffix(base, filepath.Ext(base))
			stem = strings.TrimSuffix(stem, ".jsonl") // .jsonl.zst
			add(globEscape(dir+"/"+stem) + "/**")
			if pathBase(dir) == "sessions" {
				add(globEscape(dir+"/archive/"+stem) + "__*")
			}
		}
	}
	return out
}

// unruled are characters a rule can't hold literally: parentheses end Read(...), a line break ends the
// list, and Claude Code turns \\ into \ before matching, so a name with a backslash can't be matched
// exactly. A path with any of them denies the nearest folder whose path has none.
const unruled = "()\\\n\r"

// posixPath writes p the way Claude Code matches it: with / and, on Windows, C:\x as c/x (no leading /).
func posixPath(p string) string {
	p = filepath.ToSlash(p)
	if len(p) >= 2 && p[1] == ':' {
		p = strings.ToLower(p[:1]) + p[2:]
	}
	return strings.TrimLeft(p, "/")
}

func pathDir(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[:i]
	}
	return ""
}

func pathBase(p string) string { return p[strings.LastIndex(p, "/")+1:] }

// globEscape escapes the characters gitignore patterns treat specially, so a name matches only itself.
func globEscape(s string) string {
	var b strings.Builder
	for _, r := range s {
		if strings.ContainsRune(`\*?[]!#`, r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// flagsOf returns the long options (--name) that a command's help lists, such as `claude --help`, so kiroku
// adds an option only when the installed version has it. A help that was read in full is kept per program
// (the file the command resolves to, its size and time, so an update is read again); one that failed, timed
// out or was stopped is not kept, so a slow first start doesn't leave the run without its options for good.
func flagsOf(ctx context.Context, name string, helpArgs ...string) map[string]bool {
	path, err := exec.LookPath(name)
	if err != nil {
		return nil
	}
	key := path + "\x00" + strings.Join(helpArgs, " ")
	real := path
	if r, err := filepath.EvalSymlinks(path); err == nil {
		real = r
	}
	if fi, err := os.Stat(real); err == nil {
		key += fmt.Sprintf("\x00%s\x00%d\x00%d", real, fi.Size(), fi.ModTime().UnixNano())
	}
	if v, ok := helpCache.Load(key); ok {
		return v.(map[string]bool)
	}
	hctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(hctx, path, helpArgs...)
	cmd.Env = agentEnv(os.Environ())
	cmd.Dir = os.TempDir()
	cmd.WaitDelay = 500 * time.Millisecond // a child left holding the output doesn't keep the run waiting
	out := &capWriter{max: 1 << 20}        // at most 1 MiB of help: a program that prints without end isn't kept in memory
	cmd.Stdout = out
	started, done := ownGroup(cmd)
	defer done()
	if err := cmd.Start(); err != nil {
		return nil
	}
	started()
	err = cmd.Wait()
	if cmd.Cancel != nil {
		_ = cmd.Cancel() // processes the help started are ended with it (its process group, or job object on Windows)
	}
	fl := map[string]bool{}
	for _, m := range helpFlag.FindAllString(out.b.String(), -1) {
		fl[m] = true
	}
	if err != nil || hctx.Err() != nil || out.full {
		return fl // used for this run, but read again next time
	}
	helpCache.Store(key, fl)
	return fl
}

// capWriter keeps the first max bytes and then refuses more, which ends copying the output.
type capWriter struct {
	b    bytes.Buffer
	max  int
	full bool
}

func (w *capWriter) Write(p []byte) (int, error) {
	if room := w.max - w.b.Len(); len(p) > room {
		w.b.Write(p[:max(room, 0)])
		w.full = true
		return 0, errors.New("help too long")
	}
	return w.b.Write(p)
}

var (
	helpCache sync.Map
	helpFlag  = regexp.MustCompile(`--[a-z][a-z0-9-]*`)
)

// agentEnv is kiroku's environment without what would steer the agent or hand it unrelated secrets:
//   - the variables a Claude Code session sets for the commands it runs (kiroku serve started from inside
//     Claude Code, locally or in the cloud): they could add folders, tie the run to that session or copy its
//     output elsewhere. Only the ones that choose a provider or sign-in stay (keepClaude)
//   - tokens and personal access tokens (names ending in _TOKEN or _PAT, such as GH_TOKEN, NPM_TOKEN,
//     HF_TOKEN), which the analysis never needs, except the providers' own (Anthropic, AWS for Bedrock,
//     Google Cloud for Vertex, Azure)
//
// Everything else stays, since the agent needs its own sign-in and provider settings (API keys, credentials
// files, proxies and CA bundles).
func agentEnv(env []string) []string {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		k, _, _ := strings.Cut(kv, "=")
		if dropEnv(k) {
			continue
		}
		out = append(out, kv)
	}
	return out
}

var keepClaude = map[string]bool{
	"CLAUDE_CODE_USE_BEDROCK": true, "CLAUDE_CODE_USE_VERTEX": true, "CLAUDE_CODE_USE_FOUNDRY": true,
	"CLAUDE_CODE_SKIP_BEDROCK_AUTH": true, "CLAUDE_CODE_SKIP_VERTEX_AUTH": true, "CLAUDE_CODE_SKIP_FOUNDRY_AUTH": true,
	"CLAUDE_CODE_OAUTH_TOKEN": true, "CLAUDE_CODE_API_KEY_HELPER_TTL_MS": true,
	"CLAUDE_CODE_CLIENT_CERT": true, "CLAUDE_CODE_CLIENT_KEY": true, "CLAUDE_CODE_CLIENT_KEY_PASSPHRASE": true,
	"CLAUDE_CONFIG_DIR": true,
}

func dropEnv(k string) bool {
	u := strings.ToUpper(k) // Windows names are case-insensitive
	if keepClaude[u] {
		return false
	}
	if u == "CLAUDECODE" || u == "CLAUDE_PID" || strings.HasPrefix(u, "CLAUDE_") || strings.HasPrefix(u, "SESSION_INGRESS") {
		return true // CLAUDE_CODE_*, CLAUDE_SESSION_*, CLAUDE_ADDITIONAL_DIRECTORIES and the like
	}
	if strings.HasSuffix(u, "_TOKEN") || strings.HasSuffix(u, "_PAT") || u == "TWINE_PASSWORD" {
		for _, p := range []string{"ANTHROPIC_", "AWS_", "GOOGLE_", "CLOUDSDK_", "AZURE_", "OPENAI_", "CODEX_", "KIRO_"} {
			if strings.HasPrefix(u, p) {
				return false
			}
		}
		return true
	}
	return false
}
