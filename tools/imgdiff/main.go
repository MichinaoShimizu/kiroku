// imgdiff は、2 つの画像（PNG か GIF）を画素で比べ、違いが小さなゆらぎ（文字のにじみなど）の範囲か、本当に変わったかを答える。
// スクリーンショットを撮り直したあと、変わっていない画像をコミットしないために使う（tools/screenshots/keep-changed.sh）。
//
//	go run ./tools/imgdiff [-tol 24] [-max 0.00002] <前の画像> <新しい画像>
//
// 終了コード: 0 = 同じとみなす、1 = 変わった、2 = 読めない・使い方の誤り。
//
// PNG: 画素は、RGBA のどれかのチャンネルの差が -tol（0〜255）を超えたら「違う」と数え、違う画素の割合が -max 以下なら同じとみなす。
// 同じ時刻（KIROKU_FAKE_NOW）で 2 回撮ると、違いは -tol 0 で数十画素、既定の -tol 24 で 0 画素だった。大きさが違えば変わった。
//
// GIF（docs/demo.gif）: 動画から作るので、撮るたびにコマの数と時間がずれ、圧縮による色のゆれも入る。画素をそのまま比べると
// いつも「変わった」になるので、止まって見える場面どうしを比べる（compare の説明）。-tol と -max は使わない。
package main

import (
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// 読む画像の上限。docs の画像は大きくても数 MB（demo.gif が 5 MB ほど）なので、壊れた・巨大なファイルで止まらないように
const (
	maxFile   = 64 << 20
	maxPixels = 1 << 30 // 全コマの画素の合計（demo.gif は 880x550 の 200 コマほどで 1 億ほど）
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("imgdiff", flag.ContinueOnError)
	fs.SetOutput(stderr)
	tol := fs.Int("tol", 24, "a pixel differs when one of its RGBA channels differs by more than this (0-255)")
	max := fs.Float64("max", 0.00002, "the images count as the same when at most this fraction of pixels differ")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 2 || *tol < 0 || *tol > 255 || *max < 0 || *max >= 1 {
		fmt.Fprintln(stderr, "usage: go run ./tools/imgdiff [-tol 0-255] [-max fraction] <old image> <new image>")
		return 2
	}
	oldPath, newPath := fs.Arg(0), fs.Arg(1)
	a, err := load(oldPath)
	if err != nil {
		fmt.Fprintf(stderr, "imgdiff: %s: %v\n", oldPath, err)
		return 2
	}
	b, err := load(newPath)
	if err != nil {
		fmt.Fprintf(stderr, "imgdiff: %s: %v\n", newPath, err)
		return 2
	}
	d := compare(a, b, uint32(*tol))
	if d.reason != "" {
		fmt.Fprintf(stdout, "%s: changed (%s)\n", newPath, d.reason)
		return 1
	}
	frac := float64(d.diff) / float64(d.total)
	if d.scenes {
		fmt.Fprintf(stdout, "%s: same (every still scene matches; %.4f%% of their reduced pixels differ)\n", newPath, frac*100)
		return 0
	}
	verdict := "same"
	code := 0
	if frac > *max {
		verdict = "changed"
		code = 1
	}
	fmt.Fprintf(stdout, "%s: %s (%d of %d pixels differ, %.4f%%)\n", newPath, verdict, d.diff, d.total, frac*100)
	return code
}

// picture は、比べる絵。PNG は 1 枚の絵、GIF は読んだままのコマ（重ねて描くのは比べるときに 1 コマずつ。全コマの絵を同時に持つとメモリが足りない）
type picture struct {
	still *image.RGBA
	anim  *gif.GIF
}

func load(path string) (*picture, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() {
		return nil, errors.New("not a regular file")
	}
	if st.Size() > maxFile {
		return nil, fmt.Errorf("larger than %d MB", maxFile>>20)
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png":
		cfg, err := png.DecodeConfig(io.LimitReader(f, maxFile))
		if err != nil {
			return nil, err
		}
		if int64(cfg.Width)*int64(cfg.Height) > maxPixels {
			return nil, errors.New("too many pixels")
		}
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return nil, err
		}
		img, err := png.Decode(io.LimitReader(f, maxFile))
		if err != nil {
			return nil, err
		}
		return &picture{still: toRGBA(img)}, nil
	case ".gif":
		g, err := gif.DecodeAll(io.LimitReader(f, maxFile))
		if err != nil {
			return nil, err
		}
		if len(g.Image) == 0 {
			return nil, errors.New("no frames")
		}
		if g.Config.Width < block || g.Config.Height < block {
			return nil, errors.New("too small")
		}
		if int64(g.Config.Width)*int64(g.Config.Height)*int64(len(g.Image)) > maxPixels {
			return nil, errors.New("too many pixels")
		}
		return &picture{anim: g}, nil
	}
	return nil, errors.New("only .png and .gif are supported")
}

func toRGBA(img image.Image) *image.RGBA {
	b := img.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(out, out.Bounds(), img, b.Min, draw.Src)
	return out
}

// player は GIF のコマを順に重ね、各コマを表示したときに見える絵を作る（コマの消し方 disposal も守る）
type player struct {
	g      *gif.GIF
	i      int
	canvas *image.RGBA
	saved  *image.RGBA
}

func newPlayer(g *gif.GIF) *player {
	return &player{g: g, canvas: image.NewRGBA(image.Rect(0, 0, g.Config.Width, g.Config.Height))}
}

// next は次のコマを描いて、見える絵を返す。返した絵は次の呼び出しで書き換わる
func (p *player) next() *image.RGBA {
	if p.i > 0 { // 前のコマの消し方
		prev := p.g.Image[p.i-1]
		switch p.disposal(p.i - 1) {
		case gif.DisposalBackground:
			draw.Draw(p.canvas, prev.Bounds(), image.NewUniform(color.Transparent), image.Point{}, draw.Src)
		case gif.DisposalPrevious:
			copy(p.canvas.Pix, p.saved.Pix)
		}
	}
	fr := p.g.Image[p.i]
	if p.disposal(p.i) == gif.DisposalPrevious {
		if p.saved == nil {
			p.saved = image.NewRGBA(p.canvas.Bounds())
		}
		copy(p.saved.Pix, p.canvas.Pix)
	}
	draw.Draw(p.canvas, fr.Bounds(), fr, fr.Bounds().Min, draw.Over)
	p.i++
	return p.canvas
}

func (p *player) disposal(i int) byte {
	if i < len(p.g.Disposal) {
		return p.g.Disposal[i]
	}
	return 0
}

type result struct {
	diff, total int64
	reason      string // 画素を比べるまでもなく違うとき、その理由
	scenes      bool   // GIF：止まって見える場面がすべて合った（diff と total は縮めた場面の数）
}

func compare(a, b *picture, tol uint32) result {
	if (a.anim == nil) != (b.anim == nil) {
		return result{reason: "a different image format"}
	}
	if a.still != nil {
		return compareFrames(nil, a.still, b.still, tol)
	}
	ga, gb := a.anim, b.anim
	if ga.Config.Width != gb.Config.Width || ga.Config.Height != gb.Config.Height {
		return result{reason: fmt.Sprintf("%dx%d, was %dx%d", gb.Config.Width, gb.Config.Height, ga.Config.Width, ga.Config.Height)}
	}
	// 動画から作る GIF は、撮るたびにコマの数と時間が少しずれ（録画のタイミング）、動画の圧縮で細かな色のゆれも入る。
	// そこで、各コマを縮めて（blockxblock の平均の明るさ）ゆれを消し、しばらく止まって見える場面（stableFor 以上）が、
	// もう一方の同じころ（前後 window）に同じように写っているかを、両方向に確かめる。動いている途中のコマどうしは比べない
	fa, fb := reduce(ga), reduce(gb)
	if d := fa[len(fa)-1].end - fb[len(fb)-1].end; d > window || -d > window {
		return result{reason: fmt.Sprintf("%.1f s long, was %.1f s", float64(fb[len(fb)-1].end)/100, float64(fa[len(fa)-1].end)/100)}
	}
	r := result{scenes: true}
	for _, dir := range [2][2][]frame{{fa, fb}, {fb, fa}} {
		scenes := stills(dir[0])
		if len(scenes) == 0 {
			return result{reason: "no still scenes"}
		}
		for i, sc := range scenes {
			best := int64(-1)
			for _, f := range dir[1] {
				if f.end < sc.start-window || f.start > sc.end+window {
					continue
				}
				if n := diffGray(sc.img, f.img); best < 0 || n < best {
					best = n
				}
			}
			n := int64(len(sc.img))
			if best < 0 || float64(best)/float64(n) > maxScene {
				return result{reason: fmt.Sprintf("still scene %d of %d (at %.1f s) is not in the other GIF", i+1, len(scenes), float64(sc.start)/100)}
			}
			r.diff += best
			r.total += n
		}
	}
	return r
}

const (
	block     = 4     // 縮める大きさ（block x block 画素を 1 つに）
	grayTol   = 12    // 縮めた明るさ（0〜255）の差がこれを超えたら違うと数える
	stableFor = 50    // 止まって見える場面とみなす長さ（1/100 秒）
	window    = 150   // 同じ場面を探す前後の幅（1/100 秒）。録画のずれはふつう 0.5 秒ほど
	maxScene  = 0.002 // 縮めた場面で、違ってよい割合（動画の圧縮のゆれ）
)

// frame は縮めた 1 コマと、それが見えている時間（GIF の頭からの 1/100 秒）
type frame struct {
	start, end int
	img        []uint8
}

func reduce(g *gif.GIF) []frame {
	w, h := g.Config.Width/block, g.Config.Height/block
	p := newPlayer(g)
	t := 0
	out := make([]frame, 0, len(g.Image))
	for i := range g.Image {
		fr := p.next()
		img := make([]uint8, w*h)
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				sum := 0
				for dy := 0; dy < block; dy++ {
					o := fr.PixOffset(x*block, y*block+dy)
					for dx := 0; dx < block; dx++ {
						px := fr.Pix[o+dx*4 : o+dx*4+3]
						sum += (299*int(px[0]) + 587*int(px[1]) + 114*int(px[2])) / 1000
					}
				}
				img[y*w+x] = uint8(sum / (block * block))
			}
		}
		d := 0
		if i < len(g.Delay) {
			d = g.Delay[i]
		}
		out = append(out, frame{start: t, end: t + d, img: img})
		t += d
	}
	return out
}

func diffGray(a, b []uint8) int64 {
	if len(a) != len(b) {
		return int64(len(a))
	}
	var n int64
	for i := range a {
		if d := int(a[i]) - int(b[i]); d > grayTol || -d > grayTol {
			n++
		}
	}
	return n
}

// stills は、続くコマがほとんど同じまま stableFor 以上見えている場面（の最初のコマと、見えている時間）を返す
func stills(fs []frame) []frame {
	var out []frame
	cur := -1
	flush := func(end int) {
		if cur >= 0 && end-fs[cur].start >= stableFor {
			out = append(out, frame{start: fs[cur].start, end: end, img: fs[cur].img})
		}
	}
	for i, f := range fs {
		if cur >= 0 && float64(diffGray(fs[cur].img, f.img))/float64(len(f.img)) <= maxScene/4 {
			continue
		}
		if i > 0 {
			flush(fs[i-1].end)
		}
		cur = i
	}
	flush(fs[len(fs)-1].end)
	return out
}

// compareFrames は 2 枚の絵の違う画素を数え、acc（nil なら 0 から）に足す
func compareFrames(acc *result, fa, fb *image.RGBA, tol uint32) result {
	var r result
	if acc != nil {
		r = *acc
	}
	if fa.Bounds().Size() != fb.Bounds().Size() {
		return result{reason: fmt.Sprintf("%dx%d, was %dx%d", fb.Bounds().Dx(), fb.Bounds().Dy(), fa.Bounds().Dx(), fa.Bounds().Dy())}
	}
	for j := 0; j+3 < len(fa.Pix); j += 4 {
		r.total++
		for k := 0; k < 4; k++ {
			x, y := uint32(fa.Pix[j+k]), uint32(fb.Pix[j+k])
			if x > y+tol || y > x+tol {
				r.diff++
				break
			}
		}
	}
	if r.total == 0 {
		return result{reason: "no pixels"}
	}
	return r
}
