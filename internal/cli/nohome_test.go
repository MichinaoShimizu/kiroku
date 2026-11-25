package cli

import (
	"os"
	"strings"
	"testing"
)

// HOME が空で、保存場所を決める環境変数もないとき、kiroku archive は今のフォルダの下に履歴のコピーを書かずに断る
func TestArchiveRefusesWithoutHome(t *testing.T) {
	cwd := t.TempDir()
	t.Chdir(cwd)
	for _, k := range []string{"HOME", "XDG_DATA_HOME", "KIROKU_ARCHIVE_DIR", "LocalAppData", "USERPROFILE"} {
		t.Setenv(k, "")
	}
	for _, args := range [][]string{{"serve", "--archive", "on", "--sources", "claude"}, {"serve", "--archive"}, {"serve", "--archive", "off"}} {
		err := dispatch(args)
		if err == nil || !strings.Contains(err.Error(), "KIROKU_ARCHIVE_DIR") {
			t.Errorf("%v = %v, want a refusal naming KIROKU_ARCHIVE_DIR", args, err)
		}
	}
	c := addCommon(newFS("serve", ""))
	if *c.archiveDir != "" {
		t.Errorf("--archive-dir default = %q, want empty", *c.archiveDir)
	}
	if err := c.keepFn()(); err != errNoArchiveDir {
		t.Errorf("keep from the view = %v, want errNoArchiveDir", err)
	}
	if archiveOn("") {
		t.Error("archive is on with no folder")
	}
	if es, _ := os.ReadDir(cwd); len(es) != 0 {
		t.Errorf("wrote %v in the current folder", es)
	}
}
