package usage

import (
	"context"
	"database/sql"
	"time"

	"github.com/kitdine/agent-deck/internal/quota"
	"github.com/kitdine/agent-deck/internal/store"
)

// CompletedSampleValuation uses the same read as Tokens, pinned to the
// generation returned by finish, even if a new measurement has started.
func (s *Service) CompletedSampleValuation(ctx context.Context, client quota.Client, sessionID string, completion *quota.SampleCompletion) (*quota.ValuationResult, error) {
	if completion == nil {
		return nil, &quota.SampleError{Code: quota.SampleMissing}
	}
	read, err := s.openPricingRead(ctx, s.now())
	if err != nil {
		return nil, err
	}
	defer read.tx.Rollback()

	input, err := quota.ReadSessionMeasurement(ctx, read.tx, client, sessionID, completion)
	if err != nil {
		return nil, err
	}
	filter := InvocationPricingFilter{Client: string(client), SessionID: sessionID}
	err = read.visit(ctx, filter, func(event storedEvent, at time.Time) error {
		return read.addQuotaCall(&input, event, at)
	})
	if err != nil {
		return nil, err
	}
	result, err := read.sessionQuotaValuation(ctx, input)
	if err != nil {
		return nil, err
	}
	return result, read.tx.Commit()
}

func (r *pricingRead) addQuotaCall(input *quota.ValuationInput, event storedEvent, at time.Time) error {
	attribution, priced, err := r.price(event, at)
	if err == nil {
		input.Calls = append(input.Calls, quotaCall(priced, at, attribution))
	}
	return err
}

func (r *pricingRead) sessionQuotaValuation(ctx context.Context, input quota.ValuationInput) (*quota.ValuationResult, error) {
	if input.Before != nil && input.After != nil {
		// Include the probe acquisition intervals for known interference.
		// The evaluator admits amounts only inside [start completion, finish start).
		from, to := input.Before.ProbeStartedAt, input.After.ProbeCompletedAt
		if !from.IsZero() && !to.IsZero() && !to.Before(from) {
			var last time.Time
			for _, call := range input.Calls {
				if !call.OccurredAt.After(to) && call.OccurredAt.After(last) {
					last = call.OccurredAt
				}
			}
			pending, err := quotaReportUsagePending(ctx, r.tx, input.Client, input.SessionID, from, to, last)
			if err != nil {
				return nil, err
			}
			input.Evidence.Unsettled = pending

			filter := InvocationPricingFilter{Client: string(input.Client), From: from, To: to.Add(time.Nanosecond)}
			if err := r.visit(ctx, filter, func(event storedEvent, at time.Time) error {
				if event.SessionID == input.SessionID {
					return nil
				}
				return r.addQuotaCall(&input, event, at)
			}); err != nil {
				return nil, err
			}
		}
	}
	result := quota.EvaluateValuation(input)
	if result.Status == quota.MeasurementMissing && r.supported {
		settings, err := quota.LoadSettings(ctx, store.TransactionSettings{Tx: r.tx})
		result.ProbeDisabled = err == nil && (!settings.ProbeEnabled || !settings.ClientEnabled(input.Client))
	}
	return &result, nil
}

func quotaCall(priced InvocationPricing, at time.Time, route eventAttribution) quota.ValuationCall {
	amount := priced.APIEquivalent
	call := quota.ValuationCall{
		Client: quota.Client(priced.Client), SessionID: priced.SessionID, OccurredAt: at,
		Eligibility: quota.EligibilityUnknown, Currency: amount.Currency,
		PriceComplete: amount.Status == pricingComplete && amount.Amount != nil,
	}
	// Claude's prose route supplies two windows on its shared pool. The
	// admitted Codex facts carry neither a per-call pool nor an account ID.
	if call.Client == quota.ClientClaude {
		call.Pool = "claude"
	}
	if call.PriceComplete {
		call.Amount = *amount.Amount
	} else if amount.KnownAmount != nil {
		call.Amount = *amount.KnownAmount
	}
	switch {
	case priced.Subscription.QuotaEligibility == "excluded":
		call.Eligibility = quota.EligibilityExcluded
	case validThirdPartyRoute(route) && priced.Subscription.Channel == "unknown":
		call.Eligibility = quota.EligibilityExcluded
	case priced.Subscription.Channel == "enterprise_usd",
		route.provider != "" && route.provider != "unknown" && route.provider != "official":
		// Contradictory or unresolved paid-channel evidence cannot support
		// the assumption that this was usage of the sampled subscription.
		call.Eligibility = quota.EligibilityMixed
	case priced.Subscription.QuotaEligibility == "eligible":
		call.Eligibility = quota.EligibilityIncluded
	}
	return call
}

// Look at outstanding evidence within the historical capture, not at later
// work in the same session. An older pending turn eclipsed by an admitted call
// is handled the same way as in the explicit finish gate.
func quotaReportUsagePending(ctx context.Context, tx *sql.Tx, client quota.Client, sessionID string, from, to, lastCallAt time.Time) (bool, error) {
	rows, err := tx.QueryContext(ctx, `SELECT started_at, 0 FROM usage_work_signals
		WHERE client=?1 AND session_id=?2 AND state=?3
		UNION ALL SELECT record_at, 1 FROM telemetry_observations
		WHERE ?1='codex' AND thread_id=?2 AND state='pending' AND metered=1`,
		client, sessionID, signalStatePending)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var raw string
		var metered bool
		if err := rows.Scan(&raw, &metered); err != nil {
			return false, err
		}
		at, err := time.Parse(time.RFC3339Nano, raw)
		if err != nil {
			return false, err
		}
		if !at.Before(from) && !at.After(to) && (metered || !at.Before(lastCallAt)) {
			return true, nil
		}
	}
	return false, rows.Err()
}
