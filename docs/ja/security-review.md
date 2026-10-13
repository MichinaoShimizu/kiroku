# kiroku 導入時のセキュリティ確認資料（情報システム部門向け）

この資料は、社内で kiroku の利用を認めるかどうかを判断する情報システム部門・セキュリティ担当の方向けに、kiroku が何を読み、何を書き、どこと通信するかをまとめたものです。記載はリポジトリのコード（`internal/…`）、[SECURITY.md](../../SECURITY.md)、[docs/guide.md](../guide.md)、[install.sh](../../install.sh)、`.github/workflows/` と照らし合わせて確認しています。英語の原文（SECURITY.md・guide.md）と食い違う場合は、英語の原文とコードを正としてください。

## 1. 概要

| 項目 | 内容 |
|---|---|
| 何をするか | 手元のコンピューターにある AI エージェント（Claude Code、Kiro、Amazon Q Developer CLI、Codex CLI）の履歴を読み、週・月のカレンダーや集計を HTML の画面で見せる |
| 形態 | Go で書かれた単一のバイナリ（`CGO_ENABLED=0`）。常駐サービスやインストーラーによるシステム変更はない（自動起動はユーザーが `kiroku autostart on` を実行したときだけ） |
| アカウント | 不要。ログインや外部サービスの登録はない |
| ライセンス | MIT（[LICENSE](../../LICENSE)） |
| ソース | https://github.com/MichinaoShimizu/kiroku （公開リポジトリ） |
| 対応 OS | macOS・Linux・Windows（amd64 / arm64）。`kiroku autostart` は Windows 未対応 |
| サポート | 最新リリースのみ（SECURITY.md） |

kiroku は既定では AI を呼び出しません。画面の「レビュー」「日報・週報・月報」ボタンは、利用者が自分で AI エージェントに貼り付けるためのプロンプトをクリップボードにコピーするだけです。例外は `kiroku serve --ai` で起動したときの「Run with AI」で、これは 6 章で説明します。

## 2. 読み取るデータ

kiroku が読むのは、各エージェントが利用者のホームディレクトリなどに保存している履歴です（`internal/source/`、[docs/guide.md の Histories read](../guide.md#histories-read)、[docs/sources.md](../sources.md)）。

| エージェント | 既定の場所 |
|---|---|
| Claude Code | `~/.claude/projects/*/*.jsonl`（`CLAUDE_CONFIG_DIR` があればその `projects/`） |
| Kiro IDE v1.0 以降 / Kiro CLI V3 | `~/.kiro/sessions/<hash>/sess_*/`（`KIRO_HOME` があればそこ） |
| Kiro CLI (Classic) | `~/.kiro/sessions/cli/` |
| Kiro IDE（v1.0 より前） | Kiro の `globalStorage/kiro.kiroagent/` 以下（macOS は `~/Library/Application Support/Kiro/User/globalStorage`、Windows は `%APPDATA%\Kiro\User\globalStorage`、Linux は `~/.config/Kiro/User/globalStorage` か `~/.kiro-server/data/User/globalStorage`） |
| Kiro Crew | `~/.kiro/crew/`（`KIROCREW_HOME` があればそこ） |
| Kiro CLI (SQLite) / Amazon Q Developer CLI | `data.sqlite3`（macOS は `~/Library/Application Support/<kiro-cli か amazon-q>/`、Linux は `~/.local/share/…`、Windows は `%LOCALAPPDATA%\…`）。読み取り専用（`mode=ro`）で開く（`internal/source/qstore.go`） |
| Codex CLI | `~/.codex/sessions/` と `archived_sessions/`（`CODEX_HOME` があればそこ） |

このほか、履歴の保存期間を表示するために Claude Code の設定ファイル（`~/.claude/settings.json` と組織の `managed-settings.json`）と Kiro Crew の `config.json` / `config.local.json` を読みます。

履歴には、利用者が書いたプロンプト、AI の返答、ツールの呼び出しと出力、ファイルのパス、コミットメッセージなどがそのまま含まれます。kiroku はそれを読み取るだけで、履歴そのものは書き換えません。

**git の読み取り**: 各セッションの作業フォルダが git リポジトリなら、`git log` と `git reflog` でコミットと push を読みます（`internal/gitlog/gitlog.go`）。他人が作ったリポジトリでも安全に読めるよう、次のように制限しています。

- システムの git 設定を読まない（`GIT_CONFIG_NOSYSTEM=1`）
- リポジトリの設定でプログラムが動くものを上書きする（`core.fsmonitor=false`、`core.hooksPath` を無効化、`core.pager=cat`、`git log` に `--no-ext-diff --no-textconv` など）
- 何も取得しない（`protocol.allow=never`、`GIT_ALLOW_PROTOCOL=none`、`GIT_NO_LAZY_FETCH=1`）。git がネットワークに出ることはない
- 入力を求めない（`GIT_TERMINAL_PROMPT=0`）、ロックを取らない（`GIT_OPTIONAL_LOCKS=0`）
- 利用者のグローバル設定（`~/.gitconfig`）は読む（`user.email` で本人のコミットを絞り込むため）
- Windows では、履歴にあるネットワーク（UNC）パスは読まない

**不正な履歴への対策**（SECURITY.md）: 通常のファイルしか開かない（名前付きパイプやデバイスは開かない）、64 MiB を超える行や JSON ファイルは読まない、zstd の展開は 1 GiB までに制限する、などです。

## 3. 書き出すファイルと権限

kiroku が書くファイルは次のとおりです。履歴を含むものは、本人だけが読める権限（ファイル `0600`、フォルダ `0700`）で作ります。

| ファイル | 場所 | 権限 | いつ書くか |
|---|---|---|---|
| HTML（`kiroku html`） | 既定はカレントディレクトリの `kiroku.html`（`-o` で変更） | `0600` | コマンドを実行したとき |
| JSON（`kiroku json`） | 既定は `kiroku.json`（`-o` で変更） | `0600` | 同上 |
| 履歴のコピー（`kiroku archive`） | Linux `~/.local/share/kiroku/archive`、macOS `~/Library/Application Support/kiroku/archive`、Windows `%LocalAppData%\kiroku\archive`（`--archive-dir` / `KIROKU_ARCHIVE_DIR` で変更） | フォルダ `0700`、ファイル `0600` | `kiroku archive on` を実行した後だけ。既定はオフ |
| `serve-key`（`kiroku serve` の鍵） | 設定フォルダ：Linux `~/.config/kiroku`、macOS `~/Library/Application Support/kiroku`、Windows `%AppData%\kiroku`（`KIROKU_CONFIG_DIR` で変更） | フォルダ `0700`、ファイル `0600`。他人が読める鍵ファイルは作り直す | 初めて `kiroku serve` を実行したとき |
| `open.html`（鍵付きで画面を開くための中継ページ） | 同じ設定フォルダ | `0600` | `kiroku serve` / `kiroku open` がブラウザを開くとき |
| 自動起動の設定 | macOS `~/Library/LaunchAgents/io.github.michinaoshimizu.kiroku.plist`、Linux `~/.config/systemd/user/kiroku.service` | `0644`（履歴は含まない。PATH や履歴の場所の環境変数の値は含む） | `kiroku autostart on` を実行したときだけ |
| 自動起動のログ | macOS `~/Library/Logs/kiroku.log`、Linux は `journalctl --user -u kiroku` | kiroku は権限を指定していない（launchd / systemd が作る） | 自動起動中 |

補足:

- HTML と JSON は同じフォルダの一時ファイルに書いてから置き換えるので、出力先にシンボリックリンクがあってもたどらず置き換えます（`internal/cli/cli.go` の `writePrivate`）
- `kiroku serve` が履歴を読み直さないために使うキャッシュはメモリ上だけで、ファイルには書きません（`internal/cli/cache.go`、`internal/report/cache.go`）
- 画面の表示設定はブラウザのストレージに保存されます
- `0600` / `0700` は Unix の権限です。Windows では NTFS の ACL（通常は親フォルダの継承）に従い、kiroku は ACL を設定しません
- HTML・JSON の中身はプロンプトやパスそのものです。権限は本人だけでも、ファイルを他人に渡せば中身はすべて渡ります。`kiroku html --week` / `--month` で期間を絞ることはできますが、その期間の内容はそのまま含まれます
- `kiroku archive` を有効にすると、エージェントが自動削除した履歴（利用者が意図して削除した会話や、Kiro Crew のシークレット／一時会話の本文を含む）が kiroku のフォルダに残ります（guide.md）。社内の保存期間ルールとの関係を確認してください

## 4. 通信

kiroku 自身が外部と通信するのは、GitHub Releases（`https://github.com/MichinaoShimizu/kiroku`）への次の 2 つだけです。履歴や集計結果を送る通信、テレメトリ、利用状況の収集はありません（`net/http` を使うコードは `internal/cli/update.go`、`serve.go`、`servekey.go`、`autostart.go`、`internal/ai/handler.go` で、外部へ出るのは `update.go` の関数だけです）。

| いつ | 何をするか | 止め方 |
|---|---|---|
| `kiroku update` を実行したとき | 最新版を `releases/latest` のリダイレクトで確認し、リリースのアーカイブと `checksums.txt` をダウンロードして自分自身を置き換える | 実行しなければ通信しない。`--check` は確認だけ |
| `kiroku doctor` を実行したとき | 新しい版があるかを `releases/latest` に確認する（5 秒で打ち切り） | `kiroku doctor --no-update-check`。ソースからビルドした版（`dev`）は確認しない |

- 送るのは通常の HTTPS リクエストだけで、履歴由来のデータは含みません。プロキシの環境変数（`HTTPS_PROXY` など）はそのまま使います
- `https` 以外へのリダイレクトはたどりません
- `kiroku serve`・`html`・`json`・`stats`・`archive` は外部と通信しません。自動で更新を確認する仕組みもありません
- 全体で通信を無効にする設定や環境変数はありません。`doctor` の確認を止めたい場合は `--no-update-check` を使うか、ネットワーク側で制御してください
- HTML の画面は Content-Security-Policy で外部リソースの読み込みを禁止しています（`connect-src` は保存した HTML では `'none'`、`kiroku serve` では `'self'`。`internal/web/web.go`）。画面の下の GitHub へのリンクは、利用者が押したときだけブラウザで開きます
- 料金表の更新は CI（`.github/workflows/prices.yml`）だけが行い、kiroku 本体は料金ページを取得しません
- `install.sh` と `go install` は、それぞれ GitHub と Go のモジュールプロキシからダウンロードします（kiroku 本体の通信ではありません）
- 6 章の「Run with AI」では、kiroku が起動した AI CLI が、それぞれの提供元と通信します

## 5. kiroku serve

`kiroku serve` はローカルに HTTP サーバーを立てて画面を表示します（`internal/cli/serve.go`、`servekey.go`）。

| 項目 | 内容 |
|---|---|
| 待ち受け | 既定は `127.0.0.1:8484`。`:8485` のようにポートだけ書いても `127.0.0.1` |
| 鍵 | 32 バイトの乱数を `serve-key` に保存し、鍵を持つブラウザにだけ画面・データ・履歴ファイルを返す。ブラウザには Cookie（`HttpOnly`、`SameSite=Strict`）で渡し、比較は定数時間。鍵はブラウザのコマンドライン引数に載せない（`0600` の `open.html` を経由する） |
| Host 検査 | Host が `localhost`・`127.0.0.1`・`[::1]`・待ち受けアドレス・`--allow-host` で足した名前以外なら 403（DNS リバインディング対策） |
| 変更系の要求 | `POST` には画面だけが付ける `X-Kiroku: 1` ヘッダーと同一オリジンを求める |
| 応答ヘッダー | `X-Frame-Options: DENY`、`Content-Security-Policy: frame-ancestors 'none'`、`X-Content-Type-Options: nosniff`、`Referrer-Policy: no-referrer`、`Cross-Origin-Opener-Policy` / `Cross-Origin-Resource-Policy: same-origin` |
| 画面の CSP | 自分のインラインスクリプトとスタイル（ハッシュで指定）だけを許し、外部リソースを禁止 |
| 履歴ファイルの配信 | 読み込んだセッションの `.json` / `.jsonl` で、実体（シンボリックリンクをたどった先）が履歴の場所か archive フォルダの中にあるものだけ |
| エラー | 応答にローカルのパスを含めない（詳細は端末に出す） |

**`0.0.0.0` などで待ち受けたときのリスク**: `kiroku serve 0.0.0.0:8484` のように明示的に指定すると、同じネットワークのほかの機器から接続できるようになります。鍵がなければ画面は見えませんが、通信は暗号化されていない HTTP なので、`kiroku open --print` で表示した鍵付きのアドレスや Cookie はネットワーク上を平文で流れます。鍵付きのアドレスを知った人は履歴をすべて読めます。社内ネットワークでも、原則として既定の `127.0.0.1` のままにすることを推奨します。

**共有サーバー（SSH でログインする共用マシンなど）**: `127.0.0.1` だけでは同じマシンのほかのユーザーを防げないため、鍵が必要になっています。鍵ファイルは本人だけが読めますが、root など管理者権限を持つユーザーからは保護されません。

## 6. Run with AI（`kiroku serve --ai`）

既定ではオフです。`kiroku serve --ai` で起動したときだけ、画面の「Run with AI」から、そのコンピューターにすでにインストールされている AI CLI でレビュー・報告のプロンプトを実行できます（`internal/ai/runner.go`、`handler.go`）。`--ai` なしの `kiroku serve` と保存した HTML ファイルでは、AI は一切実行されません（エンドポイントは 403 を返します）。`kiroku autostart` で登録される `kiroku serve` には `--ai` は付きません。

### データの流れ

kiroku 自身は何も送りません。ただし、kiroku が起動した AI CLI は、**プロンプトと、そのエージェントが読んだ履歴を、それぞれの提供元に送る可能性があります**。

| 選べるエージェント | 起動するコマンド | 送り先（想定） |
|---|---|---|
| Claude Code | `claude` | Anthropic（または利用者の Claude Code が設定している接続先） |
| Codex | `codex` | OpenAI（同上） |
| Kiro CLI | `kiro-cli` | AWS（同上） |

各 CLI は、利用者の環境変数・ログイン状態・設定ファイルをそのまま使って動きます（kiroku は環境変数を受け継がせ、認証情報は扱いません）。したがって、送信されたデータの扱いは、**会社と各提供元との既存の契約・設定**（データの保持、学習への利用の有無、リージョンなど）に従います。導入前に、対象の CLI が会社の契約アカウントで使われていることを確認してください。

プロンプトには、kiroku が集計した数値、セッションのプロンプトの抜粋、履歴ファイルのパスが入ります。エージェントは指示に従って履歴ファイルを読み、その中身（プロンプト、AI の返答、ツールの出力など）が提供元に送られることがあります（guide.md「Reviews and reports with your agent」）。

### 各エージェントに課す制限

| エージェント | 引数 | 制限の内容 |
|---|---|---|
| Claude Code | `claude -p --output-format text --strict-mcp-config --tools Read --add-dir <履歴の場所>…` | 使えるツールは Read だけ。読めるのは `--add-dir` で渡した履歴の場所（kiroku が読む履歴フォルダと archive フォルダ）と作業フォルダ。それ以外の読み取りは承認が必要で、`-p`（非対話）では承認されない。`--strict-mcp-config` で MCP サーバーは読み込まない |
| Codex | `codex exec --sandbox read-only --skip-git-repo-check -` | 読み取り専用のサンドボックス。書き込みはできないが、**読み取りはコンピューター上の任意のファイルが対象になり得る**（履歴の場所に限定されない） |
| Kiro CLI | `kiro-cli chat --no-interactive` | **kiroku はツールの制限を指定していない**。どのツールが使えるかは kiro-cli の既定と利用者の設定（信頼済みツールなど）に依存し、kiroku 側では確認できていない |

いずれも、Claude Code の hooks、Codex の `config.toml` など、利用者自身の CLI の設定は kiroku からは上書きしていません。これらの設定が実行時にどう効くかは各 CLI の仕様によります。

### 実行時の制限

| 項目 | 内容 |
|---|---|
| 待ち受け | `--ai` は数値のループバックアドレス（`127.0.0.1:8484` など）でしか起動できない。`0.0.0.0` やホスト名では起動を拒否 |
| 認証 | 5 章の鍵・Host 検査・`X-Kiroku` ヘッダー・同一オリジン検査をすべて通った `POST /ai/run` だけ |
| 実行ごとの確認 | 画面が実際に送るプロンプト全文を表示し、利用者が「Run」を押したときだけ送る。サーバーは `consent: true` のない要求を拒否する（確認の画面そのものはブラウザ側の処理） |
| 同時実行 | 1 件まで（実行中の要求は 429） |
| 入力の上限 | プロンプト 256 KiB |
| 時間の上限 | 2 分で打ち切り |
| 出力の上限 | 標準出力 1 MiB（超えたらエラー）、エラー出力は最後の 2 KiB だけ保持 |
| 作業フォルダ | 毎回新しく作る空の一時フォルダ（終了後に削除）。kiroku serve を起動したフォルダのプロジェクト指示・設定・リポジトリは読み込まれない |
| コマンド | 実行するのは上の 3 つの固定のコマンドだけ。要求に含まれるコマンド文字列は実行しない。プロンプトは標準入力で渡す |

## 7. 更新とインストールの検証

### install.sh（macOS / Linux）

1. GitHub Releases から OS・CPU に合ったアーカイブと `checksums.txt` をダウンロードし、SHA-256 を照合します。一致しなければ中止します
2. GitHub CLI（`gh`）があれば、`gh attestation verify` でビルドの出どころ（artifact attestation）も確認します。署名したワークフローが `release.yml`、ソースの参照が `main` かリリースのタグ、GitHub ホストのランナーであることを条件にし（`--signer-workflow`、`--source-ref`、`--deny-self-hosted-runners`）、一致しなければ中止します
3. `gh` がない・ログインしていない・GitHub に届かない場合は、警告だけで導入を続けます。**社内では `KIROKU_REQUIRE_ATTESTATION=1` を付けて、確認できない場合は止めることを推奨します**
4. スクリプト本体は関数にまとめられ最後の行で呼ばれるので、途中で切れたダウンロードは何も実行しません

```bash
curl -fsSL https://raw.githubusercontent.com/MichinaoShimizu/kiroku/main/install.sh \
  | KIROKU_VERSION=v<版> KIROKU_REQUIRE_ATTESTATION=1 sh
```

出どころの証明は v0.12.0 以降のリリースにあります。

### kiroku update

`checksums.txt` との照合は行いますが、**attestation の確認は行いません**（SECURITY.md）。受け付ける版の名前は `v1.2.3`（または `v1.2.3-rc.1`）の形だけ、`https` のリダイレクトだけをたどり、展開するのは 200 MiB までの通常ファイル 1 つだけ、古い版へは `--force` がないと戻しません。attestation まで確認したい場合は、`kiroku update` ではなく `install.sh`（`KIROKU_REQUIRE_ATTESTATION=1`）か、次の手動確認を使ってください。

### 手動での確認（Windows や社内配布の場合）

```bash
# ビルドの出どころの確認（GitHub CLI が必要）
gh attestation verify kiroku_<版>_windows_amd64.zip --repo MichinaoShimizu/kiroku

# リリースに同梱の Sigstore バンドルを使う場合（v0.14.0 より後のリリース）
gh attestation verify kiroku_<版>_linux_amd64.tar.gz --repo MichinaoShimizu/kiroku \
  --bundle kiroku_<版>.sigstore.json
```

- 各アーカイブには SPDX 形式の SBOM（`<アーカイブ名>.sbom.json`、v0.13.3 より後）が付き、Go の版と組み込まれたモジュールの一覧がわかります
- ビルドは再現可能です。`sh tools/reproduce.sh v<版> <os> <arch>` で、タグのソースからビルドしたバイナリとリリースのバイナリがバイト単位で一致するかを確かめられます（SECURITY.md「Verifying a release」）
- macOS で Apple の公証（notarization）は受けていません。ブラウザでダウンロードすると Gatekeeper の警告が出ます（guide.md）

## 8. 開発とサプライチェーン

| 項目 | 内容 |
|---|---|
| CI（`ci.yml`） | プルリクエストごとにテスト、`staticcheck`、`govulncheck`（既知の脆弱性）を実行。リリース版を `KIROKU_REQUIRE_ATTESTATION=1` で導入する確認も含む |
| CodeQL（`codeql.yml`） | Go と画面の JavaScript を、プルリクエスト・main への push・毎週解析 |
| ファジング（`fuzz.yml`） | 履歴の読み取り、git のリモート解析、HTML 画面に対し、毎週と main への push で実行 |
| OpenSSF Scorecard（`scorecard.yml`） | 毎週と main への push で評価し、結果を公開 |
| GitHub Actions | サードパーティのアクションはすべてコミット SHA で固定。ジョブごとに必要最小限の `permissions`。リリースのビルドは Actions のキャッシュを使わない |
| リリース（`release.yml`） | GitHub ホストのランナーで GoReleaser によりビルドし、`actions/attest-build-provenance` で出どころを署名 |
| Dependabot | Go モジュール・Actions・テスト用 npm を毎週更新提案、脆弱性の修正は即時 |

**依存モジュール**（`go.mod`、Go 1.26 / toolchain go1.26.9）:

| モジュール | 用途 |
|---|---|
| `github.com/klauspost/compress` | zstd（Codex の圧縮履歴、archive のコピー） |
| `modernc.org/sqlite` | SQLite（Amazon Q / Kiro CLI の履歴）。純 Go 実装 |
| 間接依存 | `github.com/dustin/go-humanize`、`github.com/google/uuid`、`github.com/mattn/go-isatty`、`github.com/ncruces/go-strftime`、`github.com/remyoudompheng/bigfft`、`golang.org/x/sys`、`modernc.org/libc`、`modernc.org/mathutil`、`modernc.org/memory` |

開発体制について: 個人が保守するオープンソースです（Copyright Michinao Shimizu）。商用サポートや SLA はありません。

## 9. 情シスが決めること・推奨設定のチェックリスト

- [ ] **導入方法**: `install.sh` を使う場合は `KIROKU_REQUIRE_ATTESTATION=1` と `KIROKU_VERSION` で版を固定する。Windows・社内配布では 7 章の `gh attestation verify` で確認してから配る
- [ ] **更新の運用**: `kiroku update` は attestation を確認しないため、検証済みの版を社内で配る運用にするか、`install.sh` での再導入に統一するかを決める。サポートは最新版のみなので、更新の頻度も決める
- [ ] **更新確認の通信**: `kiroku doctor` が GitHub に問い合わせることを許容するか（不要なら `--no-update-check` を案内、またはネットワークで制御）
- [ ] **`kiroku serve` の待ち受け**: 既定の `127.0.0.1` のままにし、`0.0.0.0` など外部から届くアドレスで起動しない。`kiroku open --print` の鍵付きアドレスを共有しない
- [ ] **Run with AI（`--ai`）を認める条件**:
  - [ ] 使える CLI（`claude` / `codex` / `kiro-cli`）を、会社の契約アカウントで認証されたものに限る
  - [ ] Codex は履歴以外のファイルも読み得ること、Kiro CLI は kiroku 側でツールを制限していないことを踏まえ、どの CLI を認めるかを決める（例: 制限が最も明確な Claude Code のみ）
  - [ ] 実行前に表示されるプロンプトを確認するよう利用者に周知する
  - [ ] 各 CLI 側の設定（Claude Code の managed settings、Codex の `config.toml`、Kiro CLI の信頼済みツール）で必要な制限をかける
- [ ] **`kiroku archive`**: 有効にすると、エージェントが削除した履歴や利用者が削除した会話のコピーが残る。社内の保存期間・削除ルールと合うかを決める（既定はオフ）
- [ ] **HTML / JSON の取り扱い**: プロンプト・パス・コミットメッセージがそのまま入る。社外への持ち出し、チャットへの添付、共有フォルダへの保存のルールを決める
- [ ] **共用マシン**: SSH で入る共用サーバーなどで使う場合、管理者からは保護されないことを前提にする
- [ ] **自動起動**: `kiroku autostart on` を認めるか（`--ai` なしで常駐する）
- [ ] **アンインストール手順**: `kiroku autostart off`、`kiroku archive off`、設定フォルダ（`serve-key`）の削除、バイナリの削除（guide.md「Uninstall」）

## 10. 問い合わせ・脆弱性報告

- 脆弱性は、GitHub の非公開の脆弱性報告（https://github.com/MichinaoShimizu/kiroku/security/advisories/new）から報告してください。公開の issue は使わず、実際の履歴は添付しないでください
- 日本語での報告も受け付けています。最初の返答は 1 週間以内が目安です（SECURITY.md）
- 脆弱性以外の質問や不具合は、GitHub の issue（https://github.com/MichinaoShimizu/kiroku/issues）へ
- セキュリティ修正は CHANGELOG.md の各版の `### Security` に記載されます
