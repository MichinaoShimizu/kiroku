# Architecture Decision Records

ADRs document **why** kiroku makes architectural choices, not just how code works. Keep them short, immutable in intent, and linked to the relevant code, issue or PR. An ADR may be retrospective; distinguish observed implementation from a historically recorded decision.

## Reverse ADR skill

The reusable [reverse-adr agent skill](../../.claude/skills/reverse-adr/SKILL.md) defines a repeatable process for discovering decisions, reconstructing rationale, verifying historical/current/test evidence, reconciling reversals, and auditing coverage. Use `audit` for read-only verification or `write` for a branch/PR workflow. This skill can be applied to other repositories; it is not tied to kiroku's specific decisions.

## Continuous design-decision workflow

Reverse ADR is a **bootstrap** step, not the end of ADR adoption:

1. Run [reverse-adr](../../.claude/skills/reverse-adr/SKILL.md) to recover the evidence-backed decision baseline and explicitly track uncertainties.
2. During design, implementation and review, run [adr-guard](../../.claude/skills/adr-guard/SKILL.md) against the proposed diff to identify preserved, extended and conflicting decisions.
3. If a conflict is intentional and material, propose a new forward ADR with rationale, alternatives, consequences and verification; link it to the previous decision. Do not rewrite historical rationale.
4. Add regression tests for changed invariants, review security/privacy and compatibility, and update the ADR index and supersession chain.
5. Periodically re-run reverse-adr in `audit` mode to detect drift between ADR claims and implementation.

These are **agent workflows, not automatic CI enforcement**. Merely adding a skill does not make every coding agent invoke it. Integrate the workflow into the repository's agent instructions and PR review process; do not block trivial fixes or treat historical ADRs as permanent prohibitions.

## Quick routing index

Scan this table first; read **only the relevant ADR bodies**. Paths are indicative, not exhaustive: cross-cutting changes (especially privacy, identity, measurement and compatibility) require semantic review even when the changed file is elsewhere. Routine cosmetic edits and localized fixes need no ADR body unless they change a documented invariant.

| Change area / path hints | Keywords and concerns | ADRs |
|---|---|---|
| `internal/source/`, `internal/core/session.go`, `internal/cli/load.go` | source adapters, session identity, deduplication, prompt provenance, remote origin | [0001](0001-local-session-foundation.md), [0004](0004-session-identity-and-deduplication.md), [0010](0010-agent-source-coverage.md), [0014](0014-prompt-provenance.md), [0020 (proposed)](0020-remote-session-import.md) |
| `internal/core/records.go`, `internal/report/`, `internal/web/` | missing vs zero, attribution, overlap, time, metrics, UI interpretation | [0002](0002-measurement-semantics.md), [0005](0005-nonjudgmental-reflection.md), [0009](0009-git-output-attribution.md), [0015](0015-period-comparison.md), [0019](0019-time-overlap-semantics.md) |
| `internal/cli/serve.go`, `internal/cli/load.go`, `internal/archive/`, `internal/gitlog/`, `SECURITY.md` | private histories, local-first, Host/key, file access, exports, network access | [0003](0003-local-privacy-boundary.md), [0007](0007-local-history-retention.md), [0012](0012-share-export-timezone.md) |
| `internal/cli/`, `docs/compatibility.md`, JSON output | public CLI/JSON, schema, flags, archives, stable contracts | [0006](0006-compatibility-contract.md), [0012](0012-share-export-timezone.md) |
| `internal/cli/cache.go`, `internal/source/watch.go` | caching, reload, incremental processing | [0008](0008-incremental-local-processing.md) |
| `internal/web/`, `docs/guide.md` | one-file HTML, rendering, accessibility, nonjudgmental UX | [0016](0016-single-file-rendering.md), [0005](0005-nonjudgmental-reflection.md), [0017](0017-regression-evidence.md) |
| `internal/report/`, report prompts | user-mediated AI reports, bounded input, no automatic AI submission | [0013](0013-user-mediated-ai-reports.md), [0003](0003-local-privacy-boundary.md) |
| `.github/workflows/`, `install.sh`, release/update code | supply-chain, signatures, provenance, compatibility | [0011](0011-release-integrity.md), [0006](0006-compatibility-contract.md) |
| `README.md`, `README.ja.md`, `internal/web/` | UI language, Japanese documentation | [0018](0018-language-policy.md) |

If multiple rows apply, inspect the union of relevant ADRs. This index is a routing aid, not proof that every architectural decision is documented.

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
- `evidence.yaml` — initial claim-level historical/current/test references and conservative statuses.
- `audit.md` — dated Reverse ADR inspection, findings, limitations and next verification gates.

## Lifecycle

Use **Observed (retrospective)** for decisions inferred from existing code when no original decision record is available; do not fabricate a past discussion. For new decisions use **Proposed → Accepted → Superseded** (or **Rejected**). Every ADR should contain context, decision, rationale, consequences, alternatives, and verification references. Changes in direction should create a new ADR with a supersedes link rather than silently rewriting accepted decisions.

## Roadmap terminology

- **Phase 1 — Local sessions (existing baseline):** native local history discovery, parsing, deduplication, local-only reporting and archival. This phase describes existing functionality; it is not a claim that every edge case has been validated.
- **Phase 2 — Remote sessions (planned):** manually imported native remote histories first; SSH synchronization next; hosted-service connectors only where export/API access exists. Each agent's coverage must be tracked explicitly.

Historical issue labels such as “Phase 0” (research) and “Phase 1” (offline import) in [#323](https://github.com/MichinaoShimizu/kiroku/issues/323)–[#328](https://github.com/MichinaoShimizu/kiroku/issues/328) refer to the **old remote-import subplan**, not this top-level roadmap. They should be relabeled separately rather than silently reinterpreted.

## Audit scope and remaining verification

The 19 retrospective Phase 1 ADRs cover the major themes recovered from merged PR descriptions. [The evidence ledger](phase1-decision-ledger.md) contains finer-grained choices and reversals. This is **not** a claim that all 318 merged PRs, all review comments, and all historical diffs were independently inspected.

Evidence verified in this review includes the linked PR descriptions and current `internal/core/agents.go`, `internal/core/records.go`, `internal/gitlog/gitlog.go`, `internal/cli/cache.go`, `internal/report/cache.go`, `docs/compatibility.md`, `docs/sources.md`, and `SECURITY.md`. **Fresh tests, full code-path audit, historical review-thread audit and published release verification remain unperformed.** Historical rationale is distinguished from current implementation. A future change in a documented policy requires a new ADR, not a silent rewrite.
