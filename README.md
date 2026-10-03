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
```

| オプション | 既定 | 説明 |
|---|---|---|
| `--sources` | `claude,kiro` | 読む履歴 |
| `--root` | `~/.claude/projects` | Claude Code の履歴の場所（`CLAUDE_CONFIG_DIR` も見ます） |
| `--gap` | `15` | 何分あいたら帯を分けるか |
| `-o`, `--out` | `kiroku.html` | 書き出す HTML |
| `--no-open` | | ブラウザを開かない |

## 画面でできること

- 週カレンダーにセッションを帯で表示（発言が多いほど濃く）
- 色分けはプロジェクト / ブランチ / ツール（Claude Code・Kiro）で切り替え。凡例クリックで表示・非表示
- 帯をクリックすると右に詳細：依頼の流れ（時刻つき）、使ったツール、変更したファイル、再開コマンド
- `◀` `▶`（矢印キー）で週移動、`t` で今週、`＋` `−` でズーム、タイトル・依頼文の検索

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

生成した HTML には依頼文やファイルパスがそのまま入ります。共有するときは中身に気をつけてください（`.gitignore` で `*.html` を除外しています）。
