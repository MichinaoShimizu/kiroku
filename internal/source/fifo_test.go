//go:build unix

package source

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/MichinaoShimizu/kiroku/internal/archive"
	"github.com/MichinaoShimizu/kiroku/internal/core"
)

// mkfifo は名前つきパイプを作る（作れない環境ではテストを飛ばす）。
func mkfifo(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Skip(err)
	}
}

// loadWithin は s.Load を走らせ、止まらずに終わったら読めた会話の数とエラーを返す。
func loadWithin(t *testing.T, s Source) (int, error) {
	t.Helper()
	type result struct {
		n   int
		err error
	}
	done := make(chan result, 1)
	go func() {
		n := 0
		err := s.Load(func(*core.Builder) { n++ })
		done <- result{n, err}
	}()
	select {
	case r := <-done:
		return r.n, r.err
	case <-time.After(10 * time.Second):
		t.Fatal("名前つきパイプを開いて止まった")
		return 0, nil
	}
}

// Claude Code の projects に名前つきパイプ（会話の記録の名前の FIFO）があっても、読み込みは止まらない。
// ほかの会話は読み、パイプは読めないファイルとして知らせる。
func TestClaudeFIFO(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "-Users-me-app")
	writeFiles(t, dir, map[string]string{
		"ok.jsonl": `{"type":"user","timestamp":"2026-09-30T01:00:00Z","cwd":"/Users/me/app","message":{"role":"user","content":"hi"}}` + "\n",
	})
	mkfifo(t, filepath.Join(dir, "pipe.jsonl"))
	n, err := loadWithin(t, &Claude{Root: root})
	if n < 1 { // 読めないファイルと同じく、読めた分（パイプは何もない）も出す
		t.Errorf("sessions = %d, want >= 1", n)
	}
	if err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Errorf("err = %v, want not a regular file", err)
	}
}

// Codex の rollout の名前の名前つきパイプがあっても、読み込みは止まらない。
func TestCodexFIFO(t *testing.T) {
	home := t.TempDir()
	writeCodex(t, home, "thr-ok",
		`{"timestamp":"2026-10-06T00:00:00.000Z","type":"session_meta","payload":{"id":"thr-ok","timestamp":"2026-10-06T00:00:00.000Z","cwd":"/Users/me/project"}}`,
		`{"timestamp":"2026-10-06T00:00:01.000Z","type":"event_msg","payload":{"type":"user_message","message":"hi"}}`,
	)
	mkfifo(t, filepath.Join(home, "sessions", "2026", "10", "11", "rollout-2026-10-11T00-00-00-thr-pipe.jsonl"))
	n, err := loadWithin(t, &Codex{Home: home})
	if n != 1 {
		t.Errorf("sessions = %d, want 1", n)
	}
	if err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Errorf("err = %v, want not a regular file", err)
	}
}

// kiroku archive は、名前つきパイプを指すシンボリックリンクがあっても止まらず、パイプは残さない。
func TestArchiveFIFOLink(t *testing.T) {
	src, dst, other := t.TempDir(), t.TempDir(), t.TempDir()
	pipe := filepath.Join(other, "pipe")
	mkfifo(t, pipe)
	if err := os.Symlink(pipe, filepath.Join(src, "link.jsonl")); err != nil {
		t.Skip(err)
	}
	type result struct {
		n   int
		err error
	}
	done := make(chan result, 1)
	go func() {
		n, err := archive.Sync(src, dst)
		done <- result{n, err}
	}()
	select {
	case r := <-done:
		if r.n != 0 || r.err != nil {
			t.Errorf("Sync = %d, %v, want 0, nil", r.n, r.err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("名前つきパイプを開いて止まった")
	}
}
