package usage

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
	"modernc.org/sqlite"
)

// Observe the real SQLite read, then attempt an independent committed write at
// the exact read-to-write upgrade window. No production timing hooks are needed.
type publicationDriver struct {
	beforeWrite func()
	beforeOnce  sync.Once
	afterRead   func()
	match       string
}

func (d *publicationDriver) Open(name string) (driver.Conn, error) {
	c, e := (&sqlite.Driver{}).Open(name)
	if e != nil {
		return nil, e
	}
	return &publicationConn{Conn: c, owner: d}, nil
}

type publicationConn struct {
	driver.Conn
	owner *publicationDriver
}

func (c *publicationConn) QueryContext(ctx context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	r, e := c.Conn.(driver.QueryerContext).QueryContext(ctx, q, args)
	if e == nil && strings.Contains(q, c.owner.match) {
		return &publicationRows{Rows: r, after: c.owner.afterRead}, nil
	}
	return r, e
}
func (c *publicationConn) ExecContext(ctx context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	if strings.Contains(q, "UPDATE usage_source_files SET cursor=cursor WHERE 0") && c.owner.beforeWrite != nil {
		c.owner.beforeOnce.Do(c.owner.beforeWrite)
	}
	return c.Conn.(driver.ExecerContext).ExecContext(ctx, q, args)
}

type publicationRows struct {
	driver.Rows
	after func()
	once  sync.Once
}

func (r *publicationRows) Close() error { e := r.Rows.Close(); r.once.Do(r.after); return e }

func TestSourcePublicationSerializesExternalWriter(t *testing.T) {
	for _, mode := range []string{"cold", "append", "rewrite"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			root := t.TempDir()
			state := filepath.Join(root, "state")
			home := filepath.Join(root, "home")
			source := filepath.Join(home, ".codex", "sessions", "fixture.jsonl")
			if e := os.MkdirAll(filepath.Dir(source), 0700); e != nil {
				t.Fatal(e)
			}
			prefix := `{"type":"session_meta","payload":{"session_id":"session"}}` + "\n" + `{"type":"turn_context","payload":{"turn_id":"turn","model":"gpt-5.4"}}` + "\n"
			event := func(n int) string {
				return fmt.Sprintf(`{"type":"event_msg","timestamp":"2026-07-20T00:00:01Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":%d},"total_token_usage":{"input_tokens":%d}}}}`+"\n", n, n)
			}
			write := func(s string) {
				t.Helper()
				if e := os.WriteFile(source, []byte(s), 0600); e != nil {
					t.Fatal(e)
				}
			}
			base, e := store.Open(ctx, state)
			if e != nil {
				t.Fatal(e)
			}
			defer base.Close()
			if _, e = base.DB.ExecContext(ctx, "CREATE TABLE external_writer(value INTEGER)"); e != nil {
				t.Fatal(e)
			}
			write(prefix + event(10))
			if mode != "cold" {
				if _, e = New(base, home).Scan(ctx); e != nil {
					t.Fatal(e)
				}
				if mode == "append" {
					write(prefix + event(10) + event(20))
				} else {
					write(prefix + event(30))
				}
			}
			external, e := sql.Open("sqlite", filepath.Join(state, "agentdeck.sqlite3")+"?_pragma=busy_timeout(0)")
			if e != nil {
				t.Fatal(e)
			}
			defer external.Close()
			attempted := 0
			var writerErr error
			d := &publicationDriver{match: "SELECT DISTINCT client,session_id FROM usage_events WHERE source_path=?", afterRead: func() { attempted++; _, writerErr = external.ExecContext(ctx, "INSERT INTO external_writer VALUES(1)") }}
			name := fmt.Sprintf("publication-%p", d)
			sql.Register(name, d)
			db, e := sql.Open(name, filepath.Join(state, "agentdeck.sqlite3")+"?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)")
			if e != nil {
				t.Fatal(e)
			}
			defer db.Close()
			if _, e = New(&store.Store{DB: db}, home).Scan(ctx); e != nil {
				t.Fatalf("scan during external write: %v (writer: %v)", e, writerErr)
			}
			if attempted != 1 {
				t.Fatalf("attempts=%d", attempted)
			}
			var busy *sqlite.Error
			if !errors.As(writerErr, &busy) || busy.Code() != 5 {
				t.Fatalf("external writer error = %v, want SQLITE_BUSY", writerErr)
			}
			if _, e = external.ExecContext(ctx, "INSERT INTO external_writer VALUES(2)"); e != nil {
				t.Fatalf("lock retained after publication: %v", e)
			}
			var tokens int
			if e = base.DB.QueryRowContext(ctx, "SELECT SUM(input_tokens) FROM usage_events").Scan(&tokens); e != nil {
				t.Fatal(e)
			}
			want := 10
			if mode == "append" {
				want = 20
			}
			if mode == "rewrite" {
				want = 30
			}
			if tokens != want {
				t.Fatalf("tokens=%d want=%d", tokens, want)
			}
			var cursor int
			if e = base.DB.QueryRowContext(ctx, "SELECT cursor FROM usage_source_files WHERE path=?", source).Scan(&cursor); e != nil {
				t.Fatal(e)
			}
			info, e := os.Stat(source)
			if e != nil {
				t.Fatal(e)
			}
			if int64(cursor) != info.Size() {
				t.Fatalf("cursor=%d size=%d", cursor, info.Size())
			}
		})
	}
}

func TestPublicationWaitPreservesSourceAndCancellation(t *testing.T) {
	for _, mode := range []string{"rewrite", "truncate", "cancel", "wait"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			root := t.TempDir()
			state := filepath.Join(root, "state")
			home := filepath.Join(root, "home")
			path := filepath.Join(home, ".codex", "sessions", "fixture.jsonl")
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			content := `{"type":"session_meta","payload":{"session_id":"session"}}` + "\n" + `{"type":"turn_context","payload":{"turn_id":"turn","model":"gpt-5.4"}}` + "\n" + `{"type":"event_msg","timestamp":"2026-07-20T00:00:01Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":10},"total_token_usage":{"input_tokens":10}}}}` + "\n"
			if err := os.WriteFile(path, []byte(content), 0600); err != nil {
				t.Fatal(err)
			}
			base, err := store.Open(ctx, state)
			if err != nil {
				t.Fatal(err)
			}
			defer base.Close()
			if _, err = base.DB.ExecContext(ctx, "CREATE TABLE external_writer(value INTEGER)"); err != nil {
				t.Fatal(err)
			}
			external, err := sql.Open("sqlite", filepath.Join(state, "agentdeck.sqlite3")+"?_pragma=busy_timeout(0)")
			if err != nil {
				t.Fatal(err)
			}
			defer external.Close()
			var externalTx *sql.Tx
			done := make(chan error, 1)
			d := &publicationDriver{match: "SELECT DISTINCT client,session_id FROM usage_events WHERE source_path=?", afterRead: func() {}}
			d.beforeWrite = func() {
				externalTx, err = external.BeginTx(context.Background(), nil)
				if err != nil {
					t.Fatal(err)
				}
				if _, err = externalTx.Exec("INSERT INTO external_writer VALUES(1)"); err != nil {
					t.Fatal(err)
				}
				time.AfterFunc(100*time.Millisecond, func() {
					if mode == "cancel" {
						cancel()
						done <- nil
						return
					}
					if mode == "rewrite" {
						err = os.WriteFile(path, []byte(strings.ReplaceAll(content, "10", "99")), 0600)
					}
					if mode == "truncate" {
						err = os.Truncate(path, 0)
					}
					if err == nil {
						err = externalTx.Commit()
					}
					done <- err
				})
			}
			name := fmt.Sprintf("publication-wait-%p", d)
			sql.Register(name, d)
			db, err := sql.Open(name, filepath.Join(state, "agentdeck.sqlite3")+"?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)")
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			db.SetMaxOpenConns(1)
			started := time.Now()
			_, scanErr := New(&store.Store{DB: db}, home).Scan(ctx)
			elapsed := time.Since(started)
			if externalTx != nil {
				_ = externalTx.Rollback()
			}
			if err := <-done; err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "cancel":
				if !errors.Is(scanErr, context.Canceled) {
					t.Errorf("scan error=%v want canceled", scanErr)
				}
				if elapsed > time.Second {
					t.Errorf("cancellation took %v", elapsed)
				}
			case "wait":
				if scanErr != nil {
					t.Errorf("scan waiting for writer: %v", scanErr)
				}
			default:
				if !errors.Is(scanErr, errUsageSourceChanged) {
					t.Errorf("scan error=%v want source changed", scanErr)
				}
			}
			var count int
			if err = base.DB.QueryRow("SELECT COUNT(*) FROM usage_events").Scan(&count); err != nil {
				t.Fatal(err)
			}
			want := 0
			if mode == "wait" {
				want = 1
			}
			if count != want {
				t.Errorf("published events=%d want=%d", count, want)
			}
			var timeout int
			if err = db.QueryRow("PRAGMA busy_timeout").Scan(&timeout); err != nil {
				t.Fatal(err)
			}
			if timeout != 5000 {
				t.Errorf("pool timeout=%d want5000", timeout)
			}
			if _, err = external.Exec("INSERT INTO external_writer VALUES(2)"); err != nil {
				t.Errorf("writer retained: %v", err)
			}
		})
	}
}

func TestPublicationLockHonorsTimeoutAndRestoresConnection(t *testing.T) {
	for _, timeout := range []int{0, 40} {
		t.Run(fmt.Sprint(timeout), func(t *testing.T) {
			ctx := context.Background()
			root := t.TempDir()
			base, err := store.Open(ctx, root)
			if err != nil {
				t.Fatal(err)
			}
			defer base.Close()
			db, err := sql.Open("sqlite", fmt.Sprintf("%s?_pragma=busy_timeout(%d)", filepath.Join(root, "agentdeck.sqlite3"), timeout))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			db.SetMaxOpenConns(1)
			writer, err := base.DB.BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer writer.Rollback()
			if _, err = writer.Exec("UPDATE usage_source_files SET cursor=cursor WHERE 0"); err != nil {
				t.Fatal(err)
			}
			start := time.Now()
			_, release, err := beginSourcePublication(ctx, db)
			elapsed := time.Since(start)
			if release != nil {
				release()
				t.Fatal("acquired despite held writer")
			}
			var busy *sqlite.Error
			if !errors.As(err, &busy) || busy.Code() != 5 {
				t.Fatalf("error=%v want BUSY", err)
			}
			if elapsed > time.Second || elapsed < time.Duration(timeout)*time.Millisecond {
				t.Errorf("timeout %d elapsed %v", timeout, elapsed)
			}
			var got int
			if err = db.QueryRow("PRAGMA busy_timeout").Scan(&got); err != nil {
				t.Fatal(err)
			}
			if got != timeout {
				t.Errorf("timeout=%d want%d", got, timeout)
			}
			if err = writer.Rollback(); err != nil {
				t.Fatal(err)
			}
			tx, release, err := beginSourcePublication(ctx, db)
			if err != nil {
				t.Fatal(err)
			}
			if err = tx.Commit(); err != nil {
				t.Fatal(err)
			}
			release()
			if err = db.QueryRow("PRAGMA busy_timeout").Scan(&got); err != nil {
				t.Fatal(err)
			}
			if got != timeout {
				t.Errorf("after commit timeout=%d want%d", got, timeout)
			}
		})
	}
}
