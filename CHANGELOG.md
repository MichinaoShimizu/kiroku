# Changelog

Notable changes to kiroku, in the style of [Keep a Changelog](https://keepachangelog.com/). Add changes under `## Unreleased`; its contents decide the next version ([Semantic Versioning](https://semver.org/), see `sh tools/next-version.sh`), and each release on GitHub uses its section here as the release notes.

## Unreleased

### Changed

- Session details count commits the same way as "Commits during this session": the card is now "Commits (by AI)", including commits made by hand during the session, and pull requests have their own card. Without git history it still shows the commits and pull requests the AI ran

## v0.3.4 - 2026-10-05

### Changed

- Year in review is hidden for now: the button next to the period controls and the `Y` shortcut no longer appear

## v0.3.3 - 2026-10-05

### Fixed

- `kiroku archive` kept nothing when the Claude Code history folder (`~/.claude/projects`) was a symbolic link, while still showing as on
- With `kiroku archive` on, a resumed Claude Code conversation lost subagent work whose files Claude Code had already deleted; kiroku now reads those from the copies

## v0.3.2 - 2026-10-04

### Changed

- Session details: a long prompt in the prompt flow opens in full with "Show all", and "Show N more" shows the prompts after the 30th (up to the first 50)

### Fixed

- The 8-week trend note on a metric that crossed a threshold was squeezed into a narrow column, one character per line, on phones and at 1000px. A marked metric now spans the full row, so the card next to it is no longer stretched with empty space
- "Review this session with AI (copy prompt)" ran past its box on narrow phones in Japanese
- A metric marked inside the collapsed "More metrics" was hidden; the section now opens and its heading shows the mark
- In the year in review, the vertical name of your light could collapse into an unreadable block when no Japanese serif font was installed, and the level was low-contrast in the light theme
- A marked metric listed the same sessions as the cards right below it, and now says how many more there are
- Japanese wording: labels now match the "Worth a look" links, units in trend notes are spaced like the text, day cells say 時間/分 instead of h/m, the empty week no longer repeats the ← → hint, and the search placeholder is no longer cut off
- Cards in By project lay out their numbers the same way regardless of the value's length

## v0.3.1 - 2026-10-04

### Changed

- The Findings panel at the top of the summary is gone. A metric that crosses a threshold is now marked (●) where it appears, with what was observed, why it matters, its 8-week trend, the related sessions and the threshold, and "Worth a look" at the top lists those metrics in priority order. The summary now starts with By project

## v0.3.0 - 2026-10-04

### Added

- Year in review (`Y`, or "Year in review" next to the period controls; 「1 年の露光」 in Japanese): your year of active time drawn like a long-exposure photo, with date across and time of day down, each session's active stretches as streaks of light colored by agent. It names the shape of your year ("Night Glow", "Long Exposure" and five more, with the rule that chose it) and makes a 1600×900 image to share, saved as a PNG or copied. The image is drawn in the page and shows only totals and the streaks; prompts, project names, branches, files and estimated cost are never included
- Skill in the year in review: four meters on a 10-step scale (shutter count for practice, focus for craft, layers for range, and noise for detours), and ten levels from Novice to Legendary before your light's name. Improving from the first half of the year to the second raises the level by one. "Overexposed" names (Blown-Out, Out-of-Film, Grainy) replace the level when you work without breaks, keep hitting usage limits or noise runs high. It can be left off the share image
- `kiroku archive on` keeps compressed copies of history that agents delete automatically (Claude Code conversations, Kiro Crew's `sessions/archive/`), and shows deleted conversations from those copies. It is off until you turn it on; the notice about Claude Code's 30-day default also offers "Keep a copy in kiroku" in `kiroku serve`. Copies stay on your computer (`--archive-dir` or `KIROKU_ARCHIVE_DIR` to choose where). `kiroku archive` shows the status, and `kiroku archive off` stops it. "Data sources" shows how many conversations came from the copies

### Changed

- The theme is now dark by default, and the button switches between dark and light. A theme saved as "auto" opens in dark
- "Cost and outputs" no longer calls commits, pull requests and edited lines 成果 (results) in Japanese; they are アウトプット (outputs), the amount produced, not its value or productivity. "Output signals" is now "Outputs"

### Fixed

- The theme button cycled auto → light → dark, so on a light system the first press stayed light and it took two presses to see a change

## v0.2.1 - 2026-10-04

### Fixed

- "Prompts with corrections or interruptions" counted ordinary requests that merely shared the words ("a different color", "add an undo button", "revert commit abc"), words in pasted errors and logs, and the first prompt of a conversation. It now looks only at the opening, human-written part of each prompt after the first, and counts phrases such as "No, that's wrong", "undo that" or 「元に戻して」. It also picks up "still not fixed" (「まだ直ってない」)
- Estimated cost from the price table ignored fast mode, which is billed at twice the standard rate, and US-only inference (1.1×). Responses recorded with `speed: "fast"` or `inference_geo: "us"` are now priced accordingly. Sessions where Claude Code records its own cost were not affected
- Edits, commits and pull requests made by Claude Code subagents were not counted. They now count toward the session's outputs
- "Sessions that reached a commit" and "Estimated cost per commit" divided by every agent's sessions and cost, although only Claude Code records outputs. They now use Claude Code sessions and cost only
- Commits an agent made in a git worktree could be shown as made by hand, and under the main checkout's project. A worktree is now read as the same repository as its main checkout
- A commit the agent ran was matched by the time it was requested, so waiting for permission or a slow pre-commit hook over 2 minutes broke the match. It now uses the time the command returned, and each run is matched to the one nearest commit, so a hand commit made right after is no longer marked as made by AI
- The "Focus blocks" explanation said breaks of up to 5 minutes; within a session, gaps up to the session gap (15 minutes by default) count as continuous

## v0.2.0 - 2026-10-04

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
