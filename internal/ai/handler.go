package ai

import (
 "encoding/json"
 "io"
 "net/http"
 "strings"
)

// Handler provides a constrained opt-in API. It must be mounted behind
// kiroku serve's existing authentication and same-origin protections.
// No handler is mounted by default.
type Handler struct { Runner Runner }

type request struct {
 Agent string `json:"agent"`
 Prompt string `json:"prompt"`
 Consent bool `json:"consent"`
}

func (h Handler) ServeHTTP(w http.ResponseWriter,r *http.Request) {
 w.Header().Set("Cache-Control","no-store")
 w.Header().Set("Content-Type","application/json")
 if r.Method!=http.MethodPost || r.Header.Get("X-Kiroku")!="1" || (r.Header.Get("Origin")!="" && r.Header.Get("Origin")!="http://"+r.Host) {
  http.Error(w,"forbidden",http.StatusForbidden);return
 }
 if !h.Runner.Enabled { http.Error(w,"AI execution disabled",http.StatusForbidden);return }
 r.Body=http.MaxBytesReader(w,r.Body,256*1024+1024)
 defer r.Body.Close()
 var in request
 if err:=json.NewDecoder(r.Body).Decode(&in);err!=nil { http.Error(w,"invalid request",http.StatusBadRequest);return }
 if !in.Consent || strings.TrimSpace(in.Prompt)=="" { http.Error(w,"consent and prompt required",http.StatusBadRequest);return }
 // One request per invocation, no arbitrary shell args or filesystem paths.
 result,err:=h.Runner.Run(r.Context(),in.Agent,in.Prompt)
 if err!=nil { http.Error(w,"AI execution failed",http.StatusBadGateway);return }
 _=json.NewEncoder(w).Encode(result)
}

// DecodeRequest is intentionally private; never accept executable arguments from the browser.
var _ io.Reader
