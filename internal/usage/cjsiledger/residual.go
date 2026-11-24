package cjsiledger

import (
	"cmp"
	"slices"
)

// A per-copy assessment can contribute a legacy call, propose a bounded
// interval, or defer to a more specific original observation. It never
// publishes an interval. Failure rejects the range across the whole replay.
type oldAssessment struct {
	call     Member
	interval bool
	model    string
	claims   []observedMeter
}

func appendReferences(dst []Reference, src ...Reference) []Reference {
	for _, r := range src {
		if !slices.Contains(dst, r) {
			dst = append(dst, r)
		}
	}
	return dst
}

// Merge the request's boundaries as well as its meter. Comparing just the
// most recent copy lets a missing detail hide a disagreement between aliases.
func mergeClaim(claims map[Key]observedMeter, n observedMeter) bool {
	prev, found := claims[n.Key]
	if !found {
		n.Proof = slices.Clone(n.Proof)
		claims[n.Key] = n
		return true
	}
	var ok bool
	if prev.lo, ok = commonCounts(prev.lo, n.lo); !ok {
		return false
	}
	if prev.hi, ok = commonCounts(prev.hi, n.hi); !ok {
		return false
	}
	if prev.Tokens, ok = commonCounts(prev.Tokens, n.Tokens); !ok {
		return false
	}
	if prev.Model != n.Model {
		prev.Model = ""
	}
	if prev.TurnID != n.TurnID {
		prev.TurnID = ""
		prev.TurnIndex = 0
	}
	if prev.TurnIndex != n.TurnIndex {
		prev.TurnIndex = 0
	}
	if prev.At.IsZero() || !n.At.IsZero() && n.At.Before(prev.At) {
		prev.At = n.At
	}
	prev.Proof = appendReferences(prev.Proof, n.Proof...)
	claims[n.Key] = prev
	return true
}

func orderedClaims(claims map[Key]observedMeter) []observedMeter {
	selected := make([]observedMeter, 0, len(claims))
	for _, n := range claims {
		selected = append(selected, n)
	}
	slices.SortFunc(selected, func(a, b observedMeter) int {
		return cmp.Or(cmp.Compare(a.lo.Input, b.lo.Input), cmp.Compare(a.lo.Output, b.lo.Output), compareKey(a.Key, b.Key))
	})
	return selected
}

func containedClaims(o observedMeter, native []observedMeter, calls map[Key]Member, rejected map[Key]bool) ([]observedMeter, string, bool) {
	model := o.intervalModel
	contained := make(map[Key]observedMeter)
	for _, n := range native {
		if !o.bounded || !n.bounded || o.epoch != n.epoch {
			return nil, "", false
		}
		if disjoint(o, n) {
			continue
		}
		if !before(o.lo, n.lo) || !before(n.hi, o.hi) || rejected[n.Key] {
			return nil, "", false
		}
		if n.Model != model {
			model = ""
		}
		if n.Key.ID != "" {
			proved := calls[n.Key]
			if proved.Model != model {
				model = ""
			}
			n.Tokens = proved.Tokens
		}
		if !mergeClaim(contained, n) {
			return nil, "", false
		}
	}
	return orderedClaims(contained), model, true
}

// Spend both a detail's known tokens and the known tokens outside that
// detail. An unknown claim removes precision from the output; it does not
// erase the bounds needed to validate other claims. Subtraction checks
// the same sums as addition without overflowing int64.
type detailBudget struct {
	known, other   int64
	bounded, exact bool
}

func newDetailBudget(whole Counts, bit DetailMask, value, parent int64) detailBudget {
	known := whole.Known&bit != 0
	return detailBudget{value, parent - value, known, known}
}
func (b *detailBudget) spend(present bool, value, parent int64) bool {
	if !b.bounded {
		return true
	}
	if !present {
		b.exact = false
		return true
	}
	if value > b.known || parent-value > b.other {
		return false
	}
	b.known -= value
	b.other -= parent - value
	return true
}

// Along an ordered sequence an omitted detail supplies no new bound. A
// later explicit detail still has to follow its last known value and the
// last known complementary count, even across sparse intervening requests.
type detailPoint struct{ value, other int64 }

func (p *detailPoint) advance(known bool, value, parent int64) bool {
	if !known {
		return true
	}
	if value < p.value || parent-value < p.other {
		return false
	}
	p.value, p.other = value, parent-value
	return true
}

type counterCursor struct {
	input, output            int64
	cached, write, reasoning detailPoint
}

func (c *counterCursor) advance(p Counts) bool {
	if p.Input < c.input || p.Output < c.output {
		return false
	}
	c.input, c.output = p.Input, p.Output
	return c.cached.advance(p.Known&CachedKnown != 0, p.Cached, p.Input) &&
		c.write.advance(p.Known&WriteKnown != 0, p.Write, p.Input) &&
		c.reasoning.advance(p.Known&ReasoningKnown != 0, p.Reasoning, p.Output)
}

// Reconcile one observation (or the final merged endpoints) with each
// distinct contained, metered request exactly once. Check the ordered
// counter positions independently of the arithmetic on the primary totals.
func unclaimedTokens(lo, hi, observed Counts, selected []observedMeter) (Counts, bool) {
	whole, ok := difference(hi, lo)
	if !ok {
		return Counts{}, false
	}
	whole, ok = commonCounts(whole, observed)
	if !ok {
		return Counts{}, false
	}
	cached := newDetailBudget(whole, CachedKnown, whole.Cached, whole.Input)
	write := newDetailBudget(whole, WriteKnown, whole.Write, whole.Input)
	reasoning := newDetailBudget(whole, ReasoningKnown, whole.Reasoning, whole.Output)
	left := Counts{Input: whole.Input, Output: whole.Output}
	var cursor counterCursor
	if !cursor.advance(lo) {
		return Counts{}, false
	}
	for _, n := range selected {
		if !cursor.advance(n.lo) || !cursor.advance(n.hi) {
			return Counts{}, false
		}
		delta, ok := difference(n.hi, n.lo)
		if !ok {
			return Counts{}, false
		}
		if _, ok = commonCounts(delta, n.Tokens); !ok {
			return Counts{}, false
		}
		if n.Key.ID == "" {
			continue
		}
		t := n.Tokens
		if t.Input > left.Input || t.Output > left.Output || !cached.spend(t.Known&CachedKnown != 0, t.Cached, t.Input) || !write.spend(t.Known&WriteKnown != 0, t.Write, t.Input) || !reasoning.spend(t.Known&ReasoningKnown != 0, t.Reasoning, t.Output) {
			return Counts{}, false
		}
		left.Input -= t.Input
		left.Output -= t.Output
	}
	if !cursor.advance(hi) {
		return Counts{}, false
	}
	if cached.exact {
		left.Cached = cached.known
		left.Known |= CachedKnown
	}
	if write.exact {
		left.Write = write.known
		left.Known |= WriteKnown
	}
	if reasoning.exact {
		left.Reasoning = reasoning.known
		left.Known |= ReasoningKnown
	}
	return left, true
}

func assessOld(o observedMeter, old, native []observedMeter, calls map[Key]Member, rejected map[Key]bool) (oldAssessment, bool) {
	selected, model, ok := containedClaims(o, native, calls, rejected)
	if !ok {
		return oldAssessment{}, false
	}
	stableClaim := len(selected) > 0
	for _, n := range selected {
		if n.Key.ID == "" {
			stableClaim = false
		}
	}
	hasOriginal := false
	if o.bounded {
		for _, other := range old {
			if !other.bounded || o.epoch != other.epoch || disjoint(o, other) {
				continue
			}
			// The range group reconciles auxiliary endpoint fields from every
			// copy. Here only the primary counter positions identify a peer.
			sameRange := o.lo.Input == other.lo.Input && o.lo.Output == other.lo.Output && o.hi.Input == other.hi.Input && o.hi.Output == other.hi.Output
			if !sameRange || !o.aggregateOnly && !other.aggregateOnly && o.Key != other.Key && !stableClaim {
				return oldAssessment{}, false
			}
			if other.intervalModel != model {
				model = ""
			}
			if !other.aggregateOnly {
				hasOriginal = true
			}
		}
		if _, valid := unclaimedTokens(o.lo, o.hi, o.Tokens, selected); !valid {
			return oldAssessment{}, false
		}
	}
	if o.aggregateOnly && hasOriginal {
		return oldAssessment{}, true
	}
	if len(selected) == 0 && !o.aggregateOnly {
		return oldAssessment{call: o.Member}, o.Key.ID != ""
	}
	if !o.aggregateOnly && len(selected) == 1 && samePoint(o.lo, selected[0].lo) && samePoint(o.hi, selected[0].hi) {
		n := selected[0]
		if n.Key.ID == "" {
			n.Key = o.Key
		}
		if n.Key.ID == "" {
			return oldAssessment{}, false
		}
		if _, ok := commonCounts(n.Tokens, o.Tokens); !ok {
			return oldAssessment{}, false
		}
		n.Proof = appendReferences(slices.Clone(n.Proof), o.Proof...)
		return oldAssessment{call: n.Member, claims: selected}, true
	}
	// Missing time cannot establish a window. Only an actual reversal
	// contradicts another copy; all numeric checks above still apply.
	if !o.bounded || !o.start.IsZero() && !o.At.IsZero() && o.At.Before(o.start) {
		return oldAssessment{}, false
	}
	return oldAssessment{interval: true, model: model, claims: selected}, true
}

type residualGroup struct {
	interval              Member
	measured              Counts
	claims                map[Key]observedMeter
	hasResidual, rejected bool
}

// Every bounded observation contributes its endpoint facts, including
// observations later superseded by a more specific legacy call.
func (g *residualGroup) observe(key Key, o observedMeter) {
	if g.rejected {
		return
	}
	if g.interval.Key.ID == "" {
		g.interval = Member{Key: key, Interval: CounterRange{Epoch: o.epoch, Lower: o.lo, Upper: o.hi}}
		g.measured = o.Tokens
		return
	}
	lo, l := commonCounts(g.interval.Interval.Lower, o.lo)
	hi, h := commonCounts(g.interval.Interval.Upper, o.hi)
	measured, m := commonCounts(g.measured, o.Tokens)
	if !l || !h || !m {
		g.rejected = true
		return
	}
	g.interval.Interval.Lower, g.interval.Interval.Upper, g.measured = lo, hi, measured
}

func (g *residualGroup) propose(o observedMeter, a oldAssessment) {
	if g.rejected {
		return
	}
	g.hasResidual = true
	// A complete pair from a single observation is required. An incomplete
	// pair neither contributes a time bound nor widens another copy's window.
	if o.start.IsZero() || o.At.IsZero() {
		return
	}
	r := &g.interval.Interval
	if r.LowerAt.IsZero() {
		g.interval.Model = a.model
		r.LowerAt, r.UpperAt = o.start, o.At
	} else {
		if g.interval.Model != a.model {
			g.interval.Model = ""
		}
		if o.start.Before(r.LowerAt) {
			r.LowerAt = o.start
		}
		if o.At.After(r.UpperAt) {
			r.UpperAt = o.At
		}
	}
	g.interval.Proof = appendReferences(g.interval.Proof, o.begin, o.Proof[0])
}

func (g *residualGroup) noteClaims(claims []observedMeter) {
	if g.rejected || len(claims) == 0 {
		return
	}
	if g.claims == nil {
		g.claims = make(map[Key]observedMeter)
	}
	for _, n := range claims {
		if !mergeClaim(g.claims, n) {
			g.rejected = true
			return
		}
	}
}

// This is the only gate that emits a TokenInterval. Requests live in their
// own ledger; a rejected range never takes an independently proved call out.
func (g *residualGroup) finish(calls map[Key]Member, linked map[Reference]bool) (Member, Gap) {
	if g.rejected {
		return Member{}, PossibleOverlap
	}
	selected := orderedClaims(g.claims)
	for i, n := range selected {
		if n.Key.ID != "" {
			call, ok := calls[n.Key]
			if !ok {
				return Member{}, PossibleOverlap
			}
			selected[i].Tokens = call.Tokens
		}
	}
	remaining, ok := unclaimedTokens(g.interval.Interval.Lower, g.interval.Interval.Upper, g.measured, selected)
	if !ok {
		return Member{}, PossibleOverlap
	}
	if !g.hasResidual || remaining.Input == 0 && remaining.Output == 0 {
		return Member{}, 0
	}
	if g.interval.Interval.LowerAt.IsZero() {
		return Member{}, UnallocatedTokens
	}
	markAnonymous(linked, selected)
	m := g.interval
	m.Tokens = remaining
	return m, UnallocatedTokens
}
func markAnonymous(linked map[Reference]bool, selected []observedMeter) {
	for _, n := range selected {
		if n.Key.ID == "" {
			for _, ref := range n.Proof {
				linked[ref] = true
			}
		}
	}
}
