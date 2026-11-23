package store

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
)

func cjsiInitialIndexRevision(ctx context.Context, q cjsiRowQueryer, seal cjsiIndexSeal) (int64, error) {
	var rev int64
	err := q.QueryRowContext(ctx, `SELECT base_revision FROM main.cjsi_index_state s JOIN main.serving_generation g USING(singleton)
	WHERE s.singleton=1 AND compacted_ticket=0 AND compacted_attempt=0 AND compacted_seal=? AND s.base_revision=g.revision
	AND protection_revision=0`, seal.digest[:]).Scan(&rev)
	return rev, err
}

// Called only under the managed drain, after verification of the installed
// file and before releasing the legacy snapshot. Read the private baseline
// first. Ready + its initial checked component then commit in core together.
func cjsiCommitInitialHead(ctx context.Context, root string, db *sql.DB, prior, next cjsiLayoutJournal) error {
	if next.phase != cjsiLayoutReady || !cjsiValidLayoutAdvance(prior, next) {
		return errCJSILayoutInvalid
	}
	var rev int64
	var digest any
	if next.prepared.present() {
		path := filepath.Join(root, cjsiPrivateIndexName)
		st, err := privateServingPath(path, false)
		if err != nil || cjsiIdentity(st) != next.prepared.identity {
			return errCJSILayoutInvalid
		}
		index, err := sql.Open("sqlite", cjsiSQLiteURI(path, "ro"))
		if err != nil {
			return err
		}
		rev, err = cjsiInitialIndexRevision(ctx, index, next.seal)
		if err = errors.Join(err, index.Close()); err != nil {
			return err
		}
		digest = next.seal.digest[:]
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = cjsiCheckJournalDurability(ctx, tx); err != nil {
		return err
	}
	r, err := tx.ExecContext(ctx, `UPDATE main.cjsi_publication_head SET index_epoch=?,base_revision=?,session_revision=?,
	    base_seal=?,session_seal=? WHERE singleton=1 AND ticket=0 AND index_epoch=0 AND protection_revision=0
	    AND coverage_revision=0 AND core_epoch=(SELECT epoch FROM main.derived_snapshot_generation WHERE singleton=1)`,
		next.epoch, rev, rev, digest, digest)
	if err != nil {
		return err
	}
	n, err := r.RowsAffected()
	if err != nil || n != 1 {
		return errors.Join(errCJSILayoutInvalid, err)
	}
	if err = cjsiAdvanceLayoutRecord(ctx, tx, prior, next); err != nil {
		return err
	}
	return tx.Commit()
}
