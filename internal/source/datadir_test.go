package source

import (
	"path/filepath"
	"testing"
)

// OS ごとの DataDir（dirs::data_local_dir()）。ホームがわからず環境変数でも決まらなければ ""
func TestDataDirPerOS(t *testing.T) {
	for _, c := range []struct {
		goos, home string
		env        map[string]string
		want       string
	}{
		{"linux", "/home/me", nil, filepath.Join("/home/me", ".local", "share", "kiro-cli")},
		{"linux", "", map[string]string{"XDG_DATA_HOME": "/data"}, filepath.Join("/data", "kiro-cli")},
		{"linux", "", map[string]string{"XDG_DATA_HOME": "rel"}, ""},
		{"linux", "/home/me", map[string]string{"XDG_DATA_HOME": "rel/share"}, filepath.Join("/home/me", ".local", "share", "kiro-cli")},
		{"linux", "", nil, ""},
		{"darwin", "/Users/me", nil, filepath.Join("/Users/me", "Library", "Application Support", "kiro-cli")},
		{"darwin", "", nil, ""},
		{"windows", `C:\Users\me`, map[string]string{"LOCALAPPDATA": `D:\local`}, filepath.Join(`D:\local`, "kiro-cli")},
		{"windows", `C:\Users\me`, nil, filepath.Join(`C:\Users\me`, "AppData", "Local", "kiro-cli")},
		{"windows", "", nil, ""},
	} {
		getenv := func(k string) string { return c.env[k] }
		if got := dataDir(c.goos, c.home, getenv, "kiro-cli"); got != c.want {
			t.Errorf("%s home=%q env=%v: %q, want %q", c.goos, c.home, c.env, got, c.want)
		}
	}
}

// HOME が空なら、古い Kiro CLI の data.sqlite3 を今のフォルダから読まない
func TestKiroCLIDBWithoutHome(t *testing.T) {
	for _, k := range []string{"HOME", "XDG_DATA_HOME", "LOCALAPPDATA", "USERPROFILE"} {
		t.Setenv(k, "")
	}
	if d := kiroCLIDB(); d != "" {
		t.Errorf("kiroCLIDB() = %q, want \"\"", d)
	}
}

// Claude Code の組織の設定の場所は OS ごとに決まっている（https://code.claude.com/docs/en/managed-settings ）。ほかの OS では読まない
func TestManagedDirPerOS(t *testing.T) {
	for goos, want := range map[string]string{
		"darwin":  "/Library/Application Support/ClaudeCode",
		"linux":   "/etc/claude-code",
		"windows": `C:\Program Files\ClaudeCode`,
		"freebsd": "",
		"":        "",
	} {
		if got := managedDir(goos); got != want {
			t.Errorf("managedDir(%q) = %q, want %q", goos, got, want)
		}
	}
}
