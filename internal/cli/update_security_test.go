package cli

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// 版の名前は v1.2.3 か v1.2.3-rc.1 の形だけを受け付ける（URL にそのまま入れるため）。
func TestValidTag(t *testing.T) {
	for tag, want := range map[string]bool{
		"v0.13.1": true, "v1.2.3-rc.1": true, "v10.20.30-beta": true,
		"": false, "v1.2": false, "1.2.3": false, "v1.2.3/../x": false, "v1.2.3?x=1": false, "v1.2.3-": false,
		"v1.2.3-rc/1": false, "v1.2.3\n": false, "v1.2.3 ": false, "v1.2.3+build": false, "../v1.2.3": false,
	} {
		if got := validTag(tag); got != want {
			t.Errorf("validTag(%q) = %v, want %v", tag, got, want)
		}
	}
}

// releases/latest のリダイレクト先の版がおかしければ、それで URL を作らない。
func TestLatestVersionRejectsOddTags(t *testing.T) {
	for _, loc := range []string{"/releases/tag/v1.2.3/../../evil", "/releases/tag/latest", "/releases/tag/v1.2.3%2F..%2Fx"} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Location", loc)
			w.WriteHeader(http.StatusFound)
		}))
		old := releaseBase
		releaseBase = srv.URL
		v, err := latestVersion(newUpdateClient())
		releaseBase = old
		srv.Close()
		if err == nil || !strings.Contains(err.Error(), "unexpected release version") {
			t.Errorf("%s: %q, %v", loc, v, err)
		}
	}
}

// ダウンロードのリダイレクトは https だけをたどる（テストの http の releaseBase と同じホストだけは許す）。
func TestUpdateRedirects(t *testing.T) {
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("evil")) }))
	defer other.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/away":
			http.Redirect(w, r, other.URL+"/x", http.StatusFound)
		case "/same":
			http.Redirect(w, r, "/ok", http.StatusFound)
		default:
			w.Write([]byte("ok"))
		}
	}))
	defer srv.Close()
	old := releaseBase
	defer func() { releaseBase = old }()
	releaseBase = srv.URL
	c := newUpdateClient()
	if b, err := fetch(c, srv.URL+"/same"); err != nil || string(b) != "ok" {
		t.Errorf("同じホスト: %q %v", b, err)
	}
	if b, err := fetch(c, srv.URL+"/away"); err == nil || !strings.Contains(err.Error(), "not https") {
		t.Errorf("ほかの http へのリダイレクトをたどった: %q %v", b, err)
	}
	// 本物の releaseBase（https）なら、http へはどこへもたどらない
	releaseBase = "https://github.com/MichinaoShimizu/kiroku"
	if b, err := fetch(c, srv.URL+"/same"); err == nil {
		t.Errorf("https の releaseBase で http のリダイレクトをたどった: %q", b)
	}
	req, _ := http.NewRequest("GET", "https://objects.example.com/x", nil)
	if err := checkRedirect(req, []*http.Request{{}}); err != nil {
		t.Errorf("https へのリダイレクト: %v", err)
	}
	if err := checkRedirect(req, make([]*http.Request, 10)); err == nil {
		t.Error("10 回より多いリダイレクトをたどった")
	}
}

// アーカイブの中の kiroku は、ふつうのファイルだけを、上限の大きさまでしか取り出さない。
func TestExtractRejects(t *testing.T) {
	tarOf := func(hs ...*tar.Header) []byte {
		var b bytes.Buffer
		gz := gzip.NewWriter(&b)
		tw := tar.NewWriter(gz)
		for _, h := range hs {
			tw.WriteHeader(h)
			if h.Size > 0 {
				tw.Write(bytes.Repeat([]byte("x"), int(h.Size)))
			}
		}
		tw.Close()
		gz.Close()
		return b.Bytes()
	}
	if _, err := extract(tarOf(&tar.Header{Name: "kiroku", Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd"}), ".tar.gz", "kiroku"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("tar のリンク: %v", err)
	}
	var zb bytes.Buffer
	zw := zip.NewWriter(&zb)
	h := &zip.FileHeader{Name: "kiroku.exe"}
	h.SetMode(os.ModeSymlink | 0o777)
	w, _ := zw.CreateHeader(h)
	w.Write([]byte("/etc/passwd"))
	zw.Close()
	if _, err := extract(zb.Bytes(), ".zip", "kiroku.exe"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("zip のリンク: %v", err)
	}

	old := maxBinary
	maxBinary = 16
	defer func() { maxBinary = old }()
	if _, err := extract(tarOf(&tar.Header{Name: "kiroku", Typeflag: tar.TypeReg, Mode: 0o755, Size: 17}), ".tar.gz", "kiroku"); err == nil || !strings.Contains(err.Error(), "larger than") {
		t.Errorf("tar の大きすぎるファイル: %v", err)
	}
	if b, err := extract(tarOf(&tar.Header{Name: "kiroku", Typeflag: tar.TypeReg, Mode: 0o755, Size: 16}), ".tar.gz", "kiroku"); err != nil || len(b) != 16 {
		t.Errorf("上限ちょうど: %d %v", len(b), err)
	}
	if _, err := extract(makeArchive(t, "windows", [2]string{"kiroku.exe", strings.Repeat("x", 17)}), ".zip", "kiroku.exe"); err == nil || !strings.Contains(err.Error(), "larger than") {
		t.Errorf("zip の大きすぎるファイル: %v", err)
	}
}
