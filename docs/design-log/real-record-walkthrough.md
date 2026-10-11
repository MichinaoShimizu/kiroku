# Decision Continuity — first real-record walkthrough (2026-10-11)

This is a **manual, document-based review of actual repository records**, not a live execution of the three agent skills. It evaluates the lifecycle and routing rules against [ADR 0021](../adr/0021-kiro-cli-v3-separate-source.md) and [ADR 0020](../adr/0020-remote-session-import.md). It does not assert that current implementation, PR history, or tests have been independently verified.

## Reverse ADR — provenance and uncertainty

- [Design Log 0010](0010-agent-source-coverage.md), [0004](0004-session-identity-and-deduplication.md), and [0006](0006-compatibility-contract.md) are historical context linked by ADR 0021. They are not independently binding decisions.
- ADR 0021 documents a distinct, explicitly approved choice: its status says **Accepted (2026-10-11, approved by the maintainer in review of #359)**. This walkthrough accepts that as the record's claim; it has **not** independently inspected the review approval.
- ADR 0021 explicitly marks V2-to-V3 conversion identity as inferred rather than verified on a real converted session. Do not turn that inference into a test result.

## ADR Guard — two concrete hypothetical changes

| Proposed change (hypothetical, not an actual PR) | Applicable record | Expected classification | Action |
|---|---|---|---|
| Relabel shared-store Kiro V3 sessions back to **Kiro IDE** | [ADR 0021](../adr/0021-kiro-cli-v3-separate-source.md), Accepted | **Conflict** with the approved source-label decision | Explain impact on reporting/JSON consumers; retain approved label or draft a new Proposed ADR to intentionally change it |
| Add an opt-in remote-session import | [ADR 0020](../adr/0020-remote-session-import.md), Proposed | **Review needed**, not a binding veto from ADR 0020 | Review privacy/security and applicable Accepted ADRs independently; don't treat ADR 0020 as approved implementation policy |

The second case does **not** establish that remote import is safe: missing or Proposed ADR coverage is not a safety guarantee. The first case does **not** establish that the code currently conforms to ADR 0021.

## Forward ADR — conditional next decision

If the product intentionally changes the Accepted Kiro V3 labeling policy, Forward ADR should **draft a new Proposed ADR** describing alternatives, migration, compatibility and user-visible consequences, and link it to ADR 0021. Do not silently change ADR 0021 or mark the new ADR Accepted. If the change instead preserves the approved policy, no new ADR is required.

## Result and limits

| Check | Result |
|---|---|
| Historical Design Logs distinguished from formal ADRs | PASS — document review |
| Accepted 0021 vs Proposed 0020 authority distinguished | PASS — document review |
| Intentional change routed to Proposed + human approval | PASS — document review |
| Actual agent invocation with skills loaded | NOT RUN |
| Source implementation and tests verified at a pinned commit | NOT RUN |
| Approval evidence in PR #359 independently reviewed | NOT RUN |

**Next gate:** run an isolated agent session with the actual three skills and this case, save transcript and model version, and compare to these expectations. Keep the [12-scenario matrix](scenario-tests.md) and [runner instructions](agent-runner.md) separate from the normal ADR workflow.
