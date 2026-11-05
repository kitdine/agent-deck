package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// OriginSchemaVersion is the core migration that adds session-origin storage
// (auxiliary-session-classification architecture §4). A read-only core opened
// below this version has never been able to hold a preference or an override.
const OriginSchemaVersion = 34

// Session-origin storage. User preferences and overrides are durable user
// facts; session_origins, origin_evidence and origin_adapter_runs are
// rebuildable derivations; origin_mutations holds idempotency receipts only.
// Every table except origin_mutations advances serving_generation.revision and
// marks the derived snapshot dirty, so a no_change receipt never publishes.
// Keyed tables also enqueue the existing session partition invalidation.
// telemetry_sessions joins the same family: classification depends on it and
// must not rely on its writes happening to share a usage transaction.
const createSessionOriginStorage = `CREATE TABLE origin_preferences (
 singleton INTEGER PRIMARY KEY CHECK(singleton = 1),
 scope TEXT NOT NULL CHECK(scope IN ('work','auxiliary','all')),
 revision INTEGER NOT NULL CHECK(revision > 0),
 updated_at TEXT NOT NULL
);
CREATE TABLE origin_overrides (
 client TEXT NOT NULL, session_id TEXT NOT NULL,
 binding_digest TEXT NOT NULL,
 binding_limited INTEGER NOT NULL CHECK(binding_limited IN (0,1)),
 binding_state TEXT NOT NULL CHECK(binding_state IN ('matched','unmatched')),
 value TEXT CHECK(value IS NULL OR value IN ('work','auxiliary')),
 revision INTEGER NOT NULL CHECK(revision > 0),
 updated_at TEXT NOT NULL,
 PRIMARY KEY(client, session_id)
);
CREATE INDEX origin_overrides_effective ON origin_overrides(client, session_id) WHERE value IS NOT NULL;
CREATE TABLE origin_mutations (
 request_id TEXT PRIMARY KEY,
 payload_hash TEXT NOT NULL,
 kind TEXT NOT NULL CHECK(kind IN ('override_set','override_clear','scope_set')),
 target_client TEXT NOT NULL, target_session_id TEXT NOT NULL,
 outcome TEXT NOT NULL CHECK(outcome IN ('saved','no_change')),
 value TEXT,
 revision INTEGER NOT NULL CHECK(revision >= 0),
 recorded_at TEXT NOT NULL
);
CREATE TABLE session_origins (
 client TEXT NOT NULL, session_id TEXT NOT NULL,
 automatic TEXT NOT NULL CHECK(automatic IN ('work','auxiliary','unknown')),
 reason_code TEXT NOT NULL CHECK(reason_code IN ('maintenance_origin','origin_metadata_missing','task_child_unconfirmed','conflicting_origin','origin_source_unavailable','origin_rule_unsupported','origin_input_mismatch')),
 rule_version TEXT NOT NULL,
 input_token TEXT NOT NULL,
 evidence_revision INTEGER NOT NULL CHECK(evidence_revision >= 0),
 PRIMARY KEY(client, session_id)
);
CREATE TABLE origin_evidence (
 adapter TEXT NOT NULL, rule_version TEXT NOT NULL,
 client TEXT NOT NULL, session_id TEXT NOT NULL,
 source_token TEXT NOT NULL,
 purpose TEXT NOT NULL CHECK(purpose IN ('work','auxiliary')),
 validated_version TEXT NOT NULL,
 observed_at TEXT NOT NULL,
 PRIMARY KEY(adapter, client, session_id, source_token)
);
CREATE INDEX origin_evidence_session ON origin_evidence(client, session_id);
CREATE TABLE origin_adapter_runs (
 adapter TEXT PRIMARY KEY,
 outcome TEXT NOT NULL CHECK(outcome IN ('ok','partial','unavailable','unsupported')),
 reason TEXT,
 validated_version TEXT,
 finished_at TEXT NOT NULL
);`

var (
	sessionOriginKeyedTables   = []string{"origin_overrides", "session_origins", "origin_evidence", "telemetry_sessions"}
	sessionOriginUnkeyedTables = []string{"origin_preferences", "origin_adapter_runs"}
)

func initializeSessionOriginStorage(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, createSessionOriginStorage); err != nil {
		return err
	}
	// One trigger per table and operation, like installUsagePartitionTrigger:
	// a second family would be parsed again by every read-only serving open.
	for _, table := range append(append([]string{}, sessionOriginKeyedTables...), sessionOriginUnkeyedTables...) {
		keyed := table != "origin_preferences" && table != "origin_adapter_runs"
		for _, op := range []string{"INSERT", "UPDATE", "DELETE"} {
			body := "UPDATE serving_generation SET revision=revision+1 WHERE singleton=1;"
			if keyed {
				body += "UPDATE usage_derived_metadata SET revision=revision+1 WHERE singleton=1;"
				for _, alias := range []string{"OLD", "NEW"} {
					if (alias == "OLD" && op == "INSERT") || (alias == "NEW" && op == "DELETE") {
						continue
					}
					body += fmt.Sprintf("INSERT INTO usage_derived_invalidations(kind,client,key) VALUES('session',%s.client,%s.session_id) ON CONFLICT(kind,client,key) DO NOTHING;", alias, alias)
				}
			}
			if err := installUsagePartitionTrigger(ctx, tx, table, op, "", body); err != nil {
				return err
			}
		}
	}
	return nil
}

// Origin mutation kinds and outcomes are the stored receipt vocabulary.
const (
	OriginMutationOverrideSet   = "override_set"
	OriginMutationOverrideClear = "override_clear"
	OriginMutationScopeSet      = "scope_set"
	OriginOutcomeSaved          = "saved"
	OriginOutcomeNoChange       = "no_change"
)

var (
	// ErrOriginRequestMismatch reports a request ID reused with another payload.
	ErrOriginRequestMismatch = errors.New("origin request id reused with a different payload")
	// ErrOriginSecureFilesFailed follows a committed mutation whose permission
	// hardening failed. The receipt returned with it is durable.
	ErrOriginSecureFilesFailed = errors.New("origin mutation persisted but securing its file permissions failed")
	errOriginInvalidMutation   = errors.New("invalid origin mutation")
)

// OriginConflictError is a compare-and-set failure against the current revision.
type OriginConflictError struct{ Expected, Current int64 }

func (e *OriginConflictError) Error() string {
	return fmt.Sprintf("origin revision is %d, expected %d", e.Current, e.Expected)
}

// OriginCommitUnknownError means COMMIT itself failed: the mutation may or may
// not be durable, and only the request receipt can tell.
type OriginCommitUnknownError struct{ Err error }

func (e *OriginCommitUnknownError) Error() string {
	return "origin commit outcome unknown: " + e.Err.Error()
}
func (e *OriginCommitUnknownError) Unwrap() error { return e.Err }

// OriginBinding is a verified stable-birth binding; it never contains a path.
type OriginBinding struct {
	Digest  string
	Limited bool
}

// OriginMutation is one idempotent write. PayloadHash covers everything the
// caller considers the request's payload, including ExpectedRevision.
type OriginMutation struct {
	RequestID        string
	PayloadHash      string
	Kind             string
	Client           string
	SessionID        string
	Value            string
	Binding          OriginBinding
	ExpectedRevision int64
	Now              time.Time
}

// OriginReceipt describes the recorded operation, not necessarily current state.
type OriginReceipt struct {
	RequestID  string
	Kind       string
	Client     string
	SessionID  string
	Outcome    string
	Value      *string
	Revision   int64
	RecordedAt string
	Replayed   bool
}

// OriginOverride is one override row; Value is nil for a tombstone.
type OriginOverride struct {
	Client         string
	SessionID      string
	Value          *string
	BindingDigest  string
	BindingLimited bool
	BindingState   string
	Revision       int64
	UpdatedAt      string
}

// OriginPreference is the stored default scope; Present is false only after a
// successful read that found no configuration.
type OriginPreference struct {
	Scope    string
	Revision int64
	Present  bool
}

// originMutationFault injects failures at named mutation stages in tests.
var originMutationFault func(stage string) error

func injectOriginFault(stage string) error {
	if originMutationFault == nil {
		return nil
	}
	return originMutationFault(stage)
}

// originStorageAbsent reports a successful read of a core that predates
// origin storage; such a database cannot hold any origin fact.
func (s *Store) originStorageAbsent(ctx context.Context) (bool, error) {
	version, err := s.SchemaVersion(ctx)
	if err != nil {
		return false, err
	}
	return version < OriginSchemaVersion, nil
}

// OriginPreference reads the default scope without creating or migrating state.
func (s *Store) OriginPreference(ctx context.Context) (OriginPreference, error) {
	absent, err := s.originStorageAbsent(ctx)
	if err != nil || absent {
		return OriginPreference{}, err
	}
	var preference OriginPreference
	err = s.DB.QueryRowContext(ctx, `SELECT scope,revision FROM origin_preferences WHERE singleton=1`).Scan(&preference.Scope, &preference.Revision)
	if errors.Is(err, sql.ErrNoRows) {
		return OriginPreference{}, nil
	}
	if err != nil {
		return OriginPreference{}, err
	}
	preference.Present = true
	return preference, nil
}

// OriginOverrideRecord reads one override row, tombstones included. found is
// false when the key has never had an override.
func (s *Store) OriginOverrideRecord(ctx context.Context, client, sessionID string) (OriginOverride, bool, error) {
	absent, err := s.originStorageAbsent(ctx)
	if err != nil || absent {
		return OriginOverride{}, false, err
	}
	row := s.DB.QueryRowContext(ctx, `SELECT client,session_id,value,binding_digest,binding_limited,binding_state,revision,updated_at FROM origin_overrides WHERE client=? AND session_id=?`, client, sessionID)
	override, err := scanOriginOverride(row)
	if errors.Is(err, sql.ErrNoRows) {
		return OriginOverride{}, false, nil
	}
	return override, err == nil, err
}

// OriginOverrides pages effective overrides (value set, unmatched included)
// ordered by client and session_id, and returns their total count.
func (s *Store) OriginOverrides(ctx context.Context, offset, limit int) ([]OriginOverride, int, error) {
	absent, err := s.originStorageAbsent(ctx)
	if err != nil || absent {
		return nil, 0, err
	}
	var total int
	if err := s.DB.QueryRowContext(ctx, `SELECT count(*) FROM origin_overrides WHERE value IS NOT NULL`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT client,session_id,value,binding_digest,binding_limited,binding_state,revision,updated_at FROM origin_overrides WHERE value IS NOT NULL ORDER BY client,session_id LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []OriginOverride
	for rows.Next() {
		override, err := scanOriginOverride(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, override)
	}
	return out, total, rows.Err()
}

type rowScanner interface{ Scan(...any) error }

func scanOriginOverride(row rowScanner) (OriginOverride, error) {
	var override OriginOverride
	var value sql.NullString
	var limited int
	if err := row.Scan(&override.Client, &override.SessionID, &value, &override.BindingDigest, &limited, &override.BindingState, &override.Revision, &override.UpdatedAt); err != nil {
		return OriginOverride{}, err
	}
	if value.Valid {
		override.Value = &value.String
	}
	override.BindingLimited = limited != 0
	return override, nil
}

// OriginReceipt reads a recorded request outcome. found is false when no
// mutation with this ID ever committed.
func (s *Store) OriginReceipt(ctx context.Context, requestID string) (OriginReceipt, bool, error) {
	absent, err := s.originStorageAbsent(ctx)
	if err != nil || absent {
		return OriginReceipt{}, false, err
	}
	receipt, _, err := readOriginReceipt(ctx, s.DB, requestID)
	if errors.Is(err, sql.ErrNoRows) {
		return OriginReceipt{}, false, nil
	}
	return receipt, err == nil, err
}

type originQueryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func readOriginReceipt(ctx context.Context, q originQueryer, requestID string) (OriginReceipt, string, error) {
	receipt := OriginReceipt{RequestID: requestID}
	var hash string
	var value sql.NullString
	err := q.QueryRowContext(ctx, `SELECT payload_hash,kind,target_client,target_session_id,outcome,value,revision,recorded_at FROM origin_mutations WHERE request_id=?`, requestID).
		Scan(&hash, &receipt.Kind, &receipt.Client, &receipt.SessionID, &receipt.Outcome, &value, &receipt.Revision, &receipt.RecordedAt)
	if err != nil {
		return OriginReceipt{}, "", err
	}
	if value.Valid {
		receipt.Value = &value.String
	}
	return receipt, hash, nil
}

// ApplyOriginMutation runs one mutation in a short IMMEDIATE transaction:
// receipt lookup, payload check, CAS, no-change test, write, receipt, commit.
// The write lock is taken before the first read, so a concurrent committer
// cannot invalidate this transaction's snapshot between its CAS and write.
func (s *Store) ApplyOriginMutation(ctx context.Context, m OriginMutation) (OriginReceipt, error) {
	if s.readOnly {
		return OriginReceipt{}, ErrDerivedSnapshotGenerationReadOnly
	}
	if err := validateOriginMutation(m); err != nil {
		return OriginReceipt{}, err
	}
	conn, err := s.DB.Conn(ctx)
	if err != nil {
		return OriginReceipt{}, err
	}
	defer conn.Close()
	if _, err = conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return OriginReceipt{}, err
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(context.WithoutCancel(ctx), "ROLLBACK")
		}
	}()
	receipt, err := applyOriginMutation(ctx, conn, m)
	if err != nil {
		return OriginReceipt{}, err
	}
	if receipt.Replayed {
		// A replay wrote nothing; release the reservation without a commit
		// boundary that could be reported as unknown.
		return receipt, nil
	}
	if err = injectOriginFault("before_commit"); err != nil {
		return OriginReceipt{}, err
	}
	_, err = conn.ExecContext(ctx, "COMMIT")
	if err == nil {
		committed = true
		err = injectOriginFault("commit")
	}
	if err != nil {
		return OriginReceipt{}, &OriginCommitUnknownError{Err: err}
	}
	if err = s.secureFiles(); err != nil {
		return receipt, fmt.Errorf("%w: %v", ErrOriginSecureFilesFailed, err)
	}
	return receipt, nil
}

func validateOriginMutation(m OriginMutation) error {
	if m.RequestID == "" || m.PayloadHash == "" || m.ExpectedRevision < 0 {
		return errOriginInvalidMutation
	}
	switch m.Kind {
	case OriginMutationOverrideSet:
		if m.Client == "" || m.SessionID == "" || (m.Value != "work" && m.Value != "auxiliary") || m.Binding.Digest == "" {
			return errOriginInvalidMutation
		}
	case OriginMutationOverrideClear:
		if m.Client == "" || m.SessionID == "" || m.Value != "" {
			return errOriginInvalidMutation
		}
	case OriginMutationScopeSet:
		if m.Client != "" || m.SessionID != "" || (m.Value != "work" && m.Value != "auxiliary" && m.Value != "all") {
			return errOriginInvalidMutation
		}
	default:
		return errOriginInvalidMutation
	}
	return nil
}

type originConn interface {
	originQueryer
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func applyOriginMutation(ctx context.Context, conn originConn, m OriginMutation) (OriginReceipt, error) {
	stored, hash, err := readOriginReceipt(ctx, conn, m.RequestID)
	if err == nil {
		if hash != m.PayloadHash {
			return OriginReceipt{}, ErrOriginRequestMismatch
		}
		stored.Replayed = true
		return stored, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return OriginReceipt{}, err
	}
	now := m.Now.UTC().Format(time.RFC3339Nano)
	receipt := OriginReceipt{RequestID: m.RequestID, Kind: m.Kind, Client: m.Client, SessionID: m.SessionID, RecordedAt: now}
	if m.Kind == OriginMutationScopeSet {
		var scope string
		var current int64
		err = conn.QueryRowContext(ctx, `SELECT scope,revision FROM origin_preferences WHERE singleton=1`).Scan(&scope, &current)
		if errors.Is(err, sql.ErrNoRows) {
			// A successfully confirmed missing row is the effective default
			// Work at revision 0, exactly as the read path reports it.
			scope, current = "work", 0
		} else if err != nil {
			return OriginReceipt{}, err
		}
		if m.ExpectedRevision != current {
			return OriginReceipt{}, &OriginConflictError{Expected: m.ExpectedRevision, Current: current}
		}
		value := m.Value
		receipt.Value, receipt.Revision, receipt.Outcome = &value, current, OriginOutcomeNoChange
		if scope != m.Value {
			receipt.Revision, receipt.Outcome = current+1, OriginOutcomeSaved
			if _, err = conn.ExecContext(ctx, `INSERT INTO origin_preferences(singleton,scope,revision,updated_at) VALUES(1,?,?,?) ON CONFLICT(singleton) DO UPDATE SET scope=excluded.scope,revision=excluded.revision,updated_at=excluded.updated_at`, m.Value, receipt.Revision, now); err != nil {
				return OriginReceipt{}, err
			}
		}
		return receipt, recordOriginReceipt(ctx, conn, m, receipt)
	}

	row := conn.QueryRowContext(ctx, `SELECT client,session_id,value,binding_digest,binding_limited,binding_state,revision,updated_at FROM origin_overrides WHERE client=? AND session_id=?`, m.Client, m.SessionID)
	current, err := scanOriginOverride(row)
	found := err == nil
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return OriginReceipt{}, err
	}
	if m.ExpectedRevision != current.Revision {
		return OriginReceipt{}, &OriginConflictError{Expected: m.ExpectedRevision, Current: current.Revision}
	}
	receipt.Revision, receipt.Outcome = current.Revision, OriginOutcomeNoChange
	if m.Kind == OriginMutationOverrideClear {
		// No effective override: never create a row, tombstone or session.
		if found && current.Value != nil {
			receipt.Revision, receipt.Outcome = current.Revision+1, OriginOutcomeSaved
			if _, err = conn.ExecContext(ctx, `UPDATE origin_overrides SET value=NULL,revision=?,updated_at=? WHERE client=? AND session_id=?`, receipt.Revision, now, m.Client, m.SessionID); err != nil {
				return OriginReceipt{}, err
			}
		}
		return receipt, recordOriginReceipt(ctx, conn, m, receipt)
	}

	value := m.Value
	receipt.Value = &value
	same := found && current.Value != nil && *current.Value == m.Value && current.BindingDigest == m.Binding.Digest &&
		current.BindingLimited == m.Binding.Limited && current.BindingState == "matched"
	if !same {
		receipt.Revision, receipt.Outcome = current.Revision+1, OriginOutcomeSaved
		if _, err = conn.ExecContext(ctx, `INSERT INTO origin_overrides(client,session_id,binding_digest,binding_limited,binding_state,value,revision,updated_at) VALUES(?,?,?,?,'matched',?,?,?)
 ON CONFLICT(client,session_id) DO UPDATE SET binding_digest=excluded.binding_digest,binding_limited=excluded.binding_limited,binding_state='matched',value=excluded.value,revision=excluded.revision,updated_at=excluded.updated_at`,
			m.Client, m.SessionID, m.Binding.Digest, boolInt(m.Binding.Limited), m.Value, receipt.Revision, now); err != nil {
			return OriginReceipt{}, err
		}
	}
	return receipt, recordOriginReceipt(ctx, conn, m, receipt)
}

func recordOriginReceipt(ctx context.Context, conn originConn, m OriginMutation, receipt OriginReceipt) error {
	var value any
	if receipt.Value != nil {
		value = *receipt.Value
	}
	_, err := conn.ExecContext(ctx, `INSERT INTO origin_mutations(request_id,payload_hash,kind,target_client,target_session_id,outcome,value,revision,recorded_at) VALUES(?,?,?,?,?,?,?,?,?)`,
		m.RequestID, m.PayloadHash, m.Kind, m.Client, m.SessionID, receipt.Outcome, value, receipt.Revision, receipt.RecordedAt)
	return err
}

// SessionOrigin is the rebuildable automatic classification of one canonical key.
type SessionOrigin struct {
	Client           string
	SessionID        string
	Automatic        string
	ReasonCode       string
	RuleVersion      string
	InputToken       string
	EvidenceRevision int64
}

// OriginEvidence is one adapter's validated purpose for a canonical key.
type OriginEvidence struct {
	Adapter          string
	RuleVersion      string
	Client           string
	SessionID        string
	SourceToken      string
	Purpose          string
	ValidatedVersion string
	ObservedAt       string
}

// OriginAdapterRun summarizes one adapter's latest coordination phase.
type OriginAdapterRun struct {
	Adapter          string
	Outcome          string
	Reason           *string
	ValidatedVersion *string
	FinishedAt       string
}

// OriginDerivation replaces the derived origin state of a bounded candidate
// set in one transaction. Evidence of Adapter and origins of the candidate
// keys are replaced as a whole; a nil Run leaves the adapter summary as is.
type OriginDerivation struct {
	Adapter    string
	Client     string
	SessionIDs []string
	Evidence   []OriginEvidence
	Origins    []SessionOrigin
	Run        *OriginAdapterRun
}

// ReplaceOriginDerivation writes evidence, classifications and the adapter run
// summary atomically; a failure leaves no partial summary.
func (s *Store) ReplaceOriginDerivation(ctx context.Context, d OriginDerivation) error {
	if s.readOnly {
		return ErrDerivedSnapshotGenerationReadOnly
	}
	for _, item := range d.Evidence {
		if item.Adapter != d.Adapter || item.Client != d.Client {
			return errOriginInvalidMutation
		}
	}
	for _, item := range d.Origins {
		if item.Client != d.Client {
			return errOriginInvalidMutation
		}
	}
	if d.Run != nil && d.Run.Adapter != d.Adapter {
		return errOriginInvalidMutation
	}
	conn, err := s.DB.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err = conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(context.WithoutCancel(ctx), "ROLLBACK")
		}
	}()
	for start := 0; start < len(d.SessionIDs); start += 200 {
		batch := d.SessionIDs[start:min(start+200, len(d.SessionIDs))]
		args := []any{d.Adapter, d.Client}
		for _, id := range batch {
			args = append(args, id)
		}
		in := "(?" + strings.Repeat(",?", len(batch)-1) + ")"
		if _, err = conn.ExecContext(ctx, `DELETE FROM origin_evidence WHERE adapter=? AND client=? AND session_id IN `+in, args...); err != nil {
			return err
		}
		if _, err = conn.ExecContext(ctx, `DELETE FROM session_origins WHERE client=? AND session_id IN `+in, args[1:]...); err != nil {
			return err
		}
	}
	for _, item := range d.Evidence {
		if _, err = conn.ExecContext(ctx, `INSERT INTO origin_evidence(adapter,rule_version,client,session_id,source_token,purpose,validated_version,observed_at) VALUES(?,?,?,?,?,?,?,?)`,
			item.Adapter, item.RuleVersion, item.Client, item.SessionID, item.SourceToken, item.Purpose, item.ValidatedVersion, item.ObservedAt); err != nil {
			return err
		}
	}
	for _, item := range d.Origins {
		if _, err = conn.ExecContext(ctx, `INSERT INTO session_origins(client,session_id,automatic,reason_code,rule_version,input_token,evidence_revision) VALUES(?,?,?,?,?,?,?)`,
			item.Client, item.SessionID, item.Automatic, item.ReasonCode, item.RuleVersion, item.InputToken, item.EvidenceRevision); err != nil {
			return err
		}
	}
	if d.Run != nil {
		if _, err = conn.ExecContext(ctx, `INSERT INTO origin_adapter_runs(adapter,outcome,reason,validated_version,finished_at) VALUES(?,?,?,?,?)
 ON CONFLICT(adapter) DO UPDATE SET outcome=excluded.outcome,reason=excluded.reason,validated_version=excluded.validated_version,finished_at=excluded.finished_at`,
			d.Run.Adapter, d.Run.Outcome, nullableOriginString(d.Run.Reason), nullableOriginString(d.Run.ValidatedVersion), d.Run.FinishedAt); err != nil {
			return err
		}
	}
	if err = injectOriginFault("derivation_before_commit"); err != nil {
		return err
	}
	if _, err = conn.ExecContext(ctx, "COMMIT"); err != nil {
		return err
	}
	committed = true
	return s.secureFiles()
}

// SessionOrigins reads stored classifications for candidate keys, 200 per batch.
func (s *Store) SessionOrigins(ctx context.Context, client string, sessionIDs []string) (map[string]SessionOrigin, error) {
	out := map[string]SessionOrigin{}
	absent, err := s.originStorageAbsent(ctx)
	if err != nil || absent {
		return out, err
	}
	err = queryOriginBatches(ctx, s.DB, `SELECT client,session_id,automatic,reason_code,rule_version,input_token,evidence_revision FROM session_origins WHERE client=? AND session_id IN `, client, sessionIDs, func(rows *sql.Rows) error {
		var item SessionOrigin
		if err := rows.Scan(&item.Client, &item.SessionID, &item.Automatic, &item.ReasonCode, &item.RuleVersion, &item.InputToken, &item.EvidenceRevision); err != nil {
			return err
		}
		out[item.SessionID] = item
		return nil
	})
	return out, err
}

// OriginEvidenceFor reads all adapters' evidence for candidate keys.
func (s *Store) OriginEvidenceFor(ctx context.Context, client string, sessionIDs []string) (map[string][]OriginEvidence, error) {
	out := map[string][]OriginEvidence{}
	absent, err := s.originStorageAbsent(ctx)
	if err != nil || absent {
		return out, err
	}
	err = queryOriginBatches(ctx, s.DB, `SELECT adapter,rule_version,client,session_id,source_token,purpose,validated_version,observed_at FROM origin_evidence WHERE client=? AND session_id IN `, client, sessionIDs, func(rows *sql.Rows) error {
		var item OriginEvidence
		if err := rows.Scan(&item.Adapter, &item.RuleVersion, &item.Client, &item.SessionID, &item.SourceToken, &item.Purpose, &item.ValidatedVersion, &item.ObservedAt); err != nil {
			return err
		}
		out[item.SessionID] = append(out[item.SessionID], item)
		return nil
	})
	return out, err
}

// OriginAdapterRunFor reads an adapter's latest run summary.
func (s *Store) OriginAdapterRunFor(ctx context.Context, adapter string) (OriginAdapterRun, bool, error) {
	absent, err := s.originStorageAbsent(ctx)
	if err != nil || absent {
		return OriginAdapterRun{}, false, err
	}
	run := OriginAdapterRun{Adapter: adapter}
	var reason, version sql.NullString
	err = s.DB.QueryRowContext(ctx, `SELECT outcome,reason,validated_version,finished_at FROM origin_adapter_runs WHERE adapter=?`, adapter).Scan(&run.Outcome, &reason, &version, &run.FinishedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return OriginAdapterRun{}, false, nil
	}
	if err != nil {
		return OriginAdapterRun{}, false, err
	}
	if reason.Valid {
		run.Reason = &reason.String
	}
	if version.Valid {
		run.ValidatedVersion = &version.String
	}
	return run, true, nil
}

func queryOriginBatches(ctx context.Context, db *sql.DB, prefix, client string, ids []string, scan func(*sql.Rows) error) error {
	for start := 0; start < len(ids); start += 200 {
		batch := ids[start:min(start+200, len(ids))]
		args := []any{client}
		for _, id := range batch {
			args = append(args, id)
		}
		rows, err := db.QueryContext(ctx, prefix+"(?"+strings.Repeat(",?", len(batch)-1)+")", args...)
		if err != nil {
			return err
		}
		for rows.Next() {
			if err := scan(rows); err != nil {
				rows.Close()
				return err
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		if err := rows.Close(); err != nil {
			return err
		}
	}
	return nil
}

func nullableOriginString(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}
