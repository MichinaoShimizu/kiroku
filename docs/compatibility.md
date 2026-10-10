# Compatibility

kiroku follows [Semantic Versioning](https://semver.org/). This page says what a version number promises: what stays compatible within a major version from v1.0.0 on, and what may change in any release.

While kiroku is in 0.x, anything here may still change in a minor release (marked `BREAKING` in [CHANGELOG.md](../CHANGELOG.md)). The list is what v1.0.0 will freeze.

## Kept compatible within a major version

A change that breaks one of these is marked `BREAKING` in the changelog and, from v1.0.0 on, only comes with a new major version.

- **Commands**: `serve`, `open`, `html`, `json`, `stats`, `archive`, `autostart`, `doctor`, `version`, `update` and `help`, and their arguments (such as `archive on|off` and the `ADDR` of `serve`)
- **Options**: the names, meanings and defaults listed under [Options](guide.md#options)
- **Environment variables** kiroku reads: `KIROKU_ARCHIVE_DIR`, `KIROKU_CONFIG_DIR`, and the agents' own (`CLAUDE_CONFIG_DIR`, `KIRO_HOME`, `KIROCREW_HOME`, `CODEX_HOME`)
- **Exit status**: 0 on success, and not 0 when a command fails
- **The fields of `kiroku json` listed under [The JSON of kiroku json](#the-json-of-kiroku-json)**, as long as `schemaVersion` stays the same
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
- **Text output** of `doctor`, `stats`, `version`, `update`, `archive` and errors: it is for people, not for scripts. For scripts, `kiroku stats --json` keeps `schemaVersion` and `period`; its `current` and `previous` have the same form as the entries of `weeks` in `kiroku json`, and change with them
- **The rest of the JSON written by `kiroku json`** (`weeks`, `months`, `meta` and the session fields not listed below), and the `/data.json` and other URLs of `kiroku serve`: they are the data behind the view and change with it
- **The HTML written by `kiroku html`**: it is a page to open, not a format to parse
- **Supported platforms and the Go version** needed to build: they follow the Go releases that are supported upstream
- **Security fixes**: when keeping something compatible would leave a security problem, the fix wins, even in a patch release. It is noted under `### Security` in the changelog

## The JSON of kiroku json

`kiroku json` writes an object with `schemaVersion` (a number, now `1`) and `sessions` (an array, one entry per session). While `schemaVersion` stays the same, each session keeps these fields with the same name, type and meaning:

| Field | Type | Meaning |
|---|---|---|
| `id` | string | The session ID the agent recorded |
| `source` | string | The agent, as shown in the view (such as `Claude Code`, `Codex`, `Kiro IDE`) |
| `project` | string | The project name shown in the view |
| `projectPath` | string | The project folder the agent recorded (can be empty) |
| `branch` | string or `null` | The git branch the agent recorded |
| `title` | string | The title the agent recorded, or else the start of the first prompt |
| `start`, `end` | number | The first and last recorded time, in Unix seconds |
| `nPrompts` | number | Prompts you sent |
| `nFiles` | number | Files the agent changed |
| `interrupts` | number | Times you stopped the agent |
| `corrections` | number | Prompts that read like a correction of the agent (kiroku's guess from the wording) |
| `cost` | number | Estimated cost in USD, subagents included (an estimate; see [How you used AI](guide.md#how-you-used-ai)) |
| `credits` | number | Kiro credits recorded in the history (0 for other agents) |

New fields can appear at any time, so ignore the ones you don't know. Removing one of these fields or changing its name, type or meaning raises `schemaVersion` and is `BREAKING`. The numbers themselves can change when kiroku reads a history better (see "May change in any release"). The file contains your prompts and file paths, so treat it like your history.

## Go packages

kiroku is a command, not a library. Everything is under `internal/`, so there is no Go API to keep compatible.
