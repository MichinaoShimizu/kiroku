# Reverse ADR audit — 2026-10-10

Status: **In progress**. Latest focused evidence pass uses main commit `ad670cb40d8617b5418e103feab18c3df29fc92e` as the current-code baseline. Historical claims are still partial; no fresh tests were executed.

## Scope and coverage

- **20 ADR files inspected**: 19 retrospective Phase 1 records and 1 proposed Phase 2 record.
- All 20 ADR headings matched their filenames; no reference to the retired `0014-remote-session-import.md` was found **inside the 20 ADR files**.
- Historical PR #1, #54, #93, #208 and #285 descriptions were re-read, along with current `internal/report/share.go`, `internal/report/share_test.go`, `internal/web/web.go`, `internal/web/web_test.go` and `docs/compatibility.md`.
- The earlier scan of 318 merged PR titles/bodies was **candidate discovery**, not a full diff/review audit. Only five PRs were re-read in this pass; all historical diffs, reviews and current code paths were **not** exhaustively inspected.
- No test suite was executed in this pass. A test definition is not a passing test result. **No claim is marked verified.**

## Focused follow-up: initial architecture and source identity

- Re-read merged PRs [#1](https://github.com/MichinaoShimizu/kiroku/pull/1), [#6](https://github.com/MichinaoShimizu/kiroku/pull/6), [#7](https://github.com/MichinaoShimizu/kiroku/pull/7), and [#9](https://github.com/MichinaoShimizu/kiroku/pull/9), including their descriptions and available unified diffs (diffs were sampled, not exhaustively reviewed).
- Inspected current `internal/cli/load.go` and `internal/core/session.go` on the pinned main baseline; no test execution or complete review-thread inspection.
- **Confirmed documented rationale:** #6 explicitly chose Go to ship a single cross-platform executable and separate agent-specific readers. This is stronger than an inferred motivation from current code.
- **Observed implementation:** `collectCached` uses `seen` keys, `Claim`/`Yield` precedence and family-scoped withholding for private Kiro Crew copies. The current code does not by itself prove every historical reason behind these mechanisms.
- **Historical limitations worth preserving:** #1 explicitly says real Kiro histories were not tested at introduction; #7 says Kiro CLI Windows path was not verified then; #9 says real Codex histories were not tested then. These are historical validation gaps, not claims that current behavior is broken.
- Added three claim-level evidence entries (ADR-0001-C2, ADR-0004-C2, ADR-0010-C2). The matrix now contains 9 sampled claims, **0 newly test-verified**. It remains far from complete coverage of 20 ADRs.

## Findings

| Severity | ADR | Claim and evidence | Status | Next action |
|---|---|---|---|---|
| Medium | 0001 | Current local adapter, dedup, caching and Git enrichment paths are cited, but the ADR contains **no linked historical PR** supporting why the architecture was selected. This is accurately labelled observed, not documented. | untested / historical rationale unknown | Trace #1, #6, #7, #9 diffs and current tests; keep unknown motivations unknown |
| Low | 0006 | #208 explicitly documents `schemaVersion: 1`, the public `kiroku json` fields, and exclusion of `/data.json` from the public contract. Current `docs/compatibility.md` agrees. | untested | Inspect `TestJSONSchema` assertions and execute it at pinned revision |
| Low | 0016 | #1 confirms standalone HTML from the original Python script; #93 documents source splitting with byte-identical output **at migration time**. Current `internal/web/web.go` uses `go:embed` for template, CSS and JS modules; `TestAssemble` and `TestCSP` exist. | untested | Run Go tests and verify actual generated HTML is standalone; do not generalize byte identity to current versions |
| Low | 0019 | #54 states overlapping minutes are apportioned; current `shares()` splits each minute equally across **distinct grouping keys** (not necessarily across sessions), as asserted by `TestShares`. #285 describes the separate Active time vs Total AI run time distinction. | untested | Run `TestShares`; inspect the active/AI-run aggregation and exact UI definitions |
| Informational | 0020 | Explicitly Proposed Phase 2, with validation gates unchecked. | proposed | Keep separate from Phase 1 and do not treat as implemented |

## Important precision improvements

1. **ADR 0019** should state that the share calculation divides each overlapping minute among **distinct grouping keys**; sessions belonging to the same key do not each take another share. This is stronger and more precise than 'apportioned'.
2. **ADR 0016** should distinguish the historical byte-for-byte equivalence at PR #93 from current single-file rendering, and should cite the actual `TestAssemble` and `TestCSP` tests.
3. **ADR 0001** should cite historical PR #1 and the Go migration #6 but continue to distinguish observed design from undocumented motivation.
4. **ADR 0006** should add explicit current-doc and test references rather than relying on a past PR alone.

## Remaining work and completion gates

- Expand each ADR into **claim-level** evidence with stable PR/diff references, current file and line, test assertion and execution result.
- Review high-risk decisions first: privacy/serve exposure, deduplication, missing-vs-zero measurement, JSON compatibility, archive retention and remote import.
- Check all 20 ADR cross-links, index and ledger programmatically; the initial heading check is not a complete link audit.
- Run targeted and full tests on a pinned commit, then record the exact commit, command, CI conclusion and any failures.
- Track the count of claims in each status before asserting evidence completeness.

This audit is intentionally conservative: source inspection and historical PR descriptions support a claim but do **not** establish a successful test run.
