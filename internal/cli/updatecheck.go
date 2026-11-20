package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// 新しい版のお知らせ。端末でコマンドを使ったとき、1 日に 1 回まで GitHub の releases/latest を見て（kiroku update と同じ、
// 版の名前だけを読む。履歴は何も送らない）、新しい版があれば知らせる。
//   - コマンドが終わったあとに「Update now? [y/N]」と聞き、y なら kiroku update と同じ手順（checksums.txt で確かめてから置きかえる）
//   - kiroku serve は聞かずに 1 行だけ知らせる（動き続けるので、答えを待って止めない）
//   - 端末でないとき（入力・出力・エラー出力のどれかがパイプ、CI、autostart）、dev ビルド、KIROKU_NO_UPDATE_CHECK=1 のときは、見にも行かない
// 調べた結果は設定の場所の update-check（0600）に残し、24 時間は見に行かない。

const (
	updateCheckFile  = "update-check"
	updateCheckEvery = 24 * time.Hour
	updateCheckWait  = 3 * time.Second // コマンドの前に始め、終わったあとはこれ以上待たない（遅い回線でコマンドを遅らせない）
)

type updateCache struct {
	Checked int64  `json:"checked"`
	Latest  string `json:"latest"`
}

// updateNotice は、1 回のコマンドのあいだの、新しい版の確かめ。
type updateNotice struct {
	done   chan struct{}
	latest string
}

// updateCheckOn は、知らせてよいときか（端末で、リリースのビルドで、止められていない）。
var updateCheckOn = func() bool {
	if version == "dev" || os.Getenv("KIROKU_NO_UPDATE_CHECK") != "" || os.Getenv("CI") != "" {
		return false
	}
	return isTerminal(os.Stdin) && isTerminal(os.Stdout) && isTerminal(os.Stderr) // 出力をパイプに流しているときは聞かない
}

func isTerminal(f *os.File) bool {
	st, err := f.Stat()
	return err == nil && st.Mode()&os.ModeCharDevice != 0
}

// startUpdateCheck は、コマンドの前に呼ぶ。知らせないときは nil。
func startUpdateCheck(cmd string) *updateNotice {
	switch cmd {
	case "update", "version", "--version", "-version", "-v", "help", "--help", "-help", "-h":
		return nil // update は自分で確かめる。版とヘルプは、すぐ終わってほしい
	}
	if !updateCheckOn() {
		return nil
	}
	n := &updateNotice{done: make(chan struct{})}
	go func() {
		defer close(n.done)
		n.latest = cachedOrFetchLatest()
	}()
	return n
}

// cachedOrFetchLatest は、24 時間以内に調べた結果があればそれを、なければ GitHub に聞いて残す。わからなければ ""。
func cachedOrFetchLatest() string {
	dir, err := configDir()
	if err != nil {
		return ""
	}
	path := filepath.Join(dir, updateCheckFile)
	var c updateCache
	if b, err := os.ReadFile(path); err == nil && json.Unmarshal(b, &c) == nil && validTag(c.Latest) &&
		time.Since(time.Unix(c.Checked, 0)) < updateCheckEvery && c.Checked <= time.Now().Unix() {
		return c.Latest
	}
	v, err := latestVersion(newUpdateClient())
	if err != nil || !validTag(v) {
		return ""
	}
	if os.MkdirAll(dir, 0o700) == nil {
		if b, err := json.Marshal(updateCache{Checked: time.Now().Unix(), Latest: v}); err == nil {
			tmp := path + ".tmp"
			if os.WriteFile(tmp, b, 0o600) == nil {
				_ = os.Rename(tmp, path)
			}
		}
	}
	return v
}

// newer は、調べ終わっていて、この版より新しい版があれば、その版。待つのは updateCheckWait まで。
func (n *updateNotice) newer(wait time.Duration) string {
	if n == nil {
		return ""
	}
	select {
	case <-n.done:
	case <-time.After(wait):
		return ""
	}
	cur := "v" + strings.TrimPrefix(version, "v")
	if n.latest == "" || compareVersions(n.latest, cur) <= 0 {
		return ""
	}
	return n.latest
}

// offer は、コマンドが終わったあとに、新しい版を知らせて聞く。y（yes）なら更新する。
func (n *updateNotice) offer(in io.Reader, out io.Writer) {
	v := n.newer(updateCheckWait)
	if v == "" {
		return
	}
	st := styleFor(os.Stderr)
	cur := "v" + strings.TrimPrefix(version, "v")
	fmt.Fprintf(out, "\n%s kiroku %s is available (this is %s). Update now? [y/N] ", st.warn("!"), v, cur)
	line, _ := bufio.NewReader(in).ReadString('\n')
	if a := strings.ToLower(strings.TrimSpace(line)); a != "y" && a != "yes" {
		fmt.Fprintf(out, "%s\n", st.dim("Skipped. Run \"kiroku update\" any time; set KIROKU_NO_UPDATE_CHECK=1 to stop these checks."))
		return
	}
	if err := updateNow(v); err != nil {
		fmt.Fprintln(out, st.bad(err.Error()))
	}
}

// note は、kiroku serve の始めに 1 行だけ知らせる（聞かない）。
func (n *updateNotice) note(out io.Writer) {
	v := n.newer(updateCheckWait)
	if v == "" {
		return
	}
	st := styleFor(os.Stderr)
	fmt.Fprintf(out, "%s kiroku %s is available (this is %s); run %s after stopping serve\n", st.warn("!"), v, "v"+strings.TrimPrefix(version, "v"), st.bold("\"kiroku update\""))
}

// updateNow は、y と答えたときの更新（kiroku update --to <版> と同じ。テストで差しかえる）。
var updateNow = func(v string) error { return runUpdate([]string{"--to", v}) }
