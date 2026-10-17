# ADR 0035: No monthly AI report

Status: Accepted (2026-10-16, approved by the maintainer)
Date: 2026-10-16
Scope: The reports "Analyze with AI" offers in `kiroku serve` (`internal/web/js/summary.js`, `panels.js` `reportPrompt`, `tools/reportbench`, the report-eval skill, README, the guide)
Related ADRs: [ADR 0034](0034-ai-analysis-by-default.md), [ADR 0030](0030-opt-in-local-agent-execution.md)
Related Decision Logs: DL 0013 — user-mediated AI reports
Supersedes/Amends: Amends [ADR 0034](0034-ai-analysis-by-default.md) point 1, first bullet ("for the weekly and monthly report and the session review"). Everything else in ADR 0034 and ADR 0030 points 3–6 stays in force
Decision owner: Repository maintainer

## Context

ADR 0034 offers "Analyze with AI" for the weekly and monthly report and the session review; the daily report was added back later next to each date in the week calendar. ADR 0030 point 5, kept by ADR 0034, limits one agent run to 2 minutes.

- Measured with `tools/reportbench` (synthetic history, 8 sessions in the month) on 2026-10-16, the monthly report took 75–105 seconds per run: the agent opened 3–4 history files and used 5–9 tool calls. A real month usually has many more sessions; when the digest is shortened to fit the prompt budget, the prompt asks the agent to read the history files for what it leaves out, which takes longer still.
- A run that hits the limit ends with an error and no usable report.
- Alternatives were discussed with the maintainer on 2026-10-16 (below). The maintainer chose to remove the monthly report.

## Decision

1. `kiroku serve` offers "Analyze with AI" for the daily report, the weekly report and the session review. The month view has no report button; the monthly summary and the month calendar stay.
2. `reportPrompt` builds only daily and weekly reports. `tools/reportbench` and the report-eval skill test day, week and session.
3. ADR 0030's limits (one run, 2 minutes, 256 KiB, 1 MiB) are unchanged.

## Alternatives

- **Keep the monthly report and cap the history files it opens.** Lowers the time per run, but a large month would still be close to the limit, and the report would rest on a thinner digest. Not chosen.
- **A longer limit for reports (for example 5 minutes).** Changes ADR 0030 point 5 for one feature, and long runs keep the dialog busy. Not chosen.
- **Build the monthly report from weekly reports.** Proposed to the maintainer; it needs a place to keep weekly reports or several confirmed runs, and weeks that cross the month's edges would bring in work outside the month. Not chosen for now.

## Consequences

- Fewer runs that end at the time limit; the report the user can run always fits a week or a day.
- Users who want a monthly view run the weekly reports, or read the monthly summary. kiroku no longer writes a monthly prompt.
- The change is in the view only: no command, option, JSON field or file kiroku writes changes, so it is not `BREAKING` under ADR 0023. It is recorded under `### Removed`.
- Reversible: a monthly report can come back, for example built from weekly reports, with a new ADR.

## Verification

- `sh tools/check.sh --e2e`: the month view shows no report bar, the weekly and daily report prompts are unchanged.
- `tools/reportbench` builds day, week and session prompts only.

## Open questions

- None for this change. A future monthly report built from weekly reports would need its own decision on where weekly reports are kept.
