# ADR 0023: What stays compatible within a major version

Status: Accepted (2026-10-11, approved by the maintainer when promoting Decision Log 0006)
Date: 2026-10-11
Scope: Public contract — CLI, `kiroku json`, `--prices`, archive copies, the serve key, release artifacts (`internal/cli`, `internal/archive`, `docs/compatibility.md`, `testdata/compat/`)
Related ADRs: [ADR 0021](0021-kiro-cli-v3-separate-source.md) (a breaking change made under this contract), [ADR 0022](0022-history-stays-local.md)
Related Decision Logs: DL 0006 — compatibility contract
Supersedes/Amends: None
Decision owner: Repository maintainer

## Context

DL 0006 records the policy set in [#208](https://github.com/MichinaoShimizu/kiroku/pull/208) and [#209](https://github.com/MichinaoShimizu/kiroku/issues/209). `docs/compatibility.md` states it in detail. As a Decision Log it is context only; this ADR makes it a binding decision.

## Decision

1. **Kept compatible within a major version:** CLI commands, options, defaults and environment variables; exit status; the fields listed for `kiroku json` (`schemaVersion: 1`); the `--prices` input format; archive copies; the serve key; serving locally by default; the shape of release artifacts that the installer and `kiroku update` rely on.
2. **May change in any release:** the view, metrics and calculated values, estimated costs, wording, generated HTML, which history formats are read, JSON fields outside the listed contract, and the internal `/data.json` endpoint.
3. **Security fixes may break compatibility** when there is no compatible fix.
4. **While kiroku is 0.x**, a breaking change is allowed but is marked `BREAKING` in `CHANGELOG.md`, which bumps the minor version.

`docs/compatibility.md` is the normative list.

## Alternatives

- **Keep it as Decision Log 0006 plus `docs/compatibility.md`.** ADR Guard wouldn't flag a change to a protected surface as a conflict. Not chosen.
- **Treat `/data.json` as public too.** It would freeze the view's internal transport. Not chosen, as in #208.

## Consequences

- Removing or renaming anything in point 1 needs a new major version (or, during 0.x, a `BREAKING` entry) and is a conflict ADR Guard should report.
- Readers of history can change freely; users' scripts should use `kiroku json`, not `/data.json`.
- Revisit when 1.0 ships, to decide whether 0.x `BREAKING` changes remain allowed.

## Verification

Run on `23116a3` (2026-10-11), all passed: `internal/cli` `TestJSONSchema`, `TestCompatArchive`, `TestCompatServeKey`.

No test checks that `/data.json` stays outside the contract; that boundary is documentary.

## Open questions

None for acceptance.
