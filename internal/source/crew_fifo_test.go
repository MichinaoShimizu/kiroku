//go:build unix

package source

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/MichinaoShimizu/kiroku/internal/core"
)

// Crew の場所に名前つきパイプ（会話の記録の名前の FIFO）があっても、読み込みは止まらない（開くだけで止まるので開かない）。
// サブエージェントの親として調べるときも同じ。
func TestKiroCrewFIFO(t *testing.T) {
	kiro, crew := t.TempDir(), t.TempDir()
	writeFiles(t, kiro, map[string]string{"sessions/cli/.keep": ""})
	writeFiles(t, crew, map[string]string{
		"subagents/sa1/state.json": `{"id": "sa1", "agent": "c", "task": "t", "parent_session": "x", "session_id": "s1"}`,
	})
	if err := os.MkdirAll(filepath.Join(crew, "sessions"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(crew, "sessions", "x.jsonl"), 0o600); err != nil {
		t.Skip(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = (&KiroCLI{Home: kiro, CrewHome: crew}).Load(func(*core.Builder) {})
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("名前つきパイプを開いて止まった")
	}
}
