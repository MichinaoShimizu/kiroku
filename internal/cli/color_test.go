package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// 色をつけるのは端末に出すときだけ。バッファやふつうのファイル、NO_COLOR・TERM=dumb のときは素のまま。
func TestStyleFor(t *testing.T) {
	if styleFor(&bytes.Buffer{}).on {
		t.Error("バッファに色をつけている")
	}
	f, err := os.Create(filepath.Join(t.TempDir(), "out"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if styleFor(f).on {
		t.Error("ふつうのファイルに色をつけている")
	}
	if runtime.GOOS == "windows" {
		return // NUL が文字デバイスとして見えるかは決まっていないので、ここまで
	}
	null, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer null.Close()
	if !styleFor(null).on {
		t.Error("端末（文字デバイス）に色がつかない")
	}
	for _, env := range []string{"NO_COLOR", "TERM"} {
		v := map[string]string{"NO_COLOR": "1", "TERM": "dumb"}[env]
		t.Setenv(env, v)
		if styleFor(null).on {
			t.Errorf("%s=%s でも色をつけている", env, v)
		}
		t.Setenv(env, "")
	}
}

// 色づけは、素のときは何も足さず、つけるときは中身をそのまま残す。
func TestStyleWrap(t *testing.T) {
	off, on := style{}, style{on: true}
	if got := off.ok("✓") + off.warn("!") + off.dim("·") + off.bad("x") + off.bold("b"); got != "✓!·xb" {
		t.Errorf("素のままにならない: %q", got)
	}
	for _, got := range []string{on.ok("✓"), on.warn("!"), on.dim("·"), on.bad("x"), on.bold("b")} {
		if !strings.HasPrefix(got, "\x1b[") || !strings.HasSuffix(got, "\x1b[0m") {
			t.Errorf("色がついていない: %q", got)
		}
	}
	if on.wrap("", "32") != "" {
		t.Error("空の文字列に色をつけている")
	}
	// 「✓」「!」「·」は、その意味の色になる（それ以外はそのまま）
	if on.mark("✓") != on.ok("✓") || on.mark("!") != on.warn("!") || on.mark("·") != on.dim("·") || on.mark("?") != "?" {
		t.Error("mark が意味どおりの色になっていない")
	}
}

// 打ってほしいコマンドは bold にしつつ、後ろの説明の桁をそろえる（色の文字は幅に数えない）。
func TestStyleCmd(t *testing.T) {
	if got := (style{}).cmd("kiroku serve", 20); got != "kiroku serve        " {
		t.Errorf("桁がそろっていない: %q", got)
	}
	on := style{on: true}
	got := on.cmd("kiroku serve", 20)
	if !strings.Contains(got, "kiroku serve") || !strings.HasSuffix(got, strings.Repeat(" ", 8)) {
		t.Errorf("色つきで桁がそろっていない: %q", got)
	}
	if got := on.cmd("kiroku archive on", 5); got != on.bold("kiroku archive on") {
		t.Errorf("幅より長いときに余計なものが付いている: %q", got)
	}
}
