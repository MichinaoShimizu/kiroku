# ADR 0008: Incremental local history processing

Status: **Active**
Basis: retrospective
Accepted by: none recorded

Scope: Phase 1.

## Context

Reparsing all agent histories on every live refresh becomes expensive as histories grow, particularly when agents append frequently.

## Documented decisions

- Start with a lightweight file-name/size/mtime fingerprint to detect changes while serving a local view ([#11](https://github.com/MichinaoShimizu/kiroku/pull/11)).
- Debounce reloads during active writes while enforcing a maximum delay to avoid indefinitely stale views ([#23](https://github.com/MichinaoShimizu/kiroku/pull/23)).
- Load only changed histories and summarize sessions relevant to a period rather than rebuilding everything ([#69](https://github.com/MichinaoShimizu/kiroku/pull/69)).
- Improve Codex/Git incremental behavior ([#129](https://github.com/MichinaoShimizu/kiroku/pull/129)), and later recalculate only affected weeks/months ([#258](https://github.com/MichinaoShimizu/kiroku/pull/258)).
- Benchmark year-scale history and avoid unnecessary processing/marshaling ([#257](https://github.com/MichinaoShimizu/kiroku/pull/257)).

## Rationale and consequences

Optimize source-unit reading and affected time ranges without changing observable counts. Caches must invalidate when source identity, parsing rules or attribution changes; correctness fixtures take precedence over cache hits.

## Historical confidence

The incremental direction is evidenced by the PR sequence. This ADR does not claim a particular cache algorithm was formally selected in advance. Inspect `internal/cli/cache.go`, `internal/source/watch.go`, and report tests before changing the implementation.
