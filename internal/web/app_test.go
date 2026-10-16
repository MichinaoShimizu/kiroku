package web

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

// マニフェストは決まった値だけで、アプリの窓は kiroku serve の / を開く（ADR 0037）。
func TestManifest(t *testing.T) {
	var m struct {
		ID, Name, StartURL, Scope, Display string
		Icons                              []struct{ Src, Sizes, Type string }
	}
	if err := json.Unmarshal(Manifest(), &m); err != nil {
		t.Fatal(err)
	}
	if m.ID != "/" || m.Name != "kiroku" || m.Display != "standalone" {
		t.Errorf("マニフェスト = %+v", m)
	}
	var raw map[string]any
	_ = json.Unmarshal(Manifest(), &raw)
	if raw["start_url"] != "/" || raw["scope"] != "/" {
		t.Errorf("start_url・scope は / だけ（鍵などを入れない）: %v %v", raw["start_url"], raw["scope"])
	}
	if len(m.Icons) != 2 {
		t.Fatalf("アイコン = %+v", m.Icons)
	}
	// アイコンは data: URL（画面の CSP の img-src data: で読める）で、書いてある大きさの PNG
	for _, ic := range m.Icons {
		b64, ok := strings.CutPrefix(ic.Src, "data:image/png;base64,")
		raw, err := base64.StdEncoding.DecodeString(b64)
		if !ok || err != nil || ic.Type != "image/png" {
			t.Fatalf("アイコン %.60q が PNG の data: URL でない", ic.Src)
		}
		checkIcon(t, raw, ic.Sizes)
	}
}

// checkIcon は、アイコンが sizes（"192x192" など）の PNG で、favicon と同じ絵（角は透明・背景は暗い色・上の棒は藍色）か確かめる。
func checkIcon(t *testing.T, raw []byte, sizes string) {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	size := img.Bounds().Dx()
	if fmt.Sprintf("%dx%d", size, img.Bounds().Dy()) != sizes {
		t.Fatalf("大きさ %v（%s のはず）", img.Bounds(), sizes)
	}
	at := func(x, y float64) color.NRGBA {
		return color.NRGBAModel.Convert(img.At(int(x*float64(size)/32), int(y*float64(size)/32))).(color.NRGBA)
	}
	if c := at(0, 0); c.A != 0 {
		t.Errorf("%d: 角が透明でない: %v", size, c)
	}
	if c := at(16, 2); c != (color.NRGBA{0x15, 0x17, 0x1c, 0xff}) {
		t.Errorf("%d: 背景 %v", size, c)
	}
	if c := at(12, 10); c != (color.NRGBA{0x81, 0x8c, 0xf8, 0xff}) {
		t.Errorf("%d: 上の棒 %v", size, c)
	}
}
