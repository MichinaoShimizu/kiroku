---
name: pre-pr
description: "PR を出す前の点検。tools/check.sh で CI と同じチェックを回し、diff をセキュリティの観点で読み、画面・履歴の読み取り・serve・git・書き出しを変えていれば hostile-tester と user-tester を並べて動かし、CHANGELOG・ドキュメント・ベンチマークの漏れを確かめてから PR を作る。コードを変えた PR を作る前に必ず使う。引数 check で点検だけ（PR は作らない）にできる。"
---

# PR の前の点検（pre-pr）

CLAUDE.md は「PR を出す前に diff をセキュリティの観点で確かめる」ことを求めている。CONTRIBUTING.md と `docs/development.md` には、CHANGELOG・ドキュメント・スクショ・ベンチマークの決まりがある。この作業は、それを毎回同じ順で漏れなく通し、CI を赤くする push を減らす。

食い違ったら CLAUDE.md・CONTRIBUTING.md・`docs/development.md` を正とし、このファイルも直す。

## 0. 範囲を決める

```bash
git fetch origin main
git diff --stat origin/main...HEAD
git status --short
```

- 点検するのは `origin/main` からの差分と、まだコミットしていない変更の両方
- 引数が `check` なら 1〜5 だけを行い、結果を報告して終わる（コミット・push・PR はしない）
- 変更がドキュメントだけなら、1 と 5 だけでよい（`/docs-refresh` の PR など）

## 1. CI と同じチェック

```bash
sh tools/check.sh          # gofmt・vet・staticcheck・govulncheck・test・build・shellcheck
sh tools/check.sh --e2e    # internal/web か tools/screenshots を変えたとき
```

- クラウドのセッションでは SessionStart フック（`.claude/hooks/session-start.sh`）が道具と `CHROMIUM` を用意している。手元で Playwright がないときは `(cd tools/screenshots && npm ci --ignore-scripts && npx playwright install chromium)`
- 失敗したら直してから先に進む。テストを消す・飛ばす・ゆるめることで通さない
- govulncheck が kiroku の呼ぶコードの脆弱性を報告したら、CLAUDE.md の決まりどおり、ほかの作業より先に Go のツールチェーンかモジュールを上げて直す（別の PR にしてよい。この PR はその後にする）
- Windows と macOS は CI でしか試せない。パス・改行・ファイルの権限・シグナルを扱うところを変えたら、`runtime.GOOS` で分かれているコードと Windows 用のテストを読み、PR の本文に「CI の Windows・macOS で確かめる」と書く

## 2. diff をセキュリティの観点で読む

`git diff origin/main...HEAD` を、変えたファイルごとに読む。CLAUDE.md の「Security」の各項目について、当てはまるところを確かめる。

| 変えたところ | 確かめること |
|---|---|
| `internal/source`（履歴の読み取り） | 壊れた・巨大な・悪意のある入力で panic しない、際限なくメモリを使わない、履歴の中のパスをそのまま開かない。新しい形式には `testdata/` の合成データと（あれば）Fuzz の種を足したか |
| `internal/web`（画面） | 履歴・git・ネットワーク由来の文字列を `innerHTML` などで HTML として入れていない（`textContent` か既存のエスケープを通す）。`href` に `javascript:` が入らない。Content-Security-Policy をゆるめていない。外部への読み込みを足していない |
| `kiroku serve` | 既定は 127.0.0.1 のまま、Host の確認と鍵の確認が残っている。返すファイルのパスを検証している。広げる変更はオプトインで、SECURITY.md に書いたか |
| git の呼び出し | 引数はシェルを通さず `exec` に配列で渡す。リポジトリの設定（`core.fsmonitor`・`core.pager` など）からコマンドが動かない |
| 書き出すファイル | 履歴を含むファイルは 0600、ディレクトリは 0700。一時ファイルも同じ |
| `kiroku update`・`install.sh` | 置き換える前に checksums.txt（と、あれば出どころの証明）で確かめる。ネットワークに出るのは GitHub のリリースだけ |
| 依存・ネットワーク | 履歴やそこから作ったデータを外に送る処理を足していない。新しい依存は本当に要るか |
| `.github/workflows` | サードパーティのアクションは SHA で固定、ジョブごとに最小の `permissions`、書き込みトークンで信頼できないコードを動かさない |

- 問題があれば直して 1 からやり直す
- セキュリティの修正なら、それを確かめるテストを足し、CHANGELOG の `### Security` に書く

## 3. テスターに攻めてもらう（当てはまるときだけ）

履歴の読み取り・画面・serve・git・書き出しのどれかを変えたら、2 つのエージェントを 1 つのメッセージで並べて起動する。変えたファイルと、何を変えたかを渡す。

- **hostile-tester**: 変えたところに絞って攻めてもらう（例: 「`internal/source/codex.go` の新しい形式の読み取りと、画面の新しい欄に出す文字列」）
- **user-tester**: 画面を変えたときだけ。変えた画面と、関係する `docs/usability.md` のシナリオを伝える

報告のうち「確認済み」の問題は直してから進む。「疑い」は自分で確かめ、本物なら直す。直さないと決めたものは理由を PR の本文に書く。

## 4. 書き忘れを確かめる

- **CHANGELOG**: ユーザーに見える変更は `## Unreleased` の `### Added`・`### Changed`・`### Fixed`・`### Security` などに英語で書いたか。コマンド・オプション・出力ファイルを壊す変更は `BREAKING` と書いたか（`sh tools/next-version.sh` で次の版が意図どおりか見る）。内部だけの変更・テスト・ドキュメントだけなら書かない
- **ドキュメント**: 画面・コマンド・オプション・指標を変えたら、README・`docs/guide.md` を同じ PR で直したか。指標の説明を変えたら `HELP` と guide の表の両方か（`TestHelpMatchesGuide`）
- **スクショ**: README と guide に載っている画面の見た目を変えたら、撮り直したか（`sh tools/screenshots/run.sh`。手順は `/docs-refresh` の 2）
- **ベンチマーク**: 読み取り・集計・描画を変えたら、変更の前と後で `go test ./internal/cli -run '^$' -bench Large -benchtime 3x -benchmem` を回し、両方の結果を PR の本文に載せる
- **データ**: `testdata/` やスクショに本物の履歴・手元のパス・ユーザー名が入っていない（`/Users/me` のような作り物だけ）

## 5. diff を読み直す

- 自分の diff を、CI やレビューで落とされる理由を探すつもりで読む。デバッグの出力、消し忘れたコード、関係のない変更が混ざっていないか
- コメントとまわりのコードの書き方（日本語のコメント、命名、エラーの扱い）が合っているか
- 1 つの PR に 1 つの目的。関係のない直しは別の PR にする

## 6. PR を作る

- ブランチにコミットし（メッセージは英語）、`git push -u origin <branch>` で push する
- `.github/pull_request_template.md` があればその見出しに沿って、なければ次の内容で本文を英語で書く: 何をなぜ変えたか、どう確かめたか（`tools/check.sh` の結果、e2e、テスターの結果、ベンチマーク）、セキュリティの観点で確かめたこと、残した「疑い」とその理由
- PR を作ったら `subscribe_pr_activity` で見守る

## やらないこと

- チェックが赤いまま PR を作らない（Windows・macOS のように手元で試せないものは、そう書く）
- テストを消したり飛ばしたりして通さない
- 本物の履歴を読んだり、テストやスクショや PR の本文に載せたりしない
- テスターの「確認済み」の問題を、黙って残さない
