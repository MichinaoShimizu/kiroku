# Compatibility

kiroku follows [Semantic Versioning](https://semver.org/). This page says what a version number promises: what stays compatible within a major version from v1.0.0 on, and what may change in any release.

While kiroku is in 0.x, anything here may still change in a minor release (marked `BREAKING` in [CHANGELOG.md](../CHANGELOG.md)). The list is what v1.0.0 will freeze.

## Kept compatible within a major version

A change that breaks one of these is marked `BREAKING` in the changelog and, from v1.0.0 on, only comes with a new major version.

- **Commands**: `serve`, `open`, `html`, `json`, `archive`, `autostart`, `doctor`, `version`, `update` and `help`, and their arguments (such as `archive on|off` and the `ADDR` of `serve`)
- **Options**: the names, meanings and defaults listed under [Options](guide.md#options)
- **Environment variables** kiroku reads: `KIROKU_ARCHIVE_DIR`, `KIROKU_CONFIG_DIR`, and the agents' own (`CLAUDE_CONFIG_DIR`, `KIRO_HOME`, `KIROCREW_HOME`, `CODEX_HOME`)
- **Exit status**: 0 on success, and not 0 when a command fails
- **The `--prices` file**: the JSON format described in [How you used AI](guide.md#how-you-used-ai), both the object and the array form
- **`kiroku archive` copies**: a later version still reads the copies an earlier one kept, and `kiroku archive off` still recognizes the folder. The layout inside the folder may change if kiroku moves the old copies itself
- **The `kiroku serve` key** kept in kiroku's config folder: a new version still accepts it, so bookmarks and `kiroku open` keep working
- **`kiroku serve` stays local by default**: it listens on `127.0.0.1` and checks the Host header, and anything that widens access stays opt-in (see [SECURITY.md](../SECURITY.md))
- **Downloads are verified**: `kiroku update` and `install.sh` check `checksums.txt` (and build provenance where available) before they replace anything. Release archive names and `checksums.txt` keep their form so older versions of `kiroku update` and `install.sh` can still update

## May change in any release

These change as kiroku and the agents it reads change. Changes are listed in the changelog, but are not `BREAKING`.

- **The view**: layout, wording, colors, keyboard shortcuts and which metrics are shown
- **Metrics and their numbers**: how a metric is defined or computed, its name, and thresholds such as those in "Worth a look". Fixing how a history is read can change past numbers
- **Estimated cost**: the price table follows the providers' public prices, so estimates for past weeks can change
- **Which histories are read**: kiroku follows the formats the agents write. When an agent changes its format, kiroku reads the new one; a history format an agent no longer writes may stop being read in a minor release, with a note in the changelog
- **Text output** of `doctor`, `version`, `update`, `archive` and errors: it is for people, not for scripts
- **The JSON written by `kiroku json`** and the `/data.json` and other URLs of `kiroku serve`: the JSON is the data behind the view, and its fields change with the view. *Open question for v1.0.0: keep it like this, or add a versioned, documented subset that scripts can rely on*
- **The HTML written by `kiroku html`**: it is a page to open, not a format to parse
- **The legacy forms** `kiroku --serve`, `--json` and `-o` (see [Commands](guide.md#commands)): *open question for v1.0.0: remove them before 1.0, or keep them until 2.0*
- **Supported platforms and the Go version** needed to build: they follow the Go releases that are supported upstream
- **Security fixes**: when keeping something compatible would leave a security problem, the fix wins, even in a patch release. It is noted under `### Security` in the changelog

## Go packages

kiroku is a command, not a library. Everything is under `internal/`, so there is no Go API to keep compatible.
