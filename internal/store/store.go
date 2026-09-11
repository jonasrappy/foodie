// Package store provides atomic operations on the existing household database.
package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"time"

	_ "modernc.org/sqlite"
	"github.com/jonasrappy/foodie/internal/auth"
)

//go:embed schema.sql
var schema string

//go:embed migration_002.sql
var migration002 string

//go:embed migration_003.sql
var migration003 string

//go:embed migration_004.sql
var migration004 string

type Store struct{ db *sql.DB }
type Invalid string

func (e Invalid) Error() string { return string(e) }

type Conflict string

func (e Conflict) Error() string { return string(e) }

type Item struct {
	Quantity  float64 `json:"quantity"`
	Unit      string  `json:"unit"`
	ID        string  `json:"id"`
	Kind      string  `json:"kind"`
	Text      string  `json:"text"`
	Checked   bool    `json:"checked"`
	Version   int64   `json:"version"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}
type State struct {
	Revision  int64  `json:"revision"`
	UpdatedAt string `json:"updated_at"`
	Shopping  []Item `json:"shopping"`
	Meals     []Item `json:"meals"`
}
type Add struct {
	Quantity  *float64 `json:"quantity,omitempty"`
	Unit      *string  `json:"unit,omitempty"`
	Kind      string   `json:"kind"`
	Texts     []string `json:"texts"`
	RequestID string   `json:"request_id"`
}
type Edit struct {
	Quantity *float64
	Unit     *string
	Version  int64
	Text     *string
	Checked  *bool
}

func openDB(path, mode string) (*sql.DB, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	u := url.URL{Scheme: "file", Path: absolute}
	q := u.Query()
	q.Set("mode", mode)
	q.Set("_txlock", "immediate")
	q.Add("_pragma", "busy_timeout(5000)")
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "synchronous(FULL)")
	u.RawQuery = q.Encode()
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		return nil, err
	}
	// SQLite serializes writers. One connection also bounds memory and contention;
	// readers use transactions so revision and rows always describe the same state.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	return db, nil
}
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return nil, err
	}
	db, err := openDB(path, "rwc")
	if err != nil {
		return nil, err
	}
	success := false
	defer func() {
		if !success {
			db.Close()
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err = db.ExecContext(ctx, "PRAGMA journal_mode=WAL"); err != nil {
		return nil, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var version int
	if err = tx.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return nil, err
	}
	if version > 4 {
		return nil, fmt.Errorf("database schema %d is newer than supported version 4", version)
	}
	if version == 0 {
		if _, err = tx.ExecContext(ctx, schema); err != nil {
			return nil, err
		}
	}
	if version < 2 {
		if _, err = tx.ExecContext(ctx, migration002); err != nil {
			return nil, err
		}
	}
	if version < 3 {
		if _, err = tx.ExecContext(ctx, migration003); err != nil {
			return nil, err
		}
	}
	if version < 4 {
		if _, err = tx.ExecContext(ctx, migration004); err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	success = true
	return &Store{db: db}, nil
}
func (s *Store) Close() error                   { return s.db.Close() }
func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }
func now() string                               { return time.Now().UTC().Format("2006-01-02T15:04:05.000Z") }
func snapshot(ctx context.Context, tx *sql.Tx) (State, error) {
	state := State{Shopping: []Item{}, Meals: []Item{}}
	if err := tx.QueryRowContext(ctx, "SELECT revision,updated_at FROM meta WHERE id=1").Scan(&state.Revision, &state.UpdatedAt); err != nil {
		return state, err
	}
	rows, err := tx.QueryContext(ctx, "SELECT id,kind,text,checked,version,created_at,updated_at,quantity,unit FROM items ORDER BY created_at,rowid")
	if err != nil {
		return state, err
	}
	defer rows.Close()
	for rows.Next() {
		var item Item
		if err = rows.Scan(&item.ID, &item.Kind, &item.Text, &item.Checked, &item.Version, &item.CreatedAt, &item.UpdatedAt, &item.Quantity, &item.Unit); err != nil {
			return state, err
		}
		if item.Kind == "shopping" {
			state.Shopping = append(state.Shopping, item)
		} else {
			state.Meals = append(state.Meals, item)
		}
	}
	return state, rows.Err()
}
func (s *Store) Snapshot(ctx context.Context) (State, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return State{}, err
	}
	defer tx.Rollback()
	state, err := snapshot(ctx, tx)
	if err != nil {
		return State{}, err
	}
	return state, tx.Commit()
}
func (s *Store) write(ctx context.Context, fn func(*sql.Tx) (bool, error)) (State, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return State{}, err
	}
	defer tx.Rollback()
	changed, err := fn(tx)
	if err != nil {
		return State{}, err
	}
	if changed {
		if _, err = tx.ExecContext(ctx, "UPDATE meta SET revision=revision+1,updated_at=? WHERE id=1", now()); err != nil {
			return State{}, err
		}
	}
	state, err := snapshot(ctx, tx)
	if err != nil {
		return State{}, err
	}
	// Return only committed state; callers publish this exact revision to SSE.
	if err = tx.Commit(); err != nil {
		return State{}, err
	}
	return state, nil
}

var requestID = regexp.MustCompile(`^[a-zA-Z0-9-]{16,80}$`)

func CleanText(text string) (string, error) {
	if strings.ContainsAny(text, "\r\n") {
		return "", Invalid("Skriv én ting pr. linje, højst 300 tegn.")
	}
	text = strings.TrimSpace(text)
	if text == "" || auth.TextLength(text) > 300 {
		return "", Invalid("Skriv én ting pr. linje, højst 300 tegn.")
	}
	return text, nil
}

type addPayload struct {
	Quantity float64  `json:"quantity"`
	Unit     string   `json:"unit"`
	Kind     string   `json:"kind"`
	Texts    []string `json:"texts"`
}

func (s *Store) Add(ctx context.Context, input Add) (State, error) {
	if (input.Kind != "shopping" && input.Kind != "meals") || len(input.Texts) == 0 || len(input.Texts) > 50 || !requestID.MatchString(input.RequestID) {
		return State{}, Invalid("Ugyldig liste eller for mange linjer (maks. 50).")
	}
	clean := make([]string, len(input.Texts))
	for i, text := range input.Texts {
		var err error
		clean[i], err = CleanText(text)
		if err != nil {
			return State{}, err
		}
	}
	quantity, unit := 1.0, "stk."
	if input.Quantity != nil {
		quantity = *input.Quantity
	}
	if input.Unit != nil {
		unit = *input.Unit
	}
	if input.Kind != "shopping" && (input.Quantity != nil || input.Unit != nil) {
		return State{}, Invalid("Antal og enhed gælder kun indkøbsvarer.")
	}
	if err := ValidateAmount(quantity, unit); err != nil {
		return State{}, err
	}
	payload := addPayload{Kind: input.Kind, Texts: clean, Quantity: quantity, Unit: unit}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return State{}, err
	}
	return s.write(ctx, func(tx *sql.Tx) (bool, error) {
		var previous string
		err := tx.QueryRowContext(ctx, "SELECT payload FROM requests WHERE id=?", input.RequestID).Scan(&previous)
		if err == nil {
			// Compare decoded values: Node and Go escape JSON strings differently.
			old := addPayload{Quantity: 1, Unit: "stk."}
			if json.Unmarshal([]byte(previous), &old) != nil || !reflect.DeepEqual(old, payload) {
				return false, Conflict("Forespørgslen er allerede brugt.")
			}
			return false, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return false, err
		}
		var count int
		if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM items").Scan(&count); err != nil {
			return false, err
		}
		if count+len(clean) > 1000 {
			return false, Invalid("Listen er fuld. Fjern nogle linjer først.")
		}
		stamp := now()
		for _, text := range clean {
			id := make([]byte, 16)
			if _, err = rand.Read(id); err != nil {
				return false, err
			}
			if _, err = tx.ExecContext(ctx, "INSERT INTO items(id,kind,text,created_at,updated_at,quantity,unit) VALUES(?,?,?,?,?,?,?)", hex.EncodeToString(id), input.Kind, text, stamp, stamp, quantity, unit); err != nil {
				return false, err
			}
		}
		_, err = tx.ExecContext(ctx, "INSERT INTO requests VALUES(?,?)", input.RequestID, string(encoded))
		return err == nil, err
	})
}
func (s *Store) Edit(ctx context.Context, id string, input Edit, remove bool) (State, error) {
	return s.write(ctx, func(tx *sql.Tx) (bool, error) {
		var item Item
		err := tx.QueryRowContext(ctx, "SELECT kind,text,checked,version,quantity,unit FROM items WHERE id=?", id).Scan(&item.Kind, &item.Text, &item.Checked, &item.Version, &item.Quantity, &item.Unit)
		if errors.Is(err, sql.ErrNoRows) {
			return false, Conflict("Linjen er allerede fjernet.")
		}
		if err != nil {
			return false, err
		}
		if input.Version != item.Version {
			return false, Conflict("Linjen er ændret på en anden enhed. Prøv igen.")
		}
		if remove {
			_, err = tx.ExecContext(ctx, "DELETE FROM items WHERE id=?", id)
			return err == nil, err
		}
		if input.Text != nil {
			item.Text, err = CleanText(*input.Text)
			if err != nil {
				return false, err
			}
		}
		if input.Checked != nil {
			if item.Kind != "shopping" {
				return false, Invalid("Ugyldig afkrydsning.")
			}
			item.Checked = *input.Checked
		}
		if input.Quantity != nil || input.Unit != nil {
			if item.Kind != "shopping" {
				return false, Invalid("Antal og enhed gælder kun indkøbsvarer.")
			}
			if input.Quantity != nil {
				item.Quantity = *input.Quantity
			}
			if input.Unit != nil {
				item.Unit = *input.Unit
			}
			if err := ValidateAmount(item.Quantity, item.Unit); err != nil {
				return false, err
			}
		}
		_, err = tx.ExecContext(ctx, "UPDATE items SET text=?,checked=?,quantity=?,unit=?,version=version+1,updated_at=? WHERE id=?", item.Text, item.Checked, item.Quantity, item.Unit, now(), id)
		return err == nil, err
	})
}
func (s *Store) Reset(ctx context.Context, revision int64) (State, error) {
	return s.write(ctx, func(tx *sql.Tx) (bool, error) {
		state, err := snapshot(ctx, tx)
		if err != nil {
			return false, err
		}
		if state.Revision != revision {
			return false, Conflict("Listerne er ændret. Hent dem igen og håndter de nye ønsker før nulstilling.")
		}
		encoded, err := json.Marshal(state)
		if err != nil {
			return false, err
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO resets(created_at,previous_revision,snapshot) VALUES(?,?,?)", now(), state.Revision, string(encoded)); err != nil {
			return false, err
		}
		_, err = tx.ExecContext(ctx, "DELETE FROM items")
		return err == nil, err
	})
}

// ValidateAmount bounds quantities and permits two decimal places, e.g. 0.5 liter.
func ValidateAmount(quantity float64, unit string) error {
	if math.IsNaN(quantity) || math.IsInf(quantity, 0) || quantity < 0.01 || quantity > 9999 || math.Abs(quantity*100-math.Round(quantity*100)) > 0.000001 {
		return Invalid("Antal skal være mellem 0,01 og 9999 med højst to decimaler.")
	}
	switch unit {
	case "stk.", "liter", "milliliter", "kilo", "gram", "pakker", "poser", "dåser", "flasker", "bundter", "bakker", "kasser":
		return nil
	}
	return Invalid("Vælg en gyldig enhed.")
}
