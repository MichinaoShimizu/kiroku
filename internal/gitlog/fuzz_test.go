package gitlog

import (
	"strings"
	"testing"
)

// リモートの URL はリポジトリの設定（他人が書いたものかもしれない）から来る。
// どんな値でも、画面に出すリンクは http(s) だけで、ホストの前に資格情報（user:token@）を残さない。
func FuzzWebURL(f *testing.F) {
	for _, s := range []string{
		"git@github.com:owner/repo.git",
		"https://user:token@gitlab.com/g/sub/repo.git",
		"ssh://git@git.example.com:2222/team/repo.git",
		"https://user@corp:token@github.com/o/r",
		"javascript:alert(1)",
		"/srv/git/repo.git",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, remote string) {
		web := WebURL(remote)
		if web == "" {
			return
		}
		scheme, rest, ok := strings.Cut(web, "://")
		if !ok || (scheme != "https" && scheme != "http") {
			t.Fatalf("WebURL(%q) = %q: not an http(s) link", remote, web)
		}
		if host, _, _ := strings.Cut(rest, "/"); strings.Contains(host, "@") {
			t.Fatalf("WebURL(%q) = %q: credentials left before the host", remote, web)
		}
		for _, u := range []string{commitURL(web, "abc"), fileURL(web, "abc", "a.go")} {
			if u != "" && !strings.HasPrefix(u, web+"/") {
				t.Fatalf("link %q is not under %q", u, web)
			}
		}
	})
}
