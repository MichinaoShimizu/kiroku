---
kind: log
category: compatibility
keywords: [Kiro IDE / CLI, Kiro CLI (Classic), sess_, sessions/cli, KiroIDE.Name, KiroCLI.Name, Agent.About]
---

# [kiroku] Kiro CLI V3 is read with Kiro IDE as "Kiro IDE / CLI", and V2 becomes "Kiro CLI (Classic)"

> A past observation, not a decision. It binds nothing.

## What happened

- C1 2026-10-11 Still true: kiro-cli 2.29.0 was found to write V3 sessions to `~/.kiro/sessions/<hash>/sess_<id>/`, the same store as Kiro IDE, with nothing recording which created a session; kiroku reads that store as one source and does not guess. History: [PR #359](https://github.com/MichinaoShimizu/kiroku/pull/359) / Now: [`docs/sources.md` @`1d7b409`](https://github.com/MichinaoShimizu/kiroku/blob/1d7b4091906cc02d4f32bdabd120f5120dcf0792/docs/sources.md)
- C2 2026-10-11 Still true: the source labels became "Kiro IDE / CLI" (was "Kiro IDE") and "Kiro CLI (Classic)" (was "Kiro CLI"), which changes the `source` value in `kiroku json` (BREAKING); slots, marks, colors, `--sources kiro` and the archive layout did not change, and `schemaVersion` stayed 1. History: [PR #361](https://github.com/MichinaoShimizu/kiroku/pull/361) / Now: [`internal/source/kiro.go` `Name` @`1d7b409`](https://github.com/MichinaoShimizu/kiroku/blob/1d7b4091906cc02d4f32bdabd120f5120dcf0792/internal/source/kiro.go), [`internal/core/agents.go` @`1d7b409`](https://github.com/MichinaoShimizu/kiroku/blob/1d7b4091906cc02d4f32bdabd120f5120dcf0792/internal/core/agents.go)

## Reason

Documented (description, AI): keeping "Kiro IDE" would credit CLI V3 work to the IDE as V3 becomes the CLI default, and V2 keeps its own source so it can be dropped on its own later; `schemaVersion` stays 1 because the field's name, type and meaning are unchanged ([PR #359](https://github.com/MichinaoShimizu/kiroku/pull/359), [PR #361](https://github.com/MichinaoShimizu/kiroku/pull/361)). Alternatives the PR's ADR 0021 lists as not chosen: a separate "Kiro CLI (V3)" source (impossible from the data), keeping the "Kiro IDE" label, merging V2 into the shared store, doing nothing.

## People involved

From the PR records; not necessarily who decided.

- PR #359: opened @MichinaoShimizu (2026-10-11), no approving review, merged @MichinaoShimizu (2026-10-11)
- PR #361: opened @MichinaoShimizu (2026-10-11), no approving review, merged @MichinaoShimizu (2026-10-11)

## Open questions

- ADR 0021 says it was "approved by the maintainer in review of #359", but #359 has no review or comment. Where was the label chosen? (ask @MichinaoShimizu)
- Do the reasons above, written by an AI agent in the PR descriptions, match the maintainer's own judgement? (ask @MichinaoShimizu)

## Sources

- [ADR 0021 (removed)](https://github.com/MichinaoShimizu/kiroku/blob/27e8298bdc0367afc623bc4db5c450ca0bad38b1/docs/adr/0021-kiro-cli-v3-separate-source.md)

## Related logs

- DL-8
