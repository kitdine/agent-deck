package desktop

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/kitdine/agent-deck/internal/provider"
	"github.com/kitdine/agent-deck/internal/quota"
)

type SubscriptionProjection struct {
	CoreEpoch int64  `json:"core_epoch"`
	Revision  int64  `json:"revision"`
	Boot      string `json:"boot"`
	Mono      int64  `json:"mono"`
}
type SubscriptionContext struct {
	InvalidatedAt string `json:"invalidated_at"`
	Reason        string `json:"reason"`
}
type SubscriptionPlanRaw struct {
	Field string `json:"field"`
	Value string `json:"value"`
}
type SubscriptionPlan struct {
	State     quota.PlanState       `json:"state"`
	Name      *string               `json:"name"`
	ShortName *string               `json:"short_name"`
	Basis     *string               `json:"basis"`
	Source    quota.PlanSource      `json:"source"`
	Raw       []SubscriptionPlanRaw `json:"raw"`
	Reason    *string               `json:"reason"`
}
type SubscriptionIdentity struct {
	State            string            `json:"state"`
	UnknownReason    *string           `json:"unknown_reason"`
	Freshness        string            `json:"freshness"`
	CurrentUntil     *string           `json:"current_until"`
	ObservedAt       *string           `json:"observed_at"`
	AttemptedAt      *string           `json:"attempted_at"`
	Failure          *string           `json:"failure"`
	FailureAt        *string           `json:"failure_at"`
	Source           quota.PlanSource  `json:"source"`
	EmailMasked      *string           `json:"email_masked"`
	OrganizationKind *string           `json:"organization_kind"`
	OrganizationName *string           `json:"organization_name"`
	PlanDisplay      *SubscriptionPlan `json:"plan_display"`
	QuotaAssociation *string           `json:"quota_association"`
}

func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
func subscriptionPlan(src quota.PlanSource, sources quota.PlanSources) *SubscriptionPlan {
	p := quota.ProjectPlan(src, sources)
	raw := make([]SubscriptionPlanRaw, 0, len(p.Raw))
	for _, field := range p.Raw {
		raw = append(raw, SubscriptionPlanRaw{field.Field, field.Value})
	}
	return &SubscriptionPlan{p.State, optional(p.Name), optional(p.ShortName), optional(string(p.Basis)), p.Source, raw, optional(string(p.Reason))}
}

func projectSubscriptionFacts(ctx context.Context, out *SubscriptionClient, r *quota.SnapshotReader, tx *sql.Tx, c quota.Client, u quota.UsageRecord, now time.Time, interval time.Duration) error {
	src := quota.PlanSourceClaudeAuthStatus
	if c == quota.ClientCodex {
		out.PlanDisplay = subscriptionPlan(quota.PlanSourceCodexRateLimits, u.Plan)
		return nil
	} else if at := timeText(u.Claude.InvalidatedAt); at != nil {
		out.QuotaContext = &SubscriptionContext{*at, string(u.Claude.Invalidation)}
	}
	v := r.ReadVersion()
	if v.Schema < 36 {
		return nil
	}
	row, _, err := r.Identity(ctx, c)
	if err != nil {
		return err
	}
	i := &SubscriptionIdentity{State: string(row.SuccessState), Source: src, Freshness: "last_read", ObservedAt: timeText(row.ObservedAt), AttemptedAt: timeText(row.AttemptedAt)}
	out.Identity = i
	switch {
	case row.FailureSeq > row.SuccessSeq:
		i.State = "unknown"
		i.Failure = optional(string(row.Failure))
		i.FailureAt = i.AttemptedAt
		i.UnknownReason = i.Failure
	case row.SuccessSeq == 0:
		i.State = "unknown"
		i.UnknownReason = optional("never_probed")
	case row.SuccessSeq > v.RevalidateSeq && !now.Before(row.ObservedAt) && !now.After(row.ObservedAt.Add(2*interval)):
		i.Freshness = "current"
		i.CurrentUntil = timeText(row.ObservedAt.Add(2 * interval))
	}
	if row.SuccessState == quota.IdentityIdentified {
		i.EmailMasked = optional(row.Display.EmailMasked)
		i.OrganizationKind = optional(string(row.Display.OrganizationKind))
		i.OrganizationName = optional(row.Display.OrganizationName)
		i.PlanDisplay = subscriptionPlan(src, row.Display.Plan)
	}

	return nil
}

// Read the same gate inputs as provider.Current and LatestObservedProvider,
// borrowing the quota read transaction. Only names and timestamps are needed.
type subscriptionInputs struct{ *sql.Tx }

func (s subscriptionInputs) Setting(ctx context.Context, key string) (string, bool, error) {
	var v string
	err := s.QueryRowContext(ctx, `SELECT value FROM settings WHERE key=?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	return v, err == nil, err
}

func (s subscriptionInputs) gate(ctx context.Context, c quota.Client, enabled bool) (bool, quota.Reason, error) {
	name, selected, err := s.selection(ctx, c)
	if err != nil {
		return false, "", err
	}
	var observed, text string
	err = s.QueryRowContext(ctx, `SELECT observed_provider,observed_at FROM usage_session_observations WHERE client=? AND observed_provider IS NOT NULL AND observed_provider!='' ORDER BY id DESC LIMIT 1`, c).Scan(&observed, &text)
	at, parseErr := time.Parse(time.RFC3339Nano, text)
	known := err == nil && parseErr == nil && !at.Before(selected)
	ok, reason := quota.Allowed(enabled, strings.EqualFold(name, provider.OfficialProviderName), known, strings.EqualFold(observed, provider.OfficialProviderName))
	return ok, reason, nil
}

func (s subscriptionInputs) selection(ctx context.Context, c quota.Client) (string, time.Time, error) {
	// Timestamps are RFC3339Nano, not lexically sortable (optional fractional
	// seconds). Match the store's completed-operation and historical fallback.
	ops, err := s.QueryContext(ctx, `SELECT id,provider_id,started_at,updated_at FROM operations WHERE kind='provider.use' AND state='completed' AND client=?`, c)
	if err != nil {
		return "", time.Time{}, err
	}
	defer ops.Close()
	var op string
	var pid sql.NullInt64
	var start, end time.Time
	for ops.Next() {
		var id, a, b string
		var p sql.NullInt64
		if err := ops.Scan(&id, &p, &a, &b); err != nil {
			return "", time.Time{}, err
		}
		from, err := time.Parse(time.RFC3339Nano, a)
		if err != nil {
			return "", time.Time{}, err
		}
		to, err := time.Parse(time.RFC3339Nano, b)
		if err != nil {
			return "", time.Time{}, err
		}
		if op == "" || to.After(end) || (to.Equal(end) && id > op) {
			op, pid, start, end = id, p, from, to
		}
	}
	if err := ops.Err(); err != nil {
		return "", time.Time{}, err
	}
	if op != "" {
		var name, text string
		err := s.QueryRowContext(ctx, `SELECT provider_name_snapshot,selected_at FROM provider_selections WHERE operation_id=?`, op).Scan(&name, &text)
		if err == nil {
			at, e := time.Parse(time.RFC3339Nano, text)
			return name, at, e
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return "", time.Time{}, err
		}
		if !pid.Valid {
			return provider.OfficialProviderName, end, nil
		}
	}
	rows, err := s.QueryContext(ctx, `SELECT ps.id,ps.provider_id,ps.provider_name_snapshot,ps.selected_at FROM provider_selections ps LEFT JOIN operations o ON o.id=ps.operation_id WHERE ps.client=? AND (ps.operation_id IS NULL OR o.state='completed')`, c)
	if err != nil {
		return "", time.Time{}, err
	}
	defer rows.Close()
	type candidate struct {
		name string
		at   time.Time
		id   int64
	}
	var latest, within candidate
	for rows.Next() {
		var x candidate
		var p sql.NullInt64
		var text string
		if err := rows.Scan(&x.id, &p, &x.name, &text); err != nil {
			return "", time.Time{}, err
		}
		x.at, err = time.Parse(time.RFC3339Nano, text)
		if err != nil {
			return "", time.Time{}, err
		}
		newer := func(y candidate) bool { return y.id == 0 || x.at.After(y.at) || (x.at.Equal(y.at) && x.id > y.id) }
		if newer(latest) {
			latest = x
		}
		if op != "" && pid == p && !x.at.Before(start) && !x.at.After(end) && newer(within) {
			within = x
		}
	}
	if within.id != 0 {
		latest = within
	}
	return latest.name, latest.at, rows.Err()
}
