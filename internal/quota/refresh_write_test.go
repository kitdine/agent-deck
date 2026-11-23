package quota

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"testing/synctest"
	"time"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

// Tap the normal database/sql path immediately around the stock driver's
// execution. The holder uses a separate, unwrapped connection to the same WAL.
type refreshTestExec func(context.Context, driver.ExecerContext, string, []driver.NamedValue) (driver.Result, error)
type refreshTestConnector struct {
	d    driver.Driver
	path string
	exec refreshTestExec
}

func (c refreshTestConnector) Driver() driver.Driver { return c.d }
func (c refreshTestConnector) Connect(context.Context) (driver.Conn, error) {
	dc, err := c.d.Open(c.path)
	if err != nil {
		return nil, err
	}
	return &refreshTestConn{dc.(refreshSQLiteConn), c.exec}, nil
}

type refreshSQLiteConn interface {
	driver.Conn
	driver.ExecerContext
	driver.QueryerContext
}
type refreshTestConn struct {
	refreshSQLiteConn
	exec refreshTestExec
}

func (c *refreshTestConn) ExecContext(ctx context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	return c.exec(ctx, c.refreshSQLiteConn, q, args)
}

type refreshTestDB interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func refreshTestSQL(t *testing.T, db refreshTestDB, q string) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(), q); err != nil {
		t.Fatal(q, err)
	}
}
func refreshWriterFixture(t *testing.T, exec refreshTestExec) (*Store, *sql.Conn) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "quota.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	holder, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { holder.Close() })
	refreshTestSQL(t, holder, "PRAGMA journal_mode=WAL")
	// Match the original read-pin proposal so the liveness regression can be
	// run against exactly the same schema, with no missing-table shortcut.
	refreshTestSQL(t, holder, "CREATE TABLE quota_cycle(singleton INTEGER PRIMARY KEY, seq INTEGER)")
	refreshTestSQL(t, holder, "INSERT INTO quota_cycle VALUES (1,0)")
	target := sql.OpenDB(refreshTestConnector{db.Driver(), path, exec})
	t.Cleanup(func() { target.Close() })
	target.SetMaxOpenConns(1)
	refreshTestSQL(t, target, "PRAGMA busy_timeout=731")
	refreshTestSQL(t, target, "CREATE TEMP TABLE connection_marker(n INTEGER)")
	return NewStore(target), holder
}

func checkRefreshReuse(t *testing.T, s *Store, rows int) {
	t.Helper()
	var busy, n int
	if err := s.db.QueryRow("PRAGMA busy_timeout").Scan(&busy); err != nil || busy != 731 {
		t.Fatal("restored timeout", busy, err)
	}
	if err := s.db.QueryRow("SELECT count(*) FROM connection_marker").Scan(&n); err != nil || n != rows {
		t.Fatal("same physical connection", n, err)
	}
	refreshTestSQL(t, s.db, "BEGIN IMMEDIATE")
	refreshTestSQL(t, s.db, "ROLLBACK")
}

func TestRefreshWriteShortRelease(t *testing.T) {
	for _, tc := range []struct{ release, parent time.Duration }{
		{0, 3 * time.Second}, {20 * time.Millisecond, 3 * time.Second}, {100 * time.Millisecond, 3 * time.Second},
		{400 * time.Millisecond, 3 * time.Second}, {40 * time.Millisecond, 150 * time.Millisecond},
	} {
		t.Run(tc.release.String(), func(t *testing.T) {
			var holder *sql.Conn
			var released chan error
			begins := 0
			exec := func(ctx context.Context, dc driver.ExecerContext, q string, args []driver.NamedValue) (driver.Result, error) {
				r, err := dc.ExecContext(ctx, q, args)
				if q == "BEGIN IMMEDIATE" {
					begins++
					if begins == 1 && tc.release > 0 {
						var native *sqlite.Error
						if !errors.As(err, &native) || native.Code()&255 != sqlite3.SQLITE_BUSY {
							t.Fatalf("first BEGIN: %v", err)
						}
						released = make(chan error, 1)
						time.AfterFunc(tc.release, func() {
							_, e := holder.ExecContext(context.Background(), "ROLLBACK")
							released <- e
						})
					}
				}
				return r, err
			}
			s, h := refreshWriterFixture(t, exec)
			holder = h
			if tc.release > 0 {
				refreshTestSQL(t, holder, "BEGIN IMMEDIATE")
			}
			ctx, cancel := context.WithTimeout(context.Background(), tc.parent)
			defer cancel()
			bodies := 0
			start := time.Now()
			out, err := s.refreshWrite(ctx, func(ctx context.Context, conn *sql.Conn) (bool, error) {
				bodies++
				var busy int
				if e := conn.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&busy); e != nil || busy != 0 {
					return false, fmt.Errorf("busy_timeout=%d: %v", busy, e)
				}
				_, e := conn.ExecContext(ctx, "INSERT INTO connection_marker VALUES(1)")
				return true, e
			})
			elapsed := time.Since(start)
			if released != nil {
				if e := <-released; e != nil {
					t.Fatal(e)
				}
			}
			t.Logf("release=%v return_after_cleanup=%v BEGINs=%d bodies=%d", tc.release, elapsed, begins, bodies)
			if err != nil || out != WriteCommitted || bodies != 1 || (tc.release > 0 && begins < 2) || (tc.release == 0 && begins != 1) {
				t.Fatal(out, err, begins, bodies)
			}
			checkRefreshReuse(t, s, 1)
		})
	}
}

// Deadline notification may lag the monotonic deadline. The parent retains
// explicit cancellation while synctest controls the monotonic clock.
type refreshDeadline struct {
	context.Context
	cutoff time.Time
}

func (p refreshDeadline) Deadline() (time.Time, bool) { return p.cutoff, true }

func TestRefreshWriteCutoff(t *testing.T) {
	for _, mode := range []string{"wake-at-cutoff", "acquired-late", "busy-then-acquired-late", "cancel-after-busy", "masked-acquisition"} {
		t.Run(mode, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				base, cancel := context.WithCancel(context.Background())
				defer cancel()
				parent := refreshDeadline{base, time.Now().Add(20 * time.Millisecond)}
				var holder *sql.Conn
				var busy error
				var begins, rollbacks, bodies int
				var events []string
				armed := false
				exec := func(ctx context.Context, dc driver.ExecerContext, q string, args []driver.NamedValue) (driver.Result, error) {
					if !armed {
						return dc.ExecContext(ctx, q, args)
					}
					if q == "BEGIN IMMEDIATE" {
						begins++
						if !time.Now().Before(parent.cutoff) {
							t.Fatalf("authorized BEGIN at %v past cutoff", time.Since(parent.cutoff))
						}
					}
					r, err := dc.ExecContext(ctx, q, args)
					events = append(events, q)
					switch q {
					case "BEGIN IMMEDIATE":
						if err != nil {
							var native *sqlite.Error
							if !errors.As(err, &native) || native.Code()&255 != sqlite3.SQLITE_BUSY {
								t.Fatal(err)
							}
							busy = err
							switch mode {
							case "busy-then-acquired-late":
								refreshTestSQL(t, holder, "ROLLBACK")
							case "cancel-after-busy":
								time.AfterFunc(time.Millisecond, cancel)
							}
						} else if mode == "masked-acquisition" {
							cancel()
							return nil, ctx.Err()
						} else {
							time.Sleep(time.Until(parent.cutoff) + time.Millisecond)
						}
					case "ROLLBACK":
						rollbacks++
					}
					return r, err
				}
				s, h := refreshWriterFixture(t, exec)
				holder = h
				held := mode == "wake-at-cutoff" || mode == "cancel-after-busy" || mode == "busy-then-acquired-late"
				if held {
					refreshTestSQL(t, holder, "BEGIN IMMEDIATE")
				}
				armed = true
				out, err := s.refreshWrite(parent, func(context.Context, *sql.Conn) (bool, error) { bodies++; return true, nil })
				armed = false
				wantErr := error(context.DeadlineExceeded)
				wantRollback := 1
				switch mode {
				case "wake-at-cutoff":
					wantErr = busy
					wantRollback = 0
				case "busy-then-acquired-late":
					wantErr = busy
				case "cancel-after-busy":
					wantErr = context.Canceled
					wantRollback = 0
				case "masked-acquisition":
					wantErr = context.Canceled
				}
				if held && mode != "busy-then-acquired-late" {
					refreshTestSQL(t, holder, "ROLLBACK")
				}
				if begins == 0 || out != WriteUnresolved || bodies != 0 || wantErr == nil || err != wantErr || rollbacks != wantRollback {
					t.Fatal(out, err, wantErr, begins, bodies, rollbacks, events)
				}
				if mode == "cancel-after-busy" && begins != 1 {
					t.Fatal("retried explicit cancellation", begins)
				}
				if mode == "wake-at-cutoff" && begins < 2 {
					t.Fatal("skipped bounded retries", begins)
				}
				if events[len(events)-1] != "PRAGMA busy_timeout=731" || (wantRollback == 1 && events[len(events)-2] != "ROLLBACK") {
					t.Fatal("cleanup order", events)
				}
				checkRefreshReuse(t, s, 0)
			})
		})
	}
}

func TestRefreshSQLiteBusyIdentitySkipsUsage(t *testing.T) {
	ctx, r, lock := refreshFixture(t)
	holder, err := r.core.DB.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Close()
	defer holder.ExecContext(ctx, "ROLLBACK")
	var identity, usage int
	s := Scheduler{Store: r.store, Interval: time.Minute, ProbeClaudeIdentity: func(context.Context, string) IdentityObservation {
		identity++
		refreshTestSQL(t, holder, "BEGIN IMMEDIATE")
		return IdentityObservation{State: IdentitySignedOut}
	}, ProbeClaudeProse: func(context.Context, time.Time, time.Duration) (ClaudeProseResult, error) {
		usage++
		return ClaudeProseResult{}, errors.New("unexpected usage")
	}}
	cycle, guard, err := s.RunRefresh(ctx, r, lock, TriggerManual, map[Client]Reason{ClientClaude: ""})
	var n int
	if dbErr := r.core.DB.QueryRow("SELECT count(*) FROM quota_identities").Scan(&n); dbErr != nil {
		t.Fatal(dbErr)
	}
	if err != nil || cycle.seq == 0 || guard != nil || identity != 1 || usage != 0 || n != 0 {
		t.Fatalf("cycle=%d guard=%v identity=%d usage=%d rows=%d err=%v", cycle.seq, guard != nil, identity, usage, n, err)
	}
}

func TestRefreshWriteOnlyRetriesBusyAtBegin(t *testing.T) {
	for _, stage := range []string{"begin-nonbusy", "body-busy", "commit-busy"} {
		t.Run(stage, func(t *testing.T) {
			var holder *sql.Conn
			var nativeErr error
			var begins, bodies int
			armed := true
			exec := func(ctx context.Context, dc driver.ExecerContext, q string, args []driver.NamedValue) (driver.Result, error) {
				if !armed {
					return dc.ExecContext(ctx, q, args)
				}
				switch q {
				case "BEGIN IMMEDIATE":
					begins++
					if stage == "begin-nonbusy" {
						_, nativeErr = dc.ExecContext(ctx, "SELECT missing_column", nil)
						return nil, nativeErr
					}
				case "COMMIT":
					if stage == "commit-busy" {
						// The target still owns the transaction. Let the other
						// SQLite connection supply a genuine BUSY to this seam.
						_, nativeErr = holder.ExecContext(ctx, "BEGIN IMMEDIATE")
						return nil, nativeErr
					}
				}
				return dc.ExecContext(ctx, q, args)
			}
			s, h := refreshWriterFixture(t, exec)
			holder = h
			out, err := s.refreshWrite(context.Background(), func(ctx context.Context, c *sql.Conn) (bool, error) {
				bodies++
				if _, err := c.ExecContext(ctx, "INSERT INTO connection_marker VALUES (1)"); err != nil {
					t.Fatal(err)
				}
				if stage == "body-busy" {
					_, nativeErr = holder.ExecContext(ctx, "BEGIN IMMEDIATE")
					return false, nativeErr
				}
				return true, nil
			})
			code := sqlite3.SQLITE_BUSY
			if stage == "begin-nonbusy" {
				code = sqlite3.SQLITE_ERROR
			}
			var native *sqlite.Error
			if out != WriteUnresolved || !errors.As(err, &native) || native.Code()&255 != code || err != nativeErr || begins != 1 || (stage == "begin-nonbusy" && bodies != 0) || (stage != "begin-nonbusy" && bodies != 1) {
				t.Fatal(out, err, begins, bodies)
			}
			armed = false
			checkRefreshReuse(t, s, 0)
		})
	}
}
