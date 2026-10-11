# ADR 0021: Read Kiro CLI V3 apart from Kiro CLI V2

Status: Proposed
Date: 2026-10-11
Scope: History sources — Kiro (`internal/source/kiro.go`, `internal/core/agents.go`, `internal/core/records.go`, `docs/sources.md`)
Related ADRs: None
Related Design Logs: [DL 0010 — agent source coverage](../design-log/0010-agent-source-coverage.md), [DL 0004 — session identity and deduplication](../design-log/0004-session-identity-and-deduplication.md), [DL 0006 — compatibility contract](../design-log/0006-compatibility-contract.md)
Supersedes/Amends: None
Decision owner: Repository maintainer (acceptance pending)

## Context

kiroku reads Kiro history in three readers:

| Reader | Store | Shown as |
|---|---|---|
| `qstore.go` | the SQLite database (`conversations` / `conversations_v2`) | "Kiro CLI (SQLite)" |
| `kiro.go` | the V2 JSON sessions in `~/.kiro/sessions/cli/` | "Kiro CLI" |
| `kiro.go` | `~/.kiro/sessions/<hash>/<id>/session.json` + `messages.jsonl` | "Kiro IDE" |

### Documented (Kiro docs and changelog, checked 2026-10-11)

- V3 is a separate agent engine, started with `kiro-cli --v3` ([V3](https://kiro.dev/docs/cli/v3/)).
- Its session store is not backward-compatible: V2 can't open V3 sessions. Trying V3 doesn't convert or delete V2 sessions ([migration guide](https://kiro.dev/docs/cli/v3/migration-guide/)).
- Every Kiro surface (IDE, CLI, Web, Mobile) is a front end to the same "unified agent harness" ([How Kiro works](https://kiro.dev/docs/how-kiro-works/)).
- `KIRO_HOME` moves the sessions ([settings](https://kiro.dev/docs/reference/settings/)).
- V3 is becoming the default. CLI 2.26 shows a deprecation notice on Classic sessions, and CLI 2.28 offers to switch an interactive Classic session to 3.0 ([changelog](https://kiro.dev/changelog/)).

### Observed in kiro-cli 2.29.0

Sources:

- The Linux x86_64 release archive, checked against the SHA-256 in its `manifest.json`.
- The `@kiro/agent` 0.66.33 bundle embedded in it (the V3 engine, which Kiro Crew calls KAS; license AWS-IPL).

Both were read as data only. Nothing was run, and no code was copied.

- **Same store as the IDE.** The V3 engine keeps sessions under `sessionsPath`, which defaults to `<home>/.kiro/sessions`.
  - Each workspace gets a bucket named by the first 16 hex characters of a SHA-256 over its workspace paths.
  - Each session is a directory `sess_<uuid>` holding `session.json` and `messages.jsonl`.
  - kiro-cli's own help for its internal session export says: "V2 uses a UUID; KAS uses the `sess_<uuid>` form", and "Sessions root. Defaults to `KIRO_HOME/.kiro/sessions`".
  - This is exactly the layout kiroku already reads as "Kiro IDE" (the comment in `kiro.go` names `sessions/<hash>/sess_<id>/`).
- **What `session.json` holds.** The schema has these fields:
  - `schemaVersion`, `id`, `title`, `agentMode`, `workspacePaths`, `createdAt`, `lastModifiedAt`, `modelId`, `status`;
  - fork and copy fields: `parentSessionId`, `forkedAtMessageId`, `parentExecutionId`;
  - `createdReason` (`human`, `rewind`, `subagent`, `tangent`);
  - `executionTarget` (`local`, `cloud-sandbox`).

  **No field records which surface (IDE or CLI) created the session.**
- **Usage lines.** `messages.jsonl` carries `usage_summary` lines with `promptTurnSummaries[].usage` (credits) and context usage, the same shapes kiroku's Kiro IDE reader parses.
- **V2 to V3 conversion.** kiro-cli has an internal V2 → KAS session conversion (`ensure-session`), and the engine resolves a legacy ID `X` as `sess_X`. A converted Classic session would then sit in both `sessions/cli/X.json` (V2) and `sessions/<bucket>/sess_X/` (V3). This is inferred from the ID compatibility code and has not been confirmed on a real converted session.

### Consequence for kiroku today

kiroku already reads Kiro CLI V3 sessions, but labels them "Kiro IDE". Nothing in the store tells a CLI V3 session from an IDE session.

## Decision

1. **Read V3 apart from V2.** V3 sessions are read by the reader for the shared store (`sessions/<bucket>/sess_*`), never by the V2 "Kiro CLI" reader. V2 keeps its own source, so it can be retired on its own (`/drop-agent`) once Kiro removes Classic sessions.
2. **Name the source for what the store is.** The store holds sessions from the Kiro IDE and the Kiro CLI V3 alike. Its display name should therefore not claim "IDE". The proposal is to keep the internal source key and change the label to one that covers both surfaces (working name "Kiro"). The final name needs a decision; see Open questions.
3. **Deduplicate V2 against its converted V3 copy.** When the same conversation exists as V2 `X` and V3 `sess_X`, count it once, preferring the V3 copy (the session that continues). This uses the existing Claim/Yield mechanism ([DL 0004](../design-log/0004-session-identity-and-deduplication.md)). It ships only with a test built from a real converted session, or with the conversion's ID mapping confirmed.
4. **No guessed surface split.** kiroku does not try to tell IDE from CLI V3 sessions by heuristics such as title shape or tool names. If Kiro adds a field that records the surface, revisit this.

## Alternatives

- **A separate "Kiro CLI (V3)" source distinct from Kiro IDE.** This was the first draft of this ADR. It isn't possible from the data, because both surfaces write identical sessions into one store with no surface field. Not chosen.
- **Keep the "Kiro IDE" label.** It costs nothing, but it misattributes CLI V3 work to the IDE. That gets worse as V3 becomes the CLI default. Not chosen unless the maintainer prefers to wait for a surface field.
- **Merge V2 into the shared-store source.** V2 has a different format and different metrics (`core.Records`), and it is being deprecated. Not chosen.
- **Do nothing.** This leaves the mislabel and the possible double count after V2 → V3 conversion. Not chosen.

## Consequences

- **Renaming the label is user-visible.** The label appears in the agent list, Data sources, reports, `kiroku json` (the `source` field) and possibly `--sources` help. Under [DL 0006](../design-log/0006-compatibility-contract.md), `--sources` names and the archive layout must stay compatible. Keep `kiro` as the `--sources` family and treat the display label as a view change. Check whether anything keys on the literal "Kiro IDE" (`core.Records`, `core.Agents`, saved filters, JSON consumers) before renaming.
- **"Kiro IDE (legacy)" (0.x) stays as it is.** The deduplication of 0.x against 1.0 is unaffected.
- **Kiro Crew.** Crew's V3 (`kas`) conversations get their tokens and cost from Crew's usage records. Crew deletes a KAS session's record when it frees the session, so overlap with the shared store is limited. Before relying on that, check whether a still-running Crew KAS session should yield as Codex-backend conversations do (#353).
- **Security.** Nothing new is read. The same files are already opened with the regular-file guard from #352.

## Verification

- A test that a `sess_<uuid>` session under `sessions/<bucket>/` is shown under the new label, and that `sessions/cli/` is still read as V2.
- For decision 3, one of:
  - a sanitized real pair (V2 `X.json` and V3 `sess_X/`) produced by switching a Classic session to 3.0 on kiro-cli 2.28 or later;
  - an otherwise confirmed ID mapping, plus a test that the pair is counted once.
- `/pre-pr` with the user-tester run, since the label change is visible.
- Acceptance: a maintainer reviews this ADR, chooses the label, and marks it Accepted.

## Open questions

- The display label for the shared store ("Kiro", "Kiro IDE / CLI", or keep "Kiro IDE" with a note).
- Whether the V2 → V3 conversion really keeps the V2 file, and whether the converted ID is always `sess_<V2 id>`.
- Whether `cloud-sandbox` sessions (`executionTarget`) are also written to the local store, and whether they should count as local activity.
