---
kind: log
category: compatibility
keywords: [docs/compatibility.md, schemaVersion, jsonSchema, legacyForm, --claude-root, testdata/compat, compatVersions, BREAKING]
---

# [kiroku] What stays compatible within a major version is written down, and kiroku json is versioned

> A past observation, not a decision. It binds nothing.

## What happened

- C1 2026-10-07 Still true: `docs/compatibility.md` lists what stays compatible within a major version from v1.0.0 (commands, options, env vars, exit status, the listed `kiroku json` fields, the `--prices` file, archive copies, the serve key, local-only serve, release file names and `checksums.txt`); everything else, including `/data.json`, may change, and security fixes win. History: [PR #208](https://github.com/MichinaoShimizu/kiroku/pull/208) / Now: [`docs/compatibility.md` @`1d7b409`](https://github.com/MichinaoShimizu/kiroku/blob/1d7b4091906cc02d4f32bdabd120f5120dcf0792/docs/compatibility.md)
- C2 2026-10-07 Still true: `kiroku json` writes `"schemaVersion": 1`. History: [PR #208](https://github.com/MichinaoShimizu/kiroku/pull/208) / Now: [`internal/cli/cli.go` `jsonSchema` @`1d7b409`](https://github.com/MichinaoShimizu/kiroku/blob/1d7b4091906cc02d4f32bdabd120f5120dcf0792/internal/cli/cli.go)
- C3 2026-10-07 Still true: the legacy forms `kiroku --serve`, `--json` and `-o` were removed (they now only name the new command), and `--root` was renamed `--claude-root` (BREAKING). History: [PR #208](https://github.com/MichinaoShimizu/kiroku/pull/208) / Now: [`internal/cli/cli.go` `legacyForm` @`1d7b409`](https://github.com/MichinaoShimizu/kiroku/blob/1d7b4091906cc02d4f32bdabd120f5120dcf0792/internal/cli/cli.go)
- C4 2026-10-07 Still true: fixtures in `testdata/compat/v0.19/` check that later versions read an old archive folder and serve key. History: [PR #211](https://github.com/MichinaoShimizu/kiroku/pull/211) / Now: [`internal/cli/compat_test.go` `compatVersions` @`1d7b409`](https://github.com/MichinaoShimizu/kiroku/blob/1d7b4091906cc02d4f32bdabd120f5120dcf0792/internal/cli/compat_test.go)

## Reason

Documented (description): the maintainer's own checklist issue, with no other participants, records the choices: "Decide the JSON output → decided: versioned", "legacy forms → decided: remove in 0.x", "decided: rename `--root` to `--claude-root`", and the other option names stay ([issue #209](https://github.com/MichinaoShimizu/kiroku/issues/209)). The PR adds that the rules are set before 1.0 freezes the public surface, and that `--root` is renamed because the other history options name their agent ([PR #208](https://github.com/MichinaoShimizu/kiroku/pull/208)).

`docs/compatibility.md` was later amended: the serve key is kept in the browser's site data (DL-1), and "Analyze with AI" is on by default as the one exception to local-only serve (DL-10).

## People involved

From the PR records; not necessarily who decided.

- Issue #209: opened @MichinaoShimizu (2026-10-07), closed as completed @MichinaoShimizu (2026-10-09)
- PR #208: opened @MichinaoShimizu (2026-10-07), no approving review, merged @MichinaoShimizu (2026-10-07)
- PR #211: opened @MichinaoShimizu (2026-10-07), no approving review, merged @MichinaoShimizu (2026-10-07)

## Open questions

- #209 was closed as completed, but its "Let it settle" and "Release v1.0.0" items are unchecked and kiroku is still 0.x. Was the 1.0 plan dropped or deferred? (ask @MichinaoShimizu)
- After 1.0, does changing a `source` value need a `schemaVersion` bump, or only a BREAKING entry (as #361 did)? (ask @MichinaoShimizu)
- The reasons in PR #208 were written by an AI agent. Do they match the maintainer's own judgement? (ask @MichinaoShimizu)

## Sources

- [ADR 0023 (removed)](https://github.com/MichinaoShimizu/kiroku/blob/27e8298bdc0367afc623bc4db5c450ca0bad38b1/docs/adr/0023-compatibility-contract.md)
- [old Decision Log 0006 (removed)](https://github.com/MichinaoShimizu/kiroku/blob/ef66c137df557123c0f71a342dbf2f82be064131/docs/decision-log/0006-compatibility-contract.md)

## Related logs

- DL-9
- DL-1
- DL-10
