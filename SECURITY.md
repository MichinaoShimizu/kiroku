# Security policy

kiroku reads AI agent histories that can contain private prompts, file paths and commit messages. Security reports are taken seriously.

## Reporting a vulnerability

Please report vulnerabilities privately through [GitHub's private vulnerability reporting](https://github.com/MichinaoShimizu/kiroku/security/advisories/new) (Security → Report a vulnerability). Do not open a public issue, and do not attach real agent history.

Reports in English or Japanese are welcome. You can expect a first response within a week.

## Scope

Especially relevant areas:

- `kiroku serve`: it shows the view, its data and history files only to a browser that has its key, so that other users of the same computer can't read your history through it. The key is 32 random bytes kept in `serve-key` in kiroku's settings folder (`0600`, folder `0700`; a key file others can read is replaced). A browser gets it by opening the view with `?key=<key>`, which sets a cookie (`HttpOnly`, `SameSite=Strict`, named after the port) and then moves, from a page on the same port, to the address without the key; keys are compared in constant time. The cookie is `Strict` because browsers don't separate cookies by port: with `Lax`, a link on another site to another port of `localhost` (perhaps a server run by another user) would carry the key there. kiroku never puts the key on a browser's command line (visible to other users): it opens a `0600` HTML file that forwards to the view. `kiroku open --print` prints the address with the key for another device; on a network it travels unencrypted. It listens on `127.0.0.1` only (a bare port such as `:8485` also stays on `127.0.0.1`) and rejects requests whose Host is not `localhost`, `127.0.0.1`, `[::1]`, the address it listens on or a name added with `--allow-host` (DNS rebinding protection). Listening on another address such as `0.0.0.0:8484` must be chosen explicitly; it then also answers this computer's own IPs and host name, and still rejects any other Host. It serves an original history file (`.json` / `.jsonl` only) only for sessions in the current snapshot, and only when the file (after following symbolic links) is inside a location kiroku reads history from or the `kiroku archive` folder; session IDs read from history that contain path separators, `..` or drive names are rejected. It has no write timeout (history files can be large) but limits how long a client may take to send its headers and how large they may be. Every response carries `X-Frame-Options: DENY`, `Content-Security-Policy: frame-ancestors 'none'`, `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer` and same-origin `Cross-Origin-Opener-Policy` / `Cross-Origin-Resource-Policy`. Error responses don't include local paths; the details go to the terminal
- Files kiroku writes: `kiroku html` and `kiroku json` write through a temporary file in the same folder that is renamed into place, readable only by you (`0600`), so a symbolic link at the destination is replaced rather than followed. `kiroku archive` keeps its copies in folders only you can open (`0700`, files `0600`). `kiroku archive off` deletes only the `.zst` copies kiroku saved, and only in a folder marked by `kiroku archive on`
- Reading history: lines longer than 64 MiB are skipped (and reported as unreadable) instead of being read into memory, and zstd-compressed history is decoded with a 1 GiB memory limit
- git: kiroku runs `git` only to read commits and pushes in folders named by your history, which may be repositories someone else made. It ignores the system git config (`GIT_CONFIG_NOSYSTEM=1`) and overrides the repository's settings that run programs (`core.fsmonitor`, `core.hooksPath`, `core.pager`, `log.showSignature`, and `--no-ext-diff --no-textconv` for `git log`), never prompts (`GIT_TERMINAL_PROMPT=0`) and takes no optional locks. Your global git config is still read, since `user.email` (to show only your commits) and `safe.directory` live there. On Windows, network (UNC) paths from history are skipped
- `kiroku autostart` refuses values with control characters (such as newlines) when writing the systemd unit
- The generated HTML: user data must be escaped, and nothing should be sent off the machine. The page carries a Content-Security-Policy that allows only its own inline script and stylesheet (by hash), no external resources, and network requests only back to `kiroku serve` (none for a saved HTML file). Text it copies for pasting elsewhere (report drafts, prompt exports, prompts for an AI) escapes Markdown, and the AI prompts mark history as data, not instructions
- `install.sh` and `kiroku update`: releases are verified against `checksums.txt` from the same release before replacing the binary. `kiroku update` accepts only version names of the form `v1.2.3` (or `v1.2.3-rc.1`), follows only `https` redirects, extracts only a regular file of at most 200 MiB, and installs an older version only with `--force`. `kiroku update` does not check attestations itself. When the GitHub CLI (`gh`) is installed, `install.sh` also checks the artifact attestation with `gh attestation verify` (signer workflow `release.yml`, source ref `main` or the release tag, GitHub-hosted runners only) and stops if it does not match. It only warns when `gh` is not logged in or cannot reach GitHub; `KIROKU_REQUIRE_ATTESTATION=1` makes any failure to check fatal (including a missing `gh`), and `KIROKU_SKIP_ATTESTATION=1` skips the check. The script body runs from its last line only, so a truncated `curl | sh` download does nothing
- Releases are built by GitHub Actions and carry artifact attestations (see "Verifying a release" below). Workflows pin third-party actions to commit SHAs, give each job only the permissions it needs, and the release build does not use the Actions cache

Only the latest release is supported.

## Automated checks

- [CodeQL](https://github.com/MichinaoShimizu/kiroku/actions/workflows/codeql.yml) analyzes the Go code and the view's JavaScript on every pull request, every push to main and weekly; findings go to the repository's code scanning alerts
- [OpenSSF Scorecard](https://scorecard.dev/viewer/?uri=github.com/MichinaoShimizu/kiroku) checks the repository's security practices (pinned dependencies, token permissions, branch protection and so on) weekly and on every push to main, and publishes the result
- Go fuzz tests feed broken and crafted input to the history readers, the git remote parser and the HTML view (which must keep all history out of its one script). CI runs their seeds on every pull request, and the [Fuzz](https://github.com/MichinaoShimizu/kiroku/actions/workflows/fuzz.yml) workflow fuzzes each one weekly and on every push to main
- CI runs `govulncheck` (known vulnerabilities in code kiroku calls) and `staticcheck`, and checks that no script runs in a view built from history full of HTML and script payloads
- Dependabot proposes updates to Go modules, GitHub Actions and the npm packages used in tests every week, and security fixes right away

## Verifying a release

Each release archive comes with an SPDX SBOM (`<archive>.sbom.json`) that lists the Go version and every Go module, with its version, built into the binary. The archives, the SBOMs and `checksums.txt` all carry build provenance. (SBOMs start with the first release after v0.13.3.)

Check that a file was built by this repository's release workflow (needs the [GitHub CLI](https://cli.github.com/)):

```bash
gh attestation verify kiroku_0.13.3_linux_amd64.tar.gz --repo MichinaoShimizu/kiroku
```

From the first release after v0.14.0, each release also includes the same provenance as a signed Sigstore bundle, `kiroku_<version>.sigstore.json`. With it, `gh` checks the files without looking the attestation up on GitHub (it still fetches Sigstore's public keys):

```bash
gh attestation verify kiroku_0.15.0_linux_amd64.tar.gz --repo MichinaoShimizu/kiroku --bundle kiroku_0.15.0.sigstore.json
```

Check that a release binary is exactly what its tag's source produces. The build is reproducible: `tools/reproduce.sh` clones the tag, builds it the way `.goreleaser.yaml` does (with the Go toolchain named in the tag's `go.mod`, `CGO_ENABLED=0`, `-trimpath`, `-ldflags "-s -w -X main.version=<version>"`), downloads the release archive, checks it against `checksums.txt` and compares the two binaries byte for byte. It needs git, Go, curl and tar (and unzip for Windows targets), and can check any target from any OS.

```bash
sh tools/reproduce.sh v0.13.3 linux amd64     # os: darwin, linux, windows; arch: amd64, arm64
```

By hand, that is:

```bash
git clone --branch v0.13.3 https://github.com/MichinaoShimizu/kiroku.git && cd kiroku
GOTOOLCHAIN=go1.26.8 CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -trimpath -ldflags "-s -w -X main.version=0.13.3" -o kiroku .
sha256sum kiroku     # compare with the kiroku in kiroku_0.13.3_linux_amd64.tar.gz
```

Build from a clean clone of the tag, not from a source tarball or a modified tree: Go records the module version and commit in the binary, so a different commit or uncommitted changes give a different binary. Only the binaries are compared, not the archives around them.
