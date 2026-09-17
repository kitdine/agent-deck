package usage

import (
	"context"
	"fmt"
	"time"
)

// SignalsBatch shares event reads and pricing across a request's overlapping
// ranges. It retains no state after the call and does not change ordinary
// Signals callers. Workflow/tooling still use their existing scoped readers.
func (s *Service) SignalsBatch(ctx context.Context, options []SignalOptions) ([]SignalReport, error) {
	if len(options) == 0 {
		return []SignalReport{}, nil
	}
	from, to := options[0].From, options[0].To
	hasActivity := false
	activityEnabled := make([]bool, len(options))
	for i, o := range options {
		if !o.From.Before(o.To) {
			return nil, fmt.Errorf("usage signals range must have from before to")
		}
		if o.Client != "" && o.Client != "codex" && o.Client != "claude" {
			return nil, fmt.Errorf("usage signals client must be codex or claude")
		}
		if o.Activity != "" && !ValidActivityFilter(o.Activity) {
			return nil, fmt.Errorf("usage signals activity must be a documented category or subcategory")
		}
		selected, err := selectedSignalFamilies(o.Kinds)
		if err != nil {
			return nil, err
		}
		hasActivity = hasActivity || selected["activity"]
		activityEnabled[i] = selected["activity"]
		if o.From.Before(from) {
			from = o.From
		}
		if o.To.After(to) {
			to = o.To
		}
	}
	// Share only when an unfiltered activity option covers the union. Otherwise
	// reading/pricing unrelated events could introduce errors absent from every
	// individual request (for example separated windows or workflow-only ranges).
	coveredUnion := false
	for i, o := range options {
		if activityEnabled[i] && o.Client == "" && o.Activity == "" && o.From.Equal(from) && o.To.Equal(to) {
			coveredUnion = true
		}
	}
	if !coveredUnion {
		reports := make([]SignalReport, 0, len(options))
		for _, o := range options {
			report, err := s.Signals(ctx, o)
			if err != nil {
				return nil, err
			}
			reports = append(reports, report)
		}
		return reports, nil
	}
	folds := make([]activityCostFold, len(options))
	starts, ends := make([]string, len(options)), make([]string, len(options))
	for i, o := range options {
		folds[i] = newActivityCostFold()
		starts[i] = o.From.UTC().Format(time.RFC3339Nano)
		ends[i] = o.To.UTC().Format(time.RFC3339Nano)
	}
	if hasActivity {
		events, err := s.eventsRange(ctx, from, to, "", "")
		if err != nil {
			return nil, err
		}
		turns, err := classifiedTurns(ctx, s.Store.DB, "", "")
		if err != nil {
			return nil, err
		}
		// Preserve the existing empty-event path: no price access is required.
		if len(events) > 0 {
			resolver, err := s.loadReadPriceResolver(ctx, s.now())
			if err != nil {
				return nil, err
			}
			for _, event := range events {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				key := turnKey{client: event.Client, session: event.SessionID, index: event.TurnIndex}
				category, covered := turns[key]
				covered = covered && event.TurnIndex > 0
				selected := make([]int, 0, len(options))
				for i, o := range options {
					if !activityEnabled[i] {
						continue
					}
					// Match eventsRange's SQL string comparisons and half-open boundaries.
					if event.EventAt < starts[i] || event.EventAt >= ends[i] || (o.Client != "" && event.Client != o.Client) {
						continue
					}
					if o.Activity != "" && (!covered || (category.kind != o.Activity && category.sub != o.Activity)) {
						continue
					}
					if !covered {
						folds[i].uncovered++
						continue
					}
					selected = append(selected, i)
				}
				if len(selected) == 0 {
					continue
				}
				attribution, err := resolver.priceForEvent(event)
				if err != nil {
					return nil, err
				}
				result, err := calculateAttributedEvent(event, attribution)
				if err != nil {
					return nil, err
				}
				known, err := decimal(result.KnownCatalogBaseCost)
				if err != nil {
					return nil, err
				}
				for _, i := range selected {
					folds[i].add(key, category, known, result.CatalogBaseCost != nil)
				}
			}
		}
	}
	reports := make([]SignalReport, 0, len(options))
	for i, o := range options {
		cost := folds[i].activityCost(o.Client)
		report, err := s.signalsWithCost(ctx, o, &cost)
		if err != nil {
			return nil, err
		}
		reports = append(reports, report)
	}
	return reports, nil
}
