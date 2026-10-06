package main

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

// cmdDoctor は kiroku doctor。どの履歴が読めたか、消える設定のままか、kiroku archive・git・自動起動はどうかを一覧にして、
// 次に打つコマンドを案内する。読むだけで、設定もファイルも変えない。
func cmdDoctor(args []string) error {
	fs := newFS("doctor", "doctor [flags]\n\nChecks what kiroku can read and whether your history is at risk of being deleted,\nand suggests what to run next. It only reads; nothing is changed.")
	c := addCommon(fs)
	if _, err := parse(fs, args, 0); err != nil {
		return quiet(err)
	}
	picked, want := c.picked()
	logw = io.Discard // エージェントごとの読み込みの行は、下の一覧にまとめて出す
	data, rep := collect(picked, want, *c.gap)
	doctorReport(os.Stdout, rep, len(data), *c.archiveDir)
	return nil
}

// doctorReport は kiroku doctor の一覧を書く。
func doctorReport(w io.Writer, rep []source.Report, total int, archiveDir string) {
	fmt.Fprintf(w, "kiroku %s\n\nHistory found\n", version)
	for _, r := range rep {
		mark, what := "✓", plural(r.N, "session")
		if r.N == 0 {
			mark, what = "·", "none"
		} else if r.Oldest > 0 {
			what += ", since " + time.Unix(int64(r.Oldest), 0).Format("2006-01-02")
		}
		if r.Archived > 0 {
			what += fmt.Sprintf(" (%d from kiroku's copy)", r.Archived)
		}
		if r.Error != nil {
			mark, what = "!", what+"; some files couldn't be read: "+*r.Error
		}
		fmt.Fprintf(w, "  %s %-18s %s\n      %s\n", mark, r.Name, what, tilde(r.Where))
	}
	if total == 0 {
		fmt.Fprintln(w, "  ! No history found. If your agents keep it somewhere else, point kiroku to it\n      (--root, --kiro-home, --codex-home, --amazonq-db; see \"kiroku doctor --help\")")
	}

	on := archive.Enabled(archiveDir)
	fmt.Fprintln(w, "\nKeeping history")
	risky := false
	for _, r := range rep {
		k := r.Keep
		if k == nil || r.N == 0 {
			continue
		}
		switch {
		case k.Set:
			fmt.Fprintf(w, "  ✓ %s keeps history for %d days (%s)\n", r.Name, k.Days, k.Setting)
		case on && k.Days > 0:
			fmt.Fprintf(w, "  ✓ %s deletes history older than %d days, but kiroku archive keeps a copy\n", r.Name, k.Days)
		case on:
			fmt.Fprintf(w, "  ✓ %s deletes old history after a period (%s), but kiroku archive keeps a copy\n", r.Name, k.Setting)
		default:
			risky = true
			if k.Days > 0 {
				fmt.Fprintf(w, "  ! %s deletes history older than %d days (%s is at its default)\n", r.Name, k.Days, k.Setting)
			} else {
				fmt.Fprintf(w, "  ! %s deletes old history after a period (%s)\n", r.Name, k.Setting)
			}
			if k.File != "" {
				fmt.Fprintf(w, "      to keep it, add \"%s\": 3650 to %s\n", k.Setting, tilde(k.File))
			}
			fmt.Fprintf(w, "      how to set it: %s\n", k.Docs)
		}
	}
	files, size := archive.Usage(archiveDir)
	switch {
	case on:
		fmt.Fprintf(w, "  ✓ kiroku archive: on (%s, %s in %s)\n", nFiles(files), humanBytes(size), tilde(archiveDir))
	case risky:
		fmt.Fprintln(w, "  ! kiroku archive: off. Run \"kiroku archive on\" to keep copies of history before it is deleted")
	default:
		fmt.Fprintln(w, "  · kiroku archive: off (not needed while your agents keep their history)")
	}

	fmt.Fprintln(w, "\nOther")
	if lookGit() {
		fmt.Fprintln(w, "  ✓ git: commits and pushes appear next to sessions")
	} else {
		fmt.Fprintln(w, "  · git: not found, so commits and pushes are not shown")
	}
	a := autostartStatus()
	fmt.Fprintf(w, "  %s autostart: %s\n", a.mark, a.text)

	fmt.Fprintln(w, "\nNext")
	if total == 0 {
		fmt.Fprintln(w, "  use an agent for a while, or point kiroku to your history, then run \"kiroku doctor\" again")
		return
	}
	if risky { // 消えた履歴は戻らないので、見るより先に
		fmt.Fprintln(w, "  kiroku archive on    keep copies of history before it is deleted (or change the setting above)")
	}
	if a.running {
		fmt.Fprintf(w, "  open %s\n", a.url)
	} else {
		fmt.Fprintln(w, "  kiroku serve         open the view in your browser")
	}
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
