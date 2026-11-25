# Quality audit log

Each run of `/quality-audit` (`.claude/skills/quality-audit/SKILL.md`) adds a section at the top. It records what was checked, what was fixed and what was left, so the next run starts from the open items rather than from scratch. Only dummy data is used; never write real history, paths or user names here.

Template for a new section:

```markdown
## YYYY-MM-DD — <full | quick | aspect> at <commit> (<version>)

### Gates
| Check | Result |
|---|---|
| tools/check.sh | pass / fail (what) |
| e2e and XSS | |
| go test -count=3 (order and time) | |
| main CI, last 20 runs | n failures (causes) |
| govulncheck | |
| Reproducible build of <version> | |
| Coverage by package | <pkg> n% (±x) … |
| Benchmark (read / aggregate / render) | n ms (±x%) … |
| Stale PRs, issues, Dependabot | |

### Findings
| Aspect | Certain | Suspected | Needs a decision | Not worth fixing | Fixed in |
|---|---|---|---|---|---|

### Open items
- <what> — why it stays open; how to check next time; due date or who decides

### Loop
| Run | Findings (certain) | Fixed | Median days to fix | Open items | Overdue | Previous items resolved |
|---|---|---|---|---|---|---|

### Next time
- <aspects to weigh more>
```

## 2026-11-25 — full, check only, at dfd6832 (v5.2.2 + 3 commits)

A full run with `check` (find and record, fix nothing) after v5.1.1 → v5.2.2 and #578–#580 (12 commits: Summary and calendar redesign, Input bars stacked by agent, Cost and output over time, Fable and Mythos in light-work, Codex usage-limit reset, faster Git reloads in serve, update through `gh` with provenance). A ran as `hostile-tester`, B as `user-tester`, C, D, E, G and H/I as general-purpose agents; J myself. F was not run: since 2026-11-22 the prompts changed only in the light-work line (Fable and Mythos, consistent across HELP, guide and both prompts per C) and the session Markdown header. Each finding below that is marked certain was confirmed by reading the code or rerunning the agent's reproduction; nothing was fixed, so every certain finding is also an open item.

### Gates
| Check | Result |
|---|---|
| tools/check.sh | pass |
| e2e and XSS | pass |
| go test -count=3 (order and time) | pass (G also: `-shuffle=on -count=2` under `TZ=Pacific/Kiritimati`, `-race` under `TZ=America/Los_Angeles`, `-cpu 1,4`; `TestCrewLogManyIDsIsLinear` 340 runs, `TestPromptTimesSameTimestampScales` 40 runs: pass) |
| main CI, last 20 runs | 0 failures (CI, CodeQL, Fuzz, Scorecard, Demo, Tag) |
| govulncheck | clean; OSV querybatch over all 26 modules and stdlib 1.26.11: 0 advisories (the GHSA REST endpoint and Dependabot alerts returned 403 from this session) |
| Reproducible build of v5.2.2 | matches (linux/amd64) |
| Coverage by package | archive 80.6% (0), cli 88.3% (−0.2), core 84.7% (−0.6), gitlog 93.4% (+0.5), report 89.3% (−0.1), source 95.1% (0), web 91.9% (0); tools/upstream 33.9% |
| Benchmark, BenchmarkLarge (n=3) | memory and allocations unchanged (read 903 MiB, 10.41M allocs). Time not comparable: taken at load 6.8 on 4 CPUs while four review agents ran (read 2061 ms, aggregate 439 ms, render 240 ms) |
| Stale PRs, issues, Dependabot | none open; no `upstream` or `prices` issue |

### Findings
| Aspect | Certain | Suspected | Needs a decision | Not worth fixing | Fixed in |
|---|---|---|---|---|---|
| A. security | 5 | 2 | 0 | 0 | — (check) |
| B. ux | 18 | 0 | 6 | 0 | — |
| C. i18n | 11 | 0 | 5 | 0 | — |
| D. docs | 10 | 1 | 0 | 0 | — |
| E. data | 2 | 4 | 0 | 1 (known approximations) | — |
| G. code | 5 | 1 | 0 | 0 | — |
| H/I. arch, supply | 5 | 0 | 3 | 0 | — |
| J. perf | 0 | 0 | 0 | 0 | — |

Duplicates merged: `install.sh` ignoring its arguments (A, D, I), `calendar.js` `Math.max(...DATA…)` (A, G), required checks (D, I), `update --help` missing from `kiroku help` (C, D), the Usage limit hits definition (C, D).

### Open items
Resolved: `serve.go` `%v` errors carrying paths (A: line 292 uses `termMsg`; the others carry no paths); Kiro credits week-over-week when history starts mid-week (B: now an absolute change with its base). Partly: gh update tests now run on macOS too (`TestGHRunUpdate` still linux/amd64 only).

New, certain (fix first, in this order):
- D-1 (high, blocks the next release): `tools/screenshots/capture.mjs:45` `#review .iskw` matches two elements since #578 ("From input to output", "Cost and output over time"), so Playwright's strict locator fails and `run.sh` stops after overwriting four PNGs; `tools/release.sh` step 4 and `/docs-refresh` stop with it. Also `capture.mjs:12` still says Color by defaults to agent. Then retake the screenshots (D-2: #578–#580 changed the layout and the dummy week; `demo.gif` predates #561–#574). Due: before v5.3.0
- G1 (medium): `internal/gitlog/gitlog.go:268-281` caches `repoOf`'s empty result after a `rev-parse` timeout or error for `TopTTL` (10 min); during that time `kiroku serve` drops the repo's commits without reporting it stale. Cache only a positive "not a repository". Due: next fix run
- B N1 (medium): "Expensive models for light work" gets no ⚠ mark: `review.js:239` maps `modelfit` to `models`, and nothing renders `data-help="models"` any more, so `placeFlags` places nothing. Add a test that every flagged item gets a mark. Due: next fix run
- A-3: `calendar.js:275` (and `:205`) `Math.max(...DATA.map(s => s.end))` throws `RangeError` at about 200,000 sessions (week view confirmed); use `reduce`. Due: next fix run
- A-4: `internal/archive/archive.go` `Sync` takes size and time from `d.Info()` (Lstat), so the archive copy of a symlinked `.jsonl` is cut to the link's length and never refreshed. Use the target's `Stat`. Due: next fix run
- A-1, A-2, I4: `install.sh` ignores its arguments (`--help` installs), writes gh's attestation errors to the terminal with control characters, and accepts loose version names (`v[0-9]*`). Due: next fix run (one install.sh PR with tests in `install_script_test.go`)
- I3: `tools/verify-release.sh:245` treats "timeout" etc. as "could not reach GitHub" before checking mismatch markers (the ordering v5.2.0 fixed in install.sh and `update_gh.go`), and attests neither SBOMs nor `checksums.txt`. Due: next supply run
- A-5: Windows quoting tests for `!`, backtick and `"` (behaviour is correct today). Due: next security run
- D-3: guide.md:226 and `changes/summary-time-charts.added.md` say "longest stretch"; `insCum` shades the stretch with the largest cost rise. Fix the text before v5.3.0's notes. D-4: sources.md:251 lists the Codex reset time under "Not read". D-5: Claude Code reset times are shown in 24-hour time, not "as written". D-6/C F13: guide Definition column drift (limits, tokensPerCommit, longctx, gitCommits, pace). D-7: stacking needs two or more agents. D-9/C F14: `kiroku update --help` (gh path) and the `kiroku help` footer. D-10/C F16: README wording and the new card. D-13/H3: development.md layout (gitlog cache, report `cache.go`/`span.go`, `tools/upstream`, `open.go`), compatibility.md without `KIROKU_AI`. Due: before v5.3.0
- C: F1 three unreachable HELP entries (findings, projects, outputs) with stale text; F4 "credits 8.4 credits"; F6 English month names in Japanese sparklines (`review.js:197`); F7 "push" in three Japanese strings; F8 mixed-language aria-labels and ASCII separators (new stacked bars, cost-over-time tooltip); F9 "Output" for both the panel and output tokens; F10 残したもの; F15 dead `localizeTitles` entries. Due: next i18n run
- B: N3 "Different this week" on a past week; N5 "kiroku ran and sent nothing"; N6 a link-coloured line that is not a link; N7 axis text in the new chart scales with width (25 px at 1000); N8 phone Sankey caption; N10 limit mark covers a label; N11 screen-reader trend reads the estimate as the value; N12 small ones (tab order in details, "$0.00" for non-AI commits, chip-row hint); and the carried L2–L7, "Close to usual" 10% rule, the "Sessions that reached a commit or PR" label, the KIRO_HOME-only path. Due: next ux run
- G: G2 kiroku 4 autostart leftovers can't be removed when an environment value has a control character (`autostart.go:200`; split out the paths and delete the content code); G4 no unit tests for `commitKind` and the new insight charts; G5 `live.js:59` says "Not connected" on a 500; G6 untested new branches (`update_gh.go` unknown error and missing gh, Kiro renamed copy's resume, gitlog non-absolute paths, the stale line). Carried: `codexNewer`, origin-index eviction, subagent `totalDurationMs`, `Main`/`cleanupOldExe`/`keepFn`/`splitList`/port-busy, Windows rollback's dropped error, per-OS default path table, unset `HOME` giving relative folders (archive under the current folder), same-second commits sorted unstably (`gitlog.go:340`), live reload errors only logged, e2e fixed waits, gen.py `uuid4`. Due: next code run
- H2: `open.go` holds only `probeAddr`; `WriteHTML`/`cmdHTML`/`scope.go` serve only tools/html. I6: tool versions outside Dependabot. Due: next supply run

New, suspected:
- E1 (high if real): Kiro Crew 0.14.0 runs Kiro Agent on KAS by default. KAS writes a V3 transcript (`sess_<uuid>`) that kiroku reads, and Crew's `kas` usage rows are counted as a separate Kiro Crew session (`crewElsewhere` returns false for `kas`), so credits and turns would count twice; and `kiroClassicYield` on a `sess_` sid gives `kiro-ide:sess_sess_<uuid>`, so a private KAS conversation would not be hidden. Confirmed in code; the rows' `provider` value is inferred from kirocrew `provider_labels.py`. Needs a KAS fixture. Due: next data run (first)
- E3: Claude Code-backed Crew subagents add turns and time next to the Claude Code transcript. E4: Codex spend-cap hits record no reset time (`individual_limit.resets_at`). Due: next data run
- A-6: `crewFilePrivate` fails open on read errors (couldn't test as root). A-7: `ghVerifyAttestation` reports "not logged in" (gh exit 4) as a mismatch (fails safe). G3: `TestCrewLogManyIDsIsLinear` has no floor for the small case. Due: next security / code run
- D-8: the "no card wider than half the page" rule between 1000 and 1299 px. Due: with D-3

Needs a decision (maintainer):
- B: N2 (was L1) Worth a look reachable only through unlabeled ⚠ icons, the summary-level one a `<span title>` with no accessible name; M4 mid-week off-hours share vs full weeks; M1 Sankey vs Input card period rule; N4 "cost per active hour" divides by session time (three hour totals on one screen); N9 partial first week "—" in charts but a value in trends; L8 first screen
- C: F2 the pace figure's four names, and "pace" also meaning the projection (plus `pace.d` still mentions the calendar); F3 推定 vs 予測; F5 the archive feature's name; F11 canonical Japanese terms; F12 "Cost" in headings against the guide's rule
- H1: the v5.0.0 exception is recorded only in PR #557 (`KIROKU_MAJOR_OK=1` used for a maintainer request, not a security fix; the Deprecated step was skipped). Record it and decide whether CI runs `major-gap.sh`
- I5: required checks are `test` ×3, `release-dry-run` and `e2e`; `install-script` and CodeQL are not; `main-2` lets the repository admin bypass always. List them in development.md and decide on `install-script`/CodeQL
- I7: `release.yml` builds and attests any `v*` tag, and the `secure-tags` ruleset doesn't restrict creation, so a tag on an unmerged commit gets accepted provenance. Restrict tag creation and/or require the tag to be on main

Carried, still open: E2 Crew rows' `acp_session_id` unused; E5 Claude Code resumed `cost-state`; GPT-5.6 Sol promotion ends 2026-11-30 (rerun `/price-update`, due 2026-12-01); Kiro CLI 3.6 (`/compact <instructions>`, conversation recaps, hook output files) and Claude Code 2.1.332 not yet in docs/upstream/agents.md, and tools/upstream doesn't track Kiro Crew (due: next data run); E7 known approximations (Codex 12-hour cache writes, Ultrafast list price, Haiku 3.5 and Claude 3.x unpriced, `KIRO_DATA_DIR`, V3 copies; decides: maintainer); the reports grader's rule for a warning that names a variable, "Next actions" without a figure, Synthetic session titles and the hop page in English only, the Go-side native metric labels, `?key=` in browser histories, SECURITY.md line lengths (decides: maintainer).

### Fixes (same day, on the same PR)
After the check, the maintainer asked to fix everything ("全部直して") and decided the open questions: Worth a look stays as it is (N2); the recommended names (one name "A day to match last week/month" with "pace" only for the projection, 推定 throughout, "Keep a copy" / コピーを残す); M4 (compare with the same days of past periods), N4 ("Estimated cost per session hour"), N9 (partial first period left out with a visible reason), L8 (a tagline under the logo) and an M1 note on the Sankey; H1, I5 and I7 recorded in the docs only (compatibility.md, development.md, Decision Log 16; no CI change for the tag or the major gap). Four agents fixed the areas in parallel worktrees, merged here.

Fixed: D-1 and the retaken screenshots (D-2); G1, G2, G4–G6, same-second commits, unset `HOME` for writes, live reload errors shown in the view, the Windows rollback error, `open.go`, deterministic gen.py, condition waits in e2e where safe; A-1–A-7, I3, I4; E1 (KAS: the Crew session yields to the V3 transcript; private KAS conversations hidden; rows record `provider: "acp"` with `harness: kas`, not `provider: kas` as the audit inferred), E2, E3, E4, G3; Kiro Crew added to tools/upstream; N1, N3, N5–N12 (except the phone chip-row hint), L2–L7, the "Close to usual" rule, the card label, both Kiro homes in `serve --check`; C F1, F4, F6–F10, F13 (and `TestHelpMatchesGuide` now checks the numbers in Definition), F15, F16; D-3–D-10, D-13, H2, H3; the maintainer's naming and Summary decisions.

`/pre-pr` on the fixes: `user-tester` found the Japanese waterfall labels overlapping (shortened by character count), the range decimals still uneven where `insBase` formats, the partial-week reason only on hover, "Left behind" in English, two dashed lines with one legend key, a backup and 予測 left in the guide and comments, 「過去のか月が 4」, a lone "?" on a wrapped label, four Japanese names for session time, overlapping end totals at 390 px and "Month-end credits" without "estimate"; all fixed with tests. `hostile-tester` found no XSS, path or command issue; it found that the archive copies any regular file linked under a history file's name (now stated in SECURITY.md), the install.sh error losing its colour and `--help extra` accepted (fixed), C1 controls not removed by `clean` (fixed), and a timeout line claiming a kept result (fixed).

Still open after the fixes:
- E: whether KAS `messages.jsonl` has V3's `usage_summary` lines (inferred from Crew's reader); Kiro CLI 3.6 `/compact <instructions>` and recaps (no sample); `acp_session_id` of subagents sharing the parent's runtime; Kiro Crew 0.14.0 only tagged as rc in git (agents.md says 0.13.1 until the weekly run). Due: next data run
- A: with `HOME` unset, the default history folders (`.claude/projects`, `.codex`, `.kiro`) still resolve against the current folder for reading (escaped, nothing written); readers don't treat an empty root alike, so it needs its own change. A size cap or format check for archived links (decides: maintainer). Due: next security run
- B: the phone chip-row scroll hint; the Data sources panel closes after "Keep a copy in this folder" (the toast confirms). Due: next ux run
- C F11 (canonical Japanese terms beyond session time) and F12 ("Cost" in headings). Decides: maintainer
- G: most `pause()` waits in smoke.mjs stay (no single condition); `claudeManagedDir` has no per-OS table test; `core.AgentEnv` is now used only by its test. Due: next code run
- I6 (tool versions outside Dependabot) and I7's ruleset part (restrict who creates `v*` tags: a Settings change for the maintainer)
- Carried unchanged: E5, E7, the GPT-5.6 Sol price (due 2026-12-01), the reports grader items, the Go-side native metric labels, `?key=` in browser histories, SECURITY.md line lengths

### Loop
| Run | Findings (certain) | Fixed | Median days to fix | Open items | Overdue | Previous items resolved |
|---|---|---|---|---|---|---|
| 2026-11-25 full (check, then fixed) | 56 (+14 in /pre-pr) | 54 (+14) | 0 (same day; PR open) | 9 groups | 0 (Sol price due 12-01) | 2 of 14 (+1 partly) |
| 2026-11-24 full | 62 (+9 in /pre-pr) | 28 (+9) | 0 (same day; PR open) | 14 groups | 0 | 4 of 13 |
| 2026-11-20 ux, i18n | 25 | 25 | 0 (same day; PR open) | 2 new (and 26 not rechecked) | 1 (F, from 2026-11-19) | — (aspect run) |
| 2026-11-19 full | 60 | 34 | 0 (same day; PR open) | 26 | 1 (F) | 9 of 24 |
| 2026-10-18 full | 64 | 36 | 0 (fixed the same day; PRs open) | 24 | 0 | 8 of 17 |

### Next time
- A fix run first: D-1 (release blocker) and D-3 before v5.3.0, then G1, N1, A-3, A-4, the install.sh PR and E1 (with a KAS fixture)
- ux and i18n again had the most findings (B 18, C 11), mostly from the Summary redesign; settle the decisions above with the maintainer before fixing the wording
- data on 2026-12-01 (price) and for Kiro CLI 3.6 and Kiro Crew 0.14 (KAS)
- J: rerun the benchmark on an idle machine

## 2026-11-24 — full at 4046bd8 (v5.1.1)

A full run after v4.6.1 → v5.1.1 (20 commits: kiroku 5's serve/update/help, the Summary IN/OUT panels and Sankey, the prompt flow colors, updates through `gh` from an organization's repository, standalone binaries, Kiro duplicate sessions). A ran as `hostile-tester`, B as `user-tester`, C, D, E, G and H/I (together) as general-purpose agents; J myself. F was not run: the daily report and session review prompts have not changed since the prompts run of 2026-11-22 (only the repository URL helper and a search sentence). The fixes are on one PR from `claude/optimistic-ptolemy-6wsgtn`; the maintainer chose to take the provenance check of `kiroku update` and the Kiro Crew `child_sid` fix into it rather than separate PRs.

### Gates
| Check | Result |
|---|---|
| tools/check.sh | pass |
| e2e and XSS | pass |
| go test -count=3 (order and time) | pass (G also ran `-shuffle=on` under `TZ=Pacific/Kiritimati`, `-race` under `TZ=America/Los_Angeles`, and the timing tests 10× at `-cpu 1,4`: pass) |
| main CI, last 20 runs | 0 failures. Prices failed on 2026-11-09 and 11-16 and passed on 11-23 (not investigated) |
| govulncheck | clean; OSV (which includes GHSA) has no advisory for any module in go.mod |
| Reproducible build of v5.1.1 | matches (linux/amd64) |
| Coverage by package | archive 80.6% (0), cli 88.5% (+0.3 vs 2026-11-20), core 85.3% (+1.6), gitlog 92.9%, report 89.4% (−0.4), source 95.1% (+0.6), web 91.9% (+0.4); tools/upstream 33.9% |
| Benchmark, BenchmarkLarge (vs 332b4db, n=6) | memory and allocations unchanged (read 901 MiB, 10.41M allocs; render −4.5% B/op). Time on an idle machine: read ≈ 970 ms (+10% vs the 881 ms recorded 2026-11-20), aggregate ≈ 410 ms (−19%), render 255–335 ms (noisy). A first run at 1853 ms for read was taken while four review agents were running |
| Stale PRs, issues, Dependabot | none open |

### Findings
| Aspect | Certain | Suspected | Needs a decision | Not worth fixing | Fixed in |
|---|---|---|---|---|---|
| A. security | 3 | 2 | 0 | 0 | this PR (control characters from a history `cwd` in the git timeout line and two other messages; Copy as Markdown header escaped; the guide verifies a standalone binary before it replaces anything) |
| B. ux | 15 | 1 (week-over-week % for credits when Kiro history starts mid-week) | 4 (M1 Sankey total vs the Input card, M4 mid-week off-hours share, L1 Worth a look hard to find, L8 first screen) | 0 | this PR (M2 "Close to usual", M3 what Repeated prompts counts; M5 by decision: the guide follows the view's "credits") |
| C. i18n | 15 | 1 (unused-looking JA keys besides "outputs") | 3 (one name for the archive feature, 推定/予測, the pace figure's three names) | 0 | this PR (untranslated "Reached a commit or PR", `serve --archive --help`, duplicate JA key with a test) |
| D. docs | 8 | 0 | 0 | 0 | this PR (7: today marker, cumulative line only in Input, `--force` for dev builds, package table, `update_gh.go`, standalone binaries in releases); install.sh has no `--help` (open) |
| E. data | 3 | 3 (`/chat load` copies, Claude Code resumed `cost-state`, `/tangent merge` summary) | 0 | 0 | this PR (#560 regression: a V3 copy naming another `id` hid that conversation; Crew `child_sid` + `harness` for pruned Codex subagents; `/chat load` added to the known limit) |
| G. code | 11 | 7 | 0 | 0 | this PR (removed command in help, browser launch error reported, dead JS/CSS, stale `serve --html` comments) |
| H/I. arch, supply | 7 | 0 | 2 (H1, I5) | 0 | this PR (running-serve probe follows no redirects; `kiroku update` through `gh` checks build provenance; standalone binaries named in `subject-path`; development.md table) |
| J. perf | 0 | 0 | 0 | 0 | — |

A side note: while checking `install.sh`'s help, D ran `sh install.sh --help`, which installed v5.1.1 into the audit container (provenance verified; no repository change). That is finding D-8.

`/pre-pr` on this PR found more in the new code, all fixed on it with tests: `hostile-tester` showed that `gh`'s message quotes certificate values (a branch or workflow name such as `fix-timeout`), so a provenance mismatch could be read as "could not reach GitHub" and only warn — in the new `kiroku update` check and, since before, in `install.sh` (now: mismatch markers first; `install.sh` has a test that runs it with a fake `curl` and `gh`); two Kiro V3 copies naming the same foreign `id` kept one ID; reading a crew-log was quadratic in its session IDs; `mdCode` overflowed the stack on very many backticks. `user-tester` found the "?" of Repeated prompts promising a list the dialog doesn't show, "Close to usual" next to a range whose shown end equals the value (now "As usual" when they round the same), and the range wrapping mid-way at 390 px.

### Open items
Resolved: Kiro Crew `child_sid` for Codex subagents (this PR, from crew-log closers and usage `harness`; the time-based split of reused slots by `acp_session_id` is not used yet, see below); "Sessions that reached a commit or PR" names Claude Code in its "?" and the Git line (the card label itself is still open); the demo prompt's UTC offset (fixed earlier); the `% labels for two figures` (gone with the Summary redesign).

Carried and new:
- H1: v5.0.0 came 2 days after v4.0.0 and removed commands and options with no `### Deprecated` release first, against docs/compatibility.md. Record the exception (Decision Log or a note in compatibility.md) and decide whether CI (not only `tools/release.sh`) should enforce the gap and the deprecation. Decides: maintainer
- I5: required checks are not listed anywhere but this log (e2e required since 2026-10-14; `non_admins`, `install-script`, CodeQL not required). List them in development.md. Decides: maintainer
- B: M1 (Sankey counts sessions started in the period, the Input card counts cost incurred in it), M4 (mid-week off-hours share compared with full weeks), L1 (Worth a look only as ⚠ icons), L8 (first screen doesn't say what kiroku is), the "Sessions that reached a commit or PR" card label, the KIRO_HOME-only path in `serve` / `serve --check` output (`kiro.go` `Where()`), and the small ones (L2 range wording at the edges, L3 relative % on shares, L4 truncated waterfall labels, L5 mid-word breaks at 1000 px, L6 accessible names that don't start with the visible text, L7 unclear small labels). Decides: maintainer for M1, M4, L1, L8; the rest due: next ux run
- C: one name for the archive feature (kiroku archive / keep a copy / バックアップ / コピー), 推定 vs 予測, the pace figure's three names, unreachable HELP entries (findings, projects, outputs) and their stale text, 残したもの / 出力 left after 投入/産出, "output tokens" in English, the calendar tooltip's "credits 8.4 credits", mixed-language aria-labels, small Japanese term splits, README wording, guide Definition column drift, dead `localizeTitles` entries and `update --help` missing from `kiroku help`. Decides: maintainer for the names; the rest due: next i18n run
- B (new, low): the unit-cost cards don't say that "Close to usual" means within 10% of the range edge (ƒx could). Due: next ux run
- A (new, low): `Math.max(...DATA.map(…))` in calendar.js and insight.js spreads arrays as arguments; a history with very many sessions could overflow the stack like `mdCode` did. Due: next code run
- D-8: `install.sh` ignores its arguments, so `sh install.sh --help` installs. Due: next supply run
- E: Kiro Crew usage rows' `acp_session_id` could replace the time-based `crewOwnerAt` split for reused slots (and the model-ID test in `crewCodexRow`); Claude Code-backed Crew subagents still don't yield. Due: next data audit
- E: Claude Code resumed `cost-state` may now carry the earlier process's totals (CHANGELOG 2.1.277, 2.1.302), which kiroku would spread again; needs a real sample. Due: next data audit
- E: GPT-5.6 Sol's promotional price ends 2026-11-30: rerun `/price-update` after 2026-12-01. Due: 2026-12-01
- E: Kiro CLI `KIRO_DATA_DIR` undocumented; V3 tangent, `/rewind`, IDE fork and `/chat load` copies counted again (and `/tangent merge`); Claude Code fork subagents. Need real samples. Decides: maintainer (samples)
- E: Haiku 3.5 and Claude 3.x unpriced; Ultrafast list price during the promotion; Codex 12-hour cache writes priced at 30 minutes. Decides: maintainer
- G: untested branches (`codexNewer`, origin-index eviction, subagent end time and usage from `totalDurationMs` without a file, two-model subagent files, `Main`, `cleanupOldExe`, the Windows rollback whose second error is dropped, `cmdServe`'s `keepFn`/`splitList`/port-busy path); the gh update tests run only on Linux; per-OS default paths have no table test; v4 autostart content code kept alive by its test; e2e fixed waits (`pause()`, `view()`, the favicon wait); `TestPromptTimesSameTimestampScales` measures time; `gen.py` uses unseeded `uuid4`; unset `HOME` makes default folders relative; `crewFilePrivate` treats read errors as "not private"; same-second commits sorted unstably; live update errors only logged. Due: next full run (crewFilePrivate: next security run)
- A: `serve.go` `%v` errors that may carry paths (101, 191, 356); Windows shell tests for `!`, backtick and `"`. Due: next security run
- H: `internal/cli/open.go` holds only `probeAddr`; `WriteHTML`/`cmdHTML`/`scope.go` serve only `tools/html` but ship in `internal/cli`. I3: `verify-release.sh` doesn't verify provenance of SBOMs and `checksums.txt`. I4: `install.sh` accepts looser version names than `validTag`. I6: shellcheck, staticcheck, govulncheck, GoReleaser and syft versions aren't tracked by Dependabot. Due: next supply run
- Previous: the reports grader's rule for a warning that names a variable; "Next actions" without a figure; Synthetic session titles and the hop page in English only; the Go-side native metric labels, `?key=` in browser histories, SECURITY.md line lengths. Decides: maintainer

### Loop
| Run | Findings (certain) | Fixed | Median days to fix | Open items | Overdue | Previous items resolved |
|---|---|---|---|---|---|---|
| 2026-11-24 full | 62 (+9 in /pre-pr) | 28 (+9) | 0 (same day; PR open) | 14 groups | 0 | 4 of 13 |
| 2026-11-20 ux, i18n | 25 | 25 | 0 (same day; PR open) | 2 new (and 26 not rechecked) | 1 (F, from 2026-11-19) | — (aspect run) |
| 2026-11-19 full | 60 | 34 | 0 (same day; PR open) | 26 | 1 (F) | 9 of 24 |
| 2026-10-18 full | 64 | 36 | 0 (fixed the same day; PRs open) | 24 | 0 | 8 of 17 |

### Next time
- i18n and ux: the most findings this run (C 15, B 15), mostly naming after the Summary redesign; settle the names with the maintainer first
- data on 2026-12-01 (price) and when a real sample of Kiro V3 forks or Claude Code resume arrives
- code: the untested branches carried since 2026-11-19

## 2026-11-22 — open items, prompts and docs at 332b4db (v4.6.1)

An aspect run to bring the open items up to date after v1.0–v4.6.1 (four major versions in four days), with F (prompts, overdue since 2026-11-01) and docs. The run's changes are on one PR from `claude/busy-clarke-dlhndw`.

### Gates
| Check | Result |
|---|---|
| tools/check.sh | pass |
| Screenshots retaken at ci.yml's `KIROKU_FAKE_NOW` | 0 changed (current); `demo.gif` not retaken (no Playwright ffmpeg in this container) |
| `/report-eval`, 2 runs per report, before → after | day 2/2 → 2/2; session 0/2 → 2/2 (`pull/77` was missing: the prompt had no PR address); week 0/2 → 0/2 by the grader (one run now advises a lighter model, but also names the secret's variable and the injected text while warning about them; the other still skips the model signal). The weekly mode was then removed (see Resolved) |
| Clarity, blind grader (sum of aspects 1–5 and 7, mean of 2 runs, out of 30) | day 25.5 → 27; week 24.5 → 25.5; session 24 → 28.5 (aspect 6: 4 → 5). Day advice fell 4 → 3: "Next actions" held routine to-dos with no figure behind them |
| Prompt size (approx tokens) | day 2070 → 2099, week 3238 → 3265, session 1338 → 1348 |

### Findings
| Aspect | Certain | Suspected | Needs a decision | Not worth fixing | Fixed in |
|---|---|---|---|---|---|
| data | 2 | 0 | 0 | 0 | this PR (Claude Code compaction-only moved file; Kiro sessions left in `~/.kiro` when `KIRO_HOME` is set) |
| prompts (F) | 2 | 0 | 0 | 0 | this PR (PR address in the session review; actions with a clear signal first) |
| docs | 25 | 0 | 0 | 0 | this PR (README still advertised "Analyze with AI"; guide, usability.md and CONTRIBUTING.md behind v4.0–v4.6.1) |
| supply / process | 1 | 0 | 0 | 0 | this PR (major versions at least 30 days apart, after a deprecation: docs/compatibility.md, `tools/major-gap.sh` in `tools/release.sh`) |

### Open items
Closed because 4.0 removed "Analyze with AI" and `kiroku serve --ai`: real runs with Codex and Kiro CLI; the Kiro and Codex read-only mechanism and Claude's `--add-dir`; `ai.js` untested paths; `/ai/run` errors in Japanese; `--ai=false` through `cmdServe`; the report prompt suggesting commands the `--ai` run couldn't use; Windows cleanup of a stopped AI run.

Resolved: the weekly report mode of `reportPrompt`, unreachable from the view since only the daily button called it, was removed by the maintainer's decision (this PR; `/report-eval` and `tools/reportbench` now cover day and session only, and the e2e checks that used the weekly prompt use the daily one); the pasted `git show` running repository programs (v0.40.0, `gitPasteFlags`, with tests); the Claude Code compaction-only moved file (this PR); `KIRO_HOME` and V2 / Kiro IDE sessions in `~/.kiro` (this PR; whether V2 or Kiro IDE ever write to `KIRO_HOME` is still undocumented, and reading both places covers either); `docs/sources.md` on V3's resume list (no longer there).

Carried:
- Reports: a run that warns about a leaked key or an injection names them, which the grader counts as a violation (seen in the weekly runs before that mode was removed). Decide whether the grader should allow a warning that names the variable without its value. Due: next prompts run
- Daily report: "Next actions" can be routine to-dos ("get PR #77 reviewed") with no figure behind them; session reviews give elapsed time ("about 48 minutes") next to 31m active, and the playbook's first prompt keeps placeholders. Due: next prompts run
- Kiro CLI `KIRO_DATA_DIR` undocumented. Due: next data audit
- Kiro Crew `child_sid` link for Codex sessions started by a Crew subagent. Due: next data audit
- Kiro V3 tangent, `/rewind` and IDE fork sessions count the parent's prompts again. Needs a real sample. Decides: maintainer
- Claude Code fork subagents and resumed `cost-state`. Needs real samples. Due: next data audit
- The "Data sources" panel shows only the `KIRO_HOME` path, not `~/.kiro` read with it. Due: next ux run
- Haiku 3.5 unpriced after it left the price page; Ultrafast list prices during the promotion. Decides: maintainer
- UX lows, the Go-side native metric labels, `?key=` in browser histories, required checks (`non_admins`, `install-script`, CodeQL), SECURITY.md line lengths: unchanged. Decides: maintainer
- G: `codexNewer`, origin-index eviction and the subagent end time from `totalDurationMs` without a file are untested branches. Due: next full run
- Synthetic session titles and the hop page are English only. Decides: maintainer

### Next time
- A full run: the last one was at v0.39.0, and the view changed through four major versions since
- prompts: the grader's rule for warnings

## 2026-11-20 — security and ux before v3.0.0 (PR #506)

An aspect run on the v3.0.0 candidate (PR #506: leaner view and CLI, Kiro IDE (legacy) and Amazon Q dropped, `kiroku json` and `kiroku stats` removed). A ran as `hostile-tester` (hostile history across every agent, serve with and without `--ai`, git config traps, file modes, broken input); B as `user-tester` (all scenarios, en and ja, 1440, 1000 and 390, dark and light, serve with `--ai`, without it and with no agent, saved HTML).

### Gates
| Check | Result |
|---|---|
| tools/check.sh --e2e | pass |
| go test -count=3 | pass |
| Coverage by package | cli 88.2% (+1.6), core 83.6%, report 89.8%, source 94.5%, web 91.5%; others unchanged |

### Findings
| Aspect | Certain | Suspected | Needs a decision | Not worth fixing | Fixed in |
|---|---|---|---|---|---|
| A. security | 2 | 0 | 1 (symlinked history files are followed when reading and archiving) | 0 | #506 (`Origin` now required on `/ai/run`, `/archive` and the open-code exchange, with tests; stale `--lang` help) |
| B. ux | 9 | 0 | 3 (page opens scrolled past the overview; daily AI button hidden until hover; one % for cost and credits, and comparisons when history starts inside the previous period) | 1 (legend chips scroll at 1000 px) | #506 (limit marks on their own block, late git marks inside the day, "Long conversations" name, list closes after a pick, no-agent reason as text, no "0" commit mark, units kept with numbers) |

### Open items
- Symlinked history files: kept as is (links are followed so history moved to another disk keeps working) and documented in SECURITY.md (#506)
- UX: the opening scroll and the hover-only daily AI button stay as the maintainer asked earlier; % labels for two figures and comparisons when history starts inside the previous period stay open. Due: next ux run
- Small: Repeated prompts rows that read the same (resolved: shown as prompt bubbles); the phone right-edge fade over the search button (resolved: the header now stacks above the fade). Due: next ux run

## 2026-11-20 — ux and i18n at 5297b6a (v1.1.2 and PR #502)

An aspect run after many view changes since v1.0 (one page scroll, the figures strip, chat-style prompt flow, month-end ranges, the palette). B ran as `user-tester` (en and ja, 1440, 1000 and 390, dark and light, serve with a stub agent, no agent and `--no-ai`, and the saved HTML); C as a general-purpose agent. Fixes are on PR #502.

### Gates
| Check | Result |
|---|---|
| tools/check.sh | pass |
| e2e and XSS | pass (en and ja; after the fixes, one new smoke step and checks for one-row figures, the week label, the sticky day headings and no inner calendar scroll) |
| go test -count=3 (order and time) | pass |
| main CI, last 20 runs | not checked (aspect run) |
| govulncheck | clean (in tools/check.sh) |
| Reproducible build of v1.1.2 | matches (linux/amd64, at release) |
| Coverage by package | ai 93.2% (-0.8), archive 80.6% (+1.2), cli 86.6% (-0.6), core 83.7% (+0.2), web 91.5%; others unchanged |
| Benchmark, BenchmarkLarge (n=6 against 99b2fcd) | read 881 vs 974 ms (p=0.59), aggregate 505 vs 465 ms (p=0.49), render 236 vs 216 ms (p=0.07): no significant change (the machine was noisy, ±15–25%). Memory unchanged |
| Stale PRs, issues, Dependabot | not checked (aspect run) |

### Findings
| Aspect | Certain | Suspected | Needs a decision | Not worth fixing | Fixed in |
|---|---|---|---|---|---|
| B. ux | 10, and 9 small ones | 1 (Codex and Kiro CLI (V2) greens) | 3 (sticky day row on phones, splitting the credits overage, the Kiro CLI (V2) color) | 0 | #502 (pace and trend lines at the wrong height in week breakdowns and their label, LIVE badge readable by touch and keyboard, "this week" on past weeks, breakdown bars on one scale, "?" glyph contrast, credits under 10 rounded to integers, the search box's name) |
| C. i18n | 6 | 0 | 1 (サマリー or まとめ) | 1 (目安コスト left in code comments only) | #502 (unused key, 数値 → 指標, 第{n}週 spacing, ranges with 〜 in Japanese, 推定費用 per commit, AI run errors in Japanese) |

Decisions the maintainer made (fixed on #502):
- Japanese uses サマリー for the summary everywhere (週のサマリー／月のサマリー; README.ja follows)
- On phones, the day headings are a separate row that sticks under the page header and follows the calendar's sideways scroll; the calendar no longer scrolls vertically on its own (the leftover 16 px)
- Kiro CLI (V2), Kiro CLI (SQLite) and Amazon Q move to the teal slot; stacked bars keep their 2 px gaps
- When only one of cost and credits is past the last period's total, the pace figure shows the other's per-day target and a short "credits over" note; the overage is in its tooltip

### Open items
- Previous items of 2026-11-19 — not rechecked in this aspect run. Due: next full run
- New: overlapping session titles — resolved on #502 (a title stops above a session cascaded over it; lanes unchanged)
- New: the day "Analyze with AI" button covering the weekday — resolved on #502 (a small ✦ button on hover devices). The day tooltip still opens with it; low
- New: the demo HTML's prompt says the browser's UTC offset while its times use the fixed demo clock (`utcOff` in session.js). Demo only. Due: next ux run
- New (small): resolved on #502 — the search box on phones, the reply's "AI worked" note, AI dialog titles, Data sources and the footer in the calendar view. Still open: "Sessions that reached a commit or PR" doesn't say "Claude Code only" on the card; the prompt to check is English in Japanese (likely intentional). Due: next ux run
- Previous: F (prompts), `/report-eval check` at this commit (2 runs each, general-purpose writers, one grader):
  - Correctness: format and numbers copied in all 6; no secrets, injection or invented links; no file outside the allowed set. Recall: day 5/5, week 8/8, session 3/4
  - Still failing as on 2026-10-18: the week report misses the "short question on Opus → a lighter model" hint (2 of 2; the skill hint missed in 1 of 2); the session review never names the PR (2 of 2); one day report and one week report mention the abandoned bun trial, and one day report names the earlier week's `feat/watermark` branch
  - Grader (1–5): summary 4–5, outcomes first 5, specificity 3–5, concision 3–4, reader 4, review format 3–4, advice 3–5
  - Prompt size: day 2.1K, week 3.3K, session 1.4K tokens (unchanged); writer tokens 74–82K
  - Fixing them needs `/report-eval` with before/after runs. Due: before the next minor release (overdue since 2026-11-01). Decides: maintainer whether to spend the runs
- Previous: "Analyze with AI" with real agents — Codex CLI 0.171.0 (npm, help only, no model call) lists every flag kiroku passes (`exec`, `--sandbox read-only`, `--skip-git-repo-check`, `--ephemeral`, `--config`). Kiro CLI isn't installable here (not on npm); a real run of either still needs an account. Decides: maintainer

### Loop
| Run | Findings (certain) | Fixed | Median days to fix | Open items | Overdue | Previous items resolved |
|---|---|---|---|---|---|---|
| 2026-11-20 ux, i18n | 25 | 25 | 0 (same day; PR open) | 2 new (and 26 not rechecked) | 1 (F, from 2026-11-19) | — (aspect run) |
| 2026-11-19 full | 60 | 34 | 0 (same day; PR open) | 26 | 1 (F) | 9 of 24 |
| 2026-10-18 full | 64 | 36 | 0 (fixed the same day; PRs open) | 24 | 0 | 8 of 17 |

### Next time
- ux: the open overlap and hover items, and a run on a real phone width after the sticky day row
- The full run's open items (F prompts is overdue)

## 2026-11-19 — full at 99b2fcd (v0.39.0)

The third full run. A, B, C+D, E (one agent each for Claude Code, Codex and Kiro), and G+H ran as agents; I and J by hand. F (prompts) was not run (see Open items). Fixes are on one branch, #481, as separate commits.

### Gates
| Check | Result |
|---|---|
| tools/check.sh | fail: govulncheck reported GO-2026-6859 (net/http `MaxBytesReader` header race, reachable through `/ai/run`) in Go 1.26.10. Fixed with Go 1.26.11 (#481); pass after |
| e2e and XSS | pass (smoke, xss and serve.mjs; en and ja) |
| go test -count=3 (order and time) | pass; also `-shuffle=on`, `-race`, TZ Kiritimati and Los_Angeles, and `-cpu 1,4 -p 1` with 8 CPU burners. `TestLiveRefresh` and the Crew live test took their baseline inside the goroutine (open since 2026-10-18); fixed |
| main CI, last 20 runs | CI, CodeQL, Scorecard, Demo, Fuzz, Upstream green. Prices failed twice (2026-11-09, 11-16): OpenAI changed every price table's columns, split cache writes into 30m and 12h, and moved Cyber models into Specialized. `tools/prices` rewritten (#481) |
| govulncheck | GO-2026-6859 (above). Go modules checked on OSV for advisories govulncheck doesn't know yet: none for compress v1.20.3, sqlite v1.60.1, x/sys v0.49.0, libc, go-humanize |
| Reproducible build of v0.39.0 | matches (linux/amd64) |
| Coverage by package | ai 94.0%, archive 79.4%, cli 87.2% (-0.2), core 83.5%, gitlog 92.4%, report 90.1%, source 94.3%, web 91.5% (+0.9), tools/upstream 33.9% (+4.6); others unchanged |
| Benchmark, BenchmarkLarge (n=6 against 7eecd3f) | read 771 ms vs 701 ms (p=0.13), aggregate 362 vs 349 ms (p=0.39), render 172 vs 181 ms (p=0.24): no significant change. Render memory -7.6% (p=0.004, #467) |
| Stale PRs, issues, Dependabot | Issue #474 (upstream, 24 days; closed by #481). Dependabot #478 (actions, 12 days), #479 (Playwright), #480 (Go modules; its ubuntu job fails on govulncheck with Go 1.26.10, which #481 fixes) |

### Findings
| Aspect | Certain | Suspected | Needs a decision | Not worth fixing | Fixed in |
|---|---|---|---|---|---|
| A. security | 6 | 2 | 2 (copied `git show` runs the repository's config; Claude's `--add-dir` includes the Crew home) | 1 (symlinked session file shown but not served, as SECURITY.md says) | #481 (paste quoting for fish and PowerShell, SQLite named pipe, quadratic prompt times, `archive off`, archive folder modes, 390 px tags) |
| B. ux | 14 | 0 | 7 (summary blank, pace tile, dark disabled button, badge contrast, calendar overlaps, default "Color by", LIVE badge hint) | 1 (sparkline label) | #481 (metric button names, repeated-prompt wording, AI check wording) |
| C. i18n | 4 | 0 | 2 (Japanese words for cost and outputs; Go pages, `/ai/run` errors and synthetic titles in Japanese) | 0 | #481 |
| D. docs | 9 | 2 | 2 (split SECURITY.md lines 15–16; write the `--no-ai` rules once) | 0 | #481 |
| E. data | 12 | 11 | 3 (Kiro "Classic" name; Kiro tangent and fork double count; Ultrafast promotion) | 0 | #481 (prices, gateway budget, subagent order, Kiro `--v3` and docs, Codex spend limits and `dynamic_tools_updated`) |
| F. prompts | — | — | — | — | not run |
| G. code | 6 | 0 | 0 | 1 (`autostart off` ignores the unregister error: `launchctl bootout` fails when not running) | #481 (dead code, live test baseline, live.js errors and demo shift, test setup errors, zstd recover) |
| H. arch | 2 | 0 | 0 | 0 | #481 (dependency table, `app.go`) |
| I. supply | 3 | 0 | 0 | 0 | #481 (Go 1.26.11, Tag only from main, Prices parser) |
| J. perf | 0 | 0 | 0 | 0 | — |

Decisions the maintainer made after the run (fixed on #481):
- Copied git commands carry `--no-pager -c core.fsmonitor=false` (and `--no-ext-diff --no-textconv` for `show`); a test pastes one into sh against a repository that sets every configured command
- The Claude Code AI run gets a `Read` deny rule (in a `--settings` file) for each Kiro Crew private conversation and its copies
- Prices: keep Haiku 3.5 removed and Ultrafast list prices
- Kiro tangent, `/rewind` and fork double count: documented as a known limit in docs/sources.md
- Japanese: README.ja uses アウトプット; `/ai/run` errors, the locked and one-time open pages, and kiroku's own session titles are translated
- SECURITY.md lines 15–16 split into sub-bullets; the `KIROKU_NO_AI` value rules live only in the guide's Options
- UX: badge contrast, dark disabled AI button and dialog controls, summary blank fixed. The week calendar's overlapping lanes stay as they are (cascading lanes already keep each title's first line; narrower boxes need a design decision)

### Open items
- Previous: "Analyze with AI" with Codex and Kiro CLI only tested with stub commands. Decides: maintainer
- Previous: Kiro CLI `KIRO_DATA_DIR` — still undocumented (settings and cli-commands pages, changelog 2.32–3.5). Due: next data audit
- Previous: Kiro and Codex read-only mechanism — unchanged. Candidates: a kiroku-written Kiro agent (`tools`/`allowedTools: ["fs_read"]`, `allowedPaths` = history folders, `--agent`, `--trust-tools=fs_read`); Codex `-c mcp_servers={}`. New: Claude's `--add-dir` includes the Kiro Crew home, so a run could read incognito Crew conversations kiroku keeps out of its prompts; user settings (`permissions.defaultMode`) still apply. Decides: maintainer
- Previous: `ai.js` untested paths (non-OK answer, stream without `done`, lost connection, too-large prompt) — not checked this run. Due: next full run
- Previous: Codex session started by a Kiro Crew subagent — now resolvable: Crew writes `child_sid` on `subagent/completed`/`failed` in the parent's crew-log, which isn't pruned. Plan: read it into `CrewInfo.Sid`, derive the provider from the spawn's `model`, and mark the Codex thread's first `user_message` as injected. Due: next data audit
- Previous: Kiro V3 tangent, `/rewind` and IDE "Fork conversation" sessions count the parent's prompts again (now confirmed by the tangent and checkpoints docs). The dashboard can tell children apart, but the field isn't documented. Needs a real sample. Decides: maintainer (accept, or guess by matching prompts)
- Previous: `KIRO_HOME` and V2 sessions — unchanged; V2 sessions in `~/.kiro/sessions/cli` are likely missed when `KIRO_HOME` is set. Due: next data audit
- Previous: Claude Code subagent time match in name order — resolved (#481)
- Previous: Claude Code moved-session file — narrower now: a later file whose only own lines are a compaction (idle compaction after `/cd`) creates no session, and its subagents and compaction are lost. A fix must keep the copied-line keys for dedupe; ignore compaction lines in `ownLines` only. Due: next data audit
- Previous: Claude Code fork subagents and resumed `cost-state` — still can't tell without real samples (both suspected). Due: next data audit
- Previous: zstd decoder panic — resolved (#481)
- Previous: UX items — mostly still open (see Needs a decision above); resolved: the AI button's reason is no longer hover-only, `lockedPage` is in Japanese. Due: maintainer decision
- Previous: Go-side native metric labels differ from the view's words. Decides: maintainer
- Previous: `?key=` saved in browser histories. Decides: maintainer
- Previous: security lows — `archive off` resolved (#481); `<code>` wraps now; symlinked session file unchanged (consistent with SECURITY.md); fish and PowerShell quoting resolved by refusing `'` and `\` (#481)
- Previous: F (prompts) — overdue since 2026-11-01. Not run this time because the run's priority was the security floor, upstream and prices. Due: before the next minor release
- Previous: G items — resolved except `codexNewer`, origin-index eviction, `--ai=false` through `cmdServe`, and the subagent end time from `totalDurationMs` without a file (all untested branches). Due: next full run
- Previous: i18n — `/ai/run` errors, the hop page and synthetic session titles still English (lockedPage done). Decides: maintainer (adds Japanese to the CSP-hashed hop script)
- Previous: supply — required checks use `enforcement_level: non_admins`; `install-script` and CodeQL not required. Decides: maintainer. `tag.yml` main guard resolved (#481). CI shellcheck still the runner's
- Previous: docs — SECURITY.md line 16 is now about 5,250 characters, line 15 about 2,500; guide paragraphs at 129, 185, 505, 55 and 110 over 1,500. Decides: maintainer (moves text in a security document)
- Previous: development.md dependency table — resolved (#481)
- New: Haiku 3.5 left the Anthropic price page; `claude-3-5-haiku` was removed from the table (#481), so those sessions are unpriced unless `--prices` is used. Ultrafast for gpt-6.2-astra/sol uses list prices (6x), twice the bill during the promotion through 2027-02-28. Decides: maintainer (keep, or add dated prices)
- New: data suspected — Claude Code: subagent folders left without their session file (before 2.1.323); unknown model IDs get no context window (Claude Code assumes 1M). Kiro: CLI 3.5 can make a second V3 copy of a V2 conversation with an undocumented ID; SQLite `conversations` resume now opens V3's newest session; text added by a UserPromptSubmit hook may count as typed; workflow step sessions before 2.32 copied the parent. Codex: guardian sessions labelled "subagent"; `attached_thread` forks. Due: next data audit
- New: the copied `git -C <repo> show <hash>` runs the repository's own `filter.*.process` and `core.pager` when pasted (kiroku itself runs git safely). Options: add `-c core.pager=cat --no-ext-diff --no-textconv` and a note next to the copy button. Decides: maintainer
- New: `docs/sources.md` line 75 still says V3's resume list doesn't show Classic sessions (the dashboard lists them now). Due: next `/docs-refresh`

### Loop
| Run | Findings (certain) | Fixed | Median days to fix | Open items | Overdue | Previous items resolved |
|---|---|---|---|---|---|---|
| 2026-11-19 full | 60 | 34 | 0 (same day; PR open) | 26 | 1 (F) | 9 of 24 |
| 2026-10-18 full | 64 | 36 | 0 (fixed the same day; PRs open) | 24 | 0 | 8 of 17 |

### Next time
- F first: run `/report-eval` for the four open prompt items
- data: the Crew `child_sid` link, the Kiro fork double count, the Claude compaction-only file; Kiro CLI keeps moving fast (3.0 → 3.5 in three weeks)
- security: the AI least-access mechanism (Kiro agent file, Claude `--add-dir` without the Crew home)
- Run the hostile tester with fish and PowerShell installed, so paste commands are checked by running them

## 2026-10-18 — full at 7eecd3f (v0.38.1)

The second full run. All ten aspects ran: A, B, C, D, E and G as agents, H and I together in one agent, F with `/report-eval check` (6 writers and 1 grader), and J by hand. Fixes went in through #464–#469.

### Gates
| Check | Result |
|---|---|
| tools/check.sh | pass |
| e2e and XSS | pass (smoke, xss and serve.mjs; en and ja) |
| go test -count=3 (order and time) | pass; also `-shuffle=on`, `-race` with TZ Los_Angeles, and TZ Kiritimati |
| main CI, last 20 runs | green (CI, CodeQL, Scorecard, Demo, Fuzz, Tag) |
| govulncheck | No vulnerabilities found (v1.9.0). But GHSA-fpmh-frq8-7rj8 in klauspost/compress v1.20.1, which is not in the Go vuln DB yet, crashed kiroku on a crafted `.zst` (found by aspect I; fixed in #464) |
| Reproducible build of v0.38.1 | matches (linux/amd64) |
| Coverage by package | ai 94.0% (+0.1), archive 79.4%, cli 87.4% (+0.4), core 83.5% (+1.4), gitlog 92.4%, report 90.1%, source 94.3% (+0.5), web 90.6% (+1.1), tools/prices 82.0%. New: imgdiff 86.8%, seclint 91.4%, upstream 29.3% (only the network code is uncovered; the parsers are covered) |
| Benchmark, BenchmarkLarge (this container, 4 CPUs, n=4 against 405c118) | read 887 ms, aggregate 498 ms, render 269 ms; no significant change in time. Render memory 216 → 268 MiB/op (p=0.057): `kiroku serve` copied the page once more for the manifest link (#445). Fixed in #467, which saves one page-sized copy (~20 MB) per reload; that is below the ±12% run-to-run spread here |
| Stale PRs, issues, Dependabot | No open issues. Dependabot #459 (compress, x/sys; superseded by #464), #460 (Playwright) and #461 (actions) are 1 day old and green |

### Findings
| Aspect | Certain | Suspected | Needs a decision | Not worth fixing | Fixed in |
|---|---|---|---|---|---|
| A. security | 5 | 3 | 1 (how to keep the serve key out of browser history) | 1 (short history-chosen kind names in `doctor --report`, reviewed by hand before pasting) | #465, #469 |
| B. ux | 15 | 1 | 1 (what the AI dialog discloses) | 0 | #466, #469 |
| C. i18n | 9 | 0 | 0 | 0 | #466, #469 |
| D. docs | 13 | 1 | 1 (screenshots in light, colored by project) | 2 (ADR numbers in released CHANGELOG entries; long lines, left for `/docs-refresh`) | #469 |
| E. data | 3 | 6 | 0 | 0 | #468 |
| F. prompts | 4 | 0 | 0 | 0 | — (open, see below) |
| G. code | 6 | 2 | 0 | 0 | — (open, see below) |
| H. arch | 5 | 0 | 1 (restore rules lost with the ADRs) | 0 | #469 |
| I. supply | 3 | 1 | 0 | 0 | #464 |
| J. perf | 1 | 0 | 0 | 0 | #467 |

Decisions the maintainer made during this run:
- Keep the serve key out of browser history with a one-time code that the page trades for the key by a same-origin POST (#465). In a real Chromium profile, the old `?key=` address was saved in History, and synced when history sync is on.
- Make docs/usability.md match the AI dialog as it is (the dialog doesn't explain `--no-ai`).
- Restore the three rules that lived only in the removed ADRs: Crew private conversations, archive is opt-in, and the AI exception in CLAUDE.md (#469).
- Leave the screenshots as they are.

F (prompts), `/report-eval check` at 7eecd3f (2 runs each):
- Every run copied the numbers table exactly and kept the format. No secret or injection leaked, and no run read outside the allowed files.
- Recall: day 5/5, week 8/8, session 3/4 (the PR is never named).
- Prompt size: day 2.1K, week 3.2K, session 1.4K tokens. Writer tokens: 55–71K.
- Grader: summaries 3–5, advice 3–4. Session split 2–3: both runs call the CI-only TZ redo a prompt gap.

### Open items
- Previous: "Analyze with AI" with Codex and Kiro CLI has only been tested with stub commands. Needs one real run each. Decides: maintainer
- Previous: `mdView` table after a paragraph, and a link URL containing `)` — resolved (#453)
- Previous: Kiro CLI `KIRO_DATA_DIR` is still undocumented (configuration and headless docs, changelogs 2.27–2.31.1). Due: next data audit, by 2026-11-07
- Previous: Kiro CLI runs in "Analyze with AI" have no read-only mode kiroku can set — documented in SECURITY.md and the guide (#469). Mechanism still open: headless `--trust-tools=read_file,grep_search` still lets the user's `allow` rules apply, and only a kiroku-written agent with `deny` rules would enforce read-only. Codex has the same gap for MCP servers: `-c mcp_servers={}` does not help, because config layers merge tables. Due: next security audit, by 2026-11-18
- Previous: Windows, stopping an AI run with a `.cmd` shim — resolved (#450). A child started between `Start` and the job assignment can escape; that window is microseconds, and a full fix needs `PROC_THREAD_ATTRIBUTE_JOB_LIST`. Low; judged not worth it now
- Previous: `ai.js` had no automated test of streaming, errors and cancel — resolved (#451, serve.mjs). Not covered yet: a non-OK HTTP answer, a stream that ends without `done`, a lost connection, and the too-large prompt message. Due: next full run
- Previous: Codex session started by a Kiro Crew subagent — unchanged (Crew's task text counts as a typed prompt; after `state.json` is gone, the subagent folds into its parent). Crew's `subagent/spawned` log now carries `model`, which could set the provider. Due: next data audit, by 2026-11-07
- Previous: Kiro CLI V3 `/rewind`, tangent and subagent sessions — the docs now confirm that a child session starts with the parent's history, so prompts (and maybe credits) are counted twice. V3 becomes the default with CLI 3.0 at the end of October. Needs a real V3 sample to find the parent field. Due: 2026-11-07
- Previous: with `KIRO_HOME` set, Classic (V2) and Kiro IDE sessions in `~/.kiro` may be missed. The docs say `KIRO_HOME` moves only V3 storage. Due: next data audit, by 2026-11-07
- Previous: Claude Code subagent collision of `agentId` and time matches — resolved (#452). New: files linked by time are matched in name order, not start order, so with two overlapping calls and no `agentId`, a later file can take the earlier call (aspect G, confirmed with a scratch test). Due: next full run
- Previous: "Analyze with AI" prompts suggested `git show` and `zstd -dc` — resolved (#449)
- Previous: SECURITY.md rebuild example and `POST /archive` — resolved (go1.26.8 already fixed; `/archive` in #469)
- Previous: UX low items — mostly resolved (#454; the year chart is gone). Still open: the tagline is hidden at 1000 px and 390 px; a blank of about 1000 px in the summary at 1440 and 1000. Due: next ux audit
- Previous: Go-side native metric labels (「見積もり」, 「最初の返事」, 「週の枠」) differ from the view's words; they are map keys in stored data. Decides: maintainer
- Previous: after #414, each browser needs `kiroku open` once — no reports; closed
- Previous: zstd 1 GiB cap per file — still no budget per reload. New hardening idea: decode synchronously (`zstd.WithDecoderConcurrency(1)`) and recover in `ZstdReader`, so a future decoder panic is reported as an unreadable file instead of stopping `kiroku serve`. Due: next full run
- Previous: Claude Code moved-session file whose only own lines create no session loses its subagents — unchanged, rare
- Keys already saved in browser histories by older versions (`?key=`) stay valid until the user changes the key (the changelog says how). Rotating it once automatically, or refusing `?key=`, would force every browser through `kiroku open` again. Decides: maintainer
- Security, low: `kiroku archive off` asks to delete N files but counts kiroku's marker and other files (13 deleted out of 14 announced); a long unbroken path in `<code>` scrolls sideways at 390 px; `/history` refuses a symlinked session file that `/data.json` still shows (consistent with SECURITY.md); the single-quote escaping of the resume command in fish and PowerShell wasn't tested. Due: next security audit
- Data, suspected (need real samples): Claude Code fork subagents (on by default in `claude -p` since 2.1.302) may copy the parent's lines and be counted twice; resumed cloud sessions' `cost-state` may carry earlier spend into a new process (2.1.302); Kiro V3 throttled re-sends and the new `tombstone` rows (Crew a9898a0e, kiro-cli 2.32.0). Due: next data audit, by 2026-11-07
- F (prompts): the week report misses the planted "short question on Opus → a lighter model" hint (2 of 2 runs); the session review calls the CI-only TZ redo a prompt gap (2 of 2), not work uncertainty; day reports repeat the AI's "129 passed, lint clean" as fact, although no run is recorded; the session review never names the PR. Adding the PR URL to the session prompt's flow alone did not make the AI cite it (1 run). Fix with `/report-eval`, comparing before and after. Due: 2026-11-01
- G (code): a latent flake in `TestLiveRefresh` and the crew-private live test (the baseline fingerprint is taken inside the goroutine; fails with a 100 ms delay); `live.js` poll reports any render error as "Not connected"; a demo serve in another time zone reloads once and applies data without the time shift; dead code (`limitedWriter`, `Usage.Len`, `OutputTotal.Empty`); untested branches (subagent usage without a file, origin-index LRU eviction, `codexNewer`, `--ai=false`); `autostart off` ignores a failed unregister command. Due: next full run
- UX, mid/low (aspect B): the "A day to match last month" tile shows "+$6.57 over" (a total) next to credits per day, and its breakdown mentions faded bars and calendar days that aren't drawn; the disabled "Analyze with AI" is brighter than the enabled one in dark, and its reason is hover-only; agent badges in light are 1.9:1–2.8:1; overlapping parallel sessions hide titles in the week calendar; metric value buttons are named only by their number; a flagged metric's breakdown has no trend or threshold; the sparkline says "8-week range" over 5 points; repeated-prompt order disagrees with "most frequent first"; native controls are unstyled in the dark AI dialog; Scenario 1 no longer shows the top project above the fold since "Color by Agent". Due: next ux audit
- i18n, low (aspect C): English-only text from Go reaches ja users (`lockedPage`, the hop page's message, `/ai/run` errors, synthetic session titles such as "Kiro Crew private conversation"). Due: next i18n audit
- Supply: required checks on main use `enforcement_level: non_admins` (admins can merge red), and `install-script` and CodeQL are not required. Decides: maintainer. `tag.yml` can release from any branch on a manual run (add a `refs/heads/main` guard); shellcheck in CI is the runner's 0.9.0, not pinned. Due: next supply audit
- Docs: SECURITY.md line 16 (about 4,000 characters) and several guide paragraphs over 1,500 characters should be split; the `--no-ai` rules are written out four times. Due: next `/docs-refresh`
- docs/development.md has no table of package dependencies; the imports are layered (core and ai import no internal package; source, archive, gitlog and web import only core; report imports core and gitlog; cli imports all). Add it with the next change to development.md

### Loop
| Run | Findings (certain) | Fixed | Median days to fix | Open items | Overdue | Previous items resolved |
|---|---|---|---|---|---|---|
| 2026-10-18 full | 64 | 36 | 0 (fixed the same day; PRs open) | 24 | 0 | 8 of 17 |

### Next time
- data first: Kiro CLI 3.0 (V3 by default) at the end of October; the V3 child-session double count and `KIRO_HOME`; Claude Code fork subagents
- prompts: run `/report-eval` for the four F items before the next full run
- code: the G items (most are small); security: the Kiro and Codex least-access mechanism
- Dependency advisories that govulncheck doesn't know yet: check each module's GitHub advisories in the supply aspect, as aspect I did this time

## 2026-10-14 — full at 405c118 (v0.34.1)

The first full run. All ten aspects ran (A–I as agents, J by hand). Fixes went in through #406–#414.

### Gates
| Check | Result |
|---|---|
| tools/check.sh | pass |
| e2e and XSS | pass. Since #413 the e2e runs at a fixed time (`KIROKU_FAKE_NOW`) and also in Japanese; since #414 it also runs `kiroku serve` (`serve.mjs`) |
| go test -count=3 (order and time) | pass; also `-shuffle=on`, `-race`, and TZ Los_Angeles, Tokyo, Kiritimati. `TestMdView` failed once under load (fixed in #409 by measuring best of 5 above 50 ms) |
| main CI, last 20 runs | green. A PR run failed on Windows once (a test setup Windows can't build; fixed in the same PR) |
| govulncheck | No vulnerabilities found (v1.8.0, then v1.9.0 from #408) |
| Reproducible build of v0.34.1 | matches |
| Coverage by package | ai 93.9% (+0.2), archive 79.4%, cli 87.0%, core 82.1% (+0.2), gitlog 92.4%, report 90.1%, source 93.8%, web 89.5% (+1.5), tools/prices 82.0% |
| Benchmark, BenchmarkLarge (this container, 4 CPUs) | read 878–975 ms, aggregate 499–557 ms, render 236–245 ms. The baseline commit 56fc970 measured at the same time gave read 945–1049 ms and aggregate 517–567 ms, with the same memory and allocations, so the +20% against the baseline entry is the machine, not the code |
| Stale PRs, issues, Dependabot | PR #378 is closed. Issues #323–#328 have had no activity for over a week (the maintainer decides whether to close them) |

### Findings
| Aspect | Certain | Suspected | Needs a decision | Not worth fixing | Fixed in |
|---|---|---|---|---|---|
| A. security | 8 | 1 | 1 (serve key in a cookie) | 1 (wide code at 390 px, from a very long scratch path) | #409, #414 |
| B. ux | 16 | 0 | 1 (provider notice) | 1 (Esc stops a run, deliberate since v0.34.1) | #407, #411 |
| C. i18n | 20 | 2 | 0 | 4 (duration units, plural counters, year-light glosses, the generic "Report" key) | #406 |
| D. docs | 8 | 0 | 0 | 1 (version wording in SECURITY.md) | #408, #409 |
| E. data | 3 | 2 | 0 | 1 (Opus 4.6 fast pricing in old sessions) | #410 |
| F. prompts | 10 | 1 | 0 | 0 | #412 |
| G. code | 7 | 2 | 0 | 0 | #409, #413 |
| H. arch | 8 | 0 | 2 (ADR 0030; accepting 0026–0028) | 0 | #411, #413 |
| I. supply | 6 | 1 | 0 | 1 (provenance check in `kiroku update`, accepted in ADR 0024) | #408 |
| J. perf | 0 | 0 | 0 | 0 | — |

Decisions the maintainer made during this run: keep the serve key in per-origin browser storage and send it as a header (ADR 0031, #414); record `kiroku serve --ai` as an exception to ADR 0022 and show the provider notice again in the run dialog (ADR 0030, #411); accept ADRs 0026 and 0027, and Phase 1 of 0028.

### Open items
- Previous: "Analyze with AI" with an older Claude Code — resolved. The flags have existed for a long time (`--include-partial-messages` since 1.0.109), and a version without them fails instead of running with more access (ADR 0030)
- Previous: the provider notice and its ADR — resolved (ADR 0030, #411)
- Previous: PR #378 — resolved (closed)
- Previous: "Analyze with AI" with Codex and Kiro CLI has only been tested with stub commands. Needs one real run each. Decides: maintainer
- Previous: `mdView`: a table right after a paragraph line, and a link URL containing `)`. Low. Due: next full run
- Previous: Kiro CLI `KIRO_DATA_DIR` is still undocumented (not in the settings reference or the changelog through 2.31.0). Due: next data audit
- Kiro CLI runs in "Analyze with AI" have no read-only mode kiroku can set; they use the user's own Kiro tool settings (ADR 0030). Revisit when Kiro CLI offers a tool allowlist. Due: next data audit
- Windows: stopping an AI run may leave the agent running when it is installed as a `.cmd` shim (only `cmd.exe` is killed). Fix with a Job Object or `taskkill /T`; needs a Windows machine to check. Due: next full run
- A zstd history file that expands to gigabytes stalled each reload for tens of seconds — resolved (#418, 1 GiB per file). Many files near the cap can still slow a reload; a budget per reload is not set
- `ai.js` (the AI dialog) has no automated test of streaming, errors and cancel. Add a node harness or a `serve.mjs` step with a stub agent. Due: next full run
- Codex session started by a Kiro Crew subagent: Crew's task text still counts as a typed prompt in the Codex session (#410 stopped the double count only); after Crew deletes `state.json` (1 hour) the subagent is folded into its parent again. Due: next data audit
- Kiro CLI V3 `/rewind`, tangent and subagent sessions may copy their parent's lines and be counted twice. Needs a real V3 sample. Due: next data audit
- With `KIRO_HOME` set, Classic (V2) and Kiro IDE sessions left in `~/.kiro` may be missed. Check where they are written. Due: next data audit
- A moved Claude Code session whose later file holds only lines already counted produces no session, and subagents only in that file are lost (found while writing the #413 test) — resolved (#419). A later file whose only own lines create no session (a summary, title or cost-state line) still loses its subagents; judged rare
- Claude Code subagent files are matched to their Agent call by `agentId`, then by time. When a time-matched file and an `agentId`-matched file land on the same call, the second replaces the first and one subagent is not counted (found while writing the #419 test). Reproduce with a test and fix. Due: next full run
- "Analyze with AI" runs a report prompt that suggests `git show` and `zstd -dc`, which the Claude run can't use (only Read). Say so in the prompt when it runs through `--ai`. Due: next prompts audit
- SECURITY.md's manual rebuild example for v0.13.3 names `go1.26.9`; the tag's go.mod says `go1.26.8`. And SECURITY.md doesn't mention `POST /archive`. Due: next docs audit
- UX, low: year chart colors differ from the main view; the phone header hides the tagline; the copy toast is narrow on phones; ArrowDown doesn't move into search results; "N more sessions" in Worth a look can't be opened; narrow blocks split words; the phone year chart cuts the first month label; a large blank at 1440 in the summary; 「今週」 nearly touches 「表示」 at 1000 px. Due: next ux audit
- i18n, low: Go-side native metric labels (「見積もり」, 「最初の返事」, 「週の枠」) differ from the view's words, but they are also map keys in stored data, so renaming needs a migration. Decides: maintainer
- After #414, each browser (and `localhost` and `127.0.0.1` separately) needs `kiroku open` once. Watch for reports

### Loop
| Run | Findings (certain) | Fixed | Median days to fix | Open items | Overdue | Previous items resolved |
|---|---|---|---|---|---|---|
| 2026-10-14 full | 86 | not counted (see "Fixed in") | 0 (fixed the same day) | 15 | 0 | 3 of 6 |

### Next time
- data (the open Kiro and Crew items above), code (the moved-session case, the zstd cap, Windows process cleanup), ux (the low items above)
- Run hostile-tester again against `kiroku serve` after #414 settles

## 2026-10-14 — baseline at 56fc970 (v0.34.1)

The first entry. It records the state after the reviews of 2026-10-14 (PRs #394–#403), so the first full run has something to compare with.

### Gates
| Check | Result |
|---|---|
| tools/check.sh | pass |
| e2e and XSS | pass |
| main CI | green; two time- or speed-dependent tests were fixed today (`smoke.mjs` reply order in #396, `TestMdView` timing in #402) |
| govulncheck | No vulnerabilities found |
| Coverage by package | ai 93.7%, archive 79.4%, cli 87.0%, core 81.9%, gitlog 92.4%, report 90.1%, source 93.8%, web 88.0%, tools/prices 82.0% |
| Benchmark, BenchmarkLarge (this cloud container, 4 CPUs) | read 806 ms / 943 MB / 10.3M allocs; aggregate 492 ms / 221 MB; render 244 ms (HTML 20.9 MB, JSON 20.4 MB) |
| Required checks | e2e is required on main since 2026-10-14 (after #395 merged with a red e2e) |

### Open items
- "Analyze with AI" with an older Claude Code: the run uses `--output-format stream-json --verbose --include-partial-messages`; a version without them fails instead of falling back. Check the oldest supported Claude Code version, or fall back to `--output-format text`. Due: next full run
- "Analyze with AI" with Codex and Kiro CLI has only been tested with stub commands. Needs one real run each on the maintainer's machine. Decides: maintainer
- The view no longer repeats that the agent may send the prompt and history to its provider (maintainer's request); the Proposed AI-execution ADR asks for an explicit disclosure before the first run. Record the decision with `/forward-adr`, or restore a one-time notice. Decides: maintainer
- `mdView` shortcomings: a table right after a paragraph line isn't recognised; a link URL containing `)` is cut at the first `)`. Low impact. Due: next full run
- PR #378 (an old Project view prototype, superseded by the released Project view) is open as a draft. Decides: maintainer (close)
- Kiro CLI `KIRO_DATA_DIR` appears in the 2.31 binary with unknown effect; not added to `core.AgentEnv` without evidence. Check the Kiro CLI docs or changelog. Due: next data audit

### Next time
- data (Claude Code, Codex and Kiro CLI release often), ux and i18n for the AI dialog, code (time-dependent tests)
