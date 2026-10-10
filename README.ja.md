# kiroku

[![CI](https://github.com/MichinaoShimizu/kiroku/actions/workflows/ci.yml/badge.svg)](https://github.com/MichinaoShimizu/kiroku/actions/workflows/ci.yml) [![Release](https://img.shields.io/github/v/release/MichinaoShimizu/kiroku)](https://github.com/MichinaoShimizu/kiroku/releases) [![License: MIT](https://img.shields.io/badge/license-MIT-blue)](LICENSE) [![Downloads](https://img.shields.io/github/downloads/MichinaoShimizu/kiroku/total)](https://github.com/MichinaoShimizu/kiroku/releases)

[English](README.md) | 日本語

**AI と積み重ねた仕事を、記録から価値へ。**

何を作り、どう働いたかを振り返り、次の AI 活用につなげる。

kiroku は、Claude Code、Kiro（IDE・CLI・Kiro Crew）、Amazon Q Developer CLI、Codex CLI のローカル履歴を 1 つにまとめます。過去の仕事をたどり、働き方の傾向を理解し、いつもの AI エージェントで根拠に基づく振り返りを行えます。アカウント・テレメトリ・アップロード・外部 AI 呼び出しは不要です。

**[デモを試す →](https://michinaoshimizu.github.io/kiroku/)**（ダミーデータ。ブラウザだけで動きます）

[![週のカレンダー、セッションの詳細、トークンの内訳、検索、月の表示（ダミーデータ）](docs/demo.gif)](https://michinaoshimizu.github.io/kiroku/)

## kiroku の特長

- **Remember — 思い出す。** エージェントをまたいで、プロンプト・応答・コマンド・コミット・プロジェクトをたどれます。
- **Understand — 理解する。** 稼働時間・トークン・推定費用や AI との働き方の変化を、セッションから年間振り返りまで確認できます。
- **Improve — 改善につなげる。** 履歴に基づくレビュープロンプトをいつもの AI エージェントに渡し、次のセッションの改善を考えられます。

- **どのエージェントも 1 つのカレンダーに。** Claude Code・Codex・Kiro・Amazon Q を並べて見られます。エージェントごとにダッシュボードを開く必要はありません。
- **数えるだけでなく、読めるセッション。** 何を頼み、AI が何と答え、その間にどんなコマンド・コミット・プルリクエストがあったかを、時間・トークン・費用と並べて見られます。
- **振り返りは、あなたのエージェントが書く。** kiroku は AI を呼びません。履歴に基づくプロンプトをワンクリックでコピーし、いつものエージェントに貼るだけです。
- **履歴はパソコンの外に出ません。** バイナリ 1 つで動き、アカウントもテレメトリもありません。

## インストール

```bash
curl -fsSL https://raw.githubusercontent.com/MichinaoShimizu/kiroku/main/install.sh | sh   # macOS と Linux
kiroku doctor   # 見つかった履歴と、消されそうな履歴がないかを表示
kiroku serve    # http://localhost:8484/ で画面を開く
```

インストーラーは、ダウンロードしたファイルを `checksums.txt` で確かめます。[GitHub CLI](https://cli.github.com/) があれば、ビルドの来歴（build provenance）も確かめます。Windows では [Releases](https://github.com/MichinaoShimizu/kiroku/releases) の zip を、Go があれば `go install github.com/MichinaoShimizu/kiroku@latest` を使ってください。詳しくは[ガイド](docs/guide.md#install)（英語）にあります。

> [!IMPORTANT]
> **Claude Code は、既定では 30 日より古い会話を消します。** kiroku が見せられるのは、残っている履歴だけです。`~/.claude/settings.json` に `"cleanupPeriodDays": 3650` を設定するか（[公式ドキュメント](https://code.claude.com/docs/en/settings-reference#cleanupperioddays)）、`kiroku archive on` で kiroku に圧縮した写しを残させてください（[詳しく](docs/guide.md#keep-a-copy-of-history-in-kiroku)）。

## できること

**すべてのセッションをカレンダーに。** いつ、どのプロジェクトで、何を頼んだかを週・月で表示し、日ごとの作業時間・トークン・費用・Git のコミットを並べます。

![週のカレンダー（ダミーデータ）](docs/screenshot.png)

**セッションを会話として読める。** 依頼の流れを時刻つきで表示し、自分で打った依頼と自動で入ったものを分けます。その間のコマンド・コミット・プルリクエスト・サブエージェント・中断も並べ、依頼ごとに AI の返事を添えます。振り返りのプロンプトもワンクリックでコピーできます（下を参照）。

<img src="docs/session.png" alt="セッションの詳細（ダミーデータ）" width="560">

**どの数字も開ける。** 上の数字や週のまとめの数字（トークン・推定費用・Git のコミットなど）を押すと、日・エージェント・プロジェクト・モデルごとの内訳と、その元になったセッション・コミット・依頼が見られます。指標のガイド（`G`）で、指標どうしの関係と、指標からはわからないことも確かめられます。

**見ておきたいこと（Worth a look）。** しきい値を超えた指標を、上の数字の下に並べます。押すと、何が起きたか、なぜ大事か、元になったセッション、8 週間の推移、試すとよいことが、カレンダーを離れずに見られます。

<img src="docs/worth.png" alt="Worth a look から開いた指標（ダミーデータ）" width="560">

**費用と成果を並べて。** プロジェクトごとの時間・トークン・推定費用（Claude Code・Codex・Kiro Crew の Kiro 以外のバックエンドは公開 API の料金で計算）・クレジットを、コミット数やコミットあたりの費用と並べます。記録する内容はエージェントによって違います。[エージェントごとに記録されること](docs/guide.md#what-each-agent-records)はガイドにまとめています。

![プロジェクトごとの週のまとめ（ダミーデータ）](docs/summary.png)

**全期間を検索。** 依頼・編集したファイル・コミットを、合った箇所の抜粋つきで探せます。

**1 年を 1 枚の写真に。** Week・Month の隣の「Year」を押すと、1 年の作業時間を光の筋として描き、シェア用の画像を作ります。画像に載せるものは自分で選べます。依頼・プロジェクト名・費用は載らず、深夜の割合や休みも、選ばなければ載りません。働き方を描くだけで、点数や順位は付けません。

![Year in review のシェア画像（ダミーデータ）](docs/year.png)

## 振り返りとアドバイスは、あなたのエージェントが書く

kiroku は AI を呼びません。代わりに、ワンクリックでプロンプトをコピーします。それを、このパソコンでいつも使っている AI エージェント（Claude Code や Codex など）に貼ってください。プロンプトには kiroku が数えた事実と履歴ファイルの場所が入っているので、エージェントは実際に起きたことを読み、決まった形で振り返りを書きます。最後には、AI エージェントとの働き方に詳しい専門家としてのアドバイスが付きます。

| 範囲 | 場所 | 得られるもの |
|---|---|---|
| 1 つのセッション | セッションの詳細の「Copy review prompt」 | 要約、数字、やり直しになった箇所とその元の依頼・原因（依頼に足りなかったこと、難しい作業、AI 自身の間違い）、アドバイス、次に使える最初の依頼の例 |
| 1 日 | 週のカレンダーの各日付の横の「Report」 | 日報: 要約、前日と比べた数字、プロジェクトごとにしたことと理由、アドバイス |
| 1 週間・1 か月 | まとめの「Copy weekly report prompt」（月の表示では monthly） | 同じ形の週報・月報。前の期間と比べた数字つきで、チームにそのまま共有できます |

アドバイスはよくある一般論ではなく、あなたの履歴に基づきます。どの指摘にも、根拠（数字、時刻つきの依頼、kiroku が数えた兆候）が書かれます。扱うのは次のことです。

- **依頼の書き方**: 足りなかった背景・制約・完了の条件、最初の依頼がはっきりしていれば要らなかった手直し
- **エージェントの使い方**: 作業の分け方と順番、結果の確かめ方、長いセッション、待ち時間
- **モデルとコンテキスト**: 作業に合ったモデルか（たとえば 1 行の質問に Opus 級のモデル）、長くなった会話、コンパクション、プロジェクトの切り替え
- **依頼から外に出せること**: 何度も打っている指示はスキルやプロジェクトのエージェント向けの指示に、別の役割はサブエージェントに、決まった手順はスクリプトに

ダミーの履歴から書かせた週報の抜粋（英語）:

> - **Turn the repeated check into a skill or script.** You typed "Run the full test suite and the linter, fix anything that fails, and show me the summary" 3 times in 3 sessions. Make it a skill or a project script the agent runs before each commit.
> - **Use a lighter model for quick questions.** On Thu, Aug 13 at 10:00 you asked a one-line question in a separate Opus session. A lighter model is enough for questions like this.

プロンプトは軽く保っています。報告のプロンプトには各セッションの短い要約だけが入り、エージェントは要約で足りないときにだけ履歴ファイルを開きます。各プロンプトの中身は[ガイド](docs/guide.md#reviews-and-reports-with-your-agent)にあります。

## プライバシー

- 履歴はパソコンの外に出ません。テレメトリも、アカウントも、AI サービスもありません。ネットワークを使うのは、kiroku 自身の新しい版を GitHub で確かめてダウンロードするときだけです。
- kiroku が履歴から書くファイルは、あなたしか読めません。`kiroku serve` は `127.0.0.1` で待ち受け、鍵を持つブラウザにだけ開きます。
- HTML には、依頼・AI の返事・ファイルのパス・コミットメッセージがそのまま入っています。人に渡す前に中身を確かめてください。
- 振り返りと報告のプロンプトは、パソコン上の AI エージェントに履歴ファイルの場所を教え、エージェントはそれを読みます。履歴を任せられるエージェントを使ってください。

詳しいことと、リリースの確かめ方は [SECURITY.md](SECURITY.md) にあります。

## 使い方

```bash
kiroku serve              # http://localhost:8484/ の画面。新しい履歴が入るたびに更新
kiroku open               # 別のブラウザで開く（最初に一度 serve の鍵が要る）
kiroku autostart on       # ログインのたびに kiroku serve を起動（macOS と Linux）
kiroku stats --week last  # 先週の時間・トークン・費用・コミットを、エージェント・モデル・プロジェクト別に端末に 1 画面で
kiroku html --week last   # 先週だけを 1 つの HTML に書き出して、人に見せる
kiroku update             # 最新の版に更新
kiroku help               # すべてのコマンド（オプションは "kiroku <command> --help"）
```

画面の見方、すべての指標、読む履歴、すべてのオプション、[アンインストールの方法](docs/guide.md#uninstall)は[ガイド](docs/guide.md)に、版の番号が約束することは[互換性](docs/compatibility.md)にあります（どちらも英語）。

## 貢献

kiroku が役に立ったら、⭐ を付けてもらえると、ほかの人が見つけやすくなります。Issue とプルリクエストは英語でも日本語でも歓迎します。[CONTRIBUTING.md](CONTRIBUTING.md) を読んでください。脆弱性は [SECURITY.md](SECURITY.md) の方法で、非公開で知らせてください。

## ライセンス

[MIT](LICENSE)
