package source

import "testing"

// ホームがわからないとき、KIROCREW_HOME の ~ を今のフォルダの「~」として読まない
func TestCrewHomeOverrideTildeWithoutHome(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")
	for _, v := range []string{"~", "~/crew", "~/.kiro/crew"} {
		if got := crewHomeOverride(v); got != "" {
			t.Errorf("crewHomeOverride(%q) = %q without a home folder, want \"\"", v, got)
		}
	}
}
