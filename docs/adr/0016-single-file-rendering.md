# ADR 0016: Maintainable web source with single-file output

Status: Retrospective
Date: Unknown
Related: None
Supersedes: None
Superseded by: None

Scope: Phase 1 — local sessions

## Context and documented decision

[#1](https://github.com/MichinaoShimizu/kiroku/pull/1) established a standalone HTML deliverable. [#93](https://github.com/MichinaoShimizu/kiroku/pull/93) separated the 260 KB monolithic web template into HTML, CSS and JavaScript source files while preserving byte-for-byte identical assembled output through Go embedding. Subsequent UI modules evolved further; the distributable remains a generated local view rather than a hosted web application.

## Rationale and consequences

Internal modularity and an easily shared, local one-file report are compatible. Template assembly must be tested, including script/CSP integrity and cross-platform line endings.

## Historical confidence and verification

Current `internal/web/web.go` embeds the HTML skeleton, CSS and JavaScript modules and assembles them for rendering. `internal/web/web_test.go` contains `TestAssemble` (LF/CRLF replacement) and `TestCSP` (inline asset hashes). These tests were inspected in the initial audit and passed at `ad670cb` in the follow-up (see [evidence.yaml](evidence.yaml)). The byte-for-byte claim refers to the #93 migration, not to all later releases.

This ADR records documented changes, not an approval meeting or an unverified current behavior.

## Decision

The recovered decisions remain in the original evidence and decision-history sections above; no new interpretation is introduced.

## Consequences

See the existing rationale and consequences discussion above. No additional outcome is asserted.

## Alternatives

Not documented as a separate set of evaluated options in this record.

## Historical confidence and open questions

Rationale is limited to the linked historical evidence. Unrecorded alternatives and approval history remain unknown.

## Verification

See existing source and test references where available. No new test execution is claimed.
