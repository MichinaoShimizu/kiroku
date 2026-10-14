package main

import "testing"

func TestNpmVersion(t *testing.T) {
	if v, err := npmVersion([]byte(`{"name":"x","version":"2.1.299"}`)); err != nil || v != "2.1.299" {
		t.Fatalf("got %q, %v", v, err)
	}
	// ネットワークからの文字は、版の形でなければ書かない（表やリンクに紛れ込ませない）
	for _, bad := range []string{`{"version":"2.1.0 | [x](https://e.x)"}`, `{"version":"latest"}`, `{"version":""}`, `not json`} {
		if _, err := npmVersion([]byte(bad)); err == nil {
			t.Errorf("%s: want an error", bad)
		}
	}
}

func TestKiroLatest(t *testing.T) {
	page := []byte(`<a href="/changelog/ide/1-3/">x</a><a href="/changelog/cli/2-31/">a</a><a href="/changelog/cli/2-9/">b</a>` +
		`<a href="/changelog/cli/2-29#patch-2-29-1">c</a><a href="/changelog/web/other/">d</a>`)
	if v, err := kiroLatest(page, "cli"); err != nil || v != "2.31" {
		t.Errorf("cli = %q, %v", v, err)
	}
	if v, err := kiroLatest(page, "ide"); err != nil || v != "1.3" {
		t.Errorf("ide = %q, %v", v, err)
	}
	if _, err := kiroLatest([]byte(`<html>changed</html>`), "cli"); err == nil {
		t.Error("want an error when the page has no entries")
	}
}
