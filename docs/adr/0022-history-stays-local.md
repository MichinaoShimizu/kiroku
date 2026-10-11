# ADR 0022: History stays on the machine and is treated as untrusted

Status: Accepted (2026-10-11, approved by the maintainer when promoting Design Log 0003)
Date: 2026-10-11
Scope: Privacy and trust boundary — reading history, git, the HTML view, `kiroku serve`, files kiroku writes, network access (`internal/source`, `internal/gitlog`, `internal/web`, `internal/cli`, `internal/archive`, `SECURITY.md`)
Related ADRs: [ADR 0024](0024-verified-releases.md) (the one allowed network use), [ADR 0025](0025-opt-in-history-copies.md)
Related Design Logs: [DL 0003 — local privacy boundary](../design-log/0003-local-privacy-boundary.md), [DL 0013 — user-mediated AI reports](../design-log/0013-user-mediated-ai-reports.md)
Supersedes/Amends: None
Decision owner: Repository maintainer

## Context

kiroku reads private AI-agent history: prompts, replies, file paths, commit messages. [DL 0003](../design-log/0003-local-privacy-boundary.md) records how the current boundary grew PR by PR, but as a Design Log it doesn't bind later changes. The same rules are already enforced through the Security section of `CLAUDE.md`, `SECURITY.md` and tests. This ADR makes them an explicit, approved decision, so ADR Guard flags a change that would cross them.

This ADR records today's decision. It doesn't claim why each historical PR made its choice; that stays in DL 0003.

## Decision

1. **History never leaves the machine.** kiroku sends no history and nothing derived from it over the network, and has no telemetry. The only network access is checking and downloading kiroku releases from GitHub ([ADR 0024](0024-verified-releases.md)). Git enrichment never fetches from remotes.
2. **No automatic AI submission.** kiroku builds prompts for an AI, but the user copies and sends them. kiroku doesn't call an AI API.
3. **Everything from history, git and the network is untrusted.** The HTML view escapes it and never inserts it as HTML; it is never passed to a shell; paths are validated before files are read or served.
4. **Files kiroku writes that contain history are private:** `0600` files, `0700` directories (HTML, JSON, archive copies, caches, the serve key).
5. **`kiroku serve` is local by default.** It listens on `127.0.0.1`, checks the Host header (DNS rebinding protection) and requires its key. Anything that widens access (another listen address, `--allow-host`) is opt-in and documented in `SECURITY.md`.
6. **Private agent sessions stay private.** Sessions an agent marks private (Kiro Crew incognito and temporary) are shown as counts and times only, in the view, the JSON and copied prompts.

`SECURITY.md` is the normative detail for each point; this ADR is the rule it implements.

## Alternatives

- **Keep these as Design Log 0003 plus `CLAUDE.md` only.** That's how they were enforced until now. It works for agents that read `CLAUDE.md`, but ADR Guard treats a Design Log as context, not a constraint, so a change that crosses the boundary is not flagged as a conflict. Not chosen.
- **A hosted backend or opt-in sync.** Not proposed in any PR; it would contradict point 1. Recording it here as not chosen doesn't claim it was ever evaluated historically.

## Consequences

- A feature that needs history off the machine (remote import, sharing, AI calls) needs a new ADR that amends or supersedes this one. [ADR 0020](0020-remote-session-import.md) (Proposed) moves history *onto* the machine and stays within this boundary.
- Local-first is a data-flow boundary, not a promise that an exported HTML or JSON file is safe to publish. Those files can contain sensitive text; that's the user's choice.
- The serve key is access control, not encryption. A URL printed for another device travels unencrypted (`SECURITY.md`).
- Revisit if kiroku ever needs a network service other than GitHub Releases.

## Verification

Run on `23116a3` (2026-10-11), all passed:

- `internal/cli`: `TestListenAddr`, `TestSameOrigin`, `TestRequireKey`, `TestServeKey`, `TestServeSecurityHeaders`, `TestServeHistoryOnlyUnderRoots`, `TestHTMLAndJSONArePrivate`
- `internal/gitlog`: `TestCollectDoesNotFetchInPartialClone`
- `internal/source`: `TestKiroCrewPrivateConversations`

No test proves the absence of other network calls; point 1 is checked in review (`CLAUDE.md`, `/pre-pr`) and by the hostile-tester agent.

## Open questions

None for acceptance.
