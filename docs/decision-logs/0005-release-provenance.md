---
kind: log
category: release
keywords: [release.yml, release-dry-run, goreleaser, trimpath, mod_timestamp, attest-build-provenance, sigstore.json, id-token: write]
---

# [kiroku] Releases are built after tests on three OSes, reproducibly, and published with signed provenance

> A past observation, not a decision. It binds nothing.

## What happened

- C1 2026-10-03 Still true: `release.yml` runs `go vet` and `go test` on Ubuntu, macOS and Windows before publishing; only the publishing job has `contents: write`; CI runs a GoReleaser snapshot (`release-dry-run`) on every PR; builds use `-trimpath` and the commit time. History: [PR #12](https://github.com/MichinaoShimizu/kiroku/pull/12) / Now: [`.github/workflows/release.yml` @`1d7b409`](https://github.com/MichinaoShimizu/kiroku/blob/1d7b4091906cc02d4f32bdabd120f5120dcf0792/.github/workflows/release.yml), [`.goreleaser.yaml` @`1d7b409`](https://github.com/MichinaoShimizu/kiroku/blob/1d7b4091906cc02d4f32bdabd120f5120dcf0792/.goreleaser.yaml)
- C2 2026-10-06 Still true: the release job attests the archives and `checksums.txt` with `actions/attest-build-provenance`, so `gh attestation verify <file> --repo MichinaoShimizu/kiroku` can check where they were built. History: [PR #144](https://github.com/MichinaoShimizu/kiroku/pull/144) / Now: [`.github/workflows/release.yml` @`1d7b409`](https://github.com/MichinaoShimizu/kiroku/blob/1d7b4091906cc02d4f32bdabd120f5120dcf0792/.github/workflows/release.yml)
- C3 2026-10-06 Still true: the signed provenance bundle is also attached to each release as `kiroku_<version>.sigstore.json`. History: [PR #170](https://github.com/MichinaoShimizu/kiroku/pull/170) / Now: [`.github/workflows/release.yml` @`1d7b409`](https://github.com/MichinaoShimizu/kiroku/blob/1d7b4091906cc02d4f32bdabd120f5120dcf0792/.github/workflows/release.yml)
- C4 2026-10-14 Still true: `tag.yml` checks out without persisted credentials and gives the token only to the step that pushes the tag. History: [PR #408](https://github.com/MichinaoShimizu/kiroku/pull/408) / Now: [`.github/workflows/tag.yml` @`1d7b409`](https://github.com/MichinaoShimizu/kiroku/blob/1d7b4091906cc02d4f32bdabd120f5120dcf0792/.github/workflows/tag.yml)

## Reason

Documented (description, AI): catch a broken release config on the PR instead of at tag time, publish nothing if any OS fails, and give write access only to the publishing job (#12). install.sh and `kiroku update` checked `checksums.txt`, but nothing showed the files were built from this repository (#144). The bundle passes OpenSSF Scorecard's Signed-Releases check and allows verifying without GitHub's attestation API (#170). The token should reach only the step that pushes (#408) ([PR #12](https://github.com/MichinaoShimizu/kiroku/pull/12), [PR #144](https://github.com/MichinaoShimizu/kiroku/pull/144), [PR #170](https://github.com/MichinaoShimizu/kiroku/pull/170), [PR #408](https://github.com/MichinaoShimizu/kiroku/pull/408)).

## People involved

From the PR records; not necessarily who decided.

- PR #12: opened @MichinaoShimizu (2026-10-03), no approving review, merged @MichinaoShimizu (2026-10-03)
- PR #144: opened @MichinaoShimizu (2026-10-06), no approving review, merged @MichinaoShimizu (2026-10-06)
- PR #170: opened @MichinaoShimizu (2026-10-06), no approving review, merged @MichinaoShimizu (2026-10-06)
- PR #408: opened @MichinaoShimizu (2026-10-14), no approving review, merged @MichinaoShimizu (2026-10-14)

## Open questions

- Were GPG or cosign signatures considered before GitHub artifact attestation? (ask @MichinaoShimizu)
- Why MIT, and was another license considered (#12 added it)? (ask @MichinaoShimizu)
- Do the reasons above, written by an AI agent in the PR descriptions, match the maintainer's own judgement? (ask @MichinaoShimizu)

## Sources

- [ADR 0024 (removed)](https://github.com/MichinaoShimizu/kiroku/blob/27e8298bdc0367afc623bc4db5c450ca0bad38b1/docs/adr/0024-verified-releases.md)

## Related logs

- DL-6
