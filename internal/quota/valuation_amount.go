package quota

import (
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"
)

type valuationUsage struct {
	totals map[string]*big.Rat

	poolUnresolved    bool
	accountChanged    bool
	otherLocal        bool
	unsettled         bool
	mixed             bool
	pricingIncomplete bool
	scopeAssumed      bool
}

func windowUsage(input ValuationInput, period ObservationPeriod, pool string, ambiguousPool bool) valuationUsage {
	usage := valuationUsage{totals: make(map[string]*big.Rat)}
	if input.SessionID == "" {
		return usage
	}
	for _, call := range input.Calls {
		if call.Client != input.Client || call.Eligibility == EligibilityExcluded ||
			(call.Pool != "" && call.Pool != pool) {
			continue
		}
		selected := call.SessionID == input.SessionID
		sampledAccount := input.Before.AccountDigest
		accountMismatch := input.Client == ClientCodex && sampledAccount != "" &&
			call.AccountDigest != "" && call.AccountDigest != sampledAccount

		if !period.contains(call.OccurredAt) {
			// Unknown times and calls during probe acquisition may affect the observed change.
			at := call.OccurredAt.UTC()
			if at.IsZero() || (!at.Before(input.Before.ProbeStartedAt.UTC()) &&
				!at.After(input.After.ProbeCompletedAt.UTC())) {
				if selected {
					usage.unsettled = true
				} else if !accountMismatch {
					usage.otherLocal = true
				}
			}
			continue
		}

		if !selected {
			usage.otherLocal = usage.otherLocal || !accountMismatch
			continue
		}
		usage.accountChanged = usage.accountChanged || accountMismatch
		if call.Pool == "" && ambiguousPool {
			usage.poolUnresolved = true
			continue
		}
		if accountMismatch {
			continue
		}
		switch call.Eligibility {
		case EligibilityIncluded:
		case EligibilityUnknown, "":
			usage.scopeAssumed = true
		default:
			usage.mixed = true
			continue
		}
		usage.add(call)
	}
	return usage
}

func (usage *valuationUsage) add(call ValuationCall) {
	amount := decimalAmount(call.Amount)
	if !call.PriceComplete || amount == nil || call.Currency == "" {
		usage.pricingIncomplete = true
	}
	if amount == nil || call.Currency == "" {
		return
	}
	total := usage.totals[call.Currency]
	if total == nil {
		total = new(big.Rat)
		usage.totals[call.Currency] = total
	}
	total.Add(total, amount)
}

func (usage valuationUsage) knownAmounts() []ValuationAmount {
	var amounts []ValuationAmount
	for currency, value := range usage.totals {
		amounts = append(amounts, ValuationAmount{Currency: currency, Amount: formatValuationMoney(value)})
	}
	sort.Slice(amounts, func(i, j int) bool { return amounts[i].Currency < amounts[j].Currency })
	return amounts
}

func decimalAmount(text string) *big.Rat {
	if strings.Trim(text, "0123456789.") != "" {
		return nil
	}
	value, ok := new(big.Rat).SetString(text)
	if !ok {
		return nil
	}
	return value
}

func percentRat(percent float64) *big.Rat {
	value, _ := new(big.Rat).SetString(strconv.FormatFloat(percent, 'f', -1, 64))
	return value
}

// Use the existing usage amount convention for decimal monetary strings.
func formatValuationMoney(value *big.Rat) string {
	return value.FloatString(9)
}

func estimateWindow(pair valuationPair, usage valuationUsage, delta *big.Rat) (*WindowEstimate, ValuationReason) {
	if len(usage.totals) != 1 {
		return nil, ValuationNoReliableDelta
	}
	var currency string
	var amount *big.Rat
	for currency, amount = range usage.totals {
	}
	before, after := pair.before, pair.after
	if amount.Sign() == 0 || delta.Sign() <= 0 ||
		!validPrecision(before.PrecisionPP) || !validPrecision(after.PrecisionPP) {
		return nil, ValuationNoReliableDelta
	}

	estimate := &WindowEstimate{Currency: currency, BasisAmount: formatValuationMoney(amount)}
	if before.PrecisionPP != nil && after.PrecisionPP != nil {
		beforeLow, beforeHigh := quantizationBounds(before)
		afterLow, afterHigh := quantizationBounds(after)
		deltaLow := afterLow.Sub(afterLow, beforeHigh)
		if deltaLow.Sign() <= 0 {
			return nil, ValuationNoReliableDelta
		}
		deltaHigh := afterHigh.Sub(afterHigh, beforeLow)
		estimate.Lower = wholeWindowAmount(amount, deltaHigh)
		estimate.Upper = wholeWindowAmount(amount, deltaLow)
	} else {
		estimate.Amount = wholeWindowAmount(amount, delta)
	}
	if before.UsedPercent == 100 || after.UsedPercent == 100 {
		return nil, ValuationSaturated
	}
	return estimate, ""
}

func validPrecision(precision *float64) bool {
	return precision == nil || (*precision > 0 && !math.IsInf(*precision, 0))
}

func quantizationBounds(window *SampleWindow) (low, high *big.Rat) {
	center := percentRat(window.UsedPercent)
	half := new(big.Rat).Quo(percentRat(*window.PrecisionPP), big.NewRat(2, 1))
	low = new(big.Rat).Sub(center, half)
	high = new(big.Rat).Add(center, half)
	if low.Sign() < 0 {
		low.SetInt64(0)
	}
	if cap := big.NewRat(100, 1); high.Cmp(cap) > 0 {
		high.Set(cap)
	}
	return low, high
}

func wholeWindowAmount(amount, delta *big.Rat) string {
	numerator := new(big.Rat).Mul(amount, big.NewRat(100, 1))
	return formatValuationMoney(numerator.Quo(numerator, delta))
}
