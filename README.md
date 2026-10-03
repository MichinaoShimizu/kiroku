# kiroku

AI エージェント（Claude Code・Kiro・Amazon Q・Codex）の利用状況を、Google カレンダーのような画面で、あなただけに見せるツールです。

限られたクレジットやトークンで AI エージェントを効果的に使えるかは、使う側の改善しだいです。けれど、改善に必要な情報を手に入れられるかどうかも、今は人しだいです。利用状況は各エージェントが手元の PC に記録していますが、場所も形式もばらばらです。kiroku はそれを読み集めて 1 つの画面にまとめ、誰でも自分の使い方を見て改善しやすくします。

- **外部 API を呼びません**。履歴はどこにも送りません
- **kiroku 自身は AI を呼びません**
- **書き出すのは静的な HTML ファイル 1 つだけ**です（`kiroku serve` も、それを自分の PC で開くだけ）

ネットにつながるのは、自分で実行する `install.sh` と `kiroku update`（GitHub Releases から kiroku 自身を落とす）のときだけです。

![kiroku の画面（ダミーデータ）](docs/screenshot.png)

いつ・どのプロジェクトで・何を頼んだかを週 / 月のカレンダーで、プロジェクトごと・日ごとの作業時間・トークン・目安コスト・クレジットをサマリーで見られます。

## インストール

macOS・Linux:

```bash
curl -fsSL https://raw.githubusercontent.com/MichinaoShimizu/kiroku/main/install.sh | sh
```

Windows は [Releases](https://github.com/MichinaoShimizu/kiroku/releases) から zip を落として展開してください。Go があれば `go install github.com/MichinaoShimizu/kiroku@latest` でも入ります。

## アップデート

```bash
kiroku update          # 最新版に入れかえる（--check で確かめるだけ）
```

`/usr/local/bin` など書き込めない場所なら `sudo kiroku update`。`go install` で入れた場合は `go install …@latest` で更新してください。

## 使い方

```bash
kiroku serve    # 画面を開く（http://localhost:8484/）。作業中に増えた履歴もその場で反映
kiroku html     # 静的な HTML（kiroku.html）を書き出して開く
kiroku help     # コマンドとオプションの一覧
```

書き出した HTML には依頼文やファイルパスがそのまま入ります。人に渡すときは中身を確かめてください。

画面の見方・指標の意味・読む履歴・オプションは [docs/guide.md](docs/guide.md)、開発は [docs/development.md](docs/development.md) にあります。

## ライセンス

[MIT](LICENSE)
