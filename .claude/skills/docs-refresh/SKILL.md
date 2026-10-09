---
name: docs-refresh
description: "README・docs/guide.md などのドキュメントとスクリーンショット（docs/*.png）が今の kiroku と合っているかを確かめ、古いものは撮り直し・書き直し、長すぎるところは意味を変えずに削って 1 つの PR にする。リリースの PR を作る前に必ず使う。画面や機能を変えた PR のあと、ドキュメントが古いと気づいたときにも使う。引数で shots（スクショだけ）か text（文章だけ）に絞れる。"
---

# ドキュメントとスクショの点検（docs-refresh）

`docs/development.md` の決まりでは、画面や機能を変えた PR が README・`docs/guide.md`・スクショを同じ PR で直すことになっている。それでも漏れは出る（#225 で撮ったスクショは、#227〜#248 の画面の変更を 1 枚も映していなかった）。この作業は、その漏れをリリースのたびにまとめて拾い、ドキュメントが長くなりすぎないようにも保つ。

CLAUDE.md の「Security」を常に守る。スクショと文章に載せるのはダミーデータ（`tools/screenshots/gen.py`・`mkgit.py`）だけで、手元の本物の履歴・パス・ユーザー名を撮ったり例に書いたりしない。

## 0. 範囲を決める

- 引数が `shots` なら 1 と 2 だけ、`text` なら 3 と 4 だけ、なければ全部
- 前回のリリースのタグを起点にする: `git describe --tags --abbrev=0`。CHANGELOG の `## Unreleased` と、そのタグから今までの変更が点検の手がかりになる
- 手順の元は `docs/development.md` の「Screenshots」と「Docs」の項目。食い違ったらそちらを正とし、このファイルも直す

## 1. スクショが古いかを確かめる

ダミーデータは乱数を固定しているが、日付を「今日」から作るので、撮るたびに中身が変わる。画像どうしをピクセルで比べても古さはわからない。古さは「最後に撮ったあとに画面が変わったか」で決める。

```bash
last=$(git log -1 --format=%H -- docs/screenshot.png)
git log --oneline "$last"..HEAD -- internal/web tools/screenshots
```

- 何も出なければ、スクショは新しい。2 を飛ばす
- 出たら、各コミットが README・guide に載っている画面（週のカレンダー、セッションの詳細、Worth a look、週のまとめ、日報・週報のボタン、Year in review）の見た目を変えたかを差分で確かめる。内部の整理だけで見た目が変わらないなら撮り直さない。迷ったら撮り直す

## 2. 撮り直して、目で確かめる

```bash
(cd tools/screenshots && npm ci --ignore-scripts && npx playwright install chromium)
sh tools/screenshots/run.sh
```

- Playwright の Chromium を取ってこられない環境（クラウドのコンテナなど）では、入っている Chromium を指定する: `CHROMIUM=/opt/pw-browsers/chromium sh tools/screenshots/run.sh`
- 撮る前の画像を一時ディレクトリに控え、撮ったあと 1 枚ずつ Read で開いて新旧を見比べる。確かめること:
  - 1 で見つけた変更が映っている
  - 崩れ・重なり・切れた文字・空の欄がない
  - 本物の履歴、手元のパス、ユーザー名が写っていない（`session.png` は右の列に一時パスが出るので左の列だけを撮る、という前提が保たれているか）
- `year.png` は Year in review が隠れているあいだ（`js/state.js` の `YEAR_ON`）は撮られない。変わっていなくて正しい
- 変わったのに理由を説明できない画像があれば、コミットせずに調べる

## 3. 文章が古いかを確かめる

対象は README.md、`docs/guide.md`、`docs/compatibility.md`、`docs/usability.md`、`docs/development.md`、SECURITY.md、CONTRIBUTING.md。`docs/sources.md` の「What history records」は `/source-audit` の担当なので、ここでは明らかな食い違いを報告するだけにする。

- CHANGELOG の `## Unreleased` と前回のタグからの各項目について、ユーザーに見える変更（新しい指標・ボタン・オプション・コマンド・既定値・名前の変更・消えたもの）がドキュメントに書かれているかを確かめる
- 逆向きにも確かめる: ドキュメントにある名前（ボタン・見出し・指標・オプション・環境変数）を `grep` で `internal/` から探し、今もあるか、同じ綴りかを見る。消えたものの説明が残っていたら消す
- `kiroku help` と各コマンドの `--help` の出力を、guide のコマンド表・オプション表と見比べる
- README は入口なので、主な機能の一覧が今の画面と合っているか（大きな機能が抜けていないか、なくなった機能を宣伝していないか）を見る
- 指標の説明は `HELP` と guide の「How to read the metrics」の表を `TestHelpMatchesGuide` が突き合わせている。表を直したら `HELP` も直す

## 4. 長すぎるところを削る

ドキュメントは足すばかりだと読まれなくなる。次の目安に当たるところを探して直す。

```bash
wc -c README.md docs/*.md
awk 'length>800{print FILENAME": "NR" ("length" chars)"}' README.md docs/*.md
```

- 同じことを 2 か所以上に書いている → 1 か所にまとめ、ほかはリンクにする（README は要点とリンク、詳しいことは guide）
- 1 つの段落や表のマスに条件・例外・数字を詰め込んでいる → 箇条書きか小さな表に分ける
- 実装の経緯や内部の事情（「以前は〜だった」「〜のために〜した」）→ CHANGELOG か `docs/development.md` に任せ、利用者向けの文書からは消す
- README は 1 画面で読み終わる長さを保つ。新しい機能を足すときは、既存の項目と合わせて短くする

削るときの決まり:

- 意味を変えない。機能・数字・しきい値・注意書き（特にプライバシーとセキュリティ）を消さない
- 削った・移した内容を PR の本文に箇条書きで残す
- 大きく組み替えたくなったら（節の入れ替え、ファイルを分けるなど）、案をユーザーに見せて決めてもらう。この作業では手を付けない

## 5. 確かめて PR にする

```bash
gofmt -l .
go vet ./...
go test ./...
```

- 文章の英語は、まわりの文体（短い文、同じものは同じ言葉で、推定は推定と書く）に合わせる
- リンク（`docs/guide.md#...` の見出しへのリンクを含む）が切れていないかを確かめる
- 差分を読み直す。消した文の中に、まだ正しくて必要なことがなかったか
- ドキュメントとスクショだけの変更なら CHANGELOG には書かない（#225 と同じ）
- ブランチにコミットし、push して PR を作る。タイトルは英語で（例: `Refresh the screenshots and bring the guide up to date`）。本文に、古かったもの（どの PR の変更が映っていなかったか）、撮り直した画像、直した文章、削った・移した内容を書く
- `subscribe_pr_activity` で見守る
- 何も古くなく、削るところもなければ、PR は作らずにそう伝える

## やらないこと

- 本物の履歴でスクショを撮らない
- 画面やコードを直さない。ドキュメントのほうが正しく画面が間違っているように見えたら、報告にとどめる
- 古さを確かめずに、念のためだけで撮り直した画像をコミットしない（1 で変更がなかったら撮らない）
- テストを消したり飛ばしたりして通さない
