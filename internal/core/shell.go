package core

import (
	"regexp"
	"strings"
	"unicode"
)

// ResumeCmd は、会話を再開するためにコピーして端末に貼るコマンド（cd <dir> && <cmd> <id>）を作る。
// dir と id は履歴から来た値なので、シェルが解釈しないように囲む。cmd は kiroku が決めた固定の文字列。
// 安全に囲めない値（制御文字、Unicode の書式文字、- で始まる値、Windows で両方のシェルが特別に扱う文字）が
// あれば "" を返し、コマンドは出さない。
func ResumeCmd(dir, cmd, id string) string {
	win := isWinPath(dir)
	d, ok := shellArg(dir, win)
	if !ok {
		return ""
	}
	out := "cd " + d + " && " + cmd
	if id != "" {
		a, ok := shellArg(id, win)
		if !ok {
			return ""
		}
		out += " " + a
	}
	return out
}

// GitCmd は、コミットや push を手元で見るためにコピーして端末に貼るコマンド（git -C <repo> <cmd> <args>）を作る。
// cmd は kiroku が決めた固定の文字列（show、log --oneline）。repo と args（ハッシュや範囲）は git から来た値なので、
// ResumeCmd と同じく 1 つずつ囲み、安全に囲めない値があれば "" を返してコマンドは出さない。
func GitCmd(repo, cmd string, args ...string) string {
	win := isWinPath(repo)
	r, ok := shellArg(repo, win)
	if !ok {
		return ""
	}
	out := "git -C " + r + " " + cmd
	for _, a := range args {
		q, ok := shellArg(a, win)
		if !ok {
			return ""
		}
		out += " " + q
	}
	return out
}

var (
	winPath    = regexp.MustCompile(`^(?:[A-Za-z]:[\\/]|\\\\)`)
	posixPlain = regexp.MustCompile(`^[A-Za-z0-9_@%+=:,./-]+$`)
	winPlain   = regexp.MustCompile(`^[A-Za-z0-9_.:\\/-]+$`)
)

// isWinPath は、ドライブ名（C:\）や UNC（\\server）で始まる Windows のパスか。
func isWinPath(p string) bool { return winPath.MatchString(p) }

// shellArg は、s を 1 つの引数として貼れる形にする。
// POSIX のシェルでは単一引用符で囲む（中の単一引用符は、引用を閉じて \' を置き、また開く）。
// Windows では cmd と PowerShell のどちらに貼られても同じ意味になるよう二重引用符で囲み、
// 二重引用符の中でも展開される文字（cmd の % !、PowerShell の $ `）と " を含むものは扱わない。
// - で始まる値は、囲んでもコマンド（cd、claude、codex など）がオプションとして読むので扱わない
// （たとえば ID が --dangerously-skip-permissions、フォルダが - の履歴）。
// Unicode の書式文字（U+202E などの双方向の制御、U+200B などの幅ゼロの文字、U+FEFF）を含むものも、
// 画面に見えるコマンドと貼られる中身が違って見えるので扱わない。
// 曲がった引用符（U+2018〜U+201F）を含むものも扱わない。PowerShell は ‘ ’ ‚ ‛ を ' と、“ ” „ を " と同じに読むので、
// 囲みを閉じてコマンドを足せる（Windows の二重引用符でも、macOS・Linux の PowerShell に貼った単一引用符でも）。
func shellArg(s string, win bool) (string, bool) {
	if s == "" || s[0] == '-' || strings.ContainsFunc(s, unsafeRune) {
		return "", false
	}
	if win {
		if winPlain.MatchString(s) {
			return s, true
		}
		if strings.ContainsAny(s, "\"%!$`") {
			return "", false
		}
		return `"` + s + `"`, true
	}
	if posixPlain.MatchString(s) {
		return s, true
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'", true
}

// unsafeRune は、貼るコマンドに入れない文字か：制御文字（C0、DEL、C1）、行と段落の区切り（U+2028、U+2029）、
// Unicode の書式文字（Cf：双方向の制御、幅ゼロの文字、U+FEFF など）、曲がった引用符（U+2018〜U+201F。PowerShell が引用符として読む）。
func unsafeRune(r rune) bool {
	return r < 0x20 || (r >= 0x7f && r <= 0x9f) || (r >= '\u2018' && r <= '\u201f') || r == '\u2028' || r == '\u2029' ||
		unicode.Is(unicode.Cf, r)
}
