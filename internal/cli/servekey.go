package cli

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// kiroku serve の鍵。127.0.0.1 で待ち受けていても、同じコンピューターのほかのユーザーはつなげてしまう。
// そこで、本人しか読めないファイル（0600）に置いた鍵を知っているブラウザにだけ履歴を見せる。
// ブラウザは一度 ?key= つきの URL を開くと Cookie を受け取り、そのあとは鍵なしの URL（ブックマーク）で開ける。

// configDir は kiroku の設定の場所（$KIROKU_CONFIG_DIR、なければ OS の設定の場所の kiroku）。
func configDir() (string, error) {
	if d := os.Getenv("KIROKU_CONFIG_DIR"); d != "" {
		return d, nil
	}
	d, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "kiroku"), nil
}

const keyFile = "serve-key"

// errNoKey は、まだ kiroku serve を一度も動かしていない（鍵がない）こと。
var errNoKey = errors.New("kiroku serve has not been started yet (run \"kiroku serve\" or \"kiroku autostart on\" first)")

// serveKey は鍵を読む。create なら、なければ作る（2 つの serve が同時に作っても、先に作ったほうの鍵にそろう）。
// ほかのユーザーが読める鍵や、形のおかしい鍵は使わない（作り直す）。
func serveKey(create bool) (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, keyFile)
	if key, ok := readKey(path); ok {
		return key, nil
	}
	if !create {
		return "", errNoKey
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	key := hex.EncodeToString(b)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, fs.ErrExist) {
		if k, ok := readKey(path); ok { // ほかの serve が先に作った
			return k, nil
		}
		// 使えない鍵（ほかのユーザーが読める・壊れている）は置きかえる
		if err := writePrivate(path, []byte(key+"\n")); err != nil {
			return "", err
		}
		return key, nil
	}
	if err != nil {
		return "", err
	}
	if _, err := f.WriteString(key + "\n"); err != nil {
		f.Close()
		return "", err
	}
	return key, f.Close()
}

// readKey は使える鍵を読む（本人だけが読める、64 桁の 16 進数）。
func readKey(path string) (string, bool) {
	st, err := os.Lstat(path)
	if err != nil || !st.Mode().IsRegular() || (runtime.GOOS != "windows" && st.Mode().Perm()&0o077 != 0) {
		return "", false
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	key := strings.TrimSpace(string(b))
	if len(key) != 64 {
		return "", false
	}
	if _, err := hex.DecodeString(key); err != nil {
		return "", false
	}
	return key, true
}

// keyCookie は鍵を入れる Cookie の名前。Cookie はポートを区別しないので、名前にポートを入れる。
func keyCookie(addr string) string {
	_, port, _ := net.SplitHostPort(addr)
	return "kiroku_key_" + port
}

// requireKey は、鍵を知っているブラウザにだけ next を見せる。
// ?key=<鍵> で開かれたら Cookie を渡し、鍵を消した URL へ移す（鍵がアドレスバーや履歴に残らないように）。
// Cookie は JavaScript から読めず（HttpOnly）、ほかのサイトから来たときには送られない（SameSite=Strict）。
// Cookie はポートを区別しないので、Lax にすると、よそのサイトのリンクで同じコンピューターのほかのポート
// （ほかのユーザーのサーバーかもしれない）へ移らされたときにも鍵が送られてしまう。
// Strict の Cookie は、よそ（open.html など）から来た移動には付かないので、移る先へは、このポートのページから移る。
func requireKey(addr, key string, next http.Handler) http.Handler {
	name := keyCookie(addr)
	match := func(s string) bool { return subtle.ConstantTimeCompare([]byte(s), []byte(key)) == 1 }
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if r.Method == http.MethodGet && q.Has("key") && match(q.Get("key")) {
			http.SetCookie(w, &http.Cookie{Name: name, Value: key, Path: "/", MaxAge: 400 * 24 * 60 * 60, HttpOnly: true, SameSite: http.SameSiteStrictMode})
			q.Del("key")
			to := "/" + strings.TrimLeft(r.URL.Path, "/") // //host のような、ほかのサイトへの URL にしない
			if len(q) > 0 {
				to += "?" + q.Encode()
			}
			to = html.EscapeString(to)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			_, _ = fmt.Fprintf(w, "<!doctype html>\n<meta charset=\"utf-8\"><meta http-equiv=\"refresh\" content=\"0;url=%s\">\n<title>kiroku</title>\n<p><a href=\"%s\">Open kiroku</a></p>\n", to, to)
			return
		}
		if c, err := r.Cookie(name); err == nil && match(c.Value) {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(lockedPage))
	})
}

const lockedPage = `<!doctype html>
<html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>kiroku</title>
<style>body{font:16px/1.6 system-ui,sans-serif;max-width:36em;margin:4em auto;padding:0 16px}code{background:rgba(127,127,127,.15);padding:.1em .3em;border-radius:4px}</style>
<h1>Open kiroku with its key</h1>
<p>kiroku serve shows your history only to a browser that has its key, so that other users of this computer can't read it.</p>
<p>Run <code>kiroku open</code> in a terminal to open this page with the key. After that, this browser can open it without the key.</p>
<p>If this browser already has the key (you came here from a link on another site), <a href="">open it again from here</a>.</p>
<p>To open it on another device, run <code>kiroku open --print</code> and open the address it prints there.</p>
</html>
`

// keyURL は鍵つきの URL（url は http://host:port/ の形）。
func keyURL(url, key string) string { return url + "?key=" + key }

// openWithKey は、鍵つきの URL をブラウザで開く。URL をそのままブラウザのコマンドに渡すと、
// 鍵がプロセスの引数としてほかのユーザーに見えてしまうので、本人しか読めない HTML ファイルを開き、そこから移る。
func openWithKey(url, key string) error {
	dir, err := configDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	to := html.EscapeString(keyURL(url, key))
	page := fmt.Sprintf("<!doctype html>\n<meta charset=\"utf-8\"><meta name=\"referrer\" content=\"no-referrer\">\n<meta http-equiv=\"refresh\" content=\"0;url=%s\">\n<title>kiroku</title>\n<p><a href=\"%s\">Open kiroku</a></p>\n", to, to)
	path := filepath.Join(dir, "open.html")
	if err := writePrivate(path, []byte(page)); err != nil {
		return err
	}
	openBrowser(path)
	return nil
}
