package store

import (
	"context"
	"database/sql"
	"errors"
)

// The private index migration owns this table. The initial seal binds to
// the on-disk index epoch; local file identity belongs to the core journal.
const cjsiLayoutSealSchema = `
CREATE TABLE main.cjsi_layout_initial_seal (
    singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
    index_epoch INTEGER NOT NULL CHECK (index_epoch > 0),
    layout_version INTEGER NOT NULL CHECK (layout_version > 0),
    seal_digest BLOB NOT NULL CHECK (length(seal_digest) = 32)
) STRICT;
`

// Commit with the private index schema and epoch. Initialization cannot
// replace a prior seal or bind to an epoch other than the index's own.
func initializeCJSIIndexSeal(ctx context.Context, tx *sql.Tx, epoch int64, seal cjsiIndexSeal) error {
	if tx == nil || epoch <= 0 || !seal.valid() {
		return errCJSILayoutInvalid
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO main.cjsi_layout_initial_seal
		(singleton, index_epoch, layout_version, seal_digest)
		SELECT 1, epoch, ?, ? FROM main.session_index_generation
		WHERE singleton=1 AND typeof(epoch)='integer' AND epoch=?`, int64(seal.version), seal.digest[:], epoch)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return errCJSILayoutInvalid
	}
	return nil
}

func readCJSIIndexSeal(ctx context.Context, tx *sql.Tx) (cjsiIndexSeal, error) {
	if tx == nil {
		return cjsiIndexSeal{}, errCJSILayoutInvalid
	}
	var epoch, version, digest, currentEpoch any
	err := tx.QueryRowContext(ctx, `SELECT s.index_epoch, s.layout_version, s.seal_digest, g.epoch
		FROM main.cjsi_layout_initial_seal s JOIN main.session_index_generation g ON g.singleton=s.singleton
		WHERE s.singleton=1`).Scan(&epoch, &version, &digest, &currentEpoch)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			err = errors.Join(errCJSILayoutInvalid, err)
		}
		return cjsiIndexSeal{}, err
	}
	boundEpoch, seal, err := cjsiDecodeLayoutSeal(epoch, version, digest)
	current, ok := currentEpoch.(int64)
	if err != nil || !ok || current != boundEpoch {
		return cjsiIndexSeal{}, errCJSILayoutInvalid
	}
	return seal, nil
}

func cjsiDecodeLayoutSeal(epoch, version, digest any) (int64, cjsiIndexSeal, error) {
	e, epochOK := epoch.(int64)
	v, versionOK := version.(int64)
	raw, digestOK := digest.([]byte)
	var seal cjsiIndexSeal
	if !epochOK || e <= 0 || !versionOK || v <= 0 || int64(int(v)) != v || !digestOK || len(raw) != len(seal.digest) {
		return 0, seal, errCJSILayoutInvalid
	}
	seal.version = int(v)
	copy(seal.digest[:], raw)
	if !seal.valid() {
		return 0, cjsiIndexSeal{}, errCJSILayoutInvalid
	}
	return e, seal, nil
}
