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
// リポジトリは他人が作ったものかもしれない。貼ったコマンドがそのリポジトリの設定（.git/config・.gitattributes）にある
// プログラムを動かさないよう、gitPasteFlags を付ける（-c はリポジトリの設定より強い）。show には GitShow を渡す
// （外部の diff・textconv と、それが呼ぶ filter.*.process を動かさない）。
func GitCmd(repo, cmd string, args ...string) string {
	win := isWinPath(repo)
	r, ok := shellArg(repo, win)
	if !ok {
		return ""
	}
	out := "git " + gitPasteFlags + " -C " + r + " " + cmd
	for _, a := range args {
		q, ok := shellArg(a, win)
		if !ok {
			return ""
		}
		out += " " + q
	}
	return out
}

// gitPasteFlags は、貼る git のコマンドに付ける、リポジトリの設定のプログラムを止めるもの。kiroku 自身が git を動かすとき
// （gitlog の gitCmd）と同じ考え。環境変数（GIT_ALLOW_PROTOCOL など）は、PowerShell・cmd に貼ると効かないので使わない。
//   - --no-pager、core.fsmonitor=false: ページャー（core.pager・pager.<コマンド>）と fsmonitor
//   - log.showSignature=false: 署名つきのコミットで gpg.program（ssh・x509 のプログラムも）
//   - diff.submodule=short: サブモジュールの差分を別の git で出させない（そちらはサブモジュールの設定の外部の diff を動かす）
//   - protocol.*.allow=never: 部分クローンで手元にない中身を取りに行かせない（そのとき remote.<名前>.uploadpack や
//     core.sshCommand を動かす）。リポジトリの protocol.<名前>.allow は protocol.allow より強いので、名前ごとにも止める
const gitPasteFlags = "--no-pager -c core.fsmonitor=false -c log.showSignature=false -c diff.submodule=short" +
	" -c protocol.allow=never -c protocol.file.allow=never -c protocol.ssh.allow=never -c protocol.ext.allow=never" +
	" -c protocol.git.allow=never -c protocol.http.allow=never -c protocol.https.allow=never"

// GitShow は、GitCmd に渡す show。リポジトリの設定の外部の diff と textconv（と、それが呼ぶ filter.*.process）を動かさない。
const GitShow = "show --no-ext-diff --no-textconv"

var (
	winPath    = regexp.MustCompile(`^(?:[A-Za-z]:[\\/]|\\\\)`)
	posixPlain = regexp.MustCompile(`^[A-Za-z0-9_@%+=:,./-]+$`)
	winPlain   = regexp.MustCompile(`^[A-Za-z0-9_.:\\/-]+$`)
)

// isWinPath は、ドライブ名（C:\）や UNC（\\server）で始まる Windows のパスか。
func isWinPath(p string) bool { return winPath.MatchString(p) }

// shellArg は、s を 1 つの引数として貼れる形にする。
// POSIX のシェルでは単一引用符で囲む（単一引用符と \ を含むものは扱わない）。
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
	if posixPlain.MatchString(s) && s[0] != '=' { // zsh は =ls を /bin/ls に置きかえるので、= で始まるものは囲む
		return s, true
	}
	// 単一引用符と \ を含むものは扱わない。sh・bash・zsh なら '\'' で閉じて開き直せるが、fish は単一引用符の中でも \' と \\ を
	// エスケープとして読み、PowerShell は \ をただの文字、'' を空の文字列として読むので、どちらでも囲みを抜けてコマンドを足せる。
	// 含まないなら '…' は sh・bash・zsh・fish・PowerShell のどれでも同じ 1 つの文字列になる
	if strings.ContainsAny(s, `'\`) {
		return "", false
	}
	return "'" + s + "'", true
}

// unsafeRune は、貼るコマンドに入れない文字か：制御文字（C0、DEL、C1）、行と段落の区切り（U+2028、U+2029）、
// Unicode の書式文字（Cf：双方向の制御、幅ゼロの文字、U+FEFF など）、曲がった引用符（U+2018〜U+201F。PowerShell が引用符として読む）。
func unsafeRune(r rune) bool {
	return r < 0x20 || (r >= 0x7f && r <= 0x9f) || (r >= '\u2018' && r <= '\u201f') || r == '\u2028' || r == '\u2029' ||
		unicode.Is(unicode.Cf, r)
}
