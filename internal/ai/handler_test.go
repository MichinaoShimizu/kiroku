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
