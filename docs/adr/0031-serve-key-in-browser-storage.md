# ADR 0031: The browser keeps the serve key in origin-scoped storage and sends it in a header

Status: Accepted (2026-10-14, approved by the maintainer during the quality audit)
Date: 2026-10-14
Scope: How a browser proves it has the `kiroku serve` key (`internal/cli/servekey.go`, `internal/cli/serve.go`, `internal/cli/autostart.go` probe, `internal/web` boot/live/session/panels/ai scripts and `loading.html`, `SECURITY.md`, `docs/guide.md`)
Related ADRs: [ADR 0022](0022-history-stays-local.md) (point 5: serve requires its key), [ADR 0023](0023-compatibility-contract.md) (the serve key is in the compatibility contract)
Related Decision Logs: [DL 0003 — local privacy boundary](../decision-log/0003-local-privacy-boundary.md)
Supersedes/Amends: Amends [ADR 0022](0022-history-stays-local.md) point 5 in detail only (how the key reaches the server); the rule that `kiroku serve` requires its key is unchanged
Decision owner: Repository maintainer

## Context

`kiroku serve` shows history only to a browser that has its key (ADR 0022, point 5). Until now the browser got the key from `/?key=K`, and kiroku stored it in a cookie named `kiroku_key_<port>` (`HttpOnly`, `SameSite=Strict`). A hostile test confirmed the cookie leaks:

- Browsers don't separate cookies by port. A cookie set by `localhost:8484` is sent to every other port of `localhost`.
- `SameSite=Strict` only stops cross-site requests. Another port of the same host is the same site, so the cookie is sent when the user types that address, follows a link to it from the kiroku view, or when the browser fetches its favicon.
- Another user of the same computer can run a server on any free port. Once they receive the cookie, they can replay it to `kiroku serve` and read all history. Any bearer cookie can be replayed this way, so changing its attributes doesn't fix the problem.

Navigation requests (typing an address, a bookmark, a link) can't carry a custom header. So the plain `/` address, which bookmarks rely on, can't require the key if the key isn't in a cookie.

## Decision

1. **The key lives in the browser's `localStorage` under `kiroku:key`.** `localStorage` is scoped to the origin (scheme, host and port), so it isn't shared with other ports, and the browser never sends it anywhere by itself.
2. **The key is sent only in an `X-Kiroku-Key` header and compared in constant time.** Every endpoint that returns history or changes state requires it: `/data.json`, `/stamp`, `/progress`, `/prompt`, `/reply`, `/history`, `/archive`, `/ai/run`. Cookies are not accepted. A `kiroku_key_<port>` cookie left by an older version is expired (`Max-Age` < 0) whenever a browser sends one.
3. **`GET /?key=K` with the right key returns a hop page.** The page doesn't contain the key. Its one fixed inline script, allowed by a CSP hash, reads the key from `location.search`, checks that it is 64 hex digits, stores it, and calls `location.replace` with the same path minus `key`, so the key stays out of the address bar and the browser's history. A wrong key gets the 403 locked page. `kiroku open` (`open.html` → `/?key=`) is unchanged.
4. **`GET /` doesn't require the key and contains no history.** When kiroku is ready, `/` is the view rendered with empty data. Its `META` holds only non-history flags (`shell: true`, `aiRun`, `aiAgents`, `demo`). The script reads the stored key, fetches `data.json` with the header, applies it, and only then does the first render (choosing the week and so on). With no key, or on a 403, it shows the locked message in the page. While kiroku is still reading, `/` stays the loading page, which polls with the header and shows the locked message on a 403. The saved HTML file (`kiroku html`) is unchanged: its data is embedded and it makes no requests.
5. **The history file link can't be a plain link any more.** The view fetches `/history?id=` with the header and saves the response through a `blob:` URL named after the history file. The CSP doesn't restrict downloads; `connect-src 'self'` already allows the fetch.
6. **All requests from the view go through one helper (`kfetch`).** It sends only to the page's own origin.

The Host check, the `127.0.0.1` default, `X-Kiroku` plus the same-origin check on POSTs, and the security headers (`Referrer-Policy: no-referrer` among them) stay as they are.

## Alternatives

- **Keep a cookie with other attributes** (`__Host-` prefix, `Secure`, a path). None of these separate ports on `http://localhost`, and the cookie stays a replayable bearer token. Not chosen.
- **A cookie bound to something only the browser knows** (for example a session cookie plus a per-page token). It's more machinery than the header, and the cookie half still goes to other ports. Not chosen.
- **Put the key in every URL** (`?key=` on all links and fetches). It leaks through the address bar, browser history, logs and copied links. Not chosen.
- **`sessionStorage` instead of `localStorage`.** It's per tab, so bookmarks and new tabs would need `kiroku open` every time. Not chosen.
- **A Unix socket or a per-user port.** The browser can't use a Unix socket, and a port doesn't prove which user owns it. Not considered further.

## Consequences

- The cookie leak to other local ports is closed: nothing the browser sends on its own carries the key.
- `localStorage` can be read by script on the same origin, whereas the old cookie was `HttpOnly`. The view's CSP allows only its own inline script (by hash), and history is always escaped (ADR 0022, point 3), so an XSS in the view would be needed. That's the same boundary that already protects the history itself.
- As before, the key doesn't protect against another user who listens on the same port while `kiroku serve` is stopped: a page served there is the same origin and can read the stored key, just as it would have received the cookie.
- `localhost:8484` and `127.0.0.1:8484` are different origins, so each needs `kiroku open` once. The same was true for the cookie, which was scoped by host.
- The view now makes one extra request before its first render. The page itself no longer embeds the data, so the total transferred is about the same.
- **Compatibility (ADR 0023):** the key file and `kiroku open` are unchanged, and bookmarks keep working once the browser has the key. A browser that only has the old cookie sees "Open kiroku with its key" until the user runs `kiroku open` once after upgrading. Under ADR 0023 point 4 this is marked `BREAKING` in `CHANGELOG.md`. `/data.json` remains internal.
- Revisit if browsers start separating cookies by port, or if kiroku ever serves over HTTPS with a per-user origin.

## Verification

- `internal/cli`: `TestRequireKey` (header required; the old cookie alone gets 403 and is expired; `?key=` returns the hop page without the key and sets no key cookie; the hop page's CSP hash matches its script), `TestServeShellHasNoHistory` (`GET /` without the key contains no synthetic title or session ID; `data.json`, `stamp` and `prompt` without the header get 403), `TestServeLive`, `TestProbeSendsKey`.
- e2e: `tools/screenshots/serve.mjs` (run by `sh tools/check.sh --e2e` and the CI e2e job) starts `kiroku serve` on synthetic history with a throwaway `HOME` and config folder. It opens `/?key=`, reloads `/` without the key, saves a history file, checks that a fresh browser context sees only the locked message, and checks that a second local HTTP server on another port receives no key in any request after the view navigates to it. Run against `main` before this change (2026-10-14), the cookie checks and the cross-port leak check fail, and so do the new cases in `TestRequireKey`, `TestServeShellHasNoHistory` and `TestServeLive`.

## Open questions

None for acceptance.
