package cli

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"

	"github.com/MichinaoShimizu/kiroku/internal/core"
)

// autostartLabel は launchd に登録する名前。
const autostartLabel = "io.github.michinaoshimizu.kiroku"

// autostartEnv は、ログイン時に起動したときにも同じ履歴を読むよう、登録するときの値を書き残す環境変数。
// launchd や systemd から起動すると、シェルで設定した値（と PATH）は引き継がれないため（PATH は git を探すのに使う）。
// エージェント自身の環境変数は core.Agents から取る。
var autostartEnv = slices.Concat([]string{"PATH"}, core.AgentEnv(), []string{"KIROKU_ARCHIVE_DIR", "KIROKU_CONFIG_DIR", "KIROKU_LANG", "XDG_CONFIG_HOME", "XDG_DATA_HOME"})

// runCmd は launchctl・systemctl を実行する（テストで差しかえる）。
var runCmd = func(name string, args ...string) error {
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		msg := fmt.Sprintf("%s %s: %v", name, strings.Join(args, " "), err)
		if o := strings.TrimSpace(string(out)); o != "" {
			msg += ": " + o
		}
		return errors.New(msg)
	}
	return nil
}

// autostartGOOS は OS、autostartExe は kiroku の実行ファイル、autostartHome はホーム（テストで差しかえる）。
var (
	autostartGOOS = runtime.GOOS
	autostartExe  = os.Executable
	autostartHome = os.UserHomeDir
)

// autostartPlan は、ログイン時に kiroku serve を起動するための設定ファイルと、登録・解除のコマンド。
type autostartPlan struct {
	exe      string     // 起動する kiroku
	path     string     // 設定ファイル
	content  string     // 設定ファイルの中身
	on, off  [][]string // 登録する・解除するコマンド（off は失敗しても続ける）
	reloadOn [][]string // 登録の前に、前の登録を外すコマンド（失敗してよい）
	log      string     // 出力の行き先（案内用）
}

// planAutostart は OS ごとの設定を作る。exe は kiroku の実行ファイル、args は serve 以降の引数。
func planAutostart(goos, home string, exe string, args []string, env map[string]string) (autostartPlan, error) {
	cmd := append([]string{exe, "serve", "--no-open"}, args...)
	switch goos {
	case "darwin":
		path := filepath.Join(home, "Library", "LaunchAgents", autostartLabel+".plist")
		log := filepath.Join(home, "Library", "Logs", "kiroku.log")
		var b strings.Builder
		b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key><string>` + autostartLabel + `</string>
	<key>ProgramArguments</key>
	<array>
`)
		for _, a := range cmd {
			b.WriteString("\t\t<string>" + xmlText(a) + "</string>\n")
		}
		b.WriteString("\t</array>\n")
		if len(env) > 0 {
			b.WriteString("\t<key>EnvironmentVariables</key>\n\t<dict>\n")
			for _, k := range autostartEnv {
				if v, ok := env[k]; ok {
					b.WriteString("\t\t<key>" + k + "</key><string>" + xmlText(v) + "</string>\n")
				}
			}
			b.WriteString("\t</dict>\n")
		}
		b.WriteString("\t<key>RunAtLoad</key><true/>\n")
		b.WriteString("\t<key>StandardOutPath</key><string>" + xmlText(log) + "</string>\n")
		b.WriteString("\t<key>StandardErrorPath</key><string>" + xmlText(log) + "</string>\n")
		b.WriteString("</dict>\n</plist>\n")
		domain := fmt.Sprintf("gui/%d", os.Getuid())
		return autostartPlan{exe: exe, path: path, content: b.String(), log: log,
			reloadOn: [][]string{{"launchctl", "bootout", domain, path}},
			on:       [][]string{{"launchctl", "bootstrap", domain, path}},
			off:      [][]string{{"launchctl", "bootout", domain, path}}}, nil
	case "linux":
		cfg := os.Getenv("XDG_CONFIG_HOME")
		if cfg == "" {
			cfg = filepath.Join(home, ".config")
		}
		path := filepath.Join(cfg, "systemd", "user", "kiroku.service")
		var b strings.Builder
		b.WriteString("[Unit]\nDescription=kiroku serve (your AI work history in the browser)\n\n[Service]\n")
		quoted := make([]string, len(cmd))
		for i, a := range cmd {
			if err := systemdValue(a); err != nil {
				return autostartPlan{}, fmt.Errorf("cannot write %q to the systemd unit: %w", a, err)
			}
			quoted[i] = systemdQuote(a)
		}
		b.WriteString("ExecStart=" + strings.Join(quoted, " ") + "\n")
		for _, k := range autostartEnv {
			if v, ok := env[k]; ok {
				if err := systemdValue(v); err != nil {
					return autostartPlan{}, fmt.Errorf("cannot write $%s to the systemd unit: %w", k, err)
				}
				b.WriteString("Environment=" + systemdEnvQuote(k+"="+v) + "\n")
			}
		}
		b.WriteString("\n[Install]\nWantedBy=default.target\n")
		return autostartPlan{exe: exe, path: path, content: b.String(), log: "journalctl --user -u kiroku",
			on:  [][]string{{"systemctl", "--user", "daemon-reload"}, {"systemctl", "--user", "enable", "kiroku.service"}, {"systemctl", "--user", "restart", "kiroku.service"}},
			off: [][]string{{"systemctl", "--user", "disable", "--now", "kiroku.service"}}}, nil
	case "windows":
		return autostartPlan{}, fmt.Errorf("autostart is not supported on Windows yet; to start kiroku when you log in, put a shortcut to \"%s serve --no-open\" in your Startup folder (press Win+R and open shell:startup)", exe)
	}
	return autostartPlan{}, fmt.Errorf("autostart is not supported on %s", goos)
}

// xmlText は plist の文字列に入れられるようにする。
func xmlText(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

// systemdQuote は ExecStart= の 1 語にする（空白などを含んでも 1 語のまま、% や $ は文字のまま）。
// ExecStart= は $NAME を環境変数に置きかえるので、$ は $$ にする。
func systemdQuote(s string) string {
	s = strings.NewReplacer(`\`, `\\`, `"`, `\"`, "%", "%%", "$", "$$").Replace(s)
	return `"` + s + `"`
}

// systemdEnvQuote は Environment= の値にする。Environment= は $ を置きかえないので、$ はそのまま
// （$$ にすると、そのまま $$ が入ってしまう）。\ と " と、指定子の % だけを書きかえる。
func systemdEnvQuote(s string) string {
	s = strings.NewReplacer(`\`, `\\`, `"`, `\"`, "%", "%%").Replace(s)
	return `"` + s + `"`
}

// systemdValue は、ユニットファイルに書けない値（改行などの制御文字）を断る。改行があると、
// 値の続きが別の設定の行として読まれてしまう。
func systemdValue(s string) error {
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("it contains a control character (%U)", r)
		}
	}
	return nil
}

// currentAutostart は、この OS での設定（kiroku の場所と環境変数は、いま動いているものを使う）。
func currentAutostart(args []string) (autostartPlan, error) {
	home, err := autostartHome()
	if err != nil {
		return autostartPlan{}, err
	}
	exe, err := autostartExe()
	if err != nil {
		return autostartPlan{}, err
	}
	if p, err := filepath.EvalSymlinks(exe); err == nil {
		exe = p
	}
	env := map[string]string{}
	for _, k := range autostartEnv {
		if v, ok := os.LookupEnv(k); ok && v != "" {
			env[k] = v
		}
	}
	return planAutostart(autostartGOOS, home, exe, args, env)
}

// autostartPort は、設定ファイルに書いた待ち受け先（なければ既定）。
var autostartPort = regexp.MustCompile(`[\w.\-\[\]]*:\d{2,5}\b`)

// autostartAddr は、autostart で動かす kiroku serve の待ち受け先（autostart がオフなら false）。
func autostartAddr(p autostartPlan) (string, bool) {
	b, err := os.ReadFile(p.path)
	if err != nil {
		return "", false
	}
	if m := autostartPort.FindString(string(b)); m != "" {
		return listenAddr(m), true
	}
	return listenAddr(defaultAddr), true
}

// removeAutostart は、kiroku 4 の「kiroku autostart on」で登録した、ログイン時の kiroku serve の起動を外す。
// 5.0 で autostart を消したので、登録したままの人が外せるよう、消したコマンドを打ったときにここを通る。登録がなければ何もしない
func removeAutostart() {
	p, err := currentAutostart(nil)
	if err != nil {
		return
	}
	if _, err := os.Stat(p.path); err != nil {
		return
	}
	for _, c := range p.off {
		_ = runCmd(c[0], c[1:]...) // 動いていなくても、設定ファイルは消す
	}
	if err := os.Remove(p.path); err != nil {
		fmt.Fprintf(os.Stderr, "could not remove the login item %s: %v\n", p.path, err)
		return
	}
	if autostartGOOS == "linux" {
		_ = runCmd("systemctl", "--user", "daemon-reload")
	}
	fmt.Println("removed the login item that kiroku 4 added: kiroku serve no longer starts when you log in")
}
