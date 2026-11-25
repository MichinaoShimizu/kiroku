package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeAutostart は、ホーム・OS・launchctl / systemctl を差しかえ、実行したコマンドを返す。
func fakeAutostart(t *testing.T, goos string) (home string, ran *[]string) {
	t.Helper()
	home = t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", "")
	oldOS, oldHome, oldRun := autostartGOOS, autostartHome, runCmd
	t.Cleanup(func() {
		autostartGOOS, autostartHome, runCmd = oldOS, oldHome, oldRun
	})
	autostartHome = func() (string, error) { return home, nil }
	autostartGOOS = goos
	var cmds []string
	runCmd = func(name string, args ...string) error {
		cmds = append(cmds, name+" "+strings.Join(args, " "))
		return nil
	}
	return home, &cmds
}

// kiroku 4 が書いた設定ファイルの場所と、外すコマンド（macOS は launchd の plist、Linux は systemd --user のユニット）
func TestAutostartPaths(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	for _, c := range []struct {
		goos, home, path, off string
		ok                    bool
	}{
		{"darwin", "/Users/me", filepath.Join("/Users/me", "Library", "LaunchAgents", autostartLabel+".plist"), "launchctl bootout", true},
		{"linux", "/home/me", filepath.Join("/home/me", ".config", "systemd", "user", "kiroku.service"), "systemctl --user disable --now kiroku.service", true},
		{"windows", `C:\Users\me`, "", "", false},
		{"freebsd", "/home/me", "", "", false},
	} {
		path, off, ok := autostartPaths(c.goos, c.home)
		if path != c.path || ok != c.ok {
			t.Errorf("%s: %q %v, want %q %v", c.goos, path, ok, c.path, c.ok)
		}
		if c.ok && (len(off) != 1 || !strings.HasPrefix(strings.Join(off[0], " "), c.off)) {
			t.Errorf("%s: off = %v, want %q…", c.goos, off, c.off)
		}
	}
	t.Setenv("XDG_CONFIG_HOME", "/home/me/cfg")
	if path, _, _ := autostartPaths("linux", "/home/me"); path != filepath.Join("/home/me/cfg", "systemd", "user", "kiroku.service") {
		t.Errorf("XDG_CONFIG_HOME: %s", path)
	}
}

// 5.0 で autostart を消した。kiroku 4 で登録したログイン時の起動は、「kiroku autostart」を打つと外れ、コマンドはエラーで止まる。
// kiroku serve --check は、登録が残っていればその外し方を言う。登録がなければ、何も外さない
func TestAutostartRemoved(t *testing.T) {
	for _, goos := range []string{"linux", "darwin"} {
		_, ran := fakeAutostart(t, goos)
		path, _, ok := currentAutostartPaths()
		if !ok {
			t.Fatal(goos)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("dummy login item\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		var b strings.Builder
		doctorReport(&b, nil, 1, t.TempDir(), release{}, false)
		if !strings.Contains(b.String(), `run "kiroku autostart" to remove it`) {
			t.Errorf("%s: serve --check does not say how to remove the old login item:\n%s", goos, b.String())
		}
		err := dispatch([]string{"autostart", "on"})
		if err == nil || !strings.Contains(err.Error(), "removed in kiroku 5.0") {
			t.Errorf("%s: kiroku autostart on = %v, want an error naming 5.0", goos, err)
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("%s: the login item %s is still there", goos, path)
		}
		if len(*ran) == 0 {
			t.Errorf("%s: the login item was not unregistered", goos)
		}
		*ran = nil
		_ = dispatch([]string{"autostart"})
		if len(*ran) != 0 {
			t.Errorf("%s: ran %v with no login item", goos, *ran)
		}
		b.Reset()
		doctorReport(&b, nil, 1, t.TempDir(), release{}, false)
		if strings.Contains(b.String(), "kiroku autostart") {
			t.Errorf("%s: serve --check still names the login item after it was removed", goos)
		}
	}
}

// 環境変数に制御文字（PATH のタブなど）があっても、登録は外れる。kiroku 4 のユニットの中身は作り直さない
func TestAutostartRemovedWithControlCharInEnv(t *testing.T) {
	_, ran := fakeAutostart(t, "linux")
	path, _, _ := currentAutostartPaths()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("dummy login item\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", os.Getenv("PATH")+":/opt/a\tb")
	t.Setenv("CODEX_HOME", "/home/me/x\ny")
	removeAutostart()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("the login item %s is still there", path)
	}
	if len(*ran) == 0 {
		t.Error("the login item was not unregistered")
	}
}
