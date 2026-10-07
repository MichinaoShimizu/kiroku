# Changelog

Notable changes to kiroku, in the style of [Keep a Changelog](https://keepachangelog.com/). Add changes under `## Unreleased`; its contents decide the next version ([Semantic Versioning](https://semver.org/), see `sh tools/next-version.sh`), and each release on GitHub uses its section here as the release notes.

## Unreleased

### Changed

- A week with no records says so once, in the calendar, with a button to the latest week that has records. The month is no longer lowercased ("jul 6"), the summary no longer repeats "No records" in its own card, and Color by, Zoom and the session count are hidden while there is nothing to act on
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
