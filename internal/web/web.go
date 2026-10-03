// Package web は、1 ファイルで完結する HTML を書き出す。
package web

import (
	_ "embed"
	"encoding/json"
	"strings"
)

//go:embed template.html
var template string

// Render は __DATA__ などの置き場に JSON を入れる。
// encoding/json は < > & を < などに変えるので、</script> で閉じられる心配はない。
// live が true なら、画面は /stamp を見張って新しい履歴を取り込む（--serve 用）。
func Render(data, weeks, months, meta any, generated float64, live bool) (string, error) {
	repl := []string{}
	for k, v := range map[string]any{"__DATA__": data, "__WEEKS__": weeks, "__MONTHS__": months, "__META__": meta, "__GEN__": generated, "__LIVE__": live} {
		b, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		repl = append(repl, k, string(b))
	}
	return strings.NewReplacer(repl...).Replace(template), nil
}
