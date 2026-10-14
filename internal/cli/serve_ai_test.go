package cli

import (
	"encoding/json"
	"flag"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MichinaoShimizu/kiroku/internal/ai"
)

// With AI execution off (kiroku serve --no-ai, or an address that isn't a loopback IP), /ai/run doesn't exist (ADR 0034).
func TestAIEndpointNotFoundWhenOff(t *testing.T) {
	l := &live{}
	r := httptest.NewRequest(http.MethodPost, "http://localhost:8485/ai/run", strings.NewReader(`{"agent":"claude","prompt":"hello","consent":true}`))
	r.Header.Set("X-Kiroku", "1")
	w := httptest.NewRecorder()
	l.handler().ServeHTTP(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", w.Code)
	}
}

// AI execution runs only on a loopback address written as an IP. Public binds, host names and shorthand IPv4 forms
// (the resolver could map them anywhere) keep it off, even though AI is on by default.
func TestAIOnlyOnLoopbackIP(t *testing.T) {
	for _, addr := range []string{"0.0.0.0:8485", "[::]:8485", "[::ffff:0.0.0.0]:8485", "localhost:8485", "127.1:8485", "2130706433:8485", "example.com:8485", "192.168.1.2:8485"} {
		if aiAddrOK(addr) {
			t.Errorf("%s: AI execution allowed, want it off", addr)
		}
	}
	for _, addr := range []string{":8485", "127.0.0.1:8484", "[::1]:8485", "127.0.0.2:8485"} {
		if !aiAddrOK(addr) {
			t.Errorf("%s: AI execution refused, want it on", addr)
		}
	}
}

// The startup line says whether agent runs are on, how to turn them off and that the agent may send data to its provider (ADR 0034).
func TestAIStartLine(t *testing.T) {
	on := aiStartLine(true, true)
	for _, s := range []string{"is on", "--no-ai", "may send the prompt and the history it reads to its provider"} {
		if !strings.Contains(on, s) {
			t.Errorf("start line %q lacks %q", on, s)
		}
	}
	if s := aiStartLine(false, true); !strings.Contains(s, "is off (--no-ai)") {
		t.Errorf("--no-ai start line = %q", s)
	}
	if s := aiStartLine(true, false); !strings.Contains(s, "is off") || !strings.Contains(s, "loopback") {
		t.Errorf("non-loopback start line = %q", s)
	}
}

// $KIROKU_NO_AI is the default of --no-ai: anything but empty, 0 or false turns AI execution off.
func TestNoAIEnv(t *testing.T) {
	for v, want := range map[string]bool{"": false, " ": false, "0": false, " 0": false, "false": false, "FALSE": false, "False": false, "1": true, "1 ": true, "true": true, "yes": true, "no": true, "off": true} {
		t.Setenv("KIROKU_NO_AI", v)
		if got := noAIEnv(); got != want {
			t.Errorf("KIROKU_NO_AI=%q: noAIEnv() = %v, want %v", v, got, want)
		}
	}
}

// The view learns from meta.aiRun whether to offer "Analyze with AI"; it is set unless AI execution is off.
func TestAIRunFlagInMeta(t *testing.T) {
	for _, on := range []bool{false, true} {
		l := &live{load: func() snapshot { return snapshot{meta: map[string]any{"x": 1}} }}
		if on {
			l.aiRun = ai.NewHandler(ai.Runner{Enabled: true})
		}
		if err := l.refresh(); err != nil {
			t.Fatal(err)
		}
		var j struct {
			Meta map[string]any `json:"meta"`
		}
		if err := json.Unmarshal(l.json, &j); err != nil {
			t.Fatal(err)
		}
		if got := j.Meta["aiRun"] == true; got != on {
			t.Errorf("aiRun in meta = %v, want %v", got, on)
		}
		if j.Meta["x"] == nil || !strings.Contains(string(l.html), `"aiRun":true`) == on {
			t.Errorf("meta lost other keys or HTML disagrees (on=%v)", on)
		}
	}
}

// --lang に書いた値が使えなければ止まる。$KIROKU_LANG の値が使えないときは無視して、ブラウザの言語に任せる
func TestLangFlag(t *testing.T) {
	for _, v := range []string{"fr", "ja-JP", `"><script>`, "__proto__"} {
		if err := checkLang(v); err == nil {
			t.Errorf("checkLang(%q) = nil, want an error", v)
		}
	}
	for _, v := range []string{"", "en", "ja"} {
		if err := checkLang(v); err != nil {
			t.Errorf("checkLang(%q) = %v", v, err)
		}
	}
	t.Setenv("KIROKU_LANG", "fr")
	if got := *langFlag(flag.NewFlagSet("x", flag.ContinueOnError)); got != "" {
		t.Errorf("KIROKU_LANG=fr gave %q, want it ignored", got)
	}
	t.Setenv("KIROKU_LANG", "ja")
	if got := *langFlag(flag.NewFlagSet("x", flag.ContinueOnError)); got != "ja" {
		t.Errorf("KIROKU_LANG=ja gave %q", got)
	}
}

// 以前の --ai=false（切る）は、--no-ai と同じに扱う。指定がなければ既定のまま。
func TestOldAIFalseTurnsOff(t *testing.T) {
	for args, want := range map[string]bool{"": false, "--ai=false": true, "--ai": false, "--ai=true": false} {
		fs := flag.NewFlagSet("t", flag.ContinueOnError)
		fs.Bool("ai", false, "")
		var a []string
		if args != "" {
			a = []string{args}
		}
		if err := fs.Parse(a); err != nil {
			t.Fatal(err)
		}
		on := fs.Lookup("ai").Value.String() == "true"
		if got := !on && flagSet(fs, "ai"); got != want {
			t.Errorf("%q: off = %v, want %v", args, got, want)
		}
	}
}
