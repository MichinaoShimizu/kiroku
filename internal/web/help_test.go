package web

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/MichinaoShimizu/kiroku/internal/core"
)

// block は画面のスクリプト（js/*.js）の `const <name> = {` から、次の `};` までを返す。
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

// ガイドの「What each agent records」の表のうち、core.Records で決まる行が、コードと食い違わないようにする。
// 列はエージェント（Session.Source）。記録するなら欄が ✓ で始まり、しないなら — で始まる。
func TestGuideRecordsTable(t *testing.T) {
	guide, err := os.ReadFile("../../docs/guide.md")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ReplaceAll(string(guide), "\r\n", "\n")
	i := strings.Index(text, "### What each agent records\n")
	if i < 0 {
		t.Fatal("docs/guide.md に「What each agent records」がない")
	}
	cells := func(line string) []string {
		var cs []string
		for _, c := range strings.Split(strings.Trim(line, "|"), "|") {
			cs = append(cs, strings.TrimSpace(c))
		}
		return cs
	}
	var head []string
	rows := map[string][]string{}
	for _, line := range strings.Split(text[i:], "\n")[1:] {
		if strings.HasPrefix(line, "#") {
			break
		}
		if !strings.HasPrefix(line, "| ") {
			continue
		}
		if cs := cells(line); head == nil {
			head = cs
		} else {
			rows[cs[0]] = cs
		}
	}
	if len(head) != len(core.Records)+1 {
		t.Fatalf("表の列 %v が core.Records のエージェント %d 個と合わない", head[1:], len(core.Records))
	}
	for _, src := range head[1:] {
		if _, ok := core.Records[src]; !ok {
			t.Errorf("表の列 %q が core.Records にない", src)
		}
	}
	for label, what := range map[string]string{
		"Interruptions (in Prompts with corrections or interruptions)": "interrupts",
		"Usage limit hits":                "limits",
		"Compactions":                     "compactions",
		"Files changed (session details)": "files",
		"Subagents":                       "subagents",
		"Outputs (AI commits, pull requests, sessions that reached a commit or PR, cost per commit)": "outputs",
	} {
		row, ok := rows[label]
		if !ok {
			t.Errorf("表に %q の行がない", label)
			continue
		}
		for j, src := range head[1:] {
			if got, want := strings.HasPrefix(row[j+1], "✓"), core.RecordsOf(src, what); got != want {
				t.Errorf("%s の %s: 表は %q、core.Records は %v", label, src, row[j+1], want)
			}
		}
	}
}
