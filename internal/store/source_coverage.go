package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"time"
)

var ErrCoverageIncomplete = errors.New("inventory has uncommitted sources")

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

// AcceptInventory freezes all discovered sources, including low-priority
// history. Acceptance is durable before the first body is read.
func (s *Store) AcceptInventory(ctx context.Context, id string, epoch int64, sources []InventorySource, capturedAt time.Time) error {
	if s.readOnly || id == "" || epoch <= 0 {
		return errors.New("invalid inventory acceptance")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `INSERT INTO inventory_runs(inventory_id,epoch,captured_at,source_count,state) VALUES (?,?,?,?,'pending')`, id, epoch, capturedAt.UTC().Format(time.RFC3339Nano), len(sources)); err != nil {
		return err
	}
	for _, source := range sources {
		if source.Path == "" || source.Identity == "" || source.End < 0 {
			return errors.New("invalid inventory source")
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO source_domain_checkpoints(inventory_id,source_path,identity,captured_end) VALUES (?,?,?,?)`, id, source.Path, source.Identity, source.End); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO domain_coverage_heads(singleton,inventory_id) VALUES (1,?) ON CONFLICT(singleton) DO UPDATE SET inventory_id=excluded.inventory_id`, id); err != nil {
		return err
	}
	return tx.Commit()
}

// CommitSourceCoverage belongs in the same transaction as the source facts and
// parser checkpoint. A rollback must not advance either watermark.
func CommitSourceCoverage(ctx context.Context, tx *sql.Tx, inventoryID, path, identity string, capturedEnd, committedEnd int64) error {
	if inventoryID == "" {
		return nil
	}
	if committedEnd < 0 || committedEnd > capturedEnd {
		return errors.New("invalid committed source offset")
	}
	result, err := tx.ExecContext(ctx, `UPDATE source_domain_checkpoints SET committed_end=?,state=CASE WHEN captured_end=? THEN 'complete' ELSE 'pending' END WHERE inventory_id=? AND source_path=? AND identity=? AND captured_end=?`, committedEnd, committedEnd, inventoryID, path, identity, capturedEnd)
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
	return tx.Commit()
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
