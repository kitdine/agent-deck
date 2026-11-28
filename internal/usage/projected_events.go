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

	"github.com/kitdine/agent-deck/internal/perftrace"
	"github.com/kitdine/agent-deck/internal/store"
)

// A partition retains pricing contributions, not transcript material. Reducers
// still own their distinct sets, medians, peaks and top rankings; cached daily
// averages or medians are never added together.
const projectedEventAlgorithmVersion = 3

type projectedEvent struct {
	Pricing       *PricingReport `json:"request_pricing"`
	Event         Event          `json:"event"`
	Quality       string         `json:"quality"`
	Reason        string         `json:"reason"`
	Provider      string         `json:"provider"`
	SpendEligible bool           `json:"spend_eligible"`
	Result        Result         `json:"result"`
	PricingError  string         `json:"pricing_error,omitempty"`
}

type projectedEventPartition struct {
	Events   []projectedEvent `json:"events"`
	Checksum string           `json:"checksum"`
}

type projectedPricing func(storedEvent) (eventAttribution, Result, *PricingReport, error)

type presentationPricingWindow struct {
	RuleSet string
	Start   time.Time
	End     time.Time
}

func (s *Service) presentationInputs(ctx context.Context, from, to, now time.Time, incremental bool) ([]storedEvent, projectedPricing, presentationPricingWindow, error) {
	if incremental && !s.Store.ReadOnly() {
		if err := s.Store.FlushUsagePartitionInvalidations(ctx); err != nil {
			return nil, nil, presentationPricingWindow{}, err
		}
	}
	read, err := s.openPricingRead(ctx, now.UTC())
	if err != nil {
		return nil, nil, presentationPricingWindow{}, err
	}
	defer read.tx.Rollback()
	window := read.rules.presentationWindow()
	if !incremental || s.Store.ReadOnly() {
		s.eventsRangeReads.Add(1)
		events := []storedEvent{}
		err := read.visit(ctx, InvocationPricingFilter{From: from, To: to}, func(event storedEvent, _ time.Time) error {
			events = append(events, event)
			return nil
		})
		if err != nil {
			return nil, nil, window, err
		}
		fullPricing := func(event storedEvent) (eventAttribution, Result, *PricingReport, error) {
			at, err := time.Parse(time.RFC3339Nano, event.EventAt)
			if err != nil {
				return eventAttribution{}, Result{}, nil, err
			}
			a, p, err := read.price(event, at)
			if err != nil {
				return a, Result{}, nil, err
			}
			return a, p.Legacy, pricingContribution(p), legacyPricingError(p)
		}
		return events, fullPricing, window, read.tx.Commit()
	}
	defer perftrace.Span(perftrace.UsageProjectedInputs)()
	generation, err := s.Store.DerivedSnapshotGeneration(ctx)
	if err != nil {
		return nil, nil, window, err
	}
	partitions, err := s.Store.UsagePartitions(ctx, from, to)
	if err != nil {
		return nil, nil, window, err
	}
	byKey := map[string]projectedEvent{}
	events := []storedEvent{}
	collect := func(values []projectedEvent) error {
		for _, value := range values {
			at, err := time.Parse(time.RFC3339Nano, value.Event.EventAt)
			if err != nil {
				return err
			}
			if at.Before(from) || !at.Before(to) {
				continue
			}
			if _, exists := byKey[value.Event.Key]; exists {
				return fmt.Errorf("duplicate projected event identity")
			}
			byKey[value.Event.Key] = value
			events = append(events, storedEvent{Event: value.Event})
		}
		return nil
	}
	type partitionRebuild struct {
		index   int
		version string
	}
	var rebuilds []partitionRebuild
	for index, p := range partitions {
		if err := ctx.Err(); err != nil {
			return nil, nil, window, err
		}
		current, found := read.resolver.priceAt(p.Client, p.Model, now.UTC())
		identity, _ := json.Marshal(struct {
			Algorithm, Parser int
			Epoch             int64
			Current           modelPrice
			Found             bool
			RuleSet           string
			WindowStart       time.Time
		}{projectedEventAlgorithmVersion, ParserVersion, generation.Epoch, current, found, window.RuleSet, window.Start})
		digest := sha256.Sum256(identity)
		version := hex.EncodeToString(digest[:])
		var values []projectedEvent
		if !p.Dirty && p.ActiveRevision == p.Revision && p.Version == version {
			var cached projectedEventPartition
			if json.Unmarshal(p.Payload, &cached) == nil {
				bytes, _ := json.Marshal(cached.Events)
				sum := sha256.Sum256(bytes)
				if cached.Checksum == hex.EncodeToString(sum[:]) && validProjectedEvents(cached.Events, p, now) {
					values = cached.Events
				}
			}
		}
		if values != nil {
			if err := collect(values); err != nil {
				return nil, nil, window, err
			}
			continue
		}
		revision, err := s.Store.BeginUsagePartition(ctx, p)
		if err != nil {
			return nil, nil, window, err
		}
		partitions[index].Revision = revision
		rebuilds = append(rebuilds, partitionRebuild{index: index, version: version})
	}
	if len(rebuilds) > 0 {
		// A dirty partition coalesces writes without bumping its revision.
		// Open the event snapshot after every Begin has made its partition
		// clean, so any later write invalidates this build's revision.
		revision := read.revision
		if err := read.tx.Rollback(); err != nil {
			return nil, nil, window, err
		}
		read, err = s.openPricingRead(ctx, now.UTC())
		if err != nil {
			return nil, nil, window, err
		}
		defer read.tx.Rollback()
		if read.revision != revision {
			return nil, nil, window, store.ErrUsagePartitionChanged
		}
	}
	for _, rebuild := range rebuilds {
		p := partitions[rebuild.index]
		start, err := time.Parse("2006-01-02", p.Day)
		if err != nil {
			return nil, nil, window, err
		}
		s.eventsRangeReads.Add(1)
		values := []projectedEvent{}
		hasCutoffEvent := false
		filter := InvocationPricingFilter{From: start, To: start.AddDate(0, 0, 1), Client: p.Client, Model: p.Model}
		err = read.visitPage(ctx, filter, 0, 0, true, func(event storedEvent, at time.Time) error {
			hasCutoffEvent = hasCutoffEvent || at.Equal(now)
			a, priced, err := read.price(event, at)
			if err != nil {
				return err
			}
			contribution := event.Event
			contribution.SourcePath, contribution.SourceOffset, contribution.Facts = "", 0, nil
			legacy := priced.Legacy
			legacy.Tokens = nil // reducers use the event's canonical token components
			values = append(values, projectedEvent{
				Event: contribution, Quality: a.quality, Reason: a.reason, Provider: a.provider,
				SpendEligible: a.spendEligible, Result: legacy, PricingError: priced.LegacyError, Pricing: pricingContribution(priced),
			})
			return nil
		})
		if err != nil {
			return nil, nil, window, err
		}
		bytes, err := json.Marshal(values)
		if err != nil {
			return nil, nil, window, err
		}
		sum := sha256.Sum256(bytes)
		payload, err := json.Marshal(projectedEventPartition{Events: values, Checksum: hex.EncodeToString(sum[:])})
		if err != nil {
			return nil, nil, window, err
		}
		// Oversized partitions remain correct but uncached; their full oracle
		// inputs never become a partially written active payload.
		if !hasCutoffEvent && len(payload) <= 8<<20 {
			if err := s.Store.PublishUsagePartition(ctx, p, p.Revision, read.revision, rebuild.version, payload); err != nil {
				return nil, nil, window, err
			}
		}
		if err := collect(values); err != nil {
			return nil, nil, window, err
		}
	}
	if err := s.Store.CheckUsagePartitions(ctx, from, to, partitions); err != nil {
		return nil, nil, window, err
	}
	if err := s.Store.CheckUsageProjectionMetadata(ctx, read.revision); err != nil {
		return nil, nil, window, err
	}

	sort.Slice(events, func(i, j int) bool {
		a, _ := time.Parse(time.RFC3339Nano, events[i].EventAt)
		b, _ := time.Parse(time.RFC3339Nano, events[j].EventAt)
		if !a.Equal(b) {
			return a.Before(b)
		}
		return events[i].Key < events[j].Key
	})
	pricing := func(event storedEvent) (eventAttribution, Result, *PricingReport, error) {
		v, ok := byKey[event.Key]
		if !ok {
			return eventAttribution{}, Result{}, nil, fmt.Errorf("missing projected event identity")
		}
		// UTC partitions may extend past a local-day or caller filter boundary.
		// Only a selected event may surface its deterministic pricing error.
		if v.PricingError != "" {
			return eventAttribution{}, Result{}, nil, errors.New(v.PricingError)
		}
		return eventAttribution{quality: v.Quality, reason: v.Reason, provider: v.Provider, spendEligible: v.SpendEligible}, v.Result, v.Pricing, nil
	}
	return events, pricing, window, read.tx.Commit()
}

func validProjectedEvents(events []projectedEvent, p store.UsagePartition, now time.Time) bool {
	if events == nil {
		return false
	}
	seen := map[string]bool{}
	for _, v := range events {
		if v.Event.Client != p.Client || v.Event.Model != p.Model || len(v.Event.EventAt) < 10 || v.Event.EventAt[:10] != p.Day || seen[v.Event.Key] {
			return false
		}
		at, err := time.Parse(time.RFC3339Nano, v.Event.EventAt)
		if err != nil || at.Equal(now) || v.Pricing == nil {
			return false
		}
		seen[v.Event.Key] = true
	}
	return true
}
