package usage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
)

// A partition retains pricing contributions, not transcript material. Reducers
// still own their distinct sets, medians, peaks and top rankings; cached daily
// averages or medians are never added together.
const projectedEventAlgorithmVersion = 1

type projectedEvent struct {
	Event         Event  `json:"event"`
	Quality       string `json:"quality"`
	Reason        string `json:"reason"`
	Provider      string `json:"provider"`
	SpendEligible bool   `json:"spend_eligible"`
	Result        Result `json:"result"`
	PricingError  string `json:"pricing_error,omitempty"`
}

type projectedEventPartition struct {
	Events   []projectedEvent `json:"events"`
	Checksum string           `json:"checksum"`
}

type projectedPricing func(storedEvent) (eventAttribution, Result, error)

func (s *Service) presentationInputs(ctx context.Context, from, to, now time.Time, incremental bool) ([]storedEvent, projectedPricing, error) {
	var metadataRevision int64
	if incremental && !s.Store.ReadOnly() {
		if err := s.Store.FlushUsagePartitionInvalidations(ctx); err != nil {
			return nil, nil, err
		}
		var err error
		metadataRevision, err = s.Store.UsageProjectionMetadataRevision(ctx)
		if err != nil {
			return nil, nil, err
		}
	}
	var fullEvents []storedEvent
	if !incremental || s.Store.ReadOnly() {
		var err error
		fullEvents, err = s.eventsRange(ctx, from, to, "", "")
		if err != nil {
			return nil, nil, err
		}
	}
	resolver, err := s.loadReadPriceResolver(ctx, now.UTC())
	if err != nil {
		return nil, nil, err
	}
	fullPricing := func(event storedEvent) (eventAttribution, Result, error) {
		a, err := resolver.priceForEvent(event)
		if err != nil {
			return a, Result{}, err
		}
		r, err := calculateAttributedEvent(event, a)
		return a, r, err
	}
	if !incremental || s.Store.ReadOnly() {
		return fullEvents, fullPricing, nil
	}
	defer s.Profile.start("projected_inputs")()
	generation, err := s.Store.DerivedSnapshotGeneration(ctx)
	if err != nil {
		return nil, nil, err
	}
	partitions, err := s.Store.UsagePartitions(ctx, from, to)
	if err != nil {
		return nil, nil, err
	}
	byKey := map[string]projectedEvent{}
	events := []storedEvent{}
	for index, p := range partitions {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		current, found := resolver.priceAt(p.Client, p.Model, now.UTC())
		identity, _ := json.Marshal(struct {
			Algorithm, Parser int
			Epoch             int64
			Current           modelPrice
			Found             bool
		}{projectedEventAlgorithmVersion, ParserVersion, generation.Epoch, current, found})
		digest := sha256.Sum256(identity)
		version := hex.EncodeToString(digest[:])
		var values []projectedEvent
		if !p.Dirty && p.ActiveRevision == p.Revision && p.Version == version {
			var cached projectedEventPartition
			if json.Unmarshal(p.Payload, &cached) == nil {
				bytes, _ := json.Marshal(cached.Events)
				sum := sha256.Sum256(bytes)
				if cached.Checksum == hex.EncodeToString(sum[:]) && validProjectedEvents(cached.Events, p) {
					values = cached.Events
				}
			}
		}
		if values == nil {
			revision, err := s.Store.BeginUsagePartition(ctx, p)
			if err != nil {
				return nil, nil, err
			}
			start, err := time.Parse("2006-01-02", p.Day)
			if err != nil {
				return nil, nil, err
			}
			inputs, err := s.eventsRangeFiltered(ctx, start, start.AddDate(0, 0, 1), p.Client, p.Model, true)
			if err != nil {
				return nil, nil, err
			}
			values = make([]projectedEvent, 0, len(inputs))
			for _, event := range inputs {
				a, r, err := fullPricing(event)
				pricingError := ""
				if err != nil {
					pricingError = err.Error()
				}
				contribution := event.Event
				contribution.SourcePath = ""
				contribution.SourceOffset = 0
				r.Tokens = nil // reducers use the event's canonical token components
				values = append(values, projectedEvent{Event: contribution, Quality: a.quality, Reason: a.reason, Provider: a.provider, SpendEligible: a.spendEligible, Result: r, PricingError: pricingError})
			}
			bytes, err := json.Marshal(values)
			if err != nil {
				return nil, nil, err
			}
			sum := sha256.Sum256(bytes)
			payload, err := json.Marshal(projectedEventPartition{Events: values, Checksum: hex.EncodeToString(sum[:])})
			if err != nil {
				return nil, nil, err
			}
			// Oversized partitions remain correct but uncached; their full oracle
			// inputs never become a partially written active payload.
			if len(payload) <= 8<<20 {
				if err := s.Store.PublishUsagePartition(ctx, p, revision, metadataRevision, version, payload); err != nil {
					return nil, nil, err
				}
			}
			partitions[index].Revision = revision
		}
		for _, value := range values {
			// Match the full SQL reader's textual half-open range exactly.
			if value.Event.EventAt < from.UTC().Format(time.RFC3339Nano) || value.Event.EventAt >= to.UTC().Format(time.RFC3339Nano) {
				continue
			}
			if _, exists := byKey[value.Event.Key]; exists {
				return nil, nil, fmt.Errorf("duplicate projected event identity")
			}
			byKey[value.Event.Key] = value
			events = append(events, storedEvent{Event: value.Event})
		}
	}
	if err := s.Store.CheckUsagePartitions(ctx, from, to, partitions); err != nil {
		return nil, nil, err
	}
	if err := s.Store.CheckUsageProjectionMetadata(ctx, metadataRevision); err != nil {
		return nil, nil, err
	}

	sort.Slice(events, func(i, j int) bool {
		if events[i].EventAt != events[j].EventAt {
			return events[i].EventAt < events[j].EventAt
		}
		return events[i].Key < events[j].Key
	})
	pricing := func(event storedEvent) (eventAttribution, Result, error) {
		v, ok := byKey[event.Key]
		if !ok {
			return eventAttribution{}, Result{}, fmt.Errorf("missing projected event identity")
		}
		// UTC partitions may extend past a local-day or caller filter boundary.
		// Only a selected event may surface its deterministic pricing error.
		if v.PricingError != "" {
			return eventAttribution{}, Result{}, errors.New(v.PricingError)
		}
		return eventAttribution{quality: v.Quality, reason: v.Reason, provider: v.Provider, spendEligible: v.SpendEligible}, v.Result, nil
	}
	return events, pricing, nil
}

func validProjectedEvents(events []projectedEvent, p store.UsagePartition) bool {
	if events == nil {
		return false
	}
	seen := map[string]bool{}
	for _, v := range events {
		if v.Event.Client != p.Client || v.Event.Model != p.Model || len(v.Event.EventAt) < 10 || v.Event.EventAt[:10] != p.Day || seen[v.Event.Key] {
			return false
		}
		seen[v.Event.Key] = true
	}
	return true
}
