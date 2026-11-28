package cli

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/MichinaoShimizu/kiroku/internal/core"
)

const prsAnswer = `{"data":{"viewer":{"login":"me-dev"},
"author":{"issueCount":70,"nodes":[
 {"url":"https://github.com/o/r/pull/12","title":"  Fix\u001b[31m the \u202ething\n","isDraft":false,"createdAt":"2026-11-25T02:00:00Z","updatedAt":"2026-11-27T01:10:00Z","reviewDecision":"CHANGES_REQUESTED","mergeable":"CONFLICTING","author":{"login":"me-dev"},"commits":{"nodes":[{"commit":{"committedDate":"2026-11-26T03:00:00Z","statusCheckRollup":{"state":"FAILURE"}}}]},"reviews":{"nodes":[{"submittedAt":"2026-11-26T09:30:00Z"}]}},
 {"url":"https://ghe.acme.internal/platform/web-app/pull/7","title":"<img src=x onerror=alert(1)>","isDraft":true,"reviewDecision":"SOMETHING_NEW","mergeable":"UNKNOWN","author":{"login":"bad login"},"commits":{"nodes":[{"commit":{"statusCheckRollup":{"state":"WEIRD"}}}]}}]},
"review":{"issueCount":1,"nodes":[
 {"url":"https://github.com/o/other/pull/3","title":"t","reviewDecision":"REVIEW_REQUIRED","author":{"login":"dependabot[bot]"},"commits":{"nodes":[{"commit":{"statusCheckRollup":{"state":"SUCCESS"}}}]}}]},
"mention":{"issueCount":60,"nodes":[
 {"url":"https://github.com/o/r/pull/12","title":"another title for the same pull request"},
 {"url":"javascript:alert(1)","title":"x"},
 {"url":"http://github.com/o/r/pull/1","title":"x"},
 {"url":"https://github.com/o/r/pull/1/files","title":"x"},
 {"url":"https://github.com/o/r/issues/1","title":"x"},
 {"url":"https://github.com/o/r/pull/1 --web; rm -rf ~","title":"x"},
 {"url":"https://evil.example/o/r/pull/0","title":"x"},
 {"url":"https://github.com/o/../pull/5","title":"x"},
 {"url":"https://github.com/o/./pull/5","title":"x"},
 {"url":"https://github.com/o/r/pull/12","title":"the same pull request again"},
 {},
 null,
 {"url":"https://github.com/o/r/pull/99","title":"","reviewDecision":"APPROVED","commits":{"nodes":[{"commit":{"statusCheckRollup":{"state":"PENDING"}}}]}}]}}}`

// gh の答え（ほかの人が書ける文字を含む）は、形を確かめてから画面に渡す：
// PR の URL の形でないものは捨て、状況は決まった語だけにし、題名から制御文字と書式文字を除き、同じ PR は 1 つにまとめる。
func TestParsePRs(t *testing.T) {
	res, ok := parsePRs([]byte(prsAnswer))
	if !ok {
		t.Fatal("not parsed")
	}
	if res.Login != "me-dev" || !res.On || res.Partial || res.Error != "" {
		t.Errorf("res = %+v", res)
	}
	if !res.More["author"] || res.More["review"] || !res.More["mention"] {
		t.Errorf("more = %v", res.More)
	}
	var urls []string
	for _, it := range res.Items {
		urls = append(urls, it.URL)
		if !prURLPattern.MatchString(it.URL) {
			t.Errorf("odd URL got through: %q", it.URL)
		}
	}
	want := []string{"https://github.com/o/r/pull/12", "https://ghe.acme.internal/platform/web-app/pull/7", "https://github.com/o/other/pull/3", "https://github.com/o/r/pull/99"}
	if strings.Join(urls, " ") != strings.Join(want, " ") {
		t.Fatalf("urls = %v, want %v", urls, want)
	}
	a := res.Items[0]
	if a.Title != "Fix[31m the thing" || a.Repo != "o/r" || a.Number != 12 || a.Review != "changes" || a.Checks != "failing" || !a.Conflict || a.Draft ||
		a.Author != "me-dev" || strings.Join(a.Why, ",") != "author,mention" || a.Updated != 1795741800 || a.Pushed != 1795662000 || a.Reviewed != 1795685400 {
		t.Errorf("first = %+v", a)
	}
	b := res.Items[1] // 知らない語は出さない。名前の形でない作者は出さない。題名はそのまま（画面がエスケープする）
	if b.Repo != "platform/web-app" || b.Number != 7 || b.Review != "" || b.Checks != "" || b.Conflict || !b.Draft || b.Author != "" || b.Created != 0 || b.Pushed != 0 || b.Reviewed != 0 || b.Title != "<img src=x onerror=alert(1)>" {
		t.Errorf("second = %+v", b)
	}
	if c := res.Items[2]; c.Review != "required" || c.Checks != "passing" || c.Author != "dependabot[bot]" || strings.Join(c.Why, ",") != "review" {
		t.Errorf("third = %+v", c)
	}
	if d := res.Items[3]; d.Review != "approved" || d.Checks != "pending" || d.Title != "" {
		t.Errorf("fourth = %+v", d)
	}

	long, _ := json.Marshal(strings.Repeat("あ", 5000))
	res, ok = parsePRs([]byte(`{"data":{"viewer":{"login":"x y"},"author":{"issueCount":1,"nodes":[{"url":"https://github.com/o/r/pull/1","title":` + string(long) + `}]},"review":null},"errors":[{"message":"x"}]}`))
	if !ok || !res.Partial || res.Login != "" || len(res.Items) != 1 || len([]rune(res.Items[0].Title)) != prsTitleMax {
		t.Errorf("partial answer: ok=%v res.Partial=%v login=%q items=%d", ok, res.Partial, res.Login, len(res.Items))
	}
	// 頼んだ数（50）より多く返されても、頼んだ数まで。同じ PR が同じわけで何度来ても、わけは 1 回
	var many []string
	for i := 1; i <= 300; i++ {
		many = append(many, `{"url":"https://github.com/o/r/pull/`+strconv.Itoa(i)+`","title":"t"}`)
	}
	dup := strings.Repeat(`{"url":"https://github.com/o/r/pull/1","title":"t"},`, 200)
	res, ok = parsePRs([]byte(`{"data":{"viewer":{"login":"x"},"author":{"issueCount":300,"nodes":[` + strings.Join(many, ",") + `]},"review":{"issueCount":1,"nodes":[` + dup + `{}]},"mention":{"issueCount":0,"nodes":[]}}}`))
	if !ok || len(res.Items) != prsPerList || strings.Join(res.Items[0].Why, ",") != "author,review" {
		t.Errorf("many: ok=%v items=%d why=%v", ok, len(res.Items), res.Items[0].Why)
	}
	for _, bad := range []string{"", "not json", `{"errors":[{"message":"x"}]}`, `{"data":null}`, `[]`} {
		if _, ok := parsePRs([]byte(bad)); ok {
			t.Errorf("parsed %q", bad)
		}
	}
}

// fakePRGH は、gh の代わりに動く sh のスクリプト。mode（GH_FAKE_MODE）で答えを選び、起動されたときの引数を 1 回 1 つのファイルに残す。
func fakePRGH(t *testing.T, mode string) (calls func() []string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake gh is a shell script")
	}
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "answer.json"), []byte(prsAnswer), 0o600)
	script := `#!/bin/sh
n=$(ls "` + dir + `" | grep -c '^call')
for a in "$@"; do printf '%s\0' "$a"; done > "` + dir + `/call$n"
case "$GH_FAKE_MODE" in
  nologin) printf 'To get started with GitHub CLI, please run:  gh auth login\n' >&2; exit 4 ;;
  nologin1) printf 'error connecting to api.github.com\ncheck your internet connection or https://githubstatus.com\nTry authenticating with:  gh auth login\n' >&2; exit 1 ;;
  fail) printf 'bad \033[31mthing\033[0m\n' >&2; exit 1 ;;
  garbage) echo "<html>"; exit 0 ;;
  partial) cat "` + dir + `/answer.json"; echo "gh: Resource not accessible by personal access token" >&2; exit 1 ;;
  hang) sleep 30; exit 0 ;;
esac
cat "` + dir + `/answer.json"
`
	gh := filepath.Join(dir, "gh")
	if err := os.WriteFile(gh, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	old := ghCommand
	t.Cleanup(func() { ghCommand = old })
	ghCommand = gh
	t.Setenv("GH_FAKE_MODE", mode)
	return func() []string {
		var out []string
		names, _ := filepath.Glob(filepath.Join(dir, "call*"))
		for _, n := range names {
			b, _ := os.ReadFile(n)
			out = append(out, string(b))
		}
		return out
	}
}

// kiroku serve --prs は、gh を決まった引数だけで起動する（シェルを通さず、履歴から来た文字は入れない）。
// 失敗は、画面に出す短い種類（nogh・auth・failed）と、端末に出すくわしいわけ（端末に出せる形）に分ける。
func TestFetchPRs(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		calls := fakePRGH(t, "")
		res, err := fetchPRs(context.Background())
		if err != nil || res.Error != "" || len(res.Items) != 4 || res.Partial {
			t.Fatalf("res = %+v, err = %v", res, err)
		}
		got := calls()
		want := strings.Join([]string{"api", "graphql", "-f", "query=" + prsQuery}, "\x00") + "\x00"
		if len(got) != 1 || got[0] != want {
			t.Errorf("gh was started with %q, want %q", got, want)
		}
	})
	t.Run("the query is fixed and names nobody", func(t *testing.T) {
		for _, bad := range []string{"$", "%s", "%v", "`"} {
			if strings.Contains(prsQuery, bad) {
				t.Errorf("prsQuery contains %q", bad)
			}
		}
		for _, q := range []string{"author:@me", "review-requested:@me", "mentions:@me"} {
			if !strings.Contains(prsQuery, q) {
				t.Errorf("prsQuery lacks %q", q)
			}
		}
		if strings.Contains(prsQuery, "mutation") {
			t.Error("prsQuery changes something on GitHub")
		}
	})
	for mode, kind := range map[string]string{"nologin": "auth", "nologin1": "auth", "fail": "failed", "garbage": "failed"} {
		t.Run(mode, func(t *testing.T) {
			fakePRGH(t, mode)
			res, err := fetchPRs(context.Background())
			if err == nil || res.Error != kind || !res.On || res.Items == nil || len(res.Items) != 0 {
				t.Errorf("res = %+v, err = %v, want kind %q", res, err, kind)
			}
			if err != nil && strings.ContainsAny(err.Error(), "\x1b\n") {
				t.Errorf("the error is not safe for a terminal: %q", err)
			}
		})
	}
	t.Run("no gh", func(t *testing.T) {
		fakePRGH(t, "")
		ghCommand = filepath.Join(t.TempDir(), "gh") // ない
		res, err := fetchPRs(context.Background())
		if !errors.Is(err, errGHNotFound) || res.Error != "nogh" {
			t.Errorf("res = %+v, err = %v", res, err)
		}
	})
	t.Run("partial", func(t *testing.T) { // 一部が取れなかっただけなら、取れた分を出して、そう断る
		fakePRGH(t, "partial")
		res, err := fetchPRs(context.Background())
		if err == nil || res.Error != "" || !res.Partial || len(res.Items) != 4 {
			t.Errorf("res = %+v, err = %v", res, err)
		}
	})
	t.Run("does not hang", func(t *testing.T) {
		fakePRGH(t, "hang")
		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
		defer cancel()
		t0 := time.Now()
		res, err := fetchPRs(ctx)
		if err == nil || res.Error != "failed" || time.Since(t0) > 10*time.Second {
			t.Errorf("res = %+v, err = %v after %s", res, err, time.Since(t0))
		}
	})
}

// 一覧を開いても、新しい結果があるうちは問い合わせ直さない。「更新」も、続けて押したぶんは問い合わせない。
func TestPRBoardAsksRarely(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	n := 0
	var logged []string
	b := &prBoard{now: func() time.Time { return now }, logf: func(f string, a ...any) { logged = append(logged, f) },
		fetch: func(context.Context) (prsResult, error) {
			n++
			if n == 3 { // 更新（失敗）
				return prsResult{On: true, Items: []prItem{}, Error: "failed"}, errors.New("boom")
			}
			return prsResult{On: true, Items: []prItem{{URL: "https://github.com/o/r/pull/1", Why: []string{"author"}}}}, nil
		}}
	steps := []struct {
		after time.Duration
		force bool
		want  int
	}{
		{0, false, 1}, // はじめて開いた
		{time.Second, false, 1},
		{prsFresh - 2*time.Second, false, 1},
		{2 * time.Second, false, 2}, // 古くなった
		{time.Second, true, 2},      // 更新を押したが、問い合わせたばかり
		{prsMinGap, true, 3},        // 更新（失敗）
		{time.Second, false, 3},     // 失敗も、すぐには問い合わせ直さない
		{0, true, 3},                // 失敗のあとの更新も、続けて押したものは問い合わせない
		{prsErrGap, true, 4},        // 失敗のあとの更新は prsErrGap で問い合わせる（gh auth login のあと）
		{time.Second, true, 4},      // 成功したので、また prsMinGap
		{prsMinGap, true, 5},
	}
	for i, s := range steps {
		now = now.Add(s.after)
		body := b.get(context.Background(), s.force)
		if n != s.want {
			t.Fatalf("step %d: asked %d times, want %d", i, n, s.want)
		}
		var res prsResult
		if err := json.Unmarshal(body, &res); err != nil || !res.On || res.Fetched == 0 || res.Items == nil {
			t.Fatalf("step %d: body %s: %v", i, body, err)
		}
	}
	if len(logged) != 1 {
		t.Errorf("logged %d lines, want 1 (the failure)", len(logged))
	}
}

// /prs は --prs のときだけある。付けなければ 404 で、gh を起動せず、画面にも「プルリクエスト」の表示を出す印（meta の prs）を渡さない。
// 「更新」（POST）は、/archive と同じく、画面だけが付ける X-Kiroku ヘッダーと同じオリジンを求める（ほかのサイトのページから問い合わせさせない）。
func TestServePRs(t *testing.T) {
	logw = io.Discard
	defer func() { logw = os.Stderr }()
	ts := 1000.0
	sb := core.NewBuilder("Claude Code", "s1")
	sb.Tick(&ts)
	sb.Prompt(&ts, "依頼")
	s := sb.Finish(15)
	asked := 0
	mk := func(on bool) (*live, *httptest.Server) {
		l := &live{load: func() snapshot { return snapshot{data: []*core.Session{s}, meta: map[string]any{}, gen: 1} }, print: io.Discard}
		if on {
			l.prs = newPRBoard(l.logf)
			l.prs.fetch = func(context.Context) (prsResult, error) {
				asked++
				return prsResult{On: true, Items: []prItem{{URL: "https://github.com/zzowner/zzrepo/pull/1", Repo: "zzowner/zzrepo", Number: 1, Title: "t", Why: []string{"author"}}}}, nil
			}
		}
		if err := l.refresh(); err != nil {
			t.Fatal(err)
		}
		l.markReady()
		srv := httptest.NewServer(secureHeaders(sameOrigin("127.0.0.1:8484", nil, l.handler())))
		t.Cleanup(srv.Close)
		return l, srv
	}
	const host = "localhost:8484"
	do := func(srv *httptest.Server, method string, hdr map[string]string) (int, string, http.Header) {
		req, _ := http.NewRequest(method, srv.URL+"/prs", nil)
		req.Host = host
		for k, v := range hdr {
			req.Header.Set(k, v)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(b), resp.Header
	}

	off, srvOff := mk(false)
	for _, m := range []string{"GET", "POST"} {
		if code, _, _ := do(srvOff, m, map[string]string{"X-Kiroku": "1", "Origin": "http://" + host}); code != http.StatusNotFound {
			t.Errorf("without --prs: %s /prs = %d, want 404", m, code)
		}
	}
	if _, has := off.snap.meta["prs"]; has || strings.Contains(string(off.json), `"prs":true`) {
		t.Error("without --prs: the view is told to show the pull request page")
	}

	on, srvOn := mk(true)
	if !strings.Contains(string(on.json), `"prs":true`) {
		t.Error("with --prs: data.json lacks the prs mark")
	}
	if strings.Contains(string(on.html), "zzowner") {
		t.Error("the keyless shell has pull requests in it")
	}
	if asked != 0 {
		t.Errorf("asked GitHub %d times before the list was opened", asked)
	}
	code, body, h := do(srvOn, "GET", nil)
	if code != 200 || !strings.Contains(body, `"url":"https://github.com/zzowner/zzrepo/pull/1"`) || h.Get("Cache-Control") != "no-store" || !strings.HasPrefix(h.Get("Content-Type"), "application/json") {
		t.Errorf("GET /prs = %d %s %v", code, body, h)
	}
	checkHeaders(t, "/prs", h)
	for _, hdr := range []map[string]string{nil, {"X-Kiroku": "1"}, {"Origin": "http://" + host}, {"X-Kiroku": "1", "Origin": "http://evil.example"}, {"X-Kiroku": "1", "Origin": "https://" + host}} {
		if code, _, _ := do(srvOn, "POST", hdr); code != http.StatusForbidden {
			t.Errorf("POST /prs with %v = %d, want 403", hdr, code)
		}
	}
	if code, _, _ := do(srvOn, "POST", map[string]string{"X-Kiroku": "1", "Origin": "http://" + host}); code != 200 {
		t.Errorf("POST /prs from the view = %d", code)
	}
	for _, m := range []string{"PUT", "DELETE"} {
		if code, _, _ := do(srvOn, m, map[string]string{"X-Kiroku": "1", "Origin": "http://" + host}); code != http.StatusMethodNotAllowed {
			t.Errorf("%s /prs = %d, want 405", m, code)
		}
	}
	if asked != 1 {
		t.Errorf("asked GitHub %d times, want 1", asked)
	}
	// よそのホスト名（DNS リバインディング）からは読めない
	req, _ := http.NewRequest("GET", srvOn.URL+"/prs", nil)
	req.Host = "evil.example:8484"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("GET /prs with another Host = %d, want 403", resp.StatusCode)
	}
}

// /prs は鍵がなければ読めない（ほかの利用者に、自分の PR の一覧を見せない。鍵なしで返すのは / とマニフェストだけ）。
func TestServePRsNeedsKey(t *testing.T) {
	l := &live{load: func() snapshot { return snapshot{meta: map[string]any{}, gen: 1} }, print: io.Discard}
	asked := 0
	l.prs = newPRBoard(nil)
	l.prs.fetch = func(context.Context) (prsResult, error) { asked++; return prsResult{On: true, Items: []prItem{}}, nil }
	if err := l.refresh(); err != nil {
		t.Fatal(err)
	}
	l.markReady()
	key := strings.Repeat("ab", 32)
	srv := httptest.NewServer(requireKey("127.0.0.1:8484", key, "", l.handler()))
	defer srv.Close()
	get := func(k string) int {
		req, _ := http.NewRequest("GET", srv.URL+"/prs", nil)
		if k != "" {
			req.Header.Set("X-Kiroku-Key", k)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	if code := get(""); code != http.StatusForbidden || asked != 0 {
		t.Errorf("without the key: %d, asked %d times", code, asked)
	}
	if code := get(strings.Repeat("cd", 32)); code != http.StatusForbidden || asked != 0 {
		t.Errorf("with a wrong key: %d, asked %d times", code, asked)
	}
	if code := get(key); code != 200 || asked != 1 {
		t.Errorf("with the key: %d, asked %d times", code, asked)
	}
}
