# kiroku

[![CI](https://github.com/MichinaoShimizu/kiroku/actions/workflows/ci.yml/badge.svg)](https://github.com/MichinaoShimizu/kiroku/actions/workflows/ci.yml) [![Release](https://img.shields.io/github/v/release/MichinaoShimizu/kiroku)](https://github.com/MichinaoShimizu/kiroku/releases) [![License: MIT](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

[English](README.md) | 日本語

AI エージェント（Claude Code・Kiro（IDE・CLI・Kiro Crew）・Amazon Q Developer CLI・Codex CLI）の利用履歴を、Google カレンダーのような画面で可視化するツールです。

**[デモを試す](https://michinaoshimizu.github.io/kiroku/)**（ダミーデータ。ブラウザだけで動きます）

## 目的

限られたクレジットやトークンの中で AI エージェントを効果的に使えるかどうかは、結局のところ使う側の改善次第です。ところが、改善に必要な情報を誰もが等しく得て、有効な手を打てるかどうかという前提さえも、今は使う側次第になっています。

各エージェントは利用履歴を PC 内に記録していますが、保存場所も形式もエージェントごとに異なります。kiroku はそれらを読み取り、1 つの画面にまとめて、あなただけに表示します。誰もが自分の使い方を把握し、改善に取り組みやすくすることが目的です。

- **外部 API を一切呼び出しません**。履歴を PC の外に送ることもありません
- **kiroku 自身は AI を呼び出しません**
- **出力は静的な HTML ファイル 1 つだけです**。`kiroku serve` も、同じ HTML を PC 内（`127.0.0.1`）で表示するだけです

ネットワークに接続するのは、`install.sh` と `kiroku update`（`--check` を含む）が kiroku 本体のために GitHub Releases に問い合わせるときだけです。

## 画面

![kiroku の画面（ダミーデータ）](docs/screenshot.png)

画像は英語表示です。画面はブラウザの言語に合わせて日本語か英語で表示されます（右上の選択欄で切り替え可）。

- **カレンダー**：いつ・どのプロジェクトで・何を依頼したかを、週 / 月のカレンダーで表示します。手元の git のコミットも並べ、どこに何をどう変更したかを確認できます
- **気づき**：基準を超えた指標を、見えたこと・なぜ気にするか・該当するセッション・8 週（月表示では 8 か月）の推移の形で、優先度の高い順に示します
- **サマリー**：プロジェクト別・日別に、コスト（作業時間・トークン・目安コスト・クレジット）と成果（コミット・PR・AI が編集した行）を並べて表示します。プロジェクト・ブランチ・エージェントごとに、作業時間の割合と、トークン・目安コスト・クレジットの割合を帯で見比べられます。各指標には、そこから言えること・言えないことの説明が付きます
- **検索と週報**：依頼文・変更したファイル・コミットを全期間から探せます。週・月ごとに、やったこと・コミット・PR をまとめた週報・月報の下書きを、文面を確かめてからコピーできます
- **AI に改善案を聞く**：表示中の週・月、または 1 つのセッションのデータから、AI に使い方の改善案を聞くためのプロンプトを作ります（コピーして、お使いの AI エージェントに貼り付けます）

![週次サマリー（ダミーデータ）](docs/summary.png)

## インストール

macOS・Linux:

```bash
curl -fsSL https://raw.githubusercontent.com/MichinaoShimizu/kiroku/main/install.sh | sh
```

`/usr/local/bin`（書き込めなければ `~/.local/bin`）に置きます。置き場所は `KIROKU_INSTALL_DIR` で変えられます。

Windows は [Releases](https://github.com/MichinaoShimizu/kiroku/releases) から zip をダウンロードして展開し、`kiroku.exe` を `PATH` の通ったフォルダに置いてください。Go がある場合は `go install github.com/MichinaoShimizu/kiroku@latest` でもインストールできます。

## 履歴を残す設定

kiroku が表示できるのは、エージェントが残している履歴だけです。**Claude Code は、既定では 30 日より古い会話を自動で消します**。消えた履歴は元に戻せないので、インストールしたらすぐ、長い期間を設定してください。`~/.claude/settings.json` に次を加えます（ファイルがあれば中の `{}` に足します。`$CLAUDE_CONFIG_DIR` を設定していればその中）（[公式ドキュメント](https://code.claude.com/docs/en/settings-reference#cleanupperioddays)）。

```json
{
  "cleanupPeriodDays": 3650
}
```

Claude Code が既定の 30 日のままのあいだは、kiroku がサマリーの上でお知らせします。Kiro Crew も古い会話の記録を消します（`session.archive_retention_days`）。画面の「計測の状態」に表示します。エージェントごとの詳細は [履歴の保存期間](docs/guide.md#履歴の保存期間) を参照してください。

## アップデート

```bash
kiroku update          # 最新版に更新（--check で確認のみ）
```

`sudo` が要る場合や `go install` で入れた場合、版を指定する場合は [コマンド](docs/guide.md#コマンド) を参照してください。

## 使い方

```bash
kiroku serve    # 画面を開く（http://localhost:8484/）。新しい履歴も自動で反映
kiroku html     # 静的な HTML（kiroku.html）を出力して開く
kiroku help     # コマンドの一覧（オプションは kiroku <command> --help）
```

出力した HTML には依頼文やファイルパス、コミットメッセージがそのまま含まれます。人に渡すときは内容を確認してください。

画面の見方、指標の定義、読み取る履歴、オプションは [docs/guide.md](docs/guide.md)、開発については [docs/development.md](docs/development.md) を参照してください。

## アンインストール

`rm "$(command -v kiroku)"` で本体を消します（`/usr/local/bin` にある場合は `sudo` を付けます）。kiroku は自分のデータを持たず、画面の設定はブラウザに保存されています。

## 貢献

Issue と Pull Request は日本語・英語のどちらでも歓迎します。[CONTRIBUTING.md](CONTRIBUTING.md) を参照し、脆弱性は [SECURITY.md](SECURITY.md) のとおり非公開で報告してください。

## ライセンス

[MIT](LICENSE)
