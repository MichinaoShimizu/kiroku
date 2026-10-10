# ADR 0011: Release integrity and provenance

Status: **Retrospective**

Scope: Phase 1 — local sessions.

## Context

This decision area evolved through multiple merged PRs. This record preserves the documented tradeoffs without inventing historical approvals or alternatives.

## Documented decision and evolution

[#56](https://github.com/MichinaoShimizu/kiroku/pull/56) automates tagging/releases from versioned CHANGELOG changes. [#144](https://github.com/MichinaoShimizu/kiroku/pull/144) adds build-provenance attestations for release artifacts and checksums. [#167](https://github.com/MichinaoShimizu/kiroku/pull/167) adds CodeQL, OpenSSF Scorecard, SBOMs and a reproducibility check. [#170](https://github.com/MichinaoShimizu/kiroku/pull/170) adds fuzz targets and release provenance bundle handling.

## Rationale and consequences

Checksums detect accidental or malicious artifact changes only relative to a trusted checksum source; attestations and reproducibility add independently checkable evidence. Security scans are safeguards, not guarantees.

## Alternatives and historical confidence

Earlier approaches and reversals are listed only where linked PRs document them. This ADR is descriptive, not a declaration that all future alternatives are rejected.

## Verification

Verify release workflows in `.github/workflows`, `.goreleaser.yaml`, `tools/reproduce.sh`, `SECURITY.md`, and actual published release artifacts. This ADR does not claim a fresh release build or external verification.
