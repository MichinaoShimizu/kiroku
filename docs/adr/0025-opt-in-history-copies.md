# ADR 0025: kiroku keeps copies of history only when the user turns it on

Status: Accepted (2026-10-11, approved by the maintainer when promoting Design Log 0007)
Date: 2026-10-11
Scope: History retention — `kiroku archive` (`internal/archive`, `internal/cli`, `docs/guide.md`)
Related ADRs: [ADR 0022](0022-history-stays-local.md), [ADR 0023](0023-compatibility-contract.md) (archive copies stay readable)
Related Design Logs: [DL 0007 — local history retention](../design-log/0007-local-history-retention.md)
Supersedes/Amends: None
Decision owner: Repository maintainer

## Context

Some agents delete their own history (Claude Code after 30 days by default, Kiro Crew's archived records). `kiroku archive` keeps compressed copies so kiroku can still show it. [DL 0007](../design-log/0007-local-history-retention.md) records how this was built; this ADR makes the rules binding.

A copy means the conversation exists in one more place, including conversations the user deleted on purpose and the text of Kiro Crew private conversations. That is why it must be the user's choice.

## Decision

1. **Opt-in.** kiroku copies history only after the user runs `kiroku archive on` (or presses the backup button in the view). It is off by default.
2. **Only history an agent deletes by itself is copied** (Claude Code conversations, Kiro Crew's `sessions/archive/`).
3. **Copies are private and local:** `0600`/`0700`, in kiroku's own folder, never sent anywhere ([ADR 0022](0022-history-stays-local.md)).
4. **Keeping is not showing.** A private conversation's copy is still shown as counts and times only.
5. **The user can remove them.** `kiroku archive off` offers to delete only the `.zst` copies kiroku made, and only in a folder where `archive on` was run.

## Alternatives

- **Copy by default.** It would protect more history, but it silently duplicates history the user may have deleted on purpose. Not chosen.
- **Keep it as Design Log 0007.** ADR Guard wouldn't flag a change that turns copying on by default. Not chosen.

## Consequences

- Users who don't turn it on lose history their agents delete; `kiroku doctor`, the view and the README point them to `kiroku archive on` or the agent's own setting.
- Turning copying on by default, copying more agents' history, or showing private copies needs a new ADR.

## Verification

Run on `23116a3` (2026-10-11), all passed: `internal/cli` `TestArchiveCommand`, `TestArchiveOffDeletesOnlyCopies`, `TestServeArchive`, `TestCompatArchive`; `internal/archive` package tests.

## Open questions

None for acceptance.
