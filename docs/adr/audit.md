# Reverse ADR audit — 2026-10-10

Status: **In progress**. Baseline: PR #331 branch `docs/phase1-decision-recovery` (moving head; pin to a commit for reproducible future runs).

## Scope and coverage

- **20 ADR files inspected**: 19 retrospective Phase 1 records and 1 proposed Phase 2 record.
- All 20 ADR headings matched their filenames; no reference to the retired `0014-remote-session-import.md` was found **inside the 20 ADR files**.
- Historical PR #1, #54, #93, #208 and #285 descriptions were re-read, along with current `internal/report/share.go`, `internal/report/share_test.go`, `internal/web/web.go`, `internal/web/web_test.go` and `docs/compatibility.md`.
- The earlier scan of 318 merged PR titles/bodies was **candidate discovery**, not a full diff/review audit. Only five PRs were re-read in this pass; all historical diffs, reviews and current code paths were **not** exhaustively inspected.
- No test suite was executed in this pass. A test definition is not a passing test result. **No claim is marked verified.**

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

## Follow-up — 2026-10-10 (pinned to `ad670cb`)

The evidence baseline is now pinned to commit `ad670cb` on `main` (PR #331's branch has merged). The original findings above are kept as recorded.

**Precision improvements:** 1–3 had already been applied to ADR 0019, 0016 and 0001 before this follow-up. Item 4 is now done: ADR 0006 has a Verification section that names `docs/compatibility.md`, `TestJSONSchema`, `TestCompatArchive` and `TestCompatServeKey`.

**Tests executed** (Linux only; Windows and macOS not run):

```
go test ./internal/web ./internal/report ./internal/cli -count=1 \
  -run 'TestAssemble$|TestCSP$|TestShares$|TestJSONSchema$|TestCompatArchive$|TestCompatServeKey$'
```

All passed. Claim statuses in [evidence.yaml](evidence.yaml): **verified 4** (ADR-0006-C1, ADR-0006-C2, ADR-0016-C1, ADR-0019-C1), **historical-only 2** (ADR-0001-C1, ADR-0019-C2), **proposed 1** (ADR-0020-C1). Limits are recorded in each claim's notes: `/data.json` exclusion is documentary only, and `TestShares` does not assert the same-key case.

**Routing index:** ADR 0002 added to the source-adapter row. New rows route `kiroku update` (`internal/cli/update.go`) to 0011/0003, and snapshot/fixture tests (`internal/cli/snapshot_test.go`, `testdata/`) to 0017.

**Workflow integration:** `pre-pr` now runs an ADR Guard check. adr-guard's `record` mode delegates to forward-adr, so only one forward ADR template is maintained.

**Still open:** claim-level audit of the remaining ADRs; ADR 0019-C2 aggregation; how a retrospective (Observed) ADR becomes Accepted, if ever. That last one is a governance question for the owner, not something an agent should decide.

## Reverse ADR follow-up: early architecture and identity

- Reviewed PR descriptions and sampled diffs for #1, #6, #7 and #9; inspected current `internal/cli/load.go` and `internal/core/session.go` at baseline `ad670cb`.
- PR #6 explicitly documents the Go single-binary distribution rationale and adapter structure. The current collection code shows `seen` keys, `Claim`/`Yield` precedence and family-scoped withholding, without proving every historical motivation.
- Preserved historical validation limits: #1 did not test real Kiro histories, #7 had not verified the Kiro CLI Windows location, and #9 had not tested real Codex histories at introduction. These are not present-day failure claims.
- Three additional evidence claims: ADR-0001-C2, ADR-0004-C2 and ADR-0010-C2. Evidence matrix now has 10 claims: 4 previously test-verified, 3 historical-only, 2 untested, 1 proposed. No new tests were executed in this follow-up; previously recorded test results were preserved.
- Review threads, full historical diffs, and most current adapter code remain unaudited.
