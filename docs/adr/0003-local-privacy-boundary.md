# ADR 0003: Local privacy and trust boundary

Status: **Retrospective**

Scope: Phase 1 — local sessions.

## Context

Agent transcripts can contain prompts, source code, project paths and other sensitive information. The local history viewer needs deliberate controls at storage, rendering, server access and subprocess boundaries.

## Documented decisions

- **Local processing and export.** The initial tool reads local histories and exports standalone HTML ([#1](https://github.com/MichinaoShimizu/kiroku/pull/1)); Go single-binary distribution retains the local execution model ([#6](https://github.com/MichinaoShimizu/kiroku/pull/6)).
- **No automatic AI submission.** Early AI-advice UX copied a prompt for users to paste themselves; kiroku did not invoke an AI API ([#22](https://github.com/MichinaoShimizu/kiroku/pull/22)). That particular suggestion panel was later removed ([#304](https://github.com/MichinaoShimizu/kiroku/pull/304)); report prompts follow the same user-mediated boundary ([#292](https://github.com/MichinaoShimizu/kiroku/pull/292)).
- **Limit server access.** Serve originally defaulted to loopback ([#11](https://github.com/MichinaoShimizu/kiroku/pull/11)); DNS rebinding protection and a history-access key were added ([#127](https://github.com/MichinaoShimizu/kiroku/pull/127), [#172](https://github.com/MichinaoShimizu/kiroku/pull/172)).
- **Treat history as untrusted.** Harden file writes, parsers, Git execution and rendering, including stored-XSS fixes and CSP ([#159](https://github.com/MichinaoShimizu/kiroku/pull/159), [#161](https://github.com/MichinaoShimizu/kiroku/pull/161), [#162](https://github.com/MichinaoShimizu/kiroku/pull/162)); reject unsafe quote characters in copyable commands ([#307](https://github.com/MichinaoShimizu/kiroku/pull/307)).
- **Respect private agent sessions.** Kiro Crew incognito/temporary histories retain timing, counts, model/usage/credits/cost, tool names and project while masking prompts and removing replies, notes, edited files, PR URLs, subagent tasks and history-file links ([#312](https://github.com/MichinaoShimizu/kiroku/pull/312)).
- **Avoid incidental network reads.** Git enrichment must not fetch remote repositories ([#293](https://github.com/MichinaoShimizu/kiroku/pull/293)).

## Rationale and consequences

Local-first is a data-flow and security boundary, not a guarantee that any generated report is safe to publish. HTML/JSON exports and user-copied prompts can contain sensitive material. Install/update are separate explicitly networked workflows; no absolute offline claim is made.

For Phase 2 remote import, downloaded transcripts are untrusted input, and remote project paths must never be treated as locally trusted Git roots by default.

## Review-thread evidence and limits

The review discussion on [#172](https://github.com/MichinaoShimizu/kiroku/pull/172) raised a CodeQL finding about the cookie `Secure` attribute. A localhost cookie and a LAN-accessible HTTP server have different transport properties: the serve key is an access control, **not encryption**. The current [SECURITY.md](../../SECURITY.md) explicitly warns that printed access URLs on a network travel unencrypted. Do not infer HTTPS or secure cross-device transport from the existence of a key.

## Alternatives and historical confidence

A kiroku-hosted backend was not established in these PRs; this record does not claim a formal historical rejection of all cloud designs.

## Verification

Inspect `SECURITY.md`, `internal/cli`, `internal/gitlog`, `internal/source` and relevant security tests. No new security testing is claimed here.
