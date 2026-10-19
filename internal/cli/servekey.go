package cli

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"html"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/MichinaoShimizu/kiroku/internal/core"
	"github.com/MichinaoShimizu/kiroku/internal/web"
)

// kiroku serve の鍵。127.0.0.1 で待ち受けていても、同じコンピューターのほかのユーザーはつなげてしまう。
// そこで、本人しか読めないファイル（0600）に置いた鍵を知っているブラウザにだけ履歴を見せる。
// kiroku open は、鍵そのものではなく使い捨てのコード（openCodeTTL のあいだ 1 回だけ使える）を ?code= に付けて開く。
// ブラウザは開いた URL を履歴に残し、同期していれば送ってしまうので、残るのが使い終わったコードだけになるように。
// 中継のページが鍵を localStorage にしまい、そのあとは鍵なしの URL（ブックマーク）で開ける。
// 以前の版の ?key=<鍵> の URL（ブックマーク）も受け付けるが、kiroku からはもう作らない。

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
	if err := core.PrivateDir(dir); err != nil {
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

// keyHeader は、画面が鍵を付けて送るヘッダー。画面は鍵をブラウザの localStorage（オリジン、つまりポートごと）に
// しまい、履歴を返すもの・状態を変えるものへの問い合わせのたびに付ける。
const keyHeader = "X-Kiroku-Key"

// keyStorage は、画面が鍵をしまう localStorage の名前（中継のページと js/state.js で同じもの）。
const keyStorage = "kiroku:key"

// oldKeyCookie は、以前の版が鍵を入れていた Cookie の名前。もう受け付けず、送られてきたら消させる。
// Cookie はポートを区別しないので、同じコンピューターのほかのポートのサーバー（ほかのユーザーのものかもしれない）にも送られてしまう。
func oldKeyCookie(addr string) string {
	_, port, _ := net.SplitHostPort(addr)
	return "kiroku_key_" + port
}

// requireKey は、鍵を知っている画面にだけ next を見せる。
// 鍵は X-Kiroku-Key ヘッダーでだけ受け付ける。Cookie は、ほかのポートのサーバーにも送られて盗まれうるので使わない。
// ?code=<使い捨てのコード>（または以前の版の ?key=<鍵>）で開かれたら、中継のページ（hopKeyPage）が鍵を localStorage にしまい、
// それを消した URL へ移る。GET / とマニフェスト（keyless）だけは鍵なしで通す。そこで返すのは履歴を入れていない画面の骨組みか
// 読み込み中の画面で、画面が localStorage の鍵を付けて data.json を読む（ブックマークなどの移動ではヘッダーを付けられないため）。
func requireKey(addr, key string, next http.Handler) http.Handler {
	old := oldKeyCookie(addr)
	match := func(s string) bool { return subtle.ConstantTimeCompare([]byte(s), []byte(key)) == 1 }
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := r.Cookie(old); err == nil { // 以前の版の Cookie は消させる（もれた鍵が、ほかのポートへ送られ続けないように）
			http.SetCookie(w, &http.Cookie{Name: old, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteStrictMode})
		}
		get := r.Method == http.MethodGet || r.Method == http.MethodHead
		q := r.URL.Query()
		if q.Has("key") || q.Has("code") {
			ok, give := false, ""
			switch {
			case q.Has("code") && q.Has("key"):
			case q.Has("code"): // 1 回だけ使える。HEAD で使い切らないよう、GET のときだけ使う
				ok = r.Method == http.MethodGet && useOpenCode(q.Get("code"))
				give = key // 鍵はページに入れて渡す（アドレスには使い終わったコードしか残らない）
			default:
				ok = get && match(q.Get("key"))
			}
			if !ok {
				locked(w)
				return
			}
			q.Del("key")
			q.Del("code")
			to := hopTarget(r.URL.Path)
			if len(q) > 0 {
				to += "?" + q.Encode()
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			_, _ = io.WriteString(w, hopKeyPage(to, give))
			return
		}
		if (get && keyless(r.URL.Path)) || match(r.Header.Get(keyHeader)) {
			next.ServeHTTP(w, r)
			return
		}
		locked(w)
	})
}

// keyless は、鍵なしの GET で返してよいパスか。/（履歴を入れていない画面の骨組み）と、アプリとしてインストールするための
// マニフェスト（ブラウザはヘッダーを付けずに読む）だけ。
func keyless(path string) bool {
	return path == "/" || slices.Contains(web.AppPaths, path)
}

// hopTarget は、?key= の中継のあとに移る、このサイトの中のパス。//host や /\host（ブラウザは \ を / と読む）、
// /<タブ>/host（ブラウザは URL のタブと改行を読み飛ばす）のような、ほかのサイトへの URL にしない。
// 先頭の / \ と制御文字を除き、残りは % で書く（中の制御文字も %09 などになり、ブラウザに読み飛ばされない）。
func hopTarget(path string) string {
	rest := strings.TrimLeftFunc(path, func(c rune) bool { return c == '/' || c == '\\' || c < 0x20 || c == 0x7f })
	return (&url.URL{Path: "/" + rest}).EscapedPath()
}

// locked は、鍵のない問い合わせへの答え（403 と、kiroku open を案内するページ）。
func locked(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusForbidden)
	_, _ = w.Write([]byte(lockedPage))
}

// lockedPage の文は、画面の骨組みが鍵なしで開かれたときの知らせ（js/live.js の showLocked）と同じ。
const lockedPage = `<!doctype html>
<html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>kiroku</title>
<style>body{font:16px/1.6 system-ui,sans-serif;max-width:36em;margin:4em auto;padding:0 16px}code{background:rgba(127,127,127,.15);padding:.1em .3em;border-radius:4px}</style>
<h1>Open kiroku with its key</h1>
<p>kiroku serve shows your history only to a browser that has its key, so that other users of this computer can't read it.</p>
<p>Run <code>kiroku open</code> in a terminal to open this page with the key. After that, this browser can open it without the key.</p>
<p>To open it on another device, run <code>kiroku open --print</code> and open the address it prints there.</p>
</html>
`

// hopKeyScript は、?code= か ?key= で開かれたときの中継のページのスクリプト。鍵は #to の data-k（?code= のとき）か、
// アドレスの ?key= から読む（スクリプトの中身は変わらないので、CSP のハッシュは 1 回だけ作る）。形（64 桁の 16 進数）を確かめて localStorage にしまい、
// 鍵を消した URL（サーバーが決めた #to のリンク先）へ、ブラウザの履歴に残さずに移る。
// しまえなかったとき（このアドレスのサイトのデータを止めているなど）は、そう伝える。
const hopKeyScript = `(() => {
  const to = document.getElementById("to"), msg = document.getElementById("msg"), k = to.dataset.k || new URLSearchParams(location.search).get("key") || "";
  let ok = false;
  if (/^[0-9a-f]{64}$/.test(k)) try { localStorage.setItem("` + keyStorage + `", k); ok = localStorage.getItem("` + keyStorage + `") === k; } catch(e){}
  if (ok){ location.replace(to.getAttribute("href")); return; }
  try { history.replaceState(null, "", to.getAttribute("href")); } catch(e){} // しまえなくても、鍵はアドレスに残さない
  msg.textContent = "This browser doesn't let kiroku keep its key (site data may be blocked for this address), so it can't show your history here.";
  msg.className = "show";
})();`

// hopKeyPage は、?code= か ?key= で開かれたときの中継のページ。to はそれを消した、このサイトの中の URL。
// give は、ページに入れて渡す鍵（?code= のとき。?key= のときは "" で、スクリプトがアドレスから読む）。
// スクリプトと <style> は CSP のハッシュで許したものだけで、外へは何も読みにいかず、Referer も送らない。
func hopKeyPage(to, give string) string {
	attrs := ` href="` + html.EscapeString(to) + `"`
	if give != "" {
		attrs += ` data-k="` + html.EscapeString(give) + `"`
	}
	return "<!doctype html>\n<html lang=\"en\"><meta charset=\"utf-8\"><meta name=\"color-scheme\" content=\"light dark\">" +
		"<meta http-equiv=\"Content-Security-Policy\" content=\"" + hopKeyCSP + "\"><meta name=\"referrer\" content=\"no-referrer\">" +
		"\n<title>kiroku</title>\n" + hopStyle +
		"\n<p><a id=\"to\"" + attrs + ">Open kiroku</a></p><p id=\"msg\"></p>\n<script>" + hopKeyScript + "</script>\n</html>\n"
}

// hopKeyCSP は中継のページの CSP。
var hopKeyCSP = "default-src 'none'; script-src " + cspHash(hopKeyScript) +
	"; style-src " + cspHash(strings.TrimSuffix(strings.TrimPrefix(hopStyle, "<style>"), "</style>")) +
	"; base-uri 'none'; form-action 'none'"

// cspHash は CSP に書く sha256 のハッシュ。
func cspHash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return "'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"
}

// hopPage は、すぐに to へ移るだけの中継のページ（open.html。?key= で開かれたときは hopKeyPage）。
// 移るまでの一瞬に白い背景と青いリンクが見えないよう、背景はローディング画面と同じ色にし、
// リンクの文字も背景と同じ色にしておく。移れなかったとき（meta refresh が止められているなど）のために、
// リンクは 2 秒たってから見えるようにする。head は <meta> などのタグだけ（中身はエスケープしない）。
func hopPage(to, head string) string {
	to = html.EscapeString(to)
	return "<!doctype html>\n<html lang=\"en\"><meta charset=\"utf-8\"><meta name=\"color-scheme\" content=\"light dark\">" + head +
		"\n<meta http-equiv=\"refresh\" content=\"0;url=" + to + "\">\n<title>kiroku</title>\n" + hopStyle +
		"\n<p><a href=\"" + to + "\">Open kiroku</a></p>\n</html>\n"
}

// hopStyle の色は loading.html の --bg と --acc。#msg.show は hopKeyPage で鍵をしまえなかったときの知らせ（すぐに見せる）。
const hopStyle = `<style>html{--bg:#f3f4f6;--acc:#4338ca;background:var(--bg);color:var(--bg)}` +
	`@media (prefers-color-scheme:dark){html{--bg:#0e0f13;--acc:#818cf8}}` +
	`body{margin:0;min-height:100vh;display:grid;place-items:center;font:13px/1.5 system-ui,sans-serif}` +
	`a{color:inherit;animation:show .3s 2s forwards}@keyframes show{to{color:var(--acc)}}` +
	`#msg.show{color:CanvasText;max-width:32em;padding:0 16px}</style>`

// openCodeTTL は、kiroku open のコードを使えるあいだ（ほかの機器で --print の URL を打ちこむ時間も見こむ）。
const openCodeTTL = 10 * time.Minute

// openCodeDir は、使い捨てのコードを置くフォルダ（鍵と同じ設定のフォルダの中。0700）。
// コードそのものは置かず、sha256 を名前にした空のファイル（0600）を置く。作った時刻はファイルの時刻。
const openCodeDir = "open-codes"

// newOpenCode は、kiroku serve の画面を 1 回だけ開ける、使い捨てのコードを作る。古いコードはここで片づける。
func newOpenCode() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	dir = filepath.Join(dir, openCodeDir)
	if err := core.PrivateDir(dir); err != nil {
		return "", err
	}
	pruneOpenCodes(dir)
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	code := hex.EncodeToString(b)
	f, err := os.OpenFile(filepath.Join(dir, openCodeName(code)), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", err
	}
	return code, f.Close()
}

// openCodeName は、コードを置くファイルの名前（sha256 の 16 進数）。フォルダを読めても、コードはわからない。
func openCodeName(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}

// useOpenCode は、コードが使えるか確かめ、使えるなら使い切る（ファイルを消せたときだけ true。同時に 2 回使われても 1 回だけ通る）。
func useOpenCode(code string) bool {
	if len(code) != 64 {
		return false
	}
	if _, err := hex.DecodeString(code); err != nil {
		return false
	}
	dir, err := configDir()
	if err != nil {
		return false
	}
	path := filepath.Join(dir, openCodeDir, openCodeName(code))
	st, err := os.Lstat(path)
	if err != nil || !st.Mode().IsRegular() {
		return false
	}
	if os.Remove(path) != nil {
		return false
	}
	age := time.Since(st.ModTime())
	return age >= -time.Minute && age <= openCodeTTL // 時計が少し戻っても通す
}

// pruneOpenCodes は、期限を過ぎたコードのファイルを消す。
func pruneOpenCodes(dir string) {
	es, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range es {
		if st, err := e.Info(); err == nil && st.Mode().IsRegular() && time.Since(st.ModTime()) > openCodeTTL {
			os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}

// codeURL は使い捨てのコードつきの URL（url は http://host:port/ の形）。
func codeURL(url string) (string, error) {
	code, err := newOpenCode()
	if err != nil {
		return "", err
	}
	return url + "?code=" + code, nil
}

// openWithKey は、使い捨てのコードつきの URL をブラウザで開く。URL をそのままブラウザのコマンドに渡すと、
// コードがプロセスの引数としてほかのユーザーに見えてしまうので、本人しか読めない HTML ファイルを開き、そこから移る。
func openWithKey(url string) error {
	dir, err := configDir()
	if err != nil {
		return err
	}
	if err := core.PrivateDir(dir); err != nil {
		return err
	}
	to, err := codeURL(url)
	if err != nil {
		return err
	}
	page := hopPage(to, `<meta name="referrer" content="no-referrer">`)
	path := filepath.Join(dir, "open.html")
	if err := writePrivate(path, []byte(page)); err != nil {
		return err
	}
	openBrowser(path)
	return nil
}
