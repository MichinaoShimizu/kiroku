# ADR 0035: The `kiroku serve` view can be installed as an app, without a service worker

Status: Proposed
Date: 2026-10-16
Scope: Opening the `kiroku serve` view in an app window of its own (`internal/web/app.go`, `internal/web/web.go` CSP and `<head>`, `internal/cli/serve.go` `/manifest.webmanifest`, `internal/cli/servekey.go` keyless paths, `SECURITY.md`, the guide)
Related ADRs: [ADR 0022](0022-history-stays-local.md) (history stays local; serve requires its key), [ADR 0023](0023-compatibility-contract.md) (release artifacts), [ADR 0024](0024-verified-releases.md) (verified releases and self-update), [ADR 0031](0031-serve-key-in-browser-storage.md) (the key in `localStorage`; only `GET /` without the key)
Related Decision Logs: [DL 0003 — local privacy boundary](../decision-log/0003-local-privacy-boundary.md)
Supersedes/Amends: Amends [ADR 0031](0031-serve-key-in-browser-storage.md) point 4 in detail only: besides `GET /`, `GET /manifest.webmanifest` answers without the key. It holds no history
Decision owner: Repository maintainer

## Context

A user asked whether kiroku can be a desktop app: a window and an icon of its own in the Dock or taskbar, instead of a browser tab. kiroku is one Go binary built with `CGO_ENABLED=0` for macOS, Linux and Windows by goreleaser. It updates itself (`kiroku update`) after checking `checksums.txt` and provenance (ADR 0024). The view is the page `kiroku serve` sends to the browser, and `kiroku autostart on` already keeps `kiroku serve` running from login.

Chromium-based browsers (Chrome, Edge) can install a site as an app when its page links a web app manifest with a name, a start URL and an icon of at least 144 px. A service worker is no longer required. `http://localhost` and `http://127.0.0.1` count as secure contexts, so plain HTTP on loopback can be installed. The installed app uses the same profile's site data as the browser, so the key that `kiroku open` stores in `localStorage` (ADR 0031) also works in the app window.

Two facts were checked against Chromium 1194 (Playwright's build) during this change:

- With the manifest's icons at `/icon-192.png`, Chromium refused to load them under the view's CSP (`img-src data: blob:`) and reported `no-acceptable-icon`. The page's CSP applies to manifest icons.
- With the icons as `data:` URLs in the manifest and `manifest-src 'self'` in the CSP, `Page.getInstallabilityErrors` reported no errors and the console showed no CSP violations.

## Decision

1. **`kiroku serve` serves a fixed manifest at `/manifest.webmanifest`** (`application/manifest+json`, `no-store`): name `kiroku`, `id`, `start_url` and `scope` `/`, `display: standalone`, and the kiroku icon at 192 and 512 px. The manifest is built from constants only; it holds no history, no settings and no key.
2. **The icons are `data:` PNG URLs inside the manifest**, drawn by the standard library from the same shapes as the view's SVG favicon. The view's `img-src` stays `data: blob:`; it doesn't gain `'self'`.
3. **The live pages (the view and the loading page) link the manifest and add `manifest-src 'self'` to their CSP.** The saved HTML file (`kiroku html`) and the demo get neither.
4. **`GET /manifest.webmanifest` answers without the key**, because browsers fetch the manifest without custom headers. Every other path keeps the key, and the Host check and security headers apply as before. `POST` and any other path under it still get 403.
5. **No service worker.** kiroku doesn't cache the view or history in the browser. The app window shows the view only while `kiroku serve` is running, like a tab. `kiroku autostart on` is the way to keep it available.

## Alternatives

- **A native window around a WebView (Wails, webview/webview).** It gives a real `.app`/`.exe` and could hand the key to the page without `localStorage`. It needs CGO, so the release would need a build per OS instead of goreleaser's cross build, macOS signing and notarization, WebView2 on Windows, and `kiroku update` would have to replace an app bundle. That changes the release artifacts (ADR 0023) and the verified update (ADR 0024). Not chosen now; revisit if a tray icon, notifications or OS integration that a browser can't give become needed.
- **Electron or Tauri.** A second toolchain and a large dependency tree to keep `govulncheck`-clean and pinned, for the same result as the WebView option. Not chosen.
- **Icons at their own keyless paths (`/icon-192.png`).** Needs `img-src 'self'` in the view's CSP, so the page could load images from its own server. Not chosen (see Context).
- **A service worker for offline use.** It would keep the view, and possibly `data.json`, in the browser's cache, which other software running as the user can read after kiroku stops. Not chosen (ADR 0022).
- **Do nothing.** Users can already pin a tab or bookmark `http://localhost:8484/`. It doesn't give a window or Dock/taskbar icon of its own.

## Consequences

- In Chrome and Edge, "Install kiroku" (in the address bar or the menu) opens the view in its own window with the kiroku icon. It opens `/` and reads history with the stored key, so no new key handling is needed.
- The key-less surface grows by one fixed document. It reveals only that kiroku is running on that port, which the locked page and the shell at `/` already reveal.
- The installed app is tied to one origin. `localhost:8484` and `127.0.0.1:8484` are different apps, and changing the port means installing again.
- If `kiroku serve` isn't running, the app window shows the browser's "can't connect" page.
- **Compatibility (ADR 0023):** additive. No option, file or release artifact changes. `/manifest.webmanifest` is not a public interface and may change.
- Reversible: removing the manifest link and route leaves already installed apps as plain windows that the user can uninstall from the browser.

## Verification

- `internal/web`: `TestManifest` (fixed fields, `start_url` and `scope` are `/`, icons are PNG `data:` URLs of the stated size and the favicon's colors), `TestCSP` (live pages link the manifest and allow `manifest-src 'self'`; the saved HTML file has neither).
- `internal/cli`: `TestRequireKey` (`GET /manifest.webmanifest` without the key is 200; `POST`, `/manifest.webmanifest/x` and `/manifest.webmanifest.json` without the key are 403), `TestServeShellHasNoHistory` (the manifest without the key is exactly `web.Manifest()`, `no-store` and `nosniff`; a foreign Host gets 403).
- Manual, 2026-10-16: Chromium 1194 (Playwright) on `kiroku serve`, `Page.getInstallabilityErrors` returned none and no CSP violation was logged. Not verified: installing and using the app window in Chrome or Edge on macOS and Windows; Safari's "Add to Dock".

## Open questions

- Safari's "Add to Dock" (macOS 14+) keeps a web app's site data apart from Safari. Whether the key reaches it, and how a user would give it the key, is unverified. Until then the guide names Chrome and Edge.
- Should the manifest follow `--lang` (a Japanese `description`)? Kept English for now, like JSON and CLI names (ADR 0029).
