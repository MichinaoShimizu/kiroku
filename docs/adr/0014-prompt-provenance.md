# ADR 0014: Prompt provenance and automatic events

Status: Retrospective
Date: Unknown
Related: None
Supersedes: None
Superseded by: None

Scope: Phase 1 — local sessions

## Context and documented decision

[#103](https://github.com/MichinaoShimizu/kiroku/pull/103) distinguishes human text, slash commands, shell commands and automatically injected material. Slash/shell commands count as human prompts; system reminders, hooks, notifications and generated summaries are tracked as notes, not human requests. Rephrase detection applies only to ordinary text prompts.

## Rationale and consequences

This classification is part of the measurement contract: automatically inserted context must not inflate human prompt counts. Source-specific parsers should preserve provenance when available and avoid silently treating unknown events as human intent.

## Historical confidence and verification

Inspect `internal/core/kind.go`, source adapters and the prompt timeline and JSON tests. Historical behavior is documented by #103; exact present-day parsing paths still require a full code audit.

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
