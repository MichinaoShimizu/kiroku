//go:build !windows

package gitlog

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MichinaoShimizu/kiroku/internal/core"
)

// 他人が作ったかもしれないリポジトリで、画面からコピーした git show を貼っても、そのリポジトリの設定（.git/config・
// .gitattributes）にある外部の diff・textconv・filter.*.process・fsmonitor・ページャーのコマンドが動かない（セキュリティ）。
func TestPastedGitShowRunsNoRepoCommands(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git がない")
	}
	dir, marks := t.TempDir(), t.TempDir()
	run(t, dir, nil, "init", "-q")
	run(t, dir, nil, "config", "user.email", "me@example.com")
	run(t, dir, nil, "config", "user.name", "me")
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a\n"), 0o644)
	os.WriteFile(filepath.Join(dir, ".gitattributes"), []byte("a.txt filter=evil diff=evil\n"), 0o644)
	run(t, dir, nil, "add", ".")
	run(t, dir, nil, "commit", "-q", "-m", "c")
	mark := func(name string) string { return "sh -c 'touch " + filepath.Join(marks, name) + "; cat'" }
	for k, v := range map[string]string{
		"filter.evil.process": mark("process"), "filter.evil.smudge": mark("smudge"), "filter.evil.clean": mark("clean"),
		"diff.evil.textconv": mark("textconv"), "diff.external": mark("external"), "core.pager": mark("pager"),
		"core.fsmonitor": mark("fsmonitor"),
	} {
		run(t, dir, nil, "config", k, v)
	}
	out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	pasteRuns(t, core.GitCmd(dir, core.GitShow, strings.TrimSpace(string(out))), marks)
}

// pasteRuns は、cmd を端末に貼ったのと同じく sh で動かし、marks に印のファイルができたら（リポジトリが決めたコマンドが動いたら）失敗にする。
// 誤りで終わるのはかまわない（手元にない中身を取りに行けずに止まるなど）。
func pasteRuns(t *testing.T, cmd, marks string) {
	t.Helper()
	if cmd == "" {
		t.Fatal("コマンドが出ない")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	sh := exec.CommandContext(ctx, "sh", "-c", cmd) // 人が端末に貼るのと同じ
	sh.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0")
	_ = sh.Run()
	if ms, _ := os.ReadDir(marks); len(ms) > 0 {
		var names []string
		for _, m := range ms {
			names = append(names, m.Name())
		}
		t.Errorf("%s がリポジトリの設定のコマンドを動かした: %v", cmd, names)
	}
}

func revParse(t *testing.T, dir, rev string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "rev-parse", rev).Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}

// 署名つきのコミット（gpgsig の行。本物の署名でなくてよい）で、log.showSignature と gpg.program を書いたリポジトリでも、
// 貼った show と log が gpg.program を動かさない（セキュリティ）。
func TestPastedGitCmdRunsNoSignatureProgram(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git がない")
	}
	dir, marks := t.TempDir(), t.TempDir()
	run(t, dir, nil, "init", "-q")
	run(t, dir, nil, "config", "user.email", "me@example.com")
	run(t, dir, nil, "config", "user.name", "me")
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a\n"), 0o644)
	run(t, dir, nil, "add", ".")
	run(t, dir, nil, "commit", "-q", "-m", "first")
	first := revParse(t, dir, "HEAD")
	tree := revParse(t, dir, "HEAD^{tree}")
	raw := "tree " + tree + "\nparent " + first + "\nauthor me <me@example.com> 1790000000 +0000\ncommitter me <me@example.com> 1790000000 +0000\n" +
		"gpgsig -----BEGIN PGP SIGNATURE-----\n \n x\n -----END PGP SIGNATURE-----\n\nsigned\n"
	hash := exec.Command("git", "-C", dir, "hash-object", "-t", "commit", "-w", "--stdin")
	hash.Stdin = strings.NewReader(raw)
	out, err := hash.Output()
	if err != nil {
		t.Fatal(err)
	}
	signed := strings.TrimSpace(string(out))
	run(t, dir, nil, "config", "log.showSignature", "true")
	// gpg.program はシェルを通さずに動かすので、スクリプトのファイルにする
	prog := filepath.Join(t.TempDir(), "gpg.sh")
	if err := os.WriteFile(prog, []byte("#!/bin/sh\ntouch '"+filepath.Join(marks, "gpg")+"'\ncat >/dev/null\nexit 1\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	run(t, dir, nil, "config", "gpg.program", prog)
	pasteRuns(t, core.GitCmd(dir, core.GitShow, signed), marks)
	pasteRuns(t, core.GitCmd(dir, "log --oneline", first+".."+signed), marks)
}

// 親のリポジトリが diff.submodule=diff なら、git show はサブモジュールの差分を別の git で出し、そちらは
// サブモジュールの設定（.git/modules/<名前>/config）の外部の diff を動かす。貼った show はそれを動かさない（セキュリティ）。
func TestPastedGitShowRunsNoSubmoduleDiff(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git がない")
	}
	sub, top, marks := t.TempDir(), t.TempDir(), t.TempDir()
	for _, d := range []string{sub, top} {
		run(t, d, nil, "init", "-q")
		run(t, d, nil, "config", "user.email", "me@example.com")
		run(t, d, nil, "config", "user.name", "me")
	}
	os.WriteFile(filepath.Join(sub, "f.txt"), []byte("1\n"), 0o644)
	run(t, sub, nil, "add", ".")
	run(t, sub, nil, "commit", "-q", "-m", "s1")
	os.WriteFile(filepath.Join(top, "a.txt"), []byte("a\n"), 0o644)
	run(t, top, nil, "add", ".")
	run(t, top, nil, "commit", "-q", "-m", "t1")
	run(t, top, nil, "-c", "protocol.file.allow=always", "submodule", "add", "-q", sub, "sub")
	run(t, top, nil, "commit", "-q", "-m", "add sub")
	os.WriteFile(filepath.Join(top, "sub", "f.txt"), []byte("2\n"), 0o644)
	run(t, filepath.Join(top, "sub"), nil, "commit", "-q", "-am", "s2")
	run(t, top, nil, "commit", "-q", "-am", "bump sub")
	run(t, top, nil, "config", "diff.submodule", "diff")
	run(t, filepath.Join(top, "sub"), nil, "config", "diff.external", "sh -c 'touch "+filepath.Join(marks, "subexternal")+"'")
	pasteRuns(t, core.GitCmd(top, core.GitShow, revParse(t, top, "HEAD")), marks)
}

// 部分クローン（手元にないファイルがある）のリポジトリでは、git show は足りない中身を取りに行き、そのとき
// remote.<名前>.uploadpack などリポジトリが決めたプログラムを動かす（リポジトリの protocol.file.allow=always は
// -c protocol.allow=never より強い）。貼った show は取りに行かない（セキュリティ）。
func TestPastedGitShowRunsNoLazyFetch(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git がない")
	}
	origin, clone, marks := t.TempDir(), filepath.Join(t.TempDir(), "r"), t.TempDir()
	run(t, origin, nil, "init", "-q")
	run(t, origin, nil, "config", "user.email", "me@example.com")
	run(t, origin, nil, "config", "user.name", "me")
	run(t, origin, nil, "config", "uploadpack.allowFilter", "true")
	os.WriteFile(filepath.Join(origin, "a.txt"), []byte("a\n"), 0o644)
	run(t, origin, nil, "add", ".")
	run(t, origin, nil, "commit", "-q", "-m", "c1")
	os.WriteFile(filepath.Join(origin, "a.txt"), []byte("b\n"), 0o644)
	run(t, origin, nil, "commit", "-q", "-am", "c2")
	cl := exec.Command("git", "clone", "-q", "--no-checkout", "--filter=blob:none", "file://"+origin, clone)
	cl.Env = append(os.Environ(), "GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=protocol.file.allow", "GIT_CONFIG_VALUE_0=always")
	if out, err := cl.CombinedOutput(); err != nil {
		t.Skipf("部分クローンが作れない: %v %s", err, out)
	}
	run(t, clone, nil, "config", "protocol.file.allow", "always")
	run(t, clone, nil, "config", "remote.origin.uploadpack", "sh -c 'touch "+filepath.Join(marks, "uploadpack")+"; exec git-upload-pack \"$@\"' --")
	pasteRuns(t, core.GitCmd(clone, core.GitShow, revParse(t, clone, "HEAD")), marks)
}
