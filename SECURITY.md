# Security policy

kiroku reads AI agent histories that can contain private prompts, file paths and commit messages. Security reports are taken seriously.

## Reporting a vulnerability

Please report vulnerabilities privately through [GitHub's private vulnerability reporting](https://github.com/MichinaoShimizu/kiroku/security/advisories/new) (Security → Report a vulnerability). Do not open a public issue, and do not attach real agent history.

Reports in English or Japanese are welcome. You can expect a first response within a week.

## Scope

Especially relevant areas:

- `kiroku serve`: it listens on `127.0.0.1` only (a bare port such as `:8485` also stays on `127.0.0.1`) and rejects requests whose Host is not `localhost`, `127.0.0.1` or `[::1]` (DNS rebinding protection). Listening on another address such as `0.0.0.0:8484` must be chosen explicitly; it then also answers this computer's own IPs and host name, plus names added with `--allow-host`, and still rejects any other Host. It serves an original history file (`.json` / `.jsonl` only) only for sessions in the current snapshot
- The generated HTML: user data must be escaped, and nothing should be sent off the machine
- `install.sh` and `kiroku update`: releases are verified against `checksums.txt` before replacing the binary. When the GitHub CLI (`gh`) is installed, `install.sh` also checks the artifact attestation with `gh attestation verify` and stops if it does not match (it only warns when `gh` is not logged in or cannot reach GitHub; `KIROKU_SKIP_ATTESTATION=1` skips the check)
- Releases are built by GitHub Actions and carry artifact attestations; `gh attestation verify <file> --repo MichinaoShimizu/kiroku` checks that a file came from this repository

Only the latest release is supported.
