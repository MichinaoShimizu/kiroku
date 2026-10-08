package web

import (
	"io/fs"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// uiRules は、共通の部品（js/ui.js）で作るものを、ほかのファイルで手書きしていないかの決まり。
// 手書きすると、見た目やエスケープ、読み上げの扱いが場所ごとにずれる（Repeated prompts のカードがそうだった）。
var uiRules = []struct {
	re   *regexp.Regexp
	want string
	skip string // この決まりの対象から外すファイル（その部品を定義しているファイル）
}{
	{regexp.MustCompile(`class="card[ "]`), "セッションのカードは sesCardH か sesCard で作る", ""},
	{regexp.MustCompile(`class="stat[ "]`), "数の枠は statH か norecStat で作る", ""},
	{regexp.MustCompile(`class="ph"`), "パネルの見出しは panelH で作る", ""},
	{regexp.MustCompile(`class="eyebrow"><span class="dot">`), "詳細パネルの頭は dHead で作る", ""},
	{regexp.MustCompile(`class="none"`), "空のときの一言は noneH で作る", ""},
	{regexp.MustCompile(`class="code[ "]`), "コードの枠（とコピー）は codeH で作る", ""},
	{regexp.MustCompile(`\.toLocaleString\(`), "数のカンマ区切りは commas（format.js）で書く", "js/format.js"},
	{regexp.MustCompile(`class="k muted"`), "見出しは本物の見出し（secH か h3）にする。小さな灰色の div で代わりにしない", ""},
	// 余白・文字の大きさ・色は style.css に書く。style="" に書いてよいのは、データから決まる位置・大きさとカスタムプロパティ（--c など）だけ
	{regexp.MustCompile(`style="(?:margin|padding|font|color|white-space|line-height|letter-spacing|text-)`), "余白や文字の見た目は style.css のクラスにする", ""},
}

func uiViolations(name, src string) []string {
	var out []string
	for i, line := range strings.Split(src, "\n") {
		for _, r := range uiRules {
			if r.skip != name && r.re.MatchString(line) {
				out = append(out, name+":"+strconv.Itoa(i+1)+": "+r.want)
			}
		}
	}
	return out
}

// 画面のスクリプトが、共通の部品を手書きしていないこと。ui.js は部品そのものなので除く。
func TestUIConventions(t *testing.T) {
	files, err := fs.Glob(jsFiles, "js/*.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if f == "js/ui.js" {
			continue
		}
		b, err := jsFiles.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, v := range uiViolations(f, string(b)) {
			t.Error(v)
		}
	}
	for _, v := range uiViolations("template.html", skeleton) {
		t.Error(v)
	}
}

// 決まりが、実際の手書きを見つけ、許されている書き方を通すこと（決まりが何も見ていない、にならないように）。
func TestUIRulesCatch(t *testing.T) {
	bad := []string{
		"`<button class=\"card ag\" data-id=\"${esc(s.id)}\">`",
		"`<div class=\"stat\"><div class=\"k\">${k}</div></div>`",
		"`<div class=\"stat norec\">`",
		"`<div class=\"ph\"><h3>Sessions</h3></div>`",
		"`<div class=\"eyebrow\"><span class=\"dot\"></span>GIT</div>`",
		"`<div style=\"margin-top:16px\" class=\"k muted\">By model</div>`",
		"`<p class=\"muted\" style=\"font-size:var(--fs-xs)\">`",
		"`<span style=\"color:var(--warn)\">`",
		"`<p class=\"none\">None</p>`",
		"`<div class=\"code\"><code>${esc(cmd)}</code></div>`",
		"`${n.toLocaleString(LOC())} characters`",
	}
	for _, s := range bad {
		if len(uiViolations("x.js", s)) == 0 {
			t.Errorf("見逃した: %s", s)
		}
	}
	ok := []string{
		"`<div class=\"cards\">`",
		"`<div class=\"stats\">`",
		"`<span style=\"--c:${colorOf(k)}\">`",
		"`<span style=\"left:${l}%;width:${w}%\">`",
		"`<div class=\"eyebrow\">Breakdown</div>`",
		"`<div class=\"codes\">`",
	}
	for _, s := range ok {
		if v := uiViolations("x.js", s); len(v) != 0 {
			t.Errorf("誤って止めた: %s: %v", s, v)
		}
	}
}

// 詳細の中にも出る部品（ボタン・チップ・カード）は transition:all を使わない。
// all だと、詳細を開いたときに引き継いだ visibility まで遅れて、開いた直後のボタンが空の枠に見える。
func TestNoTransitionAllInDrawerParts(t *testing.T) {
	for _, line := range strings.Split(style, "\n") {
		for _, sel := range []string{".pill{", ".segc button{", ".chip{", ".card{"} {
			if strings.HasPrefix(line, sel) && strings.Contains(line, "transition:all") {
				t.Errorf("%s uses transition:all", sel)
			}
		}
	}
}
