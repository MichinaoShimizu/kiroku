# kiroku

English | [日本語](README.ja.md)

See how you actually work with AI coding agents. kiroku reads the local history of Claude Code, Kiro (IDE, CLI and Kiro Crew), Amazon Q and Codex, and shows it as a Google Calendar–style view.

## Why

Getting real value out of limited credits and tokens depends on how you use your agents. But even the information you need to improve is unevenly available today: every agent keeps its history on your machine, each in a different place and format.

kiroku reads those histories, puts them on one screen, and shows them to you alone, so that anyone can understand their own usage and find something concrete to improve.

- **No external API calls.** Your history never leaves your machine.
- **kiroku never calls an AI.**
- **The output is a single static HTML file.** `kiroku serve` only shows the same HTML on your machine (`127.0.0.1`).

kiroku connects to the network only when `install.sh` or `kiroku update` downloads kiroku itself from GitHub Releases.

## What you get

![kiroku (dummy data)](docs/screenshot.png)

- **Calendar**: when, in which project, and what you asked, by week or month. Your local git commits appear alongside, so you can see what changed where.
- **Findings and things to try**: metrics that crossed a threshold, each with what was observed, why it matters, one thing to try, the related sessions and an 8-week trend. Pick one with "Try this" and the next weeks show the before-and-after.
- **Summary**: cost (time, tokens, estimated cost, credits) next to outcomes (commits, pull requests, lines edited by AI), by project and by day. Every metric explains what it can and cannot tell you.
- **Search and weekly report**: search prompts, edited files and commits across all time, and copy a weekly report draft of what you did, your commits and pull requests.
- **Improvement prompts**: build a prompt from the shown week, month or a single session that asks an AI for suggestions on how you use it. Copy it into the agent you already use.

![Weekly summary (dummy data)](docs/summary.png)

## Install

macOS and Linux:

```bash
curl -fsSL https://raw.githubusercontent.com/MichinaoShimizu/kiroku/main/install.sh | sh
```

On Windows, download the zip from [Releases](https://github.com/MichinaoShimizu/kiroku/releases) and extract it. With Go installed, `go install github.com/MichinaoShimizu/kiroku@latest` also works.

## Update

```bash
kiroku update          # update to the latest version (--check to only check)
```

If kiroku is in a location you cannot write to, such as `/usr/local/bin`, run `sudo kiroku update`. If you installed it with `go install`, run `go install github.com/MichinaoShimizu/kiroku@latest` again.

## Usage

```bash
kiroku serve    # open the view at http://localhost:8484/ (new history appears automatically)
kiroku html     # write a static HTML file (kiroku.html) and open it
kiroku help     # list commands and options
```

The HTML contains your prompts, file paths and commit messages as they are. Check its content before sharing it with anyone.

For how to read the view, metric definitions, which histories are read and all options, see [docs/guide.en.md](docs/guide.en.md). For development, see [docs/development.md](docs/development.md) (Japanese).

## License

[MIT](LICENSE)
