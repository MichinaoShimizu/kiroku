# ADR 0004: Session identity and deduplication

Status: **Observed (retrospective)**

Scope: Phase 1 — local sessions.

## Context

Multiple files and agent components may represent the same user interaction. Summing them naively inflates prompts, time, token use and outputs.

## Documented decisions

- **Unify native agent readers through an adapter contract.** The Go rewrite introduced `Source` and a common session model ([#6](https://github.com/MichinaoShimizu/kiroku/pull/6)).
- **Avoid Crew/Kiro double counts.** Crew's own logs are used for identity/title/provenance while Kiro CLI provides the underlying conversation and usage when they refer to the same work ([#8](https://github.com/MichinaoShimizu/kiroku/pull/8)).
- **Prefer canonical source copies.** Older Kiro CLI SQLite formats share an adapter with Amazon Q, with precedence for `conversations_v2` on duplicate IDs ([#7](https://github.com/MichinaoShimizu/kiroku/pull/7)).
- **Handle forks, migration and mirrored events.** Subsequent fixes addressed Crew copied rows ([#242](https://github.com/MichinaoShimizu/kiroku/pull/242)), Kiro IDE migrated histories ([#308](https://github.com/MichinaoShimizu/kiroku/pull/308)), Codex review mirrors ([#310](https://github.com/MichinaoShimizu/kiroku/pull/310)), Claude branch/fork copies ([#311](https://github.com/MichinaoShimizu/kiroku/pull/311)), and Crew duplicate rows ([#312](https://github.com/MichinaoShimizu/kiroku/pull/312)).

## Observed implementation

`internal/cli/load.go` uses `Builder.Key` and `Claim/Yield` in collection; `internal/core/session.go` defines the normalized identity and fields. These are **current-code observations**, not a claim that their exact semantics were established in one historical PR.

## Rationale and consequences

One human action must not become multiple metrics merely because an agent stores multiple representations. Identity rules require agent-specific tests, not just generic file-hash deduplication.

For Phase 2, the same upstream session IDs can appear on different machines; origin-aware identity is a **proposal**, not yet an accepted implementation ([ADR 0014](0014-remote-session-import.md)).

## Alternatives and verification

No historical evidence is claimed for unmentioned identity alternatives. Verify against source fixtures and the cited PRs; no new tests were run for this ADR.
