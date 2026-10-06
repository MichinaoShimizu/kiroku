package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/MichinaoShimizu/kiroku/internal/core"
	"github.com/MichinaoShimizu/kiroku/internal/report"
	"github.com/MichinaoShimizu/kiroku/internal/source"
	"github.com/MichinaoShimizu/kiroku/internal/web"
)

// snapshot は、ある時点で読んだ履歴の集計。
type snapshot struct {
	data   []*core.Session
	weeks  map[string]*report.Week
	months map[string]*report.Summary
	meta   map[string]any
	rep    []source.Report
	gen    float64 // 読んだ時刻（UNIX 秒）。画面はこれが変わったら取り込み直す
}

// live は kiroku serve の中身。履歴の指紋が変わったときだけ読み直し、最新の集計を配る。
type live struct {
	mu    sync.RWMutex
	snap  snapshot
	html  []byte
	json  []byte
	load  func() snapshot
	paths []string // 履歴の場所
	roots []string // /history で見せてよいファイルの場所（履歴の場所と kiroku archive のコピーの場所）
	print io.Writer
	keep  func() error // 画面から kiroku archive をオンにする（nil ならできない）
	ready bool         // 最初の読み込みが終わったか（mu で守る）。終わるまでは読み込み中の画面と 503 を返す
	fail  error        // 最初の読み込みの失敗（mu で守る）。読み込み中の画面に出す

	reload sync.Mutex // 読み直しは 1 本ずつ（load の中の loadCache は同時に使えない）
}

func (l *live) refresh() error {
	l.reload.Lock()
	defer l.reload.Unlock()
	snap := l.load()
	html, err := web.Render(snap.data, snap.weeks, snap.months, snap.meta, snap.gen, true)
	if err != nil {
		return err
	}
	js, err := json.Marshal(map[string]any{"sessions": snap.data, "weeks": snap.weeks, "months": snap.months, "meta": snap.meta, "generated": snap.gen})
	if err != nil {
		return err
	}
	l.mu.Lock()
	l.snap, l.html, l.json = snap, []byte(html), js
	l.mu.Unlock()
	return nil
}

// start は最初の読み込みをして、そのあと履歴を見張る（watch）。serveLive が待ち受けを始めてから別の goroutine で呼ぶ。
// 失敗しても止めず、読み込み中の画面にエラーを出して見張りを続ける（履歴が変われば読み直しを試す）。
func (l *live) start(every time.Duration, stop <-chan struct{}) {
	t0 := time.Now()
	err := l.refresh()
	// ここから先の読み直しでは、エージェントごとの行は出さない。
	// ready にする前に書きかえるので、/archive からの読み直しとはぶつからない
	logw = io.Discard
	if err != nil {
		l.mu.Lock()
		l.fail = err
		l.mu.Unlock()
		fmt.Fprintf(l.print, "could not read history: %v (still serving; will try again when history changes)\n", err)
	} else {
		l.markReady()
		fmt.Fprintf(l.print, "%d sessions loaded in %s (checking for new history every %s)\n", l.count(), took(time.Since(t0)), every)
	}
	l.watch(every, stop)
}

func (l *live) markReady() {
	l.mu.Lock()
	l.ready, l.fail = true, nil
	l.mu.Unlock()
}

// loaded は最初の読み込みが終わったか。まだなら 503 を返して false（fail があればその中身を本文にする）。
func (l *live) loaded(w http.ResponseWriter) bool {
	l.mu.RLock()
	ready, fail := l.ready, l.fail
	l.mu.RUnlock()
	if ready {
		return true
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Retry-After", "1")
	msg := ""
	if fail != nil { // くわしいわけ（ファイルの場所など）は端末に出してあるので、画面には出さない
		msg = "could not read history; see the terminal where kiroku serve runs for details"
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusServiceUnavailable)
	_, _ = io.WriteString(w, msg)
	return false
}

// took は読み込みにかかった時間を短く書く（1 秒未満はミリ秒、それより長いと 0.1 秒まで）。
func took(d time.Duration) time.Duration {
	if d < time.Second {
		return d.Round(time.Millisecond)
	}
	return d.Round(100 * time.Millisecond)
}

func (l *live) fingerprint() string {
	return source.Fingerprint(l.paths)
}

// settler は、履歴の書き込みが落ち着くのを待ってから読み直すための判定。
// エージェントが動いている間は履歴が数秒ごとに追記されるので、変化のたびに全部を読み直すと重い。
// 変化が settle のあいだ止まったら読み直す。書き込みが続いても、最初の変化から max たったら読み直す。
type settler struct {
	settle, max    time.Duration
	pending        bool
	first, changed time.Time
}

// step は 1 回の確認の結果（指紋が変わったか）を受け取り、いま読み直すべきかを返す。
func (s *settler) step(changed bool, now time.Time) bool {
	if changed {
		if !s.pending {
			s.pending, s.first = true, now
		}
		s.changed = now
	}
	if !s.pending || (now.Sub(s.changed) < s.settle && now.Sub(s.first) < s.max) {
		return false
	}
	s.pending = false
	return true
}

// watch は every ごとに指紋を取り、変化が落ち着いたら読み直す（落ち着くまで every の 2 倍、長くても 12 倍）。
// 読み直しは 1 本ずつしか走らない。stop が閉じられたら終わる（テスト用）。
func (l *live) watch(every time.Duration, stop <-chan struct{}) {
	last := l.fingerprint()
	s := settler{settle: 2 * every, max: 12 * every}
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
		}
		fp := l.fingerprint()
		changed := fp != last
		last = fp
		if !s.step(changed, time.Now()) {
			continue
		}
		before := l.count()
		if err := l.refresh(); err != nil {
			fmt.Fprintf(l.print, "reload failed: %v\n", err)
			continue
		}
		l.markReady() // 最初の読み込みに失敗していたときは、ここで画面が開けるようになる
		after := l.count()
		fmt.Fprintf(l.print, "%s history changed → %d sessions (%+d)\n", time.Now().Format("15:04:05"), after, after-before)
	}
}

func (l *live) count() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.snap.data)
}

func (l *live) handler() http.Handler {
	mux := http.NewServeMux()
	send := func(w http.ResponseWriter, kind string, body []byte) {
		w.Header().Set("Content-Type", kind)
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(body)
	}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		l.mu.RLock()
		ready := l.ready
		l.mu.RUnlock()
		if !ready { // 読み終わるまでは、stamp を見て読み直す小さな画面
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write(web.Loading)
			return
		}
		l.mu.RLock()
		defer l.mu.RUnlock()
		send(w, "text/html; charset=utf-8", l.html)
	})
	mux.HandleFunc("/data.json", func(w http.ResponseWriter, r *http.Request) {
		if !l.loaded(w) {
			return
		}
		l.mu.RLock()
		defer l.mu.RUnlock()
		send(w, "application/json", l.json)
	})
	// /history?id=<セッション ID> は、そのセッションの履歴ファイルをそのまま見せる（テキストの履歴だけ）。
	// 読めるのは読み込んだセッションの履歴ファイルだけで、任意のパスは受け付けない。
	mux.HandleFunc("/history", func(w http.ResponseWriter, r *http.Request) {
		if !l.loaded(w) {
			return
		}
		id := r.URL.Query().Get("id")
		l.mu.RLock()
		path := ""
		for _, d := range l.snap.data {
			if d.ID == id {
				path = d.File
				break
			}
		}
		l.mu.RUnlock()
		if path == "" || !(strings.HasSuffix(path, ".jsonl") || strings.HasSuffix(path, ".json") || strings.HasSuffix(path, ".jsonl.zst")) {
			http.NotFound(w, r)
			return
		}
		real, ok := l.allowed(path)
		if !ok {
			http.NotFound(w, r)
			return
		}
		f, err := os.Open(real)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		var body io.Reader = f
		if strings.HasSuffix(path, ".zst") { // kiroku archive や Codex の圧縮した履歴は、ほどいて見せる
			d, err := core.NewZstdReader(f)
			if err != nil {
				http.NotFound(w, r)
				return
			}
			defer d.Close()
			body = d
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = io.Copy(w, body)
	})
	// /prompt?id=<セッション ID>&i=<何件目> は、HTML には先頭だけを入れた依頼文の全文を返す（依頼の流れの「全文を読み込む」）。
	mux.HandleFunc("/prompt", func(w http.ResponseWriter, r *http.Request) {
		if !l.loaded(w) {
			return
		}
		id, i := r.URL.Query().Get("id"), -1
		if n, err := strconv.Atoi(r.URL.Query().Get("i")); err == nil {
			i = n
		}
		text := ""
		l.mu.RLock()
		for _, d := range l.snap.data {
			if d.ID == id {
				if i >= 0 && i < len(d.Prompts) {
					text = d.Prompts[i].Full()
				}
				break
			}
		}
		l.mu.RUnlock()
		if text == "" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = io.WriteString(w, text)
	})
	// POST /archive は、画面の「kiroku にコピーを残す」（kiroku archive on と同じ）。新しい集計を返す。
	// ほかのサイトのページから押させないよう、画面だけが付ける X-Kiroku ヘッダーを求める
	// （付けるとブラウザは先に確認の問い合わせ（preflight）をするが、ここは答えないので、ほかのサイトからは送れない）。
	mux.HandleFunc("/archive", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("X-Kiroku") != "1" || (r.Header.Get("Origin") != "" && r.Header.Get("Origin") != "http://"+r.Host) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if l.keep == nil {
			http.NotFound(w, r)
			return
		}
		if !l.loaded(w) { // 最初の読み込みと重ねて読まない
			return
		}
		// くわしいわけ（ファイルの場所など）は端末に出し、画面には短い文だけを返す
		if err := l.keep(); err != nil {
			l.logf("could not turn on kiroku archive: %v\n", err)
			http.Error(w, "could not turn on kiroku archive; see the terminal where kiroku serve runs for details", http.StatusInternalServerError)
			return
		}
		if err := l.refresh(); err != nil {
			l.logf("reload failed: %v\n", err)
			http.Error(w, "could not read history; see the terminal where kiroku serve runs for details", http.StatusInternalServerError)
			return
		}
		l.mu.RLock()
		defer l.mu.RUnlock()
		send(w, "application/json", l.json)
	})
	mux.HandleFunc("/stamp", func(w http.ResponseWriter, r *http.Request) {
		if !l.loaded(w) { // 読み込み中の画面は、これが 200 になったら読み直す
			return
		}
		l.mu.RLock()
		defer l.mu.RUnlock()
		send(w, "text/plain", []byte(strconv.FormatFloat(l.snap.gen, 'f', -1, 64)))
	})
	return secureHeaders(mux)
}

// secureHeaders は、すべての答えに、ほかのサイトに埋め込ませない・中身の種類を推測させない・
// どこから来たかを送らない、といったヘッダーをつける（kiroku serve の画面は自分のサイトの中だけで動く）。
// Content-Security-Policy は frame-ancestors だけ（ほかの決まりは HTML の中の meta に書く）。
func secureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Frame-Options", "DENY")
		h.Set("Content-Security-Policy", "frame-ancestors 'none'")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Cross-Origin-Resource-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}

// logf は端末に出す（print がなければ何もしない）。
func (l *live) logf(format string, args ...any) {
	if l.print != nil {
		fmt.Fprintf(l.print, format, args...)
	}
}

// allowed は、/history で path を見せてよいか。シンボリックリンクをたどった本当の場所が、
// 読んでいる履歴の場所（roots）の下にあるときだけ見せる（履歴の中身で、ほかのファイルを読ませないため）。
// 見せてよければ、たどった先の場所を返す。
func (l *live) allowed(path string) (string, bool) {
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", false
	}
	for _, root := range l.roots {
		if root == "" {
			continue
		}
		if r, err := filepath.EvalSymlinks(root); err == nil {
			root = r
		}
		if within(root, real) {
			return real, true
		}
	}
	return "", false
}

// within は p が root か、その下にあるか。
func within(root, p string) bool {
	rel, err := filepath.Rel(root, p)
	return err == nil && (rel == "." || filepath.IsLocal(rel))
}

// historyRoots は、/history で見せてよい場所: Source が読む場所と、kiroku archive のコピーの場所。
func historyRoots(picked []source.Source) []string {
	var out []string
	for _, s := range picked {
		out = append(out, source.WatchPaths(s)...)
		if k, ok := s.(source.Keeper); ok {
			for _, x := range k.Keep() {
				out = append(out, x.Dst)
			}
		}
	}
	return out
}

// sameOrigin は、ほかのサイトのページから手元の履歴を読まれないようにする（DNS リバインディング対策）。
// Host が、このコンピューターを指す名前（localhost・ループバック・待ち受けている IP・このコンピューターの IP とホスト名）か、
// allow で足した名前のときだけ通す。0.0.0.0 などで外に開いても、よそのサイトが自分のドメインをこのコンピューターに向けて
// 読みに来られないよう、Host は必ず確かめる（同じネットワークのほかの機器から開くのは、本人が選んだことなので通す）。
func sameOrigin(addr string, allow []string, next http.Handler) http.Handler {
	host, port, _ := net.SplitHostPort(addr)
	fixed := []string{"localhost", "127.0.0.1", "::1"}
	if ip := net.ParseIP(host); ip == nil || !ip.IsUnspecified() {
		fixed = append(fixed, host) // 待ち受けている名前・IP
	}
	fixed = append(fixed, allow...)
	ok := func(h string, names []string) bool {
		for _, n := range names {
			if n = strings.ToLower(strings.Trim(n, "[]")); n == "" {
				continue
			}
			if h == strings.ToLower(net.JoinHostPort(n, port)) || (port == "80" && h == n) {
				return true
			}
		}
		return false
	}
	wide := host == "" || net.ParseIP(host) != nil && net.ParseIP(host).IsUnspecified()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := strings.ToLower(r.Host)
		if !ok(h, fixed) && !(wide && ok(h, localNames())) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// localNames は、このコンピューターの IP とホスト名（0.0.0.0 などで待ち受けているときに、ほかの機器から開ける名前）。
// IP は DHCP などで変わるので、そのつど調べる。テストで差しかえる。
var localNames = func() []string {
	var out []string
	if addrs, err := net.InterfaceAddrs(); err == nil {
		for _, a := range addrs {
			if n, ok := a.(*net.IPNet); ok {
				out = append(out, n.IP.String())
			}
		}
	}
	if h, err := os.Hostname(); err == nil && h != "" {
		out = append(out, h)
		if !strings.Contains(h, ".") {
			out = append(out, h+".local")
		}
	}
	return out
}

// listenAddr は、ポートだけ（:8485）のときに手元（127.0.0.1）だけで待ち受けるようにする。
// ポートだけを全部のネットワークで待ち受けると、同じネットワークのほかの人から履歴が見えてしまうため。
// 外に開くときは 0.0.0.0:8485 のようにはっきり書く。
func listenAddr(addr string) string {
	if strings.HasPrefix(addr, ":") {
		return "127.0.0.1" + addr
	}
	return addr
}

// serveHTTP は待ち受けたポートで srv を動かし、startLive は最初の読み込みと履歴の見張りを別の goroutine で始める
// （どちらもテストで、止められるものに差しかえる）。
var (
	serveHTTP = func(srv *http.Server, ln net.Listener) error { return srv.Serve(ln) }
	startLive = func(l *live, every time.Duration) { go l.start(every, nil) }
)

// newServer は kiroku serve の http.Server。ヘッダーを送らないまま、つなぎっぱなしにする相手で詰まらないよう時間を区切る。
// 履歴ファイル（/history）は大きいことがあるので、書き出しの時間（WriteTimeout）は区切らない。
func newServer(h http.Handler) *http.Server {
	return &http.Server{
		Handler:           h,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    64 << 10,
	}
}

// serveLive は kiroku serve。allow は Host として受け付ける名前を足すもの、keep は画面から kiroku archive をオンにする関数。
// 先に待ち受けてから読む（履歴が多いと最初の読み込みに時間がかかり、その間ポートが開いていないとブラウザが「つながらない」になるため）。
// 読み終わるまでは、読み込み中の画面を出す。
func serveLive(addr string, allow []string, every time.Duration, picked []source.Source, load func() snapshot, keep func() error, open bool) error {
	if every < time.Second {
		every = time.Second
	}
	addr = listenAddr(addr)
	ln, err := net.Listen("tcp", addr) // ポートがふさがっていたら、読む前にすぐ止める
	if err != nil {
		return fmt.Errorf("could not listen on %s (try another port, e.g. \"kiroku serve :8485\"): %w", addr, err)
	}
	var paths []string
	for _, s := range picked {
		paths = append(paths, source.WatchPaths(s)...)
	}
	l := &live{load: load, paths: paths, roots: historyRoots(picked), print: logw, keep: keep}
	addr = ln.Addr().String()
	url := "http://" + addr + "/"
	if host, port, _ := net.SplitHostPort(addr); host == "127.0.0.1" || host == "::" || host == "0.0.0.0" || host == "" {
		url = "http://localhost:" + port + "/"
	}
	fmt.Fprintf(l.print, "serving %s (reading history...; press Ctrl+C to stop)\n", url)
	startLive(l, every)
	if open {
		openBrowser(url)
	}
	err = serveHTTP(newServer(secureHeaders(sameOrigin(addr, allow, l.handler()))), ln)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
