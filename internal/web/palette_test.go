package web

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"
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
	} {
		if r := contrast(c.fg, c.bg); r < 4.5 {
			t.Errorf("%s: %s on %s is %.2f:1, want at least 4.5:1", c.name, c.fg, c.bg, r)
		}
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
