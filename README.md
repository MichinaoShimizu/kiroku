# kiroku

[![CI](https://github.com/MichinaoShimizu/kiroku/actions/workflows/ci.yml/badge.svg)](https://github.com/MichinaoShimizu/kiroku/actions/workflows/ci.yml) [![Release](https://img.shields.io/github/v/release/MichinaoShimizu/kiroku)](https://github.com/MichinaoShimizu/kiroku/releases) [![License: MIT](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

**Your AI work history, visualized.** kiroku reads the local history of Claude Code, Kiro (IDE, CLI and Kiro Crew), Amazon Q Developer CLI and Codex CLI, and shows it as a Google Calendar–style view.

**[Try the live demo](https://michinaoshimizu.github.io/kiroku/)** (dummy data, runs in your browser).

## Why

Getting real value from limited credits and tokens depends on how you use your agents, yet the information you need to improve is hard to get: every agent keeps its history on your machine, each in a different place and format.

kiroku reads those histories, puts them on one screen, and shows them only to you, so that anyone can understand their own usage and find something concrete to improve.

- **No external API calls.** Your history never leaves your machine.
- **kiroku never calls an AI.**
- **The output is a single static HTML file.** `kiroku serve` only shows the same HTML on your machine (`127.0.0.1`).

kiroku connects to the network only when `install.sh` or `kiroku update` (including `--check`) contacts GitHub Releases to get kiroku itself.

## What you get

![kiroku (dummy data)](docs/screenshot.png)

- **Calendar**: when, in which project, and what you asked, by week or month. Each day (and each week in the month view) shows its active time, tokens or credits, sessions and Git commits, and your local commits appear alongside the sessions, so you can see what changed where.
- **Worth a look**: metrics that crossed a threshold are marked where they appear, with what was observed, why it matters, the related sessions and an 8-week (or 8-month) trend, and listed in priority order at the top of the summary.
- **Summary**: cost (time, tokens, estimated cost, credits) next to outputs (commits, cost per commit, sessions that reached a commit), by project, plus a daily trend chart. Share bars compare each project's, branch's or agent's share of active time with its share of tokens, estimated cost and credits. It also picks up prompts you keep typing, which you could save as a command or in CLAUDE.md. Every metric explains what it can and cannot tell you.
- **Session details**: the prompt flow with times, separating what you typed (including slash commands and `!` shell commands) from what entered the conversation on its own, with commits, subagents, interruptions and usage limits in between. Copy just your prompts, or a prompt that asks an AI to review the session.
- **Search and weekly report**: search prompts, edited files and commits across all time, and preview and copy a weekly or monthly report draft of what you did, your commits and pull requests.
- **Ask AI for suggestions**: build a prompt from the shown week, month or a single session that asks an AI for suggestions on how you use it. Copy it into the agent you already use.

![Weekly summary (dummy data)](docs/summary.png)

## Install

macOS and Linux:

```bash
curl -fsSL https://raw.githubusercontent.com/MichinaoShimizu/kiroku/main/install.sh | sh
```

It installs to `/usr/local/bin`, or `~/.local/bin` if that is not writable (set `KIROKU_INSTALL_DIR` to choose another place).

On Windows, download the zip from [Releases](https://github.com/MichinaoShimizu/kiroku/releases), extract it, and put `kiroku.exe` in a folder on your `PATH`. With Go installed, `go install github.com/MichinaoShimizu/kiroku@latest` also works.

## Keep your history

kiroku can only show history that your agents still keep, and **Claude Code deletes conversations older than 30 days by default**. Deleted history cannot be recovered, so set a longer period right after installing. Add this to `~/.claude/settings.json` (merge it into the existing object if the file already exists; under `$CLAUDE_CONFIG_DIR` if you set it) ([official docs](https://code.claude.com/docs/en/settings-reference#cleanupperioddays)):

```json
{
  "cleanupPeriodDays": 3650
}
```

If you'd rather not change it, run `kiroku archive on` and kiroku keeps compressed copies of that history on your computer and shows deleted conversations from them ([details](docs/guide.md#keep-a-copy-of-history-in-kiroku)).

kiroku shows a notice above the summary while Claude Code is still on its 30-day default. Kiro Crew also deletes old conversation records (`session.archive_retention_days`); kiroku notes this under "Data sources". See [History retention](docs/guide.md#history-retention) for each agent.

## Update

```bash
kiroku update          # update to the latest version (--check to only check)
```

See [Commands](docs/guide.md#commands) for `sudo`, `go install` and specific versions.

## Usage

```bash
kiroku serve    # open the view at http://localhost:8484/ (new history appears automatically)
kiroku html     # write a static HTML file (kiroku.html) and open it
kiroku help     # list commands ("kiroku <command> --help" for options)
```

The HTML contains your prompts, file paths and commit messages as they are. Check its content before sharing it with anyone.

For how to read the view, metric definitions, which histories are read and all options, see [docs/guide.md](docs/guide.md). For development, see [docs/development.md](docs/development.md).

## Uninstall

Delete the binary: `rm "$(command -v kiroku)"` (with `sudo` if it is in `/usr/local/bin`). kiroku keeps no data of its own; view settings are stored in your browser.

## Contributing

Issues and pull requests are welcome in English or Japanese. See [CONTRIBUTING.md](CONTRIBUTING.md), and report vulnerabilities privately as described in [SECURITY.md](SECURITY.md).

## License

[MIT](LICENSE)
