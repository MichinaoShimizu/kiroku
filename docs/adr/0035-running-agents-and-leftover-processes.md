# ADR 0035: Show running agents and the processes they left running, and stop them on request

Status: Proposed
Date: 2026-10-16
Scope: Reading this user's processes and stopping some of them (`internal/procs`, `internal/cli` `ps`, serve `/procs` and `/procs/stop`, `--no-procs` / `$KIROKU_NO_PROCS`, `internal/web/js/procs.js`, `SECURITY.md`, the guide)
Related ADRs: [ADR 0022](0022-history-stays-local.md), [ADR 0023](0023-compatibility-contract.md), [ADR 0030](0030-opt-in-local-agent-execution.md), [ADR 0033](0033-calendar-only-view.md), [ADR 0034](0034-ai-analysis-by-default.md)
Related Decision Logs: None
Supersedes/Amends: None (extends what kiroku reads beyond history; see Context)
Decision owner: Repository maintainer

## Context

AI coding agents start processes for the user: a dev server, a watcher, a build. Many are still running after the agent or its session ended. Claude Code's tracker has a run of reports: background processes from the Bash tool not cleaned up on exit (anthropics/claude-code#43944), a command ending in `&` recorded as finished while it keeps running (#93126, open), a child orphaned under PID 1 using a full core for 10 days (#81462, open), `claude` itself left running after `/exit` or a closed terminal at 10 GB (#18752, #21305), and shells from one session running inside another (#50865). Today the user finds them with `ps` and stops them with `kill`.

kiroku already knows each session's project folder, agent and times, so it can tell the user which session a process came from. No other tool compared in the maintainer's review (ccusage, history viewers, the official analytics) shows this. On 2026-10-16 the maintainer chose option C: list the processes, link them to sessions, and let the view stop one after a confirmation.

Until now kiroku read only history, git and GitHub Releases. Reading processes, and sending them signals, is a new kind of access. ADR 0022 does not cover it: no process data leaves the machine, so points 1 and 2 still hold, but `kiroku serve` gains an endpoint that changes the state of the computer. `CLAUDE.md` asks that anything widening what `kiroku serve` exposes be opt-in and documented in `SECURITY.md`.

## Decision

1. **What kiroku reads.**
   - Only the current user's processes. On Linux, `/proc`. On macOS, `/bin/ps` and `/usr/sbin/lsof` with fixed arguments and no shell. Windows and other systems are not supported: `kiroku ps` says so, and the view shows nothing.
   - From each process: the PID and parent PID, the start time, the short process name, the working folder, whether a terminal is attached, memory and CPU time.
   - The environment is read only to look for agent markers (`CLAUDECODE=1` for Claude Code's shell commands, `CODEX_SANDBOX` and `CODEX_SANDBOX_NETWORK_DISABLED=1` for Codex's sandboxed commands). Values are never kept. The command line is never read on Linux. On macOS, `ps -E` prints it with the environment, and kiroku keeps only the marker. Neither appears in any output.
2. **What kiroku reports.**
   - **Agents**: processes named `claude`, `codex` (or `codex-*`), `kiro-cli`, `kiro-cli-chat` or `qchat`, with every process under them. An agent started by another agent belongs to the outer one.
   - **Left running**: processes that carry a marker and have no running agent above them, with every process under them.
   - Each group is linked to a session of the same agent family whose project folder contains the process's folder. For an agent, the session must have been active after the process started; for a process left running, the process must have started while the session was going on (±60 s).
   - kiroku's own children (its "Analyze with AI" runs) are never reported. A group kiroku itself runs in is marked and cannot be stopped from kiroku.
3. **`kiroku ps` only reads.** It prints the groups and the `kill` command for each; `--json` prints the same fields as `/procs`.
4. **`kiroku serve` shows and stops, on by default, with an opt-out.**
   - `GET /procs` returns the groups.
   - `POST /procs/stop` sends SIGTERM, or SIGKILL when the user confirms "Force stop" after SIGTERM didn't work, to the root of a group and every process under it.
   - Both paths exist only when `kiroku serve` listens on a loopback IP (the same rule as "Analyze with AI", ADR 0034), the OS is supported, and `--no-procs` / `KIROKU_NO_PROCS` is not set. Otherwise both answer 404. `kiroku autostart` records `KIROKU_NO_PROCS` like the other variables.
   - `/procs/stop` keeps the key, the Host check, the `X-Kiroku` header and the same-origin check, like `/archive`. It takes only `{pid, start, force}` (at most 1 KiB) and refuses anything that is not the root of a group kiroku finds again at that moment with the same start time. A reused PID, a member, any other process, PID 1 and kiroku's own group are all refused. Before each signal, kiroku checks that the process still has the same start time.
   - The view asks for a confirmation that names the process, its PID and how many processes are under it, before anything is sent. kiroku serve's terminal logs each stop.
   - The startup line says the list is on and that `--no-procs` turns it off.
5. **The view.** A button next to LIVE shows how many processes are left running (in the warning color) or how many agents are running. It opens a dialog with both lists. Session details show "Still running" for that session. Process names and folders are escaped like history. The saved HTML file has none of this. This adds a dialog and a section to the calendar-only view of ADR 0033; it adds no view.

## Alternatives

- **A: guess from history only** (background commands with no recorded stop). It needs no new access, but it can't say whether anything is still running, so it would mostly raise false alarms. Not chosen; it may be added later as a hint next to sessions.
- **B: list only, no stop.** It keeps `kiroku serve` read-only. The maintainer chose to let the view stop processes, since copying `kill` commands is the step users skip. `kiroku ps` keeps the read-only form.
- **Opt-in (`--procs`) instead of opt-out.** It follows `CLAUDE.md`'s "widening is opt-in" literally. It would hide the feature from most users, and ADR 0034 set the precedent of default-on with an opt-out, loopback only, and a confirmation for each action. This is the main tradeoff for the maintainer to accept or reject.
- **Kill by process group.** Simpler, but an agent's commands often share the user's shell's group, and a process left running may have started its own group. Walking the parent tree is more precise.
- **Showing command lines** would identify processes better, but they often hold tokens and passwords. Not chosen.

## Consequences

- kiroku reads more than history. It reads only metadata of the user's own processes, and nothing leaves the machine (ADR 0022 point 1 unchanged), but `SECURITY.md` must describe it.
- A page that has the serve key can stop the user's agent-related processes. That's the same trust as the key already gives (reading all history, running "Analyze with AI"). It is bounded to agents and their descendants, PIDs re-checked by start time, and confirmation in the view.
- False positives: a process the user meant to keep, such as a server started on purpose with `nohup` through the agent, shows as "left running". The wording says what kiroku saw ("left running after Claude Code ended"), not that it should be stopped.
- False negatives: agents that set no marker (Kiro CLI, Amazon Q) report only their own running processes, not what they left. Codex's markers exist only for sandboxed commands. Revisit when agents add markers.
- macOS support relies on the output format of `ps` and `lsof` and is not covered by CI, which runs on Linux. The parsers are tested with recorded output.
- `/procs` is polled every 15 seconds while the view is visible. That's cheap with `/proc`, and on macOS it runs `ps` and `lsof`.
- Reversible: `--no-procs` turns it off. Removing the feature needs no data migration. `kiroku ps` and `--no-procs` become part of the CLI kept compatible under ADR 0023 once released.

## Verification

- `internal/procs`: `TestClassify`, `TestClassifySelf`, `TestLink`, `TestMarkerOf`, `TestAgentFamily`, `TestParseStat`, `TestParsePS`, `TestParsePSEnv`, `TestParseLsof`, `TestStopChecksIdentity`. On Linux, `TestListFindsAndStopsLeftRunning` (a real process left running with Claude Code's marker is found with its folder and stopped), `TestListOnlyThisUser` and `TestEnvironMarkerBounded` (a process can make its environment as large as it likes; kiroku reads at most 4 MiB in pieces).
- `internal/cli`:
  - `TestProcsNotFoundWhenOff`, `TestProcsList` (no command line or environment in the JSON), `TestProcsFlagInMeta`, `TestProcsStartLine`, `TestNoProcsEnv`, `TestWritePS` and `TestStopLineIsTerminalSafe` (terminal control characters removed).
  - `TestProcsStopRefusals`: GET, a missing `X-Kiroku`, another origin, a bad or oversized body, PID 1, a reused PID, a member, an unreported process and kiroku's own group are all refused.
  - On Linux, `TestProcsStopEndsProcess`.
- The hostile-tester run before the PR: `/procs/stop` refused every abuse tried, names and folders were escaped. It found unbounded memory on a huge environment and an unescaped name in the stop log; both fixed with the tests above.
- The user-tester run before the PR: wording that claimed more than kiroku knows ("after Claude Code ended" while it still ran; "never reads the environment"), focus lost on redraws, a hidden result message, a destructive default focus and an overflowing header on phones were fixed.
- Manual: `kiroku serve` in a cloud container, with a process left running from a fake session and real orphaned `chrome_crashpad` processes left by Playwright under Claude Code. Both were listed, linked and stopped from the view in English and Japanese. Not yet run on a real Mac.

## Open questions

- Opt-in or opt-out (Alternatives). The implementation follows ADR 0034's opt-out until the maintainer decides.
- Whether `kiroku doctor` should mention processes left running.
- Markers for Kiro CLI and Amazon Q, if they ever set one.
