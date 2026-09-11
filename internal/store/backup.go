package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"

	"modernc.org/sqlite"
)

// Backup uses SQLite's online backup API, including committed WAL data. It does
// not migrate the source schema. The old destination survives any failed copy.
func Backup(ctx context.Context, source, destination string) (result error) {
	src, err := filepath.Abs(source)
	if err != nil {
		return err
	}
	dst, err := filepath.Abs(destination)
	if err != nil {
		return err
	}
	if src == dst {
		return errors.New("backup destination must differ from source")
	}
	if err = os.MkdirAll(filepath.Dir(dst), 0750); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".mad-backup-*.sqlite")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if err = tmp.Close(); err != nil {
		return err
	}
	defer os.Remove(tmpPath)
	defer os.Remove(tmpPath + "-wal")
	defer os.Remove(tmpPath + "-shm")
	db, err := openDB(src, "rw")
	if err != nil {
		return err
	}
	defer db.Close()
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	err = conn.Raw(func(driverConn any) (result error) {
		backuper, ok := driverConn.(interface {
			NewBackup(string) (*sqlite.Backup, error)
		})
		if !ok {
			return errors.New("SQLite driver does not support online backup")
		}
		copy, err := backuper.NewBackup(tmpPath)
		if err != nil {
			return err
		}
		defer func() { result = errors.Join(result, copy.Finish()) }()
		for {
			if err := ctx.Err(); err != nil {
				return err
			}
			more, err := copy.Step(128)
			if err != nil {
				var sqliteErr *sqlite.Error
				if errors.As(err, &sqliteErr) && (sqliteErr.Code()&255 == 5 || sqliteErr.Code()&255 == 6) {
					select {
					case <-ctx.Done():
						return ctx.Err()
					case <-time.After(25 * time.Millisecond):
						continue
					}
				}
				return err
			}
			if !more {
				return nil
			}
		}
	})
	if err != nil {
		return fmt.Errorf("copy database: %w", err)
	}
	check, err := openDB(tmpPath, "ro")
	if err != nil {
		return err
	}
	var integrity string
	err = check.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&integrity)
	closeErr := check.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if integrity != "ok" {
		return errors.New("backup integrity check failed")
	}
	file, err := os.OpenFile(tmpPath, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	err = file.Sync()
	closeErr = file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = os.Rename(tmpPath, dst); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(dst))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

var dailyName = regexp.MustCompile(`^mad-\d{4}-\d{2}-\d{2}\.sqlite$`)

func PruneBackups(dir string, keep int) error {
	if keep < 1 {
		return errors.New("must retain at least one backup")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	names := []string{}
	for _, entry := range entries {
		if entry.Type().IsRegular() && dailyName.MatchString(entry.Name()) {
			names = append(names, entry.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	for i := keep; i < len(names); i++ {
		if err = os.Remove(filepath.Join(dir, names[i])); err != nil {
			return err
		}
	}
	return nil
}
