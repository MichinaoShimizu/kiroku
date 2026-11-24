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

// install.sh を、偽の curl（リリースのファイルを手元のフォルダから返す）と偽の gh で動かす。
// gh attestation verify の説明に、証明書の値（ブランチ名・ワークフロー名）として timeout などの言葉が入っていても、
// 証明が合わないものは入れない（「GitHub に届かない」の警告にしない）。届かないときは、これまでどおり警告だけで入れる。
func TestInstallScriptAttestation(t *testing.T) {
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
	} {
		t.Run(c.mode, func(t *testing.T) {
			dir := t.TempDir()
			rel, bin, dest := filepath.Join(dir, "rel"), filepath.Join(dir, "fakebin"), filepath.Join(dir, "dest")
			for _, d := range []string{rel, bin, dest} {
				if err := os.MkdirAll(d, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			name := "kiroku_9.9.9_" + runtime.GOOS + "_" + arch + ".tar.gz"
			arc := makeArchive(t, "linux", [2]string{"kiroku", "#!/bin/sh\necho kiroku 9.9.9\n"})
			sum := sha256.Sum256(arc)
			os.WriteFile(filepath.Join(rel, name), arc, 0o600)
			os.WriteFile(filepath.Join(rel, "checksums.txt"), []byte(hex.EncodeToString(sum[:])+"  "+name+"\n"), 0o600)
			// curl -o FILE URL：URL の最後の部分のファイルを、手元のリリースのフォルダから写す
			curl := "#!/bin/sh\nout=\"\" url=\"\"\nwhile [ $# -gt 0 ]; do case \"$1\" in -o) out=$2; shift ;; http*) url=$1 ;; esac; shift; done\n" +
				"cp \"" + rel + "/${url##*/}\" \"$out\"\n"
			gh := "#!/bin/sh\n[ \"$1 $2\" = \"attestation verify\" ] || exit 2\ncase \"$GH_ATTEST\" in\n" +
				"  ok) exit 0 ;;\n" +
				"  net) echo 'Error: failed to fetch attestations: dial tcp: lookup api.github.com: no such host' >&2; exit 1 ;;\n" +
				"  mismatch) printf 'Loaded digest sha256:abc\\nError: verifying with issuer \"sigstore.dev\"\\n' >&2; echo 'no matching CertificateIdentity found' >&2; exit 1 ;;\n" +
				"  evilref) echo 'Error: expected SourceRepositoryRef to be refs/heads/main, got refs/heads/fix-timeout' >&2; exit 1 ;;\n" +
				"  evilsan) echo 'Error: no matching CertificateIdentity found, last error: expected SAN to match x, got x/ci-timeout.yml (dial tcp)' >&2; exit 1 ;;\n" +
				"esac\nexit 3\n"
			os.WriteFile(filepath.Join(bin, "curl"), []byte(curl), 0o755)
			os.WriteFile(filepath.Join(bin, "gh"), []byte(gh), 0o755)
			cmd := exec.Command(sh, script)
			cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "GH_ATTEST="+c.mode,
				"KIROKU_VERSION=v9.9.9", "KIROKU_INSTALL_DIR="+dest, "KIROKU_REQUIRE_ATTESTATION=", "KIROKU_SKIP_ATTESTATION=", "NO_COLOR=1")
			out, err := cmd.CombinedOutput()
			_, statErr := os.Stat(filepath.Join(dest, "kiroku"))
			if installed := statErr == nil; installed != c.installed || (err == nil) != c.installed {
				t.Errorf("installed = %v (err %v), want %v\n%s", installed, err, c.installed, out)
			}
			if !strings.Contains(string(out), c.out) {
				t.Errorf("output lacks %q:\n%s", c.out, out)
			}
		})
	}
}
