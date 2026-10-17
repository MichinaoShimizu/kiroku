---
kind: log
category: product
keywords: [journeybtn, projectbtn, st.view, kiroku:view, journeyGrid, projectGrid, projectKnowledgePrompt]
---

# [kiroku] The Journey and Project views were added and then removed; the view is the calendar only

> A past observation, not a decision. It binds nothing.

## What happened

- C1 2026-10-11 Replaced: a Journey view grouped observed sessions by project and day, beside Week and Month. History: [PR #372](https://github.com/MichinaoShimizu/kiroku/pull/372) / Now: removed by C3
- C2 2026-10-12 Replaced: a Project view added project cards with sessions, commits and PRs, as a third choice (Calendar / Journey / Project). History: [PR #379](https://github.com/MichinaoShimizu/kiroku/pull/379) / Now: removed by C3
- C3 2026-10-14 Still true: the view is the Week / Month calendar only; the View switch, the Journey and Project views and "Copy knowledge prompt" were removed, and a saved `kiroku:view` of journey or project opens the calendar. History: [PR #427](https://github.com/MichinaoShimizu/kiroku/pull/427) / Now: [`internal/web/js/state.js` @`1d7b409`](https://github.com/MichinaoShimizu/kiroku/blob/1d7b4091906cc02d4f32bdabd120f5120dcf0792/internal/web/js/state.js), [`internal/web/template.html` @`1d7b409`](https://github.com/MichinaoShimizu/kiroku/blob/1d7b4091906cc02d4f32bdabd120f5120dcf0792/internal/web/template.html)

## Reason

Documented (description): #372 calls Journey an experimental prototype answering what project activity continued over time, while the calendar answers when ([PR #372](https://github.com/MichinaoShimizu/kiroku/pull/372)); #379 adds a project-first inventory with no inferred status ([PR #379](https://github.com/MichinaoShimizu/kiroku/pull/379)).

Documented (description, AI): #427 says "The maintainer decided on 2026-10-14 to simplify kiroku": the views answer overlapping questions that the calendar, the legend filters and "By project" already cover, and each costs states, accessibility, layout, translations, tests and screenshots ([PR #427](https://github.com/MichinaoShimizu/kiroku/pull/427)).

## People involved

From the PR records; not necessarily who decided.

- PR #372: opened @MichinaoShimizu (2026-10-11), no approving review, merged @MichinaoShimizu (2026-10-11)
- PR #379: opened @MichinaoShimizu (2026-10-11), no approving review, merged @MichinaoShimizu (2026-10-12)
- PR #427: opened @MichinaoShimizu (2026-10-14), no approving review, merged @MichinaoShimizu (2026-10-14)
- #372 and #379 were merged minutes after they were opened, although their descriptions asked for CI and browser review first.

## Open questions

- ADRs 0026 and 0027 were marked Accepted and then superseded by #427 on the same day. What changed in between? (ask @MichinaoShimizu)
- Do the reasons above, written by an AI agent in the PR descriptions, match the maintainer's own judgement? (ask @MichinaoShimizu)

## Sources

- [ADR 0033 (removed)](https://github.com/MichinaoShimizu/kiroku/blob/27e8298bdc0367afc623bc4db5c450ca0bad38b1/docs/adr/0033-calendar-only-view.md)
- [ADR 0026 (removed)](https://github.com/MichinaoShimizu/kiroku/blob/27e8298bdc0367afc623bc4db5c450ca0bad38b1/docs/adr/0026-complementary-history-views.md)
- [ADR 0027 (removed)](https://github.com/MichinaoShimizu/kiroku/blob/27e8298bdc0367afc623bc4db5c450ca0bad38b1/docs/adr/0027-project-view.md)
