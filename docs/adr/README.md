# Architecture Decision Records

ADRs record explicit design choices; [Decision Logs](../decision-log/README.md) preserve historical observations. See [Decision Continuity](decision-continuity.md) for illustrated workflows.

## Active ADR index

| Topic | Status | ADR |
|---|---|---|
| The view is the calendar only: Week / Month calendar, search, legend filters, session details, summary, Worth a look, Year in review; no View switch, no Journey or Project view, no knowledge prompt; a saved `view` of journey or project opens the calendar; supersedes 0026, 0027, 0028 (`internal/web` template/state/events/calendar/summary; keywords: view, calendar, Journey, Project view, knowledge, view navigation, kiroku:view) | Accepted | [0033](0033-calendar-only-view.md) |
| Opt-in local AI agent execution: only `kiroku serve --ai`, each run confirmed, provider disclosure in the dialog, fixed command lines, least access per agent, bounded runs, reply shown as text; amends 0022 (`internal/ai`, `/ai/run`, `internal/cli` serve, `internal/web/js/ai.js`, `mdview.js`; keywords: AI execution, --ai, Analyze with AI, agent, claude, codex, kiro-cli, provider, disclosure, run) | Accepted | [0030](0030-opt-in-local-agent-execution.md) |
| Remote session import (Phase 2) | Proposed (not accepted) | [0020](0020-remote-session-import.md) |
| Japanese view through a message catalog, following the browser's language; `--lang`/`KIROKU_LANG`; JSON, CLI names and AI prompts stay English; `translate="no"` keeps history out of browser translation (`internal/web`, `internal/cli` html/serve; keywords: i18n, language, Japanese, locale, translation, catalog, LOC, date format) | Accepted | [0029](0029-japanese-ui.md) |
| Kiro CLI V3 apart from V2; V3 shares the Kiro IDE store, shown as "Kiro IDE / CLI"; V2 shown as "Kiro CLI (Classic)" (`internal/source/kiro.go`, `core.Agents`, `core.Records`; keywords: Kiro CLI, V3, KAS, sess_, labels, V2 dedup, kiroku json source) | Accepted | [0021](0021-kiro-cli-v3-separate-source.md) |
| History stays on the machine: no telemetry, only GitHub Releases on the network (except the agent the user runs with `kiroku serve --ai`, ADR 0030); history, git and network input untrusted (escape, no shell, validate paths); 0600/0700 files; serve on 127.0.0.1 with Host check and key, wider access opt-in; private sessions masked (`internal/source`, `internal/gitlog`, `internal/web`, `internal/cli` serve/html/json, `SECURITY.md`; keywords: privacy, telemetry, network, AI API, XSS, escape, shell, path, permissions, serve, listen, Host, key, incognito) | Accepted | [0022](0022-history-stays-local.md) |
| Serve key kept in the browser's origin-scoped `localStorage` and sent as `X-Kiroku-Key`, never a cookie (cookies reach every port of localhost); `/?key=` hop page stores it; keyless `GET /` is a history-free shell that fetches `data.json` with the header; `/history` downloaded via Blob; amends ADR 0022 point 5 detail (`internal/cli/servekey.go`, `internal/cli/serve.go`, `internal/web/js` state/live/boot/session, `loading.html`; keywords: serve, key, cookie, localStorage, header, X-Kiroku-Key, port, bookmark, kiroku open) | Accepted | [0031](0031-serve-key-in-browser-storage.md) |
| User-mediated diagnostics: kiroku never sends diagnostics; `kiroku doctor --report` prints kinds and counts only (kiroku version, OS, per agent sessions, newest version seen, unreadable lines, unknown kinds) for the user to review and paste; names sanitized to `[A-Za-z0-9_.:-]{1,40}`, max 20 per agent, versions digits and dots; no paths, project names, IDs or content (`internal/core/format.go`, `internal/source` claude/codex, `internal/cli/diagnose.go` doctor, `internal/web/js/panels.js`; keywords: diagnostics, doctor, report, format check, unknown kinds, version, bug report, issue) | Accepted | [0032](0032-user-mediated-diagnostics.md) |
| Compatibility within a major version: CLI, exit status, `kiroku json` schemaVersion 1 fields, `--prices`, archive copies, serve key, release artifacts; `/data.json` not public; 0.x BREAKING allowed (`internal/cli`, `internal/archive`, `docs/compatibility.md`, `testdata/compat/`; keywords: breaking, rename, option, flag, env var, JSON field, schema) | Accepted | [0023](0023-compatibility-contract.md) |
| Releases verified before replacing (checksums.txt, provenance when available); GitHub Releases only; Actions pinned to SHA with least permissions; govulncheck clean (`internal/cli/update.go`, `install.sh`, `.github/workflows`, `.goreleaser.yaml`; keywords: update, install, download, checksum, attestation, workflow, permissions, dependency) | Accepted | [0024](0024-verified-releases.md) |
| History copies are opt-in (`kiroku archive on`), only for agents that delete history, private and local, private sessions still masked, removable (`internal/archive`, `internal/cli` archive; keywords: archive, retention, backup, copy, cleanup) | Accepted | [0025](0025-opt-in-history-copies.md) |

## Superseded ADRs

| Topic | Superseded by | ADR |
|---|---|---|
| Complementary Calendar and Journey history views | [0033](0033-calendar-only-view.md) | [0026](0026-complementary-history-views.md) |
| Project view | [0033](0033-calendar-only-view.md) | [0027](0027-project-view.md) |
| Knowledge discovery (Phase 1: the copied knowledge prompt) | [0033](0033-calendar-only-view.md) | [0028](0028-knowledge-discovery.md) |

Only a human decision owner may accept an ADR. Proposed records are not binding constraints. An empty index does not prove a change safe.

## Historical compatibility

Former retrospective ADRs 0001–0019 are now Decision Logs; old ADR file paths remain as redirects. They are **not** accepted constraints. Their original evidence remains in [evidence.yaml](evidence.yaml), [audit.md](audit.md) and [phase1-decision-ledger.md](phase1-decision-ledger.md). Legacy evidence IDs are preserved for traceability.

ADR Guard checks Accepted decisions, then consults Decision Logs for relevant historical context. New durable choices are drafted as Proposed ADRs and require explicit human review. Update routing and supersession links alongside decisions.
