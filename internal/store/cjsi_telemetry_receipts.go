package store

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"maps"
	"math"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/kitdine/agent-deck/internal/ingest"
	ledger "github.com/kitdine/agent-deck/internal/usage/cjsiledger"
)

// CJSIOTelAttribution pins the durable original route and pricing inputs for an
// OTel contribution across a takeover period. Evidence identifies those inputs,
// including the absence of a match.
type CJSIOTelAttribution struct {
	Evidence           [32]byte
	RouteID            sql.NullInt64
	RouteAt            string
	Provider           string
	Quality            string
	Multiplier         string
	CatalogVersion     string
	PriceModel         string
	PriceEffectiveFrom string
	PriceReference     string
}

type cjsiOTelAttribution = CJSIOTelAttribution

// CJSIOTelSettled captures the exact settled preimage of one otel_owned
// observation withdrawn by a CJSI takeover.
type CJSIOTelSettled struct {
	Contribution cjsiOTelContribution
	Event        cjsiOTelEvent
	Binding      *cjsiOTelBinding
	Attribution  CJSIOTelAttribution
}

type cjsiOTelSettled = CJSIOTelSettled

// CJSIOTelDisposition records the per-observation lifecycle state inside one
// takeover period.
type CJSIOTelDisposition string

type cjsiOTelDisposition = CJSIOTelDisposition

const (
	cjsiOTelApplied         cjsiOTelDisposition = "applied"
	cjsiOTelRestored        cjsiOTelDisposition = "restored"
	cjsiOTelReturnedPending cjsiOTelDisposition = "returned_pending"
	cjsiOTelInvalidated     cjsiOTelDisposition = "invalidated"
)

// CJSIOTelReceipt records one observation suppressed during a takeover period.
// Settled is nil for an observation that was originally pending (never enjoys
// fast-path restoration).
type CJSIOTelReceipt struct {
	Disposition CJSIOTelDisposition
	Original    TelemetryObservation
	Settled     *CJSIOTelSettled
}

type cjsiOTelReceipt = CJSIOTelReceipt

// CJSIOTelPeriod is one monotonic takeover period for a canonical Codex thread.
type CJSIOTelPeriod struct {
	Version, Number         int64
	Thread                  string
	Opened, Support, Closed ledger.ProposalStamp
	Requests                []ledger.Key
	Receipts                map[string]CJSIOTelReceipt
	Marker                  *cjsiOTelMarker
	Session                 *cjsiOTelSession
	SessionRouteID          sql.NullInt64
	SessionBindingDigest    []byte
}

type cjsiOTelPeriod = CJSIOTelPeriod

// CJSIOTelRelease proves withdrawal of the entire last supporting request set
// and a complete, non-overlapping inventory for this thread at this publication.
// A negative ledger view by itself never releases a period.
type CJSIOTelRelease struct {
	Thread           string
	Number           int64
	Support, Current ledger.ProposalStamp
	Removed          []ledger.Key
	InventoryAt      time.Time
}

type cjsiOTelRelease = CJSIOTelRelease

// CJSIOTelGuard binds the canonical thread, selected proposal stamp, and
// dependency revision checked inside the core write transaction.
type CJSIOTelGuard struct {
	Thread   string
	Stamp    ledger.ProposalStamp
	Revision int64
}

type cjsiOTelGuard = CJSIOTelGuard

// CJSIOTelSelection is the read-snapshot input resolved by CJSIOTelReceiptStore.
type CJSIOTelSelection struct {
	Revision     int64
	Thread       string
	Stamp        ledger.ProposalStamp
	Before       cjsiOTelSide
	Period       *CJSIOTelPeriod
	Attributions map[string]CJSIOTelAttribution
	Release      *CJSIOTelRelease
}

type cjsiOTelSelection = CJSIOTelSelection

// CJSIOTelReceiptStore resolves read-snapshot inputs, checks write-time guards,
// and persists takeover periods inside the caller's transaction. No method commits.
type CJSIOTelReceiptStore interface {
	Select(context.Context, *sql.Tx, string) (CJSIOTelSelection, error)
	Current(context.Context, *sql.Tx, string) (CJSIOTelGuard, error)
	Save(context.Context, *sql.Tx, *CJSIOTelPeriod, CJSIOTelPeriod) error
}

type cjsiOTelReceiptStore = CJSIOTelReceiptStore

type cjsiOTelEdit struct {
	old, next TelemetryObservation
	settled   *cjsiOTelSettled
}

// CJSIOTelChange is a validated, prepared ownership transition ready to apply
// inside the coordinator's core write transaction.
type CJSIOTelChange struct {
	before      cjsiOTelSide
	otherEvents []string
	guard       cjsiOTelGuard
	previous    *cjsiOTelPeriod
	period      cjsiOTelPeriod
	restoring   bool
	edits       []cjsiOTelEdit
	marker      *cjsiOTelMarker
	session     *cjsiOTelSession
	// Restoring only: the thread's usage_sessions images at preparation.
	// retire marks them as fed by the JSONL rows the withdrawal must remove.
	summaries []cjsiOTelSession
	retire    bool
}

type cjsiOTelChange = CJSIOTelChange

// PrepareCJSIOTel plans an explicit whole-thread OTel takeover, support update,
// or release in an independent core read snapshot before acquiring the writer.
// No durable row is changed. A nil change keeps the old side untouched.
func PrepareCJSIOTel(ctx context.Context, tx *sql.Tx, thread string, p ledger.Proposal, receipts CJSIOTelReceiptStore) (*CJSIOTelChange, error) {
	return prepareCJSIOTel(ctx, tx, thread, p, receipts)
}

func prepareCJSIOTel(ctx context.Context, tx *sql.Tx, thread string, p ledger.Proposal, receipts cjsiOTelReceiptStore) (*cjsiOTelChange, error) {
	if thread == "" || ingest.CodexLegacySessionID(thread) != thread {
		return nil, ledger.ErrProposalMismatch
	}
	if receipts == nil {
		return nil, errCJSIOTelPrecondition
	}
	s, err := receipts.Select(ctx, tx, thread)
	if err != nil {
		return nil, err
	}
	if s.Thread != thread || s.Revision <= 0 {
		return nil, ledger.ErrProposalMismatch
	}
	v, err := p.Eligibility(s.Stamp, thread)
	if err != nil {
		return nil, err
	}
	change, err := planCJSIOTel(s, v)
	if err != nil || change == nil {
		return nil, err
	}
	current, err := readCJSIOTelSide(ctx, tx, thread)
	if err != nil {
		return nil, err
	}
	if !reflect.DeepEqual(current, s.Before) {
		return nil, errCJSIOTelPrecondition
	}
	change.guard = cjsiOTelGuard{thread, s.Stamp, s.Revision}
	change.before = current
	change.previous = s.Period
	return change, nil
}

// Apply executes a prepared CJSI OTel transition inside the caller's core write
// transaction; roll back on every error. On release only, withdraw may apply
// the coordinator's separately proved JSONL removals after Save. The receipt
// consumer never deletes those JSONL keys itself. Returning successfully
// requires that every JSONL row of the canonical thread has exited and every
// OTel edit succeeded. The coordinator alone commits or opens a ticket.
func (change *CJSIOTelChange) Apply(ctx context.Context, tx *sql.Tx, receipts CJSIOTelReceiptStore, withdraw func(*sql.Tx) error) error {
	return change.apply(ctx, tx, receipts, withdraw)
}

func (change *cjsiOTelChange) apply(ctx context.Context, tx *sql.Tx, receipts cjsiOTelReceiptStore, withdraw func(*sql.Tx) error) error {
	if change == nil || receipts == nil {
		return errCJSIOTelPrecondition
	}
	current, err := receipts.Current(ctx, tx, change.guard.Thread)
	if err != nil {
		return err
	}
	if current != change.guard || (withdraw != nil && !change.restoring) {
		return errCJSIOTelPrecondition
	}
	if err = receipts.Save(ctx, tx, change.previous, change.period); err != nil {
		return err
	}
	if withdraw != nil {
		if err = withdraw(tx); err != nil {
			return err
		}
	}
	if change.restoring {
		if err = change.vacate(ctx, tx); err != nil {
			return err
		}
	}
	return change.applyRows(ctx, tx)
}

// Vacate follows the proved withdrawal in the same write transaction. Every
// dependent writer must still advance the revision; this also makes a missed
// advance fail closed. Each key seen at preparation must have exited, the whole
// canonical thread must hold no JSONL row of any key and no unexpected OTel
// side, and every usage_sessions row must still match an exact image captured
// at preparation (or have exited with the withdrawal). A JSONL-era summary is
// retired by exact preimage match so the original OTel summary is reinstated
// without overwriting a newer edit.
func (change *cjsiOTelChange) vacate(ctx context.Context, tx *sql.Tx) error {
	return change.checkVacated(ctx, tx, func(s cjsiOTelSession) error {
		return cjsiSessions.remove(ctx, tx, s)
	})
}

func (change *cjsiOTelChange) checkVacated(ctx context.Context, q CJSIOTelReadView, retire func(cjsiOTelSession) error) error {
	for _, key := range change.otherEvents {
		var present bool
		if err := q.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM usage_events WHERE event_key=?)", key).Scan(&present); err != nil {
			return err
		}
		if present {
			return errCJSIOTelPrecondition
		}
	}
	thread := change.guard.Thread
	remains, err := cjsiOTelJSONLRemains(ctx, q, thread)
	if err != nil {
		return err
	}
	if remains {
		return errCJSIOTelPrecondition
	}
	unexpectedOTel, err := cjsiOTelUnexpectedOTelExists(ctx, q, thread)
	if err != nil {
		return err
	}
	if unexpectedOTel {
		return errCJSIOTelPrecondition
	}
	summaries, err := readCJSIOTelSummaries(ctx, q, thread)
	if err != nil {
		return err
	}
	if !change.retire && !slices.Equal(summaries, change.summaries) {
		return errCJSIOTelPrecondition
	}
	for _, s := range summaries {
		if !slices.Contains(change.summaries, s) {
			return errCJSIOTelPrecondition
		}
		if change.retire && retire != nil {
			if err := retire(s); err != nil {
				return err
			}
		}
	}
	return nil
}

func compareCJSIKey(a, b ledger.Key) int {
	return cmp.Or(cmp.Compare(a.Thread, b.Thread), cmp.Compare(a.Kind, b.Kind), cmp.Compare(a.ID, b.ID))
}

func cjsiOTelRequests(v ledger.Eligibility) ([]ledger.Key, error) {
	if v.State != ledger.LocalCalls || v.Pending {
		return nil, nil
	}
	var keys []ledger.Key
	sources := make(map[ledger.SourceStamp]bool, len(v.Sources))
	for _, s := range v.Sources {
		sources[s] = true
	}
	for _, m := range v.Members {
		if m.Key.Kind == ledger.TokenInterval {
			continue
		}
		if (m.Key.Kind != ledger.Response && m.Key.Kind != ledger.LegacyCall) || m.Key.Thread != v.Thread || m.Key.ID == "" ||
			m.At.IsZero() || m.Tokens.Input < 0 || m.Tokens.Output < 0 || m.Tokens.Input > math.MaxInt64-m.Tokens.Output || (m.Tokens.Input == 0 && m.Tokens.Output == 0) || len(m.Proof) == 0 {
			return nil, ledger.ErrProposalMismatch
		}
		for _, r := range m.Proof {
			prefix := r.Source.Prefix
			if !sources[r.Source] || r.Source.Name == ([32]byte{}) || r.Source.Epoch <= 0 || prefix.Version.Parser != ledger.Parser ||
				prefix.Identity == "" || prefix.Digest == ([32]byte{}) || prefix.Progress.CompleteEnd > prefix.Progress.ReadEnd ||
				r.Span.Start < 0 || r.Span.End <= r.Span.Start || r.Span.End > r.Source.Prefix.Progress.CompleteEnd {
				return nil, ledger.ErrProposalMismatch
			}
		}
		keys = append(keys, m.Key)
	}
	slices.SortFunc(keys, compareCJSIKey)
	return keys, nil
}

func planCJSIOTel(s cjsiOTelSelection, v ledger.Eligibility) (*cjsiOTelChange, error) {
	if s.Thread != v.Thread || s.Stamp != v.Stamp {
		return nil, ledger.ErrProposalMismatch
	}
	keys, err := cjsiOTelRequests(v)
	if err != nil {
		return nil, err
	}
	old := s.Period
	active := old != nil && old.Closed == (ledger.ProposalStamp{})
	if old != nil {
		if old.Thread != s.Thread || old.Version != 1 || old.Number <= 0 || len(old.Receipts) == 0 || len(old.Requests) == 0 ||
			old.Opened.Diff == ([32]byte{}) || old.Support.Diff == ([32]byte{}) {
			return nil, errCJSIOTelPrecondition
		}
		last := old.Closed
		if active {
			last = old.Support
		}
		// A second proposal for the same ticket, or a selection older than
		// this receipt, cannot reopen or amend it. A restored root must first
		// rebind the historical receipt to its new epoch.
		if last.Basis.CoreEpoch != s.Stamp.Basis.CoreEpoch || (s.Stamp != last &&
			(s.Stamp.Basis.BaseTicket < last.Basis.Ticket || s.Stamp.Basis.Ticket <= last.Basis.Ticket)) {
			return nil, errCJSIOTelPrecondition
		}
	}
	// A proved member can survive a partial replay. Outstanding overlap
	// cannot withdraw a safe side, release a receipt, or replace its support.
	if v.UnresolvedOverlap {
		return nil, nil
	}
	if len(keys) > 0 {
		return takeCJSIOTel(s, keys, active)
	}
	if !active || v.State != ledger.NoLocalCall || s.Release == nil {
		return nil, nil
	}
	r := s.Release
	removed := slices.Clone(r.Removed)
	slices.SortFunc(removed, compareCJSIKey)
	oldReqs := slices.Clone(old.Requests)
	slices.SortFunc(oldReqs, compareCJSIKey)
	if r.Thread != s.Thread || r.Current != s.Stamp || r.Support != old.Support || r.Number != old.Number ||
		!slices.Equal(removed, oldReqs) || len(removed) == 0 || r.InventoryAt.IsZero() {
		return nil, errCJSIOTelPrecondition
	}
	return releaseCJSIOTel(s)
}

func cjsiOTelAdmitted(o TelemetryObservation) bool {
	return o.Key != "" && o.Metered && o.InputTokens != nil && *o.InputTokens >= 0 && o.OutputTokens != nil && *o.OutputTokens >= 0 &&
		o.SupportEntry == strings.Join([]string{o.Producer, o.ProducerVersion, o.Originator, o.Transport}, "/") &&
		TelemetryAdmitted(o.Producer, o.ProducerVersion, o.Originator, o.Transport, o.EventName, o.EventKind)
}

func cjsiOTelSuppressed(o TelemetryObservation) TelemetryObservation {
	o.State, o.Reason = "jsonl_owned", "jsonl_evidence"
	return o
}

func cjsiOTelHasPriceInput(a CJSIOTelAttribution) bool {
	return a.Evidence != ([32]byte{}) && (a.PriceReference != "" || a.CatalogVersion != "")
}

func validCJSIOTelSettlement(thread string, o TelemetryObservation, part cjsiOTelSettled) bool {
	e, c := part.Event, part.Contribution
	at := o.RecordAt
	if at == "" {
		at = o.ReceivedAt
	}
	return o.State == "otel_owned" && cjsiOTelAdmitted(o) && ingest.CodexLegacySessionID(o.ThreadID) == thread &&
		c.Observation == o.Key && c.Event == e.Key && e.Key != "" && e.ID == o.Key && e.Client == "codex" &&
		canonicalCJSIThread(e.Thread) == thread && strings.HasPrefix(e.Path, TelemetrySourcePrefix) && e.Model == o.Model && e.At == at && at != "" &&
		e.Input == *o.InputTokens && e.Output == *o.OutputTokens && (part.Binding == nil || part.Binding.Event == e.Key) && cjsiOTelHasPriceInput(part.Attribution)
}

func takeCJSIOTel(s cjsiOTelSelection, keys []ledger.Key, active bool) (*cjsiOTelChange, error) {
	b := s.Before
	if len(b.Events) != len(b.Contributions) || len(b.Markers) > 1 || len(b.Sessions) > 1 {
		return nil, errCJSIOTelPrecondition
	}
	hasSettled := len(b.Events) > 0
	if hasSettled != (len(b.Markers) == 1) || (hasSettled && (active || len(b.OtherEvents) != 0 || len(b.Sessions) != 1)) {
		return nil, errCJSIOTelPrecondition
	}
	p := cjsiOTelPeriod{Version: 1, Number: 1, Thread: s.Thread, Opened: s.Stamp, Receipts: map[string]cjsiOTelReceipt{}}
	if active {
		p = *s.Period
		p.Receipts = maps.Clone(p.Receipts)
	} else if s.Period != nil {
		if s.Stamp == s.Period.Closed || s.Period.Number == math.MaxInt64 {
			return nil, errCJSIOTelPrecondition
		}
		p.Number = s.Period.Number + 1
	}
	p.Support, p.Requests = s.Stamp, keys
	change := cjsiOTelChange{period: p}
	if hasSettled {
		marker, session := b.Markers[0], b.Sessions[0]
		if marker.Kind != TelemetrySessionSourceKind || marker.Settled == "" || marker.Client != "codex" || session.Client != "codex" ||
			ingest.CodexLegacySessionID(marker.Thread) != s.Thread || session.Thread != marker.Thread {
			return nil, errCJSIOTelPrecondition
		}
		change.marker, change.session = &marker, &session
		change.period.Marker, change.period.Session = &marker, &session
	}
	events := map[string]cjsiOTelEvent{}
	for _, e := range b.Events {
		events[e.Key] = e
	}
	links := map[string]cjsiOTelContribution{}
	for _, c := range b.Contributions {
		links[c.Observation] = c
	}
	bindings := map[string]cjsiOTelBinding{}
	for _, binding := range b.Bindings {
		bindings[binding.Event] = binding
	}
	for _, o := range b.Observations {
		if o.State != "pending" && o.State != "otel_owned" {
			continue
		}
		if _, exists := p.Receipts[o.Key]; exists || !cjsiOTelAdmitted(o) || ingest.CodexLegacySessionID(o.ThreadID) != s.Thread {
			return nil, errCJSIOTelPrecondition
		}
		r := cjsiOTelReceipt{Original: o, Disposition: cjsiOTelApplied}
		if o.State == "otel_owned" {
			link := links[o.Key]
			part := cjsiOTelSettled{Contribution: link, Event: events[link.Event], Attribution: s.Attributions[link.Event]}
			if binding, ok := bindings[link.Event]; ok {
				part.Binding = &binding
				delete(bindings, link.Event)
			}
			if !validCJSIOTelSettlement(s.Thread, o, part) || change.marker == nil {
				return nil, errCJSIOTelPrecondition
			}
			r.Settled = &part
			delete(links, o.Key)
			delete(events, link.Event)
		}
		change.period.Receipts[o.Key] = r
		change.edits = append(change.edits, cjsiOTelEdit{o, cjsiOTelSuppressed(o), r.Settled})
	}
	if len(links)+len(events)+len(bindings) != 0 {
		return nil, errCJSIOTelPrecondition
	}
	if len(change.edits) == 0 && (!active || (s.Period.Support == s.Stamp && slices.Equal(s.Period.Requests, keys))) {
		return nil, nil
	}
	return &change, nil
}

func releaseCJSIOTel(s cjsiOTelSelection) (*cjsiOTelChange, error) {
	p := *s.Period
	b := s.Before
	// The receipt consumer never removes JSONL events. Verify their actual
	// exit by primary key before the first restoration write.
	if len(b.Events)+len(b.Contributions)+len(b.Bindings)+len(b.Markers) != 0 {
		return nil, errCJSIOTelPrecondition
	}
	p.Receipts = maps.Clone(p.Receipts)
	p.Closed = s.Stamp
	// A summary seen beside JSONL rows is the JSONL-era one their withdrawal
	// leaves behind. Any other summary must be left exactly as it was.
	change := cjsiOTelChange{period: p, restoring: true, otherEvents: slices.Clone(b.OtherEvents),
		summaries: slices.Clone(b.Sessions), retire: len(b.OtherEvents) != 0}
	current := map[string]TelemetryObservation{}
	for _, o := range b.Observations {
		current[o.Key] = o
	}
	var surviving []cjsiOTelReceipt
	originalCount := 0
	for _, key := range slices.Sorted(maps.Keys(p.Receipts)) {
		r := p.Receipts[key]
		o := r.Original
		if r.Disposition != cjsiOTelApplied || o.Key != key || ingest.CodexLegacySessionID(o.ThreadID) != s.Thread || !cjsiOTelAdmitted(o) {
			return nil, errCJSIOTelPrecondition
		}
		if r.Settled != nil {
			originalCount++
			if !validCJSIOTelSettlement(s.Thread, o, *r.Settled) {
				return nil, errCJSIOTelPrecondition
			}
		} else if o.State != "pending" {
			return nil, errCJSIOTelPrecondition
		}
		now, exists := current[key]
		if !exists {
			return nil, errCJSIOTelPrecondition
		}
		if now.State == "conflict" || now.State == "invalid" {
			r.Disposition = cjsiOTelInvalidated
			p.Receipts[key] = r
			continue
		}
		if !reflect.DeepEqual(now, cjsiOTelSuppressed(o)) {
			return nil, errCJSIOTelPrecondition
		}
		r.Disposition = cjsiOTelReturnedPending
		if r.Settled != nil {
			r.Disposition = cjsiOTelRestored
			received, err := time.Parse(time.RFC3339Nano, o.ReceivedAt)
			if err != nil || !s.Release.InventoryAt.After(received) || s.Attributions[r.Settled.Event.Key].canonical() != r.Settled.Attribution.canonical() {
				return nil, errCJSIOTelPrecondition
			}
			surviving = append(surviving, r)
		}
		p.Receipts[key] = r
		change.edits = append(change.edits, cjsiOTelEdit{now, o, r.Settled})
	}
	if originalCount == 0 {
		if p.Marker != nil || p.Session != nil {
			return nil, errCJSIOTelPrecondition
		}
		return &change, nil
	}
	if p.Marker == nil || p.Session == nil || p.Marker.Kind != TelemetrySessionSourceKind || p.Marker.Client != "codex" ||
		p.Session.Client != "codex" || p.Session.Thread != p.Marker.Thread || ingest.CodexLegacySessionID(p.Marker.Thread) != s.Thread ||
		p.Marker.Settled == "" {
		return nil, errCJSIOTelPrecondition
	}
	if len(surviving) == 0 {
		return &change, nil
	}
	if len(b.Sessions) > 1 || (len(b.Sessions) != 0 && len(b.OtherEvents) == 0) {
		return nil, errCJSIOTelPrecondition
	}
	marker, session := *p.Marker, *p.Session
	if len(surviving) != originalCount {
		first, latest := surviving[0].Settled.Event, surviving[0]
		publicThread := first.Thread
		support := surviving[0].Original
		for _, r := range surviving[1:] {
			e := r.Settled.Event
			publicThread = min(publicThread, e.Thread) // syncTelemetrySession uses min(session_id COLLATE BINARY).
			if e.At < first.At {
				first = e
			}
			// Match the original marker's support selection by record_at.
			// The stable receipt key orders ties.
			if r.Original.RecordAt > support.RecordAt {
				support = r.Original
			}
			last := latest.Settled.Event
			if e.At > last.At || (e.At == last.At && e.Key > last.Key) {
				latest = r
			}
		}
		marker.Thread, session.Thread = publicThread, publicThread
		marker.First, marker.Last, marker.Model, marker.Support = first.At, latest.Settled.Event.At, latest.Settled.Event.Model, support.SupportEntry
		session.First, session.Last = marker.First, marker.Last
	}
	change.marker, change.session = &marker, &session
	return &change, nil
}

func (change cjsiOTelChange) applyRows(ctx context.Context, tx *sql.Tx) error {
	err := change.visitEdits(func(edit cjsiOTelEdit) error {
		args := append([]any{edit.next.State, edit.next.Reason}, cjsiObservations.fields(&edit.old)...)
		return cjsiOTelChanged(tx.ExecContext(ctx, "UPDATE telemetry_observations SET state=?,reason=? WHERE "+cjsiObservations.exact(), args...))
	}, func(s *cjsiOTelSettled) error {
		if change.restoring {
			if err := cjsiEvents.insert(ctx, tx, s.Event); err != nil {
				return err
			}
			if err := cjsiContributions.insert(ctx, tx, s.Contribution); err != nil {
				return err
			}
			if s.Binding != nil {
				if err := cjsiBindings.insert(ctx, tx, *s.Binding); err != nil {
					return err
				}
			}
		} else {
			if s.Binding != nil {
				if err := cjsiBindings.remove(ctx, tx, *s.Binding); err != nil {
					return err
				}
			}
			if err := cjsiContributions.remove(ctx, tx, s.Contribution); err != nil {
				return err
			}
			if err := cjsiEvents.remove(ctx, tx, s.Event); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if change.marker == nil {
		return nil
	}
	if change.session == nil {
		return errors.New("missing Codex telemetry session receipt")
	}
	if change.restoring {
		if err := cjsiSessions.insert(ctx, tx, *change.session); err != nil {
			return err
		}
		return cjsiMarkers.insert(ctx, tx, *change.marker)
	}
	if err := cjsiMarkers.remove(ctx, tx, *change.marker); err != nil {
		return err
	}
	return cjsiSessions.remove(ctx, tx, *change.session)
}
