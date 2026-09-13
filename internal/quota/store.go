package quota

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"time"
)

// Store persists the latest observation per (client, account_id, window_key)
// plus one prior (C7 — no time series), and the latest per-client envelope,
// on a caller-provided *sql.DB. The schema is owned by
// internal/store/migrations.go (version 24: quota_windows, quota_envelopes),
// like every other production table, so a caller obtains db from
// store.Open's already-migrated *store.Store.DB (QD-R1-F5) rather than from
// this package. Store itself only owns the C5/C7/C8 query and write rules.
type Store struct {
	db *sql.DB
}

// NewStore wraps an already-open, already-migrated database handle.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

const timeLayout = time.RFC3339Nano

// accountDigestDomain namespaces the one-way account digest so it cannot be
// correlated with a hash of the same raw value computed for an unrelated
// purpose elsewhere, and so the digest space differs per client.
const accountDigestDomain = "agent-deck:quota:account:v1"

// accountDigest is C8's isolation key made storage-safe (QD-R3-F2):
// architecture.md states account_id must "never reach a surface, a log, or
// an exported file," but the raw value previously went straight into
// quota_windows/quota_envelopes in the core database, which internal/backup
// exports verbatim with no per-table filtering. Isolation only ever needs
// equality, never the original value, so Store persists and compares only
// this one-way SHA-256 digest; the raw account ID reaches neither table.
// The empty string passes through unchanged — it is the sentinel for "no
// account," not a real account to digest, and must keep comparing equal to
// itself across every call site (Claude's forced-empty AccountID, and
// currentAccount's "not found" case).
func accountDigest(client Client, accountID string) string {
	if accountID == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(accountDigestDomain + ":" + string(client) + ":" + accountID))
	return hex.EncodeToString(sum[:])
}

// Record persists a new Observation, applying:
//   - C5 source precedence at the persistence boundary (QD-R1-F1): an
//     observation older than what is already stored for this key, or an
//     equal-age observation losing Accept's tiebreak, is rejected outright
//     and never reaches ApplyObservation — arrival order cannot override C5.
//   - C7's latest-plus-one-prior retention.
//   - C8's account-isolation discard via discardOnAccountChange, run before
//     every Codex write regardless of whether it lands on quota_windows or
//     quota_envelopes (QD-R2-F1) — see that function's doc.
//   - C8's Claude account scoping: Claude carries no account identifier, so
//     obs.AccountID is forced empty for Claude regardless of what the caller
//     passed (QD-R1-F6).
//   - C8's export boundary: only accountDigest(obs.AccountID) ever reaches
//     quota_windows; the raw account ID is not persisted (QD-R3-F2).
//
// accepted reports whether next replaced the stored state at all. resetObserved
// and observedResetAt report this call's own C6 reset derivation, per
// ApplyObservation, valid only when accepted. storedObservedResetAt is the
// sticky value now on record for this key regardless of acceptance.
func (s *Store) Record(ctx context.Context, obs Observation) (accepted, resetObserved bool, storedObservedResetAt time.Time, err error) {
	if obs.Client == ClientClaude {
		obs.AccountID = ""
	}
	digest := accountDigest(obs.Client, obs.AccountID)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, false, time.Time{}, err
	}
	defer tx.Rollback()

	if err := discardOnAccountChange(ctx, tx, obs.Client, digest); err != nil {
		return false, false, time.Time{}, err
	}

	existing, sticky, err := queryStored(ctx, tx, obs.Client, digest, obs.WindowKey)
	if err != nil {
		return false, false, time.Time{}, err
	}

	if existing != nil && !Accept(existing.Latest, obs) {
		return false, false, sticky, nil
	}

	updated, observedResetAt, reset := ApplyObservation(existing, obs)

	newSticky := sticky
	if reset {
		newSticky = observedResetAt
	}

	var priorUsedPercent sql.NullFloat64
	var priorObservedAt sql.NullString
	if updated.Prior != nil {
		priorUsedPercent = sql.NullFloat64{Float64: updated.Prior.UsedPercent, Valid: true}
		priorObservedAt = sql.NullString{String: updated.Prior.ObservedAt.Format(timeLayout), Valid: true}
	}

	stickyText := ""
	if !newSticky.IsZero() {
		stickyText = newSticky.Format(timeLayout)
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO quota_windows(
			client, account_id, window_key, source, observed_at, vendor_order,
			window_minutes, window_minutes_reason, label, used_percent, resets_at,
			observed_reset_at, prior_used_percent, prior_observed_at
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(client, account_id, window_key) DO UPDATE SET
			source=excluded.source, observed_at=excluded.observed_at, vendor_order=excluded.vendor_order,
			window_minutes=excluded.window_minutes, window_minutes_reason=excluded.window_minutes_reason,
			label=excluded.label, used_percent=excluded.used_percent, resets_at=excluded.resets_at,
			observed_reset_at=excluded.observed_reset_at,
			prior_used_percent=excluded.prior_used_percent, prior_observed_at=excluded.prior_observed_at`,
		string(obs.Client), digest, obs.WindowKey, string(obs.Source), obs.ObservedAt.Format(timeLayout), obs.VendorOrder,
		obs.WindowMinutes, string(obs.WindowMinutesReason), obs.Label, obs.UsedPercent, obs.ResetsAt.Format(timeLayout),
		stickyText, priorUsedPercent, priorObservedAt,
	); err != nil {
		return false, false, time.Time{}, err
	}

	if err := tx.Commit(); err != nil {
		return false, false, time.Time{}, err
	}
	return true, reset, newSticky, nil
}

// discardOnAccountChange implements C8's account-isolation discard: when a
// Codex write's AccountID differs from the account currently on record for
// that client, every existing Codex window and the Codex envelope are
// discarded together, rather than merged. It is a no-op for Claude, which
// carries no account identifier and is never isolated by one.
//
// QD-R2-F1: this must run before *every* Codex write, whether it lands on
// quota_windows (Record) or quota_envelopes (PutEnvelope) — not only before a
// window write. Triggering it from Record alone left two gaps: an envelope
// written before that cycle's window write got deleted by the window write's
// own discard immediately afterward, and an account whose probe legitimately
// produced no window (C2 — a well-formed response missing rateLimits is
// not_reported, not a failure) never triggered discard at all, leaving the
// previous account's windows attached under the new account's envelope.
// Running the same check from both entry points closes both gaps regardless
// of call order within one probe cycle.
func discardOnAccountChange(ctx context.Context, tx *sql.Tx, client Client, accountID string) error {
	if client != ClientCodex {
		return nil
	}
	existing, found, err := currentAccount(ctx, tx, client)
	if err != nil {
		return err
	}
	if !found || existing == accountID {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM quota_windows WHERE client = ?`, string(client)); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM quota_envelopes WHERE client = ?`, string(client)); err != nil {
		return err
	}
	return nil
}

// currentAccount resolves the account digest already on record for a client
// (accountDigest's output, never a raw account ID), checking quota_envelopes
// first and falling back to quota_windows so either write path sees a
// consistent answer regardless of which table currently holds data for the
// client.
//
// QD-R3-F1: an envelope row whose observed_at is still empty carries no
// authoritative account. PutEnvelopeFailure creates exactly such a row when
// no envelope exists yet — it writes only failure/failure_at, leaving
// account_id at the schema default "" — and that row must not be read as
// "the account is now unknown/empty" when quota_windows already has a real
// account on record: doing so made the next successful observation for that
// same account look like a change, discarding its own still-good windows.
func currentAccount(ctx context.Context, tx *sql.Tx, client Client) (accountID string, found bool, err error) {
	row := tx.QueryRowContext(ctx, `SELECT account_id, observed_at FROM quota_envelopes WHERE client = ?`, string(client))
	var observedAtText string
	switch scanErr := row.Scan(&accountID, &observedAtText); {
	case scanErr == nil:
		if observedAtText != "" {
			return accountID, true, nil
		}
		// Failure-only row; fall through to quota_windows below.
	case scanErr != sql.ErrNoRows:
		return "", false, scanErr
	}
	row = tx.QueryRowContext(ctx, `SELECT account_id FROM quota_windows WHERE client = ? LIMIT 1`, string(client))
	switch scanErr := row.Scan(&accountID); {
	case scanErr == nil:
		return accountID, true, nil
	case scanErr == sql.ErrNoRows:
		return "", false, nil
	default:
		return "", false, scanErr
	}
}

// queryStored reads the current Stored.Latest and the sticky observed_reset_at
// for one key in a single query, propagating a corrupt or unparseable
// observed_reset_at as an error rather than silently treating it as absent
// (QD-R1-F8). accountID here is an accountDigest output, and the returned
// Observation.AccountID carries that same digest, not a raw value (QD-R3-F2).
func queryStored(ctx context.Context, tx *sql.Tx, client Client, accountID, windowKey string) (stored *Stored, sticky time.Time, err error) {
	row := tx.QueryRowContext(ctx, `SELECT source, observed_at, vendor_order, window_minutes, window_minutes_reason,
			label, used_percent, resets_at, observed_reset_at
		FROM quota_windows WHERE client = ? AND account_id = ? AND window_key = ?`,
		string(client), accountID, windowKey)

	var source, observedAtText, minutesReason, label, resetsAtText, observedResetAtText string
	var vendorOrder, minutes int
	var usedPercent float64
	switch scanErr := row.Scan(&source, &observedAtText, &vendorOrder, &minutes, &minutesReason,
		&label, &usedPercent, &resetsAtText, &observedResetAtText); {
	case scanErr == sql.ErrNoRows:
		return nil, time.Time{}, nil
	case scanErr != nil:
		return nil, time.Time{}, scanErr
	}

	observedAt, err := time.Parse(timeLayout, observedAtText)
	if err != nil {
		return nil, time.Time{}, err
	}
	resetsAt, err := time.Parse(timeLayout, resetsAtText)
	if err != nil {
		return nil, time.Time{}, err
	}
	if observedResetAtText != "" {
		if sticky, err = time.Parse(timeLayout, observedResetAtText); err != nil {
			return nil, time.Time{}, err
		}
	}

	return &Stored{Latest: Observation{
		Client: client, AccountID: accountID, WindowKey: windowKey,
		Source: Source(source), ObservedAt: observedAt, VendorOrder: vendorOrder,
		WindowMinutes: minutes, WindowMinutesReason: Reason(minutesReason),
		Label: label, UsedPercent: usedPercent, ResetsAt: resetsAt,
	}}, sticky, nil
}

// Windows returns every currently stored window for a client, as domain
// Window values ready for envelope assembly, ordered by VendorOrder (C11 —
// QD-R1-F3) rather than by key.
func (s *Store) Windows(ctx context.Context, client Client) ([]Window, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT window_key, source, observed_at, vendor_order, window_minutes,
			window_minutes_reason, label, used_percent, resets_at, observed_reset_at
		FROM quota_windows WHERE client = ? ORDER BY vendor_order`, string(client))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Window
	for rows.Next() {
		var w Window
		var source, observedAtText, minutesReason, resetsAtText, observedResetAtText string
		if err := rows.Scan(&w.WindowKey, &source, &observedAtText, &w.VendorOrder, &w.WindowMinutes, &minutesReason,
			&w.Label, &w.UsedPercent, &resetsAtText, &observedResetAtText); err != nil {
			return nil, err
		}
		w.Source = Source(source)
		w.WindowMinutesReason = Reason(minutesReason)
		if w.ObservedAt, err = time.Parse(timeLayout, observedAtText); err != nil {
			return nil, err
		}
		if w.ResetsAt, err = time.Parse(timeLayout, resetsAtText); err != nil {
			return nil, err
		}
		if observedResetAtText != "" {
			if w.ObservedResetAt, err = time.Parse(timeLayout, observedResetAtText); err != nil {
				return nil, err
			}
			w.HasObservedResetAt = true
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// PutEnvelope persists the client-level half of Envelope that Window rows do
// not carry (C7 — "the latest per-client envelope"; QD-R1-F4), for a
// successful probe. It fully replaces the stored record — including clearing
// any previously recorded Failure, since a successful call proves the probe
// just succeeded — and applies C8's account-isolation discard first
// (discardOnAccountChange), so an out-of-order or window-less write for a
// new Codex account cannot leave a mix of two accounts' data on record
// (QD-R2-F1). rec.AccountID is forced empty for Claude, as Record does for
// Observation.
//
// For a failed probe, use PutEnvelopeFailure instead: a full PutEnvelope call
// carrying only Failure would zero every other field, erasing the last
// known-good state a temporary failure must leave alone (QD-R2-F2).
//
// Only accountDigest(rec.AccountID) ever reaches quota_envelopes; the raw
// account ID is not persisted (QD-R3-F2).
func (s *Store) PutEnvelope(ctx context.Context, rec EnvelopeRecord) error {
	if rec.Client == ClientClaude {
		rec.AccountID = ""
	}
	digest := accountDigest(rec.Client, rec.AccountID)
	creditsJSON, err := json.Marshal(rec.ResetAllowance.Credits)
	if err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := discardOnAccountChange(ctx, tx, rec.Client, digest); err != nil {
		return err
	}

	failureAtText := ""
	if !rec.FailureAt.IsZero() {
		failureAtText = rec.FailureAt.Format(timeLayout)
	}
	backoffUntilText := ""
	if !rec.BackoffUntil.IsZero() {
		backoffUntilText = rec.BackoffUntil.Format(timeLayout)
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO quota_envelopes(
			client, account_id, applicable, applicable_reason, source, observed_at,
			plan, plan_reason, reset_allowance_reason,
			reset_total, reset_total_reason, reset_remaining, reset_has_remaining, reset_credits_json,
			billing_balance, billing_has_balance, failure, failure_at, backoff_until
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(client) DO UPDATE SET
			account_id=excluded.account_id,
			applicable=excluded.applicable, applicable_reason=excluded.applicable_reason,
			source=excluded.source, observed_at=excluded.observed_at,
			plan=excluded.plan, plan_reason=excluded.plan_reason, reset_allowance_reason=excluded.reset_allowance_reason,
			reset_total=excluded.reset_total, reset_total_reason=excluded.reset_total_reason,
			reset_remaining=excluded.reset_remaining, reset_has_remaining=excluded.reset_has_remaining,
			reset_credits_json=excluded.reset_credits_json,
			billing_balance=excluded.billing_balance, billing_has_balance=excluded.billing_has_balance,
			failure=excluded.failure, failure_at=excluded.failure_at, backoff_until=excluded.backoff_until`,
		string(rec.Client), digest, boolToInt(rec.Applicable), string(rec.ApplicableReason), string(rec.Source), rec.ObservedAt.Format(timeLayout),
		rec.Plan, string(rec.PlanReason), string(rec.ResetAllowanceReason),
		rec.ResetAllowance.Total, string(rec.ResetAllowance.TotalReason), rec.ResetAllowance.Remaining, boolToInt(rec.ResetAllowance.HasRemaining), string(creditsJSON),
		rec.Billing.Balance, boolToInt(rec.Billing.HasBalance), string(rec.Failure), failureAtText, backoffUntilText,
	); err != nil {
		return err
	}

	return tx.Commit()
}

// PutEnvelopeFailure records that a probe attempt for client failed, without
// disturbing any field a prior success wrote (QD-R2-F2): only failure,
// failureAt, and backoffUntil change. A client's very first probe attempt
// failing has nothing to preserve, so the row is created with every other
// column at its schema default in that case. It does not run the
// account-isolation discard: a failed attempt provides no new account
// evidence to isolate against.
//
// backoffUntil is the caller's (the scheduler's) already-computed C9 backoff
// state — the earliest instant a background probe may run again — not
// derived here, so this function stays a plain, unconditional write like
// PutEnvelope.
//
// A zero failureAt is stored as empty, exactly like a zero backoffUntil and
// like PutEnvelope's own handling of both (GS-R4-F1). The scheduler passes a
// zero failureAt for a manual failure with no prior background failure to
// inherit from; formatting it would persist 0001-01-01 as if it were a real
// failure instant.
func (s *Store) PutEnvelopeFailure(ctx context.Context, client Client, failure Reason, failureAt, backoffUntil time.Time) error {
	failureAtText := ""
	if !failureAt.IsZero() {
		failureAtText = failureAt.Format(timeLayout)
	}
	backoffUntilText := ""
	if !backoffUntil.IsZero() {
		backoffUntilText = backoffUntil.Format(timeLayout)
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO quota_envelopes(client, failure, failure_at, backoff_until)
		VALUES (?,?,?,?)
		ON CONFLICT(client) DO UPDATE SET failure=excluded.failure, failure_at=excluded.failure_at, backoff_until=excluded.backoff_until`,
		string(client), string(failure), failureAtText, backoffUntilText,
	)
	return err
}

// Envelope reads the persisted client-level envelope. ok is false when
// nothing has ever been recorded for this client. The returned AccountID is
// accountDigest's output, not the raw account ID (QD-R3-F2) — Store never
// persists or returns the original value.
func (s *Store) Envelope(ctx context.Context, client Client) (rec EnvelopeRecord, ok bool, err error) {
	row := s.db.QueryRowContext(ctx, `SELECT account_id, applicable, applicable_reason, source, observed_at,
			plan, plan_reason, reset_allowance_reason,
			reset_total, reset_total_reason, reset_remaining, reset_has_remaining, reset_credits_json,
			billing_balance, billing_has_balance, failure, failure_at, backoff_until
		FROM quota_envelopes WHERE client = ?`, string(client))

	rec.Client = client
	var applicableInt, hasRemainingInt, hasBalanceInt int
	var applicableReason, source, observedAtText, planReason, resetAllowanceReason, totalReason, failure, failureAtText, backoffUntilText, creditsJSON string
	switch scanErr := row.Scan(&rec.AccountID, &applicableInt, &applicableReason, &source, &observedAtText,
		&rec.Plan, &planReason, &resetAllowanceReason,
		&rec.ResetAllowance.Total, &totalReason, &rec.ResetAllowance.Remaining, &hasRemainingInt, &creditsJSON,
		&rec.Billing.Balance, &hasBalanceInt, &failure, &failureAtText, &backoffUntilText); {
	case scanErr == sql.ErrNoRows:
		return EnvelopeRecord{}, false, nil
	case scanErr != nil:
		return EnvelopeRecord{}, false, scanErr
	}

	rec.Applicable = applicableInt != 0
	rec.ApplicableReason = Reason(applicableReason)
	rec.Source = Source(source)
	if observedAtText != "" {
		// Empty when the client has only ever had a PutEnvelopeFailure write
		// (no successful observation yet): ObservedAt correctly stays zero.
		if rec.ObservedAt, err = time.Parse(timeLayout, observedAtText); err != nil {
			return EnvelopeRecord{}, false, err
		}
	}
	rec.PlanReason = Reason(planReason)
	rec.ResetAllowanceReason = Reason(resetAllowanceReason)
	rec.ResetAllowance.TotalReason = Reason(totalReason)
	rec.ResetAllowance.HasRemaining = hasRemainingInt != 0
	rec.Billing.HasBalance = hasBalanceInt != 0
	rec.Failure = Reason(failure)
	if failureAtText != "" {
		if rec.FailureAt, err = time.Parse(timeLayout, failureAtText); err != nil {
			return EnvelopeRecord{}, false, err
		}
	}
	if backoffUntilText != "" {
		if rec.BackoffUntil, err = time.Parse(timeLayout, backoffUntilText); err != nil {
			return EnvelopeRecord{}, false, err
		}
	}
	if err := json.Unmarshal([]byte(creditsJSON), &rec.ResetAllowance.Credits); err != nil {
		return EnvelopeRecord{}, false, err
	}

	return rec, true, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
