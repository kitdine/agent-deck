package usage

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"time"
)

// beginSourcePublication pins the connection while temporarily disabling SQLite's
// blocking busy handler. Only lock acquisition retries here; no parsed data or
// publication writes are replayed. The original timeout is restored before the
// connection can return to the shared pool, including on context cancellation.
func beginSourcePublication(ctx context.Context, db *sql.DB) (*sql.Tx, func(), error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, nil, err
	}
	var timeout int
	if err = conn.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&timeout); err != nil {
		conn.Close()
		return nil, nil, err
	}
	var tx *sql.Tx
	cleanup := func() {
		if tx != nil {
			_ = tx.Rollback()
		}
		if _, restoreErr := conn.ExecContext(context.Background(), fmt.Sprintf("PRAGMA busy_timeout=%d", timeout)); restoreErr != nil {
			// Never leak an altered timeout into another pool user's transaction.
			_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		}
		_ = conn.Close()
	}
	if _, err = conn.ExecContext(ctx, "PRAGMA busy_timeout=0"); err != nil {
		cleanup()
		return nil, nil, err
	}
	tx, err = conn.BeginTx(ctx, nil)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	deadline := time.Now().Add(time.Duration(timeout) * time.Millisecond)
	for {
		if err = ctx.Err(); err != nil {
			break
		}
		_, err = tx.ExecContext(ctx, "UPDATE usage_source_files SET cursor=cursor WHERE 0")
		if err == nil {
			break
		}
		var coded interface{ Code() int }
		if !errors.As(err, &coded) || coded.Code() != 5 || time.Until(deadline) <= 0 {
			break
		}
		timer := time.NewTimer(min(10*time.Millisecond, time.Until(deadline)))
		select {
		case <-ctx.Done():
			timer.Stop()
			err = ctx.Err()
		case <-timer.C:
		}
		if ctx.Err() != nil {
			err = ctx.Err()
			break
		}
	}
	if err == nil {
		_, err = conn.ExecContext(ctx, fmt.Sprintf("PRAGMA busy_timeout=%d", timeout))
	}
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	return tx, cleanup, nil
}
