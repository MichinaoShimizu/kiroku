package archive

import (
	"path/filepath"
	"testing"
)

// OS ごとの保存場所。ホームがわからず環境変数でも決まらなければ ""（今のフォルダからの相対パスにしない）
func TestDefaultDirPerOS(t *testing.T) {
	for _, c := range []struct {
		goos, home string
		env        map[string]string
		want       string
	}{
		{"linux", "/home/me", nil, filepath.Join("/home/me", ".local", "share", "kiroku", "archive")},
		{"linux", "/home/me", map[string]string{"XDG_DATA_HOME": "/data"}, filepath.Join("/data", "kiroku", "archive")},
		{"linux", "", map[string]string{"XDG_DATA_HOME": "/data"}, filepath.Join("/data", "kiroku", "archive")},
		{"linux", "", map[string]string{"XDG_DATA_HOME": "rel"}, ""},
		{"linux", "/home/me", map[string]string{"XDG_DATA_HOME": "rel/share"}, filepath.Join("/home/me", ".local", "share", "kiroku", "archive")},
		{"linux", "", nil, ""},
		{"darwin", "/Users/me", nil, filepath.Join("/Users/me", "Library", "Application Support", "kiroku", "archive")},
		{"darwin", "", nil, ""},
		{"windows", `C:\Users\me`, map[string]string{"LocalAppData": `C:\Users\me\AppData\Local`}, filepath.Join(`C:\Users\me\AppData\Local`, "kiroku", "archive")},
		{"windows", `C:\Users\me`, nil, filepath.Join(`C:\Users\me`, "AppData", "Local", "kiroku", "archive")},
		{"windows", "", nil, ""},
		{"linux", "", map[string]string{"KIROKU_ARCHIVE_DIR": "/keep"}, "/keep"},
	} {
		getenv := func(k string) string { return c.env[k] }
		if got := defaultDir(c.goos, c.home, getenv); got != c.want {
			t.Errorf("%s home=%q env=%v: %q, want %q", c.goos, c.home, c.env, got, c.want)
		}
	}
}

// HOME が空なら、保存場所は今のフォルダの下（.local/share/kiroku/archive）ではなく ""
func TestDefaultDirWithoutHome(t *testing.T) {
	for _, k := range []string{"HOME", "XDG_DATA_HOME", "KIROKU_ARCHIVE_DIR", "LocalAppData", "USERPROFILE"} {
		t.Setenv(k, "")
	}
	if d := DefaultDir(); d != "" {
		t.Errorf("DefaultDir() = %q, want \"\"", d)
	}
}
