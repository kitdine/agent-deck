package usage

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Empty client or session means all eligible clients or sessions, respectively.
// Model and provider never restrict the request set used for token shares.
type scopedRequestWindow struct {
	from, to        time.Time
	client, session string
}

// Task 8 supplies the complete effective request set in [from,to) for this
// client/session after ownership, masking and any admitted replacements or
// withdrawals. Return each event key once in legacy summary order, with all
// storedEvent binding/timing fields. An explicit empty slice proves absence.
// Resolve pricing/route inputs from this tx or immutable inputs bound to its
// admitted view. Close request rows before returning and propagate read errors.
type readScopedRequests func(context.Context, *sql.Tx, scopedRequestWindow) ([]storedEvent, *readPriceResolver, error)

// Task 8 supplies the complete effective tool-call set in [from,to) from the
// same pinned read transaction when building Stats model activity. Return an
// explicit empty slice when no tool calls exist; nil fails closed.
type readScopedToolCalls func(context.Context, *sql.Tx, scopedRequestWindow) ([]scopedToolCall, error)

// readScopedStats reads intervals, requests, pricing, and tool calls
// exclusively from one caller-pinned read transaction and admitted selection.
func readScopedStats(ctx context.Context, tx *sql.Tx, selected intervalMemberSelection, options StatsOptions, read readScopedRequests, readTools readScopedToolCalls) (StatsReport, bool, error) {
	if tx == nil || read == nil || readTools == nil {
		return StatsReport{}, false, errors.New("missing admitted stats snapshot, request reader or tool call reader")
	}
	members, err := readSelectedIntervalMembers(ctx, tx, selected, options, "")
	if err != nil {
		return StatsReport{}, false, err
	}
	window := scopedRequestWindow{from: options.From, to: options.To, client: options.Client}
	events, resolver, err := read(ctx, tx, window)
	if err != nil {
		return StatsReport{}, false, err
	}
	if events == nil {
		return StatsReport{}, false, errors.New("admitted request set unavailable")
	}
	var price eventPricing
	if len(events) > 0 {
		if resolver == nil {
			return StatsReport{}, false, errors.New("admitted request pricing inputs unavailable")
		}
		price = resolver.priceForEvent
	}
	toolCalls, err := readTools(ctx, tx, window)
	if err != nil {
		return StatsReport{}, false, err
	}
	if toolCalls == nil {
		return StatsReport{}, false, errors.New("admitted tool call set unavailable")
	}
	return reduceScopedStatsWithToolCalls(events, toolCalls, members, options, price)
}

// tx, selected and the request reader belong to one already admitted view.
// Group/protection coverage and final admission are owned by the caller.
func readScopedUsageSummary(ctx context.Context, tx *sql.Tx, selected intervalMemberSelection, scope StatsOptions, session string, read readScopedRequests) (jointUsageSummary, error) {
	if tx == nil || read == nil {
		return jointUsageSummary{}, errors.New("missing admitted usage snapshot or request reader")
	}
	intervals, err := readScopedIntervals(ctx, tx, selected, scope, session)
	if err != nil {
		return jointUsageSummary{}, err
	}
	window := scopedRequestWindow{from: scope.From, to: scope.To, client: scope.Client, session: session}
	events, resolver, err := read(ctx, tx, window)
	if err != nil {
		return jointUsageSummary{}, err
	}
	if events == nil {
		return jointUsageSummary{}, errors.New("admitted request set unavailable")
	}
	var price eventPricing
	if len(events) > 0 {
		if resolver == nil {
			return jointUsageSummary{}, errors.New("admitted request pricing inputs unavailable")
		}
		price = resolver.priceForEvent
	}
	seen := make(map[string]struct{}, len(events))
	for _, e := range events {
		if _, duplicate := seen[e.Key]; e.Key == "" || duplicate {
			return jointUsageSummary{}, errors.New("invalid admitted request key")
		}
		seen[e.Key] = struct{}{}
	}
	return summarizeJointContributions(events, intervals, scope, session, price)
}
