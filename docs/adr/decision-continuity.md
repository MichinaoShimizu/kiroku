# Decision Continuity: Reverse ADR + ADR Guard

## Concept

**Decision Continuity** is a repository-agnostic practice for recovering the reasoning behind an existing system and keeping that reasoning available as the system evolves.

It connects two complementary activities:

- **Reverse ADR (recover):** reconstruct the historical decision trail from PRs, issues, code, tests and reviews. Separate documented intent from observed implementation and unknown rationale.
- **ADR Guard (apply):** consult relevant decisions when proposing changes and identify preserved constraints and potential conflicts.
- **Forward ADR (decide):** formulate and document a prospective choice, its alternatives and consequences, including deliberate supersession of an earlier decision.

Neither activity is sufficient alone. Reverse ADR without ongoing use becomes a static archive; ADR Guard without a reliable historical baseline may enforce accidental or imagined constraints. Together they turn decision history into a **maintained input to future decisions**.

## Lifecycle

```text
Existing system and history
          |
          v
Reverse ADR: discover -> reconstruct -> verify -> reconcile
          |
          v
Evidence-linked decision baseline + uncertainty ledger
          |
          v
New work -> lightweight ADR relevance routing
          |
          +-- No relevant decision -> normal implementation
          |
          +-- Relevant decision -> ADR Guard: preserve / extend / conflict
                                           |
                                           +-- Preserve or extend -> implement + test
                                           |
                                           +-- Intentional material change
                                                -> new Proposed ADR
                                                -> review and acceptance
                                                -> implement + test
                                                -> link supersession
          |
          v
Periodic drift audit -> refresh evidence and identify gaps
          |
          +-------------------------------------> next change
```

## Principles

1. **Evidence before authority.** A retrospective ADR is a claim about history, not a retroactive approval. Label documented, observed, inferred and unknown statements. Verify current code and tests separately.
2. **Selective retrieval.** Every task can cheaply check an ADR index; only relevant tasks need full ADR text. Match both changed paths and semantic impact. Critical cross-cutting constraints cannot rely on path matching alone.
3. **Decisions can change.** An ADR is not a permanent prohibition. Preserve the original record and explain intentional changes through a new ADR, with alternatives, consequences, migration and verification.
4. **Proportional process.** Do not require an ADR for every bug fix or cosmetic change. Prioritize durable architectural, privacy, security, data, compatibility and product-policy decisions.
5. **Traceability, not ceremonial compliance.** Connect claims to historical evidence, current behavior and actual test results. A test file is not a passing test.
6. **Explicit uncertainty.** Missing evidence is a gap to investigate, not permission to invent rationale or silently declare a decision accepted.
7. **Human governance.** Agents can surface conflicts and draft ADRs; owners decide whether to accept tradeoffs. Automated checks should not convert uncertain historical interpretations into unquestionable rules.

## Minimal artifacts

| Artifact | Purpose |
|---|---|
| ADR index / routing map | Quickly identify relevant decisions from paths, topics and interfaces |
| Reverse ADR skill or procedure | Recover decisions and mark evidence confidence |
| Decision ledger + evidence matrix | Track smaller decisions, reversals, unresolved claims and H/C/T evidence |
| ADR Guard skill or review checklist | Check change impact against relevant decisions |
| Forward ADRs and supersession links | Explain why a deliberate policy or design change was made |
| Periodic drift report | Detect discrepancies between documented decisions and current implementation |

## Adoption in an existing repository

1. Inventory important boundaries and decisions; recover a small, high-risk baseline first.
2. Publish an ADR index with concise path/topic hints and explicit evidence limitations.
3. Add a lightweight pre-implementation relevance check to agent instructions and PR reviews.
4. Require review of applicable ADRs for material changes; document intentional supersession.
5. Periodically compare the ADR baseline against current source, tests and newly merged changes.

Start with privacy, data meaning, identity, compatibility and major architectural boundaries. Expand coverage based on risk and observed change frequency, not ADR count.

## What success means

Success is **not** the number of ADRs or the percentage of tasks forced to read them. Useful signals include: fewer accidental reversals of documented constraints; faster explanation of why a design exists; fewer repeated debates; clearer impact reviews; and shorter time to resolve discrepancies between historical intent and present behavior.

These are evaluation hypotheses, not measured outcomes. The practice should remain lightweight enough that routine development is not slowed by document ceremony.

## Limits

A historical PR merge does not prove formal design approval. A current test definition does not prove it passes. ADR coverage can be incomplete, especially for decisions made outside version control. The routing map can miss indirect dependencies. Periodic audits and human review remain necessary.

## Reference implementation

kiroku keeps the corresponding procedures in [reverse-adr](../../.claude/skills/reverse-adr/SKILL.md) and [adr-guard](../../.claude/skills/adr-guard/SKILL.md), with a repository-specific routing index in [ADR README](README.md). This document describes the **general practice** and is not dependent on kiroku, Go, Claude Code or any specific agent framework.
