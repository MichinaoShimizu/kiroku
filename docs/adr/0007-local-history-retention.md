# ADR 0007: Opt-in preservation of local agent history

Status: **Retrospective**

Scope: Phase 1.

## Context

Agent tools may prune their own histories, so a viewer that only rereads the original source cannot promise indefinite recall.

## Documented decisions

- Introduce `kiroku archive` to retain local copies of agent history that agents may delete ([#75](https://github.com/MichinaoShimizu/kiroku/pull/75)).
- Preserve linked history directories and support reading previously copied subagent history after agent-side cleanup ([#86](https://github.com/MichinaoShimizu/kiroku/pull/86)).
- Inform users about upstream history retention and how to preserve it, rather than implying kiroku controls the agent's cleanup ([#51](https://github.com/MichinaoShimizu/kiroku/pull/51), [#287](https://github.com/MichinaoShimizu/kiroku/pull/287)).
- Test archive failure cases, file modes and compatibility of archived data ([#256](https://github.com/MichinaoShimizu/kiroku/pull/256), [#211](https://github.com/MichinaoShimizu/kiroku/pull/211)).

## Rationale and consequences

Preservation is separate from visualization. Archive behavior must remain explicit and must not silently convert temporary/private agent history into ordinary exportable content; privacy rules in [ADR 0003](0003-local-privacy-boundary.md) remain applicable.

## Alternatives and confidence

These choices are documented in PRs. This ADR does not claim a comprehensive historical evaluation of backups or a cloud retention service. Review current `internal/archive` and source-specific handling before changing retention behavior.
