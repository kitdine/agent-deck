package usage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"
)

// CJSIPresentationWindow is the full ProjectedPresentation input window,
// including the daily series outside the named today/7d/30d periods.
func CJSIPresentationWindow(now time.Time, loc *time.Location) (time.Time, time.Time) {
	if loc == nil {
		loc = time.Local
	}
	today := localDateStart(now, loc)
	return today.AddDate(0, 0, 1-presentationDailyLimit), today.AddDate(0, 0, 1)
}

type aggregateZoneSpan struct {
	Name   string
	Offset int
	Until  int64
}

func locationWindowSpans(loc *time.Location, from, to time.Time) []aggregateZoneSpan {
	var spans []aggregateZoneSpan
	cursor := from.UTC()
	end := to.UTC()
	for step := 0; step < 10000 && !cursor.After(end); step++ {
		local := cursor.In(loc)
		name, offset := local.Zone()
		_, zoneEnd := local.ZoneBounds()
		until := end
		advance := false
		if !zoneEnd.IsZero() && zoneEnd.After(cursor) && !zoneEnd.After(end) {
			until = zoneEnd.UTC()
			advance = true
		}
		spans = append(spans, aggregateZoneSpan{Name: name, Offset: offset, Until: until.UnixNano()})
		if !advance {
			break
		}
		cursor = until
		if cursor.Equal(end) {
			endName, endOffset := end.In(loc).Zone()
			if endName != name || endOffset != offset {
				spans = append(spans, aggregateZoneSpan{Name: endName, Offset: endOffset, Until: end.UnixNano()})
			}
			break
		}
	}
	return spans
}

func parseFixedUTCOffsetLabel(label string) (int, bool) {
	if !strings.HasPrefix(label, "UTC") || len(label) != 9 {
		return 0, false
	}
	sign := 1
	switch label[3] {
	case '+':
	case '-':
		sign = -1
	default:
		return 0, false
	}
	if label[6] != ':' {
		return 0, false
	}
	hours, errH := strconv.Atoi(label[4:6])
	mins, errM := strconv.Atoi(label[7:9])
	if errH != nil || errM != nil || hours < 0 || hours > 23 || mins < 0 || mins > 59 {
		return 0, false
	}
	return sign * (hours*3600 + mins*60), true
}

func resolveAggregateTimezone(q StatsOptions) (displayZone, canonicalZone string, spans []aggregateZoneSpan, err error) {
	if q.Location == nil || q.Location.String() == "" {
		return "", "", nil, errors.New("missing aggregate timezone location")
	}
	locName := q.Location.String()
	if locName != "Local" {
		if q.Timezone != "" && q.Timezone != locName {
			return "", "", nil, errors.New("conflicting aggregate timezones")
		}
		return locName, locName, locationWindowSpans(q.Location, q.From, q.To), nil
	}

	// q.Location is time.Local (e.g. TZ=:/etc/localtime). Confirm that any
	// caller-supplied CLI display timezone agrees with q.Location's actual
	// offset/transition behavior across [From, To], and bind those transitions
	// into the cache stamp so host timezone changes invalidate reuse. Accept a
	// fixed-offset label only if the report window uses that offset.
	localSpans := locationWindowSpans(q.Location, q.From, q.To)
	if q.Timezone == "" || q.Timezone == "Local" {
		return "Local", "Local", localSpans, nil
	}
	if loaded, loadErr := time.LoadLocation(q.Timezone); loadErr == nil {
		loadedSpans := locationWindowSpans(loaded, q.From, q.To)
		if !slices.Equal(localSpans, loadedSpans) {
			return "", "", nil, errors.New("conflicting aggregate timezones")
		}
		return q.Timezone, q.Timezone, localSpans, nil
	}
	if offset, ok := parseFixedUTCOffsetLabel(q.Timezone); ok {
		for _, span := range localSpans {
			if span.Offset == offset {
				return q.Timezone, "Local", localSpans, nil
			}
		}
	}
	return "", "", nil, errors.New("conflicting aggregate timezones")
}

// ReadCJSIAggregateCacheStamp binds the selected token and coverage
// dependencies only. Callers also bind the report shape, session selection,
// request/work facts and price inputs. coverage identifies independently
// admitted factual coverage for this window, including partial or known empty.
// A successful MAX over no stamp rows supplies no such proof by itself.
func ReadCJSIAggregateCacheStamp(ctx context.Context, tx *sql.Tx, epoch, ticket int64, q StatsOptions, coverage [32]byte) ([32]byte, error) {
	return aggregateCacheStamp(epoch, q, coverage, func(s intervalImpactScope) (int64, error) {
		return readIntervalStamp(ctx, tx, intervalStampHead{epoch, ticket}, s, q.From, q.To)
	})
}

func aggregateCacheStamp(epoch int64, q StatsOptions, coverage [32]byte, read func(intervalImpactScope) (int64, error)) ([32]byte, error) {
	if !q.From.Before(q.To) || coverage == ([32]byte{}) || (q.Client != "" && q.Client != "codex" && q.Client != "claude") {
		return [32]byte{}, errors.New("missing aggregate coverage or invalid scope")
	}
	displayZone, canonicalZone, spans, err := resolveAggregateTimezone(q)
	if err != nil {
		return [32]byte{}, err
	}
	client := q.Client
	summary := intervalImpactScope{kind: impactClientSummary, client: client}
	if client == "" {
		summary = intervalImpactScope{kind: impactAllSummary}
		client = "codex" // CJSI intervals and their coverage belong to Codex.
	}
	// Even a model/provider slice uses the unfiltered token denominator.
	total, err := read(summary)
	if err != nil {
		return [32]byte{}, err
	}
	cov, err := read(intervalImpactScope{kind: impactCoverage, client: client})
	if err != nil {
		return [32]byte{}, err
	}
	var model int64
	if q.Model != "" {
		model, err = read(intervalImpactScope{kind: impactModel, client: client, model: q.Model})
		if err != nil {
			return [32]byte{}, err
		}
	}
	key, err := json.Marshal(struct {
		Epoch, Summary, ModelStamp, CoverageStamp      int64
		Client, Model, Provider, Timezone, CanonicalTZ string
		ZoneSpans                                      []aggregateZoneSpan
		From, To                                       time.Time
		FactualCoverage                                [32]byte
	}{epoch, total, model, cov, q.Client, q.Model, q.Provider, displayZone, canonicalZone, spans, q.From.UTC(), q.To.UTC(), coverage})
	if err != nil {
		return [32]byte{}, err
	}
	return sha256.Sum256(key), nil
}
