# Compatibility

kiroku follows [Semantic Versioning](https://semver.org/). This page says what a version number promises: what stays compatible within a major version from v1.0.0 on (before v1.0.0 it could change in a minor release, marked `BREAKING` in [CHANGELOG.md](../CHANGELOG.md)), and what may change in any release.

## Kept compatible within a major version

A change that breaks one of these is marked `BREAKING` in the changelog and, from v1.0.0 on, only comes with a new major version.

- **Commands**: `serve` (with `--check` and `--archive`), `update` and `help`, and their arguments (such as `serve --archive on|off` and the `ADDR` of `serve`). `open`, `html`, `archive`, `autostart`, `doctor` and `version` were removed in 5.0 and stop with a message naming what to use instead (`autostart` first removes a login item kiroku 4 registered), as `json` and `stats` do since 3.0
- **Options**: the names, meanings and defaults listed under [Options](guide.md#options)
- **Environment variables** kiroku reads: `KIROKU_ARCHIVE_DIR`, `KIROKU_CONFIG_DIR`, `KIROKU_LANG`, and the agents' own (`CLAUDE_CONFIG_DIR`, `KIRO_HOME`, `KIROCREW_HOME`, `CODEX_HOME`)
- **Exit status**: 0 on success, and not 0 when a command fails
- **The `--prices` file**: the JSON format described in [How you used AI](guide.md#how-you-used-ai), both the object and the array form
- **`kiroku serve --archive` copies**: a later version still reads the copies an earlier one kept, and `kiroku serve --archive off` still recognizes the folder. The layout inside the folder may change if kiroku moves the old copies itself
- **The `kiroku serve` key** kept in kiroku's config folder: a new version still accepts it, so bookmarks of the plain address (not old `?key=` addresses) keep working. How the browser keeps it is not part of this: it is now in the browser's site data instead of a cookie, and a browser set up with an older version needs the view opened by `kiroku serve` once
- **`kiroku serve` stays local by default**: it listens on `127.0.0.1` and checks the Host header, and anything that widens access stays opt-in (see [SECURITY.md](../SECURITY.md)). kiroku runs no AI agent ("Analyze with AI" was removed in 4.0; `--ai` and `--no-ai` are accepted and do nothing)
- **Downloads are verified**: `kiroku update` and `install.sh` check `checksums.txt` (and build provenance where available) before they replace anything. Release archive names and `checksums.txt` keep their form so older versions of `kiroku update` and `install.sh` can still update

## How breaking changes are released

A new major version costs everyone who scripts kiroku or packages it a round of checking, so they are rare and batched:

- **Deprecate first.** Something on the list above is first marked under `### Deprecated` in a minor release and keeps working, with a one-line notice where it is used, so it can be moved away from before it goes. Its `BREAKING:` fragment waits in a branch or an issue, not in `changes/`, until it is due.
- **Batch.** Removals that are due go out together in one major version.
- **At least 30 days apart.** `tools/release.sh` stops when a new major version would come less than 30 days after the previous one (`tools/major-gap.sh`). A security fix doesn't wait: it may break compatibility in any release (see below), and `KIROKU_MAJOR_OK=1` lets such a release through, with the reason in its pull request.

## May change in any release

These change as kiroku and the agents it reads change. Changes are listed in the changelog, but are not `BREAKING`.

- **The view**: layout, wording, colors, keyboard shortcuts and which metrics are shown
- **Metrics and their numbers**: how a metric is defined or computed, its name, and thresholds such as those in "Worth a look". Fixing how a history is read can change past numbers
- **Estimated cost**: the price table follows the providers' public prices, so estimates for past weeks can change
- **Which histories are read**: kiroku follows the formats the agents write. When an agent changes its format, kiroku reads the new one; a history format an agent no longer writes may stop being read in a minor release, with a note in the changelog
- **Text output** of `serve --check`, `serve --archive`, `--version`, `update` and errors: it is for people, not for scripts
- **The `/data.json` and other URLs of `kiroku serve`**: they are the data behind the view and change with it
- **Supported platforms and the Go version** needed to build: they follow the Go releases that are supported upstream
- **Security fixes**: when keeping something compatible would leave a security problem, the fix wins, even in a patch release. It is noted under `### Security` in the changelog

## Go packages

kiroku is a command, not a library. Everything is under `internal/`, so there is no Go API to keep compatible.
