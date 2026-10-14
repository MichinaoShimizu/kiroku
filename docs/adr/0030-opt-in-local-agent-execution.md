# ADR 0030: Opt-in local AI agent execution (`kiroku serve --ai`)

Status: Accepted (2026-10-14, approved by the maintainer during the quality audit); points 1 and 2 superseded by [ADR 0034](0034-ai-analysis-by-default.md) (2026-10-14), points 3–6 still in force
Date: 2026-10-14
Scope: Running an installed AI agent on a prompt built from history (`internal/ai`, `internal/cli` serve `--ai`, `/ai/run`, `internal/web/js/ai.js`, `internal/web/js/mdview.js`, `SECURITY.md`)
Related ADRs: [ADR 0022](0022-history-stays-local.md), [ADR 0029](0029-japanese-ui.md)
Related Decision Logs: [DL 0013 — user-mediated AI reports](../decision-log/0013-user-mediated-ai-reports.md)
Supersedes/Amends: Amends [ADR 0022](0022-history-stays-local.md) points 1 and 2; replaces the unnumbered draft [ai-execution-boundary.md](ai-execution-boundary.md)
Decision owner: Repository maintainer

## Context

ADR 0022 says history never leaves the machine (point 1) and that kiroku never sends a prompt to an AI itself; the user copies it (point 2). Its Consequences say a feature that sends history off the machine, AI calls included, needs a new ADR.

`kiroku serve --ai` shipped in v0.32–v0.34 (#385, #386, #387, #401, #403) without one. It runs `claude`, `codex` or `kiro-cli` on this computer with the prompt on stdin, and lets the agent read the history files the prompt points to. The agent may send the prompt and what it reads to its provider. A draft, [ai-execution-boundary.md](ai-execution-boundary.md), described this boundary but stayed Proposed, unnumbered and outside the index. The quality audit of 2026-10-14 found the conflict. It also found that #401 had removed the sentence telling the user, in the run dialog, that the agent may send data to its provider.

## Decision

1. **An opt-in exception to ADR 0022 points 1 and 2.** Only `kiroku serve --ai` may hand a prompt and history to an AI agent, and only after the user confirms each run in the view. Without `--ai`, and in every saved HTML file, kiroku never runs an agent, and ADR 0022 applies unchanged. kiroku itself still sends nothing over the network; the agent the user picks does, under that agent's own account and terms.
2. **Disclosure where the user decides.** The run dialog shows the exact prompt and says that the agent may send the prompt and the history it reads to its provider. The same sentence is in `kiroku serve --help`, the startup line of `kiroku serve --ai`, README, the guide and `SECURITY.md`.
3. **The browser chooses only from fixed options.** `/ai/run` takes an agent from a fixed list (only those whose command is installed) and the prompt text. It never takes a command line, a flag or a path. The command lines are fixed in `internal/ai/runner.go`. The prompt goes in on stdin, with no shell.
4. **Least access per agent.** Claude Code gets only the Read tool, only inside the history folders (`--add-dir`), with no MCP servers. Codex runs with `--sandbox read-only`. Each run starts in an empty temporary folder, so no project instructions or repository are picked up.
5. **Bounded and local.** The endpoint keeps the serve key, Host check, `X-Kiroku` header and same-origin check, and listens only on a loopback address. One run at a time, a 256 KiB request, a 2-minute limit, 1 MiB of output, and the process group stopped when the run ends or the browser leaves.
6. **The reply is untrusted text.** It is shown as text or as elements built from text (`mdview.js`). It is never inserted as HTML, images are never loaded, and links are not clickable.

## Alternatives

- **Copy-only (ADR 0022 as it was).** Lowest risk, and still the default. Not enough for users who want the run without switching windows.
- **Calling a provider API directly.** It would put API keys in kiroku and tie it to each provider's API. Not chosen.
- **No notice in the dialog**, keeping it only in the docs and the terminal (the state after #401). The user doesn't see it where they confirm the run. Not chosen.
- **A notice only before the first run** (remembered per browser). It saves space, but a viewer who cleared storage or a second person at the same browser can miss it. Not chosen.

## Consequences

- "History never leaves the machine" is true only with `--ai` off. README, the guide and `SECURITY.md` say so.
- Kiro CLI has no flag kiroku can use to limit it to reading files. `kiro-cli chat --no-interactive` runs with the user's own Kiro tool settings, so a prompt injection in history could use tools the user has already trusted. This is recorded as an open item. Revisit when Kiro CLI offers a tool allowlist.
- An agent version without the flags in point 4 fails instead of running with more access.
- Revisit if kiroku ever needs to run agents outside `kiroku serve`, keep results, or send anything itself.

## Verification

- `internal/ai`: `TestClaudeReadsOnlyHistoryDirs`, `TestStoppedRunEndsWithChildren`, `TestClaudeResultLimit`, `TestClaudeResultWithLingeringChild`, `TestHandlerStreams`, `TestInstalled`
- `internal/web`: `TestAIRunDisclosure` (the run dialog carries the disclosure in both languages), `TestMdView`
- The hostile-tester run of the 2026-10-14 quality audit found no way to run a command or a path from the request (`docs/quality/audit-log.md`)

## Open questions

- A read-only mode for Kiro CLI, and Windows process-tree cleanup for agents installed as `.cmd` shims. Both are tracked in `docs/quality/audit-log.md`.
