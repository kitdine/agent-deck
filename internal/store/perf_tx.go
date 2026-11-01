package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"runtime"
	"strings"
	"sync"

	"github.com/kitdine/agent-deck/internal/perftrace"
)

// openWritableSQL opens a writable database. With measurement recording on,
// every write transaction is timed as tx.<domain>.<label> from driver Begin to
// the Commit or Rollback return: explicit transactions, transactions opened by
// a BEGIN statement, and autocommit Exec statements alike. The label is the
// calling function's code identifier, so it never carries SQL or data. With
// recording off this is a plain sql.Open.
func openWritableSQL(dsn, domain string) (*sql.DB, error) {
	if !perftrace.Enabled() {
		return sql.Open("sqlite", dsn)
	}
	probe, err := sql.Open("sqlite", "")
	if err != nil {
		return nil, err
	}
	base := probe.Driver()
	if err := probe.Close(); err != nil {
		return nil, err
	}
	return sql.OpenDB(tracedConnector{dsn: dsn, base: base, domain: domain}), nil
}

type tracedConnector struct {
	dsn    string
	base   driver.Driver
	domain string
}

func (c tracedConnector) Connect(context.Context) (driver.Conn, error) {
	conn, err := c.base.Open(c.dsn)
	if err != nil {
		return nil, err
	}
	return &tracedConn{Conn: conn, domain: c.domain}, nil
}

func (c tracedConnector) Driver() driver.Driver { return c.base }

// tracedConn forwards every optional interface the SQLite driver implements,
// so database/sql takes the same code paths as without tracing.
type tracedConn struct {
	driver.Conn
	domain string
	mu     sync.Mutex
	// open is the explicit transaction in progress on this connection.
	open func(error)
}

func (c *tracedConn) begin(skip int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.open == nil {
		c.open = perftrace.Tx(c.domain, callerLabel(skip+1))
	}
}

func (c *tracedConn) end(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.open != nil {
		c.open(err)
		c.open = nil
	}
}

func (c *tracedConn) inTx() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.open != nil
}

func (c *tracedConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	beginner, ok := c.Conn.(driver.ConnBeginTx)
	if !ok {
		return nil, errors.New("store: SQLite driver lacks BeginTx")
	}
	c.begin(2)
	tx, err := beginner.BeginTx(ctx, opts)
	if err != nil {
		c.end(err)
		return nil, err
	}
	return &tracedTx{Tx: tx, conn: c}, nil
}

func (c *tracedConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	execer, ok := c.Conn.(driver.ExecerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	switch statementControl(query) {
	case controlBegin:
		c.begin(2)
		result, err := execer.ExecContext(ctx, query, args)
		if err != nil {
			c.end(err)
		}
		return result, err
	case controlCommit, controlRollback:
		result, err := execer.ExecContext(ctx, query, args)
		recorded := err
		if statementControl(query) == controlRollback && err == nil {
			recorded = errRolledBack
		}
		c.end(recorded)
		return result, err
	}
	if c.inTx() {
		return execer.ExecContext(ctx, query, args)
	}
	// An Exec outside a transaction is its own autocommit write transaction.
	done := perftrace.Tx(c.domain, callerLabel(2))
	result, err := execer.ExecContext(ctx, query, args)
	done(err)
	return result, err
}

var errRolledBack = errors.New("rolled back")

func (c *tracedConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if queryer, ok := c.Conn.(driver.QueryerContext); ok {
		return queryer.QueryContext(ctx, query, args)
	}
	return nil, driver.ErrSkip
}

func (c *tracedConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	if preparer, ok := c.Conn.(driver.ConnPrepareContext); ok {
		return preparer.PrepareContext(ctx, query)
	}
	return c.Conn.Prepare(query)
}

func (c *tracedConn) Ping(ctx context.Context) error {
	if pinger, ok := c.Conn.(driver.Pinger); ok {
		return pinger.Ping(ctx)
	}
	return nil
}

func (c *tracedConn) ResetSession(ctx context.Context) error {
	if resetter, ok := c.Conn.(driver.SessionResetter); ok {
		return resetter.ResetSession(ctx)
	}
	return nil
}

func (c *tracedConn) IsValid() bool {
	if validator, ok := c.Conn.(driver.Validator); ok {
		return validator.IsValid()
	}
	return true
}

func (c *tracedConn) CheckNamedValue(value *driver.NamedValue) error {
	if checker, ok := c.Conn.(driver.NamedValueChecker); ok {
		return checker.CheckNamedValue(value)
	}
	return driver.ErrSkip
}

func (c *tracedConn) Close() error {
	// A connection closed mid-transaction never committed.
	c.end(errRolledBack)
	return c.Conn.Close()
}

type tracedTx struct {
	driver.Tx
	conn *tracedConn
}

func (t *tracedTx) Commit() error {
	err := t.Tx.Commit()
	t.conn.end(err)
	return err
}

func (t *tracedTx) Rollback() error {
	err := t.Tx.Rollback()
	recorded := err
	if err == nil {
		recorded = errRolledBack
	}
	t.conn.end(recorded)
	return err
}

type control int

const (
	controlNone control = iota
	controlBegin
	controlCommit
	controlRollback
)

func statementControl(query string) control {
	fields := strings.Fields(query)
	if len(fields) == 0 {
		return controlNone
	}
	switch strings.ToUpper(strings.TrimSuffix(fields[0], ";")) {
	case "BEGIN":
		return controlBegin
	case "COMMIT", "END":
		return controlCommit
	case "ROLLBACK":
		// ROLLBACK TO <savepoint> keeps the transaction open.
		if len(fields) > 1 && strings.EqualFold(fields[1], "TO") {
			return controlNone
		}
		return controlRollback
	}
	return controlNone
}

const modulePrefix = "github.com/kitdine/agent-deck/internal/"

// callerLabel names the first caller outside database/sql and this file as a
// lowercase identifier, e.g. usage_service_scanfilemode.
func callerLabel(skip int) string {
	pcs := make([]uintptr, 24)
	frames := runtime.CallersFrames(pcs[:runtime.Callers(skip+1, pcs)])
	for {
		frame, more := frames.Next()
		name := frame.Function
		if name != "" && !strings.HasPrefix(name, "database/sql") && !strings.Contains(name, "store.(*traced") &&
			!strings.HasPrefix(name, "modernc.org/") {
			return identifierLabel(strings.TrimPrefix(name, modulePrefix))
		}
		if !more {
			return "unknown"
		}
	}
}

func identifierLabel(name string) string {
	var b strings.Builder
	underscore := false
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			if b.Len() == 0 && r >= '0' && r <= '9' {
				b.WriteByte('f')
			}
			b.WriteRune(r)
			underscore = false
		} else if b.Len() > 0 && !underscore {
			b.WriteByte('_')
			underscore = true
		}
	}
	label := strings.TrimSuffix(b.String(), "_")
	if len(label) > 48 {
		label = strings.TrimSuffix(label[len(label)-48:], "_")
		label = strings.TrimLeft(label, "_0123456789")
	}
	if label == "" {
		return "unknown"
	}
	return label
}
