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
