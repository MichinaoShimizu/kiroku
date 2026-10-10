# ADR 0009: Git output is evidence, not business outcome

Status: **Retrospective**

Scope: Phase 1 — local sessions.

## Context

This decision area evolved through multiple merged PRs. This record preserves the documented tradeoffs without inventing historical approvals or alternatives.

## Documented decision and evolution

Git commits complement agent-tool events rather than proving business value. [#27](https://github.com/MichinaoShimizu/kiroku/pull/27) introduced successful Claude tool invocations as output proxies, explicitly excluding failed/unrecorded tool results. [#28](https://github.com/MichinaoShimizu/kiroku/pull/28) added local Git commits including human-authored work; it associates AI tool commits with Git records using a ±2-minute timestamp heuristic, not cryptographic provenance. [#112](https://github.com/MichinaoShimizu/kiroku/pull/112) removed PR counts and AI-edited lines from key views because only partial agent histories recorded them. [#293](https://github.com/MichinaoShimizu/kiroku/pull/293) prevented implicit remote Git fetching, even from partial clones.

## Rationale and consequences

The output count is an observable proxy, not a causal measure of AI contribution or business outcome. Git enrichment reads only local repositories; missing local repositories or unrecorded manual/remote activity limits coverage.

## Alternatives and historical confidence

Earlier approaches and reversals are listed only where linked PRs document them. This ADR is descriptive, not a declaration that all future alternatives are rejected.

## Verification

Current implementation: `internal/gitlog/gitlog.go` defines `Commit.AI` as a time-matched heuristic and documents local push-reflog limitations. Verify any later output metrics against `internal/core/output.go` and current report UI.
