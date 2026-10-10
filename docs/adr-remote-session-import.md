# ADR: Local-first remote session import (proposed)

Status: **Proposed — Phase 0**, tracked in [#324](https://github.com/MichinaoShimizu/kiroku/issues/324) under [#323](https://github.com/MichinaoShimizu/kiroku/issues/323).

## Context

kiroku reads local histories via `internal/source.Source` and `source.All(Options)`. `internal/cli/load.go` reads sources concurrently and deduplicates by `Builder.Key` and `Claim/Yield`; `internal/cli/cache.go` caches units by source and file stamps. `core.Session` stores source, ID, project path and history file, but no remote-origin identity. Git enrichment in `internal/gitlog` uses local repositories identified from session project paths. `internal/archive` stores zstd copies of selected agent histories and has its own deletion lifecycle.

## Decision proposal

1. **Pull to local storage, then parse natively.** Do not run a kiroku-hosted server or treat `kiroku json` (an output contract) as an import format. Start with manually copied native Claude Code and Codex CLI histories; reuse their readers.
2. **Separate imported storage from archive.** Use a private, kiroku-managed import root and manifest; import deletion must not be conflated with `kiroku archive off`. Use atomic writes, strict path validation and explicit cleanup commands.
3. **Version the import manifest.** Suggested v1 fields: `schemaVersion`, `originID`, `displayName`, `agentFamily`, `exportedAt`, `files[{path,sha256,size}]`. The manifest identifies a source bundle, not a parsed session. Source-provided IDs and paths are untrusted. This schema is tentative pending fixtures.
4. **Namespace session identity.** Separate upstream session ID from kiroku's global identity: conceptual key `(originID, agentFamily, upstreamSessionID)`. Keep the existing within-source `Key` and `Claim/Yield` behavior; do not blindly prefix keys before checking cross-format Kiro suppression. A file digest is for change detection, not a universal session ID.
5. **Preserve provenance.** Keep local vs imported origin visible in reports, session details and JSON; distinguish original remote project path from a local path. Remote project paths must never be used for local git enrichment unless the user explicitly maps and validates a repository.
6. **Import is idempotent.** Reimporting an unchanged bundle does not add sessions. Changed files are updated safely, with a defined policy for removed files and truncated/incomplete histories. Report imported/updated/skipped/rejected counts.
7. **Security first.** Enforce no traversal, symlink escape, unbounded decompression or unsafe executable content; do not log secrets or raw prompts. Reuse existing restrictive permissions, JSONL limits, web escaping and source-file protections. Reject ambiguous manifest versions and unsupported agents.
8. **SSH is a later transport.** Explicit opt-in, host-key verification and least-privilege transfer into the same local import pipeline. Hosted-provider APIs are independent connectors, not assumed available.

## Alternative approaches

- **One environment variable per remote origin:** useful for a manual proof of concept but not a durable multi-origin UX; process-wide overrides displace local sources.
- **Parse exported `kiroku json`:** loses raw-source fidelity and couples import to the output schema; not selected as first implementation.
- **Run kiroku on the remote and expose `serve`:** increases network and authentication attack surface; not selected.
- **Reuse archive as import storage:** archive lifecycle and deletion semantics differ; not selected.

## Validation gates before Accepted

- [ ] Audit reader ID/key construction and Kiro dedup precedence; identify exact source integration points.
- [ ] Synthetic and representative Claude Code/Codex fixtures copied from remote paths; test path and timezone behavior.
- [ ] Test same-session reimport, overlapping local/remote copies, changed/partial files, two origins with identical upstream IDs.
- [ ] Confirm `serve`, `html`, `json`, `stats`, `doctor`, Year in Review and archive behavior with imports.
- [ ] Threat-model hostile bundles and assess bounded resource use and local git path handling.
- [ ] Define import retention/deletion and recovery UX, plus stable versioning policy.

No implementation or test execution is claimed by this ADR.
