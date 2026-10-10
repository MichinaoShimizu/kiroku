# ADR 0016: Maintainable web source with single-file output

Status: **Observed (retrospective)**

Scope: Phase 1 — local sessions.

## Context and documented decision

[#1](https://github.com/MichinaoShimizu/kiroku/pull/1) established a standalone HTML deliverable. [#93](https://github.com/MichinaoShimizu/kiroku/pull/93) separated the 260 KB monolithic web template into HTML, CSS and JavaScript source files while preserving byte-for-byte identical assembled output through Go embedding. Subsequent UI modules evolved further; the distributable remains a generated local view rather than a hosted web application.

## Rationale and consequences

Internal modularity and an easily shared, local one-file report are compatible. Template assembly must be tested, including script/CSP integrity and cross-platform line endings.

## Historical confidence and verification

Inspect current `internal/web` build/assembly and its tests; the byte-for-byte claim refers to the #93 migration, not to all later releases.

This ADR records documented changes, not an approval meeting or an unverified current behavior.
