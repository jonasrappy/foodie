package store

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/jonasrappy/foodie/internal/units"
	"github.com/jonasrappy/foodie/internal/voice"
)

func TestEnglishUnitMigrationPreservesDataAndPendingConfirmations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mad.sqlite")
	db, err := openDB(path, "rwc")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(schema + migration002 + migration003 + migration004); err != nil {
		t.Fatal(err)
	}
	oldUnits := []string{"stk.", "liter", "milliliter", "kilo", "gram", "pakker", "poser", "dåser", "flasker", "bundter", "bakker", "kasser"}
	for i, unit := range oldUnits {
		if _, err = db.Exec(`INSERT INTO items(rowid,id,kind,text,checked,version,created_at,updated_at,quantity,unit) VALUES(?,?,'shopping',?,?,7,'same-time','edited-time',2,?)`, 100-i, unit, "product "+unit, (i+1)%2, unit); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = db.Exec(`INSERT INTO items VALUES('meal','meals','family recipe',0,3,'same-time','same-time',1,'stk.'); UPDATE meta SET revision=12,updated_at='meta-time'; INSERT INTO requests VALUES('old-request-0000001','{"kind":"shopping","texts":["product pakker"],"quantity":2,"unit":"pakker"}')`); err != nil {
		t.Fatal(err)
	}
	old := &Store{db: db}
	before, err := old.Snapshot(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	archived, _ := json.Marshal(before)
	if _, err = db.Exec(`INSERT INTO resets VALUES(8,'archive-time',6,?)`, string(archived)); err != nil {
		t.Fatal(err)
	}
	command := voice.Command{Text: "product pakker", Quantity: 1, Unit: "pakker"}
	encoded, _ := json.Marshal(command)
	_, signature := voiceMatches(before, command)
	question := VoiceResponse{Kind: "confirm", Speech: "a previously spoken question", ConfirmationID: "old-voice-000000001"}
	cached, _ := json.Marshal(question)
	if _, err = db.Exec(`INSERT INTO voice_intents VALUES(?,?,?, ?,0,unixepoch())`, question.ConfirmationID, string(encoded), signature, string(cached)); err != nil {
		t.Fatal(err)
	}
	// A stale question must remain stale across the unit migration.
	if _, err = db.Exec(`INSERT INTO voice_intents VALUES('stale-voice-0000001',?,'stale-signature',?,0,unixepoch())`, string(encoded), string(cached)); err != nil {
		t.Fatal(err)
	}
	transcript := "en pakke product pakker"
	if _, _, err = old.VoiceTranscript(t.Context(), question.ConfirmationID, "original-fingerprint", &transcript); err != nil {
		t.Fatal(err)
	}
	db.Close()
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	expected := before
	expected.Shopping = append([]Item{}, before.Shopping...)
	expected.Meals = append([]Item{}, before.Meals...)
	for _, rows := range [][]Item{expected.Shopping, expected.Meals} {
		for i := range rows {
			rows[i].Unit = units.NormalizeLegacy(rows[i].Unit)
		}
	}
	after, err := s.Snapshot(t.Context())
	if err != nil || !reflect.DeepEqual(expected, after) {
		t.Fatal("migration changed user data, ordering, quantities, checked state or versions", err)
	}
	var snapshotText string
	if err = s.db.QueryRow("SELECT snapshot FROM resets WHERE id=8").Scan(&snapshotText); err != nil {
		t.Fatal(err)
	}
	var archive State
	if err = json.Unmarshal([]byte(snapshotText), &archive); err != nil || !reflect.DeepEqual(expected, archive) {
		t.Fatal("archive migration changed user content", err)
	}
	s.Close()
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	after, err = s.Snapshot(t.Context())
	if err != nil || !reflect.DeepEqual(expected, after) {
		t.Fatal("reopening repeated the migration", err)
	}
	// Both an old client's retry and a new client's retry keep the original request ID.
	for _, unit := range []string{"pakker", "pack"} {
		replay, err := s.Add(t.Context(), Add{Kind: "shopping", Texts: []string{"product pakker"}, Quantity: pointer(2.0), Unit: &unit, RequestID: "old-request-0000001"})
		if err != nil || !reflect.DeepEqual(expected, replay) {
			t.Fatal("migration broke addition idempotency", err)
		}
	}
	yes := true
	reply, state, err := s.ApplyVoice(t.Context(), question.ConfirmationID, "after-migrate-yes-01", nil, &yes)
	if err != nil || reply.Kind != "added" || state.Revision != 13 {
		t.Fatal("valid pending voice confirmation lost", reply, err)
	}
	reply, state, err = s.ApplyVoice(t.Context(), "stale-voice-0000001", "after-stale-yes-001", nil, &yes)
	if err != nil || reply.Kind != "confirm" || state.Revision != 13 {
		t.Fatal("stale confirmation became valid", reply, err)
	}
	if got, found, err := s.VoiceTranscript(t.Context(), question.ConfirmationID, "original-fingerprint", nil); err != nil || !found || got != transcript {
		t.Fatal("transcript changed", err)
	}
	for _, assignment := range []string{"unit='invalid'", "unit='pakker'", "quantity=0"} {
		if _, err = s.db.Exec("UPDATE items SET " + assignment + " WHERE id='pakker'"); err == nil {
			t.Fatal("database constraints lost")
		}
	}
	var integrity string
	if err = s.db.QueryRow("PRAGMA integrity_check").Scan(&integrity); err != nil || integrity != "ok" {
		t.Fatal(integrity, err)
	}
}
