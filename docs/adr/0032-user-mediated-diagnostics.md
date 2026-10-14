# ADR 0032: User-mediated diagnostics: kinds and counts the user reviews and shares

Status: Accepted (2026-10-14, approved by the maintainer)
Date: 2026-10-14
Scope: Format diagnostics and problem reports — `kiroku doctor` ("Format check", `--report`), the view's "Data sources" (`internal/source`, `internal/core/format.go`, `internal/cli/diagnose.go`, `internal/web/js/panels.js`, `SECURITY.md`)
Related ADRs: [ADR 0022](0022-history-stays-local.md) (history stays on the machine), [ADR 0023](0023-compatibility-contract.md) (new `kiroku json` fields only)
Related Decision Logs: [DL 0013 — user-mediated AI reports](../decision-log/0013-user-mediated-ai-reports.md)
Supersedes/Amends: None
Decision owner: Repository maintainer

## Context

Agents change their history formats without notice (Claude Code's transcript and Codex's rollout are undocumented and change most often). When that happens kiroku silently skips what it doesn't know, and numbers drift without anyone noticing. The maintainer only learns about it when a user files an issue, and a useful issue needs to say which agent version wrote the history and what kiroku didn't recognize.

History is private ([ADR 0022](0022-history-stays-local.md)): kiroku has no telemetry and sends nothing derived from history over the network. Any diagnostic therefore has to reach the maintainer through the user, and must be safe for the user to share publicly.

## Decision

1. **kiroku never sends diagnostics.** There is no automatic or opt-in upload. kiroku shows a report (`kiroku doctor --report`) that the user reads and, if they choose, pastes into a GitHub issue themselves.
2. **Contents are limited to kinds and counts:** kiroku's version, OS and architecture, and per agent the number of sessions, the newest agent version seen in history, the number of lines that couldn't be parsed, and record kinds kiroku doesn't know with their counts. No paths, project names, branch names, session or message IDs, prompt or reply text, or the home folder.
3. **Names and versions from history are sanitized** before they are stored or shown: a kind name is used only if it matches `[A-Za-z0-9_.:-]{1,40}`, otherwise it is counted under `other`; at most 20 distinct names per agent are kept (the rest under `other`); a version is kept only as digits and dots (at most 4 parts, 20 characters, any `-…`/`+…` suffix dropped), otherwise dropped. Output code checks again before writing.
4. **"Unknown" is conservative.** Each source lists the kinds it reads and the kinds it knows and intentionally ignores; only kinds outside both lists are counted. Claude Code and Codex are checked first, as their formats change most often; other sources report session counts only and say that kinds are not checked.
5. The same fields appear in `kiroku doctor` ("Format check"), the view's "Data sources" (escaped like all history), and `kiroku json` (`meta.report[]`: `unknown`, `badLines`, `agentVersion`, `kindsChecked`, new optional fields per ADR 0023). Files exported for one period (`kiroku html --week`/`--month`) leave them out, as they describe all history.

## Alternatives

- **Automatic or opt-in telemetry of format errors.** Would find format changes fastest, but contradicts ADR 0022 point 1. Not chosen.
- **Include sample lines or the raw `type` values unchanged.** More useful for debugging, but a sample line contains prompts and paths, and a raw value can carry anything a damaged or hostile file puts there. Not chosen; the maintainer asks for a minimal synthetic sample in the issue when needed.
- **Nothing in kiroku; ask users to describe problems.** Users can't see what kiroku skipped, so format changes go unreported. Not chosen.

## Consequences

- Format changes become visible to users (and reportable) without anything leaving the machine unless they paste it.
- A kind name that happens to match the allowed shape and equals a project name (e.g. a malformed line whose `type` is `acme-api`) could appear in the report; that requires a damaged history, the user reviews the text before sharing, and the report says it contains kinds and counts only.
- Known/ignored lists need upkeep when agents add kinds; a missed entry shows up as a harmless "unknown" until the list is updated.
- Reading cost: one map lookup per line plus counting; the read benchmark must stay within about 5%.

## Verification

- `internal/cli`: `TestDiagnosticsHaveNoHistory` (synthetic history with paths, project names, IDs, prompt and reply text; none appears in `--report`), `TestFormatCheckQuiet`
- `internal/core`: `TestSafeKind`, `TestSafeVersion`, `TestFormatCaps`, `TestReadJSONLBad`
- `internal/source`: `TestClaudeFormat`, `TestCodexFormat`, `TestFormatTestdataClean`

## Open questions

- Version, unreadable-line and unknown-kind tracking for Kiro IDE / CLI, Kiro CLI, Kiro Crew and the SQLite stores (not tracked today).
