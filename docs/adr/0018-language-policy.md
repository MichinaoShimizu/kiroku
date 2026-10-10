# ADR 0018: English product UI and Japanese entry documentation

Status: **Retrospective**

Scope: Phase 1 — local sessions.

## Documented decision and evolution

[#46](https://github.com/MichinaoShimizu/kiroku/pull/46) first added a bilingual interface with locale detection and a manual language switch. [#110](https://github.com/MichinaoShimizu/kiroku/pull/110) deliberately removed the bilingual UI and translated application/documentation text into English, while retaining Japanese history parsing. [#317](https://github.com/MichinaoShimizu/kiroku/pull/317) restored a Japanese README for discovery while leaving the guide in English.

## Rationale and consequences

The current intended distinction is English application UI and guide, with a Japanese README as an entry point. Historical bilingual support was superseded; it should not be inferred from older screenshots or source discussions.

## Verification and confidence

Review current README files, web UI and localization tests before treating language coverage as a compatibility guarantee.

This is an evidence-linked retrospective, not an assertion of historical ADR approval.
