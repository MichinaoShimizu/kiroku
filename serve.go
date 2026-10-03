package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/MichinaoShimizu/kiroku/internal/core"
	"github.com/MichinaoShimizu/kiroku/internal/report"
	"github.com/MichinaoShimizu/kiroku/internal/source"
	"github.com/MichinaoShimizu/kiroku/internal/web"
)

// snapshot は、ある時点で読んだ履歴の集計。
type snapshot struct {
	data  []*core.Session
	weeks map[string]*report.Week
	meta  map[string]any
	rep   []source.Report
	gen   float64 // 読んだ時刻（UNIX 秒）。画面はこれが変わったら取り込み直す
}

// live は --serve の中身。履歴の指紋が変わったときだけ読み直し、最新の集計を配る。
type live struct {
	mu      sync.RWMutex
	snap    snapshot
	html    []byte
	json    []byte
	load    func() snapshot
	paths   []string // 履歴の場所
	journal string   // 判断ログのフォルダ（中までは潜らない）
	print   io.Writer
}

func (l *live) refresh() error {
	snap := l.load()
	html, err := web.Render(snap.data, snap.weeks, snap.meta, snap.gen, true)
	if err != nil {
		return err
	}
	js, err := json.Marshal(map[string]any{"sessions": snap.data, "weeks": snap.weeks, "meta": snap.meta, "generated": snap.gen})
	if err != nil {
		return err
	}
	l.mu.Lock()
	l.snap, l.html, l.json = snap, []byte(html), js
	l.mu.Unlock()
	return nil
}

// watch は every ごとに指紋を取り、変わっていたら読み直す。読み直しは 1 本ずつしか走らない。
// stop が閉じられたら終わる（テスト用）。
func (l *live) fingerprint() string {
	return source.Fingerprint(append(append([]string{}, l.paths...), source.JournalFiles(l.journal)...))
}

func (l *live) watch(every time.Duration, stop <-chan struct{}) {
	last := l.fingerprint()
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
		}
		fp := l.fingerprint()
		if fp == last {
			continue
		}
		last = fp
		before := l.count()
		if err := l.refresh(); err != nil {
			fmt.Fprintf(l.print, "読み直しに失敗したよ: %v\n", err)
			continue
		}
		after := l.count()
		fmt.Fprintf(l.print, "%s 履歴の変化を反映 → %d セッション（%+d）\n", time.Now().Format("15:04:05"), after, after-before)
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

func serveLive(addr string, every time.Duration, picked []source.Source, journal string, load func() snapshot, open bool) error {
	if every < time.Second {
		every = time.Second
	}
	var paths []string
	for _, s := range picked {
		paths = append(paths, source.WatchPaths(s)...)
	}
	l := &live{load: load, paths: paths, journal: journal, print: logw}
	if err := l.refresh(); err != nil {
		return err
	}
	logw = io.Discard // ここから先の読み直しでは、エージェントごとの行は出さない
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("%s で待ち受けできなかったよ（--serve :8485 のように別のポートを試してね）: %w", addr, err)
	}
	addr = ln.Addr().String()
	url := "http://" + addr + "/"
	if host, port, _ := net.SplitHostPort(addr); host == "127.0.0.1" || host == "::" || host == "0.0.0.0" || host == "" {
		url = "http://localhost:" + port + "/"
	}
	fmt.Fprintf(l.print, "%d セッション → %s （%s ごとに履歴の変化を確かめます。止めるときは Ctrl+C）\n", l.count(), url, every)
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
