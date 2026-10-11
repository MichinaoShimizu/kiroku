# Decision Log 0019: Time overlap and usage allocation

Status: Historical observation (not an accepted decision)
Historical decision date: Unknown
Earliest cited merged PR: [#54](https://github.com/MichinaoShimizu/kiroku/pull/54) (2026-10-04 UTC; evidence anchor, not an approval date)

Scope: Phase 1 — local sessions

## Context

This record concerns how overlapping sessions contribute to category shares and active-time reporting.

## Decision

[#54](https://github.com/MichinaoShimizu/kiroku/pull/54) introduced comparable shares by agent, project and branch. Overlapping active time was apportioned instead of fully counted for every category. Current `internal/report/share.go` allocates each active minute equally among **distinct grouping keys** present in that minute, not once per session; multiple concurrent sessions in the same grouping key share one key's allocation. `internal/report/share_test.go` (`TestShares`) asserts representative overlaps and usage attribution. [#285](https://github.com/MichinaoShimizu/kiroku/pull/285) distinguishes Active time, which counts overlaps once, from Total AI run time, which sums concurrent runs.

## Consequences

These metrics have different denominators and must not be treated as interchangeable. Display definitions alongside figures and keep tests aligned with allocation semantics.

## Verification

Inspect current `internal/report/share.go`, `docs/guide.md` and `TestShares`. `TestShares` passed at `ad670cb` (see [evidence.yaml](../adr/evidence.yaml)); it does not assert the same-key case, and the Active time vs Total AI run time aggregation has not been audited.

## Alternatives

Not documented as a separate set of evaluated options in this record.

## Historical confidence and open questions

Rationale is limited to the linked historical evidence. Unrecorded alternatives and approval history remain unknown.
