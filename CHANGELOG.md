# Changelog

Notable changes to kiroku, in the style of [Keep a Changelog](https://keepachangelog.com/). Add changes under `## Unreleased`; its contents decide the next version ([Semantic Versioning](https://semver.org/), see `sh tools/next-version.sh`), and each release on GitHub uses its section here as the release notes.

## Unreleased

### Added

- Summary: the line charts of the two unit cost panels show each point's value next to it, as the bar charts do above their bars, so you can read the numbers without hovering. A period still in progress shows its value so far next to "So far"

### Changed

- The page (including `kiroku serve`) always opens on this week's calendar instead of the view (Calendar or Summary) and Week | Month used last time. Other choices, such as the figures in the calendar and the summary's switches, are still remembered

## v5.1.1 - 2026-11-24

### Fixed

- Claude Code: running out of usage credits in the middle of a response (`Usage credits ran out before the response was finished.`, the wording since Claude Code 2.1.328) is counted as a usage limit hit again
- Kiro: the calendar no longer shows the same session twice. A Kiro IDE / CLI (V3) session stored in two workspace folders (`sessions/<hash>/sess_<id>/`) is shown once, from the copy written last that has readable lines, and a Kiro CLI (SQLite) conversation that was moved to V2 and then opened in V3 is shown once, as its V3 session, instead of also as a Kiro CLI (SQLite) session

### Security

- Kiro: a Kiro IDE / CLI (V3) copy of a private (incognito or temporary) Kiro Crew conversation is now hidden even when its `session.json` has no usable `id` (missing, not a string, or cut off while Kiro was writing it); before, such a copy was shown with its prompts

## v5.1.0 - 2026-11-24

### Added

- Each release also has the kiroku binary on its own (`kiroku-darwin-arm64`, `kiroku-windows-amd64.exe` and so on), listed in `checksums.txt` and with build provenance, so `gh release download -R MichinaoShimizu/kiroku -p kiroku-darwin-arm64 -O ~/bin/kiroku` installs it in one command
- kiroku can be handed out from an organization's own GitHub repository, private or internal: a release built by `release.yml` in another repository remembers it, and its `kiroku update` (and the update check of `kiroku serve --check`) gets releases from there through the GitHub CLI and its login (run directly with fixed arguments, no shell and no token kept by kiroku; the download is still checked with `checksums.txt`). See "Inside an organization" in the guide
  - The links at the bottom of the view point to that repository, and `KIROKU_REPO=github.com/your-org/your-repo sh tools/screenshots/run.sh --html index.html` builds the demo page with them

### Changed

- The prompt flow is easier to read: it sits on the view's own dark background, your prompts are in kiroku's purple with dark text, and replies are in a slightly lighter dark gray with light text, so your prompts stand out and long replies aren't bright
- Japanese view: the Output panel, its unit cost and the flow from input are now named 産出 (産出単価, 投入から産出の流れ), paired with 投入, instead of 出力, which reads as LLM output tokens. Output tokens are still 出力

## v5.0.0 - 2026-11-23

### Changed

- Week calendar bars are filled with a tint of their Color by color (agent or project) and a wider stripe, so switching Color by visibly recolors them; busier sessions get a stronger tint
- Agents have their own set of colors, distinguishable also for red-green color-blind readers and never green, red or amber: Claude Code blue, Kiro IDE / CLI (V3) violet, Kiro CLI (V2) and Kiro CLI (SQLite) plum, Kiro Crew pink, Codex cyan
- Saturday and Sunday share one neutral background (no red Sunday, no blue Saturday), and the month calendar's shade is a neutral gray-blue that no longer looks like an agent's color and keeps its text readable on the busiest days
- Every number in the calendar and the band above it has an English unit, also in the Japanese view: "5 commits", "44 credits", "12M tokens"; the commit count is no longer green, and day headings wrap instead of hiding figures in narrow columns
- Today is marked more quietly: its date in the accent color, without an outline around the day or a filled date chip
- The Summary's Breakdown is one box with eight cards: Output, Where it went, When you worked, What changed from the last period, Tokens by type, By agent, Most edited files and How you worked. The "Other charts" box inside it is gone
- Removed the cards that repeated the panels or the calendar, or that seldom led to a next step: Trend, Cumulative, Spread per session, Cost per session by length, Cost vs outputs per session, Busiest day and Usage. Usage's read-from-cache share, prompts, subagents and the daily amount to stay at the last period's total moved to How you worked; its tokens and month-end estimates are in the Input panel
- Summary panels no longer offer choices that have nothing to show for the period (such as PRs with Kiro alone, or Pushes with one agent picked). Before, these were dimmed, and pressing one showed its reason in a line that pushed the chart down. The divisor switch of the unit cost panels shows only the divisors that have a figure for the measure picked
- The Input and Output bars are labeled with each period's amount (such as $1.2k, 44, 12M or 18h) instead of the change from the previous period; the change is in the bar's tooltip
- The Sankey diagram's heading is now "From input to output" (投入から出力の流れ)
- In the Input and Output bar charts, a bar more than 2.5 times the next highest is cut at the top with a break mark and its amount written on it, and the scale fits the other bars, so one unusual period no longer flattens the rest. The legend explains the mark
- The Summary no longer has the "Worth a look" line at the top. Each flagged metric still has its warning triangle, which opens its dialog, and the share of cost that reached no commit or PR is flagged on the Sankey diagram's heading
- The Sankey diagram's output column has "Files changed only" for sessions that created or changed files without a commit or PR, between "Commit or PR" and "None"
- By model in Where it went now includes Kiro credits: Kiro IDE / CLI (V3), Kiro CLI (V2) and Kiro Crew record which model each turn used, and kiroku now keeps that model with the turn's credits
- A week or month without records shows the empty calendar, with a one-line note and the button to the latest period with records above it, instead of only the note
- While searching, the page shows only the results: the period, legend, calendar and summary are hidden, since the results come from all time. Pressing a result opens its details; clearing the search shows the calendar at that result's week or month
- The prompt flow's colors move further from a messaging app's: a slate background, a muted sage bubble for your prompts and an off-white one for replies, with light text for times and copy buttons
- "From input to output" is headed at the same level as Input and Output, with "Sankey diagram" one level smaller under it
- The summary opens with "From input to output" (the Sankey diagram) at the full width, above the Input, Output and unit cost panels

### Removed

- BREAKING: kiroku has three commands: `serve`, `update` and `help`. The other commands were removed and stop with a message naming what to use instead:
  - `kiroku doctor` → `kiroku serve --check`
  - `kiroku archive [on|off]` → `kiroku serve --archive [on|off]`
  - `kiroku version` → `kiroku --version`
  - `kiroku open` → `kiroku serve`: running it while kiroku serve already runs at that address opens the running view in your browser with its key (with `--no-open`, it prints the address). It tells kiroku serve apart without sending the key, so a program holding the port never gets it. There is no longer a one-time address for another device (`--print`)
  - `kiroku autostart`: kiroku no longer starts at login. Running `kiroku autostart` once removes the login item that kiroku 4 added, and `kiroku serve --check` says so while one is left
  - `kiroku html`, with `--week` and `--month`: kiroku no longer writes the view to a file. The view, with its prompts, paths and commit messages, stays in kiroku serve on your computer

## v4.6.5 - 2026-11-23

### Changed

- The prompt flow's blue-gray background and green bubbles are slightly toned down
- Summary: the Input and Output panels draw their 12-period charts as bars, since they show amounts; the period shown is the darker bar, and a period in progress shows the bar so far with a lighter dashed bar up to the end-of-period estimate. Usage limit hits sit in their own row just under the x axis, and the period names stay horizontal (every other one when space is short). The unit cost panels stay line charts
- Summary: Breakdown starts closed, with a one-line list of what is inside, to keep the page short ("Other charts" is inside it). The view remembers whether you opened it, and pressing a Worth a look item whose card is inside opens it
- Summary panels: a choice with nothing to show for the period is no longer hidden. It stays in the switch, dimmed, with the reason as its tooltip and in one line when pressed (for example "Kiro IDE / CLI (V3) doesn't record pull requests, so there is nothing to divide by", or Pushes with one agent picked)
- Unit cost per output: Commits is now offered with Credits, and with Cost or Tokens when no AI commit is recorded, as the period's whole total per Git commit (the Git commits in Output). With AI commits, Cost and Tokens still show Claude Code's figure per AI commit; the figure's name says which

### Fixed

- Where it flowed: the takeaway now accounts for the whole amount in one sentence, such as "Of $15.14 estimated cost, $8.67 (57%) went to sessions with a commit or PR, $4.42 (29%) to sessions without, and $2.04 (14%) is from agents that don't record commits", leaving out parts that are zero. It used to divide only by the recorded part, so 834 of 3,000 credits could read as "100%". Its percentages, and those listed under the chart, are rounded to add up to 100

## v4.6.4 - 2026-11-23

### Changed

- Session details: the prompt flow looks like a classic messaging app again: a blue-gray background, your prompts as green bubbles with black text on the right and the AI's replies as white bubbles with black text on the left next to a round robot mark, each with a small tail. Times sit small beside the bubble's bottom edge, with the copy button above them, and events are dark labels with white text in the middle (red for limits and interruptions). The rest of the view stays dark, and a test checks the contrast of every text and mark in the flow (at least 4.5:1 for text, 3:1 for small marks)
- The summary opens with four panels, two by two on wide windows: Input and Output side by side (were Cost trend and Output trend), and under them Unit cost per input and Unit cost per output, with a two-colored line between the input and output sides and IN / OUT at its top (one column, each with its IN or OUT label, on narrow screens)
  - Unit cost per output divides by commits, pushes, pull requests and now 100 AI-edited lines; Unit cost per input divides by active hour, active day, session or prompt. Both replace the Unit cost trend panel and the Breakdown's Unit costs card, which is gone; the choices you made there are kept
  - In a period in progress, the week-end or month-end estimate sits on the same row as the value ("$6.87 So far ≈ $16.03 End-of-week estimate") instead of under it, and each panel's legend is under its chart, so the charts of panels side by side start at the same height
  - Each panel's legend lists only what its chart draws (the unit cost panels no longer list an end-of-period estimate and projection they don't draw), and Unit cost per input's note that each divides only by the sessions that record what it divides moved into its "ƒx", so Unit cost per output no longer has an empty line under its heading
  - With one agent picked, the note that per push is shown for All agents only moved from Unit cost per output's heading into the "ƒx" of its figures, so Unit cost per input no longer has an empty line under its heading
  - A panel switch with a single choice is shown too (it says what the panel counts or divides by), and on wide windows the two unit cost panels put Measure and Divided by on a line each, so panels side by side keep headings of the same height whichever agent is picked
  - A figure outside your usual range keeps the same box as the others: the green or red background and its extra padding are gone (the amber value with ↑ or ↓, and the verdict line of unit costs, say it), so metric names line up with the panel beside them
  - A value far outside your usual range keeps its amber color with a small ↑ or ↓ next to it instead of the "Far above usual" / "Far below usual" label (the words are read by screen readers and shown as the tooltip)
- Where it flowed (the Sankey) moved out of Breakdown to right under the four panels, at the full width of the page: each project is now one band in its project color across the project column (the gap between its two nodes, where its name sits, was empty), and every name sits on a backing so it never lies on a colored band
- The Breakdown's Outputs card is now called Output, the same word as the panel and the Sankey's last column

### Fixed

- Session details: a turn where the AI wrote no text reply shows its small note without a bubble again (a stray brace in the stylesheet had dropped that rule)

### Security

- Pushes whose reflog time is negative or past the year 9999 (a reflog line edited by hand, which git prints wrapped around as about 1.8e19) are dropped instead of being passed to the page as dates it can't show

## v4.6.3 - 2026-11-23

### Changed

- The summary's three figure panels are compact: Cost trend, Output trend and Unit cost trend each show one trend chart with a switch, and the view remembers each choice
  - Cost trend switches between Cost, Credits, Tokens and Active time; Output trend between Commits, Pushes and PRs, with the change (%) on each point
  - Unit cost trend has two switches, the measure (Cost, Credits or Tokens) and what it divides by (Commits, Pushes or PRs). Per push and per PR divide the period's whole total, since pushes and pull requests can't be split into the part AI did
  - Figures that left the panels moved to the cards below: lines in Git commits and AI commits changed again within 3 days to the Git column of Outputs, AI-added lines that reached a commit to its AI edits column, the other unit costs (per 100 AI-edited lines, per active hour and day, per session and per prompt) to a new Unit costs card, and the day's spend to stay at the last period's total to Usage
  - The section of charts under the panels is now called Breakdown
  - In Japanese, the Cost switch reads 費用 everywhere; credits stay "credits", as in Kiro
- The Unit costs card is one figure with two switches, like Unit cost trend: the measure (Cost, Credits or Tokens) and what it divides by (per 100 AI-edited lines, per active hour, per active day, per session or per prompt), with its formula, usual range, verdict line and "?". Only pairs with records are offered, and the view remembers both choices
- The Outputs card is a compact list in two columns (Git, AI edits): one row per figure with its value, a mark for the usual range, the change from the previous period where the figure had one, and "ƒx" and "?". Longer notes (how many sessions, which repositories, "lower is usually better" for AI commits changed again, how much of AI's lines was rewritten or dropped) moved into the row's "ƒx"
- Lines in Git commits is back in the Trend card's metric list
- Unit cost trend offers only the denominators the chosen measure has: Commits is no longer offered with Credits (instead of an empty panel), and the note under per PR no longer calls every measure "cost". With one agent picked, per push is not offered, since git doesn't record which agent's work a push sent; one line says so, and the Pushes figure says it counts every agent
- Where it flowed puts names and amounts outside the colored bands: projects sit in a gap beside their nodes that no band crosses, and labels that would collide are moved apart. On screens 600px wide or less the chart keeps its shape and lists the names and amounts under it
- One name per measure across the summary and the calendar: Estimated cost and Kiro credits in headings, names and sentences, Cost and Credits on switches and selectors. In Japanese, credits stay "credits" (as in Kiro), and Cost trend is 費用の推移. The switches in Cost trend, Where it went and Cumulative say Active time; Where it flowed, What changed from the last period and Spread per session say Session time, and their "ƒx" explains how it differs from Active time
- The three panel charts leave room between the axes and the first and last points, so the latest point, its % label and the usage limit marks no longer touch the axis labels, and a chart with six points or fewer names every period on its x axis (month view no longer drops the middle month)
- Breakdown cards always keep their own height instead of stretching to their neighbour's
- The summary's figures have a clearer size order: the value of each of the three trend panels (Cost trend, Output trend, Unit cost trend) is the largest number on the page, and the values in the Breakdown cards (Outputs rows, Unit costs and the other number boxes) share one smaller size
- A value far outside your usual range (the same 1.5×spread rule as "far above" and Trend's red dots) is shown in amber with a "↑ Far above usual" or "↓ Far below usual" mark, in the panels and the Breakdown cards. The amber is the same in both directions and doesn't mean good or bad; unit costs, which have their own verdict, don't get it

## v4.6.2 - 2026-11-23

### Changed

- Session details: the prompt flow takes the colors of a dark-mode messaging app: your prompts are green bubbles with dark text, the AI's replies dark gray bubbles with light text, on a dark neutral gray background. Labels, links and notes inside your bubbles turn dark to stay readable, and a test checks the contrast of every text and mark in the flow (at least 4.5:1 for text, 3:1 for small marks)
- After you copy a session review or daily report prompt, the message also says that kiroku ran nothing and sent nothing
- Summary: the last column of "Where it flowed" is named "Output" (出力 in Japanese) instead of "Outcome" (成果): it shows whether a session reached a commit or a pull request, not whether the work succeeded
- Summary: "Where it flowed" says what it counts more plainly: the last column is "Commit or PR", "None" or "Not recorded" (a session with only a pull request was shown as "Committed"), the takeaway no longer says the share comes only from agents that record commits (sessions of other agents with a commit in their project count too), and the note says a commit up to 10 minutes after a session's end counts
- Summary: the figure panels are named as trends: "Cost trend", "Output trend" (was "Outputs") and "Unit cost trend" (was "Unit cost calculation"); in Japanese コスト推移, アウトプット推移 and 単価推移. The Outputs card under Charts keeps its name
- The session review prompt gives the address of each pull request the session created, so the review can name it
- The daily report prompt asks for a cheap change with a clear signal (a prompt you keep retyping, a model heavier than the work) before general advice
- Summary: "Where it flowed" (the Sankey from agent to project to commit or PR) is now the first card under Charts, since it shows the period's state at a glance; the other cards keep their order
- Summary: the column names and long agent names of "Where it flowed" stay inside the chart instead of running over the takeaway above it and the card's edge

### Fixed

- Claude Code: when a session moved to another folder (for example with `/cd`) and the later file only added a compaction (such as an idle compaction), that file's subagents and compaction were lost. They are now counted once, in the session they belong to
- `kiroku doctor` no longer suggests `--amazonq-db` when it finds no history: the option is still accepted but reads nothing
- The short page `kiroku open` and `kiroku serve` show while they hand the key to the browser is dark like the view, so it no longer flashes white before the view opens
- With `KIRO_HOME` set, Kiro CLI (V2) and Kiro IDE / CLI (V3) sessions left in `~/.kiro` (from Kiro CLI V2, Kiro IDE, or from before `KIRO_HOME` was set) are read too, instead of being missed. A session in both places is counted once, from `KIRO_HOME`. `--kiro-home` still reads only the location it names
- The page `kiroku serve` shows while it reads your history picks its language the same way as the view (`?lang=`, then `--lang` or `KIROKU_LANG`, then the browser). Before, a language chosen with the button of an earlier version still won there, so the loading page could be in a different language from the view
- The pages `kiroku serve` shows to a browser without its key, and for an old address with the key in it, are dark like the view instead of white with default blue links, and follow `--lang` (or `KIROKU_LANG`) when it is set, before the browser's language
- The page that hands the key to the browser after `kiroku open` picks its language the same way as the view (`?lang=`, then `--lang` or `KIROKU_LANG`, then the browser). Before, it ignored `--lang` and a language chosen with the button of an earlier version still won there
- The session review prompt states its time zone in a sentence of its own, instead of in parentheses after "not simply write longer prompts"

## v4.6.1 - 2026-11-22

### Changed

- Summary: "Where it went" (the donut and bars of where the cost went) is back in the Charts section instead of in "Other charts"
- Summary: in "Where it went", the list sits beside the donut at full width, so project names are no longer cut ("web-...")

## v4.6.0 - 2026-11-22

### Added

- Summary: Unit cost calculation adds estimated cost, Kiro credits and tokens per active day (the days on which the sessions that record each measure ran), with a chart and the usual range like the other unit costs

### Changed

- The view is always dark, whatever the system setting; the theme button is removed. The page shown while `kiroku serve` reads history is dark too
- Summary: cards are packed into columns at their own height instead of being stretched to the tallest card in a row, so short cards (Trend, Cumulative, Busiest day) no longer hold large empty areas
- Summary: "Kinds of work" spans the Outputs card under both columns; "By agent" shows two lines per agent on narrow cards instead of needing a double-width card; "How you worked" comes first in its section; the cost-vs-outputs chart is drawn wider on wide screens instead of growing tall; Busiest day uses taller rows when few sessions ran
- Summary: header charts are drawn whenever there is at least one period with records, so Kiro credits no longer loses its chart in the month view while it has one in the week view
- Summary: metric names in the figures are bold and dark instead of small and grey
- Summary: the Cost panel's charts label each point with its change from the previous week or month (also in the point's tooltip)
- Summary: the Cost panel is named "Cost trend" and the Unit cost panel "Unit cost calculation"
- Summary: the Output panel is named "Outputs" (アウトプット in Japanese), as is the Outputs card
- Summary: in a period in progress, figures show the value so far and the end-of-period estimate at the same size (the estimate was a small note)
- Summary: the AI edits part of the Outputs card is left out when no edited lines are recorded (such as Kiro's `editCode`), instead of showing 0
- Summary: the charts below the figures keep Where it flowed, Spread per session, Outputs (with the kinds of work) and When you worked; the others (Trend, Cumulative, Where it went, What changed, Tokens by type, cost by session length, By agent, Most edited files, Cost vs outputs, How you worked, Usage, Busiest day) move into "Other charts", a box that starts closed and remembers whether you opened it. Most edited files is its own card
- Summary: no card or figure is wider than half the page on wide windows; an odd number of charted figures ends with three in a row (instead of one stretched across the row), and Outputs, Cost vs outputs and How you worked take one column like the other cards
- Summary: Outputs, Where it flowed, Spread per session and When you worked sit under one heading, "Charts", in two half-width columns (the Cost details, Output and unit cost and How you used AI headings held one or two cards each)

## v4.5.0 - 2026-11-22

### Added

- Summary: Output adds "AI commits changed again within 3 days" (how often a file AI committed was changed again soon after) and "AI-added lines that reached a commit" (how much of what AI wrote in a Claude Code session ended up in its commits), each with its chart and usual range
- Summary: the Outputs card sorts the period's Git commits into kinds of work (features, fixes, refactoring, tests, docs, maintenance, reverts) from their subjects, with lines and, for AI commits, the estimated cost of their sessions; it also counts commits that were reverted and how many of them AI made
- Summary: the Outputs card adds the median time from a session's start to its first AI commit and from a commit to the push that sent it, and the files changed most often in Git commits, split into AI commits and commits by hand
- Summary: "What changed from the last period", a waterfall from the previous period's total through each project's increase or decrease to this period's total, for estimated cost, Kiro credits, tokens or active time
- Summary: "Where it went" adds a donut of the shares next to its bars; new cards "Where it flowed" (a Sankey from agent to project to whether the session reached a commit or PR), "Spread per session" (a box plot of cost, time or prompts per session for each period) and "Busiest day" (a Gantt of that day's sessions, showing how many ran at once)

### Changed

- Calendar: each day puts its Git commits first, in green (week headings and month days), ahead of cost and time
- Summary: "Worth a look" moves to the top, right under the title
- Summary: unit-cost verdicts say "below" or "above typical" only when the value is at least a tenth outside the usual range, and suggest a next step only on the higher side or when the week's session habits differed
- Summary: header charts draw the cumulative total and usage-limit marks only for usage (cost, credits, tokens, time), percentage charts scale to their data, and chart text no longer grows on wide single-column layouts
- Summary: the Usage card no longer repeats the per-session and per-prompt figures shown in Unit cost
- Calendar: narrow day columns drop the time, then the cost, from the heading line instead of overflowing into the next day
- Summary: off-hours share and project switches are no longer colored good or bad; ranks in the lower half read "2nd lowest of 11 weeks"; session-length bars say "4 sessions" instead of "n=4"; Kiro credits say "Kiro's own count, not an estimate"; typical ranges write the unit once ("13.1–13.8 credits"); the By agent table gets a wide card so no column is hidden; the usual-range band is easier to see in dark mode; the kinds-of-work bars line up
- Commit and push details show the repository path with a "Copy the git command" button instead of the whole git command
- Summary: the value so far in a period in progress is labeled "So far" in the header charts
- Calendar: on narrow month cells (phones), times are shortened ("5h") instead of being cut off
- Search: file matches in sessions show the path from the project, as commit matches do
- Summary: Unit cost shows one figure per row with a switch for estimated cost, Kiro credits or tokens (remembered), instead of up to three near-identical charts per row; rows without that measure say so
- Summary: the vs-last-week line puts its label first ("vs last week +108"), so it does not read like the green and red line counts
- Summary: the lower Outputs card no longer repeats the commits, lines and pull requests shown in the Output panel above
- The "?" trends and Worth a look cover 13 weeks or months, the same span as the summary charts (they were 8)
- Search results can be narrowed to a project with chips (the 8 projects with the most matches)
- Calendar: in narrow day columns, the copy-prompt button sits under the date instead of over it
- Calendar: in narrow day columns the heading keeps the figure picked in Figures and drops the commit count first; on windows up to 1,200px every day puts its copy button under the date, so all headings line up
- Summary: the Unit cost switch says Cost / Credits / Tokens and wraps under the title on narrow screens; its rows share one heading style and drop the good/bad tint (the verdict carries it); verdicts say "below usual" / "above usual" like the rest of the summary; "pace" reads "On pace for"; kept lines add how much was rewritten or dropped, and rework says lower is usually better; the "So far" label stays readable over lines
- Summary charts, the Trend card and the "?" trends share one span of up to 13 weeks or months
- Summary: the Outputs card takes a full row and the cost-vs-outputs scatter its own row (it was stretched to the Outputs card's height); the most edited files list adds how many Git commits changed each file (by AI and by hand), replacing the separate Git list; the cache figure in Usage says it is a share of input tokens
- Summary charts use larger text and every other period name on narrow cards (phones), where text was about 7px
- Worth a look lists each session with its prompts, estimated cost and active time
- Commit details for an AI commit say whether its files were changed again within 3 days, and by which commit
- Search results can also be narrowed to an agent
- Summary charts start at the first period with records (at least 4 periods, up to 13), so short histories are not squeezed into the right edge; month axes add the year to the first month and to January
- Summary: large jumps from a small base read as the change and the base ("+72.8 credits (from 6.8 credits)") instead of "+1076%"; the separate month-end tile is gone (each figure has its own month-end estimate); figures without a chart take a full row in Cost; Output tiles drop the good/bad tint; the cache share is neutral
- On phones, the edited-files and kinds-of-work lists put the name on its own line, charts drop the axis title, and first period names no longer overlap the y axis
- "1 commit" instead of "1 commits" in the edited-files list
- Worth a look sessions also show their interruptions; the Trend card says "in your lowest quarter of past weeks" instead of "below the 25th pct", and points to Worth a look at the top; the scatter uses the full card width; Most edited files has column headings; search explains itself in one line
- Calendar: up to 1,200px the copy button sits at the right of the figures line, so day headings stay low (about 69px instead of 100px)
- Worth a look sessions lead with their corrections and interruptions; "Daily spend to stay at last week's total" says what the daily figure is for; usage-limit marks sit on their own period with the count above; all charts start at the period where the history starts, so charts in one panel line up; the "So far" label sits below its point when there is room; on phones the edit count follows the line counts

## v4.4.0 - 2026-11-22

### Added

- Summary: Unit cost adds estimated cost, Kiro credits and tokens per session and per prompt, and tokens per AI commit, per 100 AI-edited lines and per active hour, each with its chart and the change from last week or month
- Summary: each unit cost says how it went in plain words (more or less efficient, lighter or heavier than usual, or as usual), why (lowest or highest of recent weeks, several weeks in a row, or the usual range), and what to do next: what may have helped (shorter sessions, fewer prompts per session), or a likely cause that opens its dialog or the sessions that used the most

### Changed

- Summary: Cost, Output and Unit cost are stacked full width with two figures per row (one on narrower windows), instead of side by side with a long Unit cost column; a figure with a verdict no longer repeats the usual-range line

## v4.3.0 - 2026-11-22

### Added

- Summary: estimated cost and Kiro credits per active hour (and tokens per active hour in the Trend), next to the unit costs per AI commit and per 100 AI-edited lines. Every unit cost can be picked in the Trend
- Summary: a "Worth a look" line under the figures lists what crossed a threshold and opens its dialog
- Summary: an "ƒx" button next to each figure's and chart's name shows its formulas
- Calendar: a thin bar in each day of the month splits its active time by Color by (agent or project), following hidden legend items and search
- Summary: each header figure has a wide trend chart of the last 12 periods overlaid with the cumulative total (right axis) or, for rates, the running average, with axis ticks, axis titles, a legend and your usual range as a band, and a note with the change from the previous week or month (percent for amounts and rates, a difference for counts)
- Summary: the header charts mark usage-limit hits with a small red triangle on the x axis at the time of the hit (×n for several in one period; the tooltip lists the times and resets)
- Summary: in a month or week in progress, every header amount and count shows its month-end or week-end estimate at the current pace, and its chart draws the estimate with dashed lines for the value and the cumulative total

### Changed

- Summary figures at the top are cut to the main ones and laid out with Cost and Output on the left and Unit cost on the right (on wide windows). Each unit cost names what it divides ("Claude Code cost per AI commit") and always shows "numerator ÷ denominator". Per session and per prompt move to Usage, and files, repositories, days with commits and pushes to the Outputs card
- Summary: formulas are hidden until you press "ƒx" (v4.2.0 always showed them)
- Summary: amounts (cost, credits, tokens, time, prompts, commits) are no longer colored good or bad; efficiency and habit figures keep their colors. Takeaways use plain words ("the lowest of 11 weeks", "slowest 10%"), ranges round like their values, and too little history is said once instead of under every figure
- Summary Trend: rates are drawn as a line with points and totals as bars, percentages use a 0–100% axis, and the list of metrics is grouped by kind
- Summary: the session length chart shows the average per session in each bucket ("Cost per session, by length"), with the number of sessions under each bucket
- Summary: the chart cards sit three to a row on wide windows and are drawn at the card's size, and Outputs takes twice the width so Git and AI edits sit side by side
- Japanese: Kiro credits are written "credits" instead of katakana, as in Kiro itself
- Calendar: each day shows its active time next to its cost figure ("$2.86 · 5h 9m · 3 commits"; cost over time in the month)
- Calendar: pressing a day in the month opens its week at that day and highlights it; closing session details after moving with ← → returns to the last session read; "This week / This month" (T) resets both the week and the month; ← → keep working after pressing the view toggles with the mouse; ‹ stops at the first period with records; an empty month says so
- Calendar: week totals in the month count only that month's days, so they add up to the month (the whole week is in the tooltip)
- Calendar: tall, narrow session bars keep their name instead of switching to the start time; days with only commits no longer show dashes; tooltips no longer repeat "credits"
- Lines added are green and lines removed are red wherever "+added −removed" appears (summary, session details, commits, breakdowns), as in git diff

### Removed

- Summary: Kiro credits per Git commit (added in v4.2.0). Kiro does not record which commits it ran, so it divided Kiro's credits by commits made by hand or by other agents
- Summary: cost per active day, replaced by cost per active hour

## v4.2.0 - 2026-11-21

### Added

- Session details: an Output section for every agent, with Git (commits during the session, with the share by AI where the agent records it, and pull requests) and AI edits (files and lines edited, with Kiro credits or estimated cost per 100 AI-edited lines). Kiro sessions used to show no output there
- Summary: estimated cost, Kiro credits and tokens per 100 AI-edited lines, in the Unit cost row and the Trend. They divide by the lines the agents edited according to history, so unlike Kiro credits per Git commit they don't depend on commits you made by hand
- Summary: estimated cost, Kiro credits and tokens per session in How you used AI, each divided by the sessions that record it
- Summary Output · Git: lines and files in Git commits, repositories and days with commits, pushes and pull requests, next to Git commits; AI edits also shows files edited by AI

### Changed

- Outputs are kept in two parts everywhere, never added together: **Git** (commits, lines in Git commits and pull requests, from your repositories) and **AI edits** (files and lines the agents changed with their edit tools, from history, committed or not). The summary's Output and Unit cost rows, the Outputs card and session details show them under their own headings and marks. Renamed: "Lines changed" is "Lines in Git commits", "Files/Lines changed by AI" are "Files/Lines edited by AI", "Most changed files" is "Most edited files", and the unit costs per commit are per Git commit
- Summary: formulas are always shown, and "Change first" and the formula switch are gone. Every metric that is worth a look has its mark next to a figure (Possible friction and Repeated prompts are now figures in How you worked)
- Summary: figures are tinted green or red by which side of your usual range they fall on, stronger when far outside it, with a line between the figures in a card
- Summary layout: the figure cards and the chart cards sit three to a row on wide windows (two, then one, as the window narrows), and a card left over at the end of a row widens to fill it. Output and Unit cost share one heading, and By agent and Outputs no longer take a full row
- A value whose label already names its unit (Kiro credits, Month-end credits, credits per Git commit and so on) no longer repeats "credits", in the band, the summary and the breakdowns
- Most edited files: the file name opens the file (as of the session's commits, or on this computer), and the number of edits opens the session that changed it most
- Calendar: today has a "Today" badge and an outlined column in the week calendar, and an outlined day in the month calendar

## v4.1.1 - 2026-11-21

### Fixed

- Claude Code and Codex: a subagent that keeps working after the parent conversation's last line (such as a background subagent) now counts in the parent session's calendar bar and active time, as Kiro Crew subagents already did; before, that part of its work was left out

## v4.1.0 - 2026-11-21

### Added

- Summary: the Outputs card shows how many files the agents changed with their edit tools in the period, the lines added and removed (an estimate from each edit's before and after), and the files changed most (「変更の多いファイル」); a file opens the session that changed it most. Lines are now counted for Codex (`apply_patch`) and Kiro (`fs_write`, `fsWrite`, `strReplace`) edits as well as Claude Code's
- Summary: the Trend can show Kiro credits per commit (credits ÷ Git commits other than the ones Claude Code ran) and tokens per commit (Claude Code tokens ÷ AI commits)
- Summary: the Trend can show Kiro credits and tokens, with the usual range and the pace of the period in progress like the other metrics
- While you look at the current week, the band and the summary's Kiro credits box show where this month's credits are heading (Month-end credits); pressing it opens the month's breakdown without leaving the week

### Changed

- Kiro credits are written as "credits" (「クレジット」 in Japanese) everywhere instead of "cr"
- Summary: the running total chart can show estimated cost, Kiro credits, tokens or active time, and is titled "Cumulative, vs last week / month" (「積み上がり（前週比）」「（前月比）」)
- Summary: the session length chart uses credits or tokens when there is no cost, instead of a count of sessions, and no longer prints n= under each bar (it is in the tooltip)
- Month calendar: the shade of each day follows the same usage as its figure (see Figures below) instead of always active time
- Session review prompt: when the history file can be read, it passes the session ID and the file and lists only what the history doesn't have (commits, pull requests and other events with their times), instead of copying the prompt flow
- Week calendar: sessions are shown with a colored edge on the left and a light fill instead of an outline and a dot
- Week and month calendars: Figures in the legend row picks the usage written for each day (estimated cost, Kiro credits, tokens or active time); the month calendar's shade follows the same choice
- Session details: moving to the previous or next session (‹ ›, `←` `→`) opens it with its prompt flow shown from the beginning; on narrow windows, where the flow comes after the key numbers, and in full screen, the flow's heading is at the top
- Summary: Where it went switches among estimated cost, Kiro credits, tokens and active time, and the Unit cost panel adds Kiro credits and tokens per prompt next to estimated cost per prompt
- Summary: the outputs card is titled "Outputs" (「出力」 in Japanese, instead of 「成果」), and the Japanese text says 出力 for outputs elsewhere
- Summary: the cost and outputs scatter can put Kiro credits or tokens on its horizontal axis instead of estimated cost, so agents without a cost estimate get it too
- Session details: the history file, models used and tools used are shown at the end of the details instead of being folded under "More details"
- Copying a review or report prompt only says it was copied and to paste it into your AI agent
- Summary view: the "Color by" and active time row of the calendar's legend is hidden; the summary has its own Agent switch and doesn't use the legend's hiding or the search
- Summary: the figures at the top are split into Cost (estimated cost, credits, the pace against the previous period, active time), Output (Git commits, lines changed by AI) and Unit cost on plain panels, without colored edges or filled boxes; a small mark under a figure says where it falls against your usual range, colored only when outside it, and what to change first is one line below them
- Switches (Week | Month, Calendar | Summary, Agent, the Trend and running total metrics) are a light track where the chosen item is raised, and buttons and drop-down lists share one plain style, instead of purple outlines and dividers
- Summary: the Trend's metrics switch becomes a drop-down list whenever it doesn't fit in one row, instead of wrapping; in the month view, Month-end (estimate) shows cost and credits side by side in one box
- Summary: figures and cards are grouped under Cost, Output and Unit cost (「単価」: cost ÷ output, such as cost, credits and tokens per commit and cost per prompt), with larger, bolder headings; How you used AI keeps its own heading, and cost per commit moves from the Outputs card to Unit cost

### Fixed

- Switching between the calendar and the summary no longer moves the period heading and the switch up and down on wide windows; the band keeps its space while hidden in the summary

## v4.0.0 - 2026-11-21

### Added

- Session details: ‹ › at the top right (or `←` `→`) open the previous or next session, `↑` `↓`, `PageUp` `PageDown`, `Home` `End` and `Space` scroll the details, and **Full screen** next to the prompt flow (or `F`) shows only the flow across the whole window

### Changed

- The weekly and monthly data no longer carry each project's top models, longest sessions and heaviest session, which the view doesn't show; with the per-agent summaries this keeps a large multi-agent history's HTML at about the size it had before them (22.3 MB instead of 31.1 MB for 9,000 sessions across three agents)
- README, the guide and the "?" help for Worth a look no longer describe "Analyze with AI" or the hidden Worth a look list as current; the Trend's note points to "Change first" and the warning marks instead of the removed Advice card
- Week calendar: each day's "Copy prompt" is always faintly visible with a mouse (not only when pointing at the day); short sessions and git marks have a larger area to press; a session running through a whole day shows that day as ending at 24:00 instead of "00:00–00:00"
- Copying a review or report prompt says that kiroku ran and sent nothing
- Summary: "Change first" uses the same name as the dialog it opens and says how many items are worth a look; the dialog moves between them with ‹ and ›, and shows "What to try" right under the heading
- Summary: "Show formulas" sits next to the Agent switch, reads "Hide formulas" while on and is filled while pressed
- Summary: the band above the calendar is hidden in the summary view, where the same figures are in the first row with their comparisons
- Summary: the Trend reads "At this pace $28.48: 3rd highest of 12 weeks"; the total in the cumulative chart matches the band's; "1 session" instead of "1 sessions"
- The warning marks for items worth a look are amber, so they don't read as the accent purple used for selection and your prompts
- Month calendar: stronger shades for active time, and days still to come are hatched instead of looking like quiet days
- Session details: each bubble's time and copy button sit right under it, on the same side, at every width
- The legend's chips wrap at 1200px and below instead of scrolling out of sight; "Over last week's total" replaces "Past last week's total"; "press to hide" replaces "click to hide"; the page footer has "Keyboard shortcuts"; "Go to the summary" is the first stop after the period controls
- The light theme uses plain grays for backgrounds and surfaces instead of Material's tinted ones; the loading page and the page that opens kiroku with its key match
- The band above the calendar no longer shows the change from the previous period next to each figure, and all its figures share one style; press a figure to see the change next to the value in its breakdown
- Summary: the Advice card is gone ("Change first" and the warning marks open the same details), and the chart cards are in two columns, with By agent and any card left alone in a row across the full width
- Summary: on narrow windows the Trend's metric is a drop-down list; before, the metrics past the edge of the scrolling switch could not be reached
- The ⚠ "Worth a look" list in the period row is hidden for now; "Change first" and the warning marks in the summary open the same dialogs
- Summary: the Trend's usual range and median are gray instead of green (green now only marks a figure better than usual)
- Summary: comparisons with your past periods read in plain words ("$31.23: above your usual $19.92–$27.35"); "Show formulas" next to the heading shows the inequalities and the formulas behind computed figures, and the choice is remembered
- Summary: the figures at the top wrap into three or two columns on narrower windows instead of scrolling sideways; on a phone the By agent table lists each agent's figures as label and value, and the Agent switch wraps
- Summary: "Change first" and the Worth a look dialogs use one order: items outside your usual range on the worse side first, then items that can't be compared, then items within the range, each by the cost involved. "Change first" is red only when its metric is worse than usual
- Summary: with one agent selected, a note says that its active time counts time shared with other agents in full (the By agent table and the legend split it) and that the figures above the calendar stay for all agents. The By agent shares use the same active time as the band
- Summary: advice about repeated prompts and long background names each agent's project instructions (CLAUDE.md, AGENTS.md or Kiro steering); without cost records the cumulative chart and the length chart are titled by what they show (active time, sessions); the outputs headline says the share is of Claude Code sessions
- Summary: the heatmap's headline says its total, and a note explains why it can be a little shorter than active time
- Week calendar: 0:00–7:00 starts unfolded in a week with sessions in those hours, so that work isn't squeezed; the toggle reads "0–7h"
- Week calendar: the usage limit mark is a small red square that no longer covers other sessions' names; pressing it opens the session at the limit hit in its prompt flow
- Session details: a note under Session review says that "Copy prompt" copies a prompt for your agent and kiroku runs and sends nothing; a turn where the AI worked without a text reply shows its time on the AI side instead of inside your bubble; copy buttons are faintly visible before you point at them; on a phone, bubbles use the full width with the time below them
- The theme follows your system's light or dark setting until you pick one with the moon / sun button; the loading page follows the same choice
- The band's daily target says how far the other figure is over ("· 2.1 cr over" instead of "credits over"), and with one day left the advice says "at most $0.59 tomorrow"
- While filtering, the session count reads "12 of the week's 20 sessions"
- Demo and period-only files: review prompts give times in the same time zone as the view

### Removed

- BREAKING: "Analyze with AI" and running AI agents from `kiroku serve`. kiroku no longer runs an AI agent or calls an AI service: `/ai/run` is gone (404), and the session review and each day's report only offer "Copy prompt", which copies the prompt to paste into your agent. `--ai`, `--no-ai`, `KIROKU_AI` and `KIROKU_NO_AI` are still accepted and do nothing; `--ai` (or `KIROKU_AI`) prints one line saying so. `kiroku autostart on` no longer writes `KIROKU_AI` or `KIROKU_NO_AI`

### Fixed

- Summary: switching the Agent from one agent to another could keep the first agent's repeated prompts in "Change first" and the advice; each agent's summary is now computed from its own sessions

### Security

- kiroku has no path that runs an AI agent any more, so text planted in your history can no longer steer an agent started from the view (`/ai/run` answers 404 even with the key; tested in the serve e2e and `TestNoAIExecution`)

## v3.3.1 - 2026-11-21

### Changed

- Session details: the prompt flow uses kiroku's own colors instead of a messaging app's: your prompts in purple bubbles on the right, the AI's replies in white (dark gray in the dark theme) bubbles on the left, on a light surface, with events as gray labels and limits and interruptions as red ones

## v3.3.0 - 2026-11-21

### Added

- A dark theme again, with a moon / sun button at the top right. The view stays light by default, whatever your system's setting; the choice is saved in the browser, and the page shown while `kiroku serve` reads your history follows it
- Summary: a "Tokens by type" card shows input, output, cache write and cache read as one stacked bar and as rows with the count and share, with the share read from cache against your past periods
- Summary: a "By agent" table (when more than one agent ran) shows each agent's active time and share, sessions, prompts, estimated cost, Kiro credits and tokens side by side
- Summary: an Agent switch (when more than one agent ran in the period) narrows the whole summary, including comparisons with past periods, to one agent. Git commits made by hand are left out of an agent's figures. The weekly and monthly data (`weeks` and `months` in `/data.json`) gain `agents` (the same figures per agent) and `only` (the agent, when just one ran)

### Changed

- Week calendar: 0:00–7:00 is folded at the top, at a quarter of the height as a shaded band, so the working hours fit the first screen and night work still shows where it happened. "0–7" at the top of the hours unfolds it to full height and folds it again; the choice is remembered
- Session details: the prompt flow looks like a messaging app: a blue-gray background, your prompts as green bubbles on the right, the AI's replies as white bubbles on the left, and events as dark labels in the middle (red for limits and interruptions)
- Summary: the figures at the top are in one row; on narrower windows the row scrolls sideways
- Summary: advice is ordered by the cost involved (the cost of the sessions each item is about, shown with its sum), largest first
- Summary: the weekday × hour heatmap ends each row with that weekday's total, so its values show without pointing at a cell
- Summary: a row of cards or stats that isn't full is filled by its last item, with no empty slot; on a phone, the trend's metric switch scrolls sideways instead of wrapping

## v3.2.0 - 2026-11-21

### Added

- The weekly and monthly summary is rebuilt as one page of cards, computed in the browser without AI, in the order you'd use them to cut cost, credits and tokens:
  - Figures (the band's figures and more, up to three per row): estimated cost, Kiro credits (or tokens), where you stand against the previous period (a day to match it, how far over it you are, the month-end estimate, or cost per active day for a finished period), active time, cost per commit, and what to change first. Each shows its formula, its change from the previous period, and how it compares with your own past periods
  - Trend of a metric you pick, with the middle half of your past periods as a band and the median; cumulative cost against the previous period; advice (up to three flagged items with what to try and their sessions)
  - Where it went (cost, credits, tokens or time by project, agent or model, largest first) and cost by session length
  - Cost against outputs per session (a scatter that marks costly sessions with no commit or PR), outputs, how you used AI, when you worked (a weekday × hour heatmap) and how you worked
- Comparisons with past periods are written as inequalities against the 25th and 75th percentiles of up to your last 12 periods, such as "$31.23 > 75th pct = $27.35", with a one-line definition; computed figures show their formula; every chart opens with a takeaway computed from the data and, where the data supports one, a next step with numbers. A period in progress is compared at its pace

### Changed

- Where kiroku can't run an AI agent (without `--ai`, with `--no-ai`, when no agent command is found, or in a saved HTML file), the session review and each day's report offer "Copy prompt" instead of "Analyze with AI (off)": it copies the same prompt, to paste into an AI agent yourself. kiroku runs nothing and sends nothing
- Session details: the two paragraphs under the prompt flow are gone. What "AI worked" and "you replied after" measure, and what the AI's reply is, now show when you point at them (and are in the guide)
- The view has a light theme only: it no longer switches to dark when your system is set to dark. The loading page and the page that opens kiroku with its key are light too

### Removed

- Summary: the four folded panels (By project, Cost and outputs, How you worked, How you used AI) and the project cards. Their figures are in the new cards; the lists of sessions with possible friction, repeated prompts and the heaviest sessions are reached from the advice, "Worth a look" and the scatter

### Fixed

- Week calendar: the hour next to the current-time marker (such as 12:00 at 11:50) is no longer hidden; it moves just past the marker in a lighter color
- A project named `__proto__` (or `constructor`, `toString` and similar) now gets its color like any other project; before, its bars in the summary had none

## v3.1.0 - 2026-11-21

### Added

- The session's prompt flow has a button that copies the whole conversation, prompts and replies in order, as Markdown, to hand the work to another agent or paste its history into a PR or a note. In `kiroku serve`, long prompts and replies are copied in full; a saved HTML file copies the part it contains and says so
- Each prompt and each AI reply in session details has a copy button, to reuse it. In `kiroku serve`, a long prompt or reply is copied in full; a saved HTML file copies the part it contains and says so

### Changed

- The view now follows Material 3: its standard color scheme (light and dark), pill-shaped buttons, segmented buttons, a pill search bar, a tonal card for the key numbers, larger corner radii on cards and dialogs, and Roboto when it is installed (no font is downloaded). Category colors (projects, agents, models) are unchanged. Buttons use a slightly denser size than the Material default, so the period controls and key numbers fit on one line down to a narrower window than before (week view from 1360px in English, 1340px in Japanese)
- Shorter animations: calendar blocks appear within about 0.4 seconds when you change the week or view (was up to 0.85), and the AI dialog folds its inputs faster

### Fixed

- The view follows your browser's language and your system's theme again even if you picked a language or theme with the buttons of an earlier version: that saved choice could no longer be changed, so a Japanese browser could stay in English. `--lang` (or `KIROKU_LANG`) and `?lang=` still pick the language
- On phones, the fade at the calendar's right edge no longer covers the search button in the header

## v3.0.1 - 2026-11-20

### Changed

- The change from the previous period is clearer:
  - With both estimated cost and credits in one figure, each gets its own percent change, in the same order as the values
  - When your history starts inside the previous period, figures and the summary show no comparison instead of counting the missing days as zero

## v3.0.0 - 2026-11-20

### Changed

- A simpler view, continued:
  - The theme follows your system's light or dark setting, and the language follows your browser or `--lang`; the theme and language buttons are gone (a theme or language chosen earlier is kept)
  - Week calendar day headings show usage and commits on one line ("$4.29 · 4")
  - "Color by" offers Project and Agent (Branch is gone)
  - The prompt flow has no "Everything / Your prompts only" switch and no legend
  - The summary has four panels: "How you spent time" and "Shape of the week / month" are one panel, "How you worked"
  - "Worth a look" flags fewer things: usage limits, long conversations, expensive models for light work, cost with no commit or pull request, corrections or friction, and repeated prompts. Cost growth, cost per commit, cache, sessions that reached a commit, switches, focus blocks, wait time and oversized prompts stay in the summary but are no longer flagged
  - The month calendar shows one figure per day (usage) with active time as the shade; commits are in the day's tooltip, and the project bars are gone
  - Agent-specific metrics (each agent's own counts, such as turns or response time) are no longer shown in the summary or session details: their definitions differ by agent, so they could not be compared
  - "Worth a look" is an icon with a count, and "This week", the period and view switches and that icon share one height and center line
  - The week calendar shades weekday hours outside 9:00–18:00, so early-morning and late-night work stands out
  - Session blocks and usage-limit marks in the week calendar have square corners
  - Usage-limit marks sit at the left edge of their own session's block, and their name says which session; late commit and push marks stay inside the day
  - Worth a look names a long-conversation flag "Long conversations" (not "Heaviest sessions"), and its list closes once you pick an item
  - With no agent installed, the reason ("No AI agent command (claude, codex or kiro-cli) was found…") is shown as text, not only on hover
  - Day headings leave out the commit mark on days without commits, and credit units stay next to their numbers on phones
- A simpler view:
  - The header shows only the logo, a small dot for LIVE (green while "Analyze with AI" is on; press it for what it means) and search. The tagline and the keyboard button are gone (press `?` for the shortcuts)
  - "Worth a look" is one button in the row with the period controls; pressing it opens the list
  - The session count next to the legend shows only while you filter
  - Days still to come no longer show "To match": the same amount is in the figures above the calendar
  - "Data sources" is always folded; a ⚠ next to its name marks something that needs your attention
  - Session details fold the history file, models, tools and the agent's own metrics under "More details"
  - The summary no longer shows Files changed, Pushes, Parallel time and Total AI run time
  - The month-end estimate shows only the 3σ range (table and dotted lines), not 1σ and 2σ
- kiroku no longer checks for a newer release on its own when a command runs; run `kiroku update` (or `kiroku doctor`) to check. `KIROKU_NO_UPDATE_CHECK` is no longer needed and is ignored

### Removed

- kiroku no longer reads Kiro IDE before v1.0 ("Kiro IDE (legacy)", the `kiro.kiroagent` folder) or Amazon Q Developer CLI. Their sessions leave the view, and the totals of the weeks that had them go down. Kiro IDE v1.0 and later, Kiro CLI (V3, V2 and the older SQLite history) and Kiro Crew are read as before
  - `--amazonq-db` and `amazonq` in `--sources` are still accepted and do nothing; using them prints one notice and the exit status is unchanged
  - Copies `kiroku archive` kept are not touched
- "Report a problem" and "Copy diagnostics" are gone from the view's footer; to report a problem, run `kiroku doctor --report` and paste its output (kinds and counts only) into a new issue
- BREAKING: `kiroku json` and `kiroku stats` are removed. Use `kiroku serve` to see your history, or `kiroku html` (with `--week` or `--month` for one period) for a file. Running either command now stops with a message naming what to use instead

### Security

- `kiroku serve` now refuses `POST` requests without an `Origin` header on `/ai/run`, `/archive` and the one-time open-code exchange, as SECURITY.md says; before, a request with the key, `X-Kiroku: 1` and no `Origin` was accepted (browsers always send `Origin` on these requests, so the view is unaffected)

## v2.0.0 - 2026-11-20

### Changed

- The period row is shorter: the "Period" and "View" labels are only read out by screen readers
- There is space between the week calendar and Data sources below it

### Removed

- The weekly report's "Analyze with AI" is gone; "Analyze with AI" remains for a day (next to each date in the week calendar) and for a session (in session details). With no agent installed, session details say why

### Security

- BREAKING: "Analyze with AI" in `kiroku serve` is off by default; start `kiroku serve --ai` (or set `KIROKU_AI=1`) to use it. An agent that reads your history reads text other people wrote (web pages, tool output, other repositories), which could try to steer it (prompt injection); kiroku now runs no agent unless you ask for it. `--no-ai` and `KIROKU_NO_AI` still keep it off, even with `--ai`. With `kiroku autostart`, run `KIROKU_AI=1 kiroku autostart on` to keep it on

## v1.1.3 - 2026-11-20

### Changed

- The figures above the calendar always stay in one row except on phones: on narrower windows their numbers are set smaller instead of wrapping to a second row
- The view's wording and figures, after a UX review:
  - On a past week or month, the summary says "in the week shown" instead of "this week"
  - In Japanese, the summary heading reads 週のサマリー／月のサマリー, matching the View button, and the metric heading reads 指標
  - Kiro credits under 10 show one decimal (1.3 cr, not 1 cr)
  - When only one of cost and credits is already past last week's (or month's) total, "A day to match last week" shows the target for the other and a short "credits over" note, instead of mixing a per-day target with a total overage; the tooltip gives the overage
  - Ranges read "1 – 3" in English and 1〜3 in Japanese everywhere (breakdown table, dotted lines, tooltips), and the week breakdown's line is labelled "Pace to match the last period"
  - Kiro CLI (V2), Kiro CLI (SQLite) and Amazon Q are teal instead of green, so they no longer look like Codex next to it
- On phones, the week calendar's day headings stick under the page header while you scroll and follow its sideways scroll
- The LIVE badge is a button: clicking or tapping it shows what it means (also with a keyboard or touch screen); the search box has a name for screen readers
- Errors from "Analyze with AI" that the server reports after starting are shown in Japanese too
- More view improvements:
  - The figures above the calendar show the change from the previous week or month in percent next to each label (compared with the same days while the period is in progress), without a good or bad color
  - In the week calendar, a session's title stops above a session that starts later on top of it, instead of showing cut-off letters beside it
  - The day heading's "Analyze with AI" appears as a small ✦ button when you point at the heading, so it no longer covers the weekday
  - On phones, the search box is a magnifier that widens when you press it, instead of a box cut to "Sear"
  - Data sources and the footer (version, Report a problem) are shown in the calendar view too
  - In the prompt flow, "AI worked … · you replied after …" sits under the AI's reply it belongs to
  - The weekly report's and the session review's AI dialogs name the week or session in their title
  - Opening this week scrolls the page to the hour the week's work starts when it would be below the first screen (not in the demo)
- The figures above the calendar show only what you used: commits, lines changed and pull requests are in each calendar day and in the summary's ②
- The week view's heading shows the week as "Week 46 of 53" (「第46週 / 全53週」), counting ISO weeks in the year the week belongs to

## v1.1.2 - 2026-11-20

### Changed

- Fewer figures above the calendar: files changed and pushes moved to the summary's ② only, and in the month view the month-end estimate replaces "A day to match last month", so a month shows six figures instead of nine
- Nothing is fixed on screen anymore: only the page scrolls, the top scrolls away, and the week calendar shows the whole day at full height without a scroll of its own. On a wide window its day headings stick under the page header while you scroll

## v1.1.1 - 2026-11-20

### Changed

- The figures above the calendar stay in one row on a wide window also in the month view, where they are set a little smaller, and the color legend is one row that scrolls sideways when it doesn't fit, with smaller chips, so the calendar gets more room

## v1.1.0 - 2026-11-20

### Added

- Month-end estimates (cost, Kiro credits, tokens) come with a range at 1σ, 2σ and 3σ, from how much the finished days of the month varied. The breakdown shows all three; the figure's tooltip and the summary show the 3σ range, and the by-day chart draws the range for the days still to come as dotted lines
- The by-day chart of a month-end estimate draws a trend line fitted to the finished days and extended to the end of the month, and the breakdown says where the month ends if that trend continues
- When a command runs in a terminal, kiroku checks at most once a day whether a newer release exists and, after the command, says so and asks "Update now? [y/N]"; `y` updates the same way as `kiroku update`. `kiroku serve` prints one line instead. Nothing is sent about your history, and nothing is asked outside a terminal, in CI or for dev builds; set `KIROKU_NO_UPDATE_CHECK=1` to turn it off

### Changed

- The page shows either the calendar or the summary, switched with View "Calendar | Summary" next to Week | Month (or `V`). On a wide and tall enough window the page no longer scrolls: the top stays put, the calendar fills the rest of the window with a clear edge, and only the calendar scrolls. On shorter or zoomed windows and while searching, only the page scrolls and the calendar shows at full height, so scrolling no longer gets caught between the page and the calendar. The weekly report's "Analyze with AI" moved into the period controls' row
- The page has more room: wider margins, more space between the period, figures, legend, calendar and summary, slightly larger corners and line spacing. The per-day "Analyze with AI" buttons in the week calendar appear when you point at or move into a day's heading (always on touch screens), so seven buttons no longer crowd the headings

### Security

- The new-version check reads its cache only when it is a small regular file (not a link, FIFO or device), writes it through a fresh 0600 temporary file instead of a fixed name, treats only real terminals (not `/dev/null`) as interactive, and discards keys typed while the command ran before asking, so an earlier keypress can't answer the update prompt

## v1.0.5 - 2026-11-20

### Changed

- The eight colors now share one saturation, so no color (such as Kiro's violet or the green) stands out more than the others; they are told apart by lightness. AI commit marks on the calendar are a light tint instead of a solid fill

## v1.0.4 - 2026-11-20

### Changed

- Colors follow one fixed, checked palette: the eight colors for projects, agents and models now share the same lightness and a restrained saturation, so they don't stand out too strongly against the background, and keep neighbors apart, also for color-blind readers, in light and dark themes. Tints (bubbles, labels, backgrounds) use three fixed strengths per theme. Kiro Crew is now magenta and Kiro CLI (V2) green
- The page as a whole is calmer: the background and text are no longer pure white and near-black, and borders, the accent, warnings, weekend tints, shadows and the "Analyze with AI" button are toned down, while text keeps a contrast of at least 4.5:1
- Buttons are no longer solid fills: "Analyze with AI", the resume command's "Copy", today's date and the summary's panel numbers use a light tint with colored text, and the "Analyze with AI" buttons on each day are plain until you point at them

## v1.0.3 - 2026-11-20

### Changed

- Session details: in the prompt flow, your prompts are now on the left next to a person mark and the AI's replies on the right next to a robot mark
- "Data sources" lists only the data kiroku read; "Report a problem" and "Copy diagnostics" are at the bottom of the page only
- In the weekly and monthly summary, ① By project and ② Cost and outputs are folded to their heading by default too, like the panels after them, so the calendar stays the main view

## v1.0.2 - 2026-11-20

### Changed

- Session details: the prompt flow reads like a chat. Your prompts are bubbles on the right, the AI's replies are bubbles on the left next to an "AI" mark, events between them (commits, pull requests, pushes, interruptions, subagents) are small labels in the middle, and a long break is a divider. Times sit at the bottom edge of each bubble
- Session details: the legend of marks above the prompt flow is folded under "Legend" and closed by default (it stays open once you open it), so the flow starts higher. The right column has headings above "Analyze with AI" ("Session review") and above the key numbers ("Metrics")
- The figures above the calendar show what was left behind on the right of what was used: Git commits (or AI commits), lines changed, files changed, pushes and pull requests, each opening its breakdown
- In the weekly and monthly summary, ③ How you spent time and the panels after it are folded to their heading by default, so the calendar stays the main view. kiroku remembers the ones you open, and a folded panel with a metric past its threshold shows the warning triangle on its heading

### Security

- "Analyze with AI" runs Claude Code with `--restricted` (or `--setting-sources project` on older versions), so your own Claude Code settings (allow rules, extra folders, `bypassPermissions`, hooks) can't widen what text planted in history could make it read
- The run is no longer kept as a new session (`--no-session-persistence` for Claude Code, `--ephemeral` for Codex), and kiroku leaves out sessions recorded in its temporary `kiroku-ai-` folder, so a run's copy of the prompt and of the history it read doesn't show up as your session or feed later prompts
- Codex runs with web search and MCP servers turned off
- The agent no longer inherits a surrounding Claude Code session's variables or tokens it doesn't need (names ending in `_TOKEN` or `_PAT`, such as `GH_TOKEN` or `NPM_TOKEN`; the AI providers' own stay)
- The reply check also catches links in capitals, without `https:`, with character references or non-ASCII hosts, and warns about images, video, embeds and `data:` URIs, which a page you paste the result into could load
- The session review's "prompts in other projects" line leaves out the projects hidden in the legend, as the weekly report does

## v1.0.1 - 2026-11-20

### Changed

- The view shows the history Kiro IDE and Kiro CLI V3 share as "Kiro IDE / CLI (V3)", next to "Kiro CLI (V2)", so it is clear where Kiro CLI V3 sessions are. The `source` value in `kiroku json` stays `Kiro IDE / CLI`, as docs/compatibility.md promises
- Session details: on wide screens the left column is only the prompt flow, and the right column has what you can do (the resume command and "Analyze with AI"), then the key numbers (active time, prompts, wait time, cost, tokens), then the rest (history file, models, tools, the agent's own metrics, commits, pull requests, files, subagents). On narrow screens they come in that order with the prompt flow right after the key numbers, and `Tab` follows the same order. "Analyze with AI" moved from the prompt flow's heading to the actions

## v1.0.0 - 2026-11-20

### Added

- The view shows the kiroku version (such as `v1.0.0`) next to the logo in the header, for release builds

### Changed

- BREAKING: Kiro CLI V2 sessions (`~/.kiro/sessions/cli/`) are now shown as "Kiro CLI (V2)" instead of "Kiro CLI (Classic)", because Kiro now uses "Classic" for its retired V1 interface. The `source` value in `kiroku json` changes the same way, so scripts that match `"Kiro CLI (Classic)"` need updating (`schemaVersion` stays 1). `--sources kiro`, the options, the environment variables and the `kiroku archive` layout are unchanged
- STABLE: kiroku 1.0.0. What [docs/compatibility.md](https://github.com/MichinaoShimizu/kiroku/blob/main/docs/compatibility.md) lists as kept compatible (commands and their arguments, options, environment variables, exit status, the listed fields of `kiroku json`, the `--prices` file, `kiroku archive` copies, the `kiroku serve` key, local-only defaults and verified downloads) now changes only with a new major version

### Removed

- BREAKING: `kiroku serve` no longer accepts an address with `?key=<key>` (made by versions before v0.38.2), even when the key is right: it shows a page that says the address no longer works and how to open kiroku instead. Such an address stays in the browser's history with the key in it, and the key itself still works, so if you ever opened one, change the key: stop `kiroku serve`, delete `serve-key` in kiroku's settings folder, start it again and run `kiroku open`. Bookmarks of the plain address and browsers already opened with `kiroku open` keep working
- BREAKING: `kiroku serve --ai` is gone. "Analyze with AI" has been on by default since v0.36.1 and `--ai` only printed a notice (and `--ai=false` turned runs off); `--ai` and `--ai=false` now stop with a message that says to use `--no-ai` to turn it off

### Fixed

- On a phone-width screen, a long history location in "Data sources" now wraps instead of widening the page

## v0.40.0 - 2026-11-20

### Added

- Prices as of November 2026:
  - Claude Code: Claude Fable 5.2 (`claude-fable-5-2`) and Claude Mythos 5.2 (`claude-mythos-5-2`) at $8 input, $40 output, $10 / $16 cache writes (5 minutes / 1 hour) and $0.20 cache reads per million tokens, with a 1M-token context window for Peak context usage
  - Codex: GPT-6.2 Astra (`gpt-6.2-astra`, $8 / $40, cached input $0.40, cache writes $10; long context $16 / $60), GPT-6.2 Sol (`gpt-6.2-sol`, $2 / $10, cached input $0.10, cache writes $2.50; long context $4 / $15) and GPT-6.1 Terra (`gpt-6.1-terra`, $1 / $5, cached input $0.05, cache writes $1.25; long context $2 / $7.50), with their Fast rates (and Flex for GPT-6.1 Terra), and GPT-6 Cyber (`gpt-6-cyber`, $20 / $100, cached input $2, cache writes $25; long context $40 / $150)
  - Codex: GPT-6.2 Astra and GPT-6.2 Sol on Ultrafast at OpenAI's list rate (6 times standard: $48 / $240 and $12 / $60). OpenAI bills 3 times standard through February 28, 2027, so until then their Ultrafast estimate is twice the bill
  - Codex: `gpt-5.5-cyber-preview`, the new name of `gpt-5.5-cyber`, at the same price; history that still names it `gpt-5.5-cyber` keeps that price

### Removed

- Claude Haiku 3.5 (`claude-3-5-haiku`) is no longer in the price table, because Anthropic's pricing page no longer lists it. Sessions that used it without a cost recorded by Claude Code are listed in "Data sources" as tokens not in the price table; add it with `--prices` (it was $0.80 input, $4 output, $1 / $1.60 cache writes and $0.08 cache reads per million tokens) to keep pricing them

### Fixed

- The summary's metric values now have names screen readers can read ("Active time 32h 55m" instead of just the number)
- "Repeated prompts" now says it lists the prompts written in the most sessions first, which is how it is sorted, and the AI result check says "All 5 sections are missing" instead of "5 of the 5 sections are missing"
- The "Usage limit hits" explanation in the view now names spend limits, and the Japanese view has a screen-reader label for AI replies
- `kiroku doctor`'s hint for history kept elsewhere now also names `--crew-home` and `--kiro-cli-db`, and `kiroku serve --help` says that `--ai=false` turns AI runs off
- Claude Code: a Claude apps gateway's pooled-budget error ("This request exceeds your group's budget … resets on November 1 at 00:00 UTC") now counts as a usage limit hit, and its reset date is shown as written. The current `spend limit reached · resets Dec 1` form also shows its reset date
- Claude Code: when two subagent calls overlap and their transcripts carry no `agentId`, each transcript is now linked to its call by start time rather than by file name. Before, a later transcript could take the earlier call, leaving one call without usage and its transcript as an extra subagent row
- Codex: a turn stopped by an API key, project or organization spend limit (`api_key_spend_limit_exceeded`, `project_spend_limit_exceeded`, `organization_spend_limit_exceeded`) now counts as a usage limit hit, as Codex itself counts it
- Codex: `kiroku doctor` no longer reports the `dynamic_tools_updated` record (written when an app-server host replaces dynamic tools) as an unknown kind
- Japanese view: only titles kiroku itself gives a session (marked `synthetic` in the data) are translated, so a session whose own title or prompt happens to read the same is shown as written, and "Analyze with AI" prompts keep these titles in English. The page shown to a browser without the key no longer breaks Japanese words or commands at odd places, and the summary's "How you used AI" spreads its cards across the row and keeps model names near their numbers
- Japanese view: errors from "Analyze with AI" (agent not installed, a run already going, prompt too long), the page shown to a browser without the key, the one-time `kiroku open` address page, the titles kiroku gives Kiro Crew and Kiro IDE sessions without one (such as "Kiro Crew private conversation"), and the private-conversation check in "Data sources" are now in Japanese
- Kiro IDE / CLI: the "Resume" command is now `kiro-cli chat --v3 --resume-id <id>`. Without `--v3`, a saved `chat.agentEngine v2` setting started V2, which can't open V3 sessions
- `kiroku serve`: when the key changes, the LIVE badge now says to open kiroku with its key instead of "Not connected", and an error while drawing new history is no longer reported as a lost connection
- `kiroku serve` with demo data in another time zone no longer reloads once and shows the times unshifted
- On a phone-width screen, a long project or branch name in the session details now wraps instead of widening the page
- Agent badges (CC, CX, KI, KW) are readable in the light theme (contrast 6:1 or more, from as low as 1.8:1) and in the dark theme
- In the dark theme, a disabled "Analyze with AI" button is no longer brighter than an enabled one, and the agent list and prompt box in the AI dialog follow the theme
- The summary no longer leaves a large blank beside "How you used AI": the panel spans the full width and lays out the agent-specific metrics side by side

### Security

- "Analyze with AI" with Claude Code can no longer read Kiro Crew private conversations (incognito and temporary): their conversation logs, their copies in Claude Code and Codex history, and Crew's own records of them (the subagents' `state.json` and the `crew-log` folders, which hold the tasks given to subagents). Those files are inside the history folders Claude Code may read, so a run could read text that kiroku keeps out of its own view and prompts. kiroku now passes a `Read` deny rule for each in a settings file (`--settings`); deny rules win over the folders it adds and over the user's own settings. A path the rule syntax can't hold exactly (with a parenthesis, backslash or line break) denies its nearest folder instead. Codex and Kiro CLI runs have no such rule (as before, kiroku sets only their read-only options)
- `kiroku archive off` now deletes only the copies kiroku writes (`.jsonl.zst`), and the number of files it offers to delete is the number it deletes. Before, it also deleted other `.zst` files in the archive folders and counted files it then left alone
- `kiroku archive on` and each read now make an archive folder and its per-agent folders (such as `claude/`) that already existed readable only by you. Folders made by older versions could stay readable by other users, who could list the project paths in their names
- The `git show` and `git log` commands kiroku shows for a commit or push no longer run commands that a repository someone else made sets in its own configuration when you paste them into a terminal. They now carry `--no-pager`, `-c core.fsmonitor=false`, `-c log.showSignature=false` (no `gpg.program` for signed commits), `-c diff.submodule=short` (no submodule `diff.external`) and `-c protocol.*.allow=never` (a partial clone doesn't fetch missing objects through the repository's `remote.*.uploadpack` or `core.sshCommand`), and `show` also carries `--no-ext-diff --no-textconv`. kiroku itself already ran git with these turned off
- Release builds now use Go 1.26.11 instead of Go 1.26.10, whose `net/http` server can crash from a header race in `MaxBytesReader` (GO-2026-6859; found with `govulncheck`). `kiroku serve` limits the size of "Analyze with AI" requests with `MaxBytesReader`, so a local page could stop the server
- A history file made of many prompts with the same timestamp no longer makes kiroku spend time growing with the square of the prompt count (about 50 seconds for a 20 MiB file, and longer for a small `.jsonl.zst` that expands to hundreds of MiB). Work and wait times per prompt are now found in one pass
- Amazon Q and Kiro CLI (SQLite): a named pipe or a folder where the database should be is now reported as unreadable instead of opened. A named pipe made kiroku wait forever, and a folder was silently read as no sessions
- The resume and `git -C <repo> show` commands kiroku shows for pasting into a terminal are no longer given for a folder, session ID or ref that contains `'` or `\` on macOS and Linux. They were quoted as `'…'\''…'`, which is safe in sh, bash and zsh, but fish and PowerShell read `\` and `'` inside single quotes differently, so a crafted value in history (a working folder, or a Kiro session ID) could add a command when pasted there
- A crash inside the zstd decoder while reading a compressed history file (`.zst`) is now reported as an unreadable file instead of stopping `kiroku serve`. kiroku decodes on the reading goroutine so it can recover, as hardening against decoder bugs such as GHSA-fpmh-frq8-7rj8

## v0.39.0 - 2026-11-05

### Added

- Kiro IDE / CLI sessions (Kiro CLI V3 and Kiro IDE v1.0 and later) now show a "Resume" command, `kiro-cli chat --resume-id <id>` in the session's workspace. Kiro doesn't record whether the IDE or the CLI created a session, so it is shown for both; Kiro CLI may not open a session that Kiro IDE created

## v0.38.3 - 2026-10-24

### Security

- Release builds now use Go 1.26.10 instead of Go 1.26.9, whose standard library still trusts compromised intermediate certificates in `crypto/x509` (GO-2026-6809; found with `govulncheck`). kiroku relies on certificate checks when `kiroku update` checks and downloads releases from GitHub
- On macOS, a one-time code from `kiroku open` could give the `kiroku serve` key to more than one request when they arrived at the same moment, because the file system can report the code's file as deleted to each of them. kiroku now checks and uses a code one request at a time

## v0.38.2 - 2026-10-19

### Fixed

- `kiroku doctor`'s "Format check" no longer reports record kinds that Codex 0.162 and later write as a matter of course (such as `world_state`, `retained_context`, `inter_agent_communication` and agent-to-agent `agent_message` items) as unknown
- Escape in the "Analyze with AI" dialog no longer also closes the session details behind it, and the arrow keys, T, W, M and ? no longer change the period behind the open dialog
- The "Not connected to kiroku serve" page says to reload the page after starting kiroku serve again, since it doesn't reconnect by itself

### Security

- `kiroku open` (and `kiroku serve` when it opens a browser) no longer puts the `kiroku serve` key in the address it opens. Browsers keep every address they open in their history, and sync it when history sync is on, so the key ended up there. kiroku now opens the view with a one-time code that works once, within 10 minutes, and the page it opens stores the key. `kiroku open --print` prints such a one-time address too
  - An address with `?key=` from an older version still works, but it is already in your browser's history with the key. Delete those entries, or change the key: stop `kiroku serve`, delete `serve-key` and start it again, then run `kiroku open` once in each browser
- A crafted `.jsonl.zst` history file (Codex) or `kiroku archive` copy could crash kiroku, including `kiroku serve`, instead of being reported as unreadable (GHSA-fpmh-frq8-7rj8 in github.com/klauspost/compress). Updated the module to v1.20.3

## v0.38.1 - 2026-10-16

### Changed

- "Analyze with AI" starts on the agent that did the work: for a session review, the agent of that session (a Codex session starts on Codex); for a daily or weekly report, the agent that ran longest that day or week. It falls back to the first agent found, as before, when that agent's command isn't on this computer. Reopening a dialog with an earlier result starts on the agent that wrote it
- The view opens in the dark theme, with the calendar colored by agent ("Color by: Agent") instead of by project. A theme or "Color by" you chose before is kept; the button at the top right still switches to light

## v0.38.0 - 2026-10-16

### Added

- "Analyze with AI" checks the reply of a report or a session review: missing sections (such as a reply cut off at the time limit), a Numbers table that differs from kiroku's, and links or commit hashes that aren't in the prompt
  - "Ask to fix" shows a prompt that asks the agent to fix those points, and sends it when you press Run (at most twice per dialog)
- The `kiroku serve` view can be installed as an app in Chrome and Edge ("Install kiroku" in the address bar or menu), so kiroku opens in a window of its own with its icon in the Dock or taskbar. It uses the key the browser already has, and installs no service worker, so the browser keeps no copy of your history (ADR 0037)

### Changed

- The daily and weekly report prompts are about 6–7% smaller: they no longer repeat the period's totals, a session's first prompt, or the flagged repeated prompts and long conversations that "Signals for advice" already lists, and their review principles are shorter
- "Signals for advice" says what a repeated prompt and a short session on an Opus-class model suggest (a skill, a custom command or project instructions; a lighter model), so reports and session reviews give that advice more often
- The live demo shows the "Analyze with AI" buttons (weekly and daily report, session review). They open the dialog with the exact prompt for the dummy data; Run is disabled there, and nothing is sent

### Removed

- The monthly report ("Analyze with AI" under "Monthly report" in month view). A month's history rarely fits in one agent run's 2-minute limit; use the weekly reports instead. The monthly summary and calendar are unchanged

### Fixed

- "Analyze with AI" with Claude Code, which kiroku lets read files but not run commands: the report and review prompts no longer leave it stuck on their `zstd -dc` and read-only `git show` steps. They now say to write from the digest, the facts or the commit subjects when the agent can only read files
- "Analyze with AI": in the formatted reply, a table right after a line of text is shown as a table (before, it was shown as part of the paragraph), and a link whose address has parentheses, such as `https://en.wikipedia.org/wiki/Go_(programming_language)`, shows its whole address instead of stopping at the first `)`
- Claude Code: when one subagent file matched an Agent call only by time and another file of the same session carried that call's `agentId`, the second replaced the first and one subagent (with its tokens and estimated cost) was not counted. Files are now linked by `agentId` first and by time after, so both are counted
- Claude Code: a subagent resumed by a second Agent call (same `agentId`, one file for both runs) was counted twice: once from its file and again from the second call's reported usage. It is now counted from the file only
- Claude Code: an Agent call line written twice no longer adds an empty second subagent
- Claude Code: linking many subagent files to their Agent calls by time no longer slows down with the square of their number (30000 of each in one session: 25 s before, about 1 s now)
- "N more sessions" in a Worth a look dialog is now a button that shows the rest of the sessions in place (before, it was only text)
- ↓ in the search box moves to the first result, like Enter, and ↑ and ↓ move between results and their "Show more" buttons (↑ on the first goes back to the search box). ← and → no longer change the week while the focus is on a search result. The shortcut list (`?`) shows ↑ ↓
- The "Copied" message is no longer squeezed into a narrow box on phones
- In Japanese at around 1000 px wide, 「今週」 no longer touches the 「期間」 label next to it
- Windows: stopping "Analyze with AI" (Stop, the time limit or closing the page) now also stops what the agent started, such as `node` behind the `.cmd` shim that npm installs for `claude` and `codex`. Before, only `cmd.exe` was stopped and the agent could keep running. Anything the agent leaves running is also stopped when the run ends

### Security

- `kiroku serve` answers `GET /manifest.webmanifest` without the key, like `GET /`. It is a fixed document built from constants (no history, settings or key), and the Host check and security headers still apply. Its icons are `data:` URLs, so the view's `img-src` stays `data: blob:`; only the live view allows `manifest-src 'self'`

## v0.37.0 - 2026-10-15

### Added

- "A day to match last week" (or last month) in the key figures of a week or month in progress: the previous period's total minus the total so far, divided by the days left, for estimated cost and Kiro credits side by side. Each day still to come in the week calendar shows the same amount as "To match" in a dashed box, and its breakdown puts the commits of both periods side by side. Once the period passes the previous total, the key figures say by how much. It needs no budget setting
- The daily report is back, with "Analyze with AI": in `kiroku serve` with an AI agent found, each date with history in the week calendar has an "Analyze with AI" button. It shows the exact prompt for that day (numbers next to the previous day, what was done in each project, expert observations and next actions) and runs it after you confirm, like the weekly and monthly report. A saved HTML file shows no such button

### Changed

- The calendar shows less, so it reads at a glance: each day (and each week total) lists only its usage and its Git commits, with cost right above commits so a day with much cost and few commits stands out. Usage is the estimated cost ("Est. cost"), tokens when no model used is in the price table, or credits when there are no tokens. Active time shows as the bars' length in the week calendar and the shading in the month calendar; active time, sessions, tokens and credits per day are in the tooltip. On phones, month cells show only the usage
- The key figures above the calendar are cut to five at most: usage (estimated cost and credits in one box), the daily pace to match the last period, the month-end estimate (cost and credits in one box), active time and Git commits. Tokens appear only when the estimated cost can't be shown. Sessions / prompts and Usage limit hits move out of the band; they still open their breakdowns from the summary. Active days moves into the Active time breakdown. When the figures don't fit, they continue on a second row instead of being cut off (before, Active days was cut off around 1000px wide and on phones)

## v0.36.1 - 2026-10-14

### Changed

- "Analyze with AI" is available in `kiroku serve` without `--ai`, for the weekly and monthly report and the session review (ADR 0034)
  - It offers the agents installed on this computer (`claude`, `codex`, `kiro-cli`). When none is installed, the button is shown disabled with the reason.
  - Each run still shows the exact prompt and waits for you to confirm.
  - The agent you choose may send the prompt and the history it reads to its provider. `kiroku serve` says so when it starts, and so do README, the guide and `SECURITY.md`; the run dialog no longer repeats it.
  - Agent runs stay off when `kiroku serve` listens on an address that isn't a loopback IP.
  - A saved HTML file never runs an agent and now shows no AI buttons.
- New `kiroku serve --no-ai` (or `KIROKU_NO_AI=1`, which `kiroku autostart on` keeps) turns agent runs off; `/ai/run` then answers 404
- `kiroku serve --ai` is still accepted but no longer needed: it does nothing except print a one-line notice. A non-loopback ADDR with `--ai` no longer fails; agent runs are just off there. An explicit `--ai=false` turns agent runs off, like `--no-ai`
- If you registered `kiroku autostart on` before this version, agent runs are on after you update. To keep them off, run `KIROKU_NO_AI=1 kiroku autostart on` once
- Pull requests now describe their user-visible changes in a file under `changes/` instead of editing `CHANGELOG.md`; the release merges them. For contributors only: the changelog itself reads the same

### Removed

- The Journey and Project views, and the View switch above the calendar: the view is the Week / Month calendar, with search, the legend's filters, session details, the summary and the Year in review as before. A browser that last showed Journey or Project opens the calendar (ADR 0033)
- "Copy knowledge prompt" on each project card under "① By project", and its prompt (knowledge discovery, ADR 0028)
- The "Report" button on each day of the week calendar and the daily report prompt. Weekly and monthly reports stay
- Every "Copy prompt" button: "Copy prompt" for the weekly and monthly report and "Copy review prompt" in session details. The report and review prompts are still built, for "Analyze with AI" only
- The metrics guide (the map button at the top right and the `G` key). The "?" next to each metric and the keyboard shortcut list (`?`) stay
- The calendar zoom ("Zoom" − + in the legend and the `+` `−` keys). The week calendar still fits the height of an hour to the week's working hours
- The Year in review: its button next to Week / Month, the `Y` shortcut and the share image ("Save as PNG", "Copy image"). The view is the week and month calendar with its summary; options saved for the Year in review are ignored

### Fixed

- `kiroku serve` reloads history written while it was reading history for the first time (which can take seconds); before, it waited for the next change

## v0.36.0 - 2026-10-14

### Added

- `kiroku doctor` has a "Format check" section: for Claude Code and Codex, the newest agent version seen in history, lines that couldn't be read and record kinds kiroku doesn't know, with counts. Unknown kinds can mean the agent changed its history format. "Data sources" in the view shows the same next to each agent, and `kiroku json` adds them to `meta.report[]` (`agentVersion`, `badLines`, `unknown`, `kindsChecked`)
- `kiroku doctor --report` prints a short block to paste into a GitHub issue: kiroku version, OS, and per agent the number of sessions, newest version seen, unreadable lines and unknown kinds with counts. It contains no paths, project names, IDs or content, and kiroku sends nothing; you review and share it yourself (ADR 0032)
- "Report a problem" in the view's footer and "Data sources" opens a new GitHub issue (new `bug.yml` form, replacing `bug_report.yml`) with only the kiroku version, OS, view language and current screen filled in; nothing from your history goes into the link. "Copy diagnostics" next to it copies the same text as `kiroku doctor --report` for you to check and paste. Both work in `kiroku serve` and in a saved HTML file. The issue form asks for the screen, agent and version, diagnostics, and a confirmation that the report contains no private information

### Security

- Compressed history (`.jsonl.zst`: Codex rollouts, `kiroku archive` copies, Kiro Crew archives) is decompressed only up to 1 GiB per file. A small file that expands much further (a 380 KB file expanding to 3 GB took 40 seconds to read) is now listed as unreadable under "Data sources", with what was read up to the limit kept, instead of stalling every `kiroku serve` reload; `kiroku serve` also cuts off the `/history` download of such a file

### Fixed

- Claude Code: when a session moved to another project folder left a second file (such as a `kiroku archive` copy from the old folder) with no lines of its own, the subagents found only next to that file were not counted; they are now counted once, with the session of the file kiroku keeps, and linked to the Agent call that started them

## v0.35.0 - 2026-10-14

### Changed

- The "Analyze with AI" dialog again says, next to the prompt you confirm, that the agent you choose may send the prompt and the history it reads to its provider. ADR 0030 records `kiroku serve --ai` as an opt-in exception to "history stays on the machine" (ADR 0022)
- The report and review prompts ask the agent to cite the figure or signal from "Signals for advice" that each next action or piece of advice rests on, treating signals as candidates, not verdicts. Reports include only the guidance for their own period (day, week or month) and, like the knowledge prompt, ask the agent not to run other commands, write files or send anything to another service. The review and knowledge prompts carry the reports' privacy rule (no secrets, personal data, command output, file contents or paths on your computer; excerpts only from your prompts and the AI's prose). The review prompt no longer repeats its role, and the note on the fenced history block says it holds kiroku's computed facts as well as quoted history
- The report bench (`tools/reportbench`, `/report-eval`) grades the current sections: Summary, Numbers, By project, Expert observations and Next actions for reports; Summary, Numbers, Professional prompt assessment, Prioritized advice and Reusable prompt playbook for a review

### Security

- BREAKING: `kiroku serve` no longer keeps its key in a browser cookie. Browsers send a cookie to every port of the same host, even with `SameSite=Strict`, so the `kiroku_key_<port>` cookie reached any other server on `localhost` (perhaps run by another user of the computer) when you typed its address, followed a link to it or the browser fetched its icon, and that server could then read your history through `kiroku serve`. The browser now keeps the key in its site data for that address and port (`localStorage`) and the view sends it in an `X-Kiroku-Key` header; cookies are no longer accepted, and an old key cookie is removed when a browser sends it. The plain address and bookmarks still work, but the page at `/` no longer contains your history: it reads it with the key, and shows how to open it with `kiroku open` when this browser has no key. The "Open" button for a session's history file now saves the file. The `?key=` page also no longer forwards an address such as `/\example.com` to another site. After upgrading, run `kiroku open` once in each browser you use (ADR 0031)
- The formatted view of "Analyze with AI" no longer slows down quadratically on a reply full of backticks that can't close inline code (one such reply froze the page for seconds on every redraw)
- Kiro Crew: a folder of archived or crew-log records that can't be listed is reported under "Data sources" instead of being read as empty, since it may hold the mark that a conversation keeps no content
- Folders kiroku copies history into (archive subfolders) and its settings folder are made private (`0700`) even when they already existed, so other users can't list session IDs and project folders
- Negative token counts in history are read as 0 instead of showing negative tokens and cost
- A resume command for a session ID that starts with `=` quotes it, so zsh doesn't expand it to a program path
- `install.sh` copies the new binary to a temporary file with a fresh random name (`mktemp`) next to the install location before moving it into place, instead of the fixed `.kiroku.new`, so a file or link someone else left there can't be written through. It and `tools/reproduce.sh` also download over HTTPS only (TLS 1.2 or later), even across redirects

### Fixed

- "Analyze with AI" failed with only "invalid request" when a report prompt was too long, which happens in a busy week or month (each session's prompts and replies are in the prompt, and Japanese text takes 3 bytes a character). Report prompts now shorten the session digest to stay under about 190 KB, fewer prompts per session first, and say so. A prompt still over the limit is not sent, and the dialog says it is too long and to copy it instead. When the connection breaks during a run, the dialog says so and names the likely causes instead of the browser's "Failed to fetch"
- In report prompts, projects hidden in the legend (when colored by project) are left out of the commits, the project sections and the repeated-prompt signals too, not only their sessions, and the prompt says the Numbers table still counts them
- In report prompts, "Main model of each session" and "Short sessions" no longer show whole-session estimated costs, which could add up to more than the period's Numbers table
- "Analyze with AI" with Claude Code returns the result when the agent has finished but something it started still holds its output open, instead of waiting for the time limit and failing
- "Data sources" in `kiroku serve` counts every history file that couldn't be read after a reload, not only the first
- The page `kiroku serve` shows while it reads your history follows the view's language (`--lang`, the language chosen in the view, or the browser's), so it is in Japanese too
- In Japanese, a file written with `kiroku html --week` no longer names its week in English ("week of …"), lists of agents and models use 「、」, and "Month-end cost" and "Month-end credits" have one name each
- The Year in review image no longer drops PRs from the totals when the labels are long (as in Japanese); it makes the text smaller instead
- A trend whose last period is still in progress says so ("Oct 12 (so far)") and draws that point hollow, so a partial week doesn't look like an improvement. The "Worth a look" help says trends are shown for metrics counted per period
- Screen readers hear the day of commit, push, pull request and usage-limit markers in the calendar, "You:" before your prompts and "AI:" before replies in the prompt flow, and session buttons in the Project view show their date
- The guide and the READMEs describe the sections the review and report prompts ask for now (Professional prompt assessment, Prioritized advice and Reusable prompt playbook for a session; Expert observations and Next actions for a report), and README.ja uses the view's words (PC, プロンプト, 応答, 基準)
- Claude Code: a compaction no longer extends a session's end or adds an activity block. Since 2.1.290 Claude Code compacts about 54 minutes after you leave (`idleCompaction`), so a session you left looked active until then. The compaction is still counted, and a manual `/compact` still counts from the command you typed
- Kiro Crew: a subagent running on the Codex backend (`provider: "codex"` in its `state.json`) is no longer counted twice. Its "Kiro Crew" session is no longer folded into its parent and is dropped when the Codex session of the same thread is loaded, as conversations on the Codex backend already were
- docs/sources.md lists the Kiro Crew archive reason `file-changes`, which is read like `compact` (lines that match a line kept elsewhere are dropped)
- The guide now says to delete `~/Library/Logs/kiroku.log` when uninstalling on macOS, that an AI agent installed while `kiroku serve --ai` runs is offered once history is read again, that kiroku ignores a `KIRO_HOME` or `KIROCREW_HOME` it can't safely use, and names the "Read more (N characters)" button of a cut prompt

## v0.34.1 - 2026-10-14

### Changed

- "Run with AI" is now called "Analyze with AI", matching the Japanese 「AI 分析」. Its dialog: Cancel and Esc during a run stop it and keep what arrived (they no longer close the dialog), the buttons stay in view while output streams, focus moves to Cancel when a run starts, reopening the dialog for the same prompt shows the last result, once a result arrives the prompt and progress fold away (smoothly, or at once with reduced motion) behind "Show prompt and progress" so the result has the room, a failed run points to the agent's own output instead of repeating it, and agents are listed by name ("Claude Code (claude)"). The daily report dialog names its date, and the AI buttons say which report they run for screen readers. With `--ai` but no agent installed, the view says to restart `kiroku serve --ai` after installing one

## v0.34.0 - 2026-10-14

### Added

- "Run with AI" (`kiroku serve --ai`) shows the run as it happens: the command kiroku ran, the files Claude Code reads, the progress and errors the agent prints, and its reply as it is written. The reply can be shown formatted (headings, lists, tables and code laid out) or as the Markdown it was written in, and the choice is remembered. The agent list offers only the agents whose command is found on this computer

### Changed

- The weekly and monthly report buttons ("Copy prompt" and "Run with AI") moved from the summary heading to just below the period's dates and numbers, like each day's "Report" button sits on the day it covers. The "Run with AI" dialog no longer repeats that the agent may send the prompt and history to its provider (the guide and SECURITY.md still say so)

## v0.33.0 - 2026-10-14

### Added

- The view in Japanese: it follows the browser's language (Japanese if the browser prefers Japanese, otherwise English), and the button at the top right switches between English and 日本語. `kiroku html --lang ja` and `kiroku serve --lang ja` (or `KIROKU_LANG`) pick the language when the viewer hasn't chosen one. Labels, tooltips, dialogs, the "?" explanations, the Year in review and its image, and dates (10月14日(水)) are translated; history, agent and model names, `kiroku json`, the terminal output and the prompts copied for an AI stay as they are (ADR 0029)
- Journey view: sessions by project and day, next to the Week and Month calendar, with session details, search and filters
- Project view: a card per project with its sessions, Git commits and recorded pull requests, a breakdown by agent and the full list of events, each opening its details
- `kiroku serve --ai`: "Run with AI" on session reviews and daily, weekly and monthly reports runs the prompt with `claude`, `codex` or `kiro-cli` on this computer after you confirm each run, and shows the result. Off unless you pass `--ai`: without it the button reads "Run with AI (off)" and its dialog says how to turn it on. The agent may send the prompt and the history it reads to its provider. It runs in an empty temporary folder, Claude Code with only the Read tool limited to the history folders and Codex in a read-only sandbox, and when it fails the view shows the agent's own error message
- "Copy knowledge prompt" in "By project" of the weekly and monthly summary: copies a prompt that asks an AI agent on this computer to extract candidate domain knowledge, decisions and open questions for a project, with evidence from the period's history. Like the report prompts, it lists the sessions' history files by path and puts the history in a fenced block marked as data

### Fixed

- Worth a look: opened for Long conversations alone, the dialog explained Heaviest sessions instead. The Possible friction count and its trend stopped at 3 (the number of sessions the card lists); `kiroku json` now has `frictionN`, the count before that cut. The trends under Heaviest sessions and Wait time (median) say what they plot (the share of cost with no commit or pull request, and the 90th percentile, now in minutes and seconds)
- `kiroku autostart` now also keeps `XDG_DATA_HOME`, so on Linux the `kiroku serve` it starts at login reads the same `kiroku archive` copies and Amazon Q / Kiro CLI history as the one you run in your shell
- Codex: a response that ended incomplete was counted as a second copy of the response before it, in history that kiroku reads from `token_count` (sessions started before Codex wrote `token_usage_record`). The incomplete response now counts what it used
- Kiro IDE / CLI: a turn that Kiro CLI 2.30 or later ran on a fallback model was counted under the session's model, and after a fallback for the rest of the session, every earlier turn was counted under the new model. Each turn is now counted under the model that answered it
- Kiro: a `KIRO_HOME` that Kiro ignores (a relative path, a filesystem root, your home folder or a folder above it) made kiroku read that location instead of `~/.kiro`, where Kiro keeps the sessions; kiroku now ignores it too
- Amazon Q / Kiro CLI (SQLite): a tool result with images that the CLI rewrote as a prompt ("See images data supplied") was counted as your prompt
- Claude Code: a session moved to another project folder (`/cd`, or entering or leaving a worktree Claude Code created) was counted twice when `kiroku archive` kept a copy from the old folder: its prompts, tokens, cost and subagents appeared in two sessions. Lines in both files are now counted once
- Claude Code: an invalid `desktopSessionCleanupPeriodDays` in `settings.json` (not a whole number of 0 or more) pauses Claude Code's cleanup, but kiroku still warned that history older than 30 days would be deleted. It now reports no deletion, as it does for an invalid `cleanupPeriodDays`
- Week calendar: pressing or tabbing to a day's "Report" button no longer scrolls the calendar back to midnight
- Search: when nothing matches anywhere, only the search results say so (the calendar no longer adds a second empty message), and dates in the results stay on one line
- Amazon Q / Kiro CLI (SQLite): when you quit at a tool approval and later ran `--resume` without input, the summary request the CLI sends ("In a few words, summarize our conversation so far.") was counted as your prompt, and so were the `/todos resume` and response-timeout requests in the same situation. The CLI stores them as the answer to the unfinished tool call, so kiroku now recognizes them there too and shows them as notes
- Kiro: a Kiro CLI Classic session that you open in Kiro CLI 3.0 (`kiro-cli chat --v3 --resume-id <id>`) was counted twice, once in "Kiro CLI (Classic)" and once in "Kiro IDE / CLI", because kiro-cli copies it to the 3.0 store as `sess_<id>` and keeps the Classic file. It is now counted once, as the 3.0 session, which holds the prompts sent after the switch. The 3.0 copy has no credit records for the turns before the switch, so their credits, turn count and model requests are taken from the Classic session (ADR 0021)
- Kiro IDE / CLI: in a Classic session opened in Kiro CLI 3.0, the prompts from before the switch were shown without their replies. The copied replies carry no usable time, so kiroku now matches each reply to the prompt before it in the history, as it already does for Kiro CLI (Classic)

### Changed

- Daily report prompt: each project now comes with its active time and share of the day, split between projects that ran at the same time as in the weekly report, so the AI no longer has to estimate them from session spans. Session lines in the report prompts separate the agent and model with "·" instead of parentheses, so names like "Kiro CLI (Classic)" no longer read as "Kiro CLI (Classic) (model)", and the prompts ask the AI to open the listed history files by their exact paths without searching the folders around them
- View wording and layout tidied: shorter hints under the report prompt buttons, the Worth a look dialog shows the threshold right under what was observed (the "Flagged because" line is gone), the Repeated prompts dialog shows which prompt each session wrote, card names match their "?" ("Possible friction", "Estimated cost", "Read from cache", "Wait time (median)"), agent-specific metrics drop a repeated unit and an n= equal to the value, credits read "cr" in compact places, the cost breakdown prints each day's amount over its bar and leaves the pricing details to "?", Year in review lists changes since the middle under one heading and shows "Your light" before the share image, "Data sources" folds agents with no sessions under "Not found: N", and a single-day range reads "Oct 5" instead of "Oct 5 – Oct 5"
- Smaller view fixes: the Repeated prompts dialog no longer repeats the headline prompt under each session, a search hit in a title highlights the title instead of repeating it, a search with no results drops the calendar's footnote and Zoom, usage limit and compaction times are listed after a colon and separated by semicolons in English (reset times in 24-hour form, like the rest of the view), "vs last week ±0" replaces "no change", the prompt flow's legend reads "Your prompt", the Shortcuts dialog drops its repeated label, the metrics guide no longer mentions SPACE twice, "?" for Agent-specific metrics says what n= is, the Project view leaves out a zero PR count, and the By project table keeps project names readable at 320px. Japanese: 「2 PR」, 「2026年」 and complete sentences in the Year in review
- Journey rows no longer repeat the "Recorded activity only" note as a tooltip on every row (the legend says it once), and the search box at phone widths no longer cuts its placeholder
- Prompt flow: a prompt or reply is no longer cut when 20 or fewer characters would be hidden; in a saved HTML file a cut one ends with "…" and a note above the flow says how to read it in full, and the wait after a reply reads "you replied after"

### Security

- The page now asks browsers not to offer page translation (`translate="no"`). A browser's built-in translation can send the page's text, which includes your prompts, titles and file paths, to the browser vendor's translation service; use the Japanese view instead
- A Kiro Crew conversation held in incognito or temporary mode, opened in Kiro CLI 3.0 (`--resume-id`), is now shown with counts and times only in its 3.0 copy too, as its Classic and SQLite copies already were; its prompts and title in the 3.0 copy were shown in full
- A Kiro Crew conversation held in incognito or temporary mode on the Claude Code backend is now shown with counts and times only in Claude Code's own history (`~/.claude/projects/<project>/<session ID>.jsonl`, which Crew doesn't delete) too; its prompts, replies and title there were shown in full
- Copies of private Kiro Crew conversations in Codex's and Claude Code's history were shown in full when the Kiro history wasn't read (`--sources codex`, `--sources claude`). kiroku now reads Crew's records to find them whichever sources you pick, and `kiroku serve` reloads when Crew's records change even when it doesn't read the Kiro history (also when Crew's folder, `session_map.json` or a folder in it is a symbolic link), so a conversation that becomes private later is hidden without waiting for other history to change. If kiroku can't read Crew's `session_map.json` for this check (malformed, not a JSON object, too large, not a regular file), it says so under "Data sources" and in the terminal (whether or not it reads the Kiro history), and hides every conversation Crew's subagent records and crew logs name, since it can't tell which are private
- Only a private Kiro Crew conversation's current Codex thread was hidden. Its earlier conversation (`discarded_sid`), its subagents' sessions while their `state.json` remains (Crew writes none for incognito or temporary subagents, so their own Codex or Claude Code sessions usually can't be identified and are still shown in full) and the conversations in its crew logs are now hidden too, in Codex's and Claude Code's history, whichever backend `session_map.json` names
- Kiro Crew crew logs: the conversation in a crew log whose header has no `slot` is now matched by its `session/opened` entry (as subagents already were), so its Codex or Claude Code copy is hidden when the conversation is private. A crew log whose `session/opened` or `session/class` entries record a memory mode other than `persistent` now marks its conversation private by itself, so its copies (and the earlier conversation it names as `previous`), its subagents' sessions named in `state.json` and other crew logs of the same conversation stay hidden after the conversation log and `session_map.json` entry are gone. Entries whose type is written with JSON escapes and a first line starting with a BOM are read too
- Errors about unreadable files that kiroku prints in the terminal and in `kiroku doctor` (which include file names from history) now have control and formatting characters removed, so a crafted file name can't change what the terminal shows

## v0.32.0 - 2026-10-11

### Changed

- BREAKING: Kiro sessions stored in `~/.kiro/sessions/<workspace>/sess_*` are now shown as "Kiro IDE / CLI" instead of "Kiro IDE", and Kiro CLI V2 sessions (`~/.kiro/sessions/cli/`) as "Kiro CLI (Classic)" instead of "Kiro CLI". Kiro IDE 1.x and Kiro CLI 3.0 (`kiro-cli --v3`) write the same store in the same format, and nothing in it records which of them created a session, so kiroku can't tell them apart; "Classic" is Kiro's own name for V2 sessions (ADR 0021). The `source` value in `kiroku json` changes the same way, so scripts that match `"Kiro IDE"` or `"Kiro CLI"` need updating (`schemaVersion` stays 1). `--sources kiro`, the options, the environment variables and the `kiroku archive` layout are unchanged, and each source keeps its color. "Data sources" now says what each Kiro history covers

### Fixed

- Kiro IDE / CLI: notifications and follow-up requests that Kiro's agent writes into a conversation (such as "[notification/info] …" or a workflow continuing) were counted as prompts you typed. They are now shown as notices or as sent by the agent, as Kiro itself tells them apart
- Kiro: a credit amount below 0 in the history no longer lowers the credit total; it counts as 0
- Amazon Q / Kiro CLI (SQLite): prompts you type that start with a tag (`<template>…`, `<html lang=en> fix it`) or "Caveat:" are counted as prompts with their text, instead of being shown as notes with their reply attached to the previous prompt. These histories store exactly what you typed, so kiroku no longer applies the Claude Code transcript rules to them
- Usage limit hits now include Claude Code's "Credit balance is too low" error (prepaid Console credits ran out), also when it comes as the API's "Your credit balance is too low to access the Anthropic API"
- Kiro Crew: usage limit hits are now counted. When a spent plan allowance stops a turn, the Crew dashboard writes an `error` line tagged `meta.kind: "usage_limit"` to the conversation log; kiroku counts each one as a usage limit hit at its time (hits within 1 minute count once), including in conversations whose prompts are in the kiro-cli history and, as times only, in private conversations. Sign-in, model and session-start errors are not counted, and the message text is never used
- Kiro Crew: conversations run on the Codex backend were counted twice, once as a "Kiro Crew" session from Crew's usage records and conversation log and once as the Codex session Codex itself records, doubling their prompts, turns and active time. Crew's session ID for these conversations is the Codex thread ID, so the "Kiro Crew" session is now left out when the Codex session with that thread ID is loaded
- Codex sessions run at the Fast or Ultrafast service tier were priced at standard rates, so their estimated cost was too low: about half for Fast and a sixth for Ultrafast. gpt-6-sol and gpt-6-luna run at Fast by default in Codex, so most of their sessions were affected. kiroku now reads the tier Codex records for the thread (`thread_settings_applied`: `priority` shown as Fast, `ultrafast`, `flex`) and prices each response at the tier in effect at that time, so a switch mid-session applies from that point, and a subagent or fork uses its own tier. Fast, Ultrafast and Flex prices come from OpenAI's pricing page, with their long-context rates over 272K input tokens (for example gpt-6-sol on Fast: $4.00 input, $0.40 cached input, $5.00 cache writes, $20.00 output per million tokens; gpt-6.1-sol on Ultrafast: $12.00, $0.60, $15.00, $60.00). A model with no published price for its tier, such as gpt-5.5-pro on Fast, is priced at standard rates and listed in "Data sources". Session details list the responses by service tier

### Security

- Any history (Claude Code, Codex, Kiro Crew and the others) with numbers too large to be real, such as a token count of 1.7e308, could make a total infinite, after which `kiroku html` and `kiroku json` failed and `kiroku serve` showed no history at all. Numbers above 10^15, and NaN or infinity, are now ignored wherever kiroku reads a number from history
- A Kiro history with credit amounts too large to be real (such as 1.7e308) could make the total infinite, after which `kiroku html` and `kiroku json` failed and `kiroku serve` showed no history at all. A single entry above 1,000,000 credits now counts as 0
- A Kiro Crew conversation held in incognito or temporary mode on the Codex backend is now shown with counts and times only in its Codex session too, as its kiro-cli and SQLite copies already were; its prompts, replies and title in Codex's own history were shown in full
- kiroku no longer hangs when a named pipe (FIFO) or another non-regular file sits where history is expected, such as `~/.claude/projects/<project>/<session>.jsonl` or a Codex `rollout-*.jsonl`. `kiroku stats`, `kiroku serve`, `kiroku html` and `kiroku archive` used to wait forever opening it; now such files are not opened and are listed as unreadable in "Data sources" ("not a regular file"). JSON history files (Kiro `session.json`, Crew `session_map.json` and `state.json`, and others) larger than 64 MiB are no longer read into memory and are listed as unreadable too

## v0.31.0 - 2026-10-10

### Added

- Year in review is back ("Year" after Week and Month, or `Y`), without skill levels, meters or under- and overexposed labels: it describes how you worked and doesn't score or rank you
  - A year of active time is drawn as streaks of light: across is the date, down is 6:00 to 6:00, so work after midnight, including New Year's Eve, lands on the previous day. Color is by agent, or by project when one agent has 90% or more of your time ("Color by" picks Auto, Agent or Project; project names never go on the image)
  - "What stands out" lists facts: the busiest month (30% above the rest, 10 hours or more, with 3 months and 8 weeks of history), the most consecutive active days (14 or more; a weekend off doesn't break a run), the longest stretch without AI (7 days or more), the first day of an agent you started later, and late nights (22:00–5:00) and weekends at 10% or more
  - Changes since the middle of the period ("Since May 22, compared with before") show only big changes in session length, "2+ sessions at once" (time when sessions actually overlapped), late nights and prompts per session, and "To reflect on" asks a neutral question about most changes and about your light; the questions stay on the screen
  - Your light names the shape of your year in one of seven photography terms with a plain description (Night Sky is new; "Refocus" is now "Bracketing"), from 8 weeks of history
  - The 1600×900 image to share leads with the streaks, with month and hour ticks, the date range, how to read it, kiroku's address, and what stands out that isn't private: the busiest month, the first day of an agent you started later and up to two changes since the middle, labelled with the date the period splits at. Checkboxes choose what else goes on it: small totals that say what the hours count (parallel sessions once, including time the AI ran on its own), your light, the color key, late nights and weekends, and the longest stretch without AI (never with dates). Late nights, weekends and the longest stretch are off until you tick them, options that would change nothing are greyed out with the reason, and the most consecutive active days never goes on the image
  - With less than 8 weeks of history it says "3 weeks with AI." instead of a year and shows only the days from the first one with history. Files that include only one week or month don't offer the year in review, since they don't have the year's history

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
