package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"

	"github.com/kitdine/agent-deck/internal/perftrace"
	"modernc.org/sqlite"
)

// A Store lease cannot account for a checked-out sql.Conn, Rows, or Tx after
// DB.Close. Pin each real SQLite connection from before driver.Open until
// driver.Close returns successfully. An uncertain close pins until process exit.
type legacyConnector struct {
	base                   driver.Driver
	root                   legacyRootID
	path, dsn, traceDomain string
}

func openLegacySQL(path, dsn, traceDomain string, root legacyRootID) (*sql.DB, error) {
	probe, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	base := probe.Driver() // sql.Open is lazy; no SQLite handle yet.
	if err := probe.Close(); err != nil {
		return nil, err
	}
	if !perftrace.Enabled() {
		traceDomain = ""
	}
	return sql.OpenDB(legacyConnector{base, root, path, dsn, traceDomain}), nil
}

func (c legacyConnector) Driver() driver.Driver { return c.base }

type legacySQLiteMethods interface {
	driver.Conn
	driver.ConnBeginTx
	driver.ConnPrepareContext
	driver.ExecerContext
	driver.QueryerContext
	driver.SessionResetter
	driver.Validator
	driver.Pinger
}

type legacyBackuper interface {
	NewBackup(string) (*sqlite.Backup, error)
}

type legacyDriverConn struct {
	legacySQLiteMethods
	legacyBackuper
	pin legacyAdmission
}

func (c legacyConnector) Connect(ctx context.Context) (driver.Conn, error) {
	pin, err := admitLegacyFile(ctx, c.path)
	if err != nil {
		return nil, err
	}
	if pin.root != c.root {
		return nil, errors.Join(ErrStateBusy, pin.close())
	}
	raw, err := c.base.Open(c.dsn)
	if err != nil {
		return nil, errors.Join(err, pin.close())
	}
	methods, ok := raw.(legacySQLiteMethods)
	backup, hasBackup := raw.(legacyBackuper)
	if !ok || !hasBackup {
		err := errors.New("incompatible legacy SQLite driver")
		if closeErr := raw.Close(); closeErr != nil {
			return nil, errors.Join(err, closeErr)
		}
		return nil, errors.Join(err, pin.close())
	}
	if c.traceDomain != "" {
		methods = &tracedConn{Conn: raw, domain: c.traceDomain}
	}
	return &legacyDriverConn{methods, backup, pin}, nil
}

func (c *legacyDriverConn) Close() error {
	if err := c.legacySQLiteMethods.Close(); err != nil {
		return err
	}
	return c.pin.close()
}
