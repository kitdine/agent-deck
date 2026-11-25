package usage

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
)

// Task 8 pins this pair in the borrowed core transaction. A range stamp is
// keyed by epoch, typed scope and window; its value is the last intersecting
// publication ticket, which may precede the pinned head.
type intervalStampHead struct {
	coreEpoch, ticket int64
}

func intervalStampScopeKind(scope intervalImpactScope) (store.IntervalDirtyScopeKind, error) {
	if scope.kind == impactAllSummary {
		if scope.client == "" && scope.model == "" {
			return store.IntervalDirtyAllSummary, nil
		}
	} else if scope.client != "" && (scope.kind == impactModel || scope.model == "") {
		switch scope.kind {
		case impactAggregateOnly:
			return store.IntervalDirtyAggregateOnly, nil
		case impactModel:
			return store.IntervalDirtyModel, nil
		case impactClientSummary:
			return store.IntervalDirtyClientSummary, nil
		case impactCoverage:
			return store.IntervalDirtyCoverage, nil
		}
	}
	return "", errors.New("invalid interval stamp scope")
}

// A read window may start or end within a day. An exclusive midnight end
// excludes that entire day; one nanosecond later includes it.
func readIntervalStamp(ctx context.Context, tx *sql.Tx, h intervalStampHead, scope intervalImpactScope, from, to time.Time) (int64, error) {
	if !from.Before(to) {
		return 0, errors.New("invalid interval stamp window")
	}
	kind, err := intervalStampScopeKind(scope)
	if err != nil {
		return 0, err
	}
	first, _ := store.UsageIntervalStampDay(from)
	end, midnight := store.UsageIntervalStampDay(to)
	if !midnight {
		end++
	}
	return store.ReadUsageIntervalDirtyStamp(ctx, tx, h.coreEpoch, h.ticket, kind, scope.client, scope.model, first, end)
}

// Called after the authoritative head CAS in its still-open transaction.
// The publisher must roll back that entire transaction on any error, including
// failures after a preceding range was written. It alone commits on success.
func writeIntervalStamps(ctx context.Context, tx *sql.Tx, h intervalStampHead, plan intervalImpactPlan) error {
	if plan == nil {
		return errors.New("interval impact plan unavailable")
	}
	ranges := make([]store.IntervalDirtyRange, 0, len(plan))
	for _, r := range plan {
		kind, err := intervalStampScopeKind(r.scope)
		if err != nil {
			return err
		}
		// The current planner emits Codex and All; an emitted model requires
		// a nonempty whole-interval witness. Empty-model lookups remain valid.
		if r.scope.kind != impactAllSummary && r.scope.client != "codex" || r.scope.kind == impactModel && r.scope.model == "" {
			return errors.New("scope cannot be produced by the interval impact planner")
		}
		first, startMidnight := store.UsageIntervalStampDay(r.from)
		end, endMidnight := store.UsageIntervalStampDay(r.to)
		// The day containing an event can begin at Go's zero instant.
		if !startMidnight || !endMidnight || end <= first {
			return errors.New("interval stamp plan requires increasing UTC day bounds")
		}
		ranges = append(ranges, store.IntervalDirtyRange{
			ScopeKind: kind,
			Client:    r.scope.client,
			Model:     r.scope.model,
			FromDay:   first,
			ToDay:     end,
		})
	}
	return store.WriteUsageIntervalDirtyRanges(ctx, tx, h.coreEpoch, h.ticket, ranges)
}
