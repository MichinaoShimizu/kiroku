package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// releaseRepo は、社内などのリポジトリで作ったビルドが更新を取りに行く場所（HOST/OWNER/REPO、例 github.com/your-org/kiroku）。
// 公開のリポジトリ（MichinaoShimizu/kiroku）以外でビルドしたときだけ、.goreleaser.yaml が -ldflags -X で埋め込む。
// 空なら、これまでどおり releaseBase から認証なしの https で取る。
// 埋め込まれていれば、プライベートのリポジトリにも届くよう、GitHub CLI（gh）とそのログインを使って取る。
// kiroku はトークンを持たず、gh はシェルを通さずに引数で起動する。確かめ方（checksums.txt）と入れかえ方は同じ。
var releaseRepo = ""

// repoPattern は releaseRepo の形（HOST/OWNER/REPO）。どの部分も - では始まらない（gh のオプションと取り違えないため）。
var repoPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.-]*/[A-Za-z0-9][A-Za-z0-9_.-]*/[A-Za-z0-9][A-Za-z0-9_.-]*$`)

// viaGH は、このビルドが gh で更新を取りに行くか。
func viaGH() bool { return releaseRepo != "" }

// releasesPage は、案内に出す Releases の場所。
func releasesPage() string {
	if viaGH() {
		return "https://" + termSafe(releaseRepo, 200) + "/releases"
	}
	return releaseBase + "/releases"
}

// upstreamURL は公開のリポジトリ。
const upstreamURL = "https://github.com/MichinaoShimizu/kiroku"

// projectURL は、画面のいちばん下のリンク（GitHub・その版のリリース）が指すリポジトリ。
// releaseRepo が埋め込まれていれば（社内のリポジトリのビルドやデモ）そこ、なければ公開のリポジトリ。
func projectURL() string {
	if viaGH() && repoPattern.MatchString(releaseRepo) {
		return "https://" + releaseRepo
	}
	return upstreamURL
}

// ghCommand は起動する gh の名前（テストで差しかえる）。
var ghCommand = "gh"

// errGHNotFound は、gh で取りに行くビルドなのに gh が見つからないとき。
var errGHNotFound = errors.New("not found")

// ghError は gh が失敗したときのエラー。notFound は、その版やファイルがなかったとき
// （HTTP 404 だけでは、見る権限がないプライベートのリポジトリと区別できないので、notFound にしない）。
type ghError struct {
	msg      string
	notFound bool
	detail   string // 標準エラーの全体（64 KiB まで、端末に出せる形で）。gh attestation verify の結果を見分けるため
}

func (e *ghError) Error() string { return e.msg }

// ghMaxStdout は gh の出力として読む上限（アーカイブの大きさの上限と同じ）。
var ghMaxStdout = 200 << 20

// capWriter は limit を超えて書かれたらエラーにする（gh の出力でメモリを使い切らないため）。
// discard なら、エラーにせず limit を超えた分を捨てる（標準エラー。説明に使うのは最初の行だけなので）。
type capWriter struct {
	buf     bytes.Buffer
	limit   int
	over    bool
	discard bool
}

func (w *capWriter) Write(p []byte) (int, error) {
	if w.buf.Len()+len(p) > w.limit {
		w.over = true
		if w.discard {
			w.buf.Write(p[:max(0, w.limit-w.buf.Len())])
			return len(p), nil
		}
		return 0, errors.New("output too large")
	}
	return w.buf.Write(p)
}

// ghWaitDelay は、gh を止めたあと（か gh が終わったあと）、gh から起動されたものが出力をつかんだままでも待つ上限。
// これがないと、gh がラッパーのスクリプトで子が残ったとき、締め切りを過ぎても終わらない。
const ghWaitDelay = 3 * time.Second

// runGH は gh を args で起動し、標準出力を返す。show があれば、届いた分をそこに見せる。
// 標準エラーは、失敗したときの説明に最初の行だけを使う（端末に出せる形にして）。
func runGH(ctx context.Context, show io.Writer, args ...string) ([]byte, error) {
	if !repoPattern.MatchString(releaseRepo) {
		return nil, errOddRepo
	}
	// 今いるフォルダの gh は使わない（exec.ErrDot。GODEBUG=execerrdot=0 でゆるめられても、絶対パスでなければ使わない）
	path, err := exec.LookPath(ghCommand)
	if err != nil || !filepath.IsAbs(path) {
		return nil, errGHNotFound
	}
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.WaitDelay = ghWaitDelay
	// 聞き返さない・gh 自身の更新の案内や色を出さない
	cmd.Env = append(os.Environ(), "GH_PROMPT_DISABLED=1", "GH_NO_UPDATE_NOTIFIER=1", "NO_COLOR=1", "GH_SPINNER_DISABLED=1")
	out := &capWriter{limit: ghMaxStdout}
	var w io.Writer = out
	if show != nil {
		p := &progress{w: show}
		defer p.done()
		w = io.MultiWriter(out, p)
	}
	stderr := capWriter{limit: 64 << 10, discard: true}
	cmd.Stdout, cmd.Stderr = w, &stderr
	if err := cmd.Run(); err != nil && !(errors.Is(err, exec.ErrWaitDelay) && ctx.Err() == nil) {
		// （ErrWaitDelay だけなら、gh は締め切りの前に成功して終わり、残った子が出力をつかんでいただけ）
		if out.over {
			return nil, fmt.Errorf("the download is larger than %d MiB", ghMaxStdout>>20)
		}
		if ctx.Err() != nil {
			return nil, fmt.Errorf("gh did not finish in time: %w", ctx.Err())
		}
		line := strings.TrimSpace(stderr.buf.String())
		if i := strings.IndexByte(line, '\n'); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
		low := strings.ToLower(line)
		nf := strings.Contains(low, "release not found") || strings.Contains(low, "no assets match")
		if line == "" {
			line = err.Error()
		}
		return nil, &ghError{msg: "gh: " + termSafe(line, 300), notFound: nf, detail: termSafe(strings.ReplaceAll(stderr.buf.String(), "\n", " "), 64<<10)}
	}
	return out.buf.Bytes(), nil
}

// errOddRepo は、埋め込まれた releaseRepo が HOST/OWNER/REPO の形でないとき（ビルドのまちがい）。
var errOddRepo = errors.New("this kiroku was built to update from a repository that is not of the form HOST/OWNER/REPO")

// ghFailure は gh のエラーを、どうすればよいかの案内つきにする。
func ghFailure(what string, err error) error {
	if errors.Is(err, errOddRepo) {
		return fmt.Errorf("%s: %w (%q)", what, err, termSafe(releaseRepo, 200))
	}
	repo := termSafe(releaseRepo, 200) // 形は確かめてあるが、念のため
	if errors.Is(err, errGHNotFound) {
		return fmt.Errorf("%s: this kiroku updates from %s, which needs the GitHub CLI (gh) to be installed and logged in (https://cli.github.com/, then \"gh auth login\")", what, repo)
	}
	var ge *ghError
	if errors.As(err, &ge) && !ge.notFound {
		return fmt.Errorf("%s from %s (is gh logged in with access to it? try \"gh auth status\"): %w", what, repo, err)
	}
	return fmt.Errorf("%s from %s: %w", what, repo, err)
}

// ghLatest は gh で最新の版を確かめる（gh release view。プレリリースは入らない、releases/latest と同じ）。
func ghLatest(timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	out, err := runGH(ctx, nil, "release", "view", "--repo", releaseRepo, "--json", "tagName", "--jq", ".tagName")
	if err != nil {
		return "", ghFailure("could not find the latest release", err)
	}
	tag := strings.TrimSpace(string(out))
	if !validTag(tag) {
		return "", fmt.Errorf("unexpected release version %q", termSafe(tag, 100))
	}
	return tag, nil
}

// ghDownloadTimeout は gh で 1 つのファイルを落とすときの上限（遅い回線でも終わるよう、HTTP のクライアントと同じ長さ）。
const ghDownloadTimeout = 30 * time.Minute

// ghFetch は tag のリリースから name を gh で落とす（標準出力に書かせて読む。ファイルには書かない）。
// tag と name はどちらも、確かめた形（validTag と、OS・CPU から作った名前）だけ。
func ghFetch(tag, name string, show io.Writer) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), ghDownloadTimeout)
	defer cancel()
	return runGH(ctx, show, "release", "download", tag, "--repo", releaseRepo, "--pattern", name, "--output", "-")
}

// ghAttestNote は、gh で出どころの証明を確かめられなかったときに出す 1 行の書き先（テストで差しかえる）。
var ghAttestNote io.Writer = os.Stderr

// ghVerifyAttestation は、gh で落としたアーカイブ（checksums.txt と合ったもの）が、releaseRepo の release.yml で
// GitHub のランナーが作ったものか（artifact attestation）を、置きかえる前に gh attestation verify で確かめる。install.sh と同じ決まり：
//   - 証明が見つからない・合わない：止める（置きかえない）
//   - 確かめられない（古い gh・GitHub に届かない）：警告だけ。KIROKU_REQUIRE_ATTESTATION=1 なら止める
//   - KIROKU_SKIP_ATTESTATION=1：確かめない
//
// gh には、本人だけが読める一時フォルダ（0700）に書いたファイル（0600）を渡す。引数はどれも確かめた形（repoPattern・validTag）だけ。
func ghVerifyAttestation(tag, name string, archive []byte) error {
	require := os.Getenv("KIROKU_REQUIRE_ATTESTATION") == "1"
	if os.Getenv("KIROKU_SKIP_ATTESTATION") == "1" {
		if require {
			return errors.New("KIROKU_SKIP_ATTESTATION=1 and KIROKU_REQUIRE_ATTESTATION=1 cannot be used together")
		}
		fmt.Fprintln(ghAttestNote, "KIROKU_SKIP_ATTESTATION=1: not checking the build provenance (checksums.txt matched)")
		return nil
	}
	if !repoPattern.MatchString(releaseRepo) || !validTag(tag) {
		return errOddRepo
	}
	parts := strings.SplitN(releaseRepo, "/", 2) // HOST と OWNER/REPO
	host, repo := parts[0], parts[1]
	dir, err := os.MkdirTemp("", "kiroku-update-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	if abs, err := filepath.Abs(dir); err == nil { // TMPDIR が - で始まる相対パスでも、gh の旗にならないように
		dir = abs
	}
	file := filepath.Join(dir, name)
	if err := os.WriteFile(file, archive, 0o600); err != nil {
		return err
	}
	unchecked := func(why string) error {
		if require {
			return fmt.Errorf("%s, and KIROKU_REQUIRE_ATTESTATION=1 is set", why)
		}
		fmt.Fprintf(ghAttestNote, "warning: %s; checksums.txt matched, so updating anyway (KIROKU_REQUIRE_ATTESTATION=1 makes this check required)\n", why)
		return nil
	}
	// 作ったのは release.yml。もとにしたのは、tag.yml が main で動いたときは main、タグを手で push したときはそのタグ
	var last error
	for _, ref := range []string{"refs/heads/main", "refs/tags/" + tag} {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		_, err := runGH(ctx, nil, "attestation", "verify", "--repo", repo, "--hostname", host,
			"--signer-workflow", releaseRepo+"/.github/workflows/release.yml", "--source-ref", ref, "--deny-self-hosted-runners", "--", file)
		cancel()
		if err == nil {
			return nil
		}
		last = err
		var ge *ghError
		if !errors.As(err, &ge) || !strings.Contains(ge.detail, "SourceRepositoryRef") {
			break // 合わなかったのが「もとにした ref」だけなら、もう一方で試す
		}
	}
	var ge *ghError
	if !errors.As(last, &ge) {
		return ghFailure("could not check the build provenance of "+name, last)
	}
	low := strings.ToLower(ge.detail + " " + ge.msg)
	// gh の説明には、証明書の値（ブランチ名・ワークフローのファイル名など、リリースを作れる人が選べる文字）がそのまま入る。
	// 「合わない」の印が 1 つでもあれば、ほかにどんな言葉（timeout など）があっても止める（確かめられなかったことにしない）
	if attestMismatch.MatchString(low) {
		return fmt.Errorf("the build provenance of %s does not match %s, not updating (gh attestation verify failed; set KIROKU_SKIP_ATTESTATION=1 to skip this check)", name, termSafe(releaseRepo, 200))
	}
	switch {
	case strings.Contains(low, "unknown command") || strings.Contains(low, "unknown flag"):
		return unchecked("this gh is too old to check the build provenance (gh attestation verify)")
	case regexp.MustCompile(`http (401|403|429|5\d\d)|sigstore verifier|dial tcp|no such host|connection (refused|reset)|timeout|tls handshake|network is unreachable`).MatchString(low):
		return unchecked("gh could not reach " + host + " to check the build provenance")
	}
	return fmt.Errorf("the build provenance of %s does not match %s, not updating (gh attestation verify failed; set KIROKU_SKIP_ATTESTATION=1 to skip this check)", name, termSafe(releaseRepo, 200))
}

// attestMismatch は、gh attestation verify が「証明が見つからない・合わない」と言うときの印（小文字で）。
// gh の書式（"expected %s to be %s, got %s"・"no matching CertificateIdentity found"・"no attestations found" など）と HTTP 404。
var attestMismatch = regexp.MustCompile(`expected \S+ to be|no matching|no attestations|does not match|verification failed|failed to verify|http 404`)
