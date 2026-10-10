# Architecture Decision Records

ADRs document **why** kiroku makes architectural choices, not just how code works. Keep them short, immutable in intent, and linked to the relevant code, issue or PR. An ADR may be retrospective; distinguish observed implementation from a historically recorded decision.

## Layout

- `0001-local-session-foundation.md` — retrospective baseline for local-only session processing (Phase 1).
- `0002-measurement-semantics.md` — data fidelity, missingness, traceability (Phase 1, retrospective).
- `0003-local-privacy-boundary.md` — local-first security and privacy boundaries (Phase 1, retrospective).
- `0004-session-identity-and-deduplication.md` — source-specific session identity and duplicate handling (Phase 1, retrospective).
- `0005-nonjudgmental-reflection.md` — product philosophy and documented reversals (Phase 1, retrospective).
- `0006-compatibility-contract.md` — public JSON/CLI and 1.0 compatibility policy (Phase 1, retrospective).
- `0007-local-history-retention.md` — preserving histories agents may prune (Phase 1, retrospective).
- `0008-incremental-local-processing.md` — changed-file reloads and affected-period recalculation (Phase 1, retrospective).
- `0009-remote-session-import.md` — proposed extension for remote/cloud histories (Phase 2).
- `phase1-decision-ledger.md` — evolving, evidence-linked historical decision inventory.

## Lifecycle

Use **Observed (retrospective)** for decisions inferred from existing code when no original decision record is available; do not fabricate a past discussion. For new decisions use **Proposed → Accepted → Superseded** (or **Rejected**). Every ADR should contain context, decision, rationale, consequences, alternatives, and verification references. Changes in direction should create a new ADR with a supersedes link rather than silently rewriting accepted decisions.

## Roadmap terminology

- **Phase 1 — Local sessions (existing baseline):** native local history discovery, parsing, deduplication, local-only reporting and archival. This phase describes existing functionality; it is not a claim that every edge case has been validated.
- **Phase 2 — Remote sessions (planned):** manually imported native remote histories first; SSH synchronization next; hosted-service connectors only where export/API access exists. Each agent's coverage must be tracked explicitly.

Historical issue labels such as “Phase 0” (research) and “Phase 1” (offline import) in [#323](https://github.com/MichinaoShimizu/kiroku/issues/323)–[#328](https://github.com/MichinaoShimizu/kiroku/issues/328) refer to the **old remote-import subplan**, not this top-level roadmap. They should be relabeled separately rather than silently reinterpreted.
