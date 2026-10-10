# Changelog

Notable changes to kiroku, in the style of [Keep a Changelog](https://keepachangelog.com/). Add changes under `## Unreleased`; its contents decide the next version ([Semantic Versioning](https://semver.org/), see `sh tools/next-version.sh`), and each release on GitHub uses its section here as the release notes.

## Unreleased

### Added

- Year in review is back ("Year" after Week and Month, or `Y`): a year of active time drawn as streaks of light, your light (the shape of your year in photography terms) and a 1600×900 image to share with totals only, which now says how to read the streaks and fills its width from 4 weeks of history (was 8). It no longer has skill levels (Novice to Legendary), the four skill meters, or the under- and overexposed labels: it describes how you worked and doesn't score or rank you

## v0.30.0 - 2026-10-10

### Added

- `kiroku stats` prints a one-screen summary of this week in the terminal: active time, sessions, tokens, estimated cost, Kiro credits and Git commits, with tables by agent, model and project. `--week` and `--month` pick another period, as in `kiroku html`. Names from history are printed with control characters removed, so they can't send escape sequences to your terminal

### Changed

- Building from source (`go install`) now needs Go 1.26 or later, instead of Go 1.25. The SQLite library (modernc.org/sqlite) is updated to v1.60.1 and the zstd library (klauspost/compress) to v1.20.1

## v0.29.2 - 2026-10-10

### Fixed

- Kiro IDE: a conversation migrated from 0.x to 1.0 is no longer counted twice. Kiro keeps the 0.x copy after migrating, so the 0.x conversation with the same ID is left out and the 1.0 one is used, as kiro-history does; "Data sources" shows how many were left out
- Kiro IDE (1.0 and later): a conversation without usage records now counts its model once, so it appears under "Models used"
- Kiro CLI (SQLite): conversations from the `conversations_v2` table now get `kiro-cli chat --resume-id <id>` as their resume command. Before, they got `kiro-cli chat --resume`, which opens the newest conversation in the folder, so an older conversation in the same folder opened a different one
- Amazon Q and Kiro CLI (SQLite): when a declined or interrupted tool turn was the first turn kept after a compaction, the CLI rewrote it to "Tool use was cancelled by the user", and kiroku counted that as your prompt. It is now shown as command output
- Codex: a `/review` no longer counts the review prompt Codex writes for the reviewer as your prompt (twice in older rollouts), and its result no longer replaces the reply to your previous prompt. The review now shows as one slash command, `/review` with what was reviewed (such as `current changes`), and the reviewer appears under "Subagents" as `review` instead of `subagent`. Its usage before a compaction in the reviewer's own history is no longer dropped
- Codex: a turn that failed because your ChatGPT plan does not include Codex is no longer counted as a usage limit hit
- Claude Code: a conversation copied with `/branch`, `--fork-session` or `/fork` is no longer counted twice. A line in the copy is recognised as copied when the same line (its `uuid`) is in the original; it then counts once, in the original. Everything else, and the whole copy if the original is gone or can't be read, is counted in the copy
- Claude Code: when a settings file exists but can't be read or parsed, kiroku no longer reports the 30-day retention. Claude Code pauses its cleanup then (unless the managed settings set `cleanupPeriodDays`), so nothing is deleted
- Claude Code: estimated cost for Sonnet 5.5 is no longer too high in sessions recorded by Claude Code 2.1.284 to 2.1.295, which priced its cache reads at twice the official rate. kiroku estimates Sonnet 5.5 from its price table there
- Kiro Crew: lines Crew moved to `sessions/archive/` only because they duplicate a line still in the conversation (`foreign-dedup`), or as the earlier form of a line that was edited during a rewind, regenerate or fork (`compact`), are no longer counted again as prompts. Turns that were undone are still shown
- Kiro Crew: tokens and cost of dashboard turns that ran on Codex are no longer counted twice (once from Crew's usage records, priced from the price table, and once from Codex's own history) when the conversation was later switched to kiro-cli. Crew records such turns with the provider `acp`, so kiroku now tells them by their Codex model ID

### Security

- Commands kiroku shows for pasting into a terminal (resuming a session, `git -C <repo> show`) are no longer offered when a folder, session ID or git value from history contains a curly quote (‘ ’ ‚ ‛ “ ” „ ‟). PowerShell reads these as quotes, so a crafted value could close the quoting and add a command when pasted on Windows (or into PowerShell on macOS and Linux)
- Kiro Crew: conversations held in incognito or temporary mode (`memory_mode` in the conversation log or its archive files, or the flag in `session_map.json`) are now shown with counts and times only, titled "Kiro Crew private conversation". Before, kiroku showed their prompts, replies, title, edited files and the prompts sent to their subagents in the view, the JSON, search and the prompts it copies for an AI, although Crew itself refuses to learn from, summarize or export them. This covers their kiro-cli conversation, its copy in the Kiro CLI SQLite history, their subagents, and archived lines kiroku can't tie to the current conversation log (from an earlier conversation under a reused key, or whose log is gone), which are shown as counts and times only. The time span, number of prompts, turns, model, tools, credits, tokens and cost are still shown, as are Git commits in that time span. A kiro-cli conversation Crew no longer links to can't be recognized and is still shown in full. `kiroku archive` still copies Crew's `sessions/archive/` files as they are, so its copy keeps their text
- Kiro Crew: a named pipe in the Crew folder in place of a conversation log no longer makes kiroku hang while reading

## v0.29.1 - 2026-10-09

### Changed

- The theme is now light by default. The button still switches between light and dark, and a theme you chose is kept
- The report prompts (daily, weekly and monthly) and the session review prompt no longer count pull requests in their numbers. The pull requests themselves are still listed with their links
- Session details: "Models used", "Tools used" and the agent's own metrics are always shown, instead of inside a "Models, tools and …" section you could fold. Each model has its own color
- Session details: when you scroll down, the title pinned at the top has the session's color and agent mark in front of it
- Prompt flow: your prompts stand out in a tinted box with a bar in the session's color, and things the agent added (system notes, notifications, hook output) in a purple box that shows up to 3 lines instead of one cut-off line. Slash commands and shell commands you typed, and command output, are shown in a monospace code box
- `kiroku serve` no longer prints "history changed" when a reload leaves the number of sessions the same
- Prompt flow: your prompts have a person icon instead of a dot, next to the ↩ on the AI's replies, so you can tell who wrote what by shape too. A prompt that looks like a correction also has a "Correction?" label and a dashed bar, so it doesn't rely on color. The legend shows the same "Command", "Shell" and "Correction?" labels as the rows. Things the agent added have an icon for their kind (system note, notification, hook output, command output, summary)
- Session details: each tool under "Tools used" has an icon for what it does (shell, edit, write, read, search, web, subagent, plan, MCP)
- On wide screens, the page now grows up to 1920px wide (before, 1560px), so the week calendar's day columns have more room for session titles. Wider than that, the page stays centered

### Removed

- **"Ask AI for suggestions" (⑥ in the summary).** The weekly and monthly report prompts already ask for advice, with more to go on (a digest of each session and signals from your prompts). Their "Signals for advice" now also list the metrics flagged under "Worth a look" and the share of input read from cache, which only ⑥ had

## v0.29.0 - 2026-10-09

### Added

- Daily reports: each day with history in the week calendar has a "Report" button next to its date. It copies the report prompt for that day, in the same format as the weekly report, with the day's numbers next to the previous day's

### Changed

- The weekly and monthly report prompts now set a fixed format (Summary, Numbers, By project with done, why, results and next, and Advice from an expert on prompting, using agents, and what to move into a skill, a subagent or a script, grounded in prompts you repeated across sessions, the tools the AI used most, subagents and compactions) and include a table of numbers kiroku counted, next to the previous week or month: active time, sessions and prompts, Git commits, pull requests, lines changed, estimated cost, tokens, Kiro credits, the correction rate and the median wait time. Each project also gets its share of active time, sessions, prompts, commits and cost
- The report prompts now carry a digest of each session (your prompts in the period and the start of the AI's reply to each, up to 8 prompts in a week and 3 in a month), so the agent writes from that and opens a history file only when it needs more. Before, the agent was asked to read every session's history file, which could mean hundreds of megabytes for a busy week or month
- Session details: the resume command is now at the top of the right column on wide screens (on narrow screens it stays at the top, under the title). "Copy review prompt" is now a filled button, so it stands out in session details
- Session details: "Copy prompts" is gone. "Copy review prompt" names the session's history file for an AI agent, which reads the full prompts there
- The session review prompt ("Copy review prompt") now sets the same kind of format as the reports: Summary, a Numbers table kiroku filled in, Rework (a table of each place that needed rework, with the quoted prompt and its cause) and the same Advice from an expert, ending with an example first prompt for next time. The advice can point to prompts in the session that you also wrote in 3 or more sessions in the last 30 days
- The Advice from an expert in the report and review prompts now also covers whether the model fits the work, long conversations and compactions, and switching between projects. The prompts add the main model of each session, short sessions with no edits on an Opus-class model, conversations whose input per response grew 4× or more, and project switches (per day in reports; prompts in other projects while the session ran in a review)
- The report buttons (daily, weekly and monthly) are filled buttons now, like "Copy review prompt", so they stand out

## v0.28.1 - 2026-10-09

### Security

- kiroku no longer lets `git` fetch anything while it reads commits. In a partial clone (`git clone --filter=...`), counting changed lines made `git` download the missing files from the remote and, in doing so, run the program the repository's settings name in `remote.<name>.uploadpack`. kiroku now turns fetching off in a way the repository's settings can't undo (`GIT_ALLOW_PROTOCOL=none`, `protocol.allow=never`, `GIT_NO_LAZY_FETCH=1`). In a partial clone, commits whose files aren't on your computer are still listed, without changed lines and files
- Prompts that kiroku copies for an AI ("Copy review prompt", "Ask AI for suggestions") put some history text into the fenced data block without making it a single line: the project name (which comes from the working directory recorded in history), the branch, model and tool names, and the history file path. A crafted working directory or branch with a line break (including NEL and the Unicode line and paragraph separators) could add lines that look like kiroku's own, such as a fake "History file:" line pointing an agent at another file. Every history value in these prompts is now one line, with line breaks and control characters turned into spaces

### Changed

- **The weekly and monthly report drafts are replaced by a report prompt.** "Weekly report draft" in the summary heading is now "Copy weekly report prompt" ("Copy monthly report prompt" in month view). Instead of opening a draft whose "what I did" was only the start of your prompts, it copies a prompt for an AI agent on this computer, such as Claude Code or Codex. The prompt carries the facts kiroku collected (time, commits and pull requests for each project, with each commit's repository so the agent can look at a commit with read-only git) and, for each session in the period, its history file and the times it ran in the period. The agent can then write what you did and why. The prompt asks the agent to read only those files and times, to base every bullet on a commit, pull request or session, and to leave out secrets, file contents, command output and paths on your computer. Amazon Q and legacy Kiro CLI sessions list your prompts instead of a file
- "Copy review prompt" is meant for an AI agent running on this computer (such as Claude Code or Codex). It now names the session's history file and asks the agent to read only that file (and, for Claude Code, its subagent records), so the review can use your full prompts, the AI's replies and its tool calls instead of guessing what the AI did. It also asks the agent not to follow instructions found in the file. Next to it, the prompt lists what happened between your prompts (interruptions, commits, pull requests, subagents, compactions and usage limits) in time order, with the time zone, and marks prompts that look like corrections, so the agent can tell which prompt led to rework instead of only seeing counts. Prompts in this list are cut to their first words, since the full text is in the file. It also asks the agent to quote the prompt behind each point, to say whether rework came from the prompt, the task itself or the AI's own mistake, and not to invent problems. For Amazon Q and the legacy Kiro CLI, whose history is one SQLite database for all sessions, the prompts are still copied in, as before

### Fixed

- A long project, branch or agent name in the legend above the calendar no longer pushes the page sideways on a narrow screen. The name is cut with "…", and hovering shows it in full

## v0.28.0 - 2026-10-09

### Added

- Metrics guide: the new map button at the top right (or `G`) shows how the metrics fit together. What you put in becomes what is left behind through four levers of how you work with AI, Compared sets the two side by side, and each lever lists its metrics with what they tell you, followed by how to make a change and what the metrics don't tell you, including where kiroku sits in the SPACE framework (Activity, and Efficiency and flow) and what it leaves to other data (quality and delivery as in DORA, satisfaction, collaboration)

### Changed

- The kiroku logo and name at the top left now link back to the start: `/` in `kiroku serve`, and the same file in an HTML file or the live demo (this week or month, with details and search closed)
- The "Your older history will be deleted" box above the summary is gone; the README and `kiroku doctor` already say it. "Data sources" still flags history that will be deleted (and opens), and now says how to keep it: the setting and the file to put it in, or a backup to the folder it names with `kiroku archive on` ("Back up to this folder" in `kiroku serve`; was "Keep a copy in kiroku"), noting that the agent's own files are not changed, nothing is sent anywhere, and `kiroku archive off` stops it
- Breakdown dialogs: the first sentence of what the metric is now sits right under the figure, so you know what was counted before reading the numbers. Before, the whole definition was at the very end, below the lists. The rest of the definition and what the metric doesn't tell you stay at the end

### Fixed

- `←` `→` no longer move to another week or month behind an open breakdown dialog, as they already didn't behind the other dialogs

## v0.27.4 - 2026-10-09

### Security

- Release builds now use Go 1.26.9 instead of Go 1.26.8, whose standard library has known vulnerabilities in code kiroku uses (`net/http`, `net/textproto` and `crypto/tls`: GO-2026-6603, GO-2026-6605, GO-2026-6607, GO-2026-6608, GO-2026-6610, GO-2026-6611, GO-2026-6612, GO-2026-6613 and GO-2026-6617; found with `govulncheck`)
- Commit and push details: the `git -C <repo> show` (or `git log`) command under "Repository" now quotes the repository path for the shell, as the Resume command already does. Before, a repository folder whose name contains shell syntax (such as `$(...)`, backticks or `;`) would run that command when you pasted it into a terminal, and a path with spaces broke the command. When the path can't be quoted safely (such as control characters, or `%` or `$` on Windows), kiroku now leaves the command out

### Changed

- Copy buttons (Copy, Copy prompts, Copy review prompt, Copy prompt, Copy setting, Copy command, Copy image) now start with a copy icon, which turns into a check mark for a moment after copying
- Session, commit, push and pull request details: the project and branch tags now start with a folder icon and a branch icon, and a commit's hash with the commit icon (each also says what it is in a tooltip and to screen readers), so the tags look different at a glance
- Prompt flow: the legend for "Commands the user typed" now shows the `/` and `>_` icons used on those prompts, instead of only a different color. In the light theme, those icons and the "Command" and "Shell" labels are a darker amber, so they are easier to read on white

## v0.27.3 - 2026-10-09

### Security

- Links to files on this computer in session and commit details no longer resolve to another computer's shared folder (a UNC path, which Windows may connect to with your credentials) when a path recorded in history uses `.`, `..` or doubled separators (such as `/a/..//host/share/x`) or the session ran in `/`. kiroku now resolves the path before making the link, and gives no link to a path that climbs above the root
- Kiro Crew: a conversation folder in `crew-log/sessions` whose name contains `*`, `?` or `[` no longer makes kiroku read logs from other folders there (including folders linked from outside the Crew folder), or attach other conversations' subagents to it

### Changed

- The live demo and the screenshots now show several agents, as kiroku does: Codex CLI, Kiro CLI and Kiro IDE sessions (with Codex subagents and Kiro credits) alongside Claude Code, instead of Claude Code alone
- Session details: "Resume" is now at the top, right under the session's title, at any window width. Before, in a narrower window it came after the prompt flow
- Session and commit details: changed files now start with a file icon, and every path is a link. Links to the remote are marked ↗; paths that have no link to a commit on the remote open the file on this computer as it is now (in the HTML file; `kiroku serve` can't open local files from the browser), and pull requests start with the pull request icon

### Fixed

- Kiro Crew subagents are no longer shown as sessions of their own (on the calendar and in the list) once Crew has cleaned up their folder, which it does an hour after a subagent's result is delivered. kiroku now also finds the parent conversation from the `subagent/spawned` records in the parent's crew log (`crew-log/sessions/`), so these subagents stay inside their parent's session with their agent name and prompt. Totals don't change

## v0.27.2 - 2026-10-08

### Fixed

- Amazon Q / Kiro CLI (SQLite): tool output is no longer counted as a prompt you typed. After `/compact` keeps recent turns (or the automatic compaction retries keeping the last one), and when tool results follow a plain reply, the CLI stores the tool output as a prompt; kiroku now shows it as "Command output" and counts that turn as one the agent ran on its own
- Amazon Q / Kiro CLI (SQLite): "Response timed out - message took too long to generate", which the CLI writes itself when a response times out, is no longer shown as the model's reply, and in the oldest history format no longer counts as a model use
- Amazon Q / Kiro CLI (SQLite): conversations whose stored JSON is broken are now reported in "Data sources" instead of being skipped without a word
- Kiro IDE (before v1.0): credits and turns from chats you hid from Kiro's chat list, and from executions not linked to any chat, are now counted instead of dropped, so the totals match what Kiro recorded. They appear as sessions without prompts ("Kiro IDE hidden chat" or "Kiro IDE executions without a chat"); hidden chats' prompts are still not shown. An execution linked to more than one chat is counted once
- Kiro IDE (before v1.0): the model and times of each execution now also come from the `.chat` files Kiro keeps per workspace, so sessions show the model that actually ran instead of the one selected in the chat, and prompts get the time they ran instead of the chat's creation time
- Claude Code sessions that used the advisor tool now count the advisor's tokens and cost, priced at the advisor model's rates. Before, only the main model's tokens were counted. When Claude Code records its own cost, the advisor's share follows that record, so it is not counted twice. In these responses, Peak context usage and Claude Haiku 5.5's price for prompts over 100K tokens now use the largest single call instead of the sum of all calls in the response
- Claude Code sessions now show the name Claude Code generated for them when you haven't named them yourself. Before, the generated name was never read, so the start of a summary or prompt was shown instead. A name you gave a session is not replaced by a generated one written later
- Peak context usage now finds the context window for Claude model IDs written with a Bedrock region (`us.anthropic.claude-opus-4-8`, `global.anthropic.claude-opus-4-8[1m]`) or with a dot (`claude-opus-4.8`), as the price table already did, and counts a model ID ending in `[1m]` as 1M. Before, these had no value or used an older model's window
- Usage limit hits no longer count Claude Code messages that only contain the number 429 (such as "~1,429 tokens"), or the "Usage limit reset · continuing automatically" notice. When Claude Code waits for a limit to reset ("Usage limit reached · continuing automatically at 3:45pm"), the time is now shown as the reset time
- When `cleanupPeriodDays` in Claude Code's settings is invalid (such as `0`, a string or a fraction), the view and `kiroku doctor` no longer warn that history older than 30 days is deleted. Claude Code pauses its cleanup in that case and deletes nothing
- Codex: input sent by a scheduled heartbeat is no longer counted as a prompt you typed. It appears in the prompt flow as added by a schedule, and the reply to it is no longer shown as the reply to your previous prompt. A prompt you type while a heartbeat turn runs still counts as yours
- Codex: a thread you reverted (`thread/revert`) is shown as one session instead of two with the same ID. It includes the kept part of the conversation once, and the turns that were reverted away are no longer counted in its prompts, tokens and cost
- Codex: `gpt-5.5-cyber` and `gpt-5.6-cyber` are priced at their own official rates instead of `gpt-5.5`'s (or not at all)
- Codex: local shell calls, web searches, tool searches and image generations now count as tool calls and appear in the session's tool list, and files changed with `apply_patch` through a local shell call appear among the edited files
- Codex: in old sessions without `user_message` events, notices Codex added as user messages without a tag (the list of plugins not installed, and warnings about the exec process limit, `apply_patch` through `exec_command` and a flagged account) are no longer counted as prompts
- Codex: in the oldest sessions, whose token counts have no per-response usage, "Responses" now counts each response instead of staying empty
- Kiro Crew credits are no longer counted twice when Crew starts a conversation over in a new kiro-cli conversation (after its context fills up, a backend switch or seeding). Crew's usage records for the conversation key now go to the kiro-cli conversation that was running at their time instead of all going to the newest one, and the earlier kiro-cli conversation (the `discarded_sid` Crew keeps, or the one in the same folder whose time span covers the records) is marked "Kiro Crew" too. Records that match no kiro-cli conversation still count as a Kiro Crew session of their own, so no credits are lost. Prompts from Crew's conversation log are split the same way
- Tokens and cost of Kiro Crew subagents and background work run on Codex are no longer counted twice (they are counted from Codex's own history): kiroku now reads the backend from a subagent's `state.json` and from the `codex` label on background and memory-consolidation records, not only from `session_map.json`
- Kiro Crew conversations that have only a conversation log now include the older lines Crew moved to `sessions/archive/`, as other Crew conversations already did
- Kiro Crew's task runner lines (`[Task: <spec>] Task <n>: <title>`) are no longer counted as prompts you typed; they are shown as sent by another agent
- Kiro Crew conversation keys with non-ASCII letters or digits (such as Japanese channel or thread names) now find their conversation log: kiroku turns keys into file names the way Crew does
- Kiro Crew archived conversation lines that landed in the same second are read in the order Crew wrote them (`-1` after the unsuffixed file, `-10` after `-9`), and another conversation's archive whose key starts with the same name is no longer mixed in

## v0.27.1 - 2026-10-08

### Fixed

- Opening the view with `kiroku open` (or when `kiroku serve` starts) no longer flashes a white page with a blue "Open kiroku" link twice before the loading screen. The two pages that pass the key on now use the loading screen's background, light or dark, and their link is the same color as the background. If the browser doesn't move on by itself, the link appears after 2 seconds

## v0.27.0 - 2026-10-08

### Added

- Session details show "Worth a look" under the title when the session is one of the sessions behind a flagged metric in the period shown. Press a name to open the same dialog as in the summary. "Oversized prompts" and "Repeated prompts" now keep every related session (the dialog still shows 6 and counts the rest), so every session behind them gets the mark

### Changed

- Kiro Crew subagents are shown inside their parent's session, like Claude Code and Codex subagents, instead of as sessions of their own: the parent's details list each one with its agent, prompt, time span, model, tool calls, tokens and cost, and the parent's credits include theirs. Credits, cost, "Conversations run from Crew" and "Of which subagents" add up to the same totals; the time a subagent worked counts toward its parent's active time, and session counts and prompts no longer count a subagent's conversation as separate work (the prompt Crew sends a subagent is not one you typed). A subagent whose parent's records are gone still appears on its own
- The resume command moved to the top of the right column in session details, so you no longer scroll past files, models and tools to reach it

### Fixed

- Opening details no longer shows the buttons in them ("Everything / Only user prompts", "Copy prompts", "Copy review prompt") and session cards as empty boxes for a moment: they now appear together with the panel instead of 0.2 seconds later

## v0.26.0 - 2026-10-08

### Added

- Every other figure in the summary's panels now opens a breakdown too: lines changed, files changed, pushes, pull requests, AI commits, estimated cost per commit, sessions that reached a commit or PR, focus blocks, prompts with corrections or interruptions, oversized prompts, project switches per day, parallel time, wait time, total AI run time, read from cache, subagents and estimated cost per prompt. Each shows what it is made of: by day, by agent or by project where that applies, how ratios are worked out (such as cost ÷ prompts), and the commits, pushes, pull requests, prompts or sessions behind it (press one to open its details). The by-day bars for parallel time and AI run time are counted minute by minute the same way as the figure, so they add up to it
- The Git commits breakdown lists the commits themselves (newest first, press one to open it) and which agent ran the ones made by AI, and counts every project with commits, including ones with no session in the period

### Changed

- Reading history is faster: a year of synthetic Claude Code history (about 2,900 sessions) loads in 1.1 seconds instead of 1.8. The check for prompts that look like corrections now skips its large pattern when none of its phrases appear; which prompts count as corrections doesn't change
- `kiroku serve` turns sessions, weeks and months into JSON once for both the page and `/data.json` each time it reads history again, instead of twice (0.40 seconds instead of 0.55 for the same year). `/data.json` is the same byte for byte
- `kiroku serve` recalculates only the weeks and months whose sessions or Git commits changed when it reads history again, instead of every week and month. With a year of history, a refresh after one session changes spends about 11 ms on this instead of 250 ms. The figures don't change

## v0.25.0 - 2026-10-08

### Added

- The figures in the summary's panels open the same breakdown dialog as the key figures at the top: active time, prompts, estimated cost, tokens, Kiro credits, Git commits, the month-end estimates and usage limit hits. Click anywhere on the figure's box; its "?" still opens the explanation. Compactions get a breakdown too: by agent, by project, and when each happened in which session

### Fixed

- A breakdown with more than eight agents, projects or models now says "3 more not shown" instead of "3 mores not shown"

### Changed

- The view builds the line shown when a list is empty, code with a Copy button and numbers with thousands separators through shared helpers (`noneH` and `codeH` in `ui.js`, `commas` in `format.js`), and `TestUIConventions` fails when a script writes them by hand. Nothing you see changes

## v0.24.1 - 2026-10-08

### Changed

- Each agent's name, `--sources` name, color, initials, environment variables and `kiroku archive` folder are kept in one table (`core.Agents`). The `--sources` default and help, the environment variables `kiroku autostart` writes down, the folders `kiroku archive off` clears, and the colors and badges in the view and in the page `kiroku serve` shows while it first reads history are made from it, instead of each keeping its own copy. Nothing you see changes
- Cards in "Repeated prompts" look like the other session cards: the badge and color of the agent of the session they open (where you last wrote the prompt) before the prompt, and a soft gradient in that color
- The subheadings in the summary's panels ("More metrics", "By model", "Subagent types", "Heaviest sessions" and "Agent-specific metrics") are real headings styled like "Possible friction" and "Repeated prompts", so screen readers can jump to them. The notes in the commit, push and pull request details use the same size as the other notes in the details panel

### Fixed

- The agent badge on session cards in the "Worth a look" dialog no longer breaks onto two lines ("K" over "C") on narrow screens; it keeps its width next to a long session name
- A long prompt in "Repeated prompts" is cut to one line like the other cards, instead of wrapping onto several lines; one with a long word such as a URL no longer runs past the card and makes the page scroll sideways at phone widths

## v0.24.0 - 2026-10-08

### Added

- Claude Haiku 5.5 (`claude-haiku-5-5`) is in the price table: $0.10 input, $0.125 / $0.20 cache writes (5 minutes / 1 hour), $0.01 cache reads and $0.50 output per million tokens, and $0.50 / $0.625 / $1 / $0.05 / $2.50 for the whole response when its prompt is over 100,000 tokens. The pricing page doesn't say whether cached tokens count toward the prompt, so kiroku counts new input plus cache writes and reads. Its context window (1M) is in the table for Peak context usage. `docs/upstream/anthropic-pricing.md` now has columns for prices by prompt length, and an entry for the model in `--prices` is used for every length

### Fixed

- Claude Sonnet 5.5: cache reads are priced at $0.10 per million tokens (0.05× input), as on the official pricing page, instead of $0.20. Estimates from the price table were too high for Sonnet 5.5 sessions that read from the cache
- Codex sessions started on an older Codex and resumed after upgrading it keep the tokens, cost and responses from before the upgrade. They were dropped once the newer version wrote its first usage record to the same file
- In older Codex histories without `user_message` events, the context Codex adds to the conversation (`<environment_context>`, `<user_instructions>`, `<turn_aborted>` and other such blocks, and AGENTS.md instructions) is no longer counted as a prompt. It appears in the prompt flow as a note instead
- Amazon Q and Kiro CLI (SQLite): text the CLI sends on your behalf is no longer counted as your prompt. The reason it sends when it rejects a tool whose arguments your settings forbid, the request `/todos resume` sends, and the lines an MCP prompt adds with `/prompts get` are shown as "Added by the agent" instead
- Amazon Q and Kiro CLI (SQLite): a conversation stored under several folders (for example after `/load`) is read from its most recently saved copy, instead of whichever copy happened to come first, which could be an older, shorter one
- Amazon Q and Kiro CLI (SQLite): rows of `data.sqlite3` that can't be read are reported in "Data sources" instead of being skipped silently
- Kiro IDE before 1.0: a prompt that starts with the steering rules Kiro adds (`<steering-reminder>`) is counted as a prompt again, with the rules shown as a system note; before, the whole message, your request included, became a note and the prompt was not counted. Messages that hold only the environment context Kiro adds (`<EnvironmentContext>`) or its `## Included Rules` block are no longer counted as prompts, and these blocks are left out of the prompt text
- Kiro IDE 1.0 and later: a reply placeholder (`...`) Kiro writes while a reply is streaming is no longer shown as the AI's reply
- Kiro Crew: messages Crew sends on its own (scheduled runs, Issue Radar wakes, the task runner, auto-go) no longer count as your prompts. Newer Crew marks the messages a person wrote, and kiroku now counts only those, showing the others as messages sent by an agent or a schedule. Conversations written before Crew had the marker are read as before
- Kiro Crew: a forked conversation no longer counts the prompts it copied from the conversation it was forked from, so forking a chat with 20 prompts no longer adds 20 more
- Kiro Crew: its data is read from `~/.kiro/crew` even when `KIRO_HOME` (or `--kiro-home`) points elsewhere, since Crew itself does not follow `KIRO_HOME`. `KIROCREW_HOME` still wins, and a leading `~` in it now means your home folder, as in Crew
- Kiro Crew: memory consolidation runs are grouped into one session per memory store and day, like Crew's other background work, instead of one session per run
- Kiro Crew: a subagent's turns and credits are no longer counted twice, once in its kiro-cli conversation and again as a separate "Kiro Crew" session. Crew's usage records for a subagent are now matched to its conversation, and the records of subagents whose conversation Crew has deleted are shown as that subagent's session

### Security

- The "Resume" command in session details is no longer shown when the session ID or project folder read from history starts with `-`. Quoting does not stop a command from reading such a value as an option, so a crafted history file could make the command you paste run, for example, `claude --resume --dangerously-skip-permissions` or `cd -`. It is also not shown when the value contains invisible Unicode formatting characters (such as right-to-left overrides or zero-width spaces), which could make the command on screen look different from what you paste

## v0.23.0 - 2026-10-08

### Added

- `kiroku serve`, `html` and `json` print how long each agent's history took to read (for example `Claude Code: 2100 sessions (~/.claude/projects) in 640ms`), and how long reading git took
- The page `kiroku serve` shows while it reads your history for the first time lists each agent as it is read, with its number of sessions and how long it took, the elapsed time, and the current step (history, git, building the view). It reads them from the new `/progress` address, which has only agent names, counts and times (no history), and is protected by the key like every other page

### Changed

- Reading history is several times faster. Agents are read side by side, and so are the conversation files of Claude Code and Codex (on as many CPU cores as you have); kiroku also reuses its read buffer instead of allocating 1 MiB for every file, and no longer runs a regular expression on every model ID. With 2,100 Claude Code sessions, `kiroku json` went from about 3.9 s to 0.9 s on 4 cores. The results are the same

## v0.22.2 - 2026-10-07

### Changed

- "More metrics (includes estimates)" under "How you spent time" is always shown instead of folded away
- Every card that starts with a session name looks the same: the agent's badge before the name and a soft gradient in the agent's color. This covers Possible friction, Heaviest sessions, the sessions in Worth a look and in the breakdown dialogs, sessions linked to a commit or push, and search results
- The "What it is / Tells you / Doesn't tell you / What to try" part of a metric's help is a table with a border, set apart from the description above it: the labels sit in a shaded column with a line between the columns and between rows. The same table is used in the "?" popover, Worth a look and the breakdown dialogs

## v0.22.1 - 2026-10-07

Tagged, but its release was never published because the release job did not start. Its changes are in v0.22.2.

## v0.22.0 - 2026-10-07

### Added

- Click any figure at the top (tokens, estimated cost, month-end estimates, credits, active time, active days, sessions, usage limit hits, Git commits) to open its breakdown in a dialog: by day, by agent, by project, by model (tokens and cost), the top sessions, and the definition. The month-end estimates show how they are worked out (so far ÷ days so far × days in the month), with the remaining days drawn at the current pace. Usage limit hits list each hit with its session and reset time. Sessions in the dialog open their details

### Changed

- Each agent has one fixed color everywhere: Claude Code blue, Codex green, Kiro IDE purple, Kiro CLI and Amazon Q (its predecessor) light blue, and Kiro Crew pink. Orange and yellow are left out so an agent never reads as a warning (other agents get a leftover color). It no longer depends on how many sessions each agent has, so "Color by: Agent", the agent badge, session cards and the Year in review all use the same color for the same agent
- Tokens, estimated cost, the month-end cost estimate, Kiro credits and the month-end credits estimate now lead the figures at the top, grouped and highlighted as the most important numbers. The month-end estimates used to appear only further down, under "How you used AI". When the figures don't fit next to the heading, they move to their own row instead of being cut off
- Agent-specific metrics are shown directly under "How you used AI" instead of folded away, one card per agent with its badge and a soft gradient in its color, like the By project cards. Session cards (Heaviest sessions, sessions linked to a commit or push) get the same gradient in their agent's color

### Fixed

- Kiro IDE before 1.0: reading history is fast again. Since v0.20.0, kiroku read every execution file in full, including the whole conversation it carries, which made `kiroku serve`, `html` and `json` several seconds slower with a lot of Kiro IDE history (about 3 s to 7 s for 1,600 sessions with 3,000 execution files). kiroku now picks out only the fields it uses and skips the rest, so reading the execution files takes about a tenth of the time; the results are the same

## v0.21.2 - 2026-10-07

### Changed

- "Worth a look" details now appear only in its dialog. The summary no longer repeats them in a box under each flagged metric; a small warning triangle next to the metric's name opens the same dialog. The dialog's "Show in the summary" button is gone
- Dialogs (Worth a look, keyboard shortcuts, Year in review) work like the details panel: they close when you click outside them, and with a × in the top-right corner instead of a "Close" button at the bottom
- The "?" help opens in a popover next to the button instead of widening the card it belongs to, so the summary no longer reflows when you read a definition. It closes with the same "?", `Esc` or a click outside
- Session details: the resume command stands out in the accent color with a filled Copy button, since it is the part you use most often
- Session details: "Models, tools and … metrics" is open by default, and each tool's bar has its own color (by rank within the session, so the bars next to each other differ)

## v0.21.1 - 2026-10-07

### Changed

- A week with no records says so once, in the calendar, with a button to the latest week that has records. The month is no longer lowercased ("jul 6"), the summary no longer repeats "No records" in its own card, and Color by, Zoom and the session count are hidden while there is nothing to act on
- Text you need to judge a metric is easier to read: why it was flagged, its explanation and the "?" help are 12px instead of 11px (the threshold line stays small), and "click to hide" is 11px instead of 10px. The top bar is solid, so numbers scrolling under it no longer show through behind the search box
- Details panel: once the title scrolls away, Back and Close become a solid bar that shows the title, instead of floating over the text, so you can tell which session you are reading. The history file path and the resume command wrap and show in full instead of being cut off mid-word
- On phones, a headline figure left alone on the last row (such as Git commits) spans the empty cells. Project cards show sessions, tokens and cost in one row when there are no credits, instead of leaving cost alone on a second row, which makes each card about a quarter shorter
- Summary on wide screens: "How you spent time" spans the full width with its cards in four columns, and "How you used AI" and "Shape of the week" share the row below. The three sections used to sit side by side with very different heights, leaving a large empty area under "How you used AI"
- Every item in "Worth a look" now looks the same where it appears. Flags on metrics without a card (Possible friction, Repeated prompts) sit in the same box as a flagged card, with the accent bar on the left, instead of as loose text under the heading
- Cards for metrics that the agents used in the period don't record (usage limit hits, compactions, subagents) now say "Not recorded" in plain text, with which agents don't record it, in a lighter dashed card. Before, a bold "—" stood where the number goes and read like 0 or a loading placeholder, and the card looked as heavy as a real figure
- Week calendar: sessions that overlap are laid out like other calendars. Sessions that start at about the same time sit side by side; a session that starts later is drawn on top of the earlier one, shifted right, so the earlier one keeps its full width. Before, any overlap halved both blocks for their whole length, so with parallel sessions titles shrank to a few letters ("Find out…")

## v0.21.0 - 2026-10-07

### Added

- The guide has a new table, "What each agent records": which metrics each agent's history supports (interruptions, usage limit hits, compactions, files changed, subagents, outputs, tokens and cost, Kiro credits, Long conversations, the resume command, and history retention), so you can tell a metric the agent doesn't record from one that was 0. A test keeps the rows that come from `core.Records` in step with the code

### Changed

- Metrics an agent's history doesn't record now say so instead of showing 0. When none of the agents used in the period records usage limit hits, compactions or subagents (for example, only Kiro), those cards show "—" and "Not recorded in … history", and the improvement prompt says the same. Session details show "Not recorded in … history" for files changed when that agent doesn't record edited files (Kiro IDE before 1.0 and Kiro Crew's own records), instead of "None". What each agent records is kept in one table in the code (`core.Records`), which also decides which sessions count for the outputs share

### Fixed

- Codex: session details now list the files Codex edited. Codex changes files with `apply_patch`, whose argument is a patch rather than a file path, so the list always read "None"; kiroku now takes the file names from the patch
- Codex: "Long conversations" now uses the context window Codex records (`model_context_window`) instead of the 100K-token fallback, like Claude Code's
- Kiro Crew: session details now show the estimated cost and tokens of turns run on backends other than kiro-cli. They were counted in the weekly and monthly summary but missing from the session itself. Sessions with both tokens and credits show both
- "Estimated cost per prompt" now divides only by the prompts of sessions that record tokens. Prompts to agents that record only credits (Kiro) made it read low when you used both
- Your year: "prompts per commit" in Focus now counts only Claude Code prompts, as the guide says. Since only Claude Code records the commits AI ran, other agents' prompts made it read high

## v0.20.0 - 2026-10-07

### Added

- New "Compactions" metric: how many times a conversation was compacted (summarized to free context, automatically or with `/compact`). The weekly and monthly summary shows the count, the number of sessions and the latest times; session details show the count and times, and the prompt flow marks each compaction ("automatic" or "manual" when Claude Code records it). Read from Claude Code (`compact_boundary` lines and the summary lines after them, counted once), Codex (`compacted` lines) and the SQLite history of Amazon Q / Kiro CLI (`latest_summary`, which keeps only the latest compaction). Kiro IDE, Kiro CLI (JSON) and Kiro Crew record no reliable marker and are not included. The session JSON has `compactions` (times) and `compactKinds` (`auto` / `manual`, only when known)
- Codex: usage limit hits are now counted. A turn that stopped on a usage limit or a rate limit (`task_complete` with `codex_error_info` `usage_limit_exceeded` or `rate_limit_exceeded`) and the usage limit response Codex records with its limit usage (`rate_limit_reached_type`) show up in "Usage limit hits" and as "Limit" marks on the calendar, like Claude Code's. A conversation that outgrew the context window does not count. The view no longer says the count is from Claude Code only
- Codex: turns you stopped (`turn_aborted` with reason `interrupted`) now count as interruptions in "Prompts with corrections or interruptions" and appear in the prompt flow
- Codex: new agent-specific metrics "Time to first token (median)" and "Turn duration (median)", from the timings Codex records when a turn finishes (`time_to_first_token_ms` and `duration_ms`; turns that ended in an error are left out)
- Claude Code: "Peak context usage" among the agent-specific metrics. Claude Code doesn't record how full the context window was, so kiroku divides each response's input (new input plus cache reads and writes) by the model's context window from a table taken from the official docs (1M for Fable 5 / 5.1, Sonnet 5 and later and Opus 4.7 and later, 200K for Haiku 4.5, Opus 4.6 and Sonnet 4.6 among others). Opus 4.6 and Sonnet 4.6 count as 1M once a response read more than 200K (their `[1m]` variant doesn't show in the history). Models not in the table get no value
- Claude Code: when a usage-limit message says when the limit resets (such as "resets 3:45pm"), the prompt flow, the "Usage limit hits" card and "Worth a look" show that text with the hit. It is shown as written, since it often has no date or time zone
- Codex sessions now have an estimated cost. The price table includes OpenAI's standard rates for gpt-5 and later models and `gpt-5.3-codex` (uncached input, cached input, cache writes and output), and a response whose input is over 272K tokens is priced at the long-context rates. Codex session details show the estimated cost and tokens. Fast mode, Flex, Batch and regional-processing rates are not applied, since the history does not record them
- "Data sources" now lists models that are not in the price table under their own ID and were priced as a similar one (for example, `claude-opus-5-6` priced as `claude-opus-5`, or `gpt-5-codex` as `gpt-5`), and opens with a mark, since a new model may cost something else
- `kiroku json` writes `schemaVersion` (now `1`). The session fields listed in the new [docs/compatibility.md](docs/compatibility.md) (`id`, `source`, `project`, `start`, `end`, `nPrompts`, `cost` and so on) keep their name, type and meaning while `schemaVersion` stays the same, so scripts can rely on them; the other fields still change with the view
- [docs/compatibility.md](docs/compatibility.md) says what a version number promises from v1.0.0 on: what stays compatible within a major version and what may change in any release
- `docs/upstream/` keeps the numbers kiroku uses from Anthropic's and OpenAI's official pricing pages, written by `go run ./tools/prices`. A test checks that the price table matches them, and a weekly workflow opens an issue when the official pages change (kiroku itself still never fetches anything but its own releases)
- Kiro Crew: turns run on backends other than kiro-cli now get tokens and cost from Crew's usage records (`input`, `output`, `cache_create`, `cache_read` and the USD `cost` in `usage/tokens/`). When a conversation records no cost, it is estimated from the price table. Turns on Crew's Claude Code backend, and on its Codex backend when `session_map.json` says so, are left to the Claude Code and Codex histories, because Crew's session ids can't be matched to those histories and adding them would count the same tokens twice; "Data sources" says how many turns were left out
- Kiro Crew: new agent-specific metrics "Peak context usage" (`context_used` ÷ `context_window`) and "Turns that did not end normally" (`stop_reason` other than `end_turn`, such as cancelled, refused or a tool stall)
- Kiro IDE: new agent-specific metrics "Peak context usage" (from `session_metadata` lines with `contextUsage`) and "Model requests" (the number of `requestIds` in `usage_summary`). Both shapes come only from the reference implementations, so sessions without them show neither
- Kiro IDE before 1.0: credits, models and turn times are now read from the execution files Kiro keeps next to `workspace-sessions` (`<32-hex workspace>/<session>/<execution>` under `kiro.kiroagent`: `usageSummary[].usage`, `modelId`, `startTime` and `endTime`), linked to conversations by `executionId` or `chatSessionId`. Prompts take the start time of the execution that answered them, and sessions without execution files count the model selected in the conversation (`selectedModel`). Only real files and folders (no symbolic links) in that layout are read. The view gains Credits, Turns and Credits per turn for these sessions
- Amazon Q and Kiro CLI (SQLite): new agent-specific metrics "Context window" (`model_info.context_window_tokens`, 200,000 when missing as in the CLI) and "Peak context usage (estimate)", computed with the CLI's own estimate (characters ÷ 4, rounded to tens) over the stored history and the last context message length. It leaves out tool definitions and history dropped by `/compact` or `/clear`

### Changed

- Codex: limit usage now shows both windows Codex records, named by their length ("Peak 5-hour limit usage", "Peak weekly limit usage" and so on) instead of a single "Peak rate-limit usage" that only read the first window. A window whose length is not recorded keeps a generic name. Snapshots of other, per-model limits (a `limit_id` other than `codex`) are no longer mixed in
- "Long conversations" in "Worth a look" now needs the peak input to reach half of the model's context window where kiroku knows the window, instead of a fixed 100K tokens, which is small for models with a 1M window. Where the window is unknown, 100K tokens still applies
- Bedrock model IDs with a cross-region prefix (`us.anthropic.`, `eu.anthropic.`, `apac.anthropic.`, `global.anthropic.` and so on) are now priced; before, only `anthropic.` was removed, so they were left out of the estimated cost. Model IDs written with a dot, such as `claude-sonnet-4.5`, are read as `claude-sonnet-4-5`
- A model ID now matches a shorter price-table ID only at a separator: `gpt-5` prices `gpt-5-codex`, but not a different version such as `gpt-5.7-sol`, which is left out of the estimated cost and listed as not in the price table
- "Data sources" says the price table holds Anthropic and OpenAI public rates

### Removed

- BREAKING: `--root` (where Claude Code keeps its history) is now `--claude-root`, next to `--kiro-home` and `--codex-home`. The old name stops with a message that names the new one
- BREAKING: the old forms `kiroku --serve`, `kiroku --json FILE` and `kiroku -o FILE` are gone, ahead of v1.0.0. Use `kiroku serve`, `kiroku json -o FILE` and `kiroku html -o FILE`; kiroku now says which one to use instead of running the old form. Options given without a command (such as `kiroku --sources kiro`) are an unknown command instead of showing the help

### Fixed

- Codex: "Peak context usage" now matches what Codex itself shows. It is worked out from the whole last response (`total_tokens`, not only input) and leaves out the 12,000 tokens Codex treats as always in the context, so it reads lower early in a conversation and reaches 100% when the context is full. When the conversation outgrew the window, it now shows 100% instead of 0%, and the "full" record Codex writes then is no longer counted as a response
- Claude Code: estimated cost from kiroku's price table now includes web searches (`server_tool_use.web_search_requests`) at $10 per 1,000 searches, counted once per response. Fast mode and US-only multipliers are not applied to them, and nothing is added where Claude Code's own cost record is used, since it already covers them
- Claude Code: history retention now follows `cleanupPeriodDays` in your organization's managed settings file (`managed-settings.json` and `managed-settings.d/*.json` in the system directory), which wins over your own `settings.json`. A value Claude Code would reject (not a whole number of 1 or more) no longer counts as set. The guide also notes that Claude Code (v2.1.248 and later) keeps sessions started or continued in Claude Desktop or Cowork unless `desktopSessionCleanupPeriodDays` is set
- The docs no longer state that Claude Code writes a cost of 0 on a subscription; the `cost-state` record is not documented, so they now describe only what kiroku does when a used model's recorded cost is 0
- Kiro Crew's `session.archive_retention_days: 0` is now shown correctly. Crew then deletes every archived conversation record at its next hourly cleanup, but "Data sources" said records are "deleted after a period" and `kiroku doctor` said Crew "keeps history for 0 days". Both now say the records are deleted at the next cleanup, within an hour, and `kiroku doctor` suggests a longer period or `kiroku archive on`

## v0.19.2 - 2026-10-07

### Security

- The "Resume" command in session details now quotes the project folder and session ID read from history. Before, a folder whose name contained shell characters (such as `;`, `|` or `$(…)`) could run another command when you pasted the command into a terminal, and a folder with a space made it fail. On Windows, a folder whose name has characters that cmd or PowerShell expand even inside quotes (`%`, `!`, `$`, a backtick) shows no resume command, and neither does a value with control characters

### Fixed

- On a phone, the legend chips above the week calendar no longer overlap the "Zoom" label. The chips now get a row of their own below "Color by" and the zoom buttons
- Usage limit hits now count Claude Code's current limit messages: "You've hit your session limit", the weekly, Opus and Sonnet limits, and the monthly spend limits and shared budget ("You've hit your monthly spend limit", "…org's monthly spend limit", "…team's shared budget")
- Usage limit hits no longer count errors that are not about your usage: the server's temporary throttling ("API Error: Server is temporarily limiting requests (not your usage limit)") and "Context limit reached" (the conversation is too long). An API key's rate limit ("API Error: Request rejected (429)") still counts
- Claude Code transcripts that Claude Code set aside (`<session>.orphaned-<timestamp>-<suffix>.jsonl`, also in `kiroku archive` copies) are no longer read as separate sessions, so a conversation is not counted twice
- Claude Code cache writes are no longer lost for older transcripts that recorded `cache_creation_input_tokens` as 0 with only the `cache_creation` 5-minute / 1-hour breakdown filled in; the breakdown is now used, which corrects tokens and estimated cost
- Kiro CLI (SQLite) and Amazon Q no longer count lines the CLI writes on your behalf as prompts. Pressing Ctrl+C during a tool run now counts as an interruption, and the text sent when you deny a tool with "n", the summary request of `--resume` without input, and the messages after a response timeout or history overflow are shown as "Added by the agent" in the prompt flow. The fixed reply after an interruption and the `--resume` summary are no longer shown as the reply to your previous prompt
- Kiro CLI (SQLite) and Amazon Q: turns that return tool results (which carry no time of their own) now use the time the request was sent, so their tool calls, response times and response sizes land on the right day in the weekly view instead of at the start of the session
- Kiro CLI (SQLite) and Amazon Q: the model is counted only for turns that actually sent a request to it, not for interruptions, MCP `/prompts` lines or other lines the CLI adds without a request
- The "Response length (average)" metric of Kiro CLI (SQLite) and Amazon Q is now "Response size (average)" in bytes. The CLI records the size in bytes (the reply plus the tool input JSON), not in characters as the label said
- kiroku now reads how long Kiro Crew keeps its conversation records instead of saying it cannot. It reads `session.archive_retention_days` from Crew's `config.json` and `config.local.json` (the overlay wins) the way Crew does: 30 days by default, and `null` or a negative number means Crew never deletes them. While it is at 30, "Data sources", the notice above the summary and `kiroku doctor` warn about it like Claude Code's default, name Kiro Crew rather than Kiro CLI, and suggest the setting in `config.local.json` in Crew's nested form; the notice no longer points every agent at `~/.claude/settings.json`. The docs link goes to Crew's configuration reference, which documents the setting
- The docs no longer say that Crew keeps its usage records (`usage/tokens/`) for about two weeks; Crew does not delete them
- Codex: tokens written to the prompt cache are no longer counted twice. `cache_write_input_tokens` is part of `input_tokens`, like cached tokens, so it is now taken out of input instead of being added on top
- Codex: newer rollouts (`history_mode: "paginated"`) now read prompts and replies from the `item_completed` events Codex writes for them. kiroku looked for `user_message` and `agent_message` items, but Codex writes `UserMessage` and `AgentMessage`, so prompts fell back to raw model input, which also holds the context Codex adds
- Codex: subagents and forks no longer count the parent's history again. Codex copies it into the child's file with the time of copying, so kiroku's time check let it through (the parent's prompts, tokens and tool calls were added to the subagent or fork). The end of the copy is now found from what Codex records (`subagent_history_start_ordinal`, or the child's own `thread_settings_applied`); older files still use the time check
- Codex: instructions Codex adds as `developer` messages are no longer shown as the AI's reply to a prompt

## v0.19.1 - 2026-10-07

### Changed

- Pressing a metric in "Worth a look" opens it in a dialog in the middle of the screen instead of jumping down to the summary, so you stay where you were, with the calendar still behind it. The dialog has what was observed, why it matters, the 8-week trend, why it was flagged, the related sessions (each opens its details), the threshold, and what the metric is, what it doesn't tell you and what to try. "Show in the summary" still takes you to the metric in place, and `Esc` or "Close" returns to the link you pressed
- "Data sources" at the bottom of the summary is folded into one line ("Data sources · 1 history read"). It opens by itself, marked with a warning triangle, when something needs your attention: a file that couldn't be read, history that will be deleted at its default setting, or models missing from the price table
- The footer shows the kiroku version, linked to its release page, and a link to the GitHub repository; the "Keyboard shortcuts" link it used to have was removed in v0.19.0. The links only open when pressed and send no referrer

### Fixed

- Kiro CLI conversations now show the AI's replies in the prompt flow. kiro-cli records no time on the lines with its replies, and kiroku dropped every reply it could not place, so none appeared; each reply is now placed at the end of its turn, or at its prompt's time when the turn has no end recorded
- Kiro IDE no longer shows the model's thinking (`operationType: "Reasoning"`) as the reply to a prompt, and finds replies whose text is nested inside `content` or `text` instead of showing none

## v0.19.0 - 2026-10-07

### Added

- The "?" explanation of a metric with a history (active time, prompts, tokens, Git commits, estimated cost, the cache share and the metrics "Worth a look" can flag) now shows its 8-week trend (8 months in month view) even when the metric is not flagged, so you can check whether something you tried changed it

### Changed

- Until you zoom, the week calendar picks the height of an hour that fits the week's working hours, so evening sessions are not cut off at the bottom on a laptop screen (44px as before, or 36px when the day runs long). Once you zoom, your choice is kept
- Search results scroll in their own boxes, so the calendar stays close below them instead of being pushed several screens down, and the note about filtered figures sits right above the calendar it describes. "Show 50 more (10 of 77 shown)" and "Show the last 17" replace "Show 50 more of 67", which read like a different total
- The legend wraps onto more lines instead of cutting off chips at 1000px and on phones, and keeps its "click to hide" hint and the zoom buttons at every width. The note shown while filtering says that sessions and commits under each date count only what is shown
- In the month view between 821px and 1180px, each week on the left shows its active time and tokens in the short form ("23.3h", "22M") instead of cutting them off. Session names in narrow calendar bars break between syllables with a hyphen rather than anywhere inside a word, and "Skip the calendar" appears over the page instead of pushing it down

- What you need is easier to find. "Worth a look" now sits right under the key figures, above the calendar, instead of below it (on a phone it used to take about a screen and a half of scrolling). Search results appear between the legend and the calendar instead of below it, newest first, 10 at a time with "Show N more of M" instead of up to 100 at once. The line above the dates says "Last week" or "Last month" when that is what you are looking at, since kiroku often opens on last week. "Copy review prompt" (renamed from "Review this session with AI (copy prompt)") sits next to "Copy prompts" above a session's prompt flow instead of after the whole flow, and the message after copying says the prompt includes your prompts
- The legend shows each project's (or branch's or agent's) active time, in the same split as "① By project", instead of its session count, which read like time, and says that an item can be clicked to hide it. "Color by" is the selector at the start of the legend at every screen width, and the zoom buttons, now labeled "Zoom", sit at the right end of the legend in the week calendar instead of in the header
- Less to read past. In "① By project" the cards no longer repeat each project's main models and top sessions (models and the heaviest sessions are in "④ How you used AI"). Total AI run time moves under "More metrics" in "③ How you spent time", agent-specific metrics fold into one line at the end of "④", and a session's models, tools and agent metrics fold into one line on the right. The prompt in "⑥ Ask AI for suggestions" is folded under "Show the prompt"; "Copy prompt" copies it as before
- Smaller wording fixes: Repeated prompts say that the counts are for the week or month shown, and that the threshold counts different prompts; flagged metrics say "N more sessions" and "N also in the list below"; Subagents say "Total run time"; the subagent types under "By model" have their own heading; the 8-week trend is drawn larger; and the month calendar says that each week on the left counts all 7 days, including days in the next or previous month
- The header's "?" button, which only ever opened the keyboard shortcuts, shows a keyboard instead, and keyboard users can jump past the calendar's bars and marks to the summary with "Skip the calendar"

### Removed

- "Daily trend" in "② Cost and outputs". Each day's active time, tokens or credits, sessions and commits are in the calendar's day headings and month cells, and the tooltip on each day adds estimated cost
- "Export prompts" in the summary heading. Copy a session's prompts with "Copy prompts" in its details, and share a week's work with the weekly report draft
- "Focus blocks" in "⑤ Shape of the week" (the count and the longest stay in "③"), and "Weekend" in "③", which the calendar already shows
- The "Marks" row under the legend (each mark has its own label, and the "?" of Git commits explains them), and the "Keyboard shortcuts" link at the bottom of the page (the button at the top right and `?` open the same list)

## v0.18.0 - 2026-10-06

### Added

- The prompt flow now shows what the AI wrote back. Each prompt is followed by the AI's reply with its own time, so a session reads as the conversation it was: the last thing the AI said to you in that turn (its own words, not its thinking, tool calls or tool output). The HTML keeps the first 160 characters, and `kiroku serve` adds "Load the full reply" to read the rest. It is read from Claude Code, Kiro IDE, Kiro CLI, Kiro Crew, Amazon Q Developer CLI and Codex CLI; "Only user prompts" hides the replies, and "Copy prompts" still copies only your prompts

### Changed

- "② Cost and outputs" now shows what the work left behind, not only how many commits it reached: beside Git commits (with the share AI ran) are lines changed with the average per commit, files changed, pushes made from this computer, and pull requests when an agent created one and it was recorded. "Prompts" joins the cost side, so what you put in reads as time, prompts and usage. Lines, files and pushes are counted from git, so they appear only when git commits were read; files changed counts up to 40 files per commit and says "at least" when a commit had more. The improvement prompt ("⑥ Ask AI for suggestions") gets files changed and pushes too
- Wording that was off in the view: "Sessions that reached a commit" is now "Sessions that reached a commit or PR", which is what it has always counted; the card that replaces Git commits when no git repository could be read is called "AI commits" in its "?" too, and the side now says that lines, files and pushes were not measured rather than leaving them out silently; "Lines changed" says "per git commit", since its average divides by every git commit while estimated cost per commit divides by the commits AI ran; a figure that did not move reads "no change" instead of "+0"; and the project cards in "① By project" say "Left behind" like the panel, instead of "Outputs"
- The two metrics that divide one side by the other (estimated cost per commit and sessions that reached a commit) are a "Compared" row under the two sides again, instead of sitting among the outputs, since neither is something the work left behind. For the same reason the right side is called "Left behind" rather than "Outputs" (the panel keeps its name): commits, lines and files are the amount produced, and work that leaves nothing in git, such as research or review, never appears there. Its "?" now says so, and says that the by-AI share, estimated cost per commit and sessions that reached a commit come from Claude Code only, the one agent whose history records the commits it ran, while the cost side counts every agent
- Session details put the prompt flow in the first view. The numbers at the top are now a smaller four-column strip, "Prompt flow" shares its line with "Everything / Only user prompts" and "Copy prompts", and "Review this session with AI (copy prompt)" has moved below the flow, where you reach it after reading. On a 1440x900 screen the flow starts about 250px higher and shows about eight entries instead of three, without scrolling
- Tokens and Kiro credits now have their own box everywhere the view used to share one. "Month-end projection (estimate)" in "④ How you used AI" is now two boxes, "Month-end cost (estimate)" and "Month-end credits (estimate)", instead of one box reading "≈ $12 · 34 credits"; on phones, a month calendar cell with both shows tokens and credits on their own lines instead of only tokens. They are separate allowances, so for anyone using both Claude Code and Kiro each is now something you can read on its own

### Fixed

- Estimated cost showed `$0.00` even after hours of work on a Claude subscription. Claude Code records its own cost in the history (`cost-state`), and on a subscription there is no per-token bill, so it writes `0` for every model. kiroku followed that figure and reported nothing spent. It now keeps its own price-table estimate for any model whose recorded cost is 0 although it was used, and session details say "(from Claude Code)" only when that record was really used
- When none of the models used are in the price table (Codex, for example), the estimated cost is now shown as `—`, with the reason when you point at it, instead of `$0.00`, which read as if nothing had been used. "Month-end cost (estimate)" is left out in that case, and the prompt for an AI review says the cost is unknown. Tokens that could not be priced were already listed under "Data sources"

## v0.17.0 - 2026-10-06

### Added

- `kiroku doctor --all` lists every agent kiroku looks at, with the place it looks, including the ones with no history yet
- `kiroku doctor` now shows which kiroku is running, and warns when another kiroku comes first in your `PATH` so `kiroku` runs that one instead (`install.sh` already warned about this)
- `kiroku update` shows how much of the download has arrived, so a slow connection no longer looks stuck

### Changed

- kiroku now uses colour and emphasis to separate what is fine (green `✓`), what needs attention (yellow `!`), what failed (red) and what is only a note (dim), and puts the commands and addresses you are meant to use in bold. Only when writing to a terminal: piped or redirected output, `NO_COLOR` and `TERM=dumb` stay plain
- `kiroku doctor`, `kiroku serve` and `kiroku html` no longer list every agent with no history. They name the ones your history was found in, and sum the rest up in one line, so the lines that ask you to do something are not pushed down the screen. Agents whose files could not be read are still listed, and when no history is found at all, every place kiroku looked is shown
- `kiroku doctor` writes `not installed` instead of `none` for the place an agent that isn't installed keeps its history, and `kiroku serve` says `1 session loaded`, not `1 sessions loaded`
- `kiroku update --to` for a version that does not exist now says so and points at Releases, instead of reporting a bare `404 Not Found`
- When `install.sh` installs into a folder that is not in your `PATH`, it now names the file to add the line to for your shell (`~/.zshrc`, or `~/.bash_profile` on macOS and `~/.bashrc` on Linux), uses `fish_add_path` for fish, says that a new terminal is needed, and shows how to start kiroku right away without one
- When `gh` is too old to check the build provenance, the warning now shows which version of `gh` was used and where to update it

## v0.16.0 - 2026-10-06

### Added

- `kiroku open` opens the view of a running `kiroku serve` with its key, and `kiroku open --print` prints the address with the key for another device

### Fixed

- The prompt that asks an AI to review a week or month listed flagged metrics with HTML escapes such as `&lt;` and `&amp;` instead of the characters. It now takes the text of the flagged metric from the HTML properly, where it used to remove anything that looked like a tag with a regular expression (a CodeQL finding)
- Code quality findings from CodeQL in the view's script: the dollar sign of estimated costs is now formatted in one place (which also escapes the `<` of `<$0.01`), the function that swaps in new data from `kiroku serve` sits next to the variables it sets, and unused variables are gone

### Security

- `kiroku serve` now shows your history only to browsers that have its key, so other users of the same computer can no longer read it at `http://localhost:8484/`. The key is kept in a file only you can read (`serve-key` in kiroku's settings folder), and `kiroku serve` and `kiroku open` give it to your browser, which then keeps it in a cookie: the plain address and bookmarks keep working in that browser. In another browser, run `kiroku open` once. See "The key of kiroku serve" in docs/guide.md

## v0.15.0 - 2026-10-06

### Added

- Each release now includes its build provenance as a signed Sigstore bundle (`kiroku_<version>.sigstore.json`), so the archives can be checked with `gh attestation verify <file> --repo MichinaoShimizu/kiroku --bundle kiroku_<version>.sigstore.json` without looking the attestation up on GitHub. See "Verifying a release" in SECURITY.md
- Fuzz tests for the code that reads history files (Claude Code, Codex, Kiro CLI, Kiro IDE), reads lines and timestamps, turns git remotes into links and builds the HTML view. Their seeds run with `go test ./...`, and a weekly workflow (and every push to main) fuzzes each one for a short while

### Security

- The view no longer shows a git remote's credentials in commit links when the user name contains `@` (`https://user@corp:token@host/...`): everything up to the last `@` before the host is dropped. Remotes with no host (`http://`) no longer become links. Found by the new fuzz tests

## v0.14.0 - 2026-10-06

### Added

- Each release archive now comes with an SPDX SBOM (`<archive>.sbom.json`) listing the Go version and every Go module, with its version, built into kiroku. The SBOMs are in `checksums.txt` and carry build provenance like the archives
- Release binaries can be rebuilt bit for bit from their tag: `sh tools/reproduce.sh v0.13.3 linux amd64` builds the tag and compares the result with the released binary. See "Verifying a release" in SECURITY.md

### Security

- CodeQL now analyzes the Go code and the view's JavaScript on every pull request and weekly, and OpenSSF Scorecard checks the repository's security practices and publishes the result (badge in the README)

## v0.13.3 - 2026-10-06

### Changed

- `kiroku serve --help` now says which host names `--allow-host` adds to: `localhost`, `127.0.0.1` and `::1` always work, and this computer's own IPs and host name only when serve listens on all interfaces (such as `0.0.0.0:8485`). Before, it said this computer's names and IPs always worked. `kiroku autostart --help` no longer ends with an empty "Flags:" heading

### Security

- `kiroku html` and `kiroku json` now write files only you can read (`0600`; before, `0644`, readable by other users of the computer). They write a temporary file in the same folder and rename it into place, so a symbolic link left at the output path is replaced instead of followed, and a failed write leaves the old file as it was
- `kiroku serve` no longer serves a Kiro IDE (before v1.0) history file outside its history folder when `sessions.json` lists a session ID such as `../../outside/creds`: such IDs are skipped and reported as unreadable. `/history` also only serves files (after following symbolic links) inside a folder kiroku reads history from or the `kiroku archive` folder
- kiroku no longer runs programs that a repository's own git settings name (fsmonitor, hooks, textconv and external diff drivers, pager) when it reads commits in folders from your history. It also ignores the system git config, never asks for credentials, and on Windows skips network (UNC) paths. Your global git config (`user.email`, `safe.directory`) is still used
- `kiroku serve` now times out clients that don't finish sending request headers within 10 seconds, limits headers to 64 KB and closes idle connections after 2 minutes
- Every `kiroku serve` response now tells the browser not to show it inside another site's frame and not to send it to or share it with other sites (`X-Frame-Options`, `Content-Security-Policy: frame-ancestors 'none'`, `X-Content-Type-Options`, `Referrer-Policy`, `Cross-Origin-Opener-Policy`, `Cross-Origin-Resource-Policy`)
- `kiroku update` accepts only version names such as `v1.2.3` from GitHub and `--to`, follows only `https` redirects, and only extracts a regular file of at most 200 MiB. Going back to an older version with `--to` now needs `--force`
- `kiroku archive off` now deletes only the compressed copies (`.zst`) kiroku saved and folders left empty, instead of the whole `claude` and `crew` folders, and only offers to delete in a folder where `kiroku archive on` was run
- History lines longer than 64 MiB are skipped and reported as unreadable instead of being read into memory, and compressed history is read with a memory limit
- `kiroku autostart on` on Linux now writes environment variables containing `$` correctly (it wrote `$$`) and refuses values with newlines or other control characters, which could add lines to the systemd unit
- When `kiroku serve` cannot read history or turn on `kiroku archive`, the page now says to look at the terminal instead of showing the error with local paths
- The HTML view now carries a Content-Security-Policy: only its own inline script and stylesheet run (pinned by their SHA-256 hashes), nothing is loaded from elsewhere, and network requests are blocked entirely in a saved HTML file and limited to `kiroku serve` itself in the live view. If text from your history ever slipped into the page as HTML, it could not run scripts or send anything off the machine. The "Reading your history…" page of `kiroku serve` has the same policy
- Text copied for pasting elsewhere no longer turns history into live Markdown: in the weekly and monthly report draft and the prompt export, session titles, prompts and commit subjects have Markdown characters escaped (so `[text](javascript:…)` is not a link and `<!here>` is not markup), `@channel`-style mentions and project and branch names are shown as code, and commit links have parentheses and spaces percent-encoded. The "Ask AI" prompts put the history in a fenced block and tell the AI to treat it as data, not instructions
- Values used as HTML class names in the session details and agent-specific metric units are now escaped
- `install.sh`: the whole script now runs from its last line, so a download cut off halfway by `curl | sh` runs nothing. The build provenance check now also requires the file to be built on a GitHub-hosted runner from `main` or the release tag. The new `KIROKU_REQUIRE_ATTESTATION=1` makes the check mandatory: the install stops when `gh` is missing or not logged in, cannot reach GitHub, or the version has no attestation
- Release builds: GitHub Actions are pinned to full commit SHAs, GoReleaser is pinned to an exact version, each job gets only the permissions it needs (the Pages deploy job alone can deploy), checkouts don't keep the token, the release build doesn't use the Actions cache, and Playwright for the browser tests is pinned with a lockfile and installed without install scripts. Archive files now get the commit's time, so the same commit builds the same archives

## v0.13.2 - 2026-10-06

### Security

- Fixed a stored cross-site scripting (XSS) hole in the view. When a prompt of 12 or more characters was repeated in 3 or more sessions, the "Worth a look" finding about it inserted the first 40 characters of the prompt as HTML instead of text. A prompt containing HTML (typed, pasted, or sent by a script or an agent running `claude -p`) could therefore run script in the page. That script could read the history in the page, and under `kiroku serve` also the original history files. The text is now escaped like everywhere else. Links in the view now also open only `http(s)` addresses. CI now builds the view from history full of HTML and script payloads and checks that nothing runs
- Please update, and delete or regenerate HTML files written by earlier versions if you share them

## v0.13.1 - 2026-10-06

### Changed

- Dates in the view and in copied text are now written one way, in English regardless of the browser's language: the period as "Sep 28 – Oct 4" or "October 2026" (instead of "9.28—10.4" and "2026.10"), days as "Sep 28" or "Mon, Sep 28" (instead of "9/28" and "Mon 9/28"), the oldest record as "Sep 1, 2026" (instead of "2026/9/1"), and "Generated Oct 6, 2026, 17:14" (instead of the browser's own format, such as "10/6/2026, 5:14:16 PM"). The weekly report draft and the "Ask AI" prompt say "Sep 28 – Oct 4, 2026" or "October 2026" instead of "2026/9/28–2026/10/4". Times stay in 24-hour form as on the calendar
- The weekly and monthly report draft now ends with a plain "_Drafted with kiroku_" line instead of an HTML comment, which showed up as is when pasted into Slack and other places that don't read Markdown as HTML
- Session names on the week calendar now wrap onto up to three lines when the bar is tall enough, instead of being cut to a few characters on one line (such as "Add E2…" at 1440px). Bars too narrow for a word still show the start time or only the color, as before
- On phones, the week calendar's right edge fades while there are more days to scroll to, and the session count ("20 / 20 sessions") is shown above the legend instead of at the end of its scrolling row, where it was off screen
- The details panel no longer has a top bar holding only the close button: Back and close now sit at the right of the first line, next to the kind of details

### Fixed

- "Active days" in a week or month still in progress counted out of the whole period ("6/31" on October 6). It now counts out of the days so far, including today, and says so: "6 of 6 so far"
- The ‹ › buttons keep their place with the new, longer date ("May 25 – May 31" is the widest)
- On the most zoomed-out week calendar, short bars showed their name below the bar, outside it. Names now stay inside bars down to 10px tall and are cut with "…"
- The Data sources panel had extra space above its heading

## v0.13.0 - 2026-10-06

### Added

- The week calendar now has a "Marks" key under the legend that shows what its marks at the right edge of each day mean: a commit (filled when run by AI), a push and a pull request, drawn the same way as on the calendar. Before, only the "?" of Git commits explained them, and pushes and pull requests were not explained at all

### Changed

- `install.sh` now also checks the build provenance (GitHub artifact attestation) of the downloaded file with `gh attestation verify` when the GitHub CLI is installed, and stops if it does not match. When `gh` is not logged in or cannot reach GitHub, it warns and installs anyway, since the file already matched `checksums.txt`. Without `gh`, or for versions before v0.12.0 (which have no attestations), it works as before. Set `KIROKU_SKIP_ATTESTATION=1` to skip the check
- The Project / Branch / Agent switch at the top now has a visible "Color by" label (on narrow screens, next to the selector left of the legend)
- Commit counts with how many AI ran are now written "22 · 14 by AI" in the key figures, project cards and session details, as in the Outputs card, instead of "22 (14)" or "Commits 22 (AI 14)"; the key figure is now called "Git commits"
- Project cards no longer show a Credits row that is all "—" when the period has no credit records
- In a file written with `kiroku html --week` / `--month`, the button that returns to the period now reads "Back to included week" / "Back to included month", ‹ › only move within the included period, and search says it looks "in this file" instead of "all time"

### Fixed

- In "Worth a look", the trend under a flagged metric said "flagged when high", so a metric flagged in a week where its value was the lowest of the 8 weeks looked wrong. The trend now only says which direction is worth a look ("higher is worth a look"), and a separate line gives the actual reason with this period's numbers, such as "Flagged because 2 sessions this week crossed the threshold". A trend shown under a related metric (Long conversations under Heaviest sessions, for example) now names the metric it tracks
- The "?" of Outputs said that commits made by hand are not included, while the card counts Git commits including them (with how many AI ran). It now explains both counts. The "?" of Estimated cost per commit now says it divides Claude Code's estimated cost by the commits AI ran, which is what the card uses, and the card no longer shows the estimated cost of all agents as if it were the number divided
- ‹ › and → could move into future weeks and months without end, where the key figures said "This week · No records". You can no longer move past the current week or month (in the demo, by its clock), and a period with no records is named by its week or month, such as "Week of 8/3"
- ‹ › moved sideways as the width of the date changed ("9.28—10.4" vs "11.24—11.30"), so clicking repeatedly could miss. The date now keeps the width of the longest one
- `kiroku update` now stops with "too many arguments" when given a version without `--to` (`kiroku update v0.11.0`). Before, it ignored the version and everything after it, so it installed the latest release instead, and `kiroku update v0.11.0 --check` replaced kiroku instead of only checking. Use `kiroku update --to v0.11.0` to install a given version
- In the view, ← and → no longer change the week while details (a session, commit, push or pull request) are open, or while the focus is on a control that uses the arrow keys itself (the Color by, View and Daily trend toggles, the search box and selects). Before, ← with details open closed them and jumped to the previous week, and the keyboard focus was lost. When moving weeks with the focus in the summary, it now moves to the period heading instead of being lost
- The browser's Back button now closes open details (or goes back one step when you moved from one detail to another inside them) instead of leaving the page. Closing details with Esc or the close button leaves the browser history as it was. This works for files opened directly and under `kiroku serve`
- Screen readers no longer read the whole weekly or monthly summary aloud each time you change the period; only the period heading is announced
- "Worth a look" links and "All-time search results ↓" now move the keyboard focus to where they scroll, so Tab continues from there instead of from the top of the page
- The details panel is announced by what it shows ("Commit details", "Push details", "Pull request details" or "Session details") followed by its title, instead of always "Session details"
- The page now has a top-level heading (kiroku), and the numbered summary sections are headings, so screen reader users can jump between them. They look the same as before
- The tooltip for a session block in the calendar now also appears when you reach the block with Tab, not only on mouse hover
- Esc now closes the most recently opened "?" explanation and returns the focus to its "?" button. Before, Esc did nothing to it
- Enter in the search box now moves the focus to the first search result (results are shown below the calendar), so you can open one with the keyboard right away

## v0.12.0 - 2026-10-06

### Added

- `kiroku doctor` now tells you when a newer kiroku is out ("! version: v0.11.0, and v0.12.0 is available") and lists `kiroku update` under "Next". It asks GitHub Releases while it reads history, waits at most 5 seconds, and says so when it could not check. `--no-update-check` skips it; builds from source are not checked

### Security

- Release builds now use Go 1.26.8 instead of Go 1.24.7, whose standard library had known vulnerabilities in code kiroku uses (`net/http`, `crypto/tls`, `crypto/x509` and others; found with `govulncheck`). The zstd library (klauspost/compress) is updated to v1.18.7 and golang.org/x/sys to v0.44.0, which also had fixes for known vulnerabilities in code kiroku does not call. Building from source (`go install`) now needs Go 1.25 or later
- CI now runs `govulncheck` on every pull request, and Dependabot proposes updates to Go modules and GitHub Actions each week
- Release files (archives and `checksums.txt`) now carry GitHub artifact attestations, signed records that they were built from this repository by its release workflow. Check a downloaded file with `gh attestation verify <file> --repo MichinaoShimizu/kiroku`

## v0.11.0 - 2026-10-06

### Added

- `kiroku autostart on` is back: it starts `kiroku serve` in the background each time you log in (launchd on macOS, a systemd user service on Linux), so the view is always at `http://localhost:8484/`. Since `kiroku serve` now answers while it reads history, `autostart on` waits a few seconds and reports whether it is answering (or reading your history), and `kiroku autostart` and `kiroku doctor` show the same. `kiroku autostart off` removes it; the guide also lists how to remove it by hand if kiroku is already deleted

## v0.10.1 - 2026-10-06

### Fixed

- `kiroku serve` now listens on its port and opens the browser right away, before reading history. Before, it read all history (and git) first, so with a lot of history the browser it opened, or anything else checking the port, could not connect for a long time and serve looked broken. While the first read runs, the page shows "Reading your history…" and switches to the view when it is done; the terminal prints the URL at once and then "N sessions loaded in Xs". If the first read fails, the page shows the error, and serve keeps running and tries again when history changes. A port already in use still fails at once, without reading history
- Kiro IDE, Kiro CLI, Kiro Crew and Codex history files that cannot be read (a damaged or cut-off file, including a damaged `kiroku archive` copy of Crew's archived transcripts or a cut-off `.jsonl.zst` Codex rollout) are now listed as unreadable files in "Data sources" instead of being skipped silently, as Claude Code already did. What could be read is still shown, other conversations load as before, and a file deleted while kiroku was reading is not counted
- `kiroku serve` now rereads a git repository when only its reflog changed, such as old push records expiring, a push recorded while the remote-tracking branch ended up where it had been before, or a commit made on a detached HEAD. Before, it kept showing the previous pushes until a ref moved. This checks the reflog files' size and time, so reloads run no extra git commands
- In time zones with daylight saving time, the week calendar placed sessions, commits and the "now" line an hour off on the day the clocks change, and the daily trend could count a session near midnight on the wrong day. Days and hours are now counted on the calendar and the clock
- Session bars too narrow for a whole word (on phones, or overlapping sessions at 1000px) showed fragments like "S… u… t…". They now show the start time instead, or only their color when even that does not fit; wide bars are unchanged
- 8-week (8-month) trends started partway along their box when the earliest periods had no records, which squeezed the line into a short stretch. The line now starts at the first period with records and uses the full width

## v0.10.0 - 2026-10-06

### Added

- Pushes and pull requests open their own details, from the calendar's right edge and from a session's prompt flow, like commits do. A push shows where it went, when, the commits it sent (each opens its commit), the session running at the time and a `git log` command for the same range; a pull request shows its link, when it was created, the session that created it and that session's pushes and commits before it. A commit's details show the push that sent it. Before, the push mark only had a tooltip and the pull request mark opened the session. The JSON output's `meta.push` entries have `prev` and `hashes` (the commits sent, up to 50)

### Removed

- BREAKING: `kiroku autostart` is gone for now. `kiroku serve` started at login could stay silent for a long time while it read history, so it looked broken. `kiroku doctor` no longer reports autostart. If you ran `kiroku autostart on` with v0.9.0, remove what it registered:
  - macOS: `launchctl bootout gui/$(id -u)/io.github.michinaoshimizu.kiroku; rm ~/Library/LaunchAgents/io.github.michinaoshimizu.kiroku.plist ~/Library/Logs/kiroku.log`
  - Linux: `systemctl --user disable --now kiroku.service; rm ~/.config/systemd/user/kiroku.service; systemctl --user daemon-reload`

## v0.9.0 - 2026-10-06

### Security

- `kiroku serve` exposed on the network (such as `0.0.0.0:8484`) no longer answers requests addressed to other host names, so a web page you visit cannot point its own domain at your computer and read your history (DNS rebinding). `localhost`, this computer's IPs and host name keep working, and `--allow-host` adds more names

### Added

- `kiroku doctor` checks your setup in one go: which agents' history was found (with the oldest date and where it looked), whether an agent will delete old history and which settings file to change, whether `kiroku archive` is on, whether git is found and whether autostart is on, then says what to run next. It only reads
- `kiroku autostart on` starts `kiroku serve` in the background each time you log in (launchd on macOS, a systemd user service on Linux), so the view is always at `http://localhost:8484/`. `kiroku autostart off` removes it, and `kiroku autostart` shows the status. Windows is not supported yet
- `kiroku html --week` and `--month` (`this`, `last`, a date or `YYYY-MM`) write an HTML file with only that week or month, to show someone a period without handing over all of your history. The file opens on that period, says at the top that it holds only that period, and shows times in the time zone it was written in wherever it is opened

### Changed

- The live demo opens on the latest week with every weekday filled in, instead of the current week (which is often nearly empty because the demo data is rebuilt every Monday). "This week" still jumps to the current week
- Short sessions in the week calendar now show their name: in smaller type when the bar is short, and just below the bar when it is too short for text and there is room
- Narrow session bars wrap names between words and shorten them with "…" instead of breaking words in the middle (on phones this showed one letter per line)
- While searching (or hiding items in the legend), the calendar's commit and push marks and the commit counts follow the matching sessions, and the totals that can't be filtered (the key figures at the top, and active time, tokens and credits under each date) are shown in grey with a note saying they cover all sessions
- 8-week (8-month) trends next to marked metrics label their first and latest points with the period and value, so they can be read without hovering
- When a week or month has both tokens and credits, every day in the calendar shows both rows (with "—" where there are none) instead of some days saying "Tokens" and others "Credits"

### Fixed

- "Worth a look" said metrics are marked (●), but they are marked with a warning triangle
- A marked metric whose related sessions also appear in the cards below (such as Long conversations under Heaviest sessions) now says how many are in the list below, so the count in its headline adds up
- The note "Links open each file as of the commits made during this session" appeared even when no file was a link
- The search results note said clicking a result opens its week; it opens the details (and moves the calendar to that week)
- README's Uninstall section no longer says kiroku keeps no data of its own: it now covers `kiroku autostart off` and removing the copies kept by `kiroku archive`
- A history line with an impossible time (such as `"timestamp": 100` or year 1) is ignored instead of making its session start in 1970, which added empty weeks and months, showed a wrong oldest record and slowed loading. Times before 2000 or more than a day in the future are not used
- A Claude Code history file that cannot be read to the end (such as a damaged `kiroku archive` copy) is now listed as an unreadable file in "Data sources" instead of silently showing a shorter conversation. What could be read is still shown, and other conversations load as before
- `install.sh` replaces kiroku in one step (it copies the new binary next to the old one, then renames it), so an interrupted install no longer leaves a broken `kiroku` behind. It also warns when another `kiroku` comes earlier in your `PATH` and would run instead of the one just installed
- `kiroku update` no longer fails on slow connections: the 60-second limit covered the whole download. It now waits up to 15 seconds to connect and 30 seconds for the server to answer, then gives the download up to 30 minutes
- `kiroku serve` no longer rereads all Codex history whenever anything under `~/.codex` changes (such as `log/codex-tui.log`). It watches only the session files and `session_index.jsonl`, and rereads only the threads that changed
- `kiroku serve` no longer rereads every git repository on each reload: a repository whose refs, `user.email` and origin are unchanged reuses the previous commits and pushes, and the commit count of each push is counted once. Each repository now has its own 30-second limit instead of one limit shared by all, so one slow repository no longer makes the commits and pushes of later repositories disappear; a repository that times out keeps its previous result and is listed in `meta.gitTimeout` in the JSON output
- Days are split correctly in time zones where daylight saving time starts at midnight (such as America/Santiago): after the skipped midnight, every later day used to start at 23:00 the day before, so September had 31 days and late-evening work counted on the next day. Weeks and months that start on such a day (`kiroku html --week` / `--month` included) now also start on the right date

## v0.8.1 - 2026-10-06

### Fixed

- Codex paginated rollout sessions now include user prompts recorded as `event_msg.item_completed` user-message items, with regression coverage for the current log shape

## v0.8.0 - 2026-10-05

### Added

- The live demo has link-card information (OGP / Twitter card) with a preview image, so sharing its URL on X and other sites shows a large card. Only the demo page gets it; the HTML you make yourself does not
- Pushes and pull requests appear next to commits on the right edge of the week calendar and in a session's prompt flow. Pushes come from your local `git reflog` ("update by push" on remote-tracking branches), so they cover pushes made from this computer; pull requests appear when an agent created one and it was recorded. kiroku still never contacts GitHub. The JSON output has `meta.push`

## v0.7.1 - 2026-10-05

### Changed

- README and screenshots show the current view, and the README describes the session details and the four daily figures in the calendar
- Small icons tell things apart by shape, not only by color: events in the prompt flow (commit, pull request, subagent, interruption, usage limit, added automatically) and its legend, Command (/) and Shell (>_) tags, the "Worth a look" mark (a warning sign instead of a dot), the usage-limit marker in the week calendar, and agent initials (CC, KI, KC, KW, Q, CX) in the legend and session details. Each icon sits next to its text label
- "Cost and outputs" no longer shows pull requests and lines edited by AI: they were only counted from Claude Code tool calls on this computer, so they often read 0. Git commits (with their lines) remain. The project cards and the "Ask AI" prompt drop them too; pull request links still appear in a session's details and in the report draft when they were recorded
- "Compared" is no longer a separate row: estimated cost per commit and sessions that reached a commit now sit under "Outputs". The two sides are now called "Cost" and "Outputs" (were "Spent" and "Left behind"), matching the panel name
- Credits are shown as whole numbers (below 1, with two decimals)
- Typography is consistent: text uses 9 sizes (10–30px, from 20 before), 4 weights (400–700), and set line heights and letter spacing. The font stack puts Latin fonts first now that the view is English (Japanese prompts still use a Japanese font), and the Japanese-only spacing setting is gone
- Calendar text is easier to read on darker days: labels use a darker gray, active time and tokens stand out by weight instead of a blue that faded on busy days, sessions and commits are quieter, and Sunday red is darker in the light theme
- The week and month calendars show the same four figures, labeled: active time, tokens (or credits), sessions and Git commits. In the month calendar, Git commits replace prompts, and the week column on the left (W40 and so on) now shows the four for that week. The week calendar's day headings use the same layout instead of the time, a usage line and a commit icon

## v0.7.0 - 2026-10-05

### Changed

- BREAKING: the view and the docs are English only. The language selector at the top right, the Japanese README and the Japanese guide are gone (`docs/guide.md` is now the English guide). Names kiroku makes up are English too, in the view, the JSON output and the command line: "(untitled)", "(unknown)", "Kiro IDE (legacy)" (was "Kiro IDE (旧)"), "Kiro Crew background work" and "Subagent …". Japanese prompts are still read and searched as before, and Japanese corrections such as 「やり直して」 still count
- The summary no longer shows the same number twice. Active time, estimated cost, tokens and credits appear only in "Cost and outputs" (no longer also at the top of "How you spent time" and in "How you used AI"); "AI commits" shows only when there are no Git commits (Git commits already show how many AI ran); the project split is no longer repeated in "Shape of the week"; project cards drop "Heaviest" (it is in "Heaviest sessions"); and the agent-specific metrics drop the cache share and subagent runs, which "How you used AI" already shows
- Sections with nothing in them are hidden: focus blocks, sessions with possible friction and repeated prompts in "Shape of the week / month" only appear when there are any
- Shorter text: notes that repeated the "?" help (wait time and switches, estimated cost, Git commits, agent-specific metrics) are gone, as are explanations under values such as "shorter is not always better". A flag on a metric no longer restates the value right above it. "Data sources" and the price table for estimated cost moved to the very bottom of the summary

## v0.6.0 - 2026-10-05

### Added

- "Oversized prompts" counts prompts of 4,000+ characters in the period (such as pasted logs or documents) and is marked in "Worth a look" when there are 3 or more. Each one also gets a "Long" tag with its length in the prompt flow
- The prompt flow tells apart what you typed from what entered the conversation on its own. Slash commands (such as `/review`) and `!` shell commands now count as your prompts, tagged "Command" or "Shell" in their own color. Notifications, `<system-reminder>`, hook and command output, automatic summaries, instructions from other agents or schedules and the expanded text of slash commands appear in another color with their kind, and are not counted as prompts. Before, all of these, slash commands included, were left out
- "Only user prompts" in the prompt flow shows just what you typed (user prompts), and "Copy prompts" copies it as Markdown with times. "Export prompts" in the summary heading does the same for the whole week or month, grouped by session
- The JSON output has `kind` on prompts (`command` or `shell`) and `notes` on sessions

### Changed

- Late-night time (22:00–6:00) is gone, since the hour alone says nothing about how you used AI: the "Late night" metric and its "Worth a look" flag, the shading of night hours in the week calendar, late night in day tooltips and the summary export, and the `night` fields in the JSON output
- "Daily usage" and the daily / weekly rhythm are now one chart, "Daily trend", at the bottom of "Cost and outputs". It switches between tokens, credits, estimated cost, active time, sessions and prompts, matching the month calendar, and is per day in month view too. Before, the rhythm only showed active time, and per week in month view
- Weekends look like holidays: Saturday is blue and Sunday is red, with a light tint, in the week calendar (date headings and columns), the month calendar (headings, dates and days) and the daily trend, in both themes
- The view now opens in English by default, whatever the browser's language. Choose 日本語 at the top right to switch; the choice is saved in the browser as before
- Shorter Japanese labels: "作業していた時間" is now "作業時間" and "作業した日" is now "作業日". In the prompt flow, "your prompts" is now "user prompts"
- Branches in Claude Code sessions use the branch most of the session was recorded on, instead of the first one, so a session that switched branches is counted under the right one
- Each day in the month calendar now lists, with labels, active time, tokens (credits when there are no tokens), sessions and prompts, instead of unlabeled numbers. Point at a day to also see estimated cost and commits. On phones it shows active time and tokens in a shorter form so they no longer wrap or get cut off
- The Japanese view, guide and README now call what you send to an agent a "プロンプト" (prompt) everywhere, instead of "依頼". The English view already said "prompt". The search box at the top is a little wider so the longer placeholder fits, and on phones the numbers in the header line up even when a label wraps
- Daily tokens, cost and credits stand out more: the "Daily trend" bars are taller with bolder values, and the per-day usage under each date in the calendar is larger and bolder
- Every chart now shows its values as soon as you point at it (or touch it): daily trend bars, project, share and model bands, each point of an 8-week trend, and the usage under calendar dates, with each value on its own labeled line. Before, some showed only after the browser's slow tooltip, and trend lines showed nothing per point
- "Cost and outputs" now reads as spent → left behind: the two sides sit left and right with an arrow between them, each in two even columns, and the metrics that compare them (estimated cost per commit and sessions that reached a commit) have their own "Compared" row that shows what was divided by what (for example, "Estimated cost $21.81 ÷ 14 AI commits"). Long notes under cards are shorter, so cards line up, and a marked metric no longer leaves a half-empty row

## v0.5.0 - 2026-10-05

### Added

- "Repeated prompts" in the summary lists prompts you wrote in 3 or more sessions in the period, grouping similar wording (prompts of 12+ characters), with how many times and in how many sessions. It is marked in "Worth a look", since a prompt you type every time can be written once as a custom command or in CLAUDE.md

## v0.4.1 - 2026-10-05

### Changed

- "What you can do" for metrics that can be marked in "Worth a look" now suggests one thing to try next period, instead of ending at "check" or "rethink" (for example, limit each day to 2–3 projects, or write long background once in CLAUDE.md instead of pasting it). The guides match
- The trend under a marked metric no longer says "lower is better" or "higher is better", which contradicted "What it can't tell you". It now says when the mark appears ("flagged when high"), and shows one value instead of a range like "3–3" when the 8 periods are all the same

### Fixed

- Closing a session or commit detail opened from the summary or from search results jumped back up to the calendar and moved focus to a calendar block. It now returns to where you were reading and focuses the card or result you opened
- Moving to another week or month with ← → while a calendar block had focus dropped focus to the page, so keyboard users had to Tab from the top again. Focus now stays in the calendar, on the first visible block of the new period
- The week calendar opened about an hour too late, so the first session of the day (such as one starting at 9:08) sat half hidden under the sticky date headings. It now opens at the hour the week's earliest daytime session starts. A block reached with Tab also scrolls clear of the date headings and the hour column
- On phones, a past week opened scrolled to Friday–Sunday; it now opens from Monday (the current week still opens at your latest day so far)
- In narrow blocks, where overlapping sessions share a day (on phones and at 1000px), names were cut to one letter. They now wrap, without the dot and times, so you can read them

## v0.4.0 - 2026-10-05

### Added

- The prompt flow in session details now shows what happened between prompts, in time order: commits (by AI or by hand, opening the commit), pull requests created, usage limit hits, interruptions and subagents starting
- Each prompt in the flow shows how long the AI worked and how long it waited for you (estimates from timestamps), and breaks of more than 30 minutes are marked. A key above the flow explains the marks
- With `kiroku serve`, "Load the full prompt" reads a prompt longer than the 400 characters kept in the HTML to the end. Without it, the flow says how long the prompt was

### Changed

- Going back from a detail opened from another (such as a commit opened from the prompt flow) returns to where you were reading, instead of the top
- Session details count commits the same way as "Commits during this session": the card is now "Commits (by AI)", including commits made by hand during the session, and pull requests have their own card. Without git history it still shows the commits and pull requests the AI ran

### Fixed

- Sessions with more than 50 prompts lost the rest: the prompt flow stopped at 50, and those prompts were left out of daily prompt counts, project switches and the prompt count by project. All prompts are now kept
- "Show all" on a prompt cut at 400 characters looked like the full text; it now says where it was cut

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
