package quota

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

func (s *Store) refreshWrite(ctx context.Context, write func(context.Context, *sql.Conn) (bool, error)) (WriteOutcome, error) {
	return s.writeImmediate(ctx, time.Second, true, write)
}

func (s *Store) quotaWrite(ctx context.Context, limit time.Duration, write func(context.Context, *sql.Conn) (bool, error)) (WriteOutcome, error) {
	return s.writeImmediate(ctx, limit, false, write)
}

// writeImmediate pins the busy limit and the transaction to one connection.
func (s *Store) writeImmediate(ctx context.Context, limit time.Duration, refresh bool, write func(context.Context, *sql.Conn) (bool, error)) (out WriteOutcome, err error) {
	entry := time.Now()
	ctx, cancel := context.WithDeadline(ctx, entry.Add(limit))
	defer cancel()
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return WriteUnresolved, err
	}
	defer conn.Close()
	var oldBusy int
	if err := conn.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&oldBusy); err != nil {
		return WriteUnresolved, err
	}
	deadline, _ := ctx.Deadline()
	// Anchor a parent deadline supplied as wall time to this operation's
	// monotonic entry time, even if acquiring the connection took a while.
	deadline = entry.Add(deadline.Sub(entry))
	var busy int64
	if !refresh {
		busy = max(0, time.Until(deadline).Milliseconds())
	}
	open := false
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), limit)
		defer cancel()
		rollbackErr := error(nil)
		if open {
			_, rollbackErr = conn.ExecContext(cleanup, "ROLLBACK")
		}
		_, restoreErr := conn.ExecContext(cleanup, fmt.Sprintf("PRAGMA busy_timeout=%d", oldBusy))
		if rollbackErr != nil || restoreErr != nil {
			_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		}
	}()
	if _, err := conn.ExecContext(ctx, fmt.Sprintf("PRAGMA busy_timeout=%d", busy)); err != nil {
		return WriteUnresolved, err
	}
	if refresh {
		open, err = beginRefreshWrite(ctx, conn, deadline)
	} else {
		_, err = conn.ExecContext(ctx, "BEGIN IMMEDIATE")
		open = err == nil
	}
	if err != nil {
		return WriteUnresolved, err
	}
	changed, err := write(ctx, conn)
	if err != nil {
		return WriteUnresolved, err
	}
	if !changed {
		return WriteDropped, nil
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return WriteUnresolved, err
	}
	open = false
	return WriteCommitted, nil
}

// beginRefreshWrite returns whether cleanup must roll back. Only a native
// BUSY from BEGIN is retryable. No transaction body has run at that point.
func beginRefreshWrite(ctx context.Context, conn *sql.Conn, deadline time.Time) (bool, error) {
	var lastBusy error
	for {
		if err := refreshWriteCutoff(ctx, deadline, lastBusy); err != nil {
			return false, err
		}
		_, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE")
		if err == nil {
			return true, refreshWriteCutoff(ctx, deadline, lastBusy)
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			// modernc can replace the native result with the context error.
			// Roll back a possible acquisition, or discard the connection if
			// rollback cannot confirm that it is clean.
			if stop := refreshWriteCutoff(ctx, deadline, lastBusy); stop != nil {
				err = stop
			}
			return true, err
		}
		var native *sqlite.Error
		if !errors.As(err, &native) || native.Code()&255 != sqlite3.SQLITE_BUSY {
			return false, err
		}
		lastBusy = err
		select {
		case <-ctx.Done():
		case <-time.After(min(5*time.Millisecond, time.Until(deadline))):
		}
	}
}

func refreshWriteCutoff(ctx context.Context, deadline time.Time, lastBusy error) error {
	err := ctx.Err()
	if err == context.Canceled {
		return err
	}
	if err == context.DeadlineExceeded || !time.Now().Before(deadline) {
		if lastBusy != nil {
			return lastBusy
		}
		return context.DeadlineExceeded
	}
	return nil
}

func (s *Store) BeginRefreshCycle(ctx context.Context, lock *store.Lock, at time.Time) (RefreshCycle, error) {
	if s.core == nil || at.IsZero() {
		return RefreshCycle{}, errors.New("invalid quota refresh store or observation time")
	}
	if err := s.core.CheckQuotaRefreshLock(lock); err != nil {
		return RefreshCycle{}, err
	}
	cycle := RefreshCycle{owner: s, lock: lock, at: at}
	_, err := s.refreshWrite(ctx, func(ctx context.Context, conn *sql.Conn) (bool, error) {
		err := conn.QueryRowContext(ctx, "UPDATE quota_cycle SET seq=seq+1 WHERE singleton=1 RETURNING seq").Scan(&cycle.seq)
		return err == nil, err
	})
	if err != nil {
		return RefreshCycle{}, err
	}
	return cycle, nil
}

func (s *Store) checkCycle(cycle RefreshCycle) error {
	if s.core == nil || cycle.owner != s || cycle.seq <= 0 {
		return errors.New("invalid quota refresh cycle")
	}
	return s.core.CheckQuotaRefreshLock(cycle.lock)
}

func (s *Store) cycleWrite(ctx context.Context, cycle RefreshCycle, write func(context.Context, *sql.Conn) (bool, error)) (WriteOutcome, error) {
	if err := s.checkCycle(cycle); err != nil {
		return WriteUnresolved, err
	}
	return s.refreshWrite(ctx, func(ctx context.Context, conn *sql.Conn) (bool, error) {
		var latest CycleSeq
		if err := conn.QueryRowContext(ctx, "SELECT seq FROM quota_cycle WHERE singleton=1").Scan(&latest); err != nil {
			return false, err
		}
		if latest != cycle.seq {
			return false, nil
		}
		return write(ctx, conn)
	})
}

// Legacy producers still operate on fresh state during the staged rollout.
// Once guarded state exists an unguarded Claude writer cannot replace it.
func legacyQuotaWriteAllowed(ctx context.Context, tx quotaSQL, client Client) (bool, error) {
	if client != ClientClaude {
		return true, nil
	}
	var guarded bool
	err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM quota_envelopes_v2
		WHERE client='claude' AND (context_generation<>0 OR cycle_seq<>0))`).Scan(&guarded)
	return !guarded, err
}

func timeValue(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(timeLayout)
}
