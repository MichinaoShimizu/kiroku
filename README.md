# kiroku

[![CI](https://github.com/MichinaoShimizu/kiroku/actions/workflows/ci.yml/badge.svg)](https://github.com/MichinaoShimizu/kiroku/actions/workflows/ci.yml) [![Release](https://img.shields.io/github/v/release/MichinaoShimizu/kiroku)](https://github.com/MichinaoShimizu/kiroku/releases) [![License: MIT](https://img.shields.io/badge/license-MIT-blue)](LICENSE) [![CodeQL](https://github.com/MichinaoShimizu/kiroku/actions/workflows/codeql.yml/badge.svg)](https://github.com/MichinaoShimizu/kiroku/actions/workflows/codeql.yml) [![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/MichinaoShimizu/kiroku/badge)](https://scorecard.dev/viewer/?uri=github.com/MichinaoShimizu/kiroku) [![Go](https://img.shields.io/github/go-mod/go-version/MichinaoShimizu/kiroku)](go.mod) [![Downloads](https://img.shields.io/github/downloads/MichinaoShimizu/kiroku/total)](https://github.com/MichinaoShimizu/kiroku/releases)

**Your AI work history, visualized.** No AI. No external APIs. No uploads.

kiroku reads the local history of Claude Code, Kiro (IDE, CLI and Kiro Crew), Amazon Q Developer CLI and Codex CLI, and shows it on one calendar, only to you.

**[Try the live demo →](https://michinaoshimizu.github.io/kiroku/)** (dummy data, runs in your browser)

![The week calendar (dummy data)](docs/screenshot.png)

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

**Every session on a calendar.** When, in which project and what you asked, by week or month, with each day's active time, tokens, cost and Git commits.

**The whole story of a session.** The prompt flow with times, separating what you typed from what entered on its own, with commands, commits, pull requests, subagents and interruptions in between, and the AI's reply after each prompt, so a session reads as the conversation it was. Copy a prompt that asks an AI agent on your computer to review the session from its history file.

<img src="docs/session.png" alt="Session details (dummy data)" width="560">

**Every number opens up.** Click any figure, at the top or in the weekly summary, such as tokens, estimated cost or Git commits, to see it by day, agent, project and model, with the sessions, commits or prompts behind it. The metrics guide (`G`) shows how the metrics fit together, and what they can't tell you.

**What's worth a look.** Metrics that crossed a threshold are listed under the key figures. Press one to see what was observed, why it matters, the sessions behind it, an 8-week trend and what to try, without leaving the calendar.

<img src="docs/worth.png" alt="A flagged metric opened from Worth a look (dummy data)" width="560">

**Cost next to output.** Time, tokens, estimated cost (Claude Code, Codex and Kiro Crew's non-Kiro backends, at public API rates) and credits by project, next to commits and cost per commit. Not every agent records everything; the guide lists [what each agent records](docs/guide.md#what-each-agent-records).

![Weekly summary by project (dummy data)](docs/summary.png)

**A weekly report your agent can write.** One click copies a prompt for an AI agent on your computer, such as Claude Code. It has the week's commits, pull requests and time for each project, and points to the history files of that week's sessions, so the agent can write what you did and why. Search covers prompts, edited files and commits across all time.

## Privacy

- Your history never leaves your computer: no telemetry, no account, no AI service. The only network access is checking for and downloading kiroku's own releases from GitHub.
- Files kiroku writes from your history are readable only by you, and `kiroku serve` listens on `127.0.0.1` and opens only for browsers that have its key.
- The HTML contains your prompts, the AI's replies, file paths and commit messages as they are. Check it before sharing.
- The review and report prompts point an AI agent on your computer to your history files, and the agent reads them. Use an agent you'd trust with them.

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

Issues and pull requests are welcome in English or Japanese. See [CONTRIBUTING.md](CONTRIBUTING.md), and report vulnerabilities privately as described in [SECURITY.md](SECURITY.md).

## License

[MIT](LICENSE)
