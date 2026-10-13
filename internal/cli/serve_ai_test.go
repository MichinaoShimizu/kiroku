package cli

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
