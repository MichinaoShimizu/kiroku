# 手順 0〜3：範囲を決め、候補を読み、確かめ、振り分ける（decision-log）

## 0. 範囲と既存の記録

1. リポジトリ名（`git remote get-url origin` の末尾）、基準のコミット（`git rev-parse HEAD`）、範囲を決める。
2. 既存の Decision log の、`category` と `keywords` の行だけを読む（`grep -H -E '^(category|keywords):' <置き場所>/*.md`）。全文は、手順 1 で候補と重なったものだけ読む。索引の `README.md` は参考にとどめる。
3. 履歴の一覧を手元で 1 回で取る。`TZ=Asia/Tokyo git log --first-parent --date=short-local --format='@@ %h %ad %s' --name-only [-- <scope のパス>]`。`TZ=` は先頭にそのまま書き、`env` を付けない（`env TZ=…` は確認なしの許可に当たらない）。PR の一覧もこの git の履歴から作り、`gh pr list` は使わない。日付はすべて JST（日本時間）の YYYY-MM-DD で書く（ほかの時間帯を頼まれたら `TZ=<時間帯>` に替え、報告に書く。このときは確認が出る）。PR の `mergedAt` などは UTC なので、9 時間足して JST の日付にする（例：`gh pr view 101 --json mergedAt --jq '.mergedAt | fromdate + 32400 | strftime("%Y-%m-%d")'`）。メッセージの `(#123)` や `Merge pull request #123` で PR の番号に結びつける。マージコミットの件名はブランチ名なので（`Merge pull request #12 from x/fix-typo`）、`git log --first-parent --merges --format='@@ %h %b' [-- <scope のパス>]` も取り、本文の 1 行目を PR のタイトルとして読む。浅いクローンなら先に `git fetch --unshallow`。できなければ、見えた範囲を報告に書く。
4. マージされた変更を全部数え、総数とどこまで見たかを記録する。

## 1. 候補を見つける

タイトルと変更ファイルだけでは、名前と中身が違う変更（「fix typo」でタイムアウトを変えたもの）や、`settings.py`・`constants.ts` の中の値の変更を見落とす。次の 2 つを合わせて候補にする。

1. **一覧から**：タイトルと変更ファイルで、各変更を「分類」に当てはめる。手がかりはファイルの場所である（例：`migrations/` → `data`、`auth` → `security`、`openapi` → `compatibility`、`timeout`・`retry` → `reliability`、デプロイ・`monitoring`・`alerts` → `release`、`.github/workflows` のテストや lint・`.eslintrc` → `engineering`、`terraform` のインスタンスの種類・サンプリング → `cost`）。
2. **差分から**：全件の差分を手元の git で洗う。次の 3 つを別々に回し、当たったものを合わせる。`-i` は正規表現の全体に効くので、1 と 2 をつながない（つなぐと、2 があらゆる小文字の代入に当たる）。lockfile とスナップショットは、依存の更新が当たり続けるので外す。
   1. 言葉（大文字と小文字を区別しない）：`git log --first-parent -i -G '<言葉>' --format='%h' -- <scope のパス（なければ .）> ':(exclude)*lock*' ':(exclude)*.snap'`。`<言葉>` は、既定の言葉と、手順 0 で読んだ既存の Decision log の `keywords` をつないだもの。
      - 既定：`timeout|retr(y|ies)|limit|max_|ttl|expire|auth|permission|role|token|secret|encrypt|signature|hmac|verify|email|phone|schema|migrat|deprecat|feature_?flag|instance_type|sampling|sample_rate`
   2. 定数の値の変更（区別する）：1 と同じ形で、`-i` を付けずに回す。
      - 定数・設定の値の変更：`^[A-Z][A-Z0-9_]{2,} *[:=]`
   3. 設定ファイルの変更（中身は問わない）：`git log --first-parent --format='%h' -- '*.yml' '*.yaml' '*.toml' '*.ini' '*.properties' '*.env' ':(exclude)*lock*'`。scope があれば、その下に絞る（例：`'<scope>/*.yml'`）。小文字の設定（`retention_days: 90`）は 1 と 2 では拾えないので、これで拾う。

言葉の洗い出しがほぼ全件に当たったら（手順書や仕様の文書が中心のリポジトリなど）、並べる手がかりにしない。定数と設定ファイルの結果、一覧からの分類で並べ、そう報告に書く。

依存の更新、見た目の変更、ふつうのバグ修正は落とす。ただし、落とすのは 2 に当たらなかったものだけにする。タイトルだけで落とさない。

候補を、大事なもの、2 に当たったのにタイトルからは分からないもの、戻されたもの（`Revert`）、あいまいなもの、議論があったものの順に並べ、上から `depth` 件だけ本文とレビューをまとめて取る。`depth` の外に残った候補は、分類ごとの数と番号を報告に書く。

- GitHub：PR ごとに `gh pr view <番号> --json number,title,body,author,createdAt,mergedAt,mergedBy,url,reviews,comments,files`。コマンドを `;`・`&&`・`|` やループでつながない（確認なしで通るのは 1 つずつのコマンドだけ）。多いときは `gh api graphql` で 1 回に 20〜50 件取ってよいが、毎回確認が出る。`gh pr view` が失敗したら（GraphQL が止められている環境など）、REST API の GET で読む：`gh api repos/<owner>/<repo>/pulls/<番号>`、`…/pulls/<番号>/reviews`、`…/issues/<番号>/comments`。`-X`・`-f`・`-F`・`--input` は付けない。`gh` がなければ MCP ツール。
- GitLab：MR ごとに `glab mr view <番号>`。
- どちらも使えない：git の一覧と `git show` だけで進める。履歴の証拠はコミットメッセージだけになり、レビューは見ていないと書く。
- issue とリリースは、候補の本文から参照されているものだけを開く。

差分は先に `git show --stat <コミット>` を見て、当たるファイルだけ `git show <コミット> -- <パス>` で読む。lockfile、生成物、スナップショットは読まない。

一つの流れの変更は 1 つの Decision log にまとめ、別の判断は分ける。PR ごとに作らない。理由が分からない大事な変更も Decision log にし、理由は「不明」のまま問いを残す。推測は「推測：」と明記して書く。

## サブエージェントに分ける

読む候補が 5 件を超えたとき（SKILL.md の「手順」）、分類ごとに 1 つ、同時に呼ぶ。差分が小さい、自分で読んだほうが速い、は分けない理由にしない。

- `.kiro/agents/decision-log-reader.json` か `.kiro/agents/decision-log-reader.md`（Kiro）か `.claude/agents/decision-log-reader.md`（Claude Code）があるときだけ、名前で指定して呼ぶ。Claude Code のプラグインで入れたときは `decision-continuity:decision-log-reader` という名前で呼ぶ。例：「decision-log-reader サブエージェントで、reliability の PR #101、#117 を基準のコミット abc1234 で読んで」。
- ないとき、または名前で呼んで失敗したとき（Kiro は登録のない名前を既定に切り替えない）は、既定のサブエージェントにリポジトリ・基準のコミット・分類・PR の番号を渡し、「起きたこと」「理由」「関わった人」の行、決定の言葉、問い、keywords の候補を返させる。ファイルは書かせない。
- 返ってきたものの重なりを除き、自分で Decision log にまとめる。

## 2. 主張ごとに確かめる

主張ごとに 2 つの証拠を別々に持つ。**履歴**（日付のある PR、issue、差分、レビュー、コミット）と、**現在**（基準のコミットのコード、設定、ドキュメント）である。履歴のない主張は書かない。テストの成否は調べない。

| 状態 | 条件 |
|---|---|
| いまも同じ | 履歴の変更が、基準のコミットにもそのまま残っている |
| 置き換え済み | 後の変更で置き換わった |
| 競合 | 証拠どうしが矛盾する（履歴とコード、PR の本文と差分など） |
| 履歴のみ | 履歴はあるが、基準のコミットで該当するコードが見つからない |

**競合は報告の先頭に出す。**どちらが正しいかは決めない。

## 3. 振り分ける

| 見つけたもの | 行き先 |
|---|---|
| 変更は分かるが、決定の言葉も決めた人もない | 新しい Decision log |
| 既存の Decision log と同じ流れの、後の変更（戻した、広げた） | その Decision log への追記 |
| 決定の言葉と決めた人が PR やレビューにある | 報告の「決定の候補」と、Decision log の出典 |
| 大事な変更なのに理由が不明 | 「分かっていないこと」に、知っていそうな人への問いを書く |
| 証拠どうしが競合する | 報告の先頭 |

決定の候補では、決めた人を PR やレビューに書かれた役割で書き、なければ「未確認」とする。承認が見えなければ「承認は見えない」と書くが、候補からは外さない。Decision log を決定に書き換えない。
