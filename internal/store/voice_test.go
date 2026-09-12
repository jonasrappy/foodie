package store

import (
	"context"
	"encoding/json"
	"github.com/jonasrappy/foodie/internal/voice"
	"strings"
	"sync"
	"testing"
)

func cmd(text string, q float64, unit string) *voice.Command {
	return &voice.Command{Text: text, Quantity: q, Unit: unit}
}
func TestVoiceAdditionAndDuplicateDialog(t *testing.T) {
	s := testStore(t)
	ctx := t.Context()
	id := "voice-new-0000000001"
	reply, state, err := applyDanishVoice(s, ctx, id, id, cmd("vindruer", 3, "piece"), nil)
	if err != nil || reply.Speech != "Tilføjet 3 stk. vindruer til indkøbslisten." || len(state.Shopping) != 1 {
		t.Fatal(reply, state, err)
	}
	original := state.Revision
	_, state, err = applyDanishVoice(s, ctx, id, id, nil, nil)
	if err != nil || state.Revision != original {
		t.Fatal("replay mutated", err)
	}
	id = "voice-duplicate-000001"
	reply, state, err = applyDanishVoice(s, ctx, id, id, cmd("VINDRUER", 1, "piece"), nil)
	if err != nil || reply.Kind != "confirm" || !strings.Contains(reply.Speech, "allerede 3 stk.") || !strings.Contains(reply.Speech, "bliver 4 stk.") || state.Revision != original {
		t.Fatal(reply, state, err)
	}
	yes := true
	reply, state, err = applyDanishVoice(s, ctx, id, "voice-answer-00000001", nil, &yes)
	if err != nil || reply.Kind != "added" || len(state.Shopping) != 1 || state.Shopping[0].Quantity != 4 {
		t.Fatal(reply, state, err)
	}
	_, again, err := applyDanishVoice(s, ctx, id, "voice-answer-00000001", nil, &yes)
	if err != nil || again.Revision != state.Revision || again.Shopping[0].Quantity != 4 {
		t.Fatal("duplicate answer", err)
	}
	id = "voice-decline-0000001"
	_, _, _ = applyDanishVoice(s, ctx, id, id, cmd("vindruer", 1, "piece"), nil)
	no := false
	reply, state, err = applyDanishVoice(s, ctx, id, "voice-no-answer-00001", nil, &no)
	if err != nil || reply.Kind != "cancelled" || state.Shopping[0].Quantity != 4 {
		t.Fatal(reply, state, err)
	}
}
func TestVoiceConfirmationMustMatchCurrentRowsAndAnswer(t *testing.T) {
	s := testStore(t)
	ctx := t.Context()
	_, state, _ := applyDanishVoice(s, ctx, "voice-seed-00000001", "voice-seed-00000001", cmd("vindruer", 3, "piece"), nil)
	id := "voice-pending-0000001"
	_, _, _ = applyDanishVoice(s, ctx, id, id, cmd("vindruer", 1, "piece"), nil)
	_, err := s.Edit(ctx, state.Shopping[0].ID, Edit{Version: state.Shopping[0].Version, Quantity: pointer(8.0)}, false)
	if err != nil {
		t.Fatal(err)
	}
	yes := true
	answer := "voice-answer-stale-01"
	reply, state, err := applyDanishVoice(s, ctx, id, answer, nil, &yes)
	if err != nil || reply.Kind != "confirm" || state.Shopping[0].Quantity != 8 || !strings.Contains(reply.Speech, "bliver 9") {
		t.Fatal(reply, state, err)
	}
	// Retrying the OLD yes must repeat the question, not confirm the updated quantity.
	replay, state, err := applyDanishVoice(s, ctx, id, answer, nil, &yes)
	if err != nil || replay != reply || state.Shopping[0].Quantity != 8 {
		t.Fatal("stale yes replay applied", replay, state, err)
	}
	reply, state, err = applyDanishVoice(s, ctx, id, "voice-fresh-answer-01", nil, &yes)
	if err != nil || reply.Kind != "added" || state.Shopping[0].Quantity != 9 {
		t.Fatal(reply, state, err)
	}
}
func TestVoiceCheckedUnitsAndReset(t *testing.T) {
	s := testStore(t)
	ctx := t.Context()
	_, state, _ := applyDanishVoice(s, ctx, "voice-seed-00000001", "voice-seed-00000001", cmd("vindruer", 3, "piece"), nil)
	state, _ = s.Edit(ctx, state.Shopping[0].ID, Edit{Version: 1, Checked: pointer(true)}, false)
	id := "voice-checked-000001"
	reply, _, err := applyDanishVoice(s, ctx, id, id, cmd("vindruer", 1, "piece"), nil)
	if err != nil || !strings.Contains(reply.Speech, "krydset af") {
		t.Fatal(reply, err)
	}
	yes := true
	_, state, err = applyDanishVoice(s, ctx, id, "voice-checked-yes-01", nil, &yes)
	if err != nil || len(state.Shopping) != 2 || !state.Shopping[0].Checked || state.Shopping[1].Checked {
		t.Fatal(state, err)
	}
	id = "voice-mixed-units-01"
	reply, _, err = applyDanishVoice(s, ctx, id, id, cmd("vindruer", 2, "tray"), nil)
	if err != nil || !strings.Contains(reply.Speech, "ny vare") {
		t.Fatal(reply, err)
	}
	// A Grok reset during a question must trigger a fresh question before adding anything.
	id = "voice-before-reset1"
	_, _, _ = applyDanishVoice(s, ctx, id, id, cmd("vindruer", 1, "piece"), nil)
	state, err = s.Reset(ctx, state.Revision)
	if err != nil {
		t.Fatal(err)
	}
	reply, state, err = applyDanishVoice(s, ctx, id, "voice-reset-answer1", nil, &yes)
	if err != nil || reply.Kind != "confirm" || len(state.Shopping) != 0 {
		t.Fatal(reply, state, err)
	}
}
func TestConcurrentVoiceAndDurableTranscript(t *testing.T) {
	s := testStore(t)
	ctx := t.Context()
	var wait sync.WaitGroup
	for i := 0; i < 8; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			if _, _, err := applyDanishVoice(s, ctx, "voice-race-000000001", "voice-race-000000001", cmd("mælk", 2, "liter"), nil); err != nil {
				t.Error(err)
			}
		}()
	}
	wait.Wait()
	state, _ := s.Snapshot(ctx)
	if len(state.Shopping) != 1 || state.Shopping[0].Quantity != 2 || state.Revision != 1 {
		t.Fatal(state)
	}
	text := "to liter mælk"
	if _, found, err := s.VoiceTranscript(ctx, "transcript-00000001", "hash1", nil); found || err != nil {
		t.Fatal(found, err)
	}
	_, _, _ = s.VoiceTranscript(ctx, "transcript-00000001", "hash1", &text)
	if got, found, err := s.VoiceTranscript(ctx, "transcript-00000001", "hash1", nil); got != text || !found || err != nil {
		t.Fatal(got, found, err)
	}
	if _, _, err := s.VoiceTranscript(ctx, "transcript-00000001", "hash2", nil); err == nil {
		t.Fatal("accepted reused ID with different audio")
	}
	blob, _ := json.Marshal(state)
	if strings.Contains(string(blob), "voiceToken") {
		t.Fatal("secret in state")
	}
}

func TestVoiceFractionalIncrement(t *testing.T) {
	s := testStore(t)
	_, _, err := applyDanishVoice(s, t.Context(), "fraction-seed-000001", "fraction-seed-000001", cmd("mælk", .1, "liter"), nil)
	if err != nil {
		t.Fatal(err)
	}
	reply, _, err := applyDanishVoice(s, t.Context(), "fraction-next-000001", "fraction-next-000001", cmd("mælk", .2, "liter"), nil)
	if err != nil || !strings.Contains(reply.Speech, "bliver 0,3 liter") {
		t.Fatal(reply, err)
	}
	yes := true
	_, state, err := applyDanishVoice(s, t.Context(), "fraction-next-000001", "fraction-answer-001", nil, &yes)
	if err != nil || state.Shopping[0].Quantity != .3 {
		t.Fatal(state, err)
	}
}

func applyDanishVoice(s *Store, ctx context.Context, id, replyID string, command *voice.Command, answer *bool) (VoiceResponse, State, error) {
	return s.ApplyVoiceLocalized(ctx, id, replyID, command, answer, "da")
}
