// Package web は、1 ファイルで完結する HTML を書き出す。
package web

import (
	_ "embed"
	"encoding/json"
	"strings"

	"github.com/MichinaoShimizu/kiroku/internal/core"
)

// 画面は template.html（骨組み）・style.css・app.js に分けて書き、
// 起動時に置き場（/*@style.css*/ と //@app.js の行）へはめこんで 1 ファイルにする。
var (
	//go:embed template.html
	skeleton string
	//go:embed style.css
	style string
	//go:embed app.js
	script string

	template = assemble(skeleton, "/*@style.css*/", style, "//@app.js", script)
)

// assemble は置き場の行（行末の改行ごと）を中身に入れかえる。Windows で CRLF に変わっていても同じように扱う。
// 置き場がちょうど 1 つずつないときは、作りの誤りなので止める。
func assemble(s string, pairs ...string) string {
	for i := 0; i < len(pairs); i += 2 {
		mark, body := pairs[i], pairs[i+1]
		if strings.Count(s, mark) != 1 {
			panic("web: " + mark + " の置き場がちょうど 1 つではない")
		}
		j := strings.Index(s, mark)
		k := j + len(mark)
		k += len(s[k:]) - len(strings.TrimPrefix(strings.TrimPrefix(s[k:], "\r"), "\n"))
		s = s[:j] + body + s[k:]
	}
	return s
}

// Render は __DATA__ などの置き場に JSON を入れる。
// encoding/json は < > & を < などに変えるので、</script> で閉じられる心配はない。
// live が true なら、画面は /stamp を見張って新しい履歴を取り込む（--serve 用）。
func Render(data, weeks, months, meta any, generated float64, live bool) (string, error) {
	repl := []string{}
	for k, v := range map[string]any{"__DATA__": data, "__WEEKS__": weeks, "__MONTHS__": months, "__META__": meta, "__GEN__": generated, "__LIVE__": live, "__PROMPT_RUNES__": core.PromptRunes} {
		b, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		repl = append(repl, k, string(b))
	}
	return strings.NewReplacer(repl...).Replace(template), nil
}
