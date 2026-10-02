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

	"github.com/kitdine/agent-deck/internal/store"
	"modernc.org/sqlite"
)

// Observe the real SQLite read, then attempt an independent committed write at
// the exact read-to-write upgrade window. No production timing hooks are needed.
type publicationDriver struct {
	afterRead func()
	match     string
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
