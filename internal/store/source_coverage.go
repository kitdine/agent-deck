package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

var ErrCoverageIncomplete = errors.New("inventory has uncommitted sources")

// PreserveSourceCoverage snapshots complete JSONL coverage in the caller's
// transaction. Call the returned function after synthetic OTel writes only:
// these never change captured file facts or checkpoints. A pre-existing pending
// head remains pending; a concurrent file writer cannot commit inside this tx.
func PreserveSourceCoverage(ctx context.Context, tx *sql.Tx) (func() error, error) {
	noop := func() error { return nil }
	var present bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name='domain_coverage_heads')`).Scan(&present); err != nil {
		return nil, err
	}
	if !present {
		return noop, nil
	}
	var id string
	err := tx.QueryRowContext(ctx, `SELECT h.inventory_id FROM domain_coverage_heads h JOIN inventory_runs i USING(inventory_id) WHERE h.singleton=1 AND i.state='complete'`).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return noop, nil
	}
	if err != nil {
		return nil, err
	}
	return func() error {
		_, err := tx.ExecContext(ctx, `UPDATE inventory_runs SET state='complete' WHERE inventory_id=? AND state='update_pending' AND EXISTS(SELECT 1 FROM domain_coverage_heads h WHERE h.singleton=1 AND h.inventory_id=?)`, id, id)
		return err
	}, nil
}

type InventorySource struct {
	Path     string
	Identity string
	End      int64
}

type SourceCoverage struct {
	State       string `json:"state"`
	InventoryID string `json:"inventory_id,omitempty"`
	ObservedAt  string `json:"observed_at,omitempty"`
	Sources     int    `json:"sources"`
	Committed   int    `json:"committed"`
}

const inventoryInsertBatch = 128 // 512 parameters, below SQLite's legacy 999 limit.

// AcceptInventory freezes all discovered sources, including low-priority
// history. Acceptance is durable before the first body is read.
//
// The inventory is staged in short transactions of inventoryInsertBatch rows
// and becomes visible only through the final transaction, which moves the
// coverage head. Staged rows are reached through no head: a failure removes
// them, and rows a crash leaves behind stay unreachable like every superseded
// inventory.
func (s *Store) AcceptInventory(ctx context.Context, id string, epoch int64, sources []InventorySource, capturedAt time.Time) error {
	if s.readOnly || id == "" || epoch <= 0 {
		return errors.New("invalid inventory acceptance")
	}
	for _, source := range sources {
		if source.Path == "" || source.Identity == "" || source.End < 0 {
			return errors.New("invalid inventory source")
		}
	}
	if err := s.stageInventory(ctx, id, epoch, sources, capturedAt); err != nil {
		return errors.Join(err, s.discardInventory(ctx, id))
	}
	if err := s.switchCoverageHead(ctx, id); err != nil {
		return errors.Join(err, s.discardInventory(ctx, id))
	}
	return nil
}

func (s *Store) stageInventory(ctx context.Context, id string, epoch int64, sources []InventorySource, capturedAt time.Time) error {
	insertSQL := func(rows int) string {
		return `INSERT INTO source_domain_checkpoints(inventory_id,source_path,identity,captured_end) VALUES ` + strings.Repeat("(?,?,?,?),", rows-1) + "(?,?,?,?)"
	}
	if _, err := s.DB.ExecContext(ctx, `INSERT INTO inventory_runs(inventory_id,epoch,captured_at,source_count,state) VALUES (?,?,?,?,'pending')`, id, epoch, capturedAt.UTC().Format(time.RFC3339Nano), len(sources)); err != nil {
		return err
	}
	args := make([]any, 0, inventoryInsertBatch*4)
	for start := 0; start < len(sources); start += inventoryInsertBatch {
		end := min(start+inventoryInsertBatch, len(sources))
		args = args[:0]
		for _, source := range sources[start:end] {
			args = append(args, id, source.Path, source.Identity, source.End)
		}
		if _, err := s.DB.ExecContext(ctx, insertSQL(end-start), args...); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) switchCoverageHead(ctx context.Context, id string) error {
	_, err := s.DB.ExecContext(ctx, `INSERT INTO domain_coverage_heads(singleton,inventory_id) VALUES (1,?) ON CONFLICT(singleton) DO UPDATE SET inventory_id=excluded.inventory_id`, id)
	return err
}

// discardInventory removes one staged inventory that never became the head.
func (s *Store) discardInventory(ctx context.Context, id string) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var head int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM domain_coverage_heads WHERE inventory_id=?`, id).Scan(&head); err != nil || head != 0 {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM source_domain_checkpoints WHERE inventory_id=?`, id); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM inventory_runs WHERE inventory_id=?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

// CommitSourceCoverage belongs in the same transaction as the source facts and
// parser checkpoint. A rollback must not advance either watermark.
func CommitSourceCoverage(ctx context.Context, tx *sql.Tx, inventoryID, path, identity string, capturedEnd, committedEnd int64) error {
	return commitSourceCoverage(inventoryID, path, identity, capturedEnd, committedEnd, func(args ...any) (sql.Result, error) {
		return tx.ExecContext(ctx, commitSourceCoverageSQL, args...)
	})
}

const commitSourceCoverageSQL = `UPDATE source_domain_checkpoints SET committed_end=?,state=CASE WHEN captured_end=? THEN 'complete' ELSE 'pending' END WHERE inventory_id=? AND source_path=? AND identity=? AND captured_end=?`

// SourceCoverageCommitter reuses SQL preparation within one existing source
// transaction. It neither owns nor changes the transaction's terminal boundary.
type SourceCoverageCommitter struct{ statement *sql.Stmt }

func PrepareSourceCoverage(ctx context.Context, tx *sql.Tx) (*SourceCoverageCommitter, error) {
	statement, err := tx.PrepareContext(ctx, commitSourceCoverageSQL)
	if err != nil {
		return nil, err
	}
	return &SourceCoverageCommitter{statement: statement}, nil
}

func (c *SourceCoverageCommitter) Close() error { return c.statement.Close() }

func (c *SourceCoverageCommitter) Commit(ctx context.Context, inventoryID, path, identity string, capturedEnd, committedEnd int64) error {
	return commitSourceCoverage(inventoryID, path, identity, capturedEnd, committedEnd, func(args ...any) (sql.Result, error) {
		return c.statement.ExecContext(ctx, args...)
	})
}

func commitSourceCoverage(inventoryID, path, identity string, capturedEnd, committedEnd int64, execute func(...any) (sql.Result, error)) error {
	if inventoryID == "" {
		return nil
	}
	if committedEnd < 0 || committedEnd > capturedEnd {
		return errors.New("invalid committed source offset")
	}
	result, err := execute(committedEnd, committedEnd, inventoryID, path, identity, capturedEnd)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return errors.New("source coverage does not match accepted inventory")
	}
	return nil
}

func (s *Store) FinishInventory(ctx context.Context, id, state string) error {
	if state != "complete" && state != "partial" && state != "paused" && state != "failed" {
		return errors.New("invalid inventory terminal state")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if state == "complete" {
		var total, committed, expected int
		if err = tx.QueryRowContext(ctx, `SELECT i.source_count,count(c.source_path),coalesce(sum(c.state='complete' AND c.committed_end=c.captured_end),0) FROM inventory_runs i LEFT JOIN source_domain_checkpoints c ON c.inventory_id=i.inventory_id WHERE i.inventory_id=? GROUP BY i.inventory_id`, id).Scan(&expected, &total, &committed); err != nil {
			return err
		}
		if expected != total || total != committed {
			return ErrCoverageIncomplete
		}
	}
	result, err := tx.ExecContext(ctx, `UPDATE inventory_runs SET state=? WHERE inventory_id=? AND state='pending'`, state, id)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil || n != 1 {
		return errors.New("inventory is not pending")
	}
	// Advance the revision for terminal coverage, including a zero-source run.
	if _, err = tx.ExecContext(ctx, `UPDATE serving_generation SET revision=revision+1 WHERE singleton=1`); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	return s.reclaimSupersededInventories(ctx)
}

// Reclaim in bounded transactions. Keep the head and the most recent complete
// inventory (telemetry's ownership evidence), including while the next head is
// pending. Never remove another acceptance's staged, pending inventory.
func (s *Store) reclaimSupersededInventories(ctx context.Context) error {
	const obsolete = `state<>'pending' AND inventory_id NOT IN (SELECT inventory_id FROM domain_coverage_heads)
 AND inventory_id<>coalesce((SELECT inventory_id FROM inventory_runs WHERE state='complete' ORDER BY captured_at DESC,inventory_id DESC LIMIT 1),'')`
	for {
		tx, err := s.DB.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		checkpoints, err := tx.ExecContext(ctx, `DELETE FROM source_domain_checkpoints WHERE rowid IN (SELECT rowid FROM source_domain_checkpoints WHERE inventory_id IN (SELECT inventory_id FROM inventory_runs WHERE `+obsolete+`) LIMIT ?)`, inventoryInsertBatch)
		if err != nil {
			tx.Rollback()
			return err
		}
		runs, err := tx.ExecContext(ctx, `DELETE FROM inventory_runs WHERE inventory_id IN (SELECT inventory_id FROM inventory_runs WHERE `+obsolete+` AND NOT EXISTS(SELECT 1 FROM source_domain_checkpoints c WHERE c.inventory_id=inventory_runs.inventory_id) LIMIT ?)`, inventoryInsertBatch)
		if err != nil {
			tx.Rollback()
			return err
		}
		if err = tx.Commit(); err != nil {
			return err
		}
		removedCheckpoints, _ := checkpoints.RowsAffected()
		removedRuns, _ := runs.RowsAffected()
		if removedCheckpoints+removedRuns == 0 {
			return nil
		}
	}
}

// ReadSourceCoverage is stored-only. Legacy absence is unknown; SQL failures
// are errors, never manufactured complete coverage.
func (s *Store) ReadSourceCoverage(ctx context.Context) (SourceCoverage, error) {
	var present int
	if err := s.DB.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_master WHERE type='table' AND name='domain_coverage_heads'`).Scan(&present); err != nil {
		return SourceCoverage{}, err
	}
	if present == 0 {
		return SourceCoverage{State: "unknown"}, nil
	}
	var coverage SourceCoverage
	var runState string
	epochTable := "derived_snapshot_generation"
	if filepath.Base(s.path) == "sessions.sqlite3" {
		epochTable = "session_index_generation"
	}
	query := fmt.Sprintf(`SELECT i.inventory_id,i.captured_at,i.source_count,i.state,(SELECT count(*) FROM source_domain_checkpoints c WHERE c.inventory_id=i.inventory_id AND c.state='complete' AND c.committed_end=c.captured_end) FROM domain_coverage_heads h JOIN inventory_runs i ON i.inventory_id=h.inventory_id JOIN %s g ON g.singleton=h.singleton AND g.epoch=i.epoch WHERE h.singleton=1`, epochTable)
	err := s.DB.QueryRowContext(ctx, query).Scan(&coverage.InventoryID, &coverage.ObservedAt, &coverage.Sources, &runState, &coverage.Committed)
	if errors.Is(err, sql.ErrNoRows) {
		return SourceCoverage{State: "unknown"}, nil
	}
	if err != nil {
		return SourceCoverage{}, err
	}
	coverage.State = "partial"
	if runState == "complete" && coverage.Committed == coverage.Sources {
		coverage.State = "complete"
	} else if runState == "update_pending" || (runState == "pending" && coverage.Committed == 0) {
		coverage.State = "update_pending"
	}
	return coverage, nil
}
