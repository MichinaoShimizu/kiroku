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
		page, err := Render(data, map[string]any{}, map[string]any{}, map[string]any{"report": []any{}, "git": []any{}}, 0, live)
		if err != nil {
			t.Fatal(err)
		}
		checkCSP(t, page, live)
	}
	checkCSP(t, string(Loading), true)
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
	if old, _ := Render(data, weeks, months, meta, 1759999999.25, true); old != page {
		t.Error("Render と Parts.HTML の結果が違う")
	}
}
