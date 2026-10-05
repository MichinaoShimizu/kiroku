package web

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// block は画面のスクリプト（app.js）の `const <name> = {` から、次の `};` までを返す。
func block(t *testing.T, name string) string {
	t.Helper()
	start := "const " + name + " = {"
	i := strings.Index(template, start)
	if i < 0 {
		t.Fatalf("%s が見つからない", name)
	}
	j := strings.Index(template[i:], "\n};")
	if j < 0 {
		t.Fatalf("%s の終わりが見つからない", name)
	}
	return template[i : i+j]
}

// 画面の「?」の説明（HELP）と、ガイドの「How to read the metrics」の表が食い違わないようにする。
func TestHelpMatchesGuide(t *testing.T) {
	guide, err := os.ReadFile("../../docs/guide.md")
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`\{n: "([^"]+)", d: "[^"]+", c: "([^"]+)", x: "([^"]+)", a: "([^"]+)"\}`)
	rows := re.FindAllStringSubmatch(block(t, "HELP"), -1)
	if len(rows) == 0 {
		t.Fatal("HELP が見つからない")
	}
	for _, r := range rows {
		want := "| " + r[1] + " | " + r[2] + " | " + r[3] + " | " + r[4] + " |"
		if !strings.Contains(string(guide), want) {
			t.Errorf("docs/guide.md の「How to read the metrics」に %s の行がない（または画面と違う）", r[1])
		}
	}
}

// HELP のどの行も 4 つの欄が埋まっていて、日本語が残っていないこと。
func TestHelpFields(t *testing.T) {
	fields := regexp.MustCompile(`\{n: "[^"]+", d: "[^"]+", c: "[^"]+", x: "[^"]+", a: "[^"]+"\}`)
	for _, line := range strings.Split(block(t, "HELP"), "\n")[1:] {
		if strings.TrimSpace(line) != "" && !fields.MatchString(line) {
			t.Errorf("HELP の行の形が違う（n・d・c・x・a がそろっていない）: %.60s", strings.TrimSpace(line))
		}
	}
	if regexp.MustCompile(`\p{Han}|\p{Hiragana}|\p{Katakana}`).MatchString(block(t, "HELP")) {
		t.Error("HELP に日本語が残っている")
	}
}
