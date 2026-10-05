package report

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/MichinaoShimizu/kiroku/internal/core"
)

// モデルの記録がないセッション（古い Kiro IDE など）だけのプロジェクトでも、一覧は null ではなく空にする。
// 画面は null の一覧を扱えず、その週のサマリーと詳細が開かなくなっていた。
func TestProjectStatsNoNullLists(t *testing.T) {
	ws := time.Date(2026, 9, 28, 0, 0, 0, 0, time.Local)
	start := float64(ws.Add(10 * time.Hour).Unix())
	s := &core.Session{ID: "s1", Source: "Kiro IDE (legacy)", Project: "app", Start: start, End: start + 1800,
		Segs: [][3]float64{{start, start + 1800, 2}}, Prompts: []core.Prompt{}}
	w := Summarize([]*core.Session{s}, ws, ws.AddDate(0, 0, 7))
	if w == nil || len(w.ProjectStats) != 1 {
		t.Fatalf("プロジェクト別 = %+v", w)
	}
	b, _ := json.Marshal(w.ProjectStats)
	for _, k := range []string{`"models":null`, `"top":null`} {
		if strings.Contains(string(b), k) {
			t.Errorf("%s を含む: %s", k, b)
		}
	}
}
