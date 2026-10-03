# kiroku

AI エージェント（Claude Code・Kiro・Kiro Crew・Amazon Q・Codex）の作業履歴を、週カレンダーと月カレンダーで振り返るツールです。

- **一目でわかる**：いつ・どのプロジェクトで・何を頼んでいたかを、見慣れた縦型の週カレンダーで見られます
- **まとめて見られる**：週次サマリー・月次サマリーで、時間の使い方と AI の使い方（トークン・目安コスト・クレジット）をまとめて見られます。Markdown にも書き出せます
- **手元で完結**：各エージェントが自分の PC に残している履歴を読むだけです。どこにも送りません

![kiroku の画面（ダミーデータ）](docs/screenshot.png)

## 入れ方

macOS・Windows・Linux で、実行ファイル 1 つで動きます。ほかに入れるものはありません。

**macOS・Linux**（おすすめ）:

```bash
curl -fsSL https://raw.githubusercontent.com/MichinaoShimizu/kiroku/main/install.sh | sh
```

OS と CPU（Intel / Apple Silicon・ARM）に合ったファイルを [Releases](https://github.com/MichinaoShimizu/kiroku/releases) から落とし、`checksums.txt` で確かめてから `/usr/local/bin`（書き込めなければ `~/.local/bin`）に置きます。版は `KIROKU_VERSION=v0.1.1`、置き場所は `KIROKU_INSTALL_DIR=~/bin` のように変えられます（`curl … | KIROKU_INSTALL_DIR=~/bin sh`）。

**Windows**: [Releases](https://github.com/MichinaoShimizu/kiroku/releases) から `kiroku_<版>_windows_<amd64 か arm64>.zip` を落として展開します。

**Go が入っていれば**: `go install github.com/MichinaoShimizu/kiroku@latest`

> macOS で、ブラウザから落とした `kiroku` を開くと「“kiroku”は開いていません」と止められることがあります（Apple の公証をしていないため）。上の `install.sh` か `go install` なら出ません。ブラウザから落とした場合は `xattr -d com.apple.quarantine ./kiroku` で外すか、システム設定 → プライバシーとセキュリティ の「このまま開く」で開けます。

## 使い方

```bash
kiroku              # これまでの履歴を kiroku.html に書き出して、ブラウザで開く
kiroku --serve      # 手元にサーバーを立てて開く。作業中に増えた履歴もその場で反映する
kiroku --weekly     # 最新の週の週次サマリーを Markdown で書き出す
kiroku --monthly    # 最新の月の月次サマリーを Markdown で書き出す
```

- **`kiroku`** は、起動した時点までの履歴をすべて読み、1 つの HTML に書き出します。そのあと増えた履歴は、もう一度実行すると入ります
- **`kiroku --serve`** は `http://localhost:8484/` で画面を出し続けます。数秒ごとに履歴フォルダの変化（ファイルの名前・大きさ・更新時刻だけ）を確かめ、変わっていたら読み直します。画面は見ている週・月や選んでいるセッションをそのままに新しい履歴を取り込み、左上に `LIVE` が出ます。止めるときは Ctrl+C
- **`kiroku --weekly [日付]`** は、その日を含む週の週次サマリーを `kiroku-week-<月曜日>.md` に、**`kiroku --monthly [YYYY-MM]`** は月次サマリーを `kiroku-month-<年-月>.md` に書き出します（下の「週次・月次サマリー」）

ほかのオプションは、下の「オプション」にまとめています。

## 画面でできること

- 縦型の週カレンダーに、セッションを帯で表示します（発言が多いほど濃く）。重なったセッションは横に並べます
- 右上の「週 / 月」で切り替えます。月の表示は、日ごとの作業時間を濃さで、プロジェクトの配分を細い帯で見せる月カレンダーです。日付を押すと、その週の週カレンダーを開きます
- カレンダーの上に、その週・月の要点（作業していた時間・作業した日・セッション / 依頼・トークン・目安コスト・Kiro クレジット）が出ます
- 日ごとのトークン・目安コスト・クレジットを、週カレンダーの曜日の見出しと月カレンダーの各日に出します。サマリーの「日ごとの使用量」では、トークン / 目安コスト / クレジットを切り替えて棒グラフで見られます
- 色分けはプロジェクト / ブランチ / ツール（エージェント）で切り替え。凡例クリックで表示・非表示
- 帯をクリックすると、右から詳細が開きます：依頼の流れ（時刻つき）、使ったモデル、サブエージェント、使ったツール、変更したファイル、再開コマンド
- カレンダーの下に、週次サマリー・月次サマリーを 3 列で出します（時間の使い方・AI の使い方・週 / 月のかたち）
- キーボード：`←` `→` で週・月の移動、`T` で今週・今月、`W` `M` で週・月の切り替え、`/` で検索、`+` `−` でズーム、`Esc` で詳細を閉じる、`?` で一覧
- テーマは自動・ライト・ダークを切り替えられます（右上のボタン）。色分け・ズーム・テーマはブラウザに覚えておきます
- スマホ幅では週カレンダーを横にスクロールでき、サマリーは 1 列になります
- 色は Okabe–Ito の配色をもとにした 8 色で、色覚の違いがあっても見分けやすくしています。9 番目以降は灰色になります

## 週次・月次サマリー

カレンダーの下に、その週・月のまとめが出ます。`--weekly` / `--monthly` で同じ内容を Markdown にも書き出せるので、そのまま週報・月報の下書きや Claude への相談に使えます。

![月の表示（ダミーデータ）](docs/summary.png)

```bash
kiroku --weekly 2026-09-30            # その日を含む週
kiroku --monthly 2026-09              # 2026 年 9 月
kiroku --monthly --md-dir ~/notes     # 書き出す場所を変える
```

| 指標 | 意味 |
|---|---|
| 作業していた時間 | どれかのセッションが動いていた時間（重なりは 1 回分） |
| AI の延べ稼働 | 並列で動かした分も足した合計 |
| 集中ブロック | 5 分までの切れ目を許して 60 分以上続いた作業と、その中で一番多かったプロジェクト |
| 1 日の切り替え | 続けて出した依頼のプロジェクトが変わった回数 |
| 並列で動かした時間 | 2 本以上のセッションが同時に動いていた時間と、最大の本数 |
| 待たせ時間 | AI が返してから次の依頼を出すまで（30 分以内のもの）の中央値と 90% 点 |
| 深夜・週末 | 22〜6 時と土日に作業していた時間 |
| 言い直し・中断のあった依頼 | 依頼文の言葉（「違う」「やり直して」など）と中断から推定した割合。依頼の数（n）を添えます |
| こじれたかもしれないセッション | 言い直し、中断、依頼 15 回以上が多いもの |
| 日ごと・週ごとのリズム | 週は日ごと、月は週ごとの作業時間（うち深夜） |

### AI の使い方

| 指標 | 意味 |
|---|---|
| 目安コスト（API 換算） | 履歴のトークン数に API の公開料金をかけた換算。サブスクリプションの請求額とは別物 |
| トークン | 入力・出力・キャッシュの読み書きの合計。1 つの応答が複数行に分かれて記録されるので、メッセージ ID ごとにまとめてから数えます |
| キャッシュから読んだ割合 | 入力のうちキャッシュから読んだ割合 |
| モデル別 | モデルごとの目安コストとトークン |
| サブエージェント | `Task` / `Agent` で呼んだ回数、種類、延べ時間。セッションの詳細では、いつ・どの種類に・何を頼んで・どれくらいかかったかを並べます |
| Kiro クレジット | Kiro の履歴に残っている実際のクレジット |
| 日ごとの使用量 | 日ごとのトークン・目安コスト・クレジット。記録された時刻の日に入れます |
| 1 依頼あたりの目安コスト | 目安コスト ÷ 依頼の数 |
| 重かったセッション | 目安コストの大きい順に 3 つ |

料金表は 2026 年 10 月時点の [公開料金](https://platform.claude.com/docs/en/about-claude/pricing) を入れています。変わったときや、知らないモデルを足したいときは JSON で上書きできます（モデル ID の先頭一致、USD / 100 万トークン）。

```json
{"claude-opus-5-5": {"input": 4, "output": 20, "cache_write": 5, "cache_write_1h": 8, "cache_read": 0.2}}
```

```bash
kiroku --prices my-prices.json
```

### エージェント別の参考指標

それぞれのエージェントが履歴に残している数字を、エージェントごとに並べます（週次・月次サマリー、その Markdown、セッションの詳細）。定義がエージェントごとに違うので、**エージェント同士では比べないでください**。どれも参照値で、目標ではありません。記録がない数字は 0 ではなく、出しません。

| エージェント | 参考指標 |
|---|---|
| Claude Code | 応答の数、1 応答あたりの出力トークン、入力のうちキャッシュから読んだ割合、ツール呼び出し、サブエージェントの実行 |
| Kiro CLI | クレジット、ターン、1 ターンあたりのクレジット、モデルへのリクエスト、組み込みツールの実行 |
| Kiro IDE | クレジット、ターン、1 ターンあたりのクレジット、ツール呼び出し |
| Kiro Crew | Crew から動かした会話、うちサブエージェント、クレジット、ターン |
| Kiro CLI（SQLite）・Amazon Q | 最初の返事までの時間（中央値）、応答にかかった時間（中央値）、応答の長さ（平均）、ツール呼び出し |
| Codex | 応答の数、推論トークン、出力のうち推論の割合、入力のうちキャッシュから読んだ割合、コンテキストの最大使用率、レート制限の最大使用率、ツール呼び出し |

時刻はこのマシンのタイムゾーンで数えます。どれも履歴から推定した目安です。データがないときは 0 ではなく「不明」と出します。トークン・目安コスト・AI の延べ稼働は使った量の目安で、生産性や節約した時間ではありません。

> 自分の使い方を振り返るための数字です。人と比べたり、評価に使ったりするためのものではありません。

## 読む履歴

| ツール | 場所 | 時刻の細かさ |
|---|---|---|
| Claude Code | `~/.claude/projects/*/*.jsonl` | 発言ごと |
| Kiro IDE（v1.0 以降） | `~/.kiro/sessions/<hash>/sess_*/` | 発言ごと |
| Kiro CLI | `~/.kiro/sessions/cli/` | 依頼ごと |
| Kiro IDE（v1.0 より前） | `<globalStorage>/kiro.kiroagent/workspace-sessions/` | 開始と最終更新だけ |
| Kiro CLI（古い版） | `kiro-cli/data.sqlite3`（下の表） | 依頼ごと |
| Amazon Q Developer CLI | `amazon-q/data.sqlite3`（下の表） | 依頼ごと |
| Codex CLI | `~/.codex/sessions/YYYY/MM/DD/rollout-*.jsonl`（`.jsonl.zst` も）と `archived_sessions/` | 発言ごと |

`data.sqlite3` の場所:

| OS | 場所 |
|---|---|
| macOS | `~/Library/Application Support/<kiro-cli か amazon-q>/` |
| Linux | `$XDG_DATA_HOME`（なければ `~/.local/share`）`/<kiro-cli か amazon-q>/` |
| Windows | `%LOCALAPPDATA%\<kiro-cli か amazon-q>\`（Kiro CLI は未確認。違っていたら `--kiro-cli-db` で指定してください） |

- `KIRO_HOME` が設定されていればそちらを見ます
- Kiro CLI は新しい形式（`~/.kiro/sessions/cli`）と SQLite に同じ会話が残ることがあります。同じ会話 ID のものは新しい形式のほうだけを数えます（クレジットが入っているため）。外した数は「計測の状態」に出ます
- SQLite のほうにはトークンやクレジットが残っていないので、目安コストやクレジットには入りません
- **Kiro Crew** は kiro-cli を動かすので、会話そのものは Kiro CLI の履歴（`~/.kiro/sessions/cli`）に残ります。kiroku はそちらを数え、Crew の `session_map.json` と `subagents/*/state.json` に載っている会話には「Kiro Crew」の目印と、Crew のタイトル（サブエージェントならエージェント名と依頼内容）をつけます。Crew のダッシュボードから動かした会話は、クレジットが kiro-cli の履歴に残らないことがあるので、Crew が 1 ターンごとに書く使用量の記録（`usage/tokens/<日付>.jsonl`）も読みます。同じ会話は二重に数えないよう、会話ごとに kiro-cli の記録と Crew の記録の多いほうを使います。kiro-cli の会話に結びつかない記録（Crew の裏方の処理 `_bg` は 1 日ごと、ダッシュボードのチャットは会話ごと）は「Kiro Crew」のセッションとして数えます
- Kiro のクレジットは、履歴に記録された値をそのまま足します（モデルごとの倍率は Kiro が記録時にかけたものを使い、kiroku ではかけ直しません）。古い Kiro IDE（v1.0 より前）と Kiro CLI（SQLite）の履歴にはクレジットが残っていないので、その分は入りません。Crew の使用量の記録は Crew が残している期間（およそ 2 週間）だけです。アカウントページの数字とは、期間（請求期間）やほかの PC で使った分の違いもあります
- **Codex** のトークンは、同じ値が何度も書き直されるので、同じものは 1 回だけ数えます。新しい版の `token_usage_record` があればそちらを使います。サブエージェントやフォークのファイルは親の履歴を先頭に写しているので、そのファイルが作られた時刻より前の行は数えません。サブエージェントは親のセッションの「サブエージェント」にまとめます。タイトルは `session_index.jsonl` から取ります
- Codex のモデル（OpenAI）は料金表に入れていないので、目安コストには入りません（「計測の状態」に、料金表にないトークンとして出ます）。入れたいときは `--prices` で足せます

Kiro の形式には公式ドキュメントがないため、[kiro-history](https://github.com/pajaydev/kiro-history) と [codeburn](https://github.com/getagentseal/codeburn) の実装を参考にしています。SQLite の形は [amazon-q-developer-cli](https://github.com/aws/amazon-q-developer-cli) のソースに合わせています（`conversations_v2` は Kiro CLI だけにあり、参考実装をもとにしています）。読めない履歴があれば issue で教えてください。

## オプション

| オプション | 既定 | 説明 |
|---|---|---|
| `--sources` | `claude,kiro,amazonq,codex` | 読む履歴（カンマ区切り。Kiro Crew は `kiro` に入ります） |
| `--root` | `~/.claude/projects` | Claude Code の履歴の場所（`CLAUDE_CONFIG_DIR` も見ます） |
| `--kiro-home` | `~/.kiro` | Kiro のデータの場所（`KIRO_HOME` も見ます） |
| `--crew-home` | `~/.kiro/crew` | Kiro Crew のデータの場所（`KIROCREW_HOME` も見ます） |
| `--kiro-cli-db` | OS ごと | Kiro CLI（古い版）の `data.sqlite3` |
| `--amazonq-db` | OS ごと | Amazon Q Developer CLI の `data.sqlite3` |
| `--codex-home` | `~/.codex` | Codex のデータの場所（`CODEX_HOME` も見ます） |
| `--gap` | `15` | 何分あいたら帯を分けるか |
| `-o`, `--out` | `kiroku.html` | 書き出す HTML |
| `--no-open` | | ブラウザを開かない |
| `--weekly [日付]` | | その日を含む週の週次サマリーを `kiroku-week-<月曜日>.md` に書き出す（日付なしなら最新の週） |
| `--monthly [YYYY-MM]` | | その月の月次サマリーを `kiroku-month-<年-月>.md` に書き出す（なしなら最新の月） |
| `--prices` | | 料金表を JSON で上書き（下の「AI の使い方」参照） |
| `--md-dir` | `.` | `--weekly` / `--monthly` の Markdown を置くフォルダ |
| `--json` | | 集計結果を JSON で書き出す（ほかのツールに渡したいとき） |
| `--serve [待ち受け先]` | `127.0.0.1:8484` | サーバーを立て、増えた履歴をその場で画面に反映する。`--serve :8485` のようにポートを変えられます |
| `--interval` | `5s` | `--serve` で履歴の変化を確かめる間隔 |
| `--version` | | 版を表示する |

## 開発

```bash
go test ./...      # testdata/ の合成データで、集計と Markdown が正解と一致するかを確かめる
go build .         # ./kiroku ができる
```

- エージェントを足すときは `internal/source` に `Source` を実装して、`source.All` に加えます。集計（`internal/report`）と画面（`internal/web/template.html`）は、共通のセッションの形（`internal/core`）だけを見ます
- `testdata/golden.json` と `testdata/golden-week.md` は、Go に移す前の Python 版が同じ合成データから出した結果です。Go 版はこれと同じ数字を出します
- PR ごとに、3 OS でのテストに加えて、リリースの予行演習（`goreleaser release --snapshot`、公開はしない）を CI で走らせます

### リリースの出し方

```bash
git tag v0.1.0
git push origin v0.1.0
```

`v` で始まるタグを push すると、GitHub Actions が 3 OS でテストしてから、GoReleaser で macOS・Linux・Windows（amd64 / arm64）向けのファイルとチェックサムを作り、Releases に載せます。`v0.2.0-rc.1` のように `-` のつくタグはプレリリースになります。

## 注意

- 書き出した HTML と Markdown には、依頼文やファイルパスがそのまま入ります。人に渡すときは中身を確かめてください（このリポジトリの `.gitignore` では `*.html`・`kiroku-week-*.md`・`kiroku-month-*.md` を除外しています）
- `--weekly` / `--monthly` の Markdown は `--md-dir ~/notes/kiroku` のように、自分用のフォルダに置くのがおすすめです
- `--serve` は既定で自分の PC からしか開けません（`127.0.0.1` で待ち受け、ほかのホスト名で来たリクエストは断ります）。`--serve 0.0.0.0:8484` のように外に開くと、同じネットワークの人も履歴を見られます

## ライセンス

[MIT](LICENSE)
