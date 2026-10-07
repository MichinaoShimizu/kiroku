# How histories are read

This page summarizes where kiroku reads each agent's history from and how. The list of locations is in [the guide's "Histories read"](guide.md#histories-read). Every adapter converts what it reads into the common session shape (`Builder` in `internal/core`). History lines longer than 64 MiB are skipped and the file is listed as unreadable in "Data sources", and compressed history (`.zst`) is read with a memory limit.

## Claude Code

`~/.claude/projects/*/*.jsonl` (or `projects` under `CLAUDE_CONFIG_DIR` when it is set). `internal/source/claude.go`

- Timestamps are per message. Prompts are `type: "user"` lines; the AI's activity is `assistant` lines
- A prompt sent while the AI is working is recorded only as a `queued_command` (`origin.kind: "human"`) in a `type: "attachment"` line, not as a `user` line, so it is picked up from there too. Messages from other agents (`peer`) and task notifications (`task-notification`) are not counted as prompts
- `isCompactSummary` lines written when a long conversation is summarized automatically ("This session is being continued…") are not counted as prompts
- The text of `user` lines is classified in `internal/core/kind.go`. Slash commands (`<command-name>` and `<command-args>`) become "/name args" and `<bash-input>` becomes "! command", and both count as prompts the person typed (`kind` is `command` or `shell`). `<system-reminder>` is stripped even when it appears inside the text. Text starting with tags such as `<task-notification>`, `<user-prompt-submit-hook>` or `<local-command-stdout>`, `Caveat:`, `isMeta` lines (such as the expanded text of a slash command), summaries, and `queued_command` entries not sent by a person are not counted as prompts; they are kept in `notes` (kind and first 160 characters, up to 300 per session) and shown in the prompt flow in a different color
- A single response is recorded across several lines, so tokens are grouped by message ID before counting (taking the largest value for each field)
- The text the AI wrote back (`text` blocks of `assistant` lines) is kept as that prompt's reply: the last one in the turn, with lines of the same message ID joined. Thinking, tool calls, subagent (`isSidechain`) lines and the messages Claude Code itself writes (`isApiErrorMessage`, model `<synthetic>`) are not replies. The HTML keeps the first 160 characters; `kiroku serve` can show the rest
- Subagents are read from `Task` / `Agent` calls and from `<session>/subagents/agent-*.jsonl` (older versions assign `isSidechain` lines by time)
- Estimated cost follows the `type: "cost-state"` lines Claude Code writes (per-model running totals `modelUsage[].costUSD` since the process started at `startTime`) when they exist. The latest total for each process is used and spread over the responses from process start up to the time of the preceding line, in proportion to kiroku's price-table estimates (by token ratio for models it cannot price). Cost for models with no responses in the history (such as title generation) is added as one entry at that time. Periods with no such record (older versions, or after the last record) are estimated from kiroku's price table
- A model whose `costUSD` is 0 although it was used is estimated from kiroku's price table instead. On a subscription there is no per-token bill, so Claude Code writes 0, and following it would show $0.00 for a week of real usage. The "(from Claude Code)" note in session details appears only when its record was actually used

## Kiro IDE

- Replies in the prompt flow come from `assistant` payloads (v1.0 and later), whether their `content` is a string, `text` blocks or nested `content` / `text`; ones with `operationType: "Reasoning"` are the model's thinking and are not replies. Versions that do not record them simply show no reply
- v1.0 and later: `<hash>/sess_*/session.json` + `messages.jsonl` under `~/.kiro/sessions/` (`~/.kiro` means `KIRO_HOME` when it is set; the same applies below). Per-message timestamps, and credits from `promptTurnSummaries` in `usage_summary`
- Before v1.0: `<globalStorage>/kiro.kiroagent/workspace-sessions/`. There are no per-message timestamps, so it is shown roughly with start = creation time and end = file modification time. No credits are recorded. Session IDs in `sessions.json` that contain path separators, `..` or drive names are skipped and reported as unreadable, so nothing outside the history folder is read

`internal/source/kiro.go`

## Kiro CLI

`~/.kiro/sessions/cli/<id>.json` (metadata) + `<id>.jsonl` (conversation). `internal/source/kiro.go`

- Credits come from `session_state.conversation_metadata.user_turn_metadatas[].metering_usage` (the `value` where `unit` is `credit`)
- Replies in the prompt flow come from the `text` content of `AssistantMessage` lines (new format; these lines carry no timestamp, so a reply is placed at the end of its turn, `user_turn_metadatas[].end_timestamp`, or at its prompt's time when that is missing) and from `assistant.Response` / `assistant.ToolUse` `content` (SQLite; timed by `stream_end_timestamp_ms`)
- Older versions use SQLite (`conversations` / `conversations_v2` in `data.sqlite3`). `internal/source/qstore.go`
- The same conversation can appear in both the new format and SQLite. For a given conversation ID, only the new format is counted (because it has credits). The number left out appears in "Data sources"
- The SQLite history has no tokens or credits, so it is not included in estimated cost or credits

## Kiro Crew

`~/.kiro/crew` (or `KIROCREW_HOME` when it is set). `internal/source/crew.go`

- Crew runs kiro-cli over ACP, so the conversations themselves are recorded in the Kiro CLI history. kiroku counts them there and marks the conversations listed in `session_map.json` and `subagents/*/state.json` as "Kiro Crew", with Crew's title (for a subagent, the agent name and the prompt)
- Conversations run from the Crew dashboard may not have their credits recorded in the kiro-cli history. kiroku also reads the usage records Crew writes every turn (`_type: "tokens"` lines in `usage/tokens/<date>.jsonl`)
- In usage records, the `slot` of a dashboard conversation is recorded as `chat-<sequence>-<UNIX seconds>`. Conversation keys (in `session_map.json` and the conversation log file names) are `dashboard:chat-…`, so they are normalized with the same rule as Crew's `spend_key_for_slot` before matching
- To avoid counting a conversation twice, whichever of the kiro-cli record and the Crew record is larger is used for each conversation
- Records that do not match a kiro-cli conversation become "Kiro Crew" sessions: Crew's background work (`slot: "_bg"`) per day, and dashboard chats per conversation
- Crew's conversation logs (`sessions/<conversation key>.jsonl`; the first line is metadata, then `role`, `content`, `ts` and `tools` from the second line) are read too. For conversations whose prompts are not in the kiro-cli history (such as those run from the dashboard) and conversations that do not match a kiro-cli conversation, the prompt flow (including the `assistant` lines as replies), times and tools used are filled in from here. A conversation that has only a conversation log, with neither usage records nor a kiro-cli conversation, also becomes a "Kiro Crew" session
- Older lines that overflow a conversation log are moved to `sessions/archive/<name>__<datetime>.jsonl` (how long they are kept is set by Crew's `session.archive_retention_days`, 30 days by default; see [History retention](#history-retention)), so those are also read, oldest first, while they exist. For a conversation whose log is gone because it was closed or expired, the session comes from usage records only and has no prompt flow
- Crew does not delete its usage records (its own dashboard shows only the last 30 days of them), so kiroku reads every day that is still in `usage/tokens/`

## Amazon Q Developer CLI

`amazon-q/data.sqlite3` (location in the table in [guide.md](guide.md#histories-read)). Same format as older Kiro CLI versions. `internal/source/qstore.go`

## Codex CLI

`~/.codex/sessions/YYYY/MM/DD/rollout-*.jsonl` (and `.jsonl.zst`) and `archived_sessions/` (under `CODEX_HOME` when it is set). `internal/source/codex.go`

- Replies in the prompt flow come from `agent_message` events (new and old shapes) and from `message` items whose role is not `user`
- The same token values are written repeatedly, so each is counted once. Newer versions' `token_usage_record` is used when present
- Subagent and fork files copy the parent's history at the top, so lines before the file was created are not counted. Subagents are grouped under "Subagents" in the parent session
- Titles come from `session_index.jsonl`
- `kiroku serve` watches only `sessions/`, `archived_sessions/` and `session_index.jsonl` (not logs or other state under `~/.codex`), and rereads only the threads whose files changed (a parent thread together with its subagents)
- Models (OpenAI) are not in the price table, so they are not included in estimated cost (they appear in "Data sources" as tokens not in the price table). They can be added with `--prices`

## Git

Commits by you (`user.email`) are read with the local `git log` from the git repository in the session's working directory (cwd), and pushes from the reflog of remote-tracking branches ("update by push"). Environments without git, and locations that are not repositories, are skipped, as are network (UNC) paths on Windows. Since these folders may hold repositories someone else made, git runs without the system config and with the repository's settings that run programs (fsmonitor, hooks, pager, textconv and external diff) turned off, and never asks for credentials. `internal/gitlog/gitlog.go`

## History retention

Adapters that implement `Retainer` return the settings that delete old history. Claude Code reads `cleanupPeriodDays` in `settings.json` (30 days by default). Kiro Crew reads `session.archive_retention_days` the way Crew does: `config.json` in the Crew folder with `config.local.json` merged over it (the overlay wins), written as `{"session": {"archive_retention_days": 30}}`. It is 30 days when the key is missing, when the value is not a whole number (a numeric string such as `"45"` counts, as in Crew) and when a file is missing or broken; `null` or a negative number turns Crew's cleanup off, so no retention is reported. Crew writes every key to `config.json`, so the value counts as set only when it differs from the default 30, and the setting is suggested in `config.local.json`, which wins over `config.json`. Crew deletes the files in `sessions/archive/` whose modification time is older than that, at most once an hour, and expires closed sessions' crew logs on the same setting. See [the guide's "History retention"](guide.md#history-retention) for details

Copies with `kiroku archive`: adapters that implement `Keeper` (Claude Code, and Crew's part of Kiro CLI) return the source locations to keep and where to put the copies. When on, `internal/archive.Sync` runs before reading and compresses each `.jsonl` in the source location to a `.jsonl.zst` at the same relative path (only files whose modification time changed; if the source location is a symbolic link, the target is kept). When reading, only copies whose original file is gone are added (Claude Code: `<archive location>/claude/<project>/<conversation ID>.jsonl.zst` and subagents; subagent files that were deleted are added from the copy even if the conversation itself remains. Crew: `<archive location>/crew/sessions/archive/`). When the original exists it is read instead, so nothing is counted twice

## Credits and model multipliers

Kiro credits are added up exactly as recorded in the history. Per-model multipliers are the ones Kiro applied when recording, and kiroku does not apply them again.

## References

Kiro's formats have no official documentation, so kiroku follows the implementations of [kiro-history](https://github.com/pajaydev/kiro-history) and [codeburn](https://github.com/getagentseal/codeburn). The SQLite layout follows the source of [amazon-q-developer-cli](https://github.com/aws/amazon-q-developer-cli) (`conversations_v2` exists only in Kiro CLI and is based on the reference implementations). Kiro Crew follows the usage records read by Crew's bundled `credit_spend.py` and the conversation logs written by Crew's `history.py`.
