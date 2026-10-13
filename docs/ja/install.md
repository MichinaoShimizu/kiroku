# kiroku インストールと社内展開のガイド

社員のみなさんと情報システム部門の方に向けて、kiroku のインストール、社内での配り方、最初の使い方、アンインストールをまとめます。英語の[ガイド](../guide.md)と [README](../../README.ja.md) を補うもので、細かいオプションはガイドにあります。セキュリティ審査の資料は[セキュリティレビュー資料](security-review.md)を見てください。

kiroku は、このコンピューターにある AI エージェント（Claude Code・Kiro・Amazon Q Developer CLI・Codex CLI）の履歴を読み、ブラウザで見られる画面にします。バイナリ 1 つで動き、アカウントもテレメトリもありません。ネットワークを使うのは、kiroku 自身の新しい版を GitHub で確かめてダウンロードするときだけです（`kiroku serve --ai` については「[Run with AI](#run-with-ai)」を見てください）。

## 動作環境

| OS | CPU | 配布ファイル | 注意 |
|---|---|---|---|
| macOS | Intel（amd64）・Apple Silicon（arm64） | `kiroku_<版>_darwin_<amd64 か arm64>.tar.gz` | ブラウザでダウンロードすると「開けません」と出ることがあります（[困ったとき](#困ったとき)） |
| Linux | amd64・arm64 | `kiroku_<版>_linux_<amd64 か arm64>.tar.gz` | `kiroku autostart` は systemd のユーザーサービスを使います |
| Windows | amd64・arm64 | `kiroku_<版>_windows_<amd64 か arm64>.zip` | 下を見てください |

Windows での制約:

- `install.sh` は使えません。[Releases](https://github.com/MichinaoShimizu/kiroku/releases) の zip を展開してください
- `kiroku autostart` はまだ使えません。代わりに、スタートアップフォルダー（Win+R で `shell:startup`）に `kiroku serve --no-open` のショートカットを置いてください
- Kiro CLI（SQLite）の履歴の場所（`%LOCALAPPDATA%\kiro-cli\`）は確かめられていません。違うときは `--kiro-cli-db` で指定してください

`go install` で入れる場合は Go 1.26 以降が要ります。

## インストール

### macOS・Linux：install.sh（おすすめ）

```bash
curl -fsSL https://raw.githubusercontent.com/MichinaoShimizu/kiroku/main/install.sh | sh
```

OS と CPU に合ったファイルを GitHub Releases から落とし、`checksums.txt` で確かめてから `/usr/local/bin`（書き込めなければ `~/.local/bin`）に置きます。[GitHub CLI](https://cli.github.com/)（`gh`）があれば、ビルドの出どころ（このリポジトリの `release.yml` が GitHub のランナーで作ったこと）も `gh attestation verify` で確かめ、合わなければ止まります。スクリプトの本体は最後の行から呼ばれるので、ダウンロードが途中で切れても何も実行されません。

版や置き場所は環境変数で決められます。

```bash
curl -fsSL https://raw.githubusercontent.com/MichinaoShimizu/kiroku/main/install.sh | KIROKU_VERSION=v0.32.0 KIROKU_INSTALL_DIR=~/bin sh
```

| 環境変数 | 意味 |
|---|---|
| `KIROKU_VERSION` | 入れる版（例: `v0.32.0`）。なければ最新 |
| `KIROKU_INSTALL_DIR` | 置き場所。なければ `/usr/local/bin`（書き込めなければ `~/.local/bin`） |
| `KIROKU_REQUIRE_ATTESTATION=1` | 出どころの確認を必ず行う。`gh` がない・ログインしていない・GitHub に届かない・v0.12.0 より前の版のときは、入れずに止まる |
| `KIROKU_SKIP_ATTESTATION=1` | `gh` があっても出どころを確かめない |

会社では `KIROKU_REQUIRE_ATTESTATION=1` をおすすめします。これがないと、`gh` がない・ログインしていないときは、`checksums.txt` が合っていれば警告だけで入れます。

### Windows・手動：GitHub Releases から

[Releases](https://github.com/MichinaoShimizu/kiroku/releases) から、OS と CPU に合ったファイルと `checksums.txt` を落とし、確かめてから展開します。

```bash
# macOS・Linux：checksums.txt の該当行で確かめる（macOS は sha256sum の代わりに shasum -a 256）
grep ' kiroku_0.32.0_linux_amd64.tar.gz$' checksums.txt | sha256sum -c
tar -xzf kiroku_0.32.0_linux_amd64.tar.gz kiroku
```

```powershell
# Windows（PowerShell）：表示された値を checksums.txt の同じファイル名の行と比べる
Get-FileHash .\kiroku_0.32.0_windows_amd64.zip -Algorithm SHA256
```

出どころは `gh` で確かめられます（v0.12.0 以降）。

```bash
gh attestation verify kiroku_0.32.0_linux_amd64.tar.gz --repo MichinaoShimizu/kiroku
```

`kiroku_<版>.sigstore.json` が付いている版（v0.14.0 より後）では、`--bundle kiroku_<版>.sigstore.json` を足すと、GitHub に証明を問い合わせずに確かめられます（Sigstore の公開鍵は取りに行きます）。ソースからビルドしてバイナリが同じになることの確かめ方は [SECURITY.md](../../SECURITY.md#verifying-a-release) にあります。

### Go

```bash
go install github.com/MichinaoShimizu/kiroku@latest
```

この方法で入れた kiroku は版が `dev` になり、`kiroku update` では置きかわりません。更新も `go install …@latest` で行います。

## 社内配布の手順

社内のファイルサーバーやソフトウェア配布ツール（MDM など）で配る場合の流れです。`install.sh` は GitHub からダウンロードする作りで、社内のサーバーを指す設定はありません。社内配布ではバイナリそのものを配ってください。

1. 配る版を決めます。[CHANGELOG.md](../../CHANGELOG.md) の `### Security` も確かめてください
2. その版の配布ファイル（必要な OS・CPU の分）と `checksums.txt` を GitHub Releases から落とします
3. 上の手順で `checksums.txt` と `gh attestation verify` の両方を確かめます。合わないファイルは配らないでください
4. 展開した `kiroku`（Windows は `kiroku.exe`）を配り、全員が `PATH` の通った場所に置きます
5. 配ったあと、各自のコンピューターで `kiroku version` と `kiroku doctor` を実行して確かめます

### kiroku update を使わない運用にする場合

- kiroku は自動では更新しません。`kiroku update` を実行したときだけ、GitHub から最新（または `--to` の版）を落とし、`checksums.txt` で確かめてから自分自身を置きかえます。`kiroku update` は出どころの証明（attestation）は確かめません
- **更新を止める設定・環境変数はありません。** 利用者が `kiroku update` を実行すれば、書き込める場所にある kiroku は置きかわります
- 止めたい場合は、利用者が書き込めない場所（`/usr/local/bin` を root の持ち物にする、Windows なら `C:\Program Files\` の下など）に置いてください。`kiroku update` は「cannot write to … (it may need sudo)」で止まります。あわせて、利用者に `sudo kiroku update` を実行しないよう伝えてください
- `kiroku doctor` も GitHub に新しい版があるかを問い合わせ、あれば `kiroku update` を勧めます。問い合わせないようにするには `kiroku doctor --no-update-check` を使ってください。GitHub に届かないときは「could not check for a newer release」と出るだけで、ほかの点検は続きます
- 新しい版を配るときは、上の 1〜5 をくり返して置きかえます。Windows で `kiroku update` を使った場合に残る `kiroku.exe.old` は、次に kiroku を起動したときに消えます

## プロキシ環境

| 何が通信するか | 使うもの | プロキシ |
|---|---|---|
| `install.sh` | `curl`（`wget` は使いません。`curl` がないと止まります） | `curl` の決まりどおり、`https_proxy`（または `HTTPS_PROXY`）と `no_proxy` を使います |
| `kiroku update`・`kiroku doctor` | Go の `net/http`（既定の設定をそのまま使います） | `HTTPS_PROXY`（または `https_proxy`）と `NO_PROXY` を使います。GitHub は https なので、`HTTP_PROXY` だけでは使われません |
| `gh attestation verify` | GitHub CLI | `gh` も `HTTPS_PROXY` を使います |

```bash
export HTTPS_PROXY=http://proxy.example.co.jp:8080
curl -fsSL https://raw.githubusercontent.com/MichinaoShimizu/kiroku/main/install.sh | sh
kiroku update
```

- 通信先は `github.com`、`raw.githubusercontent.com`（install.sh）と、Releases のダウンロードのリダイレクト先（GitHub のファイル配信用ホスト）です。`kiroku update` は https 以外へのリダイレクトをたどりません
- TLS を検査するプロキシでは、そのプロキシの CA 証明書を OS に信頼させてください。kiroku は OS の証明書ストアを使います（Linux では `SSL_CERT_FILE`・`SSL_CERT_DIR` でも指定できます）
- 認証つきのプロキシは `http://ユーザー:パスワード@host:port` の形（Basic 認証）だけが使えます。NTLM や Kerberos が要るプロキシは通れないので、社内配布の手順で配ってください
- `kiroku serve` は `127.0.0.1` で待ち受けるだけで、外には通信しません。ブラウザにプロキシを設定している場合は、`localhost`・`127.0.0.1` をプロキシの対象外にしてください

## 初回の使い方

```bash
kiroku doctor             # 見つかった履歴、消されそうな履歴、次に打つコマンド
kiroku serve              # http://localhost:8484/ で画面を開く。新しい履歴が入るたびに更新（Ctrl+C で終わる）
kiroku open               # 別のブラウザで開く（ブラウザごとに最初の一度だけ要る）
kiroku stats --week last  # 先週の時間・トークン・費用・コミットを端末に 1 画面で
kiroku html --week last   # 先週だけを 1 つの HTML に書き出す（kiroku-YYYY-MM-DD.html）
kiroku autostart on       # ログインのたびに kiroku serve を起動（macOS と Linux）
kiroku help               # すべてのコマンド（オプションは kiroku <command> --help）
```

- `kiroku serve` は、鍵を持つブラウザにだけ画面を見せます。鍵はフォルダー `~/.config/kiroku`（Linux）、`~/Library/Application Support/kiroku`（macOS）、`%AppData%\kiroku`（Windows）の `serve-key` にあり、本人しか読めません。`kiroku serve` が開いたブラウザと `kiroku open` で開いたブラウザは鍵を受け取り、その後は `http://localhost:8484/` だけで開けます
- `kiroku html` と `kiroku json` が書くファイル（`0600`）には、依頼・AI の返事・ファイルのパス・コミットメッセージがそのまま入ります。人に渡す前に中身を確かめてください
- `kiroku autostart on` は `kiroku serve --no-open` を登録します（macOS は `~/Library/LaunchAgents/io.github.michinaoshimizu.kiroku.plist`、Linux は `~/.config/systemd/user/kiroku.service`）。状態は `kiroku autostart` で見られます
- ほかの機器から開けるようにする（`kiroku serve 0.0.0.0:8484` など）と、同じネットワークの人も届きます。会社の端末では既定の `127.0.0.1` のまま使ってください

## Run with AI

kiroku は、既定では AI を呼びません。振り返りや週報のプロンプトをコピーし、自分でエージェントに貼ります。

```bash
kiroku serve --ai
```

- `--ai` を付けて起動したときだけ、画面に「Run with AI」が出ます。プロンプトをそのまま見せ、確認したあとで、**このコンピューターにある** `claude`・`codex`・`kiro-cli` のどれかを実行して結果を見せます。kiroku 自身はどこにも送りません
- ただし、エージェントは**プロンプトと、読んだ履歴を、それぞれの提供元（Anthropic・OpenAI・AWS など）に送ることがあります**。使う前に、会社の AI 利用ルール（使ってよいサービス・契約・扱ってよい情報）に従っているか確かめてください
- `--ai` は数字のループバックアドレスでだけ使えます（既定の `127.0.0.1:8484` や `:8485` は可。`0.0.0.0` などは不可）
- `kiroku autostart` は `--ai` なしで登録します。`--ai` を付けて自動起動はできません。オンにしたいときは、自動起動を止めるか別のポートで、手で `kiroku serve --ai` を起動してください（すでに動いている serve にはあとから付けられないので、Ctrl+C で止めて起動し直します）
- `--ai` が効いているときは、画面の上に「LIVE · AI on」と出ます。保存した HTML ファイルでは、AI は動きません

## 履歴の保存期間と kiroku archive

エージェントの中には、古い履歴を自動で消すものがあります。消えた履歴は kiroku にも出せず、元にも戻せません。早めに設定してください。

| エージェント | 自動で消すか | 設定 |
|---|---|---|
| Claude Code | **消します。** 既定で 30 日より古い会話を、起動時に知らせずに消します | `~/.claude/settings.json` の `cleanupPeriodDays`（日数。1 以上の整数。`0` は不可） |
| Kiro Crew | **消します。** `sessions/archive/` に移した会話の記録のうち、30 日より古いものを消します | `~/.kiro/crew/config.local.json` の `session.archive_retention_days`（`-1` で消さない） |
| Kiro IDE・CLI、Amazon Q、Codex | 公式ドキュメントに、日数での自動削除の記載はありません | — |

Claude Code の例（`~/.claude/settings.json`）:

```json
{
  "cleanupPeriodDays": 3650
}
```

会社の管理設定（`managed-settings.json`。macOS は `/Library/Application Support/ClaudeCode/`、Linux は `/etc/claude-code/`、Windows は `C:\Program Files\ClaudeCode\`）に `cleanupPeriodDays` があると、そちらが優先されます。保存期間を全社で決めたい場合は、情報システム部門で管理設定に書いてください。

設定を変えたくない場合は、kiroku に圧縮した写しを残させます（既定はオフ）。

```bash
kiroku archive on    # いまある履歴をすぐ保存し、以後は kiroku が履歴を読むたびに保存
kiroku archive       # 状態・保存先・ファイルの数と大きさ
kiroku archive off   # 保存をやめる（端末で y と答えると、残した写しも消す）
```

- 写しを残すのは、自動で消すエージェント（Claude Code の会話、Kiro Crew の `sessions/archive/`）の分だけです
- 保存先は `~/.local/share/kiroku/archive`（Linux）、`~/Library/Application Support/kiroku/archive`（macOS）、`%LocalAppData%\kiroku\archive`（Windows）です。`--archive-dir` か `KIROKU_ARCHIVE_DIR` で変えられます。本人しか読めず、どこにも送りません
- kiroku を開いていない間に消えた履歴は残せません。保存期間が過ぎる前に、少なくとも一度は kiroku を開いてください（`kiroku autostart on` と組み合わせると確実です）
- 自分で消した会話も写しには残ります。社内の情報の保存ルールと合うか確かめてください

## アンインストール

1. 自動起動をオンにしていたら、止めて登録を外します

   ```bash
   kiroku autostart off
   ```

2. `kiroku archive` をオンにしていたら、`kiroku archive off` で `y` と答えて写しを消します。またはフォルダーを自分で消します（場所は `kiroku archive` で確かめられます）
3. 設定フォルダー（`serve` の鍵がある）を消します

   ```bash
   rm -rf ~/.config/kiroku                          # Linux
   rm -rf ~/Library/Application\ Support/kiroku     # macOS（archive の既定の保存先も含む）
   rm -rf ~/.local/share/kiroku                     # Linux の archive の既定の保存先
   ```

   Windows では `%AppData%\kiroku` と `%LocalAppData%\kiroku` を消します。`KIROKU_CONFIG_DIR` や `KIROKU_ARCHIVE_DIR` を使っていた場合は、その場所を消してください

4. バイナリを消します

   ```bash
   rm "$(command -v kiroku)"   # /usr/local/bin にあるときは sudo を付ける
   ```

ほかに kiroku が残すものはありません（`kiroku html` で書いた HTML は、書いた場所に残ります。画面の設定はブラウザに保存されています）。kiroku を先に消してしまい自動起動が残った場合は、ガイドの[手で外す方法](../guide.md#turn-it-off)を見てください。

## 困ったとき

まず `kiroku doctor` を実行してください。見つかった履歴、自動起動の状態、`PATH` で先に見つかる別の kiroku などを表示します。

| 症状 | 原因と対処 |
|---|---|
| `could not listen on 127.0.0.1:8484 (try another port …)` | ポートが使われています。多くは `kiroku autostart` で起動した serve がすでに動いています。`kiroku autostart` で確かめ、動いていれば `kiroku open` で開いてください。別のポートなら `kiroku serve :8485` |
| ブラウザに「Open kiroku with its key」（HTTP 403） | そのブラウザが鍵を持っていません（別のブラウザ、cookie を消した、ほかのサイトのリンクから来た）。端末で `kiroku open` を実行してください |
| ブラウザに `forbidden` とだけ出る（HTTP 403） | `localhost`・`127.0.0.1` 以外の名前で開いています。`http://localhost:8484/` で開くか、`--allow-host` で名前を足してください |
| `kiroku open` が `nothing answers at …` | `kiroku serve` が動いていません。`kiroku serve` か `kiroku autostart on` で起動してください |
| `kiroku autostart on` が `did not answer within …` | 上に出たログを見てください（macOS は `~/Library/Logs/kiroku.log`、Linux は `journalctl --user -u kiroku`）。自分で起動した serve がポートを使っていることがよくあります |
| install.sh が `… is not in your PATH` | 表示された 1 行をシェルの設定ファイルに足し、端末を開き直してください |
| install.sh が `the build provenance … does not match` | 出どころの証明が合いません。入れずに、情報システム部門に知らせてください |
| `kiroku update` が `cannot write to … (it may need sudo)` | 書き込めない場所にあります。社内配布の版なら更新は配布で行います。自分で入れたものなら `sudo kiroku update` |
| `kiroku update` が `dev build from source` | `go install` で入れたものです。`go install github.com/MichinaoShimizu/kiroku@latest` で更新してください |
| `--ai needs a loopback address …` | `--ai` は `127.0.0.1:8484` や `:8485` のような形でだけ使えます |
| macOS で「“kiroku” は開けません」 | ブラウザでダウンロードしたためです。`xattr -d com.apple.quarantine ./kiroku` を実行するか、システム設定 → プライバシーとセキュリティ の「このまま開く」で開いてください（install.sh と go install では出ません） |
| 画面に履歴が出ない・少ない | `kiroku doctor --all` で、エージェントごとに見ている場所を確かめてください。場所を変えている場合は `CLAUDE_CONFIG_DIR`・`KIRO_HOME`・`CODEX_HOME` などを設定します |

解決しない場合は [Issues](https://github.com/MichinaoShimizu/kiroku/issues) に日本語か英語で書いてください（履歴の中身やパスは貼らないでください）。脆弱性は [SECURITY.md](../../SECURITY.md) の方法で、非公開で知らせてください。
