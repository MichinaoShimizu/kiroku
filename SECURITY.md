# Security policy

kiroku reads AI agent histories that can contain private prompts, file paths and commit messages. Security reports are taken seriously.

## Reporting a vulnerability

Please report vulnerabilities privately through [GitHub's private vulnerability reporting](https://github.com/MichinaoShimizu/kiroku/security/advisories/new) (Security → Report a vulnerability). Do not open a public issue, and do not attach real agent history.

Reports in English or Japanese are welcome. You can expect a first response within a week.

## Scope

Especially relevant areas:

- `kiroku serve`: it listens on `127.0.0.1` only (a bare port such as `:8485` also stays on `127.0.0.1`) and rejects requests whose Host is not `localhost`, `127.0.0.1` or `[::1]` (DNS rebinding protection). Listening on another address such as `0.0.0.0:8484` must be chosen explicitly and turns that check off. It serves an original history file (`.json` / `.jsonl` only) only for sessions in the current snapshot
- The generated HTML: user data must be escaped, and nothing should be sent off the machine
- `install.sh` and `kiroku update`: releases are verified against `checksums.txt` before replacing the binary

Only the latest release is supported.
