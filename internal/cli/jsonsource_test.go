package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/MichinaoShimizu/kiroku/internal/core"
)

// 画面では「Kiro IDE / CLI (V3)」と出すが、kiroku json の source は 1.0 で約束した「Kiro IDE / CLI」のまま（docs/compatibility.md）。
// 書き出すときに写しで戻し、kiroku serve のキャッシュと共有しているセッションは変えない。
func TestJSONKeepsCompatibleSource(t *testing.T) {
	v3 := &core.Session{ID: "a", Source: "Kiro IDE / CLI (V3)"}
	cc := &core.Session{ID: "b", Source: "Claude Code"}
	out := filepath.Join(t.TempDir(), "k.json")
	if err := writeJSON(snapshot{data: []*core.Session{v3, cc}}, out); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Sessions []struct{ Source string } `json:"sessions"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Sessions) != 2 || doc.Sessions[0].Source != "Kiro IDE / CLI" || doc.Sessions[1].Source != "Claude Code" {
		t.Errorf("sources = %+v", doc.Sessions)
	}
	if v3.Source != "Kiro IDE / CLI (V3)" {
		t.Errorf("共有しているセッションを変えた: %q", v3.Source)
	}
}
