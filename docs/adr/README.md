# Architecture Decision Records

ADRs document **why** kiroku makes architectural choices, not just how code works. Keep them short, immutable in intent, and linked to the relevant code, issue or PR. An ADR may be retrospective; distinguish observed implementation from a historically recorded decision.

## General practice

[Decision Continuity](decision-continuity.md) describes the repository-agnostic combination of Reverse ADR (recover past decisions), ADR Guard (apply relevant decisions to changes), and forward ADRs (record intentional departures). It includes principles, a lifecycle, adoption steps, evaluation criteria and limitations.

## Reverse ADR skill

The reusable [reverse-adr agent skill](../../.claude/skills/reverse-adr/SKILL.md) defines a repeatable process for discovering decisions, reconstructing rationale, verifying historical/current/test evidence, reconciling reversals, and auditing coverage. Use `audit` for read-only verification or `write` for a branch/PR workflow. This skill can be applied to other repositories; it is not tied to kiroku's specific decisions.

## Continuous design-decision workflow

Reverse ADR is a **bootstrap** step, not the end of ADR adoption. The ADR index is a shared, maintained discovery interface: Reverse ADR constructs it, ADR Guard consults and audits its relevance, and Forward ADR updates it when new decisions are proposed or accepted. Index edits belong in the same change as ADR edits. Status and supersession links must stay current.

The workflow:

1. Run [reverse-adr](../../.claude/skills/reverse-adr/SKILL.md) to recover the evidence-backed decision baseline and explicitly track uncertainties.
2. During design, implementation and review, run [adr-guard](../../.claude/skills/adr-guard/SKILL.md) against the proposed diff to identify preserved, extended and conflicting decisions.
3. If a conflict is intentional and material, use [forward-adr](../../.claude/skills/forward-adr/SKILL.md) to draft a new Proposed ADR with rationale, alternatives, consequences and verification; link it to the previous decision. Do not rewrite historical rationale.
4. Add regression tests for changed invariants, review security/privacy and compatibility, and update the ADR index and supersession chain.
5. Periodically re-run reverse-adr in `audit` mode to detect drift between ADR claims and implementation.

These are **agent workflows, not automatic CI enforcement**. Merely adding a skill does not make every coding agent invoke it. Integrate the workflow into the repository's agent instructions and PR review process; do not block trivial fixes or treat historical ADRs as permanent prohibitions.

## Uncertainty Preservation

A confirmed historical implementation change may have no recorded rationale (for example, a merged PR with an empty description). Retain the observed before/after facts, mark the rationale unknown, and route future related changes to that record for proportionate investigation. An observed change is **not** automatically an accepted architectural constraint. See [Decision Continuity](decision-continuity.md#uncertainty-preservation).

## Quick routing index

Scan this table first; read **only the relevant ADR bodies**. Paths are indicative, not exhaustive: cross-cutting changes (especially privacy, identity, measurement and compatibility) require semantic review even when the changed file is elsewhere. Routine cosmetic edits and localized fixes need no ADR body unless they change a documented invariant.

| Change area / path hints | Keywords and concerns | ADRs |
|---|---|---|
| `internal/source/`, `internal/core/session.go`, `internal/cli/load.go` | source adapters, session identity, deduplication, prompt provenance, missing vs zero, remote origin | [0001](0001-local-session-foundation.md), [0002](0002-measurement-semantics.md), [0004](0004-session-identity-and-deduplication.md), [0010](0010-agent-source-coverage.md), [0014](0014-prompt-provenance.md), [0020 (proposed)](0020-remote-session-import.md) |
| `internal/core/records.go`, `internal/report/`, `internal/web/` | missing vs zero, attribution, overlap, time, metrics, UI interpretation | [0002](0002-measurement-semantics.md), [0005](0005-nonjudgmental-reflection.md), [0009](0009-git-output-attribution.md), [0015](0015-period-comparison.md), [0019](0019-time-overlap-semantics.md) |
| `internal/cli/serve.go`, `internal/cli/load.go`, `internal/archive/`, `internal/gitlog/`, `SECURITY.md` | private histories, local-first, Host/key, file access, exports, network access | [0003](0003-local-privacy-boundary.md), [0007](0007-local-history-retention.md), [0012](0012-share-export-timezone.md) |
| `internal/cli/`, `docs/compatibility.md`, JSON output | public CLI/JSON, schema, flags, archives, stable contracts | [0006](0006-compatibility-contract.md), [0012](0012-share-export-timezone.md) |
| `internal/cli/cache.go`, `internal/source/watch.go` | caching, reload, incremental processing | [0008](0008-incremental-local-processing.md) |
| `internal/web/`, `docs/guide.md` | one-file HTML, rendering, accessibility, nonjudgmental UX | [0016](0016-single-file-rendering.md), [0005](0005-nonjudgmental-reflection.md), [0017](0017-regression-evidence.md) |
| `internal/report/`, report prompts | user-mediated AI reports, bounded input, no automatic AI submission | [0013](0013-user-mediated-ai-reports.md), [0003](0003-local-privacy-boundary.md) |
| `.github/workflows/`, `install.sh`, `internal/cli/update.go`, `.goreleaser.yaml` | supply-chain, checksums, provenance, self-replacement, network access, compatibility | [0011](0011-release-integrity.md), [0006](0006-compatibility-contract.md), [0003](0003-local-privacy-boundary.md) |
| `internal/cli/snapshot_test.go`, `testdata/`, `docs/usability.md` | snapshot fixtures, synthetic history, regression and usability evidence | [0017](0017-regression-evidence.md), [0002](0002-measurement-semantics.md) |
| `README.md`, `README.ja.md`, `internal/web/` | UI language, Japanese documentation | [0018](0018-language-policy.md) |

When adding or changing an ADR, update this table and the ADR list with its scope, path/semantic routing hints, status and supersession relationships. Review stale entries during periodic audits. If multiple rows apply, inspect the union of relevant ADRs. This index is a routing aid, not proof that every architectural decision is documented.

## ADR metadata and index contract

The routing table above is the **shared discovery index**. Its path hints and semantic keywords identify candidate ADRs; they are not exhaustive. The layout list below is the canonical inventory of published ADR IDs and filenames. Both must be updated together whenever an ADR is added, renamed, superseded, or changes scope.

For new and materially revised ADRs, use the following consistent header (keep historical ADR content intact when adding metadata):

```markdown
# ADR NNNN: Decision title

Status: Retrospective | Proposed | Accepted | Rejected | Deprecated | Superseded by NNNN
Date: YYYY-MM-DD | Unknown
Scope: Phase / subsystem
Related: ADR links or None
Supersedes: ADR links or None
Superseded by: ADR links or None
Decision owner: role or Unknown (required for forward ADRs)
```

Retrospective records should use sections Context, Decision, Consequences, Alternatives, Historical confidence and open questions, Verification. Forward proposals use Context, Decision, Alternatives, Consequences, Verification, Open questions. Do not manufacture a decision date or owner. Evidence labels apply to individual claims, not to the Status field; test execution is recorded separately in `evidence.yaml`.

**Index update checklist:** (1) keep the stable ID and filename in Layout; (2) add or update routing rows for path *and* semantic relevance; (3) state lifecycle status and supersession links in the ADR header; (4) keep reciprocal supersession references and links consistent; (5) check that every referenced ADR exists. A routing row can reference multiple ADRs; ADR Guard reads their union and verifies their statuses in the linked records. Do not treat the index as an authoritative claim that an ADR is Accepted.

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
- `decision-continuity.md` — reusable practice combining retrospective recovery with ongoing change governance.

## Lifecycle and record format

**Status is the lifecycle of the decision; evidence labels describe how well each claim is supported.** Keep them on separate axes. Never write `Status: Documented` or `Status: Unknown`.

| Status | Meaning |
|---|---|
| **Retrospective** | Recovered from history. Records what happened and any documented rationale; not an accepted constraint until owners review it. Use it when no original decision record exists, and keep it when the rationale is unknown. Formerly named `Observed (retrospective)`; renamed so the status is not confused with the Observed evidence label. |
| **Proposed** | A new decision awaiting human review. |
| **Accepted** / **Rejected** | Set only by the decision owner. |
| **Superseded by NNNN** / **Deprecated** | Replaced by a later accepted ADR, or no longer applies. |

Evidence labels (**Documented / Observed / Inferred / Unknown**) apply per claim, in the ADR body and in [evidence.yaml](evidence.yaml), together with the separate historical/current/tested (H/C/T) checks. A retrospective ADR may contain documented claims and claims with unknown rationale at the same time.

Every ADR contains the common ADR elements: title, status, date, context, decision, consequences (costs as well as benefits), alternatives (or "not documented"), and verification. Retrospective ADRs date the source change (PR merge date) or say the date is unknown; they also state historical confidence and open questions. Forward ADRs add a decision owner and `Supersedes`/`Amends` links. The Phase 1 retrospective ADRs predate the explicit `Date` field and are dated through their linked PRs; add the field when an ADR is next updated. Templates: [reverse-adr](../../.claude/skills/reverse-adr/SKILL.md#retrospective-adr-template) and [forward-adr](../../.claude/skills/forward-adr/SKILL.md#template).

Changes in direction create a new ADR rather than rewriting an old one. When the new ADR is accepted, the only edit to the old ADR is its status line (`Superseded by NNNN`, with a link); its body and rationale stay as recorded.

## Roadmap terminology

- **Phase 1 — Local sessions (existing baseline):** native local history discovery, parsing, deduplication, local-only reporting and archival. This phase describes existing functionality; it is not a claim that every edge case has been validated.
- **Phase 2 — Remote sessions (planned):** manually imported native remote histories first; SSH synchronization next; hosted-service connectors only where export/API access exists. Each agent's coverage must be tracked explicitly.

Historical issue labels such as “Phase 0” (research) and “Phase 1” (offline import) in [#323](https://github.com/MichinaoShimizu/kiroku/issues/323)–[#328](https://github.com/MichinaoShimizu/kiroku/issues/328) refer to the **old remote-import subplan**, not this top-level roadmap. They should be relabeled separately rather than silently reinterpreted.

## Audit scope and remaining verification

The 19 retrospective Phase 1 ADRs cover the major themes recovered from merged PR descriptions. [The evidence ledger](phase1-decision-ledger.md) contains finer-grained choices and reversals. This is **not** a claim that all 318 merged PRs, all review comments, and all historical diffs were independently inspected.

Evidence verified in this review includes the linked PR descriptions and current `internal/core/agents.go`, `internal/core/records.go`, `internal/gitlog/gitlog.go`, `internal/cli/cache.go`, `internal/report/cache.go`, `docs/compatibility.md`, `docs/sources.md`, and `SECURITY.md`. Targeted tests for a small claim sample passed at `ad670cb` (see the [audit follow-up](audit.md#follow-up--2026-10-10-pinned-to-ad670cb)); **a full test-to-claim mapping, full code-path audit, historical review-thread audit and published release verification remain unperformed.** Historical rationale is distinguished from current implementation. A future change in a documented policy requires a new ADR, not a silent rewrite.
