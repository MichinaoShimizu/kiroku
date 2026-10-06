package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MichinaoShimizu/kiroku/internal/archive"
	"github.com/MichinaoShimizu/kiroku/internal/source"
)

// keepCopies は、古い履歴を消すエージェント（source.Keeper）の履歴のコピーを、保存場所に圧縮して残す。保存した数を返す。
func keepCopies(picked []source.Source, dir string) int {
	n := 0
	for _, s := range picked {
		k, ok := s.(source.Keeper)
		if !ok {
			continue
		}
		for _, x := range k.Keep() {
			m, err := archive.Sync(x.Src, x.Dst)
			n += m
			if err != nil {
				fmt.Fprintf(logw, "  %s: could not keep a copy of some files in %s (%v)\n", s.Name(), dir, err)
			}
		}
	}
	return n
}

// keptDirs は保存場所の下で kiroku archive が使うフォルダ（source.All の Archive の下の名前）。
var keptDirs = []string{"claude", "crew"}

func cmdArchive(args []string) error {
	fs := newFS("archive", "archive [flags] [on|off]\n\nAgents such as Claude Code delete old history (Claude Code: after 30 days unless cleanupPeriodDays is set).\n\"kiroku archive on\" makes kiroku keep compressed copies of that history every time it reads it,\nand show deleted conversations from the copies. Copies stay on this computer only.\n\n  kiroku archive        show the status\n  kiroku archive on     start keeping copies (and keep a copy of the current history now)\n  kiroku archive off    stop keeping copies (asks whether to delete the copies already kept)")
	c := addCommon(fs)
	pos, err := parse(fs, args, 1)
	if err != nil {
		return quiet(err)
	}
	dir := *c.archiveDir
	cmd := ""
	if len(pos) == 1 {
		cmd = pos[0]
	}
	switch cmd {
	case "":
		state := "off"
		if archive.Enabled(dir) {
			state = "on"
		}
		files, size := archive.Usage(dir)
		fmt.Printf("archive: %s\nlocation: %s\nkept: %s, %s\n", state, dir, nFiles(files), humanBytes(size))
		if state == "off" {
			fmt.Println(`run "kiroku archive on" to keep copies of history that agents delete`)
		}
		return nil
	case "on":
		if err := archive.Enable(dir); err != nil {
			return fmt.Errorf("could not create %s: %w", dir, err)
		}
		picked, _ := c.picked()
		n := keepCopies(picked, dir)
		files, size := archive.Usage(dir)
		fmt.Printf("archive is on: kiroku keeps copies of history that agents delete in %s\nkept %s now (%s, %s in total); new history is kept each time kiroku reads it\n", dir, nFiles(n), nFiles(files), humanBytes(size))
		return nil
	case "off":
		if err := archive.Disable(dir); err != nil {
			return err
		}
		files, size := archive.Usage(dir)
		fmt.Println("archive is off: kiroku no longer keeps new copies")
		if files == 0 {
			return nil
		}
		if askYes(fmt.Sprintf("delete the %s (%s) already kept in %s? deleted history cannot be shown again [y/N] ", nFiles(files), humanBytes(size), dir)) {
			for _, d := range keptDirs {
				if err := os.RemoveAll(filepath.Join(dir, d)); err != nil {
					return err
				}
			}
			fmt.Println("deleted the kept copies")
			return nil
		}
		fmt.Printf("the kept copies stay in %s, and kiroku still shows them\n", dir)
		return nil
	}
	return fmt.Errorf("unknown argument: %s (use \"kiroku archive\", \"kiroku archive on\" or \"kiroku archive off\")", cmd)
}

// askYes は端末で y と答えたときだけ true（端末でなければ聞かずに false）。
func askYes(q string) bool {
	if st, err := os.Stdin.Stat(); err != nil || st.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	fmt.Print(q)
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		fmt.Println() // 答えがないまま終わった（/dev/null など）。消さない
	}
	a := strings.ToLower(strings.TrimSpace(line))
	return a == "y" || a == "yes"
}

func nFiles(n int) string {
	if n == 1 {
		return "1 file"
	}
	return fmt.Sprintf("%d files", n)
}

func humanBytes(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d B", n)
}
