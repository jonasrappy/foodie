package store

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"

	"github.com/jonasrappy/foodie/internal/units"
	"github.com/jonasrappy/foodie/internal/voice"
)

//go:embed migration_005.sql
var migration005 string

// migrateUnits preserves request IDs, row order, revisions and archived user
// content. A pending confirmation is carried over only if it was still valid.
func migrateUnits(ctx context.Context, tx *sql.Tx) error {
	before, err := snapshot(ctx, tx)
	if err != nil {
		return err
	}
	after := before
	after.Shopping = append([]Item{}, before.Shopping...)
	after.Meals = append([]Item{}, before.Meals...)
	for _, items := range [][]Item{after.Shopping, after.Meals} {
		for i := range items {
			items[i].Unit = units.NormalizeLegacy(items[i].Unit)
		}
	}
	if _, err = tx.ExecContext(ctx, migration005); err != nil {
		return err
	}
	// Copy with the original rowids; tied timestamps must keep their ordering.
	if _, err = tx.ExecContext(ctx, `INSERT INTO items_v5(rowid,id,kind,text,checked,version,created_at,updated_at,quantity,unit) SELECT rowid,id,kind,text,checked,version,created_at,updated_at,quantity,'piece' FROM items`); err != nil {
		return err
	}
	for _, item := range append(after.Shopping, after.Meals...) {
		if _, err = tx.ExecContext(ctx, `UPDATE items_v5 SET unit=? WHERE id=?`, item.Unit, item.ID); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, `DROP TABLE items; ALTER TABLE items_v5 RENAME TO items;`); err != nil {
		return err
	}
	// Update only unit fields in JSON, never text that happens to name a unit.
	for _, table := range []string{"requests", "resets", "voice_intents"} {
		column := map[string]string{"requests": "payload", "resets": "snapshot", "voice_intents": "command"}[table]
		rows, err := tx.QueryContext(ctx, "SELECT id,"+column+" FROM "+table)
		if err != nil {
			return err
		}
		type update struct {
			id    any
			value string
		}
		updates := []update{}
		for rows.Next() {
			var id any
			var raw string
			if err = rows.Scan(&id, &raw); err != nil {
				rows.Close()
				return err
			}
			var document map[string]json.RawMessage
			if err = json.Unmarshal([]byte(raw), &document); err != nil {
				rows.Close()
				return err
			}
			if table == "resets" {
				for _, key := range []string{"shopping", "meals"} {
					if len(document[key]) == 0 {
						continue
					}
					var items []map[string]json.RawMessage
					if err = json.Unmarshal(document[key], &items); err != nil {
						rows.Close()
						return err
					}
					for _, item := range items {
						if err = normalizeJSONUnit(item); err != nil {
							rows.Close()
							return err
						}
					}
					document[key], _ = json.Marshal(items)
				}
			} else if err = normalizeJSONUnit(document); err != nil {
				rows.Close()
				return err
			}
			encoded, _ := json.Marshal(document)
			updates = append(updates, update{id, string(encoded)})
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		for _, update := range updates {
			if table == "voice_intents" {
				var command voice.Command
				if err = json.Unmarshal([]byte(update.value), &command); err != nil {
					return err
				}
				// Matching hashes include row units. Update only a signature that
				// matched the pre-migration rows; stale confirmations stay stale.
				_, oldSignature := voiceMatches(before, command)
				_, newSignature := voiceMatches(after, command)
				if _, err = tx.ExecContext(ctx, "UPDATE voice_intents SET signature=? WHERE id=? AND signature=?", newSignature, update.id, oldSignature); err != nil {
					return err
				}
			}
			if _, err = tx.ExecContext(ctx, "UPDATE "+table+" SET "+column+"=? WHERE id=?", update.value, update.id); err != nil {
				return err
			}
		}
	}
	_, err = tx.ExecContext(ctx, "PRAGMA user_version=5")
	return err
}

func normalizeJSONUnit(document map[string]json.RawMessage) error {
	if raw, ok := document["unit"]; ok {
		var unit string
		if err := json.Unmarshal(raw, &unit); err != nil {
			return err
		}
		document["unit"], _ = json.Marshal(units.NormalizeLegacy(unit))
	}
	return nil
}
