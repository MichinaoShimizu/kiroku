package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestCompareVersions(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want int
	}{
		{"v0.1.1", "v0.1.0", 1}, {"v0.1.0", "v0.1.1", -1}, {"v0.10.0", "v0.9.9", 1}, {"0.1.1", "v0.1.1", 0},
		{"v1.0.0", "v1.0.0-rc.1", 1}, {"v1.0.0-rc.1", "v1.0.0", -1}, {"v1.0.0-rc.2", "v1.0.0-rc.1", 1},
	} {
		if got := compareVersions(c.a, c.b); got != c.want {
			t.Errorf("compareVersions(%s, %s) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

// 偽の Releases を立てて、最新の版を読み、落として確かめ、自分自身を置きかえるところまで通す。
func TestSelfUpdate(t *testing.T) {
	newBin := []byte("#!/bin/sh\necho new kiroku\n")
	var arc bytes.Buffer
	gz := gzip.NewWriter(&arc)
	tw := tar.NewWriter(gz)
	for _, f := range []struct{ name, body string }{{"README.md", "readme"}, {"kiroku", string(newBin)}} {
		tw.WriteHeader(&tar.Header{Name: f.name, Mode: 0o755, Size: int64(len(f.body)), Typeflag: tar.TypeReg})
		tw.Write([]byte(f.body))
	}
	tw.Close()
	gz.Close()
	name := "kiroku_9.9.9_linux_amd64.tar.gz"
	sum := sha256.Sum256(arc.Bytes())
	sums := hex.EncodeToString(sum[:]) + "  " + name + "\n"

	mux := http.NewServeMux()
	mux.HandleFunc("/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/releases/tag/v9.9.9", http.StatusFound)
	})
	mux.HandleFunc("/releases/download/v9.9.9/"+name, func(w http.ResponseWriter, r *http.Request) { w.Write(arc.Bytes()) })
	mux.HandleFunc("/releases/download/v9.9.9/checksums.txt", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, sums) })
	srv := httptest.NewServer(mux)
	defer srv.Close()
	defer func(b string) { releaseBase = b }(releaseBase)
	releaseBase = srv.URL

	if v, err := latestVersion(srv.Client()); err != nil || v != "v9.9.9" {
		t.Fatalf("latestVersion = %q, %v", v, err)
	}
	exe := filepath.Join(t.TempDir(), "kiroku")
	os.WriteFile(exe, []byte("old"), 0o755)
	if err := selfUpdate(srv.Client(), "v9.9.9", "linux", "amd64", exe); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(exe); !bytes.Equal(got, newBin) {
		t.Fatalf("入れかわっていない: %q", got)
	}
	if st, _ := os.Stat(exe); runtime.GOOS != "windows" && st.Mode().Perm()&0o100 == 0 { // Windows には実行の権限ビットがない
		t.Error("実行できる権限が落ちた")
	}

	// チェックサムが合わなければ入れかえない
	sums = strings.Repeat("0", 64) + "  " + name + "\n"
	os.WriteFile(exe, []byte("old"), 0o755)
	if err := selfUpdate(srv.Client(), "v9.9.9", "linux", "amd64", exe); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("チェックサム違いを通した: %v", err)
	}
	if got, _ := os.ReadFile(exe); string(got) != "old" {
		t.Fatal("失敗したのに元の kiroku が変わった")
	}
	// その版にファイルがなければエラー
	if err := selfUpdate(srv.Client(), "v9.9.9", "darwin", "arm64", exe); err == nil {
		t.Fatal("ないファイルでエラーにならない")
	}
}

// kiroku update のクライアントは、つながるまで・ヘッダーまでは短く待ち、遅い回線のダウンロードは途中で切らない。
func TestUpdateClientTimeouts(t *testing.T) {
	c := newUpdateClient()
	tr, ok := c.Transport.(*http.Transport)
	if !ok || tr.ResponseHeaderTimeout <= 0 || tr.ResponseHeaderTimeout > time.Minute || tr.TLSHandshakeTimeout <= 0 || tr.DialContext == nil {
		t.Fatalf("つながるまで・ヘッダーまでの上限がない: %+v", c.Transport)
	}
	if c.Timeout != 0 && c.Timeout < 10*time.Minute {
		t.Errorf("全体の上限 %v は遅い回線のダウンロードには短い", c.Timeout)
	}

	// 本体をゆっくり送っても読み切れる。最新の版の確認もこのクライアントで通る
	mux := http.NewServeMux()
	mux.HandleFunc("/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/releases/tag/v9.9.9", http.StatusFound)
	})
	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		for i := 0; i < 3; i++ {
			fmt.Fprint(w, "x")
			w.(http.Flusher).Flush()
			time.Sleep(50 * time.Millisecond)
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	defer func(b string) { releaseBase = b }(releaseBase)
	releaseBase = srv.URL
	if v, err := latestVersion(c); err != nil || v != "v9.9.9" {
		t.Fatalf("latestVersion = %q, %v", v, err)
	}
	if b, err := fetch(c, srv.URL+"/slow"); err != nil || string(b) != "xxx" {
		t.Fatalf("fetch = %q, %v", b, err)
	}
}

func TestUnknownSubcommand(t *testing.T) {
	if err := dispatch([]string{"updat"}); err == nil || !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("err = %v", err)
	}
}
