---
kind: log
category: data
keywords: [kiroku archive, Sync, Keeper, .jsonl.zst, KIROKU_ARCHIVE_DIR, archive_retention_days, Retention]
---

# [kiroku] kiroku keeps copies of history that agents delete, only after the user turns it on

> A past observation, not a decision. It binds nothing.

## What happened

- C1 2026-10-04 Still true: `kiroku archive` is off until `kiroku archive on`. It then copies Claude Code and Kiro Crew history as `.jsonl.zst` under the same relative paths in kiroku's data folder (`--archive-dir`, `KIROKU_ARCHIVE_DIR`), and a copy is read only when the original is gone. History: [PR #75](https://github.com/MichinaoShimizu/kiroku/pull/75) / Now: [`internal/archive/archive.go` `Sync` @`1d7b409`](https://github.com/MichinaoShimizu/kiroku/blob/1d7b4091906cc02d4f32bdabd120f5120dcf0792/internal/archive/archive.go), [`internal/source/source.go` `Keeper` @`1d7b409`](https://github.com/MichinaoShimizu/kiroku/blob/1d7b4091906cc02d4f32bdabd120f5120dcf0792/internal/source/source.go)
- C2 2026-10-05 Still true: a symlinked history root is followed, and a resumed conversation reads its deleted subagents from the copies. History: [PR #86](https://github.com/MichinaoShimizu/kiroku/pull/86) / Now: [`internal/archive/archive.go` `Sync` @`1d7b409`](https://github.com/MichinaoShimizu/kiroku/blob/1d7b4091906cc02d4f32bdabd120f5120dcf0792/internal/archive/archive.go)
- C3 2026-10-07 Still true: the retention notice that suggests the archive reads Kiro Crew's own `session.archive_retention_days` instead of reporting it as unknown. History: [PR #197](https://github.com/MichinaoShimizu/kiroku/pull/197) / Now: [`internal/source/crew.go` `crewRetentionDays` @`1d7b409`](https://github.com/MichinaoShimizu/kiroku/blob/1d7b4091906cc02d4f32bdabd120f5120dcf0792/internal/source/crew.go)

## Reason

Documented (description, AI): Claude Code deletes conversations older than 30 days by default and Kiro Crew deletes archived segments, and kiroku could only warn about it; the archive is off by default ([PR #75](https://github.com/MichinaoShimizu/kiroku/pull/75), [PR #86](https://github.com/MichinaoShimizu/kiroku/pull/86), [PR #197](https://github.com/MichinaoShimizu/kiroku/pull/197)). Why opt-in was written down only later, in ADR 0025 (2026-10-11): a copy duplicates conversations the user may have deleted on purpose.

## People involved

From the PR records; not necessarily who decided.

- PR #75: opened @MichinaoShimizu (2026-10-04), no approving review, merged @MichinaoShimizu (2026-10-04)
- PR #86: opened @MichinaoShimizu (2026-10-05), no approving review, merged @MichinaoShimizu (2026-10-05)
- PR #197: opened @MichinaoShimizu (2026-10-07), no approving review, merged @MichinaoShimizu (2026-10-07)

## Open questions

- Was opt-in rather than on-by-default considered at the time of #75, or only in ADR 0025? (ask @MichinaoShimizu)
- Why copy only Claude Code and Kiro Crew, and not Codex? (ask @MichinaoShimizu)
- Do the reasons above, written by an AI agent in the PR descriptions, match the maintainer's own judgement? (ask @MichinaoShimizu)

## Sources

- [ADR 0025 (removed)](https://github.com/MichinaoShimizu/kiroku/blob/27e8298bdc0367afc623bc4db5c450ca0bad38b1/docs/adr/0025-opt-in-history-copies.md)
- [old Decision Log 0007 (removed)](https://github.com/MichinaoShimizu/kiroku/blob/ef66c137df557123c0f71a342dbf2f82be064131/docs/decision-log/0007-local-history-retention.md)
