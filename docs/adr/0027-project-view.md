# ADR 0027: Project-centered evidence inventory

- Status: Accepted for prototype
- Date: 2026-10-11

## Context

Calendar answers **when** work was observed; Journey shows **how observed activity is distributed over dates**. Neither gives a compact, project-first inventory of recorded work across agents.

## Decision

Add a third **Project** view sharing the existing Week/Month period selector, search, hidden-project filters, and detail drawer. Group matching sessions by recorded project name; list sessions and recorded Git commits and PR creation events as separate evidence. Display first and last observed session dates and counts without inferring completion, causality, business outcomes, or productivity. Keep all processing local in the existing HTML and introduce no API or data contract.

## Consequences

Users can review what projects they worked on without scanning the time grid. Project names remain history-derived, so renamed or missing project identifiers are not automatically reconciled. PRs appear only when the agent history records them. Counts represent recorded events, not progress. Limit displayed event links to keep the cards manageable; deeper history remains available through other views.

## Alternatives

Expanding Journey with summary cards would conflate project inventory with chronology. Inferring project status or relationships from commit messages would create unsupported claims.
