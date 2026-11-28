package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/kitdine/agent-deck/internal/quota"
)

type quotaValuationText struct {
	summary []priceTextField
	details []priceTextField
}

func quotaValuationFields(value *quota.ValuationResult) quotaValuationText {
	if value == nil {
		return quotaValuationText{}
	}
	report := quotaValuationText{}
	switch value.Status {
	case quota.MeasurementMissing:
		message := "No saved measurement"
		if value.ProbeDisabled {
			message = "Probe disabled · " + message
		}
		report.summary = append(report.summary, priceTextField{"Quota valuation", message})
		return report
	case quota.MeasurementActive:
		report.summary = append(report.summary, priceTextField{"Quota valuation", "Measurement in progress · finish to see window changes"})
	case quota.MeasurementComplete:
		message := "Measurement complete"
		if value.Coverage == quota.CoveragePortion {
			message = "Measured portion"
		}
		if len(value.Windows) == 0 {
			message += " · No comparable windows"
		}
		report.summary = append(report.summary, priceTextField{"Quota valuation", message})
	}
	if value.Period != nil {
		report.details = append(report.details, priceTextField{"Measured interval",
			renderDisplayTimeWithZone(value.Period.Start) + " – " + renderDisplayTimeWithZone(value.Period.End)})
	} else if value.StartedAt != nil {
		report.details = append(report.details, priceTextField{"Measurement start", renderDisplayTimeWithZone(*value.StartedAt)})
	}
	pools := make(map[string]struct{})
	for _, window := range value.Windows {
		pools[window.Pool] = struct{}{}
	}
	for _, window := range value.Windows {
		label := window.Label
		if len(pools) > 1 && window.Pool != "" {
			label += " · " + window.Pool
		}
		var lines []string
		if window.ObservedDeltaPP != nil {
			lines = append(lines, quotaPercent(*window.ObservedDeltaPP)+" pp observed")
		}
		if window.Estimate != nil {
			lines = append(lines, quotaEstimateText(window.Estimate))
		} else if window.Reason != "" {
			lines = append(lines, quotaValuationReason(window))
		}
		report.summary = append(report.summary, priceTextField{label, strings.Join(lines, " · ")})
		report.details = append(report.details, quotaWindowDetails(label, window)...)
	}
	return report
}

func quotaValuationLines(value *quota.ValuationResult, width int) []string {
	report := quotaValuationFields(value)
	var lines []string
	for _, field := range append(report.summary, report.details...) {
		lines = append(lines, sessionShowPricingFieldLines(field, width)...)
	}
	return lines
}

func quotaPercent(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func quotaMoney(currency, amount string) string {
	if strings.Contains(amount, ".") {
		amount = strings.TrimRight(strings.TrimRight(amount, "0"), ".")
	}
	return currency + " " + amount
}

func quotaEstimateText(estimate *quota.WindowEstimate) string {
	if estimate.Amount != "" {
		return "about " + quotaMoney(estimate.Currency, estimate.Amount) + " if this session used all of it"
	}
	return quotaMoney(estimate.Currency, estimate.Lower) + "–" + quotaMoney(estimate.Currency, estimate.Upper) + " if this session used all of it"
}

func quotaWindowDetails(label string, window quota.WindowValuation) []priceTextField {
	var fields []priceTextField
	for _, item := range []struct {
		name    string
		reading *quota.ValuationReading
	}{{"before", window.Before}, {"after", window.After}} {
		if item.reading != nil {
			fields = append(fields, priceTextField{label + " " + item.name,
				fmt.Sprintf("%s%% used · %s · %s", quotaPercent(item.reading.UsedPercent),
					renderDisplayTimeWithZone(item.reading.ObservedAt), quotaReadingSource(item.reading.Source))})
		}
	}
	if window.Estimate != nil {
		fields = append(fields, priceTextField{label + " API basis", quotaMoney(window.Estimate.Currency, window.Estimate.BasisAmount)})
	} else {
		for _, amount := range window.KnownBasis {
			fields = append(fields, priceTextField{label + " API basis", quotaMoney(amount.Currency, amount.Amount) + " known"})
		}
	}
	if len(window.Assumptions) > 0 {
		assumptions := make([]string, 0, len(window.Assumptions))
		for _, assumption := range window.Assumptions {
			assumptions = append(assumptions, quotaAssumptionText[assumption])
		}
		fields = append(fields, priceTextField{label + " assumptions", strings.Join(assumptions, " · ")})
	}
	return fields
}

func quotaReadingSource(source quota.Source) string {
	switch source {
	case quota.SourceClaudeProse:
		return "Claude /usage"
	case quota.SourceCodex:
		return "Codex rate limits"
	default:
		return "Source unavailable"
	}
}

func quotaValuationReason(window quota.WindowValuation) string {
	if window.Reason == quota.ValuationWindowMissing {
		if window.Before == nil {
			return "No comparable start reading"
		}
		return "No comparable finish reading"
	}
	return quotaReasonText[window.Reason]
}

var quotaReasonText = map[quota.ValuationReason]string{
	quota.ValuationMeasurementMissing:    "No saved measurement",
	quota.ValuationMeasurementInProgress: "Waiting for finish",
	quota.ValuationWindowUnverified:      "Window or account could not be verified",
	quota.ValuationPoolUnresolved:        "Quota pool unresolved",
	quota.ValuationAccountChanged:        "Account changed during measurement",
	quota.ValuationResetDetected:         "Reset during measurement",
	quota.ValuationOtherLocalUsage:       "Other local usage detected · estimate unavailable",
	quota.ValuationUnsettled:             "Usage is still settling",
	quota.ValuationMixedBilling:          "Mixed or unresolved billing",
	quota.ValuationPricingIncomplete:     "Pricing incomplete",
	quota.ValuationCurrencyMismatch:      "Pricing currencies differ",
	quota.ValuationNoReliableDelta:       "No reliable change or eligible amount for an estimate",
	quota.ValuationSaturated:             "Window saturated · estimate unavailable",
}

var quotaAssumptionText = map[quota.ValuationAssumption]string{
	quota.AssumeSessionUsesSampledAccount:  "Session uses the sampled account",
	quota.AssumeAccountUnverified:          "Account unverified · assumes the same account",
	quota.AssumeAllChangeFromSelectedUsage: "All change came from the selected usage",
	quota.AssumeReportedUsageSettled:       "Reported usage has settled",
	quota.AssumeReportedPrecisionUnknown:   "Based on reported percentages · rounding unknown",
	quota.AssumeSubscriptionScope:          "Subscription eligibility assumed",
}
