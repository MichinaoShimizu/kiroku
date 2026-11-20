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
	"sync"
	"time"

	"github.com/MichinaoShimizu/kiroku/internal/core"
	"github.com/MichinaoShimizu/kiroku/internal/web"
)

// kiroku serve の鍵。127.0.0.1 で待ち受けていても、同じコンピューターのほかのユーザーはつなげてしまう。
// そこで、本人しか読めないファイル（0600）に置いた鍵を知っているブラウザにだけ履歴を見せる。
// kiroku open は、鍵そのものではなく使い捨てのコード（openCodeTTL のあいだ 1 回だけ使える）を ?code= に付けて開く。
// ブラウザは開いた URL を履歴に残し、同期していれば送ってしまうので、残るのが使い終わったコードだけになるように。
// 中継のページが鍵を localStorage にしまい、そのあとは鍵なしの URL（ブックマーク）で開ける。
// 以前の版の ?key=<鍵> の URL は受け付けない（1.0 から）。ブラウザの履歴に鍵が残るので、開かれたら鍵のない案内（locked）を返す。

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
// ?code=<使い捨てのコード> で開かれたら、中継のページ（hopKeyPage）が鍵を localStorage にしまい、
// それを消した URL へ移る。GET / とマニフェスト（keyless）だけは鍵なしで通す。そこで返すのは履歴を入れていない画面の骨組みか
// 読み込み中の画面で、画面が localStorage の鍵を付けて data.json を読む（ブックマークなどの移動ではヘッダーを付けられないため）。
func requireKey(addr, key string, next http.Handler) http.Handler {
	old := oldKeyCookie(addr)
	_, port, _ := net.SplitHostPort(addr)
	match := func(s string) bool { return subtle.ConstantTimeCompare([]byte(s), []byte(key)) == 1 }
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := r.Cookie(old); err == nil { // 以前の版の Cookie は消させる（もれた鍵が、ほかのポートへ送られ続けないように）
			http.SetCookie(w, &http.Cookie{Name: old, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteStrictMode})
		}
		get := r.Method == http.MethodGet || r.Method == http.MethodHead
		q := r.URL.Query()
		if q.Has("key") { // 以前の版の ?key=<鍵>。鍵が合っていても使わない（kiroku open で開き直してもらう）
			lockedPage403(w, r, oldKeyPage, oldKeyPageJA)
			return
		}
		if q.Has("code") {
			// ?code= の POST は、中継のページのスクリプトがコードを鍵に換えるところ。コードは 1 回だけ使え、鍵は答えの本文で返す。
			// GET では鍵を渡さず、コードも使わない（リンクの先読みやチャットのプレビューが、コードを使い切ったり鍵を受け取ったりしないように）。
			// ほかのサイトのページから換えさせないよう、/archive と同じく X-Kiroku ヘッダーと同じオリジンを求める
			if r.Method == http.MethodPost {
				if r.Header.Get("X-Kiroku") != "1" || (r.Header.Get("Origin") != "" && r.Header.Get("Origin") != "http://"+r.Host) || !useOpenCode(q.Get("code"), port) {
					http.Error(w, "forbidden", http.StatusForbidden)
					return
				}
				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				w.Header().Set("Cache-Control", "no-store")
				_, _ = io.WriteString(w, key)
				return
			}
			if !get {
				locked(w, r)
				return
			}
			q.Del("code")
			to := hopTarget(r.URL.Path)
			if len(q) > 0 {
				to += "?" + q.Encode()
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			_, _ = io.WriteString(w, hopKeyPage(to))
			return
		}
		if (get && keyless(r.URL.Path)) || match(r.Header.Get(keyHeader)) {
			next.ServeHTTP(w, r)
			return
		}
		locked(w, r)
	})
}

// keyless は、鍵なしの GET で返してよいパスか。/（履歴を入れていない画面の骨組み）と、アプリとしてインストールするための
// マニフェスト（ブラウザはヘッダーを付けずに読む）だけ。
func keyless(path string) bool {
	return path == "/" || slices.Contains(web.AppPaths, path)
}

// hopTarget は、?code= の中継のあとに移る、このサイトの中のパス。//host や /\host（ブラウザは \ を / と読む）、
// /<タブ>/host（ブラウザは URL のタブと改行を読み飛ばす）のような、ほかのサイトへの URL にしない。
// 先頭の / \ と制御文字を除き、残りは % で書く（中の制御文字も %09 などになり、ブラウザに読み飛ばされない）。
func hopTarget(path string) string {
	rest := strings.TrimLeftFunc(path, func(c rune) bool { return c == '/' || c == '\\' || c < 0x20 || c == 0x7f })
	return (&url.URL{Path: "/" + rest}).EscapedPath()
}

// locked は、鍵のない問い合わせへの答え（403 と、kiroku open を案内するページ）。ブラウザーの言語（Accept-Language）が
// 日本語なら日本語のページ（画面の showLocked と同じ文）にする。
func locked(w http.ResponseWriter, r *http.Request) { lockedPage403(w, r, lockedPage, lockedPageJA) }

// lockedPage403 は、決まったページ（英語か日本語）を 403 で返す。ページに問い合わせの中身は入れない。
func lockedPage403(w http.ResponseWriter, r *http.Request, en, ja string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Add("Vary", "Accept-Language")
	w.WriteHeader(http.StatusForbidden)
	page := en
	if prefersJA(r.Header.Get("Accept-Language")) {
		page = ja
	}
	_, _ = w.Write([]byte(page))
}

// prefersJA は、Accept-Language のいちばん先の言語が日本語か。
func prefersJA(al string) bool {
	first, _, _ := strings.Cut(al, ",")
	first, _, _ = strings.Cut(first, ";")
	first = strings.ToLower(strings.TrimSpace(first))
	return first == "ja" || strings.HasPrefix(first, "ja-")
}

// lockedPage の文は、画面の骨組みが鍵なしで開かれたときの知らせ（js/live.js の showLocked）と同じ。
const lockedPage = `<!doctype html>
<html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>kiroku</title>
<style>body{font:16px/1.6 system-ui,sans-serif;max-width:36em;margin:4em auto;padding:0 16px}code{background:rgba(127,127,127,.15);padding:.1em .3em;border-radius:4px;white-space:nowrap}html:lang(ja){word-break:auto-phrase}</style>
<h1>Open kiroku with its key</h1>
<p>kiroku serve shows your history only to a browser that has its key, so that other users of this computer can't read it.</p>
<p>Run <code>kiroku open</code> in a terminal to open this page with the key. After that, this browser can open it without the key.</p>
<p>To open it on another device, run <code>kiroku open --print</code> and open the address it prints there.</p>
</html>
`

// lockedPageJA は lockedPage の日本語（i18n.js の showLocked の文と同じ）。
const lockedPageJA = `<!doctype html>
<html lang="ja"><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>kiroku</title>
<style>body{font:16px/1.6 system-ui,sans-serif;max-width:36em;margin:4em auto;padding:0 16px}code{background:rgba(127,127,127,.15);padding:.1em .3em;border-radius:4px;white-space:nowrap}html:lang(ja){word-break:auto-phrase}</style>
<h1>鍵を付けて kiroku を開いてください</h1>
<p>kiroku serve は、この PC のほかのユーザーに履歴を読まれないよう、鍵を持つブラウザーにだけ履歴を見せます。</p>
<p>ターミナルで <code>kiroku open</code> を実行すると、鍵を付けてこの画面を開きます。そのあとは、このブラウザーなら鍵なしで開けます。</p>
<p>ほかの機器で開くときは、<code>kiroku open --print</code> を実行し、表示されたアドレスをその機器で開いてください。</p>
</html>
`

// oldKeyPage は、以前の版の ?key=<鍵> のアドレスで開かれたときの案内。そのアドレスがもう使えないこと、kiroku open のあとは
// 鍵のない / を開く（ブックマークし直す）こと、履歴とブックマークから消して鍵も変えることを言う。リンクは決まった / だけ。
const oldKeyPage = `<!doctype html>
<html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>kiroku</title>
<style>body{font:16px/1.6 system-ui,sans-serif;max-width:36em;margin:4em auto;padding:0 16px}code{background:rgba(127,127,127,.15);padding:.1em .3em;border-radius:4px;white-space:nowrap}html:lang(ja){word-break:auto-phrase}</style>
<h1>This address no longer opens kiroku</h1>
<p>It was made by kiroku before v0.38.2 and has the key in it, so kiroku 1.0 and later refuse it.</p>
<p>Run <code>kiroku open</code> in a terminal. After that, open <a href="/">kiroku without the key in the address</a> and bookmark that instead.</p>
<p>Delete this address from your browser's history and bookmarks. Because it was in your history with the key, also change the key: stop <code>kiroku serve</code>, delete <code>serve-key</code> in kiroku's settings folder, and start it again.</p>
</html>
`

// oldKeyPageJA は oldKeyPage の日本語。
const oldKeyPageJA = `<!doctype html>
<html lang="ja"><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>kiroku</title>
<style>body{font:16px/1.6 system-ui,sans-serif;max-width:36em;margin:4em auto;padding:0 16px}code{background:rgba(127,127,127,.15);padding:.1em .3em;border-radius:4px;white-space:nowrap}html:lang(ja){word-break:auto-phrase}</style>
<h1>このアドレスでは kiroku を開けません</h1>
<p>v0.38.2 より前の kiroku が作った、鍵の入ったアドレスです。kiroku 1.0 からは受け付けません。</p>
<p>ターミナルで <code>kiroku open</code> を実行してください。そのあとは、<a href="/">鍵の入っていないアドレスの kiroku</a> を開き、そちらをブックマークしてください。</p>
<p>このアドレスは、ブラウザーの履歴とブックマークから消してください。鍵が履歴に残っていたので、鍵も変えてください（<code>kiroku serve</code> を止め、kiroku の設定の場所の <code>serve-key</code> を消して、起動し直します）。</p>
</html>
`

// hopKeyScript は、?code= で開かれたときの中継のページのスクリプト（中身は変わらないので、CSP のハッシュは 1 回だけ作る）。
// 同じアドレスへの POST でコードを鍵に換える。形（64 桁の 16 進数）を確かめて localStorage にしまい、
// コードや鍵を消した URL（サーバーが決めた #to のリンク先）へ、ブラウザの履歴に残さずに移る。
// 換えられなかった（使い終わった・期限切れ）ときや、しまえなかったとき（このアドレスのサイトのデータを止めているなど）は、そう伝える。
const hopKeyScript = `(async () => {
  const to = document.getElementById("to"), msg = document.getElementById("msg"), q = new URLSearchParams(location.search);
  const ja = (() => { try { const v = q.get("lang") || JSON.parse(localStorage.getItem("kiroku:lang") || "null"); if (v === "ja" || v === "en") return v === "ja"; } catch(e){}
    try { return (navigator.languages && navigator.languages[0] || navigator.language || "").toLowerCase().startsWith("ja"); } catch(e){ return false; } })();
  if (ja){ document.documentElement.lang = "ja"; to.textContent = "kiroku を開く"; }
  let k = "", ok = false, why = ja ? "このブラウザーは kiroku の鍵をしまえない（このアドレスのサイトのデータが止められているかもしれません）ので、ここでは履歴を見せられません。" : "This browser doesn't let kiroku keep its key (site data may be blocked for this address), so it can't show your history here.";
  if (q.has("code")){
    k = "";
    try { const r = await fetch(location.href, {method: "POST", headers: {"X-Kiroku": "1"}, cache: "no-store"}); if (r.ok) k = await r.text(); } catch(e){}
    if (!k) why = ja ? "この一度だけのアドレスは、もう使われたか期限が切れています。もう一度 kiroku open を実行してください。" : "This one-time address was already used or has expired. Run kiroku open again.";
  }
  if (/^[0-9a-f]{64}$/.test(k)) try { localStorage.setItem("` + keyStorage + `", k); ok = localStorage.getItem("` + keyStorage + `") === k; } catch(e){}
  if (ok){ location.replace(to.getAttribute("href")); return; }
  try { history.replaceState(null, "", to.getAttribute("href")); } catch(e){} // しまえなくても、鍵やコードはアドレスに残さない
  msg.textContent = why;
  msg.className = "show";
})();`

// hopKeyPage は、?code= で開かれたときの中継のページ。to はそれを消した、このサイトの中の URL。ページに鍵は入れない。
// スクリプトと <style> は CSP のハッシュで許したものだけで、問い合わせは同じオリジンへの POST だけ、Referer も送らない。
func hopKeyPage(to string) string {
	to = html.EscapeString(to)
	return "<!doctype html>\n<html lang=\"en\"><meta charset=\"utf-8\"><meta name=\"color-scheme\" content=\"light dark\">" +
		"<meta http-equiv=\"Content-Security-Policy\" content=\"" + hopKeyCSP + "\"><meta name=\"referrer\" content=\"no-referrer\">" +
		"\n<title>kiroku</title>\n" + hopStyle +
		"\n<p><a id=\"to\" href=\"" + to + "\">Open kiroku</a></p><p id=\"msg\"></p>\n<script>" + hopKeyScript + "</script>\n</html>\n"
}

// hopKeyCSP は中継のページの CSP。
var hopKeyCSP = "default-src 'none'; script-src " + cspHash(hopKeyScript) +
	"; style-src " + cspHash(strings.TrimSuffix(strings.TrimPrefix(hopStyle, "<style>"), "</style>")) +
	"; connect-src 'self'; base-uri 'none'; form-action 'none'"

// cspHash は CSP に書く sha256 のハッシュ。
func cspHash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return "'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"
}

// hopPage は、すぐに to へ移るだけの中継のページ（open.html。?code= で開かれたときは hopKeyPage）。
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
const hopStyle = `<style>html{--bg:#f4f4f2;--acc:#4f4aa8;background:var(--bg);color:var(--bg)}` +
	`@media (prefers-color-scheme:dark){html{--bg:#121418;--acc:#8a90d8}}` +
	`body{margin:0;min-height:100vh;display:grid;place-items:center;font:13px/1.5 system-ui,sans-serif}` +
	`a{color:inherit;animation:show .3s 2s forwards}@keyframes show{to{color:var(--acc)}}` +
	`#msg.show{color:CanvasText;max-width:32em;padding:0 16px}</style>`

// openCodeTTL は、kiroku open のコードを使えるあいだ（ほかの機器で --print の URL を打ちこむ時間も見こむ）。
const openCodeTTL = 10 * time.Minute

// openCodeDir は、使い捨てのコードを置くフォルダ（鍵と同じ設定のフォルダの中。0700）。
// コードそのものは置かず、sha256 を名前にした空のファイル（0600）を置く。作った時刻はファイルの時刻。
const openCodeDir = "open-codes"

// newOpenCode は、port で待ち受ける kiroku serve の画面を 1 回だけ開ける、使い捨てのコードを作る。古いコードはここで片づける。
func newOpenCode(port string) (string, error) {
	dir, err := openCodesDir(true)
	if err != nil {
		return "", err
	}
	pruneOpenCodes(dir)
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	code := hex.EncodeToString(b)
	f, err := os.OpenFile(filepath.Join(dir, openCodeName(port, code)), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", err
	}
	return code, f.Close()
}

// openCodesDir は、コードを置くフォルダ。本人だけが開ける本物のフォルダ（シンボリックリンクではない）でなければ使わない。
func openCodesDir(create bool) (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	dir = filepath.Join(dir, openCodeDir)
	if create {
		if err := core.PrivateDir(filepath.Dir(dir)); err != nil {
			return "", err
		}
		if err := core.PrivateDir(dir); err != nil {
			return "", err
		}
	}
	st, err := os.Lstat(dir)
	if err != nil {
		return "", err
	}
	if !st.IsDir() || !private(st) {
		return "", errors.New(dir + " is not a folder only you can open")
	}
	return dir, nil
}

// private は、ほかのユーザーが読み書きできないか（Windows では権限を使わないので確かめない）。
func private(st fs.FileInfo) bool { return runtime.GOOS == "windows" || st.Mode().Perm()&0o077 == 0 }

// openCodeName は、コードを置くファイルの名前（待ち受けるポートとコードの sha256 の 16 進数）。フォルダを読めても、コードはわからず、
// ほかのポートの kiroku serve では使えない（止めているあいだにそのポートで待ち受けたほかのユーザーが、コードを本物のほうで使えないように）。
func openCodeName(port, code string) string {
	sum := sha256.Sum256([]byte(port + "\n" + code))
	return hex.EncodeToString(sum[:])
}

// openCodeMu は、コードを確かめて消すあいだを 1 つずつにする。macOS（APFS）では、同じファイルを同時に消すと
// どちらも成功を返すことがあり、ファイルを消せたかだけでは 1 回にならない。ポートを待ち受ける kiroku serve は 1 つなので、プロセスの中で足りる。
var openCodeMu sync.Mutex

// useOpenCode は、port で待ち受ける kiroku serve のコードが使えるか確かめ、使えるなら使い切る
// （ファイルを消せたときだけ true。同時に 2 回使われても 1 回だけ通る）。
func useOpenCode(code, port string) bool {
	if len(code) != 64 {
		return false
	}
	if _, err := hex.DecodeString(code); err != nil {
		return false
	}
	dir, err := openCodesDir(false)
	if err != nil {
		return false
	}
	path := filepath.Join(dir, openCodeName(port, code))
	openCodeMu.Lock()
	defer openCodeMu.Unlock()
	st, err := os.Lstat(path)
	if err != nil || !st.Mode().IsRegular() || !private(st) {
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
		if st, err := e.Info(); err == nil && st.Mode().IsRegular() && (time.Since(st.ModTime()) > openCodeTTL || time.Until(st.ModTime()) > time.Minute) {
			os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}

// codeURL は使い捨てのコードつきの URL（u は http://host:port/ の形）。
func codeURL(u string) (string, error) {
	p, err := url.Parse(u)
	if err != nil || p.Port() == "" {
		return "", errors.New("no port in " + u)
	}
	code, err := newOpenCode(p.Port())
	if err != nil {
		return "", err
	}
	return u + "?code=" + code, nil
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
