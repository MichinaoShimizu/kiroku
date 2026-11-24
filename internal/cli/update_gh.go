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
		return nil, &ghError{msg: "gh: " + termSafe(line, 300), notFound: nf}
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
