package main

import (
	"bytes"
	"image"
	"image/color"
	"image/color/palette"
	"image/draw"
	"image/gif"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writePNG(t *testing.T, path string, img image.Image) {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
}

func plain(w, h int, c color.Color) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), image.NewUniform(c), image.Point{}, draw.Src)
	return img
}

func runDiff(t *testing.T, args ...string) (int, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := run(args, &out, &errOut)
	return code, out.String() + errOut.String()
}

func TestPNG(t *testing.T) {
	dir := t.TempDir()
	base := plain(200, 100, color.White)
	writePNG(t, filepath.Join(dir, "old.png"), base)

	// にじみ程度のゆれ（-tol 以下の差）は同じ
	noisy := plain(200, 100, color.White)
	for x := 0; x < 50; x++ {
		noisy.Set(x, 10, color.RGBA{240, 240, 240, 255})
	}
	writePNG(t, filepath.Join(dir, "noisy.png"), noisy)

	// はっきり違う画素が -max より少ない（20000 画素の 0.002% は 0.4 画素なので、1 画素でも変わったになる）
	one := plain(200, 100, color.White)
	one.Set(5, 5, color.Black)
	writePNG(t, filepath.Join(dir, "one.png"), one)

	// 文字が 1 つ変わったくらいの違い
	text := plain(200, 100, color.White)
	draw.Draw(text, image.Rect(20, 20, 30, 34), image.NewUniform(color.Black), image.Point{}, draw.Src)
	writePNG(t, filepath.Join(dir, "text.png"), text)

	writePNG(t, filepath.Join(dir, "size.png"), plain(201, 100, color.White))
	for _, name := range []string{"broken.png", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("not a png"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	old := filepath.Join(dir, "old.png")
	for _, c := range []struct {
		args []string
		code int
		want string
	}{
		{[]string{old, old}, 0, "same (0 of 20000"},
		{[]string{old, filepath.Join(dir, "noisy.png")}, 0, "same (0 of 20000"},
		{[]string{"-tol", "0", old, filepath.Join(dir, "noisy.png")}, 1, "changed (50 of 20000"},
		{[]string{old, filepath.Join(dir, "one.png")}, 1, "changed (1 of 20000"},
		{[]string{"-max", "0.001", old, filepath.Join(dir, "one.png")}, 0, "same (1 of 20000"},
		{[]string{old, filepath.Join(dir, "text.png")}, 1, "changed (140 of 20000"},
		{[]string{old, filepath.Join(dir, "size.png")}, 1, "changed (201x100, was 200x100)"},
		{[]string{old, filepath.Join(dir, "broken.png")}, 2, "broken.png"},
		{[]string{old, filepath.Join(dir, "missing.png")}, 2, "missing.png"},
		{[]string{old, dir}, 2, "not a regular file"},
		{[]string{old, filepath.Join(dir, "notes.txt")}, 2, "only .png and .gif"},
		{[]string{old}, 2, "usage"},
		{[]string{"-tol", "300", old, old}, 2, "usage"},
	} {
		code, out := runDiff(t, c.args...)
		if code != c.code || !strings.Contains(out, c.want) {
			t.Errorf("%v: got %d %q, want %d and %q", c.args, code, out, c.code, c.want)
		}
	}
}

// scene は 1 つの場面（w x h の地の色に、四角を 1 つ置いた絵）と、それを見せる時間（1/100 秒）のコマの並び
type scene struct {
	bg     color.Color
	box    image.Rectangle
	delays []int
}

// writeGIF は場面を順にコマにする。各場面のあいだに、動いている途中のコマ（毎回違う絵）を挟む
func writeGIF(t *testing.T, path string, w, h int, moving int, scenes ...scene) {
	t.Helper()
	g := &gif.GIF{Config: image.Config{Width: w, Height: h, ColorModel: color.Palette(palette.Plan9)}}
	add := func(img image.Image, d int) {
		p := image.NewPaletted(image.Rect(0, 0, w, h), palette.Plan9)
		draw.Draw(p, p.Bounds(), img, image.Point{}, draw.Src)
		g.Image = append(g.Image, p)
		g.Delay = append(g.Delay, d)
	}
	for i, s := range scenes {
		img := plain(w, h, s.bg)
		draw.Draw(img, s.box, image.NewUniform(color.Black), image.Point{}, draw.Src)
		for _, d := range s.delays {
			add(img, d)
		}
		for j := 0; j < moving; j++ { // 動いている途中：点が場面ごと・コマごとに違う場所にある
			m := plain(w, h, s.bg)
			draw.Draw(m, image.Rect(i*7+j*13%w, j*5%h, i*7+j*13%w+8, j*5%h+8), image.NewUniform(color.RGBA{200, 0, 0, 255}), image.Point{}, draw.Src)
			add(m, 10)
		}
	}
	var b bytes.Buffer
	if err := gif.EncodeAll(&b, g); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestGIF(t *testing.T) {
	dir := t.TempDir()
	white, gray := color.White, color.RGBA{220, 220, 220, 255}
	a := scene{white, image.Rect(10, 10, 40, 30), []int{10, 10, 10, 10, 10, 10, 10, 10}}
	b := scene{gray, image.Rect(50, 20, 90, 60), []int{10, 10, 10, 10, 10, 10, 10, 10, 10, 10}}
	old := filepath.Join(dir, "old.gif")
	writeGIF(t, old, 120, 80, 3, a, b)

	// 録画のずれ：コマの数と時間が違い、動いている途中のコマも違うが、止まって見える場面は同じ
	jitter := a
	jitter.delays = []int{20, 10, 10, 10, 10, 10, 10, 10, 10}
	b2 := b
	b2.delays = []int{30, 30, 30}
	writeGIF(t, filepath.Join(dir, "jitter.gif"), 120, 80, 5, jitter, b2)

	// 2 つ目の場面の中身が変わった
	moved := b
	moved.box = image.Rect(50, 20, 110, 70)
	writeGIF(t, filepath.Join(dir, "moved.gif"), 120, 80, 3, a, moved)

	// 場面が 1 つ増えた（長さも延びた）
	c := scene{white, image.Rect(0, 0, 60, 40), []int{100}}
	writeGIF(t, filepath.Join(dir, "more.gif"), 120, 80, 3, a, b, c)

	// 長さが window より大きく違う
	long := c
	long.delays = []int{500, 500}
	writeGIF(t, filepath.Join(dir, "long.gif"), 120, 80, 3, a, b, long)

	writeGIF(t, filepath.Join(dir, "size.gif"), 124, 80, 3, a, b)
	writePNG(t, filepath.Join(dir, "still.png"), plain(120, 80, white))

	for _, c := range []struct {
		new  string
		code int
		want string
	}{
		{"old.gif", 0, "same (every still scene matches"},
		{"jitter.gif", 0, "same (every still scene matches"},
		{"moved.gif", 1, "changed (still scene 2 of 2"},
		{"more.gif", 1, "changed (still scene 3 of 3 (at 2.4 s) is not in the other GIF)"},
		{"long.gif", 1, "changed (12.7 s long, was 2.4 s)"},
		{"size.gif", 1, "changed (124x80, was 120x80)"},
		{"still.png", 1, "a different image format"},
	} {
		code, out := runDiff(t, old, filepath.Join(dir, c.new))
		if code != c.code || !strings.Contains(out, c.want) {
			t.Errorf("%s: got %d %q, want %d and %q", c.new, code, out, c.code, c.want)
		}
	}
}
