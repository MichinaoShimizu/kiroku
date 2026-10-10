# ADR 0019: Time overlap and usage allocation

Status: **Observed (retrospective)**

Scope: Phase 1 — local sessions.

## Evidence

[#54](https://github.com/MichinaoShimizu/kiroku/pull/54) introduced comparable shares by agent, project and branch. Overlapping active time was apportioned instead of fully counted for every category. Current `internal/report/share.go` allocates each active minute equally among **distinct grouping keys** present in that minute, not once per session; multiple concurrent sessions in the same grouping key share one key's allocation. `internal/report/share_test.go` (`TestShares`) asserts representative overlaps and usage attribution. [#285](https://github.com/MichinaoShimizu/kiroku/pull/285) distinguishes Active time, which counts overlaps once, from Total AI run time, which sums concurrent runs.

## Consequences

These metrics have different denominators and must not be treated as interchangeable. Display definitions alongside figures and keep tests aligned with allocation semantics.

## Verification limits

Inspect current `internal/report/share.go`, `docs/guide.md` and `TestShares`. `TestShares` passed at `ad670cb` (see [evidence.yaml](evidence.yaml)); it does not assert the same-key case, and the Active time vs Total AI run time aggregation has not been audited.
