# Changelog

Notable changes to kiroku, in the style of [Keep a Changelog](https://keepachangelog.com/). Add changes under `## Unreleased`; its contents decide the next version ([Semantic Versioning](https://semver.org/), see `sh tools/next-version.sh`), and each release on GitHub uses its section here as the release notes.

## Unreleased

### Added

- Month-end projection (estimate) in the monthly summary while the month is in progress: the estimated cost and credits if the pace so far continues. Not shown for the first 7 days or on the last day of the month

### Changed

- `kiroku serve` reloads much faster with a long history: it re-reads only the Claude Code conversations that changed (and skips agents whose history did not change), and builds the weekly and monthly summaries without scanning every session for each period
- For a week or month in progress, comparisons with the previous period (active time, estimated cost, git commits, the cost finding and the "Ask AI" prompt) use the same days of it, such as "vs 9/1–9/20", instead of the whole previous period, which always made the current one look smaller. AI commits and pull requests are not compared until the period ends

### Fixed

- A "?" in "How you spent time" or "How you used AI" opened the explanation of the same metric higher up the page instead of next to it

## v0.1.10 - 2026-10-04

### Fixed

- "Data sources" never showed that tokens from models not in the price table (such as Codex models) were left out of the estimated cost. It now shows them and names the models, and "By model" shows — instead of $0.00 for those models

## v0.1.9 - 2026-10-04

### Fixed

- Claude Code: prompts you sent while the agent was working were missing from the prompt flow and counts
- Claude Code: the automatic "This session is being continued…" summary after a context compaction was counted as a prompt, and its wording could count as a correction

## v0.1.8 - 2026-10-04

### Changed

- The Japanese view calls the "Agent" switch エージェント instead of ツール, matching the docs
- Clearer help text: `kiroku help`, `serve --help`, `update --force` and the "no history found" hint

### Fixed

- `kiroku serve :8485` (a port alone) listened on every network interface and turned off the Host check, so others on the same network could open your history. It now listens on `127.0.0.1` only; write `0.0.0.0:8485` to open it to other devices on purpose

## v0.1.7 - 2026-10-04

### Changed

- The weekly and monthly report draft opens as text you can read before copying
- Findings are just "Findings": the "Try this" box on each card is gone. What you can try for each metric is still in its "?" explanation, which "See … →" opens

### Fixed

- On phones, the page was wider than the screen, so it could be zoomed out and the session detail could open off screen
- Finding cards grew wider than the screen when a session name was long, and project cards overflowed on 320px-wide screens

## v0.1.6 - 2026-10-04

### Added

- A notice when an agent is set to delete old history, with a link to the official docs and the setting to copy (Claude Code `cleanupPeriodDays`, Kiro Crew `session.archive_retention_days`). Data sources show the oldest record and the retention period
- English guide (`docs/guide.en.md`)
- Live demo with dummy data on GitHub Pages
- `SECURITY.md` and `CONTRIBUTING.md`
- The date of the price table used for estimated cost
- Share bars in "By project": what share of active time, tokens, estimated cost and credits went to each project, branch or agent (following the Project / Branch / Agent switch), side by side, with a table of the percentages

### Changed

- The demo shows daytime work whatever time zone you open it from
- "Corrections" also count English phrases such as "that's wrong" or "try again"

### Removed

- The "Try this" button on findings and the before-and-after bar it showed. Each finding still suggests one thing to try, and its 8-week trend shows whether it changed

## v0.1.5 - 2026-10-04

### Added

- English UI, chosen from the browser language and switchable in the header. Command output is in English
- English README (the Japanese one moves to `README.ja.md`)
- Findings based on solid records, 8-week trends, and a way to check whether what you tried worked
- A weekly report draft, and search across all periods, including files and commits
- Usage-limit hits, with estimated metrics folded away
- Links to commits, changed files, pull requests and history files

### Changed

- A larger detail view that puts "what was done" and "what was left" side by side on wide screens
- Reworked information design: findings first, then things to try

### Fixed

- Kiro credits are counted whatever the case or spacing of the unit
- The detail view did not open in weeks with a session that had no model record
- Problems found in user-perspective testing

## v0.1.4 - 2026-10-04

### Added

- Details of git commits: where, what and how they changed

### Changed

- Estimated cost uses Claude Code's own cost records when they exist

### Fixed

- Kiro Crew dashboard conversation keys now match

## v0.1.3 - 2026-10-04

### Added

- Local git commits on the calendar and in summaries
- The request flow of Kiro Crew sessions, filled in from Crew's conversation records

## v0.1.2 - 2026-10-04

### Added

- `install.sh` for one-line installs on macOS and Linux
- `kiroku version` and `kiroku update`
- Per-project summaries in the weekly and monthly views
- A prompt that asks an AI for improvement ideas
- Outcome marks: commits, pull requests created and lines changed
- Metric explanations shown by default

### Changed

- Subcommands are the main interface, with `serve` first
- Session details open in a centered modal
- `serve` re-reads history after writing settles

### Removed

- The Markdown export

### Fixed

- Layout at phone width

## v0.1.1 - 2026-10-03

### Added

- Kiro Crew credits from its usage records

## v0.1.0 - 2026-10-03

First release: a calendar of Claude Code, Kiro, Amazon Q Developer CLI and Codex sessions, with weekly and monthly summaries, tokens, estimated cost and Kiro credits, and a live view with `kiroku serve`.
