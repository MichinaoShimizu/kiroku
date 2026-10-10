# ADR 0006: Reflection without grading

Status: **Observed (retrospective)**

Scope: Phase 1 — product philosophy and UX.

## Context

A history viewer can help users understand their own AI-assisted work, but advice, rankings and invented productivity judgments can be mistaken for objective measurement.

## Decision history and reversals

1. Initial weekly reflection and productivity diagnostics were added ([#2](https://github.com/MichinaoShimizu/kiroku/pull/2)).
2. Reflection was reshaped into a KPI-playbook-inspired decision flow ([#5](https://github.com/MichinaoShimizu/kiroku/pull/5)).
3. The prescribed flow was removed in favor of descriptive weekly/monthly summaries because it imposed too strong a viewpoint ([#14](https://github.com/MichinaoShimizu/kiroku/pull/14)).
4. Intervention buttons and trial/comparison prompts were reduced or removed ([#41](https://github.com/MichinaoShimizu/kiroku/pull/41), [#53](https://github.com/MichinaoShimizu/kiroku/pull/53), [#81](https://github.com/MichinaoShimizu/kiroku/pull/81)).
5. Year in Review was introduced with a shareable image, then skill grades were added, the view was hidden, and it returned without ranking users ([#76](https://github.com/MichinaoShimizu/kiroku/pull/76), [#78](https://github.com/MichinaoShimizu/kiroku/pull/78), [#88](https://github.com/MichinaoShimizu/kiroku/pull/88), [#320](https://github.com/MichinaoShimizu/kiroku/pull/320)).
6. The current product positioning emphasizes Remember → Understand → Improve ([#322](https://github.com/MichinaoShimizu/kiroku/pull/322)).

## Resulting principle

Offer evidence, context, trends and optional prompts; do not label a person as skilled/unskilled, good/bad, or prescribe a single correct way to work. Improvement belongs to the user.

## Consequences

Avoid reintroducing grading through colors, labels, badges, “healthy” thresholds or gamification without a new explicit decision. Any insight should expose its basis and distinguish observations from inferences.

## Historical confidence

The sequence above is supported by linked PR descriptions. It does not claim that each change was adopted as a formal ADR at the time, nor that every UX choice is permanently fixed.

## Verification

Review the PR sequence, current view and reports. No fresh visual regression testing is claimed.
