# ADR 0001: Local-session foundation

Status: **Observed (retrospective)**

Scope: **Phase 1 — local sessions**

## Context

kiroku started from local AI-agent history and local-first reporting. This record documents architectural properties observed in the repository at the time of writing, **not** a claim that an earlier ADR or formal approval existed.

## Observed decisions and evidence

Historical anchors: [#1](https://github.com/MichinaoShimizu/kiroku/pull/1) introduced the original Python, local-history, standalone-HTML workflow; [#6](https://github.com/MichinaoShimizu/kiroku/pull/6) migrated to Go and source adapters. The detailed current architecture below is **observed from code**, not proof of every original design motivation.

1. **Native histories are the input.** Agent adapters implement `Source` (`Name`, `Family`, `Where`, `Load`) and are registered by `source.All(Options)` in `internal/source/source.go`. Existing readers include Claude Code, Codex, Kiro IDE/CLI/Crew and Amazon Q.
2. **Normalize into one session model.** Readers emit `core.Builder`, which produces `core.Session` in `internal/core/session.go`. The model includes project, source, ID, prompts, usage, outputs and file information.
3. **Deduplicate at collection.** `internal/cli/load.go` processes sources concurrently and uses `Builder.Key` and `Claim/Yield` to suppress copies; special withholding logic protects Kiro Crew incognito/temporary histories across matching family sources. The exact identity behavior remains an implementation contract to test.
4. **Cache by source and unit changes.** `internal/cli/cache.go` and `internal/source/watch.go` use file stamps and source-specific units to avoid unnecessary reparsing.
5. **Enrich from local Git repositories.** `internal/gitlog/gitlog.go` uses session project paths to locate local repositories and collect commit/push context.
6. **Keep history local.** `kiroku serve` defaults to loopback; HTML/JSON reports are generated locally. `internal/archive/archive.go` optionally preserves selected histories as compressed local copies, with its own enable/disable lifecycle.

## Rationale and consequences

These properties enable a local-only product without a hosted kiroku account or AI API calls. They also imply constraints for future remote import: project paths may not exist locally, session IDs may collide across machines, imported histories must not be mistaken for archive copies, and existing privacy/dedup rules must remain intact.

## Alternatives

Historical alternatives were not recorded; this retrospective ADR does **not** claim that remote hosting, a central database, or a different parser design was formally considered and rejected at the time.

## Verification

Source references: `internal/source/source.go`, `internal/core/session.go`, `internal/cli/load.go`, `internal/cli/cache.go`, `internal/gitlog/gitlog.go`, `internal/archive/archive.go`, `docs/guide.md`, `SECURITY.md`. Existing fixture roots include `testdata/home/.claude` and `testdata/codex`. This ADR records a source review, not a fresh execution of tests.
