package main

import (
	"bytes"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// TestRepo は、このリポジトリそのものに問題がないこと（allow と baseline が今のコードと合っていること）を確かめる
func TestRepo(t *testing.T) {
	var out bytes.Buffer
	if code := run("../..", allow, baseline, &out); code != 0 {
		t.Fatalf("seclint found problems in the repository (exit %d):\n%s", code, out.String())
	}
}

// TestRules は、各決まりが testdata/repo の作り物に当たることを確かめる（「bad」の行だけ、1 度ずつ）
func TestRules(t *testing.T) {
	fs, err := scan("testdata/repo")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, f := range fs {
		got = append(got, fmt.Sprintf("%s:%d %s", f.File, f.Line, f.Rule))
	}
	sort.Strings(got)
	want := []string{
		".github/workflows/ci.yml:10 unpinned-action", // 38 文字の SHA
		".github/workflows/ci.yml:11 unpinned-action", // docker のタグ
		".github/workflows/ci.yml:7 unpinned-action",  // @v4
		".github/workflows/ci.yml:9 unpinned-action",  // @main
		"cmd/fixture.go:13 shell-exec",
		"cmd/fixture.go:14 shell-exec",
		"cmd/fixture.go:15 shell-exec",
		"cmd/fixture.go:16 shell-exec",
		"cmd/fixture.go:22 file-mode",
		"cmd/fixture.go:24 file-mode",
		"cmd/fixture.go:26 file-mode",
		"cmd/fixture.go:28 file-mode",
		"cmd/fixture.go:29 file-mode",
		"cmd/fixture.go:34 listen",
		"cmd/fixture.go:35 listen",
		"cmd/fixture.go:36 listen",
		"cmd/fixture.go:40 listen",
		"cmd/fixture.go:41 listen",
		"cmd/fixture_test.go:12 shell-exec",
		"internal/web/js/view.js:10 js-html",
		"internal/web/js/view.js:11 js-html",
		"internal/web/js/view.js:2 js-html",
		"internal/web/js/view.js:3 js-html",
		"internal/web/js/view.js:4 js-html",
		"internal/web/js/view.js:5 js-html",
		"internal/web/js/view.js:6 js-html",
	}
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("findings:\n got  %q\n want %q", got, want)
	}
	// 何も認めなければ失敗する
	var out bytes.Buffer
	if code := run("testdata/repo", nil, nil, &out); code != 1 || !strings.Contains(out.String(), "26 problem(s)") {
		t.Errorf("run without exceptions: exit %d\n%s", code, out.String())
	}
}

// TestExceptions は、allow と baseline の当て方と、古くなった項目を失敗にすることを確かめる
func TestExceptions(t *testing.T) {
	fs, err := scan("testdata/repo")
	if err != nil {
		t.Fatal(err)
	}
	const js = "internal/web/js/view.js"
	allowAll := []exception{
		{Rule: "unpinned-action", File: ".github/workflows/ci.yml", Match: "x"}, // 当たらない（Match は js-html の行だけに使う）
		{Rule: "shell-exec", File: "cmd/fixture.go", Func: "shells"},
		{Rule: "shell-exec", File: "cmd/fixture_test.go", Func: "helpers"},
		{Rule: "file-mode", File: "cmd/fixture.go", Func: "modes"},
		{Rule: "listen", File: "cmd/fixture.go", Func: "listens"},
		{Rule: "js-html", File: js, Match: "document.write(s)"},
	}
	problems := apply(fs, allowAll, []exception{{Rule: "js-html", File: js, Count: 6}})
	// 残るのは workflow の 4 つと、当たらない allow の項目だけ
	var left []string
	for _, p := range problems {
		if !strings.Contains(p, "unpinned-action") {
			left = append(left, p)
		}
	}
	if len(left) != 0 || len(problems) != 5 || !strings.Contains(strings.Join(problems, "\n"), "matches nothing; remove it") {
		t.Errorf("with exceptions:\n%s", strings.Join(problems, "\n"))
	}
	// baseline より多い・少ない
	more := apply(fs, allowAll[1:], []exception{{Rule: "js-html", File: js, Count: 5}})
	if !containsAll(more, "6 HTML insertions, the baseline allows 5") {
		t.Errorf("above the baseline:\n%s", strings.Join(more, "\n"))
	}
	fewer := apply(fs, allowAll[1:], []exception{{Rule: "js-html", File: js, Count: 9}})
	if !containsAll(fewer, "fewer than the baseline of 9; lower Count") {
		t.Errorf("below the baseline:\n%s", strings.Join(fewer, "\n"))
	}
}

func containsAll(lines []string, want ...string) bool {
	all := strings.Join(lines, "\n")
	for _, w := range want {
		if !strings.Contains(all, w) {
			return false
		}
	}
	return true
}
