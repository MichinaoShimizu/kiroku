---
kind: log
category: product
keywords: [reportPrompt, rptbar, monthly report, tools/reportbench, aiDemo, KIROKU_DEMO, runAIReview]
---

# [kiroku] "Analyze with AI" offers no monthly report, and the live demo previews the prompt without running it

> A past observation, not a decision. It binds nothing.

## What happened

- C1 2026-10-16 Still true: the month view has no "Analyze with AI" report; reports are daily and weekly, plus the session review. History: [PR #446](https://github.com/MichinaoShimizu/kiroku/pull/446) / Now: [`internal/web/js/summary.js` @`1d7b409`](https://github.com/MichinaoShimizu/kiroku/blob/1d7b4091906cc02d4f32bdabd120f5120dcf0792/internal/web/js/summary.js), [`internal/web/js/panels.js` `reportPrompt` @`1d7b409`](https://github.com/MichinaoShimizu/kiroku/blob/1d7b4091906cc02d4f32bdabd120f5120dcf0792/internal/web/js/panels.js)
- C2 2026-10-16 Still true: in the `KIROKU_DEMO` build (the GitHub Pages demo), the "Analyze with AI" buttons open the dialog with the exact prompt, Run is disabled and nothing is sent; other saved HTML shows no AI actions. History: [PR #447](https://github.com/MichinaoShimizu/kiroku/pull/447) / Now: [`internal/web/js/state.js` `aiDemo` @`1d7b409`](https://github.com/MichinaoShimizu/kiroku/blob/1d7b4091906cc02d4f32bdabd120f5120dcf0792/internal/web/js/state.js), [`internal/web/js/ai.js` `runAIReview` @`1d7b409`](https://github.com/MichinaoShimizu/kiroku/blob/1d7b4091906cc02d4f32bdabd120f5120dcf0792/internal/web/js/ai.js)

## Reason

Documented (description, AI): measured with `tools/reportbench`, a small synthetic month took 75–105 s against the 2-minute limit, and real months are larger ([PR #446](https://github.com/MichinaoShimizu/kiroku/pull/446)); the demo is a saved HTML file, so visitors never saw the feature or its prompt, and the maintainer asked to show it ([PR #447](https://github.com/MichinaoShimizu/kiroku/pull/447)). Not chosen for the monthly report: a cap on files opened, a longer limit, a report built from weekly reports. Not chosen for the demo: no buttons, or a canned example result that could go stale or be read as a real run.

## People involved

From the PR records; not necessarily who decided.

- PR #446: opened @MichinaoShimizu (2026-10-16), no approving review, merged @MichinaoShimizu (2026-10-16)
- PR #447: opened @MichinaoShimizu (2026-10-16), no approving review, merged @MichinaoShimizu (2026-10-16)
- Both PR descriptions call their ADR Proposed; commits by the agent marked them Accepted minutes before the merges.

## Open questions

- Is a monthly report built from weekly reports still wanted later? (ask @MichinaoShimizu)
- Do the reasons above, written by an AI agent in the PR descriptions, match the maintainer's own judgement? (ask @MichinaoShimizu)

## Sources

- [ADR 0035 (removed)](https://github.com/MichinaoShimizu/kiroku/blob/27e8298bdc0367afc623bc4db5c450ca0bad38b1/docs/adr/0035-no-monthly-ai-report.md)
- [ADR 0036 (removed)](https://github.com/MichinaoShimizu/kiroku/blob/27e8298bdc0367afc623bc4db5c450ca0bad38b1/docs/adr/0036-ai-prompt-preview-in-demo.md)

## Related logs

- DL-10
