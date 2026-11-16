package store

import (
	"context"
	"database/sql"
	"testing"
)

func TestWritableStoresKeepStatementJournalsInMemory(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	created, err := Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if err = created.Close(); err != nil {
		t.Fatal(err)
	}
	stores := map[string]func() (*Store, error){
		"created core":  func() (*Store, error) { return Open(ctx, t.TempDir()) },
		"existing core": func() (*Store, error) { return OpenExisting(ctx, root) },
		"sessions":      func() (*Store, error) { return OpenSessions(ctx, root) },
	}
	for name, open := range stores {
		db, openErr := open()
		if openErr != nil {
			t.Fatalf("%s: %v", name, openErr)
		}
		// Every pooled connection must carry the pragma, not only the first one.
		conns := make([]*sql.Conn, 0, 3)
		for range 3 {
			conn, connErr := db.DB.Conn(ctx)
			if connErr != nil {
				t.Fatal(connErr)
			}
			conns = append(conns, conn)
			var mode, foreignKeys int
			if err = conn.QueryRowContext(ctx, "PRAGMA temp_store").Scan(&mode); err != nil || mode != 2 {
				t.Fatalf("%s temp_store=%d err=%v, want memory", name, mode, err)
			}
			if err = conn.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&foreignKeys); err != nil || foreignKeys != 1 {
				t.Fatalf("%s foreign_keys=%d err=%v", name, foreignKeys, err)
			}
		}
		for _, conn := range conns {
			conn.Close()
		}
		if err = db.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
