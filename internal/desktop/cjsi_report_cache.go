package desktop

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/kitdine/agent-deck/internal/usage"
)

// CJSIReportCacheInputs identifies all non-report dependencies proved by the
// caller's admission. Business time is taken from the borrowed report view.
type CJSIReportCacheInputs struct {
	CoreEpoch, Ticket                                     int64
	Database, Publisher                                   string
	Schema, UsageParser, SessionParser                    int
	Facts, Sessions, Protection, Coverage, Prices, Routes [32]byte
	Location                                              *time.Location
	ValidUntil                                            time.Time
}

type CJSIReportCache struct {
	key      cjsiDerivedCacheKey
	payload  derivedSnapshotCachePayload
	from, to time.Time
	reused   []byte
}

// PrepareCJSIReportCache only collects detached report inputs. Call Encode
// after the outer core/index borrow has returned successfully.
func PrepareCJSIReportCache(ctx context.Context, r *usage.PublishedReports, in CJSIReportCacheInputs, prior []byte) (CJSIReportCache, error) {
	var out CJSIReportCache
	if r == nil {
		return out, errors.New("missing borrowed reports")
	}
	epoch, ticket, clock := r.Publication()
	if epoch != in.CoreEpoch || ticket != in.Ticket {
		return out, errors.New("desktop report selection changed")
	}
	view := cjsiDerivedView{epoch, ticket, in.Database, in.Schema, WireVersion, in.UsageParser, in.SessionParser, cjsiDerivedProof{in.Publisher, in.Facts, in.Sessions, in.Protection, in.Coverage, in.Prices, in.Routes}}
	key, err := view.cacheKey(clock, in.ValidUntil, in.Location, func(q usage.StatsOptions, c [32]byte) ([32]byte, error) { return r.AggregateCacheStamp(ctx, q, c) })
	if err != nil {
		return out, err
	}
	out.key = key
	out.from, out.to = localDay(clock, in.Location)
	if p, ok := key.reuse(prior, clock); ok {
		out.payload = p
		out.reused = slices.Clone(prior)
		return out, nil
	}
	p, err := r.Presentation(ctx, in.Location, false)
	if err != nil {
		return CJSIReportCache{}, err
	}
	signals, err := buildWorkSignalsSnapshot(ctx, clock, in.Location, r.SignalsBatch)
	if err != nil {
		return CJSIReportCache{}, err
	}
	out.payload = cjsiDerivedSnapshotPayload(p, signals)
	return out, nil
}

func (c CJSIReportCache) Snapshots() (UsageSnapshot, WorkSignalsSnapshot) {
	return usageSnapshotFromPresentation(c.payload.Presentation, c.from, c.to), c.payload.WorkSignals
}

func (c CJSIReportCache) Encode() ([]byte, error) {
	if c.reused != nil {
		return slices.Clone(c.reused), nil
	}
	return c.key.encode(c.payload)
}
