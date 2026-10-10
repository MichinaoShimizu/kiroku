---
name: forward-adr
description: "Create and maintain prospective Architecture Decision Records for material architectural, product, data, privacy, security, and compatibility choices. Use when a new durable decision is needed or an existing ADR is intentionally superseded or amended. Draft as Proposed and require human acceptance."
---

# Forward ADR — record deliberate future decisions

Use after ADR Guard identifies a material choice that is new, intentionally changes a prior decision, or requires explicit tradeoff review. This is repository-agnostic; follow local ADR conventions and numbering.

## Modes

- `draft` (default): prepare a Proposed ADR, without accepting it.
- `review`: evaluate a Proposed ADR's evidence, alternatives, consequences, reversibility and validation plan; report gaps without silently rewriting intent.
- `record`: write the Proposed ADR to a branch or PR when authorized; do not merge or mark Accepted automatically.

## Workflow

1. Read repository instructions, ADR index and relevant prior ADRs; use ADR Guard to classify preservation, extension or intentional conflict.
2. Determine whether a durable decision exists. Avoid ADRs for trivial fixes, routine refactors and stylistic changes.
3. State the decision question, context, constraints, stakeholders and evidence. Distinguish facts from assumptions and unresolved questions.
4. Describe a specific **proposed** choice and why it is preferred; compare genuine alternatives, including doing nothing when applicable. Do not fabricate deliberation.
5. Analyze consequences: benefits, costs, reversibility, migration, compatibility, security/privacy, operations, observability and failure modes as relevant.
6. Define verification and a review/acceptance owner or process. Tests not executed must be marked unverified.
7. Link related ADRs. If replacing an earlier decision, set `Supersedes`; if changing only part, set `Amends`. Preserve the old record's rationale; update the routing index in the same change: stable ID/link, scope, path/component hints, semantic keywords, Proposed status and relationships. After authorized acceptance or supersession, update index status and reciprocal links together; do not preemptively mark acceptance.
8. Output a concise review summary of unresolved tradeoffs, risks and required approvals. A draft is not an accepted decision.

## Template

```markdown
# ADR NNNN: <decision title>

Status: Proposed
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
- Do not mark Accepted, Superseded or implemented solely because a draft or PR exists.
- Do not silently alter old ADRs to fit a new choice.
- Avoid duplicated decisions: prefer referencing a still-current ADR when no actual decision changes.
- Require explicit human review for high-impact security, privacy, data meaning or compatibility choices.

## Relationship to other skills

- **reverse-adr** recovers historical decisions and uncertainty.
- **adr-guard** identifies which prior decisions a change touches.
- **forward-adr** documents the next decision and its deliberate tradeoffs.
- Periodic audits check whether accepted decisions still describe the implementation.
