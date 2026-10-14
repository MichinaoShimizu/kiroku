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
