package source

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/MichinaoShimizu/kiroku/internal/core"
)

// source.All の名前・--sources の名前・kiroku archive のフォルダが、core.Agents と食い違っていない。
func TestAgentsCoverEverySource(t *testing.T) {
	root := t.TempDir()
	fams := map[string]bool{}
	for _, s := range All(Options{Archive: root, CrewHome: t.TempDir()}) {
		a, ok := core.AgentOf(s.Name())
		if !ok {
			t.Errorf("core.Agents に %q がない", s.Name())
			continue
		}
		if a.Family != s.Family() {
			t.Errorf("%s: core.Agents の Family %q, Source は %q", s.Name(), a.Family, s.Family())
		}
		fams[s.Family()] = true
		if k, ok := s.(Keeper); ok {
			for _, kept := range k.Keep() {
				rel, err := filepath.Rel(root, kept.Dst)
				dir := strings.Split(filepath.ToSlash(rel), "/")[0]
				if err != nil || !slices.Contains(core.ArchiveDirs(), dir) {
					t.Errorf("%s のコピーの置き場 %s が core.Agents の Archive %v の下にない", s.Name(), kept.Dst, core.ArchiveDirs())
				}
			}
		}
	}
	for _, f := range core.Families() {
		if !fams[f] {
			t.Errorf("core.Agents の Family %q を読む Source が source.All にない", f)
		}
	}
}
