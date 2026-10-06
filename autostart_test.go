package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeAutostart は、ホーム・OS・launchctl / systemctl・待ち受けの確認を差しかえ、実行したコマンドを返す。
func fakeAutostart(t *testing.T, goos string, answer int) (home string, ran *[]string) {
	t.Helper()
	home = t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", "")
	oldOS, oldExe, oldHome, oldRun, oldProbe, oldRetry, oldWait := autostartGOOS, autostartExe, autostartHome, runCmd, probe, autostartRetry, autostartWait
	t.Cleanup(func() {
		autostartGOOS, autostartExe, autostartHome, runCmd, probe, autostartRetry, autostartWait = oldOS, oldExe, oldHome, oldRun, oldProbe, oldRetry, oldWait
	})
	autostartWait = 0
	autostartHome = func() (string, error) { return home, nil }
	autostartRetry = 0
	autostartGOOS = goos
	autostartExe = func() (string, error) { return filepath.Join(home, "bin", "kiroku"), nil }
	var cmds []string
	runCmd = func(name string, args ...string) error {
		cmds = append(cmds, name+" "+strings.Join(args, " "))
		return nil
	}
	probe = func(string) int { return answer }
	return home, &cmds
}

// macOS は launchd の plist、Linux は systemd --user のユニット。引数と環境変数は 1 語のまま、記号もそのまま入る。
func TestPlanAutostart(t *testing.T) {
	env := map[string]string{"PATH": "/opt/homebrew/bin:/usr/bin", "CLAUDE_CONFIG_DIR": "/Users/me/my claude"}
	p, err := planAutostart("darwin", "/Users/me", "/Users/me/bin/kiroku & co", []string{":8485"}, env)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"<string>/Users/me/bin/kiroku &amp; co</string>", "<string>serve</string>", "<string>--no-open</string>", "<string>:8485</string>",
		"<key>CLAUDE_CONFIG_DIR</key><string>/Users/me/my claude</string>", "<key>RunAtLoad</key><true/>", xmlText(filepath.Join("/Users/me", "Library", "Logs", "kiroku.log"))} {
		if !strings.Contains(p.content, want) {
			t.Errorf("plist に %q がない:\n%s", want, p.content)
		}
	}
	if p.path != filepath.Join("/Users/me", "Library", "LaunchAgents", autostartLabel+".plist") {
		t.Errorf("plist の場所: %s", p.path)
	}

	t.Setenv("XDG_CONFIG_HOME", "")
	p, err = planAutostart("linux", "/home/me", "/home/me/my bin/kiroku", nil, map[string]string{"PATH": "/usr/bin", "CODEX_HOME": "/home/me/100%$x"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`ExecStart="/home/me/my bin/kiroku" "serve" "--no-open"` + "\n", `Environment="CODEX_HOME=/home/me/100%%$$x"`, "WantedBy=default.target"} {
		if !strings.Contains(p.content, want) {
			t.Errorf("ユニットに %q がない:\n%s", want, p.content)
		}
	}
	if p.path != filepath.Join("/home/me", ".config", "systemd", "user", "kiroku.service") {
		t.Errorf("ユニットの場所: %s", p.path)
	}

	if _, err := planAutostart("windows", `C:\Users\me`, `C:\bin\kiroku.exe`, nil, nil); err == nil || !strings.Contains(err.Error(), "shell:startup") {
		t.Errorf("Windows は Startup フォルダを案内する: %v", err)
	}
}

// kiroku autostart on で設定を書いて登録し、off で外して消す。登録に失敗したら設定を残さない。
func TestAutostartCommand(t *testing.T) {
	home, ran := fakeAutostart(t, "linux", loading)
	unit := filepath.Join(home, ".config", "systemd", "user", "kiroku.service")
	if err := dispatch([]string{"autostart", "on", ":8485"}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(unit)
	if err != nil || !strings.Contains(string(b), `"serve" "--no-open" ":8485"`) {
		t.Fatalf("ユニットが書かれていない: %v\n%s", err, b)
	}
	if got := strings.Join(*ran, "\n"); !strings.Contains(got, "systemctl --user enable kiroku.service") || !strings.Contains(got, "systemctl --user restart kiroku.service") {
		t.Errorf("登録していない:\n%s", got)
	}
	if s := autostartStatus(); !s.on || !s.running || s.url != "http://localhost:8485/" || !strings.Contains(s.text, "reading your history") {
		t.Errorf("状態: %+v", s)
	}
	*ran = nil
	if err := dispatch([]string{"autostart", "off"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(unit); !os.IsNotExist(err) {
		t.Error("off でユニットが消えていない")
	}
	if got := strings.Join(*ran, "\n"); !strings.Contains(got, "systemctl --user disable --now kiroku.service") {
		t.Errorf("登録を外していない:\n%s", got)
	}
	if s := autostartStatus(); s.on {
		t.Errorf("off のあとも on: %+v", s)
	}

	*ran = nil
	runCmd = func(name string, args ...string) error {
		*ran = append(*ran, name+" "+strings.Join(args, " "))
		if strings.Contains(strings.Join(args, " "), "restart") {
			return os.ErrPermission
		}
		return nil
	}
	if err := dispatch([]string{"autostart", "on"}); err == nil {
		t.Error("登録に失敗したのにエラーにならない")
	}
	if got := strings.Join(*ran, "\n"); strings.Count(got, "restart kiroku.service") != 4 || !strings.HasSuffix(got, "systemctl --user disable --now kiroku.service") {
		t.Errorf("何回か試してから、途中までの登録を外していない:\n%s", got)
	}
	if _, err := os.Stat(unit); !os.IsNotExist(err) {
		t.Error("登録に失敗したのにユニットが残っている")
	}
	if err := dispatch([]string{"autostart", "of"}); err == nil {
		t.Error("打ちまちがいがエラーにならない")
	}
}

// 登録しても serve が答えなければ、登録は残したまま（ログで原因を見られるよう）エラーで知らせる。
func TestAutostartOnNoAnswer(t *testing.T) {
	home, _ := fakeAutostart(t, "linux", noAnswer)
	err := dispatch([]string{"autostart", "on"})
	if err == nil || !strings.Contains(err.Error(), "did not answer") {
		t.Errorf("エラー: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".config", "systemd", "user", "kiroku.service")); err != nil {
		t.Error("答えないときも、登録は残す")
	}
	if s := autostartStatus(); !s.on || s.running || !strings.Contains(s.text, "nothing answers") {
		t.Errorf("状態: %+v", s)
	}
}
