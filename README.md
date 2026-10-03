# kiroku

Claude Code と Kiro の作業履歴を、Google カレンダーみたいな週表示で振り返るツールです。
いつ・どのプロジェクトで・何を頼んでいたかが一目でわかります。

![kiroku の画面（ダミーデータ）](docs/screenshot.png)

## 使い方

Python 3.8 以上があれば動きます。追加のインストールは不要です。

```bash
python3 kiroku.py                    # kiroku.html を作ってブラウザで開く
python3 kiroku.py --sources kiro     # Kiro だけ
python3 kiroku.py --sources claude   # Claude Code だけ
python3 kiroku.py -o out.html --no-open --gap 20
python3 kiroku.py --weekly           # 最新の週のふりかえりを Markdown で書き出す
python3 kiroku.py --weekly 2026-09-30  # その日を含む週
```

| オプション | 既定 | 説明 |
|---|---|---|
| `--sources` | `claude,kiro` | 読む履歴 |
| `--root` | `~/.claude/projects` | Claude Code の履歴の場所（`CLAUDE_CONFIG_DIR` も見ます） |
| `--gap` | `15` | 何分あいたら帯を分けるか |
| `-o`, `--out` | `kiroku.html` | 書き出す HTML |
| `--no-open` | | ブラウザを開かない |
| `--weekly [日付]` | | 週のふりかえりを `kiroku-week-<月曜日>.md` に書き出す |

## 画面でできること

- 週カレンダーにセッションを帯で表示（発言が多いほど濃く）
- 色分けはプロジェクト / ブランチ / ツール（Claude Code・Kiro）で切り替え。凡例クリックで表示・非表示
- 帯をクリックすると右に詳細：依頼の流れ（時刻つき）、使ったツール、変更したファイル、再開コマンド
- キーボード：`←` `→` で週移動、`T` で今週、`/` で検索、`+` `−` でズーム、`Esc` で週のまとめに戻る、`?` で一覧
- テーマは自動・ライト・ダークを切り替えられます（右上のボタン）。色分け・ズーム・テーマはブラウザに覚えておきます
- スマホ幅では 1 日ずつ表示します（上の曜日タブで切り替え）
- 色は色覚の違いがあっても隣同士を見分けられるように検証した 8 色（藍・朱・青竹・山吹・牡丹・萌黄・藤・紅）。9 番目以降は灰色になります

## 週のまとめ（ふりかえり用）

<img src="docs/summary.png" alt="週のまとめパネル（ダミーデータ）" width="360" align="right">

右のパネルに、表示している週のまとめが出ます（セッションを選んでいるときは「週のまとめ」ボタンで戻れます）。`--weekly` で同じ内容を Markdown にも書き出せます。メモ欄つきなので、そのまま週次のふりかえりや Claude への相談に使えます。

| 指標 | 意味 |
|---|---|
| 作業していた時間 | どれかのセッションが動いていた時間（重なりは 1 回分） |
| AI の延べ稼働 | 並列で動かした分も足した合計 |
| 集中ブロック | 5 分までの切れ目を許して 60 分以上続いた作業と、その中で一番多かったプロジェクト |
| 1 日の切り替え | 続けて出した依頼のプロジェクトが変わった回数 |
| 並列で動かした時間 | 2 本以上のセッションが同時に動いていた時間と、最大の本数 |
| 待たせ時間 | AI が返してから次の依頼を出すまで（30 分以内のもの）の中央値と 90% 点 |
| 深夜・週末 | 22〜6 時と土日に作業していた時間 |
| こじれたかもしれないセッション | 言い直し（「違う」「やり直して」など）、中断、依頼 15 回以上が多いもの |

時刻はこのマシンのタイムゾーンで数えます。どれも履歴から推定した目安です。

> 自分のふりかえり用の数字です。人と比べたり、評価に使ったりするためのものではありません。

## 読む履歴

| ツール | 場所 | 時刻の細かさ |
|---|---|---|
| Claude Code | `~/.claude/projects/*/*.jsonl` | 発言ごと |
| Kiro IDE（v1.0 以降） | `~/.kiro/sessions/<hash>/sess_*/` | 発言ごと |
| Kiro CLI | `~/.kiro/sessions/cli/` | 依頼ごと |
| Kiro IDE（v1.0 より前） | `<globalStorage>/kiro.kiroagent/workspace-sessions/` | 開始と最終更新だけ |

`KIRO_HOME` が設定されていればそちらを見ます。古い Kiro CLI の `data.sqlite3` は時刻がほとんど残っていないため対象外です。

Kiro の形式には公式ドキュメントがないため、[kiro-history](https://github.com/pajaydev/kiro-history) と [codeburn](https://github.com/getagentseal/codeburn) の実装を参考にしています。読めない履歴があれば issue で教えてください。

## 注意

生成した HTML には依頼文やファイルパスがそのまま入ります。共有するときは中身に気をつけてください（`.gitignore` で `*.html` と `kiroku-week-*.md` を除外しています）。
