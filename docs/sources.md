# 履歴の読み方

kiroku が各エージェントの履歴をどこから、どう読んでいるかのまとめです。置き場所の一覧は [ガイドの「読み取る履歴」](guide.md#読み取る履歴) にあります。どのアダプターも、読んだ結果を共通のセッションの形（`internal/core` の `Builder`）にそろえます。

## Claude Code

`~/.claude/projects/*/*.jsonl`（`CLAUDE_CONFIG_DIR` があればその下の `projects`）。`internal/source/claude.go`

- 時刻は発言ごと。依頼は `type: "user"` の行、AI の動きは `assistant` の行
- トークンは、1 つの応答が複数行に分かれて記録されるので、メッセージ ID ごとにまとめてから数える（項目ごとに最大の値を使う）
- サブエージェントは `Task` / `Agent` の呼び出しと、`<セッション>/subagents/agent-*.jsonl`（古い版は `isSidechain` の行を時間で割り当て）から読む
- 目安コストは、Claude Code が書く `type: "cost-state"` の行（プロセスの起動 `startTime` からのモデル別の累計 `modelUsage[].costUSD`）があればそれに合わせる。プロセスごとに最新の累計を使い、起動から直前の行の時刻までの応答に、kiroku の料金表での見積もりの比で配る（見積もれないモデルはトークンの比）。履歴に応答がないモデル（タイトル付けなど）の分は、その時刻の 1 件として足す。記録のない期間（古い版、最後の記録より後）は kiroku の料金表で見積もる

## Kiro IDE

- v1.0 以降: `~/.kiro/sessions/<hash>/sess_*/session.json` + `messages.jsonl`。発言ごとの時刻と、`usage_summary` の `promptTurnSummaries` にあるクレジット
- v1.0 より前: `<globalStorage>/kiro.kiroagent/workspace-sessions/`。発言ごとの時刻がないので、開始 = 作成日時、終了 = ファイルの更新時刻のざっくり表示。クレジットは残っていない

`internal/source/kiro.go`

## Kiro CLI

`~/.kiro/sessions/cli/<id>.json`（メタ）+ `<id>.jsonl`（会話）。`internal/source/kiro.go`

- クレジットは `session_state.conversation_metadata.user_turn_metadatas[].metering_usage`（`unit` が `credit` の `value`）
- 古い版は SQLite（`data.sqlite3` の `conversations` / `conversations_v2`）。`internal/source/qstore.go`
- 新しい形式と SQLite に同じ会話が残ることがある。同じ会話 ID のものは新しい形式のほうだけを数える（クレジットが入っているため）。外した数は「計測の状態」に出る
- SQLite のほうにはトークンやクレジットが残っていないので、目安コストやクレジットには入らない

## Kiro Crew

`~/.kiro/crew`（`KIROCREW_HOME` があればそちら）。`internal/source/crew.go`

- Crew は kiro-cli を ACP で動かすので、会話そのものは Kiro CLI の履歴に残る。kiroku はそちらを数え、`session_map.json` と `subagents/*/state.json` に載っている会話に「Kiro Crew」の目印と、Crew のタイトル（サブエージェントならエージェント名と依頼内容）をつける
- Crew のダッシュボードから動かした会話は、クレジットが kiro-cli の履歴に残らないことがある。Crew が 1 ターンごとに書く使用量の記録（`usage/tokens/<日付>.jsonl` の `_type: "tokens"` の行）も読む
- 同じ会話は二重に数えないよう、会話ごとに kiro-cli の記録と Crew の記録の多いほうを使う
- kiro-cli の会話に結びつかない記録は「Kiro Crew」のセッションにする。Crew の裏方の処理（`slot: "_bg"`）は 1 日ごと、ダッシュボードのチャットは会話ごと
- Crew の会話の記録（`sessions/<会話キー>.jsonl`。1 行目がメタデータ、2 行目から `role`・`content`・`ts`・`tools`）も読む。kiro-cli の履歴に依頼が残っていない会話（ダッシュボードから動かしたものなど）と、kiro-cli の会話に結びつかない会話は、ここから依頼の流れ・時刻・使ったツールを補う。使用量の記録も kiro-cli の会話もなく、会話の記録だけがあるものも「Kiro Crew」のセッションにする
- Crew の使用量の記録は、Crew が残している期間（およそ 2 週間）だけ

## Amazon Q Developer CLI

`amazon-q/data.sqlite3`（場所は [guide.md](guide.md#読み取る履歴) の表）。Kiro CLI の古い版と同じ形。`internal/source/qstore.go`

## Codex CLI

`~/.codex/sessions/YYYY/MM/DD/rollout-*.jsonl`（`.jsonl.zst` も）と `archived_sessions/`（`CODEX_HOME` があればそちら）。`internal/source/codex.go`

- トークンは同じ値が何度も書き直されるので、同じものは 1 回だけ数える。新しい版の `token_usage_record` があればそちらを使う
- サブエージェントやフォークのファイルは親の履歴を先頭に写しているので、そのファイルが作られた時刻より前の行は数えない。サブエージェントは親のセッションの「サブエージェント」にまとめる
- タイトルは `session_index.jsonl` から取る
- モデル（OpenAI）は料金表に入れていないので、目安コストには入らない（「計測の状態」に、料金表にないトークンとして出る）。`--prices` で足せる

## クレジットとモデルの倍率

Kiro のクレジットは、履歴に記録された値をそのまま足します。モデルごとの倍率は Kiro が記録するときにかけたものを使い、kiroku ではかけ直しません。

## 参考にしたもの

Kiro の形式には公式ドキュメントがないため、[kiro-history](https://github.com/pajaydev/kiro-history) と [codeburn](https://github.com/getagentseal/codeburn) の実装を参考にしています。SQLite の形は [amazon-q-developer-cli](https://github.com/aws/amazon-q-developer-cli) のソースに合わせています（`conversations_v2` は Kiro CLI だけにあり、参考実装をもとにしています）。Kiro Crew は、Crew 付属の `credit_spend.py` が読んでいる使用量の記録と、Crew の `history.py` が書く会話の記録に合わせています。
