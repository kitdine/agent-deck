package sessionorigin

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

func publicationAt(selection Selection, vector Vector) *Publication {
	value := NewPublication(Snapshot{Selection: selection, Vector: vector}, time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC))
	return &value
}

func TestPublicationIdentityIsTheSelectionAndItsGeneration(t *testing.T) {
	selection := Selection{Scope: ScopeWork, Client: "codex", Period: "today", From: time.Date(2026, 11, 1, 4, 0, 0, 0, time.UTC), To: time.Date(2026, 11, 2, 5, 0, 0, 0, time.UTC)}
	vector := Vector{CoreEpoch: 1, UsageRevision: 10, SessionEpoch: 2, SessionRevision: 20}
	base := publicationAt(selection, vector).ID
	if len(base) != 64 || publicationAt(selection, vector).ID != base {
		t.Fatalf("identity %q is not a stable digest", base)
	}
	seen := map[string]string{base: "base"}
	variants := map[string]*Publication{}
	for name, change := range map[string]func(*Selection, *Vector){
		"scope":            func(s *Selection, _ *Vector) { s.Scope = ScopeAll },
		"client":           func(s *Selection, _ *Vector) { s.Client = "claude" },
		"period":           func(s *Selection, _ *Vector) { s.Period = "7d" },
		"window":           func(s *Selection, _ *Vector) { s.From = s.From.Add(time.Hour) },
		"query":            func(s *Selection, _ *Vector) { s.Query = "x" },
		"core epoch":       func(_ *Selection, v *Vector) { v.CoreEpoch++ },
		"usage revision":   func(_ *Selection, v *Vector) { v.UsageRevision++ },
		"session epoch":    func(_ *Selection, v *Vector) { v.SessionEpoch++ },
		"session revision": func(_ *Selection, v *Vector) { v.SessionRevision++ },
	} {
		s, v := selection, vector
		change(&s, &v)
		variants[name] = publicationAt(s, v)
	}
	for name, variant := range variants {
		if other, dup := seen[variant.ID]; dup {
			t.Fatalf("%s collides with %s", name, other)
		}
		seen[variant.ID] = name
	}
}

func TestPublishRejectsLateResultsAndNeverComparesAcrossEpochs(t *testing.T) {
	selection := Selection{Scope: ScopeWork}
	current := publicationAt(selection, Vector{CoreEpoch: 1, UsageRevision: 10, SessionEpoch: 1, SessionRevision: 10})
	for _, c := range []struct {
		name     string
		vector   Vector
		accepted bool
	}{
		{"newer usage", Vector{1, 11, 1, 10}, true},
		{"newer session", Vector{1, 10, 1, 11}, true},
		{"same generation", Vector{1, 10, 1, 10}, true},
		{"late usage", Vector{1, 9, 1, 10}, false},
		{"late session", Vector{1, 10, 1, 9}, false},
		{"incomparable", Vector{1, 11, 1, 9}, false},
		{"restored core epoch, smaller revisions", Vector{7, 1, 1, 1}, true},
		{"rebuilt session epoch, smaller revisions", Vector{1, 1, 9, 1}, true},
	} {
		candidate := publicationAt(selection, c.vector)
		observed := c.vector // the live identity at the publication boundary
		view, retained := Publish(selection, current, candidate, nil, &observed)
		if !view.Available || view.Stale {
			t.Fatalf("%s: view %+v", c.name, view)
		}
		if want := map[bool]*Publication{true: candidate, false: current}[c.accepted]; retained != want || view.Publication != want {
			t.Fatalf("%s: accepted=%v", c.name, retained == candidate)
		}
	}
	// A result for another selection is never compared with or kept for this one.
	other := publicationAt(Selection{Scope: ScopeAll}, Vector{1, 1, 1, 1})
	live := Vector{1, 1, 1, 1}
	if view, retained := Publish(Selection{Scope: ScopeAll}, current, other, nil, &live); retained != other || view.Publication != other || view.Stale {
		t.Fatal("another selection's publication blocked a new one")
	}
	if view, retained := Publish(selection, current, other, nil, &live); retained != current || !view.Stale {
		t.Fatalf("a candidate for another selection was published: %+v", view)
	}
	if view, retained := Publish(Selection{Scope: ScopeAll}, current, nil, ErrSelectionStale, nil); view.Available || retained != nil {
		t.Fatalf("another selection's result kept as stale: %+v", view)
	}
}

func TestFailedBuildsKeepTheLastCompleteResultAsStale(t *testing.T) {
	selection := Selection{Scope: ScopeAuxiliary}
	previous := publicationAt(selection, Vector{CoreEpoch: 1, UsageRevision: 5, SessionEpoch: 1, SessionRevision: 5})
	for err, reason := range map[error]string{
		ErrSelectionStale: PublicationReasonStale,
		fmt.Errorf("%w: x", ErrSessionsUnavailable):       PublicationReasonSessionsUnavailable,
		fmt.Errorf("%w: x", ErrClassificationUnavailable): PublicationReasonClassificationUnavailable,
		fmt.Errorf("%w: x", ErrGenerationUnavailable):     PublicationReasonGenerationUnavailable,
	} {
		view, retained := Publish(selection, previous, nil, err, nil)
		if !view.Available || !view.Stale || view.Reason != reason || view.Publication != previous || retained != previous {
			t.Fatalf("%v: %+v", err, view)
		}
		// With nothing complete to keep the section is unavailable, not empty.
		if view, retained := Publish(selection, nil, nil, err, nil); view.Available || view.Publication != nil || retained != nil || view.Reason != reason {
			t.Fatalf("%v without previous: %+v", err, view)
		}
	}
	// A restore or rebuild invalidates the previous identity: no stale fallback.
	observed := Vector{CoreEpoch: 2, UsageRevision: 100, SessionEpoch: 1, SessionRevision: 5}
	if view, retained := Publish(selection, previous, nil, ErrSelectionStale, &observed); view.Available || retained != nil {
		t.Fatalf("old epoch kept after restore: %+v", view)
	}
	same := Vector{CoreEpoch: 1, UsageRevision: 100, SessionEpoch: 1, SessionRevision: 50}
	if view, _ := Publish(selection, previous, nil, ErrSelectionStale, &same); !view.Available || !view.Stale {
		t.Fatalf("same-epoch stale fallback lost: %+v", view)
	}
	if view, _ := Publish(selection, previous, nil, errors.New("unclassified"), nil); view.Reason != PublicationReasonStale {
		t.Fatalf("unclassified failure reason %q", view.Reason)
	}
}

// ASC-SP-R1-F1: a request that finished before a restore or rebuild must not
// republish its retired identity after the new epoch was published, whatever
// the numeric relation of the epochs or revisions.
func TestRetiredEpochResultsNeverReplaceTheLivePublication(t *testing.T) {
	selection := Selection{Scope: ScopeWork}
	retired := Vector{CoreEpoch: 9, UsageRevision: 99, SessionEpoch: 9, SessionRevision: 99}
	for name, live := range map[string]Vector{
		"core restored":    {CoreEpoch: 2, UsageRevision: 1, SessionEpoch: 9, SessionRevision: 99},
		"sessions rebuilt": {CoreEpoch: 9, UsageRevision: 99, SessionEpoch: 2, SessionRevision: 1},
		"both replaced":    {CoreEpoch: 2, UsageRevision: 1, SessionEpoch: 2, SessionRevision: 1},
		"larger new epoch": {CoreEpoch: 11, UsageRevision: 1, SessionEpoch: 9, SessionRevision: 99},
	} {
		t.Run(name, func(t *testing.T) {
			old := publicationAt(selection, retired)
			fresh := publicationAt(selection, live)
			observed := live
			// The first legal switch: the live epoch replaces the retired one.
			if view, retained := Publish(selection, old, fresh, nil, &observed); retained != fresh || view.Publication != fresh || !view.Available || view.Stale {
				t.Fatalf("live epoch not accepted: %+v", view)
			}
			// The retired request arrives late.
			view, retained := Publish(selection, fresh, old, nil, &observed)
			if retained != fresh || view.Publication != fresh || !view.Available || view.Stale {
				t.Fatalf("retired epoch republished: available=%v stale=%v kept=%v", view.Available, view.Stale, view.Publication == fresh)
			}
			// With nothing live published, a retired result is still not shown.
			if view, retained := Publish(selection, nil, old, nil, &observed); view.Available || retained != nil || view.Reason != PublicationReasonGenerationUnavailable {
				t.Fatalf("retired result shown with nothing live: %+v", view)
			}
			// A failed build never falls back to the retired publication.
			if view, retained := Publish(selection, old, nil, ErrSelectionStale, &observed); view.Available || retained != nil {
				t.Fatalf("retired publication kept as stale: %+v", view)
			}
			// Without a trusted live identity an epoch change is not shown either way.
			for _, pair := range [][2]*Publication{{old, fresh}, {fresh, old}} {
				if view, retained := Publish(selection, pair[0], pair[1], nil, nil); view.Available || view.Publication != nil || retained != pair[0] || view.Reason != PublicationReasonGenerationUnavailable {
					t.Fatalf("unverified epoch change shown: %+v", view)
				}
			}
		})
	}
}

// ASC-SP-R1-F1 Round 2: without a trusted live identity a successful build
// only proves the epoch it read. No candidate is shown as fresh, whether or
// not a previous publication exists and whatever epoch it has.
func TestMissingLiveIdentityNeverPublishesACandidateAsFresh(t *testing.T) {
	selection := Selection{Scope: ScopeWork}
	before := Vector{CoreEpoch: 9, UsageRevision: 99, SessionEpoch: 9, SessionRevision: 99}
	candidate := publicationAt(selection, before)

	// Nothing published yet: unavailable, nothing retained as live.
	if view, retained := Publish(selection, nil, candidate, nil, nil); view.Available || view.Publication != nil || retained != nil || view.Reason != PublicationReasonGenerationUnavailable {
		t.Fatalf("no previous: %+v", view)
	}
	// A previous of the same epoch, older or not, stays visible only as stale
	// pending verification; the candidate does not replace it.
	for name, vector := range map[string]Vector{
		"same generation": before,
		"older revisions": {CoreEpoch: 9, UsageRevision: 90, SessionEpoch: 9, SessionRevision: 90},
	} {
		previous := publicationAt(selection, vector)
		view, retained := Publish(selection, previous, candidate, nil, nil)
		if !view.Available || !view.Stale || view.Publication != previous || retained != previous || view.Reason != PublicationReasonGenerationUnavailable {
			t.Fatalf("%s: %+v", name, view)
		}
	}
	// Once the live identity is read, the same candidate publishes normally.
	observed := before
	if view, retained := Publish(selection, nil, candidate, nil, &observed); !view.Available || view.Stale || retained != candidate {
		t.Fatalf("verified candidate rejected: %+v", view)
	}
}
