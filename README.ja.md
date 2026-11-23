# kiroku

[![CI](https://github.com/MichinaoShimizu/kiroku/actions/workflows/ci.yml/badge.svg)](https://github.com/MichinaoShimizu/kiroku/actions/workflows/ci.yml) [![Release](https://img.shields.io/github/v/release/MichinaoShimizu/kiroku)](https://github.com/MichinaoShimizu/kiroku/releases) [![License: MIT](https://img.shields.io/badge/license-MIT-blue)](LICENSE) [![CodeQL](https://github.com/MichinaoShimizu/kiroku/actions/workflows/codeql.yml/badge.svg)](https://github.com/MichinaoShimizu/kiroku/actions/workflows/codeql.yml) [![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/MichinaoShimizu/kiroku/badge)](https://scorecard.dev/viewer/?uri=github.com/MichinaoShimizu/kiroku) [![Go](https://img.shields.io/github/go-mod/go-version/MichinaoShimizu/kiroku)](go.mod) [![Downloads](https://img.shields.io/github/downloads/MichinaoShimizu/kiroku/total)](https://github.com/MichinaoShimizu/kiroku/releases)

[English](README.md) | 日本語

**AI と積み重ねた仕事を、記録から価値へ。**

何を作り、どう働いたかを振り返り、次の AI 活用につなげる。

kiroku は、Claude Code、Kiro（IDE・CLI・Kiro Crew）、Codex CLI のローカル履歴を 1 つにまとめます。アカウント・テレメトリ・アップロードは不要です。kiroku が AI エージェントを動かすこともありません。

**[デモを試す →](https://michinaoshimizu.github.io/kiroku/)**（ダミーデータ。ブラウザーだけで動きます）

[![週のカレンダー、セッションの詳細、トークンの内訳、検索、月の表示（ダミーデータ）](docs/demo.gif)](https://michinaoshimizu.github.io/kiroku/)

## kiroku の特長

- **どのエージェントも 1 つのカレンダーに。** Claude Code・Codex・Kiro を並べて見られます。エージェントごとにダッシュボードを開く必要はありません。
- **数えるだけでなく、読めるセッション。** 何を頼み、AI が何と答え、その間にどんなコマンド・コミット・プルリクエストがあったかを、時間・トークン・費用と並べて見られます。
- **AI との働き方の変化がわかる。** 稼働時間・トークン・推定費用と、その移り変わりを、1 つのセッションから 1 か月まで確かめられます。
- **振り返りは、あなたのエージェントが書く。** 「プロンプトをコピー」で、履歴に基づく振り返りや報告のプロンプトを、いつものエージェントに貼れます。
- **履歴は PC の外に出ません。** バイナリ 1 つで動き、アカウントもテレメトリもありません。kiroku 自身が AI エージェントを動かすこともありません。

## インストール

```bash
curl -fsSL https://raw.githubusercontent.com/MichinaoShimizu/kiroku/main/install.sh | sh   # macOS と Linux
kiroku doctor   # 見つかった履歴と、消されそうな履歴がないかを表示
kiroku serve    # http://localhost:8484/ で画面を開く
```

インストーラーは、ダウンロードしたファイルを `checksums.txt` で確かめます。[GitHub CLI](https://cli.github.com/) があれば、ビルドの来歴（build provenance）も確かめます。Windows では [Releases](https://github.com/MichinaoShimizu/kiroku/releases) の zip を、Go があれば `go install github.com/MichinaoShimizu/kiroku@latest` を使ってください。詳しくは[ガイド](docs/guide.md#install)（英語）にあります。

> [!IMPORTANT]
> **Claude Code は、既定では 30 日より古い会話を消します。** kiroku が見せられるのは、残っている履歴だけです。`~/.claude/settings.json` に `"cleanupPeriodDays": 3650` を設定するか（[公式ドキュメント](https://code.claude.com/docs/en/settings-reference#cleanupperioddays)）、`kiroku archive on` で kiroku に圧縮したコピーを残させてください（[詳しく](docs/guide.md#keep-a-copy-of-history-in-kiroku)）。

## できること

**すべてのセッションをカレンダーに。** いつ、どのプロジェクトで、何を頼んだかを週・月で表示し、日ごとの稼働時間・推定費用・Git コミットを並べます。作業の記録で、進み具合を示すものではありません。

![週のカレンダー（ダミーデータ）](docs/screenshot.png)

**セッションを会話として読める。** プロンプトの流れを時刻つきで表示し、自分で打ったプロンプトと自動で入ったものを分けます。その間のコマンド・コミット・プルリクエスト・サブエージェント・中断も並べ、プロンプトごとに AI の応答を添えます。「プロンプトをコピー」で、振り返りのプロンプトをあなたのエージェントに貼れます（下を参照）。

<img src="docs/session.png" alt="セッションの詳細（ダミーデータ）" width="560">

**どの数字も開ける。** 上の数字や週のサマリーの数字（推定費用・Git コミットなど）を押すと、日・エージェント・プロジェクト・モデルごとの内訳と、その元になったセッション・コミット・プロンプトが見られます。

**見直す候補。** 基準を超えた指標にはサマリーで印が付きます。押すと、何が起きたか、なぜ大事か、元になったセッション、13 週の推移、試すとよいことが、カレンダーを離れずに見られます。

<img src="docs/worth.png" alt="見直す候補から開いた指標（ダミーデータ）" width="560">

**何を変えればよいかがわかるサマリー。** 週・月のサマリーはグラフの 1 ページです。時間・トークン・推定費用（Claude Code・Codex・Kiro Crew の Kiro 以外のバックエンドは公開 API の料金で計算）・credits と、その使い道（プロジェクト・エージェント・モデル・セッションの長さ）を、残ったもの（Git のコミット・プッシュ・PR と、履歴から数えた AI の編集行を分けて）と、AI コミットあたり・プッシュあたり・AI が編集した 100 行あたりなどの単価と並べます。頭にはコスト・アウトプット・単価の 3 つの小さなパネルがあり、どれも量を切り替えて見るグラフ 1 つです（選んだものは覚えます）。どの数字にも、過去 12 週（または月）の推移のグラフ（いつもの範囲・累積・利用上限への到達つき）と前週比・前月比が付き、自分の過去といつもの範囲で比べます（例：「$31.23：いつもの範囲（$19.92〜$27.35）より上」）。ƒx を押すと、同じ比較を不等式で、計算した数字はその式も見られ、グラフごとに読み取りを 1 文で示します。週や月の途中なら、量の数字に月末（週末）の予測を出し、まだ来ていない日に、前の期間と同じ費用に収める 1 日の目安を出します（予算の設定は要りません）。記録する内容はエージェントによって違います。[エージェントごとに記録されること](docs/guide.md#what-each-agent-records)はガイドにまとめています。

![週のサマリー（ダミーデータ）](docs/summary.png)

**全期間を検索。** プロンプト・編集したファイル・コミットを、合った箇所の抜粋つきで探せます。

画面は、ブラウザーの言語が日本語なら日本語で表示されます。`--lang`（または `KIROKU_LANG`）で English と日本語を選べます。

## 振り返りとアドバイスは、あなたのエージェントが書く

セッションの詳細の「プロンプトをコピー」（セッションの振り返り）と、週のカレンダーの日付の横のコピーのアイコン（日報）で、いつも使っている AI エージェント（Claude Code や Codex など）に貼るプロンプトをコピーできます。kiroku はエージェントを動かさず、何も送りません。どこに貼るかはあなたが決めます。プロンプトには kiroku が数えた事実と履歴ファイルの場所が入っているので、アドバイスはよくある一般論ではなく、実際に起きたことに基づきます。プロンプトの書き方、エージェントの使い方、モデルとコンテキスト、プロンプトからスキル・サブエージェント・スクリプトに出せることに触れます。どの指摘にも、根拠（数字、時刻つきのプロンプト、kiroku が数えた兆候）が書かれます。ダミーの履歴から書かせた助言の抜粋（英語）:

> - **Turn the repeated check into a skill or script.** You typed "Run the full test suite and the linter, fix anything that fails, and show me the summary" 3 times in 3 sessions. Make it a skill or a project script the agent runs before each commit.
> - **Use a lighter model for quick questions.** On Thu, Aug 13 at 10:00 you asked a one-line question in a separate Opus session. A lighter model is enough for questions like this.

各プロンプトの中身と、得られるものの形は[ガイド](docs/guide.md#reviews-and-reports-with-your-agent)にあります。

## プライバシー

- 履歴は PC の外に出ません。テレメトリも、アカウントも、AI サービスもなく、kiroku が AI エージェントを動かすこともありません。ネットワークを使うのは、kiroku 自身の新しい版を GitHub で確かめてダウンロードするときだけです。
- kiroku が履歴から書くファイルは、あなたしか読めません。`kiroku serve` は `127.0.0.1` で待ち受け、鍵を持つブラウザーにだけ開きます。
- HTML には、プロンプト・AI の応答・ファイルのパス・コミットメッセージがそのまま入っています。人に渡す前に中身を確かめてください。
- コピーした振り返りと報告のプロンプトは、AI エージェントに履歴ファイルの場所を教え、エージェントはそれを読んで、提供元に送ることがあります。履歴を任せられるエージェントを使ってください。

詳しいことと、リリースの確かめ方は [SECURITY.md](SECURITY.md) にあります。

## 使い方

```bash
kiroku serve              # http://localhost:8484/ の画面。新しい履歴が入るたびに更新
kiroku open               # 別のブラウザーで開く（最初に一度 serve の鍵が要る）
kiroku autostart on       # ログインのたびに kiroku serve を起動（macOS と Linux）
kiroku html --week last   # 先週だけを 1 つの HTML に書き出して、人に見せる
kiroku update             # 最新の版に更新
kiroku help               # すべてのコマンド（オプションは "kiroku <command> --help"）
```

画面の見方、すべての指標、読む履歴、すべてのオプション、[アンインストールの方法](docs/guide.md#uninstall)は[ガイド](docs/guide.md)に、版の番号が約束することは[互換性](docs/compatibility.md)にあります（どちらも英語）。

## 貢献

kiroku が役に立ったら、⭐ を付けてもらえると、ほかの人が見つけやすくなります。Issue とプルリクエストは英語でも日本語でも歓迎します。[CONTRIBUTING.md](CONTRIBUTING.md) を読んでください。脆弱性は [SECURITY.md](SECURITY.md) の方法で、非公開で知らせてください。

## ライセンス

[MIT](LICENSE)
