---
kind: log
category: security
keywords: [--tools Read, --add-dir, --strict-mcp-config, --skip-git-repo-check, MkdirTemp, kiroku-ai-, tailWriter]
---

# [kiroku] Agents run by "Analyze with AI" get the least access each one allows, in an empty folder

> A past observation, not a decision. It binds nothing.

## What happened

- C1 2026-10-14 Still true: Claude runs with `--strict-mcp-config --tools Read` and `--add-dir` for each existing history folder only; Codex gets `--skip-git-repo-check`; every agent runs in a new empty temporary folder that is removed afterwards; the last 2 KiB of the agent's stderr is shown when it fails. History: [PR #387](https://github.com/MichinaoShimizu/kiroku/pull/387) / Now: [`internal/ai/runner.go` @`1d7b409`](https://github.com/MichinaoShimizu/kiroku/blob/1d7b4091906cc02d4f32bdabd120f5120dcf0792/internal/ai/runner.go), [`SECURITY.md` @`1d7b409`](https://github.com/MichinaoShimizu/kiroku/blob/1d7b4091906cc02d4f32bdabd120f5120dcf0792/SECURITY.md)

## Reason

Documented (description, AI): with no tools, Claude could not open the history files the prompts point to; `codex exec` refuses to run outside a git repository; running in serve's folder let that folder's CLAUDE.md, settings or repository inject instructions; and a bare "HTTP 502" hid errors such as not being logged in. Checked against Claude CLI 2.1.296, a file outside the added folders was denied ([PR #387](https://github.com/MichinaoShimizu/kiroku/pull/387)). Left open in the PR: Codex's read-only sandbox can read any file, and kiro-cli has no read-only mode.

## People involved

From the PR records; not necessarily who decided.

- PR #387: opened @MichinaoShimizu (2026-10-14), no approving review, merged @MichinaoShimizu (2026-10-14)

## Open questions

- Is it still accepted that kiro-cli runs with the user's own tool settings, with no read-only mode? (ask @MichinaoShimizu)
- Do the reasons above, written by an AI agent in the PR descriptions, match the maintainer's own judgement? (ask @MichinaoShimizu)

## Sources

- [ADR 0030 (removed), points 3–6](https://github.com/MichinaoShimizu/kiroku/blob/27e8298bdc0367afc623bc4db5c450ca0bad38b1/docs/adr/0030-opt-in-local-agent-execution.md)

## Related logs

- DL-10
