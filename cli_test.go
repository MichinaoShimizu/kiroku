package main

import (
	"flag"
	"fmt"
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

// サブコマンドで週次・月次サマリーを書き出せる（日付は後ろに置いても読める）。
func TestWeeklyMonthlySubcommands(t *testing.T) {
	setup(t) // タイムゾーンと古い Kiro IDE の更新時刻をそろえる
	h := filepath.Join("testdata", "home")
	dir := t.TempDir()
	common := []string{"--root", filepath.Join(h, ".claude", "projects"), "--kiro-home", filepath.Join(h, ".kiro"), "--md-dir", dir, "--sources", "claude,kiro"}
	if err := dispatch(append([]string{"weekly"}, append(common, "2026-09-30")...)); err != nil {
		t.Fatal(err)
	}
	if err := dispatch(append([]string{"monthly", "2026-09"}, common...)); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"kiroku-week-2026-09-28.md", "kiroku-month-2026-09.md"} {
		if m, _ := filepath.Glob(filepath.Join(dir, f)); len(m) != 1 {
			t.Errorf("%s が書き出されていない", f)
		}
	}
}
