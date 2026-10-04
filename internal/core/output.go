package core

import (
	"regexp"
	"strings"
)

// Output は AI が実行して形になったアウトプット（コミット・PR の作成・変更した行）。
// 価値や生産性ではなく、出したものの量として数える。
type Output struct {
	T    *float64
	Kind string // commit / pr / added / removed
	V    float64
	URL  string // PR の URL（ツールの結果からわかったとき）
}

// OutputTotal はセッションや期間のアウトプットの合計。
type OutputTotal struct {
	Commits float64 `json:"commits"`
	PRs     float64 `json:"prs"`
	Added   float64 `json:"added"`   // 追加した行（編集の前後を比べた目安）
	Removed float64 `json:"removed"` // 削除した行
}

func (o *OutputTotal) Add(x Output) {
	switch x.Kind {
	case "commit":
		o.Commits += x.V
	case "pr":
		o.PRs += x.V
	case "added":
		o.Added += x.V
	case "removed":
		o.Removed += x.V
	}
}

// Empty は何も記録がないとき true。
func (o OutputTotal) Empty() bool {
	return o.Commits == 0 && o.PRs == 0 && o.Added == 0 && o.Removed == 0
}

var (
	commitRe = regexp.MustCompile(`(^|[;&|(\s])git(\s+-[Cc]\s+\S+)*\s+commit\b`)
	prRe     = regexp.MustCompile(`(^|[;&|(\s])gh\s+pr\s+create\b`)
)

// Outputs は 1 回のツール呼び出しがアウトプットにあたるかを調べる（成功したかは呼び出し側が確かめる）。
//   - シェル: git commit（--dry-run を除く）・gh pr create
//   - PR を作るツール（名前が create_pull_request で終わるもの。GitHub の MCP など）
//   - ファイルの編集・作成: 変更前後の行を比べた追加・削除の行数
func Outputs(name string, input any, t *float64) []Output {
	in := Map(input)
	var out []Output
	add := func(kind string, v float64) {
		if v > 0 {
			out = append(out, Output{T: t, Kind: kind, V: v})
		}
	}
	switch {
	case name == "Bash" || name == "shell" || name == "execute_bash":
		cmd := Str(in["command"])
		if commitRe.MatchString(cmd) && !strings.Contains(cmd, "--dry-run") {
			add("commit", 1)
		}
		if prRe.MatchString(cmd) {
			add("pr", 1)
		}
	case strings.HasSuffix(name, "create_pull_request"):
		add("pr", 1)
	case name == "Edit":
		a, r := lineDiff(Str(in["old_string"]), Str(in["new_string"]))
		add("added", a)
		add("removed", r)
	case name == "MultiEdit":
		for _, e := range List(in["edits"]) {
			em := Map(e)
			a, r := lineDiff(Str(em["old_string"]), Str(em["new_string"]))
			add("added", a)
			add("removed", r)
		}
	case name == "Write":
		add("added", float64(len(lines(Str(in["content"])))))
	}
	return out
}

func lines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(s, "\n"), "\n")
}

// lineDiff は前後で共通する先頭・末尾の行を除き、残りを削除・追加の行数とみなす（diff の目安）。
func lineDiff(before, after string) (added, removed float64) {
	a, b := lines(before), lines(after)
	for len(a) > 0 && len(b) > 0 && a[0] == b[0] {
		a, b = a[1:], b[1:]
	}
	for len(a) > 0 && len(b) > 0 && a[len(a)-1] == b[len(b)-1] {
		a, b = a[:len(a)-1], b[:len(b)-1]
	}
	return float64(len(b)), float64(len(a))
}

var prURL = regexp.MustCompile(`https?://[^\s"'<>()\[\]\\]+/(?:pull|pulls|-/merge_requests|merge_requests|pull-requests)/\d+`)

// PRURL はツールの結果の文字列から、PR（マージリクエスト）の URL を探す。なければ ""。
func PRURL(result string) string {
	return prURL.FindString(result)
}
