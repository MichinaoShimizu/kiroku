# ADR 0029: Offer the view in Japanese through a message catalog, following the browser's language

Status: Accepted (2026-10-14, the maintainer asked to make the view's language switchable)
Date: 2026-10-14
Scope: `internal/web` (view text, dates, numbers), `internal/cli` (language option for `html` and `serve`), `docs/guide.md` and README.ja.md
Related ADRs: [0022](0022-history-stays-local.md) (history stays local), [0023](0023-compatibility-contract.md) (compatibility)
Related Decision Logs: 0018 (English product UI and Japanese entry documentation)
Supersedes/Amends: none. It changes the current behavior that Decision Log 0018 records; that log is a historical observation, not an accepted decision.
Decision owner: maintainer

## Context

kiroku is being prepared for an internal rollout at Japanese companies. The rollout needs Japanese review documents and install guides, which are documentation rather than a design decision. It also needs a Japanese view: many intended users read English slowly, and the view's labels, tooltips, "?" explanations and dialogs are where they decide what a number means and what a button sends.

Evidence:

- **Documented:** [#46](https://github.com/MichinaoShimizu/kiroku/pull/46) added a bilingual view with `tr(ja, en)` pairs, locale detection and a language switch.
- **Documented:** [#110](https://github.com/MichinaoShimizu/kiroku/pull/110) then removed it on purpose. It made the view, `HELP` and documents English only, renamed Go-generated names to English (`(untitled)`, `Kiro IDE (legacy)`, and so on), and kept Japanese prompt parsing and correction detection. The PR describes what changed but not why. The reason is **Unknown**. Likely costs, which are **Inferred** and not recorded: every UI change had to be written twice, inline `tr()` pairs made templates hard to read, and the pairs could drift apart.
- **Observed:**
  - `internal/web/js` is about 2,700 lines in 17 files, with English strings inline in template literals.
  - `LOC()` returns `"en-US"` in `state.js`.
  - `hm()` formats with `en-US`.
  - The Go side still produces both `why` (Japanese) and `whyEn` for friction reasons (`internal/report/week.go`).
- **Observed:** `template.html` has no `translate="no"`. Users who can't read English may turn on the browser's page translation, which can send the page text to the browser vendor's translation service. That text is history: titles, prompts, file paths. This conflicts with ADR 0022's intent that history doesn't leave the machine.

Constraints:

- ADR 0022: no new network access. Translation must be static text shipped in the binary, with no runtime service.
- ADR 0023: `kiroku json` fields and values, CLI names and `/data.json` semantics must not change with the language. `source` values such as "Kiro IDE / CLI" stay English.
- History is untrusted. Any new interpolation must keep escaping history values; catalog entries are fixed text, not HTML.

## Decision

1. **Message catalog, not inline pairs.**
   - `internal/web/js/i18n.js` holds `t(s, vars)` and one catalog, `JA_TEXT`. The key is the English sentence itself (gettext style), so templates keep reading as English and the English output stays byte-for-byte the same.
   - `{name}` placeholders take values. Whole sentences are keys, because Japanese word order differs.
   - `t` returns plain text. Callers escape it, and any history values filled into it, with `esc()` when building HTML, or use `textContent`.
   - Catalog entries contain no markup. Markup stays in the templates.
   - A sentence without a Japanese entry is shown in English, so an untranslated string never blocks a release.
   - Static text in `template.html` is marked with `data-t` and translated once at start-up. Dynamic areas, which hold history, are never looked up.
2. **Follow the browser's language, with English as the fallback.** The language is chosen by, in order:
   - `?lang=` in the address;
   - the viewer's choice from the button at the top right, stored in `localStorage` (a convenience; switching also reloads with `?lang=`, so it works without storage);
   - `kiroku html --lang ja` and `kiroku serve --lang ja`, also settable with `KIROKU_LANG` (`kiroku autostart` keeps it);
   - the browser's preferred language (`navigator.languages[0]`): Japanese if it starts with `ja`;
   - otherwise English.

   The maintainer chose to follow the browser's language (2026-10-14) over always starting in English. Screenshots and docs stay in English: `tools/screenshots` and the e2e tests run the browser with the `en-US` locale.
3. **What gets translated:**
   - **Phase 1:** the view's labels, buttons, tooltips, `aria-label`s, dialogs, toasts, empty states, and the "?" metric explanations (`HELP` and the JS help tables).
   - **Also in Phase 1:** dates and numbers. `LOC()` and `hm()` follow the language, with Japanese date forms such as `10月14日(水)`.
   - **Not translated:**
     - Go-generated names and `kiroku json` values (ADR 0023)
     - CLI help and terminal output
     - `docs/guide.md`, which stays English, with README.ja.md as the Japanese entry point
     - the AI prompts, which already ask the agent to answer in the language the person writes prompts in, and whose output quality is measured by `report-eval` in English
4. **Keep the view's text out of browser translation.** Add `translate="no"` to the page, so the browser's built-in translation doesn't send history text to a translation service. The Japanese catalog is the supported way to read the view in Japanese. Document this in SECURITY.md and the Japanese security review.
5. **Tests guard the two catalogs from drifting:**
   - **Catalog check (`TestI18nCatalog`):** no `<` or `>` in entries, the same `{placeholders}` as the English key, and `t()` lookup and fallback work. Keys used in `t("…")` without a Japanese entry are listed but don't fail.
   - **e2e:** runs the existing four widths in `ja` too, since Japanese labels wrap differently.

## Alternatives

- **Do nothing (English view, Japanese documents only).** The cheapest option, and it keeps the reason for #110. It leaves the rollout's main readers with an English view, and they will turn to browser translation, which leaks history (see Context). If chosen, item 4 (`translate="no"`) should still be done on its own.
- **Bring back #46's inline `tr(ja, en)` pairs.** Fast to write, but it brings back the likely reasons #110 removed them: noisy templates and duplicated edits. It also has no fallback or parity check.
- **Rely on browser translation.** No cost to kiroku, but it sends history to a third-party service, conflicting with ADR 0022, and translates domain terms inconsistently. Rejected.
- **Japanese only for a company build (a fork).** It avoids changing upstream, but the fork has to be maintained and would drift from security fixes. Not preferred.
- **Not considered in depth:** pluralization and ICU message formatting libraries. A small hand-written formatter for the few plural cases (`plural()`) is assumed to be enough. Revisit if more languages are added.

## Consequences

- **Benefits:** Japanese readers can use the view without a third-party translator. Domain terms are translated once and consistently. The view becomes safer against accidental leaks through browser translation for everyone.
- **Costs:**
  - Every UI change needs a catalog key, and ideally a Japanese entry, roughly the doubled work #110 removed. This is mitigated by the English fallback and the parity test, but it doesn't go away.
  - Moving about 2,700 lines of templates to `t()` is a large, mechanical change with merge-conflict risk.
  - Japanese entries need a native reviewer.
  - Screenshots and `docs/usability.md` scenarios need a `ja` pass.
- **Security:**
  - `t()` must never return or accept HTML. A catalog entry that contains `<`, or `{var}` placeholders filled with unescaped history, would be an XSS path. The parity test also checks that entries contain no `<`.
  - `translate="no"` reduces exposure.
- **Compatibility:** no change to `kiroku json`, `/data.json`, CLI names or archive. `--lang` and `KIROKU_LANG` are additions.
- **Reversibility:** high. Removing the `ja` catalog and the option returns to English-only without touching data.

## Verification

- Unit tests (Node, as in `internal/web/script_test.go`):
  - catalog parity and no `<` in entries;
  - `t()` with missing keys and variables;
  - a template given a history value containing `<img onerror>` renders it inert in both languages.
- e2e at 1440, 1000, 390 and 320 widths in `en` and `ja`, with no horizontal overflow.
- `hostile-tester` on the language switch and the catalog interpolation. `user-tester` on `docs/usability.md` scenarios in Japanese.
- Acceptance: the maintainer accepts this ADR. Phase 1 merges only after the tests above exist and pass.

## Open questions

- Why did #110 remove the bilingual view? If the reason is something other than maintenance cost (for example, a product focus on English-speaking users), it may change this decision.
- Should `kiroku stats` and the CLI help get Japanese later, or stay English like `kiroku json`?
- Who reviews Japanese terminology for metrics (for example, "Active time", "Worth a look", "Corrections")?
- Should `translate="no"` (item 4) ship ahead of the rest, as a standalone security change?
