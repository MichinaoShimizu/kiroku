# ADR 0010: Native source adapters and recording coverage

Status: **Active**
Basis: retrospective
Accepted by: none recorded

Scope: Phase 1 — local sessions.

## Context

This decision area evolved through multiple merged PRs. This record preserves the documented tradeoffs without inventing historical approvals or alternatives.

## Documented decision and evolution

[#6](https://github.com/MichinaoShimizu/kiroku/pull/6) introduced a Go `Source` contract. [#7](https://github.com/MichinaoShimizu/kiroku/pull/7) reads legacy Kiro/Amazon Q SQLite and prefers `conversations_v2` on duplicates. [#8](https://github.com/MichinaoShimizu/kiroku/pull/8) uses Crew metadata without double-counting CLI conversations. [#9](https://github.com/MichinaoShimizu/kiroku/pull/9) reads Codex rollouts and handles cumulative token records. [#214](https://github.com/MichinaoShimizu/kiroku/pull/214) centralizes which metrics each source can record. [#247](https://github.com/MichinaoShimizu/kiroku/pull/247) centralizes agent registry metadata in `core.Agents`.

## Rationale and consequences

A shared session model does not imply identical source coverage. Unknown, absent and zero are distinct. New agents require registry, parser, fixture, UI, archive and documentation review.

## Alternatives and historical confidence

Earlier approaches and reversals are listed only where linked PRs document them. This ADR is descriptive, not a declaration that all future alternatives are rejected.

## Verification

Current implementation: `internal/core/agents.go`, `internal/core/records.go`, `internal/source/source.go`, and `docs/sources.md`. The per-agent matrix is maintained in those sources, not duplicated as immutable ADR facts.
