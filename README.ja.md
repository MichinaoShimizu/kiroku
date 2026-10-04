# kiroku

[![CI](https://github.com/MichinaoShimizu/kiroku/actions/workflows/ci.yml/badge.svg)](https://github.com/MichinaoShimizu/kiroku/actions/workflows/ci.yml) [![Release](https://img.shields.io/github/v/release/MichinaoShimizu/kiroku)](https://github.com/MichinaoShimizu/kiroku/releases) [![License: MIT](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

[English](README.md) | 日本語

AI エージェント（Claude Code・Kiro・Kiro Crew・Amazon Q・Codex）の利用履歴を、Google カレンダーのような画面で可視化するツールです。

**[デモを試す](https://michinaoshimizu.github.io/kiroku/)**（ダミーデータ。ブラウザだけで動きます）

## 目的

限られたクレジットやトークンの中で AI エージェントを効果的に使えるかどうかは、結局のところ使う側の改善次第です。ところが、改善に必要な情報を誰もが等しく得て、有効な手を打てるかどうかという前提さえも、今は使う側次第になっています。

各エージェントは利用履歴を PC 内に記録していますが、保存場所も形式もエージェントごとに異なります。kiroku はそれらを読み取り、1 つの画面にまとめて、あなただけに表示します。誰もが自分の使い方を把握し、改善に取り組みやすくすることが目的です。

- **外部 API を一切呼び出しません**。履歴を PC の外に送ることもありません
- **kiroku 自身は AI を呼び出しません**
- **出力は静的な HTML ファイル 1 つだけです**。`kiroku serve` も、同じ HTML を PC 内（`127.0.0.1`）で表示するだけです

ネットワークに接続するのは、`install.sh` と `kiroku update` で kiroku 本体を GitHub Releases からダウンロードするときだけです。

## 画面

![kiroku の画面（ダミーデータ）](docs/screenshot.png)

画像は英語表示です。画面はブラウザの言語に合わせて日本語か英語で表示され、日本語のブラウザでは日本語で表示されます。右上の言語の選択欄で切り替えることもできます。

- **カレンダー**：いつ・どのプロジェクトで・何を依頼したかを、週 / 月のカレンダーで表示します。手元の git のコミットも並べ、どこに何をどう変更したかを確認できます
- **気づきと、試せること**：基準を超えた指標を、見えたこと・試せること・該当するセッション・8 週の推移の形で、優先度の高い順に示します
- **サマリー**：プロジェクト別・日別に、コスト（作業時間・トークン・目安コスト・クレジット）と成果（コミット・PR・AI が編集した行）を並べて表示します。プロジェクト・ブランチ・エージェントごとに、時間の割合とトークンや目安コストの割合を帯で見比べられます。各指標には、そこから言えること・言えないことの説明が付きます
- **検索と週報**：依頼文・変更したファイル・コミットを全期間から探せます。週・月ごとに、やったこと・コミット・PR をまとめた週報の下書きをコピーできます
- **改善案プロンプト**：表示中の週・月、または 1 つのセッションのデータから、AI に使い方の改善案を聞くためのプロンプトを作ります（コピーして、お使いの AI エージェントに貼り付けます）

![週次サマリー（ダミーデータ）](docs/summary.png)

## インストール

macOS・Linux:

```bash
curl -fsSL https://raw.githubusercontent.com/MichinaoShimizu/kiroku/main/install.sh | sh
```

Windows は [Releases](https://github.com/MichinaoShimizu/kiroku/releases) から zip をダウンロードして展開してください。Go がある場合は `go install github.com/MichinaoShimizu/kiroku@latest` でもインストールできます。

## 履歴を残す設定

kiroku が表示できるのは、エージェントが残している履歴だけです。**Claude Code は、既定では 30 日より古い会話を自動で消します**。消えた履歴は元に戻せないので、インストールしたらすぐ、長い期間を設定してください。`~/.claude/settings.json` に次を加えます（[公式ドキュメント](https://code.claude.com/docs/en/settings-reference#cleanupperioddays)）。

```json
{
  "cleanupPeriodDays": 3650
}
```

Kiro Crew も古い会話の記録を消します（`session.archive_retention_days`）。既定のまま消える設定のときは、kiroku の画面でもお知らせします。エージェントごとの詳細は [履歴の保存期間](docs/guide.md#履歴の保存期間) を参照してください。

## アップデート

```bash
kiroku update          # 最新版に更新（--check で確認のみ）
```

`/usr/local/bin` など書き込み権限のない場所にある場合は `sudo kiroku update` を実行してください。`go install` でインストールした場合は `go install github.com/MichinaoShimizu/kiroku@latest` で更新します。

## 使い方

```bash
kiroku serve    # 画面を開く（http://localhost:8484/）。新しい履歴も自動で反映
kiroku html     # 静的な HTML（kiroku.html）を出力して開く
kiroku help     # コマンドとオプションの一覧
```

出力した HTML には依頼文やファイルパス、コミットメッセージがそのまま含まれます。人に渡すときは内容を確認してください。

画面の見方、指標の定義、読み取る履歴、オプションは [docs/guide.md](docs/guide.md)、開発については [docs/development.md](docs/development.md) を参照してください。

## 貢献

Issue と Pull Request は日本語・英語のどちらでも歓迎します。[CONTRIBUTING.md](CONTRIBUTING.md) を参照し、脆弱性は [SECURITY.md](SECURITY.md) のとおり非公開で報告してください。

## ライセンス

[MIT](LICENSE)
