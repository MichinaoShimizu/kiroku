# Contributing

Thanks for your interest in kiroku. Issues and pull requests are welcome in English or Japanese.

## Principles

Changes must keep kiroku's promises:

- No external API calls, and no history leaves the machine
- kiroku itself never calls an AI
- The output is a single static HTML file

## Development

```bash
go test ./...                      # tests
gofmt -l . && go vet ./...         # format and vet (CI runs both)
go run . serve                     # try it with your own history
sh tools/screenshots/run.sh --html /tmp/kiroku-demo.html   # an HTML with dummy data
```

Go 1.23 or later is required. [docs/development.md](docs/development.md) (Japanese) explains the layout and how adapters work.

## Changelog

Add user-visible changes to `## Unreleased` in [CHANGELOG.md](CHANGELOG.md), in English (create the heading at the top if it is not there). That section becomes the release notes.

## Changing the view

The view is `internal/web/template.html`. When you change it:

- Write both languages with `tr(Japanese, English)`. Metric explanations live in `HELP` and `HELP_EN`, and tests check them against `docs/guide.md` and `docs/guide.en.md`
- Check light and dark, and 1440px, 1000px and phone widths
- Update the README, the guides and the screenshots (`sh tools/screenshots/run.sh`) in the same change
- Try the user scenarios in [docs/usability.md](docs/usability.md) (Japanese)

## Privacy

Never commit or attach real agent history, even in issues. Tests use synthetic data under `testdata/` and `/Users/me`.
