package cjsiledger

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"
	"time"

	"github.com/kitdine/agent-deck/internal/ingest"
)

func counts(c ingest.CodexCounts) (Counts, bool) {
	total, ok := c.Total()
	if !ok || c.ReportedTotal.State == ingest.CodexValid && c.ReportedTotal.Value != total || c.CacheWrite.Value > c.Input.Value {
		return Counts{}, false
	}
	n := Counts{Input: c.Input.Value, Output: c.Output.Value, Cached: c.Cached.Value, Write: c.CacheWrite.Value, Reasoning: c.Reasoning.Value}
	if c.Cached.State == ingest.CodexValid {
		n.Known |= CachedKnown
	}
	if c.CacheWrite.State == ingest.CodexValid {
		n.Known |= WriteKnown
	}
	if c.Reasoning.State == ingest.CodexValid {
		n.Known |= ReasoningKnown
	}
	return n, true
}
func difference(hi, lo Counts) (Counts, bool) {
	n := Counts{Input: hi.Input - lo.Input, Output: hi.Output - lo.Output, Known: hi.Known & lo.Known}
	if n.Known&CachedKnown != 0 {
		n.Cached = hi.Cached - lo.Cached
	}
	if n.Known&WriteKnown != 0 {
		n.Write = hi.Write - lo.Write
	}
	if n.Known&ReasoningKnown != 0 {
		n.Reasoning = hi.Reasoning - lo.Reasoning
	}
	return n, n.Input >= 0 && n.Output >= 0 && n.Cached >= 0 && n.Cached <= n.Input && n.Write >= 0 && n.Write <= n.Input && n.Reasoning >= 0 && n.Reasoning <= n.Output
}
func commonCounts(a, b Counts) (Counts, bool) {
	if a.Input != b.Input || a.Output != b.Output {
		return Counts{}, false
	}
	for _, f := range []struct {
		bit  DetailMask
		a, b *int64
	}{{CachedKnown, &a.Cached, &b.Cached}, {WriteKnown, &a.Write, &b.Write}, {ReasoningKnown, &a.Reasoning, &b.Reasoning}} {
		if a.Known&b.Known&f.bit != 0 && *f.a != *f.b {
			return Counts{}, false
		}
		if b.Known&f.bit != 0 {
			*f.a = *f.b
		}
	}
	a.Known |= b.Known
	return a, true
}

type observedMeter struct {
	aggregateOnly bool
	intervalModel string
	Member
	bounded bool
	epoch   [32]byte
	lo, hi  Counts
	start   time.Time
	begin   Reference
}

func collect(s Source) (native, old []observedMeter, gaps Gap) {
	var base Counts
	var baseAt time.Time
	var baseRef Reference
	var epochStart int64 = -1
	var epoch [32]byte
	var comparable bool
	var previousNative Counts
	var previousNativeAt time.Time
	var previousNativeRef Reference
	var nativeBaseline bool
	var lastThreadTotal Counts
	var haveThreadTotal bool
	for _, m := range s.meters {
		r := m.record
		if epochStart != m.epochStart {
			epochStart = m.epochStart
			comparable = false
			nativeBaseline = false
			haveThreadTotal = false
			epoch = m.epochHash
		}
		ref := Reference{s.stamp, m.span}
		obs := observedMeter{Member: Member{Key: Key{Thread: s.thread}, At: r.At, TurnID: m.turn, TurnIndex: m.turnIndex, Model: m.model, Proof: []Reference{ref}}, epoch: epoch}
		if r.Kind == ingest.CodexRequestUsage {
			hi, hasTotal := counts(r.ThreadTotal)
			// Legacy totals and native totals can describe the same progress out of
			// record order. Detect a native rewind against its own previous sample.
			if !hasTotal || haveThreadTotal && !before(lastThreadTotal, hi) {
				comparable = false
				nativeBaseline = false
				epoch = m.prefixHash
				obs.epoch = epoch
			}
			lastThreadTotal, haveThreadTotal = hi, hasTotal
			n, ok := counts(r.Usage)
			if !ok {
				gaps |= InvalidMeter
				nativeBaseline = false
				continue
			}
			obs.Key.Kind, obs.Key.ID, obs.Tokens = Response, r.ResponseID, n
			if hasTotal {
				obs.hi = hi
				obs.lo, obs.bounded = difference(hi, n)
				if r.ResponseID == "" && obs.bounded && nativeBaseline && samePoint(previousNative, obs.lo) && !previousNativeAt.IsZero() && !r.At.Before(previousNativeAt) && n.Input+n.Output > 0 {
					interval := obs
					interval.aggregateOnly = true
					interval.start, interval.begin = previousNativeAt, previousNativeRef
					interval.intervalModel = m.modelAcross(previousNativeRef)
					old = append(old, interval)
				}
				previousNative, previousNativeAt, previousNativeRef, nativeBaseline = hi, r.At, ref, true
			}
			native = append(native, obs)
			continue
		}
		hi, valid := counts(r.LegacyTotal)
		n, hasLast := counts(r.LegacyLast)
		if r.LegacyTotal.State != ingest.CodexAbsent && !valid || r.LegacyLast.State != ingest.CodexAbsent && !hasLast {
			gaps |= InvalidMeter
		}
		if !valid {
			epoch = m.prefixHash
			haveThreadTotal = false
		}
		validDelta := hasLast
		if valid && comparable {
			delta, ok := difference(hi, base)
			if ok {
				obs.bounded = true
				obs.intervalModel = m.modelAcross(baseRef)
				obs.lo, obs.hi, obs.start, obs.begin = base, hi, baseAt, baseRef
				n, validDelta = delta, true
			} else {
				haveThreadTotal = false
				epoch = m.prefixHash
			}
		}
		obs.epoch = epoch
		comparable = valid
		base, baseAt, baseRef = hi, r.At, ref
		previousNative, previousNativeAt, previousNativeRef, nativeBaseline = hi, r.At, ref, valid
		if !validDelta || n.Input+n.Output == 0 {
			continue
		}
		obs.Key.Kind, obs.Key.ID, obs.Tokens = LegacyCall, m.legacyKey, n
		old = append(old, obs)
	}
	return
}

func (m meter) modelAcross(lower Reference) string {
	if m.modelFrom > lower.Span.Start {
		return ""
	}
	return m.model
}

func residualKey(epoch [32]byte, lo, hi Counts) string {
	b, _ := json.Marshal(struct {
		Epoch        [32]byte
		Lower, Upper [2]int64
	}{epoch, [2]int64{lo.Input, lo.Output}, [2]int64{hi.Input, hi.Output}})
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func (r CounterRange) InWindow(from, to time.Time) (contained, intersects bool) {
	if r.LowerAt.IsZero() || r.UpperAt.IsZero() {
		return false, true
	}
	return !r.LowerAt.Before(from) && r.UpperAt.Before(to), !r.UpperAt.Before(from) && r.LowerAt.Before(to)
}

func samePoint(a, b Counts) bool       { _, ok := commonCounts(a, b); return ok }
func before(a, b Counts) bool          { _, ok := difference(b, a); return ok }
func disjoint(a, b observedMeter) bool { return before(a.hi, b.lo) || before(b.hi, a.lo) }

func classify(ctx context.Context, sources []Source) ([]Member, Gap, error) {
	calls := map[Key]Member{}
	rejected := map[Key]bool{}
	var native, old []observedMeter
	var gaps Gap
	add := func(m Member) {
		if rejected[m.Key] {
			return
		}
		prior, ok := calls[m.Key]
		if !ok {
			calls[m.Key] = m
			return
		}
		joined, agree := commonCounts(prior.Tokens, m.Tokens)
		if !agree {
			delete(calls, m.Key)
			rejected[m.Key] = true
			gaps |= ConflictingResponse
			return
		}
		prior.Tokens = joined
		prior.Proof = appendReferences(prior.Proof, m.Proof...)
		if prior.Model != m.Model {
			prior.Model = ""
		}
		if prior.TurnID != m.TurnID {
			prior.TurnID = ""
			prior.TurnIndex = 0
		}
		if prior.TurnIndex != m.TurnIndex {
			prior.TurnIndex = 0
		}
		if prior.At.IsZero() || !m.At.IsZero() && m.At.Before(prior.At) {
			prior.At = m.At
		}
		calls[m.Key] = prior
	}
	for _, s := range sources {
		if err := ctx.Err(); err != nil {
			return nil, 0, err
		}
		ns, os, g := collect(s)
		native = append(native, ns...)
		old = append(old, os...)
		gaps |= g
	}
	for _, n := range native {
		if n.Key.ID != "" {
			add(n.Member)
		}
	}
	// A legacy observation can only claim a request in an authenticated
	// comparable epoch. A gap in either range leaves the confirmed call alone.
	old = splitLegacyRanges(old)
	linked := map[Reference]bool{}
	groups := make(map[Key]*residualGroup)
	type legacyWitness struct {
		group      *residualGroup
		assessment oldAssessment
	}
	var pending []legacyWitness
	for _, o := range old {
		if err := ctx.Err(); err != nil {
			return nil, 0, err
		}
		var group *residualGroup
		if o.bounded {
			key := Key{TokenInterval, o.Key.Thread, residualKey(o.epoch, o.lo, o.hi)}
			group = groups[key]
			if group == nil {
				group = &residualGroup{}
				groups[key] = group
			}
			group.observe(key, o)
		}
		a, ok := assessOld(o, old, native, calls, rejected)
		if !ok {
			gaps |= PossibleOverlap
			if group != nil {
				group.rejected = true
			}
			continue
		}
		if group != nil {
			group.noteClaims(a.claims)
		}
		if a.call.Key.ID != "" {
			pending = append(pending, legacyWitness{group, a})
		}
		if a.interval {
			group.propose(o, a)
		}
	}
	for _, group := range groups {
		if err := ctx.Err(); err != nil {
			return nil, 0, err
		}
		member, g := group.finish(calls, linked)
		gaps |= g
		if g&PossibleOverlap != 0 {
			group.rejected = true
		}
		if member.Key.Kind == TokenInterval {
			calls[member.Key] = member
		}
	}
	// Only a surviving group can attach its old representative or alias.
	// Stable responses already exist independently in calls.
	for _, w := range pending {
		if w.group != nil && w.group.rejected {
			continue
		}
		add(w.assessment.call)
		markAnonymous(linked, w.assessment.claims)
	}
	for _, n := range native {
		if n.Key.ID == "" && !linked[n.Proof[0]] {
			gaps |= PossibleOverlap
		}
	}
	result := make([]Member, 0, len(calls))
	for _, m := range calls {
		result = append(result, m)
	}
	slices.SortFunc(result, func(a, b Member) int { return compareKey(a.Key, b.Key) })
	return result, gaps, nil
}
