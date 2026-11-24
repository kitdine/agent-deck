package sessionorigin

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/ingest"
	"github.com/kitdine/agent-deck/internal/session"
	"github.com/kitdine/agent-deck/internal/store"
)

const (
	parent = "2026-01-02T08:00:00Z"
	head0  = "2026-01-02T09:00:00Z"
	head1  = "2026-01-02T09:01:00Z"
	execAt = "2026-01-02T09:04:00Z"
	otelAt = "2026-01-02T09:05:00Z"
)

var child = Target{"codex", "C"}

// The fixture serves only the eligible copies of a fixed view. Raw metadata
// and source first_at deliberately contain parent time; neither proves birth.
type copyFixture map[Target][]session.EffectiveSessionCopy

func (f copyFixture) EffectiveCopies(_ context.Context, client, id string) ([]session.EffectiveSessionCopy, error) {
	return f[Target{client, id}], nil
}

func (f copyFixture) SettledBirth(context.Context, Target) (string, error) {
	return otelAt, nil
}

func borrow[T any](v EffectiveCopyReader, inputs T, after error) OriginView[T] {
	return func(read func(EffectiveCopyReader, T) error) error {
		if err := read(v, inputs); err != nil {
			return err
		}
		return after
	}
}

func nativeCopy(name, head, birth string, selected bool) session.EffectiveSessionCopy {
	return session.EffectiveSessionCopy{
		Metadata: session.Metadata{Client: "codex", SessionID: "C", SourcePath: name, Project: "project-" + name, Model: name, FirstAt: parent, LastAt: otelAt},
		Source:   session.SourceCopy{Identity: "id-" + name, PrefixHash: "hash-" + name, ParserVersion: 7, FirstAt: parent},
		HeadAt:   head, Birth: birth, Selected: selected,
	}
}

func markerCopy(at string) session.EffectiveSessionCopy {
	c := nativeCopy("otel:codex/C", "", "", true)
	c.Source.FirstAt, c.Metadata.FirstAt = at, at
	c.Source.Identity, c.Source.PrefixHash = c.Metadata.SourcePath, ""
	return c
}

func bindingAt(at string) store.OriginBinding { return BindingFor("codex", "C", Birth{at}) }

func TestCJSIBinding(t *testing.T) {
	t.Run("protection-revokes-cached-search", func(t *testing.T) {
		old := store.CJSIIndexStamp{Ticket: 1, ProtectionRevision: 1}
		s := Snapshot{Selection: Selection{Scope: ScopeAll, Query: "needle"}, published: &old, Documents: []session.Document{{Text: "formerly-visible"}}}
		p := NewPublication(s, time.Now())
		newStamp := old
		newStamp.ProtectionRevision++
		ok, keep := PublishAt(s.Selection, nil, &p, nil, &s.Vector, &old)
		if !ok.Available || keep == nil {
			t.Fatal("did not serve admitted selection")
		}
		for _, stamp := range []*store.CJSIIndexStamp{nil, &newStamp} {
			got, keep := PublishAt(s.Selection, &p, &p, ErrSessionsUnavailable, &s.Vector, stamp)
			if got.Available || got.Publication != nil || keep != nil {
				t.Fatal("revoked content in fallback")
			}
		}
		moved := s.Vector
		moved.UsageRevision++
		if got, _ := PublishAt(s.Selection, &p, nil, ErrClassificationUnavailable, &moved, &old); got.Available {
			t.Fatal("replayed selection after user mutation")
		}
	})
	for _, test := range []struct {
		name  string
		times []session.EffectiveSessionCopy
		want  string
	}{
		{"all-heads-before-execution", []session.EffectiveSessionCopy{nativeCopy("selected", "", execAt, true), nativeCopy("live", head1, head1, false), nativeCopy("archive", head0, head0, false)}, head0},
		{"outer-head-fallback", []session.EffectiveSessionCopy{nativeCopy("live", "bad", execAt, true), nativeCopy("archive", head1, head1, false)}, head1},
		{"head-priority-even-over-earlier-execution", []session.EffectiveSessionCopy{nativeCopy("live", "", parent, true), nativeCopy("archive", head1, head1, false)}, head1},
		{"local-execution", []session.EffectiveSessionCopy{nativeCopy("live", "", execAt, true)}, execAt},
		{"untrusted-parent-mtime-and-first-at-ignored", []session.EffectiveSessionCopy{nativeCopy("mtime-08:00", "", "", true)}, ""},
		{"settled-otel-only", []session.EffectiveSessionCopy{markerCopy(otelAt)}, otelAt},
		{"native-does-not-borrow-otel", func() []session.EffectiveSessionCopy {
			m := markerCopy(otelAt)
			m.Selected = false
			return []session.EffectiveSessionCopy{nativeCopy("native", "", "", true), m}
		}(), ""},
		{"timezone-not-string-order", []session.EffectiveSessionCopy{nativeCopy("copy", "2026-01-02T11:00:00.000+02:00", "", true), nativeCopy("selected-second", head1, "", false)}, head0},
	} {
		t.Run(test.name, func(t *testing.T) {
			b, err := ResolveEffectiveBinding(t.Context(), child, borrow(copyFixture{child: test.times}, selectionStamp(1), nil))
			if err != nil || b == nil || !fixtureBindingMatches(b, test.want) {
				t.Fatalf("binding %v, %v; want %v", b, err, bindingAt(test.want))
			}
		})
	}
	t.Run("absent-unavailable-revoked-distinct", func(t *testing.T) {
		empty := copyFixture{}
		revoked := errors.New("protection changed")
		if b, err := ResolveEffectiveBinding(t.Context(), child, borrow(empty, selectionStamp(1), nil)); b != nil || err != nil {
			t.Fatal(b, err)
		}
		if b, err := ResolveEffectiveBinding(t.Context(), child, nil); b != nil || !errors.Is(err, ErrSessionsUnavailable) {
			t.Fatal(b, err)
		}
		if b, err := ResolveEffectiveBinding(t.Context(), child, borrow(empty, store.CJSIIndexStamp{}, nil)); b != nil || !errors.Is(err, ErrSessionsUnavailable) {
			t.Fatal("missing stamp", b, err)
		}
		for _, f := range []copyFixture{empty, {child: {nativeCopy("valid", head0, "", true)}}} {
			if b, err := ResolveEffectiveBinding(t.Context(), child, borrow(f, selectionStamp(1), revoked)); b != nil || !errors.Is(err, revoked) {
				t.Fatal(b, err)
			}
		}
		wrong := nativeCopy("parent", parent, parent, true)
		wrong.Metadata.SessionID = "P"
		if b, err := ResolveEffectiveBinding(t.Context(), child, borrow(copyFixture{child: {wrong}}, selectionStamp(1), nil)); b != nil || err == nil {
			t.Fatal("foreign key returned a binding", b, err)
		}
	})
	t.Run("continuity-requires-both-actual-observations", func(t *testing.T) {
		old, next, inputs := requestPair()
		for _, bad := range []struct {
			name string
			edit func(*OriginChangeInputs)
		}{
			{"valid", nil},
			{"missing-stable-id", func(in *OriginChangeInputs) { in.OldFirst[0].ResponseID = "" }},
			{"different-id", func(in *OriginChangeInputs) { in.LocalRequests[0].Record.Record.ResponseID += "x" }},
			{"different-time", func(in *OriginChangeInputs) { r := &in.LocalRequests[0].Record.Record; r.At = r.At.Add(time.Second) }},
			{"not-old-birth", func(in *OriginChangeInputs) {
				in.OldFirst[0].At = execAt
				in.LocalRequests[0].Record.Record.At, _ = time.Parse(time.RFC3339, execAt)
			}},
			{"same-total-different-components", func(in *OriginChangeInputs) {
				u := &in.LocalRequests[0].Record.Record.Usage
				u.Output.Value--
				u.Input.Value++
			}},
			{"missing-components", func(in *OriginChangeInputs) {
				in.OldFirst[0].Components = ingest.CodexCounts{}
				in.LocalRequests[0].Record.Record.Usage = ingest.CodexCounts{}
			}},
			{"wrong-generation", func(in *OriginChangeInputs) { in.LocalRequests[0].Source.PrefixHash += "stale" }},
			{"parent-request", func(in *OriginChangeInputs) { in.LocalRequests[0].Record.ThreadID = "P" }},
			{"inherited-request", func(in *OriginChangeInputs) { in.LocalRequests[0].Record.Execution = ingest.CodexForeign }},
			{"unresolved-request", func(in *OriginChangeInputs) { in.LocalRequests[0].Record.Execution = ingest.CodexUnresolved }},
			{"duplicate-local", func(in *OriginChangeInputs) { in.LocalRequests = append(in.LocalRequests, in.LocalRequests[0]) }},
			{"ambiguous-old", func(in *OriginChangeInputs) { in.OldFirst = append(in.OldFirst, in.OldFirst[0]) }},
			{"reuse-or-conflict", func(in *OriginChangeInputs) { in.IdentityConflict = true }},
		} {
			t.Run(bad.name, func(t *testing.T) {
				_, _, in := requestPair()
				if bad.edit != nil {
					bad.edit(&in)
				}
				b, s := inferBinding(child, old, next, in)
				if bad.edit == nil {
					if b != bindingAt(head0).Digest || s != BindingMatched {
						t.Fatal("lost valid request proof", b, s)
					}
				} else if b != inputs.Override.BindingDigest || s != BindingUnmatched {
					t.Fatal("applied unproved request", b, s)
				}
			})
		}
	})
	t.Run("native-first-request-retired-generation", testRetiredNativeFirst)
	t.Run("continuing-native-head-and-limited-identity", func(t *testing.T) {
		for _, tc := range []struct {
			old, next []session.EffectiveSessionCopy
			matched   bool
		}{
			{[]session.EffectiveSessionCopy{nativeCopy("same", head1, head1, true)}, []session.EffectiveSessionCopy{nativeCopy("other", head0, head0, true), nativeCopy("same", head1, head1, false)}, true},
			{[]session.EffectiveSessionCopy{nativeCopy("same", "", execAt, true)}, []session.EffectiveSessionCopy{nativeCopy("other", head0, head0, true), nativeCopy("same", "", execAt, false)}, true},
			{[]session.EffectiveSessionCopy{nativeCopy("same", head1, head1, true)}, []session.EffectiveSessionCopy{nativeCopy("same", head0, head0, true)}, false},
			{[]session.EffectiveSessionCopy{nativeCopy("same", "", "", true)}, []session.EffectiveSessionCopy{nativeCopy("same", "", "", true)}, true},
			{[]session.EffectiveSessionCopy{nativeCopy("same", "", "", true)}, []session.EffectiveSessionCopy{nativeCopy("other", "", "", true)}, false},
			{[]session.EffectiveSessionCopy{nativeCopy("same", "", "", true)}, []session.EffectiveSessionCopy{nativeCopy("same", head0, head0, true)}, false},
		} {
			old, err := readEffectiveTarget(t.Context(), copyFixture{child: tc.old}, child)
			if err != nil {
				t.Fatal(err)
			}
			next, err := readEffectiveTarget(t.Context(), copyFixture{child: tc.next}, child)
			if err != nil {
				t.Fatal(err)
			}
			before := bindingAt(old.birth.StartedAt)
			value := ClassAuxiliary
			row := store.OriginOverride{BindingState: BindingMatched, BindingLimited: before.Limited, BindingDigest: before.Digest, Value: &value}
			d, s := inferBinding(child, old, next, OriginChangeInputs{Override: &row})
			if (s == BindingMatched) != tc.matched || (!tc.matched && d != before.Digest) || (tc.matched && d != bindingAt(next.birth.StartedAt).Digest) {
				t.Fatal(tc, d, s)
			}
		}
	})
}

func testRetiredNativeFirst(t *testing.T) {
	for _, mode := range []string{"unique", "wrong-old-generation", "inherited-old", "other-old-birth", "missing-old-proof", "duplicate-new", "changed-new-time", "counter-conflict"} {
		t.Run(mode, func(t *testing.T) {
			ctx := t.Context()
			s := openStores(t)
			_, _, in := requestPair()
			old := nativeCopy("retired", "", execAt, true)
			next := nativeCopy("replacement", head0, head0, true)
			in.Previous = copyFixture{child: {old}}
			in.OldFirst[0].Source = generation(old.Source)
			in.OldFirst[0].At = execAt
			in.LocalRequests[0].Source = generation(next.Source)
			in.LocalRequests[0].Record.Record.At, _ = time.Parse(time.RFC3339, execAt)
			in.OldLocal = []LocalOriginRequest{in.LocalRequests[0]}
			in.OldLocal[0].Source = generation(old.Source)
			switch mode {
			case "wrong-old-generation":
				in.OldLocal[0].Source.PrefixHash += "other"
			case "inherited-old":
				in.OldLocal[0].Record.Execution = ingest.CodexForeign
			case "other-old-birth":
				in.Previous = copyFixture{child: {old, nativeCopy("proved-head", execAt, execAt, false)}}
			case "missing-old-proof":
				in.OldLocal = nil
			case "duplicate-new":
				in.LocalRequests = append(in.LocalRequests, in.LocalRequests[0])
			case "changed-new-time":
				in.LocalRequests[0].Record.Record.At = in.LocalRequests[0].Record.Record.At.Add(time.Second)
			case "counter-conflict":
				in.LocalRequests[0].Record.Record.Usage.Input.Value++
			}
			b := bindingAt(execAt)
			receipt, err := (Service{Store: s.core}).SetOverride(ctx, OverrideSetRequest{Client: "codex", SessionID: "C", RequestID: "native-set", Value: ClassAuxiliary, Binding: &b})
			if err != nil {
				t.Fatal(err)
			}
			before, _, err := s.core.OriginOverrideRecord(ctx, "codex", "C")
			if err != nil {
				t.Fatal(err)
			}
			in.Override = &before
			plan, err := Reconcile(ctx, nil, nil, ReconcileOptions{Published: &OriginReconciliation{SessionID: "C", View: borrow(copyFixture{child: {next}}, in, nil)}})
			if err != nil || plan.Prepared == nil {
				t.Fatalf("prepare: %v", err)
			}
			if err := applyFixture(ctx, s.core, *plan.Prepared, true); err != nil {
				t.Fatal(err)
			}
			reader, err := store.OpenReadOnly(ctx, s.root)
			if err != nil {
				t.Fatal(err)
			}
			defer reader.Close()
			got, _, err := reader.OriginOverrideRecord(ctx, "codex", "C")
			wantState, wantDigest := BindingUnmatched, b.Digest
			if mode == "unique" {
				wantState, wantDigest = BindingMatched, bindingAt(head0).Digest
			}
			if err != nil || got.BindingState != wantState || got.BindingDigest != wantDigest || got.Revision != before.Revision || got.UpdatedAt != before.UpdatedAt || *got.Value != *before.Value {
				t.Fatal("wrong persisted rebind", got, err)
			}
			back, err := (Service{Store: reader}).ReadReceipt(ctx, "native-set")
			if err != nil || !reflect.DeepEqual(back.Receipt, receipt) {
				t.Fatal("changed user receipt", back, err)
			}
		})
	}
}

func requestPair() (*effectiveTarget, *effectiveTarget, OriginChangeInputs) {
	old := markerCopy(otelAt)
	next := nativeCopy("live", head0, head0, true)
	good := func(n int64) ingest.CodexCount { return ingest.CodexCount{State: ingest.CodexValid, Value: n} }
	a := OriginRequest{Target: child, Source: generation(old.Source), ResponseID: "response-1", At: otelAt,
		Components: ingest.CodexCounts{State: ingest.CodexValid, Input: good(13), Output: good(4), Cached: good(0)}}
	at, _ := time.Parse(time.RFC3339, "2026-01-02T01:05:00-08:00")
	b := LocalOriginRequest{Source: generation(next.Source), Record: ingest.CodexResolvedRecord{Execution: ingest.CodexExplicit, ThreadID: "C",
		Record: ingest.CodexRecord{Kind: ingest.CodexRequestUsage, ResponseID: a.ResponseID, Usage: a.Components, At: at}}}
	value := ClassAuxiliary
	return &effectiveTarget{birth: Birth{otelAt}, copies: []session.EffectiveSessionCopy{old}}, &effectiveTarget{birth: Birth{head0}, copies: []session.EffectiveSessionCopy{next}},
		OriginChangeInputs{Target: child, Previous: copyFixture{child: {old}}, OldFirst: []OriginRequest{a}, LocalRequests: []LocalOriginRequest{b},
			Override: &store.OriginOverride{Client: child.Client, SessionID: child.SessionID, Value: &value, BindingDigest: bindingAt(otelAt).Digest, BindingState: BindingMatched}}
}

func applyFixture(ctx context.Context, core *store.Store, plan PreparedOrigin, commit bool) error {
	tx, err := core.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := plan.ApplyTx(ctx, tx); err != nil {
		return err
	}
	if commit {
		return tx.Commit()
	}
	return nil
}

func TestCJSIClassification(t *testing.T) {
	for _, tc := range []struct {
		name, oldAt, nextAt, status string
		proof, restored             bool
	}{
		{"unchanged-normalized", otelAt, "2026-01-02T10:05:00+01:00", BindingMatched, false, false},
		{"proved-change", otelAt, head0, BindingMatched, true, false},
		{"unproved-change", otelAt, head0, BindingUnmatched, false, false},
		{"restored-same", otelAt, otelAt, BindingUnmatched, false, true},
		{"restored-limited", "", head0, BindingUnmatched, false, true},
		{"native-limited", "", "", BindingMatched, false, false},
		{"limited-gains-time", "", head0, BindingUnmatched, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			s := openStores(t)
			svc := Service{Store: s.core, Now: func() time.Time { return receivedAt }}
			b := bindingAt(tc.oldAt)
			_, err := svc.SetOverride(ctx, OverrideSetRequest{RequestID: "user-set", Client: "codex", SessionID: "C", Value: ClassAuxiliary, Binding: &b})
			if err != nil {
				t.Fatal(err)
			}
			if tc.restored {
				if _, err := s.core.DB.ExecContext(ctx, `UPDATE origin_overrides SET binding_state='unmatched'`); err != nil {
					t.Fatal(err)
				}
			}
			original, found, err := s.core.OriginReceipt(ctx, "user-set")
			if err != nil || !found {
				t.Fatal(original, err)
			}
			_, _, in := requestPair()
			old := markerCopy(tc.oldAt)
			if tc.oldAt == "" {
				old = nativeCopy("live", "", "", true)
			}
			in.Previous = copyFixture{child: {old}}
			row, found, err := s.core.OriginOverrideRecord(ctx, "codex", "C")
			if err != nil || !found {
				t.Fatal(row, err)
			}
			in.Override = &row
			if !tc.proof {
				in.OldFirst, in.LocalRequests = nil, nil
			}
			selected := nativeCopy("display", "", "", true)
			next := copyFixture{child: {selected, nativeCopy("live", tc.nextAt, tc.nextAt, false)}}
			plan, err := PrepareEffectiveOrigin(ctx, "C", receivedAt.Add(time.Hour), borrow(next, in, nil))
			if err != nil {
				t.Fatal(err)
			}
			if err := applyFixture(ctx, s.core, plan, false); err != nil {
				t.Fatal(err)
			}
			prior, _, err := s.core.OriginOverrideRecord(ctx, "codex", "C")
			if err != nil || !reflect.DeepEqual(prior, row) {
				t.Fatal("rollback changed user row", prior, err)
			}
			priorState, err := s.core.ReadOriginSelectionState(ctx)
			if err != nil || len(priorState.Origins) != 0 {
				t.Fatal("rollback leaked origin", priorState, err)
			}
			if err := applyFixture(ctx, s.core, plan, true); err != nil {
				t.Fatal(err)
			}
			// A fresh core-only handle reads the persisted list before a detail lookup.
			reader, err := store.OpenReadOnly(ctx, s.root)
			if err != nil {
				t.Fatal(err)
			}
			defer reader.Close()
			page, err := (Service{Store: reader}).ListOverrides(ctx, 1, 20)
			if err != nil || page.Total != 1 || len(page.Items) != 1 || page.Items[0].BindingStatus != tc.status || page.Items[0].Revision != 1 || page.Items[0].Value != ClassAuxiliary {
				t.Fatal(page, err)
			}
			st, err := reader.ReadOriginSelectionState(ctx)
			if err != nil || len(st.Overrides) != 1 || len(st.Origins) != 1 {
				t.Fatal(st, err)
			}
			got := st.Overrides[0]
			expectedDigest := b.Digest
			if tc.status == BindingMatched {
				expectedDigest = bindingAt(instant(tc.nextAt)).Digest
			}
			if got.BindingDigest != expectedDigest || got.BindingState != tc.status || got.BindingLimited != b.Limited || got.UpdatedAt != row.UpdatedAt || got.Revision != row.Revision || *got.Value != *row.Value {
				t.Fatal("persisted binding", got)
			}
			r, found, err := reader.OriginReceipt(ctx, "user-set")
			if err != nil || !found || !reflect.DeepEqual(r, original) {
				t.Fatal("changed historical receipt", r, err)
			}
			// Re-preparing the same effective result publishes no derived change.
			in.Previous, in.Override, in.Origin = next, &got, &st.Origins[0]
			repeat, err := PrepareEffectiveOrigin(ctx, "C", receivedAt.Add(2*time.Hour), borrow(next, in, nil))
			if err != nil {
				t.Fatal(err)
			}
			if err := applyFixture(ctx, s.core, repeat, true); err != nil {
				t.Fatal(err)
			}
			unchanged, err := reader.ReadOriginSelectionState(ctx)
			if err != nil || unchanged.Revision != st.Revision {
				t.Fatal("no-op advanced serving state", unchanged.Revision, st.Revision, err)
			}
			for _, scope := range []string{ScopeAll, ScopeWork, ScopeAuxiliary} {
				input := EffectiveSelectionInputs{Stamp: selectionStamp(st.Epoch), Core: st, Targets: []Target{child}, Matches: []session.Document{{Client: "codex", SessionID: "C", Text: "hit"}}}
				snap, err := SelectEffective(ctx, Selection{Scope: scope, Query: "hit"}, borrow(next, input, nil))
				aux := tc.status == BindingMatched
				included := scope == ScopeAll || (scope == ScopeAuxiliary) == aux
				want := 0
				if included {
					want = 1
				}
				if err != nil || snap.Counts.Sessions != want || len(snap.Documents) != want || snap.SelectedSessionCount != want || snap.SessionFilter().Allows("codex", "C") != included || (scope == ScopeAll && snap.SessionFilter() != nil) {
					t.Fatal(scope, snap, err)
				}
				if included {
					item := snap.Sessions[0]
					want := selected.Metadata
					want.FirstAt = instant(tc.nextAt)
					if item.Metadata != want || item.Origin.Automatic != ClassWork || item.Origin.Override == nil || item.Origin.Override.BindingStatus != tc.status || item.Origin.Override.Applied != aux || (item.Origin.Effective == ClassAuxiliary) != aux || item.Origin.Revision != 1 {
						t.Fatal("classify detail", item)
					}
				}
			}
			if tc.status == BindingUnmatched {
				current, err := ResolveEffectiveBinding(ctx, child, borrow(next, selectionStamp(1), nil))
				if err != nil {
					t.Fatal(err)
				}
				// This CAS fixture uses the legacy core, which has no publication head.
				current.Selection = nil
				saved, err := svc.SetOverride(ctx, OverrideSetRequest{RequestID: "explicit-rebind", Client: "codex", SessionID: "C", Value: ClassAuxiliary, ExpectedRevision: 1, Binding: current})
				if err != nil || saved.Revision != 2 || saved.Outcome != OutcomeSaved {
					t.Fatal(saved, err)
				}
				rebound, found, err := reader.OriginOverrideRecord(ctx, "codex", "C")
				if err != nil || !found || rebound.BindingState != BindingMatched || rebound.BindingDigest != current.Digest || rebound.BindingLimited != current.Limited {
					t.Fatal(rebound, err)
				}
				cleared, err := svc.ClearOverride(ctx, OverrideClearRequest{RequestID: "explicit-clear", Client: "codex", SessionID: "C", ExpectedRevision: &saved.Revision})
				if err != nil || cleared.Revision != 3 || cleared.ClassificationReason != ReasonNotQueried {
					t.Fatal(cleared, err)
				}
			}
		})
	}
	t.Run("user-writes-win", testPreparedAfterUserWrite)
	t.Run("effective-marker-and-revocation", testEffectiveMarker)
	t.Run("authorized-selection-boundaries", testEffectiveSelectionBoundaries)
}

// Task 5 owns filtering the rows. Model its output for each protection
// change, then assert what actually reaches selection, search and binding.
func testEffectiveSelectionBoundaries(t *testing.T) {
	p := Target{"codex", "P"}
	primary := nativeCopy("primary", head0, head0, true)
	replica := nativeCopy("archive", head1, head1, false)
	pcopy := nativeCopy("p", parent, parent, true)
	pcopy.Metadata.SessionID = "P"
	for _, tc := range []struct {
		name   string
		keys   []Target
		copies copyFixture
		want   []string
		birth  string
	}{
		{"all", []Target{p, child}, copyFixture{p: {pcopy}, child: {primary, replica}}, []string{"p", "primary"}, head0},
		{"parent-id-excluded", []Target{child}, copyFixture{child: {primary, replica}}, []string{"primary"}, head0},
		{"primary-path-excluded", []Target{p, child}, func() copyFixture { r := replica; r.Selected = true; return copyFixture{p: {pcopy}, child: {r}} }(), []string{"p", "archive"}, head1},
		{"child-project-excluded", []Target{p}, copyFixture{p: {pcopy}}, []string{"p"}, ""},
		{"client-excluded", nil, copyFixture{}, nil, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := EffectiveSelectionInputs{Core: store.OriginSelectionState{Epoch: 1}, Stamp: selectionStamp(1), Targets: tc.keys}
			for _, key := range tc.keys {
				in.Matches = append(in.Matches, session.Document{Client: key.Client, SessionID: key.SessionID, Text: tc.copies[key][0].Metadata.SourcePath})
			}
			snap, err := SelectEffective(t.Context(), Selection{Scope: ScopeAll, Query: "needle"}, borrow(tc.copies, in, nil))
			if err != nil || len(snap.Documents) != len(tc.want) || len(snap.Sessions) != len(tc.want) || snap.SessionFilter() != nil {
				t.Fatal(snap, err)
			}
			for i, want := range tc.want {
				if snap.Sessions[i].SourcePath != want || snap.Documents[i].Text != want {
					t.Fatal(snap)
				}
			}
			b, err := ResolveEffectiveBinding(t.Context(), child, borrow(tc.copies, selectionStamp(1), nil))
			if err != nil || (b == nil) != (tc.birth == "") || b != nil && !fixtureBindingMatches(b, tc.birth) {
				t.Fatal(b, err)
			}
		})
	}
}

func testPreparedAfterUserWrite(t *testing.T) {
	for _, action := range []string{"clear", "set", "first-set"} {
		t.Run(action, func(t *testing.T) {
			ctx := t.Context()
			s := openStores(t)
			svc := Service{Store: s.core, Now: func() time.Time { return receivedAt }}
			old := bindingAt(otelAt)
			_, _, in := requestPair()
			var revision int64
			in.Override = nil
			if action != "first-set" {
				r, err := svc.SetOverride(ctx, OverrideSetRequest{RequestID: "old", Client: "codex", SessionID: "C", Value: ClassAuxiliary, Binding: &old})
				if err != nil {
					t.Fatal(err)
				}
				revision = r.Revision
				row, found, err := s.core.OriginOverrideRecord(ctx, "codex", "C")
				if err != nil || !found {
					t.Fatal(err)
				}
				in.Override = &row
			}
			next := copyFixture{child: {nativeCopy("live", head0, head0, true)}}
			plan, err := PrepareEffectiveOrigin(ctx, "C", receivedAt, borrow(next, in, nil))
			if err != nil {
				t.Fatal(err)
			}
			if action == "clear" {
				r, err := svc.ClearOverride(ctx, OverrideClearRequest{RequestID: "later", Client: "codex", SessionID: "C", ExpectedRevision: &revision})
				if err != nil || r.Revision != revision+1 {
					t.Fatal(r, err)
				}
			} else {
				b, err := ResolveEffectiveBinding(ctx, child, borrow(next, selectionStamp(1), nil))
				if err != nil {
					t.Fatal(err)
				}
				// This concurrent writer uses the legacy fixture core.
				b.Selection = nil
				r, err := svc.SetOverride(ctx, OverrideSetRequest{RequestID: "later", Client: "codex", SessionID: "C", Binding: b, ExpectedRevision: revision, Value: ClassWork})
				if err != nil || r.Revision != revision+1 {
					t.Fatal(r, err)
				}
			}
			if err := applyFixture(ctx, s.core, plan, true); !errors.Is(err, store.ErrOriginProjectionStale) {
				t.Fatal("accepted stale plan", err)
			}
			row, found, err := s.core.OriginOverrideRecord(ctx, "codex", "C")
			if err != nil || !found || row.Revision != revision+1 {
				t.Fatal(row, err)
			}
			if action == "clear" && row.Value != nil || action != "clear" && (row.Value == nil || *row.Value != ClassWork || row.BindingDigest != bindingAt(head0).Digest || row.BindingState != BindingMatched) {
				t.Fatal("stale inference overwrote user action", row)
			}
			st, err := s.core.ReadOriginSelectionState(ctx)
			if err != nil || len(st.Origins) != 0 {
				t.Fatal("partial classification", st, err)
			}
			receipt, err := svc.ReadReceipt(ctx, "later")
			if err != nil || !receipt.Known || receipt.Receipt.Revision != revision+1 {
				t.Fatal(receipt, err)
			}
			if action == "clear" {
				in.Override = &row
				retry, err := PrepareEffectiveOrigin(ctx, "C", receivedAt, borrow(next, in, nil))
				if err != nil {
					t.Fatal(err)
				}
				if err := applyFixture(ctx, s.core, retry, true); err != nil {
					t.Fatal(err)
				}
				page, err := svc.ListOverrides(ctx, 1, 20)
				if err != nil || page.Total != 0 {
					t.Fatal("clear resurrected on reprepare", page, err)
				}
			}
		})
	}
}

func testEffectiveMarker(t *testing.T) {
	ctx := t.Context()
	s := openStores(t)
	m := markerCopy(otelAt)
	old := copyFixture{child: {m}}
	revoked := errors.New("view revoked after callback")
	in := OriginChangeInputs{Target: child, Previous: copyFixture{}, Marker: &store.TelemetrySessionOriginMarker{SourceKind: store.TelemetrySessionSourceKind, SupportEntry: "codex-app-server/0.165.0/codex-tui/sse", SettledAt: otelAt}}
	if p, err := PrepareEffectiveOrigin(ctx, "P", receivedAt, borrow(old, in, nil)); !errors.Is(err, ErrClassificationUnavailable) || p.id != "" {
		t.Fatal("foreign marker scope accepted", p, err)
	}
	native := nativeCopy("child", head0, head0, true)
	m.Selected = false
	retained := copyFixture{child: {native, m}}
	for _, view := range []copyFixture{old, retained} {
		p, err := PrepareEffectiveOrigin(ctx, "C", receivedAt, borrow(view, in, nil))
		if err != nil {
			t.Fatal(err)
		}
		if err := applyFixture(ctx, s.core, p, true); err != nil {
			t.Fatal(err)
		}
		st, err := s.core.OriginDerivedState(ctx)
		if err != nil || len(st.Origins) != 1 || len(st.Evidence) != 1 || st.Origins[0].Automatic != ClassAuxiliary {
			t.Fatal("retained marker", st, err)
		}
		in.Previous, in.Origin, in.Evidence = view, &st.Origins[0], st.Evidence
	}
	in.Marker = nil
	in.TaskChild = true
	next := copyFixture{child: {native}}
	p, err := PrepareEffectiveOrigin(ctx, "C", receivedAt, borrow(next, in, revoked))
	if !errors.Is(err, revoked) || p.id != "" {
		t.Fatal("revoked plan escaped", p, err)
	}
	if err := applyFixture(ctx, s.core, p, true); err == nil {
		t.Fatal("zero plan applied")
	}
	p, err = PrepareEffectiveOrigin(ctx, "C", receivedAt, borrow(next, in, nil))
	if err != nil {
		t.Fatal(err)
	}
	if err := applyFixture(ctx, s.core, p, true); err != nil {
		t.Fatal(err)
	}
	st, err := s.core.ReadOriginSelectionState(ctx)
	if err != nil || len(st.Origins) != 1 || st.Origins[0].Automatic != ClassWork || st.Origins[0].ReasonCode != ReasonTaskChildUnconfirmed {
		t.Fatal("withdrawn marker classified", st, err)
	}
	facts, err := s.core.OriginDerivedState(ctx)
	if err != nil || len(facts.Evidence) != 0 {
		t.Fatal("withdrawn marker evidence persisted", facts, err)
	}
	input := EffectiveSelectionInputs{Stamp: selectionStamp(st.Epoch), Targets: []Target{child}, Core: st}
	snap, err := SelectEffective(ctx, Selection{Scope: ScopeAll}, borrow(next, input, revoked))
	if !errors.Is(err, revoked) || !reflect.DeepEqual(snap, Snapshot{}) {
		t.Fatal("revoked output escaped", snap, err)
	}
	snap, err = SelectEffective(ctx, Selection{Scope: ScopeAll}, borrow(copyFixture{}, input, nil))
	if !errors.Is(err, ErrSessionsUnavailable) || !reflect.DeepEqual(snap, Snapshot{}) {
		t.Fatal("mixed enumeration returned success", snap, err)
	}
	in.Previous, in.Origin, in.Evidence = next, &st.Origins[0], nil
	p, err = PrepareEffectiveOrigin(ctx, "C", receivedAt, borrow(copyFixture{}, in, nil))
	if err != nil {
		t.Fatal(err)
	}
	if err := applyFixture(ctx, s.core, p, true); err != nil {
		t.Fatal(err)
	}
	facts, err = s.core.OriginDerivedState(ctx)
	if err != nil || len(facts.Origins)+len(facts.Evidence) != 0 {
		t.Fatal("admitted removal did not clear derivation", facts, err)
	}
	snap, err = SelectEffective(ctx, Selection{Scope: ScopeAll}, borrow(copyFixture{}, EffectiveSelectionInputs{Stamp: selectionStamp(st.Epoch), Core: st}, nil))
	if err != nil || snap.Counts.Sessions != 0 || snap.Sessions == nil || snap.SessionFilter() != nil {
		t.Fatal("admitted empty selection", snap, err)
	}
}

func fixtureBindingMatches(b *store.OriginBinding, at string) bool {
	want := bindingAt(at)
	return b.Digest == want.Digest && b.Limited == want.Limited && b.Selection != nil && *b.Selection == selectionStamp(1)
}

func selectionStamp(epoch int64) store.CJSIIndexStamp {
	return store.CJSIIndexStamp{Ticket: 3, CoreEpoch: epoch, IndexEpoch: 2, SessionRevision: 5, ProtectionRevision: 1, Device: 1, Inode: 9}
}

func TestCJSIEffectiveSelectionReadmission(t *testing.T) {
	selection := Selection{Scope: ScopeAll, Query: "synthetic"}
	inputs := EffectiveSelectionInputs{
		Targets: []Target{child},
		Matches: []session.Document{{Client: child.Client, SessionID: child.SessionID, Text: "synthetic-old-document"}},
		Stamp:   selectionStamp(1),
		Core:    store.OriginSelectionState{Epoch: 1, Revision: 7},
	}
	visible := copyFixture{child: {nativeCopy("visible-before-protection", head0, head0, true)}}
	view, err := SelectEffective(t.Context(), selection, borrow(visible, inputs, nil))
	if err != nil || len(view.Documents) != 1 {
		t.Fatal("precondition: old protected reader result was visible", err)
	}
	previous := NewPublication(view, time.Now())
	revoked := errors.New("after private protection revocation")
	_, buildErr := SelectEffective(t.Context(), selection, borrow(visible, inputs, revoked))
	if !errors.Is(buildErr, revoked) {
		t.Fatal("precondition: revoked reader returned a result")
	}
	// Protection is an independent revision. A matching fact vector alone
	// cannot authorize reusing a managed document.
	if result, retained := Publish(selection, &previous, nil, buildErr, &view.Vector); result.Available || retained != nil {
		t.Fatal("managed snapshot with no protection stamp reused old document")
	}
	if result, _ := PublishAt(selection, nil, &previous, nil, &view.Vector, &inputs.Stamp); !result.Available {
		t.Fatal("exact stamp rejected")
	}
	moved := inputs.Stamp
	moved.ProtectionRevision++
	if result, _ := PublishAt(selection, &previous, nil, ErrSessionsUnavailable, &view.Vector, &moved); result.Available {
		t.Fatal("old protection accepted")
	}
	missing := inputs
	missing.Stamp = store.CJSIIndexStamp{}
	if _, err := SelectEffective(t.Context(), selection, borrow(visible, missing, nil)); !errors.Is(err, ErrSessionsUnavailable) {
		t.Fatal("missing stamp accepted", err)
	}

}

func TestCJSIGenericMatchedCodexConsistency(t *testing.T) {
	value := ClassAuxiliary
	copy := nativeCopy("child", head0, head0, true)
	binding := bindingAt(head0)
	for _, bad := range []string{"digest", "limited"} {
		t.Run(bad, func(t *testing.T) {
			o := store.OriginOverride{Client: child.Client, SessionID: child.SessionID, Value: &value, BindingState: BindingMatched, BindingDigest: binding.Digest, BindingLimited: binding.Limited}
			if bad == "digest" {
				o.BindingDigest = "obsolete"
			} else {
				o.BindingLimited = !o.BindingLimited
			}
			in := EffectiveSelectionInputs{Targets: []Target{child}, Stamp: selectionStamp(1), Core: store.OriginSelectionState{Epoch: 1, Overrides: []store.OriginOverride{o}}}
			view, err := SelectEffective(t.Context(), Selection{Scope: ScopeAll}, borrow(copyFixture{child: {copy}}, in, nil))
			if !errors.Is(err, ErrClassificationUnavailable) || !reflect.DeepEqual(view, Snapshot{}) {
				t.Fatalf("inconsistent %s returned %d sessions, err=%v", bad, len(view.Sessions), err)
			}
		})
	}
}
