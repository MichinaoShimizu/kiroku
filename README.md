# kiroku

[![CI](https://github.com/MichinaoShimizu/kiroku/actions/workflows/ci.yml/badge.svg)](https://github.com/MichinaoShimizu/kiroku/actions/workflows/ci.yml) [![Release](https://img.shields.io/github/v/release/MichinaoShimizu/kiroku)](https://github.com/MichinaoShimizu/kiroku/releases) [![License: MIT](https://img.shields.io/badge/license-MIT-blue)](LICENSE) [![CodeQL](https://github.com/MichinaoShimizu/kiroku/actions/workflows/codeql.yml/badge.svg)](https://github.com/MichinaoShimizu/kiroku/actions/workflows/codeql.yml) [![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/MichinaoShimizu/kiroku/badge)](https://scorecard.dev/viewer/?uri=github.com/MichinaoShimizu/kiroku) [![Go](https://img.shields.io/github/go-mod/go-version/MichinaoShimizu/kiroku)](go.mod) [![Downloads](https://img.shields.io/github/downloads/MichinaoShimizu/kiroku/total)](https://github.com/MichinaoShimizu/kiroku/releases)

**AI と積み重ねた仕事を、記録から価値へ。**

Claude Code・Kiro（IDE・CLI・Kiro Crew）・Codex CLI のローカル履歴を 1 つのカレンダーにまとめ、何を作りどう働いたかを振り返れます。アカウント・テレメトリ・アップロードは不要で、kiroku が AI エージェントを動かすこともありません。

**[デモを試す →](https://michinaoshimizu.github.io/kiroku/)**（ダミーデータ。ブラウザーだけで動きます）

[![週のカレンダー、セッションの詳細、推定費用の内訳、検索、月の表示（ダミーデータ）](docs/demo.gif)](https://michinaoshimizu.github.io/kiroku/)

## インストール

```bash
curl -fsSL https://raw.githubusercontent.com/MichinaoShimizu/kiroku/main/install.sh | sh   # macOS と Linux
kiroku serve --check  # 見つかった履歴と、消されそうな履歴がないかを表示
kiroku serve          # http://localhost:8484/ で画面を開く
```

インストーラーはダウンロードを `checksums.txt`（[GitHub CLI](https://cli.github.com/) があればビルドの来歴も）で確かめます。Windows は [Releases](https://github.com/MichinaoShimizu/kiroku/releases) の zip を、Go があれば `go install github.com/MichinaoShimizu/kiroku@latest` を使えます（[ガイド](docs/guide.md#install)）。

> [!IMPORTANT]
> **Claude Code は既定で 30 日より古い会話を消します。** `~/.claude/settings.json` に `"cleanupPeriodDays": 3650` を設定するか（[公式](https://code.claude.com/docs/en/settings-reference#cleanupperioddays)）、`kiroku serve --archive on` で kiroku にコピーを残させてください（[詳しく](docs/guide.md#keep-a-copy-of-history-in-kiroku)）。

## できること

- **カレンダー**: すべてのセッションを週・月で並べ、日ごとの稼働時間・推定費用・Git コミットを添えます。
- **セッションを会話として読む**: プロンプトと AI の応答、その間のコマンド・コミット・PR・サブエージェントを時刻つきで並べます。
- **サマリー**: 投入（費用・credits・トークン・稼働時間）と出力（コミット・プッシュ・PR）、それぞれの単価を、自分のいつもの範囲と比べて見られます。量の流れはサンキー図で示します。
- **見直す候補**: 基準を超えた指標に印が付き、根拠のセッションと試すとよいことを示します。
- **検索**: プロンプト・編集したファイル・コミットを全期間から探せます。
- **振り返りはあなたのエージェントが書く**: 「プロンプトをコピー」で、履歴に基づく振り返りや日報のプロンプトを、いつものエージェントに貼れます。kiroku は何も送りません（[ガイド](docs/guide.md#reviews-and-reports-with-your-agent)）。

![週のカレンダー（ダミーデータ）](docs/screenshot.png)

### サマリーの主要なグラフ

サマリーは、期間の全体の流れから始まり、その下の 4 枚のパネルで自分のいつもと比べます。

**投入から出力の流れ**: 期間の量が、エージェント → プロジェクト → 出力（コミットか PR に届いた・ファイルを変えただけ・何も残らなかった・記録なし）へどう流れたかを、帯の太さで示すサンキーダイアグラムです。上の 1 文が、全体のうち何 % がどこへ行ったかを言い切ります。

![投入から出力の流れ（ダミーデータ）](docs/sum-sankey.png)

**投入と出力**: 左が投入（推定費用・credits・トークン・稼働時間から選ぶ）、右が出力（Git のコミット・プッシュ・PR から選ぶ）です。棒は 1 週（または 1 か月）ごとの量で、上に実際の数を書きます。濃い棒が表示中の期間、灰色の帯が自分のいつもの範囲（過去の期間の真ん中の半分）、点線がその中央値です。投入には左端からの累積の線が重なり、利用上限に当たった期間には赤い三角が付きます。

![投入と出力のパネル（ダミーデータ）](docs/sum-io.png)

**投入単価と出力単価**: 量を、使った時間・日・セッション・プロンプト（投入単価）や、コミット・プッシュ・PR・AI が書いた 100 行（出力単価）で割った値の推移です。いつもの範囲より下か上かを 1 行で言い、オレンジの線はここまでの平均です。数字のない組み合わせは選択肢に出ません。

![投入単価と出力単価のパネル（ダミーデータ）](docs/sum-unit.png)


<img src="docs/session.png" alt="セッションの詳細（ダミーデータ）" width="49%"> <img src="docs/worth.png" alt="見直す候補から開いた指標（ダミーデータ）" width="49%">

推定費用は公開 API の料金による目安で、記録される内容はエージェントによって違います（[エージェントごとに記録されること](docs/guide.md#what-each-agent-records)）。

## プライバシー

- 履歴は PC の外に出ません。ネットワークを使うのは、kiroku 自身の新しい版を GitHub で確かめてダウンロードするときだけです。
- kiroku が書くファイルはあなたしか読めません。`kiroku serve` は `127.0.0.1` で待ち受け、鍵を持つブラウザーにだけ開きます。
- HTML にはプロンプト・応答・パス・コミットメッセージがそのまま入ります。人に渡す前に確かめてください。
- コピーしたプロンプトを貼ったエージェントは履歴ファイルを読み、提供元に送ることがあります。履歴を任せられるエージェントを使ってください。

詳しくは [SECURITY.md](SECURITY.md) にあります。

## 使い方

```bash
kiroku serve                        # 画面を開き、新しい履歴が入るたびに更新
kiroku serve --open                 # 別のブラウザーで開く
kiroku serve --autostart on         # ログインのたびに kiroku serve を起動（macOS と Linux）
kiroku serve --archive on           # エージェントが消す履歴のコピーを残す
kiroku serve --html --week last     # 先週だけを 1 つの HTML に書き出す
kiroku update                       # 最新の版に更新
kiroku help                         # すべての使い方
```

画面の見方・指標・オプション・[アンインストール](docs/guide.md#uninstall)は[ガイド](docs/guide.md)、版の約束は[互換性](docs/compatibility.md)にあります（どちらも英語）。

## 貢献・ライセンス

Issue と PR は英語でも日本語でも歓迎します（[CONTRIBUTING.md](CONTRIBUTING.md)）。脆弱性は [SECURITY.md](SECURITY.md) の方法で非公開で知らせてください。ライセンスは [MIT](LICENSE) です。
