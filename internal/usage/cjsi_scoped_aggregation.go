package usage

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"sort"
	"strings"
	"time"

	ledger "github.com/kitdine/agent-deck/internal/usage/cjsiledger"
)

const intervalDetails = ledger.CachedKnown | ledger.WriteKnown | ledger.ReasoningKnown

// Interval-only contribution to a single slice. Counts holds observed subtotals;
// Known says which details were observed at least once, missing which were
// absent on at least one allocated member. Neither mask describes requests.
type scopedIntervalSum struct {
	counts            ledger.Counts
	missing           ledger.DetailMask
	sessions          map[SessionKey]struct{} // union with request membership, never add cardinalities
	allocated         int                     // number of intervals allocated to this slice
	shareBase         int64                   // unfiltered eligible interval tokens in [From,To) for client/session
	partial           bool                    // an intersecting non-excluded interval could not be allocated to this slice
	unfilteredPartial bool                    // an intersecting interval crossed [From,To), leaving unfiltered total incomplete
}

// validIntervalCounts verifies non-negativity, primary-sum overflow safety,
// subset bounds (cached <= input, write <= input, reasoning <= output), and
// that unknown details carry zero values.
func validIntervalCounts(c ledger.Counts) bool {
	return c.Input >= 0 && c.Output >= 0 && c.Input <= math.MaxInt64-c.Output &&
		c.Known&^intervalDetails == 0 &&
		c.Cached >= 0 && c.Cached <= c.Input && (c.Known&ledger.CachedKnown != 0 || c.Cached == 0) &&
		c.Write >= 0 && c.Write <= c.Input && (c.Known&ledger.WriteKnown != 0 || c.Write == 0) &&
		c.Reasoning >= 0 && c.Reasoning <= c.Output && (c.Known&ledger.ReasoningKnown != 0 || c.Reasoning == 0)
}

// detailFitsRange checks exact integer feasibility of a subset detail across
// the cumulative interval lo -> hi and its split into residual n and claimed
// remainder (span - n). All comparisons use subtraction to avoid int64 overflow.
func detailFitsRange(nKnown bool, nVal, nParent int64, loKnown bool, loVal, loParent int64, hiKnown bool, hiVal, hiParent int64) bool {
	spanP := hiParent - loParent
	if spanP < 0 || nParent < 0 || nParent > spanP {
		return false
	}
	remP := spanP - nParent

	loMin, loMax := int64(0), loParent
	if loKnown {
		loMin, loMax = loVal, loVal
	}
	hiMin, hiMax := int64(0), hiParent
	if hiKnown {
		hiMin, hiMax = hiVal, hiVal
	}

	var spanMin int64
	if hiMin > loMax {
		spanMin = hiMin - loMax
	}
	if hiMax < loMin {
		return false
	}
	spanMax := spanP
	if diff := hiMax - loMin; diff < spanMax {
		spanMax = diff
	}
	if spanMin > spanMax {
		return false
	}

	nMin, nMax := int64(0), nParent
	if nKnown {
		nMin, nMax = nVal, nVal
	}
	if nMin > spanMax {
		return false
	}
	if spanMin > nMax && spanMin-nMax > remP {
		return false
	}
	return true
}

// residualFitsRange is the single shared feasibility validator used by both
// the selected SQLite interval reader and the interval impact planner.
func residualFitsRange(n ledger.Counts, r ledger.CounterRange) bool {
	lo, hi := r.Lower, r.Upper
	if hi.Input < lo.Input || hi.Output < lo.Output {
		return false
	}
	return n.Input <= hi.Input-lo.Input && n.Output <= hi.Output-lo.Output &&
		detailFitsRange(n.Known&ledger.CachedKnown != 0, n.Cached, n.Input,
			lo.Known&ledger.CachedKnown != 0, lo.Cached, lo.Input,
			hi.Known&ledger.CachedKnown != 0, hi.Cached, hi.Input) &&
		detailFitsRange(n.Known&ledger.WriteKnown != 0, n.Write, n.Input,
			lo.Known&ledger.WriteKnown != 0, lo.Write, lo.Input,
			hi.Known&ledger.WriteKnown != 0, hi.Write, hi.Input) &&
		detailFitsRange(n.Known&ledger.ReasoningKnown != 0, n.Reasoning, n.Output,
			lo.Known&ledger.ReasoningKnown != 0, lo.Reasoning, lo.Output,
			hi.Known&ledger.ReasoningKnown != 0, hi.Reasoning, hi.Output)
}

// validateUniqueIntervals validates and deduplicates TokenInterval members in
// source-order-independent form. Non-interval members (Response, LegacyCall)
// are ignored; unknown kinds or conflicting duplicate keys fail closed.
func validateUniqueIntervals(members []ledger.Member, sessionID string) ([]ledger.Member, error) {
	seen := make(map[ledger.Key]int)
	unique := make([]ledger.Member, 0, len(members))
	for i := range members {
		m := &members[i]
		switch m.Key.Kind {
		case ledger.Response, ledger.LegacyCall:
			continue
		case ledger.TokenInterval:
		default:
			return nil, errors.New("unknown usage member kind")
		}
		if m.Key.Thread == "" || m.Key.ID == "" {
			return nil, errors.New("invalid token interval key")
		}
		r, n := m.Interval, m.Tokens // residual, not the raw cumulative endpoints
		if r.LowerAt.IsZero() || r.UpperAt.IsZero() || r.UpperAt.Before(r.LowerAt) || !validIntervalCounts(n) {
			return nil, errors.New("invalid interval bounds or residual counts")
		}
		if r.Epoch != ([32]byte{}) || r.Lower != (ledger.Counts{}) || r.Upper != (ledger.Counts{}) {
			if !validIntervalCounts(r.Lower) || !validIntervalCounts(r.Upper) || !residualFitsRange(n, r) {
				return nil, errors.New("inconsistent interval cumulative bounds and residual counts")
			}
		}
		if j, ok := seen[m.Key]; ok {
			prev := &members[j]
			p := prev.Interval
			if prev.Tokens != n || prev.Model != m.Model || p.Epoch != r.Epoch || p.Lower != r.Lower || p.Upper != r.Upper ||
				!p.LowerAt.Equal(r.LowerAt) || !p.UpperAt.Equal(r.UpperAt) {
				return nil, errors.New("conflicting token interval images")
			}
			continue
		}
		seen[m.Key] = i
		if sessionID != "" && sessionID != m.Key.Thread {
			continue
		}
		unique = append(unique, *m)
	}
	return unique, nil
}

// reduceScopedIntervals operates on a bounded, already selected member set.
// Validate the window first, then short-circuit clients disjoint from Codex
// before requiring the Codex member slice to be available. A successfully read
// empty set is an empty non-nil slice; nil means unavailable.
func reduceScopedIntervals(members []ledger.Member, scope StatsOptions, sessionID string) (scopedIntervalSum, error) {
	if !scope.From.Before(scope.To) {
		return scopedIntervalSum{}, errors.New("interval aggregation requires a nonempty window")
	}
	var sum scopedIntervalSum
	if scope.Client != "" && scope.Client != "codex" {
		return sum, nil
	}
	if members == nil {
		return scopedIntervalSum{}, errors.New("interval aggregation requires available members")
	}
	unique, err := validateUniqueIntervals(members, sessionID)
	if err != nil {
		return scopedIntervalSum{}, err
	}
	for i := range unique {
		m := &unique[i]
		r, n := m.Interval, m.Tokens
		contained, intersects := r.InWindow(scope.From, scope.To)
		if !intersects {
			continue
		}
		// A whole-interval proved local model different from scope.Model proves
		// this interval is disjoint from the filtered model slice.
		modelExcluded := scope.Model != "" && m.Model != "" && m.Model != scope.Model
		if !contained {
			sum.unfilteredPartial = true
			if !modelExcluded {
				sum.partial = true
			}
			continue
		}
		total := n.Input + n.Output
		if total > math.MaxInt64-sum.shareBase {
			return scopedIntervalSum{}, errors.New("interval token total overflow")
		}
		sum.shareBase += total

		// Task 3 emits no whole-interval independent runtime provider witness.
		// Never allocate an interval to any provider filter (including "unknown").
		if scope.Provider != "" {
			if !modelExcluded {
				sum.partial = true
			}
			continue
		}
		if scope.Model != "" {
			if modelExcluded {
				continue
			}
			// Only an interval proved to have the same local model across the
			// entire span enters that model slice (A4b).
			if m.Model == "" {
				sum.partial = true
				continue
			}
		}
		if n.Write > math.MaxInt64-sum.counts.Write {
			return scopedIntervalSum{}, errors.New("interval cache write total overflow")
		}
		// The shareBase check bounds the primary sums and their subsets.
		sum.counts.Input += n.Input
		sum.counts.Output += n.Output
		sum.counts.Cached += n.Cached
		sum.counts.Write += n.Write
		sum.counts.Reasoning += n.Reasoning
		sum.counts.Known |= n.Known
		sum.missing |= intervalDetails &^ n.Known
		sum.allocated++
		if sum.sessions == nil {
			sum.sessions = make(map[SessionKey]struct{})
		}
		sum.sessions[SessionKey{Client: "codex", SessionID: m.Key.Thread}] = struct{}{}
	}
	return sum, nil
}

// jointUsageSummary holds the merged request and token-interval view for one
// scoped slice. Invocation counts, attribution reasons, and known costs remain
// strictly request-only; token totals and distinct sessions include allocated
// intervals; shareBase keeps the unfiltered eligible token denominator.
type jointUsageSummary struct {
	summary         Summary
	totals          PresentationTotals
	shareBase       int64
	intervalCounts  ledger.Counts
	intervalMissing ledger.DetailMask
	partial         bool
}

func (j jointUsageSummary) knownTokenShare() string {
	if j.shareBase <= 0 {
		return "0.00"
	}
	return new(big.Rat).Mul(big.NewRat(j.totals.Tokens, j.shareBase), big.NewRat(100, 1)).FloatString(2)
}

func addInt64Checked(a, b int64) (int64, bool) {
	if b < 0 || a < 0 || b > math.MaxInt64-a {
		return 0, false
	}
	return a + b, true
}

func eventTokenTotalChecked(client string, tokens map[string]int64) (int64, bool) {
	in := tokens["input_tokens"]
	out := tokens["output_tokens"]
	total, ok := addInt64Checked(in, out)
	if !ok {
		return 0, false
	}
	if client == "codex" {
		return total, true
	}
	cacheWrite, ok := addInt64Checked(tokens["cache_write_5m_tokens"], tokens["cache_write_1h_tokens"])
	if !ok {
		return 0, false
	}
	if cacheWrite == 0 {
		cacheWrite = tokens["cache_creation_tokens"]
		if cacheWrite < 0 {
			return 0, false
		}
	}
	if total, ok = addInt64Checked(total, tokens["cache_read_tokens"]); !ok {
		return 0, false
	}
	return addInt64Checked(total, cacheWrite)
}

func addEventToStatsAccumulatorChecked(a *statsAccumulator, event storedEvent, result Result, base, providerCost *big.Rat) error {
	reqTokens, ok := eventTokenTotalChecked(event.Client, event.Tokens)
	if !ok {
		return errors.New("request token total overflow")
	}
	if _, ok = addInt64Checked(a.tokens, reqTokens); !ok {
		return errors.New("request token sum overflow")
	}
	if _, ok = addInt64Checked(a.input, event.Tokens["input_tokens"]); !ok {
		return errors.New("request input sum overflow")
	}
	if _, ok = addInt64Checked(a.output, event.Tokens["output_tokens"]); !ok {
		return errors.New("request output sum overflow")
	}
	if event.Client == "codex" {
		if _, ok = addInt64Checked(a.cachedRead, event.Tokens["cached_input_tokens"]); !ok {
			return errors.New("request cached input sum overflow")
		}
		if _, ok = addInt64Checked(a.cacheWrite, event.Tokens["cache_write_tokens"]); !ok {
			return errors.New("request cache write sum overflow")
		}
	} else {
		if _, ok = addInt64Checked(a.cachedRead, event.Tokens["cache_read_tokens"]); !ok {
			return errors.New("request cache read sum overflow")
		}
		cacheWrite, ok := addInt64Checked(event.Tokens["cache_write_5m_tokens"], event.Tokens["cache_write_1h_tokens"])
		if !ok {
			return errors.New("request cache write sum overflow")
		}
		if cacheWrite == 0 {
			cacheWrite = event.Tokens["cache_creation_tokens"]
			if cacheWrite < 0 {
				return errors.New("negative request cache creation tokens")
			}
		}
		if _, ok = addInt64Checked(a.cacheWrite, cacheWrite); !ok {
			return errors.New("request cache write sum overflow")
		}
	}
	return a.addWithParsedCosts(event, result, base, providerCost)
}

func addIntervalToStatsAccumulator(acc *statsAccumulator, n ledger.Counts, thread string) error {
	total, ok := addInt64Checked(n.Input, n.Output)
	if !ok {
		return errors.New("interval token total overflow")
	}
	if acc.tokens, ok = addInt64Checked(acc.tokens, total); !ok {
		return errors.New("interval token sum overflow")
	}
	if acc.input, ok = addInt64Checked(acc.input, n.Input); !ok {
		return errors.New("interval input sum overflow")
	}
	if acc.output, ok = addInt64Checked(acc.output, n.Output); !ok {
		return errors.New("interval output sum overflow")
	}
	if n.Known&ledger.CachedKnown != 0 {
		if acc.cachedRead, ok = addInt64Checked(acc.cachedRead, n.Cached); !ok {
			return errors.New("interval cached read sum overflow")
		}
	}
	if n.Known&ledger.WriteKnown != 0 {
		if acc.cacheWrite, ok = addInt64Checked(acc.cacheWrite, n.Write); !ok {
			return errors.New("interval cache write sum overflow")
		}
	}
	acc.sessions["codex\x00"+thread] = struct{}{}
	acc.catalogComplete = false
	acc.providerComplete = false
	return nil
}

// summarizeJointContributions combines already-admitted requests (for the
// unfiltered client/time/session scope) with reduced intervals. Request order
// is preserved so warnings accumulate in first-observed legacy order.
func summarizeJointContributions(events []storedEvent, intervals scopedIntervalSum, scope StatsOptions, sessionID string, price eventPricing) (jointUsageSummary, error) {
	if !scope.From.Before(scope.To) {
		return jointUsageSummary{}, errors.New("joint summary requires a nonempty window")
	}
	if events == nil {
		return jointUsageSummary{}, errors.New("admitted request set unavailable")
	}
	if len(events) > 0 && price == nil {
		return jointUsageSummary{}, errors.New("admitted request pricing unavailable")
	}
	if !validIntervalCounts(intervals.counts) || intervals.shareBase < 0 || intervals.allocated < 0 {
		return jointUsageSummary{}, errors.New("invalid scoped interval sum")
	}
	if scope.Client != "" && scope.Client != "codex" && (intervals.allocated != 0 || intervals.shareBase != 0 || intervals.partial || len(intervals.sessions) != 0) {
		return jointUsageSummary{}, errors.New("codex intervals supplied to disjoint client")
	}

	reasons := make(map[string]int64, len(attributionReasons()))
	for _, reason := range attributionReasons() {
		reasons[reason] = 0
	}
	out := Summary{
		Tokens:             map[string]int64{},
		Counts:             map[string]int64{"events": 0, "exact": 0, "estimated": 0, "unattributed": 0, "priced": 0, "unpriced": 0},
		AttributionReasons: reasons,
		Models:             []ModelCoverage{},
		Unpriced:           []string{},
		Warnings:           []string{},
	}
	base := new(big.Rat)
	provider := new(big.Rat)
	unattributedBase := new(big.Rat)
	complete, providerComplete, unattributedComplete := true, true, true
	warned := map[string]bool{}
	unpriced := map[string]bool{}
	coverage := map[string]*ModelCoverage{}
	acc := newStatsAccumulator()

	var shareBase int64
	seenKeys := make(map[string]struct{}, len(events))
	for _, e := range events {
		if e.Key == "" {
			return jointUsageSummary{}, errors.New("invalid admitted request key")
		}
		if _, dup := seenKeys[e.Key]; dup {
			return jointUsageSummary{}, errors.New("duplicate admitted request key")
		}
		seenKeys[e.Key] = struct{}{}
		if scope.Client != "" && e.Client != scope.Client {
			return jointUsageSummary{}, errors.New("admitted request outside client scope")
		}
		if sessionID != "" && e.SessionID != sessionID {
			return jointUsageSummary{}, errors.New("admitted request outside session scope")
		}
		at, err := time.Parse(time.RFC3339Nano, e.EventAt)
		if err != nil {
			return jointUsageSummary{}, errors.New("invalid admitted request timestamp")
		}
		if at.Before(scope.From) || !at.Before(scope.To) {
			return jointUsageSummary{}, errors.New("admitted request outside time window")
		}
		reqTokens, ok := eventTokenTotalChecked(e.Client, e.Tokens)
		if !ok {
			return jointUsageSummary{}, errors.New("admitted request token total overflow")
		}
		if shareBase, ok = addInt64Checked(shareBase, reqTokens); !ok {
			return jointUsageSummary{}, errors.New("share base token overflow")
		}
		if scope.Model != "" && e.Model != scope.Model {
			continue
		}
		attribution, err := price(e)
		if err != nil {
			return jointUsageSummary{}, err
		}
		if scope.Provider != "" && attribution.provider != scope.Provider {
			continue
		}
		r, err := calculateAttributedEvent(e, attribution)
		if err != nil {
			return jointUsageSummary{}, err
		}
		r = aggregateAttributedResult(r, attribution)

		out.Counts["events"]++
		out.Counts[attribution.quality]++
		out.AttributionReasons[attribution.reason]++
		if attribution.quality != "exact" && !warned[attribution.quality] {
			out.Warnings = append(out.Warnings, attribution.quality+" attribution")
			warned[attribution.quality] = true
		}
		for _, warning := range r.Warnings {
			if !warned[warning] {
				out.Warnings = append(out.Warnings, warning)
				warned[warning] = true
			}
		}
		for k, v := range e.Tokens {
			if v < 0 {
				return jointUsageSummary{}, errors.New("negative admitted request token component")
			}
			if out.Tokens[k], ok = addInt64Checked(out.Tokens[k], v); !ok {
				return jointUsageSummary{}, errors.New("request token component overflow")
			}
		}
		coverageKey := e.Client + "\x00" + e.Model
		model := coverage[coverageKey]
		if model == nil {
			model = &ModelCoverage{Client: e.Client, Model: e.Model}
			coverage[coverageKey] = model
		}
		model.Events++

		knownBase, err := decimal(r.KnownCatalogBaseCost)
		if err != nil {
			return jointUsageSummary{}, err
		}
		knownProvider, err := decimal(r.KnownProviderCost)
		if err != nil {
			return jointUsageSummary{}, err
		}
		base.Add(base, knownBase)
		provider.Add(provider, knownProvider)
		if r.ProviderCost == nil {
			providerComplete = false
		}
		if attribution.reason == attributionReasonBeforeAdoption || attribution.reason == attributionReasonCoverageGap {
			if r.CatalogBaseCost == nil {
				unattributedComplete = false
			} else {
				unattributedBase.Add(unattributedBase, knownBase)
			}
		}
		if r.CatalogBaseCost == nil {
			complete = false
			out.Counts["unpriced"]++
			model.UnpricedEvents++
			for _, u := range r.Unpriced {
				unpriced[u] = true
			}
		} else {
			out.Counts["priced"]++
			model.PricedEvents++
		}
		if err := addEventToStatsAccumulatorChecked(acc, e, r, knownBase, knownProvider); err != nil {
			return jointUsageSummary{}, err
		}
	}

	var ok bool
	if shareBase, ok = addInt64Checked(shareBase, intervals.shareBase); !ok {
		return jointUsageSummary{}, errors.New("joint share base token overflow")
	}

	if intervals.allocated > 0 {
		ic := intervals.counts
		total, ok := addInt64Checked(ic.Input, ic.Output)
		if !ok {
			return jointUsageSummary{}, errors.New("interval token total overflow")
		}
		if out.Tokens["input_tokens"], ok = addInt64Checked(out.Tokens["input_tokens"], ic.Input); !ok {
			return jointUsageSummary{}, errors.New("joint input token overflow")
		}
		if out.Tokens["output_tokens"], ok = addInt64Checked(out.Tokens["output_tokens"], ic.Output); !ok {
			return jointUsageSummary{}, errors.New("joint output token overflow")
		}
		if ic.Known&ledger.CachedKnown != 0 {
			if out.Tokens["cached_input_tokens"], ok = addInt64Checked(out.Tokens["cached_input_tokens"], ic.Cached); !ok {
				return jointUsageSummary{}, errors.New("joint cached input token overflow")
			}
		}
		if ic.Known&ledger.WriteKnown != 0 {
			if out.Tokens["cache_write_tokens"], ok = addInt64Checked(out.Tokens["cache_write_tokens"], ic.Write); !ok {
				return jointUsageSummary{}, errors.New("joint cache write token overflow")
			}
		}
		if acc.tokens, ok = addInt64Checked(acc.tokens, total); !ok {
			return jointUsageSummary{}, errors.New("joint presentation token overflow")
		}
		if acc.input, ok = addInt64Checked(acc.input, ic.Input); !ok {
			return jointUsageSummary{}, errors.New("joint presentation input overflow")
		}
		if acc.output, ok = addInt64Checked(acc.output, ic.Output); !ok {
			return jointUsageSummary{}, errors.New("joint presentation output overflow")
		}
		if ic.Known&ledger.CachedKnown != 0 {
			if acc.cachedRead, ok = addInt64Checked(acc.cachedRead, ic.Cached); !ok {
				return jointUsageSummary{}, errors.New("joint presentation cached overflow")
			}
		}
		if ic.Known&ledger.WriteKnown != 0 {
			if acc.cacheWrite, ok = addInt64Checked(acc.cacheWrite, ic.Write); !ok {
				return jointUsageSummary{}, errors.New("joint presentation write overflow")
			}
		}
		for sk := range intervals.sessions {
			acc.sessions[sk.Client+"\x00"+sk.SessionID] = struct{}{}
		}
	}

	if intervals.allocated > 0 || intervals.partial {
		complete = false
		providerComplete = false
		acc.catalogComplete = false
		acc.providerComplete = false
	}

	knownBase, knownProvider := money(base), money(provider)
	out.KnownCatalogBaseCost = &knownBase
	out.KnownProviderCost = &knownProvider
	if complete {
		c := knownBase
		out.CatalogBaseCost = &c
	}
	if providerComplete {
		p := knownProvider
		out.ProviderCost = &p
	}
	if unattributedComplete {
		value := money(unattributedBase)
		out.UnattributedCatalogBaseCost = &value
	}
	for u := range unpriced {
		out.Unpriced = append(out.Unpriced, u)
	}
	sort.Strings(out.Unpriced)
	for _, model := range coverage {
		out.Models = append(out.Models, *model)
	}
	sort.Slice(out.Models, func(i, j int) bool {
		if out.Models[i].Client == out.Models[j].Client {
			return out.Models[i].Model < out.Models[j].Model
		}
		return out.Models[i].Client < out.Models[j].Client
	})

	totals := presentationTotals(acc)
	return jointUsageSummary{
		summary:         out,
		totals:          totals,
		shareBase:       shareBase,
		intervalCounts:  intervals.counts,
		intervalMissing: intervals.missing,
		partial:         intervals.partial,
	}, nil
}

// scopedToolCall carries one same-view tool-call observation for managed Stats
// model activity, supplied from the same pinned read transaction as requests.
type scopedToolCall struct {
	Client     string
	SessionID  string
	Model      string
	ToolName   string
	StartedAt  string
	Status     string
	DurationMS *int64
	Provider   string
}

// reduceScopedStats produces a StatsReport combining already-admitted requests
// (for the unfiltered Client/From/To window, before Model/Provider filtering)
// and selected token-only intervals. It preserves A4b semantics across Totals,
// Buckets, Models, Clients, Providers, Activity, Peak, and Coverage.
func reduceScopedStatsWithToolCalls(events []storedEvent, toolCalls []scopedToolCall, members []ledger.Member, options StatsOptions, price eventPricing) (StatsReport, bool, error) {
	location := options.Location
	if location == nil {
		location = time.Local
	}
	if !options.From.Before(options.To) {
		return StatsReport{}, false, errors.New("usage stats range must have from before to")
	}
	if options.GroupBy != "hour" && options.GroupBy != "day" && options.GroupBy != "week" && options.GroupBy != "month" {
		return StatsReport{}, false, errors.New("invalid usage stats group-by")
	}
	if options.Metric != "tokens" && options.Metric != "cost" && options.Metric != "sessions" {
		return StatsReport{}, false, errors.New("invalid usage stats metric")
	}
	if events == nil || toolCalls == nil {
		return StatsReport{}, false, errors.New("admitted request or tool call set unavailable")
	}
	if len(events) > 0 && price == nil {
		return StatsReport{}, false, errors.New("admitted request pricing unavailable")
	}

	reduced, err := reduceScopedIntervals(members, options, "")
	if err != nil {
		return StatsReport{}, false, err
	}
	var uniqueIntervals []ledger.Member
	if options.Client == "" || options.Client == "codex" {
		uniqueIntervals, err = validateUniqueIntervals(members, "")
		if err != nil {
			return StatsReport{}, false, err
		}
	}

	total := newStatsAccumulator()
	warnings := map[string]bool{}
	buckets := map[string]*statsAccumulator{}
	models := map[string]*statsAccumulator{}
	modelActivity := map[string]*statsModelActivityAccumulator{}
	clients := map[string]*statsAccumulator{}
	providers := map[string]*statsAccumulator{}
	sessions := map[string]*statsSessionAccumulator{}
	activity := map[[2]int]*statsAccumulator{}
	cacheReadIncomplete := map[*statsAccumulator]bool{}

	var unfilteredTokens int64
	seenKeys := make(map[string]struct{}, len(events))
	for _, event := range events {
		if event.Key == "" {
			return StatsReport{}, false, errors.New("invalid admitted request key")
		}
		if _, dup := seenKeys[event.Key]; dup {
			return StatsReport{}, false, errors.New("duplicate admitted request key")
		}
		seenKeys[event.Key] = struct{}{}
		if options.Client != "" && event.Client != options.Client {
			return StatsReport{}, false, errors.New("admitted request outside client scope")
		}
		at, parseErr := time.Parse(time.RFC3339Nano, event.EventAt)
		if parseErr != nil {
			return StatsReport{}, false, parseErr
		}
		if at.Before(options.From) || !at.Before(options.To) {
			return StatsReport{}, false, errors.New("admitted request outside time window")
		}
		reqTokens, ok := eventTokenTotalChecked(event.Client, event.Tokens)
		if !ok {
			return StatsReport{}, false, errors.New("stats request token overflow")
		}
		if unfilteredTokens, ok = addInt64Checked(unfilteredTokens, reqTokens); !ok {
			return StatsReport{}, false, errors.New("stats unfiltered token overflow")
		}
		if options.Model != "" && event.Model != options.Model {
			continue
		}
		attribution, priceErr := price(event)
		if priceErr != nil {
			return StatsReport{}, false, priceErr
		}
		runtimeProvider := attribution.provider
		if options.Provider != "" && runtimeProvider != options.Provider {
			continue
		}
		result, calculateErr := calculateAttributedEvent(event, attribution)
		if calculateErr != nil {
			return StatsReport{}, false, calculateErr
		}
		result = aggregateAttributedResult(result, attribution)
		for _, warning := range result.Warnings {
			warnings[warning] = true
		}
		start := bucketStart(at, options.GroupBy, location)
		key := start.Format(time.RFC3339Nano)
		if buckets[key] == nil {
			buckets[key] = newStatsAccumulator()
		}
		modelKey := event.Client + "\x00" + event.Model
		if models[modelKey] == nil {
			models[modelKey] = newStatsAccumulator()
		}
		if modelActivity[modelKey] == nil {
			modelActivity[modelKey] = newStatsModelActivityAccumulator()
		}
		if clients[event.Client] == nil {
			clients[event.Client] = newStatsAccumulator()
		}
		providerKey := event.Client + "\x00" + runtimeProvider
		if providers[providerKey] == nil {
			providers[providerKey] = newStatsAccumulator()
		}
		if attribution.viaWrapper {
			providers[providerKey].wrapperEvents++
		}
		sessionKey := event.Client + "\x00" + event.SessionID
		if sessions[sessionKey] == nil {
			sessions[sessionKey] = &statsSessionAccumulator{statsAccumulator: newStatsAccumulator(), models: map[string]struct{}{}}
		}
		local := at.In(location)
		activityKey := [2]int{(int(local.Weekday()) + 6) % 7, local.Hour()}
		if activity[activityKey] == nil {
			activity[activityKey] = newStatsAccumulator()
		}
		for _, accumulator := range []*statsAccumulator{total, buckets[key], models[modelKey], clients[event.Client], providers[providerKey], activity[activityKey]} {
			if err = addEventToStatsAccumulatorChecked(accumulator, event, result, nil, nil); err != nil {
				return StatsReport{}, false, err
			}
		}
		if err = addEventToStatsAccumulatorChecked(sessions[sessionKey].statsAccumulator, event, result, nil, nil); err != nil {
			return StatsReport{}, false, err
		}
		sessions[sessionKey].models[event.Model] = struct{}{}
		if sessions[sessionKey].firstAt == "" || event.EventAt < sessions[sessionKey].firstAt {
			sessions[sessionKey].firstAt = event.EventAt
		}
		if event.EventAt > sessions[sessionKey].lastAt {
			sessions[sessionKey].lastAt = event.EventAt
		}
		if err = modelActivity[modelKey].observe(event.Client, event.SessionID, event.EventAt, location); err != nil {
			return StatsReport{}, false, err
		}
	}

	// Observe same-view tool calls into modelActivity only (intervals never
	// contribute to tool calls or model activity).
	for _, tc := range toolCalls {
		if tc.Model == "" {
			continue
		}
		if options.Client != "" && tc.Client != options.Client {
			return StatsReport{}, false, errors.New("admitted tool call outside client scope")
		}
		startedAt, parseErr := time.Parse(time.RFC3339Nano, tc.StartedAt)
		if parseErr != nil {
			return StatsReport{}, false, parseErr
		}
		if startedAt.Before(options.From) || !startedAt.Before(options.To) {
			return StatsReport{}, false, errors.New("admitted tool call outside time window")
		}
		if options.Model != "" && tc.Model != options.Model {
			continue
		}
		if options.Provider != "" && tc.Provider != options.Provider {
			continue
		}
		modelKey := tc.Client + "\x00" + tc.Model
		if models[modelKey] == nil {
			models[modelKey] = newStatsAccumulator()
		}
		if modelActivity[modelKey] == nil {
			modelActivity[modelKey] = newStatsModelActivityAccumulator()
		}
		value := modelActivity[modelKey]
		if err = value.observe(tc.Client, tc.SessionID, tc.StartedAt, location); err != nil {
			return StatsReport{}, false, err
		}
		value.calls++
		value.tools[tc.ToolName]++
		switch tc.Status {
		case "completed":
			value.completed++
		case "failed":
			value.failed++
		}
		if tc.DurationMS != nil {
			nextDuration, ok := addInt64Checked(value.duration, *tc.DurationMS)
			if !ok {
				return StatsReport{}, false, errors.New("stats tool duration overflow")
			}
			value.timed++
			value.duration = nextDuration
		}
	}

	var ok bool
	if unfilteredTokens, ok = addInt64Checked(unfilteredTokens, reduced.shareBase); !ok {
		return StatsReport{}, false, errors.New("stats unfiltered token overflow")
	}

	// Apply allocated intervals to Totals, Clients, Models (only when proved
	// across the whole interval), and mark affected dimensions' cost incomplete.
	hasUnprovedModelInterval := false
	hasAnyCodexProviderInterval := false
	for i := range uniqueIntervals {
		m := &uniqueIntervals[i]
		contained, intersects := m.Interval.InWindow(options.From, options.To)
		if !intersects {
			continue
		}
		modelExcluded := options.Model != "" && m.Model != "" && m.Model != options.Model
		if !modelExcluded {
			hasAnyCodexProviderInterval = true
			if m.Model == "" {
				hasUnprovedModelInterval = true
			}
		}
		if !contained {
			if !modelExcluded && m.Model != "" {
				if acc := models["codex\x00"+m.Model]; acc != nil {
					acc.catalogComplete = false
					acc.providerComplete = false
				}
			}
			continue
		}
		if options.Provider != "" {
			continue
		}
		if options.Model != "" && (modelExcluded || m.Model == "") {
			continue
		}
		if err := addIntervalToStatsAccumulator(total, m.Tokens, m.Key.Thread); err != nil {
			return StatsReport{}, false, err
		}
		if m.Tokens.Known&ledger.CachedKnown == 0 {
			cacheReadIncomplete[total] = true
		}
		if clients["codex"] == nil {
			clients["codex"] = newStatsAccumulator()
		}
		if err := addIntervalToStatsAccumulator(clients["codex"], m.Tokens, m.Key.Thread); err != nil {
			return StatsReport{}, false, err
		}
		if m.Tokens.Known&ledger.CachedKnown == 0 {
			cacheReadIncomplete[clients["codex"]] = true
		}
		if m.Model != "" {
			modelKey := "codex\x00" + m.Model
			if models[modelKey] == nil {
				models[modelKey] = newStatsAccumulator()
			}
			if err := addIntervalToStatsAccumulator(models[modelKey], m.Tokens, m.Key.Thread); err != nil {
				return StatsReport{}, false, err
			}
			if m.Tokens.Known&ledger.CachedKnown == 0 {
				cacheReadIncomplete[models[modelKey]] = true
			}
		}
	}
	if reduced.allocated > 0 || reduced.partial {
		total.catalogComplete = false
		total.providerComplete = false
		if acc := clients["codex"]; acc != nil {
			acc.catalogComplete = false
			acc.providerComplete = false
		}
	}
	if hasUnprovedModelInterval {
		for key, acc := range models {
			if strings.HasPrefix(key, "codex\x00") {
				acc.catalogComplete = false
				acc.providerComplete = false
			}
		}
	}
	if hasAnyCodexProviderInterval {
		for key, acc := range providers {
			if strings.HasPrefix(key, "codex\x00") {
				acc.catalogComplete = false
				acc.providerComplete = false
			}
		}
	}

	// Build buckets and apply intervals strictly by half-open containment inside
	// the bucket clipped to [options.From, options.To) (no proportional splitting
	// across buckets, and intervals outside the query window never enter a wider
	// week/month bucket or mark it cost-incomplete).
	timezone := options.Timezone
	if timezone == "" {
		timezone = location.String()
	}
	report := StatsReport{
		Range:    StatsRange{From: options.From.In(location).Format(time.RFC3339Nano), To: options.To.In(location).Format(time.RFC3339Nano)},
		Timezone: timezone, GroupBy: options.GroupBy, Metric: options.Metric,
		Warnings: []string{}, Buckets: []StatsBucket{}, Models: []StatsDimension{}, Clients: []StatsDimension{}, Providers: []StatsDimension{}, CacheSessions: []StatsCacheSession{}, Activity: []StatsActivity{}, UnpricedModels: []StatsUnpricedModel{}, ShowModelActivity: options.Activity,
		Coverage: StatsCoverage{PricedEvents: total.priced, UnpricedEvents: total.unpriced, TotalEvents: total.events, Percent: statsCoverage(total.priced, total.unpriced)},
	}
	completeProvider, _ := statsCost(total)
	knownBase := money(total.base)
	var completeBase *string
	if total.catalogComplete {
		value := knownBase
		completeBase = &value
	}
	zeroCost := "0.000000000"
	report.Totals = StatsTotals{Tokens: total.tokens, InputTokens: total.input, OutputTokens: total.output, CachedReadTokens: total.cachedRead, CacheWriteTokens: total.cacheWrite, Sessions: int64(len(total.sessions)), Events: total.events, CatalogBaseCost: completeBase, ProviderCost: completeProvider, KnownCatalogBaseCost: knownBase, KnownProviderCost: money(total.provider), AverageTokens: "0.00", AverageCost: &zeroCost, KnownAverageCost: zeroCost}
	if len(total.sessions) > 0 {
		report.Totals.AverageTokens = new(big.Rat).Quo(big.NewRat(total.tokens, 1), big.NewRat(int64(len(total.sessions)), 1)).FloatString(2)
		report.Totals.KnownAverageCost = money(new(big.Rat).Quo(total.provider, big.NewRat(int64(len(total.sessions)), 1)))
		if total.providerComplete {
			value := report.Totals.KnownAverageCost
			report.Totals.AverageCost = &value
		} else {
			report.Totals.AverageCost = nil
		}
	} else if !total.providerComplete {
		report.Totals.AverageCost = nil
	}

	// A bucket can only intersect intervals whose lower endpoint precedes
	// its end. Carry forward only intervals whose upper endpoint reaches
	// the next bucket, including intervals ending exactly at that boundary.
	sort.Slice(uniqueIntervals, func(i, j int) bool {
		return uniqueIntervals[i].Interval.LowerAt.Before(uniqueIntervals[j].Interval.LowerAt)
	})
	nextInterval := 0
	var activeIntervals []int
	start := bucketStart(options.From, options.GroupBy, location)
	peakValue := new(big.Rat)
	for count := 0; start.Before(options.To); count++ {
		if count >= 10000 {
			return StatsReport{}, false, errors.New("usage stats range produces too many buckets")
		}
		end := nextBucket(start, options.GroupBy)
		effStart := start
		if effStart.Before(options.From) {
			effStart = options.From
		}
		effEnd := end
		if options.To.Before(effEnd) {
			effEnd = options.To
		}
		key := start.Format(time.RFC3339Nano)
		value := buckets[key]
		if value == nil {
			value = newStatsAccumulator()
		}
		if effStart.Before(effEnd) {
			for nextInterval < len(uniqueIntervals) && uniqueIntervals[nextInterval].Interval.LowerAt.Before(effEnd) {
				activeIntervals = append(activeIntervals, nextInterval)
				nextInterval++
			}
			remaining := activeIntervals[:0]
			for _, i := range activeIntervals {
				m := &uniqueIntervals[i]
				contained, intersects := m.Interval.InWindow(effStart, effEnd)
				if !intersects {
					continue
				}
				if !m.Interval.UpperAt.Before(effEnd) {
					remaining = append(remaining, i)
				}
				modelExcluded := options.Model != "" && m.Model != "" && m.Model != options.Model
				if modelExcluded {
					continue
				}
				if !contained || options.Provider != "" || (options.Model != "" && m.Model == "") {
					value.catalogComplete = false
					value.providerComplete = false
					continue
				}
				if err := addIntervalToStatsAccumulator(value, m.Tokens, m.Key.Thread); err != nil {
					return StatsReport{}, false, err
				}
			}
			activeIntervals = remaining
		}
		cost, known := statsCost(value)
		metricComplete, metricKnown := statsMetricValues(options.Metric, value)
		bucket := StatsBucket{Start: start.Format(time.RFC3339Nano), End: end.Format(time.RFC3339Nano), Tokens: value.tokens, InputTokens: value.input, OutputTokens: value.output, CachedReadTokens: value.cachedRead, CacheWriteTokens: value.cacheWrite, Sessions: int64(len(value.sessions)), Events: value.events, ProviderCost: cost, KnownProviderCost: known, MetricValue: metricComplete, KnownMetricValue: metricKnown, Coverage: statsCoverage(value.priced, value.unpriced)}
		report.Buckets = append(report.Buckets, bucket)
		candidate := statsMetricRat(options.Metric, value)
		if len(report.Buckets) == 1 || candidate.Cmp(peakValue) > 0 {
			peakValue.Set(candidate)
			report.Peak = StatsPeak{Start: bucket.Start, End: bucket.End, Value: bucket.MetricValue, KnownValue: bucket.KnownMetricValue, Coverage: bucket.Coverage}
		}
		start = end
	}

	// Token shares use the unfiltered eligible token denominator (including
	// unallocated contained intervals), never renormalizing over allocated rows.
	totalMetric := statsMetricRat(options.Metric, total)
	if options.Metric == "tokens" {
		totalMetric = big.NewRat(unfilteredTokens, 1)
	}
	for key, value := range models {
		parts := strings.SplitN(key, "\x00", 2)
		dimension := statsDimension(parts[1], parts[0], options.Metric, value, totalMetric, total.providerComplete)
		if cacheReadIncomplete[value] {
			dimension.CacheHitRate = nil
		}
		if modelActivity[key] != nil {
			summary := modelActivity[key].summary()
			dimension.Activity = &summary
		}
		report.Models = append(report.Models, dimension)
		if len(value.missing) > 0 {
			missing := make([]string, 0, len(value.missing))
			for component := range value.missing {
				missing = append(missing, component)
			}
			sort.Strings(missing)
			report.UnpricedModels = append(report.UnpricedModels, StatsUnpricedModel{Client: parts[0], Model: parts[1], Components: missing})
		}
	}
	for name, value := range clients {
		dimension := statsDimension(name, "", options.Metric, value, totalMetric, total.providerComplete)
		if cacheReadIncomplete[value] {
			dimension.CacheHitRate = nil
		}
		report.Clients = append(report.Clients, dimension)
	}
	for key, value := range providers {
		parts := strings.SplitN(key, "\x00", 2)
		dimension := statsDimension(parts[1], parts[0], options.Metric, value, totalMetric, total.providerComplete)
		if cacheReadIncomplete[value] {
			dimension.CacheHitRate = nil
		}
		report.Providers = append(report.Providers, dimension)
	}
	for key, value := range sessions {
		if value.cachedRead == 0 && value.cacheWrite == 0 {
			continue
		}
		parts := strings.SplitN(key, "\x00", 2)
		modelNames := make([]string, 0, len(value.models))
		for model := range value.models {
			modelNames = append(modelNames, model)
		}
		sort.Strings(modelNames)
		logicalInput := value.input
		if parts[0] == "claude" {
			var ok bool
			if logicalInput, ok = addInt64Checked(logicalInput, value.cachedRead); !ok {
				return StatsReport{}, false, errors.New("stats cache session logical input overflow")
			}
			if logicalInput, ok = addInt64Checked(logicalInput, value.cacheWrite); !ok {
				return StatsReport{}, false, errors.New("stats cache session logical input overflow")
			}
		}
		report.CacheSessions = append(report.CacheSessions, StatsCacheSession{Client: parts[0], SessionID: parts[1], Models: modelNames, InputTokens: value.input, OutputTokens: value.output, CachedReadTokens: value.cachedRead, CacheWriteTokens: value.cacheWrite, LogicalInputTokens: logicalInput, CacheHitRate: percentPointer(value.cachedRead, logicalInput), Events: value.events, FirstAt: value.firstAt, LastAt: value.lastAt, DetailCommand: fmt.Sprintf("agentdeck session show %s --client %s --activity", parts[1], parts[0])})
	}
	sort.Slice(report.Models, func(i, j int) bool {
		left := statsKnownMetricRat(report.Models[i].KnownMetricValue)
		right := statsKnownMetricRat(report.Models[j].KnownMetricValue)
		if comparison := left.Cmp(right); comparison != 0 {
			return comparison > 0
		}
		if report.Models[i].Client == report.Models[j].Client {
			return report.Models[i].Name < report.Models[j].Name
		}
		return report.Models[i].Client < report.Models[j].Client
	})
	sort.Slice(report.Providers, func(i, j int) bool {
		left := statsKnownMetricRat(report.Providers[i].KnownMetricValue)
		right := statsKnownMetricRat(report.Providers[j].KnownMetricValue)
		if comparison := left.Cmp(right); comparison != 0 {
			return comparison > 0
		}
		if report.Providers[i].Client == report.Providers[j].Client {
			return report.Providers[i].Name < report.Providers[j].Name
		}
		return report.Providers[i].Client < report.Providers[j].Client
	})
	sort.Slice(report.Clients, func(i, j int) bool { return report.Clients[i].Name < report.Clients[j].Name })
	sort.Slice(report.CacheSessions, func(i, j int) bool {
		if report.CacheSessions[i].CachedReadTokens != report.CacheSessions[j].CachedReadTokens {
			return report.CacheSessions[i].CachedReadTokens > report.CacheSessions[j].CachedReadTokens
		}
		if report.CacheSessions[i].LogicalInputTokens != report.CacheSessions[j].LogicalInputTokens {
			return report.CacheSessions[i].LogicalInputTokens > report.CacheSessions[j].LogicalInputTokens
		}
		if report.CacheSessions[i].Client != report.CacheSessions[j].Client {
			return report.CacheSessions[i].Client < report.CacheSessions[j].Client
		}
		return report.CacheSessions[i].SessionID < report.CacheSessions[j].SessionID
	})
	sort.Slice(report.UnpricedModels, func(i, j int) bool {
		if report.UnpricedModels[i].Client == report.UnpricedModels[j].Client {
			return report.UnpricedModels[i].Model < report.UnpricedModels[j].Model
		}
		return report.UnpricedModels[i].Client < report.UnpricedModels[j].Client
	})

	// Activity heatmap (weekday x hour): allocate an interval only when both
	// endpoints fall in the exact same local hour; crossing intervals never split.
	for i := range uniqueIntervals {
		m := &uniqueIntervals[i]
		contained, intersects := m.Interval.InWindow(options.From, options.To)
		if !intersects {
			continue
		}
		modelExcluded := options.Model != "" && m.Model != "" && m.Model != options.Model
		if modelExcluded {
			continue
		}
		loLocal := m.Interval.LowerAt.In(location)
		hiLocal := m.Interval.UpperAt.In(location)
		sameHour := loLocal.Year() == hiLocal.Year() && loLocal.Month() == hiLocal.Month() &&
			loLocal.Day() == hiLocal.Day() && loLocal.Hour() == hiLocal.Hour() &&
			loLocal.Format("-0700") == hiLocal.Format("-0700")
		if contained && sameHour && options.Provider == "" && (options.Model == "" || m.Model != "") {
			activityKey := [2]int{(int(loLocal.Weekday()) + 6) % 7, loLocal.Hour()}
			if activity[activityKey] == nil {
				activity[activityKey] = newStatsAccumulator()
			}
			if err := addIntervalToStatsAccumulator(activity[activityKey], m.Tokens, m.Key.Thread); err != nil {
				return StatsReport{}, false, err
			}
		} else {
			markCrossedActivityHours(activity, m.Interval.LowerAt, m.Interval.UpperAt, options.From, options.To, location)
		}
	}
	for weekday := 0; weekday < 7; weekday++ {
		for hour := 0; hour < 24; hour++ {
			value := activity[[2]int{weekday, hour}]
			if value == nil {
				value = newStatsAccumulator()
			}
			metricComplete, metricKnown := statsMetricValues(options.Metric, value)
			report.Activity = append(report.Activity, StatsActivity{Weekday: weekday, Hour: hour, Tokens: value.tokens, Sessions: int64(len(value.sessions)), Events: value.events, KnownCost: money(value.provider), MetricValue: metricComplete, KnownMetricValue: metricKnown})
		}
	}
	for warning := range warnings {
		report.Warnings = append(report.Warnings, warning)
	}
	sort.Strings(report.Warnings)
	partial := reduced.partial || reduced.allocated > 0
	return report, partial, nil
}

func markCrossedActivityHours(activity map[[2]int]*statsAccumulator, lo, hi, from, to time.Time, loc *time.Location) {
	if lo.Before(from) {
		lo = from
	}
	if !hi.Before(to) {
		hi = to.Add(-time.Nanosecond)
	}
	if hi.Before(lo) {
		return
	}
	// At most 168 weekday*hour cells exist; cap steps to 168 hours so multi-year
	// intervals terminate in O(1) time.
	cursor := bucketStart(lo, "hour", loc)
	for step := 0; step < 7*24 && !cursor.After(hi); step++ {
		local := cursor.In(loc)
		key := [2]int{(int(local.Weekday()) + 6) % 7, local.Hour()}
		if activity[key] == nil {
			activity[key] = newStatsAccumulator()
		}
		activity[key].catalogComplete = false
		activity[key].providerComplete = false
		cursor = cursor.Add(time.Hour)
	}
}
