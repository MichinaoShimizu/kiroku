package web

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// カテゴリーの 8 色（--c0〜--c7）は、明るい地と暗い地のそれぞれで、決めた基準を満たす組み合わせだけを使う。
// プロジェクトは多い順にこの順で色を取るので、隣どうし（積み上げの帯・凡例で並ぶ色）が見分けられることを確かめる:
//   - 明るさ（OKLCH の L）がそろっている（明るい地 0.43〜0.77、暗い地 0.48〜0.67）
//   - 鮮やかさ（C）が 0.10 以上（灰色に見えない）
//   - 隣どうしの OKLab の差（×100）が、ふつうの見え方で 15 以上、1 型・2 型の色覚（Machado 2009）で 6 以上
//
// 色を変えるときは、このテストが通る値を選ぶ（目で選ばない）。
func TestCategoricalPalette(t *testing.T) {
	css := style
	for _, m := range []struct {
		name, sel string
		lo, hi    float64
	}{
		{"light", ":root{", 0.43, 0.77},
		{"dark", `:root[data-theme="dark"]{`, 0.48, 0.67},
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
		for _, c := range pal {
			L, C := lch(c)
			if L < m.lo || L > m.hi {
				t.Errorf("%s: %s の明るさ %.3f が %.2f〜%.2f の外", m.name, c, L, m.lo, m.hi)
			}
			if C < 0.10 {
				t.Errorf("%s: %s の鮮やかさ %.3f が 0.10 未満（灰色に見える）", m.name, c, C)
			}
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
