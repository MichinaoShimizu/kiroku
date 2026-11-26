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
}

// OutputTotal はセッションや期間のアウトプットの合計。
type OutputTotal struct {
	Commits   float64 `json:"commits"`
	PRs       float64 `json:"prs"`
	Merges    float64 `json:"merges,omitempty"`    // AI がマージした PR
	Reviews   float64 `json:"reviews,omitempty"`   // AI が出したレビュー（アプルーブ・変更の依頼・コメント）
	Approvals float64 `json:"approvals,omitempty"` // そのうちアプルーブ
	Added     float64 `json:"added"`               // 追加した行（編集の前後を比べた目安）
	Removed   float64 `json:"removed"`             // 削除した行
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
	case "added":
		o.Added += x.V
	case "removed":
		o.Removed += x.V
	}
}

var (
	commitRe = regexp.MustCompile(`(^|[;&|(\s])git(\s+-[Cc]\s+\S+)*\s+commit\b`)
	// gh pr …: コマンドの頭（文の始め・;・&・|・(・改行のあと。VAR=値 の前置きは可）にあるものだけ。
	// rg や echo の引数、コミットのメッセージの中の文は数えない
	ghPR      = `(?:^|[;&|(\n])\s*(?:[A-Za-z_][A-Za-z0-9_]*=\S*\s+)*gh\s+pr\s+`
	prRe      = regexp.MustCompile(ghPR + `create\b`)
	mergeRe   = regexp.MustCompile(ghPR + `merge\b`)
	reviewRe  = regexp.MustCompile(ghPR + `review\b`)
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
	for _, k := range PRActions(name, input) {
		add(k, 1)
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

// PRActions は、ツールの呼び出しが PR に何をするものかを、エージェントによらず返す（成功したかは呼び出し側が確かめる）。
// 種類は pr（作成）・merge・review・approve（アプルーブ。review と一緒に返す）。PR の操作でなければ nil。
//   - シェルのコマンド（command は文か配列（Codex の shell・local_shell）、exec_command は cmd）:
//     gh pr create・gh pr merge（--auto・--disable-auto は自動マージの予約なので除く）・gh pr review（--approve・-a はアプルーブ）
//   - ツールの名前（GitHub の MCP など。Codex の MCP は「サーバー__ツール」の名前になる）:
//     create_pull_request・merge_pull_request、レビューを出すもの（pull_request_review_write の create（event つき）と
//     submit_pending、create_pull_request_review（event つき）、submit_pending_pull_request_review）。event が APPROVE ならアプルーブ
func PRActions(name string, input any) []string {
	in := Map(input)
	review := func(approve bool) []string {
		if approve {
			return []string{"review", "approve"}
		}
		return []string{"review"}
	}
	event := strings.ToUpper(Str(in["event"]))
	switch {
	case strings.HasSuffix(name, "create_pull_request"):
		return []string{"pr"}
	case strings.HasSuffix(name, "merge_pull_request"):
		return []string{"merge"}
	case strings.HasSuffix(name, "pull_request_review_write"):
		if m := Str(in["method"]); m == "submit_pending" || m == "create" && event != "" {
			return review(event == "APPROVE")
		}
		return nil
	case strings.HasSuffix(name, "submit_pending_pull_request_review"),
		strings.HasSuffix(name, "create_pull_request_review") && event != "":
		return review(event == "APPROVE")
	}
	if !shellTool(name) {
		return nil
	}
	cmd := CommandText(input)
	if helpRe.MatchString(cmd) {
		return nil
	}
	var out []string
	if prRe.MatchString(cmd) {
		out = append(out, "pr")
	}
	if mergeRe.MatchString(cmd) && !autoRe.MatchString(cmd) {
		out = append(out, "merge")
	}
	if reviewRe.MatchString(cmd) {
		out = append(out, review(approveRe.MatchString(cmd))...)
	}
	return out
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
