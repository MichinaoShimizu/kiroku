package web

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

// 置き場の行は、改行が LF でも CRLF（Windows のチェックアウト）でも、行ごと中身に入れかわる。
func TestAssemble(t *testing.T) {
	for _, nl := range []string{"\n", "\r\n"} {
		got := assemble("a"+nl+"/*@x*/"+nl+"b"+nl+"//@y"+nl+"c", "/*@x*/", "X"+nl, "//@y", "Y"+nl)
		if want := "a" + nl + "X" + nl + "b" + nl + "Y" + nl + "c"; got != want {
			t.Errorf("%q: got %q, want %q", nl, got, want)
		}
	}
}

// 画面には CSP の meta があり、スクリプトと <style> のハッシュが中身と合っている。
// 通信は kiroku serve のときだけ自分のところへ、HTML ファイルでは一切できない。
func TestCSP(t *testing.T) {
	for _, live := range []bool{false, true} {
		data := []any{map[string]any{"title": "</script><script>alert(1)</script>"}}
		page, err := render(data, map[string]any{}, map[string]any{}, map[string]any{"report": []any{}, "git": []any{}}, 0, live)
		if err != nil {
			t.Fatal(err)
		}
		checkCSP(t, page, live)
	}
	checkCSP(t, string(LoadingFor("")), true)
}

func checkCSP(t *testing.T, page string, live bool) {
	t.Helper()
	// Windows のチェックアウトでは埋めこむファイルが CRLF になる。ブラウザは HTML を読むときに CRLF を LF にしてから
	// ハッシュを比べるので、ここでも同じようにしてから確かめる
	page = strings.ReplaceAll(strings.ReplaceAll(page, "\r\n", "\n"), "\r", "\n")
	m := regexp.MustCompile(`<meta http-equiv="Content-Security-Policy" content="([^"]*)">`).FindAllStringSubmatch(page, -1)
	if len(m) != 1 {
		t.Fatalf("CSP の meta が %d 個", len(m))
	}
	csp := m[0][1]
	if !strings.Contains(page, `<meta charset="utf-8">`+"\n"+m[0][0]) || strings.Index(page, m[0][0]) > strings.Index(page, "<style>") {
		t.Error("CSP の meta は charset のすぐあと、<style> より前に置く")
	}
	if strings.Contains(page, "__"+"CSP"+"__") {
		t.Error("CSP の置き場が残っている")
	}
	sum := func(re string) string {
		x := regexp.MustCompile(re).FindAllStringSubmatch(page, -1)
		if len(x) != 1 {
			t.Fatalf("%s が %d 個", re, len(x))
		}
		h := sha256.Sum256([]byte(x[0][1]))
		return "'sha256-" + base64.StdEncoding.EncodeToString(h[:]) + "'"
	}
	for _, want := range []string{
		"default-src 'none';",
		"script-src " + sum(`(?s)<script>(.*?)</script>`) + ";",
		"style-src-elem " + sum(`(?s)<style>(.*?)</style>`) + ";",
		"style-src-attr 'unsafe-inline';",
		"img-src data: blob:;",
		"base-uri 'none';", "form-action 'none';", "object-src 'none'",
	} {
		if !strings.Contains(csp, want) {
			t.Errorf("CSP に %q がない: %s", want, csp)
		}
	}
	connect := "connect-src 'none';"
	if live {
		connect = "connect-src 'self';"
	}
	if !strings.Contains(csp, connect) {
		t.Errorf("live=%v なのに %q がない: %s", live, connect, csp)
	}
	// kiroku serve の画面だけが、アプリとしてインストールするためのマニフェストを読む
	if hasLink, hasSrc := strings.Count(page, manifestLink) == 1, strings.Contains(csp, "; manifest-src 'self'"); hasLink != live || hasSrc != live {
		t.Errorf("live=%v でマニフェストのリンク %v・manifest-src %v: %s", live, hasLink, hasSrc, csp)
	}
	if strings.Contains(csp, "unsafe-eval") || strings.Contains(csp, "script-src 'unsafe-inline'") || strings.Contains(csp, "*") {
		t.Errorf("スクリプトを広く許している: %s", csp)
	}
}

// 履歴の中身に CSP の置き場と同じ文字があっても、入れかえるのは <head> の 1 つだけ。
// スクリプトが 2 つあるページは、ハッシュがずれるので誤りにする。
func TestWithCSP(t *testing.T) {
	mark := "__" + "CSP" + "__"
	page := `<head><meta content="` + mark + `"><style>a{}</style></head><script>var s="` + mark + `"</script>`
	got, err := withCSP(page, false)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(got, mark) != 1 || !strings.Contains(got, `var s="`+mark+`"`) {
		t.Errorf("データの側まで入れかえた: %s", got)
	}
	// kiroku serve の画面: CSP とマニフェストのリンクを、<title> と CSP の置き場がどちらの順でも 1 つずつ入れる
	for _, head := range []string{titleTag + `<meta content="` + mark + `">`, `<meta content="` + mark + `">` + titleTag} {
		page := `<head>` + head + `<style>a{}</style></head><script>var s="` + mark + `"</script>`
		got, err := withCSP(page, true)
		if err != nil {
			t.Fatal(err)
		}
		js, _ := inline(page, "script")
		css, _ := inline(page, "style")
		want := strings.Replace(strings.Replace(page, titleTag, titleTag+"\n"+manifestLink, 1), mark, policy(js, css, true), 1)
		if got != want {
			t.Errorf("live のページが違う:\n got %s\nwant %s", got, want)
		}
	}
	for _, bad := range []string{
		`<head><meta content="` + mark + `"><style></style></head><script>a</script><script>b</script>`,
		`<head><meta content="` + mark + `"><style></style></head><script src="x"></script><script>b</script>`,
		`<head><meta content="` + mark + `"><style></style></head><script>a`,
		`<head><meta content="` + mark + `"><style></style><style></style></head><script>a</script>`,
		`<head><meta content="x"><style></style></head><script>a</script>`,
	} {
		if _, err := withCSP(bad, false); err == nil {
			t.Errorf("誤りにならない: %s", bad)
		}
	}
	// CRLF でも、ブラウザと同じく LF にそろえたハッシュになる
	if hash("a\r\nb\rc") != hash("a\nb\nc") {
		t.Error("改行をそろえていない")
	}
}

// Parts.JSON（kiroku serve の /data.json）は、map を json.Marshal したときとバイトまで同じ。
// HTML に入れる JSON も、Render が値をそのまま JSON にしたときと同じ（履歴の < > & はエスケープしたまま）。
func TestPartsJSON(t *testing.T) {
	data := []any{map[string]any{"title": "</script><b>&", "start": 1.5}}
	weeks := map[string]any{"2026-09-28": map[string]any{"active": 3}}
	months := map[string]any{"2026-09": map[string]any{"active": 3}}
	meta := map[string]any{"report": []any{}, "git": []any{}, "note": "<!--"}
	p, err := Marshal(data, weeks, months, meta, 1759999999.25)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := json.Marshal(map[string]any{"sessions": data, "weeks": weeks, "months": months, "meta": meta, "generated": 1759999999.25})
	if got := p.JSON(); !bytes.Equal(got, want) {
		t.Errorf("JSON =\n%s\nwant\n%s", got, want)
	}
	page, err := p.HTML(true)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(page, "</script><b>") || strings.Contains(page, `"<!--"`) {
		t.Error("履歴の文字をエスケープせずに HTML に入れた")
	}
}

// 読み込み中の画面は --lang を属性に入れる。知らない値は入れない（属性を抜け出させない）
func TestLoadingForLang(t *testing.T) {
	if s := string(LoadingFor("ja")); !strings.Contains(s, `data-lang="ja"`) {
		t.Fatal("ja not set")
	}
	for _, bad := range []string{"", `"><script>alert(1)</script>`, "fr"} {
		s := string(LoadingFor(bad))
		if !strings.Contains(s, `data-lang=""`) || strings.Contains(s, "alert(1)") || strings.Contains(s, "__LANG__") {
			t.Fatalf("%q: lang leaked or placeholder left", bad)
		}
		checkCSP(t, s, true)
	}
}

// kiroku は AI のエージェントを動かさない（4.0 で「Analyze with AI」を外した）。画面には AI を呼ぶ道（/ai/run への送信や実行のダイアログ）がなく、
// 振り返り・日報はプロンプトをコピーするだけ
func TestNoAIExecution(t *testing.T) {
	for _, live := range []bool{false, true} {
		page, err := render([]any{}, map[string]any{}, map[string]any{}, map[string]any{"report": []any{}, "git": []any{}}, 0, live)
		if err != nil {
			t.Fatal(err)
		}
		for _, bad := range []string{"ai/run", "ai-review-dialog", "Analyze with AI", "aiRun", "consent"} {
			if strings.Contains(page, bad) {
				t.Errorf("live=%v: the page still has %q", live, bad)
			}
		}
		if !strings.Contains(page, "function copyPrompt") && !strings.Contains(page, "const copyPrompt") {
			t.Errorf("live=%v: no prompt copy", live)
		}
	}
}

// 取り除いた画面の部品（言語・テーマの切り替え、指標ガイド、ズーム、流れの出し方の切り替えと凡例、「まず見直す」の行と期間の行の ⚠、
// 改善のアドバイスの枠、式をまとめて開くボタン、プロジェクトのカード、単価の表のカード、問題の報告・診断のコピー、週報のボタン、
// Journey・Project の表示）が template.html と js/*.js に戻っていない。ブラウザの e2e で 1 つずつ数える代わりに、ここでまとめて見る。
// id は id="x"・#x・getElementById("x") の形で、class は class="…" の中の語・セレクタの .x・classList の引数の形で探す（ほかの語の一部は数えない）
func TestRemovedUIStaysGone(t *testing.T) {
	ids := []string{"lang", "theme", "guide", "mg", "zin", "zout", "flowBy", "worth", "fxbtn", "rptcopy", "rptbar", "rptrun", "journeybtn", "projectbtn", "calendarbtn"}
	classes := []string{"zoom", "tlkey", "tlkeyd", "icall", "insadv", "insadvc", "pcard", "ptab", "iunits", "iusw", "reportlink", "diagcopy", "insgo", "iksub"}
	files := map[string]string{"template.html": skeleton}
	ents, err := jsFiles.ReadDir("js")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range ents {
		b, err := jsFiles.ReadFile("js/" + e.Name())
		if err != nil {
			t.Fatal(err)
		}
		files["js/"+e.Name()] = string(b)
	}
	if len(files) < 5 {
		t.Fatalf("read only %d files", len(files))
	}
	const end = `(?:[^\w-]|$)`
	classAttr := regexp.MustCompile(`class=["']([^"']*)["']`)
	for _, id := range ids {
		q := regexp.QuoteMeta(id)
		re := regexp.MustCompile(`id=["']` + q + `["']|#` + q + end + `|getElementById\(\s*["'` + "`" + `]` + q + `["'` + "`" + `]`)
		for name, s := range files {
			if m := re.FindString(s); m != "" {
				t.Errorf("%s: removed id %q is back (%q)", name, id, m)
			}
		}
	}
	for _, c := range classes {
		q := regexp.QuoteMeta(c)
		re := regexp.MustCompile(`(?:^|[\s"'` + "`" + `,(>~+])\.` + q + end + `|classList\.\w+\(\s*["'` + "`" + `]` + q + `["'` + "`" + `]`)
		for name, s := range files {
			if m := re.FindString(s); m != "" {
				t.Errorf("%s: removed class %q is back (%q)", name, c, m)
			}
			for _, a := range classAttr.FindAllStringSubmatch(s, -1) {
				for _, tok := range strings.Fields(a[1]) {
					if tok == c {
						t.Errorf("%s: removed class %q is back (%q)", name, c, a[0])
					}
				}
			}
		}
	}
}

// 画面はいつもダーク。読み込み中の画面も同じ（ダークの配色と暗い地の色だけを持ち、ライトの配色は持たない）
func TestLoadingIsDark(t *testing.T) {
	s := string(LoadingFor(""))
	for _, want := range []string{`<meta name="color-scheme" content="dark">`, `:root{color-scheme:dark;--bg:#141218;`} {
		if !strings.Contains(s, want) {
			t.Errorf("loading page lacks %q", want)
		}
	}
	for _, bad := range []string{"color-scheme:light", "light dark", "data-theme", "prefers-color-scheme"} {
		if strings.Contains(s, bad) {
			t.Errorf("loading page still has %q", bad)
		}
	}
}

// render は Marshal と HTML を続けて呼ぶ。
func render(data, weeks, months, meta any, generated float64, live bool) (string, error) {
	p, err := Marshal(data, weeks, months, meta, generated)
	if err != nil {
		return "", err
	}
	return p.HTML(live)
}
