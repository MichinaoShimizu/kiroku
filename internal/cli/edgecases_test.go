package cli

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/MichinaoShimizu/kiroku/internal/gitlog"
)

// Windows で新しい kiroku を置けなかったときは元に戻す。戻すのにも失敗したら、そのことと .old の場所もエラーに入れる
func TestSwapExe(t *testing.T) {
	type call struct{ from, to string }
	fake := func(fail map[int]error) (func(string, string) error, *[]call) {
		var calls []call
		return func(from, to string) error {
			calls = append(calls, call{from, to})
			return fail[len(calls)]
		}, &calls
	}
	exe := filepath.Join("C:", "bin", "kiroku.exe")
	ren, calls := fake(nil)
	if err := swapExe("linux", "tmp", exe, ren); err != nil || len(*calls) != 1 || (*calls)[0] != (call{"tmp", exe}) {
		t.Errorf("linux: %v %v", err, *calls)
	}
	ren, calls = fake(nil)
	if err := swapExe("windows", "tmp", exe, ren); err != nil || !slices.Equal(*calls, []call{{exe, exe + ".old"}, {"tmp", exe}}) {
		t.Errorf("windows: %v %v", err, *calls)
	}
	place := errors.New("place failed")
	ren, calls = fake(map[int]error{2: place})
	if err := swapExe("windows", "tmp", exe, ren); !errors.Is(err, place) || len(*calls) != 3 || (*calls)[2] != (call{exe + ".old", exe}) || strings.Contains(err.Error(), "put the previous") {
		t.Errorf("windows, placing fails: %v %v, want the error and a rollback", err, *calls)
	}
	back := errors.New("rollback failed")
	ren, _ = fake(map[int]error{2: place, 3: back})
	err := swapExe("windows", "tmp", exe, ren)
	if !errors.Is(err, place) || !errors.Is(err, back) || !strings.Contains(err.Error(), exe+".old") {
		t.Errorf("windows, rollback fails too: %v, want both errors and where the previous kiroku is", err)
	}
	ren, calls = fake(map[int]error{1: place})
	if err := swapExe("windows", "tmp", exe, ren); !errors.Is(err, place) || len(*calls) != 1 {
		t.Errorf("windows, moving aside fails: %v %v", err, *calls)
	}
}

// Windows で update したあとに残る kiroku.exe.old は、次に起動したときに消す。ほかの OS では何もしない
func TestRemoveOldExe(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "kiroku.exe")
	for _, p := range []string{exe, exe + ".old"} {
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	executable := func() (string, error) { return exe, nil }
	removeOldExe("linux", executable)
	if _, err := os.Stat(exe + ".old"); err != nil {
		t.Errorf("linux removed %s.old: %v", exe, err)
	}
	removeOldExe("windows", func() (string, error) { return "", errors.New("unknown") }) // わからなければ何もしない
	removeOldExe("windows", executable)
	if _, err := os.Stat(exe + ".old"); !os.IsNotExist(err) {
		t.Errorf("windows kept %s.old: %v", exe, err)
	}
	if _, err := os.Stat(exe); err != nil {
		t.Errorf("removed the running kiroku: %v", err)
	}
}

func TestSplitList(t *testing.T) {
	for in, want := range map[string][]string{"": nil, " , ,": nil, "a": {"a"}, "a, b ,,c ": {"a", "b", "c"}} {
		if got := splitList(in); !slices.Equal(got, want) {
			t.Errorf("splitList(%q) = %q, want %q", in, got, want)
		}
	}
}

// kiroku serve: ポートがほかのプログラムにふさがれていたら、オプションを読んだあと、履歴を読む前に止まる
func TestCmdServeBusyPort(t *testing.T) {
	other := httptest.NewServer(http.NotFoundHandler()) // kiroku serve ではないもの
	defer other.Close()
	addr := strings.TrimPrefix(other.URL, "http://")
	t.Setenv("KIROKU_CONFIG_DIR", t.TempDir())
	start := time.Now()
	err := dispatch([]string{"serve", addr, "--no-open", "--sources", "claude", "--claude-root", t.TempDir(), "--archive-dir", t.TempDir(), "--allow-host", "a.example, b.example"})
	if err == nil || !strings.Contains(err.Error(), "could not listen on "+addr) {
		t.Errorf("err = %v, want could not listen on %s", err, addr)
	}
	if time.Since(start) > 10*time.Second {
		t.Errorf("took %v", time.Since(start))
	}
	if err := dispatch([]string{"serve", "not an address", "--no-open"}); err == nil || !strings.Contains(err.Error(), "must look like") {
		t.Errorf("bad address: %v", err)
	}
}

// どのリポジトリにあるかを調べる git が時間切れになった作業場所は、端末に 1 行出し、画面の META（gitTimeout）にも入る
func TestLoaderReportsGitTimeout(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git がない")
	}
	old, oldLog := gitlog.RepoTimeout, logw
	t.Cleanup(func() { gitlog.RepoTimeout, logw = old, oldLog })
	gitlog.RepoTimeout = time.Nanosecond // どの git もすぐ時間切れになる
	var out strings.Builder
	logw = &out
	work := t.TempDir()
	root := t.TempDir()
	line := fmt.Sprintf(`{"type":"user","sessionId":"s1","uuid":"u1","timestamp":"2026-10-05T01:00:00Z","cwd":%q,"message":{"role":"user","content":"hello"}}`, work)
	writeBranchFile(t, filepath.Join(root, "p", "s1.jsonl"), line)
	fs := newFS("serve", "")
	c := addCommon(fs)
	if err := fs.Parse([]string{"--sources", "claude", "--claude-root", root, "--archive-dir", t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	_, load, err := c.loader()
	if err != nil {
		t.Fatal(err)
	}
	snap := load()
	if len(snap.data) != 1 {
		t.Fatalf("sessions = %d, want 1", len(snap.data))
	}
	if got, _ := snap.meta["gitTimeout"].([]string); !slices.Equal(got, []string{work}) {
		t.Errorf("meta gitTimeout = %v, want [%s]", snap.meta["gitTimeout"], work)
	}
	if !strings.Contains(out.String(), gitTimeoutLine(work)) {
		t.Errorf("terminal = %q, want %q", out.String(), gitTimeoutLine(work))
	}
}
