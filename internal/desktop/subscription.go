package desktop

import (
	"context"
	"fmt"
	"time"

	"github.com/kitdine/agent-deck/internal/provider"
	"github.com/kitdine/agent-deck/internal/quota"
	"github.com/kitdine/agent-deck/internal/store"
	"github.com/kitdine/agent-deck/internal/usage"
)

// SubscriptionSnapshot is architecture.md C11's additive section. It is also
// the data `agentdeck quota` prints (C12). Available is false only when the
// section could not be built at all; every per-client state, including the
// gate and reading off, is a client record rather than a missing section.
type SubscriptionSnapshot struct {
	Available bool                 `json:"available"`
	Clients   []SubscriptionClient `json:"clients"`
}

// SubscriptionClient follows the prototype's quota fixture: an absent field is
// null with a *_reason from C6's closed set. It carries no account identifier
// and no billing balance (C11).
type SubscriptionClient struct {
	Client               string                      `json:"client"`
	Applicable           bool                        `json:"applicable"`
	ApplicableReason     *string                     `json:"applicable_reason"`
	Source               *string                     `json:"source"`
	ObservedAt           *string                     `json:"observed_at"`
	Stale                bool                        `json:"stale"`
	AttributionConfirmed bool                        `json:"attribution_confirmed"`
	Plan                 *string                     `json:"plan"`
	PlanReason           *string                     `json:"plan_reason"`
	Windows              []SubscriptionWindow        `json:"windows"`
	TightestWindowKey    *string                     `json:"tightest_window_key"`
	ResetAllowance       *SubscriptionResetAllowance `json:"reset_allowance"`
	ResetAllowanceReason *string                     `json:"reset_allowance_reason"`
	ObservedResetAt      *string                     `json:"observed_reset_at"`
	Failure              *string                     `json:"failure"`
}

// SubscriptionWindow keeps the vendor's order within Clients[].Windows (C11).
type SubscriptionWindow struct {
	Key                 string  `json:"key"`
	Label               *string `json:"label"`
	WindowMinutes       *int    `json:"window_minutes"`
	WindowMinutesReason *string `json:"window_minutes_reason"`
	UsedPercent         float64 `json:"used_percent"`
	ResetsAt            *string `json:"resets_at"`
	// ObservedAt is this window's own observation instant (additive at
	// unchanged WireVersion=1, matching the subscription section itself):
	// the client-level observed_at is derived from the newest window or
	// envelope (BuildSubscription), so a surface that must date the windows
	// it actually displays -- not the client as a whole -- needs each
	// window's own instant, not the client's.
	ObservedAt *string `json:"observed_at"`
	// Source is this window's own provenance, additive for the same reason
	// as ObservedAt: a status-line refresh that updates only one Claude
	// window leaves the other stored window from the prose route (C5), and
	// newestWindowSource's single client-level Source would otherwise
	// mislabel it as coming from whichever route reported most recently
	// (Codex PR #5 sixth review, P2).
	Source *string `json:"source"`
}

// SubscriptionResetAllowance is the Codex reset allowance (C6). Credits carry
// no account-scoped credit identifier: Key is positional.
type SubscriptionResetAllowance struct {
	Remaining       *int                      `json:"remaining"`
	RemainingReason *string                   `json:"remaining_reason"`
	Total           *int                      `json:"total"`
	TotalReason     *string                   `json:"total_reason"`
	Credits         []SubscriptionResetCredit `json:"credits"`
}

type SubscriptionResetCredit struct {
	Key       string  `json:"key"`
	Title     string  `json:"title"`
	Status    string  `json:"status"`
	GrantedAt *string `json:"granted_at"`
	ExpiresAt *string `json:"expires_at"`
}

func unavailableSubscription() SubscriptionSnapshot {
	return SubscriptionSnapshot{Clients: []SubscriptionClient{}}
}

// ReadingOffSubscription is the section before any state exists: every
// setting is its default, so reading is off for both clients.
func ReadingOffSubscription() SubscriptionSnapshot {
	clients := make([]SubscriptionClient, 0, 2)
	for _, client := range []quota.Client{quota.ClientCodex, quota.ClientClaude} {
		entry, _ := subscriptionClient(context.Background(), nil, client, false, quota.ReasonProbeDisabled, 0, time.Time{})
		clients = append(clients, entry)
	}
	return SubscriptionSnapshot{Available: true, Clients: clients}
}

func (s Service) loadSubscription(ctx context.Context, core *store.Store, now time.Time, result *Result) {
	subscription, err := s.BuildSubscription(ctx, core, now)
	if err != nil {
		result.warn("subscription_unavailable")
		return
	}
	result.Snapshot.Subscription = subscription
}

// BuildSubscription assembles the section from core, which may be opened
// read-only: it reads the quota settings, the provider gate's inputs, and the
// stored quota state, and writes nothing.
func (s Service) BuildSubscription(ctx context.Context, core *store.Store, now time.Time) (SubscriptionSnapshot, error) {
	// core may be opened read-only (store.OpenReadOnly never migrates) and can
	// therefore still be on a schema version older than quota.MinSchemaVersion
	// -- an existing install that has not yet run any write command since
	// upgrading. Querying quota_windows/quota_envelopes on such a database
	// would fail with "no such table"; report the same reading-off default a
	// fresh install reports instead (Codex PR #5 sixth review, P1).
	version, err := core.SchemaVersion(ctx)
	if err != nil {
		return unavailableSubscription(), err
	}
	if version < quota.MinSchemaVersion {
		return ReadingOffSubscription(), nil
	}
	settings, err := quota.LoadSettings(ctx, core)
	if err != nil {
		return unavailableSubscription(), err
	}
	selections, err := (provider.Service{Store: core}).Current(ctx)
	if err != nil {
		selections = nil
	}
	usageService := usage.New(core, s.Home)
	quotaStore := quota.NewStore(core.DB)
	clients := make([]SubscriptionClient, 0, 2)
	for _, client := range []quota.Client{quota.ClientCodex, quota.ClientClaude} {
		recordedOfficial, selectedAt := quotaCurrentSelection(selections, client)
		observedKnown, observedOfficial := quotaObservedOfficial(ctx, usageService, client, selectedAt)
		allowed, reason := quota.Allowed(settings.ProbeEnabled, recordedOfficial, observedKnown, observedOfficial)
		entry, err := subscriptionClient(ctx, quotaStore, client, allowed, reason, settings.ProbeInterval, now)
		if err != nil {
			return unavailableSubscription(), err
		}
		clients = append(clients, entry)
	}
	return SubscriptionSnapshot{Available: true, Clients: clients}, nil
}

// subscriptionClient projects one client. Reading off keeps applicable true
// and reports probe_disabled as the failure, as the prototype's reading-off
// variant does; a provider other than official makes the client not
// applicable. A parse failure after the last success presents no figure
// (requirements.md clause 6); a probe failure keeps the last figure with its
// real age (C9).
func subscriptionClient(ctx context.Context, qs *quota.Store, client quota.Client, allowed bool, gateReason quota.Reason, interval time.Duration, now time.Time) (SubscriptionClient, error) {
	out := SubscriptionClient{
		Client:               string(client),
		Applicable:           true,
		AttributionConfirmed: quota.AttributionConfirmed(client),
		Windows:              []SubscriptionWindow{},
	}
	if client == quota.ClientClaude {
		// Claude reports neither a plan nor a reset allowance (C6).
		out.PlanReason = reasonText(quota.ReasonNotReported)
		out.ResetAllowanceReason = reasonText(quota.ReasonNotReported)
	}
	absent := func(reason quota.Reason) {
		if client == quota.ClientCodex {
			out.PlanReason = reasonText(reason)
		}
		out.ResetAllowanceReason = reasonText(reason)
	}

	if !allowed {
		if gateReason == quota.ReasonProbeDisabled {
			out.Failure = reasonText(gateReason)
		} else {
			out.Applicable = false
			out.ApplicableReason = reasonText(gateReason)
		}
		absent(gateReason)
		return out, nil
	}

	windows, err := qs.Windows(ctx, client)
	if err != nil {
		return SubscriptionClient{}, err
	}
	rec, hasRecord, err := qs.Envelope(ctx, client)
	if err != nil {
		return SubscriptionClient{}, err
	}
	windowSource, windowObservedAt := newestWindowSource(windows)
	source, observedAt := windowSource, windowObservedAt
	if hasRecord && rec.ObservedAt.After(observedAt) {
		source, observedAt = rec.Source, rec.ObservedAt
	}
	// PutEnvelope clears Failure only on a genuine success, so Failure != ""
	// alone proves the envelope's own route (Codex app-server, or Claude
	// prose) failed more recently than its own ObservedAt — including for a
	// manual failure, which deliberately leaves FailureAt untouched
	// (GS-R3-F1) and so cannot be compared against observedAt the way a
	// background failure's FailureAt can (WC-R1-F1). The one route that
	// bypasses the envelope entirely is Claude's status-line: a status-line
	// window newer than the envelope's own ObservedAt is a success the
	// envelope never recorded, and it supersedes a stale prose failure.
	supersededByStatusLine := windowSource == quota.SourceClaudeStatusLine && windowObservedAt.After(rec.ObservedAt)
	failed := hasRecord && rec.Failure != "" && !supersededByStatusLine

	switch {
	case observedAt.IsZero() && !failed:
		out.Failure = reasonText(quota.ReasonNeverProbed)
		absent(quota.ReasonNeverProbed)
		return out, nil
	case failed && (observedAt.IsZero() || rec.Failure == quota.ReasonParseFailed):
		out.Failure = reasonText(rec.Failure)
		// FailureObservedAt, not FailureAt: a manual failure leaves FailureAt
		// untouched to protect the backoff chain (GS-R3-F1), but always
		// carries its own real attempt instant in FailureObservedAt
		// (WC-R2-F1) — requirements.md clause 6's "observation instant of the
		// failed attempt".
		out.ObservedAt = timeText(rec.FailureObservedAt)
		out.Source = sourceText(source)
		absent(rec.Failure)
		return out, nil
	}

	out.Source = sourceText(source)
	out.ObservedAt = timeText(observedAt)
	out.Stale = quota.Stale(now, windows, interval)
	if failed {
		out.Failure = reasonText(rec.Failure)
	}

	var tightest *quota.Window
	var observedResetAt time.Time
	for i := range windows {
		w := windows[i]
		out.Windows = append(out.Windows, subscriptionWindow(w))
		if tightest == nil || w.UsedPercent > tightest.UsedPercent {
			tightest = &windows[i]
		}
		if w.HasObservedResetAt && w.ObservedResetAt.After(observedResetAt) {
			observedResetAt = w.ObservedResetAt
		}
	}
	if tightest != nil {
		key := tightest.WindowKey
		out.TightestWindowKey = &key
	}
	out.ObservedResetAt = timeText(observedResetAt)

	if client == quota.ClientCodex {
		if !hasRecord || rec.ObservedAt.IsZero() {
			absent(quota.ReasonNotReported)
		} else {
			if rec.Plan != "" {
				out.Plan = &rec.Plan
			} else {
				out.PlanReason = reasonText(reasonOr(rec.PlanReason, quota.ReasonNotReported))
			}
			if rec.ResetAllowanceReason != "" {
				out.ResetAllowanceReason = reasonText(rec.ResetAllowanceReason)
			} else {
				out.ResetAllowance = resetAllowance(rec.ResetAllowance)
			}
		}
	}
	return out, nil
}

// newestWindowSource is C5 at the client level: the newest window's source,
// preferring the status-line route at equal age.
func newestWindowSource(windows []quota.Window) (quota.Source, time.Time) {
	var source quota.Source
	var observedAt time.Time
	for _, w := range windows {
		if w.ObservedAt.After(observedAt) ||
			(w.ObservedAt.Equal(observedAt) && w.Source == quota.SourceClaudeStatusLine && source != quota.SourceClaudeStatusLine) {
			source, observedAt = w.Source, w.ObservedAt
		}
	}
	return source, observedAt
}

func subscriptionWindow(w quota.Window) SubscriptionWindow {
	out := SubscriptionWindow{Key: w.WindowKey, UsedPercent: w.UsedPercent, ResetsAt: timeText(w.ResetsAt), ObservedAt: timeText(w.ObservedAt), Source: sourceText(w.Source)}
	if w.Label != "" {
		label := w.Label
		out.Label = &label
	}
	if w.WindowMinutesReason != "" {
		out.WindowMinutesReason = reasonText(w.WindowMinutesReason)
	} else {
		minutes := w.WindowMinutes
		out.WindowMinutes = &minutes
	}
	return out
}

func resetAllowance(allowance quota.ResetAllowance) *SubscriptionResetAllowance {
	out := &SubscriptionResetAllowance{Credits: []SubscriptionResetCredit{}}
	if allowance.HasRemaining {
		remaining := allowance.Remaining
		out.Remaining = &remaining
	} else {
		out.RemainingReason = reasonText(quota.ReasonNotReported)
	}
	if allowance.TotalReason != "" {
		out.TotalReason = reasonText(allowance.TotalReason)
	} else {
		total := allowance.Total
		out.Total = &total
	}
	for i, credit := range allowance.Credits {
		out.Credits = append(out.Credits, SubscriptionResetCredit{
			Key:       fmt.Sprintf("c%d", i+1),
			Title:     credit.Title,
			Status:    credit.Status,
			GrantedAt: timeText(credit.GrantedAt),
			ExpiresAt: timeText(credit.ExpiresAt),
		})
	}
	return out
}

// sourceText maps the domain's route names to the wire names the prototype
// fixes, so the surfaces read the vocabulary they were designed against.
func sourceText(source quota.Source) *string {
	var name string
	switch source {
	case quota.SourceCodex:
		name = "codex_app_server"
	case quota.SourceClaudeStatusLine:
		name = "claude_statusline"
	case quota.SourceClaudeProse:
		name = "claude_usage_prose"
	default:
		return nil
	}
	return &name
}

func reasonText(reason quota.Reason) *string {
	if reason == "" {
		return nil
	}
	text := string(reason)
	return &text
}

func reasonOr(reason, fallback quota.Reason) quota.Reason {
	if reason == "" {
		return fallback
	}
	return reason
}

func timeText(t time.Time) *string {
	if t.IsZero() {
		return nil
	}
	text := t.UTC().Format(time.RFC3339Nano)
	return &text
}
