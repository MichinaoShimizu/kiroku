# ADR 0013: User-mediated AI report generation

Status: **Retrospective**

Scope: Phase 1 — local sessions.

## Context

This decision area evolved through multiple merged PRs. This record preserves the documented tradeoffs without inventing historical approvals or alternatives.

## Documented decision and evolution

[#22](https://github.com/MichinaoShimizu/kiroku/pull/22) offered copyable AI advice prompts without invoking AI. [#292](https://github.com/MichinaoShimizu/kiroku/pull/292) replaced report drafts derived from unreliable first-prompt titles with prompts for a user-chosen agent, supplying facts and bounded history references. [#298](https://github.com/MichinaoShimizu/kiroku/pull/298) standardizes report structure, numbers and daily coverage while reducing heavy history reads. [#299](https://github.com/MichinaoShimizu/kiroku/pull/299) adds repeatable evaluation for report correctness and clarity. [#304](https://github.com/MichinaoShimizu/kiroku/pull/304) removes the separate “Ask AI for suggestions” panel, while keeping report prompts.

## Rationale and consequences

kiroku produces grounded inputs and a prompt; a user explicitly chooses whether and where to run it. An AI-written report is an interpretation and must not be represented as kiroku-verified facts. Local agent invocation may itself use external services, outside kiroku's data-flow boundary.

## Alternatives and historical confidence

Earlier approaches and reversals are listed only where linked PRs document them. This ADR is descriptive, not a declaration that all future alternatives are rejected.

## Verification

Review `internal/web/js/panels.js`, `.claude/skills/report-eval/SKILL.md`, `SECURITY.md` and prompt-evaluation fixtures. No live agent evaluation was performed in this audit.
