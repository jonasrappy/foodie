package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "mad.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
func pointer[T any](v T) *T { return &v }
func addItem(t *testing.T, s *Store, name string) State {
	t.Helper()
	state, err := s.Add(t.Context(), Add{Kind: "shopping", Texts: []string{name}, RequestID: "request-" + name + "-0000000000"})
	if err != nil {
		t.Fatal(err)
	}
	return state
}
func TestLegacyMigrationAndIdempotency(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mad.sqlite")
	db, err := openDB(path, "rwc")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(schema + "PRAGMA user_version=0;"); err != nil {
		t.Fatal(err)
	}
	stamp := "2026-09-11T12:00:00.000Z"
	if _, err = db.Exec("INSERT INTO items(id,kind,text,created_at,updated_at) VALUES('legacy','shopping','Mælk',?,?)", stamp, stamp); err != nil {
		t.Fatal(err)
	}
	// Payload is byte-for-byte Node JSON.stringify output (HTML is not escaped).
	if _, err = db.Exec("INSERT INTO requests VALUES(?,?)", "node-request-0000001", `{"kind":"shopping","texts":["<æble> & pære"]}`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	state, err := s.Snapshot(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Shopping) != 1 || state.Shopping[0].Quantity != 1 || state.Shopping[0].Unit != "stk." || state.Shopping[0].CreatedAt != stamp {
		t.Fatalf("legacy migration lost data: %+v", state)
	}
	state, err = s.Add(t.Context(), Add{Kind: "shopping", Texts: []string{"<æble> & pære"}, RequestID: "node-request-0000001"})
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Shopping) != 1 || state.Revision != 0 {
		t.Fatal("legacy retry duplicated data")
	}
	_, err = s.Add(t.Context(), Add{Kind: "shopping", Texts: []string{"<æble> & pære"}, RequestID: "node-request-0000001", Quantity: pointer(2.0)})
	var conflict Conflict
	if !errors.As(err, &conflict) {
		t.Fatalf("changed retry was accepted: %v", err)
	}
	var version int
	if err = s.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil || version != 4 {
		t.Fatalf("schema version=%d, error=%v", version, err)
	}
}
func TestQuantitiesValidationAndEdit(t *testing.T) {
	s := testStore(t)
	state, err := s.Add(t.Context(), Add{Kind: "shopping", Texts: []string{"Mælk", "Juice"}, RequestID: "quantity-request-0001", Quantity: pointer(1.5), Unit: pointer("liter")})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range state.Shopping {
		if item.Quantity != 1.5 || item.Unit != "liter" {
			t.Fatal("batch quantity missing")
		}
	}
	item := state.Shopping[0]
	state, err = s.Edit(t.Context(), item.ID, Edit{Version: item.Version, Quantity: pointer(500.0), Unit: pointer("milliliter"), Checked: pointer(true)}, false)
	if err != nil {
		t.Fatal(err)
	}
	item = state.Shopping[0]
	if item.Quantity != 500 || item.Unit != "milliliter" || !item.Checked {
		t.Fatalf("edit failed: %+v", item)
	}
	for _, amount := range []float64{0, -1, 10000, 0.001, 1.001} {
		_, err = s.Edit(t.Context(), item.ID, Edit{Version: item.Version, Quantity: &amount}, false)
		var invalid Invalid
		if !errors.As(err, &invalid) {
			t.Fatalf("accepted quantity %v: %v", amount, err)
		}
	}
	if _, err = s.Edit(t.Context(), item.ID, Edit{Version: item.Version, Unit: pointer("invalid")}, false); err == nil {
		t.Fatal("accepted unknown unit")
	}
	unchanged, err := s.Snapshot(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(unchanged, state) {
		t.Fatal("failed edits changed state")
	}
}
func TestConcurrentWritesEditsAndReset(t *testing.T) {
	s := testStore(t)
	ctx := t.Context()
	var wg sync.WaitGroup
	errs := make(chan error, 64)
	for i := range 64 {
		wg.Go(func() {
			_, err := s.Add(ctx, Add{Kind: "shopping", Texts: []string{fmt.Sprint(i)}, RequestID: fmt.Sprintf("parallel-request-%04d", i)})
			errs <- err
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	state, err := s.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if state.Revision != 64 || len(state.Shopping) != 64 {
		t.Fatalf("lost concurrent writes: rev=%d, count=%d", state.Revision, len(state.Shopping))
	}
	item := state.Shopping[0]
	results := make(chan error, 16)
	for range 16 {
		wg.Go(func() {
			_, err := s.Edit(ctx, item.ID, Edit{Version: item.Version, Checked: pointer(true)}, false)
			results <- err
		})
	}
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		var conflict Conflict
		if err == nil {
			successes++
		} else if !errors.As(err, &conflict) {
			t.Fatal(err)
		}
	}
	if successes != 1 {
		t.Fatalf("%d edits accepted for same version", successes)
	}
	state, err = s.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	oldRevision := state.Revision
	newer := addItem(t, s, "new")
	if _, err = s.Reset(ctx, oldRevision); err == nil {
		t.Fatal("stale reset accepted")
	}
	after, err := s.Snapshot(ctx)
	if err != nil || !reflect.DeepEqual(newer, after) {
		t.Fatal("failed reset changed lists", err)
	}
	results = make(chan error, 16)
	for range 16 {
		wg.Go(func() { _, err := s.Reset(ctx, newer.Revision); results <- err })
	}
	wg.Wait()
	close(results)
	successes = 0
	for err := range results {
		if err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("%d resets accepted for same revision", successes)
	}
	var saved string
	if err = s.db.QueryRow("SELECT snapshot FROM resets").Scan(&saved); err != nil {
		t.Fatal(err)
	}
	var archive State
	if err = json.Unmarshal([]byte(saved), &archive); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(archive, newer) {
		t.Fatal("reset archive does not match removed state")
	}
}
func TestResetRacingAddNeverDeletesNewItem(t *testing.T) {
	s := testStore(t)
	state := addItem(t, s, "old")
	start := make(chan struct{})
	results := make(chan error, 2)
	go func() { <-start; _, err := s.Reset(t.Context(), state.Revision); results <- err }()
	go func() {
		<-start
		_, err := s.Add(t.Context(), Add{Kind: "shopping", Texts: []string{"new"}, RequestID: "race-new-request-0001"})
		results <- err
	}()
	close(start)
	for range 2 {
		err := <-results
		var conflict Conflict
		if err != nil && !errors.As(err, &conflict) {
			t.Fatal(err)
		}
	}
	state, err := s.Snapshot(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range state.Shopping {
		if item.Text == "new" {
			found = true
		}
	}
	if !found {
		t.Fatal("reset deleted a newly added item")
	}
}
func TestBackupPersistenceAndFailureSafety(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "mad.sqlite")
	s, err := Open(source)
	if err != nil {
		t.Fatal(err)
	}
	expected := addItem(t, s, "backup")
	dest := filepath.Join(dir, "backups", "mad-2026-09-11.sqlite")
	if err = Backup(t.Context(), source, dest); err != nil {
		t.Fatal(err)
	}
	copy, err := Open(dest)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := copy.Snapshot(t.Context())
	copy.Close()
	if err != nil || !reflect.DeepEqual(expected, actual) {
		t.Fatal("backup differs from live WAL database", err)
	}
	bytesBefore, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err = Backup(ctx, source, dest); err == nil {
		t.Fatal("canceled backup succeeded")
	}
	bytesAfter, err := os.ReadFile(dest)
	if err != nil || !reflect.DeepEqual(bytesBefore, bytesAfter) {
		t.Fatal("failed backup replaced previous backup")
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	actual, err = s.Snapshot(t.Context())
	if err != nil || !reflect.DeepEqual(expected, actual) {
		t.Fatal("state lost on restart", err)
	}
	for i := range 20 {
		name := time.Date(2026, 8, i+1, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
		if err = os.WriteFile(filepath.Join(dir, "backups", "mad-"+name+".sqlite"), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err = PruneBackups(filepath.Join(dir, "backups"), 14); err != nil {
		t.Fatal(err)
	}
	files, err := os.ReadDir(filepath.Join(dir, "backups"))
	if err != nil || len(files) != 14 {
		t.Fatalf("retention count=%d, error=%v", len(files), err)
	}
}
