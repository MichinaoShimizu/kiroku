package cli

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// releaseBase は Releases の置き場所、updateExe は置きかえる kiroku の実行ファイル（テストで差しかえる）。
var (
	releaseBase = "https://github.com/MichinaoShimizu/kiroku"
	updateExe   = os.Executable
)

// runVersion は kiroku version。
func runVersion() error {
	fmt.Printf("kiroku %s (%s/%s)\n", version, runtime.GOOS, runtime.GOARCH)
	return nil
}

// runUpdate は kiroku update: 最新（か --to の版）を落とし、checksums.txt で確かめてから自分自身を置きかえる。
func runUpdate(args []string) error {
	fs := newFS("update", "update [flags]\n\nDownloads the release for this OS/arch, verifies it against checksums.txt\nand replaces the running binary.")
	check := fs.Bool("check", false, "only check whether a newer release exists")
	to := fs.String("to", "", "install this `version` (e.g. v0.1.1) instead of the latest")
	force := fs.Bool("force", false, "replace even when already up to date or running a dev build (--to alone can also install an older version)")
	// 位置引数は受け付けない（fs.Parse だけだと、kiroku update v0.1.1 の v0.1.1 とその後ろのオプションを黙って捨て、最新を入れてしまう）
	if _, err := parse(fs, args, 0); err != nil {
		return quiet(err)
	}
	client := newUpdateClient()
	target := *to
	if target == "" {
		v, err := latestVersion(client)
		if err != nil {
			return fmt.Errorf("could not check the latest version: %w", err)
		}
		target = v
	}
	if !strings.HasPrefix(target, "v") {
		target = "v" + target
	}
	cur := "v" + strings.TrimPrefix(version, "v")
	newer := compareVersions(target, cur) > 0
	if *check {
		switch {
		case version == "dev":
			fmt.Printf("latest is %s (this kiroku is a dev build from source)\n", target)
		case newer:
			fmt.Printf("a new version is available: %s → %s (run \"kiroku update\")\n", cur, target)
		default:
			fmt.Printf("already up to date (%s)\n", cur)
		}
		return nil
	}
	if version == "dev" && !*force {
		return fmt.Errorf("this kiroku is a dev build from source, so it will not replace itself; run \"go install github.com/MichinaoShimizu/kiroku@latest\" or pass --force")
	}
	if !newer && *to == "" && !*force {
		fmt.Printf("already up to date (%s)\n", cur)
		return nil
	}
	exe, err := updateExe()
	if err != nil {
		return err
	}
	if p, err := filepath.EvalSymlinks(exe); err == nil {
		exe = p
	}
	fmt.Printf("downloading kiroku %s…\n", target)
	if err := selfUpdate(client, target, runtime.GOOS, runtime.GOARCH, exe); err != nil {
		return err
	}
	fmt.Printf("updated %s → %s (%s)\n", cur, target, exe)
	return nil
}

// newUpdateClient は kiroku update の HTTP クライアント。
// つながるまで・返事（ヘッダー）が来るまでは短く待ち、本体のダウンロードには全体で長めの上限だけをかける。
// 全体を 60 秒で切ると、遅い回線（おおよそ 170KB/s より遅い）ではダウンロードが終わらずに失敗するため。
func newUpdateClient() *http.Client {
	t := http.DefaultTransport.(*http.Transport).Clone() // プロキシの環境変数などはそのまま使う
	t.DialContext = (&net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}).DialContext
	t.TLSHandshakeTimeout = 15 * time.Second
	t.ResponseHeaderTimeout = 30 * time.Second
	return &http.Client{Transport: t, Timeout: 30 * time.Minute}
}

// latestCheckTimeout は最新の版を確かめるときの上限（リダイレクトのヘッダーを読むだけなので、すぐ終わるはず）。
const latestCheckTimeout = 30 * time.Second

// latestVersion は releases/latest のリダイレクト先から最新の版を読む（API の回数制限にかからない）。
func latestVersion(client *http.Client) (string, error) {
	c := *client
	if c.Timeout == 0 || c.Timeout > latestCheckTimeout {
		c.Timeout = latestCheckTimeout
	}
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := c.Get(releaseBase + "/releases/latest")
	if err != nil {
		return "", err
	}
	resp.Body.Close()
	loc := resp.Header.Get("Location")
	i := strings.LastIndex(loc, "/tag/")
	if i < 0 {
		return "", fmt.Errorf("release not found (%s)", resp.Status)
	}
	return loc[i+len("/tag/"):], nil
}

// compareVersions は v1.2.3 の形を比べる（-rc などは数字の部分だけで比べ、同じならついていないほうを新しいとみなす）。
func compareVersions(a, b string) int {
	split := func(v string) ([3]int, string) {
		v = strings.TrimPrefix(v, "v")
		pre := ""
		if i := strings.IndexAny(v, "-+"); i >= 0 {
			v, pre = v[:i], v[i:]
		}
		var n [3]int
		for i, p := range strings.SplitN(v, ".", 3) {
			n[i], _ = strconv.Atoi(p)
		}
		return n, pre
	}
	na, pa := split(a)
	nb, pb := split(b)
	for i := range na {
		if na[i] != nb[i] {
			if na[i] > nb[i] {
				return 1
			}
			return -1
		}
	}
	switch {
	case pa == pb:
		return 0
	case pa == "":
		return 1
	case pb == "":
		return -1
	}
	return strings.Compare(pa, pb)
}

// selfUpdate は tag の goos/goarch 向けのファイルを落として確かめ、exe を置きかえる。
func selfUpdate(client *http.Client, tag, goos, goarch, exe string) error {
	ext := ".tar.gz"
	if goos == "windows" {
		ext = ".zip"
	}
	name := fmt.Sprintf("kiroku_%s_%s_%s%s", strings.TrimPrefix(tag, "v"), goos, goarch, ext)
	base := releaseBase + "/releases/download/" + tag + "/"
	archive, err := fetch(client, base+name)
	if err != nil {
		return fmt.Errorf("could not download %s: %w", name, err)
	}
	sums, err := fetch(client, base+"checksums.txt")
	if err != nil {
		return fmt.Errorf("could not download checksums.txt: %w", err)
	}
	want := ""
	for _, line := range strings.Split(string(sums), "\n") {
		if f := strings.Fields(line); len(f) == 2 && f[1] == name {
			want = f[0]
		}
	}
	if want == "" {
		return fmt.Errorf("%s is not listed in checksums.txt", name)
	}
	if got := sha256.Sum256(archive); hex.EncodeToString(got[:]) != want {
		return fmt.Errorf("checksum mismatch, not updating (%s)", name)
	}
	bin := "kiroku"
	if goos == "windows" {
		bin = "kiroku.exe"
	}
	body, err := extract(archive, ext, bin)
	if err != nil {
		return err
	}
	return replaceExe(exe, body)
}

func fetch(client *http.Client, url string) ([]byte, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 200<<20))
}

// extract はアーカイブから bin を取り出す。
func extract(archive []byte, ext, bin string) ([]byte, error) {
	if ext == ".zip" {
		zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
		if err != nil {
			return nil, err
		}
		for _, f := range zr.File {
			if filepath.Base(f.Name) == bin {
				rc, err := f.Open()
				if err != nil {
					return nil, err
				}
				defer rc.Close()
				return io.ReadAll(rc)
			}
		}
		return nil, fmt.Errorf("%s not found in the archive", bin)
	}
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, err
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil, fmt.Errorf("%s not found in the archive", bin)
		}
		if err != nil {
			return nil, err
		}
		if filepath.Base(h.Name) == bin && h.Typeflag == tar.TypeReg {
			return io.ReadAll(tr)
		}
	}
}

// replaceExe は同じフォルダに書いてから入れかえる（途中で失敗しても元の kiroku は残る）。
// Windows は動いている exe を上書きできないので、いったん .old に名前を変えてから置く。
func replaceExe(exe string, body []byte) error {
	dir := filepath.Dir(exe)
	tmp, err := os.CreateTemp(dir, ".kiroku-new-*")
	if err != nil {
		return fmt.Errorf("cannot write to %s (it may need sudo): %w", dir, err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(body); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	mode := os.FileMode(0o755)
	if st, err := os.Stat(exe); err == nil {
		mode = st.Mode().Perm() | 0o111
	}
	if err := os.Chmod(tmp.Name(), mode); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		old := exe + ".old"
		os.Remove(old)
		if err := os.Rename(exe, old); err != nil {
			return err
		}
		if err := os.Rename(tmp.Name(), exe); err != nil {
			os.Rename(old, exe)
			return err
		}
		return nil
	}
	return os.Rename(tmp.Name(), exe)
}
