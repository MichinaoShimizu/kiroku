package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// autostartLabel は launchd に登録する名前。
const autostartLabel = "io.github.michinaoshimizu.kiroku"

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

// autostartGOOS は OS、autostartHome はホーム（テストで差しかえる）。
var (
	autostartGOOS = runtime.GOOS
	autostartHome = os.UserHomeDir
)

// autostartPaths は、kiroku 4 の「kiroku autostart on」が書いた設定ファイルの場所と、登録を外すコマンド（失敗しても続ける）。
// 外すのに要るのはこれだけなので、設定ファイルの中身は作らない（環境変数に制御文字があっても外せるように）。
// autostart のない OS（Windows など）では ok が false。
func autostartPaths(goos, home string) (path string, off [][]string, ok bool) {
	switch goos {
	case "darwin":
		path = filepath.Join(home, "Library", "LaunchAgents", autostartLabel+".plist")
		return path, [][]string{{"launchctl", "bootout", fmt.Sprintf("gui/%d", os.Getuid()), path}}, true
	case "linux":
		cfg := os.Getenv("XDG_CONFIG_HOME")
		if cfg == "" {
			cfg = filepath.Join(home, ".config")
		}
		return filepath.Join(cfg, "systemd", "user", "kiroku.service"), [][]string{{"systemctl", "--user", "disable", "--now", "kiroku.service"}}, true
	}
	return "", nil, false
}

// currentAutostartPaths は、この OS とホームでの autostartPaths。ホームがわからなければ ok は false。
func currentAutostartPaths() (path string, off [][]string, ok bool) {
	home, err := autostartHome()
	if err != nil || home == "" {
		return "", nil, false
	}
	return autostartPaths(autostartGOOS, home)
}

// hasAutostart は、kiroku 4 の autostart で登録したログイン時の起動が残っているか（kiroku serve --check が外し方を言う）。
func hasAutostart() bool {
	path, _, ok := currentAutostartPaths()
	if !ok {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

// removeAutostart は、kiroku 4 の「kiroku autostart on」で登録した、ログイン時の kiroku serve の起動を外す。
// 5.0 で autostart を消したので、登録したままの人が外せるよう、消したコマンドを打ったときにここを通る。登録がなければ何もしない
func removeAutostart() {
	path, off, ok := currentAutostartPaths()
	if !ok {
		return
	}
	if _, err := os.Stat(path); err != nil {
		return
	}
	for _, c := range off {
		_ = runCmd(c[0], c[1:]...) // 動いていなくても、設定ファイルは消す
	}
	if err := os.Remove(path); err != nil {
		fmt.Fprintf(os.Stderr, "could not remove the login item %s: %v\n", path, err)
		return
	}
	if autostartGOOS == "linux" {
		_ = runCmd("systemctl", "--user", "daemon-reload")
	}
	fmt.Println("removed the login item that kiroku 4 added: kiroku serve no longer starts when you log in")
}
