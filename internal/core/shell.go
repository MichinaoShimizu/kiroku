package core

import (
	"regexp"
	"strings"
)

// ResumeCmd は、会話を再開するためにコピーして端末に貼るコマンド（cd <dir> && <cmd> <id>）を作る。
// dir と id は履歴から来た値なので、シェルが解釈しないように囲む。cmd は kiroku が決めた固定の文字列。
// 安全に囲めない値（制御文字、Windows で両方のシェルが特別に扱う文字）があれば "" を返し、コマンドは出さない。
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
func shellArg(s string, win bool) (string, bool) {
	if s == "" || strings.ContainsFunc(s, func(r rune) bool { return r < 0x20 || (r >= 0x7f && r <= 0x9f) || r == '\u2028' || r == '\u2029' }) {
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
