// Package web は、1 ファイルで完結する HTML を書き出す。
package web

import (
	"embed"
	"encoding/json"
	"io/fs"
	"path"
	"strings"

	"github.com/MichinaoShimizu/kiroku/internal/core"
)

// 画面は template.html（骨組み）・style.css・js/*.js に分けて書き、
// 起動時に置き場（/*@style.css*/ と //@js/*.js の行）へはめこんで 1 ファイルにする。
var (
	//go:embed template.html
	skeleton string
	//go:embed style.css
	style string
	//go:embed js/*.js
	jsFiles embed.FS

	template = assemble(skeleton, "/*@style.css*/", style, "//@js/*.js", script())
)

// scripts は js/ のファイルをつなぐ順。どれも同じ 1 つの <script> に入るので、上のファイルで
// 決めた名前は下のファイルから使える。読み込んだときに動く行（const や、はじめの描画）が
// まだ決まっていない名前を使わないよう、この順は変えない。ファイルを足したらここにも書く。
var scripts = []string{
	"state.js",    // 埋め込んだデータ・画面の状態
	"format.js",   // 書き方の小物・色・テーマ
	"calendar.js", // render・週と月のカレンダー・ツールチップ
	"summary.js",  // サマリー・指標の説明 HELP
	"review.js",   // 改善案のプロンプト・見直す候補と推移
	"panels.js",   // 検索結果・週報の下書き・アウトプット・計測の状態など
	"git.js",      // 詳細：コミット・push・PR
	"session.js",  // 詳細：セッション・プロンプトの流れ
	"year.js",     // 1 年の露光
	"events.js",   // ボタンとキーの操作
	"boot.js",     // 起動
	"live.js",     // kiroku serve の自動更新
}

// script は scripts の順に js/ のファイルをつなぐ。js/ にあって scripts にないファイルがあれば、
// 書き忘れなので止める。
func script() string {
	listed := map[string]bool{}
	var b strings.Builder
	for _, name := range scripts {
		body, err := jsFiles.ReadFile("js/" + name)
		if err != nil {
			panic("web: " + err.Error())
		}
		listed[name] = true
		b.Write(body)
	}
	all, err := fs.Glob(jsFiles, "js/*.js")
	if err != nil {
		panic("web: " + err.Error())
	}
	for _, p := range all {
		if !listed[path.Base(p)] {
			panic("web: " + p + " が scripts の並びにない")
		}
	}
	return b.String()
}

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

// Loading は、kiroku serve が最初の読み込みを終えるまで出す画面（stamp を見て、読み終わったら本物の画面に切りかわる）。
//
//go:embed loading.html
var Loading []byte
