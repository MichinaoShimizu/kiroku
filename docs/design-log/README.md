# Design Logs

Historical observations, implementation evolution and uncertainties; **not Accepted ADRs**. See [Decision Continuity](../adr/decision-continuity.md) for diagrams and the [ADR index](../adr/README.md) for explicit decisions.

## Historical index

- [0001 — local session foundation](0001-local-session-foundation.md)
- [0002 — measurement semantics](0002-measurement-semantics.md)
- [0003 — local privacy boundary](0003-local-privacy-boundary.md)
- [0004 — session identity and deduplication](0004-session-identity-and-deduplication.md)
- [0005 — nonjudgmental reflection](0005-nonjudgmental-reflection.md)
- [0006 — compatibility contract](0006-compatibility-contract.md)
- [0007 — local history retention](0007-local-history-retention.md)
- [0008 — incremental local processing](0008-incremental-local-processing.md)
- [0009 — git output attribution](0009-git-output-attribution.md)
- [0010 — agent source coverage](0010-agent-source-coverage.md)
- [0011 — release integrity](0011-release-integrity.md)
- [0012 — share export timezone](0012-share-export-timezone.md)
- [0013 — user mediated ai reports](0013-user-mediated-ai-reports.md)
- [0014 — prompt provenance](0014-prompt-provenance.md)
- [0015 — period comparison](0015-period-comparison.md)
- [0016 — single file rendering](0016-single-file-rendering.md)
- [0017 — regression evidence](0017-regression-evidence.md)
- [0018 — language policy](0018-language-policy.md)
- [0019 — time overlap semantics](0019-time-overlap-semantics.md)

Historical claim IDs and original PR citations are preserved. A merged PR does not prove formal approval; unknown rationale remains unknown.

## Record contract

A Design Log has a stable historical ID, scope, source evidence, observations, known uncertainties and verification limits. `Historical decision date: Unknown` is not an ADR approval date. Do not add `Supersedes` or `Superseded by` lifecycle metadata unless linking to a genuine, separately governed ADR; instead use explicit `Related ADR` references. Existing historical headings called `Decision` describe recorded or observed choices, **not** an Accepted architectural rule. A Design Log may be amended with newly verified evidence without rewriting history.

## Evidence and audit trail

The historical [decision ledger](../adr/phase1-decision-ledger.md), [claim evidence matrix](../adr/evidence.yaml), and [audit snapshot](../adr/audit.md) retain their original paths and legacy `ADR-xxxx` claim identifiers for compatibility. These identifiers represent historical claims, not Accepted ADRs. The audit snapshot was compiled before the Design Log migration and does not establish exhaustive review or formal approval.

## Validation scenarios

[Decision Continuity scenario tests](scenario-tests.md) document twelve expected behaviors, static instruction checks and the outstanding live-agent execution gate.
