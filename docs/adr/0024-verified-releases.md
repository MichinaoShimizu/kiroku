# ADR 0024: Releases are verified before they replace anything

Status: Accepted (2026-10-11, approved by the maintainer when promoting Design Log 0011)
Date: 2026-10-11
Scope: Supply chain — `kiroku update`, `install.sh`, the release workflow, GitHub Actions, dependencies (`internal/cli/update.go`, `install.sh`, `.github/workflows`, `.goreleaser.yaml`, `SECURITY.md`)
Related ADRs: [ADR 0022](0022-history-stays-local.md), [ADR 0023](0023-compatibility-contract.md)
Related Design Logs: [DL 0011 — release integrity](../design-log/0011-release-integrity.md)
Supersedes/Amends: None
Decision owner: Repository maintainer

## Context

`kiroku update` and `install.sh` download a binary and replace the one on the user's machine; the user runs it with access to all their history. [DL 0011](../design-log/0011-release-integrity.md) records how checksums, build provenance, SBOMs and a reproducibility check were added. The rules below are already in `CLAUDE.md`; this ADR makes them a binding decision.

## Decision

1. **Verify before replacing.** A downloaded release must match `checksums.txt` before it replaces anything. Where build provenance is available (`gh attestation verify` in `install.sh`), it is checked too.
2. **GitHub Releases is the only download source** (the one network use [ADR 0022](0022-history-stays-local.md) allows).
3. **GitHub Actions:** third-party actions are pinned to a full commit SHA; each job gets the least `permissions` it needs; untrusted code never runs with write tokens.
4. **`govulncheck` stays clean.** A reported vulnerability in code kiroku calls is fixed (Go toolchain or module update) and released before other work.

## Alternatives

- **Keep these as Design Log 0011 plus `CLAUDE.md`.** ADR Guard wouldn't flag a change that weakens verification. Not chosen.
- **Require provenance on every install.** `install.sh` can't assume `gh` is installed and signed in, so provenance is checked when available and required only with `KIROKU_REQUIRE_ATTESTATION=1`. Checksums are always required.

## Consequences

- A change that skips or weakens verification, adds another download source, or loosens Actions permissions conflicts with this ADR.
- Checksums only prove the file matches `checksums.txt` from the same release; provenance and the reproducibility check add independent evidence. None of these is a guarantee.
- Revisit if releases move off GitHub or if Go gets a built-in way to verify binaries.

## Verification

Run on `23116a3` (2026-10-11), all passed: `internal/cli` `TestSelfUpdate`, `TestSelfUpdateFailures` (including a checksum mismatch), `TestSelfUpdateZip`.

`govulncheck` and Action pinning are checked in CI (`.github/workflows/ci.yml`, `tools/check.sh`, OpenSSF Scorecard); this ADR doesn't claim a fresh run of them.

## Open questions

None for acceptance.
