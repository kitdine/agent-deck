package desktop

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/kitdine/agent-deck/internal/usage"
)

// Identities supplied by admission over the effective views, not stamps
// inferred from the presence of the cache. Facts includes all request, work
// classification, workflow and tooling inputs; Sessions includes the selected
// generation and membership. Zero means unavailable, even for empty results.
type cjsiDerivedProof struct {
	Publisher                                             string
	Facts, Sessions, Protection, Coverage, Prices, Routes [32]byte
}

type cjsiDerivedView struct {
	epoch, ticket                            int64
	database                                 string
	schema, wire, usageParser, sessionParser int
	proof                                    cjsiDerivedProof
}

// The v2 clock contract rejects entries encoded with the old civil-time deadline.
const cjsiDerivedCacheAlgorithmVersion = 2

type cjsiDerivedCacheKey struct{ header derivedSnapshotCacheHeader }

// validUntil comes from the admitted view's price, route and other validity
// boundaries. The clock boundary also limits reuse of date/hour projections.
func newCJSIDerivedCacheKey(ctx context.Context, tx *sql.Tx, v cjsiDerivedView, now, validUntil time.Time, loc *time.Location) (cjsiDerivedCacheKey, error) {
	d := v.proof
	zero := [32]byte{}
	if loc == nil || loc.String() == "" || now.IsZero() || !now.Before(validUntil) || v.database == "" ||
		v.schema <= 0 || v.wire <= 0 || v.usageParser <= 0 || v.sessionParser <= 0 || d.Publisher == "" ||
		d.Facts == zero || d.Sessions == zero || d.Protection == zero || d.Coverage == zero || d.Prices == zero || d.Routes == zero {
		return cjsiDerivedCacheKey{}, errors.New("incomplete admitted desktop dependencies")
	}
	from, end := usage.CJSIPresentationWindow(now, loc)
	stamp, err := usage.ReadCJSIAggregateCacheStamp(ctx, tx, v.epoch, v.ticket, usage.StatsOptions{From: from, To: end, Timezone: loc.String(), Location: loc}, d.Coverage)
	if err != nil {
		return cjsiDerivedCacheKey{}, err
	}
	// All/Codex/Claude presentation, including compact rows, is cached as
	// one document. Every interval change marks All; the full 90 local days
	// contain every period. Summary uses today. WorkSignals uses the named
	// periods below and remains entirely request/work-fact based.
	type period struct {
		Name     string
		From, To time.Time
	}
	var periods []period
	for _, p := range desktopPeriodRanges(now, loc) {
		periods = append(periods, period{p.name, p.start.UTC(), p.end.UTC()})
	}
	key, err := json.Marshal(struct {
		Proof        cjsiDerivedProof
		Presentation [32]byte
		Periods      []period
	}{d, stamp, periods})
	if err != nil {
		return cjsiDerivedCacheKey{}, err
	}
	sum := sha256.Sum256(key)
	today, tomorrow := localDay(now, loc)
	if boundary := cjsiDerivedCacheClockBoundary(now, loc, tomorrow); boundary.Before(validUntil) {
		validUntil = boundary
	}
	if !now.Before(validUntil) {
		return cjsiDerivedCacheKey{}, errors.New("expired desktop clock boundary")
	}
	return cjsiDerivedCacheKey{derivedSnapshotCacheHeader{
		Version: 2, AlgorithmVersion: cjsiDerivedCacheAlgorithmVersion, WireVersion: v.wire, SchemaVersion: v.schema,
		DatabaseIdentity: v.database, Epoch: v.epoch, UsageParserVersion: v.usageParser, SessionParserVersion: v.sessionParser,
		Timezone: loc.String(), WindowStart: today.UTC().Format(time.RFC3339Nano), WindowEnd: tomorrow.UTC().Format(time.RFC3339Nano),
		CreatedAt: now.UTC().Format(time.RFC3339Nano), ValidUntil: validUntil.UTC().Format(time.RFC3339Nano), Dependencies: hex.EncodeToString(sum[:]),
	}}, nil
}

// Advance to the next local-hour boundary under the current offset.
// A civil-time reconstruction can pick the later occurrence of a folded hour.
func cjsiDerivedCacheClockBoundary(now time.Time, loc *time.Location, nextDay time.Time) time.Time {
	local := now.In(loc)
	part := time.Duration(local.Minute())*time.Minute +
		time.Duration(local.Second())*time.Second + time.Duration(local.Nanosecond())
	next := now.Add(time.Hour - part)
	// Only future offset transitions may truncate this deadline; extrapolated
	// far-future rules can report a ZoneBounds end in the past.
	_, zoneEnd := local.ZoneBounds()
	if !zoneEnd.IsZero() && zoneEnd.After(now) && zoneEnd.Before(next) {
		next = zoneEnd
	}
	if nextDay.Before(next) {
		return nextDay
	}
	return next
}

func cjsiDerivedSnapshotPayload(presentation usage.PresentationReport, workSignals WorkSignalsSnapshot) derivedSnapshotCachePayload {
	return derivedSnapshotCachePayload{
		Presentation: presentation,
		Summary:      presentation.Summary,
		WorkSignals:  workSignals,
		PriceAvailability: derivedSnapshotPriceAvailability{
			Validated:          true,
			PricingComplete:    summaryPricingComplete(presentation.Summary),
			UnpricedComponents: len(presentation.Summary.Unpriced),
		},
	}
}

func validCJSIDerivedPayload(p derivedSnapshotCachePayload) bool {
	return validDerivedSnapshotCachePayload(p) && p.Presentation.Scopes != nil && p.Presentation.ClientSubtotals.Items != nil && p.PriceAvailability.UnpricedComponents == len(p.Summary.Unpriced) &&
		(!p.PriceAvailability.PricingComplete || (p.Summary.CatalogBaseCost != nil && p.Summary.ProviderCost != nil && len(p.Summary.Unpriced) == 0))
}

func (k cjsiDerivedCacheKey) encode(p derivedSnapshotCachePayload) ([]byte, error) {
	if k.header.Version != 2 || k.header.AlgorithmVersion != cjsiDerivedCacheAlgorithmVersion || k.header.Dependencies == "" || !validCJSIDerivedPayload(p) {
		return nil, errors.New("unavailable derived cache inputs or payload")
	}
	return encodeDerivedSnapshotCache(k.header, p)
}

// Pass admitted slot bytes or the result of readDerivedSnapshotCacheBytes.
// A miss returns no usable payload. Task 8 rebuilds from admitted facts,
// retains its factual coverage status, and rechecks dependencies before use.
func (k cjsiDerivedCacheKey) reuse(contents []byte, now time.Time) (derivedSnapshotCachePayload, bool) {
	if k.header.Version != 2 || k.header.AlgorithmVersion != cjsiDerivedCacheAlgorithmVersion || k.header.Dependencies == "" {
		return derivedSnapshotCachePayload{}, false
	}
	observed, err := time.Parse(time.RFC3339Nano, k.header.CreatedAt)
	if err != nil || now.Before(observed) {
		return derivedSnapshotCachePayload{}, false
	}
	doc, err := decodeDerivedSnapshotCache(contents)
	if err != nil || !derivedSnapshotCacheHeaderMatches(doc.Header, k.header, now) || parseDerivedSnapshotCacheValidUntil(doc.Header).After(parseDerivedSnapshotCacheValidUntil(k.header)) || !validCJSIDerivedPayload(doc.Payload) {
		return derivedSnapshotCachePayload{}, false
	}
	doc.Payload.Presentation.Summary = doc.Payload.Summary // excluded from report JSON
	return doc.Payload, true
}
