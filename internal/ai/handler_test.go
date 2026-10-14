package ai

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlerDisabledByDefault(t *testing.T) {
	h := Handler{Runner: Runner{Enabled: false}}
	r := httptest.NewRequest(http.MethodPost, "http://localhost:8485/ai/run", strings.NewReader(`{"agent":"claude","prompt":"hello","consent":true}`))
	r.Header.Set("X-Kiroku", "1")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatalf("status %d", w.Code)
	}
}

func TestHandlerRejectsMissingCSRFHeader(t *testing.T) {
	h := Handler{Runner: Runner{Enabled: true}}
	r := httptest.NewRequest(http.MethodPost, "http://localhost:8485/ai/run", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatalf("status %d", w.Code)
	}
}

func TestHandlerRequiresConsent(t *testing.T) {
	h := Handler{Runner: Runner{Enabled: true}}
	r := httptest.NewRequest(http.MethodPost, "http://localhost:8485/ai/run", strings.NewReader(`{"agent":"claude","prompt":"hello"}`))
	r.Header.Set("X-Kiroku", "1")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 400 {
		t.Fatalf("status %d", w.Code)
	}
}

func TestHandlerRejectsForeignOrigin(t *testing.T) {
	h := Handler{Runner: Runner{Enabled: true}}
	r := httptest.NewRequest(http.MethodPost, "http://localhost:8485/ai/run", strings.NewReader(`{}`))
	r.Header.Set("X-Kiroku", "1")
	r.Header.Set("Origin", "https://evil.example")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatalf("status %d", w.Code)
	}
}

func TestHandlerRejectsUnknownFields(t *testing.T) {
	h := Handler{Runner: Runner{Enabled: true}}
	r := httptest.NewRequest(http.MethodPost, "http://localhost:8485/ai/run", strings.NewReader(`{"agent":"claude","prompt":"hi","consent":true,"command":"rm"}`))
	r.Header.Set("X-Kiroku", "1")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 400 {
		t.Fatalf("status %d", w.Code)
	}
}

func TestHandlerRejectsTrailingJSON(t *testing.T) {
	h := Handler{Runner: Runner{Enabled: true}}
	r := httptest.NewRequest(http.MethodPost, "http://localhost:8485/ai/run", strings.NewReader(`{"agent":"claude","prompt":"hi","consent":true} {}`))
	r.Header.Set("X-Kiroku", "1")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 400 {
		t.Fatalf("status %d", w.Code)
	}
}

func TestHandlerRejectsOversizedBody(t *testing.T) {
	h := Handler{Runner: Runner{Enabled: true}}
	body := `{"agent":"claude","consent":true,"prompt":"` + strings.Repeat("a", 264000) + `"}`
	r := httptest.NewRequest(http.MethodPost, "http://localhost:8485/ai/run", strings.NewReader(body))
	r.Header.Set("X-Kiroku", "1")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d", w.Code)
	}
}

func TestHandlerRejectsGet(t *testing.T) {
	h := Handler{Runner: Runner{Enabled: true}}
	r := httptest.NewRequest(http.MethodGet, "http://localhost:8485/ai/run", nil)
	r.Header.Set("X-Kiroku", "1")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status %d", w.Code)
	}
}
func TestHandlerRejectsConcurrentRun(t *testing.T) {
	h := NewHandler(Runner{Enabled: true})
	h.Slots <- struct{}{}
	r := httptest.NewRequest(http.MethodPost, "http://localhost:8485/ai/run", strings.NewReader(`{"agent":"claude","prompt":"hello","consent":true}`))
	r.Header.Set("X-Kiroku", "1")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("status %d", w.Code)
	}
}

// A run streams one JSON object per line: the command, what the agent writes, and a last "done" line
// with the result. A failing agent ends with ok false and its message.
func TestHandlerStreams(t *testing.T) {
	stubAgent(t, "codex", `cat >/dev/null; echo "working" >&2; echo "<b>hi</b>"`)
	h := NewHandler(Runner{Enabled: true})
	r := httptest.NewRequest(http.MethodPost, "http://localhost:8485/ai/run", strings.NewReader(`{"agent":"codex","prompt":"hello","consent":true}`))
	r.Header.Set("X-Kiroku", "1")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 || w.Header().Get("Content-Type") != "application/x-ndjson" {
		t.Fatalf("status %d, type %q", w.Code, w.Header().Get("Content-Type"))
	}
	lines := strings.Split(strings.TrimSpace(w.Body.String()), "\n")
	if len(lines) != 4 || !strings.HasPrefix(lines[0], `{"t":"cmd","argv":["codex"`) || !strings.HasSuffix(lines[3], `"ok":true,"output":"\u003cb\u003ehi\u003c/b\u003e"}`) {
		t.Fatalf("body:\n%s", w.Body.String())
	}
	if strings.Contains(w.Body.String(), "<b>") {
		t.Error("< and > must be escaped in the stream")
	}

	stubAgent(t, "codex", `cat >/dev/null; echo "Not logged in" >&2; exit 3`)
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodPost, "http://localhost:8485/ai/run", strings.NewReader(`{"agent":"codex","prompt":"hello","consent":true}`))
	r.Header.Set("X-Kiroku", "1")
	h.ServeHTTP(w, r)
	last := strings.TrimSpace(w.Body.String())
	last = last[strings.LastIndex(last, "\n")+1:]
	if !strings.Contains(last, `"ok":false`) || !strings.Contains(last, "Not logged in") {
		t.Errorf("last line %q, want ok false with the agent's message", last)
	}
}

// An unknown agent is refused before anything runs.
func TestHandlerRejectsUnknownAgent(t *testing.T) {
	h := NewHandler(Runner{Enabled: true})
	r := httptest.NewRequest(http.MethodPost, "http://localhost:8485/ai/run", strings.NewReader(`{"agent":"sh","prompt":"hello","consent":true}`))
	r.Header.Set("X-Kiroku", "1")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 400 {
		t.Fatalf("status %d", w.Code)
	}
}
