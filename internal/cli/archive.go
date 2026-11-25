package cli

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/MichinaoShimizu/kiroku/internal/archive"
	"github.com/MichinaoShimizu/kiroku/internal/core"
	"github.com/MichinaoShimizu/kiroku/internal/source"
)

// keepCopies は、古い履歴を消すエージェント（source.Keeper）の履歴のコピーを、保存場所に圧縮して残す。保存した数と、
// 大きすぎてコピーを残さなかったファイルのエラー（エージェントの名前ごと。画面の「Data sources」に読めなかったファイルとして出す）を返す。
func keepCopies(picked []source.Source, dir string) (int, map[string]error) {
	n := 0
	var skipped map[string]error
	for _, s := range picked {
		k, ok := s.(source.Keeper)
		if !ok {
			continue
		}
		for _, x := range k.Keep() {
			m, err := archive.Sync(x.Src, x.Dst)
			n += m
			if err != nil {
				fmt.Fprintf(logw, "  %s: could not keep a copy of some files in %s (%s)\n", s.Name(), termMsg(dir), termMsg(err.Error()))
			}
			if errors.Is(err, archive.ErrTooLarge) && skipped[s.Name()] == nil {
				if skipped == nil {
					skipped = map[string]error{}
				}
				skipped[s.Name()] = err
			}
		}
	}
	return n, skipped
}

// withSkipped は、大きすぎてコピーを残さなかったファイル（keepCopies）を、読み込みの結果の読めなかったファイルとして足す
// （ほかに読めなかったファイルがあれば、そちらを出す）。rep の写しを返す。
func withSkipped(rep []source.Report, skipped map[string]error) []source.Report {
	if len(skipped) == 0 {
		return rep
	}
	out := append([]source.Report(nil), rep...)
	for i := range out {
		if err := skipped[out[i].Name]; err != nil && out[i].Error == nil {
			msg := err.Error()
			out[i].Error = &msg
		}
	}
	return out
}

// errNoArchiveDir は、ホームがわからず（HOME が空など）、kiroku archive の保存場所が決まらないときのエラー。
// 今のフォルダの下に履歴のコピーを書かないよう、オンにするのを断る。
var errNoArchiveDir = errors.New("kiroku could not find your home folder ($HOME is not set), so it has no place to keep archive copies; set HOME, or choose a folder with KIROKU_ARCHIVE_DIR or --archive-dir")

// archiveOn は保存がオンか（保存場所が決まらなければオフ。今のフォルダを見ない）。
func archiveOn(dir string) bool { return dir != "" && archive.Enabled(dir) }

// archiveUsage は保存したコピーの数と大きさ（保存場所が決まらなければ 0）。
func archiveUsage(dir string) (int, int64) {
	if dir == "" {
		return 0, 0
	}
	return archive.Usage(dir)
}

// keptDirs は保存場所の下で kiroku archive が使うフォルダ（core.Agents の Archive）。
var keptDirs = core.ArchiveDirs()

func cmdArchive(args []string) error {
	fs := newFS("serve --archive", "serve --archive [flags] [on|off]\n\nAgents such as Claude Code delete old history (Claude Code: after 30 days unless cleanupPeriodDays is set).\n\"kiroku serve --archive on\" makes kiroku keep compressed copies of that history every time it reads it,\nand show deleted conversations from the copies. Copies stay on this computer only.\n\n  kiroku serve --archive        show the status\n  kiroku serve --archive on     start keeping copies (and keep a copy of the current history now)\n  kiroku serve --archive off    stop keeping copies (asks whether to delete the copies already kept)")
	c := addCommon(fs)
	pos, err := parse(fs, args, 1)
	if err != nil {
		return quiet(err)
	}
	dir := *c.archiveDir
	if dir == "" {
		return errNoArchiveDir
	}
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
			fmt.Println(`run "kiroku serve --archive on" to keep copies of history that agents delete`)
		}
		return nil
	case "on":
		if err := archive.Enable(dir); err != nil {
			return fmt.Errorf("could not create %s: %w", dir, err)
		}
		picked, _, _ := c.picked()
		n, _ := keepCopies(picked, dir)
		files, size := archive.Usage(dir)
		fmt.Printf("archive is on: kiroku keeps copies of history that agents delete in %s\nkept %s now (%s, %s in total); new history is kept each time kiroku reads it\n", dir, nFiles(n), nFiles(files), humanBytes(size))
		return nil
	case "off":
		marked := archive.Marked(dir) // オフにすると印の 1 つ（enabled）が消えるので、先に確かめる
		if err := archive.Disable(dir); err != nil {
			return err
		}
		files, size := archive.Kept(dir, keptDirs) // 消すものと同じ決まりで数える（kiroku が書いたコピーだけ）
		fmt.Println("archive is off: kiroku no longer keeps new copies")
		if files == 0 {
			return nil
		}
		if !marked { // まちがった --archive-dir や KIROKU_ARCHIVE_DIR で、ほかのフォルダのファイルを消さない
			fmt.Printf("%s does not look like a kiroku archive folder (kiroku serve --archive on was not run there), so kiroku did not offer to delete anything in it\n", dir)
			return nil
		}
		if askYes(fmt.Sprintf("delete the %s (%s) already kept in %s? deleted history cannot be shown again [y/N] ", nFiles(files), humanBytes(size), dir)) {
			// 消すのは kiroku が書いたコピー（.zst）だけ。ほかのファイルは残し、空になったフォルダだけを消す
			n, err := archive.Clear(dir, keptDirs)
			if err != nil {
				return fmt.Errorf("deleted %s, but could not delete the rest: %w", nFiles(n), err)
			}
			fmt.Printf("deleted the kept copies (%s)\n", nFiles(n))
			return nil
		}
		fmt.Printf("the kept copies stay in %s, and kiroku still shows them\n", dir)
		return nil
	}
	return fmt.Errorf("unknown argument: %s (use \"kiroku serve --archive\", \"kiroku serve --archive on\" or \"kiroku serve --archive off\")", cmd)
}

// askYes は端末で y と答えたときだけ true（端末でなければ聞かずに false）。テストで差しかえる。
var askYes = func(q string) bool {
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
