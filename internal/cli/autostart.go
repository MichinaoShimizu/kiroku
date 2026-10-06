package cli

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// autostartLabel は launchd に登録する名前。
const autostartLabel = "io.github.michinaoshimizu.kiroku"

// autostartEnv は、ログイン時に起動したときにも同じ履歴を読むよう、登録するときの値を書き残す環境変数。
// launchd や systemd から起動すると、シェルで設定した値（と PATH）は引き継がれないため（PATH は git を探すのに使う）。
var autostartEnv = []string{"PATH", "CLAUDE_CONFIG_DIR", "KIRO_HOME", "KIROCREW_HOME", "CODEX_HOME", "KIROKU_ARCHIVE_DIR"}

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

// autostartRetry は、登録のコマンドが失敗したときに待つ時間（テストで 0 にする）。
// launchctl は、前の登録を外した直後だと bootstrap に失敗することがあるため、少し待って何回か試す。
var autostartRetry = 500 * time.Millisecond

func runRetry(c []string) error {
	var err error
	for i := 0; i < 4; i++ {
		if err = runCmd(c[0], c[1:]...); err == nil {
			return nil
		}
		time.Sleep(autostartRetry)
	}
	return err
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
			quoted[i] = systemdQuote(a)
		}
		b.WriteString("ExecStart=" + strings.Join(quoted, " ") + "\n")
		for _, k := range autostartEnv {
			if v, ok := env[k]; ok {
				b.WriteString("Environment=" + systemdQuote(k+"="+v) + "\n")
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

// systemdQuote は systemd の設定の 1 語にする（空白などを含んでも 1 語のまま、% や $ は文字のまま）。
func systemdQuote(s string) string {
	s = strings.NewReplacer(`\`, `\\`, `"`, `\"`, "%", "%%", "$", "$$").Replace(s)
	return `"` + s + `"`
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

// autostartState は、自動起動の状態（kiroku doctor と kiroku autostart で出す）。
type autostartState struct {
	on, running bool
	mark, text  string
	url, addr   string
}

// autostartPort は、設定ファイルに書いた待ち受け先（なければ既定）。
var autostartPort = regexp.MustCompile(`[\w.\-\[\]]*:\d{2,5}\b`)

// probe の答え。kiroku serve は先に待ち受けてから履歴を読むので、読んでいる間は /stamp が 503 を返す。
const (
	noAnswer = iota // 何も答えない（まだ起動していない・起動に失敗した）
	loading         // 答えるが、まだ履歴を読んでいる
	ready           // 画面を開ける
)

// probe は、待ち受け先で kiroku serve が答えるか（手元への問い合わせだけ）。テストで差しかえる。
var probe = func(addr string) int {
	c := http.Client{Timeout: 700 * time.Millisecond}
	r, err := c.Get("http://" + addr + "/stamp")
	if err != nil {
		return noAnswer
	}
	r.Body.Close()
	switch r.StatusCode {
	case http.StatusOK:
		return ready
	case http.StatusServiceUnavailable:
		return loading
	}
	return noAnswer
}

// autostartWait は、登録したあと kiroku serve が答えるまで待つ時間（テストで 0 にする）。
var autostartWait = 10 * time.Second

func autostartStatus() autostartState {
	p, err := currentAutostart(nil)
	if err != nil {
		return autostartState{mark: "·", text: "not available on this system"}
	}
	b, err := os.ReadFile(p.path)
	if err != nil {
		return autostartState{mark: "·", text: "off (run \"kiroku autostart on\" to start kiroku serve when you log in)"}
	}
	addr := defaultAddr
	if m := autostartPort.FindString(string(b)); m != "" {
		addr = listenAddr(m)
	}
	_, port, _ := net.SplitHostPort(addr)
	url := "http://localhost:" + port + "/"
	s := autostartState{on: true, url: url}
	host, _, _ := net.SplitHostPort(addr)
	if host == "" || host == "0.0.0.0" || host == "::" || host == "[::]" {
		addr = "127.0.0.1:" + port
	}
	s.addr = addr
	s.describe(probe(addr), p.log)
	return s
}

// describe は、probe の答えから状態の一言を決める。
func (s *autostartState) describe(answer int, log string) {
	switch answer {
	case ready:
		s.running, s.mark, s.text = true, "✓", "on, running at "+s.url
	case loading:
		s.running, s.mark, s.text = true, "✓", fmt.Sprintf("on, reading your history at %s (the page opens the view when it is ready)", s.url)
	default:
		s.running, s.mark, s.text = false, "!", fmt.Sprintf("on, but nothing answers at %s (see the output: %s)", s.url, log)
	}
}

func cmdAutostart(args []string) error {
	fs := newFS("autostart", "autostart [on [ADDR] | off]\n\nStarts \"kiroku serve\" in the background each time you log in, so the view is always at\nhttp://localhost:8484/ (macOS: launchd, Linux: systemd --user).\n\n  kiroku autostart          show the status\n  kiroku autostart on       start now and at every login (ADDR such as :8485 changes the port)\n  kiroku autostart off      stop it and remove it from login")
	pos, err := parse(fs, args, 2)
	if err != nil {
		return quiet(err)
	}
	cmd := ""
	if len(pos) > 0 {
		cmd = pos[0]
	}
	var extra []string
	if len(pos) == 2 {
		if cmd != "on" || !addrRe.MatchString(pos[1]) {
			return fmt.Errorf("unknown argument: %s (use \"kiroku autostart on :8485\" to change the port)", pos[1])
		}
		extra = []string{pos[1]}
	}
	switch cmd {
	case "":
		s := autostartStatus()
		fmt.Printf("autostart: %s\n", s.text)
		return nil
	case "on":
		p, err := currentAutostart(extra)
		if err != nil {
			return err
		}
		if strings.Contains(p.exe, "go-build") {
			return fmt.Errorf("kiroku is running from a temporary build (go run); install it first (go install or install.sh), then run \"kiroku autostart on\" again")
		}
		if err := os.MkdirAll(filepath.Dir(p.path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(p.path, []byte(p.content), 0o644); err != nil {
			return err
		}
		for _, c := range p.reloadOn {
			_ = runCmd(c[0], c[1:]...) // まだ登録していなければ失敗するが、それでよい
		}
		for _, c := range p.on {
			if err := runRetry(c); err != nil {
				for _, c := range p.off {
					_ = runCmd(c[0], c[1:]...)
				}
				os.Remove(p.path) // 登録できなければ、途中まで登録したものも、書いた設定も残さない
				hint := ""
				if autostartGOOS == "linux" {
					hint = "\nsystemd user services may not be available here (WSL or a container, for example); run \"kiroku serve\" yourself instead"
				}
				return fmt.Errorf("could not register kiroku serve to start at login: %w%s", err, hint)
			}
		}
		fmt.Printf("autostart is on: kiroku serve starts now and each time you log in\nsettings: %s\noutput: %s\n", p.path, p.log)
		// serve は起動するとすぐ待ち受けるので、答えるまで少しだけ待って結果を出す（前は答えないまま「on」とだけ出て、壊れて見えた）
		s := autostartStatus()
		answer := noAnswer
		for end := time.Now().Add(autostartWait); s.on; time.Sleep(250 * time.Millisecond) {
			if answer = probe(s.addr); answer != noAnswer || !time.Now().Before(end) {
				break
			}
		}
		s.describe(answer, p.log)
		fmt.Printf("%s %s\n", s.mark, s.text)
		if !s.running {
			return fmt.Errorf("kiroku serve did not answer within %s; check the output above, or run \"kiroku doctor\"", autostartWait)
		}
		return nil
	case "off":
		p, err := currentAutostart(nil)
		if err != nil {
			return err
		}
		if _, err := os.Stat(p.path); os.IsNotExist(err) {
			fmt.Println("autostart is already off")
			return nil
		}
		for _, c := range p.off {
			_ = runCmd(c[0], c[1:]...) // 動いていなくても、設定ファイルは消す
		}
		if err := os.Remove(p.path); err != nil {
			return err
		}
		if autostartGOOS == "linux" {
			_ = runCmd("systemctl", "--user", "daemon-reload")
		}
		fmt.Println("autostart is off: kiroku serve no longer starts when you log in")
		return nil
	}
	return fmt.Errorf("unknown argument: %s (use \"kiroku autostart\", \"kiroku autostart on\" or \"kiroku autostart off\")", cmd)
}
