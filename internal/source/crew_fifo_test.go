//go:build unix

package source

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
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

// Kiro を選んでいないときの調べ（ScanWithheldClaims）も、session_map.json が名前つきパイプなら開かずに止まらず終わり、
// 読めなかったことを知らせる。どの会話キーが中身を残さないかわからないので、state.json からわかる会話 ID は隠す。
func TestKiroCrewScanFIFOSessionMap(t *testing.T) {
	crew := t.TempDir()
	writeFiles(t, crew, map[string]string{
		"subagents/sa1/state.json": `{"agent": "c", "parent_session": "dashboard:chat-1-100", "session_id": "s1"}`,
	})
	mkfifo(t, filepath.Join(crew, "session_map.json"))
	type result struct {
		claims []string
		err    error
	}
	done := make(chan result, 1)
	go func() {
		c, err := (&KiroCLI{CrewHome: crew}).ScanWithheldClaims()
		done <- result{c, err}
	}()
	select {
	case r := <-done:
		if r.err == nil || !strings.Contains(r.err.Error(), "session_map.json") {
			t.Errorf("err = %v, want session_map.json が読めないこと", r.err)
		}
		if !slices.Contains(r.claims, claudeClaim("s1")) || !slices.Contains(r.claims, codexClaim("s1")) {
			t.Errorf("claims = %v, want s1 の写しを隠す", r.claims)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("session_map.json の名前つきパイプで止まった")
	}
}
