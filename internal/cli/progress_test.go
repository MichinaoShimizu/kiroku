package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// 最初の読み込みのあいだ、/progress は読み終わったエージェントから数と時間を出す（履歴の中身は出さない）。
// 読み終わると ready になり、そのあとの読み直しでは進み具合を記録しない。
func TestServeProgress(t *testing.T) {
	logw = io.Discard
	defer func() { logw = os.Stderr }()
	release := make(chan struct{})
	l := &live{print: io.Discard, prog: newProgress([]string{"Claude Code", "Codex"}),
		load: func() snapshot {
			tracker.agentDone("Claude Code", 12, 1500*time.Millisecond)
			tracker.setStage("git")
			<-release
			return snapshot{gen: 1}
		}}
	stop, done := make(chan struct{}), make(chan struct{})
	go func() { l.start(time.Hour, stop); close(done) }()
	defer func() { close(stop); <-done }()
	srv := httptest.NewServer(l.handler())
	defer srv.Close()

	type prog struct {
		Ready, Failed bool
		Stage         string
		Agents        []agentProgress
	}
	get := func() (p prog, raw string) {
		r, err := http.Get(srv.URL + "/progress")
		if err != nil {
			t.Fatal(err)
		}
		defer r.Body.Close()
		b, _ := io.ReadAll(r.Body)
		if r.StatusCode != 200 || r.Header.Get("Cache-Control") != "no-store" {
			t.Fatalf("/progress = %d %v", r.StatusCode, r.Header)
		}
		if err := json.Unmarshal(b, &p); err != nil {
			t.Fatal(err)
		}
		return p, string(b)
	}
	waitFor(t, "Claude Code が読み終わる", func() bool { p, _ := get(); return p.Stage == "git" })
	p, raw := get()
	want := []agentProgress{{Name: "Claude Code", N: 12, Ms: 1500, Done: true}, {Name: "Codex"}}
	if p.Ready || p.Failed || len(p.Agents) != 2 || p.Agents[0] != want[0] || p.Agents[1] != want[1] {
		t.Errorf("読み込み中の /progress = %s", raw)
	}
	close(release)
	waitFor(t, "読み終わる", func() bool { p, _ := get(); return p.Ready })
	l.mu.RLock() // start は ready にする前に tracker を外す
	off := tracker == nil
	l.mu.RUnlock()
	if !off {
		t.Error("最初の読み込みのあとも進み具合を記録している")
	}
}

// 進み具合を記録していないとき（tracker が nil）も、呼んでよい。
func TestProgressNil(t *testing.T) {
	var p *loadProgress
	p.agentDone("x", 1, time.Second)
	p.setStage("git")
	if s := string(p.json(true, false)); !strings.Contains(s, `"ready":true`) || !strings.Contains(s, `"agents":[]`) {
		t.Errorf("nil の json = %s", s)
	}
}

// parallel は、どの番号もちょうど 1 回ずつ呼ぶ。
func TestParallel(t *testing.T) {
	for _, n := range []int{0, 1, 3, 257} {
		calls := make([]atomic.Int32, n)
		parallel(n, func(i int) { calls[i].Add(1) })
		for i := range calls {
			if c := calls[i].Load(); c != 1 {
				t.Errorf("n=%d: %d 番を %d 回呼んだ", n, i, c)
			}
		}
	}
}
