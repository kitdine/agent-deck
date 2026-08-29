package usage

import (
	"context"
	"database/sql"
	"math/big"
	"strconv"
	"time"

	"github.com/kitdine/agent-deck/internal/activity"
)

// Decision 4's cost_basis discriminator. It travels with every cost figure this
// file produces, because an unavailable cost and a measured zero print the same
// character and mean opposite things.
const (
	CostBasisTurn    = "turn"
	CostBasisPartial = "partial"
	CostBasisNone    = "none"
)

// activityKindPrecedence is Decision 3's category order. It is the render order
// on both surfaces and the last of Decision 5's three tie-breaks, which is why
// one list serves both rather than each site restating it.
var activityKindPrecedence = []string{
	activity.KindDelegation,
	activity.KindDebugging,
	activity.KindCoding,
	activity.KindConversation,
}

// activitySubOrder is the Subcategories table, in the order it lists them. Every
// subcategory is emitted even when empty, so a category's expanded list does not
// change shape with the data.
var activitySubOrder = map[string][]string{
	activity.KindDelegation:   {activity.SubSubagent, activity.SubWorkflow},
	activity.KindDebugging:    {activity.SubInvestigation, activity.SubRepair},
	activity.KindCoding:       {activity.SubFeature, activity.SubRefactoring, activity.SubTesting, activity.SubMaintenance},
	activity.KindConversation: {activity.SubExploration, activity.SubBrainstorming, activity.SubPlanning},
}

// ActivityCostSub is one subcategory's slice of the scope. Share is a percentage
// of the whole scope rather than of the parent, because the parent's bar is the
// only proportion drawn and a second denominator underneath it would not add up
// to the bar the reader is looking at.
type ActivityCostSub struct {
	Kind   string  `json:"kind"`
	Share  float64 `json:"share"`
	Cost   string  `json:"cost"`
	Events int64   `json:"events"`
}

// ActivityCostKind is one of the four categories with the cost that reached it
// through its turns. Turns carries no wire field: Decision 9's item shape stops
// at events, and the count exists here for Decision 5's tie-break and for the
// surfaces' "no turn in the selected scope" test.
type ActivityCostKind struct {
	Kind   string            `json:"kind"`
	Share  float64           `json:"share"`
	Cost   string            `json:"cost"`
	Events int64             `json:"events"`
	Turns  int64             `json:"-"`
	Sub    []ActivityCostSub `json:"sub"`
}

// ActivityCost is Decision 4's derivation for one scope: always four category
// rows, plus the discriminator saying how much of the scope those figures cover.
type ActivityCost struct {
	CostBasis string             `json:"cost_basis"`
	Kinds     []ActivityCostKind `json:"kinds"`
}

// SessionCategory is Decision 5's reduction of one session to one word. Kind is
// empty when CostBasis is `none`: the reduction divides by attributed cost, and
// with no priced event covering the session there is nothing to divide.
type SessionCategory struct {
	CostBasis string `json:"cost_basis"`
	Kind      string `json:"kind,omitempty"`
}

// ActivityCostRange folds one period's usage events onto the categories their
// turns were classified as. The join is structural rather than temporal: the
// parser recorded which turn each event fell after, so nothing here re-derives
// that from a timestamp window.
func (s *Service) ActivityCostRange(ctx context.Context, from, to time.Time, client string) (ActivityCost, error) {
	return s.ActivityCostForScope(ctx, SignalScope{From: from, To: to, Client: client})
}

// ActivityCostForScope applies the CLI's optional activity filter before the
// fold, so excluded turns do not become uncovered spend. The resulting shares
// are therefore renormalized while cost and event counts remain the selected
// turns' real values.
func (s *Service) ActivityCostForScope(ctx context.Context, scope SignalScope) (ActivityCost, error) {
	events, err := s.eventsRange(ctx, scope.From, scope.To, scope.Client, scope.Session)
	if err != nil {
		return ActivityCost{}, err
	}
	turns, err := classifiedTurns(ctx, s.Store.DB, scope.Client, scope.Session)
	if err != nil {
		return ActivityCost{}, err
	}
	if scope.Activity != "" {
		selected := make(map[turnKey]turnCategory)
		for key, category := range turns {
			if category.kind == scope.Activity || category.sub == scope.Activity {
				selected[key] = category
			}
		}
		filtered := make([]storedEvent, 0, len(events))
		for _, event := range events {
			key := turnKey{client: event.Client, session: event.SessionID, index: event.TurnIndex}
			if _, ok := selected[key]; ok {
				filtered = append(filtered, event)
			}
		}
		events, turns = filtered, selected
	}
	fold, err := s.foldActivityCost(ctx, events, turns)
	if err != nil {
		return ActivityCost{}, err
	}
	return fold.activityCost(scope.Client), nil
}

// SessionActivityCategory answers the one word on `session show --activity`.
// Cost is the divisor rather than turn count because a session of twenty
// one-sentence conversation turns and three long coding turns is a coding
// session, and counting turns would call it a conversation.
func (s *Service) SessionActivityCategory(ctx context.Context, client, sessionID string) (SessionCategory, error) {
	events, err := s.events(ctx, client, sessionID)
	if err != nil {
		return SessionCategory{}, err
	}
	turns, err := classifiedTurns(ctx, s.Store.DB, client, sessionID)
	if err != nil {
		return SessionCategory{}, err
	}
	fold, err := s.foldActivityCost(ctx, events, turns)
	if err != nil {
		return SessionCategory{}, err
	}
	result := SessionCategory{CostBasis: fold.basis()}
	if result.CostBasis == CostBasisNone {
		return result, nil
	}
	result.Kind = fold.dominantKind()
	return result, nil
}

type turnKey struct {
	client, session string
	index           int
}

type turnCategory struct{ kind, sub string }

// classifiedTurns reads the turns an aggregate is allowed to see. Decision 11
// makes a `pending` row a turn that does not exist yet, so the state filter sits
// here rather than at each call site: counting one would inflate turn counts,
// and attributing cost to one would attribute it to a classification nobody has
// made.
func classifiedTurns(ctx context.Context, db *sql.DB, client, session string) (map[turnKey]turnCategory, error) {
	query := `SELECT client,session_id,turn_index,activity_kind,activity_sub FROM usage_work_signals WHERE state=?`
	args := []any{signalStateClassified}
	if client != "" {
		query += ` AND client=?`
		args = append(args, client)
	}
	if session != "" {
		query += ` AND session_id=?`
		args = append(args, session)
	}
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	turns := map[turnKey]turnCategory{}
	for rows.Next() {
		var key turnKey
		var category turnCategory
		if err = rows.Scan(&key.client, &key.session, &key.index, &category.kind, &category.sub); err != nil {
			return nil, err
		}
		turns[key] = category
	}
	return turns, rows.Err()
}

// activityCostFold accumulates one scope. It keeps the running cost as a big.Rat
// for the same reason the rest of the package does: a percentage taken over
// rounded money is not the percentage of the money.
type activityCostFold struct {
	total      *big.Rat
	kindCost   map[string]*big.Rat
	subCost    map[string]map[string]*big.Rat
	kindEvents map[string]int64
	subEvents  map[string]map[string]int64
	kindTurns  map[string]map[turnKey]struct{}
	priced     int64
	uncovered  int64
}

func newActivityCostFold() activityCostFold {
	fold := activityCostFold{
		total:      new(big.Rat),
		kindCost:   map[string]*big.Rat{},
		subCost:    map[string]map[string]*big.Rat{},
		kindEvents: map[string]int64{},
		subEvents:  map[string]map[string]int64{},
		kindTurns:  map[string]map[turnKey]struct{}{},
	}
	for _, kind := range activityKindPrecedence {
		fold.kindCost[kind] = new(big.Rat)
		fold.kindTurns[kind] = map[turnKey]struct{}{}
		fold.subCost[kind] = map[string]*big.Rat{}
		fold.subEvents[kind] = map[string]int64{}
		for _, sub := range activitySubOrder[kind] {
			fold.subCost[kind][sub] = new(big.Rat)
		}
	}
	return fold
}

// foldActivityCost prices every event once and files it under the category of
// the turn it belongs to. The cost figure is the catalog base cost, which is the
// one Decision 4's `none` is defined against — this package calls an event
// `priced` exactly when that figure exists, and a provider-adjusted figure would
// report an unpriced scope as a confident zero.
func (s *Service) foldActivityCost(ctx context.Context, events []storedEvent, turns map[turnKey]turnCategory) (activityCostFold, error) {
	fold := newActivityCostFold()
	if len(events) == 0 {
		return fold, nil
	}
	resolver, err := s.loadReadPriceResolver(ctx, s.now())
	if err != nil {
		return fold, err
	}
	for _, event := range events {
		key := turnKey{client: event.Client, session: event.SessionID, index: event.TurnIndex}
		category, covered := turns[key]
		if event.TurnIndex <= 0 || !covered {
			fold.uncovered++
			continue
		}
		attribution, priceErr := resolver.priceForEvent(event)
		if priceErr != nil {
			return fold, priceErr
		}
		result, calculateErr := calculateAttributedEvent(event, attribution)
		if calculateErr != nil {
			return fold, calculateErr
		}
		known, decodeErr := decimal(result.KnownCatalogBaseCost)
		if decodeErr != nil {
			return fold, decodeErr
		}
		fold.add(key, category, known, result.CatalogBaseCost != nil)
	}
	return fold, nil
}

func (f *activityCostFold) add(key turnKey, category turnCategory, cost *big.Rat, priced bool) {
	f.total.Add(f.total, cost)
	kind := f.kindBucket(category.kind)
	kind.Add(kind, cost)
	sub := f.subBucket(category.kind, category.sub)
	sub.Add(sub, cost)
	f.kindEvents[category.kind]++
	f.subEvents[category.kind][category.sub]++
	f.kindTurns[category.kind][key] = struct{}{}
	if priced {
		f.priced++
	}
}

// kindBucket and subBucket create on demand so a value outside Decision 3's
// vocabulary still lands somewhere instead of panicking on a nil map. Nothing
// renders it — activityCost emits the fixed four — but losing it silently would
// make the shares disagree with the total that produced them.
func (f *activityCostFold) kindBucket(kind string) *big.Rat {
	if f.kindCost[kind] == nil {
		f.kindCost[kind] = new(big.Rat)
		f.kindTurns[kind] = map[turnKey]struct{}{}
		f.subCost[kind] = map[string]*big.Rat{}
		f.subEvents[kind] = map[string]int64{}
	}
	return f.kindCost[kind]
}

func (f *activityCostFold) subBucket(kind, sub string) *big.Rat {
	f.kindBucket(kind)
	if f.subCost[kind][sub] == nil {
		f.subCost[kind][sub] = new(big.Rat)
	}
	return f.subCost[kind][sub]
}

// basis reads Decision 4's table literally. `turn` and `partial` turn on whether
// every event in scope reached a classified turn; pricing decides only `none`.
// An event that predates the backfill and one whose turn is still pending are
// the same thing to a reader of the cost figure — spend the figure does not
// cover — so both count as uncovered.
func (f activityCostFold) basis() string {
	if f.priced == 0 {
		return CostBasisNone
	}
	if f.uncovered > 0 {
		return CostBasisPartial
	}
	return CostBasisTurn
}

// dominantKind is Decision 5's reduction. Its three tie-breaks are ordered by
// the loop rather than by a comparator: iterating in Decision 3's precedence
// order and replacing only on a strict improvement leaves the earliest category
// holding an exact tie, which is the third rule.
func (f activityCostFold) dominantKind() string {
	best := ""
	for _, kind := range activityKindPrecedence {
		if best == "" {
			best = kind
			continue
		}
		switch f.kindCost[kind].Cmp(f.kindCost[best]) {
		case 1:
			best = kind
		case 0:
			if len(f.kindTurns[kind]) > len(f.kindTurns[best]) {
				best = kind
			}
		}
	}
	return best
}

func (f activityCostFold) activityCost(client string) ActivityCost {
	out := ActivityCost{CostBasis: f.basis(), Kinds: make([]ActivityCostKind, 0, len(activityKindPrecedence))}
	for _, kind := range activityKindPrecedence {
		row := ActivityCostKind{
			Kind:   kind,
			Share:  shareOfCost(f.kindCost[kind], f.total),
			Cost:   money(f.kindCost[kind]),
			Events: f.kindEvents[kind],
			Turns:  int64(len(f.kindTurns[kind])),
			Sub:    []ActivityCostSub{},
		}
		for _, sub := range activitySubOrder[kind] {
			if !subcategoryHasClientSignal(client, kind, sub) {
				continue
			}
			row.Sub = append(row.Sub, ActivityCostSub{
				Kind:   sub,
				Share:  shareOfCost(f.subCost[kind][sub], f.total),
				Cost:   money(f.subCost[kind][sub]),
				Events: f.subEvents[kind][sub],
			})
		}
		out.Kinds = append(out.Kinds, row)
	}
	return out
}

// subcategoryHasClientSignal implements Decision 3's omission rule: a
// subcategory with no signal for the selected client is left out of the expanded
// list rather than rendered as a permanently empty row, which would invite the
// reader to conclude the user never uses skills when the truth is that the
// measurement does not exist there. Codex has no skill tool, so
// `delegation/workflow` is the only such subcategory.
func subcategoryHasClientSignal(client, kind, sub string) bool {
	return client != "codex" || kind != activity.KindDelegation || sub != activity.SubWorkflow
}

// shareOfCost renders a percentage at the precision the surfaces print, so the
// panel and the CLI carry the same number instead of two residues of the same
// division.
func shareOfCost(part, total *big.Rat) float64 {
	if total.Sign() == 0 {
		return 0
	}
	percent := new(big.Rat).Mul(new(big.Rat).Quo(part, total), big.NewRat(100, 1))
	value, err := strconv.ParseFloat(percent.FloatString(1), 64)
	if err != nil {
		return 0
	}
	return value
}
