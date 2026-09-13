package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/binary"
	"errors"
	"fmt"
)

var ErrDerivedSnapshotGenerationReadOnly = errors.New("derived snapshot generation is read-only")

var generateGenerationEpoch = randomGenerationEpoch

func randomGenerationEpoch() (int64, error) {
	for {
		var value [8]byte
		if _, err := rand.Read(value[:]); err != nil {
			return 0, err
		}
		epoch := int64(binary.BigEndian.Uint64(value[:]) & ((1 << 63) - 1))
		if epoch != 0 {
			return epoch, nil
		}
	}
}

// DerivedSnapshotGeneration identifies the committed core inputs used by a
// derived desktop cache. It is independent from source-processing checkpoints.
type DerivedSnapshotGeneration struct {
	Epoch         int64
	Revision      int64
	Dirty         bool
	SchemaVersion int
}

func (s *Store) DerivedSnapshotGeneration(ctx context.Context) (DerivedSnapshotGeneration, error) {
	var generation DerivedSnapshotGeneration
	var dirty int
	if err := s.DB.QueryRowContext(ctx, `SELECT epoch,revision,dirty FROM derived_snapshot_generation WHERE singleton=1`).Scan(&generation.Epoch, &generation.Revision, &dirty); err != nil {
		return DerivedSnapshotGeneration{}, err
	}
	version, err := s.SchemaVersion(ctx)
	if err != nil {
		return DerivedSnapshotGeneration{}, err
	}
	generation.Dirty = dirty != 0
	generation.SchemaVersion = version
	return generation, nil
}

// BeginDerivedSnapshotGenerationBuild establishes a clean revision before the
// caller reads inputs. The first later application write increments that
// revision and marks it dirty, while further dirty writes coalesce.
func (s *Store) BeginDerivedSnapshotGenerationBuild(ctx context.Context) (DerivedSnapshotGeneration, error) {
	if s.readOnly {
		return DerivedSnapshotGeneration{}, ErrDerivedSnapshotGenerationReadOnly
	}
	var generation DerivedSnapshotGeneration
	var dirty int
	err := s.DB.QueryRowContext(ctx, `UPDATE derived_snapshot_generation SET revision=revision+1,dirty=0 WHERE singleton=1 RETURNING epoch,revision,dirty`).Scan(&generation.Epoch, &generation.Revision, &dirty)
	if err != nil {
		return DerivedSnapshotGeneration{}, err
	}
	version, err := s.SchemaVersion(ctx)
	if err != nil {
		return DerivedSnapshotGeneration{}, err
	}
	generation.Dirty = dirty != 0
	generation.SchemaVersion = version
	return generation, nil
}

// MintDerivedSnapshotEpoch invalidates every generation identity copied from a
// different database lifecycle, such as a portable restore.
func (s *Store) MintDerivedSnapshotEpoch(ctx context.Context) error {
	if s.readOnly {
		return ErrDerivedSnapshotGenerationReadOnly
	}
	epoch, err := generateGenerationEpoch()
	if err != nil {
		return err
	}
	result, err := s.DB.ExecContext(ctx, `UPDATE derived_snapshot_generation SET epoch=?,revision=0,dirty=1 WHERE singleton=1`, epoch)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		return fmt.Errorf("derived snapshot generation rows=%d, want 1", changed)
	}
	return nil
}

type sessionGenerationExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func ensureSessionIndexGeneration(ctx context.Context, executor sessionGenerationExecutor) error {
	if _, err := executor.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS session_index_generation (
  singleton INTEGER PRIMARY KEY CHECK(singleton = 1),
  epoch INTEGER NOT NULL CHECK(epoch > 0)
)`); err != nil {
		return err
	}
	var count int
	if err := executor.QueryRowContext(ctx, `SELECT count(*) FROM session_index_generation`).Scan(&count); err != nil {
		return err
	}
	if count == 1 {
		return nil
	}
	if count != 0 {
		return fmt.Errorf("session index generation rows=%d, want at most 1", count)
	}
	epoch, err := generateGenerationEpoch()
	if err != nil {
		return err
	}
	_, err = executor.ExecContext(ctx, `INSERT INTO session_index_generation(singleton,epoch) VALUES (1,?)`, epoch)
	return err
}

// MintSessionIndexEpoch gives a rebuilt or restored session index a new
// checkpoint identity. Callers may pass either the owning database or the
// transaction that atomically replaces its source rows.
func MintSessionIndexEpoch(ctx context.Context, executor sessionGenerationExecutor) (int64, error) {
	if err := ensureSessionIndexGeneration(ctx, executor); err != nil {
		return 0, err
	}
	epoch, err := generateGenerationEpoch()
	if err != nil {
		return 0, err
	}
	result, err := executor.ExecContext(ctx, `UPDATE session_index_generation SET epoch=? WHERE singleton=1`, epoch)
	if err != nil {
		return 0, err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	if changed != 1 {
		return 0, fmt.Errorf("session index generation rows=%d, want 1", changed)
	}
	return epoch, nil
}

func (s *Store) SessionIndexEpoch(ctx context.Context) (int64, error) {
	var epoch int64
	err := s.DB.QueryRowContext(ctx, `SELECT epoch FROM session_index_generation WHERE singleton=1`).Scan(&epoch)
	return epoch, err
}
