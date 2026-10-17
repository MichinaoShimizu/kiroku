---
kind: log
category: release
keywords: [install.sh, checksums.txt, gh attestation verify, KIROKU_SKIP_ATTESTATION, KIROKU_REQUIRE_ATTESTATION, replaceExe, kiroku update, com.apple.quarantine]
---

# [kiroku] install.sh and kiroku update verify a download before it replaces anything

> A past observation, not a decision. It binds nothing.

## What happened

- C1 2026-10-04 Still true: `install.sh` installs with `curl | sh` on macOS and Linux, finds the latest version from the `releases/latest` redirect, and checks the archive against `checksums.txt` before installing. History: [PR #17](https://github.com/MichinaoShimizu/kiroku/pull/17) / Now: [`install.sh` @`1d7b409`](https://github.com/MichinaoShimizu/kiroku/blob/1d7b4091906cc02d4f32bdabd120f5120dcf0792/install.sh)
- C2 2026-10-04 Still true: `kiroku update` downloads the archive for this OS, checks it against `checksums.txt`, writes a temp file in the same folder and renames it over the binary (on Windows the running exe is first renamed to `.old`); a `dev` build is not replaced without `--force`. History: [PR #18](https://github.com/MichinaoShimizu/kiroku/pull/18) / Now: [`internal/cli/update.go` `replaceExe` @`1d7b409`](https://github.com/MichinaoShimizu/kiroku/blob/1d7b4091906cc02d4f32bdabd120f5120dcf0792/internal/cli/update.go)
- C3 2026-10-06 Still true: when `gh` is installed, `install.sh` also runs `gh attestation verify` with `release.yml` as the signer. A 404 or a signer mismatch stops the install; not being logged in, an old `gh` or a network error only warns. `KIROKU_SKIP_ATTESTATION=1` skips the check. History: [PR #151](https://github.com/MichinaoShimizu/kiroku/pull/151) / Now: [`install.sh` @`1d7b409`](https://github.com/MichinaoShimizu/kiroku/blob/1d7b4091906cc02d4f32bdabd120f5120dcf0792/install.sh)
- C4 2026-10-14 Still true: `install.sh` copies the binary to a fresh `mktemp` file before the rename (instead of a fixed `.kiroku.new`), and every download uses `curl --proto '=https' --tlsv1.2`. History: [PR #408](https://github.com/MichinaoShimizu/kiroku/pull/408) / Now: [`install.sh` @`1d7b409`](https://github.com/MichinaoShimizu/kiroku/blob/1d7b4091906cc02d4f32bdabd120f5120dcf0792/install.sh)

## Reason

Documented (description, AI): files fetched with `curl` get no quarantine attribute, so macOS Gatekeeper doesn't block the unnotarized binary, and the redirect avoids the API rate limit (#17). Writing next to the binary and renaming leaves the old kiroku intact if anything fails (#18). When `gh` cannot check provenance, `checksums.txt` has still matched, so the script only warns; a tampered file also shows as 404, so 404 is fatal; the signer is `release.yml` because the certificate names the called workflow (#151). A fixed temp name could be written through a file or symlink someone else left (#408) ([PR #17](https://github.com/MichinaoShimizu/kiroku/pull/17), [PR #18](https://github.com/MichinaoShimizu/kiroku/pull/18), [PR #151](https://github.com/MichinaoShimizu/kiroku/pull/151), [PR #408](https://github.com/MichinaoShimizu/kiroku/pull/408)).

## People involved

From the PR records; not necessarily who decided.

- PR #17: opened @MichinaoShimizu (2026-10-03), no approving review, merged @MichinaoShimizu (2026-10-04)
- PR #18: opened @MichinaoShimizu (2026-10-04), no approving review, merged @MichinaoShimizu (2026-10-04)
- PR #151: opened @MichinaoShimizu (2026-10-06), no approving review, merged @MichinaoShimizu (2026-10-06)
- PR #408: opened @MichinaoShimizu (2026-10-14), no approving review, merged @MichinaoShimizu (2026-10-14)

## Open questions

- `kiroku update` checks only `checksums.txt`, while install.sh also checks provenance. Is that difference intended? (ask @MichinaoShimizu)
- Is "verify provenance only when gh is there, warn when it cannot run" the lasting policy, or a step before requiring it? (ask @MichinaoShimizu)
- Was Apple notarization considered instead of `curl | sh`? (ask @MichinaoShimizu)
- Do the reasons above, written by an AI agent in the PR descriptions, match the maintainer's own judgement? (ask @MichinaoShimizu)

## Sources

- [ADR 0024 (removed)](https://github.com/MichinaoShimizu/kiroku/blob/27e8298bdc0367afc623bc4db5c450ca0bad38b1/docs/adr/0024-verified-releases.md)

## Related logs

- DL-5
