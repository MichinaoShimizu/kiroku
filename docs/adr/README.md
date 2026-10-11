# Architecture Decision Records

ADRs record explicit design choices; [Design Logs](../design-log/README.md) preserve historical observations. See [Decision Continuity](decision-continuity.md) for illustrated workflows.

## Active ADR index

| Topic | Status | ADR |
|---|---|---|
| Remote session import (Phase 2) | Proposed (not accepted) | [0020](0020-remote-session-import.md) |
| Kiro CLI V3 apart from V2; V3 shares the Kiro IDE store (`internal/source/kiro.go`, `core.Records`; keywords: Kiro CLI, V3, KAS, sess_, Kiro IDE label, V2 dedup) | Proposed (not accepted) | [0021](0021-kiro-cli-v3-separate-source.md) |

There are no currently Accepted ADRs in this index. Only a human decision owner may accept an ADR. Proposed records are not binding constraints. An empty index does not prove a change safe.

## Historical compatibility

Former retrospective ADRs 0001–0019 are now Design Logs; old ADR file paths remain as redirects. They are **not** accepted constraints. Their original evidence remains in [evidence.yaml](evidence.yaml), [audit.md](audit.md) and [phase1-decision-ledger.md](phase1-decision-ledger.md). Legacy evidence IDs are preserved for traceability.

ADR Guard checks Accepted decisions, then consults Design Logs for relevant historical context. New durable choices are drafted as Proposed ADRs and require explicit human review. Update routing and supersession links alongside decisions.
