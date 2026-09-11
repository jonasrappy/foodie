package web

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestSpeechDiagnosticBoundaryAndRateLimit(t *testing.T) {
	s, server, f := testServer(t)
	var log bytes.Buffer
	s.logger = slog.New(slog.NewJSONHandler(&log, nil))
	url := server.URL + "/api/voice/diagnostic"
	valid := `{"version":"1.2.1","sdk":34,"provider":"com.example/com.example.RecognitionService","phase":"recognition","code":12,"ready":true,"heard":true}`
	for _, tc := range []struct {
		token string
		want  int
	}{{"", 401}, {f.Bot, 403}} {
		if code, _ := request(t, url, "POST", valid, tc.token); code != tc.want {
			t.Fatal("diagnostics require device auth", code)
		}
	}
	for _, invalid := range []string{`{}`, strings.TrimSuffix(valid, "}") + `,"text":"private groceries"}`, strings.Replace(valid, `"code":12`, `"code":null`, 1), strings.Replace(valid, `"phase":"recognition"`, `"phase":"free text"`, 1)} {
		if code, _ := request(t, url, "POST", invalid, f.Token); code != 400 {
			t.Fatal("invalid diagnostic accepted", code)
		}
	}
	if log.Len() != 0 {
		t.Fatal("invalid or unauthorized data reached diagnostic logs")
	}
	for i := 0; i < 15; i++ {
		if code, _ := request(t, url, "POST", valid, f.Token); code != 204 {
			t.Fatal("diagnostic rejected", code)
		}
	}
	if code, _ := request(t, url, "POST", valid, f.Token); code != 429 {
		t.Fatal("diagnostics were not rate limited", code)
	}
	if !strings.Contains(log.String(), `"code":12`) || strings.Contains(log.String(), f.Token) || strings.Contains(log.String(), "private groceries") {
		t.Fatal("incorrect diagnostic log content")
	}
	state, err := s.store.Snapshot(t.Context())
	if err != nil || state.Revision != 0 || len(state.Shopping) != 0 {
		t.Fatal("diagnostics changed household data")
	}
}
