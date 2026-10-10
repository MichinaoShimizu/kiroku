# ADR 0017: Synthetic fixtures and user-oriented verification

Status: Retrospective
Date: Unknown
Related: None
Supersedes: None
Superseded by: None

Scope: Phase 1 — local sessions

## Documented decision and evolution

[#68](https://github.com/MichinaoShimizu/kiroku/pull/68) added a whole-dataset snapshot covering seven source formats and Go-only metrics because Python-parity golden tests missed Codex and later adapters. The PR explicitly tested timezone stability and deliberately mutated Codex token calculations to demonstrate detection. [#42](https://github.com/MichinaoShimizu/kiroku/pull/42) introduced goal-based usability scenarios and a user-tester agent, explicitly noting that AI-simulated user tests do not replace real people.

## Rationale and consequences

Use both numerical snapshots and task-oriented usability tests. Snapshot stability does not establish semantic correctness, and simulated users are not a substitute for actual users.

## Verification and confidence

Review `internal/cli/snapshot_test.go`, `testdata/snapshot.json`, `docs/usability.md` and current CI. Historical PR checks are not a fresh execution of these tests.

This is an evidence-linked retrospective, not an assertion of historical ADR approval.

## Decision

The recovered decisions remain in the original evidence and decision-history sections above; no new interpretation is introduced.

## Consequences

See the existing rationale and consequences discussion above. No additional outcome is asserted.

## Alternatives

Not documented as a separate set of evaluated options in this record.

## Historical confidence and open questions

Rationale is limited to the linked historical evidence. Unrecorded alternatives and approval history remain unknown.
