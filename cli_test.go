package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// サブコマンドがなければヘルプだけ（履歴は読まない）。前の書き方でも、何も選ばなければヘルプ。
func TestHelpWithoutSubcommand(t *testing.T) {
	for _, args := range [][]string{nil, {"help"}, {"-h"}, {"--sources", "kiro"}} {
		if err := dispatch(args); err != nil {
			t.Errorf("%v: %v", args, err)
		}
	}
	if err := dispatch([]string{"serv"}); err == nil || !strings.Contains(err.Error(), "知らないサブコマンド") {
		t.Errorf("打ちまちがい: %v", err)
	}
}

// 位置引数はオプションの前にも後ろにも置ける。多すぎればエラー。
func TestParseInterspersed(t *testing.T) {
	for in, want := range map[string]string{
		"2026-09 --md-dir x":          "[2026-09] x",
		"--md-dir x 2026-09":          "[2026-09] x",
		"--md-dir x":                  "[] x",
		"--gap 20 2026-09 --md-dir x": "[2026-09] x",
	} {
		fs := flag.NewFlagSet("t", flag.ContinueOnError)
		dir := fs.String("md-dir", ".", "")
		fs.Int("gap", 15, "")
		pos, err := parse(fs, strings.Fields(in), 1)
		if err != nil {
			t.Fatal(err)
		}
		if got := fmt.Sprint(pos) + " " + *dir; got != want {
			t.Errorf("%q → %s, want %s", in, got, want)
		}
	}
	fs := flag.NewFlagSet("t", flag.ContinueOnError)
	if _, err := parse(fs, []string{"a", "b"}, 1); err == nil {
		t.Error("位置引数が多すぎるのにエラーにならない")
	}
}

// json サブコマンドで集計を書き出せる（-o はオプションの前後どちらでも）。weekly / monthly はもうない。
func TestJSONSubcommand(t *testing.T) {
	setup(t) // タイムゾーンと古い Kiro IDE の更新時刻をそろえる
	h := filepath.Join("testdata", "home")
	out := filepath.Join(t.TempDir(), "k.json")
	args := []string{"json", "--root", filepath.Join(h, ".claude", "projects"), "--kiro-home", filepath.Join(h, ".kiro"), "--sources", "claude,kiro", "-o", out}
	if err := dispatch(args); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(out)
	if err != nil || !strings.Contains(string(b), `"months"`) || !strings.Contains(string(b), `"weeks"`) {
		t.Fatalf("JSON が書き出されていない: %v", err)
	}
	for _, sub := range []string{"weekly", "monthly"} {
		if err := dispatch([]string{sub}); err == nil || !strings.Contains(err.Error(), "知らないサブコマンド") {
			t.Errorf("%s: %v", sub, err)
		}
	}
	if err := dispatch([]string{"--weekly"}); err == nil || !strings.Contains(err.Error(), "なくなりました") {
		t.Errorf("--weekly: %v", err)
	}
}
