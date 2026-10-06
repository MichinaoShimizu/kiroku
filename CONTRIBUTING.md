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
test -z "$(gofmt -l .)" && go vet ./...   # format and vet (CI runs both)
GOTOOLCHAIN=$(go env GOVERSION) go run honnef.co/go/tools/cmd/staticcheck@2025.1.1 ./...   # static analysis (CI runs it on Ubuntu)
go run . serve                     # try it with your own history
sh tools/screenshots/run.sh --html /tmp/kiroku-demo.html   # an HTML with dummy data
```

Go 1.23 or later is required. The dummy-data HTML needs Python 3 and git; refreshing the screenshots also needs Node.js and Playwright (`npm i playwright && npx playwright install chromium` in `tools/screenshots`).

[docs/development.md](docs/development.md) covers the layout, how to add an agent, tests and golden data, CI, and the release procedure. A release is a PR that renames `## Unreleased` to `## vX.Y.Z - YYYY-MM-DD`; merging it tags and releases automatically (`.github/workflows/tag.yml`).

## Changelog

Add user-visible changes to `## Unreleased` in [CHANGELOG.md](CHANGELOG.md), in English (create the heading at the top if it is not there). That section becomes the release notes, and decides the next version: `BREAKING` (a change that breaks commands, options or output files) bumps minor while in 0.x and major from 1.0, `### Added` bumps minor, and anything else bumps patch (`sh tools/next-version.sh`).

## Changing the view

The view lives in `internal/web`: `template.html` (markup), `style.css` and `app.js`, put together into one HTML file by `web.go`. When you change it:

- The view is English only. Metric explanations live in `HELP`, and a test checks them against the "How to read the metrics" table in `docs/guide.md`
- Check light and dark, and 1440px, 1000px and phone widths
- Update the README, the guides and the screenshots (`sh tools/screenshots/run.sh`) in the same change
- Try the user scenarios in [docs/usability.md](docs/usability.md)

## Privacy

Never commit or attach real agent history, even in issues. Tests use synthetic data under `testdata/`, with placeholder paths such as `/Users/me`.
