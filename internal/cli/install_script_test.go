package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// installRun は install.sh を、偽の curl（リリースのファイルを手元のフォルダから返す）と偽の gh で動かす。
// gh の振る舞いは ghMode で選ぶ。env は KIROKU_VERSION などの上書き。
type installRun struct {
	out       string
	err       error
	installed bool
}

func runInstallScript(t *testing.T, ghMode string, args []string, env ...string) installRun {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("install.sh is for macOS and Linux")
	}
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no sh")
	}
	_, self, _, _ := runtime.Caller(0) // テストのあいだに作業フォルダが変わっても、このファイルから探す
	script := filepath.Join(filepath.Dir(self), "..", "..", "install.sh")
	arch := map[string]string{"amd64": "amd64", "arm64": "arm64"}[runtime.GOARCH]
	if arch == "" {
		t.Skip("install.sh supports amd64 and arm64 only")
	}
	dir := t.TempDir()
	rel, bin, dest := filepath.Join(dir, "rel"), filepath.Join(dir, "fakebin"), filepath.Join(dir, "dest")
	for _, d := range []string{rel, bin, dest} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	// 版の形の確かめで弾かれなかったものは、どれも 9.9.9 のファイルを返す（URL の版は見ない）
	for _, v := range []string{"9.9.9", "9.9.9-rc.1"} {
		name := "kiroku_" + v + "_" + runtime.GOOS + "_" + arch + ".tar.gz"
		arc := makeArchive(t, "linux", [2]string{"kiroku", "#!/bin/sh\necho kiroku 9.9.9\n"})
		sum := sha256.Sum256(arc)
		os.WriteFile(filepath.Join(rel, name), arc, 0o600)
		f, _ := os.OpenFile(filepath.Join(rel, "checksums.txt"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		f.WriteString(hex.EncodeToString(sum[:]) + "  " + name + "\n")
		f.Close()
	}
	// curl -o FILE URL：URL の最後の部分のファイルを、手元のリリースのフォルダから写す。
	// -I（releases/latest のリダイレクト先を読む）には FAKE_LATEST の URL を返す
	curl := "#!/bin/sh\nout=\"\" url=\"\" head=\"\"\nwhile [ $# -gt 0 ]; do case \"$1\" in -o) out=$2; shift ;; -w) shift ;; -*I*) head=1 ;; http*) url=$1 ;; esac; shift; done\n" +
		"if [ -n \"$head\" ]; then printf '%s' \"$FAKE_LATEST\"; exit 0; fi\n" +
		"cp \"" + rel + "/${url##*/}\" \"$out\"\n"
	gh := "#!/bin/sh\n[ \"$1 $2\" = \"attestation verify\" ] || exit 2\ncase \"$GH_ATTEST\" in\n" +
		"  ok) exit 0 ;;\n" +
		"  net) echo 'Error: failed to fetch attestations: dial tcp: lookup api.github.com: no such host' >&2; exit 1 ;;\n" +
		"  mismatch) printf 'Loaded digest sha256:abc\\nError: verifying with issuer \"sigstore.dev\"\\n' >&2; echo 'no matching CertificateIdentity found' >&2; exit 1 ;;\n" +
		"  evilref) echo 'Error: expected SourceRepositoryRef to be refs/heads/main, got refs/heads/fix-timeout' >&2; exit 1 ;;\n" +
		"  evilsan) echo 'Error: no matching CertificateIdentity found, last error: expected SAN to match x, got x/ci-timeout.yml (dial tcp)' >&2; exit 1 ;;\n" +
		// 証明書の値に端末の制御文字（ESC の色・題名の書き換え、BEL、CR）が入っているとき
		"  escmismatch) printf 'Error: expected SourceRepositoryRef to be refs/heads/main, got refs/heads/\\033]0;pwned\\007x\\033[2J\\rEVIL\\n' >&2; exit 1 ;;\n" +
		"  escnet) printf 'Error: dial tcp: \\033[31mred\\033[0m\\007 timeout\\n' >&2; exit 1 ;;\n" +
		"  escother) printf 'Error: \\033[2Jstrange\\007\\n' >&2; exit 1 ;;\n" +
		"esac\nexit 3\n"
	os.WriteFile(filepath.Join(bin, "curl"), []byte(curl), 0o755)
	os.WriteFile(filepath.Join(bin, "gh"), []byte(gh), 0o755)
	cmd := exec.Command(sh, append([]string{script}, args...)...)
	cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "GH_ATTEST="+ghMode,
		"KIROKU_VERSION=v9.9.9", "KIROKU_INSTALL_DIR="+dest, "KIROKU_REQUIRE_ATTESTATION=", "KIROKU_SKIP_ATTESTATION=", "NO_COLOR=1",
		"FAKE_LATEST=https://github.com/MichinaoShimizu/kiroku/releases/tag/v9.9.9")
	cmd.Env = append(cmd.Env, env...)
	out, err := cmd.CombinedOutput()
	_, statErr := os.Stat(filepath.Join(dest, "kiroku"))
	return installRun{out: string(out), err: err, installed: statErr == nil}
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode()
	}
	return -1
}

// gh attestation verify の説明に、証明書の値（ブランチ名・ワークフロー名）として timeout などの言葉が入っていても、
// 証明が合わないものは入れない（「GitHub に届かない」の警告にしない）。届かないときは、これまでどおり警告だけで入れる。
func TestInstallScriptAttestation(t *testing.T) {
	for _, c := range []struct {
		mode      string
		installed bool
		out       string
	}{
		{"ok", true, "verified the build provenance"},
		{"net", true, "could not reach GitHub"},
		{"mismatch", false, "does not match"},
		{"evilref", false, "does not match"},
		{"evilsan", false, "does not match"},
		{"escmismatch", false, "does not match"},
		{"escnet", true, "could not reach GitHub"},
		{"escother", false, "does not match"},
	} {
		t.Run(c.mode, func(t *testing.T) {
			r := runInstallScript(t, c.mode, nil)
			if r.installed != c.installed || (r.err == nil) != c.installed {
				t.Errorf("installed = %v (err %v), want %v\n%s", r.installed, r.err, c.installed, r.out)
			}
			if !strings.Contains(r.out, c.out) {
				t.Errorf("output lacks %q:\n%s", c.out, r.out)
			}
			// gh の出力は、制御文字を消してから端末に出す（改行とタブのほかは出さない）
			for _, b := range []byte(r.out) {
				if (b < 0x20 && b != '\n' && b != '\t') || b == 0x7f {
					t.Fatalf("output has control character %#x:\n%q", b, r.out)
				}
			}
		})
	}
}

// -h / --help は使い方を見せて何も入れない。ほかの引数は止める（以前は引数を見ずに入れていた）
func TestInstallScriptArgs(t *testing.T) {
	for _, c := range []struct {
		args []string
		code int
		out  string
	}{
		{[]string{"--help"}, 0, "KIROKU_INSTALL_DIR"},
		{[]string{"-h"}, 0, "KIROKU_REQUIRE_ATTESTATION"},
		{[]string{"--version", "v1.0.0"}, 2, "takes no arguments"},
		{[]string{"v1.0.0"}, 2, "takes no arguments"},
	} {
		t.Run(strings.Join(c.args, " "), func(t *testing.T) {
			r := runInstallScript(t, "ok", c.args)
			if r.installed {
				t.Errorf("installed with args %q:\n%s", c.args, r.out)
			}
			if got := exitCode(r.err); got != c.code {
				t.Errorf("exit code = %d, want %d\n%s", got, c.code, r.out)
			}
			if !strings.Contains(r.out, c.out) {
				t.Errorf("output lacks %q:\n%s", c.out, r.out)
			}
		})
	}
}

// 版は vX.Y.Z（-pre つき）の形だけ。KIROKU_VERSION からも、releases/latest のリダイレクト先からも
func TestInstallScriptVersionFormat(t *testing.T) {
	latest := "FAKE_LATEST=https://github.com/MichinaoShimizu/kiroku/releases/tag/"
	for _, c := range []struct {
		name      string
		env       []string
		installed bool
	}{
		{"exact", []string{"KIROKU_VERSION=v9.9.9"}, true},
		{"no v", []string{"KIROKU_VERSION=9.9.9"}, true},
		{"prerelease", []string{"KIROKU_VERSION=v9.9.9-rc.1"}, true},
		{"latest", []string{"KIROKU_VERSION=", latest + "v9.9.9"}, true},
		{"two parts", []string{"KIROKU_VERSION=v9.9"}, false},
		{"trailing junk", []string{"KIROKU_VERSION=v9.9.9/../x"}, false},
		{"space", []string{"KIROKU_VERSION=v9.9.9 x"}, false},
		{"newline", []string{"KIROKU_VERSION=v9.9.9\nv1"}, false},
		{"empty pre", []string{"KIROKU_VERSION=v9.9.9-"}, false},
		{"latest junk", []string{"KIROKU_VERSION=", latest + "v9x"}, false},
		{"latest page", []string{"KIROKU_VERSION=", "FAKE_LATEST=https://github.com/MichinaoShimizu/kiroku/releases"}, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := runInstallScript(t, "ok", nil, c.env...)
			if r.installed != c.installed {
				t.Errorf("installed = %v, want %v\n%s", r.installed, c.installed, r.out)
			}
			if !c.installed && !strings.Contains(r.out, "unexpected version format") {
				t.Errorf("output lacks the version error:\n%s", r.out)
			}
		})
	}
}
