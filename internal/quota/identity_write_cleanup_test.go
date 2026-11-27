package quota

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"modernc.org/sqlite"
)

func TestQuotaWriteDiscardsConnectionOnCleanupFailure(t *testing.T) {
	for _, tc := range []struct {
		name      string
		statement string
	}{
		{name: "rollback", statement: "rollback"},
		{name: "restore_busy_timeout", statement: "pragmabusy_timeout=731"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dsn := "file:" + filepath.Join(t.TempDir(), "quota.sqlite3")
			other, err := sql.Open("sqlite", dsn+"?_pragma=busy_timeout(0)")
			if err != nil {
				t.Fatal(err)
			}
			defer other.Close()
			if _, err := other.Exec("PRAGMA journal_mode=WAL; CREATE TABLE write_probe(value INTEGER)"); err != nil {
				t.Fatal(err)
			}

			fault := &quotaCleanupFaultConnector{
				dsn:       dsn + "?_pragma=busy_timeout(731)",
				statement: tc.statement,
			}
			db := sql.OpenDB(fault)
			defer db.Close()
			db.SetMaxOpenConns(1)
			s := &Store{db: db}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			out, err := s.refreshWrite(ctx, func(ctx context.Context, conn *sql.Conn) (bool, error) {
				_, err := conn.ExecContext(ctx, "INSERT INTO write_probe VALUES (1)")
				cancel()
				return false, err
			})
			if out == WriteCommitted || (err != nil && !errors.Is(err, context.Canceled)) {
				t.Fatalf("dropped write returned %v, %v", out, err)
			}
			if fault.injected.Load() != 1 {
				t.Fatalf("cleanup fault injections = %d, want 1", fault.injected.Load())
			}
			if _, err := other.Exec("BEGIN IMMEDIATE; ROLLBACK"); err != nil {
				t.Fatalf("another connection could not immediately acquire the write lock: %v", err)
			}
			var busy, rows int
			if err := db.QueryRow("PRAGMA busy_timeout").Scan(&busy); err != nil || busy != 731 {
				t.Fatalf("next pool lease busy_timeout = %d, want 731: %v", busy, err)
			}
			if got := fault.opens.Load(); got != 2 {
				t.Fatalf("physical connections opened = %d, want 2", got)
			}
			if err := other.QueryRow("SELECT COUNT(*) FROM write_probe").Scan(&rows); err != nil || rows != 0 {
				t.Fatalf("dropped write left %d rows, want 0: %v", rows, err)
			}
		})
	}
}

type quotaCleanupFaultConnector struct {
	dsn       string
	statement string
	injected  atomic.Int32
	opens     atomic.Int32
}

func (c *quotaCleanupFaultConnector) Driver() driver.Driver { return &sqlite.Driver{} }

func (c *quotaCleanupFaultConnector) Connect(ctx context.Context) (driver.Conn, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	conn, err := c.Driver().Open(c.dsn)
	if err != nil {
		return nil, err
	}
	c.opens.Add(1)
	return &quotaCleanupFaultConn{Conn: conn, fault: c}, nil
}

type quotaCleanupFaultConn struct {
	driver.Conn
	fault *quotaCleanupFaultConnector
}

func (c *quotaCleanupFaultConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if strings.Join(strings.Fields(strings.ToLower(query)), "") == c.fault.statement {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if _, ok := ctx.Deadline(); !ok {
			return nil, errors.New("quota cleanup context has no deadline")
		}
		c.fault.injected.Add(1)
		return nil, errors.New("injected quota cleanup failure")
	}
	return c.Conn.(driver.ExecerContext).ExecContext(ctx, query, args)
}
