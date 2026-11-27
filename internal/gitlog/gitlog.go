// Package gitlog は、エージェントが作業したプロジェクトのローカル git リポジトリからコミットを読む。
// 外部には何も送らない。git がない環境や、リポジトリでない場所は黙って飛ばす。
package gitlog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/MichinaoShimizu/kiroku/internal/core"
)

// Commit は 1 つのコミット。
type Commit struct {
	Hash    string     `json:"hash"`          // ハッシュ（画面では先頭 7 文字）
	URL     string     `json:"url,omitempty"` // リモート（GitHub など）のコミットのページ
	T       float64    `json:"t"`             // 作成日時（author date、UNIX 秒）
	Project string     `json:"project"`       // セッションと同じプロジェクト名
	Subject string     `json:"subject"`
	Body    string     `json:"body,omitempty"`   // 件名のあとの本文（先頭 600 文字）
	Branch  string     `json:"branch,omitempty"` // たどり着いた ref（git log --source）。ブランチの目安
	Repo    string     `json:"repo"`             // リポジトリのルート
	Cmd     string     `json:"cmd,omitempty"`    // 手元で見るために貼るコマンド（git -C <repo> show <hash>。安全に囲めなければ空）
	Added   int        `json:"added"`
	Removed int        `json:"removed"`
	Files   []FileStat `json:"files"`             // 変更したファイル（先頭 MaxFiles 件）
	NFiles  int        `json:"nFiles"`            // 変更したファイルの数
	AI      bool       `json:"ai"`                // エージェントがツールで実行したコミット（時刻が 2 分以内で一致）
	Session string     `json:"session,omitempty"` // AI が実行したときの、そのセッションの ID
}

// Push は、この PC から行った push の 1 回（リモート追跡ブランチの reflog の「update by push」）。
// ほかの PC からの push や、reflog の期限（既定 90 日）より古いものは残っていない。
type Push struct {
	T       float64  `json:"t"`                // push した時刻（UNIX 秒）
	Project string   `json:"project"`          // セッションと同じプロジェクト名
	Repo    string   `json:"repo"`             // リポジトリのルート
	Ref     string   `json:"ref"`              // 送った先（origin/main など）
	Hash    string   `json:"hash"`             // push したあとの先頭のコミット
	Commits int      `json:"commits"`          // 送ったコミットの数（前の位置がわからなければ 0）
	Prev    string   `json:"prev,omitempty"`   // push する前の位置（わからなければ空）
	Hashes  []string `json:"hashes,omitempty"` // 送ったコミット（新しい順、先頭 MaxPushHashes 件）
	URL     string   `json:"url,omitempty"`    // リモートでの先頭のコミットのページ
	Cmd     string   `json:"cmd,omitempty"`    // 手元で見るために貼るコマンド（git -C <repo> log/show。安全に囲めなければ空）
}

// MaxPushHashes は、1 回の push について残す送ったコミットの数（画面の push の詳細に出す）。
const MaxPushHashes = 50

// MaxPushRefs は 1 つのリポジトリから reflog を読むリモート追跡ブランチの上限。
const MaxPushRefs = 200

// FileStat は 1 つのファイルの変更行数（バイナリは -1）。
type FileStat struct {
	Path    string `json:"path"`
	URL     string `json:"url,omitempty"` // リモートでのそのコミットのファイル
	Added   int    `json:"added"`
	Removed int    `json:"removed"`
}

// MaxFiles は 1 つのコミットについて残すファイルの数。
const MaxFiles = 40

// MaxPerRepo は 1 つのリポジトリから読むコミットの上限。
const MaxPerRepo = 5000

// git は外から差しかえられる（テスト用）。
var git = func(ctx context.Context, dir string, args ...string) (string, error) {
	out, err := gitCmd(ctx, dir, args...).Output()
	return string(out), err
}

// gitCmd は、履歴に書かれた場所（だれかが作ったリポジトリかもしれない）で、git を読むだけのために動かすコマンド。
// リポジトリの .git/config は信用しない: そこに書かれたプログラム（fsmonitor・フック・textconv・外部 diff・ページャー・
// 署名を確かめるプログラム）を動かさないよう、コマンドラインの -c で上書きする（-c はリポジトリの設定より強い）。
//   - protocol.allow=never: どこからも取ってこない。部分クローン（--filter）で手元にないファイルを、git は
//     変更行数を数えるときに取りに行き、そのとき remote.<名前>.uploadpack などリポジトリが決めたプログラムを動かす。
//     kiroku はネットワークに出ないので、取りに行かせない。リポジトリの設定の protocol.<名前>.allow は -c protocol.allow より
//     強いので、どの protocol.* の設定より強い GIT_ALLOW_PROTOCOL も付ける（どこにもない名前 none だけを許す。空の値は
//     Windows で消えることがある）。GIT_NO_LAZY_FETCH=1 は、それを知っている git のため
//   - GIT_CONFIG_NOSYSTEM=1: システムの設定（/etc/gitconfig など）は読まない
//   - 利用者のグローバルな設定（~/.gitconfig）は読む。user.email（自分のコミットだけを読む）と safe.directory
//     （ほかのユーザーのリポジトリを読んでよいか）はそこにあり、本人が書いたものなので信用してよい
//   - GIT_TERMINAL_PROMPT=0: 何も聞いてこない。GIT_OPTIONAL_LOCKS=0: 読むだけで index などを書きかえない
func gitCmd(ctx context.Context, dir string, args ...string) *exec.Cmd {
	pre := []string{"-C", dir,
		"-c", "core.fsmonitor=false",
		"-c", "core.hooksPath=" + os.DevNull,
		"-c", "log.showSignature=false",
		"-c", "core.pager=cat",
		"-c", "protocol.allow=never",
	}
	if len(args) > 0 && args[0] == "log" { // 変更行数（--numstat）を数えるときに、リポジトリが決めたプログラムを通さない
		args = append([]string{"log", "--no-ext-diff", "--no-textconv"}, args[1:]...)
	}
	cmd := exec.CommandContext(ctx, "git", append(pre, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0", "GIT_NO_LAZY_FETCH=1", "GIT_ALLOW_PROTOCOL=none")
	return cmd
}

// remotePath は、Windows でネットワーク上の場所（UNC パス: \\server\share や //server/share）を指すか。
// そこへは os.Stat も git もしない（履歴に書かれた名前で、ほかのコンピューターにつなぎに行かないため）。
// \\?\C:\… は手元のドライブなので読む。
func remotePath(goos, p string) bool {
	if goos != "windows" || len(p) < 2 {
		return false
	}
	slash := func(c byte) bool { return c == '\\' || c == '/' }
	if !slash(p[0]) || !slash(p[1]) {
		return false
	}
	if len(p) >= 7 && (p[2] == '?' || p[2] == '.') && slash(p[3]) && p[5] == ':' && slash(p[6]) {
		return false
	}
	return true
}

type aiCommit struct {
	t       float64
	session string
}

type repo struct {
	top, project string
	email        string // user.email（空なら全員のコミット）
	web          string // リモートの Web の URL（https://github.com/owner/repo など）。わからなければ ""
	since        float64
	ai           []aiCommit
	rootProject  string // project を決めたセッションの作業ツリーのルート
}

// repoOf は、作業場所の作業ツリーのルートと、共通の git ディレクトリ（git worktree でも本体と同じ）を返す。リポジトリでなければ空。
// sure は答えが確かか: リポジトリだった、作業場所がない、git が終わって「リポジトリでない」と答えた（終了コードが 0 以外）ときは true。
// 時間切れや git を動かせなかったときは false（覚えておかず、次の読み直しで調べ直す）。
func repoOf(ctx context.Context, p string) (tc [2]string, sure bool) {
	if st, err := os.Stat(p); err != nil || !st.IsDir() {
		return [2]string{}, true
	}
	out, err := git(ctx, p, "rev-parse", "--show-toplevel", "--git-common-dir")
	if err != nil {
		var ee *exec.ExitError
		return [2]string{}, ctx.Err() == nil && errors.As(err, &ee)
	}
	f := strings.Split(strings.TrimSpace(out), "\n")
	if len(f) < 2 {
		return [2]string{}, true
	}
	top, common := filepath.Clean(strings.TrimSpace(f[0])), strings.TrimSpace(f[1])
	if !filepath.IsAbs(common) { // 古い git は作業場所からの相対パスで返す
		common = filepath.Join(p, common)
	}
	if c, err := filepath.EvalSymlinks(common); err == nil {
		common = c
	}
	return [2]string{top, filepath.Clean(common)}, true
}

// RepoTimeout は 1 つのリポジトリを読む時間の上限。リポジトリごとに数える（大きなリポジトリが 1 つあっても、ほかは読める）。
var RepoTimeout = 30 * time.Second

// Cache は、前回読んだリポジトリの結果を覚えておき、変わっていないリポジトリを読み直さない（kiroku serve 用）。
// ref の位置・reflog・user.email・origin・セッションから決まる範囲が同じなら、コミットも push も同じ。
type Cache struct {
	repos  map[string]repoCache // 共通の git ディレクトリごと
	mu     sync.Mutex           // counts を守る（リポジトリは並べて読む）
	counts map[string]pushed    // push で送ったコミット（prev..hash ごと。あとから変わらない）
	tops   map[string]topCache  // 作業場所 → 作業ツリーのルートと共通の git ディレクトリ（読み直しのたびに git rev-parse を呼ばない）
}

// topCache は repoOf の結果と、それを確かめた時刻・作業場所の更新時刻。
// 作業場所の更新時刻が変わったとき（git init で .git ができた、など）と、TopTTL を過ぎたときに調べ直す。
// リポジトリだった作業場所は、作業ツリーのルートか共通の git ディレクトリがなくなったときにも調べ直す
type topCache struct {
	tc    [2]string
	at    time.Time
	mtime time.Time
}

// TopTTL は、作業場所がどのリポジトリにあるかを覚えておく長さ。外から気づけない変化（親のディレクトリでの git init など）も、この長さのうちには拾う
var TopTTL = 10 * time.Minute

// workers は、git を同時に動かす数（リポジトリの数だけ動かすと、プロセスを作るのに CPU とディスクを取り合う）
func workers(n int) int { return max(1, min(n, 8, max(2, runtime.NumCPU()))) }

// each は f(0)..f(n-1) を、workers(n) 本で並べて動かし、全部が終わるまで待つ
func each(n int, f func(i int)) {
	next := make(chan int)
	var wg sync.WaitGroup
	for range workers(n) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				f(i)
			}
		}()
	}
	for i := range n {
		next <- i
	}
	close(next)
	wg.Wait()
}

// top は作業場所 p の repoOf を、覚えていればそれを使って返す（覚えた結果がまだ正しいかは、ファイルの情報だけで確かめる）
func (c *Cache) top(p string, now time.Time) (topCache, bool) {
	tc, ok := c.tops[p]
	if !ok || now.Sub(tc.at) >= TopTTL {
		return tc, false
	}
	st, err := os.Stat(p)
	if err != nil || !st.IsDir() || !st.ModTime().Equal(tc.mtime) {
		return tc, false
	}
	if tc.tc[0] != "" {
		if _, err := os.Stat(tc.tc[0]); err != nil {
			return tc, false
		}
		if _, err := os.Stat(tc.tc[1]); err != nil {
			return tc, false
		}
	}
	return tc, true
}

type repoCache struct {
	key     string
	commits []Commit
	pushes  []Push
}

func NewCache() *Cache {
	return &Cache{repos: map[string]repoCache{}, counts: map[string]pushed{}, tops: map[string]topCache{}}
}

// Collect は CollectAll と同じ。前回から変わっていないリポジトリは読み直さない。
// 時間切れになったリポジトリは前回の結果を使い（なければ何も出さない）、そのルートを stale で返す。
// どのリポジトリにあるかを調べる git rev-parse が時間切れになり、前の答えもない作業場所は、その作業場所を stale で返す。
func (c *Cache) Collect(data []*core.Session) (commits []Commit, pushes []Push, stale []string) {
	if _, err := exec.LookPath("git"); err != nil {
		return nil, nil, nil
	}
	// 作業場所 → [作業ツリーのルート, 共通の git ディレクトリ]（リポジトリでなければ空）。覚えていない作業場所だけ、並べて git rev-parse で調べる
	now, tops, seenP := time.Now(), map[string]topCache{}, map[string]bool{}
	var ask []string
	for _, d := range data {
		p := d.ProjectPath
		if p == "" || !filepath.IsAbs(p) || remotePath(runtime.GOOS, p) || seenP[p] {
			continue
		}
		seenP[p] = true
		if tc, ok := c.top(p, now); ok {
			tops[p] = tc
		} else {
			ask = append(ask, p)
		}
	}
	got := make([]topCache, len(ask))
	sure := make([]bool, len(ask))
	each(len(ask), func(i int) {
		p := ask[i]
		var mt time.Time
		if st, err := os.Stat(p); err == nil { // 調べる前の更新時刻（調べている間に変われば、次の読み直しで調べ直す）
			mt = st.ModTime()
		}
		ctx, cancel := context.WithTimeout(context.Background(), RepoTimeout)
		tc, ok := repoOf(ctx, p)
		got[i], sure[i] = topCache{tc: tc, at: now, mtime: mt}, ok
		cancel()
	})
	for i, p := range ask {
		if sure[i] {
			tops[p] = got[i]
			continue
		}
		// 時間切れ・git を動かせなかった: 答えは覚えない（次の読み直しで調べ直す）。前にリポジトリだとわかっていればそれを使い、
		// わからなければそのコミットは出せないので、ほかの時間切れと同じく stale で知らせる
		if prev, ok := c.tops[p]; ok && prev.tc[0] != "" {
			tops[p] = topCache{tc: prev.tc, mtime: prev.mtime} // at はゼロ（次の読み直しでまた調べる）
		} else {
			stale = append(stale, p)
		}
	}
	c.tops = tops               // 今回の履歴にない作業場所は忘れる
	repos := map[string]*repo{} // 共通の git ディレクトリごと。本体と git worktree は同じリポジトリとしてまとめる
	var order []string
	for _, d := range data {
		p := d.ProjectPath
		tc, ok := tops[p]
		if !ok {
			continue
		}
		top, common := tc.tc[0], tc.tc[1]
		if top == "" {
			continue
		}
		r := repos[common]
		if r == nil {
			r = &repo{top: top, project: d.Project, since: d.Start}
			repos[common] = r
			order = append(order, common)
		}
		if main := filepath.Dir(common); filepath.Base(common) == ".git" && top == main && r.top != main {
			r.top = main // 本体の作業ツリーがわかれば、そちらを使う
		}
		if filepath.Clean(p) == top && (r.rootProject == "" || top == r.top) { // ルートで作業したセッションの名前を優先する（本体のルートをいちばんに）
			r.project, r.rootProject = d.Project, top
		}
		r.since = math.Min(r.since, d.Start)
		// AI が実行したコミット：Claude Code は commit で記録する。ほかのエージェント（Codex・Kiro）は、成功した git commit の呼び出しを
		// rancommit で記録する（Claude Code も同じ呼び出しを rancommit にも持つので、commit があるセッションでは rancommit を使わない）
		hasCommit := false
		for _, o := range d.OEv {
			hasCommit = hasCommit || o.Kind == "commit"
		}
		for _, o := range d.OEv {
			if (o.Kind == "commit" || o.Kind == "rancommit" && !hasCommit) && o.T != nil {
				r.ai = append(r.ai, aiCommit{*o.T, d.ID})
			}
		}
	}
	seen := map[string]bool{}
	next := map[string]repoCache{}
	type res struct {
		rc repoCache
		ok bool
	}
	rs := make([]res, len(order)) // リポジトリは並べて読み、結果はいつも同じ順に並べる
	each(len(order), func(i int) { rc, ok := c.read(order[i], repos[order[i]]); rs[i] = res{rc, ok} })
	for i, common := range order {
		r := repos[common]
		rc, ok := rs[i].rc, rs[i].ok
		if !ok {
			stale = append(stale, r.top)
		}
		if rc.key != "" {
			next[common] = rc
		}
		pushes = append(pushes, rc.pushes...)
		for _, cm := range rc.commits {
			if seen[cm.Hash] {
				continue
			}
			seen[cm.Hash] = true
			commits = append(commits, cm)
		}
	}
	c.repos = next
	sort.SliceStable(commits, func(i, j int) bool { return commits[i].T < commits[j].T }) // 同じ秒のコミットは読んだ順（リポジトリの順、git log の順）
	sort.SliceStable(pushes, func(i, j int) bool { return pushes[i].T < pushes[j].T })
	return commits, pushes, stale
}

// read は 1 つのリポジトリを読む。印（key）が前回と同じなら、前回の結果をそのまま使う。
// 時間切れなら、途中までの結果は使わずに前回の結果を返す（ok は false）。
func (c *Cache) read(common string, r *repo) (rc repoCache, ok bool) {
	ctx, cancel := context.WithTimeout(context.Background(), RepoTimeout)
	defer cancel()
	prev := c.repos[common]
	key := repoKey(ctx, common, r)
	if ctx.Err() != nil {
		return prev, false
	}
	if prev.key == key {
		return prev, true
	}
	rc = repoCache{key: key, commits: readRepo(ctx, r)}
	rc.pushes = readPushes(ctx, r, &c.mu, c.counts)
	if ctx.Err() != nil {
		return prev, false
	}
	return rc, true
}

// repoKey は、読む結果を決めるものの印: セッションから決まる範囲、user.email、origin、
// ref と HEAD の位置（git worktree の HEAD も）、reflog の印（reflogStamp）。commit・push・fetch で ref が動けば変わる。
// ついでに r.email と r.web を決める。
func repoKey(ctx context.Context, common string, r *repo) string {
	email, _ := git(ctx, r.top, "config", "user.email")
	r.email = strings.TrimSpace(email)
	if remote, err := git(ctx, r.top, "remote", "get-url", "origin"); err == nil {
		r.web = WebURL(remote)
	}
	refs, _ := git(ctx, r.top, "show-ref", "--head") // コミットがまだないと失敗するが、空のまま使う
	wts, _ := git(ctx, r.top, "worktree", "list", "--porcelain")
	sort.SliceStable(r.ai, func(i, j int) bool { return r.ai[i].t < r.ai[j].t }) // 同じ秒の実行はセッションの順（印が読み直しごとに変わらない）
	h := sha256.New()
	fmt.Fprintf(h, "%s\x00%s\x00%v\x00%v\x00%s\x00%s\x00%s\x00%s\x00%s", r.top, r.project, r.since, r.ai, r.email, r.web, refs, wts, reflogStamp(common))
	return hex.EncodeToString(h.Sum(nil))
}

// reflogStamp は reflog のファイルの大きさと更新時刻の印。git は呼ばない（読み直しのたびに呼び出しを増やさないため）。
// ref の位置だけでは気づけない変化に気づくため:
//   - リモート追跡ブランチの reflog（logs/refs/remotes）: push の記録が増えたのに位置は前に見たのと同じ、
//     期限切れの行が消えた（git gc・git reflog expire）
//   - HEAD の reflog（本体と git worktree の logs/HEAD）: どの ref にもない detached HEAD でのコミット
//
// reftable 形式のリポジトリは reflog も reftable/ にあるので、その一覧（tables.list）を見る。
func reflogStamp(common string) string {
	if common == "" {
		return ""
	}
	var sb strings.Builder
	add := func(p string, fi fs.FileInfo) {
		fmt.Fprintf(&sb, "%s\x00%d\x00%d\n", p, fi.Size(), fi.ModTime().UnixNano())
	}
	filepath.WalkDir(filepath.Join(common, "logs", "refs", "remotes"), func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if fi, err := d.Info(); err == nil {
				add(p, fi)
			}
		}
		return nil // 読めないところは飛ばす
	})
	files := []string{filepath.Join(common, "logs", "HEAD"), filepath.Join(common, "reftable", "tables.list")}
	wts, _ := filepath.Glob(filepath.Join(common, "worktrees", "*", "logs", "HEAD"))
	sort.Strings(wts)
	for _, p := range append(files, wts...) {
		if fi, err := os.Stat(p); err == nil {
			add(p, fi)
		}
	}
	return sb.String()
}

// maxPushTime は push の時刻として受け入れる最後の時刻（9999-12-31T23:59:59Z）。reflog の時刻は書き換えられる。
// 負の値や大きすぎる値（git は 2^64 で折り返して 1.8e19 のように出す）を画面に渡すと、日付が作れない。
const maxPushTime = 253402300799

// pushTimeOK は reflog の時刻が push の時刻として使えるか（0 以上、maxPushTime 以下、有限）。
func pushTimeOK(t float64) bool {
	return !math.IsNaN(t) && t >= 0 && t <= maxPushTime
}

// readPushes はリモート追跡ブランチの reflog から「update by push」の行を読む。repoKey のあと（r.web が決まってから）に呼ぶ。
// pushed は、1 回の push で送ったコミットの数と、そのうち先頭 MaxPushHashes 件。
type pushed struct {
	n      int
	hashes []string
}

// 送ったコミットは counts に覚えておき、同じ push のために git rev-list を何度も呼ばない。
func readPushes(ctx context.Context, r *repo, mu *sync.Mutex, counts map[string]pushed) []Push {
	refs, err := git(ctx, r.top, "for-each-ref", "--format=%(refname)", "refs/remotes")
	if err != nil {
		return nil
	}
	var out []Push
	for i, ref := range strings.Fields(refs) {
		if i >= MaxPushRefs {
			break
		}
		if strings.HasSuffix(ref, "/HEAD") {
			continue
		}
		raw, err := git(ctx, r.top, "reflog", "show", "--date=unix", "--format=%H%x1f%gd%x1f%gs", ref, "--")
		if err != nil {
			continue
		}
		lines := strings.Split(strings.TrimSpace(raw), "\n") // 新しい順
		for j := len(lines) - 1; j >= 0; j-- {               // 古い順に足す（同じ秒の push も順番どおりに並ぶよう）
			line := lines[j]
			f := strings.SplitN(line, "\x1f", 3)
			if len(f) < 3 || !strings.HasPrefix(f[2], "update by push") {
				continue
			}
			at := strings.LastIndex(f[1], "@{")
			if at < 0 {
				continue
			}
			t, err := strconv.ParseFloat(strings.TrimSuffix(f[1][at+2:], "}"), 64)
			if err != nil || !pushTimeOK(t) || t < r.since-86400 {
				continue
			}
			p := Push{T: t, Project: r.project, Repo: r.top, Ref: branchOf(ref), Hash: f[0], URL: commitURL(r.web, f[0])}
			if j+1 < len(lines) { // ひとつ前の位置から、送ったコミットの数を数える
				if prev := strings.SplitN(lines[j+1], "\x1f", 2)[0]; prev != "" && prev != f[0] {
					rng := prev + ".." + f[0]
					p.Prev = prev
					mu.Lock()
					c, ok := counts[rng]
					mu.Unlock()
					if ok {
						p.Commits, p.Hashes = c.n, c.hashes
					} else if out, err := git(ctx, r.top, "rev-list", rng); err == nil {
						hs := strings.Fields(out)
						c := pushed{n: len(hs), hashes: hs[:min(len(hs), MaxPushHashes)]}
						mu.Lock()
						counts[rng] = c
						mu.Unlock()
						p.Commits, p.Hashes = c.n, c.hashes
					}
				}
			}
			if p.Prev != "" {
				p.Cmd = core.GitCmd(r.top, "log --oneline", abbrev(p.Prev)+".."+abbrev(p.Hash))
			} else {
				p.Cmd = core.GitCmd(r.top, core.GitShow, abbrev(p.Hash))
			}
			out = append(out, p)
		}
	}
	return out
}

// readRepo は repoKey のあと（r.email と r.web が決まってから）に呼ぶ。
func readRepo(ctx context.Context, r *repo) []Commit {
	args := []string{"log", "--all", "--source", "--no-merges", "--no-color", "-n", strconv.Itoa(MaxPerRepo),
		"--since=@" + strconv.FormatInt(int64(r.since)-86400, 10), "--format=%x1e%H%x1f%at%x1f%S%x1f%s%x1f%b%x1d", "--numstat"}
	if r.email != "" {
		args = append(args, "--author="+r.email)
	}
	raw, err := git(ctx, r.top, args...)
	if err != nil && ctx.Err() == nil { // 部分クローンで手元にないファイルがあると、変更行数は数えられない。コミットだけは読む
		raw, err = git(ctx, r.top, slices.DeleteFunc(args, func(a string) bool { return a == "--numstat" })...)
	}
	if err != nil {
		return nil
	}
	var out []Commit
	for _, rec := range strings.Split(raw, "\x1e") {
		head, stat, ok := strings.Cut(rec, "\x1d")
		if !ok {
			continue
		}
		f := strings.SplitN(head, "\x1f", 5)
		if len(f) < 5 {
			continue
		}
		t, err := strconv.ParseFloat(f[1], 64)
		if err != nil {
			continue
		}
		c := Commit{Hash: f[0], URL: commitURL(r.web, f[0]), T: t, Project: r.project, Repo: r.top, Branch: branchOf(f[2]), Subject: core.Runes(f[3], 160),
			Body: core.Runes(strings.TrimSpace(f[4]), 600), Files: []FileStat{}, Cmd: core.GitCmd(r.top, core.GitShow, f[0])}
		for _, line := range strings.Split(stat, "\n") {
			cols := strings.SplitN(line, "\t", 3)
			if len(cols) != 3 {
				continue
			}
			fs := FileStat{Path: cols[2], URL: fileURL(r.web, f[0], cols[2]), Added: -1, Removed: -1}
			if a, err := strconv.Atoi(cols[0]); err == nil {
				fs.Added = a
				c.Added += a
			}
			if d, err := strconv.Atoi(cols[1]); err == nil {
				fs.Removed = d
				c.Removed += d
			}
			c.NFiles++
			if len(c.Files) < MaxFiles {
				c.Files = append(c.Files, fs)
			}
		}
		out = append(out, c)
	}
	// エージェントが実行したコミット（前後 2 分）と結びつける。1 回の実行は、いちばん近い 1 つのコミットにだけ結びつける
	// （近くで手で行ったコミットまで、AI のコミットとしないため）
	for _, a := range r.ai {
		best := -1
		for i := range out {
			if d := math.Abs(out[i].T - a.t); d <= 120 && !out[i].AI && (best < 0 || d < math.Abs(out[best].T-a.t)) {
				best = i
			}
		}
		if best >= 0 {
			out[best].AI, out[best].Session = true, a.session
		}
	}
	slices.Reverse(out) // git log は新しい順。古い順にしておけば、同じ秒のコミットも親が先に並ぶ（Collect の並べ替えは安定）
	return out
}

// branchOf は git log --source の ref をブランチ名らしく短くする（refs/heads/x → x、refs/remotes/origin/x → origin/x）。
func branchOf(ref string) string {
	ref = strings.TrimSpace(ref)
	for _, p := range []string{"refs/heads/", "refs/remotes/", "refs/"} {
		if strings.HasPrefix(ref, p) {
			return strings.TrimPrefix(ref, p)
		}
	}
	return ref
}

// WebURL はリモートの URL を Web で開ける URL にする（git@host:owner/repo.git → https://host/owner/repo）。
// ローカルのパスなど、Web にならないものは ""。
func WebURL(remote string) string {
	r := strings.TrimSpace(remote)
	switch {
	case strings.HasPrefix(r, "git@"): // git@github.com:owner/repo.git
		host, path, ok := strings.Cut(strings.TrimPrefix(r, "git@"), ":")
		if !ok {
			return ""
		}
		r = "https://" + host + "/" + path
	case strings.HasPrefix(r, "ssh://"): // ssh://git@host:22/owner/repo.git
		host, path, ok := strings.Cut(dropUserinfo(strings.TrimPrefix(r, "ssh://")), "/")
		if !ok {
			return ""
		}
		if h, _, ok := strings.Cut(host, ":"); ok {
			host = h
		}
		r = "https://" + host + "/" + path
	case strings.HasPrefix(r, "https://"), strings.HasPrefix(r, "http://"):
		scheme, rest, _ := strings.Cut(r, "://")
		r = scheme + "://" + dropUserinfo(rest) // https://user:token@host/... の資格情報は落とす
	default:
		return ""
	}
	r = strings.TrimSuffix(strings.TrimSuffix(r, "/"), ".git")
	_, rest, _ := strings.Cut(r, "://")
	if host, _, _ := strings.Cut(rest, "/"); host == "" || strings.Contains(host, "@") { // ホストがない・読みとれない
		return ""
	}
	return r
}

// dropUserinfo は host/path の前の user:password@ を落とす。ユーザー名に @ が入っていても
// （user@corp:token@host）資格情報が残らないよう、ホストの直前の @ までを落とす。
func dropUserinfo(rest string) string {
	host, _, _ := strings.Cut(rest, "/")
	if i := strings.LastIndex(host, "@"); i >= 0 {
		return rest[i+1:]
	}
	return rest
}

func isBitbucket(web string) bool { return strings.Contains(web, "://bitbucket.org/") }

func commitURL(web, hash string) string {
	if web == "" {
		return ""
	}
	if isBitbucket(web) {
		return web + "/commits/" + hash
	}
	return web + "/commit/" + hash // GitHub・GitLab（/-/commit へ転送される）・Gitea など
}

func fileURL(web, hash, path string) string {
	if web == "" || strings.Contains(path, "=>") { // 名前の変更（a => b）は 1 つのパスにならない
		return ""
	}
	if isBitbucket(web) {
		return web + "/src/" + hash + "/" + path
	}
	return web + "/blob/" + hash + "/" + path
}

// abbrev は、貼るコマンドに入れるハッシュの先頭 12 文字。
func abbrev(h string) string { return h[:min(len(h), 12)] }
