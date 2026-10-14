# ADR 0033: The view is the calendar only

Status: Accepted (2026-10-14, approved by the maintainer); amended 2026-10-14 (Year in review removed, see [Amendment](#amendment-2026-10-14-year-in-review-removed))
Date: 2026-10-14
Scope: Local HTML history exploration and view navigation (`internal/web`: `template.html`, `js/state.js`, `js/events.js`, `js/calendar.js`, `js/summary.js`, `style.css`)
Related ADRs: [ADR 0022](0022-history-stays-local.md), [ADR 0023](0023-compatibility-contract.md), [ADR 0029](0029-japanese-ui.md)
Related Decision Logs: [DL 0016](../decision-log/0016-single-file-rendering.md)
Supersedes/Amends: Supersedes [ADR 0026](0026-complementary-history-views.md), [ADR 0027](0027-project-view.md) and [ADR 0028](0028-knowledge-discovery.md)
Decision owner: Repository maintainer

## Context

ADR 0026 added a View switch next to the Week / Month period, with a Journey view (sessions, commits and pull requests in project lanes by day). ADR 0027 added a third, Project view (one card per project with its sessions, commits and recorded pull requests). ADR 0028 accepted Phase 1 of knowledge discovery: a "Copy knowledge prompt" button on each card under "① By project" in the weekly and monthly summary, with later phases still Proposed.

On 2026-10-14 the maintainer decided to simplify kiroku. The three views answer overlapping questions over the same records: the week and month calendar already shows when and in which project work happened, the legend filters by project, and "① By project" in the summary lists time, usage and commits per project. Keeping three views costs UI states, keyboard and accessibility work, responsive layout, translations, e2e steps and screenshots for each. Knowledge discovery Phase 1 was one more copy-only prompt next to the report prompts, and its later phases (local storage, review UI) were never started.

## Decision

1. **The view is the calendar only**: the Week and Month calendar, with search, the legend's filters, the session details panel, the weekly and monthly summary, "Worth a look" and the Year in review. There is no View switch; the Period switch (Week / Month) and the Year in review stay.
2. **Journey and Project are removed**, with their markup, state, CSS, catalog strings, e2e steps and docs.
3. **Knowledge discovery is removed**: the "Copy knowledge prompt" button and its prompt. Later phases of ADR 0028 are not pursued.
4. **Old saved state falls back to the calendar.** A browser that saved `view` as `journey` or `project` (`localStorage` key `kiroku:view`) opens the calendar; the stored value is ignored. No URL ever named a view.
5. **No contract change.** The views were never part of `kiroku json`, a CLI option or an environment variable; under [ADR 0023](0023-compatibility-contract.md) point 2 the view may change in any release, so this is not BREAKING.

## Alternatives

- **Keep Journey and Project (ADR 0026, 0027).** More ways to look at the same records, at the cost above. Not chosen by the maintainer.
- **Keep only Project as a summary card list.** "① By project" in the summary already covers time, usage and commits per project. Not chosen.
- **Keep the knowledge prompt on the summary's project cards.** It is independent of the views, but it was the only project-scoped prompt and its later phases were not planned. Not chosen.

## Consequences

- Fewer UI states, less CSS and fewer catalog strings, e2e steps and screenshots to maintain.
- Users who used Journey or Project lose those layouts; the calendar, legend filters, search and "① By project" remain.
- ADR 0022 is unaffected: nothing new leaves the machine.
- Reversible: the views can be restored from history (`git log` before this change) without migrating data.
- Revisit if users need a project-first view that the calendar and the summary can't give, and justify it as a distinct question (as ADR 0026 required).

## Verification

- `sh tools/check.sh` and `sh tools/check.sh --e2e`: `smoke.mjs` checks that no View switch exists, that the region is labeled "Calendar", and that a saved `view` of `journey` or `project` opens the calendar.
- `TestI18nCatalog` passes with the removed strings gone from the catalog.

## Open questions

None.

## Amendment (2026-10-14): Year in review removed

On 2026-10-14 the maintainer also decided to remove the Year in review "for now". Point 1 no longer keeps it: the view is the Week and Month calendar, with search, the legend's filters, the session details panel, the weekly and monthly summary and "Worth a look".

- Removed: the "Year in review" button, the `Y` shortcut and its row in the shortcut list, the year dialog and its share image ("Save as PNG", "Copy image"), `js/year.js`, its CSS and catalog strings, its tests, e2e steps and screenshots (`docs/year.png`, `docs/year-params.png`).
- Old saved state falls back to the calendar: the stored Year in review options (`localStorage` key `kiroku:yrOpt`) are ignored, and `Y` does nothing. No URL ever opened it.
- No contract change, as in point 5: the Year in review was never part of `kiroku json`, a CLI option or an environment variable, so under [ADR 0023](0023-compatibility-contract.md) point 2 this is not BREAKING. ADR 0022 is unaffected; the share image was made in the browser and sent nowhere.
- Reversible from history (`git log` before this change). Revisit if a yearly look back is wanted again.
- Verification: `smoke.mjs` checks that no Year in review button or dialog exists, that the shortcut list has no `Y`, and that a saved `yrOpt` still opens the calendar with `Y` opening nothing.
