# ADR 0021: Read Kiro CLI V3 apart from Kiro CLI V2

Status: Accepted (2026-10-11, approved by the maintainer in review of #359)
Date: 2026-10-11
Scope: History sources — Kiro (`internal/source/kiro.go`, `internal/core/agents.go`, `internal/core/records.go`, `docs/sources.md`)
Related ADRs: None
Related Decision Logs: [DL 0010 — agent source coverage](../decision-log/0010-agent-source-coverage.md), [DL 0004 — session identity and deduplication](../decision-log/0004-session-identity-and-deduplication.md), [DL 0006 — compatibility contract](../decision-log/0006-compatibility-contract.md)
Supersedes/Amends: None
Decision owner: Repository maintainer

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
- **V2 to V3 conversion.** kiro-cli has an internal V2 → KAS session conversion (`ensure-session`), and the engine resolves a legacy ID `X` as `sess_X`. A converted Classic session would then sit in both `sessions/cli/X.json` (V2) and `sessions/<bucket>/sess_X/` (V3). Confirmed on 2026-10-11 by the maintainer: a Classic session opened with `kiro-cli chat --v3 --resume-id X` showed up in both sources in `kiroku serve`. V3's own resume list (`--resume`, `--resume-picker`) doesn't show Classic sessions, so only an explicit `--resume-id` converts one. The converter's strings in kiro-cli 2.29.0 name the payloads it writes (user, assistant, tool_call, tool_result, summarization, checkpoint_revert, tombstone) and no `usage_summary`, so the copy has no credits for the turns before the switch.

### Consequence for kiroku today

kiroku already reads Kiro CLI V3 sessions, but labels them "Kiro IDE". Nothing in the store tells a CLI V3 session from an IDE session.

## Decision

1. **Read V3 apart from V2.** V3 sessions are read by the reader for the shared store (`sessions/<bucket>/sess_*`), never by the V2 "Kiro CLI" reader. V2 keeps its own source, so it can be retired on its own (`/drop-agent`) once Kiro removes Classic sessions.
2. **Name the sources for what the stores are, using Kiro's own terms.**
   - The shared store (Kiro IDE 1.x and Kiro CLI V3) is shown as **"Kiro IDE / CLI"** instead of "Kiro IDE".
   - The V2 JSON store is shown as **"Kiro CLI (Classic)"** instead of "Kiro CLI", since Kiro calls V2 sessions "Classic".
   - "Kiro CLI (SQLite)", "Kiro IDE (legacy)" and the `--sources` family `kiro` don't change.

   Both renames ship in one release, so users see one breaking change, not two.
3. **Deduplicate V2 against its converted V3 copy.** When the same conversation exists as V2 `X` and V3 `sess_X`, count it once, preferring the V3 copy (the session that continues). This uses the existing Claim/Yield mechanism ([DL 0004](../decision-log/0004-session-identity-and-deduplication.md)). It ships only with a test built from a real converted session, or with the conversion's ID mapping confirmed.
   - Implemented: each Classic session yields to `kiro-ide:sess_<id>`. Because the V3 copy has no credit records for the turns before the switch, the V3 session takes the credits, turns and model requests of the Classic turns that ended before its first `usage_summary`. If a future converter copies the usage, that first `usage_summary` comes no later than the Classic turns, and nothing is added.
4. **No guessed surface split.** kiroku does not try to tell IDE from CLI V3 sessions by heuristics such as title shape or tool names. If Kiro adds a field that records the surface, revisit this.

## Alternatives

- **A separate "Kiro CLI (V3)" source distinct from Kiro IDE.** This was the first draft of this ADR. It isn't possible from the data, because both surfaces write identical sessions into one store with no surface field. Not chosen.
- **Keep the "Kiro IDE" label.** It costs nothing, but it misattributes CLI V3 work to the IDE. That gets worse as V3 becomes the CLI default. Not chosen unless the maintainer prefers to wait for a surface field.
- **Merge V2 into the shared-store source.** V2 has a different format and different metrics (`core.Records`), and it is being deprecated. Not chosen.
- **Do nothing.** This leaves the mislabel and the possible double count after V2 → V3 conversion. Not chosen.

## Consequences

- **The rename is BREAKING for `kiroku json`.** [compatibility.md](../compatibility.md) keeps the meaning of `source` ("the agent, as shown in the view") and gives `Kiro IDE` as an example value. Consumers that match `"Kiro IDE"` or `"Kiro CLI"` must update. The CHANGELOG marks it `BREAKING`, which bumps the minor version while kiroku is 0.x.
  - `schemaVersion` stays 1, because the field's name, type and meaning don't change.
  - `--sources`, environment variables and the archive layout don't change.
  - Inside kiroku, `core.Agents`, `core.Records`, `core.Native`, the per-agent colors and any saved per-agent view state keyed by the label must move to the new names.
- **Weighing it.** Keeping the old labels would avoid the breaking change. But once V3 is the CLI default, most Kiro CLI work would be shown as "Kiro IDE". The maintainer accepted the rename as the lesser cost.
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

- Whether a converted V1 (SQLite) conversation is also copied as `sess_<conversation id>`. kiro-cli has a V1 → V2 export step, but no converted V1 conversation has been seen, so "Kiro CLI (SQLite)" doesn't yield to V3.
- Whether `cloud-sandbox` sessions (`executionTarget`) are also written to the local store, and whether they should count as local activity.
