package web

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

var diagnosticVersion = regexp.MustCompile(`^[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}$`)
var diagnosticProvider = regexp.MustCompile(`^(default|[A-Za-z0-9_.$]+/[A-Za-z0-9_.$]+)$`)

// Only structured Android failure metadata is accepted: no audio or transcript.
// This is device-authenticated, rate-limited and never changes household data.
func (s *Server) voiceDiagnostic(w http.ResponseWriter, r *http.Request) {
	if strings.Split(r.Header.Get("Content-Type"), ";")[0] != "application/json" {
		s.problem(w, 415, "Brug JSON til fejlkoden.")
		return
	}
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1024))
	if err != nil {
		s.problem(w, 413, "Fejlrapporten er for lang.")
		return
	}
	var input struct {
		Version  string `json:"version"`
		Provider string `json:"provider"`
		Phase    string `json:"phase"`
		Code     *int   `json:"code"`
		SDK      *int   `json:"sdk"`
		Ready    *bool  `json:"ready"`
		Heard    *bool  `json:"heard"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil || decoder.Decode(new(any)) != io.EOF || !diagnosticVersion.MatchString(input.Version) || len(input.Provider) > 240 || !diagnosticProvider.MatchString(input.Provider) || input.Code == nil || *input.Code < 1 || *input.Code > 101 || input.SDK == nil || *input.SDK < 23 || *input.SDK > 100 || input.Ready == nil || input.Heard == nil {
		s.problem(w, 400, "Ugyldig fejlrapport.")
		return
	}
	switch input.Phase {
	case "start", "recognition", "readiness_timeout", "result_timeout":
	default:
		s.problem(w, 400, "Ugyldig fejlfase.")
		return
	}
	digest := sha256.Sum256([]byte(r.Header.Get("Authorization")))
	if !s.diagnosticLimiter.Allow(hex.EncodeToString(digest[:]), time.Now()) {
		s.problem(w, 429, "For mange fejlrapporter.")
		return
	}
	s.logger.Warn("android speech recognition failed", "version", input.Version, "sdk", *input.SDK,
		"provider", input.Provider, "phase", input.Phase, "code", *input.Code, "ready", *input.Ready, "heard", *input.Heard)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}
