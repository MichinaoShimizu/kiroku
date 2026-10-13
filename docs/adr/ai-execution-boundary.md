# ADR: Optional local AI execution boundary

Status: Proposed

## Context
kiroku currently renders a self-contained HTML report and, in serve mode, exposes local authenticated endpoints for history and prompt retrieval. Reviews are generated as prompts in browser JavaScript. Optional AI execution introduces a new trust boundary: local private history can be passed to an installed agent that may contact a remote provider.

## Decision
Keep the default product offline and preserve Copy prompt. Introduce an explicit opt-in local execution service only in authenticated serve mode. The browser may request one of a fixed set of review scopes and an allowlisted agent, never provide executable command strings or arbitrary filesystem paths. Reuse existing prompt builders or move them to a shared tested representation, without silently changing report semantics.

Enforce same-origin, existing serve key, POST-only requests, request size caps, CSRF header, concurrency and runtime limits, bounded output, cancellation and restrictive local result storage. Prompt content is untrusted and cannot override execution policy. The selected adapter runs with read-only permissions where supported, and no automatic approval of writes. Show an explicit data disclosure before first run. Agent availability does not imply authentication or supported CLI version.

A generated result is commentary, not a source of truth; preserve provenance and evidence references. Do not auto-publish ADRs.

## Alternatives
- Keep copy-only: lowest risk, but forces context switching.
- Direct provider API: richer streaming but introduces credential management and new provider-specific contracts.
- Local CLI adapter: reuses installed agent credentials and tooling, but needs version-specific safety validation.

## Consequences
The product's 'no AI calls' claim remains true only when optional execution is disabled. CLI invocation may result in network calls, billing and data disclosure. Standalone HTML must remain read-only and never gain a privileged execution endpoint.

## Uncertainties and validation
Verify Claude and Codex noninteractive flags, stdin behavior, tool restrictions, cwd isolation, environment exposure, process-tree cancellation and output formats on supported platforms before exposing Run in the UI. Kiro compatibility remains unverified. Test hostile prompts, path traversal, CSRF, oversized requests, output flood, cancellation and concurrent jobs.
