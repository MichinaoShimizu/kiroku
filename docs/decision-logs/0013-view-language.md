---
kind: log
category: product
keywords: [i18n.js, JA_TEXT, data-t, inEnglish, --lang, KIROKU_LANG, translate="no", tr(]
---

# [kiroku] The view went bilingual, then English only, then Japanese again through a message catalog

> A past observation, not a decision. It binds nothing.

## What happened

- C1 2026-10-04 Replaced: the view and command output became bilingual, with Japanese and English written side by side (`tr(ja, en)`) and the language taken from the browser. History: [PR #46](https://github.com/MichinaoShimizu/kiroku/pull/46) / Now: replaced by C2, then C3
- C2 2026-10-05 Replaced: the view and docs became English only; the language switch, the Japanese help, README.ja.md and the Japanese guide were removed, and names made in Go became English. History: [PR #110](https://github.com/MichinaoShimizu/kiroku/pull/110) / Now: the English names in Go remain; English only was replaced by C3
- C3 2026-10-14 Still true: the view can be Japanese through a catalog keyed by the English sentence (a sentence with no entry stays English), chosen by `?lang=`, the viewer's choice, `--lang` or `KIROKU_LANG`, then the browser's language; AI prompts, `kiroku json`, `/data.json` and CLI output stay English; the page sets `translate="no"`. History: [PR #389](https://github.com/MichinaoShimizu/kiroku/pull/389) / Now: [`internal/web/js/i18n.js` @`1d7b409`](https://github.com/MichinaoShimizu/kiroku/blob/1d7b4091906cc02d4f32bdabd120f5120dcf0792/internal/web/js/i18n.js), [`internal/web/js/state.js` @`1d7b409`](https://github.com/MichinaoShimizu/kiroku/blob/1d7b4091906cc02d4f32bdabd120f5120dcf0792/internal/web/js/state.js), [`internal/cli/cli.go` `langFlag` @`1d7b409`](https://github.com/MichinaoShimizu/kiroku/blob/1d7b4091906cc02d4f32bdabd120f5120dcf0792/internal/cli/cli.go)

## Reason

Documented (description, AI) for C1 and C3: #46 aimed to spread kiroku overseas while keeping the Japanese view ([PR #46](https://github.com/MichinaoShimizu/kiroku/pull/46)); #389 says the maintainer asked for a switchable view that follows the browser, for use at Japanese companies, and that `translate="no"` keeps browser translation from sending history to a translation service ([PR #389](https://github.com/MichinaoShimizu/kiroku/pull/389)). Not chosen in #389's ADR 0029: bringing back the side-by-side pairs, relying on browser translation, a company fork.

For C2 the reason is unknown: #110 says what changed, not why.

## People involved

From the PR records; not necessarily who decided.

- PR #46: opened @MichinaoShimizu (2026-10-04), no approving review, merged @MichinaoShimizu (2026-10-04)
- PR #110: opened @MichinaoShimizu (2026-10-05), no approving review, merged @MichinaoShimizu (2026-10-05)
- PR #389: opened @MichinaoShimizu (2026-10-14), no approving review, merged @MichinaoShimizu (2026-10-14)

## Open questions

- Why was the bilingual view removed a day after it was added (#110)? (ask @MichinaoShimizu)
- #389's description says it updates SECURITY.md and a Japanese security review, but the merged diff removed that review and left SECURITY.md unchanged. Should the `translate="no"` reason be in SECURITY.md? (ask @MichinaoShimizu)
- Do the reasons above, written by an AI agent in the PR descriptions, match the maintainer's own judgement? (ask @MichinaoShimizu)

## Sources

- [ADR 0029 (removed)](https://github.com/MichinaoShimizu/kiroku/blob/27e8298bdc0367afc623bc4db5c450ca0bad38b1/docs/adr/0029-japanese-ui.md)
- [old Decision Log 0018 (removed)](https://github.com/MichinaoShimizu/kiroku/blob/ef66c137df557123c0f71a342dbf2f82be064131/docs/decision-log/0018-language-policy.md)
