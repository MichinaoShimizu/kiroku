package cli

import (
	"encoding/json"
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
	err := serveLiveWithAI("0.0.0.0:8485", nil, 0, nil, nil, nil, false, true)
	if err == nil {
		t.Fatal("expected AI execution to reject public listener")
	}
}

func TestAIRejectsHostnameBind(t *testing.T) {
	err := serveLiveWithAI("localhost:8485", nil, 0, nil, nil, nil, false, true)
	if err == nil {
		t.Fatal("expected numeric loopback requirement")
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
