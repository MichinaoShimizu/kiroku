# 開発

kiroku は Go 1 本で書いた、外部ライブラリの少ない CLI です（SQLite は `modernc.org/sqlite`、zstd は `klauspost/compress` を使い、cgo なしでビルドします）。

## ビルドとテスト

```bash
go test ./...      # testdata/ の合成データで、集計が正解と一致するかを確かめる
go vet ./...
gofmt -l .         # 何も出なければ OK
go build .         # ./kiroku ができる（./kiroku serve で画面を開く）
```

テストには個人の履歴を使いません。`testdata/` はすべて合成データです。

## 作り

| 場所 | 役目 |
|---|---|
| `cli.go` | サブコマンド（`serve`・`html`・`json`・`version`・`update`）とオプションの読み取り |
| `main.go` | 履歴の読み込み（重複を外す）、料金表の上書き |
| `update.go` | `kiroku update`（Releases から落として確かめ、自分自身を入れかえる） |
| `serve.go` | `kiroku serve`（履歴の変化を見張って読み直し、画面に配る） |
| `internal/source` | エージェントごとの履歴を読むアダプター。読み方の細かい決まりは [sources.md](sources.md) |
| `internal/core` | 共通のセッションの形（`Builder` → `Session`）、トークンと料金、エージェント別の参考指標 |
| `internal/report` | 週・月の集計（`Summarize`）とプロジェクト別のまとめ |
| `internal/web/template.html` | 画面。1 ファイルの HTML に集計の JSON を埋め込んで書き出す |

### エージェントを足すとき

1. `internal/source` に `Source`（`Name` / `Family` / `Where` / `Load`）を実装して、`source.All` に加える
2. `Load` では、会話ごとに `core.Builder` を組み立てて `emit` する（時刻・依頼・ツール・モデル・トークン・クレジット）
3. 同じ会話がほかの場所にも残るなら `Builder.Key` をそろえる（先に読んだほうだけを使う）
4. エージェントだけが記録している数字は `Builder.Measure` で残し、`core.NativeDefs` に定義を足す
5. `kiroku serve` で見張る場所が `Where()` だけで足りなければ `Watch()` を実装する（`internal/source/watch.go`）
6. 合成データを `testdata/` に置いてテストを書く

集計（`internal/report`）と画面は共通のセッションの形だけを見るので、ふつうは触らなくて済みます。

## 正解データ（golden）

`testdata/golden.json` は集計の JSON です。Go に移す前の Python 版が同じ合成データから出した数字で、Go 版はこれと同じ数字を出します（Go 版で足した項目は比べません）。集計を変えて数字が変わるときは、なぜ変わるのかを PR に書いてから更新します。

## CI

PR ごとに次を走らせます（`.github/workflows/ci.yml`）。

- `test`（Ubuntu・macOS・Windows）: gofmt・vet・テスト・ビルド
- `release-dry-run`: `goreleaser release --snapshot`（公開はしない）。リリースの設定が壊れていないかを、タグを打つ前に確かめる
- `install-script`（Ubuntu・macOS）: `install.sh` に shellcheck をかけ、実際に最新のリリースを入れて `kiroku --version` を確かめる

## リリースの出し方

```bash
git tag v0.1.0
git push origin v0.1.0
```

`v` で始まるタグを push すると、GitHub Actions（`.github/workflows/release.yml`）が 3 OS でテストしてから、GoReleaser で macOS・Linux・Windows（amd64 / arm64）向けのファイルとチェックサムを作り、Releases に載せます。`v0.2.0-rc.1` のように `-` のつくタグはプレリリースになります。

## 画面を変えるとき

画面（`internal/web/template.html`）を変えるときは、機能だけでなく情報設計・UI・UX も毎回見直します。

- **情報設計**：利用者が「何が起きたか → なぜ気にするか → 次に何をするか → どう確かめるか」の順にたどれるか。結論を先に書き、確度の低い数値は奥にしまう
- **言葉**：同じものは同じ用語で呼ぶ（例：「作業していた時間」）。推定の値には推定であることと基準を添え、良し悪しの判定にしない
- **UI**：ライト・ダーク、デスクトップ（1440px）・狭い画面（1000px）・スマホ（390px）で崩れや重なりがないか
- **UX**：マウスを載せないと読めない情報をなくす（タッチ端末向け）。キーボードで操作でき、読み上げで意味が通るか
- **ドキュメント**：README・`docs/guide.md`（指標の表は `TestHelpMatchesGuide` で画面と照合）・スクリーンショットを同じ変更で更新する

## スクリーンショット

README とガイドの画像（`docs/screenshot.png`・`docs/summary.png`）は、ダミーデータから撮り直せます。画面を変えたときは、あわせて更新してください。

```bash
cd tools/screenshots && npm i playwright && cd ../..
sh tools/screenshots/run.sh
```

`gen.py` が架空の 4 プロジェクト・約 5 週間の Claude Code の履歴を、`mkgit.py` がそれに合わせた git のリポジトリを一時ディレクトリに作ります。`capture.mjs` が先週の週カレンダーと週次サマリーを 1440x900（Asia/Tokyo・ライトテーマ）で撮ります。

## 利用者目線のテスト

画面を変えたら、[docs/usability.md](usability.md) のシナリオで、使う側から目的を達成できるかを確かめます。Claude Code では `user-tester` エージェント（`.claude/agents/user-tester.md`）が、このシナリオに沿ってテストします。
