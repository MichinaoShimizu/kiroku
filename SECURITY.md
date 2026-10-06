# Security policy

kiroku reads AI agent histories that can contain private prompts, file paths and commit messages. Security reports are taken seriously.

## Reporting a vulnerability

Please report vulnerabilities privately through [GitHub's private vulnerability reporting](https://github.com/MichinaoShimizu/kiroku/security/advisories/new) (Security → Report a vulnerability). Do not open a public issue, and do not attach real agent history.

Reports in English or Japanese are welcome. You can expect a first response within a week.

## Scope

Especially relevant areas:

- `kiroku serve`: it listens on `127.0.0.1` only (a bare port such as `:8485` also stays on `127.0.0.1`) and rejects requests whose Host is not `localhost`, `127.0.0.1`, `[::1]`, the address it listens on or a name added with `--allow-host` (DNS rebinding protection). Listening on another address such as `0.0.0.0:8484` must be chosen explicitly; it then also answers this computer's own IPs and host name, and still rejects any other Host. It serves an original history file (`.json` / `.jsonl` only) only for sessions in the current snapshot, and only when the file (after following symbolic links) is inside a location kiroku reads history from or the `kiroku archive` folder; session IDs read from history that contain path separators, `..` or drive names are rejected. It has no write timeout (history files can be large) but limits how long a client may take to send its headers and how large they may be. Every response carries `X-Frame-Options: DENY`, `Content-Security-Policy: frame-ancestors 'none'`, `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer` and same-origin `Cross-Origin-Opener-Policy` / `Cross-Origin-Resource-Policy`. Error responses don't include local paths; the details go to the terminal
- Files kiroku writes: `kiroku html` and `kiroku json` write through a temporary file in the same folder that is renamed into place, readable only by you (`0600`), so a symbolic link at the destination is replaced rather than followed. `kiroku archive` keeps its copies in folders only you can open (`0700`, files `0600`). `kiroku archive off` deletes only the `.zst` copies kiroku saved, and only in a folder marked by `kiroku archive on`
- Reading history: lines longer than 64 MiB are skipped (and reported as unreadable) instead of being read into memory, and zstd-compressed history is decoded with a 1 GiB memory limit
- git: kiroku runs `git` only to read commits and pushes in folders named by your history, which may be repositories someone else made. It ignores the system git config (`GIT_CONFIG_NOSYSTEM=1`) and overrides the repository's settings that run programs (`core.fsmonitor`, `core.hooksPath`, `core.pager`, `log.showSignature`, and `--no-ext-diff --no-textconv` for `git log`), never prompts (`GIT_TERMINAL_PROMPT=0`) and takes no optional locks. Your global git config is still read, since `user.email` (to show only your commits) and `safe.directory` live there. On Windows, network (UNC) paths from history are skipped
- `kiroku autostart` refuses values with control characters (such as newlines) when writing the systemd unit
- The generated HTML: user data must be escaped, and nothing should be sent off the machine
- `install.sh` and `kiroku update`: releases are verified against `checksums.txt` from the same release before replacing the binary. `kiroku update` accepts only version names of the form `v1.2.3` (or `v1.2.3-rc.1`), follows only `https` redirects, extracts only a regular file of at most 200 MiB, and installs an older version only with `--force`. `kiroku update` does not check attestations itself. When the GitHub CLI (`gh`) is installed, `install.sh` additionally checks the artifact attestation with `gh attestation verify` and stops if it does not match (it only warns when `gh` is not logged in or cannot reach GitHub; `KIROKU_SKIP_ATTESTATION=1` skips the check)
- Releases are built by GitHub Actions and carry artifact attestations; `gh attestation verify <file> --repo MichinaoShimizu/kiroku` checks that a file came from this repository

Only the latest release is supported.
