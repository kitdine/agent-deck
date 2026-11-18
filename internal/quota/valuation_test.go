package quota

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"
	"time"
)

func valuationFixture() ValuationInput {
	start := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	reset := start.Add(4 * time.Hour)
	window := SampleWindow{
		Window: Window{
			WindowKey: "codex", Label: "Five hours", Source: SourceCodex, WindowMinutes: 300,
			UsedPercent: 40, ResetsAt: reset,
		},
		Pool: "codex", Slot: "primary",
	}
	before := &ValuationEndpoint{
		ProbeStartedAt: start, ProbeCompletedAt: start.Add(time.Minute),
		Source: SourceCodex, AccountDigest: "sampled-account", Windows: []SampleWindow{window},
	}
	window.UsedPercent = 44
	after := &ValuationEndpoint{
		ProbeStartedAt: start.Add(20 * time.Minute), ProbeCompletedAt: start.Add(21 * time.Minute),
		Source: SourceCodex, AccountDigest: "sampled-account", Windows: []SampleWindow{window},
	}
	return ValuationInput{Client: ClientCodex, SessionID: "selected", Before: before, After: after,
		Calls: []ValuationCall{{
			Client: ClientCodex, SessionID: "selected", OccurredAt: start.Add(10 * time.Minute),
			Pool: "codex", Eligibility: EligibilityIncluded, Currency: "USD", Amount: "6.00", PriceComplete: true,
		}},
	}
}

func valuationWeeklyWindow(input *ValuationInput) {
	before := input.Before.Windows[0]
	before.WindowKey, before.Label, before.Slot = "codex_secondary", "Weekly", "secondary"
	before.WindowMinutes = 10080
	before.ResetsAt = before.ResetsAt.Add(72 * time.Hour)
	after := before
	after.UsedPercent = 42
	input.Before.Windows = append(input.Before.Windows, before)
	input.After.Windows = append(input.After.Windows, after)
}

func valuationRow(t *testing.T, input ValuationInput) WindowValuation {
	t.Helper()
	result := EvaluateValuation(input)
	if len(result.Windows) != 1 {
		t.Fatalf("got %d windows, want 1", len(result.Windows))
	}
	return result.Windows[0]
}

func assertObservedDelta(t *testing.T, row WindowValuation, want float64) {
	t.Helper()
	if row.ObservedDeltaPP == nil || *row.ObservedDeltaPP != want {
		t.Fatalf("expected observation %v pp, got %+v", want, row)
	}
}

func TestEvaluateValuationIndependentWindows(t *testing.T) {
	input := valuationFixture()
	valuationWeeklyWindow(&input)
	result := EvaluateValuation(input)
	if result.Version != 1 || result.Status != MeasurementComplete || result.Coverage != CoverageFull ||
		result.StartedAt == nil || !result.StartedAt.Equal(input.Before.ProbeStartedAt) ||
		result.FinishedAt == nil || !result.FinishedAt.Equal(input.After.ProbeCompletedAt) ||
		result.Period == nil || !result.Period.Start.Equal(input.Before.ProbeCompletedAt) ||
		!result.Period.End.Equal(input.After.ProbeStartedAt) || len(result.Windows) != 2 {
		t.Fatalf("unexpected measurement %+v", result)
	}
	for i, want := range []struct {
		key, amount string
		delta       float64
	}{{"codex", "150.000000000", 4}, {"codex_secondary", "300.000000000", 2}} {
		row := result.Windows[i]
		assertObservedDelta(t, row, want.delta)
		estimate := WindowEstimate{Currency: "USD", Amount: want.amount, BasisAmount: "6.000000000"}
		assumptions := []ValuationAssumption{AssumeSessionUsesSampledAccount,
			AssumeAllChangeFromSelectedUsage, AssumeReportedUsageSettled, AssumeReportedPrecisionUnknown}
		if row.WindowKey != want.key || row.Estimate == nil || *row.Estimate != estimate ||
			row.Reason != "" || !reflect.DeepEqual(row.Assumptions, assumptions) {
			t.Fatalf("unexpected valued window %+v, estimate %+v", row, row.Estimate)
		}
		if row.Before.UsedPercent != 40 || row.After.UsedPercent != 40+want.delta ||
			row.Before.Source != SourceCodex || row.After.Source != SourceCodex ||
			!row.Before.ObservedAt.Equal(input.Before.ProbeCompletedAt) ||
			!row.After.ObservedAt.Equal(input.After.ProbeCompletedAt) {
			t.Fatalf("wrong reading provenance: before %+v after %+v", row.Before, row.After)
		}
	}
	if !reflect.DeepEqual(result, EvaluateValuation(input)) {
		t.Fatal("same input yielded a different result")
	}
}

func TestEvaluateValuationMeasurementStates(t *testing.T) {
	input := valuationFixture()
	missing := EvaluateValuation(ValuationInput{})
	if missing.Status != MeasurementMissing || missing.Coverage != CoverageUnknown ||
		missing.StartedAt != nil || missing.FinishedAt != nil || missing.Windows == nil || len(missing.Windows) != 0 {
		t.Fatalf("unexpected missing result %+v", missing)
	}

	finish := input.After
	input.After = nil
	active := EvaluateValuation(input)
	if active.Status != MeasurementActive || active.Coverage != CoverageUnknown ||
		active.StartedAt == nil || active.FinishedAt != nil || len(active.Windows) != 1 {
		t.Fatalf("unexpected active result %+v", active)
	}
	row := active.Windows[0]
	if row.Reason != ValuationMeasurementInProgress || row.Before == nil || row.After != nil ||
		row.Estimate != nil || row.ObservedDeltaPP != nil {
		t.Fatalf("unexpected active window %+v", row)
	}

	input.After = finish
	input.After.Windows[0].WindowKey = "finish_only"
	result := EvaluateValuation(input)
	if result.Status != MeasurementComplete || len(result.Windows) != 2 {
		t.Fatalf("unexpected complete result %+v", result)
	}
	first, second := result.Windows[0], result.Windows[1]
	if first.WindowKey != "codex" || first.Before == nil || first.After != nil ||
		second.WindowKey != "finish_only" || second.Before != nil || second.After == nil {
		t.Fatalf("expected before-only then after-only, got %+v", result.Windows)
	}
	for _, row := range result.Windows {
		if row.Reason != ValuationWindowMissing || row.ObservedDeltaPP != nil || row.Estimate != nil {
			t.Fatalf("unexpected unpaired window %+v", row)
		}
	}
}

func TestEvaluateValuationRejectsIncomparableObservations(t *testing.T) {
	tests := []struct {
		name   string
		change func(*ValuationInput)
		reason ValuationReason
	}{
		{"missing Codex account", func(in *ValuationInput) { in.Before.AccountDigest = "" }, ValuationWindowUnverified},
		{"different Codex account", func(in *ValuationInput) { in.After.AccountDigest = "different" }, ValuationAccountChanged},
		{"different pool", func(in *ValuationInput) { in.After.Windows[0].Pool = "review" }, ValuationPoolUnresolved},
		{"different slot", func(in *ValuationInput) { in.After.Windows[0].Slot = "secondary" }, ValuationWindowUnverified},
		{"different length", func(in *ValuationInput) { in.After.Windows[0].WindowMinutes *= 2 }, ValuationWindowUnverified},
		{"duplicate key", func(in *ValuationInput) { in.After.Windows = append(in.After.Windows, in.After.Windows[0]) }, ValuationWindowUnverified},
		{"probes reversed", func(in *ValuationInput) { in.Before, in.After = in.After, in.Before }, ValuationWindowUnverified},
		{"probes overlap", func(in *ValuationInput) { in.After.ProbeStartedAt = in.Before.ProbeStartedAt }, ValuationWindowUnverified},
		{"end before start", func(in *ValuationInput) { in.After.ProbeCompletedAt = in.Before.ProbeStartedAt }, ValuationWindowUnverified},
		{"missing reset", func(in *ValuationInput) { in.After.Windows[0].ResetsAt = time.Time{} }, ValuationWindowUnverified},
		{"known ambiguous reset time", func(in *ValuationInput) { in.After.Windows[0].ResetTimeAmbiguous = true }, ValuationWindowUnverified},
		{"known ambiguous chronology", func(in *ValuationInput) { in.Evidence.ClockAmbiguous = true }, ValuationWindowUnverified},
		{"observed at reset anchor", func(in *ValuationInput) {
			in.Before.Windows[0].ResetsAt = in.After.ProbeCompletedAt
			in.After.Windows[0].ResetsAt = in.After.ProbeCompletedAt
		}, ValuationWindowUnverified},
		{"probe starts before window", func(in *ValuationInput) {
			in.Before.Windows[0].ResetsAt = in.Before.ProbeCompletedAt.Add(5 * time.Hour)
		}, ValuationWindowUnverified},
		{"source changed and returned", func(in *ValuationInput) { in.Evidence.SourceChanged = true }, ValuationWindowUnverified},
		{"source replaced", func(in *ValuationInput) { in.After.Source = SourceClaudeStatusLine }, ValuationWindowUnverified},
		{"window source differs from its probe", func(in *ValuationInput) { in.After.Windows[0].Source = SourceClaudeProse }, ValuationWindowUnverified},
		{"nonfinite percentage", func(in *ValuationInput) { in.After.Windows[0].UsedPercent = math.NaN() }, ValuationWindowUnverified},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := valuationFixture()
			tt.change(&input)
			row := valuationRow(t, input)
			if row.Estimate != nil || row.ObservedDeltaPP != nil || row.Reason != tt.reason {
				t.Fatalf("unexpected unpaired window %+v, want %q", row, tt.reason)
			}
		})
	}
}

func TestEvaluateValuationRetainsDeltaForDifferentCallAccount(t *testing.T) {
	input := valuationFixture()
	input.Calls[0].AccountDigest = "different"
	row := valuationRow(t, input)
	assertObservedDelta(t, row, 4)
	if row.Reason != ValuationAccountChanged || row.Estimate != nil {
		t.Fatalf("unexpected valuation with a different call account: %+v", row)
	}
}

func TestEvaluateValuationClaudeAccountAndAnchor(t *testing.T) {
	input := valuationFixture()
	input.Client, input.Calls[0].Client = ClientClaude, ClientClaude
	input.Calls[0].Pool = "claude"
	for _, endpoint := range []*ValuationEndpoint{input.Before, input.After} {
		endpoint.AccountDigest = ""
		endpoint.Source, endpoint.Windows[0].Source = SourceClaudeProse, SourceClaudeProse
		endpoint.Windows[0].WindowKey = ClaudeWindowFiveHour
		endpoint.Windows[0].Pool = "claude"
	}
	input.After.Windows[0].ResetsAt = input.After.Windows[0].ResetsAt.In(time.FixedZone("offset", -5*3600))
	row := valuationRow(t, input)
	assertObservedDelta(t, row, 4)
	wantAssumptions := []ValuationAssumption{AssumeAccountUnverified, AssumeAllChangeFromSelectedUsage,
		AssumeReportedUsageSettled, AssumeReportedPrecisionUnknown}
	if row.Estimate == nil || row.Estimate.Amount != "150.000000000" ||
		row.Reason != "" || !reflect.DeepEqual(row.Assumptions, wantAssumptions) {
		t.Fatalf("unexpected Claude estimate %+v", row)
	}
	input.Evidence.AccountChanged = true
	row = valuationRow(t, input)
	if row.Estimate != nil || row.ObservedDeltaPP != nil || row.Reason != ValuationAccountChanged {
		t.Fatalf("known Claude account switch: %+v", row)
	}

	input.Evidence.AccountChanged = false
	anchor := input.After.Windows[0].ResetsAt
	input.After.Windows[0].ResetsAt = anchor.Add(time.Minute)
	row = valuationRow(t, input)
	if row.Estimate != nil || row.ObservedDeltaPP != nil || row.Reason != ValuationResetDetected {
		t.Fatalf("distinct Claude minute anchors: %+v", row)
	}

	input.After.Windows[0].ResetsAt = anchor.Add(time.Second)
	row = valuationRow(t, input)
	if row.Estimate != nil || row.ObservedDeltaPP != nil || row.Reason != ValuationWindowUnverified {
		t.Fatalf("invalid finer-grained Claude anchor: %+v", row)
	}

	input.After.Windows[0].ResetsAt = anchor
	input.After.Source, input.After.Windows[0].Source = SourceClaudeStatusLine, SourceClaudeStatusLine
	row = valuationRow(t, input)
	if row.Estimate != nil || row.ObservedDeltaPP != nil || row.Reason != ValuationWindowUnverified {
		t.Fatalf("Claude prose mixed with status line: %+v", row)
	}
	input.Before.Source, input.Before.Windows[0].Source = SourceClaudeStatusLine, SourceClaudeStatusLine
	row = valuationRow(t, input)
	if row.Estimate != nil || row.ObservedDeltaPP != nil || row.Reason != ValuationWindowUnverified {
		t.Fatalf("status line used as explicit Claude sample: %+v", row)
	}
}

func TestEvaluateValuationWindowResetIsScoped(t *testing.T) {
	for _, how := range []string{"scheduled anchor changed", "intervening reset", "start probe reset", "finish probe reset"} {
		t.Run(how, func(t *testing.T) {
			input := valuationFixture()
			valuationWeeklyWindow(&input)
			// The unchanged weekly window can use its own UTC second anchor.
			weekly := &input.After.Windows[1]
			weekly.ResetsAt = weekly.ResetsAt.In(time.FixedZone("offset", 3*3600)).Add(time.Nanosecond)
			switch how {
			case "scheduled anchor changed":
				input.Before.Windows[0].ResetsAt = input.Before.ProbeStartedAt.Add(12 * time.Minute)
				input.After.Windows[0].ResetsAt = input.Before.Windows[0].ResetsAt.Add(5 * time.Hour)
			case "intervening reset":
				input.After.Windows[0].ObservedResetAt = input.Calls[0].OccurredAt
				input.After.Windows[0].HasObservedResetAt = true
			case "start probe reset":
				input.Before.Windows[0].ResetObserved = true
			case "finish probe reset":
				input.After.Windows[0].ResetObserved = true
			}
			windows := EvaluateValuation(input).Windows
			short, week := windows[0], windows[1]
			if short.Reason != ValuationResetDetected || short.ObservedDeltaPP != nil || short.Estimate != nil ||
				short.Before == nil || short.After == nil {
				t.Fatalf("reset window %+v", short)
			}
			assertObservedDelta(t, week, 2)
			if week.Estimate == nil || week.Estimate.Amount != "300.000000000" || week.Reason != "" {
				t.Fatalf("reset affected unrelated weekly window %+v", week)
			}
		})
	}
	input := valuationFixture()
	input.After.Windows[0].ObservedResetAt = input.Before.ProbeStartedAt.Add(-time.Hour)
	input.After.Windows[0].HasObservedResetAt = true
	if row := valuationRow(t, input); row.Estimate == nil || row.Reason != "" {
		t.Fatalf("sticky reset from before observation blocked valuation: %+v", row)
	}
}

func TestEvaluateValuationPriceAndDenominator(t *testing.T) {
	one, tenth, invalid := 1.0, 0.1, math.Inf(1)
	tests := []struct {
		name     string
		change   func(*ValuationInput)
		delta    float64
		reason   ValuationReason
		estimate *WindowEstimate
	}{
		{"nearest one point", func(in *ValuationInput) {
			in.Before.Windows[0].PrecisionPP, in.After.Windows[0].PrecisionPP = &one, &one
		}, 4, "", &WindowEstimate{Currency: "USD", Lower: "120.000000000", Upper: "200.000000000", BasisAmount: "6.000000000"}},
		{"clipped range at zero", func(in *ValuationInput) {
			in.Before.Windows[0].UsedPercent, in.After.Windows[0].UsedPercent = 0, 4
			in.Before.Windows[0].PrecisionPP, in.After.Windows[0].PrecisionPP = &one, &one
		}, 4, "", &WindowEstimate{Currency: "USD", Lower: "133.333333333", Upper: "200.000000000", BasisAmount: "6.000000000"}},
		{"one unknown rounding increment", func(in *ValuationInput) {
			in.After.Windows[0].PrecisionPP = &one
		}, 4, "", &WindowEstimate{Currency: "USD", Amount: "150.000000000", BasisAmount: "6.000000000"}},
		{"rounded denominator touches zero", func(in *ValuationInput) {
			in.After.Windows[0].UsedPercent = 41
			in.Before.Windows[0].PrecisionPP, in.After.Windows[0].PrecisionPP = &one, &one
		}, 1, ValuationNoReliableDelta, nil},
		{"fractional rounded denominator touches zero", func(in *ValuationInput) {
			in.Before.Windows[0].UsedPercent, in.After.Windows[0].UsedPercent = 2.2, 2.3
			in.Before.Windows[0].PrecisionPP, in.After.Windows[0].PrecisionPP = &tenth, &tenth
		}, 0.1, ValuationNoReliableDelta, nil},
		{"unknown precision at saturation", func(in *ValuationInput) {
			in.After.Windows[0].UsedPercent = 100
		}, 60, ValuationSaturated, nil},
		{"rounded at saturation", func(in *ValuationInput) {
			in.After.Windows[0].UsedPercent = 100
			in.Before.Windows[0].PrecisionPP, in.After.Windows[0].PrecisionPP = &one, &one
		}, 60, ValuationSaturated, nil},
		{"zero denominator", func(in *ValuationInput) { in.After.Windows[0].UsedPercent = 40 }, 0, ValuationNoReliableDelta, nil},
		{"negative denominator", func(in *ValuationInput) { in.After.Windows[0].UsedPercent = 39 }, -1, ValuationNoReliableDelta, nil},
		{"invalid precision", func(in *ValuationInput) { in.After.Windows[0].PrecisionPP = &invalid }, 4, ValuationNoReliableDelta, nil},
		{"known zero dollar", func(in *ValuationInput) { in.Calls[0].Amount = "0" }, 4, ValuationNoReliableDelta, nil},
		{"no price", func(in *ValuationInput) { in.Calls[0].Amount, in.Calls[0].PriceComplete = "", false }, 4, ValuationPricingIncomplete, nil},
		{"no currency", func(in *ValuationInput) { in.Calls[0].Currency = "" }, 4, ValuationPricingIncomplete, nil},
		{"malformed decimal", func(in *ValuationInput) { in.Calls[0].Amount = "6/1" }, 4, ValuationPricingIncomplete, nil},
		{"observed ambiguity and incomplete price", func(in *ValuationInput) {
			in.After.Windows[0].UsedPercent = 40
			in.Calls[0].PriceComplete = false
		}, 0, ValuationPricingIncomplete, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := valuationFixture()
			tt.change(&input)
			row := valuationRow(t, input)
			assertObservedDelta(t, row, tt.delta)
			if !reflect.DeepEqual(row.Estimate, tt.estimate) || row.Reason != tt.reason {
				t.Fatalf("estimate %+v reason %q, want %+v and %q", row.Estimate, row.Reason, tt.estimate, tt.reason)
			}
			if row.Estimate == nil && len(row.Assumptions) != 0 {
				t.Fatalf("a missing estimate carried estimate assumptions %+v", row)
			}
			if tt.name == "known zero dollar" &&
				!reflect.DeepEqual(row.KnownBasis, []ValuationAmount{{"USD", "0.000000000"}}) {
				t.Fatalf("known zero basis not retained: %+v", row.KnownBasis)
			}
		})
	}
}

func TestEvaluateValuationEligibilityCompetitionAndPartialScope(t *testing.T) {
	input := valuationFixture()
	input.Calls[0].Eligibility = EligibilityUnknown
	prior := input.Calls[0]
	prior.OccurredAt = input.Before.ProbeStartedAt.Add(-time.Minute)
	prior.Amount = "18"
	excluded := input.Calls[0]
	excluded.Eligibility, excluded.Currency, excluded.Amount = EligibilityExcluded, "EUR", ""
	unrelated := input.Calls[0]
	unrelated.SessionID, unrelated.Pool = "other", "review"
	otherAccount := input.Calls[0]
	otherAccount.SessionID, otherAccount.AccountDigest = "other", "different"
	otherClient := input.Calls[0]
	otherClient.Client = ClientClaude
	later := input.Calls[0]
	later.OccurredAt = input.After.ProbeCompletedAt.Add(time.Minute)
	input.Calls = append(input.Calls, prior, excluded, unrelated, otherAccount, otherClient, later)
	result := EvaluateValuation(input)
	row := result.Windows[0]
	if result.Coverage != CoveragePortion || row.Estimate == nil || row.Estimate.BasisAmount != "6.000000000" ||
		len(row.Assumptions) == 0 || row.Assumptions[len(row.Assumptions)-1] != AssumeSubscriptionScope {
		t.Fatalf("unexpected partial result %+v, row %+v", result, row)
	}
	input.Calls[0].Eligibility = EligibilityExcluded
	row = valuationRow(t, input)
	assertObservedDelta(t, row, 4)
	if row.Estimate != nil || row.Reason != ValuationNoReliableDelta || len(row.KnownBasis) != 0 || len(row.Assumptions) != 0 {
		t.Fatalf("excluded-only numerator produced a value or assumption: %+v", row)
	}

	input.Calls[0].Eligibility = EligibilityIncluded
	local := input.Calls[0]
	local.SessionID, local.Pool, local.Eligibility = "other", "", EligibilityUnknown
	input.Calls = append(input.Calls, local)
	row = valuationRow(t, input)
	assertObservedDelta(t, row, 4)
	if row.Estimate != nil || row.Reason != ValuationOtherLocalUsage ||
		!reflect.DeepEqual(row.KnownBasis, []ValuationAmount{{"USD", "6.000000000"}}) {
		t.Fatalf("known local competition result %+v", row)
	}
}

func TestEvaluateValuationPoolMapping(t *testing.T) {
	input := valuationFixture()
	reviewBefore, reviewAfter := input.Before.Windows[0], input.After.Windows[0]
	reviewBefore.WindowKey, reviewBefore.Pool = "review", "review"
	reviewAfter.WindowKey, reviewAfter.Pool = "review", "review"
	input.Before.Windows = append(input.Before.Windows, reviewBefore)
	input.After.Windows = append(input.After.Windows, reviewAfter)
	input.Calls[0].Pool = ""
	for _, row := range EvaluateValuation(input).Windows {
		assertObservedDelta(t, row, 4)
		if row.Estimate != nil || row.Reason != ValuationPoolUnresolved {
			t.Fatalf("unresolved pool produced %+v", row)
		}
	}

	input.Calls[0].Pool = "codex"
	review := input.Calls[0]
	review.Pool, review.Amount = "review", "3"
	input.Calls = append(input.Calls, review)
	rows := EvaluateValuation(input).Windows
	if rows[0].Estimate == nil || rows[0].Estimate.Amount != "150.000000000" ||
		rows[1].Estimate == nil || rows[1].Estimate.Amount != "75.000000000" {
		t.Fatalf("explicit pool mapping produced %+v", rows)
	}
	input.Calls[0].Pool = ""
	input.Before.Windows, input.After.Windows = input.Before.Windows[:1], input.After.Windows[:1]
	row := valuationRow(t, input)
	if row.Estimate == nil || row.Estimate.BasisAmount != "6.000000000" {
		t.Fatalf("single possible pool did not match: %+v", row)
	}
}

func TestEvaluateValuationReasonPrecedence(t *testing.T) {
	input := valuationFixture()
	competing := input.Calls[0]
	competing.SessionID = "other"
	mixed := input.Calls[0]
	mixed.Eligibility = EligibilityMixed
	incomplete := input.Calls[0]
	incomplete.Amount, incomplete.PriceComplete = "1.25", false
	foreign := input.Calls[0]
	foreign.Currency, foreign.Amount = "EUR", "2"
	input.Calls = append(input.Calls, foreign, incomplete, mixed, competing)
	input.After.Windows[0].UsedPercent = 40
	input.Evidence = ValuationEvidence{SourceChanged: true, AccountChanged: true, Unsettled: true}
	input.After.Windows[0].ResetObserved = true

	tests := []struct {
		want   ValuationReason
		remove func()
		delta  bool
	}{
		{ValuationWindowUnverified, func() { input.Evidence.SourceChanged = false }, false},
		{ValuationAccountChanged, func() { input.Evidence.AccountChanged = false }, false},
		{ValuationResetDetected, func() { input.After.Windows[0].ResetObserved = false }, false},
		{ValuationOtherLocalUsage, func() { input.Calls = input.Calls[:4] }, true},
		{ValuationUnsettled, func() { input.Evidence.Unsettled = false }, true},
		{ValuationMixedBilling, func() { input.Calls = input.Calls[:3] }, true},
		{ValuationPricingIncomplete, func() { input.Calls = input.Calls[:2] }, true},
		{ValuationCurrencyMismatch, func() { input.Calls = input.Calls[:1] }, true},
		{ValuationNoReliableDelta, func() {}, true},
	}
	for _, tt := range tests {
		t.Run(string(tt.want), func(t *testing.T) {
			row := valuationRow(t, input)
			if row.Reason != tt.want || row.Estimate != nil || (row.ObservedDeltaPP != nil) != tt.delta {
				t.Fatalf("got %+v, want %q without estimate, delta present %t", row, tt.want, tt.delta)
			}
			if tt.want == ValuationPricingIncomplete {
				want := []ValuationAmount{{"EUR", "2.000000000"}, {"USD", "7.250000000"}}
				if !reflect.DeepEqual(row.KnownBasis, want) {
					t.Fatalf("known partial totals %+v, want %+v", row.KnownBasis, want)
				}
			}
		})
		tt.remove()
	}
}

func TestEvaluateValuationProbeAcquisitionBoundaries(t *testing.T) {
	input := valuationFixture()
	times := []struct {
		name string
		at   time.Time
		want ValuationReason
	}{
		{"call at begin completion", input.Before.ProbeCompletedAt, ""},
		{"call during begin", input.Before.ProbeStartedAt.Add(time.Second), ValuationUnsettled},
		{"call at finish start", input.After.ProbeStartedAt, ValuationUnsettled},
		{"call during finish", input.After.ProbeStartedAt.Add(time.Second), ValuationUnsettled},
		{"call at finish completion", input.After.ProbeCompletedAt, ValuationUnsettled},
		{"call has unknown chronology", time.Time{}, ValuationUnsettled},
	}
	for _, tt := range times {
		t.Run(tt.name, func(t *testing.T) {
			input.Calls[0].OccurredAt = tt.at
			row := valuationRow(t, input)
			assertObservedDelta(t, row, 4)
			if row.Reason != tt.want || (row.Estimate == nil) != (tt.want != "") {
				t.Fatalf("unexpected valuation at %v: %+v", tt.at, row)
			}
		})
	}
}

func TestEvaluateValuationCompetingProbeCalls(t *testing.T) {
	fixture := valuationFixture()
	tests := []struct {
		name    string
		at      time.Time
		account string
		want    ValuationReason
	}{
		{"during begin", fixture.Before.ProbeStartedAt.Add(time.Second), fixture.Before.AccountDigest, ValuationOtherLocalUsage},
		{"during finish", fixture.After.ProbeStartedAt.Add(time.Second), fixture.Before.AccountDigest, ValuationOtherLocalUsage},
		{"unknown chronology", time.Time{}, fixture.Before.AccountDigest, ValuationOtherLocalUsage},
		{"different account during finish", fixture.After.ProbeStartedAt.Add(time.Second), "different", ""},
		{"after finish", fixture.After.ProbeCompletedAt.Add(time.Second), fixture.Before.AccountDigest, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := valuationFixture()
			competingCall := input.Calls[0]
			competingCall.SessionID = "other"
			competingCall.OccurredAt = tt.at
			competingCall.AccountDigest = tt.account
			input.Calls = append(input.Calls, competingCall)
			row := valuationRow(t, input)
			assertObservedDelta(t, row, 4)
			if row.Reason != tt.want || (row.Estimate == nil) != (tt.want != "") {
				t.Fatalf("unexpected valuation with competing call at %v: %+v", tt.at, row)
			}
		})
	}
}

func TestEvaluateValuationWireDistinguishesZeroFromAbsent(t *testing.T) {
	input := valuationFixture()
	input.After.Windows[0].UsedPercent = 40
	window := valuationRow(t, input)
	encoded, err := json.Marshal(window)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	if string(fields["observed_delta_pp"]) != "0" || string(fields["reason"]) != `"no_reliable_delta"` ||
		fields["estimate"] != nil || fields["KnownBasis"] != nil {
		t.Fatalf("unexpected zero-observation wire %s", encoded)
	}

	input.After.AccountDigest = ""
	encoded, err = json.Marshal(valuationRow(t, input))
	if err != nil {
		t.Fatal(err)
	}
	fields = nil
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	if fields["observed_delta_pp"] != nil || fields["estimate"] != nil || fields["before"] == nil || fields["after"] == nil {
		t.Fatalf("unexpected unknown-observation wire %s", encoded)
	}
}
