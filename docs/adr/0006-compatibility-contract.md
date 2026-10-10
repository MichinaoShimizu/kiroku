# ADR 0006: Compatibility contract for the 1.0 boundary

Status: **Observed (retrospective)**

Scope: Phase 1.

## Context

During 0.x, CLI commands, output fields and archive files changed frequently. A stable public contract was needed before 1.0.

## Documented decisions

- Treat 1.0 as a compatibility boundary, with future breaking changes requiring a new major version ([#209](https://github.com/MichinaoShimizu/kiroku/issues/209)).
- Version the public `kiroku json` schema using `schemaVersion: 1`; do **not** treat the internal `/data.json` endpoint as the same versioned public API ([#208](https://github.com/MichinaoShimizu/kiroku/pull/208), [#209](https://github.com/MichinaoShimizu/kiroku/issues/209)).
- Remove legacy `--serve`, `--json`, `-o` forms and rename `--root` to `--claude-root` while still in 0.x ([#208](https://github.com/MichinaoShimizu/kiroku/pull/208)).
- Protect readability of archive copies and the serve key across later versions with compatibility fixtures ([#211](https://github.com/MichinaoShimizu/kiroku/pull/211)).

## Compatibility boundary

The policy in [#208](https://github.com/MichinaoShimizu/kiroku/pull/208) protects, within a major version: CLI commands/options/defaults and environment variables; exit status; the enumerated public `kiroku json` fields; the `--prices` input format; archive copies; the serve key; local-by-default serving; and the shape of verified release artifacts used by installers/updaters.

The policy explicitly permits changes in any release to the UI, metrics and calculated values, estimated costs, readable prose, generated HTML, history formats supported and JSON fields outside the enumerated contract. Security fixes may override compatibility.

## Rationale and consequences

Users and scripts need stable semantics, but stabilizing too early would preserve accidental CLI shapes. Distinguish public JSON contracts from internal browser transport and from on-disk user data.

## Historical confidence

The decisions are explicit in issue #209 and related PRs. This ADR documents the chosen **policy**; it does not assert that v1.0.0 has already shipped. Consult `docs/compatibility.md` for normative details.
