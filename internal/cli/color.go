package cli

import (
	"io"
	"os"
	"strings"
)

// 出力の色づけ。意味は3つだけに絞る: うまくいっている（緑）、気をつけてほしい（黄）、失敗した（赤）。
// それ以外に、読まなくてもよい補足を dim、打ってほしいコマンドなど大事なところを bold にする。
//
// 色をつけるのは端末に出すときだけ。パイプやファイル、NO_COLOR（https://no-color.org/）、
// TERM=dumb のときは素のまま出す。テストは bytes.Buffer に書くので、こちらも素のままになる。

// style は、出す先に合わせて色をつける（端末でなければ何もしない）。
type style struct{ on bool }

// styleFor は w に合った style を返す。
func styleFor(w io.Writer) style {
	f, ok := w.(*os.File)
	if !ok {
		return style{}
	}
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return style{}
	}
	st, err := f.Stat()
	return style{on: err == nil && st.Mode()&os.ModeCharDevice != 0}
}

func (s style) ok(t string) string   { return s.wrap(t, "32") } // うまくいっている
func (s style) warn(t string) string { return s.wrap(t, "33") } // 気をつけてほしい
func (s style) bad(t string) string  { return s.wrap(t, "31") } // 失敗した
func (s style) dim(t string) string  { return s.wrap(t, "2") }  // 読まなくてもよい補足
func (s style) bold(t string) string { return s.wrap(t, "1") }  // 大事なところ

// wrap は t を code で囲む。囲んだあとも %-18s のような桁合わせができるよう、空の文字列はそのまま返す。
func (s style) wrap(t, code string) string {
	if !s.on || t == "" {
		return t
	}
	return "\x1b[" + code + "m" + t + "\x1b[0m"
}

// cmd は「打ってほしいコマンド」を bold にする。後ろの説明との桁をそろえるため、
// 幅はコマンドの見かけの長さで合わせる（色の文字は幅に数えない）。
func (s style) cmd(name string, width int) string {
	if pad := width - len([]rune(name)); pad > 0 {
		return s.bold(name) + strings.Repeat(" ", pad)
	}
	return s.bold(name)
}

// mark は「✓」「!」「·」に、その意味の色をつける。
func (s style) mark(m string) string {
	switch m {
	case "✓":
		return s.ok(m)
	case "!":
		return s.warn(m)
	case "·":
		return s.dim(m)
	}
	return m
}
