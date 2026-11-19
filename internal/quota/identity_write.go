package quota

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
)

// quotaWrite pins the busy limit and the immediate transaction to one
// connection. A failed cleanup must not return altered state to the pool.
func (s *Store) quotaWrite(ctx context.Context, limit time.Duration, write func(context.Context, *sql.Conn) (bool, error)) (out WriteOutcome, err error) {
	ctx, cancel := context.WithTimeout(ctx, limit)
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
	busy := max(0, time.Until(deadline).Milliseconds())
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
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return WriteUnresolved, err
	}
	open = true
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

func (s *Store) BeginRefreshCycle(ctx context.Context, lock *store.Lock, at time.Time) (RefreshCycle, error) {
	if s.core == nil || at.IsZero() {
		return RefreshCycle{}, errors.New("invalid quota refresh store or observation time")
	}
	if err := s.core.CheckQuotaRefreshLock(lock); err != nil {
		return RefreshCycle{}, err
	}
	cycle := RefreshCycle{owner: s, lock: lock, at: at}
	_, err := s.quotaWrite(ctx, time.Second, func(ctx context.Context, conn *sql.Conn) (bool, error) {
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
	return s.quotaWrite(ctx, time.Second, func(ctx context.Context, conn *sql.Conn) (bool, error) {
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
