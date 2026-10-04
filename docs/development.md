# 開発

kiroku は Go 1 本で書いた、外部ライブラリの少ない CLI です（SQLite は `modernc.org/sqlite`、zstd は `klauspost/compress` を使い、cgo なしでビルドします）。

## ビルドとテスト

Go 1.23 以降が要ります。Node があると `internal/web/script_test.go` が `node --check` で画面のスクリプトの構文も確かめます（なければ省略）。スクリーンショットとデモには Python 3 と git、撮影にはさらに Node.js と Playwright が要ります。

```bash
go test ./...      # testdata/ の合成データで、集計が正解と一致するかを確かめる
go vet ./...
gofmt -l .         # 何も出なければ OK
go build .         # ./kiroku ができる（./kiroku serve で画面を開く）
```

テストには個人の履歴を使いません。`testdata/` はすべて合成データで、パスも `/Users/me` のような架空のものを使います。実際の履歴は、issue を含めてコミット・添付しません。

## 作り

| 場所 | 役目 |
|---|---|
| `cli.go` | サブコマンド（`serve`・`html`・`json`・`version`・`update`・`help`）とオプションの読み取り、前の書き方（`kiroku --serve` など） |
| `main.go` | 履歴の読み込み（重複を外す）、料金表の上書き |
| `update.go` | `kiroku update`（Releases から落として確かめ、自分自身を入れかえる） |
| `serve.go` | `kiroku serve`（履歴の変化を見張って読み直し、画面に配る） |
| `internal/source` | エージェントごとの履歴を読むアダプター。読み方の細かい決まりは [sources.md](sources.md) |
| `internal/core` | 共通のセッションの形（`Builder` → `Session`）、トークンと料金、エージェント別の参考指標 |
| `internal/report` | 週・月の集計（`Summarize`）、プロジェクト別のまとめ（`project.go`）、ブランチ・エージェントごとの配分（`share.go`） |
| `internal/gitlog` | セッションの作業場所の git リポジトリからコミットを読む（git がなければ飛ばす） |
| `internal/web` | `template.html` が画面。`web.go` が集計の JSON を埋め込んで 1 ファイルの HTML にする。`help_test.go`・`script_test.go` が画面の説明とスクリプトを確かめる |
| `testdata/` | 合成の履歴（`home/`・`codex/`・`crew/`・`sqlite/`）、`golden.json`、`mtimes.json` |
| `tools/` | `release-notes.sh`・`next-version.sh`（リリース）、`screenshots/`（ダミーデータ・デモ・スクリーンショット・画面の e2e） |
| `install.sh`・`.goreleaser.yaml` | インストーラーと、リリースのファイルの作り方 |

料金表は `internal/core/usage.go` の `Prices` です。更新したら `PricesAsOf`（画面に出る時点）も変えます。

### エージェントを足すとき

1. `internal/source` に `Source`（`Name` / `Family` / `Where` / `Load`）を実装して、`source.All` に加える
2. `Load` では、会話ごとに `core.Builder` を組み立てて `emit` する（時刻・依頼・ツール・モデル・トークン・クレジット）
3. 同じ会話がほかの場所にも残るなら `Builder.Key` をそろえる（先に読んだほうだけを使う）
4. エージェントだけが記録している数字は `Builder.Measure` で残し、`core.NativeDefs` に定義を足す
5. `kiroku serve` で見張る場所が `Where()` だけで足りなければ `Watch()` を実装する（`internal/source/watch.go`）
6. 履歴を自動で消すエージェントなら `Retainer`（`Retention()`）を、計測の状態に一言添えるなら `Detailer` を実装する
7. 新しい `Family` なら `cli.go` の `--sources` の既定値に加える。置き場所を変えられるようにするなら `source.Options`・`addCommon` のオプション・環境変数（`Default…`）を足す
8. 合成データを `testdata/` に置いてテストを書く（golden は Python 版の 4 つの履歴だけなので、新しいアダプターは `internal/source/<名前>_test.go` で確かめる）
9. ガイド（英・日）の「読み取る履歴」と「履歴の保存期間」、`docs/sources.md`、README の対応エージェント、`cli.go` のヘルプを更新する

集計（`internal/report`）と画面は共通のセッションの形だけを見るので、ふつうは触らなくて済みます。

## 正解データ（golden）

`testdata/golden.json` は集計の JSON です。Go に移す前の Python 版が同じ合成データ（`testdata/home`）から出した数字で、`TestMatchesPythonVersion`（`main_test.go`）が Go 版の数字と比べます。

- 比べるのは Python 版にあった 4 つの履歴（Claude Code・Kiro IDE・Kiro CLI・Kiro IDE（旧））だけで、時刻は Asia/Tokyo で区切ります。古い Kiro IDE はファイルの更新時刻を使うので、テストは `testdata/mtimes.json` から戻します
- JSON に項目を足したときは、`main_test.go` の `compare` の除外の一覧に加えます（加えないと、golden にない項目として失敗します）
- 集計を変えて数字が変わるときは、なぜ変わるのかを PR に書いてから golden を更新します

## CI

PR と main への push で、`.github/workflows/ci.yml` が次を走らせます。

- `test`（Ubuntu・macOS・Windows）: gofmt（Windows 以外）・vet・テスト・ビルド
- `release-dry-run`: `goreleaser release --snapshot`（公開はしない。`go mod tidy -diff` で go.mod の整理漏れも止まる）、CHANGELOG のいちばん上の節からのリリースノートの抜き出し、その節がまだタグのない版なら番号が `tools/next-version.sh` の結果と合うかの確認
- `e2e`: ダミーデータの HTML を Chromium で開き、`tools/screenshots/smoke.mjs` で大事な流れ（週の移動・セッションの詳細の開閉・週報の下書き・検索・月表示とショートカット）が動くか、横にはみ出さないか、スクリプトのエラーがないかを、日本語・英語・ダーク・1440px・1000px・390px で確かめる
- `install-script`（Ubuntu・macOS）: `install.sh` に shellcheck をかけ（Ubuntu のみ）、実際に最新のリリースを入れて `kiroku --version` を確かめる

ほかのワークフロー:

- `Tag`（`tag.yml`）: main で CHANGELOG.md が変わったとき（と手動）。下の「リリースの出し方」を参照
- `Release`（`release.yml`）: `v*` タグの push か、Tag からの呼び出し
- `Demo`（`pages.yml`）: main への push・毎週月曜（UTC 3:17）・手動で、ダミーデータの HTML を作って GitHub Pages に公開する（README の Live demo）。使うには Settings → Pages → Source を「GitHub Actions」にする。ダミーデータは日本時間で作るので、集計も日本時間（`TZ=Asia/Tokyo`）で区切り、画面は `KIROKU_DEMO` の目印があるとき、見る人の時間帯にかかわらず日本時間の時計で表示する（海外から開いても深夜の作業に見えないように）

## リリースの出し方

版の番号は、`## Unreleased` の中身からセマンティック バージョニングで決めます。`sh tools/next-version.sh` で次の番号がわかります。

| Unreleased の中身 | 上げる桁 |
|---|---|
| `BREAKING` と書いた変更（コマンド・オプション・出力ファイルなど、使い方の互換が崩れるもの。画面の見た目だけの変更は含めない） | 1.0 以降は major、0.x の間は minor |
| `### Added` がある | minor |
| それ以外（`### Changed`・`### Fixed`・`### Removed` など） | patch |

`CHANGELOG.md` の `## Unreleased` を `## v0.1.8 - 2026-10-05` のように、その番号に書き換える PR を作り、main にマージします（英語で書く。中身がそのままリリースノートになる）。マージすると、`.github/workflows/tag.yml` がいちばん上の節の版でタグを打ち、そのまま Release を動かします。タグを手で打つ必要はありません。

うまく動かなかったときは、Actions → Tag → Run workflow で動かし直せます。手でタグを打って push しても、これまでどおりリリースされます。

```bash
git tag v0.1.8
git push origin v0.1.8
```

変更を入れる PR では、利用者に見える変化を `## Unreleased` に英語で足しておきます（リリースの直後で見出しがなければ、いちばん上に `## Unreleased` から作ります）。CHANGELOG にタグと同じ版の節がないと、リリースは作られずに止まります（`sh tools/release-notes.sh v0.1.8` で手元でも確かめられます）。番号が中身と合わないときも、CI（release-dry-run）と Tag で止まります。

Release（`.github/workflows/release.yml`）は、3 OS でテストしてから、GoReleaser で macOS・Linux・Windows（amd64 / arm64）向けのファイルとチェックサムを作り、Releases に載せます。`v0.2.0-rc.1` のように `-` のつく版はプレリリースになります（`.goreleaser.yaml` の `prerelease: auto`）。番号の確認では `-` より前（`v0.2.0`）を `next-version.sh` の結果と比べます。

## 画面を変えるとき

画面（`internal/web/template.html`）を変えるときは、機能だけでなく情報設計・UI・UX も毎回見直します。

- **情報設計**：利用者が「何が起きたか → なぜ気にするか → 次に何をするか → どう確かめるか」の順にたどれるか。結論を先に書き、確度の低い数値は奥にしまう
- **言葉**：同じものは同じ用語で呼ぶ（例：「作業していた時間」）。推定の値には推定であることと基準を添え、良し悪しの判定にしない
- **UI**：日本語・英語（英語は文言が長くなりがち）、ライト・ダーク、デスクトップ（1440px）・狭い画面（1000px）・スマホ（390px）で崩れや重なりがないか
- **UX**：マウスを載せないと読めない情報をなくす（タッチ端末向け）。キーボードで操作でき、読み上げで意味が通るか
- **2 か国語**：文言は `tr(日本語, English)` で両方書く。指標の説明は `HELP` と `HELP_EN`（`TestHelpEnMatchesHelp` でそろっているかを確かめる）
- **動作**：`smoke.mjs` で大事な流れが動くかを確かめる（下の「画面の e2e」）。要素の id やキー操作を変えたら、`smoke.mjs` も合わせる
- **ドキュメント**：README（英・日）・`docs/guide.md` と `docs/guide.en.md`（指標の表は `TestHelpMatchesGuide`・`TestHelpEnMatchesGuide` で画面と照合）・スクリーンショットを同じ変更で更新する

## 画面の e2e

CI の `e2e` と同じことを手元で走らせるには、Node.js と Playwright が要ります。

```bash
(cd tools/screenshots && npm i --no-save playwright && npx playwright install chromium)
sh tools/screenshots/run.sh --html /tmp/kiroku.html
node tools/screenshots/smoke.mjs /tmp/kiroku.html   # 失敗した項目だけ FAIL と出て、終了コードが 1 になる
```

見るのは「動くか」だけです。わかりやすさや言葉は、[usability.md](usability.md) のシナリオで確かめます。

## スクリーンショット

README とガイドの画像（`docs/screenshot.png`・`docs/summary.png`）は、ダミーデータから撮り直せます。画面を変えたときは、あわせて更新してください。

```bash
cd tools/screenshots && npm i playwright && npx playwright install chromium && cd ../..
sh tools/screenshots/run.sh
```

`gen.py` が架空の 4 プロジェクト・約 5 週間の Claude Code の履歴を、`mkgit.py` がそれに合わせた git のリポジトリを一時ディレクトリに作ります。`capture.mjs` が先週の週カレンダーと週次サマリーを 1440x900（英語表示・ダークテーマ・Asia/Tokyo）で撮ります。日本語のブラウザでは画面は日本語で表示されますが、README（英語・日本語）とガイドでは同じ英語の画像を使います。

`sh tools/screenshots/run.sh --html <出力先.html>` は、ダミーデータの HTML だけを作ります（Node は不要）。`KIROKU_DEMO=1` がつき、どの時間帯から開いても日本時間の時計で表示します。一時ディレクトリの git リポジトリは消えるので、コミットのリンクは開けません。

## 利用者目線のテスト

画面を変えたら、[docs/usability.md](usability.md) のシナリオで、使う側から目的を達成できるかを確かめます。Claude Code では `user-tester` エージェント（`.claude/agents/user-tester.md`）が、このシナリオに沿ってテストします。
