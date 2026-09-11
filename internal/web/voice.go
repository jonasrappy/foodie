package web

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/jonasrappy/foodie/internal/i18n"
	"github.com/jonasrappy/foodie/internal/store"
	"github.com/jonasrappy/foodie/internal/voice"
	"io"
	"net/http"
	"strings"
)

func (s *Server) voiceCommand(w http.ResponseWriter, r *http.Request) {
	select {
	case s.voiceSlots <- struct{}{}:
		defer func() { <-s.voiceSlots }()
	default:
		s.problem(w, 429, "Jeg er optaget. Prøv igen om lidt.")
		return
	}
	if strings.Split(r.Header.Get("Content-Type"), ";")[0] != "application/json" {
		s.problem(w, 415, "Brug en talekommando som tekst.")
		return
	}
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 4096))
	if err != nil {
		s.problem(w, 413, "Kommandoen er for lang.")
		return
	}
	var input struct {
		Text string `json:"text"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil || decoder.Decode(new(any)) != io.EOF || len(input.Text) > 2000 || strings.TrimSpace(input.Text) == "" {
		s.problem(w, 400, "Ugyldig talekommando.")
		return
	}
	id, confirmation := r.Header.Get("X-Request-ID"), r.Header.Get("X-Voice-Confirmation")
	hash := sha256.Sum256(append([]byte(confirmation+"\x00"), data...))
	fingerprint := hex.EncodeToString(hash[:])
	transcript, found, err := s.store.VoiceTranscript(r.Context(), id, fingerprint, nil)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if !found {
		transcript, _, err = s.store.VoiceTranscript(r.Context(), id, fingerprint, &input.Text)
		if err != nil {
			s.fail(w, r, err)
			return
		}
	}
	var command *voice.Command
	var answer *bool
	intentID := id
	if confirmation != "" {
		intentID = confirmation
		if yes, clear := voice.Confirmation(transcript); clear {
			answer = &yes
		} else {
			respond(w, 200, store.VoiceResponse{Kind: "confirm", Speech: i18n.Text(s.language, "Vil du tilføje det ekstra? Sig ja eller nej."), ConfirmationID: confirmation})
			return
		}
	} else {
		parsed, parseErr := voice.Parse(transcript)
		if errors.Is(parseErr, voice.ErrCancelled) {
			respond(w, 200, store.VoiceResponse{Kind: "cancelled", Speech: i18n.Text(s.language, parseErr.Error())})
			return
		}
		if parseErr != nil {
			respond(w, 200, store.VoiceResponse{Kind: "retry", Speech: i18n.Text(s.language, parseErr.Error())})
			return
		}
		command = &parsed
	}
	reply, state, err := s.store.ApplyVoiceLocalized(r.Context(), intentID, id, command, answer, s.language)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.hub.publish(state)
	respond(w, 200, reply)
}
