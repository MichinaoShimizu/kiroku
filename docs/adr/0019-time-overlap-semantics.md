# ADR 0019: Time overlap and usage allocation

Status: **Observed (retrospective)**

Scope: Phase 1 — local sessions.

## Evidence

[#54](https://github.com/MichinaoShimizu/kiroku/pull/54) introduced comparable shares by agent, project and branch. Overlapping active time was apportioned instead of fully counted for every category. [#285](https://github.com/MichinaoShimizu/kiroku/pull/285) distinguishes Active time, which counts overlaps once, from Total AI run time, which sums concurrent runs.

## Consequences

These metrics have different denominators and must not be treated as interchangeable. Display definitions alongside figures and keep tests aligned with allocation semantics.

## Verification limits

Inspect current `internal/report/share.go`, `docs/guide.md` and `TestShares`. No fresh arithmetic audit was performed.
