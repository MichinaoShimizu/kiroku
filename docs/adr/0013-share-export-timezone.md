# ADR 0013: Scoped exports and time-zone fidelity

Status: **Observed (retrospective)**

Scope: Phase 1 — local sessions.

## Context

This decision area evolved through multiple merged PRs. This record preserves the documented tradeoffs without inventing historical approvals or alternatives.

## Documented decision and evolution

[#124](https://github.com/MichinaoShimizu/kiroku/pull/124) adds period-scoped `kiroku html --week/--month` exports alongside local diagnostics/autostart. [#127](https://github.com/MichinaoShimizu/kiroku/pull/127) fixes the writer/viewer time-zone discrepancy: exported scoped pages use the writer's time zone and carry `meta.scope.offset`/`zone`. [#28](https://github.com/MichinaoShimizu/kiroku/pull/28) notes exported HTML can include Git commit messages.

## Rationale and consequences

A shareable file is not automatically safe to share. The scope restricts time, not necessarily the sensitivity of paths, prompts, commit messages or repository information. Date boundaries must not silently change when a file opens in another time zone.

## Alternatives and historical confidence

Earlier approaches and reversals are listed only where linked PRs document them. This ADR is descriptive, not a declaration that all future alternatives are rejected.

## Verification

Review `internal/web`, export/period tests, `docs/guide.md`, and `SECURITY.md`. This ADR records the observed approach rather than asserting every export field has been privacy-audited.
