package usage

import (
	"cmp"
	"errors"
	"slices"
	"time"

	ledger "github.com/kitdine/agent-deck/internal/usage/cjsiledger"
)

// Task 8 supplies the effective images on each side for the same bounded set
// of affected keys. Owner is from the admitted group, independently of the
// member. Whole-interval model proof comes either from member.Model (emitted
// by Task 3 when one local model covers the entire span) or an explicit model
// witness matching the key and bounds.
type intervalImpactImage struct {
	owner  SessionKey
	member ledger.Member
	model  *wholeIntervalModel
}

type wholeIntervalModel struct {
	key    ledger.Key
	bounds ledger.CounterRange
	model  string
}

// Private dependencies of aggregate reads. None is a priced-event partition.
type intervalImpactKind uint8

const (
	impactAggregateOnly intervalImpactKind = iota + 1
	impactModel
	impactClientSummary
	impactAllSummary
	impactCoverage
)

type intervalImpactScope struct {
	kind          intervalImpactKind
	client, model string
}

type intervalDirtyRange struct {
	scope    intervalImpactScope
	from, to time.Time // UTC midnight, [from,to)
}
type intervalImpactPlan []intervalDirtyRange

// A nil side is unavailable. An admitted empty side proves absence, including
// when the last interval is withdrawn. Private restaging alone changes no key.
// Both sides are validated completely before comparing OLD vs NEW.
func planIntervalImpact(old, next []intervalImpactImage) (intervalImpactPlan, error) {
	before, err := intervalImpactFacts(old)
	if err != nil {
		return nil, err
	}
	after, err := intervalImpactFacts(next)
	if err != nil {
		return nil, err
	}
	plan := intervalImpactPlan{}
	for key, prior := range before {
		current, ok := after[key]
		if ok && current == prior {
			continue
		}
		plan = prior.appendRanges(plan)
	}
	for key, current := range after {
		if prior, ok := before[key]; !ok || prior != current {
			plan = current.appendRanges(plan)
		}
	}
	slices.SortFunc(plan, func(a, b intervalDirtyRange) int {
		return cmp.Or(cmp.Compare(a.scope.kind, b.scope.kind), cmp.Compare(a.scope.client, b.scope.client),
			cmp.Compare(a.scope.model, b.scope.model), a.from.Compare(b.from), a.to.Compare(b.to))
	})
	merged := plan[:0]
	for _, r := range plan {
		if n := len(merged); n > 0 && merged[n-1].scope == r.scope && !r.from.After(merged[n-1].to) {
			if r.to.After(merged[n-1].to) {
				merged[n-1].to = r.to
			}
			continue
		}
		merged = append(merged, r)
	}
	return merged, nil
}

type intervalImpactFact struct {
	bounds ledger.CounterRange
	tokens ledger.Counts
	model  string // only a verified whole-interval model
}

func intervalImpactFacts(images []intervalImpactImage) (map[ledger.Key]intervalImpactFact, error) {
	if images == nil {
		return nil, errors.New("selected interval images unavailable")
	}
	facts := make(map[ledger.Key]intervalImpactFact, len(images))
	for _, image := range images {
		m := &image.member
		if image.owner.Client != "codex" || image.owner.SessionID != m.Key.Thread || m.Key.Thread == "" ||
			m.Key.Kind != ledger.TokenInterval || m.Key.ID == "" {
			return nil, errors.New("invalid selected interval owner or key")
		}
		if _, ok := facts[m.Key]; ok {
			return nil, errors.New("duplicate selected interval key")
		}
		r := utcCounterRange(m.Interval)
		if r.LowerAt.IsZero() || r.UpperAt.IsZero() || r.UpperAt.Before(r.LowerAt) || r.Epoch == ([32]byte{}) ||
			!validIntervalCounts(r.Lower) || !validIntervalCounts(r.Upper) || !validIntervalCounts(m.Tokens) ||
			!residualFitsRange(m.Tokens, r) {
			return nil, errors.New("invalid selected interval bounds or counts")
		}
		f := intervalImpactFact{bounds: r, tokens: m.Tokens, model: m.Model}
		if proof := image.model; proof != nil {
			if proof.model == "" || proof.key != m.Key || utcCounterRange(proof.bounds) != r {
				return nil, errors.New("whole-interval model witness mismatch")
			}
			if f.model != "" && f.model != proof.model {
				return nil, errors.New("conflicting whole-interval model witness")
			}
			f.model = proof.model
		}
		facts[m.Key] = f
	}
	return facts, nil
}

func utcCounterRange(r ledger.CounterRange) ledger.CounterRange {
	r.LowerAt, r.UpperAt = r.LowerAt.UTC(), r.UpperAt.UTC()
	return r
}

func (f intervalImpactFact) appendRanges(plan intervalImpactPlan) intervalImpactPlan {
	day := func(t time.Time) time.Time {
		y, m, d := t.UTC().Date()
		return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	}
	from, to := day(f.bounds.LowerAt), day(f.bounds.UpperAt).AddDate(0, 0, 1)
	target := intervalImpactScope{kind: impactAggregateOnly, client: "codex"}
	if f.model != "" {
		target = intervalImpactScope{kind: impactModel, client: "codex", model: f.model}
	}
	return append(plan, intervalDirtyRange{target, from, to},
		intervalDirtyRange{intervalImpactScope{kind: impactClientSummary, client: "codex"}, from, to},
		intervalDirtyRange{intervalImpactScope{kind: impactAllSummary}, from, to},
		intervalDirtyRange{intervalImpactScope{kind: impactCoverage, client: "codex"}, from, to})
}

// Match one cache dependency against a half-open query in any timezone.
// A filtered read also checks its client/day coverage dependency. Day-level
// invalidation deliberately does not allocate tokens to any touched bucket.
// Valid UTC day boundaries may equal Go's zero instant (0001-01-01T00:00:00Z).
func (p intervalImpactPlan) intersects(scope intervalImpactScope, from, to time.Time) (bool, error) {
	if p == nil || !from.Before(to) {
		return false, errors.New("unavailable interval impact plan or invalid query window")
	}
	switch scope.kind {
	case impactAllSummary:
		if scope.client != "" || scope.model != "" {
			return false, errors.New("invalid all-summary dependency")
		}
	case impactAggregateOnly, impactClientSummary, impactCoverage, impactModel:
		if scope.client == "" || scope.kind != impactModel && scope.model != "" {
			return false, errors.New("invalid interval dependency")
		}
	default:
		return false, errors.New("unknown interval dependency")
	}
	for _, r := range p {
		if r.scope == scope && r.from.Before(to) && from.Before(r.to) {
			return true, nil
		}
	}
	return false, nil
}
