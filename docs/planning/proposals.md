# Feature proposals

Each run of `/feature-plan` (`.claude/skills/feature-plan/SKILL.md`) adds a section at the top. It proposes features and changes for kiroku from upstream changes (new versions, official docs and prices of the agents kiroku reads) and gaps in the current behavior, along three lines: the experience of the engineers who use kiroku (dx), the cost of using agents (cost) and how well people understand agent work (literacy). Proposals are material for the maintainer, not decisions: work starts only from a `[request]` issue the maintainer opens (`/request`). Recording what was declined and why keeps the next run from proposing it again. Only dummy data is used; never write real history, paths or user names here.

Template for a new section:

```markdown
## YYYY-MM-DD — <all | upstream | spec | dx | cost | literacy | quick> at <commit> (<version>)

### Upstream versions seen
| Agent | Version | Changes that matter to users |
|---|---|---|

### Proposals
| ID | Title | Axes | Score | Size | Status |
|---|---|---|---|---|---|
| P-YYYYMM-1 | | cost, literacy | | S | proposed |

Status: proposed · adopted #<issue> · declined <reason> · deferred <until / waiting for>

### Updates to earlier proposals
- P-YYYYMM-n: <new status> (<issue, PR or the maintainer's answer>)

### Not proposed
- <idea> — <reason: which precondition, already exists, declined earlier (ID or decision log)>

### Passed on
- <mismatch for /source-audit, /price-update or /quality-audit>
```

## 2026-11-24 — all at 1a5487b (v5.1.1)

First run; no earlier proposals to update.

### Upstream versions seen
| Agent | Version | Changes that matter to users |
|---|---|---|
| Claude Code | 2.1.251–2.1.331 (CHANGELOG) | Opus 5.5, Sonnet 5.5, Haiku 5.5 and Fable 5.2 (all 1M context; prices already in the table); idle compaction of 200K+ conversations before the 1-hour cache expires (2.1.299, `idleCompaction`); effort for subagents (`CLAUDE_CODE_SUBAGENT_EFFORT`, Agent `effort`); `/cost` and status line show prompt-cache hits and likely miss causes (2.1.251, 2.1.260); "5-hour limit" wording (2.1.312); fast mode for Opus 4.8 removed on 2026-11-30 (2.1.316); transcripts record each request's system prompt, tools and parameters (2.1.300) |
| Codex | codex-rs HEAD 1f43c02 (2026-11-24); release notes not reachable | `turn_context.effort` (reasoning effort per turn), `collaboration_mode` (plan/default), thread goals with `token_budget` and `turn_aborted.reason = budget_limited`, `rate_limits.*.resets_at`, credits balance, `individual_limit`, `spend_control_reached`; model reroutes are no longer persisted |
| Kiro IDE / CLI | IDE 1.5.11, CLI 3.5.0 | Claude Fable 5.2 Preview (Enterprise, 1M, 6x credits); per-model credit multipliers with long-context surcharges; model fallback settings; CLI V3 auto-compaction threshold (default 80%) |
| Kiro Crew | 0.14.0 | Kiro Agent runs on Crew's own engine (KAS) by default; `kirocrew usage` (credits by day, model, agent, session); crew log `compaction/applied`, `context/measured`, `model/selected`, `background/completed` |

### Proposals
| ID | Title | Axes | Score | Size | Status |
|---|---|---|---|---|---|
| P-202611-1 | "Expensive models for light work" also counts Fable (not only Opus) | cost, literacy | 3.0 | S | adopted #569 |
| P-202611-2 | Reset time on Codex usage limit hits | dx, cost | 2.0 | S | adopted #570 |
| P-202611-3 | Usage limit hits in the day report | cost, literacy | 2.0 | S | proposed |
| P-202611-4 | Codex reasoning effort per turn in the session view | cost, literacy | 1.5 | M | proposed (needs a `/source-audit` row first) |
| P-202611-5 | Kiro Crew compactions from the crew log (kiro-cli backend only) | cost, literacy | 0.5 | M | proposed (candidate) |

Status: proposed · adopted #<issue> · declined <reason> · deferred <until / waiting for>

Notes:
- P-202611-1: the price table puts Fable 5.2 at $8/$40, twice Opus 5.5 ($4/$20), but `lightOf` (internal/web/js/review.js:21-22) and the day report (internal/web/js/panels.js:136) match `/opus/i` only. A price-based bar was considered and left for later: "Opus-class" has no fixed price (Opus 4.1 $15, Opus 4.x $5, Opus 5.5 $4), Codex service tiers change what was paid, `--prices` overrides need the Go side to send a per-model flag, and the bar would move with every `/price-update`. The wording "Opus-class" (summary.js:54, i18n.js:605, 665) changes with it. Kiro Opus sessions (credits, no USD) stay in the day report signal and stay out of Worth a look, as now.
- P-202611-2: `RateLimitWindow.resets_at` (codex-rs/protocol/src/protocol.rs:2788) is not read (docs/sources.md "Usage limits"). Take it from the snapshot that recorded the hit, from the window at or over 100%; credits-depleted hits have no reset time. Claude Code already shows "(resets 21:00)".
- P-202611-3: the day report (panels.js:101-142) has no limit hits; they have timestamps, so they can be counted for the day. Corrections and interruptions are per session and need their timestamps (`InterruptsAt`, `Fix`) to be counted per day; cache share is not in the daily aggregate. Both are left for later. A change to `reportPrompt` goes through `/report-eval`.
- P-202611-4: `TurnContextItem.effort` (protocol.rs:3792) is in every `turn_context`; codex.go:548-555 reads only the model and cwd. docs/sources.md has no effort row, so `/source-audit` adds it first (precondition 3). Effort can change per turn; a missing value means the model's default, and `none` is a real level. Kiro Crew carries it as a model suffix (`gpt-6-astra[max]`); Claude Code is unconfirmed. Show the value without good or bad.
- P-202611-5: `compaction/applied` is written by default (kirocrew crew_log/emit_turn.py), but `KIROCREW_CREW_LOG` can turn the log off, so no log is not 0. Codex- and Claude Code-backed conversations already count compactions in their own history; count only kiro-cli-backed rows. Checked by the reviewer, not by this run (certainty ×0.5).

### Not proposed
- Worth a look for cache written again after an idle gap — the write after a gap mixes the expired prefix with new content, every session's first response writes the cache, Claude Code's idle compaction writes near the 1-hour TTL, and Codex records no TTL; a finding would claim a cause the history can't show (precondition 3). At most a Claude Code-only upper bound later.
- Claude Code compaction trigger and size before compaction — the trigger is already shown in the prompt flow (session.js:149); `preTokens` is close to the input of the response before it, which is already read (peak context).
- Anything that runs an agent (precondition 2; decision logs 0010, 0011), a monthly AI report (0012), the weekly report prompt (removed), the year chart (removed, #454), Journey/Project views (0014).

### Passed on
- `/price-update`: GPT-5.6 Sol promotional price ends 2026-11-30; fast mode for Opus 4.8 is removed on 2026-11-30 (Claude Code 2.1.316).
- `/source-audit`: add rows for reasoning effort (Codex `turn_context.effort`, Kiro Crew model suffix, Claude Code unconfirmed); Codex `model_reroute` is no longer persisted, so the reader's entry matches only older files; Claude Code 2.1.300 writes each request's system prompt, tools and parameters into the transcript (shape unknown; check that nothing of it is shown or copied); where Kiro Crew's KAS engine stores conversations; Kiro V3 compaction and fallback-model records (docs are silent).
- `/source-audit`: Codex release notes could not be read in this run; check the versions behind the fields above.
