---
name: forward-adr
description: "Create and maintain prospective Architecture Decision Records for material architectural, product, data, privacy, security, and compatibility choices. Use when a new durable decision is needed or an existing ADR is intentionally superseded or amended. Draft as Proposed; the decision takes effect when its implementation merges, and records human acceptance only when someone actually gives it."
---

# Forward ADR — record deliberate future decisions

Use after ADR Guard identifies a material choice that is new, intentionally changes a prior decision, or requires explicit tradeoff review. This is repository-agnostic; follow local ADR conventions and numbering.

## Modes

- `draft` (default): prepare a Proposed ADR, without accepting it.
- `review`: evaluate a Proposed ADR's evidence, alternatives, consequences, reversibility and validation plan; report gaps without silently rewriting intent.
- `record`: write the Proposed ADR to a branch or PR when authorized; do not merge it, and never record acceptance on your own.
- `activate`: in the PR that implements the decision, set Status `Active` with Basis `merged`. Use Basis `accepted` with a link instead if a person with authority accepted it. Update the index in the same change. The header lands only if the implementation merges.

## Workflow

1. Read repository instructions, ADR index and relevant prior ADRs; use ADR Guard to classify preservation, extension or intentional conflict.
2. Determine whether a durable decision exists. Avoid ADRs for trivial fixes, routine refactors and stylistic changes.
3. State the decision question, context, constraints, stakeholders and evidence. Distinguish facts from assumptions and unresolved questions. Preserve unknown rationale in related retrospective records: a new explicit decision must not retroactively invent a reason for a prior observed change.
4. Describe a specific **proposed** choice and why it is preferred; compare genuine alternatives, including doing nothing when applicable. Do not fabricate deliberation.
5. Analyze consequences: benefits, costs, reversibility, migration, compatibility, security/privacy, operations, observability and failure modes as relevant.
6. Define verification and, if one exists, a review/acceptance owner or process. If nobody can approve, say so (`Decision owner: unknown`); the decision can still take effect on merge. Tests not executed must be marked unverified.
7. Link related ADRs. If replacing an earlier decision, set `Supersedes`; if changing only part, set `Amends`. Preserve the old record's rationale; update the routing index in the same change: stable ID/link, scope, path/component hints, semantic keywords, Proposed status and relationships. In the PR that implements the decision, set Status `Active` with Basis `merged`; an ADR-only PR stays Proposed. Set Basis `accepted` only with a linked, recorded human acceptance; a merge is not one. On supersession, update the index status and reciprocal links together. Do not mark acceptance preemptively.
8. Output a concise review summary of unresolved tradeoffs, risks and, where an approver exists, the approvals still needed. A draft is not in effect, and a merged decision is not an accepted one.

## Template

```markdown
# ADR NNNN: <decision title>

Status: Proposed   # Proposed | Active | Superseded | Rejected | Withdrawn
Basis: —           # for Active: retrospective | merged | accepted
Accepted by: none recorded   # or <name/role> (<link to evidence>)
Date: <YYYY-MM-DD>
Scope: <system or domain>
Related: <ADR links>
Supersedes/Amends: <ADR links, if any>
Decision owner: <role or unknown>

## Context
<problem, constraints, evidence and uncertainty>

## Decision
<proposed choice and rationale>

## Alternatives
<actual alternatives and why not chosen; mark unconsidered options honestly>

## Consequences
<benefits, costs, risks, compatibility, migration and reversibility>

## Verification
<tests, rollout, observability and acceptance criteria>

## Open questions
<what needs resolution before acceptance>
```

## Quality gates

- Do not invent consensus, approvals, rejected alternatives or historical rationale.
- Do not mark Active, Superseded or implemented solely because a draft or PR exists; Active needs the implementation merged.
- Do not set Basis `accepted` or fill `Accepted by` without linked evidence of a person's explicit acceptance. A merge, an auto-merge or an agent's own judgement is not acceptance.
- Do not silently alter old ADRs to fit a new choice.
- Avoid duplicated decisions: prefer referencing a still-current ADR when no actual decision changes.
- Require explicit human review for high-impact security, privacy, data meaning or compatibility choices.

## Relationship to other skills

- **reverse-adr** recovers historical decisions and uncertainty.
- **adr-guard** identifies which prior decisions a change touches.
- **forward-adr** documents the next decision and its deliberate tradeoffs.
- Periodic audits check whether Active decisions still describe the implementation.
