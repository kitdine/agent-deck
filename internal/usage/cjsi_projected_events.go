package usage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
)

// Task 8 supplies the effective publisher/format identity, actual parser,
// pricing dependency revision and immutable price/route inputs for this
// partition in the admitted view. A Codex client label is not admission.
type cjsiProjectionView struct {
	head          intervalStampHead
	publisher     string
	parser        int
	priceRevision int64
	prices        *readPriceResolver
}

// Read all effective request rows for exactly client, model (including "")
// and [from,to) from the borrowed transaction, in legacy order. The caller
// applies ownership and protection and returns a non-nil slice on success.
type readCJSIPartitionRequests func(ctx context.Context, tx *sql.Tx, client, model string, from, to time.Time) ([]storedEvent, error)

type cjsiRequestPartition struct {
	events  []projectedEvent
	version string
	payload []byte // nil for an oversized rebuild
}

// p and the price inputs must come from the same admitted core view. Reuse
// requires authority for the entire partition at that revision, including
// current ownership and protection. A scoped or masked subset cannot use a
// whole-partition cache. The caller owns final checks and persistence.
func readCJSIRequestPartition(ctx context.Context, tx *sql.Tx, v cjsiProjectionView, p store.UsagePartition, read readCJSIPartitionRequests) (cjsiRequestPartition, error) {
	if p.Client != "codex" || p.Revision <= 0 || v.publisher == "" || v.parser <= 0 || v.priceRevision < 0 || v.prices == nil || v.prices.current.IsZero() {
		return cjsiRequestPartition{}, errors.New("invalid admitted request partition inputs")
	}
	start, err := time.Parse(time.DateOnly, p.Day)
	if err != nil {
		return cjsiRequestPartition{}, errors.New("invalid request partition day")
	}
	end := start.AddDate(0, 0, 1)
	modelStamp, err := readIntervalStamp(ctx, tx, v.head, intervalImpactScope{kind: impactModel, client: p.Client, model: p.Model}, start, end)
	if err != nil {
		return cjsiRequestPartition{}, err
	}
	coverageStamp, err := readIntervalStamp(ctx, tx, v.head, intervalImpactScope{kind: impactCoverage, client: p.Client}, start, end)
	if err != nil {
		return cjsiRequestPartition{}, err
	}

	// Coverage is the shared client/day dependency even for unattributed
	// intervals. An empty event model remains in the typed model namespace.
	current, found := v.prices.priceAt(p.Client, p.Model, v.prices.current)
	identity, err := json.Marshal(struct {
		Pricing, Publisher, Client, Model, Day                  string
		EventRevision, PriceRevision, ModelStamp, CoverageStamp int64
	}{projectedPriceVersion(v.parser, v.head.coreEpoch, current, found), v.publisher, p.Client, p.Model, p.Day,
		p.Revision, v.priceRevision, modelStamp, coverageStamp})
	if err != nil {
		return cjsiRequestPartition{}, err
	}
	digest := sha256.Sum256(identity)
	version := "v2:" + hex.EncodeToString(digest[:])
	if values := cachedProjectedEvents(p, version, version); values != nil && validCJSIRequestRows(values, start, end) {
		return cjsiRequestPartition{values, version, p.Payload}, nil
	}

	if read == nil {
		return cjsiRequestPartition{}, errors.New("admitted request partition reader unavailable")
	}
	inputs, err := read(ctx, tx, p.Client, p.Model, start, end)
	if err != nil {
		return cjsiRequestPartition{}, err
	}
	if inputs == nil {
		return cjsiRequestPartition{}, errors.New("admitted request partition unavailable")
	}
	values := projectRequestEvents(inputs, v.prices.projectPrice)
	// Warnings is omitempty in Result. Match the decoded representation when
	// returning a fresh projection, without changing the legacy cold path.
	for i := range values {
		if len(values[i].Result.Warnings) == 0 {
			values[i].Result.Warnings = nil
		}
	}
	if !validProjectedEvents(values, p) || !validCJSIRequestRows(values, start, end) {
		return cjsiRequestPartition{}, errors.New("invalid admitted request partition rows")
	}
	payload, err := encodeProjectedEvents(values, version)
	if err != nil {
		return cjsiRequestPartition{}, err
	}
	if len(payload) > 8<<20 {
		payload = nil
	}
	return cjsiRequestPartition{values, version, payload}, nil
}

// v1 membership checks the stored day prefix. In v2 also verify the actual
// UTC instant and require the sanitized request shape produced by the encoder.
func validCJSIRequestRows(values []projectedEvent, start, end time.Time) bool {
	for _, v := range values {
		at, err := time.Parse(time.RFC3339Nano, v.Event.EventAt)
		if err != nil || at.Before(start) || !at.Before(end) || v.Event.Key == "" ||
			v.Event.SourcePath != "" || v.Event.SourceOffset != 0 || v.Result.Tokens != nil {
			return false
		}
	}
	return true
}
