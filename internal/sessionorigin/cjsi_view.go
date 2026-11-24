package sessionorigin

import (
	"context"
	"errors"
	"time"

	"github.com/kitdine/agent-deck/internal/session"
	"github.com/kitdine/agent-deck/internal/store"
)

// EffectiveCopyReader is borrowed from an already admitted index view.
// It has no method to open a store, fall back to base rows, or read files.
type EffectiveCopyReader interface {
	EffectiveCopies(context.Context, string, string) ([]session.EffectiveSessionCopy, error)
	// SettledBirth is the published marker's first event, not its settlement
	// time. The reader must use the very same (old or proposed) core view.
	SettledBirth(context.Context, Target) (string, error)
}

// OriginView lends the reader and the corresponding core/query inputs inside
// one synchronous callback. The owner must check the ticket and protection
// again before returning nil. A callback result is provisional until then.
type OriginView[T any] func(func(EffectiveCopyReader, T) error) error

func readOriginView[T, R any](with OriginView[T], f func(EffectiveCopyReader, T) (R, error)) (R, error) {
	var zero, result R
	var called bool
	var readErr error
	if with == nil {
		return zero, ErrSessionsUnavailable
	}
	err := with(func(v EffectiveCopyReader, inputs T) error {
		if v == nil || called || readErr != nil {
			readErr = ErrSessionsUnavailable
			return readErr
		}
		called = true
		result, readErr = f(v, inputs)
		return readErr
	})
	if err != nil {
		return zero, err
	}
	if !called {
		return zero, ErrSessionsUnavailable
	}
	if readErr != nil {
		return zero, readErr
	}
	return result, nil
}

type effectiveTarget struct {
	indexed session.IndexedSession
	copies  []session.EffectiveSessionCopy
	birth   Birth
}

func (t *effectiveTarget) hasMarker() bool {
	for _, c := range t.copies {
		if session.IsTelemetrySource(c.Metadata.SourcePath) {
			return true
		}
	}
	return false
}

func instant(s string) string {
	at, err := time.Parse(time.RFC3339Nano, s)
	if err != nil || at.IsZero() {
		return ""
	}
	return at.UTC().Format(time.RFC3339Nano)
}

func readEffectiveTarget(ctx context.Context, view EffectiveCopyReader, target Target) (*effectiveTarget, error) {
	if err := validTarget(target.Client, target.SessionID); err != nil {
		return nil, err
	}
	copies, err := view.EffectiveCopies(ctx, target.Client, target.SessionID)
	if err != nil || len(copies) == 0 {
		return nil, err
	}
	return targetFromCopies(ctx, view, target, copies)
}

type settledBirthReader interface {
	SettledBirth(context.Context, Target) (string, error)
}

// copies are ordered by the admitted display winner, then eligible replicas.
func targetFromCopies(ctx context.Context, view settledBirthReader, target Target, copies []session.EffectiveSessionCopy) (*effectiveTarget, error) {
	var native, head bool
	for i, c := range copies {
		m := c.Metadata
		if m.Client != target.Client || m.SessionID != target.SessionID || m.SourcePath == "" || c.Selected != (i == 0) {
			return nil, ErrSessionsUnavailable
		}
		if !session.IsTelemetrySource(m.SourcePath) {
			native = true
			head = head || instant(c.HeadAt) != ""
		} else if m.SourcePath != session.TelemetrySourcePath(target.Client, target.SessionID) || c.Source.Identity != m.SourcePath {
			return nil, ErrSessionsUnavailable
		}
	}
	t := &effectiveTarget{indexed: session.IndexedSession{Metadata: copies[0].Metadata}, copies: copies}
	var sources []store.SessionOriginSource
	for _, c := range copies {
		// Preserve the display winner intact. FirstAt in the classifier's
		// private copy list is exclusively the verified binding input.
		source := c.Source
		switch {
		case target.Client != "codex":
			// Claude is outside the Codex birth migration.
			source.FirstAt = instant(source.FirstAt)
		case native && session.IsTelemetrySource(c.Metadata.SourcePath):
			source.FirstAt = ""
		case session.IsTelemetrySource(c.Metadata.SourcePath):
			at, err := view.SettledBirth(ctx, target)
			if err != nil {
				return nil, err
			}
			at = instant(at)
			if at == "" || at != instant(source.FirstAt) || (c.Birth != "" && instant(c.Birth) != at) {
				return nil, ErrClassificationUnavailable
			}
			source.FirstAt = at
		case head:
			source.FirstAt = instant(c.HeadAt)
		default:
			source.FirstAt = instant(c.Birth)
		}
		t.indexed.Copies = append(t.indexed.Copies, source)
		sources = append(sources, store.SessionOriginSource{FirstAt: source.FirstAt})
	}
	t.birth = BirthOf(sources)
	if target.Client == "codex" {
		t.indexed.Metadata.FirstAt = t.birth.StartedAt
	}
	return t, nil
}

// ResolveEffectiveBinding is the classify-set input. Only a successful empty
// lookup yields (nil, nil); a revoked view discards even a completed lookup.
// The lender supplies Selection() from the same capsule as the copies.
func ResolveEffectiveBinding(ctx context.Context, target Target, with OriginView[store.CJSIIndexStamp]) (*store.OriginBinding, error) {
	return readOriginView(with, func(v EffectiveCopyReader, stamp store.CJSIIndexStamp) (*store.OriginBinding, error) {
		if !validOriginStamp(stamp) {
			return nil, ErrSessionsUnavailable
		}
		item, err := readEffectiveTarget(ctx, v, target)
		if err != nil || item == nil {
			return nil, err
		}
		return stampedBinding(target, item.birth, stamp), nil
	})
}

func validOriginStamp(stamp store.CJSIIndexStamp) bool {
	return stamp.IndexEpoch > 0 && stamp.CoreEpoch > 0 && stamp.Inode != 0
}

func stampedBinding(target Target, birth Birth, stamp store.CJSIIndexStamp) *store.OriginBinding {
	b := BindingFor(target.Client, target.SessionID, birth)
	b.Selection = &stamp
	return &b
}

func overridesByTarget(rows []store.OriginOverride) map[Target]store.OriginOverride {
	result := make(map[Target]store.OriginOverride, len(rows))
	for _, o := range rows {
		result[Target{o.Client, o.SessionID}] = o
	}
	return result
}

// Only Codex births participate in the joint publication. A stale matched
// Codex row is a publication error; other clients keep read-time unmatching.
func effectiveOverrideStatus(key Target, birth Birth, o store.OriginOverride) (string, error) {
	if o.Value == nil || o.BindingState != BindingMatched {
		return BindingUnmatched, nil
	}
	b := BindingFor(key.Client, key.SessionID, birth)
	if b.Digest == o.BindingDigest && b.Limited == o.BindingLimited {
		return BindingMatched, nil
	}
	if key.Client == "codex" {
		return "", ErrClassificationUnavailable
	}
	return BindingUnmatched, nil
}

// EffectiveSelectionInputs contains the admitted List order and protected
// search hits, plus the core portion of this very same read. A raw base list
// or hits from an independently opened FTS reader are not valid inputs.
// Stamp is the full Selection() of the capsule that admitted those inputs.
type EffectiveSelectionInputs struct {
	Targets []Target
	Matches []session.Document
	Stamp   store.CJSIIndexStamp
	Core    store.OriginSelectionState
}

func SelectEffective(ctx context.Context, selection Selection, with OriginView[EffectiveSelectionInputs]) (Snapshot, error) {
	if err := selection.Validate(); err != nil {
		return Snapshot{}, err
	}
	return readOriginView(with, func(v EffectiveCopyReader, in EffectiveSelectionInputs) (Snapshot, error) {
		stamp := in.Stamp
		if !validOriginStamp(stamp) || stamp.CoreEpoch != in.Core.Epoch {
			return Snapshot{}, ErrSessionsUnavailable
		}
		index := session.IndexSnapshot{Epoch: stamp.IndexEpoch, Revision: stamp.SessionRevision, Documents: in.Matches}
		overrides := overridesByTarget(in.Core.Overrides)
		seen := make(map[Target]bool, len(in.Targets))
		for _, target := range in.Targets {
			item, err := readEffectiveTarget(ctx, v, target)
			if err != nil {
				return Snapshot{}, err
			}
			if item == nil || seen[target] {
				return Snapshot{}, errors.Join(ErrSessionsUnavailable, errors.New("effective enumeration and copies disagree"))
			}
			if _, err := effectiveOverrideStatus(target, item.birth, overrides[target]); err != nil {
				return Snapshot{}, err
			}
			seen[target] = true
			index.Sessions = append(index.Sessions, item.indexed)
		}
		return assemble(Snapshot{Selection: selection, Vector: Vector{CoreEpoch: in.Core.Epoch, UsageRevision: in.Core.Revision,
			SessionEpoch: stamp.IndexEpoch, SessionRevision: stamp.SessionRevision}, published: &stamp}, index, in.Core), nil
	})
}
