package web

import (
	"strings"
	"testing"
)

// 履歴の文字列はどんな中身でも、画面のスクリプトの外へ出ない（</script> で閉じたり、別のタグを始めたりしない）。
// スクリプトは 1 つだけで、CSP のハッシュは中身と合っている。
func FuzzRender(f *testing.F) {
	for _, s := range []string{
		"</script><script>alert(1)</script>",
		"<!--<script>", "</SCRIPT >", "  ", "__DATA__ __META__ __" + "CSP" + "__", "<img src=x onerror=alert(1)>",
	} {
		f.Add(s)
	}
	meta := map[string]any{"report": []any{}, "git": []any{}}
	f.Fuzz(func(t *testing.T, s string) {
		data := []any{map[string]any{"title": s, "prompts": []any{s}, "project": s}}
		page, err := Render(data, map[string]any{s: s}, map[string]any{}, meta, 0, false)
		if err != nil {
			t.Fatal(err)
		}
		lower := strings.ToLower(page)
		if n := strings.Count(lower, "<script"); n != 1 {
			t.Fatalf("%d <script> tags", n)
		}
		if n := strings.Count(lower, "</script"); n != 1 {
			t.Fatalf("%d </script> tags", n)
		}
		if strings.Contains(lower, "<!--") {
			t.Fatal("history opened an HTML comment")
		}
		if !strings.Contains(s, "__"+"CSP"+"__") { // 置き場と同じ文字が残っていないかは、データにないときだけ見られる
			checkCSP(t, page, false)
		}
	})
}
