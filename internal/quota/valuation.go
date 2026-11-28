package quota

import (
	"math"
	"time"
)

type MeasurementStatus string

const (
	MeasurementMissing  MeasurementStatus = "missing"
	MeasurementActive   MeasurementStatus = "active"
	MeasurementComplete MeasurementStatus = "complete"
)

type ValuationCoverage string

const (
	CoverageFull    ValuationCoverage = "full"
	CoveragePortion ValuationCoverage = "portion"
	CoverageUnknown ValuationCoverage = "unknown"
)

type ValuationReason string

const (
	ValuationMeasurementMissing    ValuationReason = "measurement_missing"
	ValuationMeasurementInProgress ValuationReason = "measurement_in_progress"
	ValuationWindowMissing         ValuationReason = "window_missing"
	ValuationWindowUnverified      ValuationReason = "window_unverified"
	ValuationPoolUnresolved        ValuationReason = "pool_unresolved"
	ValuationAccountChanged        ValuationReason = "account_changed"
	ValuationResetDetected         ValuationReason = "reset_detected"
	ValuationOtherLocalUsage       ValuationReason = "other_local_usage"
	ValuationUnsettled             ValuationReason = "unsettled"
	ValuationMixedBilling          ValuationReason = "mixed_billing"
	ValuationPricingIncomplete     ValuationReason = "pricing_incomplete"
	ValuationCurrencyMismatch      ValuationReason = "currency_mismatch"
	ValuationNoReliableDelta       ValuationReason = "no_reliable_delta"
	ValuationSaturated             ValuationReason = "saturated"
)

type ValuationAssumption string

const (
	AssumeSessionUsesSampledAccount  ValuationAssumption = "session_uses_sampled_account"
	AssumeAccountUnverified          ValuationAssumption = "account_unverified"
	AssumeAllChangeFromSelectedUsage ValuationAssumption = "all_change_from_selected_usage"
	AssumeReportedUsageSettled       ValuationAssumption = "reported_usage_settled"
	AssumeReportedPrecisionUnknown   ValuationAssumption = "reported_precision_unknown"
	AssumeSubscriptionScope          ValuationAssumption = "subscription_scope_assumed"
)

// SampleWindow adds measurement evidence to a window returned by a probe.
type SampleWindow struct {
	Window
	Pool string
	Slot string

	// PrecisionPP is the rounding increment in percentage points, when known.
	PrecisionPP *float64
	// ResetObserved identifies an active reset reported during this probe.
	// Window.ObservedResetAt can also identify an intervening, recorded reset.
	ResetObserved      bool
	ResetTimeAmbiguous bool
}

// ValuationEndpoint is one successful explicit probe. AccountDigest contains
// only the comparable local digest. An empty digest means it is unavailable.
type ValuationEndpoint struct {
	ProbeStartedAt   time.Time
	ProbeCompletedAt time.Time
	Source           Source
	AccountDigest    string
	Windows          []SampleWindow
}

type SubscriptionEligibility string

const (
	EligibilityUnknown  SubscriptionEligibility = "unknown"
	EligibilityIncluded SubscriptionEligibility = "included"
	EligibilityExcluded SubscriptionEligibility = "excluded"
	EligibilityMixed    SubscriptionEligibility = "mixed"
)

// ValuationCall is one deduplicated, admitted local call. An empty Pool or
// AccountDigest means membership is unknown. Amount is the known API equivalent
// in its original currency, including a partial amount when PriceComplete is false.
type ValuationCall struct {
	Client        Client
	SessionID     string
	OccurredAt    time.Time
	Pool          string
	AccountDigest string
	Eligibility   SubscriptionEligibility
	Currency      string
	Amount        string
	PriceComplete bool
}

// ValuationEvidence records conflicts the two endpoint values alone cannot
// show, such as a route change followed by a return to the original route.
type ValuationEvidence struct {
	AccountChanged bool
	SourceChanged  bool
	ClockAmbiguous bool
	Unsettled      bool
}

type ValuationInput struct {
	Client    Client
	SessionID string
	Before    *ValuationEndpoint
	After     *ValuationEndpoint
	Calls     []ValuationCall
	Evidence  ValuationEvidence
}

// ObservationPeriod is the usable request interval. Its start is inclusive
// and its end precedes every reading obtained by the finish probe.
type ObservationPeriod struct {
	Start time.Time
	End   time.Time
}

type ValuationResult struct {
	Version    int               `json:"version"`
	Status     MeasurementStatus `json:"status"`
	StartedAt  *time.Time        `json:"started_at,omitempty"`
	FinishedAt *time.Time        `json:"finished_at,omitempty"`
	Coverage   ValuationCoverage `json:"coverage"`
	Windows    []WindowValuation `json:"windows"`

	// Period is the exact bracket used for matching requests.
	Period *ObservationPeriod `json:"-"`
	// ProbeDisabled is a local advisory for a missing measurement.
	ProbeDisabled bool `json:"-"`
}

type WindowValuation struct {
	WindowKey       string                `json:"window_key"`
	Label           string                `json:"label"`
	Pool            string                `json:"pool"`
	WindowMinutes   int                   `json:"window_minutes"`
	Before          *ValuationReading     `json:"before,omitempty"`
	After           *ValuationReading     `json:"after,omitempty"`
	ObservedDeltaPP *float64              `json:"observed_delta_pp,omitempty"`
	Estimate        *WindowEstimate       `json:"estimate,omitempty"`
	Assumptions     []ValuationAssumption `json:"assumptions"`
	Reason          ValuationReason       `json:"reason,omitempty"`

	// KnownBasis retains partial subtotals for the reporting layer even when
	// Estimate is absent. In the v1 window wire format, basis_amount belongs to Estimate.
	KnownBasis []ValuationAmount `json:"-"`
}

type ValuationReading struct {
	UsedPercent float64   `json:"used_percent"`
	ObservedAt  time.Time `json:"observed_at"`
	Source      Source    `json:"source"`
}

type ValuationAmount struct {
	Currency string
	Amount   string
}

type WindowEstimate struct {
	Currency    string `json:"currency"`
	Amount      string `json:"amount,omitempty"`
	Lower       string `json:"lower,omitempty"`
	Upper       string `json:"upper,omitempty"`
	BasisAmount string `json:"basis_amount"`
}

type valuationPair struct {
	before, after *SampleWindow
	duplicate     bool
}

// EvaluateValuation pairs the endpoints of an explicit measurement and
// evaluates each window independently. Calls must already be deduplicated.
func EvaluateValuation(input ValuationInput) ValuationResult {
	result := ValuationResult{
		Version: 1, Status: MeasurementMissing, Coverage: CoverageUnknown,
		Windows: []WindowValuation{},
	}
	if input.Before != nil {
		result.Status = MeasurementActive
		result.StartedAt = observedTime(input.Before.ProbeStartedAt)
		if input.After != nil {
			result.Status = MeasurementComplete
			result.FinishedAt = observedTime(input.After.ProbeCompletedAt)
			if orderedProbes(input) {
				result.Period = &ObservationPeriod{
					Start: input.Before.ProbeCompletedAt.UTC(),
					End:   input.After.ProbeStartedAt.UTC(),
				}
				result.Coverage = measurementCoverage(input, *result.Period)
			}
		}
	}

	// Preserve the first probe's window order, then append finish-only windows.
	keys, pairs := pairWindows(input.Before, input.After)
	pools := make(map[string]struct{})
	for _, endpoint := range []*ValuationEndpoint{input.Before, input.After} {
		if endpoint == nil {
			continue
		}
		for _, window := range endpoint.Windows {
			pools[window.Pool] = struct{}{}
		}
	}
	for _, key := range keys {
		pair := pairs[key]
		window := pair.before
		if pair.after != nil {
			window = pair.after
		}
		row := WindowValuation{
			WindowKey: key, Label: window.Label, Pool: window.Pool, WindowMinutes: window.WindowMinutes,
			Before: reading(input.Before, pair.before), After: reading(input.After, pair.after),
			Assumptions: []ValuationAssumption{},
		}
		row.Reason = evaluateWindow(input, result.Status, result.Period, pair, len(pools) > 1, &row)
		result.Windows = append(result.Windows, row)
	}
	return result
}

func observedTime(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	utc := value.UTC()
	return &utc
}

func pairWindows(before, after *ValuationEndpoint) ([]string, map[string]valuationPair) {
	var keys []string
	pairs := make(map[string]valuationPair)
	for side, endpoint := range []*ValuationEndpoint{before, after} {
		if endpoint == nil {
			continue
		}
		for i := range endpoint.Windows {
			window := &endpoint.Windows[i]
			key := window.WindowKey
			pair, found := pairs[key]
			if !found {
				keys = append(keys, key)
			}
			if side == 0 {
				pair.duplicate = pair.duplicate || pair.before != nil
				if pair.before == nil {
					pair.before = window
				}
			} else {
				pair.duplicate = pair.duplicate || pair.after != nil
				if pair.after == nil {
					pair.after = window
				}
			}
			pairs[key] = pair
		}
	}
	return keys, pairs
}

func reading(endpoint *ValuationEndpoint, window *SampleWindow) *ValuationReading {
	if window == nil || !validPercent(window.UsedPercent) || endpoint.ProbeCompletedAt.IsZero() {
		return nil
	}
	source := window.Source
	if source == "" {
		source = endpoint.Source
	}
	return &ValuationReading{
		UsedPercent: window.UsedPercent,
		ObservedAt:  endpoint.ProbeCompletedAt.UTC(),
		Source:      source,
	}
}

func validPercent(value float64) bool {
	return value >= 0 && value <= 100
}

func orderedProbes(input ValuationInput) bool {
	before, after := input.Before, input.After
	return !input.Evidence.ClockAmbiguous &&
		!before.ProbeStartedAt.IsZero() && !before.ProbeCompletedAt.IsZero() &&
		!after.ProbeStartedAt.IsZero() && !after.ProbeCompletedAt.IsZero() &&
		!before.ProbeCompletedAt.UTC().Before(before.ProbeStartedAt.UTC()) &&
		after.ProbeStartedAt.UTC().After(before.ProbeCompletedAt.UTC()) &&
		!after.ProbeCompletedAt.UTC().Before(after.ProbeStartedAt.UTC())
}

func validSample(window *SampleWindow, endpoint *ValuationEndpoint) bool {
	if window.WindowKey == "" || window.WindowMinutes <= 0 || window.WindowMinutesReason != "" ||
		!validPercent(window.UsedPercent) || window.ResetsAt.IsZero() || window.ResetTimeAmbiguous ||
		(window.Source != "" && window.Source != endpoint.Source) {
		return false
	}
	if int64(window.WindowMinutes) > math.MaxInt64/int64(time.Minute) {
		return false
	}
	reset := resetAnchor(endpoint.Source, window.ResetsAt)
	// Prose already reports the reset at minute precision. A finer-grained
	// input does not establish the same parsed anchor.
	if endpoint.Source == SourceClaudeProse && !window.ResetsAt.Equal(reset) {
		return false
	}
	start := reset.Add(-time.Duration(window.WindowMinutes) * time.Minute)
	return !endpoint.ProbeStartedAt.UTC().Before(start) && endpoint.ProbeCompletedAt.UTC().Before(reset)
}

func resetAnchor(source Source, instant time.Time) time.Time {
	precision := time.Second
	if source == SourceClaudeProse {
		precision = time.Minute
	}
	return instant.UTC().Truncate(precision)
}

func recordedReset(input ValuationInput, pair valuationPair) bool {
	if !resetAnchor(input.Before.Source, pair.before.ResetsAt).Equal(resetAnchor(input.After.Source, pair.after.ResetsAt)) {
		return true
	}
	for _, window := range []*SampleWindow{pair.before, pair.after} {
		at := window.ObservedResetAt.UTC()
		if window.ResetObserved || (window.HasObservedResetAt &&
			!at.Before(input.Before.ProbeStartedAt.UTC()) && !at.After(input.After.ProbeCompletedAt.UTC())) {
			return true
		}
	}
	return false
}

func evaluateWindow(input ValuationInput, status MeasurementStatus, period *ObservationPeriod, pair valuationPair, ambiguousPool bool, row *WindowValuation) ValuationReason {
	before, after := pair.before, pair.after
	switch {
	case status == MeasurementMissing:
		return ValuationMeasurementMissing
	case status == MeasurementActive:
		return ValuationMeasurementInProgress
	case before == nil || after == nil:
		return ValuationWindowMissing
	case pair.duplicate, period == nil, input.Evidence.SourceChanged,
		!validMeasurementSource(input.Client, input.Before.Source), input.Before.Source != input.After.Source,
		before.WindowMinutes != after.WindowMinutes, before.Slot != after.Slot,
		input.Client == ClientCodex && before.Slot == "",
		!validSample(before, input.Before), !validSample(after, input.After):
		return ValuationWindowUnverified
	case before.Pool == "" || before.Pool != after.Pool:
		return ValuationPoolUnresolved
	}

	usage := windowUsage(input, *period, before.Pool, ambiguousPool)
	endpointAccountChanged := input.Evidence.AccountChanged ||
		(input.Client == ClientCodex && input.Before.AccountDigest != "" &&
			input.After.AccountDigest != "" && input.Before.AccountDigest != input.After.AccountDigest)
	accountMissing := input.Client == ClientCodex &&
		(input.Before.AccountDigest == "" || input.After.AccountDigest == "")
	reset := recordedReset(input, pair)
	delta := percentRat(after.UsedPercent)
	delta.Sub(delta, percentRat(before.UsedPercent))
	if !endpointAccountChanged && !accountMissing && !reset {
		observed, _ := delta.Float64()
		row.ObservedDeltaPP = &observed
		if input.Client == ClientClaude {
			row.Assumptions = append(row.Assumptions, AssumeAccountUnverified)
		}
	}

	row.KnownBasis = usage.knownAmounts()
	switch {
	case usage.poolUnresolved:
		return ValuationPoolUnresolved
	case endpointAccountChanged || usage.accountChanged:
		return ValuationAccountChanged
	case accountMissing:
		return ValuationWindowUnverified
	case reset:
		return ValuationResetDetected
	case usage.otherLocal:
		return ValuationOtherLocalUsage
	case usage.unsettled || input.Evidence.Unsettled:
		return ValuationUnsettled
	case usage.mixed:
		return ValuationMixedBilling
	case usage.pricingIncomplete:
		return ValuationPricingIncomplete
	case len(row.KnownBasis) > 1:
		return ValuationCurrencyMismatch
	}

	estimate, reason := estimateWindow(pair, usage, delta)
	if reason != "" {
		return reason
	}
	row.Estimate = estimate
	if input.Client == ClientCodex {
		row.Assumptions = append(row.Assumptions, AssumeSessionUsesSampledAccount)
	}
	row.Assumptions = append(row.Assumptions, AssumeAllChangeFromSelectedUsage, AssumeReportedUsageSettled)
	if estimate.Amount != "" {
		row.Assumptions = append(row.Assumptions, AssumeReportedPrecisionUnknown)
	}
	if usage.scopeAssumed {
		row.Assumptions = append(row.Assumptions, AssumeSubscriptionScope)
	}
	return ""
}

func validMeasurementSource(client Client, source Source) bool {
	return client == ClientCodex && source == SourceCodex ||
		client == ClientClaude && source == SourceClaudeProse
}

func (period ObservationPeriod) contains(at time.Time) bool {
	return !at.IsZero() && !at.UTC().Before(period.Start) && at.UTC().Before(period.End)
}

func measurementCoverage(input ValuationInput, period ObservationPeriod) ValuationCoverage {
	if input.SessionID == "" {
		return CoverageUnknown
	}
	var found, outside bool
	for _, call := range input.Calls {
		if call.Client != input.Client || call.SessionID != input.SessionID {
			continue
		}
		if call.OccurredAt.IsZero() {
			return CoverageUnknown
		}
		found = true
		outside = outside || !period.contains(call.OccurredAt)
	}
	if !found {
		return CoverageUnknown
	}
	if outside {
		return CoveragePortion
	}
	return CoverageFull
}
