# 開発

kiroku は Go 1 本で書いた、外部ライブラリの少ない CLI です（SQLite は `modernc.org/sqlite`、zstd は `klauspost/compress` を使い、cgo なしでビルドします）。

## ビルドとテスト

```bash
go test ./...      # testdata/ の合成データで、集計と Markdown が正解と一致するかを確かめる
go vet ./...
gofmt -l .         # 何も出なければ OK
go build .         # ./kiroku ができる
```

テストには個人の履歴を使いません。`testdata/` はすべて合成データです。

## 作り

| 場所 | 役目 |
|---|---|
| `main.go` | オプションの読み取り、履歴の読み込み、HTML / Markdown / JSON の書き出し |
| `serve.go` | `--serve`（履歴の変化を見張って読み直し、画面に配る） |
| `internal/source` | エージェントごとの履歴を読むアダプター。読み方の細かい決まりは [sources.md](sources.md) |
| `internal/core` | 共通のセッションの形（`Builder` → `Session`）、トークンと料金、エージェント別の参考指標 |
| `internal/report` | 週・月の集計（`Summarize`）と、週次・月次サマリーの Markdown |
| `internal/web/template.html` | 画面。1 ファイルの HTML に集計の JSON を埋め込んで書き出す |

### エージェントを足すとき

1. `internal/source` に `Source`（`Name` / `Family` / `Where` / `Load`）を実装して、`source.All` に加える
2. `Load` では、会話ごとに `core.Builder` を組み立てて `emit` する（時刻・依頼・ツール・モデル・トークン・クレジット）
3. 同じ会話がほかの場所にも残るなら `Builder.Key` をそろえる（先に読んだほうだけを使う）
4. エージェントだけが記録している数字は `Builder.Measure` で残し、`core.NativeDefs` に定義を足す
5. `--serve` で見張る場所が `Where()` だけで足りなければ `Watch()` を実装する（`internal/source/watch.go`）
6. 合成データを `testdata/` に置いてテストを書く

集計（`internal/report`）と画面は共通のセッションの形だけを見るので、ふつうは触らなくて済みます。

## 正解データ（golden）

| ファイル | 中身 |
|---|---|
| `testdata/golden.json` | 集計の JSON。Go に移す前の Python 版が同じ合成データから出した数字で、Go 版はこれと同じ数字を出す（Go 版で足した項目は比べない） |
| `testdata/golden-week.md` / `golden-month.md` | 週次・月次サマリーの Markdown をそのまま保存したもの |

Markdown の書式を変えたときは、作り直して差分を目で確かめます。

```bash
KIROKU_UPDATE_GOLDEN=1 go test .
git diff testdata/
```

## CI

PR ごとに次を走らせます（`.github/workflows/ci.yml`）。

- `test`（Ubuntu・macOS・Windows）: gofmt・vet・テスト・ビルド
- `release-dry-run`: `goreleaser release --snapshot`（公開はしない）。リリースの設定が壊れていないかを、タグを打つ前に確かめる

## リリースの出し方

```bash
git tag v0.1.0
git push origin v0.1.0
```

`v` で始まるタグを push すると、GitHub Actions（`.github/workflows/release.yml`）が 3 OS でテストしてから、GoReleaser で macOS・Linux・Windows（amd64 / arm64）向けのファイルとチェックサムを作り、Releases に載せます。`v0.2.0-rc.1` のように `-` のつくタグはプレリリースになります。
