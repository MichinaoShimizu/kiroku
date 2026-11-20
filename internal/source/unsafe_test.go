package source

import (
	"slices"
	"strings"
	"testing"

	"github.com/MichinaoShimizu/kiroku/internal/core"
)

func TestSafeName(t *testing.T) {
	for name, want := range map[string]bool{
		"ok": true, "a-b_c.1": true, "セッション": true, "..x": true,
		"": false, ".": false, "..": false, "../x": false, "a/b": false, `a\b`: false, `..\x`: false,
		"C:": false, "C:x": false, "a\x00b": false, "a\nb": false, "/abs": false, `\\server\share`: false,
	} {
		if got := safeName(name); got != want {
			t.Errorf("safeName(%q) = %v, want %v", name, got, want)
		}
	}
}

// Codex の履歴に長すぎる行があれば、その行だけ飛ばして、読めないファイルとして知らせる。
func TestCodexReportsLongLines(t *testing.T) {
	old := core.MaxLine
	core.MaxLine = 1 << 10
	defer func() { core.MaxLine = old }()
	home := t.TempDir()
	writeFiles(t, home, map[string]string{
		"sessions/2026/09/30/rollout-a.jsonl": `{"timestamp": "2026-09-30T01:00:00Z", "type": "session_meta", "payload": {"id": "a", "cwd": "/w"}}` + "\n" +
			`{"timestamp": "2026-09-30T01:00:01Z", "type": "event_msg", "payload": {"type": "user_message", "message": "` + strings.Repeat("x", 4096) + `"}}` + "\n" +
			`{"timestamp": "2026-09-30T01:00:02Z", "type": "event_msg", "payload": {"type": "user_message", "message": "短い依頼"}}` + "\n",
	})
	ids, err := loadAllErr(&Codex{Home: home})
	if !slices.Contains(ids, "a") {
		t.Errorf("読めた会話 = %v", ids)
	}
	if err == nil || !strings.Contains(err.Error(), "rollout-a.jsonl") || !strings.Contains(err.Error(), "longer than") {
		t.Errorf("Load のエラー = %v", err)
	}
}
