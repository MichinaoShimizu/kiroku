# ADR 0036: "Analyze with AI" prompt preview in the live demo

Status: Accepted (2026-10-16, approved by the maintainer)
Date: 2026-10-16
Scope: The public demo built with `KIROKU_DEMO` (`tools/screenshots/run.sh --html`, `.github/workflows/pages.yml`), `internal/web/js` `state.js` (`aiDemo`), `ai.js` (`aiBtn`, `runAIReview`), `calendar.js`, the guide
Related ADRs: [ADR 0034](0034-ai-analysis-by-default.md), [ADR 0030](0030-opt-in-local-agent-execution.md), [ADR 0022](0022-history-stays-local.md)
Related Decision Logs: DL 0013 — user-mediated AI reports
Supersedes/Amends: Amends [ADR 0034](0034-ai-analysis-by-default.md) point 1 ("A saved HTML file (`kiroku html`) never runs an agent and shows no AI action at all") for the demo build only
Decision owner: Repository maintainer

## Context

"Analyze with AI" is the main way kiroku turns history into a review or report, but it exists only in `kiroku serve` (ADR 0034). The live demo on GitHub Pages is a saved HTML file of dummy data, so visitors never see the feature or the prompt it builds. On 2026-10-16 the maintainer asked to show the button in the demo too.

A static page can't run an agent: there is no `kiroku serve`, no `/ai/run` and no key.

## Decision

1. In an HTML file built with `KIROKU_DEMO` (`META.demo`, only the dummy-data demo), the view shows the "Analyze with AI" buttons where `kiroku serve` shows them: the weekly report, the daily report next to each date, and the session review.
2. They open the same dialog with the exact prompt (read-only). The dialog says it is the demo and that `kiroku serve` sends this prompt to an agent on the user's computer. There is no agent choice, Run is disabled, and the dialog never calls `/ai/run` or any other address.
3. Every other saved HTML file (`kiroku html` without `KIROKU_DEMO`) still shows no AI action, as ADR 0034 says.

## Alternatives

- **Keep the demo without AI buttons (ADR 0034 as is).** Visitors can't see the feature. Not chosen.
- **Show a canned example result in the demo.** It would show the output too, but a fixed text can go stale as prompts change, and it could be read as a real run. Not chosen for now.

## Consequences

- Visitors see what kiroku would send and in which format, using dummy data only.
- `KIROKU_DEMO` is an environment variable anyone can set; a user who builds their own HTML with it gets the same disabled preview of their own prompt, which the file already contains. Nothing more leaves the machine.
- Tests: `smoke.mjs` (which runs on the demo build) checks the buttons, the dialog text, the disabled Run and that no `/ai/run` request is made; `xss.mjs` (a normal saved HTML) checks that no AI control is shown.
- Reversible by removing the `aiDemo` branch.

## Verification

- `sh tools/check.sh --e2e` passes with the checks above.

## Open questions

- None.
