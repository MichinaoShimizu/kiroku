# kiroku guide

This guide supplements the [README](../README.md). It covers installation and commands in detail, how to read the view, metric definitions, which histories are read, and options.

## Install

### macOS and Linux (recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/MichinaoShimizu/kiroku/main/install.sh | sh
```

This downloads the file for your OS and CPU (Intel / Apple Silicon and ARM) from [Releases](https://github.com/MichinaoShimizu/kiroku/releases), verifies it with `checksums.txt`, and places it in `/usr/local/bin` (or `~/.local/bin` if that is not writable). You can set the version and install location with environment variables.

```bash
curl -fsSL https://raw.githubusercontent.com/MichinaoShimizu/kiroku/main/install.sh | KIROKU_VERSION=v0.1.7 KIROKU_INSTALL_DIR=~/bin sh
```

| Variable | Meaning |
| --- | --- |
| `KIROKU_VERSION` | The version to install (e.g. `v0.1.7`). The latest if not set |
| `KIROKU_INSTALL_DIR` | Where to place `kiroku`. `/usr/local/bin` if not set (`~/.local/bin` if that is not writable) |
| `KIROKU_SKIP_ATTESTATION=1` | Do not check the build provenance with `gh`, even if it is installed |
| `KIROKU_REQUIRE_ATTESTATION=1` | Require the build provenance check: stop instead of installing when `gh` is missing or not logged in, cannot reach GitHub, or the version is older than v0.12.0 |

Each release file from v0.12.0 on also carries a signed record that it was built from this repository by its release workflow (a GitHub artifact attestation). If the [GitHub CLI](https://cli.github.com/) (`gh`) is installed, `install.sh` also checks this record with `gh attestation verify` after `checksums.txt` (built by `release.yml` on a GitHub-hosted runner from `main` or the release tag), and stops if it does not match. If `gh` is not logged in, cannot reach GitHub or is too old to have `gh attestation`, it shows a warning and installs anyway (the file already matched `checksums.txt`), unless `KIROKU_REQUIRE_ATTESTATION=1` is set. Without `gh`, and for versions before v0.12.0, it installs as before (again unless `KIROKU_REQUIRE_ATTESTATION=1` is set). The whole script is wrapped in a function that runs only from its last line, so a download cut off halfway runs nothing. You can also check a file you downloaded yourself:

```bash
gh attestation verify kiroku_<version>_darwin_arm64.tar.gz --repo MichinaoShimizu/kiroku
```

### Windows

Download `kiroku_<version>_windows_<amd64 or arm64>.zip` from [Releases](https://github.com/MichinaoShimizu/kiroku/releases) and extract it.

### Go

Go 1.26 or later is required.

```bash
go install github.com/MichinaoShimizu/kiroku@latest
```

> On macOS, opening a `kiroku` downloaded with a browser may show "“kiroku” Not Opened" (because it is not notarized by Apple). `install.sh` and `go install` do not trigger this warning. If you downloaded it with a browser, run `xattr -d com.apple.quarantine ./kiroku`, or open it with "Open Anyway" in System Settings → Privacy & Security.

## Commands

| Command | Description |
|---|---|
| `kiroku serve [ADDR]` | Serves the view at `http://localhost:8484/` and keeps it up to date. It answers right away: while it reads your history for the first time (which can take a while with a lot of history or many git repositories), the page shows "Reading your history…" with each agent as it is read (its number of sessions and how long it took) and switches to the view when it is done, and the terminal shows how many sessions each agent had, how long each one took, and the total. If the port is already in use, it stops at once without reading history. Every few seconds it checks the history folders for changes (file names, sizes and modification times only) and reloads once writing settles (when changes stop for 10 seconds; even if an agent keeps writing, it reloads within 60 seconds at most). New history appears while the week or month you are viewing and the selected session stay as they are, and `LIVE` is shown at the top left. Change the port with, for example, `kiroku serve :8485` (a bare port still listens on `127.0.0.1` only). Only browsers that have its key can open the view (see "[The key of kiroku serve](#the-key-of-kiroku-serve)"); the browser it opens gets the key. Press Ctrl+C to quit |
| `kiroku open [ADDR]` | Opens the view of a running `kiroku serve` in your browser with its key. A browser needs this once; after that, `http://localhost:8484/` (or your bookmark) opens it. `--print` prints the address with the key instead, to open the view on another device. ADDR defaults to the address `kiroku autostart` uses, or `127.0.0.1:8484` |
| `kiroku html [-o FILE]` | Reads all history up to now and writes it to a single HTML file, then opens it in your browser (`--no-open` to skip). Use it to carry the view around or to look at it without starting a server. Run it again to include later history. The file is readable only by you (`0600`). `--week` or `--month` writes only one period (see "[Share one week or month](#share-one-week-or-month)") |
| `kiroku json [-o FILE]` | Writes the aggregated data as JSON (`-o -` for stdout). Like `kiroku html`, it writes a file only you can read (`0600`). The fields scripts can rely on are listed in [Compatibility](compatibility.md#the-json-of-kiroku-json) |
| `kiroku stats` | Shows a day, week or month of your AI work in the terminal, without opening a browser: active time, sessions, prompts, tokens, estimated cost, Kiro credits, Git commits, cost per commit and rework rate, each next to the period before, then a bar for each day, and the top projects, agents, models, longest sessions and sessions worth a look. With no option it shows this week; `--day today`, `--week last` or `--month 2026-09` choose another period. A period still in progress is compared with the period before up to the same day (this week through Wednesday with last week through Wednesday). `--project` limits it to some projects, `--top` sets how many rows each list shows, and `--json` prints both periods as JSON for scripts ([Compatibility](compatibility.md#may-change-in-any-release)). It only reads and writes no file |
| `kiroku doctor` | Checks your setup in one go and only reads: which histories were found (sessions and the oldest date for each agent, and where it looked), whether an agent will delete old history (Claude Code on its 30-day default) and which settings file to change, whether `kiroku archive` is on, whether git is found, whether autostart is on, which kiroku is running, and whether a newer kiroku is out (it asks GitHub Releases while it reads history; `--no-update-check` skips this, and builds from source are not checked). Agents with no history yet are summed up in one line, so the lines that ask you to do something stay in view; `--all` lists each of them with the place kiroku looks. If another kiroku comes first in your `PATH`, so that `kiroku` runs that one instead, it says so. It ends with what to run next, including `kiroku update` when a newer release exists. Run it right after installing, or when something you expect is missing from the view |
| `kiroku autostart [on\|off]` | Starts `kiroku serve` in the background each time you log in, so the view is always at `http://localhost:8484/` (see "[Start kiroku when you log in](#start-kiroku-when-you-log-in)"). With no argument, shows the status |
| `kiroku archive [on\|off]` | Keeps compressed copies of history that agents delete automatically (Claude Code, Kiro Crew) in kiroku's own folder (see "[Keep a copy of history in kiroku](#keep-a-copy-of-history-in-kiroku)"). With no argument, shows the status (on or off, location, number and size of files) |
| `kiroku version` | Prints the version |
| `kiroku update` | Downloads the latest release for the same OS and CPU from GitHub Releases, verifies it with `checksums.txt`, and replaces itself. In a terminal it shows how much of the download has arrived. In a location you cannot write to (such as `/usr/local/bin`), run `sudo kiroku update`. kiroku installed with `go install` or built from source (version `dev`) is not replaced, so update it with `go install …@latest`. `--to v1.2.3` installs a chosen version (only version names of that form are accepted); going back to an older version than the one running also needs `--force`. v0.1.1 and earlier have no `update`, so reinstall once with `install.sh` or from Releases |

`kiroku help` lists commands; `kiroku <command> --help` shows its options.

The old forms `kiroku --serve`, `--json`, `-o` and `--weekly` / `--monthly` have been removed; kiroku names the command to use instead.

## View

- The vertical week calendar shows sessions as bars. The more messages per minute, the darker the bar, and sessions that overlap in time are placed side by side. Short bars show their name in smaller type, and bars too short for that show it just below when there is room. In narrow bars, names wrap between words and are shortened with "…"; a bar too narrow for the first word (or about 4 letters) shows its start time instead, or only its color when even that does not fit. Hover or click it for the name
- Switch between "Week / Month" at the top right. The month calendar shows each day's active time as color intensity and the project breakdown as thin bars. Each day lists its active time, tokens or credits, sessions and Git commits, each labeled, and the column on the left shows the same for each week (on phones, only active time and usage, with tokens and credits on their own lines when there are both). Tokens and credits follow the week or month shown: tokens when it has only tokens, credits when it has only credits, and both on every day (with "—" where there are none) when it has both. Point at a day or week to also see estimated cost. The day headings of the week calendar show the same. Click a date, or "W##" on the left, to go to that week in the week calendar
- Above the calendar, the key figures for the week or month are shown. The most important ones come first, on a tinted background: Tokens, Estimated cost, Month-end cost (estimate), Kiro credits and Month-end credits (estimate); the month-end estimates appear only in the month view while the month is in progress (see the metrics table). Then come Active time, Active days, Sessions / prompts, Usage limit hits and Git commits with how many AI ran, such as "22 · 14 by AI". Active days counts days with any active time; for a week or month still in progress it is out of the days so far, including today (such as "6 of 6 so far"), not the whole period. Items with no records are not shown. When the figures don't fit next to the period's name, they move to their own row
- Click a key figure, or any figure in the summary's panels, to open its breakdown in a dialog. What the metric counts is right under the figure; the finer points and what it doesn't tell you are at the end:
  - Most figures: by day, by agent, by project and, for tokens and estimated cost, by model, then the top sessions (press one to open its details). The day, agent, project and model rows come from the same totals as the figure, so they add up to it; the session figures cover each whole session, including any part outside the period
  - The month-end estimates show how they are worked out (so far ÷ days so far × days in the month), with the days still to come drawn as hatched bars at the current pace
  - Usage limit hits and compactions are split by agent and by project and list each one with its session (and, for limit hits, the reset time)
  - Git commits are split by project, by whether AI made them and by which agent ran them, with the list of commits
  - The other figures in the panels: commits, pushes and pull requests list each one; focus blocks, oversized prompts and the session figures list the blocks, prompts or sessions behind them; wait time is split by how long you took to reply; and ratios such as cost per prompt show how they are worked out
- The period is written like "Sep 28 – Oct 4" or "October 2026" (in English, whatever the browser's language), and the line above it says "This week", "Last week", "This month" or "Last month" when that is what you are looking at. ‹ › (or ← →) move to the previous or next week or month. You can't move past the current week or month; "This week" / "This month" brings you back to it. A period with no records is named by its week or month, such as "Week of Sep 28"
- Each day in the calendar shows its own figures (in the day headings of the week calendar and in each day of the month calendar). Point at a bar, band or point in any chart (or touch it) to see the values for that day or item
- "Worth a look", just under the key figures, lists the metrics that crossed a threshold in priority order; press one to open it in a dialog in the middle of the screen, without leaving where you are: what was observed, why it matters, its 8-week trend, why it was flagged, the related sessions (press one to open its details), the threshold, and what the metric is, what it doesn't tell you and what to try. `Esc`, the × in its top-right corner or a click outside the dialog returns to the link you pressed
- "Color by" Project / Branch / Agent is the selector at the start of the legend. The numbers in the legend are active time, split between sessions that ran at the same time like in "① By project"; click an item to show or hide it. In the week calendar, "Zoom" − + at the right end of the legend (or `+` `−`) changes the height of an hour. Until you zoom, each week picks the height (44px, or 36px when needed) that fits its working hours, from the morning start to the latest end, in the calendar; after you zoom, your choice is kept
- Click a bar to show its details. The resume command is at the top, under the title (on wide screens, at the top of the right column); then come the prompt flow with times, subagents and files changed (each path is a link: to the commit on the remote when there is one, otherwise to the file on this computer in the HTML file), with models, tools and the agent's own metrics folded under one line on the right. On wide screens, numbers and the prompt flow (what was done) are on the left, and commits, pull requests and files changed (what was left behind) are on the right
  - When the session is behind a metric flagged in "Worth a look" for the period shown, those metrics are listed under the title; press one to open the same dialog
  - The prompt flow puts your prompts and what happened between them (commits, pull requests created, usage limit hits with the reset time the message gave, interruptions and subagents starting) in one line, in time order. Opening a commit from the flow and pressing back returns to where you were reading
  - Each prompt shows how long the AI worked (from the prompt to the AI's last activity before the next prompt) and the wait (from there to the next prompt), and a break of more than 30 minutes is marked like "13:50–15:20: 1h 30m gap" (both are estimates from the history's timestamps; time while a subagent was running is not counted as a wait)
  - Prompts that look like corrections (guessed from the wording) get a different dot, and a key above the flow explains the marks. Slash commands you typed (such as `/review`) and `!` shell commands count as prompts and get a "Command" or "Shell" tag in their own color
  - Things that entered the conversation without you typing them (background task notifications, `<system-reminder>`, hook and command output, automatic conversation summaries, instructions sent by another agent or a schedule, the expanded text of a slash command, the steering rules and environment context older Kiro IDE versions add to your message and so on) are not counted as prompts and appear in the flow in another color, labeled with their kind
  - The AI's reply follows each prompt, with its own time, so a session reads as the conversation it was: the last thing the AI wrote back to you in that turn, in its own words, not its thinking, its tool calls or their output. The HTML keeps the first 160 characters, and when opened with `kiroku serve`, "Load the full reply" reads it to the end. A turn where the AI only ran tools, or whose words the agent does not record, has no reply line
  - The flow shows 30 prompts first, and "Show N more prompts" shows the rest. "Show all" opens a long prompt. The HTML keeps the first 400 characters of each prompt; longer ones show "Read more (N characters)", and when opened with `kiroku serve`, "Load the full prompt" reads it to the end
  - "Only user prompts" above the flow narrows it to what the user typed, and hides the replies too, but only on screen: they are still in the HTML, so a file you share still has them in it.
  - "Copy review prompt", next to "Everything" / "Only user prompts", copies a prompt that asks an AI agent on this computer (such as Claude Code or Codex) to review the session, with advice on how you prompted and used the agent. See [Reviews and reports with your agent](#reviews-and-reports-with-your-agent)
- In the week calendar, days with commits show commit marks on the right edge at their times (they don't overlap session bars; a commit mark is filled when run by AI; touch one to see its short hash; the number under the date is that day's commit count). Click a mark to see the commit details (subject, message body, project, branch, hash, lines added and removed per file changed, the session that made the commit, and a `git show` command). Session details list the commits made during that session
- The same right edge also shows pushes and pull requests. Pushes are read from your local `git reflog` of remote-tracking branches ("update by push"), so only pushes made from this computer appear, and only as far back as git keeps its reflog (90 days by default). Pull requests appear only when an agent created one and its result was recorded (`gh pr create` or GitHub tools in Claude Code); kiroku never asks GitHub. Both also appear in a session's prompt flow. Click a push (on the calendar or in the prompt flow) to see where it went, when, the commits it sent (each opens its commit details; up to the latest 50), the session running at the time and a `git log` command for the same range. Click a pull request to see its link, when it was created, the session that created it and the pushes and commits in that session before it (kiroku never asks GitHub, so the title, status and reviews are not shown). A commit's details show the push that sent it
- Anything that can be a link is a link
  - Git commits and files changed: if the remote (`origin`) is GitHub, GitLab, Bitbucket or similar, they open the page for that commit or file
  - A session's "Files changed": a file changed by a commit made during that session opens the page for that file as of that commit
  - A session's "Pull requests created": opens pull requests created by AI (URLs that appeared in the results of `gh pr create` or GitHub tools)
  - A session's "History file": opens the original history of that session. In `kiroku serve` it opens from the view (only history files of loaded sessions, inside the folders kiroku reads history from or `kiroku archive`'s folder, are served, and only to the view on your machine); in `kiroku html` it opens via `file://`. SQLite histories cannot be opened, so you can only copy the path
- Below the calendar, the weekly and monthly summary is shown
  - Worth a look: a metric that crossed a threshold has a small warning triangle next to its name in the summary; press it to open the same dialog as "Worth a look" (see above). The dialog shows what was observed, why it matters, its 8-week trend (8 months in month view; it starts at the first period with records and uses the full width, and that period and the period shown are labeled with their values, and a note says whether higher or lower is worth a look), why it was flagged in this period (such as "Flagged because 2 sessions this week crossed the threshold"; a metric is flagged by a fixed threshold, not by being high or low within its trend), the related sessions and the threshold; the summary itself doesn't repeat them. "Worth a look" under the key figures, above the calendar, lists those metrics in priority order. To see whether something you tried worked, check the trend in later periods
  - ① By project: starts with bars and a table showing what share of active time, tokens, estimated cost and credits went where (top 5 plus Other). The grouping follows the Project / Branch / Agent switch at the top. Comparing the rows shows, for example, a project, branch or agent whose share of estimated cost is larger than its share of time. Below that, the cards show, for each project, its active time, sessions and prompts, tokens, estimated cost and credits, and what it left behind (beyond 6 projects, press "Show N more projects"). Models and the heaviest sessions are in ④
  - ② Cost and outputs: Cost (active time, prompts, estimated cost, tokens, credits) → what it left behind (Git commits with how many AI ran, lines changed with the average per git commit, files changed, pushes, and pull requests when an agent created one), side by side under the heading "Left behind". Below them, "Compared" holds the two metrics that divide one side by the other (estimated cost per commit and sessions that reached a commit or PR), each showing what was divided by what. Lines, files and pushes come from git, so they only appear when git commits were read, and when no git repository could be read the side says so and shows "AI commits" in place of "Git commits"; files changed counts up to 40 files per commit and says "at least" when a commit had more. Credits are shown as whole numbers
  - ③ How you spent time: focus blocks, usage limit hits and compactions; metrics that include estimates or that are background (corrections and interruptions, oversized prompts, switches, parallel time, wait time and total AI run time) follow under "More metrics (includes estimates)"
  - Then come ④ How you used AI (cache, subagents and their types, models, heaviest sessions and so on, with agent-specific metrics at the end, one card per agent; active time, estimated cost, tokens and credits appear only in ②) and ⑤ Shape of the week / month (sessions with possible friction and repeated prompts, each shown only when there are any), with "Data sources" (history read, retention and the price table for estimated cost) at the very bottom, folded into one line unless something needs your attention (a file that couldn't be read, history about to be deleted at its default setting or at the next cleanup because Kiro Crew keeps it 0 days, or models not in the price table, or models priced as a similar model ID), in which case it is open and marked. The footer under it shows when the view was generated, the kiroku version (a link to its release) and a link to the GitHub repository; they only open in your browser when you press them
- Press "?" on any metric to see, in a popover next to it, its definition, then a small table of what it tells you, what it doesn't tell you, and what to try. For metrics with a history (such as active time, prompts, tokens, Git commits, estimated cost, the cache share and the metrics "Worth a look" can flag), it also shows the 8-week trend (8 months in month view), whether or not the metric is flagged, so you can check whether something you tried changed it
- Search (top right, `/`): in addition to prompts, titles, projects, branches and agents, it searches files changed, pull requests and commits made during each session (subject, hash, files). The calendar shows only matching sessions and the commits and pushes made during them (plus commits that match themselves). Active time, tokens and credits can't be split by search, so while searching (or while items are hidden in the legend) the key figures at the top and those figures under each date stay totals for all sessions and are shown in grey, with a note above the calendar. All-time results appear between the legend and the calendar (and the summary is hidden while searching) in two lists, Sessions and Commits (matched on subject, body, hash and files changed), newest first, with excerpts showing where they matched. Each list shows 10 results first and scrolls in its own box, so the calendar stays close below; "Show 50 more (10 of M shown)" adds up to 50 at a time, and "Show the last N" the rest. Items hidden in the legend are left out. Enter in the search box moves the focus to the first result. Press a result to show its details (the calendar moves to its week)
- Daily reports: in the week calendar, each day with history has a "Report" button next to its date (just an icon on narrow screens). It copies a prompt for an AI agent on this computer to write that day's report
- Weekly and monthly reports: "Copy weekly report prompt" ("Copy monthly report prompt" in month view) in the summary heading copies a prompt for an AI agent on this computer, such as Claude Code or Codex, to write the report. See [Reviews and reports with your agent](#reviews-and-reports-with-your-agent) for what these prompts contain
- Year in review (currently hidden; the following describes it for when it returns): "Year in review" next to the period controls (`Y`) opens the year's "exposure". Across is the date and down is the time of day (6:00 to 6:00 the next morning, so late-night work lands at the bottom of the previous day's column); each stretch of active time in a session is drawn as a streak of light, colored by agent and brighter where sessions overlapped. Choose the year at the top right
  - An image to share: active time, sessions, commits, pull requests, the agent breakdown and the streaks of light as a 1600×900 image. Get it with "Save as PNG" or "Copy image", and choose what to include (commits and PRs, your light, skill, the agent breakdown). The image is made in the page and sent nowhere; prompts, project names, branches, files and estimated cost are never included. The streaks start from the first day with history (at least 8 weeks)

    ![Year in review share image (dummy data)](year.png)

  - Your light: the shape of your year, named in photography terms (not a verdict). The first match from the top is chosen

    | Your light | Rule |
    |---|---|
    | Daybreak | 25% or more of active time is between 5:00 and 9:00 (compared using the total active stretches of sessions) |
    | Multiple Exposure | 3 or more agents, each with 10% or more of active time |
    | Long Exposure | 45 minutes or more of active time per session, with 10 prompts or fewer on average |
    | Burst | 15 or more prompts per session on average |
    | Refocus | 15% or more of prompts had a correction or interruption |
    | Daylight | None of the above |

  - Skill: separate from your light (the type), four meters on a 10-step scale, and a level from the score that goes before the name ("Master Long Exposure"). The numbers are rough proxies that can move for reasons unrelated to skill, so treat it as a game. It is not used in the weekly or monthly summary

    | Meter | Shows | Based on (full marks in brackets) |
    |---|---|---|
    | Shutter count | Practice | Active days (180), prompts (3,000, log scale), active time (800 hours, log scale) |
    | Focus | Follow-through | Sessions that reached a commit or pull request (50%), few prompts per commit (5; Claude Code only) |
    | Layers | Range | Your best two of: share of time in parallel (30%), subagents (200 runs, log scale), using several agents (three used evenly), using several models (three with 10%+) |
    | Noise | Detours (fewer means a cleaner shot) | Prompts with corrections or interruptions (zero at 5%, full at 25%), long conversations (15% of sessions), cost of sessions with no commit or pull request (40% of cost), expensive models for light work (20% of cost) |

    For a year in progress or your first year, full marks are scaled down by the time elapsed (to as little as half). The score is the average of shutter count, focus and layers, minus 40% of noise. There are ten levels, one per 10%: Novice, Beginner, Apprentice, Competent, Skilled, Expert, Master, Virtuoso, Grandmaster and Legendary; Legendary needs all three at 80% or more and noise at 20% or less. With 4 or more months of history, if corrections drop by 3 points or cost per commit falls by 20% from the first half to the second, you are "improving" and go up one level. A score under 40% shows as "Underexposed" (room to grow), otherwise "Well exposed". If one of these matches, it shows as "Overexposed" and its name replaces the level (the first match from the top)

    | Overexposed | Rule |
    |---|---|
    | Blown-Out | 30+ days in a row without a break at 2+ hours a day |
    | Out-of-Film | Hit a usage limit 10 or more times |
    | Grainy | Noise at 60% or more |
- Opening details moves focus into them, and closing them returns focus to the bar or card you opened them from. When you move from one detail to another (a commit or session), "Back" takes you back. The browser's Back button closes the details (or goes back one step after moving between details) instead of leaving the page
- Keyboard shortcuts: `←` `→` to move by week or month, `T` for this week or month, `W` `M` to switch between week and month, `/` to search, `+` `−` to zoom, `Esc` to close details, `G` (or the map button at the top right) to open the metrics guide, `?` (or the keyboard button at the top right) to show the shortcut list. `←` `→` don't change the period while details are open or while the focus is on a control that uses the arrow keys itself (such as the View toggle and the search box). With the keyboard, "Skip the calendar" (the first stop after the legend) jumps past the calendar's bars and marks to the summary. `Esc` closes an open "?" explanation first
- Metrics guide (the map button at the top right, `G`): how the metrics fit together on one page
  - What you put in (Cost) becomes what is left behind (Left behind) through four levers of how you work with AI. Hand over more adds to Left behind; keep your attention unbroken, get it across the first time, and fit the context and the model each take away from Cost. Compared sets the two side by side
  - Each lever lists its metrics with what they tell you, followed by the steps for making a change (Worth a look, the sessions, one change, the same metric next period) and what the metrics don't tell you
  - In the terms of the [SPACE framework](https://queue.acm.org/detail.cfm?id=3454124), kiroku covers Activity and Efficiency and flow. Quality and delivery (the [DORA metrics](https://dora.dev/)), satisfaction and collaboration need data kiroku doesn't read
- The kiroku logo at the top left goes back to the start: this week or month, with details and search closed
- Switch between light (the default) and dark themes with the button at the top right. Color by, zoom, theme and week / month view are saved in the browser
- At smartphone widths, the week calendar scrolls horizontally (when opened, it shows the last day you worked up to today and the day before; the right edge fades while there are more days to the right), the session count ("20 / 20 sessions") is shown above the legend, and the summary is shown in a single column
- The palette has 8 colors based on Okabe–Ito, chosen to stay distinguishable across types of color vision. Projects and branches get them in order of how many sessions they have; from the 9th item on, items are gray
- Each agent has one fixed color everywhere (Color by Agent, the agent's badge, the cards and the Year in review): Claude Code blue, Codex green, Kiro IDE purple, Kiro CLI and Amazon Q light blue, and Kiro Crew pink. Orange and yellow are never used for agents, so an agent never reads as a warning; other agents get a color left over
- Every card that starts with a session name (Possible friction, Heaviest sessions, the sessions in Worth a look and in the breakdowns, the session behind a commit or push, and search results) shows the agent's initials as a badge before the name and a soft gradient in the agent's color. Cards in Repeated prompts look the same, with the prompt in place of the name and the agent of the session they open (the one where you last wrote it). Long names and prompts are cut to one line

## Reviews and reports with your agent

kiroku never calls an AI. For a session, a day, a week or a month, it copies a prompt that you paste into the AI agent you already use on this computer (such as Claude Code or Codex). The agent reads the facts in the prompt and the history files it points to, and writes a review or report in a fixed format, ending with advice from an expert on working with AI agents.

| Scope | Button | Format |
|---|---|---|
| A session | "Copy review prompt" in session details, next to "Everything" / "Only user prompts" | Summary, Numbers, Rework, Advice from an expert (with an example first prompt for similar work next time) |
| A day | "Report" next to each date with history in the week calendar (an icon on narrow screens) | Summary, Numbers (next to the previous day), By project, Advice from an expert |
| A week or month | "Copy weekly report prompt" ("Copy monthly report prompt" in month view) in the summary heading | Summary, Numbers (next to the previous week or month), By project, Advice from an expert |

What the prompts give the agent:

- **Numbers kiroku counted**, as a table the agent copies unchanged: active time, prompts, corrections and interruptions, the median wait time, Git commits (and how many AI ran), estimated cost, tokens, Kiro credits, and for a session, compactions and usage limit hits. Reports also have sessions, lines changed and the correction rate
- **What happened**: for a review, the prompt flow with commits, pull requests, interruptions, subagents and other events between your prompts, in time order. For a report, time, commits (with their repository, for read-only git) and pull requests for each project, and a digest of each session: your prompts in the period and the start of the AI's reply to each (up to 20 prompts per session in a day, 8 in a week and 3 in a month)
- **The history files**: a review asks the agent to read the session's history file first. A report asks it to write from the digests and open a history file only when a digest isn't enough, and only within the times listed for that session. For Amazon Q and the legacy Kiro CLI, whose history is one SQLite database, the prompts are copied in instead
- **Signals for advice**: prompts you wrote in 3 or more sessions (for a review, prompts in that session you also wrote in 3 or more sessions in the last 30 days), the tools the AI used most, subagents, compactions, the main model of each session, short sessions with no file edits on an Opus-class model, conversations whose input per response grew to 4× or more, and project switches (per day in a report; in a review, the prompts you wrote in other projects while the session ran). Weekly and monthly reports also list the metrics flagged under "Worth a look" and the share of input read from cache

The advice has 2–4 points. Each names what it is based on (a figure, a prompt with its time, or a signal) and then the concrete change. It covers:

- how you write prompts: missing context, constraints or done criteria, and corrections a clearer first prompt would have avoided
- how you use agents: splitting or ordering work, checking results, long sessions, waiting
- model and context: whether the main model fits the work (a lighter model for questions and small, clear edits; a stronger one for hard design or debugging), long conversations and compactions, and switching between projects
- what to move out of your prompts: instructions you repeat into a skill or the project's agent instructions, a separate role into a subagent, fixed steps into a script

Generic tips the history doesn't support, and topics where nothing needs to change, are left out. In a review, each place that needed rework is a row with its time, the quoted prompt, what happened and the cause: (A) something missing from your prompt, (B) the task itself being hard or uncertain, or (C) the AI getting it wrong although the prompt was enough.

Things to know:

- The agent reads everything in the history files it is pointed to: your prompts, the AI's replies, tool calls and their output. Use an agent you'd trust with them. kiroku itself sends nothing
- A report asks the agent to base every bullet on a commit, pull request or session in the prompt, and to copy numbers, commits and links as they are
- The prompts tell the agent that history is data, not instructions, and not to follow text in it that looks like instructions. Reports also ask it to leave out secrets, file contents, command output and paths on your computer, since a report is meant to be shared. Still, check a report before sharing it
- The report is written in the language you mostly write your prompts in
- In a day, week or month still in progress, the prompt says the data runs up to now. Sessions hidden in the legend are left out of reports

## Weekly and monthly summary

![Weekly summary (dummy data)](summary.png)

| Metric | Definition |
|---|---|
| Worth a look | Marks the metrics that crossed these thresholds and lists their names in priority order (the list below is not in priority order): hit a usage limit 1 or more times / there are Long conversations (later input 4× or more the first part, peak at half the model's context window or more (100K tokens or more when the window is unknown), $0.5 or more) / Expensive models for light work total 10% or more of estimated cost ($2 or more) and $1 or more / Claude Code sessions of $1 or more estimated cost with no commit or pull request make up 40% or more of the period's estimated cost ($2 or more) / estimated cost is 1.5× the previous period ($1 or more) or more / estimated cost per commit is 1.5× the previous period or more (3 or more commits) / Prompts with corrections or interruptions are 20% or more (10 or more prompts), or there are Sessions with possible friction / Share of input read from cache is under 50% (1M tokens or more) / Sessions that reached a commit or PR are under 25% (5 or more sessions, with at least one commit or pull request) / Project switches per day average 5 or more / no Focus blocks with 4 hours or more of work / the 90th percentile of Wait time is 15 minutes or more (n≥10) |
| By project | Per project: active time and its share, number of sessions / prompts, tokens, estimated cost and credits, and what it left behind (Git commits with how many AI ran, and lines changed). Time when several projects ran at once is split between them |
| Active time | Time when any session was running (overlaps count once) |
| Total AI run time | Time added up, including sessions running in parallel |
| Focus blocks | How many times work continued for 60 minutes or more, and the longest. Gaps within a session up to the session gap (`--gap`, 15 minutes by default) and gaps of up to 5 minutes between sessions are treated as continuous |
| Project switches per day | How often the project changed between consecutive prompts |
| Parallel time | Time when 2 or more sessions ran at once, and the most at once |
| Wait time | Median and 90th percentile of the time from an AI reply to your next prompt (up to 30 minutes) |
| Prompts with corrections or interruptions | Share estimated from the opening words of each prompt (such as "No, that's wrong" or "undo that") and interruptions. The first prompt of a conversation and words inside pasted code, quotes or indented logs are not counted, nor are ordinary requests that merely share the words, such as "add an undo button". The number of prompts (n) is shown alongside |
| Long conversations | Sessions where the input read per response (new input plus cache reads and writes) in the last quarter of the conversation was at least 4 times that of the first quarter, peaking at half the model's context window or more (estimated cost $0.5 or more; only sessions with 8 or more responses, from agents that record tokens). For Claude Code the window comes from kiroku's table of Claude models (see Peak context usage under "Agent-specific metrics"), and for Codex from `model_context_window` in its history; where it is unknown, the peak must be 100K tokens or more |
| Expensive models for light work | Total for sessions that mainly used Opus-class models, had 3 or fewer prompts, edited no files and cost $0.3 or more |
| Usage limit hits | Count and times of usage limit errors (usage caps and rate limits) left in Claude Code and Codex history. For Claude Code, this includes spend limits and API-key rate limits (`Request rejected (429)`); errors that are not about your usage do not count: the server's temporary throttling ("not your usage limit"), a conversation that outgrew the context window ("Context limit reached") and the notice that a limit has reset ("Usage limit reset · continuing automatically"). For Codex, it counts turns that stopped on a usage limit or a rate limit, and the usage limit responses Codex records with its limit usage; a conversation that outgrew the context window and a plan that does not include Codex do not count. A hit in a subagent counts for its parent session. Hits within 1 minute count once. The calendar shows them with a red "Limit" mark. For Claude Code, when the message says when the limit resets (such as "resets 3:45pm" or "continuing automatically at 3:45pm"), the prompt flow, the "Usage limit hits" card and "Worth a look" show that text as written; it often has no date or time zone, so kiroku does not turn it into a time |
| Compactions | How many times a conversation was compacted (summarized to free context, automatically or with `/compact`) in the period, and in how many sessions, with the times of the last ones. Read from Claude Code (`compact_boundary` lines, and the summary lines that follow them; the two count as one compaction, and summaries from older versions without `compact_boundary` count on their own), Codex (one `compacted` line per compaction, whether local, remote or token-budget) and the SQLite history of Amazon Q / Kiro CLI (`latest_summary`, the summary the CLI keeps from its last `/compact` or automatic compaction; earlier ones are overwritten, so at most one per conversation, the latest, is counted). Compactions within 1 minute count once. Only the main conversation counts, not subagents. Kiro IDE, Kiro CLI (JSON) and Kiro Crew record no compaction kiroku can rely on, so they are not included. Session details show the count and times, and the prompt flow marks each one ("automatic" or "manual" when Claude Code records it). It is not in "Worth a look": automatic compaction is how long conversations normally continue, and "Long conversations" already flags conversations that grew heavy |
| Sessions with possible friction | Up to 3 sessions started in the period with at least one correction or interruption, or 15 or more prompts, most first |
| Oversized prompts | Number of prompts of 4,000+ characters in the period, and the length of the longest |
| Repeated prompts | Prompts of 12+ characters in the period, grouped by how similar their text is; up to 3 written in 3 or more sessions, most first |

### How you used AI

| Metric | Definition |
|---|---|
| Estimated cost (API pricing) | Usage priced at public API rates. For sessions where Claude Code records its own cost (`cost-state`), that value is used (this also covers price changes, new models and calls not left in the history, such as auto-compaction), except where it records 0 for a model the history shows was in fact used, and for Sonnet 5.5 in records from Claude Code before 2.1.296, which priced its cache reads too high. For Kiro Crew turns run on backends other than kiro-cli, the USD cost Crew records is used (turns on its Claude Code and Codex backends are counted from those agents' own history instead). Otherwise, tokens in the history are multiplied by the kiroku price table, doubled for responses recorded in fast mode (`speed: "fast"`) and multiplied by 1.1 for US-only inference (`inference_geo: "us"`), plus $10 per 1,000 web searches recorded in the response's usage (`server_tool_use.web_search_requests`; the multipliers don't apply to searches). Claude Haiku 5.5 is priced at its higher rates for a response whose prompt (new input plus cache writes and reads) was over 100K tokens. Codex tokens are priced at OpenAI's standard rates (uncached input, cached input, cache writes and output), at the long-context rates for a response whose input was over 272K tokens. Where every model used is missing from the price table, it is shown as "—" rather than $0.00. It differs from what a subscription bills |
| Month-end cost (estimate) | Shown only in the month view while the month is in progress: the estimated cost from the 1st through today, divided by the days so far (including today) and multiplied by the days in the month. It assumes the pace so far continues, and is not shown for the first 7 days or on the last day. For a period in progress, comparisons with the previous period use the same days of it (for example "vs Sep 1 – Sep 20"). AI commits and pull requests have no daily figures, so they are not compared while a period is in progress |
| Month-end credits (estimate) | The same projection for Kiro credits, in its own box next to the estimated cost. Tokens and credits are separate allowances, so they are never added together or shown as one figure |
| Tokens | Input, output, cache reads and cache writes combined. Because one response is recorded across several lines, they are grouped by message ID before counting. For Claude Code, calls to the advisor tool's model are included and priced at that model's rates |
| Share of input read from cache | The share of input read from cache |
| By model | Estimated cost and tokens per model |
| Subagents | Number of `Task` / `Agent` calls (Claude Code), subagent threads (Codex) and Kiro Crew subagents, their types and total run time. Session details show when, to which type, what was asked, and how long it took |
| Kiro credits | Actual credits recorded in Kiro history |
| Estimated cost per prompt | Estimated cost ÷ number of prompts in sessions that record tokens (Claude Code, Codex, and Kiro Crew on backends other than kiro-cli); prompts to agents that record only credits are left out |
| Heaviest sessions | The top 3 sessions by estimated cost |

The price table is used for Codex, and for Claude Code sessions where Claude Code does not record its cost (older versions of Claude Code, models it records as 0 although they were used, or Sonnet 5.5 before Claude Code 2.1.296). It contains Anthropic's [public prices](https://platform.claude.com/docs/en/about-claude/pricing) and OpenAI's [standard prices](https://developers.openai.com/api/docs/pricing) as of October 2026 (the numbers it uses are kept in [docs/upstream](upstream/)). A model ID is first looked up without its version marks (a date such as `-20251001`, Bedrock's `-v1:0` and region prefix such as `us.anthropic.`); a model not in the table under its own ID gets the price of the longest ID it starts with, such as `claude-opus-5` for a new `claude-opus-5-6`, and "Data sources" lists it as "priced as" that ID, since a new model may cost something else. For price changes or models it does not include, override it with JSON (model IDs match the same way; units are USD per million tokens). Fields you leave out are treated as 0. An array `[input, output, cache_write, cache_write_1h, cache_read]` also works; for an OpenAI model, `cache_read` is the cached input price. An entry you add is used for all context lengths, including for a model priced by prompt length (OpenAI's long context, Claude Haiku 5.5 over 100K tokens).

```json
{"claude-opus-5-5": {"input": 4, "output": 20, "cache_write": 5, "cache_write_1h": 8, "cache_read": 0.2}}
```

```bash
kiroku serve --prices my-prices.json
```

### Outputs

Numbers shown next to costs such as time and tokens, to check whether your usage led to work that left a trace. They count how much was produced (outputs), not its value or productivity.

- **Git commits** are counted by reading the repositories agents worked in (each session's working directory) with your local `git`, counting your own commits (`git config user.email`; in repositories without `user.email`, everyone's commits are counted). Commits made by hand are included. Commits whose time matches (within 2 minutes of when the tool returned) a commit an agent ran with a tool are counted as "Run by AI". Each run is matched to the one nearest commit only. A git worktree is treated as the same repository as its main checkout. kiroku never fetches from a remote, so in a partial clone (`git clone --filter=...`), commits whose files aren't on your computer are listed without changed lines and files. The calendar shows them as commit marks and short-hash badges (filled ones were run by AI). They are not read where the `git` command is unavailable
- Everything else counts only what AI ran with tools and succeeded (currently Claude Code only)

| Metric | Definition |
|---|---|
| Git commits | Your own commits in local repositories (excluding merge commits). Read from the day before the first session in that repository |
| AI commits | Number of successful `git commit` runs by AI (excluding `--dry-run`). Commits made by hand are not included. Shown in place of Git commits when no git repository could be read |
| Sessions that reached a commit or PR | Number and share of sessions that made a commit or created a pull request in the period (including those made by subagents). Only Claude Code sessions are counted, since only Claude Code outputs are recorded |
| Estimated cost per commit | Estimated cost of Claude Code sessions ÷ number of commits AI ran in them (Commits above; the card shows it as "÷ N AI commits"). Other agents' cost and commits made by hand are left out |

### Agent-specific metrics

The numbers each agent records in its history are shown per agent (in the weekly and monthly summary and in session details). Definitions differ by agent, so **don't compare agents with each other**. All of them are for reference, not targets. Numbers that are not recorded are not shown, rather than shown as 0.

| Agent | Metrics |
|---|---|
| Claude Code | Responses, Output tokens per response, Peak context usage, Tool calls |
| Kiro CLI | Credits, Turns, Credits per turn, Model requests, Built-in tool runs |
| Kiro IDE | Credits, Turns, Credits per turn, Model requests, Peak context usage, Tool calls |
| Kiro IDE (legacy) | Credits, Turns, Credits per turn (from execution files) |
| Kiro Crew | Conversations run from Crew (Of which subagents), Credits, Turns, Peak context usage, Turns that did not end normally |
| Kiro CLI (SQLite), Amazon Q | Time to first reply (median), Response time (median), Response size (average, in bytes), Tool calls, Peak context usage (estimate, with the CLI's characters ÷ 4 over the stored history), Context window |
| Codex | Responses, Reasoning tokens, Share of output spent on reasoning, Peak context usage (worked out the way Codex shows it, leaving out the 12,000 tokens that are always in the context, and 100% when the conversation outgrew the window), Peak usage of each usage limit window (such as "Peak 5-hour limit usage" and "Peak weekly limit usage"; "Peak rate-limit usage" and "Peak secondary rate-limit usage" when the history does not say how long the window is; a response can be missing when Codex recorded the snapshot of another, per-model limit in its place), Time to first token (median), Turn duration (median; turns that ended in an error are left out), Tool calls |

Claude Code does not record how full the context window was, so for Claude Code, Peak context usage is computed by kiroku:

- The input of each response (new input plus cache reads and writes; for a response that made several calls with the advisor tool, its largest single call) divided by the model's context window, from a table in kiroku (`ContextWindows` in `internal/core/context.go`, from the official docs). Models not in the table get no value
- On the Anthropic API, Fable 5.1, Fable 5, Sonnet 5 and later, Opus 4.7 and later and Haiku 5.5 have 1M tokens
- Opus 4.6 and Sonnet 4.6 have 200K unless you picked their `[1m]` variant, which the history doesn't show, so kiroku assumes 200K and switches to 1M for a model once a response read more than 200K
- With `CLAUDE_CODE_DISABLE_1M_CONTEXT`, or on Amazon Bedrock, Google Cloud and Microsoft Foundry with Claude Code before v2.1.287, the window may be 200K where kiroku assumes 1M, so the value reads low there

Codex records its own window.

Times are calculated in your computer's time zone. All numbers are rough estimates from history. When there is no data, "Unknown" is shown instead of 0. When none of the agents used in the period records something (for example, Kiro records no usage limit hits), the metric shows "—" and "Not recorded in … history" instead of 0; session details do the same for files changed. Tokens, estimated cost and total AI run time are rough measures of usage; they do not show productivity or time saved.

> These numbers are for reflecting on how you work. Don't use them to compare or evaluate people.

### What each agent records

Not every agent's history records everything kiroku shows. Active time, Total AI run time, Focus blocks, Parallel time, Project switches, Wait time, corrections, Oversized and Repeated prompts, the prompt flow with replies, and Git commits read from your repositories work for every agent. The rest depends on the agent:

| Metric | Claude Code | Codex | Kiro IDE | Kiro IDE (legacy) | Kiro CLI | Kiro Crew | Kiro CLI (SQLite) | Amazon Q |
|---|---|---|---|---|---|---|---|---|
| Interruptions (in Prompts with corrections or interruptions) | ✓ | ✓ | — | — | — | — | ✓ | ✓ |
| Usage limit hits | ✓ | ✓ | — | — | — | — | — | — |
| Compactions | ✓ | ✓ | — | — | — | — | ✓ (latest only) | ✓ (latest only) |
| Files changed (session details) | ✓ | ✓ | ✓ | — | ✓ | — (listed for conversations kept in the kiro-cli history) | ✓ | ✓ |
| Subagents | ✓ | ✓ | — | — | — | ✓ | — | — |
| Outputs (AI commits, pull requests, sessions that reached a commit or PR, cost per commit) | ✓ | — | — | — | — | — | — | — |
| Tokens and estimated cost | ✓ | ✓ | — | — | — | Partly (backends other than kiro-cli) | — | — |
| Kiro credits | — | — | ✓ | ✓ | ✓ | ✓ | — | — |
| Long conversations | ✓ | ✓ | — | — | — | Partly (backends other than kiro-cli) | — | — |
| Resume command | ✓ | ✓ | — | — | ✓ | — (shown for conversations kept in the kiro-cli history) | ✓ | ✓ |
| History retention and `kiroku archive` | ✓ | — | — | — | — | ✓ | — | — |

Where none of the agents used in the period records a metric, the view shows "—" and "Not recorded in … history" instead of 0. Why each one is missing is in [What history records](sources.md#what-history-records).

### How to read the metrics

Press "?" on any metric in the view to see the same explanation, and open the metrics guide (`G`) to see how the metrics fit together. It is also included as background in the improvement prompts. Don't judge from a single number; read it against your own past periods.

| Metric | Tells you | Doesn't tell you | What to try |
|---|---|---|---|
| Worth a look | Which metric is worth reviewing first | Whether something is good or bad. Thresholds are generic and may not fit how you work | Pick one, try it next period, and check the change with the same metric |
| By project | How you divided time and AI across projects | How important or successful a project was | If the split differs from what you intended, revisit how you work or your priorities |
| Active time | Total time you worked together with AI | Whether you were focused. Includes time you left a session idle | Compare with the previous period to see how much more or less you rely on AI |
| Total AI run time | How much work you gave to AI. The gap from active time shows how much ran in parallel | Time saved or productivity | If it is close to active time, you could do other work while waiting |
| Focus blocks | Whether you had long stretches of uninterrupted work | The quality of the work in that time | If your time is fragmented, group the work you hand to AI into blocks of time |
| Project switches per day | How much you moved between projects | Whether switching is bad (you may just be using wait time well) | If high, next period limit each day to 2–3 projects |
| Parallel time | Whether you kept several sessions going in parallel | What the parallel work achieved | If low, give AI another task while it works |
| Wait time | How quickly you responded to AI replies | Shorter is not always better (you may be moving on without checking) | If long, use notifications or batch your reviews |
| Prompts with corrections or interruptions | Roughly how often a first prompt did not get your intent across | Estimates can be wrong, and they don't show the cause | If high, add background, constraints and done criteria to your prompts |
| Long conversations | Sessions where each response got heavier as the conversation went on | Whether continuing the conversation was the right call (some work needs the earlier context) | At a good stopping point, write down the key points and continue in a new session |
| Expensive models for light work | How much went to short tasks on an expensive model | Whether that model was needed (some research or design questions are hard) | Try a lighter model first for research and questions, and switch if it falls short |
| Usage limit hits | When and during which work you hit a limit and had to stop | How much headroom is left. Usage from other agents, the browser or the app | Spread heavy work over time, use lighter models, and start new sessions for long conversations |
| Compactions | Which conversations grew long enough to need summarizing | Whether anything important was lost in the summary, or whether the conversation should have been split | When one task keeps compacting, finish it at a good stopping point and start the next in a new session |
| Sessions with possible friction | Sessions where rework piled up | Why it went wrong | Next period, split big requests into one step per prompt |
| Oversized prompts | Whether you hand over large inputs at once, such as pasting whole logs or documents | Whether that length was needed (some long prompts, like design explanations, are fine) | Next period, put long logs or documents in a file and write only its path and the part to look at |
| Repeated prompts | Routine requests you type every time | Whether those requests worked well | Next period, write the most repeated one once as a custom command or in CLAUDE.md |
| Estimated cost (API pricing) | A rough way to compare how heavy usage was, in money | What you are actually billed (subscriptions differ) | Open the sessions behind the increase, and next period keep that kind of work in shorter conversations |
| Month-end cost (estimate) | Roughly where this month's cost is heading at the current pace | Your actual bill, or how you will work from now on (it is off if the pace changes) | If it is too high, look at the heavy sessions and models |
| Month-end credits (estimate) | Roughly how many credits this month is heading for at the current pace | What your account page will show, or how you will work from now on (it is off if the pace changes) | If it is heading past your limit, spread the heavy work out or move some of it to another agent |
| Tokens | How much you consumed | Whether more or less is good | Look for skew by project and by day |
| Share of input read from cache | Whether the same context was reused | Why it is low (it may just be many short sessions) | If low, next period write long background once in a project file (such as CLAUDE.md) instead of pasting it every time |
| By model | Which models your usage leaned toward | Whether that model was needed | Next period, try lighter models for routine work (formatting, renames, adding tests) |
| Subagents | Whether you delegated research and similar work | How much delegating helped | Check that you use them to save the main conversation's context |
| Kiro credits | Credits actually consumed | Differences from your account page (period boundaries or use on other machines) | Track your pace against your limit |
| Estimated cost per prompt | How heavy a typical prompt was | Differences in prompt size | Watch the trend to see how the size of your prompts changes |
| Heaviest sessions | Sessions that drove usage up | Whether the result was worth it | Next period, restart long conversations in a new session, and carry work to a commit in small steps |
| Prompts | How much you had to put in by hand to get the work done | The size or quality of each prompt, or how much thought went into it | Read it next to active time: many prompts in little time often means steering turn by turn instead of handing over a whole task |
| Cost and outputs | Whether the cost turned into work that left a trace | Value, quality or productivity. Work that leaves nothing in git | Put the two sides next to each other and look for usage that left nothing |
| Git commits | How much of your time with AI became recorded changes | The value of the changes. Work outside the repositories or commits by others | On days with much time or cost but few commits, check where the time went |
| Lines changed | Roughly how much the period produced, without depending on how often you commit | The value or difficulty of the changes. A big number can be one generated file | Read it next to the number of commits to see your usual commit size |
| Files changed | How widely the work spread | How much of each file changed, or whether the files are related | If few files took much cost, open those sessions to see what the time went into |
| Pushes | How often the work left your machine | Whether it was reviewed or merged. kiroku never asks GitHub | If commits pile up without pushes, push in smaller steps so the work is shared sooner |
| Pull requests | How often work reached a request for review | Whether they were merged, their size or their quality. kiroku never asks GitHub | Compare with commits to see how much work is waiting to be shared |
| AI commits | Roughly how often work reached a checkpoint | The value or size of the changes. Commit size varies by person and task | In periods with few commits for the cost, check where the time went |
| Sessions that reached a commit or PR | The share of sessions that left something behind | The value of sessions not meant to commit, such as research or discussion | If low, next period state at the start of each session what done looks like (when to commit) |
| Estimated cost per commit | Roughly how heavy it was to reach a checkpoint | Differences in commit size, or work that went into commits you made by hand | Next period, keep each prompt to one change and commit often |
| Agent-specific metrics | Trends within the same agent | Comparisons between agents (definitions differ) | Only look at changes over time for the same agent |

## Histories read

| Agent | Location | Time granularity |
|---|---|---|
| Claude Code | `~/.claude/projects/*/*.jsonl` | Per message |
| Kiro IDE (v1.0 and later) | `~/.kiro/sessions/<hash>/sess_*/` | Per message |
| Kiro CLI | `~/.kiro/sessions/cli/` (the opt-in V3 storage, `kiro-cli --v3`, is not read yet) | Per prompt |
| Kiro IDE (before v1.0) | `<globalStorage>/kiro.kiroagent/workspace-sessions/`, the execution files in `<globalStorage>/kiro.kiroagent/<32-hex workspace>/<session>/` and the `.chat` files in `<globalStorage>/kiro.kiroagent/<32-hex workspace>/` (`<globalStorage>` is `~/Library/Application Support/Kiro/User/globalStorage` on macOS, `%APPDATA%\Kiro\User\globalStorage` on Windows, and `~/.config/Kiro/User/globalStorage` or `~/.kiro-server/data/User/globalStorage` on Linux) | Start and last update, plus the start and end of each execution when its file exists |
| Kiro Crew | `~/.kiro/crew/` (`session_map.json`, `usage/tokens/`) | Per prompt or turn |
| Kiro CLI (old versions) | `kiro-cli/data.sqlite3` (table below) | Per prompt |
| Amazon Q Developer CLI | `amazon-q/data.sqlite3` (table below) | Per prompt |
| Codex CLI | `~/.codex/sessions/YYYY/MM/DD/rollout-*.jsonl` (including `.jsonl.zst`) and `archived_sessions/` | Per message |
| Git (your own commits) | Repositories in each session's working directory (local `git log`) | Per commit |

Location of `data.sqlite3`:

| OS | Location |
|---|---|
| macOS | `~/Library/Application Support/<kiro-cli or amazon-q>/` |
| Linux | `$XDG_DATA_HOME` (`~/.local/share` if unset)`/<kiro-cli or amazon-q>/` |
| Windows | `%LOCALAPPDATA%\<kiro-cli or amazon-q>\` (unverified for Kiro CLI; if it differs, specify it with `--kiro-cli-db`) |

- If `KIRO_HOME`, `KIROCREW_HOME`, `CODEX_HOME` or `CLAUDE_CONFIG_DIR` is set, that location is read (for `CLAUDE_CONFIG_DIR`, its `projects/` folder). `KIRO_HOME` does not move Kiro Crew, which always uses `~/.kiro/crew` unless `KIROCREW_HOME` is set
- Even if the same conversation is recorded in two places, it is counted once (Kiro CLI in both JSON and SQLite counts the JSON one; a Kiro IDE conversation migrated to v1.0, whose pre-1.0 copy Kiro keeps, counts the v1.0 one). The number excluded is shown under "Data sources" (at the very bottom of the summary)
- Kiro credits are the values recorded in history, summed as they are (per-model multipliers are not reapplied). Kiro IDE before v1.0 records credits only in its execution files, so conversations without them have none, and Kiro CLI (SQLite) does not record credits, so that usage is not included. Variations in how the unit is written (`credit`, `Credits` and so on) are treated the same. Numbers may differ from your account page because of the period (billing period), use on other computers, old history Kiro has deleted, and use outside chat (such as agent hooks, which leave no history)
- Codex models (OpenAI) are priced at OpenAI's standard rates, including the Cyber models such as `gpt-5.5-cyber`. Fast mode (priority processing), Flex, Batch and the regional-processing uplift are not reflected, and models older than gpt-5 are not in the price table (add them with `--prices`)

How each history is read and how duplicates are excluded is described in [sources.md](sources.md). If some history cannot be read, let us know in an issue.

### History retention

Some agents delete old history automatically. Deleted history cannot be shown by kiroku and cannot be recovered, so set this up early if you want to look back further. In the view, "Data sources" shows the oldest record for each agent, and while Claude Code or Kiro Crew is still on its 30-day default, it is flagged there (and the section opens) with a link to the docs and the two ways to keep the history: the setting and the file to put it in, or a backup to kiroku's folder, which it names. `kiroku doctor` shows the same under "Keeping history".

| Agent | Deletes automatically? | Setting |
|---|---|---|
| Claude Code | **Yes.** By default it silently deletes conversation history older than 30 days at startup. Since v2.1.248, sessions you started or last continued in Claude Desktop or Cowork are kept at any age unless `desktopSessionCleanupPeriodDays` is set (or your organization's managed settings set `cleanupPeriodDays`) | [`cleanupPeriodDays`](https://code.claude.com/docs/en/settings-reference#cleanupperioddays) in `~/.claude/settings.json` (days, a whole number, minimum 1; `0` fails validation, so use a large value such as `3650` for long retention) |
| Kiro Crew | **Yes.** By default it deletes conversation records moved to `sessions/archive/` (and the logs of closed sessions) that are older than 30 days. It checks whenever it moves records there, at most once an hour | Crew's [`session.archive_retention_days`](https://github.com/kirodotdev/kirocrew/blob/main/src/kiro_crew/docs/configuration.md) in `~/.kiro/crew/config.local.json` (or `config.json`; under `KIROCREW_HOME` if set). Days; `-1` or `null` turns the cleanup off, and `0` deletes all archived records at the next cleanup |
| Kiro IDE, Kiro CLI, Amazon Q, Codex | Their official docs do not describe age-based automatic deletion (manual cleanup exists) | — |

Example for Claude Code:

```json
{
  "cleanupPeriodDays": 3650
}
```

kiroku reads your user settings (`~/.claude/settings.json`, or under `CLAUDE_CONFIG_DIR` if set) and your organization's managed settings file, which wins over your own: `managed-settings.json` and the `managed-settings.d/*.json` files next to it (later files win) in `/Library/Application Support/ClaudeCode/` on macOS, `/etc/claude-code/` on Linux and WSL and `C:\Program Files\ClaudeCode\` on Windows ([managed settings](https://code.claude.com/docs/en/managed-settings)).

- If `cleanupPeriodDays` in your settings is not a whole number of 1 or more (such as `0` or `"30"`), or a settings file exists but can't be read or parsed, Claude Code pauses its cleanup and deletes nothing until you fix it (unless the managed settings set `cleanupPeriodDays`), so kiroku shows no notice. Claude Code also pauses for other settings errors when `cleanupPeriodDays` is set; kiroku can't check that
- If the period is set in project settings, in server-managed settings from claude.ai or by MDM (a macOS profile or the Windows registry), the notice may not match the actual period
- kiroku shows one period for all Claude Code history; it doesn't single out Claude Desktop and Cowork sessions, which Claude Code keeps longer as described above

Example for Kiro Crew (`~/.kiro/crew/config.local.json`; its values win over `config.json`. If the file already has a `"session"` object, add the key inside it):

```json
{
  "session": {
    "archive_retention_days": 3650
  }
}
```

`-1` instead of `3650` turns Crew's cleanup off, and kiroku then shows no retention for it. With `0`, Crew deletes every archived record at its next cleanup (the next time it moves records to `sessions/archive/`), so "Data sources" and `kiroku doctor` say so (unless `kiroku archive` keeps a copy).

kiroku reads `config.json` and `config.local.json` the same way Crew does. Crew writes every setting to `config.json`, including this one at 30, so kiroku treats the period as set only when it is not 30.

### Keep a copy of history in kiroku

If you'd rather not change the setting, kiroku can keep a copy of the history instead. It is off until you turn it on.

- Run `kiroku archive on`, or press "Back up to this folder" under "Data sources" in the `kiroku serve` view (the folder is named next to it). kiroku saves the current history right away, then saves new and appended history each time it reads history (`serve` reloads, `html`, `json`)
- Only history from agents that delete it automatically is saved (Claude Code conversations, and Kiro Crew's `sessions/archive/`). Each file is compressed with zstd and kept in the same layout as the original
- When the original conversation is deleted, kiroku shows it from the copy. "Data sources" shows how many conversations came from the copy, and the number and size of the saved files. Claude Code conversations shown from the copy have no "Resume" command (Claude Code no longer has them)
- The copies live in `~/.local/share/kiroku/archive` on Linux (under `XDG_DATA_HOME` if set), `~/Library/Application Support/kiroku/archive` on macOS and `%LocalAppData%\kiroku\archive` on Windows. Change it with `--archive-dir` or `KIROKU_ARCHIVE_DIR`. Copies stay on this computer and are never sent anywhere
- `kiroku archive off` stops saving and asks whether to delete the copies already kept (only if you answer `y` in a terminal). It deletes only the compressed copies (`.zst` files) kiroku saved, and folders left empty; other files in the folder stay. It only offers this in a folder where `kiroku archive on` was run, so a mistyped `--archive-dir` never deletes anything. If you keep them, kiroku still shows them
- History deleted while kiroku is not opened cannot be saved, so open kiroku at least once before the period ends. The conversations exist in one more place, so even conversations you deleted on purpose remain in kiroku's copy, and so does the text of Kiro Crew's incognito and temporary conversations that Crew moved to `sessions/archive/` (kiroku does not show it; see the note on Kiro Crew under "Notes")

### Start kiroku when you log in

`kiroku autostart on` registers `kiroku serve --no-open` to start each time you log in, starts it right away and waits a few seconds until it answers. The view is then always at `http://localhost:8484/` (run `kiroku open` once to give your browser the key); while the first read of your history is running, the page says so and switches to the view by itself. With `kiroku archive on`, history is kept as soon as it is written.

- macOS: a launchd agent, `~/Library/LaunchAgents/io.github.michinaoshimizu.kiroku.plist`. Output goes to `~/Library/Logs/kiroku.log`
- Linux: a systemd user service, `~/.config/systemd/user/kiroku.service` (under `XDG_CONFIG_HOME` if set). See its output with `journalctl --user -u kiroku`
- Windows is not supported yet. Put a shortcut to `kiroku serve --no-open` in your Startup folder (Win+R, `shell:startup`) instead
- To use another port, run `kiroku autostart on :8485`. A login session does not see the variables set in your shell, so `PATH` (to find git), `CLAUDE_CONFIG_DIR`, `KIRO_HOME`, `KIROCREW_HOME`, `CODEX_HOME`, `KIROKU_ARCHIVE_DIR`, `KIROKU_CONFIG_DIR` and `XDG_CONFIG_HOME` (where the key of kiroku serve is kept) are written into the settings as they are when you run it (on Linux, a value with a newline or another control character is refused). Run `kiroku autostart on` again after changing them
- It uses the `kiroku` you ran, at its path. `kiroku update` replaces it in place; after reinstalling it somewhere else, run `kiroku autostart on` again
- `kiroku autostart` (and `kiroku doctor`) shows whether it is on, reading your history, or answering. If `autostart on` reports that nothing answered, the output above (log) says why; a common cause is another `kiroku serve` you started yourself already using the port

#### Turn it off

`kiroku autostart off` stops it and removes it from login. Run it before deleting kiroku.

If kiroku is already gone (or you used `kiroku autostart on` with v0.9.0, whose newer versions had no `autostart` command), remove it by hand:

```bash
# macOS
launchctl bootout gui/$(id -u)/io.github.michinaoshimizu.kiroku
rm ~/Library/LaunchAgents/io.github.michinaoshimizu.kiroku.plist ~/Library/Logs/kiroku.log

# Linux
systemctl --user disable --now kiroku.service
rm ~/.config/systemd/user/kiroku.service
systemctl --user daemon-reload
```

## The key of kiroku serve

`127.0.0.1` keeps other computers out, but not other users of the same computer (a shared server you log in to with SSH, for example): any of them could open `http://localhost:8484/`. So `kiroku serve` shows your history only to a browser that has its key.

- The key is made the first time `kiroku serve` runs and kept in `serve-key` in kiroku's settings folder (`~/.config/kiroku` on Linux, `~/Library/Application Support/kiroku` on macOS, `%AppData%\kiroku` on Windows, or `$KIROKU_CONFIG_DIR`), readable only by you. Every `kiroku serve` of yours uses the same key
- `kiroku serve` (unless `--no-open`) and `kiroku open` open the view with the key. They don't pass the key to the browser on its command line (other users can see that); they open `open.html` in the same folder, readable only by you, which forwards to the view with the key
- The browser then keeps the key in a cookie for that port (for 400 days), and the address without the key works, including bookmarks. Another browser, or a browser whose cookies were cleared, shows "Open kiroku with its key"; run `kiroku open` again. The cookie is not sent when you arrive from a link on another site, so that page also appears then; its "open it again from here" link opens the view
- To open the view on another device (with `kiroku serve 0.0.0.0:8484`), run `kiroku open --print` and open the printed address there, with `localhost` replaced by this computer's address. Anyone with that address can read your history, and the connection is not encrypted, so do this only on a network you trust
- To change the key, stop `kiroku serve`, delete `serve-key` and start it again. Browsers then need `kiroku open` again

## Share one week or month

`kiroku html --week last` (or `this`, or any date in the week such as `2026-10-05`) and `kiroku html --month last` (or `this`, or `2026-09`) write an HTML file with only that period, named like `kiroku-2026-09-28.html` or `kiroku-2026-09.html` unless you give `-o`. Use it to show a week to a teammate without handing over all of your history.

- The file has only the sessions that overlap the period, and only the commits and pushes made within it. A session that crosses the edge of the period is included whole, prompts and all
- It opens on that week or month and says at the very top that it holds only that period; "Back to included week" / "Back to included month" brings you back to it, and ‹ › (or ← →) only move within that period. Search looks only in this file. Other periods are empty, so 8-period trends have only that period. A week-only file has no month view
- Times, days and weeks follow the time zone it was written in, wherever it is opened, so the figures match; the note at the top names that time zone when it differs from the viewer's
- "Data sources" counts only the sessions in the file and lists only the agents that have some, and leaves out the oldest record, how to keep history that is about to be deleted and kiroku archive's folder
- It still contains the prompts, file paths and commit messages of that period as they are. Check it before giving it to anyone

## Options

Common to the commands that read history (`serve`, `html`, `json`, `stats`, `doctor`, `archive`):

| Option | Default | Description |
|---|---|---|
| `--sources` | `claude,kiro,amazonq,codex` | Histories to read (comma-separated; Kiro Crew is included in `kiro`) |
| `--claude-root` | `~/.claude/projects` (`$CLAUDE_CONFIG_DIR/projects` if set) | Location of Claude Code history |
| `--kiro-home` | `~/.kiro` | Location of Kiro data (`KIRO_HOME` is also used) |
| `--crew-home` | `~/.kiro/crew` (`$KIROCREW_HOME` if set) | Location of Kiro Crew data. Like Crew itself, it does not move with `--kiro-home` or `KIRO_HOME`; a leading `~` in `KIROCREW_HOME` means your home folder |
| `--kiro-cli-db` | Per OS | `data.sqlite3` of Kiro CLI (old versions) |
| `--amazonq-db` | Per OS | `data.sqlite3` of Amazon Q Developer CLI |
| `--codex-home` | `~/.codex` | Location of Codex data (`CODEX_HOME` is also used) |
| `--gap` | `15` | Idle time (minutes) that splits session bars |
| `--prices` | | Override the price table with JSON (see "How you used AI") |
| `--archive-dir` | Per-OS location (see "Keep a copy of history in kiroku") | Where `kiroku archive` keeps copies (`KIROKU_ARCHIVE_DIR` is also used) |

Per command:

| Command | Option | Default | Description |
|---|---|---|---|
| `serve` | `[ADDR]` | `127.0.0.1:8484` | A port alone, such as `:8485`, also works (it listens on `127.0.0.1`). To open it from other devices, write an address such as `0.0.0.0:8485` (anyone on your network can then see your history) |
| `serve` | `--allow-host` | | Extra host names the view may be opened by, comma-separated (for example a name in your hosts file). `localhost`, `127.0.0.1` and `::1` always work, and so do this computer's IPs and host name when it listens on all interfaces (such as `0.0.0.0:8485`); requests addressed to any other name are refused |
| `serve` | `--interval` | `5s` | How often to check history for changes. Reloads when changes stop for twice the interval (at most 12 times the interval) |
| `serve`, `html` | `--no-open` | | Don't open the browser |
| `html` | `-o`, `--out` | `kiroku.html` | HTML file to write |
| `doctor` | `--no-update-check` | | Don't ask GitHub whether a newer release exists |
| `open` | `[ADDR]` | the autostart address, or `127.0.0.1:8484` | `--print`: print the address with the key instead of opening a browser |
| `autostart on` | `[ADDR]` | `127.0.0.1:8484` | Where the started `kiroku serve` listens, as for `serve` |
| `html` | `--week` / `--month` | | Write only one week (`this`, `last` or a date in it) or month (`this`, `last` or `YYYY-MM`); see "[Share one week or month](#share-one-week-or-month)" |
| `stats` | `--day` / `--week` / `--month` | this week | The period to show: a day (`today`, `yesterday` or `YYYY-MM-DD`), a week (`this`, `last` or a date in it) or a month (`this`, `last` or `YYYY-MM`) |
| `stats` | `--project` | | Only these projects, comma-separated, as named in the view (case does not matter) |
| `stats` | `--top` | `5` | How many rows to list for projects, models, sessions and sessions worth a look |
| `stats` | `--json` | | Print both periods as JSON instead of text |
| `json` | `-o`, `--out` | `kiroku.json` | JSON file to write (`-` for stdout) |
| `update` | `--check` / `--to <version>` / `--force` | | Only check / choose a version / replace even a dev build, the same version or an older version given with `--to` |

## Uninstall

1. If you turned on autostart, run `kiroku autostart off`. It stops `kiroku serve` and removes it from login (if kiroku is already deleted, see [how to remove it by hand](#turn-it-off))
2. If you turned on `kiroku archive`, run `kiroku archive off` and answer `y` to delete the copies of history it kept. Or delete the folder yourself: `kiroku archive` shows where it is (by default `~/.local/share/kiroku` on Linux, `~/Library/Application Support/kiroku` on macOS, `%LocalAppData%\kiroku` on Windows)
3. Delete kiroku's settings folder, which holds the key of `kiroku serve` (`~/.config/kiroku` on Linux, `~/Library/Application Support/kiroku` on macOS, `%AppData%\kiroku` on Windows, or `$KIROKU_CONFIG_DIR`)
4. Delete the binary: `rm "$(command -v kiroku)"` (with `sudo` if it is in `/usr/local/bin`)

Apart from these, kiroku keeps nothing of its own (HTML files you wrote with `kiroku html` stay where you put them); view settings are stored in your browser.

## Notes

- In a terminal, kiroku colours its output: green `✓` for what is fine, yellow `!` for what needs attention, red for what failed, dim for notes you can skip, and bold for the commands and addresses you are meant to use. Output that goes to a pipe or a file is plain, and so is a terminal where `NO_COLOR` is set (see [no-color.org](https://no-color.org/)) or `TERM=dumb`. `install.sh` follows the same rules
- The HTML and JSON output contain your prompts, the AI's replies, file paths and commit messages as they are. Check the content before giving them to anyone (this repository's `.gitignore` excludes `*.html` and `kiroku.json`). kiroku writes them readable only by you (`0600`), through a temporary file that replaces the old one, so a symbolic link at the output path is replaced rather than followed
- Kiro Crew conversations you held in incognito or temporary mode are shown with counts and times only (time span, prompts, turns, model, tools, credits, tokens and cost), titled "Kiro Crew private conversation": their prompts, replies, title and files are not in the view, the JSON or the prompts kiroku copies for an AI, and neither is their copy in the Kiro CLI SQLite history. Git commits made during their time span are still listed, since they come from the repository. kiroku can only tell a kiro-cli conversation is private while Crew still links it: one Crew no longer links to (such as a temporary subagent's, while it runs or after a crash) is shown in full. Lines Crew archived before the current conversation log began, or whose log is gone, are shown as counts and times only, since kiroku can't tell whether they came from a private conversation. `kiroku archive` copies Crew's `sessions/archive/` files as they are, so its copy still holds their text
- By default, `kiroku serve` can be reached only from your own computer (it listens on `127.0.0.1` and rejects requests addressed to other host names), and only by browsers that have its key, so other users of the computer can't view your history. If you expose it, as in `kiroku serve 0.0.0.0:8484`, devices on the same network can reach it too, and anyone who gets the address with the key can view your history. Even then, it only answers requests addressed to this computer's own names and IPs (plus `--allow-host`), so a web page cannot point its own domain at your computer and read your history (DNS rebinding). Its responses also tell the browser not to show the view inside other sites or share it with them. See [SECURITY.md](../SECURITY.md) for details
