# Development

kiroku is a CLI written entirely in Go with few external libraries (SQLite via `modernc.org/sqlite` and zstd via `klauspost/compress`, built without cgo).

## Building and testing

Go 1.25 or later is required. If Node is available, `internal/web/script_test.go` also checks the view's script syntax with `node --check` (skipped otherwise). Screenshots and the demo need Python 3 and git; taking screenshots also needs Node.js and Playwright.

```bash
go test ./...      # checks that the aggregates match the expected values, using synthetic data in testdata/
go vet ./...
GOTOOLCHAIN=$(go env GOVERSION) go run honnef.co/go/tools/cmd/staticcheck@2025.1.1 ./...   # static analysis (OK if nothing is printed)
GOTOOLCHAIN=$(go env GOVERSION) go run golang.org/x/vuln/cmd/govulncheck@v1.8.0 ./...   # known vulnerabilities in the standard library and modules that kiroku calls
go test ./internal/source -run '^$' -fuzz '^FuzzClaude$' -fuzztime 30s   # fuzz one target (list them with: go test -list '^Fuzz' ./...)
gofmt -l .         # OK if nothing is printed
go build .         # builds ./kiroku (open the view with ./kiroku serve)
```

Tests never use personal history. Everything in `testdata/` is synthetic, with made-up paths such as `/Users/me`. Never commit or attach real history, including in issues.

## Layout

| Location | Role |
|---|---|
| `main.go` | The entry point only (kept at the root so `go install github.com/MichinaoShimizu/kiroku@latest` works); the commands are in `internal/cli` |
| `internal/cli/cli.go` | Subcommands (`serve`, `html`, `json`, `archive`, `autostart`, `doctor`, `version`, `update`, `help`), option parsing, and the old syntax (`kiroku --serve` and so on) |
| `internal/cli/load.go` | Loading history (removing duplicates), overriding the price table |
| `internal/cli/archive.go` | `kiroku archive` (status, `on`, and `off`, which offers to delete the copies only in a folder marked by `on`) |
| `internal/cli/update.go` | `kiroku update` (downloads from Releases, verifies, and replaces itself) |
| `internal/cli/doctor.go` | `kiroku doctor` (lists what was found and what to run next; only reads) |
| `internal/cli/autostart.go` | `kiroku autostart` (a launchd agent on macOS, a systemd user service on Linux) |
| `internal/cli/scope.go` | `kiroku html --week` / `--month` (keeps only one period's sessions, commits and pushes) |
| `internal/cli/serve.go` | `kiroku serve` (watches history for changes, reloads, and pushes to the view) |
| `internal/cli/cache.go` | On reload, skips history unchanged since last time (using a per-agent fingerprint and the per-conversation marks of `source.Splitter`) |
| `internal/cli/color.go` | Colour and emphasis for what is written to the terminal (`styleFor` turns it off for pipes, files, `NO_COLOR` and `TERM=dumb`) |
| `internal/archive` | The copies `kiroku archive` keeps: the on/off marks, compressing history to `.zst` (`Sync`), usage, and deleting only the `.zst` copies (`Clear`) |
| `internal/source` | Adapters that read each agent's history. Details on how they read are in [sources.md](sources.md) |
| `internal/core` | The common session shape (`Builder` → `Session`), tokens and pricing, agent-specific metrics |
| `internal/report` | Weekly and monthly aggregates (`Summarize`), per-project summaries (`project.go`), shares by branch and agent (`share.go`) |
| `internal/gitlog` | Reads commits, and pushes from the reflog, from the git repository in each session's working directory (skipped without git), with the repository's own settings that run programs turned off |
| `internal/web` | The view. Written as `template.html` (markup), `style.css` and the script in `js/*.js`, split by role (`state.js` data and view state, `format.js` helpers, `calendar.js` week and month calendars, `summary.js` / `review.js` / `panels.js` the summary, `git.js` / `session.js` the details panel, `year.js`, `events.js` input and keyboard, `boot.js` start-up, `live.js` updates under `kiroku serve`; `loading.html` is the page `kiroku serve` shows while it first reads history); `web.go` joins the script files in the fixed order of its `scripts` list into one `<script>` and combines everything with the aggregate JSON into one HTML file. `help_test.go` and `script_test.go` check the view's explanations and script |
| `testdata/` | Synthetic history (`home/`, `codex/`, `crew/`, `sqlite/`), `golden.json`, `snapshot.json`, `mtimes.json` |
| `tools/` | `release-notes.sh` and `next-version.sh` (releases), `reproduce.sh` (rebuilds a released binary from its tag and compares it), `screenshots/` (dummy data with `gen.py` and `mkgit.py`, the demo's link card with `ogp.py`, screenshots with `capture.mjs`, and the view's e2e: `smoke.mjs`, plus `hostile.py` and `xss.mjs` for XSS) |
| `install.sh`, `.goreleaser.yaml` | The installer, and how release files are built |

The price table is `Prices` in `internal/core/usage.go`. When you update it, also change `PricesAsOf` (the date shown in the view).

### Adding an agent

1. Implement `Source` (`Name` / `Family` / `Where` / `Load`) in `internal/source` and add it to `source.All`
2. In `Load`, build a `core.Builder` for each conversation and `emit` it (times, prompts, tools, models, tokens, credits)
3. If the same conversation is also stored elsewhere, make `Builder.Key` match (only the one read first is used)
4. Record numbers only that agent tracks with `Builder.Measure`, and add their definitions to `core.NativeDefs`
5. If `Where()` alone is not enough for what `kiroku serve` watches, implement `Watch()` (`internal/source/watch.go`)
6. If each conversation is in its own file and can be read without cross-checking other files, also implement `Splitter` (`Units` / `LoadUnit`). `kiroku serve` then reloads only the conversations that changed (without it, the whole agent is reloaded when a watched location changes)
7. For an agent that deletes history automatically, implement `Retainer` (`Retention()`) and `Keeper` (`Keep()`, which keeps a copy with `kiroku archive` and reads the copy once the original is gone); implement `Detailer` to add a note to the data sources status
8. For a new `Family`, add it to the default of `--sources` in `internal/cli/cli.go`. To make its location configurable, add `source.Options`, an option in `addCommon`, and an environment variable (`Default…`)
9. Put synthetic data in `testdata/` and write tests (golden covers only the Python version's 4 histories, so check new adapters in `internal/source/<name>_test.go`). Also add it to the loading in `internal/cli/snapshot_test.go` and regenerate the snapshot
10. Update "Histories read" and "History retention" in the guide, `docs/sources.md`, the supported agents in the README, and the help in `internal/cli/cli.go`

Aggregation (`internal/report`) and the view only see the common session shape, so you usually don't need to touch them.

## Golden data

`testdata/golden.json` is the aggregate JSON. It holds the numbers the Python version (before the port to Go) produced from the same synthetic data (`testdata/home`), and `TestMatchesPythonVersion` (`internal/cli/load_test.go`) compares the Go version's numbers against them.

- Only the 4 histories the Python version had are compared (Claude Code, Kiro IDE, Kiro CLI, Kiro IDE (legacy)), with time split in Asia/Tokyo. The legacy Kiro IDE uses file modification times, so the test restores them from `testdata/mtimes.json`
- When you add a field to the JSON, add it to the exclusion list in `compare` in `internal/cli/load_test.go` (otherwise it fails as a field missing from golden)
- When a change to aggregation changes the numbers, explain why in the PR before updating golden

## Snapshot

`testdata/snapshot.json` is the Go version's output (sessions, weeks, months and data sources status) from reading all synthetic data in `testdata/` (Claude Code, Kiro IDE, Kiro CLI, Kiro Crew, Kiro CLI and Amazon Q in SQLite, Kiro IDE (legacy), Codex). `TestSnapshot` (`internal/cli/snapshot_test.go`) checks that numbers haven't changed unintentionally, including histories and fields golden doesn't cover.

- The comparison allows only rounding differences in numbers (which vary by OS and CPU), and reports added or removed fields as differences. Path separators are normalized to `/`, and Codex's temporary directory to `$CODEX`
- When you change aggregation or JSON fields, check that the difference is intended and explain why in the PR before regenerating

```bash
go test -run TestSnapshot -update ./internal/cli
```

## CI

On PRs and pushes to main, `.github/workflows/ci.yml` runs the following.

- `test` (Ubuntu, macOS, Windows): gofmt (except Windows), vet, staticcheck and govulncheck (Ubuntu only, pinned to 2025.1.1 and v1.8.0), tests, build
- `release-dry-run`: `goreleaser release --snapshot` with the same pinned GoReleaser and syft as the release and no Go cache (does not publish; `go mod tidy -diff` also catches an untidy go.mod, and the SBOMs are generated too), extracting release notes from the top section of the CHANGELOG, and, if that section is a version not yet tagged, checking that its number matches `tools/next-version.sh`
- `e2e`: opens the dummy-data HTML in Chromium and uses `tools/screenshots/smoke.mjs` to check that the key flows work (switching themes, moving between weeks, opening and closing session details and where focus returns after closing, the weekly report draft, search, month view and shortcuts), that nothing overflows sideways, and that there are no script errors and no Content-Security-Policy violations, at 1440px, 1000px, 390px and 320px. Playwright is pinned in `tools/screenshots/package.json` and `package-lock.json` and installed with `npm ci --ignore-scripts`; this job runs npm packages, so it does not use the Go cache. It also builds the view from synthetic history full of HTML and script payloads (`tools/screenshots/hostile.py`) and drives it with `tools/screenshots/xss.mjs` to check that no script runs, no element is injected and nothing is loaded from the network
- `install-script` (Ubuntu, macOS): runs shellcheck on `install.sh` and `tools/*.sh` (Ubuntu only), actually installs the latest release, and checks `kiroku --version`; it installs once more with `KIROKU_REQUIRE_ATTESTATION=1` so the build provenance check must pass

How the workflows are locked down:

- Every third-party action is pinned to a full commit SHA with the version in a trailing comment (`uses: actions/checkout@<sha> # v7.0.1`). Dependabot updates the SHA and the comment together. To bump one by hand, resolve the tag with `git ls-remote https://github.com/<owner>/<repo> 'refs/tags/<tag>*'` (for an annotated tag, use the `^{}` line)
- GoReleaser itself is pinned (`version:` in `release.yml` and `ci.yml`), and so is syft, which GoReleaser runs to write the SBOMs (`syft-version:` of `anchore/sbom-action/download-syft` in the same two files); Dependabot tracks neither, so bump them by hand
- Each job declares the least `permissions` it needs. Only `release` (in `release.yml`) and `tag` write to the repository, only `deploy` in `pages.yml` gets `pages: write` and `id-token: write`, and `codeql.yml` and `scorecard.yml` get only `security-events: write` to upload results (plus `id-token: write` in Scorecard, to prove where a published result came from)
- `actions/checkout` runs with `persist-credentials: false` everywhere except `tag.yml`, which pushes the tag
- The release build (`release.yml`), `release-dry-run`, `e2e` and the demo build run `setup-go` with `cache: false`, so a cache written by a job that runs npm packages can't leak into the release build
- Check the workflows with `go run github.com/rhysd/actionlint/cmd/actionlint@latest` after changing them

Other workflows:

- `Tag` (`tag.yml`): when CHANGELOG.md changes on main (and manually). See "Making a release" below
- `Release` (`release.yml`): on a push of a `v*` tag, or when called from Tag. GoReleaser builds the archives, an SPDX SBOM per archive (`sboms:` in `.goreleaser.yaml`, written by syft) and `checksums.txt` (which covers the SBOMs too), and `attest-build-provenance` signs all of them. The signed provenance bundle is also attached to the release as `kiroku_<version>.sigstore.json`. The binaries are reproducible from the tag; see "Verifying a release" in [SECURITY.md](../SECURITY.md) and `tools/reproduce.sh`
- `CodeQL` (`codeql.yml`): on PRs, pushes to main and every Tuesday, analyzes Go (built with `go build ./...`, `setup-go` without cache) and JavaScript (`internal/web/js`, `tools/screenshots`) and uploads the results to code scanning. It is the "advanced setup", so CodeQL's default setup must stay off in Settings → Code security
- `Scorecard` (`scorecard.yml`): on pushes to main, every Monday and when branch protection rules change, runs OpenSSF Scorecard, publishes the result to scorecard.dev (the README badge) and uploads it to code scanning. Publishing makes scorecard.dev check the workflow itself: no top-level `env` or `defaults`, no workflow-level write permissions, and only `actions/checkout`, `actions/upload-artifact`, `github/codeql-action/upload-sarif`, `ossf/scorecard-action` and `step-security/harden-runner` in its job. Keep it that way, or publishing fails
- `Fuzz` (`fuzz.yml`): on pushes to main that change Go code or testdata, every Wednesday and manually, runs each `Fuzz*` test for 45 seconds. It doesn't run on PRs, so a newly found input never turns an unrelated PR red. When it finds a failing input, the job fails and uploads `testdata/fuzz/` as an artifact: reproduce it with `go test -run '<Fuzz name>/<file>' ./<package>`, fix the code, and commit the input under the package's `testdata/fuzz/<Fuzz name>/` so `go test ./...` keeps checking it
- Dependabot (`.github/dependabot.yml`): each week, one pull request each for Go modules, GitHub Actions and Playwright (npm in `tools/screenshots`) (security fixes come right away)
- `Demo` (`pages.yml`): on pushes to main, every Monday (3:17 UTC) and manually, builds the dummy-data HTML and publishes it to GitHub Pages (the Live demo in the README). To use it, set Settings → Pages → Source to "GitHub Actions". The dummy data is made in Japan time, so aggregation is also split in Japan time (`TZ=Asia/Tokyo`), and when the `KIROKU_DEMO` marker is present the view shows a Japan-time clock regardless of the viewer's time zone (so it doesn't look like late-night work when opened from abroad). The demo also opens on the latest week whose weekdays (Mon–Fri) all have records, since the current week is usually thin right after the Monday rebuild

## Making a release

The version number is decided from the contents of `## Unreleased` by semantic versioning. `sh tools/next-version.sh` prints the next number.

| Contents of Unreleased | Part to bump |
|---|---|
| A change marked `BREAKING` (one that breaks compatibility in usage, such as commands, options or output files; changes only to the view's appearance don't count) | major from 1.0, minor while in 0.x |
| Has `### Added` | minor |
| Anything else (`### Changed`, `### Fixed`, `### Removed` and so on) | patch |

Make a PR that renames `## Unreleased` in `CHANGELOG.md` to that number, like `## v0.1.8 - 2026-10-05`, and merge it into main (write it in English; its contents become the release notes as is). On merge, `.github/workflows/tag.yml` tags the version of the top section and runs Release directly. There is no need to tag by hand.

If it didn't work, you can rerun it from Actions → Tag → Run workflow. Tagging and pushing by hand also still releases as before.

```bash
git tag v0.1.8
git push origin v0.1.8
```

In PRs with changes, add user-visible changes to `## Unreleased` in English (right after a release, when the heading is missing, create `## Unreleased` at the top). If the CHANGELOG has no section for the tagged version, the release stops without being created (you can check locally with `sh tools/release-notes.sh v0.1.8`). If the number doesn't match the contents, CI (release-dry-run) and Tag also stop.

Release (`.github/workflows/release.yml`) tests on 3 OSes, then uses GoReleaser to build files and checksums for macOS, Linux and Windows (amd64 / arm64) and publishes them to Releases. It also writes an SPDX SBOM for each archive with syft, and then attests the archives, the SBOMs and `checksums.txt` with `actions/attest-build-provenance` (the job has `id-token: write` and `attestations: write`), so `gh attestation verify <file> --repo MichinaoShimizu/kiroku` and `install.sh` can check where they were built. Versions with a `-`, such as `v0.2.0-rc.1`, become prereleases (`prerelease: auto` in `.goreleaser.yaml`). The number check compares the part before the `-` (`v0.2.0`) with the result of `next-version.sh`.

## Changing the view

When you change the view (`template.html`, `style.css` and `js/*.js` in `internal/web`), review not just the feature but also the information architecture, UI and UX every time.

- **Information architecture**: can users follow "what happened → why it matters → what to do next → how to check" in that order? Put the conclusion first, and tuck low-confidence numbers further in
- **Wording**: call the same thing by the same term (e.g. "Active time"). Label estimated values as estimates with their basis, and don't turn them into verdicts of good or bad
- **UI**: no breakage or overlap in light and dark, on desktop (1440px), narrow screens (1000px) and phones (390px), including long labels
- **UX**: no information that can only be read by hovering (for touch devices). Can it be operated by keyboard, and does it make sense read aloud
- **Metric explanations**: they live in `HELP` (`TestHelpMatchesGuide` checks them against the "How to read the metrics" table in `docs/guide.md`)
- **Behavior**: check that the key flows work with `smoke.mjs` (see "The view's e2e" below). If you change element ids or key bindings, update `smoke.mjs` to match
- **Docs**: update the README, `docs/guide.md` and the screenshots in the same change

## The view's e2e

To run the same thing as CI's `e2e` locally, you need Node.js and Playwright.

```bash
(cd tools/screenshots && npm ci --ignore-scripts && npx playwright install chromium)
sh tools/screenshots/run.sh --html /tmp/kiroku.html
node tools/screenshots/smoke.mjs /tmp/kiroku.html   # prints ok or FAIL for each check, and exits with code 1 if any failed
```

This only checks that things work. Clarity and wording are checked with the scenarios in [usability.md](usability.md).

## Screenshots

The images in the README and the guide (`docs/screenshot.png`, `docs/session.png`, `docs/worth.png`, `docs/summary.png`, `docs/report.png`, `docs/year.png`) and the demo's link-card image (`docs/og.png`) can be retaken from dummy data. When you change the view, update them as well.

```bash
(cd tools/screenshots && npm ci --ignore-scripts && npx playwright install chromium)
sh tools/screenshots/run.sh
```

`gen.py` creates about 5 weeks of Claude Code history for 4 made-up projects, and `mkgit.py` creates matching git repositories, in a temporary directory. `capture.mjs` takes the week calendar, the weekly summary, the flagged metrics (`worth.png`) and the weekly report draft (`report.png`) for last week at 1440x900 (dark theme, Asia/Tokyo); the left column of the session whose prompt flow has the most kinds of events at 2x (`session.png`; the right column shows the dummy data's temporary paths, so it is left out); the week calendar again at 1440x754 at 2x for `og.png` (2880x1508), and the Year in review share image (1600x900) for `year.png`. While Year in review is hidden (`YEAR_ON` in `js/state.js`), `year.png` is left as it is. The `Demo` workflow adds the link-card tags that point at `og.png` with `ogp.py`; HTML you write yourself never gets them.

`sh tools/screenshots/run.sh --html <output.html>` builds only the dummy-data HTML (no Node needed). It sets `KIROKU_DEMO=1` and shows a Japan-time clock from any time zone. The git repositories in the temporary directory are deleted, so commit links don't open.

## User testing

When you change the view, use the scenarios in [docs/usability.md](usability.md) to check that users can reach their goals. In Claude Code, the `user-tester` agent (`.claude/agents/user-tester.md`) tests along these scenarios.
