---
kind: log
category: privacy
keywords: [memory_mode, incognito, temporary, HideText, Withholder, ClaimWithholder, ClaimScanner, crewPrivateSids, claudeClaim]
---

# [kiroku] Conversations an agent marks private are shown as counts and times only, in every copy kiroku reads

> A past observation, not a decision. It binds nothing.

## What happened

- C1 2026-10-10 Still true: Kiro Crew conversations in `incognito` or `temporary` memory mode keep only counts and times: no text, title, replies, files, PR links, subagent tasks or history file. The marker is looked for in every metadata line, `session_map.json` flags and subagent state; anything not `persistent` counts as private (fail closed). The Kiro CLI SQLite copy of such a conversation is hidden too. History: [PR #312](https://github.com/MichinaoShimizu/kiroku/pull/312) / Now: [`internal/core/session.go` `HideText` @`1d7b409`](https://github.com/MichinaoShimizu/kiroku/blob/1d7b4091906cc02d4f32bdabd120f5120dcf0792/internal/core/session.go), [`internal/source/crew.go` `crewPrivateTitle` @`1d7b409`](https://github.com/MichinaoShimizu/kiroku/blob/1d7b4091906cc02d4f32bdabd120f5120dcf0792/internal/source/crew.go)
- C2 2026-10-14 Still true: the Claude Code and Codex copies of a private Crew conversation are hidden as well, also when `--sources` leaves Kiro out (Crew metadata is scanned anyway, and serve watches it). An unreadable `session_map.json` hides every ID found in it. History: [PR #392](https://github.com/MichinaoShimizu/kiroku/pull/392) / Now: [`internal/source/crew.go` `crewPrivateSids` @`1d7b409`](https://github.com/MichinaoShimizu/kiroku/blob/1d7b4091906cc02d4f32bdabd120f5120dcf0792/internal/source/crew.go), [`internal/source/source.go` `ClaimScanner` @`1d7b409`](https://github.com/MichinaoShimizu/kiroku/blob/1d7b4091906cc02d4f32bdabd120f5120dcf0792/internal/source/source.go)

## Reason

Documented (description, AI): Crew itself treats these modes as conversations nothing may be learned from and refuses to export or summarize them, but kiroku showed them in full (checked against Crew's public source on 2026-10-09). Crew's cleanup deletes only kiro-cli files, so the Claude and Codex transcripts remained ([PR #312](https://github.com/MichinaoShimizu/kiroku/pull/312), [PR #392](https://github.com/MichinaoShimizu/kiroku/pull/392)).

#312 tried and reverted making `kiroku archive` keep Crew conversation logs, saying that widening what the archive stores is a separate decision. Limits it documents: unlinked kiro-cli conversations, temporary subagents without `state.json`, and git subjects and tool names, which are still shown.

## People involved

From the PR records; not necessarily who decided.

- PR #312: opened @MichinaoShimizu (2026-10-10), no approving review, merged @MichinaoShimizu (2026-10-10)
- PR #392: opened @MichinaoShimizu (2026-10-14), no approving review, merged @MichinaoShimizu (2026-10-14)

## Open questions

- #392 assumes the Claude transcript file name equals the ACP session ID. Was that confirmed upstream? (ask @MichinaoShimizu)
- Is showing git commit subjects inside a private conversation's time window an accepted exposure? (ask @MichinaoShimizu)
- Do the reasons above, written by an AI agent in the PR descriptions, match the maintainer's own judgement? (ask @MichinaoShimizu)

## Sources

- [old Decision Log 0003 (removed)](https://github.com/MichinaoShimizu/kiroku/blob/ef66c137df557123c0f71a342dbf2f82be064131/docs/decision-log/0003-local-privacy-boundary.md)
- [ADR 0022 (removed)](https://github.com/MichinaoShimizu/kiroku/blob/27e8298bdc0367afc623bc4db5c450ca0bad38b1/docs/adr/0022-history-stays-local.md)
