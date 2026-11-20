package ai

import (
	"context"
	"slices"
	"strings"
	"testing"
)

// Claude Code: the user's own settings (allow rules, extra folders, bypassPermissions, hooks) must not widen
// what history can ask the agent to read, and the run must not be kept as a new session. Options are added
// only when the installed version lists them.
func TestClaudeIgnoresUserSettingsAndKeepsNoSession(t *testing.T) {
	for _, c := range []struct {
		name, help string
		want, not  []string
	}{
		{"restricted", "  --restricted  Restricted mode\n  --setting-sources <s>\n  --no-session-persistence  keep no session", []string{"--restricted", "--no-session-persistence"}, []string{"--setting-sources"}},
		{"older", "  --setting-sources <sources>  Comma-separated", []string{"--setting-sources project"}, []string{"--restricted", "--no-session-persistence"}},
		{"oldest", "Usage: claude [options]", nil, []string{"--restricted", "--setting-sources", "--no-session-persistence"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			stubAgent(t, "claude", `if [ "$1" = --help ]; then printf '%s\n' '`+c.help+`'; exit 0; fi
cat >/dev/null; echo '{"type":"result","result":"ok"}'`)
			var argv []string
			if _, err := (Runner{Enabled: true}).Stream(context.Background(), "claude", "hi", func(e Event) {
				if e.T == "cmd" {
					argv = e.Argv
				}
			}); err != nil {
				t.Fatal(err)
			}
			line := strings.Join(argv, " ")
			for _, w := range c.want {
				if !strings.Contains(line, w) {
					t.Errorf("command %q lacks %q", line, w)
				}
			}
			for _, n := range c.not {
				if slices.Contains(argv, n) {
					t.Errorf("command %q has %q", line, n)
				}
			}
		})
	}
}

// Codex: no session kept, no web search and no MCP servers, when this codex has the options.
func TestCodexEphemeralWithoutWebOrMCP(t *testing.T) {
	stubAgent(t, "codex", `if [ "$2" = --help ]; then echo '  -c, --config <key=value>'; echo '      --ephemeral'; exit 0; fi
cat >/dev/null; echo ok`)
	var argv []string
	if _, err := (Runner{Enabled: true}).Stream(context.Background(), "codex", "hi", func(e Event) {
		if e.T == "cmd" {
			argv = e.Argv
		}
	}); err != nil {
		t.Fatal(err)
	}
	line := strings.Join(argv, " ")
	for _, w := range []string{"--sandbox read-only", "--ephemeral", `-c web_search="disabled"`, "-c tools.web_search=false", "-c mcp_servers={}"} {
		if !strings.Contains(line, w) {
			t.Errorf("command %q lacks %q", line, w)
		}
	}
	if argv[len(argv)-1] != "-" {
		t.Errorf("the prompt must still come from stdin: %q", line)
	}
}

// The agent gets kiroku's environment without a surrounding Claude Code session's variables and without
// tokens it never needs; its own sign-in stays.
func TestAgentEnvDropsSessionAndUnrelatedTokens(t *testing.T) {
	t.Setenv("CLAUDECODE", "1")
	t.Setenv("CLAUDE_ADDITIONAL_DIRECTORIES", "/")
	t.Setenv("CLAUDE_CODE_SESSION_ID", "x")
	t.Setenv("GH_TOKEN", "ghp_secret")
	t.Setenv("GITHUB_TOKEN", "ghp_secret")
	t.Setenv("NPM_TOKEN", "npm_secret")
	t.Setenv("ANTHROPIC_API_KEY", "sk-keep")
	t.Setenv("CLAUDE_SESSION_INGRESS_TOKEN_FILE", "/x")
	t.Setenv("SESSION_INGRESS_URL", "http://x")
	t.Setenv("CLAUDE_CODE_TEE_SDK_STDOUT", "/x")
	t.Setenv("CLAUDE_CODE_REMOTE_SDK_URL", "http://x")
	t.Setenv("GITHUB_PAT", "x")
	t.Setenv("HF_TOKEN", "x")
	t.Setenv("Gh_Token", "x")
	t.Setenv("AWS_SESSION_TOKEN", "keep")
	t.Setenv("CLAUDE_CONFIG_DIR", "/keep")
	t.Setenv("CLAUDE_CODE_USE_BEDROCK", "1")
	stubAgent(t, "claude", `cat >/dev/null; printf '{"type":"result","result":"%s"}\n' "$(env | cut -d= -f1 | tr '\n' ' ')"`)
	res, err := (Runner{Enabled: true}).Run(context.Background(), "claude", "hi")
	if err != nil {
		t.Fatal(err)
	}
	names := strings.Fields(res.Output)
	for _, gone := range []string{"CLAUDECODE", "CLAUDE_ADDITIONAL_DIRECTORIES", "CLAUDE_CODE_SESSION_ID", "GH_TOKEN", "GITHUB_TOKEN", "NPM_TOKEN",
		"CLAUDE_SESSION_INGRESS_TOKEN_FILE", "SESSION_INGRESS_URL", "CLAUDE_CODE_TEE_SDK_STDOUT", "CLAUDE_CODE_REMOTE_SDK_URL", "GITHUB_PAT", "HF_TOKEN", "Gh_Token"} {
		if slices.Contains(names, gone) {
			t.Errorf("agent got %s", gone)
		}
	}
	for _, kept := range []string{"ANTHROPIC_API_KEY", "CLAUDE_CODE_USE_BEDROCK", "AWS_SESSION_TOKEN", "CLAUDE_CONFIG_DIR", "PATH"} {
		if !slices.Contains(names, kept) {
			t.Errorf("agent lost %s", kept)
		}
	}
}

// The help is read again after a slow or failed first read, so one timeout doesn't leave every later run
// without its options; a help that never ends isn't kept in memory.
func TestHelpReadAgainAfterTimeout(t *testing.T) {
	dir := t.TempDir()
	stubAgent(t, "claude", `if [ "$1" = --help ]; then
  if [ ! -e "`+dir+`/once" ]; then touch "`+dir+`/once"; sleep 7; fi
  echo '  --restricted'; echo '  --no-session-persistence'; exit 0; fi
cat >/dev/null; echo '{"type":"result","result":"ok"}'`)
	argvOf := func() string {
		var argv []string
		if _, err := (Runner{Enabled: true}).Stream(context.Background(), "claude", "hi", func(e Event) {
			if e.T == "cmd" {
				argv = e.Argv
			}
		}); err != nil {
			t.Fatal(err)
		}
		return strings.Join(argv, " ")
	}
	first := argvOf()
	if second := argvOf(); !strings.Contains(second, "--restricted") || !strings.Contains(second, "--no-session-persistence") {
		t.Errorf("after a timed-out help (first run %q), the next run is still without its options: %q", first, second)
	}
}

func TestHelpOutputIsCapped(t *testing.T) {
	stubAgent(t, "claude", `if [ "$1" = --help ]; then echo '  --restricted'; head -c 50000000 /dev/zero; exit 0; fi
cat >/dev/null; echo '{"type":"result","result":"ok"}'`)
	fl := flagsOf(context.Background(), "claude", "--help")
	if !fl["--restricted"] {
		t.Errorf("flags = %v, want --restricted read from the first MiB", fl)
	}
}
