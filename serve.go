package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
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
	print io.Writer
}

func (l *live) refresh() error {
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
		defer l.mu.RUnlock()
		send(w, "text/html; charset=utf-8", l.html)
	})
	mux.HandleFunc("/data.json", func(w http.ResponseWriter, r *http.Request) {
		l.mu.RLock()
		defer l.mu.RUnlock()
		send(w, "application/json", l.json)
	})
	// /history?id=<セッション ID> は、そのセッションの履歴ファイルをそのまま見せる（テキストの履歴だけ）。
	// 読めるのは読み込んだセッションの履歴ファイルだけで、任意のパスは受け付けない。
	mux.HandleFunc("/history", func(w http.ResponseWriter, r *http.Request) {
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
		if path == "" || !(strings.HasSuffix(path, ".jsonl") || strings.HasSuffix(path, ".json")) {
			http.NotFound(w, r)
			return
		}
		f, err := os.Open(path)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		_, _ = io.Copy(w, f)
	})
	mux.HandleFunc("/stamp", func(w http.ResponseWriter, r *http.Request) {
		l.mu.RLock()
		defer l.mu.RUnlock()
		send(w, "text/plain", []byte(strconv.FormatFloat(l.snap.gen, 'f', -1, 64)))
	})
	return mux
}

// sameOrigin は、ほかのサイトのページから手元の履歴を読まれないようにする（DNS リバインディング対策）。
// 手元だけで待ち受けているときは、Host が localhost / 127.0.0.1 / ::1 のときだけ通す。
// 0.0.0.0 などで外に開いたときは、本人が選んだことなので Host は問わない。
func sameOrigin(addr string, next http.Handler) http.Handler {
	host, port, _ := net.SplitHostPort(addr)
	if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() {
		return next
	}
	ok := map[string]bool{addr: true, "localhost:" + port: true, "127.0.0.1:" + port: true, "[::1]:" + port: true}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !ok[r.Host] {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func serveLive(addr string, every time.Duration, picked []source.Source, load func() snapshot, open bool) error {
	if every < time.Second {
		every = time.Second
	}
	var paths []string
	for _, s := range picked {
		paths = append(paths, source.WatchPaths(s)...)
	}
	l := &live{load: load, paths: paths, print: logw}
	if err := l.refresh(); err != nil {
		return err
	}
	logw = io.Discard // ここから先の読み直しでは、エージェントごとの行は出さない
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("could not listen on %s (try another port, e.g. \"kiroku serve :8485\"): %w", addr, err)
	}
	addr = ln.Addr().String()
	url := "http://" + addr + "/"
	if host, port, _ := net.SplitHostPort(addr); host == "127.0.0.1" || host == "::" || host == "0.0.0.0" || host == "" {
		url = "http://localhost:" + port + "/"
	}
	fmt.Fprintf(l.print, "%d sessions → %s (checking for new history every %s; press Ctrl+C to stop)\n", l.count(), url, every)
	go l.watch(every, nil)
	if open {
		openBrowser(url)
	}
	err = http.Serve(ln, sameOrigin(addr, l.handler()))
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
