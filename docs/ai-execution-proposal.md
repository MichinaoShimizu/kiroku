# Optional AI execution — implementation proposal

Status: Proposal (not implemented)

## Goal
Allow users to run existing Session, Daily, Weekly, and Monthly reviews from kiroku, while preserving the existing Copy prompt workflow and local-first defaults. Extend the same execution layer to Project Review and ADR Discovery.

## Principles
- AI execution is explicitly opt-in; no external AI calls by default.
- Keep Copy prompt available for every existing workflow.
- Clearly disclose that local agent CLIs may send selected context to remote model providers.
- Never claim support for a provider until its CLI, authentication, headless invocation, output and cancellation semantics are verified.
- No arbitrary shell commands from the browser; backend uses an allowlist of adapters and fixed argument construction.
- Prefer read-only execution with bounded time, input, output, and resource usage.
- Do not silently publish reports or ADRs. Human approval is required before writing source-controlled ADRs.

## Proposed design
UI action -> local Go HTTP endpoint -> scoped context builder -> execution job manager -> agent adapter -> structured result store -> UI.

Agent adapter interface: availability check, start, stream output, cancel, normalize result. Initial candidates: Claude Code and Codex CLI; Kiro support subject to compatibility validation. Avoid placing secrets or sensitive history in process arguments or logs.

Execution jobs should include an id, scope, selected agent, input references, status, timestamps, error category, result location and provenance. Store results locally with restrictive permissions; distinguish observed facts, cited evidence, model inference and user-approved decisions. Treat model output as untrusted.

## Feature matrix
| Scope | Existing | Proposed |
| --- | --- | --- |
| Session | Copy review prompt | Run Review + Copy |
| Daily | Copy report prompt | Generate Daily Report + Copy |
| Weekly | Copy report prompt | Generate Weekly Report + Copy |
| Monthly | Copy report prompt | Generate Monthly Report + Copy |
| Project | Project view | Generate Project Review |
| ADR | Not implemented | Discover decision candidates with evidence; approve before PR |

## Delivery stages
1. Inventory current prompt builders, server routes, local persistence and tests; record an ADR for the adapter and trust boundaries.
2. Implement job manager, explicit opt-in, cancellation, provider detection and a single adapter; integrate Session and Daily using existing prompt generation.
3. Integrate Weekly and Monthly; add result persistence, rerun and error states; test disabled mode and Copy regressions.
4. Add Project Review and ADR discovery with source references, confidence/uncertainty and human review; keep canonical ADR Markdown in source repositories.
5. Update README.md, README.ja.md, docs/guide.md, docs/compatibility.md, SECURITY.md, screenshots/demo, changelog and release notes **with implementation**, not ahead of it.

## Acceptance criteria
- No change in network behavior when AI execution is disabled.
- All four existing review/report scopes have Run and Copy actions.
- Execution is cancellable, bounded and does not permit arbitrary user-controlled shell invocation.
- Provider missing, login expired, timeout, malformed output and sensitive-context warnings have clear UI states.
- Each generated assertion can link back to evidence where possible; no unsupported output presented as fact.
- Tests cover API authorization, local-only defaults, prompt parity, path traversal, command injection, concurrency and result permissions.
- English and Japanese documentation, security disclosures, screenshots and release notes match the shipped behavior.

This proposal is intentionally documentation-only; no AI integration is implemented by this change.
