# Security policy

kiroku reads AI agent histories that can contain private prompts, file paths and commit messages. Security reports are taken seriously.

## Reporting a vulnerability

Please report vulnerabilities privately through [GitHub's private vulnerability reporting](https://github.com/MichinaoShimizu/kiroku/security/advisories/new) (Security → Report a vulnerability). Do not open a public issue, and do not attach real agent history.

Reports in English or Japanese are welcome. You can expect a first response within a week.

## Scope

Especially relevant areas:

- `kiroku serve`: it listens on `127.0.0.1` only, rejects requests from other origins, and serves the original history file only for sessions in the current snapshot
- The generated HTML: user data must be escaped, and nothing should be sent off the machine
- `install.sh` and `kiroku update`: releases are verified against `checksums.txt` before replacing the binary

Only the latest release is supported.
