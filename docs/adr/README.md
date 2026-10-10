# Architecture Decision Records

ADRs document **why** kiroku makes architectural choices, not just how code works. Keep them short, immutable in intent, and linked to the relevant code, issue or PR. An ADR may be retrospective; distinguish observed implementation from a historically recorded decision.

## Reverse ADR skill

The reusable [reverse-adr agent skill](../../.claude/skills/reverse-adr/SKILL.md) defines a repeatable process for discovering decisions, reconstructing rationale, verifying historical/current/test evidence, reconciling reversals, and auditing coverage. Use `audit` for read-only verification or `write` for a branch/PR workflow. This skill can be applied to other repositories; it is not tied to kiroku's specific decisions.

## Layout

- `0001-local-session-foundation.md` — retrospective baseline for local-only session processing (Phase 1).
- `0002-measurement-semantics.md` — data fidelity, missingness, traceability (Phase 1, retrospective).
- `0003-local-privacy-boundary.md` — local-first security and privacy boundaries (Phase 1, retrospective).
- `0004-session-identity-and-deduplication.md` — source-specific session identity and duplicate handling (Phase 1, retrospective).
- `0005-nonjudgmental-reflection.md` — product philosophy and documented reversals (Phase 1, retrospective).
- `0006-compatibility-contract.md` — public JSON/CLI and 1.0 compatibility policy (Phase 1, retrospective).
- `0007-local-history-retention.md` — preserving histories agents may prune (Phase 1, retrospective).
- `0008-incremental-local-processing.md` — changed-file reloads and affected-period recalculation (Phase 1, retrospective).
- `0009-git-output-attribution.md` — local Git evidence and attribution limits (Phase 1, retrospective).
- `0010-agent-source-coverage.md` — native adapters and source recording capabilities (Phase 1, retrospective).
- `0011-release-integrity.md` — supply-chain checks and release provenance (Phase 1, retrospective).
- `0012-share-export-timezone.md` — shareable exports and writer-local time (Phase 1, retrospective).
- `0013-user-mediated-ai-reports.md` — grounded prompts for user-chosen AI agents (Phase 1, retrospective).
- `0014-prompt-provenance.md` — human prompts versus agent-injected notes (Phase 1, retrospective).
- `0015-period-comparison.md` — partial-period comparisons and projections (Phase 1, retrospective).
- `0016-single-file-rendering.md` — modular web sources with one-file export (Phase 1, retrospective).
- `0017-regression-evidence.md` — synthetic snapshots and usability tests (Phase 1, retrospective).
- `0018-language-policy.md` — English UI and Japanese README (Phase 1, retrospective).
- `0019-time-overlap-semantics.md` — overlapping active time and category shares (Phase 1, retrospective).
- `0020-remote-session-import.md` — proposed extension for remote/cloud histories (Phase 2).
- `phase1-decision-ledger.md` — evolving, evidence-linked historical decision inventory.

## Lifecycle

Use **Observed (retrospective)** for decisions inferred from existing code when no original decision record is available; do not fabricate a past discussion. For new decisions use **Proposed → Accepted → Superseded** (or **Rejected**). Every ADR should contain context, decision, rationale, consequences, alternatives, and verification references. Changes in direction should create a new ADR with a supersedes link rather than silently rewriting accepted decisions.

## Roadmap terminology

- **Phase 1 — Local sessions (existing baseline):** native local history discovery, parsing, deduplication, local-only reporting and archival. This phase describes existing functionality; it is not a claim that every edge case has been validated.
- **Phase 2 — Remote sessions (planned):** manually imported native remote histories first; SSH synchronization next; hosted-service connectors only where export/API access exists. Each agent's coverage must be tracked explicitly.

Historical issue labels such as “Phase 0” (research) and “Phase 1” (offline import) in [#323](https://github.com/MichinaoShimizu/kiroku/issues/323)–[#328](https://github.com/MichinaoShimizu/kiroku/issues/328) refer to the **old remote-import subplan**, not this top-level roadmap. They should be relabeled separately rather than silently reinterpreted.

## Audit scope and remaining verification

The 19 retrospective Phase 1 ADRs cover the major themes recovered from merged PR descriptions. [The evidence ledger](phase1-decision-ledger.md) contains finer-grained choices and reversals. This is **not** a claim that all 318 merged PRs, all review comments, and all historical diffs were independently inspected.

Evidence verified in this review includes the linked PR descriptions and current `internal/core/agents.go`, `internal/core/records.go`, `internal/gitlog/gitlog.go`, `internal/cli/cache.go`, `internal/report/cache.go`, `docs/compatibility.md`, `docs/sources.md`, and `SECURITY.md`. **Fresh tests, full code-path audit, historical review-thread audit and published release verification remain unperformed.** Historical rationale is distinguished from current implementation. A future change in a documented policy requires a new ADR, not a silent rewrite.
