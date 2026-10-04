package web

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// block は template.html の `const <name> = {` から、次の `};` までを返す。
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

// 画面の「?」の説明（HELP）と、ガイドの「指標の読み方」の表が食い違わないようにする。
func TestHelpMatchesGuide(t *testing.T) {
	guide, err := os.ReadFile("../../docs/guide.md")
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`\{n:"([^"]+)", d:"[^"]+", c:"([^"]+)", x:"([^"]+)", a:"([^"]+)"\}`)
	rows := re.FindAllStringSubmatch(block(t, "HELP"), -1)
	if len(rows) == 0 {
		t.Fatal("HELP が見つからない")
	}
	for _, r := range rows {
		want := "| " + r[1] + " | " + r[2] + " | " + r[3] + " | " + r[4] + " |"
		if !strings.Contains(string(guide), want) {
			t.Errorf("docs/guide.md の「指標の読み方」に %s の行がない（または画面と違う）", r[1])
		}
	}
}

// 英語表示の説明（HELP_EN）が、HELP と同じ指標を同じ順で持ち、どれも 4 つの欄が埋まっていること。
func TestHelpEnMatchesHelp(t *testing.T) {
	keyRe := regexp.MustCompile(`(?m)^\s*(\w+):\s*\{`)
	keys := func(name string) []string {
		var ks []string
		for _, m := range keyRe.FindAllStringSubmatch(block(t, name), -1) {
			ks = append(ks, m[1])
		}
		return ks
	}
	ja, en := keys("HELP"), keys("HELP_EN")
	if len(ja) == 0 {
		t.Fatal("HELP のキーが見つからない")
	}
	if strings.Join(ja, ",") != strings.Join(en, ",") {
		t.Errorf("HELP と HELP_EN のキーが違う\nHELP:    %v\nHELP_EN: %v", ja, en)
	}
	fields := regexp.MustCompile(`\{n: "[^"]+", d: "[^"]+", c: "[^"]+", x: "[^"]+", a: "[^"]+"\}`)
	for _, line := range strings.Split(block(t, "HELP_EN"), "\n")[1:] {
		if strings.TrimSpace(line) != "" && !fields.MatchString(line) {
			t.Errorf("HELP_EN の行の形が違う（n・d・c・x・a がそろっていない）: %.60s", strings.TrimSpace(line))
		}
	}
	if regexp.MustCompile(`\p{Han}|\p{Hiragana}|\p{Katakana}`).MatchString(block(t, "HELP_EN")) {
		t.Error("HELP_EN に日本語が残っている")
	}
}
