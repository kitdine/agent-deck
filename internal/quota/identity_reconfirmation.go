package quota

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"
)

func (s *Store) CommitClaudeUsage(ctx context.Context, cycle RefreshCycle, expected Generation, usage ClaudeProseResult, identity ClaudeIdentityResult) (WriteOutcome, error) {
	if len(usage.Windows) == 0 {
		return WriteUnresolved, errors.New("missing Claude usage windows")
	}
	return s.cycleWrite(ctx, cycle, func(ctx context.Context, conn *sql.Conn) (bool, error) {
		state, seq, err := readClaudeContext(ctx, conn)
		if err != nil || expected != state.Generation || cycle.seq <= seq {
			return false, err
		}
		for _, window := range usage.Windows {
			window.Client, window.Source, window.ObservedAt = ClientClaude, SourceClaudeProse, cycle.at
			if _, _, _, err := writeQuotaWindow(ctx, conn, window, expected, cycle.seq); err != nil {
				return false, err
			}
		}
		if err := pruneWindows(ctx, conn, ClientClaude, "", observedWindowKeys(usage.Windows)); err != nil {
			return false, err
		}
		if err := putEnvelope(ctx, conn, EnvelopeRecord{
			Client: ClientClaude, Applicable: true, Source: SourceClaudeProse, ObservedAt: cycle.at,
		}, false, expected, cycle.seq); err != nil {
			return false, err
		}
		confirmed := state.ConfirmedGeneration
		if proof := identity.proof; identity.Outcome == WriteCommitted && identity.Generation == expected &&
			proof != nil && proof.owner == s && proof.seq == cycle.seq && state.Gate == CaptureBlocked {
			var matches bool
			err := conn.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM quota_identities WHERE client='claude'
				AND baseline_kind='subscription' AND baseline_correlation=? AND success_cycle_seq=?)`,
				hex.EncodeToString(proof.correlation[:]), proof.seq).Scan(&matches)
			if err != nil {
				return false, err
			}
			if matches {
				confirmed = expected
			}
		}
		err = finishClaudeUsage(ctx, conn, confirmed)
		return err == nil, err
	})
}

// Usage alone clears the invalidation notice but never confirms a generation.
func finishClaudeUsage(ctx context.Context, q quotaSQL, confirmed Generation) error {
	_, err := q.ExecContext(ctx, `UPDATE quota_envelopes_v2 SET invalidated_at='',invalidated_reason='',
		confirmed_generation=? WHERE client='claude'`, confirmed)
	return err
}

func (s *Store) CommitClaudeUsageFailure(ctx context.Context, cycle RefreshCycle, expected Generation, failure UsageFailure) (WriteOutcome, error) {
	if err := validateUsageFailure(ClientClaude, failure); err != nil {
		return WriteUnresolved, err
	}
	return s.cycleWrite(ctx, cycle, func(ctx context.Context, conn *sql.Conn) (bool, error) {
		state, seq, err := readClaudeContext(ctx, conn)
		if err != nil || expected != state.Generation || cycle.seq <= seq {
			return false, err
		}
		err = writeUsageFailure(ctx, conn, ClientClaude, cycle.seq, expected, failure)
		return err == nil, err
	})
}

func validateUsageFailure(client Client, f UsageFailure) error {
	if (f.Trigger != TriggerBackground && f.Trigger != TriggerManual) ||
		(f.Reason != ReasonProbeFailed && f.Reason != ReasonParseFailed) || f.FailureObservedAt.IsZero() {
		return errors.New("invalid quota usage failure")
	}
	switch f.Detail {
	case "":
		return nil
	case DetailMissingLine, DetailShapeInvalid, DetailTimezoneUnresolved:
		if client == ClientClaude && f.Reason == ReasonParseFailed {
			return nil
		}
	}
	return errors.New("invalid quota usage failure detail")
}

func writeUsageFailure(ctx context.Context, q quotaSQL, client Client, seq CycleSeq, gen Generation, f UsageFailure) error {
	if f.authenticationFailed {
		if err := invalidateSamplesIfInstalled(ctx, q, client); err != nil {
			return err
		}
	}
	background := f.Trigger == TriggerBackground
	if !background {
		f.FailureAt, f.BackoffUntil = time.Time{}, time.Time{}
	}
	_, err := q.ExecContext(ctx, `INSERT INTO quota_envelopes_v2(client,cycle_seq,context_generation,failure,failure_detail,
		failure_observed_at,failure_at,backoff_until) VALUES(?,?,?,?,?,?,?,?)
		ON CONFLICT(client) DO UPDATE SET cycle_seq=excluded.cycle_seq,failure=excluded.failure,
		failure_detail=excluded.failure_detail,failure_observed_at=excluded.failure_observed_at,
		failure_at=CASE WHEN ? THEN excluded.failure_at ELSE failure_at END,
		backoff_until=CASE WHEN ? THEN excluded.backoff_until ELSE backoff_until END`,
		client, seq, gen, f.Reason, f.Detail, timeValue(f.FailureObservedAt), timeValue(f.FailureAt),
		timeValue(f.BackoffUntil), background, background)
	return err
}

func (s *Store) PublishClaudeCaptureGate(ctx context.Context, cycle RefreshCycle, expected Generation, stamp BootStamp) (WriteOutcome, error) {
	if stamp.Boot == "" || stamp.Mono < 0 {
		return WriteDropped, nil
	}
	return s.cycleWrite(ctx, cycle, func(ctx context.Context, conn *sql.Conn) (bool, error) {
		res, err := conn.ExecContext(ctx, `UPDATE quota_envelopes_v2 SET capture_gate='open',
			capture_open_boot=?,capture_open_mono=? WHERE client='claude' AND context_generation=?
			AND confirmed_generation=context_generation AND (capture_gate='blocked' OR
			(capture_gate='open' AND capture_open_boot<>?))`, stamp.Boot, stamp.Mono, expected, stamp.Boot)
		if err != nil {
			return false, err
		}
		n, err := res.RowsAffected()
		return n != 0, err
	})
}

func (s *Store) RecordStatusLineCapture(ctx context.Context, entryAt time.Time, stamp *BootStamp, windows []Observation) (WriteOutcome, error) {
	if entryAt.IsZero() {
		return WriteUnresolved, errors.New("missing capture observation time")
	}
	return s.quotaWrite(ctx, 200*time.Millisecond, func(ctx context.Context, conn *sql.Conn) (bool, error) {
		state, _, err := readClaudeContext(ctx, conn)
		if err != nil {
			return false, err
		}
		switch state.Gate {
		case CaptureOpenInitial:
		case CaptureOpen:
			if stamp == nil || stamp.Boot == "" || stamp.Boot != state.OpenedAt.Boot ||
				stamp.Mono <= state.OpenedAt.Mono || state.ConfirmedGeneration != state.Generation {
				return false, nil
			}
		default:
			return false, nil
		}
		changed := false
		for _, window := range windows {
			window.Client, window.Source, window.ObservedAt = ClientClaude, SourceClaudeStatusLine, entryAt
			accepted, _, _, err := writeQuotaWindow(ctx, conn, window, state.Generation, 0)
			if err != nil {
				return false, err
			}
			changed = changed || accepted
		}
		return changed, nil
	})
}

func (s *Store) CommitCodexCycle(ctx context.Context, cycle RefreshCycle, result CodexCycleResult, backoffBaseAt, backoffUntil time.Time) (WriteOutcome, error) {
	if result.Usage != nil && result.Failure != nil {
		return WriteUnresolved, errors.New("Codex usage success and failure are mutually exclusive")
	}
	if result.Identity != nil {
		if err := validateIdentity(ClientCodex, *result.Identity); err != nil {
			return WriteUnresolved, err
		}
	}
	if result.Failure != nil {
		if err := validateUsageFailure(ClientCodex, *result.Failure); err != nil {
			return WriteUnresolved, err
		}
	}
	return s.cycleWrite(ctx, cycle, func(ctx context.Context, conn *sql.Conn) (bool, error) {
		identityWritten := false
		if result.Identity != nil {
			var prior CycleSeq
			err := conn.QueryRowContext(ctx, "SELECT attempt_cycle_seq FROM quota_identities WHERE client='codex'").Scan(&prior)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return false, err
			}
			if cycle.seq > prior {
				if err := writeIdentity(ctx, conn, ClientCodex, cycle, *result.Identity, backoffBaseAt, backoffUntil); err != nil {
					return false, err
				}
				identityWritten = true
			}
		}
		var seq CycleSeq
		var observedAt string
		err := conn.QueryRowContext(ctx, "SELECT cycle_seq,observed_at FROM quota_envelopes_v2 WHERE client='codex'").Scan(&seq, &observedAt)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return false, err
		}
		if cycle.seq <= seq {
			return identityWritten, nil
		}
		if result.Failure != nil {
			err := writeUsageFailure(ctx, conn, ClientCodex, cycle.seq, 0, *result.Failure)
			return err == nil, err
		}
		if result.Usage == nil {
			return identityWritten, nil
		}
		if observedAt != "" {
			previous, err := time.Parse(timeLayout, observedAt)
			if err != nil {
				return false, errors.New("invalid Codex quota observation time")
			}
			if previous.After(cycle.at) {
				return identityWritten, nil
			}
		}
		if err := writeCodexUsage(ctx, conn, cycle, result); err != nil {
			return false, err
		}
		if identityWritten && result.Identity.State == IdentityIdentified && result.proof.matched {
			_, err := conn.ExecContext(ctx, "UPDATE quota_identities SET association='confirmed',association_cycle_seq=? WHERE client='codex'", cycle.seq)
			return err == nil, err
		}
		return true, nil
	})
}

func writeCodexUsage(ctx context.Context, conn *sql.Conn, cycle RefreshCycle, result CodexCycleResult) error {
	usage := result.Usage
	if _, err := invalidateSamplesForAccount(ctx, conn, ClientCodex, accountDigest(ClientCodex, usage.AccountID)); err != nil {
		return err
	}
	for _, window := range usage.Windows {
		window.Client, window.Source, window.ObservedAt = ClientCodex, SourceCodex, cycle.at
		window.AccountID = usage.AccountID
		if _, _, _, err := writeQuotaWindow(ctx, conn, window, 0, cycle.seq); err != nil {
			return err
		}
	}
	if err := pruneWindows(ctx, conn, ClientCodex, usage.AccountID, observedWindowKeys(usage.Windows)); err != nil {
		return err
	}
	return putEnvelope(ctx, conn, EnvelopeRecord{
		Client: ClientCodex, AccountID: usage.AccountID, Applicable: true,
		Source: SourceCodex, ObservedAt: cycle.at, Plan: usage.Plan, PlanReason: usage.PlanReason,
		Billing: usage.Billing, ResetAllowance: usage.ResetAllowance,
	}, usage.planUnsafe, 0, cycle.seq)
}
