// Package gitlog は、エージェントが作業したプロジェクトのローカル git リポジトリからコミットを読む。
// 外部には何も送らない。git がない環境や、リポジトリでない場所は黙って飛ばす。
package gitlog

import (
	"context"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
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
	Added   int        `json:"added"`
	Removed int        `json:"removed"`
	Files   []FileStat `json:"files"`             // 変更したファイル（先頭 MaxFiles 件）
	NFiles  int        `json:"nFiles"`            // 変更したファイルの数
	AI      bool       `json:"ai"`                // エージェントがツールで実行したコミット（時刻が 2 分以内で一致）
	Session string     `json:"session,omitempty"` // AI が実行したときの、そのセッションの ID
}

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
	out, err := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...).Output()
	return string(out), err
}

type aiCommit struct {
	t       float64
	session string
}

type repo struct {
	top, project string
	web          string // リモートの Web の URL（https://github.com/owner/repo など）。わからなければ ""
	since        float64
	ai           []aiCommit
}

// Collect はセッションの作業場所にあるリポジトリから、利用者自身（user.email）のコミットを読む。
// 期間は、そのリポジトリでの最初のセッションの 1 日前から。
func Collect(data []*core.Session) []Commit {
	if _, err := exec.LookPath("git"); err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	tops := map[string]string{} // 作業場所 → リポジトリのルート（なければ ""）
	repos := map[string]*repo{}
	var order []string
	for _, d := range data {
		p := d.ProjectPath
		if p == "" || !filepath.IsAbs(p) {
			continue
		}
		top, ok := tops[p]
		if !ok {
			if st, err := os.Stat(p); err == nil && st.IsDir() {
				out, err := git(ctx, p, "rev-parse", "--show-toplevel")
				if err == nil {
					top = filepath.Clean(strings.TrimSpace(out))
				}
			}
			tops[p] = top
		}
		if top == "" {
			continue
		}
		r := repos[top]
		if r == nil {
			r = &repo{top: top, project: d.Project, since: d.Start}
			repos[top] = r
			order = append(order, top)
		}
		if filepath.Clean(p) == top { // ルートで作業したセッションの名前を優先する
			r.project = d.Project
		}
		r.since = math.Min(r.since, d.Start)
		for _, o := range d.OEv {
			if o.Kind == "commit" && o.T != nil {
				r.ai = append(r.ai, aiCommit{*o.T, d.ID})
			}
		}
	}
	seen := map[string]bool{}
	var out []Commit
	for _, top := range order {
		r := repos[top]
		cs := readRepo(ctx, r)
		for _, c := range cs {
			if seen[c.Hash] {
				continue
			}
			seen[c.Hash] = true
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].T < out[j].T })
	return out
}

func readRepo(ctx context.Context, r *repo) []Commit {
	email, _ := git(ctx, r.top, "config", "user.email")
	if remote, err := git(ctx, r.top, "remote", "get-url", "origin"); err == nil {
		r.web = WebURL(remote)
	}
	args := []string{"log", "--all", "--source", "--no-merges", "--no-color", "-n", strconv.Itoa(MaxPerRepo),
		"--since=@" + strconv.FormatInt(int64(r.since)-86400, 10), "--format=%x1e%H%x1f%at%x1f%S%x1f%s%x1f%b%x1d", "--numstat"}
	if e := strings.TrimSpace(email); e != "" {
		args = append(args, "--author="+e)
	}
	raw, err := git(ctx, r.top, args...)
	if err != nil {
		return nil
	}
	sort.Slice(r.ai, func(i, j int) bool { return r.ai[i].t < r.ai[j].t })
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
			Body: core.Runes(strings.TrimSpace(f[4]), 600), Files: []FileStat{}}
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
		i := sort.Search(len(r.ai), func(i int) bool { return r.ai[i].t >= t-120 })
		if i < len(r.ai) && r.ai[i].t <= t+120 {
			c.AI, c.Session = true, r.ai[i].session
		}
		out = append(out, c)
	}
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
		rest := strings.TrimPrefix(r, "ssh://")
		if i := strings.Index(rest, "@"); i >= 0 {
			rest = rest[i+1:]
		}
		host, path, ok := strings.Cut(rest, "/")
		if !ok {
			return ""
		}
		if h, _, ok := strings.Cut(host, ":"); ok {
			host = h
		}
		r = "https://" + host + "/" + path
	case strings.HasPrefix(r, "https://"), strings.HasPrefix(r, "http://"):
		scheme, rest, _ := strings.Cut(r, "://")
		if i := strings.Index(rest, "@"); i >= 0 && i < strings.Index(rest+"/", "/") { // https://user:token@host/... の資格情報は落とす
			rest = rest[i+1:]
		}
		r = scheme + "://" + rest
	default:
		return ""
	}
	return strings.TrimSuffix(strings.TrimSuffix(r, "/"), ".git")
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
