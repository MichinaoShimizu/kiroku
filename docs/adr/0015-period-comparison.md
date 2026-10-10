# ADR 0015: Comparable periods and explicitly estimated forecasts

Status: **Active**
Basis: retrospective
Accepted by: none recorded

Scope: Phase 1 — local sessions.

## Context and documented decision

[#70](https://github.com/MichinaoShimizu/kiroku/pull/70) corrected partial-week/month comparisons to use the same elapsed number of days in the previous period instead of comparing a partial period with a complete one. The same PR labeled end-of-month projections as estimates conditional on the current pace and excluded certain incomplete or boundary periods.

## Rationale and consequences

A comparison must disclose its denominator and time range; a projection is not an observation. Output metrics without reliable daily attribution should not be compared as though the partial-period data were complete.

## Historical confidence and verification

Inspect current comparison and forecast UI before treating the original #70 display as still present. The decision principle is supported; the present visibility of each tile is not independently established.

This ADR records documented changes, not an approval meeting or an unverified current behavior.
