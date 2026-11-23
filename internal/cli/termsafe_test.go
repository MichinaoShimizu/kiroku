package cli

import (
	"strings"
	"testing"
)

// 履歴から来た文字（ファイルの場所など）を端末に出すときは、制御文字・向きを変える文字・改行を取り除き、長いものは切る。
func TestTermSafe(t *testing.T) {
	if got := termSafe("a\x1b[31mred\x1b[0m", 20); strings.ContainsRune(got, 0x1b) {
		t.Errorf("ESC が残った: %q", got)
	}
	if got := termSafe("a\u202egnp\u200b.exe\u2028\r\n", 20); got != "agnp.exe" {
		t.Errorf("向きを変える文字と改行: %q", got)
	}
	if got := termSafe(strings.Repeat("a", 40), 10); got != "aaaaaaaaa…" {
		t.Errorf("長い名前を切る: %q", got)
	}
	if got := termSafe("\x1b", 10); got != "—" {
		t.Errorf("制御文字だけの名前: %q", got)
	}
	if got := termMsg("open /x/\x1b]0;evil\x07file: denied"); strings.ContainsAny(got, "\x1b\x07") {
		t.Errorf("termMsg に制御文字が残った: %q", got)
	}
}

// 1.x の kiroku json と kiroku stats は 3.0 でなくした。使うと、代わり（kiroku serve・kiroku serve --html）を教えてエラーで終わる
func TestRemovedJSONAndStats(t *testing.T) {
	for _, c := range []string{"json", "stats"} {
		err := dispatch([]string{c})
		if err == nil || !strings.Contains(err.Error(), "was removed") || !strings.Contains(err.Error(), "kiroku serve") {
			t.Errorf("%s: err = %v", c, err)
		}
	}
	if err := dispatch([]string{"--json"}); err == nil || !strings.Contains(err.Error(), "kiroku serve --html") {
		t.Errorf("--json: err = %v", err)
	}
}
