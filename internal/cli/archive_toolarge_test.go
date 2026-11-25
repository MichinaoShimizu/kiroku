package cli

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MichinaoShimizu/kiroku/internal/core"
	"github.com/MichinaoShimizu/kiroku/internal/source"
)

// keeperStub は、Src の下を残す Keeper（読むものはない）。
type keeperStub struct{ src, dst string }

func (k keeperStub) Name() string                   { return "Claude Code" }
func (k keeperStub) Family() string                 { return "claude" }
func (k keeperStub) Where() string                  { return k.src }
func (k keeperStub) Load(func(*core.Builder)) error { return nil }
func (k keeperStub) Keep() []source.Kept            { return []source.Kept{{Src: k.src, Dst: k.dst}} }

// 大きすぎてコピーを残さなかったファイルは、そのエージェントの読み込みの結果に、読めなかったファイルとして出る（Data sources・serve --check）。
// ほかに読めなかったファイルがあれば、そちらを残す。
func TestKeepCopiesReportsFilesOverTheCap(t *testing.T) {
	old := core.MaxZstd
	core.MaxZstd = 1 << 20
	t.Cleanup(func() { core.MaxZstd = old })
	oldW := logw
	logw = io.Discard
	t.Cleanup(func() { logw = oldW })

	src, dst := t.TempDir(), t.TempDir()
	os.MkdirAll(filepath.Join(src, "proj"), 0o700)
	f, err := os.Create(filepath.Join(src, "proj", "big.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	f.Truncate(core.MaxZstd + 1) // 穴のあいたファイル（ディスクは使わない）
	f.Close()

	n, skipped := keepCopies([]source.Source{keeperStub{src, dst}}, dst)
	if n != 0 || skipped["Claude Code"] == nil {
		t.Fatalf("keepCopies = %d %v, want 0 and an error for Claude Code", n, skipped)
	}
	other := "x.jsonl: permission denied"
	rep := []source.Report{{Name: "Claude Code"}, {Name: "Codex"}, {Name: "Claude Code", Error: &other}}
	got := withSkipped(rep, skipped)
	if got[0].Error == nil || !strings.Contains(*got[0].Error, "big.jsonl") || !strings.Contains(*got[0].Error, "too large") {
		t.Errorf("Claude Code error = %v, want the skipped file", got[0].Error)
	}
	if got[1].Error != nil {
		t.Errorf("Codex error = %q", *got[1].Error)
	}
	if *got[2].Error != other {
		t.Errorf("an earlier error was replaced: %q", *got[2].Error)
	}
	if rep[0].Error != nil {
		t.Error("withSkipped changed the report it was given")
	}
}
