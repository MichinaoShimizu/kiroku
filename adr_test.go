package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// ADR の番号は 1 つに 1 つ。同じ番号が 2 つあると「ADR 0035」がどちらを指すかわからなくなる（あとから入った方の番号を付け直す）。
// ファイル名の番号と、1 行目の見出し（# ADR NNNN:）の番号も同じにする。
func TestADRNumbersAreUnique(t *testing.T) {
	entries, err := os.ReadDir("docs/adr")
	if err != nil {
		t.Fatal(err)
	}
	name := regexp.MustCompile(`^(\d{4})-.+\.md$`)
	title := regexp.MustCompile(`^# ADR (\d{4}):`)
	seen := map[string]string{}
	for _, e := range entries {
		m := name.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		if prev, ok := seen[m[1]]; ok {
			t.Errorf("ADR %s is used by both %s and %s", m[1], prev, e.Name())
		}
		seen[m[1]] = e.Name()
		b, err := os.ReadFile("docs/adr/" + e.Name())
		if err != nil {
			t.Fatal(err)
		}
		first, _, _ := strings.Cut(string(b), "\n")
		if h := title.FindStringSubmatch(first); h == nil || h[1] != m[1] {
			t.Errorf("%s: the first line should start with \"# ADR %s:\", got %q", e.Name(), m[1], first)
		}
	}
	if len(seen) == 0 {
		t.Fatal("no ADRs found in docs/adr")
	}
}
