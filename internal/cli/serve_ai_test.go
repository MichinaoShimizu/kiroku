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

func TestAIEndpointDisabledByDefault(t *testing.T) {
	l := &live{}
	r := httptest.NewRequest(http.MethodPost, "http://localhost:8485/ai/run", strings.NewReader(`{"agent":"claude","prompt":"hello","consent":true}`))
	r.Header.Set("X-Kiroku", "1")
	w := httptest.NewRecorder()
	l.handler().ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("want forbidden, got %d", w.Code)
	}
}

func TestAIRejectsPublicBind(t *testing.T) {
	for _, addr := range []string{"0.0.0.0:8485", "[::]:8485", "[::ffff:0.0.0.0]:8485"} {
		if err := serveLiveWithAI(addr, nil, 0, nil, nil, nil, false, true, "", nil); err == nil || !strings.Contains(err.Error(), "loopback") {
			t.Errorf("%s: err = %v, want the loopback requirement", addr, err)
		}
	}
}

// Host names and shorthand IPv4 forms are refused before listening, as the resolver could map them anywhere.
func TestAIRejectsHostnameBind(t *testing.T) {
	for _, addr := range []string{"localhost:8485", "127.1:8485", "2130706433:8485", "example.com:8485"} {
		if err := serveLiveWithAI(addr, nil, 0, nil, nil, nil, false, true, "", nil); err == nil || !strings.Contains(err.Error(), "loopback") {
			t.Errorf("%s: err = %v, want the loopback requirement", addr, err)
		}
	}
}

// The view learns from meta.aiRun whether to offer "Run with AI"; only kiroku serve --ai sets it.
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
