package store

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/jonasrappy/foodie/internal/voice"
)

func tableContents(t *testing.T, db *sql.DB) map[string]string {
	t.Helper()
	contents := make(map[string]string)
	for _, name := range []string{"items", "meta", "requests", "resets", "voice_transcripts", "voice_intents", "voice_replies"} {
		rows, err := db.Query("SELECT rowid,* FROM " + name + " ORDER BY rowid")
		if err != nil {
			t.Fatal(err)
		}
		columns, err := rows.Columns()
		if err != nil {
			t.Fatal(err)
		}
		data := [][]any{}
		for rows.Next() {
			values, targets := make([]any, len(columns)), make([]any, len(columns))
			for i := range values {
				targets[i] = &values[i]
			}
			if err := rows.Scan(targets...); err != nil {
				t.Fatal(err)
			}
			data = append(data, values)
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		rows.Close()
		encoded, err := json.Marshal(data)
		if err != nil {
			t.Fatal(err)
		}
		contents[name] = string(encoded)
	}
	return contents
}

func TestKasserMigrationPreservesDataAndPendingConfirmation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mad.sqlite")
	db, err := openDB(path, "rwc")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(schema + migration002 + migration003); err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
INSERT INTO items VALUES('b','shopping','pepsi max',0,7,'same-time','edited-time',2,'pakker');
INSERT INTO items VALUES('a','shopping','pepsi max',1,2,'same-time','earlier-time',1,'stk.');
INSERT INTO items VALUES('meal','meals','lasagne',0,1,'same-time','same-time',1,'stk.');
UPDATE items SET rowid=41 WHERE id='a';
UPDATE meta SET revision=12,updated_at='meta-time';
INSERT INTO requests VALUES('old-request-0000001','{"kind":"shopping","texts":["old"]}');
INSERT INTO resets VALUES(8,'archive-time',6,'{"revision":6,"shopping":[],"meals":[]}');`)
	if err != nil {
		t.Fatal(err)
	}
	old := &Store{db: db}
	command := voice.Command{Text: "pepsi max", Quantity: 1, Unit: "pakker"}
	text := "en pakke pepsi max"
	if _, _, err = old.VoiceTranscript(t.Context(), "old-voice-000000001", "original-fingerprint", &text); err != nil {
		t.Fatal(err)
	}
	question, _, err := old.ApplyVoice(t.Context(), "old-voice-000000001", "old-voice-000000001", &command, nil)
	if err != nil || question.Kind != "confirm" {
		t.Fatal(question, err)
	}
	before := tableContents(t, db)
	db.Close()
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if after := tableContents(t, s.db); !reflect.DeepEqual(before, after) {
		t.Fatal("migration changed existing values, rowids, order or cached requests")
	}
	s.Close()
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if !reflect.DeepEqual(before, tableContents(t, s.db)) {
		t.Fatal("reopening repeated or changed the migration")
	}
	yes := true
	reply, state, err := s.ApplyVoice(t.Context(), question.ConfirmationID, "after-migrate-yes-01", nil, &yes)
	if err != nil || reply.Kind != "added" || state.Shopping[0].Quantity != 3 {
		t.Fatal("pending voice confirmation was invalidated by migration", reply, state, err)
	}
	state, err = s.Add(t.Context(), Add{Kind: "shopping", Texts: []string{"squash"}, Quantity: pointer(1.0), Unit: pointer("kasser"), RequestID: "new-kasser-request1"})
	found := false
	for _, item := range state.Shopping {
		found = found || (item.Text == "squash" && item.Quantity == 1 && item.Unit == "kasser")
	}
	if err != nil || !found {
		t.Fatal("new unit cannot be saved after migration", state, err)
	}
	if _, err = s.db.Exec("UPDATE items SET unit='invalid' WHERE id='a'"); err == nil {
		t.Fatal("unit constraint was lost")
	}
	if _, err = s.db.Exec("UPDATE items SET quantity=0 WHERE id='a'"); err == nil {
		t.Fatal("quantity constraint was lost")
	}
	var integrity string
	if err = s.db.QueryRow("PRAGMA integrity_check").Scan(&integrity); err != nil || integrity != "ok" {
		t.Fatal(integrity, err)
	}
}
