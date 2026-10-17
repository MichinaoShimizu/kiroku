# ADR 0034: "Analyze with AI" by default in `kiroku serve`

Status: Accepted (2026-10-14, approved by the maintainer); point 1 amended by [ADR 0035](0035-no-monthly-ai-report.md) and [ADR 0036](0036-ai-prompt-preview-in-demo.md) (2026-10-16)
Date: 2026-10-14
Scope: Running an installed AI agent on a prompt built from history (`internal/cli` serve `--no-ai` / `--ai` / `$KIROKU_NO_AI`, `/ai/run`, `internal/ai`, `internal/web/js/ai.js`, `state.js`, `summary.js`, `session.js`, `SECURITY.md`, README, the guide)
Related ADRs: [ADR 0022](0022-history-stays-local.md), [ADR 0023](0023-compatibility-contract.md), [ADR 0030](0030-opt-in-local-agent-execution.md), [ADR 0033](0033-calendar-only-view.md)
Related Decision Logs: DL 0013 — user-mediated AI reports
Supersedes/Amends: Supersedes [ADR 0030](0030-opt-in-local-agent-execution.md) points 1 and 2 (points 3–6 stay in force). Amends [ADR 0022](0022-history-stays-local.md) points 1 and 2 in the same way ADR 0030 did
Decision owner: Repository maintainer

## Context

ADR 0030 made running an agent an opt-in exception to ADR 0022: only `kiroku serve --ai` could run one (point 1), and the run dialog said that the agent may send the prompt and the history it reads to its provider (point 2). Without `--ai`, the view offered "Copy prompt" buttons so the user could paste the prompt into an agent themselves.

On 2026-10-14 the maintainer decided to simplify kiroku: the copy buttons and the daily report were removed (#428), so in a `kiroku serve` started without `--ai` the report and review prompts had no use at all. The maintainer chose to make "Analyze with AI" available by default in `kiroku serve`, and to drop the provider sentence from the run dialog while keeping it in the documentation.

`CLAUDE.md` requires anything that widens access to be opt-in. Running an agent that may send history to its provider widens what leaves the machine. The maintainer chose default-on anyway; this ADR records that choice, the opt-out that stands in for the opt-in, and the protections that stay.

## Decision

1. **Default-on in `kiroku serve`, `--no-ai` to turn it off, each run confirmed** (supersedes ADR 0030 point 1).
   - `kiroku serve` offers "Analyze with AI" for the weekly and monthly report and the session review without any flag. The agents offered are the installed ones (`claude`, `codex`, `kiro-cli`); when none is installed the button is shown disabled with the reason.
   - `--no-ai` (or `KIROKU_NO_AI=1`, which `kiroku autostart on` records like the other `KIROKU_*` variables) turns agent runs off. Then `/ai/run` does not exist (404) and the view shows "Analyze with AI (off)" with how to turn it on.
   - Agent runs are also off whenever `kiroku serve` listens on an address that is not a loopback IP (such as `0.0.0.0:8484` or a host name). This keeps ADR 0030 point 5 without making such addresses fail, as `--ai` used to.
   - Every run is still confirmed in a dialog that shows the exact prompt, read-only, before anything runs.
   - A saved HTML file (`kiroku html`) never runs an agent and shows no AI action at all.
   - `--ai` is still accepted and does nothing except print a one-line notice that it is no longer needed. Under [ADR 0023](0023-compatibility-contract.md) an option stays accepted within a major version, so this is not `BREAKING`.
   - The default of `kiroku serve` changes (agent runs off → on). ADR 0023 point 1 lists defaults, but the maintainer classified this as `### Changed`, not `BREAKING`: every invocation that worked before still works (`--ai` included), no output that scripts read (`kiroku json`, exit status) changes, and nothing runs without a confirmation in the view.
   - This is the safeguard that stands in for the opt-in that `CLAUDE.md` asks for: one flag or environment variable, documented in `kiroku serve --help`, the guide and `SECURITY.md`, turns it off; runs never start without a confirmation; and nothing runs from a saved file or on a non-loopback address.
2. **Disclosure in the docs and the startup line, not in the dialog** (supersedes ADR 0030 point 2).
   - `kiroku serve` prints at startup that agent runs are on, that `--no-ai` turns them off, and that the agent may send the prompt and the history it reads to its provider. With `--no-ai` it says they are off.
   - `SECURITY.md`, README, README.ja and the guide say the same.
   - The run dialog keeps the exact prompt and the Run button, but no longer carries the provider sentence. The "Analyze with AI (off)" explanation still mentions it.
3. **ADR 0030 points 3–6 are unchanged.**
   - Fixed command lines, the prompt on stdin and no shell.
   - Least access per agent (Claude Code: Read only, history folders only, no MCP; Codex: read-only sandbox; an empty temporary working folder).
   - One run at a time, a 256 KiB request, a 2-minute limit and 1 MiB of output.
   - The serve key, the Host check, the `X-Kiroku` header and the same-origin check.
   - The reply rendered as text.
4. **ADR 0022 points 1 and 2** are amended as ADR 0030 amended them, with "only `kiroku serve --ai`" replaced by "`kiroku serve` unless started with `--no-ai`". kiroku itself still sends nothing over the network; the agent the user confirms does, under its own account and terms.

## Alternatives

- **Keep opt-in (`--ai`, ADR 0030 point 1).** It matches `CLAUDE.md`'s "widening is opt-in" literally. Without the copy buttons, though, the prompts would be unreachable in the default `kiroku serve`. Not chosen by the maintainer.
- **Bring back the copy buttons and keep opt-in.** This reverses #428's simplification. Not chosen.
- **Default-on but keep the provider sentence in the dialog (ADR 0030 point 2).** This keeps the notice where the user decides. The maintainer chose a shorter dialog, with the notice in the startup line and the docs. Not chosen.
- **Make `--ai` an error.** It would break scripts and `kiroku autostart` setups that pass it. Not chosen; it is accepted and does nothing.

## Consequences

- "History never leaves the machine" is true in `kiroku serve` only with `--no-ai`, or until the user confirms a run. README, the guide and `SECURITY.md` say so.
- A user who never reads the startup line or the docs sees no provider notice before a run; the dialog still shows the exact prompt and requires a confirmation. This is the cost of point 2.
- `kiroku autostart` now starts `kiroku serve` with agent runs on, unless `KIROKU_NO_AI=1` was set when running `kiroku autostart on`.
- The Kiro CLI open item from ADR 0030 (no read-only flag) now applies by default to users who have `kiro-cli` installed and confirm a run with it.
- Reversible: making `--no-ai` the default again restores ADR 0030 point 1 without data migration.
- Revisit if a provider-side change makes the agents send more than the prompt and what they read, if Kiro CLI gains a tool allowlist, or if users report runs they didn't expect.

## Verification

- `internal/cli`:
  - `TestAIEndpointNotFoundWhenOff`: `/ai/run` answers 404 when agent runs are off.
  - `TestAIOnlyOnLoopbackIP`: agent runs stay off for public binds, host names and shorthand IPv4 forms.
  - `TestAIStartLine`: the startup line says on or off, `--no-ai` and the provider disclosure.
  - `TestNoAIEnv`: `$KIROKU_NO_AI`.
  - `TestAIRunFlagInMeta` and `TestServeLive`: AI off means no `aiRun` in `/data.json`, and `/ai/run` answers 404.
- `internal/web` `TestAIRunDisclosure`: the run dialog shows the exact prompt, and `SECURITY.md`, README, README.ja and the guide carry the provider disclosure and `--no-ai`.
- `internal/ai` tests for ADR 0030 points 3–6 are unchanged.
- e2e:
  - `smoke.mjs`: a saved HTML file shows no AI action.
  - `serve.mjs`: `kiroku serve` without flags offers "Analyze with AI" (or shows it disabled with a reason when no agent is installed), and its dialog shows the prompt without running anything until Run is pressed.

## Open questions

- Kiro CLI read-only mode and Windows process-tree cleanup, carried over from ADR 0030.
