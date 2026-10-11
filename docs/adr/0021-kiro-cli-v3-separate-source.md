# ADR 0021: Read Kiro CLI V3 as a separate source from Kiro CLI V2

Status: Proposed
Date: 2026-10-11
Scope: History sources — Kiro CLI (`internal/source/kiro.go`, `internal/core/agents.go`, `internal/core/records.go`, `docs/sources.md`)
Related ADRs: None
Related Design Logs: [DL 0010 — agent source coverage](../design-log/0010-agent-source-coverage.md), [DL 0004 — session identity and deduplication](../design-log/0004-session-identity-and-deduplication.md)
Supersedes/Amends: None
Decision owner: Repository maintainer (acceptance pending)

## Context

kiroku reads Kiro CLI history from two stores: the SQLite database (`conversations` / `conversations_v2`, shown as "Kiro CLI (SQLite)" and read by `qstore.go`) and the V2 JSON sessions in `~/.kiro/sessions/cli/` (shown as "Kiro CLI" and read by `kiro.go`). It does not read Kiro CLI V3.

Documented (checked 2026-10-11):

- V3 is a separate agent engine. It is started with `kiro-cli --v3`, or `kiro-cli acp --agent-engine=v3` for ACP clients, and runs alongside 2.x ([V3](https://kiro.dev/docs/cli/v3/)).
- Its session store is "not backward-compatible": V2 cannot open a V3 session because the formats differ. Existing V2 sessions are not converted or deleted when V3 is used ([migration guide](https://kiro.dev/docs/cli/v3/migration-guide/)).
- Every Kiro surface (IDE, CLI, Web, Mobile) is a front end to the same "unified agent harness" ([How Kiro works](https://kiro.dev/docs/how-kiro-works/)).
- V3 is becoming the default:
  - CLI 2.26 shows a deprecation notice on Classic sessions.
  - CLI 2.28 offers to switch Classic sessions to 3.0.
  - CLI 2.29 adds memory across sessions for V3 only ([changelog](https://kiro.dev/changelog/)).
- Local sessions are kept in "a SQLite database in `~/.kiro/`". `KIRO_HOME` moves the sessions ([session management](https://kiro.dev/docs/cli/chat/session-management/), [settings](https://kiro.dev/docs/reference/settings/)).

Observed in Kiro Crew's public source (kirodotdev/kirocrew, 2026-10-10):

- The V3 engine is the KAS backend, and it "keeps its own store", separate from kiro-cli's sessions directory (`acp/runtime.py`, `acp/session_handle.py`).
- Crew deletes a KAS session's record when it frees the session.
- A global `chat.agentEngine` setting makes plain `kiro-cli chat` use V3 (`dashboard/handlers/sessions.py`).

Unknown:

- The exact path, file names and format of the V3 store.
- Whether V3 records credits, context usage, requests or compactions.
- Whether V3 CLI sessions share a layout with Kiro IDE 1.0 sessions (`~/.kiro/sessions/<hash>/<id>/session.json`). If they do, kiroku's Kiro IDE reader could count them as Kiro IDE today.

No public reference implementation (kiro-history, codeburn) reads V3.

## Decision

1. **A separate source.** When kiroku reads Kiro CLI V3, it does so through its own `Source`. That source has its own display name (working name "Kiro CLI (V3)"), its own `core.Records` entry, its own `--sources` handling within the `kiro` family, and its own column in "What history records". It is not folded into the V2 "Kiro CLI" source.
2. **Share the parser, not the identity.** If the V3 store turns out to use the same format as Kiro IDE 1.0, the parser code may be shared with the IDE reader. Each session still keeps the source label of the surface that created it.
3. **Make sure V3 sessions are not counted as Kiro IDE.** A V3 session must never be counted as a Kiro IDE session, whatever the store layout turns out to be.
4. **Evidence first.** The reader is built only from a real V3 store (its paths, schema and a sanitized sample) or from published documentation. It is not built from guesses. Until then, the docs say that V3 sessions are not read.
5. **No V2–V3 deduplication by default.** V2 sessions are not converted to V3, so the two stores are not deduplicated against each other. The existing V1 (SQLite) ↔ V2 deduplication is unchanged.

## Alternatives

- **Merge V3 into the existing "Kiro CLI" source.** One name is simpler for users. However, `core.Records` is per source, so mixing V2 (which records requests) with V3 (whose metrics are unknown) would break "not recorded" versus zero. It would also make it harder to drop V2 cleanly once it is retired (`/drop-agent`). Not chosen.
- **Read V3 as part of the Kiro IDE source, because it shares the harness.** That would mislabel CLI work as IDE work. Not chosen.
- **Do nothing.** Once V3 is the default, most Kiro CLI history becomes invisible to kiroku, and it may be counted as Kiro IDE. Not chosen.

## Consequences

- Users see one more source name in the agent list, the Data sources panel and `--sources` help. The docs need to explain V2 versus V3.
- The UI gains a fourth Kiro CLI–related label. It can be revisited after V2 is retired, for example by renaming V3 to "Kiro CLI" and dropping V2. That later change would need its own review, because it touches compatibility ([DL 0006](../design-log/0006-compatibility-contract.md)).
- Kiro Crew's KAS-backend conversations get their tokens and cost from Crew's usage records today. If the V3 store also holds them, the new reader will need a claim/yield rule like the one Codex-backend conversations got in #353. Otherwise they would be counted twice. Crew deletes KAS session records at teardown, which limits the overlap.
- Security:
  - The V3 store is untrusted input like any other history.
  - If it is SQLite, it must be opened read-only, as `qstore.go` does.
  - Paths derived from session fields must be validated.

## Verification

- Before implementation, collect `kiro-cli --version`, a file listing of `${KIRO_HOME:-~/.kiro}` (names only) and the store's schema (`sqlite3 … .schema`) from a machine that has used V3.
- The implementation PR adds:
  - fixtures derived from that sample;
  - a test that a V3 session is never counted as Kiro IDE;
  - a `core.Records` entry and a docs column;
  - the hostile-tester and user-tester runs required by `/pre-pr`.
- Acceptance: a maintainer reviews this ADR and marks it Accepted, or amends the naming.

## Open questions

- The final display name ("Kiro CLI (V3)" or something else) and the `--sources` spelling.
- The V3 store location and format, and whether it overlaps the Kiro IDE 1.0 layout.
- Whether V3 records credits per turn, as V2's `metering_usage` does.
