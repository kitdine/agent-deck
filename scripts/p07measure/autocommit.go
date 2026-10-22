package p07measure

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

// statementKind deliberately admits only a single, unambiguous statement.
// Unknown SQL is measured as an invalid observation, never silently excluded.
// Quoted literals/identifiers and comments cannot hide a second statement.
func statementKind(query string) string {
	var clean strings.Builder
	for i := 0; i < len(query); {
		c := query[i]
		if c == '-' && i+1 < len(query) && query[i+1] == '-' {
			i += 2
			for i < len(query) && query[i] != '\n' {
				i++
			}
			clean.WriteByte(' ')
			continue
		}
		if c == '/' && i+1 < len(query) && query[i+1] == '*' {
			end := strings.Index(query[i+2:], "*/")
			if end < 0 {
				return "unknown"
			}
			i += end + 4
			clean.WriteByte(' ')
			continue
		}
		if c == '\'' || c == '"' || c == '`' || c == '[' {
			end := c
			if c == '[' {
				end = ']'
			}
			i++
			closed := false
			for i < len(query) {
				if query[i] == end {
					i++
					if i < len(query) && query[i] == end {
						i++
						continue
					}
					closed = true
					break
				}
				i++
			}
			if !closed {
				return "unknown"
			}
			clean.WriteString(" ? ")
			continue
		}
		clean.WriteByte(c)
		i++
	}
	text := strings.TrimSpace(clean.String())
	text = strings.TrimSpace(strings.TrimSuffix(text, ";"))
	if strings.Contains(text, ";") {
		return "unknown"
	}
	text = strings.ToUpper(text)
	end := 0
	for end < len(text) && text[end] >= 'A' && text[end] <= 'Z' {
		end++
	}
	keyword := text[:end]
	switch keyword {
	case "SELECT", "VALUES", "EXPLAIN":
		return "read"
	case "WITH":
		if readOnlyCTE(query) {
			return "read"
		}
	case "INSERT", "UPDATE", "DELETE", "REPLACE", "CREATE", "ALTER", "DROP":
		return "write"
	case "PRAGMA":
		// WAL initialization can change the database journal mode. Admit only
		// this exact single statement as a measured write, never as a read.
		if strings.Join(strings.Fields(text), "") == "PRAGMAJOURNAL_MODE=WAL" {
			return "initialization_write"
		}
		// These settings are connection-local, not SQLite write transactions.
		name := strings.TrimSpace(text[end:])
		stop := strings.IndexAny(name, " =(\t\r\n")
		if stop >= 0 {
			name = name[:stop]
		}
		if name == "BUSY_TIMEOUT" || name == "FOREIGN_KEYS" {
			return "read"
		}
	}
	return "unknown"
}

func insideExplicit(receiver any) bool {
	state.Lock()
	defer state.Unlock()
	c, ok := receiver.(*sql.Conn)
	if !ok {
		return false
	}
	_, raw := state.active[c]
	return raw || state.connTx[c] != nil
}

func statementStart(site string, receiver any, ctx context.Context, query string, suppress bool) (span, bool) {
	kind := statementKind(query)
	if kind == "read" || suppress && insideExplicit(receiver) {
		return span{}, false
	}
	s, enabled := start(receiver, site, ctx, false)
	s.record.Boundary = "autocommit"
	if kind == "initialization_write" {
		s.record.Operation = "journal_mode_wal"
	}
	if kind != "write" && kind != "initialization_write" {
		s.unsupported = "unknown_statement"
	}
	return s, enabled
}

func statementEnd(s span, ended time.Time, err error, unsupported string) {
	state.Lock()
	defer state.Unlock()
	outcome, valid := "implicit_commit", err == nil
	if err != nil {
		outcome = "implicit_error"
	}
	if s.unsupported != "" {
		outcome, valid = s.unsupported, false
	}
	if unsupported != "" {
		outcome, valid = unsupported, false
	}
	emit(s, ended, outcome, valid)
}

func execStatement(site string, receiver any, ctx context.Context, query string, invoke func() (sql.Result, error)) (sql.Result, error) {
	s, enabled := statementStart(site, receiver, ctx, query, true)
	s.started = time.Now()
	result, err := invoke()
	ended := time.Now()
	if enabled {
		statementEnd(s, ended, err, "")
	}
	return result, err
}

func ExecDB(site string, db *sql.DB, ctx context.Context, query string, args ...any) (sql.Result, error) {
	return execStatement(site, db, ctx, query, func() (sql.Result, error) { return db.ExecContext(ctx, query, args...) })
}
func ExecDBNoContext(site string, db *sql.DB, query string, args ...any) (sql.Result, error) {
	return execStatement(site, db, context.Background(), query, func() (sql.Result, error) { return db.Exec(query, args...) })
}

type rowQueryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func rowStatement(site string, receiver any, ctx context.Context, query string, invoke func() *sql.Row) *sql.Row {
	s, enabled := statementStart(site, receiver, ctx, query, true)
	s.started = time.Now()
	row := invoke()
	if enabled {
		state.Lock()
		state.active[row] = s
		state.Unlock()
	}
	return row
}
func QueryRowSQL(site string, db rowQueryer, ctx context.Context, query string, args ...any) *sql.Row {
	return rowStatement(site, db, ctx, query, func() *sql.Row { return db.QueryRowContext(ctx, query, args...) })
}
func QueryRowDBNoContext(site string, db *sql.DB, query string, args ...any) *sql.Row {
	return rowStatement(site, db, context.Background(), query, func() *sql.Row { return db.QueryRow(query, args...) })
}

// QueryRow is lazy: only Scan consumes and closes its Rows. The same *sql.Row
// escapes unchanged; its existing Scan is invoked once, without eager execution.
func ScanRow(row *sql.Row, dest ...any) error {
	err := row.Scan(dest...)
	ended := time.Now()
	outcome := "implicit_commit"
	if err != nil {
		outcome = "implicit_error"
	}
	finish(row, ended, outcome, err == nil)
	return err
}

type rowsQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func queryRows(site string, receiver any, ctx context.Context, query string, invoke func() (*sql.Rows, error)) (*sql.Rows, error) {
	s, enabled := statementStart(site, receiver, ctx, query, true)
	s.started = time.Now()
	rows, err := invoke()
	ended := time.Now()
	// A mutating multi-row stream has no observed terminal boundary in this
	// collector. Keep a gap; never count QueryContext's early return as commit.
	if enabled {
		statementEnd(s, ended, err, "unobserved_query_rows")
	}
	return rows, err
}
func QuerySQL(site string, db rowsQueryer, ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return queryRows(site, db, ctx, query, func() (*sql.Rows, error) { return db.QueryContext(ctx, query, args...) })
}
func QueryDBNoContext(site string, db *sql.DB, query string, args ...any) (*sql.Rows, error) {
	return queryRows(site, db, context.Background(), query, func() (*sql.Rows, error) { return db.Query(query, args...) })
}

type preparer interface {
	PrepareContext(context.Context, string) (*sql.Stmt, error)
}

func prepareSQL(site string, receiver any, ctx context.Context, query string, invoke func() (*sql.Stmt, error)) (*sql.Stmt, error) {
	// Preparing is not a write transaction. A possible deferred write has no
	// execution boundary here, so explicitly invalidate coverage rather than
	// fabricate a duration or lose a prepared statement escaping a raw Tx.
	s, enabled := statementStart(site, receiver, ctx, query, false)
	s.started = time.Now()
	stmt, err := invoke()
	ended := time.Now()
	if enabled {
		statementEnd(s, ended, err, "unobserved_prepared_write")
	}
	return stmt, err
}
func PrepareSQL(site string, db preparer, ctx context.Context, query string) (*sql.Stmt, error) {
	return prepareSQL(site, db, ctx, query, func() (*sql.Stmt, error) { return db.PrepareContext(ctx, query) })
}
func PrepareDBNoContext(site string, db *sql.DB, query string) (*sql.Stmt, error) {
	return prepareSQL(site, db, context.Background(), query, func() (*sql.Stmt, error) { return db.Prepare(query) })
}
