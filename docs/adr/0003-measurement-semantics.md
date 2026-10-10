# ADR 0003: Measurement semantics and evidence fidelity

Status: **Observed (retrospective)**

Scope: Phase 1 — local sessions.

## Context

AI-agent histories are heterogeneous: fields, units and events vary by agent and version. A uniform dashboard must not imply that missing data means zero, that estimated prices are bills, or that two duplicated transcript rows represent two actions.

## Documented decisions

- **Keep measurements descriptive rather than normative.** Native agent-specific metrics were introduced as reference values with sample counts, not performance grades ([#10](https://github.com/MichinaoShimizu/kiroku/pull/10)). Earlier prescriptive reflection was removed in favor of weekly/monthly descriptive summaries ([#14](https://github.com/MichinaoShimizu/kiroku/pull/14)).
- **Preserve missingness.** Show “Not recorded” rather than 0 when the source does not record a metric ([#214](https://github.com/MichinaoShimizu/kiroku/pull/214)); document per-agent recording coverage ([#215](https://github.com/MichinaoShimizu/kiroku/pull/215)).
- **Deduplicate source events before counting.** Aggregate split Claude usage by message/request ID ([#4](https://github.com/MichinaoShimizu/kiroku/pull/4)); correct Codex review mirroring and Claude branch copies ([#310](https://github.com/MichinaoShimizu/kiroku/pull/310), [#311](https://github.com/MichinaoShimizu/kiroku/pull/311)).
- **Make the basis inspectable.** Provide breakdowns from summary figures ([#253](https://github.com/MichinaoShimizu/kiroku/pull/253), [#259](https://github.com/MichinaoShimizu/kiroku/pull/259)).

## Rationale

Different source formats cannot support identical metrics; making unknown values look precise misleads users. Correctness and traceability take precedence over filling every cell.

## Consequences

Source adapters must identify supported and unsupported fields, and regressions require representative fixtures. Estimates and derived metrics must be labeled as such. Cross-agent comparisons must respect differing recording coverage.

## Alternatives and historical confidence

Earlier coaching-oriented and grading-oriented UI was tried and subsequently reduced or removed; see [ADR 0006](0006-nonjudgmental-reflection.md). No evidence is asserted for alternatives not documented in linked PRs.

## Verification

Review linked PRs, `internal/core`, `internal/source`, `internal/report`, `docs/sources.md`, and regression fixtures. This retrospective document does not certify fresh test execution.
