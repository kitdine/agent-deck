package quota

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

type quotaSQL interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

// SnapshotReader borrows the caller's read transaction. ReadVersion and quota
// queries use the same snapshot as other consumers of that transaction.
type SnapshotReader struct {
	tx      *sql.Tx
	version ReadVersion
}

func NewSnapshotReader(ctx context.Context, tx *sql.Tx) (*SnapshotReader, error) {
	r := &SnapshotReader{tx: tx}
	if err := tx.QueryRowContext(ctx, "SELECT version FROM schema_metadata").Scan(&r.version.Schema); err != nil {
		return nil, err
	}
	if r.version.Schema >= MinSchemaVersion {
		if err := tx.QueryRowContext(ctx, "SELECT epoch FROM derived_snapshot_generation WHERE singleton=1").Scan(&r.version.CoreEpoch); err != nil {
			return nil, err
		}
	}
	if r.version.Schema >= identitySchemaVersion {
		if err := tx.QueryRowContext(ctx, `SELECT revision,revalidate_seq FROM quota_projection_revision
			JOIN quota_cycle USING(singleton) WHERE singleton=1`).Scan(&r.version.Revision, &r.version.RevalidateSeq); err != nil {
			return nil, err
		}
	}
	return r, nil
}

func (r *SnapshotReader) ReadVersion() ReadVersion { return r.version }

func readQuota[T any](db *sql.DB, ctx context.Context, read func(*SnapshotReader) (T, error)) (T, error) {
	var zero T
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return zero, err
	}
	defer tx.Rollback()
	reader, err := NewSnapshotReader(ctx, tx)
	if err != nil {
		return zero, err
	}
	return read(reader)
}

func (r *SnapshotReader) Identity(ctx context.Context, client Client) (rec IdentityRecord, present bool, err error) {
	if r.version.Schema < identitySchemaVersion {
		return rec, false, nil
	}
	var attempted, observed, base, until, plans string
	err = r.tx.QueryRowContext(ctx, `SELECT attempt_cycle_seq,attempted_at,attempt_failure,failure_cycle_seq,
		backoff_base_at,backoff_until,success_cycle_seq,success_state,success_observed_at,email_masked,
		org_kind,org_name,plan_sources_json,association_cycle_seq FROM quota_identities WHERE client=?`,
		client).Scan(&rec.AttemptSeq, &attempted, &rec.Failure, &rec.FailureSeq, &base, &until, &rec.SuccessSeq,
		&rec.SuccessState, &observed, &rec.Display.EmailMasked, &rec.Display.OrganizationKind,
		&rec.Display.OrganizationName, &plans, &rec.AssociationSeq)
	if errors.Is(err, sql.ErrNoRows) {
		return IdentityRecord{}, false, nil
	}
	if err != nil {
		return IdentityRecord{}, false, err
	}
	for _, field := range []struct {
		text string
		dest *time.Time
	}{{attempted, &rec.AttemptedAt}, {observed, &rec.ObservedAt}, {base, &rec.BackoffBaseAt}, {until, &rec.BackoffUntil}} {
		if field.text != "" {
			if *field.dest, err = time.Parse(timeLayout, field.text); err != nil {
				return IdentityRecord{}, false, errors.New("invalid identity timestamp")
			}
		}
	}
	if plans != "" {
		var fields [3]persistedPlanField
		if err := json.Unmarshal([]byte(plans), &fields); err != nil {
			return IdentityRecord{}, false, errors.New("invalid identity plan fields")
		}
		rec.Display.Plan = PlanSources{fields[0].planField(), fields[1].planField(), fields[2].planField()}
	}
	return rec, true, nil
}

// Encode classified values explicitly. Raw rejected values are discarded
// even if a caller accidentally supplies one alongside its rejection flag.
type persistedPlanField struct {
	Value    string `json:"value,omitempty"`
	Rejected bool   `json:"rejected,omitempty"`
}

func (f persistedPlanField) planField() planField {
	if f.Rejected || (f.Value != "" && !safePlanToken(f.Value)) {
		return planField{rejected: true}
	}
	return planField{value: f.Value}
}

func encodePlanSources(sources PlanSources) (string, error) {
	var fields [3]persistedPlanField
	for i, f := range [...]planField{sources.planType, sources.planTier, sources.subscriptionType} {
		clean := (persistedPlanField{Value: f.value, Rejected: f.rejected}).planField()
		fields[i] = persistedPlanField{Value: clean.value, Rejected: clean.rejected}
	}
	data, err := json.Marshal(fields)
	return string(data), err
}

func safePlanToken(s string) bool {
	if len(s) == 0 || len(s) > 32 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' {
			continue
		}
		if i == 0 || (c != '.' && c != '_' && c != '-') {
			return false
		}
	}
	return true
}
