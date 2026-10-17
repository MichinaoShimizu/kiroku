---
kind: log
category: security
keywords: [sameOrigin, allow-host, requireKey, serve-key, X-Kiroku-Key, kiroku:key, kiroku open, localNames]
---

# [kiroku] Who may read history through kiroku serve: loopback, then the Host check, then a key, then the key out of cookies

> A past observation, not a decision. It binds nothing.

## What happened

- C1 2026-10-03 Replaced: `kiroku --serve` served the view live on `127.0.0.1:8484` and checked Host (`sameOrigin`) only when listening on loopback; on any other address the check was skipped. History: [PR #11](https://github.com/MichinaoShimizu/kiroku/pull/11) / Now: [PR #127](https://github.com/MichinaoShimizu/kiroku/pull/127) checks Host on every address, and [PR #208](https://github.com/MichinaoShimizu/kiroku/pull/208) removed the `--serve` form
- C2 2026-10-06 Still true: the Host check also runs when serve listens on `0.0.0.0` or a LAN IP. It allows localhost, the loopback IPs, the bound address and, on an all-interfaces address, this computer's interface IPs and host name (looked up per request); `--allow-host` adds names. Anything else gets 403. History: [PR #127](https://github.com/MichinaoShimizu/kiroku/pull/127) / Now: [`internal/cli/serve.go` `sameOrigin` @`1d7b409`](https://github.com/MichinaoShimizu/kiroku/blob/1d7b4091906cc02d4f32bdabd120f5120dcf0792/internal/cli/serve.go)
- C3 2026-10-06 Replaced: serve required a random key kept in a 0600 `serve-key` file under the config folder; `kiroku open` hands it to the browser through a 0600 page so it never appears on a command line, and the browser kept it in a `kiroku_key_<port>` cookie (HttpOnly, SameSite=Strict). History: [PR #172](https://github.com/MichinaoShimizu/kiroku/pull/172) / Now: the key file and `kiroku open` remain ([`internal/cli/servekey.go` `serveKey` @`1d7b409`](https://github.com/MichinaoShimizu/kiroku/blob/1d7b4091906cc02d4f32bdabd120f5120dcf0792/internal/cli/servekey.go)); the cookie was replaced by C4
- C4 2026-10-14 Still true: the browser keeps the key in origin-scoped `localStorage` (`kiroku:key`) and sends it as `X-Kiroku-Key`; cookies are no longer accepted and an old one is expired. `GET /` without the key returns a shell with no history, and `?key=` goes through a hop page that stores the key and drops it from the URL. Marked BREAKING (a browser with only the old cookie needs `kiroku open` once). History: [PR #414](https://github.com/MichinaoShimizu/kiroku/pull/414) / Now: [`internal/cli/servekey.go` `requireKey` @`1d7b409`](https://github.com/MichinaoShimizu/kiroku/blob/1d7b4091906cc02d4f32bdabd120f5120dcf0792/internal/cli/servekey.go), [`SECURITY.md` @`1d7b409`](https://github.com/MichinaoShimizu/kiroku/blob/1d7b4091906cc02d4f32bdabd120f5120dcf0792/SECURITY.md)

## Reason

Documented (description, AI): each step closed a gap the previous one left. #11 used loopback plus a Host check against DNS rebinding. #127: an exposed serve accepted any Host, so a page could rebind its domain to the machine and read `/data.json` and `/history`; LAN viewing by IP or host name was kept on purpose. #172: 127.0.0.1 keeps other computers out but not other users of the same computer, and history files were already 0600, so serve was the gap. #414: browsers don't separate cookies by port and `SameSite=Strict` doesn't block same-site requests, so the cookie reached other localhost servers (confirmed by a hostile test) ([PR #127](https://github.com/MichinaoShimizu/kiroku/pull/127), [PR #172](https://github.com/MichinaoShimizu/kiroku/pull/172), [PR #414](https://github.com/MichinaoShimizu/kiroku/pull/414)).

Alternatives #414 lists as not chosen: other cookie attributes (`__Host-`, `Secure`, path), a cookie bound to a per-page token, the key in every URL, `sessionStorage`, a Unix socket or per-user port. Residual risks it states: same-origin script can read `localStorage`, and a process squatting the port while serve is stopped can read the stored key.

## People involved

From the PR records; not necessarily who decided.

- PR #11: opened @MichinaoShimizu (2026-10-03), no approving review, merged @MichinaoShimizu (2026-10-03)
- PR #127: opened @MichinaoShimizu (2026-10-06), no approving review, merged @MichinaoShimizu (2026-10-06)
- PR #172: opened @MichinaoShimizu (2026-10-06), no approving review, merged @MichinaoShimizu (2026-10-06)
- PR #414: opened @MichinaoShimizu (2026-10-14), no approving review, merged @MichinaoShimizu (2026-10-14)
- The CodeQL "Secure cookie" alerts on #172 and #414 were answered by the author's agent ("Leaving this as is, on purpose"), not by another person.

## Open questions

- Was skipping the Host check on a non-loopback listener in #11 a deliberate trust decision or an oversight? (ask @MichinaoShimizu)
- Was the cookie leak to other localhost ports, found in #414, known and accepted at the time of #172? (ask @MichinaoShimizu)
- Where was ADR 0031 approved "during the quality audit", as its status line says? (ask @MichinaoShimizu)
- Do the reasons above, written by an AI agent in the PR descriptions, match the maintainer's own judgement? (ask @MichinaoShimizu)

## Sources

- [ADR 0031 (removed)](https://github.com/MichinaoShimizu/kiroku/blob/27e8298bdc0367afc623bc4db5c450ca0bad38b1/docs/adr/0031-serve-key-in-browser-storage.md)
- [ADR 0022 (removed)](https://github.com/MichinaoShimizu/kiroku/blob/27e8298bdc0367afc623bc4db5c450ca0bad38b1/docs/adr/0022-history-stays-local.md)
