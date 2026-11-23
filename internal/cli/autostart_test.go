package cli

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
	oldOS, oldExe, oldHome, oldRun, oldProbe := autostartGOOS, autostartExe, autostartHome, runCmd, probe
	t.Cleanup(func() {
		autostartGOOS, autostartExe, autostartHome, runCmd, probe = oldOS, oldExe, oldHome, oldRun, oldProbe
	})
	autostartHome = func() (string, error) { return home, nil }
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
	for _, want := range []string{`ExecStart="/home/me/my bin/kiroku" "serve" "--no-open"` + "\n", `Environment="CODEX_HOME=/home/me/100%%$x"`, "WantedBy=default.target"} {
		if !strings.Contains(p.content, want) {
			t.Errorf("ユニットに %q がない:\n%s", want, p.content)
		}
	}
	if p.path != filepath.Join("/home/me", ".config", "systemd", "user", "kiroku.service") {
		t.Errorf("ユニットの場所: %s", p.path)
	}

	// Environment= の値は $ をそのまま、\ と " はエスケープする。ExecStart= は $ を $$ にする
	p, err = planAutostart("linux", "/home/me", "/home/me/$bin/kiroku", nil, map[string]string{"CLAUDE_CONFIG_DIR": `/home/me/a "b" \c $d`})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`ExecStart="/home/me/$$bin/kiroku"`, `Environment="CLAUDE_CONFIG_DIR=/home/me/a \"b\" \\c $d"` + "\n"} {
		if !strings.Contains(p.content, want) {
			t.Errorf("ユニットに %q がない:\n%s", want, p.content)
		}
	}
	// 改行などの制御文字は、ユニットの別の行として読まれてしまうので断る
	for _, c := range []struct {
		exe string
		env map[string]string
	}{
		{"/home/me/kiroku", map[string]string{"CODEX_HOME": "/x\nExecStartPre=/bin/false"}},
		{"/home/me/kiroku", map[string]string{"PATH": "/usr/bin\r"}},
		{"/home/me/kiroku", map[string]string{"KIRO_HOME": "/x\ty"}},
		{"/home/me/kiro\nku", nil},
	} {
		if _, err := planAutostart("linux", "/home/me", c.exe, nil, c.env); err == nil || !strings.Contains(err.Error(), "control character") {
			t.Errorf("%q %v: err = %v", c.exe, c.env, err)
		}
	}

	if _, err := planAutostart("windows", `C:\Users\me`, `C:\bin\kiroku.exe`, nil, nil); err == nil || !strings.Contains(err.Error(), "shell:startup") {
		t.Errorf("Windows は Startup フォルダを案内する: %v", err)
	}
}

// 5.0 で autostart を消した。kiroku 4 で登録したログイン時の起動は、「kiroku autostart」を打つと外れ、コマンドはエラーで止まる。
// kiroku serve --check は、登録が残っていればその外し方を言う。登録がなければ、何も外さない
func TestAutostartRemoved(t *testing.T) {
	for _, goos := range []string{"linux", "darwin"} {
		_, ran := fakeAutostart(t, goos, ready)
		p, err := currentAutostart(nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(p.path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p.path, []byte(p.content), 0o644); err != nil {
			t.Fatal(err)
		}
		var b strings.Builder
		doctorReport(&b, nil, 1, t.TempDir(), release{}, false)
		if !strings.Contains(b.String(), `run "kiroku autostart" to remove it`) {
			t.Errorf("%s: serve --check does not say how to remove the old login item:\n%s", goos, b.String())
		}
		err = dispatch([]string{"autostart", "on"})
		if err == nil || !strings.Contains(err.Error(), "removed in kiroku 5.0") {
			t.Errorf("%s: kiroku autostart on = %v, want an error naming 5.0", goos, err)
		}
		if _, err := os.Stat(p.path); !os.IsNotExist(err) {
			t.Errorf("%s: the login item %s is still there", goos, p.path)
		}
		if len(*ran) == 0 {
			t.Errorf("%s: the login item was not unregistered", goos)
		}
		*ran = nil
		_ = dispatch([]string{"autostart"})
		if len(*ran) != 0 {
			t.Errorf("%s: ran %v with no login item", goos, *ran)
		}
	}
}
