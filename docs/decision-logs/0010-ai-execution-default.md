---
kind: log
category: security
keywords: [--ai, --no-ai, KIROKU_NO_AI, /ai/run, aiAddrOK, aiStartLine, AI_SENDS, internal/ai]
---

# [kiroku] "Analyze with AI" went from effectively on, to opt-in with --ai, to on by default with --no-ai

> A past observation, not a decision. It binds nothing.

## What happened

- C1 2026-10-13 Replaced: `kiroku serve` gained `POST /ai/run`, which runs `claude`, `codex` or `kiro-cli` with the exact prompt after a confirmation, behind the serve key and same-origin checks, with a 256 KiB prompt, 1 MiB output and 2-minute limit. Despite the PR title ("opt-in"), it was on whenever serve listened on loopback. History: [PR #385](https://github.com/MichinaoShimizu/kiroku/pull/385) / Now: the runner and endpoint remain ([`internal/ai/runner.go` @`1d7b409`](https://github.com/MichinaoShimizu/kiroku/blob/1d7b4091906cc02d4f32bdabd120f5120dcf0792/internal/ai/runner.go)); the on/off rule was replaced by C2 and C4
- C2 2026-10-14 Replaced: AI execution became opt-in with `kiroku serve --ai` (off by default, 403 without it). History: [PR #386](https://github.com/MichinaoShimizu/kiroku/pull/386) / Now: replaced by C4
- C3 2026-10-14 Replaced: the opt-in was recorded as ADR 0030, and the run dialog again said the chosen agent may send the prompt and the history it reads to its provider. History: [PR #411](https://github.com/MichinaoShimizu/kiroku/pull/411) / Now: replaced by C4; the ADR file was removed
- C4 2026-10-14 Still true: "Analyze with AI" is on by default in `kiroku serve`; `--no-ai` or `KIROKU_NO_AI` turns it off (`/ai/run` is then 404), it is always off on an address that isn't a loopback IP, `--ai` is accepted and does nothing, the startup line states the provider disclosure, the run dialog no longer does, and saved HTML never runs an agent. History: [PR #429](https://github.com/MichinaoShimizu/kiroku/pull/429) / Now: [`internal/cli/serve.go` `aiAddrOK` @`1d7b409`](https://github.com/MichinaoShimizu/kiroku/blob/1d7b4091906cc02d4f32bdabd120f5120dcf0792/internal/cli/serve.go), [`SECURITY.md` @`1d7b409`](https://github.com/MichinaoShimizu/kiroku/blob/1d7b4091906cc02d4f32bdabd120f5120dcf0792/SECURITY.md), [`docs/compatibility.md` @`1d7b409`](https://github.com/MichinaoShimizu/kiroku/blob/1d7b4091906cc02d4f32bdabd120f5120dcf0792/docs/compatibility.md)

## Reason

Documented (description): #385 keeps the copy-prompt workflow and adds local CLI execution with consent per run; its draft ADR compares copy-only, a provider API with credentials, and a local CLI adapter (chosen) ([PR #385](https://github.com/MichinaoShimizu/kiroku/pull/385)).

Documented (description, AI): #386 calls the loopback default a regression against the rule that widening access is opt-in ([PR #386](https://github.com/MichinaoShimizu/kiroku/pull/386)); #429 says "The maintainer decided on 2026-10-14 that `kiroku serve` offers 'Analyze with AI' by default", because the copy buttons had been removed and the prompts had no other use ([PR #429](https://github.com/MichinaoShimizu/kiroku/pull/429)). Alternatives #429's ADR 0034 lists as not chosen: keep opt-in, bring back the copy buttons, keep the provider sentence in the dialog, make `--ai` an error.

## People involved

From the PR records; not necessarily who decided.

- PR #385: opened @MichinaoShimizu (2026-10-13), no approving review, merged @MichinaoShimizu (2026-10-13)
- PR #386: opened @MichinaoShimizu (2026-10-14), no approving review, merged @MichinaoShimizu (2026-10-14)
- PR #411: opened @MichinaoShimizu (2026-10-14), no approving review, merged @MichinaoShimizu (2026-10-14)
- PR #429: opened @MichinaoShimizu (2026-10-14), no approving review, merged @MichinaoShimizu (2026-10-14)

## Open questions

- Was dropping `--ai` inside #385 (on by default on loopback) deliberate, given that #386 reverted it the next day and #429 restored a default-on? (ask @MichinaoShimizu)
- CLAUDE.md says anything that widens what serve exposes must be opt-in; only SECURITY.md and docs/compatibility.md now record this exception. Is that the intended place? (ask @MichinaoShimizu)
- Was moving the provider disclosure from the dialog to the startup line a deliberate trade-off? (ask @MichinaoShimizu)
- Do the reasons above, written by an AI agent in the PR descriptions, match the maintainer's own judgement? (ask @MichinaoShimizu)

## Sources

- [ADR 0030 (removed)](https://github.com/MichinaoShimizu/kiroku/blob/27e8298bdc0367afc623bc4db5c450ca0bad38b1/docs/adr/0030-opt-in-local-agent-execution.md)
- [ADR 0034 (removed)](https://github.com/MichinaoShimizu/kiroku/blob/27e8298bdc0367afc623bc4db5c450ca0bad38b1/docs/adr/0034-ai-analysis-by-default.md)

## Related logs

- DL-11
- DL-12
- DL-1
