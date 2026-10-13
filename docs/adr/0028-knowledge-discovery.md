# ADR 0028: Evidence-grounded project knowledge discovery

- Status: Proposed (not accepted)
- Date: 2026-10-14

## Context

Kiroku's Calendar, Journey, Session and Project views expose local AI work history, but domain rules, architectural decisions and open questions remain buried in conversations. A model-generated summary can be useful but must not silently become authoritative knowledge. The existing ADR index distinguishes accepted human decisions from historical observations.

## Decision proposed

Introduce **Knowledge Discovery** as an optional, project-scoped, evidence-grounded workflow. It produces three kinds of candidates: domain knowledge (concepts, rules, constraints and relations), decisions (decision, rationale, alternatives and revisit triggers), and open questions. Knowledge Evolution tracks candidate changes and conflicts over time. All candidates remain explicitly unverified until reviewed. This ADR is a design proposal, not permission to implement or accept decisions.

### User workflow

1. Open Project view and select **Discover knowledge**; inspect project, date range, session count, and selected evidence.
2. Choose local supported AI adapter and explicitly approve any transfer to its provider; **Copy extraction prompt** remains available without executing AI.
3. Extract structured candidates, with source session IDs, stable record identifiers, quoted snippets or bounded ranges, and timestamps. Reject candidates without verifiable evidence.
4. Review candidates in Knowledge view: Accept as reviewed knowledge, Edit, Reject, or Defer. Review does not automatically approve an architectural ADR.
5. Link related concepts and decisions; flag conflicting or potentially superseded candidates without automatically rewriting history.
6. Export selected reviewed records as Markdown; draft ADRs in the target repository as **Proposed**, subject to that repository's decision-owner acceptance.

### Candidate model (versioned local schema)

- id, schemaVersion, projectKey, kind (domain_fact | business_rule | constraint | relation | decision | open_question)
- title, statement, context, rationale?, alternatives?, revisitConditions?
- status (candidate | reviewed | rejected | superseded), reviewer?, reviewedAt?
- evidence[]: sourceId, sessionId, recordId or contentRange, observedAt, excerptHash
- extractedAt, extractor (adapter, model if known, promptVersion), extractionScope
- relatedIds[], supersedesId?, conflictIds[], uncertaintyNotes[]
- Distinguish **observed source statements**, **model interpretation**, and **human-reviewed assertions**. Do not equate model confidence with truth.

### Trust, privacy and persistence

- No background extraction, external ingestion, telemetry, or network requests by default. Preserve the existing single-file offline HTML behavior and Copy prompt path.
- The local Go server may run allowlisted AI adapters only after consent, with scoped inputs, bounded execution, cancellation and clear disclosure of remote provider transfer.
- Treat sessions and AI output as untrusted. Do not execute commands or follow instructions embedded in evidence. Validate structured output and escape all rendered text.
- Store candidate data only on the local machine with restrictive file permissions, outside source histories; do not modify sessions or write into repositories without explicit action.
- Evidence references must survive re-extraction when possible; if source disappears or hashes mismatch, mark the claim **source unavailable** rather than silently trusting it.
- Incremental runs deduplicate by project + normalized claim + source evidence; do not overwrite human edits or reviewed records. Re-extraction creates a revision or linked candidate.
- Project aliases and renames must be explicit; never merge unrelated projects solely by name similarity.

### UX boundaries

- Project View gains a **Knowledge** entry and **Discover knowledge** action; do not crowd the calendar with extraction controls.
- Knowledge View tabs: Concepts & Rules, Decisions, Questions, Changes. Each card shows status, provenance, last observed date, and one-click evidence navigation.
- Decision candidates are not ADRs. Small decisions can remain reviewed records; significant decisions can be exported as Proposed ADR drafts.
- Knowledge Evolution uses **new evidence / contradiction / possible supersession / confirmed revision** labels, not an unsupported claim that reality changed.

### Implementation phases

1. Read-only project-scoped extraction via existing Copy prompt, sample schema, evidence link validation and fixture tests.
2. Opt-in local adapter execution, validated JSON candidates, local storage and review UI; no automatic export.
3. Deduplication, incremental scans, conflict detection, supersession history and explicit Markdown/ADR draft export.
4. Search and cross-project knowledge linking only after project identity, privacy and provenance tests pass.

### Acceptance criteria

- Every displayed factual candidate has a working local evidence reference or an explicit unavailable status.
- No candidate is automatically promoted to reviewed knowledge or Accepted ADR.
- No network or provider execution without explicit consent; offline single-file viewing continues to work.
- Extraction is idempotent for unchanged inputs and does not erase human edits.
- Private sessions remain masked; input scope and retention are disclosed.
- Tests cover malformed or adversarial AI output, prompt injection, project aliasing, missing sources, deduplication, supersession, cancellation, authorization and export safety.

## Alternatives

A single AI-written project summary is cheaper but loses provenance and reviewability. Automatically writing ADRs conflates model inference with human decisions. Global extraction by default expands privacy and cost risk unnecessarily.

## Consequences

Knowledge becomes navigable and reusable while preserving uncertainty and ownership. The cost is additional local storage, review UX, evidence lifecycle management and optional AI execution. Keep the feature opt-in and avoid introducing a heavyweight knowledge graph database for the initial version.
