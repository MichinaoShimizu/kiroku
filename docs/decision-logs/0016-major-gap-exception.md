---
kind: log
category: compatibility
keywords: [tools/major-gap.sh, KIROKU_MAJOR_OK, Deprecated, 30 days, tools/release.sh, v5.0.0]
---

# [kiroku] v5.0.0 went out three days after v4.0.0, past the 30-day gap and with no Deprecated release first

> A past observation, not a decision. It binds nothing.

## What happened

- C1 2026-11-23 Still true: major versions must be at least 30 days apart, after a `### Deprecated` minor release; `tools/release.sh` stops otherwise (`tools/major-gap.sh`), and `KIROKU_MAJOR_OK=1` lets a security fix through. History: [PR #543](https://github.com/MichinaoShimizu/kiroku/pull/543) / Now: [`docs/compatibility.md:25` @`622cef4`](https://github.com/MichinaoShimizu/kiroku/blob/622cef4/docs/compatibility.md#L25)
- C2 2026-11-24 History only: v5.0.0 (released 2026-11-24, [`a71a684`](https://github.com/MichinaoShimizu/kiroku/commit/a71a6847df55b625b8d4daaff0f4846548fe7e48)) came three days after v4.0.0 (2026-11-21, [`e68231f`](https://github.com/MichinaoShimizu/kiroku/commit/e68231fff7f0d155893feaa03c7a8d3e03a07b31)), made with `KIROKU_MAJOR_OK=1`; CHANGELOG.md has no `### Deprecated` section before it. History: [PR #557](https://github.com/MichinaoShimizu/kiroku/pull/557)
- C3 2026-11-25 Still true: compatibility.md lists v5.0.0 as a recorded one-off exception and keeps `KIROKU_MAJOR_OK=1` for security fixes. History: the 2026-11-25 quality audit ([`docs/quality/audit-log.md`](../quality/audit-log.md), H1) / Now: [`docs/compatibility.md:26` @`622cef4`](https://github.com/MichinaoShimizu/kiroku/blob/622cef4/docs/compatibility.md#L26)

## Reason

Documented (description, AI): "The maintainer asked to release this now, without waiting the 30 days `tools/major-gap.sh` asks for after v4.0.0" (PR #557, section "Major gap"). The PR does not say why the release could not wait, and does not mention the skipped Deprecated step.

For C3, the maintainer chose "docs only" in the 2026-11-25 quality audit: record the exception, keep `KIROKU_MAJOR_OK=1` for security fixes, and add no CI check for the gap.

## People involved

From the PR records. Not necessarily who decided.

- PR #543: opened @MichinaoShimizu (2026-11-23), no reviews, merged @MichinaoShimizu (2026-11-23)
- PR #557: opened @MichinaoShimizu (2026-11-23), no reviews, merged @MichinaoShimizu (2026-11-24)

## Open questions

- Why v5.0.0 could not wait for the gap and a Deprecated release (ask @MichinaoShimizu).
- PR #557's reason was written with Claude Code: does it match the maintainer's own reasoning (ask @MichinaoShimizu)?
- `tools/major-gap.sh` runs only in `tools/release.sh`, not in CI or `tag.yml`, so a release made another way is not checked.

## Related logs

- DL-8
