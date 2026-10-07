package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/MichinaoShimizu/kiroku/internal/archive"
	"github.com/MichinaoShimizu/kiroku/internal/source"
)

// lookGit は git を探す（テストで差しかえる）。
var lookGit = func() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

// doctorLatest は kiroku doctor で最新の版を確かめる（テストで差しかえる）。doctor を待たせないよう、短めに切る。
var doctorLatest = func() (string, error) {
	c := newUpdateClient()
	c.Timeout = 5 * time.Second
	return latestVersion(c)
}

// release は kiroku doctor が確かめた最新の版（checked が false なら確かめていない）。
type release struct {
	checked bool
	latest  string
	err     error
}

// cmdDoctor は kiroku doctor。どの履歴が読めたか、消える設定のままか、kiroku archive・git・自動起動・新しい版はどうかを一覧にして、
// 次に打つコマンドを案内する。読むだけで、設定もファイルも変えない。
func cmdDoctor(args []string) error {
	fs := newFS("doctor", "doctor [flags]\n\nChecks what kiroku can read, whether your history is at risk of being deleted\nand whether a newer kiroku is out, and suggests what to run next.\nIt only reads; nothing is changed.")
	noCheck := fs.Bool("no-update-check", false, "do not ask GitHub whether a newer release exists")
	all := fs.Bool("all", false, "list every agent kiroku looks at, including the ones with no history")
	c := addCommon(fs)
	if _, err := parse(fs, args, 0); err != nil {
		return quiet(err)
	}
	var rel release
	done := make(chan struct{})
	if *noCheck || version == "dev" { // 手元でビルドしたものは kiroku update で入れかえないので、確かめない
		close(done)
	} else {
		go func() { // 履歴を読むあいだに確かめる
			defer close(done)
			rel.latest, rel.err = doctorLatest()
			rel.checked = true
		}()
	}
	picked, want := c.picked()
	logw = io.Discard // エージェントごとの読み込みの行は、下の一覧にまとめて出す
	data, rep := collect(picked, want, *c.gap)
	<-done
	doctorReport(os.Stdout, rep, len(data), *c.archiveDir, rel, *all)
	return nil
}

// doctorReport は kiroku doctor の一覧を書く。all が true なら、履歴がなかったエージェントも場所まで並べる。
func doctorReport(w io.Writer, rep []source.Report, total int, archiveDir string, rel release, all bool) {
	s := styleFor(w)
	fmt.Fprintf(w, "kiroku %s\n\n%s\n", version, s.bold("History found"))
	// 履歴がなかったものは1行にまとめる。0 件が並ぶと、読んでほしい行がその下に押し出されてしまうため。
	// どこを探したのか知りたいときのために、どれも 0 件のときと --all では場所まで出す
	var none []string
	for _, r := range rep {
		if r.N == 0 && r.Error == nil && total > 0 && !all {
			none = append(none, r.Name)
			continue
		}
		mark, what := s.ok("✓"), plural(r.N, "session")
		if r.N == 0 {
			mark, what = s.dim("·"), "none"
		} else if r.Oldest > 0 {
			what += ", since " + time.Unix(int64(r.Oldest), 0).Format("2006-01-02")
		}
		if r.Archived > 0 {
			what += fmt.Sprintf(" (%d from kiroku's copy)", r.Archived)
		}
		if r.Error != nil {
			mark, what = s.warn("!"), what+"; some files couldn't be read: "+*r.Error
		}
		fmt.Fprintf(w, "  %s %-18s %s\n      %s\n", mark, r.Name, what, s.dim(where(r.Where)))
	}
	if len(none) > 0 {
		fmt.Fprintf(w, "  %s %-18s %s\n", s.dim("·"), "no history yet", s.dim(strings.Join(none, ", ")))
		fmt.Fprintf(w, "      %s\n", s.dim("\"kiroku doctor --all\" shows where kiroku looks for each one"))
	}
	if total == 0 {
		fmt.Fprintf(w, "  %s No history found. If your agents keep it somewhere else, point kiroku to it\n      %s\n", s.warn("!"), s.dim("(--root, --kiro-home, --codex-home, --amazonq-db; see \"kiroku doctor --help\")"))
	}

	on := archive.Enabled(archiveDir)
	fmt.Fprintf(w, "\n%s\n", s.bold("Keeping history"))
	risky := false
	for _, r := range rep {
		k := r.Keep
		if k == nil || r.N == 0 {
			continue
		}
		who := r.Name // 履歴を消すもの（Kiro CLI の履歴なら Kiro Crew）
		if k.Who != "" {
			who = k.Who
		}
		switch {
		case k.Now && on: // 0 日の設定（Kiro Crew）。Days の 0 は「わからない」なので Now で見分ける
			fmt.Fprintf(w, "  %s %s deletes old history at its next cleanup (%s is 0), but kiroku archive keeps a copy\n", s.ok("✓"), who, k.Setting)
		case k.Now:
			risky = true
			fmt.Fprintf(w, "  %s %s deletes old history at its next cleanup, within an hour (%s is 0)\n", s.warn("!"), who, k.Setting)
			if k.File != "" && k.Snippet != "" {
				fmt.Fprintf(w, "      to keep it, add %s to %s\n", s.bold(k.Snippet), tilde(k.File))
			}
			fmt.Fprintf(w, "      %s\n", s.dim("how to set it: "+k.Docs))
		case k.Set:
			fmt.Fprintf(w, "  %s %s keeps history for %d days (%s)\n", s.ok("✓"), who, k.Days, k.Setting)
		case on && k.Days > 0:
			fmt.Fprintf(w, "  %s %s deletes history older than %d days, but kiroku archive keeps a copy\n", s.ok("✓"), who, k.Days)
		case on:
			fmt.Fprintf(w, "  %s %s deletes old history after a period (%s), but kiroku archive keeps a copy\n", s.ok("✓"), who, k.Setting)
		default:
			risky = true
			if k.Days > 0 {
				fmt.Fprintf(w, "  %s %s deletes history older than %d days (%s is at its default)\n", s.warn("!"), who, k.Days, k.Setting)
			} else {
				fmt.Fprintf(w, "  %s %s deletes old history after a period (%s)\n", s.warn("!"), who, k.Setting)
			}
			if k.File != "" && k.Snippet != "" {
				fmt.Fprintf(w, "      to keep it, add %s to %s\n", s.bold(k.Snippet), tilde(k.File))
			}
			fmt.Fprintf(w, "      %s\n", s.dim("how to set it: "+k.Docs))
		}
	}
	files, size := archive.Usage(archiveDir)
	switch {
	case on:
		fmt.Fprintf(w, "  %s kiroku archive: on (%s, %s in %s)\n", s.ok("✓"), nFiles(files), humanBytes(size), tilde(archiveDir))
	case risky:
		fmt.Fprintf(w, "  %s kiroku archive: off. Run %s to keep copies of history before it is deleted\n", s.warn("!"), s.bold("\"kiroku archive on\""))
	default:
		fmt.Fprintf(w, "  %s kiroku archive: off %s\n", s.dim("·"), s.dim("(not needed while your agents keep their history)"))
	}

	fmt.Fprintf(w, "\n%s\n", s.bold("Other"))
	if lookGit() {
		fmt.Fprintf(w, "  %s git: commits and pushes appear next to sessions\n", s.ok("✓"))
	} else {
		fmt.Fprintf(w, "  %s git: not found, so commits and pushes are not shown\n", s.dim("·"))
	}
	a := autostartStatus()
	fmt.Fprintf(w, "  %s autostart: %s\n", s.mark(a.mark), a.text)
	cur := "v" + strings.TrimPrefix(version, "v")
	newer := false
	switch {
	case version == "dev":
		fmt.Fprintf(w, "  %s version: dev build from source %s\n", s.dim("·"), s.dim("(update it with \"go install github.com/MichinaoShimizu/kiroku@latest\")"))
	case !rel.checked:
		fmt.Fprintf(w, "  %s version: %s %s\n", s.dim("·"), cur, s.dim("(not checked for a newer release)"))
	case rel.err != nil:
		fmt.Fprintf(w, "  %s version: %s %s\n", s.dim("·"), cur, s.dim("(could not check for a newer release; \"kiroku update --check\" shows why)"))
	case compareVersions(rel.latest, cur) > 0:
		newer = true
		fmt.Fprintf(w, "  %s version: %s, and %s is available\n", s.warn("!"), cur, rel.latest)
	default:
		fmt.Fprintf(w, "  %s version: %s, the latest\n", s.ok("✓"), cur)
	}
	// どの kiroku が動いているか。install.sh と同じで、PATH の前のほうに別の kiroku があると "kiroku" はそちらを動かす
	exe := runningExe()
	if exe != "" {
		fmt.Fprintf(w, "      %s\n", s.dim(tilde(exe)))
	}
	if first := pathExe(); version != "dev" && exe != "" && first != "" && first != exe {
		fmt.Fprintf(w, "  %s another kiroku comes first in your PATH, so \"kiroku\" runs %s\n", s.warn("!"), tilde(first))
		fmt.Fprintf(w, "      %s\n", s.dim("remove it, or put this one's folder before it in your PATH"))
	}

	fmt.Fprintf(w, "\n%s\n", s.bold("Next"))
	if total == 0 {
		fmt.Fprintln(w, "  use an agent for a while, or point kiroku to your history, then run \"kiroku doctor\" again")
		return
	}
	if risky { // 消えた履歴は戻らないので、見るより先に
		fmt.Fprintf(w, "  %s keep copies of history before it is deleted (or change the setting above)\n", s.cmd("kiroku archive on", 20))
	}
	if newer {
		fmt.Fprintf(w, "  %s install %s\n", s.cmd("kiroku update", 20), rel.latest)
	}
	if a.running {
		fmt.Fprintf(w, "  open %s\n", s.bold(a.url))
	} else {
		fmt.Fprintf(w, "  %s open the view in your browser\n", s.cmd("kiroku serve", 20))
	}
}

// runningExe は、動いている kiroku の場所（テストで差しかえる）。
var runningExe = func() string {
	p, err := updateExe()
	if err != nil {
		return ""
	}
	if r, err := filepath.EvalSymlinks(p); err == nil {
		p = r
	}
	return p
}

// pathExe は、PATH でいちばん先に見つかる kiroku（テストで差しかえる）。
var pathExe = func() string {
	p, err := exec.LookPath("kiroku")
	if err != nil {
		return ""
	}
	if r, err := filepath.EvalSymlinks(p); err == nil {
		p = r
	}
	return p
}

// where は履歴の置き場所を書く。見つからなかったエージェントは、場所のかわりにそう書く。
func where(p string) string {
	if p == "" || p == "none" {
		return "not installed"
	}
	return tilde(p)
}

// plural は「1 session」「2 sessions」。
func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}

// tilde は、ホームの下のパスを ~ から書く（一覧を短くするため）。
func tilde(p string) string {
	h, err := os.UserHomeDir()
	if err != nil || h == "" {
		return p
	}
	if p == h {
		return "~"
	}
	if rel, err := filepath.Rel(h, p); err == nil && filepath.IsAbs(p) && !strings.HasPrefix(rel, "..") {
		return filepath.Join("~", rel)
	}
	return p
}
