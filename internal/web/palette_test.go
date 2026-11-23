package web

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/MichinaoShimizu/kiroku/internal/core"
)

// カテゴリーの 8 色（--c0〜--c7）は、画面の暗い地（いつもダーク）で、決めた基準を満たす組み合わせだけを使う。
// プロジェクトは多い順にこの順で色を取るので、隣どうし（積み上げの帯・凡例で並ぶ色）が見分けられることを確かめる:
//   - 明るさ（OKLCH の L）がそろっている（0.48〜0.67）
//   - 鮮やかさ（C）が 0.10 以上（灰色に見えない）で、8 色の差が 0.02 以内（どれかだけが強く見えない）
//   - 隣どうしの OKLab の差（×100）が、ふつうの見え方で 15 以上、1 型・2 型の色覚（Machado 2009）で 6 以上
//
// 色を変えるときは、このテストが通る値を選ぶ（目で選ばない）。
func TestCategoricalPalette(t *testing.T) {
	css := style
	for _, m := range []struct {
		name, sel string
		lo, hi    float64
	}{
		{"dark", ":root{", 0.48, 0.67},
	} {
		i := strings.Index(css, m.sel)
		if i < 0 {
			t.Fatalf("%s: %s がない", m.name, m.sel)
		}
		block := css[i : i+strings.Index(css[i:], "\n}")]
		var pal []string
		for k := 0; k < 8; k++ {
			mm := regexp.MustCompile(`--c` + strconv.Itoa(k) + `:(#[0-9a-fA-F]{6})`).FindStringSubmatch(block)
			if mm == nil {
				t.Fatalf("%s: --c%d がない", m.name, k)
			}
			pal = append(pal, mm[1])
		}
		minC, maxC := 1.0, 0.0
		for _, c := range pal {
			L, C := lch(c)
			minC, maxC = math.Min(minC, C), math.Max(maxC, C)
			if L < m.lo || L > m.hi {
				t.Errorf("%s: %s の明るさ %.3f が %.2f〜%.2f の外", m.name, c, L, m.lo, m.hi)
			}
			if C < 0.10 {
				t.Errorf("%s: %s の鮮やかさ %.3f が 0.10 未満（灰色に見える）", m.name, c, C)
			}
		}
		if maxC-minC > 0.02 {
			t.Errorf("%s: 鮮やかさが %.3f〜%.3f とばらつく（0.02 以内に）", m.name, minC, maxC)
		}
		for k := 0; k+1 < len(pal); k++ {
			a, b := pal[k], pal[k+1]
			if d := deltaE(a, b, nil); d < 15 {
				t.Errorf("%s: --c%d %s と --c%d %s の差 %.1f が 15 未満（ふつうの見え方でも近い）", m.name, k, a, k+1, b, d)
			}
			for name, mx := range map[string]*[3][3]float64{"protan": &protan, "deutan": &deutan} {
				if d := deltaE(a, b, mx); d < 6 {
					t.Errorf("%s: --c%d %s と --c%d %s の %s での差 %.1f が 6 未満", m.name, k, a, k+1, b, name, d)
				}
			}
		}
	}
}

// 画面はいつもダーク。配色は :root の 1 組だけで、ライトの配色や OS の設定に合わせる切り替えは残さない
func TestViewIsDarkOnly(t *testing.T) {
	if n := strings.Count(style, ":root{"); n != 1 {
		t.Errorf("style.css has %d :root{ blocks, want 1", n)
	}
	if !strings.Contains(strings.ReplaceAll(style, "\r\n", "\n"), ":root{\n  color-scheme:dark;") { // Windows のチェックアウトは CRLF
		t.Error("style.css :root does not start with color-scheme:dark")
	}
	if !strings.Contains(skeleton, `<meta name="color-scheme" content="dark">`) {
		t.Error(`template.html lacks <meta name="color-scheme" content="dark">`)
	}
	for name, s := range map[string]string{"style.css": style, "template.html": skeleton} {
		for _, bad := range []string{"color-scheme:light", "light dark", "data-theme", "prefers-color-scheme"} {
			if strings.Contains(s, bad) {
				t.Errorf("%s still has %q", name, bad)
			}
		}
	}
}

var (
	protan = [3][3]float64{{0.152286, 1.052583, -0.204868}, {0.114503, 0.786281, 0.099216}, {-0.003882, -0.048116, 1.051998}}
	deutan = [3][3]float64{{0.367322, 0.860646, -0.227968}, {0.280085, 0.672501, 0.047413}, {-0.011820, 0.042940, 0.968881}}
)

func linRGB(h string) [3]float64 {
	var out [3]float64
	for i := 0; i < 3; i++ {
		v, _ := strconv.ParseUint(h[1+2*i:3+2*i], 16, 8)
		c := float64(v) / 255
		if c <= 0.04045 {
			out[i] = c / 12.92
		} else {
			out[i] = math.Pow((c+0.055)/1.055, 2.4)
		}
	}
	return out
}

func oklab(rgb [3]float64) [3]float64 {
	l := math.Cbrt(0.4122214708*rgb[0] + 0.5363325363*rgb[1] + 0.0514459929*rgb[2])
	m := math.Cbrt(0.2119034982*rgb[0] + 0.6806995451*rgb[1] + 0.1073969566*rgb[2])
	s := math.Cbrt(0.0883024619*rgb[0] + 0.2817188376*rgb[1] + 0.6299787005*rgb[2])
	return [3]float64{0.2104542553*l + 0.7936177850*m - 0.0040720468*s, 1.9779984951*l - 2.4285922050*m + 0.4505937099*s, 0.0259040371*l + 0.7827717662*m - 0.8086757660*s}
}

func lch(h string) (float64, float64) {
	o := oklab(linRGB(h))
	return o[0], math.Hypot(o[1], o[2])
}

func deltaE(a, b string, sim *[3][3]float64) float64 {
	conv := func(h string) [3]float64 {
		c := linRGB(h)
		if sim != nil {
			var d [3]float64
			for i := 0; i < 3; i++ {
				d[i] = math.Max(0, math.Min(1, sim[i][0]*c[0]+sim[i][1]*c[1]+sim[i][2]*c[2]))
			}
			c = d
		}
		return oklab(c)
	}
	x, y := conv(a), conv(b)
	return 100 * math.Sqrt((x[0]-y[0])*(x[0]-y[0])+(x[1]-y[1])*(x[1]-y[1])+(x[2]-y[2])*(x[2]-y[2]))
}

// プロンプトの流れ（.tl）の色は、メッセージアプリ（LINE の昔からのトーク画面）に寄せた吹き出し。字と地の組み合わせが読めることを確かめる:
//   - 吹き出しの字・時刻・小さな字（4.5:1 以上。WCAG 2.x の相対輝度のコントラスト比）
//   - コピーのボタン・AI の印・出来事の印・言い直しの破線など小さな印（3:1 以上）
//   - AI の白い吹き出しと流れの地（3:1 以上）
//
// 形は同じでも、色は LINE そのもの（地 #7494c0・緑 #8de055）から少しずらしてある。同じ値に戻らないことも確かめる。
//
// 半透明の地（--chat-me-tag と出来事の札の --chat-sys）と、ふだん薄く見せるコピーのボタン（最後に書いた .tl li .pcopy{opacity:…}）は、下の地と混ぜた色で比べる。
func TestChatContrast(t *testing.T) {
	i := strings.Index(style, ".tl{--chat-bg:")
	if i < 0 {
		t.Fatal("style.css has no .tl{--chat-bg: block")
	}
	block := style[i : i+strings.Index(style[i:], "}")]
	v := func(name string) string {
		m := regexp.MustCompile(`--` + name + `:(#[0-9a-fA-F]{6})[;}]?`).FindStringSubmatch(block + "}")
		if m == nil {
			t.Fatalf("--%s is not a #rrggbb color in .tl", name)
		}
		return m[1]
	}
	black := func(name string) float64 { // rgba(0,0,0,a) の a
		m := regexp.MustCompile(`--` + name + `:rgba\(0,0,0,([0-9.]+)\)`).FindStringSubmatch(block)
		if m == nil {
			t.Fatalf("--%s is not rgba(0,0,0,a) in .tl", name)
		}
		a, _ := strconv.ParseFloat(m[1], 64)
		return a
	}
	ops := regexp.MustCompile(`\.tl li \.pcopy\{opacity:([0-9.]+)\}`).FindAllStringSubmatch(style, -1)
	if ops == nil {
		t.Fatal("style.css has no .tl li .pcopy{opacity:…}")
	}
	o, _ := strconv.ParseFloat(ops[len(ops)-1][1], 64) // 後に書いたものが効く
	bg, me, ai, ink := v("chat-bg"), v("chat-me"), v("chat-ai"), v("chat-ink")
	tag := blend("#000000", me, black("chat-me-tag"))
	sys := blend("#000000", bg, black("chat-sys"))
	for _, c := range []struct {
		name, fg, bg string
		min          float64
	}{
		{"your prompt", v("chat-me-ink"), me, 4.5},
		{"your prompt's meta and cut note", v("chat-me-meta"), me, 4.5},
		{"tag in your prompt", v("chat-me-ink"), tag, 4.5},
		{"correction or long tag in your prompt", v("chat-me-warn"), tag, 4.5},
		{"correction's dashed outline", v("chat-me-warn"), me, 3},
		{"AI reply", v("chat-ai-ink"), ai, 4.5},
		{"AI bubble on the background", ai, bg, 3},
		{"AI reply meta and cut note", v("chat-ai-meta"), ai, 4.5},
		{"link in an AI reply", v("chat-link"), ai, 4.5},
		{"AI mark in its round avatar", v("chat-ai-mark"), ai, 3},
		{"time beside a bubble, gap and quiet reply text", ink, bg, 4.5},
		{"focus ring on the background", ink, bg, 3},
		{"copy button at rest", blend(ink, bg, o), bg, 3},
		{"event text", v("chat-sys-ink"), sys, 4.5},
		{"note mark", v("chat-note"), sys, 3},
		{"limit or interruption text", v("chat-warn-ink"), v("chat-warn"), 4.5},
	} {
		if r := contrast(c.fg, c.bg); r < c.min {
			t.Errorf("%s: %s on %s is %.2f:1, want at least %.1f:1", c.name, c.fg, c.bg, r, c.min)
		}
	}
	for _, c := range []struct{ name, got, line string }{
		{"background", bg, "#7494c0"},
		{"your bubble", me, "#8de055"},
	} {
		if d := deltaE(c.got, c.line, nil); d < 3 {
			t.Errorf("%s %s is too close to LINE's %s (ΔE %.1f), want at least 3", c.name, c.got, c.line, d)
		}
	}
}

// style.css の { } がそろっている。余分な } があると、ブラウザはその後ろの規則をひとつ黙って捨てる
// （プロンプトの流れの、返事の文がない回の見た目がこれで消えていた）
func TestStyleBraces(t *testing.T) {
	s := regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllStringFunc(style, func(c string) string { return strings.Repeat("\n", strings.Count(c, "\n")) })
	depth := 0
	for i, line := range strings.Split(s, "\n") {
		for _, ch := range line {
			switch ch {
			case '{':
				depth++
			case '}':
				depth--
				if depth < 0 {
					t.Fatalf("style.css line %d: } without a matching {", i+1)
				}
			}
		}
	}
	if depth != 0 {
		t.Fatalf("style.css ends with %d unclosed {", depth)
	}
}

// サマリーの数字の字と地（4.5:1 以上）。パネルとカードの地は --s-lowest、数の枠（.stat）の地は --surface-2。
// いつもの範囲から大きく外れた量の色（--odd-ink）も、値と札の字なので同じ基準で確かめる
func TestSummaryContrast(t *testing.T) {
	i := strings.Index(style, ":root{")
	block := style[i : i+strings.Index(style[i:], "\n}")]
	v := func(name string) string {
		m := regexp.MustCompile(`--` + name + `:(#[0-9a-fA-F]{6});`).FindStringSubmatch(block)
		if m == nil {
			t.Fatalf("--%s is not a #rrggbb color in :root", name)
		}
		return m[1]
	}
	for _, c := range []struct{ name, fg, bg string }{
		{"panel and card value", v("ink"), v("s-lowest")},
		{"secondary line in a card", v("ink-2"), v("s-lowest")},
		{"value in a stat box", v("ink"), v("surface-2")},
		{"unusual value and its mark in a panel or card", v("odd-ink"), v("s-lowest")},
		{"unusual value and its mark in a stat box", v("odd-ink"), v("surface-2")},
		{"IN label above the panels", v("in"), v("bg")},
		{"OUT label above the panels", v("out"), v("bg")},
		{"IN label in a panel (one column)", v("in"), v("s-lowest")},
		{"OUT label in a panel (one column)", v("out"), v("s-lowest")},
		{"Sankey name on its backing", v("ink-2"), v("s-lowest")},
		{"Sankey amount on its backing", v("ink-3"), v("s-lowest")},
	} {
		if r := contrast(c.fg, c.bg); r < 4.5 {
			t.Errorf("%s: %s on %s is %.2f:1, want at least 4.5:1", c.name, c.fg, c.bg, r)
		}
	}
}

// 投入（IN）と出力（OUT）の側を分ける縦線とパネルの見出しの印：線は地に対して 3:1 以上（WCAG の UI の部品）。2 色は互いに違う色相で、良し悪しの緑・赤ではない
func TestSideColors(t *testing.T) {
	i := strings.Index(style, ":root{")
	block := style[i : i+strings.Index(style[i:], "\n}")]
	v := func(name string) string {
		m := regexp.MustCompile(`--` + name + `:(#[0-9a-fA-F]{6});`).FindStringSubmatch(block)
		if m == nil {
			t.Fatalf("--%s is not a #rrggbb color in :root", name)
		}
		return m[1]
	}
	for _, side := range []string{"in", "out"} {
		for _, bg := range []string{"bg", "s-lowest"} {
			if r := contrast(v(side), v(bg)); r < 3 {
				t.Errorf("--%s on --%s is %.2f:1, want at least 3:1", side, bg, r)
			}
		}
		for _, other := range []string{"go", "warn", "odd-ink"} {
			if v(side) == v(other) {
				t.Errorf("--%s is the same color as --%s, which means good, bad or unusual", side, other)
			}
		}
	}
	if v("in") == v("out") {
		t.Error("--in and --out are the same color")
	}
}

// blend は、fg を不透明度 a で bg の上に重ねた色（sRGB の値で混ぜる。ブラウザーの合成と同じ）
func blend(fg, bg string, a float64) string {
	out := "#"
	for i := 0; i < 3; i++ {
		f, _ := strconv.ParseUint(fg[1+2*i:3+2*i], 16, 8)
		b, _ := strconv.ParseUint(bg[1+2*i:3+2*i], 16, 8)
		out += strconv.FormatInt(int64(math.Round(a*float64(f)+(1-a)*float64(b)))+0x100, 16)[1:]
	}
	return out
}

func contrast(a, b string) float64 {
	lum := func(h string) float64 { c := linRGB(h); return 0.2126*c[0] + 0.7152*c[1] + 0.0722*c[2] }
	x, y := lum(a), lum(b)
	if x < y {
		x, y = y, x
	}
	return (x + 0.05) / (y + 0.05)
}

// rootVar は style.css の :root にある #rrggbb の色
func rootVar(t *testing.T, name string) string {
	t.Helper()
	i := strings.Index(style, ":root{")
	block := style[i : i+strings.Index(style[i:], "\n}")]
	m := regexp.MustCompile(`--` + name + `:(#[0-9a-fA-F]{6});`).FindStringSubmatch(block)
	if m == nil {
		t.Fatalf("--%s is not a #rrggbb color in :root", name)
	}
	return m[1]
}

func hue(h string) float64 {
	o := oklab(linRGB(h))
	return math.Mod(math.Atan2(o[2], o[1])*180/math.Pi+360, 360)
}

// エージェントの色（--a0〜、core.Agents の Slot）。週のカレンダーの帯・凡例・月のマスの細い棒・Sankey のエージェントの列・By agent の表で同じ色:
//   - どの 2 色も、ふつうの見え方で ΔE 15 以上、1 型・2 型の色覚で 6 以上（区分の 8 色の隣どうしと同じ基準を、すべての組に）
//   - 良し悪しの緑・赤、外れた量の琥珀に見えない（OKLCH の色相が 195°〜355°、青・紫・ピンクの側）。IN・OUT・琥珀・良い側の緑とも ΔE 12 以上離す
//   - 帯の左端の線は地に対して 3:1 以上
//   - 読み込み中の画面（loading.html）も同じ値
func TestAgentPalette(t *testing.T) {
	var pal []string
	for k := 0; k < core.AgentColors; k++ {
		pal = append(pal, rootVar(t, "a"+strconv.Itoa(k)))
	}
	for i, a := range pal {
		if h := hue(a); h < 195 || h > 355 {
			t.Errorf("--a%d %s has hue %.0f°, which reads as green, red or amber (want 195°–355°)", i, a, h)
		}
		for _, o := range []string{"in", "out", "amber-ink", "odd-ink", "go", "go-ink", "warn", "other"} {
			if d := deltaE(a, rootVar(t, o), nil); d < 12 {
				t.Errorf("--a%d %s is too close to --%s (ΔE %.1f, want at least 12)", i, a, o, d)
			}
		}
		for _, bg := range []string{"surface", "s-lowest"} {
			if r := contrast(a, rootVar(t, bg)); r < 3 {
				t.Errorf("--a%d %s on --%s is %.2f:1, want at least 3:1", i, a, bg, r)
			}
		}
		if !strings.Contains(loading, "--a"+strconv.Itoa(i)+":"+a+";") {
			t.Errorf("loading.html does not set --a%d:%s like style.css", i, a)
		}
		for j := i + 1; j < len(pal); j++ {
			b := pal[j]
			if d := deltaE(a, b, nil); d < 15 {
				t.Errorf("--a%d %s and --a%d %s differ by %.1f, want at least 15", i, a, j, b, d)
			}
			for name, mx := range map[string]*[3][3]float64{"protan": &protan, "deutan": &deutan} {
				if d := deltaE(a, b, mx); d < 6 {
					t.Errorf("--a%d %s and --a%d %s differ by %.1f for %s, want at least 6", i, a, j, b, d, name)
				}
			}
		}
	}
}

// 週のカレンダーの帯は、色分け（エージェント・プロジェクト）の色を地（--surface）に --fill だけ混ぜた塗り。
// いちばん濃い帯（calendar.js の --fill の上限）でも、名前（--ink）と時刻（--ink-2）が 4.5:1 以上。区分の 8 色・エージェントの色・灰色のすべてで確かめる
func TestCalendarBlockContrast(t *testing.T) {
	if !strings.Contains(style, "background:color-mix(in srgb,var(--c) var(--fill,") {
		t.Fatal("style.css .run does not mix --c into the fill in srgb (the blend below assumes it)")
	}
	src, err := jsFiles.ReadFile("js/calendar.js")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`--fill:\$\{Math\.round\((\d+)\+(\d+)\*dens\)\}%`).FindStringSubmatch(string(src))
	if m == nil {
		t.Fatal("calendar.js has no --fill:${Math.round(A+B*dens)}%")
	}
	lo, _ := strconv.Atoi(m[1])
	span, _ := strconv.Atoi(m[2])
	if lo < 20 {
		t.Errorf("the lightest block mixes %d%% of its color, want at least 20%% so Color by shows", lo)
	}
	maxFill := float64(lo+span) / 100
	cols := []string{"other"}
	for k := 0; k < 8; k++ {
		cols = append(cols, "c"+strconv.Itoa(k))
	}
	for k := 0; k < core.AgentColors; k++ {
		cols = append(cols, "a"+strconv.Itoa(k))
	}
	surface := rootVar(t, "surface")
	for _, c := range cols {
		fill := blend(rootVar(t, c), surface, maxFill)
		for _, ink := range []string{"ink", "ink-2"} {
			if r := contrast(rootVar(t, ink), fill); r < 4.5 {
				t.Errorf("--%s on a block of --%s at %.0f%% (%s) is %.2f:1, want at least 4.5:1", ink, c, maxFill*100, fill, r)
			}
		}
	}
}

// 月のカレンダーの濃さ（--heat-c を --heatk まで地に混ぜる）は中立の灰青: エージェントの色と取り違えない（濃さは鮮やかさ 0.05 以下、エージェントの色は 0.10 以上）。
// 凡例の 5 段（0・¼・½・¾・1）は隣どうし ΔE 4 以上で見分けられ、いちばん濃いマス（土日の淡い地を重ねても）の字が 4.5:1 以上。
// 土日の地は土曜も日曜も同じ中立の色で、赤や青の曜日の色は使わない
func TestMonthShadeAndWeekend(t *testing.T) {
	hc := rootVar(t, "heat-c")
	if _, c := lch(hc); c > 0.05 {
		t.Errorf("--heat-c %s has chroma %.3f, want a neutral (at most 0.05)", hc, c)
	}
	for k := 0; k < core.AgentColors; k++ { // エージェントの色は鮮やか（0.10 以上）、濃さは灰色（0.05 以下）なので取り違えない
		if a := rootVar(t, "a"+strconv.Itoa(k)); func() bool { _, c := lch(a); return c < 0.10 }() {
			t.Errorf("--a%d %s is too gray to tell from the month shade", k, a)
		}
	}
	m := regexp.MustCompile(`--heatk:(\d+)%`).FindStringSubmatch(style)
	if m == nil {
		t.Fatal("no --heatk")
	}
	k, _ := strconv.ParseFloat(m[1], 64)
	k /= 100
	for _, sel := range []string{"\n.cell{", "\n.mlegend i{"} {
		i := strings.Index(style, sel)
		if i < 0 || !strings.Contains(style[i:i+strings.Index(style[i:], "}")], "color-mix(in srgb,var(--heat-c)") {
			t.Errorf("%s does not mix --heat-c in srgb", strings.TrimSpace(sel))
		}
	}
	surface := rootVar(t, "surface")
	var steps []string
	for _, v := range []float64{0, .25, .5, .75, 1} {
		steps = append(steps, blend(hc, surface, k*v))
	}
	for i := 0; i+1 < len(steps); i++ {
		if d := deltaE(steps[i], steps[i+1], nil); d < 4 {
			t.Errorf("month shade steps %d and %d (%s, %s) differ by %.1f, want at least 4", i, i+1, steps[i], steps[i+1], d)
		}
	}
	off := regexp.MustCompile(`--off:rgba\((\d+),(\d+),(\d+),([0-9.]+)\)`).FindStringSubmatch(style)
	if off == nil {
		t.Fatal("--off is not rgba(r,g,b,a) in :root")
	}
	var rgb [3]int
	for i := range rgb {
		rgb[i], _ = strconv.Atoi(off[1+i])
	}
	if lo, hi := min(rgb[0], rgb[1], rgb[2]), max(rgb[0], rgb[1], rgb[2]); hi-lo > 12 {
		t.Errorf("--off rgb(%d,%d,%d) is tinted, want a neutral", rgb[0], rgb[1], rgb[2])
	}
	if !strings.Contains(style, "--weekend:var(--off);") {
		t.Error("--weekend is not the same neutral as the off-hours (--off)")
	}
	a, _ := strconv.ParseFloat(off[4], 64)
	top := blend("#"+strconv.FormatInt(int64(rgb[0]<<16|rgb[1]<<8|rgb[2])+0x1000000, 16)[1:], steps[len(steps)-1], a)
	for _, ink := range []string{"ink", "ink-2"} {
		if r := contrast(rootVar(t, ink), top); r < 4.5 {
			t.Errorf("--%s on the darkest weekend day (%s) is %.2f:1, want at least 4.5:1", ink, top, r)
		}
	}
	for _, bad := range []string{"--sat", "--sun", ".sat", ".sun"} {
		if strings.Contains(style, bad) {
			t.Errorf("style.css still has %q (Saturday and Sunday share one neutral weekend look)", bad)
		}
	}
}
