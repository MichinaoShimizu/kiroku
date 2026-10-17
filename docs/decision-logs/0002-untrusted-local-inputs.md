---
kind: log
category: security
keywords: [writePrivate, safeName, gitCmd, GIT_CONFIG_NOSYSTEM, core.fsmonitor, core.hooksPath, secureHeaders, checkRedirect, maxBinary, .kiroku-archive, OpenRegular]
---

# [kiroku] kiroku treats history, git and its own downloads as untrusted local input

> A past observation, not a decision. It binds nothing.

## What happened

- C1 2026-10-06 Still true: `html` and `json` output is written by `writePrivate` (0600 temp file, then rename, so a symlink at the destination is replaced, not followed). History: [PR #161](https://github.com/MichinaoShimizu/kiroku/pull/161) / Now: [`internal/cli/cli.go` `writePrivate` @`1d7b409`](https://github.com/MichinaoShimizu/kiroku/blob/1d7b4091906cc02d4f32bdabd120f5120dcf0792/internal/cli/cli.go)
- C2 2026-10-06 Still true: `/history` serves only files that resolve under the watched history roots or the archive, and unsafe Kiro IDE (legacy) session IDs are rejected. History: [PR #161](https://github.com/MichinaoShimizu/kiroku/pull/161) / Now: [`internal/cli/serve.go` `allowed` @`1d7b409`](https://github.com/MichinaoShimizu/kiroku/blob/1d7b4091906cc02d4f32bdabd120f5120dcf0792/internal/cli/serve.go), [`internal/source/source.go` `safeName` @`1d7b409`](https://github.com/MichinaoShimizu/kiroku/blob/1d7b4091906cc02d4f32bdabd120f5120dcf0792/internal/source/source.go)
- C3 2026-10-06 Still true: git runs with `GIT_CONFIG_NOSYSTEM=1`, no terminal prompt, no optional locks, `core.fsmonitor=false`, an empty hooks path, `core.pager=cat` and `--no-ext-diff --no-textconv`; the global config is kept. History: [PR #161](https://github.com/MichinaoShimizu/kiroku/pull/161) / Now: [`internal/gitlog/gitlog.go` `gitCmd` @`1d7b409`](https://github.com/MichinaoShimizu/kiroku/blob/1d7b4091906cc02d4f32bdabd120f5120dcf0792/internal/gitlog/gitlog.go)
- C4 2026-10-06 Still true: serve sets header and idle timeouts, a 64 KiB header limit and security headers on every response, and error bodies don't show local paths; `kiroku update` accepts only valid tags, https redirects and binaries up to 200 MiB; `archive off` deletes only `.zst` copies in a folder marked `.kiroku-archive`. History: [PR #161](https://github.com/MichinaoShimizu/kiroku/pull/161) / Now: [`internal/cli/serve.go` `secureHeaders` @`1d7b409`](https://github.com/MichinaoShimizu/kiroku/blob/1d7b4091906cc02d4f32bdabd120f5120dcf0792/internal/cli/serve.go), [`internal/cli/update.go` `checkRedirect` @`1d7b409`](https://github.com/MichinaoShimizu/kiroku/blob/1d7b4091906cc02d4f32bdabd120f5120dcf0792/internal/cli/update.go), [`internal/archive/archive.go` `Clear` @`1d7b409`](https://github.com/MichinaoShimizu/kiroku/blob/1d7b4091906cc02d4f32bdabd120f5120dcf0792/internal/archive/archive.go)
- C5 2026-10-11 Still true: history files are opened only if they are regular files (a FIFO made kiroku block forever). History: [PR #352](https://github.com/MichinaoShimizu/kiroku/pull/352) / Now: [`internal/core/open.go` `OpenRegular` @`1d7b409`](https://github.com/MichinaoShimizu/kiroku/blob/1d7b4091906cc02d4f32bdabd120f5120dcf0792/internal/core/open.go)

## Reason

Documented (description, AI): each item names its threat. Output was 0644 and followed symlinks; a session ID like `../../outside/creds` made `/history` serve any `.json`; repositories named by history can run fsmonitor, hook, textconv, ext-diff, pager or gpg programs; `archive off` used to remove whole folders. The global git config is kept because it holds `user.email` and `safe.directory`; there is no write timeout because history files can be large ([PR #161](https://github.com/MichinaoShimizu/kiroku/pull/161), [PR #352](https://github.com/MichinaoShimizu/kiroku/pull/352)).

## People involved

From the PR records; not necessarily who decided.

- PR #161: opened @MichinaoShimizu (2026-10-06), no approving review, merged @MichinaoShimizu (2026-10-06)
- PR #352: opened @MichinaoShimizu (2026-10-11), no approving review, merged @MichinaoShimizu (2026-10-11)

## Open questions

- #161 fixes "the Go-side findings from the security audit". Where is that audit and its full list of findings? (ask @MichinaoShimizu)
- Do the reasons above, written by an AI agent in the PR descriptions, match the maintainer's own judgement? (ask @MichinaoShimizu)
