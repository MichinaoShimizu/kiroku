# CLAUDE.md

## Language

- Write commit messages, pull request titles and descriptions, and comments on GitHub (reviews, replies, issues) in English.

## Security

kiroku reads private AI-agent history (prompts, file paths, commit messages). Treat security as a requirement of every change, not a separate task.

- Before opening a PR, review the diff for security: untrusted input from history files, git output and the network; escaping in the generated HTML; file paths; permissions of files kiroku writes; what `kiroku serve` exposes; downloads and self-replacement in `kiroku update` and `install.sh`.
- History never leaves the machine. Don't add telemetry or any network request that sends history or derived data. The only network access is checking and downloading releases from GitHub.
- Everything from history, git and the network is untrusted. Escape it in the HTML view (never insert it as HTML), never pass it to a shell, and validate paths before reading or serving files.
- Files kiroku writes that contain history (HTML, JSON, archive copies, caches) must not be readable by other users (0600 files, 0700 directories).
- `kiroku serve` listens on 127.0.0.1 by default and keeps the Host check; anything that widens access must be opt-in and documented in SECURITY.md.
- Downloads must be verified (checksums.txt, and build provenance where available) before they replace anything.
- Keep `govulncheck` clean. When it reports a vulnerability in code kiroku calls, fix it (update the Go toolchain or the module) before other work, and release the fix.
- GitHub Actions: pin third-party actions to a full commit SHA, give each job the least `permissions` it needs, and don't run untrusted code with write tokens.
- Add a test for every security fix, and note it under `### Security` in CHANGELOG.md.

## Decision history

Use the `decision-log` skill (`.claude/skills/decision-log/`, installed from decision-continuity; update it with `npx skills add MichinaoShimizu/decision-continuity --skill decision-log -a claude-code -y` and copy `agents/claude/decision-log-reader.md` from the same release to `.claude/agents/`; `npx skills update` would replace the copy with a symlink into `.agents/`. Never edit it here) to recover why something is the way it is. Write Decision Logs in English (`lang=en`); they go to `docs/decision-logs/`. They are past observations, not rules: the rules are in this file, SECURITY.md and docs/compatibility.md.
