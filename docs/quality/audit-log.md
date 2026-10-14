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

### Next time
- <aspects to weigh more>
```

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
