package source

import (
	"path/filepath"
	"testing"
)

// KIRO_HOME を使っていても ~/.kiro の会話も読むので、読む場所（serve --check と Data sources に出る）には両方を並べる。
// ~/.kiro が KIRO_HOME と同じなら 1 つだけ
func TestKiroWhereListsBothHomes(t *testing.T) {
	home, legacy := filepath.Join("/x", "kiro-home"), filepath.Join("/u", ".kiro")
	for _, c := range []struct {
		name string
		got  string
		want string
	}{
		{"IDE both", (&KiroIDE{Home: home, Legacy: legacy}).Where(), filepath.Join(home, "sessions") + ", " + filepath.Join(legacy, "sessions")},
		{"IDE one", (&KiroIDE{Home: legacy, Legacy: legacy}).Where(), filepath.Join(legacy, "sessions")},
		{"CLI both", (&KiroCLI{Home: home, Legacy: legacy}).Where(), filepath.Join(home, "sessions", "cli") + ", " + filepath.Join(legacy, "sessions", "cli")},
		{"CLI one", (&KiroCLI{Home: home}).Where(), filepath.Join(home, "sessions", "cli")},
	} {
		if c.got != c.want {
			t.Errorf("%s: Where() = %q, want %q", c.name, c.got, c.want)
		}
	}
}
