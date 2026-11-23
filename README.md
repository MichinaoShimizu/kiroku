# kiroku

[![CI](https://github.com/MichinaoShimizu/kiroku/actions/workflows/ci.yml/badge.svg)](https://github.com/MichinaoShimizu/kiroku/actions/workflows/ci.yml) [![Release](https://img.shields.io/github/v/release/MichinaoShimizu/kiroku)](https://github.com/MichinaoShimizu/kiroku/releases) [![License: MIT](https://img.shields.io/badge/license-MIT-blue)](LICENSE) [![CodeQL](https://github.com/MichinaoShimizu/kiroku/actions/workflows/codeql.yml/badge.svg)](https://github.com/MichinaoShimizu/kiroku/actions/workflows/codeql.yml) [![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/MichinaoShimizu/kiroku/badge)](https://scorecard.dev/viewer/?uri=github.com/MichinaoShimizu/kiroku) [![Go](https://img.shields.io/github/go-mod/go-version/MichinaoShimizu/kiroku)](go.mod) [![Downloads](https://img.shields.io/github/downloads/MichinaoShimizu/kiroku/total)](https://github.com/MichinaoShimizu/kiroku/releases)

English | [日本語](README.ja.md)

**Your AI work, remembered.**

See what you built, understand how you worked, and make your next AI session better.

kiroku brings the local history of Claude Code, Kiro (IDE, CLI and Kiro Crew) and Codex CLI together in one place. No account, telemetry or uploads, and kiroku never runs an AI agent.

**[Try the live demo →](https://michinaoshimizu.github.io/kiroku/)** (dummy data, runs in your browser)

[![The week calendar, a session, the token breakdown, search and the month view (dummy data)](docs/demo.gif)](https://michinaoshimizu.github.io/kiroku/)

## Why kiroku

- **Every agent on one calendar.** Claude Code, Codex and Kiro side by side, instead of one dashboard per agent.
- **Sessions you can read, not just count.** What you asked, what the AI replied, and the commands, commits and pull requests in between, next to time, tokens and cost.
- **How your work with AI changes.** Active time, tokens, estimated cost and patterns over time, from one session to a month.
- **Reviews written by your own agent.** "Copy prompt" gives you a review or report prompt grounded in your history to paste into the agent you already use.
- **Your history stays on your computer.** A single binary, no account, no telemetry, and kiroku never runs an AI agent itself.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/MichinaoShimizu/kiroku/main/install.sh | sh   # macOS and Linux
kiroku doctor   # what kiroku found, and whether any history is about to be deleted
kiroku serve    # open the view at http://localhost:8484/
```

The installer checks the download against `checksums.txt` and, with the [GitHub CLI](https://cli.github.com/), its build provenance. On Windows, use the zip from [Releases](https://github.com/MichinaoShimizu/kiroku/releases); with Go, `go install github.com/MichinaoShimizu/kiroku@latest`. See the [guide](docs/guide.md#install) for more.

> [!IMPORTANT]
> **Claude Code deletes conversations older than 30 days by default**, and kiroku can only show what is still there. Set `"cleanupPeriodDays": 3650` in `~/.claude/settings.json` ([docs](https://code.claude.com/docs/en/settings-reference#cleanupperioddays)), or run `kiroku archive on` to let kiroku keep compressed copies ([details](docs/guide.md#keep-a-copy-of-history-in-kiroku)).

## What you get

**Every session on a calendar.** When, in which project and what you asked, by week or month, with each day's active time, estimated cost and Git commits side by side. It records activity, not progress.

![The week calendar (dummy data)](docs/screenshot.png)

**The whole story of a session.** The prompt flow with times, separating what you typed from what entered on its own, with commands, commits, pull requests, subagents and interruptions in between, and the AI's reply after each prompt, so a session reads as the conversation it was. "Copy prompt" gives you a review prompt for it to paste into your own agent (see below).

<img src="docs/session.png" alt="Session details (dummy data)" width="560">

**Every number opens up.** Click any figure, at the top or in the weekly summary, such as estimated cost or Git commits, to see it by day, agent, project and model, with the sessions, commits or prompts behind it.

**What's worth a look.** Metrics that crossed a threshold are marked in the summary. Press one to see what was observed, why it matters, the sessions behind it, a 13-week trend and what to try, without leaving the calendar.

<img src="docs/worth.png" alt="A flagged metric opened from Worth a look (dummy data)" width="560">

**A summary that tells you what to change.** The weekly and monthly summary is a page of charts: time, tokens, estimated cost (Claude Code, Codex and Kiro Crew's non-Kiro backends, at public API rates) and credits, where they went (project, agent, model, session length), next to what it left behind, in two parts kept apart (Git commits, pushes and pull requests, and the lines AI edited according to history) and unit costs such as per AI commit, per push and per 100 AI-edited lines. It opens with three compact panels, cost, outputs and unit cost, each one chart with a switch for the measure (remembered). Each figure has a chart of its last 12 weeks or months with your usual range, the running total and usage-limit hits, the change from last week or month, and a comparison with your own past in plain words (such as "$31.23: above your usual $19.92–$27.35"). ƒx shows the same comparison as an inequality and the formula behind computed figures, and every chart opens with a takeaway. While a week or month is in progress, each amount shows its month-end (or week-end) estimate, and the days still to come show how much a day keeps it at the last one's total, with no budget to set. Not every agent records everything; the guide lists [what each agent records](docs/guide.md#what-each-agent-records).

![Weekly summary (dummy data)](docs/summary.png)

**Search across all time.** Prompts, edited files and commits, with excerpts showing where they matched.

The view is in Japanese when your browser prefers Japanese; `--lang` (or `KIROKU_LANG`) picks English or 日本語.

## Reviews and advice, written by your own agent

"Copy prompt" in session details (a session review) and the copy icon next to each date in the week calendar (a daily report) copy a prompt to paste into the AI agent you already use, such as Claude Code or Codex. kiroku runs no agent and sends nothing; where you paste it is your choice. The prompt carries the facts kiroku counted and points the agent to the history files, so the advice is grounded in what really happened, not generic tips: how you write prompts, how you use agents, model and context, and what to move out of your prompts into a skill, a subagent or a script. Each point names what it is based on: a figure, a prompt with its time, or a signal kiroku counted. An excerpt of advice written from dummy history:

> - **Turn the repeated check into a skill or script.** You typed "Run the full test suite and the linter, fix anything that fails, and show me the summary" 3 times in 3 sessions. Make it a skill or a project script the agent runs before each commit.
> - **Use a lighter model for quick questions.** On Thu, Aug 13 at 10:00 you asked a one-line question in a separate Opus session. A lighter model is enough for questions like this.

See the [guide](docs/guide.md#reviews-and-reports-with-your-agent) for what each prompt contains and the format of what you get.

## Privacy

- Your history never leaves your computer: no telemetry, no account, no AI service, and kiroku never runs an AI agent. The only network access is checking for and downloading kiroku's own releases from GitHub.
- Files kiroku writes from your history are readable only by you, and `kiroku serve` listens on `127.0.0.1` and opens only for browsers that have its key.
- The HTML contains your prompts, the AI's replies, file paths and commit messages as they are. Check it before sharing.
- The review and report prompts you copy point an AI agent to your history files, and the agent reads them and may send them to its provider. Use an agent you'd trust with them.

See [SECURITY.md](SECURITY.md) for details and for verifying a release.

## Usage

```bash
kiroku serve              # the view at http://localhost:8484/, updated as new history arrives
kiroku open               # open it in another browser (it needs serve's key once)
kiroku autostart on       # start kiroku serve each time you log in (macOS and Linux)
kiroku html --week last   # write only last week as one HTML file, to show someone
kiroku update             # update to the latest release
kiroku help               # all commands ("kiroku <command> --help" for options)
```

The [guide](docs/guide.md) explains the view, every metric, which histories are read, all options, and [how to uninstall](docs/guide.md#uninstall). [Compatibility](docs/compatibility.md) says what a version number promises.

## Contributing

If kiroku is useful to you, a ⭐ helps others find it. Issues and pull requests are welcome in English or Japanese. See [CONTRIBUTING.md](CONTRIBUTING.md), and report vulnerabilities privately as described in [SECURITY.md](SECURITY.md).

## License

[MIT](LICENSE)
