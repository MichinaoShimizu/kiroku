---
name: quality-audit
description: "kiroku 全体を決まった観点で精査し、確かめた問題を直して PR にし、結果を docs/quality/audit-log.md に残す。これを定期的に回していれば一定の品質を保てる、という手順。月に 1 回（full）と週に 1 回（quick）、minor のリリースの前、エージェントの新しい版や料金の変更があったときに使う。引数で full（既定）・quick・観点の名前（security・ux・i18n・docs・data・prompts・code・arch・supply・perf）に絞れる。引数 check で点検と記録だけ（直さない）にできる。"
---

# 全体の精査（quality-audit）

PR ごとの点検（`/pre-pr`）は、その PR が変えたところしか見ない。変えていないところの古び（取り込み元の新しい版、料金、文言のずれ、時刻しだいで落ちるテスト、記録されないまま残る保留）は、誰かが全体を見ないと溜まる。この手順は、その全体を決まった観点で決まった順に見て、確かめた問題だけを直し、見たこと・直したこと・残したことを記録に残す。

前提として CLAUDE.md の「Security」と「Architecture decisions」を常に守る。精査で使うのはダミーデータ（`tools/screenshots/gen.py`・`testdata/`）だけで、本物の履歴を読んだり記録に書いたりしない。

食い違ったら CLAUDE.md・`docs/development.md`・各スキルを正とし、このファイルも直す。

## 品質の床（いつでも成り立っていること）

精査は、次がすべて成り立っているかを確かめ、崩れていれば最優先で戻す。

1. main の CI（3 OS のテスト・e2e・XSS・release-dry-run・CodeQL）が緑で、直近 10 回に原因のわからない失敗がない
2. `govulncheck` が kiroku の呼ぶコードに脆弱性を報告しない
3. 履歴・git・ネットワーク・AI の返事を HTML として入れる箇所がない。外へ出る通信は GitHub のリリースだけ（`kiroku serve` の「Analyze with AI」が動かすエージェントを除く。`--no-ai` で切れる）
4. 書き出すファイルは 0600・ディレクトリは 0700。`kiroku serve` は既定で 127.0.0.1・Host と鍵の確認つき
5. 読める各エージェントの最新版の履歴を、二重に数えず・取りこぼさずに読める（`docs/sources.md` の表が今の版に合っている）
6. 画面の文言に英語と日本語で食い違い・訳し漏れ・同じ説明の重複がない。主要なシナリオ（`docs/usability.md`）を en・ja の 3 つの幅で通せる
7. README・guide・SECURITY.md・スクショが今の画面と動きに合っている
8. 記録（`docs/quality/audit-log.md`）の「保留」に、期限か判断待ちの相手が書かれていない項目がない

## 0. 範囲と前回を確かめる

```bash
git fetch origin main && git checkout --detach origin/main
git describe --tags --abbrev=0                 # 前回のリリース
sed -n '1,80p' docs/quality/audit-log.md       # 前回の精査と、残した項目
```

- 引数がなければ full。quick は 1 と、2 のうち A・B・C だけ。観点の名前なら 1 とその観点だけ
- 前回の記録の「保留」を、今回の確かめる項目に必ず入れる（解決したか・まだか・期限を過ぎたか）
- 前回の精査からの変更（`git log <前回の commit>..origin/main --oneline`）と、取り込み元の新しい版を、各観点に渡す手がかりにする。新しい版は、開いている `upstream` ラベルの issue（週 1 回の Upstream ワークフローが立てる。`docs/upstream/agents.md` との差分）と、各 changelog で見る。`upstream` の issue があれば data 観点を必ず入れ、直した PR で `go run ./tools/upstream` の結果を控えに入れて issue を閉じる

## 1. 機械で確かめる（毎回・全部）

1 つずつ実行し、結果を記録の「ゲート」の表に写す。失敗はそのまま 3 の「確実」に入れる。

```bash
sh tools/check.sh                       # gofmt・vet・staticcheck・govulncheck・test・build・shellcheck
sh tools/check.sh --e2e                 # 画面の e2e（smoke.mjs）と XSS
go test -count=3 ./...                  # 3 回まわして、時刻や順番で落ちるテストを探す
go test -cover ./... | sort -k5         # カバレッジ（前回より大きく下がったパッケージを見る）
go test ./internal/cli -run '^$' -bench Large -benchtime 3x -benchmem   # 前回の記録と比べる（前回の精査の commit と比べるなら sh tools/bench-compare.sh <commit>）
```

- main の CI の直近の結果: `gh api "repos/MichinaoShimizu/kiroku/actions/runs?branch=main&per_page=20" --jq '.workflow_runs[] | "\(.name) \(.conclusion)"'`。失敗があれば原因を調べる（「flake」は原因ではない）
- Fuzz: `fuzz.yml` の最近の結果。新しい失敗の入力（`testdata/fuzz/`）がないか
- 再現ビルド: `sh tools/reproduce.sh <最新の版> linux amd64` が一致するか
- 開いている PR・issue・Dependabot: 1 週間以上動いていないものを記録に書く（閉じるかどうかは利用者が決める）

## 2. 観点ごとに精査する

エージェントは 1 つのメッセージで並べて起動する（一度に 4 つまで）。どれもリポジトリを変えない。渡すのは、観点・前回からの変更・前回の保留・報告の形（下）だけにして、結論を先に言わない。

報告の形（全観点で同じ）: 1 件ずつ「どこ（file:line か画面と操作）・何が起きるか・根拠（再現手順・出力・取り込み元の出典）・重さ（高・中・低）・確かさ（確認済み・疑い）・直し方」。確かめた結果問題がなかったことも、短く並べてもらう。

| 観点 | 担当 | 見ること |
|---|---|---|
| A. security | `hostile-tester` | 全体（履歴の読み取り・画面・serve・`/ai/run`・git・書き出し・update・install.sh）。前回からの変更を重点に。CLAUDE.md の「Security」の各項目 |
| B. ux | `user-tester` | `docs/usability.md` の全シナリオと、AI 分析のダイアログ。en・ja、1440・1000・390。キーボードと読み上げの名前 |
| C. i18n | general-purpose | 英語と日本語の言葉のずれ（同じものに違う名前、画面とドキュメントの違い）、訳し漏れ、`JA_TEXT` の使われていないキー、同じ説明の重複 |
| D. docs | `/docs-refresh text` の 3 の確かめる部分（直すのは 4 で） | README・README.ja・guide・SECURITY.md・development.md・CHANGELOG とコード（フラグ・既定値・環境変数・権限）の食い違い、リンク切れ、古いスクショ |
| E. data | `/source-audit` の調べる部分（エージェントごと。直す PR は 4 で） | 各エージェントの新しい版の履歴の形と、kiroku の読み取りの食い違い（二重計上・取りこぼし・モデルやトークンの取り違え）。`/price-update` で料金表の古さ |
| F. prompts | `/report-eval check` | 日報・週報・振り返りのプロンプトの正しさと長さ（前回の点数と比べる） |
| G. code | general-purpose | 時刻・乱数・順番・機械の速さに頼るテスト、使われていないコード、握りつぶしたエラー、テストのない分岐、`runtime.GOOS` で分かれるところ |
| H. arch | `/adr-guard check` | 前回からの変更が ADR に反していないか、Proposed のまま止まっている ADR、索引（`docs/adr/README.md`）の古さ |
| I. supply | general-purpose | ワークフローのアクションの SHA 固定と `permissions`、必須チェックの設定（e2e が必須か）、release と install.sh の検証、依存の更新 |
| J. perf | 自分で | 1 のベンチマークの前回との差（±15% を超えたら原因を調べる）、大きな履歴での `kiroku serve` のメモリ、画面の描画（1 MB の AI の返事など） |

- quick は A（前回からの変更だけ）・B（シナリオ 1 と 2）・C
- 観点の担当が使えない環境（スキルやエージェントがない）なら、同じことを自分で行い、そう記録する

## 3. 確かめて分ける

報告をそのまま直さない。1 件ずつ自分で再現するか、根拠を読み、次の 4 つに分ける。

- **確実**: 再現した、または根拠が明らか。直す
- **疑い**: 再現できない・根拠が弱い。もう一度確かめ、だめなら記録の「保留」に、確かめ方と一緒に書く
- **判断が要る**: 仕様・方針・見た目の好みに関わる（例: 文言を消す、表示の場所を変える、ADR に反する）。利用者に、選択肢と推奨を短く聞く。返事が来るまで直さない
- **見送り**: 直す価値が直すコストに見合わない。理由を記録に書く

同じ問題を複数の観点が報告したら 1 件にまとめる。

## 4. 直す（引数 check のときは飛ばす）

- 優先順: 品質の床の崩れ（特に govulncheck とセキュリティ）→ 高 → 中 → 低
- PR は領域ごとに分ける（セキュリティ・取り込み元のエージェントごと・画面と文言・ドキュメント）。1 つの PR を大きくしない
- 各 PR は `/pre-pr` を通す。直した不具合には、直す前に落ちることを確かめたテストを足す。セキュリティの修正は変更の断片 `changes/<短い名前>.security.md` に
- テストを消す・飛ばす・ゆるめることで通さない。機械の速さに頼るテストは、比（入力を 4 倍にしたときの時間など）で確かめる形に直す
- マージは CI（e2e を含む必須チェック）が緑になってから。順に入れ、衝突は両方の変更を残して解く（変更の断片は PR ごとに別のファイルなので、CHANGELOG ではぶつからない）

## 5. 記録する

`docs/quality/audit-log.md` の先頭に、今回の節を足す（形はファイルの先頭の雛形のとおり）。

- 日付・対象（commit と版）・範囲（full・quick・観点）
- ゲート: 1 の各項目の結果（数字はそのまま。カバレッジとベンチマークは前回との差も）
- 見つけたこと: 観点ごとに 確実・疑い・判断・見送り の件数と、直した PR の番号
- 保留: 1 件ずつ、内容・なぜ残すか・次に確かめる方法・期限か判断待ちの相手。前回の保留で解決したものは「解決（PR #n）」にして残す
- ループの成績: 前回からの数を「ループ」の表に 1 行足す（観点ごとの指摘の数・直した数・直るまでの日数の中央値・保留の数・期限切れの保留の数・前回の保留のうち解決した数）。増えている観点・期限切れが続く項目は、次回の重点にする
- 次回: 次に重点を置く観点（今回問題が多かった観点、ループの表で悪くなっている観点、取り込み元の新しい版が出そうなもの）

記録は `/pre-pr` を通した PR に入れる（ドキュメントだけの PR）。ユーザーに見える修正があり、利用者がリリースを望めば、`/docs-refresh` のあとリリースの PR を作る。

## 回す間隔

始まりは人か、次の知らせ: 開いている `upstream` の issue（取り込み元の新しい版。data 観点だけでよい）、`prices` の issue（`/price-update`）、main の CI の失敗、利用者からの報告。


- **full**: 月に 1 回と、minor のリリースの前
- **quick**: 週に 1 回
- **観点だけ**: 取り込み元の新しい版（data）、料金の変更（data）、画面の大きな変更（ux・i18n）、ワークフローの変更（supply）のとき

`create_trigger` で定期実行にするときは、毎回新しいセッションで動かし、プロンプトには「`/quality-audit quick`（または full）を実行し、判断が要るものは直さずに一覧で報告する」と書く。直す PR の作成とマージまで任せるかは、利用者が決める。

## やらないこと

- 本物の履歴を読まない・記録に書かない
- 確かめていない報告をそのまま直さない。推測で問題を作らない
- 判断が要るものを、利用者に聞かずに直さない
- 品質の床を、テストやチェックを外して保ったことにしない
