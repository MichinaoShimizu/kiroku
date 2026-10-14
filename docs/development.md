# Development

kiroku is a CLI written entirely in Go with few external libraries (SQLite via `modernc.org/sqlite` and zstd via `klauspost/compress`, built without cgo).

## Building and testing

Go 1.26 or later is required. If Node is available, `internal/web/script_test.go` also checks the view's script syntax with `node --check` (skipped otherwise). Screenshots and the demo need Python 3 and git; taking screenshots also needs Node.js and Playwright.

```bash
sh tools/check.sh  # runs gofmt, go vet, staticcheck, govulncheck, go test, go build and shellcheck as CI does, and lists what failed (--e2e adds the view's e2e)
go test ./...      # checks that the aggregates match the expected values, using synthetic data in testdata/
sh tools/changelog.sh --check && sh tools/changelog_test.sh   # the changelog fragments in changes/, and the scripts that merge them (check.sh runs both)
go vet ./...
GOTOOLCHAIN=$(go env GOVERSION) go run honnef.co/go/tools/cmd/staticcheck@2026.2.2 ./...   # static analysis (OK if nothing is printed)
GOTOOLCHAIN=$(go env GOVERSION) go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...   # known vulnerabilities in the standard library and modules that kiroku calls
go test ./internal/source -run '^$' -fuzz '^FuzzClaude$' -fuzztime 30s   # fuzz one target (list them with: go test -list '^Fuzz' ./...)
sh tools/bench-compare.sh   # the benchmark below on origin/main and on your working tree, compared with benchstat (or: sh tools/bench-compare.sh <ref>)
go test ./internal/cli -run '^$' -bench Large -benchtime 3x -benchmem   # a year of synthetic history: time to read, aggregate and render, and the size of the HTML and /data.json
gofmt -l .         # OK if nothing is printed
go build .         # builds ./kiroku (open the view with ./kiroku serve)
```

`tools/check.sh` reads the staticcheck and govulncheck versions from `.github/workflows/ci.yml`, so bump them there only. It skips shellcheck when it isn't installed.

`tools/check.sh` also runs a security lint, `go run ./tools/seclint` (its test, `TestRepo`, runs in `go test ./...` too, so CI checks it). It catches cheap, deterministic regressions of the Security rules in `CLAUDE.md`; it doesn't replace reading the diff or the XSS e2e:

- `js-html`: assigning `innerHTML` or `outerHTML`, `insertAdjacentHTML` or `document.write` in `internal/web/js` (assigning `""` is fine)
- `shell-exec`: `exec.Command` or `exec.CommandContext` running a shell (`sh`, `bash`, `cmd` and so on) or given `-c`
- `file-mode`: outside tests, `os.WriteFile` or `os.OpenFile` wider than 0600, `os.Mkdir` or `os.MkdirAll` wider than 0700, or a mode that isn't a constant
- `listen`: outside tests, `http.ListenAndServe`, `net.Listen` and the like on an address that isn't a loopback constant
- `unpinned-action`: a `uses:` in `.github/workflows` not pinned to a full 40-character commit SHA (local `./` workflows are fine)

Exceptions live in `tools/seclint/main.go`, each with its file and a reason. `allow` holds uses known to be safe: the constant icon table in `format.js`, the autostart unit file, the developer tools that write `docs/upstream`, and `kiroku serve`'s documented listen path. `baseline` holds the view's existing HTML templates, which escape history with `esc()` and are covered by `xss.mjs`. The baseline is a count per file: a new insertion fails, and when a file has fewer, the lint prints a note to lower the count (without failing, so a PR that removes HTML insertions is never blocked). An `allow` entry that no longer matches anything fails. To add one, explain why in the PR.

In Claude Code, the `/pre-pr` skill (`.claude/skills/pre-pr/SKILL.md`) is the check before every pull request: it runs `tools/check.sh`, reads the diff against the Security rules in `CLAUDE.md`, starts `hostile-tester` and `user-tester` when the change touches what they cover, and checks the changelog, docs, screenshots and benchmark. In Claude Code cloud sessions, the SessionStart hook (`.claude/hooks/session-start.sh`) installs what those checks need beforehand: the Go modules, staticcheck and govulncheck at CI's versions, shellcheck, and Playwright from `tools/screenshots/package-lock.json` (with `--ignore-scripts`), and sets `CHROMIUM` to the container's Chromium. It does nothing on your own machine.

`/pre-pr` only looks at what a change touches. The `/quality-audit` skill (`.claude/skills/quality-audit/SKILL.md`) looks at the whole project on a schedule: full once a month and before a minor release, quick once a week. It runs the same gates, then reviews security, UX in English and Japanese, wording, docs, how each agent's history is read, the report prompts, code health, ADR conformance, the supply chain and performance. It verifies each finding before fixing it, asks before anything that needs a decision, and records the results and the open items in [docs/quality/audit-log.md](quality/audit-log.md), so the next run starts from what was left.

Run the benchmark before and after a change to reading, aggregating or rendering, and put the comparison in the PR:

```bash
git fetch origin main
sh tools/bench-compare.sh          # or against a ref: sh tools/bench-compare.sh v0.36.0
```

It measures the same as `go test ./internal/cli -run '^$' -bench Large -benchmem -count 6` on the base (in a temporary `git worktree` that it removes afterwards) and on your working tree, uncommitted changes included. It builds each side's test binary once and runs them one after the other, six times, so drift in the machine's load doesn't look like a difference (running all of one side first showed +17% on identical code). It compares them with benchstat, pinned in the script and run with `go run` like staticcheck. When benchstat can't run, or with `KIROKU_BENCH_PLAIN=1`, it prints a table of means instead (no significance test). `KIROKU_BENCH_COUNT` changes the number of runs, `KIROKU_BENCH_TIME` sets `-benchtime` (for example `3x`), and `KIROKU_BENCH_DAYS` and `KIROKU_BENCH_PER_DAY` change the size of the history (365 days of 8 sessions by default).

Tests never use personal history. Everything in `testdata/` is synthetic, with made-up paths such as `/Users/me`. Never commit or attach real history, including in issues.

## Layout

| Location | Role |
|---|---|
| `main.go` | The entry point only (kept at the root so `go install github.com/MichinaoShimizu/kiroku@latest` works); the commands are in `internal/cli` |
| `internal/cli/cli.go` | Subcommands (`serve`, `open`, `html`, `json`, `stats`, `archive`, `autostart`, `doctor`, `version`, `update`, `help`), option parsing, and the message that points the removed old syntax (`kiroku --serve` and so on) to the new one |
| `internal/cli/load.go` | Loading history (removing duplicates), overriding the price table |
| `internal/cli/archive.go` | `kiroku archive` (status, `on`, and `off`, which offers to delete the copies only in a folder marked by `on`) |
| `internal/cli/update.go` | `kiroku update` (downloads from Releases, verifies, and replaces itself) |
| `internal/cli/doctor.go` | `kiroku doctor` (lists what was found and what to run next; only reads) |
| `internal/cli/autostart.go` | `kiroku autostart` (a launchd agent on macOS, a systemd user service on Linux) |
| `internal/cli/scope.go` | `kiroku html --week` / `--month` (keeps only one period's sessions, commits and pushes) |
| `internal/cli/serve.go` | `kiroku serve` (watches history for changes, reloads, and pushes to the view) |
| `internal/cli/servekey.go` | The key of `kiroku serve` (`serve-key`, `0600`; the `X-Kiroku-Key` header and the `?key=` page that stores it in the browser) |
| `internal/cli/open.go` | `kiroku open` |
| `internal/cli/stats.go` | `kiroku stats` |
| `internal/cli/cache.go` | On reload, skips history unchanged since last time (using a per-agent fingerprint and the per-conversation marks of `source.Splitter`) |
| `internal/cli/color.go` | Colour and emphasis for what is written to the terminal (`styleFor` turns it off for pipes, files, `NO_COLOR` and `TERM=dumb`) |
| `internal/ai` | "Analyze with AI" in `kiroku serve` (on unless `--no-ai`, ADR 0034): runs `claude`, `codex` or `kiro-cli` in an empty temporary folder with read-only limits (`Runner`), and the `/ai/run` handler (checks, one run at a time, size limits) |
| `internal/archive` | The copies `kiroku archive` keeps: the on/off marks, compressing history to `.zst` (`Sync`), usage, and deleting only the `.zst` copies (`Clear`) |
| `internal/source` | Adapters that read each agent's history. Details on how they read are in [sources.md](sources.md) |
| `internal/core` | The common session shape (`Builder` → `Session`), tokens and pricing, agent-specific metrics, and `Agents` (`agents.go`: each agent's name, `--sources` name, color, initials, environment variables and `kiroku archive` folder) |
| `internal/report` | Weekly and monthly aggregates (`Summarize`), per-project summaries (`project.go`), shares by branch and agent (`share.go`) |
| `internal/gitlog` | Reads commits, and pushes from the reflog, from the git repository in each session's working directory (skipped without git), with the repository's own settings that run programs turned off |
| `internal/web` | The view. Written as `template.html` (markup), `style.css` and the script in `js/*.js`, split by role (`state.js` data and view state, `format.js` helpers, `ui.js` shared components, `calendar.js` week and month calendars, `summary.js` / `review.js` / `panels.js` the summary, `metric.js` the breakdown dialogs, `git.js` / `session.js` the details panel, `ai.js` Analyze with AI, `markdown.js` escaping for copied text, `mdview.js` the formatted view of an AI reply (elements and text only, never HTML), `i18n.js` the Japanese catalog, `year.js`, `events.js` input and keyboard, `boot.js` start-up, `live.js` updates under `kiroku serve`; `loading.html` is the page `kiroku serve` shows while it first reads history); `web.go` joins the script files in the fixed order of its `scripts` list into one `<script>` and combines everything with the aggregate JSON into one HTML file. `help_test.go`, `script_test.go` and `ui_test.go` check the view's explanations, script and use of the shared components |
| `testdata/` | Synthetic history (`home/`, `codex/`, `crew/`, `sqlite/`), `golden.json`, `snapshot.json`, `mtimes.json`, and `compat/` (files an earlier kiroku wrote) |
| `tools/` | `check.sh` (runs CI's checks locally), `release.sh`, `changelog.sh`, `next-version.sh`, `release-notes.sh` and `verify-release.sh` (releases, with `changelog_test.sh` and its fixtures in `testdata/changelog/`), `imgdiff/` (compares retaken images with the committed ones), `reproduce.sh` (rebuilds a released binary from its tag and compares it), `prices/` (fetches the official price pages and writes the numbers kiroku uses to `docs/upstream/`), `reportbench/` (synthetic history with known answers, and a grader, for testing the daily, weekly and monthly report prompts with the `/report-eval` skill), `screenshots/` (dummy data with `gen.py` and `mkgit.py`, the demo's link card with `ogp.py`, screenshots with `capture.mjs`, the README's animation with `demo.mjs`, `keep-changed.sh` to keep only images that really changed, and the view's e2e: `smoke.mjs`, plus `hostile.py` and `xss.mjs` for XSS) |
| `install.sh`, `.goreleaser.yaml` | The installer, and how release files are built |

The price tables are `Prices` and `LongPrices` (Anthropic; `LongPrices` holds models priced by prompt length, such as Claude Haiku 5.5 over 100K tokens), `OpenAIPrices` and `OpenAILongPrices` (OpenAI, for Codex), and `OpenAITiers` (OpenAI's Fast, Ultrafast and Flex rates, picked by the service tier Codex records; the `### Fast` and other sections of `openai-pricing.md`) in `internal/core/usage.go`. Their source of truth is the official pricing pages, of which `docs/upstream/anthropic-pricing.md` and `docs/upstream/openai-pricing.md` keep only the numbers kiroku uses. To update them:

```bash
go run ./tools/prices     # fetches the official pages and rewrites docs/upstream/*.md (only the files whose numbers changed)
go test ./internal/core   # TestPricesMatchUpstream: the tables in usage.go must match docs/upstream
```

Then change the tables in `usage.go` to match, and `PricesAsOf` (the month shown in the view; the test fails if it is older than a snapshot's fetch date). The tool reads only the expected tables of the pages' Markdown versions and fails without writing anything when their shape changes; then fix its parser (`tools/prices/main.go`) rather than editing the snapshots by hand. The `Prices` workflow runs the same tool weekly (see CI).

In Claude Code, the `/price-update` skill (`.claude/skills/price-update/SKILL.md`) goes through these steps, including new models' context windows, the docs, the CHANGELOG and the test data, and opens one PR.

The context windows of Claude models (for Claude Code's Peak context usage and the "Long conversations" threshold) are `ContextWindows` in `internal/core/context.go`; when you update them, also change `ContextWindowAsOf`.

### Adding an agent

1. Add the agent to `core.Agents` (`internal/core/agents.go`): the name shown in the view (its `Source.Name()`), its `--sources` name (`Family`), its color and initials in the view, the environment variables that move its history, and the `kiroku archive` folder if it deletes history. The default and help of `--sources`, the environment variables `kiroku autostart` writes down, the folders `kiroku archive off` clears, and the view's colors and badges all come from this table; `TestAgentsCoverEverySource`, `TestNoAgentTablesInScripts` and `TestGuideNamesEveryAgent` check that nothing disagrees with it
2. Implement `Source` (`Name` / `Family` / `Where` / `Load`) in `internal/source` and add it to `source.All`
3. In `Load`, build a `core.Builder` for each conversation and `emit` it (times, prompts, tools, models, tokens, credits)
4. If the same conversation is also stored elsewhere, make `Builder.Key` match (only the one read first is used)
5. Record numbers only that agent tracks with `Builder.Measure`, and add their definitions to `core.NativeDefs`
6. Add the agent's `Name` to `core.Records` (`internal/core/records.go`) with what its history records (interruptions, usage limits, compactions, edited files, subagents, outputs). The view shows "Not recorded" and "… doesn't record this" instead of 0 for anything not listed, and `TestRecordsCoverEverySource` fails if the name is missing. Add a column for it to "What each agent records" in the guide; `TestGuideRecordsTable` checks the rows that come from `core.Records`
7. If `Where()` alone is not enough for what `kiroku serve` watches, implement `Watch()` (`internal/source/watch.go`)
8. If each conversation is in its own file and can be read without cross-checking other files, also implement `Splitter` (`Units` / `LoadUnit`). `kiroku serve` then reloads only the conversations that changed (without it, the whole agent is reloaded when a watched location changes)
9. For an agent that deletes history automatically, implement `Retainer` (`Retention()`) and `Keeper` (`Keep()`, which keeps a copy with `kiroku archive` and reads the copy once the original is gone); implement `Detailer` to add a note to the data sources status
10. To make its location configurable, add `source.Options`, an option in `addCommon`, and an environment variable (`Default…`)
11. Put synthetic data in `testdata/` and write tests (golden covers only the Python version's 4 histories, so check new adapters in `internal/source/<name>_test.go`). Also add it to the loading in `internal/cli/snapshot_test.go` and regenerate the snapshot
12. Update "Histories read" and "History retention" in the guide, `docs/sources.md` (including the "What history records" table), the supported agents in the README, and the help in `internal/cli/cli.go`

Aggregation (`internal/report`) and the view only see the common session shape, so you usually don't need to touch them.

To check the adapters against the agents' public sources and official docs again (after an agent changes its format or prices, or from time to time), run the `/source-audit` skill in Claude Code (`.claude/skills/source-audit/SKILL.md`). It updates the "What history records" table in `docs/sources.md` and fixes what it finds in one PR per agent.

To stop supporting an agent, run the `/drop-agent` skill (`.claude/skills/drop-agent/SKILL.md`). It removes the adapter and everything that only that agent uses (the view's colors and marks, records, metrics, test data, docs) in one PR, while keeping its options, environment variables and `--sources` name working with a warning and leaving `kiroku archive` copies in place, as [compatibility.md](compatibility.md) promises.

## Golden data

`testdata/golden.json` is the aggregate JSON. It holds the numbers the Python version (before the port to Go) produced from the same synthetic data (`testdata/home`), and `TestMatchesPythonVersion` (`internal/cli/load_test.go`) compares the Go version's numbers against them.

- Only the 4 histories the Python version had are compared (Claude Code, Kiro IDE / CLI, Kiro CLI (Classic), Kiro IDE (legacy)), with time split in Asia/Tokyo. The legacy Kiro IDE uses file modification times, so the test restores them from `testdata/mtimes.json`
- When you add a field to the JSON, add it to the exclusion list in `compare` in `internal/cli/load_test.go` (otherwise it fails as a field missing from golden)
- When a change to aggregation changes the numbers, explain why in the PR before updating golden

## Snapshot

`testdata/snapshot.json` is the Go version's output (sessions, weeks, months and data sources status) from reading all synthetic data in `testdata/` (Claude Code, Kiro IDE / CLI, Kiro CLI (Classic), Kiro Crew, Kiro CLI and Amazon Q in SQLite, Kiro IDE (legacy), Codex). `TestSnapshot` (`internal/cli/snapshot_test.go`) checks that numbers haven't changed unintentionally, including histories and fields golden doesn't cover.

`testdata/compat/<version>/` holds files kiroku itself wrote at that version: a `kiroku archive` folder (with one copy of a synthetic Claude Code session) and a `kiroku serve` key. `TestCompatArchive` and `TestCompatServeKey` (`internal/cli/compat_test.go`) check that the current version still reads them, as [compatibility.md](compatibility.md) promises. When you change how kiroku writes them, keep the old folder, add one for the new version (written by the new code) and add it to `compatVersions`.

- The comparison allows only rounding differences in numbers (which vary by OS and CPU), and reports added or removed fields as differences. Path separators are normalized to `/`, and Codex's temporary directory to `$CODEX`
- When you change aggregation or JSON fields, check that the difference is intended and explain why in the PR before regenerating

```bash
go test -run TestSnapshot -update ./internal/cli
```

## CI

On PRs and pushes to main, `.github/workflows/ci.yml` runs the following.

- `test` (Ubuntu, macOS, Windows): gofmt (except Windows), vet, staticcheck and govulncheck (Ubuntu only, pinned to 2026.2.2 and v1.9.0), tests, build
- `release-dry-run`: `goreleaser release --snapshot` with the same pinned GoReleaser and syft as the release and no Go cache (does not publish; `go mod tidy -diff` also catches an untidy go.mod, and the SBOMs are generated too), extracting release notes from the top section of the CHANGELOG, and, if that section is a version not yet tagged, checking that its number matches `tools/next-version.sh`
- `e2e`: opens the dummy-data HTML in Chromium and uses `tools/screenshots/smoke.mjs` to check that the key flows work (switching themes, moving between weeks, opening and closing session details and where focus returns after closing, the weekly report prompt, search, month view and shortcuts), that nothing overflows sideways, and that there are no script errors and no Content-Security-Policy violations, at 1440px, 1000px, 390px and 320px, and opens the main views in Japanese at the same widths to check they render without script errors or overflow and that no translated label is left in English. Playwright is pinned in `tools/screenshots/package.json` and `package-lock.json` and installed with `npm ci --ignore-scripts`; this job runs npm packages, so it does not use the Go cache. It also builds the view from synthetic history full of HTML and script payloads (`tools/screenshots/hostile.py`) and drives it with `tools/screenshots/xss.mjs` to check that no script runs, no element is injected and nothing is loaded from the network
- `install-script` (Ubuntu, macOS): runs shellcheck on `install.sh`, `tools/*.sh`, `tools/*/*.sh` and `.claude/hooks/*.sh` (Ubuntu only), actually installs the latest release, and checks `kiroku --version`; it installs once more with `KIROKU_REQUIRE_ATTESTATION=1` so the build provenance check must pass

How the workflows are locked down:

- Every third-party action is pinned to a full commit SHA with the version in a trailing comment (`uses: actions/checkout@<sha> # v7.0.1`). Dependabot updates the SHA and the comment together. To bump one by hand, resolve the tag with `git ls-remote https://github.com/<owner>/<repo> 'refs/tags/<tag>*'` (for an annotated tag, use the `^{}` line)
- GoReleaser itself is pinned (`version:` in `release.yml` and `ci.yml`), and so is syft, which GoReleaser runs to write the SBOMs (`syft-version:` of `anchore/sbom-action/download-syft` in the same two files); Dependabot tracks neither, so bump them by hand
- Each job declares the least `permissions` it needs. Only `release` (in `release.yml`) and `tag` write to the repository, only `deploy` in `pages.yml` gets `pages: write` and `id-token: write`, only `issue` in `prices.yml` and `upstream.yml` gets `issues: write` (to open the price-change or new-version issue), and `codeql.yml` and `scorecard.yml` get only `security-events: write` to upload results (plus `id-token: write` in Scorecard, to prove where a published result came from)
- `actions/checkout` runs with `persist-credentials: false` everywhere; `tag.yml` hands the token only to the step that pushes the tag
- The release build (`release.yml`), `release-dry-run`, `e2e` and the demo build run `setup-go` with `cache: false`, so a cache written by a job that runs npm packages can't leak into the release build
- Check the workflows with `go run github.com/rhysd/actionlint/cmd/actionlint@latest` after changing them

Beyond what CI checks, the `hostile-tester` agent (`.claude/agents/hostile-tester.md`) attacks a local build with crafted history for every agent, git repositories with settings that run programs, requests to `kiroku serve` without its key or with another Host, and broken or huge input, and reports what held and what broke without changing the code. Run it after changing how history is read or shown, `kiroku serve`, git or the files kiroku writes, and before a release.

Other workflows:

- `Tag` (`tag.yml`): when CHANGELOG.md changes on main (and manually). See "Making a release" below
- `Release` (`release.yml`): on a push of a `v*` tag, or when called from Tag. GoReleaser builds the archives, an SPDX SBOM per archive (`sboms:` in `.goreleaser.yaml`, written by syft) and `checksums.txt` (which covers the SBOMs too), and `attest-build-provenance` signs all of them. The signed provenance bundle is also attached to the release as `kiroku_<version>.sigstore.json`. The binaries are reproducible from the tag; see "Verifying a release" in [SECURITY.md](../SECURITY.md) and `tools/reproduce.sh`
- `CodeQL` (`codeql.yml`): on PRs, pushes to main and every Tuesday, analyzes Go (built with `go build ./...`, `setup-go` without cache) and JavaScript (`internal/web/js`, `tools/screenshots`) and uploads the results to code scanning. It is the "advanced setup", so CodeQL's default setup must stay off in Settings → Code security
- `Scorecard` (`scorecard.yml`): on pushes to main, every Monday and when branch protection rules change, runs OpenSSF Scorecard, publishes the result to scorecard.dev (the README badge) and uploads it to code scanning. Publishing makes scorecard.dev check the workflow itself: no top-level `env` or `defaults`, no workflow-level write permissions, and only `actions/checkout`, `actions/upload-artifact`, `github/codeql-action/upload-sarif`, `ossf/scorecard-action` and `step-security/harden-runner` in its job. Keep it that way, or publishing fails
- `Fuzz` (`fuzz.yml`): on pushes to main that change Go code or testdata, every Wednesday and manually, runs each `Fuzz*` test for 45 seconds. It doesn't run on PRs, so a newly found input never turns an unrelated PR red. When it finds a failing input, the job fails and uploads `testdata/fuzz/` as an artifact: reproduce it with `go test -run '<Fuzz name>/<file>' ./<package>`, fix the code, and commit the input under the package's `testdata/fuzz/<Fuzz name>/` so `go test ./...` keeps checking it
- Dependabot (`.github/dependabot.yml`): each week, one pull request each for Go modules, GitHub Actions and Playwright (npm in `tools/screenshots`) (security fixes come right away)
- `Prices` (`prices.yml`): every Monday (5:23 UTC) and manually, runs `go run ./tools/prices` and compares the result with `docs/upstream`. When the official numbers changed, it opens an issue labeled `prices` with the difference, or updates the open one (one issue at a time); it never pushes code, so updating the price tables stays a reviewed pull request. If a page's shape changed so the tool can't read it, the run fails instead. The job that fetches and parses the pages (`check`) has only `contents: read`; the job that writes the issue (`issue`) has only `issues: write`, does not check out or run any code from the repository, and gets the difference from `check` as a job output. This is the only place kiroku's project fetches price pages: kiroku itself never does
- `Upstream` (`upstream.yml`): every Monday (5:41 UTC) and manually, runs `go run ./tools/upstream`, which reads the latest versions of Claude Code and Codex from npm and of Kiro CLI and Kiro IDE from kiro.dev's changelog, and compares them with `docs/upstream/agents.md`. When one changed, it opens an issue labeled `upstream` (or updates the open one) as the signal to run `/source-audit` for that agent; it never pushes code, and a page it can't read fails the run. The tool accepts only version-shaped strings from the network. Amazon Q (no release since April 2026) and Kiro Crew (no versioned releases) are not watched. The jobs are split like `Prices`: `check` reads with `contents: read`, `issue` writes with `issues: write` and runs no repository code
- Requests: changes start from the maintainer. An issue opened by the maintainer with the "Request (maintainer)" template (title `[request]`, label `request`) is picked up every morning by a scheduled Claude Code routine that follows `.claude/skills/request/SKILL.md`: it asks on the issue when the request is unclear or touches a security or ADR decision, otherwise implements it on `claude/request-<number>` and opens a pull request that the maintainer reviews and merges. Issues and comments by anyone else are never treated as instructions; `upstream` issues and other people's reports are listed for the maintainer to turn into requests
- `Demo` (`pages.yml`): on pushes to main, every Monday (3:17 UTC) and manually, builds the dummy-data HTML and publishes it to GitHub Pages (the Live demo in the README). To use it, set Settings → Pages → Source to "GitHub Actions". The dummy data is made in Japan time, so aggregation is also split in Japan time (`TZ=Asia/Tokyo`), and when the `KIROKU_DEMO` marker is present the view shows a Japan-time clock regardless of the viewer's time zone (so it doesn't look like late-night work when opened from abroad). The demo also opens on the latest week whose weekdays (Mon–Fri) all have records, since the current week is usually thin right after the Monday rebuild

## Making a release

### Changelog entries

A PR with a user-visible change adds a fragment under `changes/` instead of editing `CHANGELOG.md`, so PRs that are open at the same time don't conflict: `changes/<short-name>.<section>.md`, where the section is `added`, `changed`, `deprecated`, `removed`, `fixed` or `security`, holding one or more `- ` bullets in English (the format is in [changes/README.md](../changes/README.md)). `sh tools/changelog.sh --check` checks them; `tools/check.sh` and CI (release-dry-run) run it. Entries already under `## Unreleased` in `CHANGELOG.md` still count and are kept.

### The version number

The version number is decided by semantic versioning from what is unreleased: the fragments in `changes/` together with anything under `## Unreleased`. `sh tools/next-version.sh` prints the next number. What counts as breaking (and what doesn't) is in [compatibility.md](compatibility.md).

| Unreleased changes (fragments and `## Unreleased`) | Part to bump |
|---|---|
| A change marked `BREAKING` (one that breaks something [compatibility.md](compatibility.md) keeps compatible, such as commands, options or `kiroku archive` copies; changes to the view or the metrics don't count) | major from 1.0, minor while in 0.x |
| Has an Added change (`*.added.md`, `### Added`) | minor |
| Anything else (Changed, Fixed, Removed and so on) | patch |

### The release pull request

Before the release, run the `/docs-refresh` skill in Claude Code (`.claude/skills/docs-refresh/SKILL.md`). It checks whether the screenshots and the docs still match the view since the last release, retakes and rewrites what is out of date, trims what has grown too long, and opens one PR (or says nothing needs changing). Merge it first.

Then, on a clean working tree:

```bash
sh tools/release.sh                   # or --no-screenshots
```

`tools/release.sh` never pushes or tags. It:

1. stops if the working tree has changes (commit them or move them to a branch; don't use a bare `git stash`)
2. fetches `origin/main`, computes the version with `next-version.sh`, and creates `claude/release-vX.Y.Z` from `origin/main`
3. runs `sh tools/changelog.sh --release vX.Y.Z`, which merges the fragments and `## Unreleased` into `## vX.Y.Z - <today, UTC>` (sections in the order Added, Changed, Deprecated, Removed, Fixed, Security, with existing entries first) and deletes the merged fragments
4. when Chromium is available (`CHROMIUM`, or Playwright's own), retakes the screenshots and keeps only the images that really changed (see "Screenshots")
5. runs `sh tools/check.sh` and `sh tools/release-notes.sh vX.Y.Z`, commits `Release vX.Y.Z`, and prints the next steps: push the branch and open the PR with the release notes as its body

If `tools/check.sh` fails, the script stops on the release branch with the changes uncommitted: fix them there, commit, and continue with the next steps by hand. To prepare the section by hand instead, run `sh tools/changelog.sh --release vX.Y.Z` (it is idempotent; a fragment merged into main after that is added to the same section when you run it again).

Merge the PR into main (its section becomes the release notes as is). On merge, `.github/workflows/tag.yml` tags the version of the top section and runs Release directly. There is no need to tag by hand.

If it didn't work, you can rerun it from Actions → Tag → Run workflow. Tagging and pushing by hand also still releases as before.

```bash
git tag v0.1.8
git push origin v0.1.8
```

If the CHANGELOG has no section for the tagged version, the release stops without being created (you can check locally with `sh tools/release-notes.sh v0.1.8`). If the number doesn't match the contents, CI (release-dry-run) and Tag also stop. CI also stops a release PR that leaves fragments in `changes/`, since they would miss the release.

### After the release

```bash
sh tools/verify-release.sh v0.1.8     # --keep leaves the downloads in place
```

It downloads the release's assets over HTTPS into a new temporary directory, never extracts or runs them, and checks that the asset names and count match `.goreleaser.yaml` and `release.yml` (read from the tag when it is fetched locally), that every archive and SBOM matches `checksums.txt` and nothing else is listed there, and, when `gh` is installed, that each archive's build provenance verifies (`gh attestation verify`, the same conditions as `install.sh`, falling back to the release's `kiroku_<version>.sigstore.json` when gh can't reach the attestation API). Without `gh` it says it skipped that check. It exits 1 when anything is wrong.

Release (`.github/workflows/release.yml`) tests on 3 OSes, then uses GoReleaser to build files and checksums for macOS, Linux and Windows (amd64 / arm64) and publishes them to Releases. It also writes an SPDX SBOM for each archive with syft, and then attests the archives, the SBOMs and `checksums.txt` with `actions/attest-build-provenance` (the job has `id-token: write` and `attestations: write`), so `gh attestation verify <file> --repo MichinaoShimizu/kiroku` and `install.sh` can check where they were built. Versions with a `-`, such as `v0.2.0-rc.1`, become prereleases (`prerelease: auto` in `.goreleaser.yaml`). The number check compares the part before the `-` (`v0.2.0`) with the result of `next-version.sh`. `tools/release.sh` makes only regular versions; for a prerelease, run `sh tools/changelog.sh --release v0.2.0-rc.1` yourself.

## Changing the view

When you change the view (`template.html`, `style.css` and `js/*.js` in `internal/web`), review not just the feature but also the information architecture, UI and UX every time.

- **Information architecture**: can users follow "what happened → why it matters → what to do next → how to check" in that order? Put the conclusion first, and tuck low-confidence numbers further in
- **Wording**: call the same thing by the same term (e.g. "Active time"). Label estimated values as estimates with their basis, and don't turn them into verdicts of good or bad. Pass text shown in the view through `t()` and add its Japanese to `js/i18n.js` (plain text, matching `{placeholders}`; `TestI18nCatalog` checks both); text copied for an AI stays English inside `inEnglish()`
- **UI**: no breakage or overlap in light and dark, on desktop (1440px), narrow screens (1000px) and phones (390px), including long labels
- **UX**: no information that can only be read by hovering (for touch devices). Can it be operated by keyboard, and does it make sense read aloud
- **Components**: build what appears in more than one place with the shared components in `js/ui.js`, not by writing the same HTML again: session cards (`sesCardH`, `sesCard`, with `sesMeta` for the line under the title), the head of the details panel (`dHead`), headings (`panelH` for a summary panel, `secH` or a plain `<h3>` inside one; never a small grey `<div>` standing in for a heading, which screen readers can't jump to), number tiles (`statH`, `norecStat`), the line shown when a list is empty (`noneH`), code with a Copy button (`codeH`), icons (`ico`) and agent badges (`agMark`). Write numbers with thousands separators through `commas` in `format.js`. Spacing, font sizes and colours go in `style.css`; a `style=""` in the script holds only positions and sizes computed from the data, and custom properties such as `--c`. `TestUIConventions` (`ui_test.go`) fails when a script outside `ui.js` builds these by hand. When you add a component, put it in `ui.js` and add its hand-written form to `uiRules`
- **Metric explanations**: they live in `HELP` (`TestHelpMatchesGuide` checks them against the "How to read the metrics" table in `docs/guide.md`)
- **Behavior**: check that the key flows work with `smoke.mjs` (see "The view's e2e" below). If you change element ids or key bindings, update `smoke.mjs` to match
- **Docs**: update the README, `docs/guide.md` and the screenshots in the same change (`/docs-refresh` catches what slips through before each release)

## The view's e2e

To run the same thing as CI's `e2e` locally, you need Node.js and Playwright.

```bash
(cd tools/screenshots && npm ci --ignore-scripts && npx playwright install chromium)
sh tools/screenshots/run.sh --html /tmp/kiroku.html
node tools/screenshots/smoke.mjs /tmp/kiroku.html   # prints ok or FAIL for each check, and exits with code 1 if any failed
```

`sh tools/check.sh --e2e` runs this and the XSS check as the `e2e` job does. `smoke.mjs` and `xss.mjs` honor `CHROMIUM` like `capture.mjs`.

The dummy data is made relative to "now", so the e2e pins it: CI's `e2e` job sets `KIROKU_FAKE_NOW` (an ISO 8601 time with an offset, such as `2026-10-14T15:00:00+09:00`), `gen.py` and `mkgit.py` use it as "now", and `smoke.mjs` starts the browser's clock at the same time. `tools/check.sh --e2e` reads the value from `ci.yml`; pass `KIROKU_FAKE_NOW` yourself to try another time. Without it, the commands above use the current time.

This only checks that things work. Clarity and wording are checked with the scenarios in [usability.md](usability.md).

## Screenshots

The README's animation (`docs/demo.gif`) and the images in the README and the guide (`docs/screenshot.png`, `docs/session.png`, `docs/worth.png`, `docs/summary.png`, `docs/year.png`, `docs/year-params.png`) and the demo's link-card image (`docs/og.png`) can be retaken from dummy data. When you change the view, update them as well.

```bash
(cd tools/screenshots && npm ci --ignore-scripts && npx playwright install chromium)
sh tools/screenshots/run.sh
```

Where Playwright can't download its Chromium, point it at an installed one with `CHROMIUM=/path/to/chromium sh tools/screenshots/run.sh`. `docs/demo.gif` also needs `ffmpeg`; without it, the GIF is left as it is.

The dummy data is dated from now, so images taken on another day differ in their dates. With `KIROKU_FAKE_NOW` (for example `2026-10-14T15:00:00+09:00`, the time CI's e2e uses), `gen.py`, `mkgit.py` and the browser clock in `capture.mjs` and `demo.mjs` all start from that time, and retaking gives the same images unless the view changed. After retaking, `sh tools/screenshots/keep-changed.sh` compares each changed image with the committed one (`go run ./tools/imgdiff`, Go's image packages) and restores the ones whose difference is only rendering noise, so only real changes are committed:

- PNG: a pixel counts as different when a channel differs by more than 24 (of 255), and the image counts as unchanged when at most 0.002% of its pixels differ. Two runs at the same `KIROKU_FAKE_NOW` differed in 0 pixels at that tolerance (15–20 pixels with no tolerance); a change of one dummy session moved 0.008% of `session.png`
- `demo.gif`: it is made from a video, so the frame count and timing shift on every run and compression adds faint color noise. Instead of pixels, it compares the scenes that stay still for at least half a second (frames reduced to 4x4-pixel blocks of brightness) and keeps the new GIF only when a still scene of one GIF is missing from the other around the same time (±1.5 s), or the length changed by more than that

`tools/release.sh` runs both with `KIROKU_FAKE_NOW` set to CI's time unless you set it yourself (`KIROKU_FAKE_NOW=` with an empty value uses the real time).

`gen.py` creates about 5 weeks of history for 4 made-up projects from Claude Code, Codex CLI (with subagents), Kiro CLI (Classic) and Kiro IDE / CLI, and `mkgit.py` creates matching git repositories, in a temporary directory. `capture.mjs` takes the week calendar, the weekly summary and the flagged metrics (`worth.png`) for last week at 1440x900 (English, light theme, Asia/Tokyo); the left column of the session whose prompt flow has the most kinds of events at 2x (`session.png`; the right column shows the dummy data's temporary paths, so it is left out); the week calendar again at 1440x754 at 2x for `og.png` (2880x1508), and the Year in review share image (1600x900) for `year.png`, then `year-params.png` from `year-params.html`, which lays the streaks of `year.png` under how to read them and how your light is chosen (keep its wording in step with `LIGHTS` in `js/year.js` and the guide). `demo.mjs` records a 1280x800 video of last week's calendar, a session, the token breakdown, search and the month view, with a dot standing in for the mouse pointer, and `run.sh` turns it into `demo.gif` (880 wide, 10 fps). `gen.py` also writes about 11 months of shorter Claude Code history before those 5 weeks, so `year.png` shows a year; when this year has less than 8 weeks of history, `capture.mjs` uses the year with the most history. The `Demo` workflow adds the link-card tags that point at `og.png` with `ogp.py`; HTML you write yourself never gets them.

`sh tools/screenshots/run.sh --html <output.html>` builds only the dummy-data HTML (no Node needed). kiroku reads only the dummy data: every history location, including the `kiroku archive` folder, points into the temporary directory, and `HOME` is replaced too (Kiro IDE before v1.0 has no option for its location), so your own history never ends up in a screenshot or the demo. It sets `KIROKU_DEMO=1` and shows a Japan-time clock from any time zone. The git repositories in the temporary directory are deleted, so commit links don't open.

## User testing

When you change the view, use the scenarios in [docs/usability.md](usability.md) to check that users can reach their goals. In Claude Code, the `user-tester` agent (`.claude/agents/user-tester.md`) tests along these scenarios.
