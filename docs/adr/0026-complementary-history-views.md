# ADR 0026: Support complementary views of local AI activity

Status: Superseded by [ADR 0033](0033-calendar-only-view.md) (2026-10-14). Previously Accepted (2026-10-14, approved by the maintainer during the quality audit; Calendar and Journey shipped in #372–#376, extended by the Project view in [ADR 0027](0027-project-view.md))
Date: 2026-10-11
Scope: Local HTML history exploration and view navigation (`internal/web`)
Related ADRs: [ADR 0022](0022-history-stays-local.md), [ADR 0023](0023-compatibility-contract.md)
Related Decision Logs: DL 0016
Supersedes/Amends: None
Decision owner: Repository maintainer

## Context

The current Week and Month calendar emphasizes *when* an AI session happened. A project-oriented Journey view can answer *what project activity continued over time*. More views may follow. The existing single-file, offline-capable HTML viewer and shared search, time selection, and session detail are constraints. The proposed view must not imply progress, causality, or outcomes not present in source records.

This is a prospective product/design choice, not a reconstruction of past approval. The Journey prototype is under review in PR #372; its existence does not constitute acceptance.

## Decision

**Proposed:** Keep View (Calendar/Journey) independent of Period (Week/Month), with Year in Review as a separate report. Retain Calendar as the default and allow complementary, explicitly labeled views over the same local session records. Start with a Journey view grouping observed sessions by project and date. Reuse the existing search/filter and session-detail semantics rather than maintaining separate data interpretations. Display counts as activity, not as productivity or business outcomes. Do not infer relationships, progress, or intent from mere temporal proximity.

Keep all views inside the existing assembled single-file HTML with no external AI, telemetry, or history upload. Future views should justify their distinct question before being added; do not introduce a generic view framework until repeated implementations demonstrate a need.

## Alternatives

1. **Calendar only:** simpler navigation and maintenance, but weak project-level longitudinal exploration.
2. **Replace calendar with Journey:** prioritizes project continuity but loses familiar daily/weekly time inspection.
3. **Multiple independent view-specific datasets:** potentially flexible, but risks inconsistent filters, identities, and meanings.

These are design alternatives for review, not claims about historical deliberation.

## Consequences

- Benefit: users can explore the same observed history by time or project without duplicating source ingestion.
- Cost: additional UI states, keyboard/accessibility, responsive layout, rendering performance, documentation, and regression tests.
- Risk: project names may be missing or change; session timestamps and overlap require consistent semantics; high-volume periods can overwhelm a lane.
- Privacy: all labels and titles are untrusted and must be escaped; ADR 0022 remains binding.
- Compatibility: additive UI only; no CLI, JSON schema, or exported-data contract changes.
- Reversible: Journey can be disabled or removed without migrating persisted history.

## Verification

Before merge: run `sh tools/check.sh` and `sh tools/check.sh --e2e`; test week/month, project grouping, overlapping sessions, search/filter, selection, empty states, keyboard navigation, hostile titles, mobile overflow, and single-file rendering/CSP. Update the guide, changelog, and screenshots as appropriate. Record exact passing revisions; no tests are claimed to have passed by this ADR.

## Open questions

- View and Period are separate controls; switching one preserves the other.
- What is the expected behavior for long months and hundreds of sessions per project?
- How should missing or renamed project identities be presented?
- Which user scenario demonstrates a benefit over existing project summaries?

Acceptance requires explicit maintainer review. A merged PR alone does not accept this ADR.
