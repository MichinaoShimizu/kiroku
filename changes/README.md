# Changelog fragments

A pull request with a user-visible change adds a file here instead of editing `CHANGELOG.md`, so pull requests that are open at the same time don't conflict. The release pull request (`sh tools/release.sh`) merges every fragment into `CHANGELOG.md` with `sh tools/changelog.sh` and deletes them.

## Format

One file per kind of change: `changes/<short-name>.<section>.md`

- `<short-name>`: lowercase letters, digits and `-`, starting with a letter or digit, for example the branch name (`serve-key`, `codex-subagents`). It only keeps files apart; it never appears in the changelog.
- `<section>`: one of `added`, `changed`, `deprecated`, `removed`, `fixed`, `security`. It becomes `### Added`, `### Changed` and so on.
- Content: one or more `- ` bullets in English, written as they should appear in the release notes. A line that continues a bullet, or a nested `  - ` bullet, is indented by two spaces. Blank lines are dropped. Nothing else: no headings, no text outside a bullet.
- Start a bullet with `BREAKING:` for a change that breaks what [docs/compatibility.md](../docs/compatibility.md) keeps compatible. It bumps the minor version while in 0.x and the major version from 1.0.

A pull request with changes of more than one kind adds one file per section, for example `serve-key.security.md` and `serve-key.fixed.md`.

```markdown
- `kiroku serve` reads history from Kiro Crew archives
  - Archives that can't be listed are reported under "Data sources"
```

## What the tools do with them

- `sh tools/changelog.sh --check` checks every file here and names each bad one (wrong name, unknown section, a line that isn't part of a bullet). CI runs it.
- `sh tools/next-version.sh` decides the next version from these files together with anything already under `## Unreleased` in `CHANGELOG.md` (the rules are in "Making a release" in [docs/development.md](../docs/development.md)).
- `sh tools/changelog.sh` merges them under `## Unreleased`: entries already there stay first, the fragments follow in file-name order, and the sections are ordered Added, Changed, Deprecated, Removed, Fixed, Security. Running it again changes nothing.
- `sh tools/changelog.sh --release vX.Y.Z` does the same and names the section `## vX.Y.Z - <today, UTC>`. `tools/release.sh` runs it.

Internal changes, tests and docs-only changes need no fragment.
