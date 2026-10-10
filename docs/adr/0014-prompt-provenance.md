# ADR 0014: Prompt provenance and automatic events

Status: **Observed (retrospective)**

Scope: Phase 1 — local sessions.

## Context and documented decision

[#103](https://github.com/MichinaoShimizu/kiroku/pull/103) distinguishes human text, slash commands, shell commands and automatically injected material. Slash/shell commands count as human prompts; system reminders, hooks, notifications and generated summaries are tracked as notes, not human requests. Rephrase detection applies only to ordinary text prompts.

## Rationale and consequences

This classification is part of the measurement contract: automatically inserted context must not inflate human prompt counts. Source-specific parsers should preserve provenance when available and avoid silently treating unknown events as human intent.

## Historical confidence and verification

Inspect `internal/core/kind.go`, source adapters and the prompt timeline and JSON tests. Historical behavior is documented by #103; exact present-day parsing paths still require a full code audit.

This ADR records documented changes, not an approval meeting or an unverified current behavior.
