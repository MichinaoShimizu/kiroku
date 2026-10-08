---
name: drop-agent
description: "kiroku が読むエージェントのうち 1 つ（例: Amazon Q、Kiro IDE (legacy)、Codex）のサポートをやめる。読み取りの登録・エージェント固有のコード・画面の表・テストデータ・ドキュメントを消し、オプション・環境変数・--sources の名前・kiroku archive の写しは互換のために残して、1 つの PR にする。エージェントが提供を終えたとき、履歴の形式が読めなくなったとき、保守をやめると決めたときに使う。引数でエージェントを指定する（画面に出る名前か --sources の名前）。"
---

# エージェントのサポートをやめる（drop-agent）

kiroku のエージェントへの対応は、読み取り（`internal/source/<agent>.go`）だけでなく、CLI・archive・集計の表・画面・テスト・ドキュメントに散らばっている。この作業では、それを漏れなく消しつつ、`docs/compatibility.md` の約束を破らないようにする。

CLAUDE.md の「Security」を常に守る。利用者のファイル（履歴、`kiroku archive` の写し）は消さない。

## 0. 決める（ユーザーに確かめる）

始める前に、次をユーザーに見せて決めてもらう。決まるまでコードは変えない。

- **何をやめるか**: 画面に出る名前（`Source.Name()`）と `--sources` の名前（`Family()`）を、`internal/source/source.go` の `All` から特定する。1 つの `Family` に複数の `Name` があることがある（`kiro` = Kiro IDE・Kiro CLI・Kiro Crew・Kiro CLI (SQLite)・Kiro IDE (legacy)）。`Family` の一部だけをやめるのか、全部をやめるのかをはっきりさせる
- **共有しているコード**: 同じ実装を使うものがあれば、そのものは残す（例: `QStore` は Kiro CLI (SQLite) と Amazon Q が共有、Kiro Crew は `KiroCLI` の中で読む、Kiro IDE (legacy) は `kiro.go` の中）。やめるものだけに使われている分岐・関数・定数だけを消す
- **その結果できなくなること**: やめたエージェントのセッションは画面と `kiroku json` から消え、過去の週の数字も変わる。`docs/compatibility.md` の「Which histories are read」により、マイナーリリースで変えてよいが、CHANGELOG に書く
- **golden に入っているか**: `internal/cli/load_test.go` の `python`（Claude Code・Kiro IDE・Kiro CLI・Kiro IDE (legacy)）に入っているエージェントをやめる場合、`testdata/golden.json` は Python 版の数字なので作り直せない。比べ方をどう変えるか（そのエージェントのセッションを比べる対象から外すなど）をユーザーと決める

## 1. 消し残しの一覧を作る

エージェントの名前・`Family`・オプション名・環境変数・ファイル名で、リポジトリ全体を検索し、当たった場所をすべて一覧にする。

```bash
grep -rniE '<Name>|<family>|<option>|<ENV>|<adapter file>' --exclude-dir=.git . | grep -v '^./CHANGELOG.md'
```

既知の場所（ここにない場所が見つかったら、このファイルにも足す）:

| 場所 | 何があるか | どうする |
|---|---|---|
| `internal/core/agents.go` の `Agents` | 名前、`--sources` の名前、画面の色と頭文字、環境変数、`kiroku archive` のフォルダ。`--sources` の既定値と説明、`autostartEnv`、`keptDirs`、画面の色と頭文字（`__AGENTS__`）はここから作る | 行を消す。ただし 2・3 で残す環境変数とフォルダは、`autostartEnv`・`keptDirs` に名前を書き足して残す（表から消すとそこからも消えるため）。`TestAgentsDerived` の期待値も直す |
| `internal/source/source.go` の `All`・`Options`・`Default…` | 登録、保存場所の設定 | 登録を消す。`Options` のフィールドはオプションを残すなら残してよい |
| `internal/source/<agent>.go`・`watch.go` | 読み取り、監視の場所 | やめるものだけのコードを消す（0 の「共有しているコード」） |
| `internal/core/records.go` の `Records` | 履歴が何を記録するか | 消す（`TestRecordsCoverEverySource`） |
| `internal/core/native.go` | エージェント固有の指標 | 消す |
| `internal/core/usage.go` など | そのエージェントだけが使う料金・ルール（例: Codex → `OpenAIPrices`・`OpenAILongPrices`、Kiro → クレジット） | ほかに使うエージェントがなければ消す。料金なら `docs/upstream/*.md`・`tools/prices`・`.github/workflows/prices.yml`・`TestPricesMatchUpstream` もそろえる。消すかどうかはユーザーに確かめる |
| `internal/cli/cli.go` | オプション、help の文、「no history found」のエラー文（`--sources` の既定値と説明は `core.Agents` から作る） | help の文から外す。オプションは 2 のとおり残す |
| `internal/cli/doctor.go` | 見つかったものの説明 | エージェント固有の文を消す |
| `internal/report/` | エージェント固有の集計（例: Crew の裏方の処理） | やめるものだけのものを消す |
| `internal/web/js/*.js`・`template.html` | 説明文の中のエージェント名（例: 「Claude Code and Codex」） | 文を直す。`help_test.go` などが説明を確かめている |
| `testdata/` | 合成の履歴 | 消す。ほかのエージェントのテストが使っていないか確かめる |
| `internal/source/<agent>_test.go`・`fuzz_test.go` | テスト、fuzz | やめるものだけのテストと `Fuzz…` を消す（`fuzz.yml` は `go test -list` で拾うので変えなくてよい） |
| `internal/cli/snapshot_test.go`・`testdata/snapshot.json` | 全エージェントの出力 | 読み込みから外し、作り直す（5） |
| `internal/cli/load_test.go` の `python` | golden の対象 | 0 で決めたとおり |
| `tools/screenshots/gen.py` など | 画面のダミーデータ、スクリーンショット | ダミーデータから外す。README の画像が古くなるなら撮り直す（`docs/development.md` の手順） |
| `README.md`・`docs/guide.md`（Histories read・History retention・What each agent records・Agent-specific metrics・Options）・`docs/sources.md`（そのエージェントの節と「What history records」の列）・`docs/compatibility.md`・`docs/usability.md`・`SECURITY.md` | 説明 | 消す、または「サポートをやめた」と書く。`TestGuideRecordsTable` が表を確かめている |
| `.claude/skills/source-audit/SKILL.md`・`.claude/skills/price-update/SKILL.md`・`.claude/agents/hostile-tester.md` | 点検の対象 | 外す |

## 2. 互換のために残すもの

`docs/compatibility.md` で、オプションと環境変数は v1 以降、メジャーバージョンの中で残すと約束している。v1 より前でも、急に消して利用者のスクリプトや自動起動の設定を壊さないよう、次のようにする。

- **オプション**（例: `--codex-home`）: 受け付けたまま何もしない。使われたら「<agent> のサポートはやめた（vX.Y.Z）。このオプションは何もしない」と標準エラーに 1 回だけ出す。help の説明もそう書く
- **環境変数**（例: `CODEX_HOME`）: 読まなくなるだけで、エラーにはしない。`core.Agents` の行を消すと `autostartEnv` からも消える。既存の自動起動の設定を作り直しても困らないかで決め、残すなら `autostartEnv` に名前を書き足す
- **`--sources` の名前**（例: `--sources codex`）: 今のコードは知らない名前を黙って無視するので、やめた名前には同じ警告を出す。ほかに読むものがなければ、今の「no history found」のエラーの前にその警告が出るようにする
- これらにテストを足す（警告が出ること、終了コードが 0 のままであること）

`Family` の一部だけをやめる場合は、その `Family` の名前は残るので警告は要らない。やめたものだけが使うオプションに警告を出す。

## 3. `kiroku archive` の写し

`docs/compatibility.md` で、前の版が残した写しを後の版も扱えると約束している。

- 写しは消さない。`core.Agents` の行を消すと `keptDirs` からフォルダ名も消えるので、`keptDirs`（`internal/cli/archive.go`）に名前を書き足して残し、`kiroku archive off` がその写しを今までどおり消せるようにする
- やめたエージェントの写しがあるときは、`kiroku archive`（引数なし）と `kiroku doctor` で「<agent> の写しが N 件ある。kiroku はもう読まない。要らなければ `kiroku archive off` で消せる」と知らせる
- `testdata/compat/` の古い版の写しは消さず、`TestCompatArchive` が通ることを確かめる（読まないエージェントの写しを、エラーにせず飛ばすこと）

## 4. 消す

1 の一覧の順に消す。消すたびに `go build ./...` と `go vet ./...` を通し、使われなくなった関数や定数が残っていないかを `staticcheck` で確かめる（`docs/development.md` のコマンド）。

## 5. テストデータ

```bash
go test ./...
go test -run TestSnapshot -update ./internal/cli
git diff --stat testdata/snapshot.json
```

snapshot の違いが「やめたエージェントのセッション・データソースの行がなくなり、それを含んでいた週と月の合計が減った」だけで説明できることを確かめる。ほかのエージェントの数字が変わっていたら、消しすぎているので止めて調べる。

## 6. 消し残しの確認

1 の検索をもう一度実行する。残っていてよいのは次だけで、それぞれ PR の本文に理由を書く。

- 2 で残したオプション・環境変数・`--sources` の名前と、その警告とテスト
- 3 の `keptDirs` と写しの知らせ、`testdata/compat/`
- `CHANGELOG.md` の過去の項目
- 共有しているコードの中の、残すエージェントのための記述

## 7. CHANGELOG と PR

- `CHANGELOG.md` の `## Unreleased` の `### Removed` に、やめたエージェント、その理由、残したもの（オプションは警告を出して何もしない、archive の写しは消さず `kiroku archive off` で消せる）を書く
- `gofmt -l .`・`go vet ./...`・`go test ./...`・staticcheck を通す
- 差分を自分で読み直す。特に、ほかのエージェントの読み取り・色・数字を変えていないか、利用者のファイルを消すコードを足していないか
- push して PR を作り、`subscribe_pr_activity` で見守る。タイトルは英語で（例: `Stop reading Amazon Q history`）。本文に、0 で決めたこと、snapshot の違いの説明、6 で残したものとその理由を書く

## やらないこと

- 利用者の履歴や `kiroku archive` の写しを消すコードを足さない
- オプション・環境変数・`--sources` の名前を、エラーになる形で消さない
- golden を作り直さない
- ほかのエージェントのテストを消したり飛ばしたりして通さない
