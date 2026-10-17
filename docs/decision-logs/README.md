# Decision logs (kiroku)

> Rebuilt by decision-log from each Decision log; don't edit it by hand. These are past observations and bind nothing.

## Architecture (architecture)

Splitting and merging, direction of dependencies, adopted technology, platform migrations

- 2026-10-16 [DL-15](0015-installable-serve-view.md) The kiroku serve view can be installed as an app window, without a service worker or a native wrapper

## Data meaning and storage (data)

Schemas, meaning and units of values, where data is stored, retention (personal data is privacy)

- 2026-10-04 [DL-7](0007-opt-in-history-copies.md) kiroku keeps copies of history that agents delete, only after the user turns it on

## Authentication, permissions and security (security)

Authentication methods, permission boundaries, secrets, signing and verification, encryption

- 2026-10-03 [DL-1](0001-serve-access.md) Who may read history through kiroku serve: loopback, then the Host check, then a key, then the key out of cookies
- 2026-10-06 [DL-2](0002-untrusted-local-inputs.md) kiroku treats history, git and its own downloads as untrusted local input
- 2026-10-13 [DL-10](0010-ai-execution-default.md) "Analyze with AI" went from effectively on, to opt-in with --ai, to on by default with --no-ai
- 2026-10-14 [DL-11](0011-ai-agent-least-access.md) Agents run by "Analyze with AI" get the least access each one allows, in an empty folder

## Personal data (privacy)

Whether personal data is collected, where it is shown, sent or kept (logs, destinations, retention), masking

- 2026-10-10 [DL-3](0003-private-sessions.md) Conversations an agent marks private are shown as counts and times only, in every copy kiroku reads
- 2026-10-14 [DL-4](0004-user-mediated-diagnostics.md) Format problems reach the maintainer only through the user, as kinds and counts

## Compatibility and external contracts (compatibility)

Public APIs, formats, versions, deprecation

- 2026-10-07 [DL-8](0008-compatibility-contract.md) What stays compatible within a major version is written down, and kiroku json is versioned
- 2026-10-11 [DL-9](0009-kiro-cli-v3-labels.md) Kiro CLI V3 is read with Kiro IDE as "Kiro IDE / CLI", and V2 becomes "Kiro CLI (Classic)"

## Release and operations (release)

Deployment steps, flags, environments, monitoring, alerts and who gets them, logging and tracing

- 2026-10-03 [DL-5](0005-release-provenance.md) Releases are built after tests on three OSes, reproducibly, and published with signed provenance
- 2026-10-04 [DL-6](0006-verified-downloads.md) install.sh and kiroku update verify a download before it replaces anything

## Product policy (product)

Where the spec draws the line, things decided not to do

- 2026-10-04 [DL-13](0013-view-language.md) The view went bilingual, then English only, then Japanese again through a message catalog
- 2026-10-11 [DL-14](0014-calendar-only-view.md) The Journey and Project views were added and then removed; the view is the calendar only
- 2026-10-16 [DL-12](0012-ai-reports-scope.md) "Analyze with AI" offers no monthly report, and the live demo previews the prompt without running it
