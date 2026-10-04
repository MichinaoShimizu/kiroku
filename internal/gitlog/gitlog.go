// Package gitlog は、エージェントが作業したプロジェクトのローカル git リポジトリからコミットを読む。
// 外部には何も送らない。git がない環境や、リポジトリでない場所は黙って飛ばす。
package gitlog

import (
	"context"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/MichinaoShimizu/kiroku/internal/core"
)

// Commit は 1 つのコミット。
type Commit struct {
	Hash    string  `json:"hash"`    // 短いハッシュ
	T       float64 `json:"t"`       // 作成日時（author date、UNIX 秒）
	Project string  `json:"project"` // セッションと同じプロジェクト名
	Subject string  `json:"subject"`
	Added   int     `json:"added"`
	Removed int     `json:"removed"`
	AI      bool    `json:"ai"` // エージェントがツールで実行したコミット（時刻が 2 分以内で一致）
}

// MaxPerRepo は 1 つのリポジトリから読むコミットの上限。
const MaxPerRepo = 5000

// git は外から差しかえられる（テスト用）。
var git = func(ctx context.Context, dir string, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...).Output()
	return string(out), err
}

type repo struct {
	top, project string
	since        float64
	aiTimes      []float64
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
				r.aiTimes = append(r.aiTimes, *o.T)
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

var statRe = regexp.MustCompile(`(\d+) insertion|(\d+) deletion`)

func readRepo(ctx context.Context, r *repo) []Commit {
	email, _ := git(ctx, r.top, "config", "user.email")
	args := []string{"log", "--all", "--no-merges", "--no-color", "-n", strconv.Itoa(MaxPerRepo),
		"--since=@" + strconv.FormatInt(int64(r.since)-86400, 10), "--format=%x1e%h%x1f%at%x1f%s", "--shortstat"}
	if e := strings.TrimSpace(email); e != "" {
		args = append(args, "--author="+e)
	}
	raw, err := git(ctx, r.top, args...)
	if err != nil {
		return nil
	}
	sort.Float64s(r.aiTimes)
	var out []Commit
	for _, rec := range strings.Split(raw, "\x1e") {
		rec = strings.TrimSpace(rec)
		if rec == "" {
			continue
		}
		head, stat, _ := strings.Cut(rec, "\n")
		f := strings.SplitN(head, "\x1f", 3)
		if len(f) < 3 {
			continue
		}
		t, err := strconv.ParseFloat(f[1], 64)
		if err != nil {
			continue
		}
		c := Commit{Hash: f[0], T: t, Project: r.project, Subject: core.Runes(f[2], 120)}
		for _, m := range statRe.FindAllStringSubmatch(stat, -1) {
			if m[1] != "" {
				c.Added, _ = strconv.Atoi(m[1])
			}
			if m[2] != "" {
				c.Removed, _ = strconv.Atoi(m[2])
			}
		}
		i := sort.SearchFloat64s(r.aiTimes, t-120)
		c.AI = i < len(r.aiTimes) && r.aiTimes[i] <= t+120
		out = append(out, c)
	}
	return out
}
