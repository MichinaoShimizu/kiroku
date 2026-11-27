package core

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// Output は AI が実行して形になったアウトプット（コミット・PR の作成・マージ・レビュー・変更した行）。
// 価値や生産性ではなく、出したものの量として数える。
type Output struct {
	T    *float64
	Kind string // commit / pr / merge / review / approve / added / removed（approve は review の内訳で、両方に入る）
	V    float64
	URL  string // PR の URL（ツールの結果からわかったとき）
	Ref  string // マージ・レビューした PR（「#11」か「repo#11」。わかったとき）
}

// OutputTotal はセッションや期間のアウトプットの合計。
type OutputTotal struct {
	Commits   float64 `json:"commits"`
	PRs       float64 `json:"prs"`
	Merges    float64 `json:"merges,omitempty"`    // AI がマージした PR
	Reviews   float64 `json:"reviews,omitempty"`   // AI が出したレビュー（アプルーブ・変更の依頼・コメント）
	Approvals float64 `json:"approvals,omitempty"` // そのうちアプルーブ
	GitHubOps float64 `json:"ghOps,omitempty"`
	// AI がそのセッションで実行して成功した git commit・git push（どのエージェントでも。Commits は Claude Code が実行を記録したコミットで、別に数える）
	RanCommits float64 `json:"ranCommits,omitempty"`
	RanPushes  float64 `json:"ranPushes,omitempty"` // そのほかの GitHub の書き込み（PR・issue のコメント、issue の作成、PR・issue を閉じる・編集するなど）
	Added      float64 `json:"added"`               // 追加した行（編集の前後を比べた目安）
	Removed    float64 `json:"removed"`             // 削除した行
}

func (o *OutputTotal) Add(x Output) {
	switch x.Kind {
	case "commit":
		o.Commits += x.V
	case "pr":
		o.PRs += x.V
	case "merge":
		o.Merges += x.V
	case "review":
		o.Reviews += x.V
	case "approve":
		o.Approvals += x.V
	case "ghop":
		o.GitHubOps += x.V
	case "rancommit":
		o.RanCommits += x.V
	case "ranpush":
		o.RanPushes += x.V
	case "added":
		o.Added += x.V
	case "removed":
		o.Removed += x.V
	}
}

var (
	commitRe  = regexp.MustCompile(`(^|[;&|(\s])git(\s+-[Cc]\s+\S+)*\s+commit\b`)
	autoRe    = regexp.MustCompile(`\s--(disable-)?auto\b`) // gh pr merge --auto は自動マージの予約で、マージではない
	helpRe    = regexp.MustCompile(`\s(--help|-h)\b`)       // 使い方を見ただけ
	approveRe = regexp.MustCompile(`\s(--approve|-a)\b`)
)

// Outputs は 1 回のツール呼び出しがアウトプットにあたるかを調べる（成功したかは呼び出し側が確かめる）。
//   - シェル: git commit（--dry-run を除く）
//   - PR の操作（PRActions）: 作成・マージ・レビュー
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
	for _, a := range PRActions(name, input) {
		out = append(out, Output{T: t, Kind: a.Kind, V: 1, Ref: a.Ref})
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

// PRAction は PR への操作 1 つ。Kind は pr（作成）・merge・review・approve（承認。review と一緒に返す）。
// Ref は、わかれば相手の PR（「#11」か「repo#11」。コマンドの引数や MCP の pullNumber から。ブランチ名などは入れない）。
type PRAction struct {
	Kind, Ref string
}

// PRActions は、ツールの呼び出しが PR に何をするものかを、エージェントによらず書いた順に返す（成功したかは呼び出し側が確かめる）。
// PR の操作でなければ nil。
//   - シェルのコマンド（command は文か配列（Codex の shell・local_shell）、exec_command は cmd）を &&・||・;・|・改行で
//     区切り、それぞれの頭の gh pr create・gh pr merge（--auto・--disable-auto は自動マージの予約なので除く）・
//     gh pr review（--approve・-a は承認）を見る。--help・-h は使い方を見ただけなので除く
//   - ツールの名前（GitHub の MCP など。Codex の MCP は「サーバー__ツール」の名前になる）:
//     create_pull_request・merge_pull_request、レビューを出すもの（pull_request_review_write の create（event つき）と
//     submit_pending、create_pull_request_review（event つき）、submit_pending_pull_request_review）。event が APPROVE なら承認
func PRActions(name string, input any) []PRAction {
	in := Map(input)
	review := func(approve bool, ref string) []PRAction {
		if approve {
			return []PRAction{{"review", ref}, {"approve", ref}}
		}
		return []PRAction{{"review", ref}}
	}
	event := strings.ToUpper(Str(in["event"]))
	ref := mcpPRRef(in)
	switch {
	case strings.HasSuffix(name, "create_pull_request"):
		return []PRAction{{"pr", ""}}
	case strings.HasSuffix(name, "merge_pull_request"):
		return []PRAction{{"merge", ref}}
	case strings.HasSuffix(name, "pull_request_review_write"):
		if m := Str(in["method"]); m == "submit_pending" || m == "create" && event != "" {
			return review(event == "APPROVE", ref)
		}
		return nil
	case strings.HasSuffix(name, "submit_pending_pull_request_review"),
		strings.HasSuffix(name, "create_pull_request_review") && event != "":
		return review(event == "APPROVE", ref)
	case ghWriteTool(name):
		return []PRAction{{"ghop", ref}}
	}
	if !shellTool(name) {
		return nil
	}
	var out []PRAction
	for _, seg := range cmdSplit.Split(CommandText(input), -1) {
		m := ghPRCmd.FindStringSubmatch(seg)
		if m == nil || helpRe.MatchString(" "+m[2]) {
			continue
		}
		args := " " + m[2]
		switch m[1] {
		case "create":
			out = append(out, PRAction{"pr", ""})
		case "merge":
			if !autoRe.MatchString(args) {
				out = append(out, PRAction{"merge", ghPRRef(m[2])})
			}
		case "review":
			out = append(out, review(approveRe.MatchString(args), ghPRRef(m[2]))...)
		}
	}
	for _, seg := range cmdSplit.Split(CommandText(input), -1) {
		if m := ghWriteCmd.FindStringSubmatch(seg); m != nil && !helpRe.MatchString(" "+m[1]) {
			out = append(out, PRAction{"ghop", ""})
		}
		// git commit・git push（コマンドの頭にあるものだけ。--dry-run・-n と --help は除く）
		if m := gitRunCmd.FindStringSubmatch(seg); m != nil && !dryRe.MatchString(" "+m[2]) && !helpRe.MatchString(" "+m[2]) && !(m[1] == "push" && pushDryRe.MatchString(" "+m[2])) {
			out = append(out, PRAction{"ran" + m[1], ""})
		}
	}
	return out
}

// ghWriteTool は、PR の作成・マージ・レビューのほかに GitHub へ書き込むツールの名前か（GitHub の MCP の
// コメント・issue の作成と更新・PR の更新など）。読むだけのツール（get_・list_・search_）は入れない
func ghWriteTool(name string) bool {
	for _, suf := range []string{"add_issue_comment", "add_reply_to_pull_request_comment", "update_issue_comment", "create_issue", "update_issue", "issue_write", "sub_issue_write", "update_pull_request", "update_pull_request_branch"} {
		if strings.HasSuffix(name, suf) {
			return true
		}
	}
	return false
}

var (
	// cmdSplit は、シェルのコマンドを 1 つずつに分ける区切り（&&・||・;・|・&・改行・括弧）
	cmdSplit = regexp.MustCompile(`&&|\|\||[;|&\n()]`)
	// ghPRCmd は、1 つのコマンドの頭の gh pr create・merge・review（VAR=値 の前置きは可）と、その引数
	// ghWriteCmd は、1 つのコマンドの頭の gh pr・gh issue のうち、GitHub に書き込むもの（作成・マージ・レビューは上で数える）
	// gitRunCmd は、1 つのコマンドの頭の git commit・git push（-C と -c の前置きは可）と、その引数
	gitRunCmd  = regexp.MustCompile(`^\s*(?:[A-Za-z_][A-Za-z0-9_]*=\S*\s+)*git(?:\s+-[Cc]\s+\S+)*\s+(commit|push)\b(.*)$`)
	dryRe      = regexp.MustCompile(`\s--dry-run\b`)
	pushDryRe  = regexp.MustCompile(`\s-n\b`)
	ghWriteCmd = regexp.MustCompile(`^\s*(?:[A-Za-z_][A-Za-z0-9_]*=\S*\s+)*gh\s+(?:pr\s+(?:comment|close|reopen|edit|ready)|issue\s+(?:create|comment|close|reopen|edit))\b(.*)$`)
	ghPRCmd    = regexp.MustCompile(`^\s*(?:[A-Za-z_][A-Za-z0-9_]*=\S*\s+)*gh\s+pr\s+(create|merge|review)\b(.*)$`)
	prNumRe    = regexp.MustCompile(`^#?(\d{1,9})$`)
	prURLRe    = regexp.MustCompile(`^https?://[^/\s]+/[^/\s]+/([A-Za-z0-9._-]{1,100})/pull/(\d{1,9})(?:[/?#].*)?$`)
	repoArg    = regexp.MustCompile(`(?:^|\s)(?:-R|--repo)(?:\s+|=)['"]?[A-Za-z0-9._-]{1,100}/([A-Za-z0-9._-]{1,100})['"]?(?:\s|$)`)
	repoName   = regexp.MustCompile(`^[A-Za-z0-9._-]{1,100}$`)
)

// ghPRRef は gh pr merge・review の引数から相手の PR を「#11」か「repo#11」にする（番号か PR の URL のときだけ。
// ブランチ名や引数なし（今のブランチの PR）は ""）。
func ghPRRef(args string) string {
	repo := ""
	if m := repoArg.FindStringSubmatch(args); m != nil {
		repo = m[1]
	}
	for _, f := range strings.Fields(args) {
		f = strings.Trim(f, `'"`)
		if m := prURLRe.FindStringSubmatch(f); m != nil {
			return m[1] + "#" + m[2]
		}
		if m := prNumRe.FindStringSubmatch(f); m != nil {
			return repo + "#" + m[1]
		}
	}
	return ""
}

// mcpPRRef は GitHub の MCP の引数（repo・pullNumber）から相手の PR を「repo#11」にする（わからなければ ""）。
func mcpPRRef(in map[string]any) string {
	n, ok := Num(in["pullNumber"])
	if !ok {
		n, ok = Num(in["pull_number"])
	}
	if !ok || n < 1 || n > 999999999 || n != float64(int64(n)) {
		return ""
	}
	repo := Str(in["repo"])
	if !repoName.MatchString(repo) {
		repo = ""
	}
	return fmt.Sprintf("%s#%d", repo, int64(n))
}

// shellTool は、シェルのコマンドを動かすツールの名前か（Claude Code の Bash・PowerShell、Codex の shell・shell_command・
// exec_command・local_shell、Kiro の execute_bash・execute_cmd など）。Kiro IDE / CLI (V3) のシェルの名前は文書にないので、
// 名前に bash・shell・exec・cmd・powershell を含むものにする。apply_patch など、ほかのツールの command は見ない。
func shellTool(name string) bool {
	n := strings.ToLower(name)
	for _, k := range []string{"bash", "shell", "exec", "cmd", "powershell"} {
		if strings.Contains(n, k) {
			return true
		}
	}
	return false
}

// CommandText は、シェルのツールの引数からコマンドの文を取り出す（command の文・配列、cmd の文。なければ ""）。
// 配列が [bash -lc 文] のような形なら、その文。apply_patch の配列（パッチの本文）はコマンドではないので "" にする。
func CommandText(input any) string {
	in := Map(input)
	if cmd := Str(in["cmd"]); cmd != "" {
		return cmd
	}
	if cmd := Str(in["command"]); cmd != "" {
		return cmd
	}
	var argv []string
	for _, a := range List(in["command"]) {
		argv = append(argv, Str(a))
	}
	if len(argv) > 0 && (argv[0] == "apply_patch" || argv[0] == "applypatch") {
		return ""
	}
	if n := len(argv); n >= 3 && strings.HasPrefix(argv[n-2], "-") && strings.HasSuffix(argv[n-2], "c") {
		return argv[n-1] // bash -lc "…"・sh -c "…"
	}
	return strings.Join(argv, " ")
}

// exitCodeRe は、シェルの結果の文に書かれた終了コード（「Exit code: N」・「Process exited with code N」）。
var exitCodeRe = regexp.MustCompile(`(?m)^(?:Exit code:|Process exited with code)\s*(-?\d+)`)

// ResultFailed は、ツールの結果の文が失敗を示すかを、エージェントによらず調べる。
//   - JSON の結果: metadata.exit_code・exit_code・exitCode・exit_status（数か文）が 0 でない、または success が false（真偽値か文）
//   - 文の結果: 「Exit code: N」・「Process exited with code N」の N が 0 でない
//   - Codex の exec_command がまだ動いている（「Process running with session ID」）: 終わったかわからないので失敗に数える
//
// 失敗とわからないものは失敗にしない（Claude Code の is_error がない結果と同じ）。
func ResultFailed(text string) bool {
	t := strings.TrimSpace(text)
	if strings.HasPrefix(t, "{") {
		d := json.NewDecoder(strings.NewReader(t))
		d.UseNumber() // 1e400 のような数でも読めるように（読めないと失敗を見落とす）
		var m map[string]any
		if d.Decode(&m) == nil {
			for _, v := range []any{Get(m, "metadata", "exit_code"), m["exit_code"], m["exitCode"], m["exit_status"]} {
				if v == nil {
					continue
				}
				if code := strings.TrimSpace(fmt.Sprint(v)); code != "0" && code != "" {
					return true
				}
			}
			if Falsy(m["success"]) {
				return true
			}
		}
	}
	if m := exitCodeRe.FindStringSubmatch(t); m != nil && m[1] != "0" {
		return true
	}
	return strings.HasPrefix(t, "Process running with session ID") || strings.Contains(t, "\nProcess running with session ID")
}

// Falsy は、success のような値が false（真偽値か "false" の文）か。
func Falsy(v any) bool {
	switch x := v.(type) {
	case bool:
		return !x
	case string:
		return strings.EqualFold(strings.TrimSpace(x), "false")
	}
	return false
}

var prURL = regexp.MustCompile(`https?://[^\s"'<>()\[\]\\]+/(?:pull|pulls|-/merge_requests|merge_requests|pull-requests)/\d+`)

// PRURL はツールの結果の文字列から、PR（マージリクエスト）の URL を探す。なければ ""。
// 向きを変える文字（U+202E など）や幅のない文字（U+200B など、Unicode の Cf）を含むものは、見た目と行き先が
// 食い違うので使わない（URL のない PR として数える）。
func PRURL(result string) string {
	u := prURL.FindString(result)
	if strings.IndexFunc(u, func(r rune) bool { return unicode.Is(unicode.Cf, r) }) >= 0 {
		return ""
	}
	return u
}
