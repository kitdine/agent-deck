package store

import (
	"context"
	"database/sql"
	"encoding/binary"
	"errors"
	"math"
)

// The numbered core migration owns applying this DDL. The journal records
// installation evidence only. A ready row does not publish a ticket.
const cjsiLayoutJournalSchema = `
CREATE TABLE main.cjsi_layout_journal (
    singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
    phase INTEGER NOT NULL CHECK (phase BETWEEN 1 AND 5),
    root_identity BLOB NOT NULL CHECK (length(root_identity) = 16),
    legacy_file BLOB CHECK (length(legacy_file) = 56),
    snapshot_file BLOB CHECK (length(snapshot_file) = 56),
    fence_identity BLOB CHECK (length(fence_identity) = 16),
    prepared_file BLOB CHECK (length(prepared_file) = 56),
    index_epoch INTEGER CHECK (index_epoch > 0),
    seal_version INTEGER CHECK (seal_version > 0),
    seal_digest BLOB CHECK (length(seal_digest) = 32)
) STRICT;
`

const cjsiLayoutJournalFields = `phase, root_identity, legacy_file, snapshot_file,
	fence_identity, prepared_file, index_epoch, seal_version, seal_digest`

var errCJSILayoutJournalStale = errors.New("Codex session index layout journal changed")

// Call inside the same transaction that installs the journal schema and
// advances the core schema. The caller supplies the guarded physical root.
func initializeCJSILayoutJournal(ctx context.Context, tx *sql.Tx, root cjsiFileIdentity) error {
	j := cjsiLayoutJournal{phase: cjsiFenceInstalling, root: root}
	if tx == nil || !j.valid() {
		return errCJSILayoutInvalid
	}
	if err := cjsiCheckJournalDurability(ctx, tx); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO main.cjsi_layout_journal (singleton, `+cjsiLayoutJournalFields+
		`) VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, cjsiLayoutJournalArgs(j)...)
	return err
}

type cjsiRowQueryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type cjsiSQLiteLayoutJournal struct{ db *sql.DB }

func (s cjsiSQLiteLayoutJournal) load(ctx context.Context) (cjsiLayoutJournal, error) {
	if s.db == nil {
		return cjsiLayoutJournal{}, errCJSILayoutInvalid
	}
	return cjsiReadLayoutJournal(ctx, s.db)
}

func cjsiReadLayoutJournal(ctx context.Context, q cjsiRowQueryer) (cjsiLayoutJournal, error) {
	if q == nil {
		return cjsiLayoutJournal{}, errCJSILayoutInvalid
	}
	var phase, root, legacy, snapshot, fence, prepared, epoch, version, digest any
	err := q.QueryRowContext(ctx, `SELECT `+cjsiLayoutJournalFields+` FROM main.cjsi_layout_journal WHERE singleton=1`).
		Scan(&phase, &root, &legacy, &snapshot, &fence, &prepared, &epoch, &version, &digest)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			err = errors.Join(errCJSILayoutInvalid, err)
		}
		return cjsiLayoutJournal{}, err
	}
	step, ok := phase.(int64)
	if !ok || step < int64(cjsiFenceInstalling) || step > int64(cjsiLayoutReady) {
		return cjsiLayoutJournal{}, errCJSILayoutInvalid
	}

	j := cjsiLayoutJournal{phase: cjsiInstallPhase(step)}
	j.root, err = cjsiDecodeLayoutIdentity(root)
	if err != nil {
		return cjsiLayoutJournal{}, err
	}
	j.fence, err = cjsiDecodeLayoutIdentity(fence)
	if err != nil {
		return cjsiLayoutJournal{}, err
	}
	for _, field := range []struct {
		value any
		file  *cjsiLayoutFile
	}{{legacy, &j.legacy}, {snapshot, &j.snapshot}, {prepared, &j.prepared}} {
		*field.file, err = cjsiDecodeLayoutFile(field.value)
		if err != nil {
			return cjsiLayoutJournal{}, err
		}
	}
	if epoch != nil || version != nil || digest != nil {
		j.epoch, j.seal, err = cjsiDecodeLayoutSeal(epoch, version, digest)
		if err != nil {
			return cjsiLayoutJournal{}, err
		}
	}
	if !j.valid() {
		return cjsiLayoutJournal{}, errCJSILayoutInvalid
	}
	return j, nil
}

// One autocommit statement compares every stored value, including absence.
// A failed commit or stale caller never installs a partial newer record.
func (s cjsiSQLiteLayoutJournal) advance(ctx context.Context, prior, next cjsiLayoutJournal) error {
	if s.db == nil || !cjsiValidLayoutAdvance(prior, next) {
		return errCJSILayoutInvalid
	}
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := cjsiCheckJournalDurability(ctx, conn); err != nil {
		return err
	}
	return cjsiAdvanceLayoutRecord(ctx, conn, prior, next)
}

func cjsiAdvanceLayoutRecord(ctx context.Context, exec sessionGenerationExecutor, prior, next cjsiLayoutJournal) error {
	if exec == nil || !cjsiValidLayoutAdvance(prior, next) {
		return errCJSILayoutInvalid
	}
	args := append(cjsiLayoutJournalArgs(next), cjsiLayoutJournalArgs(prior)...)
	result, err := exec.ExecContext(ctx, `UPDATE main.cjsi_layout_journal SET (`+cjsiLayoutJournalFields+`)
		= (?, ?, ?, ?, ?, ?, ?, ?, ?)
		WHERE singleton=1 AND phase=? AND root_identity=? AND legacy_file IS ?
		  AND snapshot_file IS ? AND fence_identity IS ? AND prepared_file IS ?
		  AND index_epoch IS ? AND seal_version IS ? AND seal_digest IS ?`, args...)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return errCJSILayoutJournalStale
	}
	return nil
}

// Filesystem recovery relies on these commits surviving a restart. A
// connection configured for asynchronous or in-memory journaling cannot
// supply the durability this adapter promises.
func cjsiCheckJournalDurability(ctx context.Context, conn sessionGenerationExecutor) error {
	var synchronous int
	if err := conn.QueryRowContext(ctx, "PRAGMA main.synchronous").Scan(&synchronous); err != nil {
		return err
	}
	if synchronous != 2 && synchronous != 3 { // FULL or EXTRA
		return errCJSILayoutInvalid
	}
	var mode string
	if err := conn.QueryRowContext(ctx, "PRAGMA main.journal_mode").Scan(&mode); err != nil {
		return err
	}
	switch mode {
	case "delete", "truncate", "persist", "wal":
		return nil
	default:
		return errCJSILayoutInvalid
	}
}

func cjsiValidLayoutAdvance(prior, next cjsiLayoutJournal) bool {
	if !prior.valid() || !next.valid() {
		return false
	}
	expected := prior
	switch prior.phase {
	case cjsiFenceInstalling:
		expected.phase = cjsiLegacyCaptured
		expected.legacy, expected.snapshot = next.legacy, next.snapshot
	case cjsiLegacyCaptured:
		expected.phase = cjsiFenceDurable
		expected.fence = next.fence
	case cjsiFenceDurable:
		expected.phase = cjsiLayoutReady
		if prior.legacy.present() {
			expected.phase = cjsiPrivatePrepared
			expected.prepared, expected.epoch, expected.seal = next.prepared, next.epoch, next.seal
		}
	case cjsiPrivatePrepared:
		expected.phase = cjsiLayoutReady
	default:
		return false
	}
	return expected == next
}

func cjsiLayoutJournalArgs(j cjsiLayoutJournal) []any {
	var epoch, version, digest any
	if j.prepared.present() {
		epoch, version, digest = j.epoch, int64(j.seal.version), j.seal.digest[:]
	}
	return []any{
		int64(j.phase), cjsiEncodeLayoutIdentity(j.root),
		cjsiEncodeLayoutFile(j.legacy), cjsiEncodeLayoutFile(j.snapshot),
		cjsiEncodeLayoutIdentity(j.fence), cjsiEncodeLayoutFile(j.prepared),
		epoch, version, digest,
	}
}

// Device and inode use fixed-width unsigned big endian. SQLite INTEGER is
// signed, while local stat identities can use the full uint64 bit pattern.
func cjsiEncodeLayoutIdentity(id cjsiFileIdentity) any {
	if id.inode == 0 {
		return nil
	}
	var raw [16]byte
	cjsiPutLayoutIdentity(raw[:], id)
	return raw[:]
}

func cjsiPutLayoutIdentity(raw []byte, id cjsiFileIdentity) {
	binary.BigEndian.PutUint64(raw[:8], id.device)
	binary.BigEndian.PutUint64(raw[8:], id.inode)
}

func cjsiDecodeLayoutIdentity(value any) (cjsiFileIdentity, error) {
	if value == nil {
		return cjsiFileIdentity{}, nil
	}
	raw, ok := value.([]byte)
	if !ok || len(raw) != 16 {
		return cjsiFileIdentity{}, errCJSILayoutInvalid
	}
	id := cjsiFileIdentity{device: binary.BigEndian.Uint64(raw[:8]), inode: binary.BigEndian.Uint64(raw[8:])}
	if id.inode == 0 {
		return cjsiFileIdentity{}, errCJSILayoutInvalid
	}
	return id, nil
}

func cjsiEncodeLayoutFile(file cjsiLayoutFile) any {
	if !file.present() {
		return nil
	}
	var raw [56]byte
	cjsiPutLayoutIdentity(raw[:16], file.identity)
	binary.BigEndian.PutUint64(raw[16:24], uint64(file.size))
	copy(raw[24:], file.digest[:])
	return raw[:]
}

func cjsiDecodeLayoutFile(value any) (cjsiLayoutFile, error) {
	if value == nil {
		return cjsiLayoutFile{}, nil
	}
	raw, ok := value.([]byte)
	if !ok || len(raw) != 56 {
		return cjsiLayoutFile{}, errCJSILayoutInvalid
	}
	id, err := cjsiDecodeLayoutIdentity(raw[:16])
	size := binary.BigEndian.Uint64(raw[16:24])
	if err != nil || size == 0 || size > math.MaxInt64 {
		return cjsiLayoutFile{}, errCJSILayoutInvalid
	}
	file := cjsiLayoutFile{identity: id, size: int64(size)}
	copy(file.digest[:], raw[24:])
	if !file.valid() {
		return cjsiLayoutFile{}, errCJSILayoutInvalid
	}
	return file, nil
}
