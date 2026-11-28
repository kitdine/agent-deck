package quota

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"
)

type identityBaseline struct {
	kind, correlation, keyID string
}

func nextClaudeBaseline(prev identityBaseline, obs IdentityObservation) (identityBaseline, ContextInvalidation) {
	switch obs.State {
	case IdentityIdentified:
		next := identityBaseline{
			kind:        "subscription",
			correlation: hex.EncodeToString(obs.claude.correlation[:]),
			keyID:       hex.EncodeToString(obs.claude.keyID[:]),
		}
		if prev.kind == "" || prev == next {
			return next, ""
		}
		if prev.kind == "subscription" && prev.keyID != next.keyID {
			return next, InvalidatedIdentityKeyReset
		}
		return next, InvalidatedContextChanged
	case IdentitySignedOut, IdentityNonSubscription:
		next := identityBaseline{kind: string(obs.State)}
		if next == prev {
			return next, ""
		}
		return next, ContextInvalidation(obs.State)
	default:
		return prev, ""
	}
}

func validateIdentity(client Client, obs IdentityObservation) error {
	switch obs.State {
	case IdentityIdentified:
		if obs.Failure == "" && obs.Display.EmailMasked != "" && ((client == ClientClaude) == (obs.claude != nil)) {
			return nil
		}
	case IdentitySignedOut, IdentityNonSubscription:
		if obs.Failure == "" && obs.claude == nil && obs.Display == (IdentityDisplay{}) {
			return nil
		}
	case "":
		if obs.claude == nil && obs.Display == (IdentityDisplay{}) {
			switch obs.Failure {
			case IdentityProbeFailed, IdentityParseFailed, IdentityIncomparable:
				return nil
			}
		}
	}
	return errors.New("invalid quota identity observation")
}

func (s *Store) CommittedClaudeContext(ctx context.Context, cycle RefreshCycle) (ClaudeContext, error) {
	if err := s.checkCycle(cycle); err != nil {
		return ClaudeContext{}, err
	}
	state, _, err := readClaudeContext(ctx, s.db)
	return state, err
}

func readClaudeContext(ctx context.Context, q quotaSQL) (state ClaudeContext, seq CycleSeq, err error) {
	var at string
	err = q.QueryRowContext(ctx, `SELECT context_generation,confirmed_generation,capture_gate,capture_open_boot,
		capture_open_mono,invalidated_at,invalidated_reason,cycle_seq FROM quota_envelopes_v2 WHERE client='claude'`).
		Scan(&state.Generation, &state.ConfirmedGeneration, &state.Gate, &state.OpenedAt.Boot,
			&state.OpenedAt.Mono, &at, &state.Invalidation, &seq)
	if errors.Is(err, sql.ErrNoRows) {
		return ClaudeContext{Gate: CaptureOpenInitial, ConfirmedGeneration: -1}, 0, nil
	}
	if err == nil && at != "" {
		state.InvalidatedAt, err = time.Parse(timeLayout, at)
	}
	return state, seq, err
}

func (s *Store) ApplyClaudeIdentity(ctx context.Context, cycle RefreshCycle, obs IdentityObservation, backoffBaseAt, backoffUntil time.Time, beforeCommit BeforeClaudeInvalidationCommit) (ClaudeIdentityResult, error) {
	if beforeCommit == nil {
		return ClaudeIdentityResult{}, errors.New("Claude identity requires a delivery guard")
	}
	if err := validateIdentity(ClientClaude, obs); err != nil {
		return ClaudeIdentityResult{}, err
	}
	var result ClaudeIdentityResult
	outcome, err := s.cycleWrite(ctx, cycle, func(ctx context.Context, conn *sql.Conn) (bool, error) {
		var priorSeq CycleSeq
		var prior identityBaseline
		err := conn.QueryRowContext(ctx, `SELECT attempt_cycle_seq,baseline_kind,baseline_correlation,baseline_key_id
			FROM quota_identities WHERE client='claude'`).Scan(&priorSeq, &prior.kind, &prior.correlation, &prior.keyID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return false, err
		}
		if cycle.seq <= priorSeq {
			return false, nil
		}
		state, _, err := readClaudeContext(ctx, conn)
		if err != nil {
			return false, err
		}
		if err := writeIdentity(ctx, conn, ClientClaude, cycle, obs, backoffBaseAt, backoffUntil); err != nil {
			return false, err
		}
		result.Generation = state.Generation
		if obs.Failure != "" {
			return true, nil
		}
		next, reason := nextClaudeBaseline(prior, obs)
		if _, err := conn.ExecContext(ctx, `UPDATE quota_identities SET baseline_kind=?,baseline_correlation=?,
			baseline_key_id=?,baseline_observed_at=? WHERE client='claude'`, next.kind, next.correlation, next.keyID, timeValue(cycle.at)); err != nil {
			return false, err
		}
		if reason != "" {
			result.Generation++
			result.Invalidation = reason
			if err := invalidateClaude(ctx, conn, result.Generation, cycle.at, reason); err != nil {
				return false, err
			}
			if err := beforeCommit(ctx, result.Generation); err != nil {
				return false, err
			}
		}
		if obs.claude != nil {
			result.proof = &claudeReconfirmProof{owner: s, seq: cycle.seq, correlation: obs.claude.correlation}
		}
		return true, nil
	})
	if outcome != WriteCommitted {
		return ClaudeIdentityResult{Outcome: outcome}, err
	}
	result.Outcome = outcome
	return result, err
}

func writeIdentity(ctx context.Context, q quotaSQL, client Client, cycle RefreshCycle, obs IdentityObservation, base, until time.Time) error {
	at := timeValue(cycle.at)
	if obs.Failure != "" {
		_, err := q.ExecContext(ctx, `INSERT INTO quota_identities(client,attempt_cycle_seq,attempted_at,attempt_failure,
			failure_cycle_seq,backoff_base_at,backoff_until) VALUES(?,?,?,?,?,?,?)
			ON CONFLICT(client) DO UPDATE SET attempt_cycle_seq=excluded.attempt_cycle_seq,attempted_at=excluded.attempted_at,
			attempt_failure=excluded.attempt_failure,failure_cycle_seq=excluded.failure_cycle_seq,
			backoff_base_at=excluded.backoff_base_at,backoff_until=excluded.backoff_until`,
			client, cycle.seq, at, obs.Failure, cycle.seq, timeValue(base), timeValue(until))
		return err
	}
	plans, err := encodePlanSources(obs.Display.Plan)
	if err != nil {
		return err
	}
	_, err = q.ExecContext(ctx, `INSERT INTO quota_identities(client,attempt_cycle_seq,attempted_at,
		success_cycle_seq,success_state,success_observed_at,account_kind,email_masked,org_kind,org_name,plan_sources_json)
		VALUES(?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(client) DO UPDATE SET attempt_cycle_seq=excluded.attempt_cycle_seq,attempted_at=excluded.attempted_at,
		attempt_failure='',failure_cycle_seq=0,backoff_base_at='',backoff_until='',success_cycle_seq=excluded.success_cycle_seq,
		success_state=excluded.success_state,success_observed_at=excluded.success_observed_at,account_kind=excluded.account_kind,
		email_masked=excluded.email_masked,org_kind=excluded.org_kind,org_name=excluded.org_name,plan_sources_json=excluded.plan_sources_json,
		association='',association_cycle_seq=0`,
		client, cycle.seq, at, cycle.seq, obs.State, at, obs.accountKind, obs.Display.EmailMasked,
		obs.Display.OrganizationKind, obs.Display.OrganizationName, plans)
	return err
}

func invalidateClaude(ctx context.Context, q quotaSQL, gen Generation, at time.Time, reason ContextInvalidation) error {
	if err := invalidateSamplesIfInstalled(ctx, q, ClientClaude); err != nil {
		return err
	}
	for _, statement := range []string{
		"DELETE FROM quota_windows_v2 WHERE client='claude'",
		"DELETE FROM quota_alert_notices_v2 WHERE client='claude'",
	} {
		if _, err := q.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	_, err := q.ExecContext(ctx, `INSERT INTO quota_envelopes_v2(client,context_generation,invalidated_at,invalidated_reason,capture_gate)
		VALUES('claude',?,?,?,'blocked') ON CONFLICT(client) DO UPDATE SET
		context_generation=excluded.context_generation,invalidated_at=excluded.invalidated_at,invalidated_reason=excluded.invalidated_reason,
		capture_gate='blocked',confirmed_generation=-1,capture_open_boot='',capture_open_mono=0,
		source='',observed_at='',cycle_seq=0,failure='',failure_at='',backoff_until='',failure_observed_at='',failure_detail=''`,
		gen, timeValue(at), reason)
	return err
}
