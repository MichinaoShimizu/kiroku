package web

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"sync"
)

// kiroku serve の画面を、ブラウザの「アプリをインストール」でアプリの窓として開けるようにするもの。
// マニフェストとアイコンは鍵なしで返すので（ブラウザはヘッダーを付けずに読む）、履歴から作ったものは何も入れない。
// service worker は使わない（履歴をブラウザのキャッシュに残さないため）。

// AppPaths は、マニフェストのパス。kiroku serve はこれだけを鍵なしで返す（GET / のほかに）。
// アイコンはマニフェストに data: URL で入れる。ブラウザはマニフェストのアイコンも画面の CSP の img-src で確かめるので、
// /icon.png のように置くと img-src に 'self' を足すことになる（画面が自分のところから画像を読めるようになる）。
var AppPaths = []string{"/manifest.webmanifest"}

// manifestLink は、kiroku serve の画面の <head> に足すマニフェストへのリンク（保存した HTML には入れない）。
const manifestLink = `<link rel="manifest" href="/manifest.webmanifest">`

// Manifest は /manifest.webmanifest の中身。決まった値だけで、履歴にもその日の設定にもよらない。
// アイコンを描くので、kiroku serve のほかのコマンドの起動を遅くしないよう、はじめて使うときに 1 回だけ作る。
var Manifest = sync.OnceValue(func() []byte {
	b, err := json.Marshal(map[string]any{
		"id":               "/",
		"name":             "kiroku",
		"short_name":       "kiroku",
		"description":      "Your AI work, remembered.",
		"start_url":        "/",
		"scope":            "/",
		"display":          "standalone",
		"background_color": "#15171c",
		"theme_color":      "#15171c",
		"icons": []map[string]string{
			{"src": iconURL(192), "sizes": "192x192", "type": "image/png", "purpose": "any"},
			{"src": iconURL(512), "sizes": "512x512", "type": "image/png", "purpose": "any"},
		},
	})
	if err != nil {
		panic("web: manifest: " + err.Error())
	}
	return b
})

func iconURL(size int) string {
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(drawIcon(size))
}

// iconRect は、favicon の SVG（32×32）の角の丸い四角。
type iconRect struct {
	x, y, w, h, r float64
	c             color.NRGBA
}

var iconRects = []iconRect{
	{0, 0, 32, 32, 8, color.NRGBA{0x15, 0x17, 0x1c, 0xff}},
	{6, 8, 13, 4, 2, color.NRGBA{0x81, 0x8c, 0xf8, 0xff}},
	{11, 14, 15, 4, 2, color.NRGBA{0xff, 0xff, 0xff, 0xff}},
	{6, 20, 9, 4, 2, color.NRGBA{0xff, 0xff, 0xff, 0x8c}}, // opacity .55
}

// drawIcon は iconRects を size×size の PNG に描く（画面の favicon の SVG と同じ絵）。縁は 1 つの点を 4×4 に分けて、どれだけ覆うかで透かす。
func drawIcon(size int) []byte {
	const sub = 4
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	scale := 32 / float64(size)
	for py := 0; py < size; py++ {
		for px := 0; px < size; px++ {
			var r, g, b, a float64 // 重ねた色（アルファを掛けた値）
			for _, q := range iconRects {
				n := 0
				for sy := 0; sy < sub; sy++ {
					for sx := 0; sx < sub; sx++ {
						x := (float64(px) + (float64(sx)+0.5)/sub) * scale
						y := (float64(py) + (float64(sy)+0.5)/sub) * scale
						if q.covers(x, y) {
							n++
						}
					}
				}
				if n == 0 {
					continue
				}
				ca := float64(q.c.A) / 255 * float64(n) / (sub * sub)
				r = float64(q.c.R)*ca + r*(1-ca)
				g = float64(q.c.G)*ca + g*(1-ca)
				b = float64(q.c.B)*ca + b*(1-ca)
				a = ca + a*(1-ca)
			}
			if a > 0 {
				img.SetNRGBA(px, py, color.NRGBA{uint8(r/a + 0.5), uint8(g/a + 0.5), uint8(b/a + 0.5), uint8(a*255 + 0.5)})
			}
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		panic("web: icon: " + err.Error())
	}
	return buf.Bytes()
}

// covers は、点 (x, y) が角の丸い四角の中にあるか。
func (q iconRect) covers(x, y float64) bool {
	if x < q.x || y < q.y || x >= q.x+q.w || y >= q.y+q.h {
		return false
	}
	cx := min(max(x, q.x+q.r), q.x+q.w-q.r)
	cy := min(max(y, q.y+q.r), q.y+q.h-q.r)
	dx, dy := x-cx, y-cy
	return dx*dx+dy*dy <= q.r*q.r
}
