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

