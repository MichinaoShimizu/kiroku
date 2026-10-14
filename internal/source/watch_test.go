package source

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Fingerprint は、読む側が辿るシンボリックリンクの先（場所そのもの・ファイル・フォルダ）が変わっても変わる。
// リンクがループしていても止まり、もう見た場所を指すリンクは 2 度見ない。
func TestFingerprintFollowsSymlinks(t *testing.T) {
	dir := t.TempDir()
	home, other := filepath.Join(dir, "home"), filepath.Join(dir, "other")
	writeFiles(t, home, map[string]string{"sessions/a.jsonl": "a\n"})
	writeFiles(t, other, map[string]string{"map.json": "{}", "sub/state.json": "{}"})
	link := filepath.Join(dir, "home-link")
	for _, l := range [][2]string{
		{home, link},
		{filepath.Join(other, "map.json"), filepath.Join(home, "session_map.json")},
		{filepath.Join(other, "sub"), filepath.Join(home, "subagents", "a1")},
		{home, filepath.Join(home, "loop")}, // 自分を指す
	} {
		if err := os.MkdirAll(filepath.Dir(l[1]), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(l[0], l[1]); err != nil {
			t.Fatal(err)
		}
	}
	later := time.Now().Add(time.Hour)
	for _, f := range []string{"map.json", "sub/state.json"} {
		before := Fingerprint([]string{link})
		p := filepath.Join(other, f)
		if err := os.WriteFile(p, []byte(`{"flags": {"incognito": true}}`), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, later, later); err != nil {
			t.Fatal(err)
		}
		if Fingerprint([]string{link}) == before {
			t.Errorf("リンクの先の %s が変わったのに、指紋が変わらない", f)
		}
	}
	if Fingerprint([]string{link}) != Fingerprint([]string{home}) {
		t.Error("リンクとその先で指紋が違う")
	}
}
