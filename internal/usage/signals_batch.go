package usage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"time"
)

// SignalsBatch shares event reads and pricing across a request's overlapping
// ranges. It retains no state after the call and does not change ordinary
// Signals callers. Workflow/tooling still use their existing scoped readers.
func (s *Service) SignalsBatch(ctx context.Context, options []SignalOptions) ([]SignalReport, error) {
	return s.signalsBatch(ctx, options, false)
}

func (s *Service) ProjectedSignalsBatch(ctx context.Context, options []SignalOptions) ([]SignalReport, error) {
	return s.signalsBatch(ctx, options, true)
}

func (s *Service) signalsBatch(ctx context.Context, options []SignalOptions, projected bool) ([]SignalReport, error) {
	read := func(ctx context.Context, window scopedRequestWindow) ([]storedEvent, projectedPricing, error) {
		if projected {
			return s.presentationInputs(ctx, window.from, window.to, s.now(), true)
		}
		events, err := s.eventsRange(ctx, window.from, window.to, window.client, window.session)
		if err != nil || len(events) == 0 {
			return events, nil, err
		}
		resolver, err := s.loadReadPriceResolver(ctx, s.now())
		if err != nil {
			return nil, nil, err
		}
		return events, signalEventPricing(resolver.priceForEvent), nil
	}
	return readSignalBatch(ctx, s.Store.DB, options, read, s.Signals)
}

// readScopedSignalsBatch reads the published work and tooling facts from the
// caller's admitted transaction. The request reader supplies effective
// requests and pricing from the same view.
func readScopedSignalsBatch(ctx context.Context, tx *sql.Tx, options []SignalOptions, read readScopedRequests) ([]SignalReport, error) {
	if tx == nil || read == nil {
		return nil, errors.New("missing admitted signal snapshot or request reader")
	}
	return readSignalBatch(ctx, tx, options, scopedSignalInputs(tx, read), nil)
}

func readScopedSessionSignals(ctx context.Context, tx *sql.Tx, client, session string, read readScopedRequests) (SessionSignalSummary, bool, error) {
	if tx == nil || read == nil {
		return SessionSignalSummary{}, false, errors.New("missing admitted signal snapshot or request reader")
	}
	return readSessionSignals(ctx, tx, client, session, func(scope SignalScope) (SessionCategory, error) {
		events, pricing, err := scopedSignalInputs(tx, read)(ctx, scopedRequestWindow{scope.From, scope.To, scope.Client, scope.Session})
		if err != nil {
			return SessionCategory{}, err
		}
		turns, err := classifiedTurns(ctx, tx, scope.Client, scope.Session)
		if err != nil {
			return SessionCategory{}, err
		}
		fold, err := foldSignalActivity(events, turns, pricing)
		return fold.sessionCategory(), err
	})
}

type signalBatchInputs func(context.Context, scopedRequestWindow) ([]storedEvent, projectedPricing, error)

func scopedSignalInputs(tx *sql.Tx, read readScopedRequests) signalBatchInputs {
	return func(ctx context.Context, window scopedRequestWindow) ([]storedEvent, projectedPricing, error) {
		events, resolver, err := read(ctx, tx, window)
		if err != nil {
			return nil, nil, err
		}
		if events == nil {
			return nil, nil, errors.New("admitted signal request set unavailable")
		}
		if len(events) == 0 {
			return events, nil, nil
		}
		if resolver == nil {
			return nil, nil, errors.New("admitted signal pricing inputs unavailable")
		}
		return events, signalEventPricing(resolver.priceForEvent), nil
	}
}

func readSignalBatch(ctx context.Context, db signalQueryer, options []SignalOptions, read signalBatchInputs, legacyFallback func(context.Context, SignalOptions) (SignalReport, error)) ([]SignalReport, error) {
	if len(options) == 0 {
		return []SignalReport{}, nil
	}
	from, to := options[0].From, options[0].To
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
		for i, o := range options {
			if legacyFallback != nil {
				report, err := legacyFallback(ctx, o)
				if err != nil {
					return nil, err
				}
				reports = append(reports, report)
				continue
			}
			window := scopedRequestWindow{from: o.From, to: o.To, client: o.Client}
			result, err := readSignalBatchWindow(ctx, db, options[i:i+1], activityEnabled[i:i+1], window, read)
			if err != nil {
				return nil, err
			}
			reports = append(reports, result[0])
		}
		return reports, nil
	}
	return readSignalBatchWindow(ctx, db, options, activityEnabled, scopedRequestWindow{from: from, to: to}, read)
}

func readSignalBatchWindow(ctx context.Context, db signalQueryer, options []SignalOptions, activityEnabled []bool, window scopedRequestWindow, read signalBatchInputs) ([]SignalReport, error) {
	folds := make([]activityCostFold, len(options))
	starts, ends := make([]string, len(options)), make([]string, len(options))
	for i, o := range options {
		folds[i] = newActivityCostFold()
		starts[i] = o.From.UTC().Format(time.RFC3339Nano)
		ends[i] = o.To.UTC().Format(time.RFC3339Nano)
	}
	if slices.Contains(activityEnabled, true) {
		events, pricing, err := read(ctx, window)
		if err != nil {
			return nil, err
		}
		turns, err := classifiedTurns(ctx, db, window.client, window.session)
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
				if event.EventAt < starts[i] || event.EventAt >= ends[i] || (o.Client != "" && event.Client != o.Client) || !o.Sessions.Allows(event.Client, event.SessionID) {
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
			_, result, err := pricing(event)
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
	reports := make([]SignalReport, 0, len(options))
	for i, o := range options {
		cost := folds[i].activityCost(o.Client)
		report, err := readSignalReport(ctx, db, o, func(context.Context, SignalScope) (ActivityCost, error) {
			return cost, nil
		})
		if err != nil {
			return nil, err
		}
		reports = append(reports, report)
	}
	return reports, nil
}
