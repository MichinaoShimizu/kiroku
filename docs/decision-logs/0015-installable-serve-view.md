---
kind: log
category: architecture
keywords: [manifest.webmanifest, manifest-src, AppPaths, keyless, display: standalone, service worker]
---

# [kiroku] The kiroku serve view can be installed as an app window, without a service worker or a native wrapper

> A past observation, not a decision. It binds nothing.

## What happened

- C1 2026-10-16 Still true: `kiroku serve` answers `/manifest.webmanifest` (fixed content, `display: standalone`, icons as `data:` PNGs) without the key; only live pages link it and allow `manifest-src 'self'`; saved HTML does not. There is no service worker and no Wails or Electron wrapper. History: [PR #445](https://github.com/MichinaoShimizu/kiroku/pull/445) / Now: [`internal/web/app.go` @`1d7b409`](https://github.com/MichinaoShimizu/kiroku/blob/1d7b4091906cc02d4f32bdabd120f5120dcf0792/internal/web/app.go), [`internal/cli/servekey.go` `keyless` @`1d7b409`](https://github.com/MichinaoShimizu/kiroku/blob/1d7b4091906cc02d4f32bdabd120f5120dcf0792/internal/cli/servekey.go), [`SECURITY.md` @`1d7b409`](https://github.com/MichinaoShimizu/kiroku/blob/1d7b4091906cc02d4f32bdabd120f5120dcf0792/SECURITY.md)

## Reason

Documented (description, AI): a user asked whether kiroku can be a desktop app; the icons are `data:` URLs because Chromium checks manifest icons against the page's CSP; no service worker, so the browser keeps no copy of history ([PR #445](https://github.com/MichinaoShimizu/kiroku/pull/445)). Not chosen: Wails or a webview (needs CGO and changes the release and update path), Electron or Tauri, icons at keyless paths, a service worker, doing nothing.

## People involved

From the PR records; not necessarily who decided.

- PR #445: opened @MichinaoShimizu (2026-10-16), no approving review, merged @MichinaoShimizu (2026-10-16)

## Open questions

- Its ADR (0037) was still Proposed when the ADRs were removed. Is the design accepted? (ask @MichinaoShimizu)
- Was installing in Chrome and Edge on macOS and Windows checked by hand, as the PR asks? (ask @MichinaoShimizu)
- Do the reasons above, written by an AI agent in the PR descriptions, match the maintainer's own judgement? (ask @MichinaoShimizu)

## Sources

- [ADR 0037 (removed, Proposed)](https://github.com/MichinaoShimizu/kiroku/blob/27e8298bdc0367afc623bc4db5c450ca0bad38b1/docs/adr/0037-installable-serve-view.md)

## Related logs

- DL-1
