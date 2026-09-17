package store

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestSchemaProbeWALPreservesSource(t *testing.T) {
	for _, live := range []bool{false, true} {
		t.Run(map[bool]string{false: "closed", true: "committed-wal"}[live], func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "agentdeck.sqlite3")
			db, err := sql.Open("sqlite", path)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			for _, query := range []string{
				"PRAGMA journal_mode=WAL",
				"CREATE TABLE schema_metadata (singleton INTEGER PRIMARY KEY, version INTEGER NOT NULL)",
				"INSERT INTO schema_metadata VALUES (1, 23)",
				"PRAGMA wal_checkpoint(TRUNCATE)",
				"UPDATE schema_metadata SET version=99",
			} {
				if _, err := db.Exec(query); err != nil {
					t.Fatal(err)
				}
			}
			if !live {
				if err := db.Close(); err != nil {
					t.Fatal(err)
				}
			} else {
				tx, err := db.Begin()
				if err != nil {
					t.Fatal(err)
				}
				defer tx.Rollback()
				if _, err := tx.Exec("UPDATE schema_metadata SET version=100"); err != nil {
					t.Fatal(err)
				}
			}
			before := probeSourceFiles(t, root)
			if live && len(before["agentdeck.sqlite3-wal"]) == 0 {
				t.Fatal("fixture lacks committed WAL")
			}
			lock, err := AcquireLock(context.Background(), root, 0)
			if err != nil {
				t.Fatal(err)
			}
			defer lock.Release()
			_, err = open(context.Background(), root, func(context.Context, string, time.Duration) (stateLock, error) { return nil, ErrStateBusy })
			assertSchemaAhead(t, err, 99)
			after := probeSourceFiles(t, root)
			if !reflect.DeepEqual(before, after) {
				t.Fatal("probe created or changed database/sidecar bytes")
			}
		})
	}
}

func probeSourceFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	result := map[string]string{}
	for _, suffix := range []string{"", "-wal", "-shm", "-journal"} {
		name := "agentdeck.sqlite3" + suffix
		data, err := os.ReadFile(filepath.Join(root, name))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		result[name] = string(data)
	}
	return result
}
