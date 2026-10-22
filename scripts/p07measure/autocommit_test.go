package p07measure

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWALInitializationIncludesPoolWaitAndModeChange(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "agentdeck.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	ctx := context.Background()
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer
	Start(&log)
	Register(db, "core")
	defer Start(nil)
	done := make(chan error, 1)
	go func() { _, err := ExecDB("actual-wal-init", db, ctx, "PRAGMA journal_mode=WAL"); done <- err }()
	// Exec cannot acquire its connection before this release.
	time.Sleep(30 * time.Millisecond)
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	var mode string
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil || mode != "wal" {
		t.Fatalf("mode=%s err=%v", mode, err)
	}
	values := records(t, &log)
	if len(values) != 1 || !values[0].Valid || values[0].ReadOnly || values[0].Operation != "journal_mode_wal" || values[0].DurationNS < int64(25*time.Millisecond) {
		t.Fatalf("missing full WAL initialization boundary: %+v", values)
	}
	for _, query := range []string{"PRAGMA journal_mode", "PRAGMA journal_mode=DELETE", "PRAGMA journal_mode=WAL;SELECT 1", "PRAGMA wal_checkpoint(TRUNCATE)"} {
		if statementKind(query) != "unknown" {
			t.Fatalf("unexpected exemption: %s", query)
		}
	}
}

func implicitFixture(t *testing.T) (*sql.DB, *bytes.Buffer) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("CREATE TABLE value(n INTEGER);INSERT INTO value VALUES(0)"); err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer
	Start(&log)
	Register(db, "core")
	t.Cleanup(func() { Flush(); Start(nil); db.Close() })
	return db, &log
}

func TestImplicitExecAndReturningScan(t *testing.T) {
	db, log := implicitFixture(t)
	ctx := context.Background()
	row := QueryRowSQL("returning", db, ctx, "UPDATE value SET n=n+1 RETURNING n")
	if len(records(t, log)) != 0 {
		t.Fatal("QueryRowContext was incorrectly treated as commit")
	}
	time.Sleep(12 * time.Millisecond)
	var n int
	if err := ScanRow(row, &n); err != nil || n != 1 {
		t.Fatalf("Scan/result changed: %d %v", n, err)
	}
	first := records(t, log)
	if len(first) != 1 || first[0].DurationNS < int64(12*time.Millisecond) {
		t.Fatalf("Scan delay missing: %+v", first)
	}
	if err := ScanRow(row, &n); err == nil {
		t.Fatal("second Scan changed sql.Row semantics")
	}
	result, err := ExecDB("publish", db, ctx, "UPDATE value SET n=n+1")
	if err != nil {
		t.Fatal(err)
	}
	affected, err := result.RowsAffected()
	if err != nil || affected != 1 {
		t.Fatal("RowsAffected changed")
	}
	if err = ScanRow(QueryRowSQL("read", db, ctx, "SELECT n FROM value"), &n); err != nil || n != 2 {
		t.Fatal("read/oracle changed")
	}
	values := records(t, log)
	if len(values) != 2 {
		t.Fatalf("want each implicit write once, got %+v", values)
	}
	for _, v := range values {
		if !v.Valid || v.Boundary != "autocommit" || v.Outcome != "implicit_commit" || v.Domain != "core" || v.EndNS-v.BeginNS != v.DurationNS {
			t.Fatal(v)
		}
	}
	if Flush() != 0 {
		t.Fatal("active implicit span")
	}
}

func TestConnExplicitStatementsAreNotDoubleCounted(t *testing.T) {
	db, log := implicitFixture(t)
	ctx := context.Background()
	c, err := Conn(db, ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseConn(c)
	tx, err := BeginTx("explicit", c, ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ExecConn("inside-tx", c, ctx, "UPDATE value SET n=n+1"); err != nil {
		t.Fatal(err)
	}
	var n int
	if err = ScanRow(QueryRowSQL("inside-tx-row", c, ctx, "UPDATE value SET n=n+1 RETURNING n"), &n); err != nil {
		t.Fatal(err)
	}
	if err = Commit(tx); err != nil {
		t.Fatal(err)
	}
	if _, err = ExecConn("raw", c, ctx, "BEGIN IMMEDIATE"); err != nil {
		t.Fatal(err)
	}
	if _, err = ExecConn("inside-raw", c, ctx, "UPDATE value SET n=99"); err != nil {
		t.Fatal(err)
	}
	if err = ScanRow(QueryRowSQL("inside-raw-row", c, ctx, "UPDATE value SET n=n+1 RETURNING n"), &n); err != nil {
		t.Fatal(err)
	}
	if _, err = ExecConn("raw", c, ctx, "ROLLBACK"); err != nil {
		t.Fatal(err)
	}
	values := records(t, log)
	if len(values) != 2 {
		t.Fatalf("explicit SQL double counted: %+v", values)
	}
	for _, v := range values {
		if !v.Valid || v.Boundary != "explicit" {
			t.Fatal(v)
		}
	}
	if _, err = ExecConn("outside", c, ctx, "UPDATE value SET n=n+1"); err != nil {
		t.Fatal(err)
	}
	values = records(t, log)
	if len(values) != 3 || values[2].Boundary != "autocommit" {
		t.Fatalf("explicit membership leaked: %+v", values)
	}
	if err = c.QueryRowContext(ctx, "SELECT n FROM value").Scan(&n); err != nil || n != 3 {
		t.Fatalf("commit/rollback state changed: %d %v", n, err)
	}
}

func TestUnknownAndUnobservedImplicitWrites(t *testing.T) {
	db, log := implicitFixture(t)
	ctx := context.Background()
	var n int
	row := QueryRowSQL("missing-scan", db, ctx, "UPDATE value SET n=n+1 RETURNING n")
	if Flush() != 1 {
		t.Fatal("missing Scan was silently lost")
	}
	if err := row.Scan(&n); err != nil {
		t.Fatal(err)
	} // cleanup original Row without an observer-induced close
	if err := ScanRow(QueryRowSQL("unknown", db, ctx, "WITH input AS (SELECT 1) UPDATE value SET n=n+1 RETURNING n"), &n); err != nil {
		t.Fatal(err)
	}
	if _, err := ExecDB("multiple", db, ctx, "UPDATE value SET n=n+1;UPDATE value SET n=n+1"); err != nil {
		t.Fatal(err)
	}
	rows, err := QuerySQL("unobserved-rows", db, ctx, "UPDATE value SET n=n+1 RETURNING n")
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		if err = rows.Scan(&n); err != nil {
			t.Fatal(err)
		}
	}
	rows.Close()
	stmt, err := PrepareSQL("unobserved-prepare", db, ctx, "UPDATE value SET n=n+1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = stmt.Exec(); err != nil {
		t.Fatal(err)
	}
	stmt.Close()
	if _, err = ExecDB("sql-error", db, ctx, "UPDATE missing_table SET n=1"); err == nil {
		t.Fatal("SQL error was swallowed")
	}
	values := records(t, log)
	if len(values) != 6 {
		t.Fatalf("unknown/unobserved writes lost: %+v", values)
	}
	want := []string{"terminal_unobserved", "unknown_statement", "unknown_statement", "unobserved_query_rows", "unobserved_prepared_write", "implicit_error"}
	for i, v := range values {
		if v.Valid || v.Outcome != want[i] {
			t.Fatalf("gap falsely passed: %+v", v)
		}
	}
}

func TestSQLClassificationFailsClosed(t *testing.T) {
	for query, want := range map[string]string{
		"-- c\n UPDATE value SET n=1; /* done */":   "write",
		"UPDATE value SET n='a;b'":                  "write",
		"UPDATE value SET \"n;\"=1":                 "write",
		"SELECT ';UPDATE value SET n=2'":            "read",
		"SELECT 1; UPDATE value SET n=2":            "unknown",
		"WITH x AS (SELECT 1) UPDATE value SET n=2": "unknown",
		"UPDATE value SET n=1;UPDATE value SET n=2": "unknown",
		"/* unterminated":                           "unknown",
		"UPDATE value SET n='unterminated":          "unknown",
		"PRAGMA user_version=32":                    "unknown",
	} {
		if got := statementKind(query); got != want {
			t.Fatalf("class=%s want %s for %q", got, want, query)
		}
	}
}

func TestAutocommitSyntheticOverhead(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec("CREATE TABLE value(n INTEGER);INSERT INTO value VALUES(0)"); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, api := range []string{"exec", "returning-scan"} {
		for pair := 0; pair < 3; pair++ {
			order := []bool{false, true}
			if pair%2 == 1 {
				order = []bool{true, false}
			}
			for _, enabled := range order {
				Start(nil)
				f, err := os.CreateTemp(t.TempDir(), "autocommit-")
				if err != nil {
					t.Fatal(err)
				}
				if enabled {
					Start(f)
					Register(db, "core")
				}
				began := time.Now()
				for i := 0; i < 100; i++ {
					if api == "exec" {
						if enabled {
							_, err = ExecDB("calibration", db, ctx, "UPDATE value SET n=n+1")
						} else {
							_, err = db.ExecContext(ctx, "UPDATE value SET n=n+1")
						}
					} else {
						var n int
						if enabled {
							err = ScanRow(QueryRowSQL("calibration", db, ctx, "UPDATE value SET n=n+1 RETURNING n"), &n)
						} else {
							err = db.QueryRowContext(ctx, "UPDATE value SET n=n+1 RETURNING n").Scan(&n)
						}
					}
					if err != nil {
						t.Fatal(err)
					}
				}
				value, _ := json.Marshal(map[string]any{"api": api, "pair": pair, "enabled": enabled, "writes": 100, "wall_ns": time.Since(began).Nanoseconds(), "sink": "file"})
				t.Log("P07_AUTOCOMMIT_OVERHEAD " + string(value))
				if Flush() != 0 {
					t.Fatal("incomplete calibration")
				}
				Start(nil)
				f.Close()
			}
		}
	}
}
