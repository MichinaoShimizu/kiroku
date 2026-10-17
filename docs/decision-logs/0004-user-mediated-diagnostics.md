---
kind: log
category: privacy
keywords: [kiroku doctor --report, formatCheck, SafeKind, SafeVersion, FormatChecker, badLines, agentVersion]
---

# [kiroku] Format problems reach the maintainer only through the user, as kinds and counts

> A past observation, not a decision. It binds nothing.

## What happened

- C1 2026-10-14 Still true: `kiroku doctor` has a "Format check", and `kiroku doctor --report` prints only kiroku's version, OS and architecture, and per agent the number of sessions, the newest agent version, unreadable lines and unknown record kinds with counts. Kind names must match `[A-Za-z0-9_.:-]{1,40}` (else `other`), at most 20 per agent, and versions keep only digits and dots. kiroku sends nothing; the user reads and pastes it. History: [PR #423](https://github.com/MichinaoShimizu/kiroku/pull/423) / Now: [`internal/cli/diagnose.go` `diagnostics` @`1d7b409`](https://github.com/MichinaoShimizu/kiroku/blob/1d7b4091906cc02d4f32bdabd120f5120dcf0792/internal/cli/diagnose.go), [`internal/core/format.go` `SafeKind` @`1d7b409`](https://github.com/MichinaoShimizu/kiroku/blob/1d7b4091906cc02d4f32bdabd120f5120dcf0792/internal/core/format.go), [`SECURITY.md` @`1d7b409`](https://github.com/MichinaoShimizu/kiroku/blob/1d7b4091906cc02d4f32bdabd120f5120dcf0792/SECURITY.md)

## Reason

Documented (description, AI): agents change their formats without notice and kiroku skips what it doesn't know, but history never leaves the machine, so the signal has to go through the user ([PR #423](https://github.com/MichinaoShimizu/kiroku/pull/423)). Alternatives the PR's ADR 0032 lists as not chosen: automatic or opt-in telemetry, sample lines or raw `type` values (they carry prompts and paths), and nothing at all.

## People involved

From the PR records; not necessarily who decided.

- PR #423: opened @MichinaoShimizu (2026-10-14), no approving review, merged @MichinaoShimizu (2026-10-14)

## Open questions

- The PR says the design was "approved by the maintainer on 2026-10-14". Where did that happen? (ask @MichinaoShimizu)
- Format tracking for the Kiro sources and the SQLite readers is still "not checked yet". Is it planned? (ask @MichinaoShimizu)
- Do the reasons above, written by an AI agent in the PR descriptions, match the maintainer's own judgement? (ask @MichinaoShimizu)

## Sources

- [ADR 0032 (removed)](https://github.com/MichinaoShimizu/kiroku/blob/27e8298bdc0367afc623bc4db5c450ca0bad38b1/docs/adr/0032-user-mediated-diagnostics.md)
