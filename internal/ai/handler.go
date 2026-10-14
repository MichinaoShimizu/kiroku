package ai

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os/exec"
	"strings"
)

// Handler provides a constrained opt-in API. It must be mounted behind
// kiroku serve's existing authentication and same-origin protections.
type Handler struct {
	Runner Runner
	Slots  chan struct{}
}

// NewHandler limits concurrent agent invocations. A zero-value Handler remains disabled.
func NewHandler(r Runner) Handler {
	return Handler{Runner: r, Slots: make(chan struct{}, 1)}
}

type request struct {
	Agent   string `json:"agent"`
	Prompt  string `json:"prompt"`
	Consent bool   `json:"consent"`
}

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost || r.Header.Get("X-Kiroku") != "1" || (r.Header.Get("Origin") != "" && r.Header.Get("Origin") != "http://"+r.Host) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if !h.Runner.Enabled {
		http.Error(w, "AI execution disabled", http.StatusForbidden)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 256*1024+1024)
	defer r.Body.Close()
	var in request
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&in); err != nil {
		if tooBig := (*http.MaxBytesError)(nil); errors.As(err, &tooBig) { // 何が悪いかわかるように（"invalid request" ではなく）
			http.Error(w, "the prompt is too long to run (limit 256 KB); copy it and paste it into the agent instead", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		http.Error(w, "unexpected trailing data", http.StatusBadRequest)
		return
	}
	if !in.Consent || strings.TrimSpace(in.Prompt) == "" {
		http.Error(w, "consent and prompt required", http.StatusBadRequest)
		return
	}
	if h.Slots != nil {
		select {
		case h.Slots <- struct{}{}:
			defer func() { <-h.Slots }()
		default:
			http.Error(w, "AI execution already running", http.StatusTooManyRequests)
			return
		}
	}
	binary, _, err := h.Runner.command(in.Agent)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if _, err := exec.LookPath(binary); err != nil {
		http.Error(w, binary+" is not installed on this computer", http.StatusBadRequest)
		return
	}
	// The run streams one JSON object per line (application/x-ndjson): the command, then what the agent
	// writes and does as it happens, then {"t":"done"} with the result or the error. The status is 200
	// once streaming starts, so a failure is reported in the last line.
	w.Header().Set("Content-Type", "application/x-ndjson")
	rc := http.NewResponseController(w)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(true)
	write := func(v any) {
		_ = enc.Encode(v)
		_ = rc.Flush()
	}
	result, err := h.Runner.Stream(r.Context(), in.Agent, in.Prompt, func(e Event) { write(e) })
	if err != nil { // the agent's own message (such as "not logged in") tells the person what to fix
		write(done{T: "done", Agent: in.Agent, Error: "AI execution failed: " + err.Error()})
		return
	}
	write(done{T: "done", Agent: result.Agent, OK: true, Output: result.Output})
}

// done is the last line of a run.
type done struct {
	T      string `json:"t"`
	Agent  string `json:"agent"`
	OK     bool   `json:"ok"`
	Output string `json:"output,omitempty"`
	Error  string `json:"error,omitempty"`
}
