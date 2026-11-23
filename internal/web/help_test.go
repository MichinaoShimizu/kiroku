package web

import (
	"os"
	"regexp"
	"slices"
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

// guideMetricsHead は、ガイドの指標の表（定義と読み方を 1 つにした表）の見出しの行。
const guideMetricsHead = "| Metric | Definition | Tells you | Doesn't tell you | What to try |"

// 画面の「?」の説明（HELP）と、ガイドの指標の表（Tells you・Doesn't tell you・What to try の欄）が食い違わないようにする。
// どの HELP の行もガイドにちょうど 1 行あり、3 つの欄が同じ文であること。ガイドの表の行も、どれも HELP にあること。
func TestHelpMatchesGuide(t *testing.T) {
	guide, err := os.ReadFile("../../docs/guide.md")
	if err != nil {
		t.Fatal(err)
	}
	// 指標の表の行を、名前 → 読み方の 3 つの欄にする
	rowsOf := map[string][]string{}
	tables := 0
	lines := strings.Split(strings.ReplaceAll(string(guide), "\r\n", "\n"), "\n")
	for i := 0; i < len(lines); i++ {
		if lines[i] != guideMetricsHead {
			continue
		}
		tables++
		if i+1 >= len(lines) || lines[i+1] != "|---|---|---|---|---|" {
			t.Errorf("docs/guide.md の %d 行目の指標の表に区切りの行がない", i+1)
			continue
		}
		for i += 2; i < len(lines) && strings.HasPrefix(lines[i], "| "); i++ {
			line := lines[i]
			if !strings.HasSuffix(line, " |") {
				t.Errorf("docs/guide.md の %d 行目が表の行の形でない: %.60s", i+1, line)
				continue
			}
			cs := strings.Split(line[2:len(line)-2], " | ")
			if len(cs) != 5 || slices.Contains(cs, "") {
				t.Errorf("docs/guide.md の %d 行目は 5 つの欄が埋まっていない: %.60s", i+1, line)
				continue
			}
			if _, dup := rowsOf[cs[0]]; dup {
				t.Errorf("docs/guide.md の指標の表に %s の行が 2 つある", cs[0])
			}
			rowsOf[cs[0]] = cs[2:]
		}
	}
	if tables == 0 {
		t.Fatalf("docs/guide.md に指標の表（%s）がない", guideMetricsHead)
	}
	re := regexp.MustCompile(`\{n: "([^"]+)", d: "[^"]+", c: "([^"]+)", x: "([^"]+)", a: "([^"]+)"\}`)
	rows := re.FindAllStringSubmatch(block(t, "HELP"), -1)
	if len(rows) == 0 {
		t.Fatal("HELP が見つからない")
	}
	inHelp := map[string]bool{}
	for _, r := range rows {
		inHelp[r[1]] = true
		got, ok := rowsOf[r[1]]
		if !ok {
			t.Errorf("docs/guide.md の指標の表に %s の行がない", r[1])
			continue
		}
		for j, col := range []string{"Tells you", "Doesn't tell you", "What to try"} {
			if got[j] != r[j+2] {
				t.Errorf("docs/guide.md の %s の「%s」が画面と違う: %q、HELP は %q", r[1], col, got[j], r[j+2])
			}
		}
	}
	for name := range rowsOf {
		if !inHelp[name] {
			t.Errorf("docs/guide.md の指標の表の %s が HELP にない", name)
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
		"Usage limit hits":                     "limits",
		"Compactions":                          "compactions",
		"Files edited by AI (session details)": "files",
		"Subagents":                            "subagents",
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

// 見直す候補（review.js の add("…")）は、どれも説明（HELP）を持つこと。GOTO で別の指標に印を付けるものは、その先の説明を。
// 説明がないと、サマリーの「見直す候補」の行とダイアログ（openWorth）で開けず、候補が見えなくなる
func TestFindingsHaveHelp(t *testing.T) {
	src, err := jsFiles.ReadFile("js/review.js")
	if err != nil {
		t.Fatal(err)
	}
	help := block(t, "HELP")
	goTo := map[string]string{}
	if m := regexp.MustCompile(`const GOTO = \{([^}]*)\}`).FindSubmatch(src); m != nil {
		for _, kv := range regexp.MustCompile(`(\w+):\s*"(\w+)"`).FindAllSubmatch(m[1], -1) {
			goTo[string(kv[1])] = string(kv[2])
		}
	}
	ids := regexp.MustCompile(`\badd\("(\w+)"`).FindAllSubmatch(src, -1)
	if len(ids) == 0 {
		t.Fatal("review.js に見直す候補（add(\"…\")）が見つからない")
	}
	for _, m := range ids {
		id := string(m[1])
		if g, ok := goTo[id]; ok {
			id = g
		}
		if !strings.Contains(help, "\n  "+id+": {n: ") {
			t.Errorf("見直す候補 %s（印は %s）に HELP の説明がない", m[1], id)
		}
	}
}
