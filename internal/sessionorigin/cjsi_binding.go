package sessionorigin

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/kitdine/agent-deck/internal/ingest"
	"github.com/kitdine/agent-deck/internal/session"
	"github.com/kitdine/agent-deck/internal/store"
)

// OriginRequest is a retained, ownership-checked request observation, not a
// heuristic based on a session string or a token total. Components preserve
// absent vs present zero. The source owner must supply the complete candidates
// for the old first observation and its possible local counterparts (after
// verified mirror deduplication); omit the proof when that cannot be done.
type OriginRequest struct {
	Target     Target
	Source     SourceGeneration
	ResponseID string
	At         string
	Components ingest.CodexCounts
}

type LocalOriginRequest struct {
	Source SourceGeneration
	Record ingest.CodexResolvedRecord
}

// OriginChangeInputs belongs to one admitted comparison. Previous borrows the
// verified old side; the callback's EffectiveCopyReader is the proposed side.
// Marker is the *remaining* effective marker after the ownership decision.
type OriginChangeInputs struct {
	Target                      Target
	Previous                    EffectiveCopyReader
	Override                    *store.OriginOverride
	Origin                      *store.SessionOrigin
	Evidence                    []store.OriginEvidence
	Marker                      *store.TelemetrySessionOriginMarker
	TaskChild, IdentityConflict bool
	OldFirst                    []OriginRequest
	// OldLocal is required when the earlier birth came from native
	// execution. It is the complete retained local first-request proof
	// from the old admitted generation, not a reconstructed OTel row.
	OldLocal      []LocalOriginRequest
	LocalRequests []LocalOriginRequest
}

func generation(s session.SourceCopy) SourceGeneration {
	return SourceGeneration{Identity: s.Identity, PrefixHash: s.PrefixHash, ParserVersion: s.ParserVersion}
}

func nativeGeneration(g SourceGeneration) bool {
	return g.Identity != "" && g.PrefixHash != "" && g.ParserVersion > 0
}

func (t *effectiveTarget) hasSource(g SourceGeneration, telemetry bool) bool {
	for _, c := range t.copies {
		if session.IsTelemetrySource(c.Metadata.SourcePath) == telemetry && generation(c.Source) == g {
			return true
		}
	}
	return false
}

func validComponents(c ingest.CodexCounts) bool {
	if _, ok := c.Total(); !ok || c.Cached.Value > c.Input.Value || c.Reasoning.Value > c.Output.Value {
		return false
	}
	for _, v := range []ingest.CodexCount{c.Input, c.Output, c.Cached, c.CacheWrite, c.Reasoning, c.ReportedTotal} {
		if v.State == ingest.CodexAbsent && v.Value == 0 {
			continue
		}
		if v.State != ingest.CodexValid || v.Value < 0 {
			return false
		}
	}
	return true
}

func requestContinues(target Target, old, next *effectiveTarget, in OriginChangeInputs) bool {
	if len(in.OldFirst) != 1 {
		return false
	}
	a := in.OldFirst[0]
	if a.Target != target || strings.TrimSpace(a.ResponseID) == "" || instant(a.At) != old.birth.StartedAt ||
		!validComponents(a.Components) {
		return false
	}
	// An old native request must be from the generation that actually
	// supplied the old first execution, not just another copy of this ID.
	proved := false
	for _, c := range old.copies {
		if generation(c.Source) != a.Source {
			continue
		}
		if session.IsTelemetrySource(c.Metadata.SourcePath) {
			proved = c.Source.Identity == session.TelemetrySourcePath(target.Client, target.SessionID) &&
				instant(c.Source.FirstAt) == old.birth.StartedAt && len(old.copies) == 1
		} else if nativeGeneration(a.Source) && instant(c.HeadAt) == "" && instant(c.Birth) == old.birth.StartedAt {
			proved = len(in.OldLocal) == 1 && in.OldLocal[0].Source == a.Source && sameOriginRequest(in.OldLocal[0], a)
			for _, other := range old.copies {
				if !session.IsTelemetrySource(other.Metadata.SourcePath) && instant(other.HeadAt) != "" {
					proved = false
					break
				}
			}
		}
		if proved {
			break
		}
	}
	if !proved {
		return false
	}
	matches := 0
	for _, b := range in.LocalRequests {
		r := b.Record
		if r.Record.ResponseID != a.ResponseID {
			continue
		}
		if !sameOriginRequest(b, a) || !nativeGeneration(b.Source) || !next.hasSource(b.Source, false) {
			return false
		}
		matches++
	}
	return matches == 1
}

func sameOriginRequest(b LocalOriginRequest, a OriginRequest) bool {
	r := b.Record
	return r.Record.Kind == ingest.CodexRequestUsage && r.Local() && r.ThreadID == a.Target.SessionID &&
		r.Record.ResponseID == a.ResponseID && !r.Record.At.IsZero() && r.Record.At.UTC().Format(time.RFC3339Nano) == instant(a.At) && r.Record.Usage == a.Components
}

func inferBinding(target Target, old, next *effectiveTarget, in OriginChangeInputs) (digest, state string) {
	o := in.Override
	if o == nil {
		return "", ""
	}
	if o.Value == nil || o.BindingState != BindingMatched {
		return o.BindingDigest, o.BindingState
	}
	if old == nil || next == nil || in.IdentityConflict {
		return o.BindingDigest, BindingUnmatched
	}
	before, after := BindingFor(target.Client, target.SessionID, old.birth), BindingFor(target.Client, target.SessionID, next.birth)
	if before.Digest != o.BindingDigest || before.Limited != o.BindingLimited || before.Limited != after.Limited {
		return o.BindingDigest, BindingUnmatched
	}
	if !after.Limited && before.Digest == after.Digest {
		return before.Digest, BindingMatched
	}
	// Match the old birth's verified tier, without multiplying the two
	// copy sets. A source move can keep its generation; a rewrite cannot.
	type retained struct {
		gen  SourceGeneration
		at   string
		head bool
	}
	witness := func(c session.EffectiveSessionCopy) retained {
		at := instant(c.HeadAt)
		if at != "" {
			return retained{generation(c.Source), at, true}
		}
		return retained{generation(c.Source), instant(c.Birth), false}
	}
	records := make(map[retained]bool, len(old.copies))
	oldHasHead := false
	for _, c := range old.copies {
		if !session.IsTelemetrySource(c.Metadata.SourcePath) && instant(c.HeadAt) != "" {
			oldHasHead = true
			break
		}
	}
	for _, a := range old.copies {
		record := witness(a)
		if !session.IsTelemetrySource(a.Metadata.SourcePath) && nativeGeneration(record.gen) && record.head == oldHasHead && record.at == old.birth.StartedAt {
			records[record] = true
		}
	}
	for _, b := range next.copies {
		g := generation(b.Source)
		if !nativeGeneration(g) || session.IsTelemetrySource(b.Metadata.SourcePath) {
			continue
		}
		// A native limited binding stays local to an unchanged source.
		// For a changed complete birth, generation alone is insufficient.
		if records[witness(b)] {
			return after.Digest, BindingMatched
		}
	}
	if !after.Limited && requestContinues(target, old, next, in) {
		return after.Digest, BindingMatched
	}
	return o.BindingDigest, BindingUnmatched
}

// PreparedOrigin contains no file handles or reader. ApplyTx is deliberately
// not a writer entry point: the publication owner supplies the transaction,
// checks the rest of the ticket, and discards the whole transaction on error.
type PreparedOrigin struct {
	id         string
	expected   *store.OriginOverride
	digest     string
	state      string
	derivation *store.OriginDerivation
}

func (p PreparedOrigin) ApplyTx(ctx context.Context, tx *sql.Tx) error {
	return store.ApplyOriginProjectionTx(ctx, tx, p.id, p.expected, p.digest, p.state, p.derivation)
}

func PrepareEffectiveOrigin(ctx context.Context, id string, observedAt time.Time, with OriginView[OriginChangeInputs]) (PreparedOrigin, error) {
	target := Target{"codex", id}
	return readOriginView(with, func(v EffectiveCopyReader, in OriginChangeInputs) (PreparedOrigin, error) {
		if in.Target != target || in.Previous == nil || observedAt.IsZero() {
			return PreparedOrigin{}, ErrClassificationUnavailable
		}
		old, err := readEffectiveTarget(ctx, in.Previous, target)
		if err != nil {
			return PreparedOrigin{}, err
		}
		next, err := readEffectiveTarget(ctx, v, target)
		if err != nil {
			return PreparedOrigin{}, err
		}
		p := PreparedOrigin{id: id}
		if o := in.Override; o != nil {
			if o.Client != target.Client || o.SessionID != id {
				return PreparedOrigin{}, ErrClassificationUnavailable
			}
			row := *o
			if row.Value != nil {
				value := *row.Value
				row.Value = &value
			}
			p.expected = &row
		}
		p.digest, p.state = inferBinding(target, old, next, in)
		var previous store.SessionOrigin
		if in.Origin != nil {
			previous = *in.Origin
			if previous.Client != target.Client || previous.SessionID != id {
				return PreparedOrigin{}, ErrClassificationUnavailable
			}
		}
		for _, e := range in.Evidence {
			if e.Client != target.Client || e.SessionID != id || e.Adapter != SourceCodexOTelBackground {
				return PreparedOrigin{}, ErrClassificationUnavailable
			}
		}
		d := store.OriginDerivation{Client: target.Client, Adapter: SourceCodexOTelBackground, SessionIDs: []string{id}}
		if next == nil {
			if in.Origin != nil || len(in.Evidence) > 0 {
				p.derivation = &d
			}
			return p, nil
		}
		var gens []SourceGeneration
		var assessment Assessment
		if in.Marker != nil {
			// A path exclusion can hide just the projected copy while the
			// settled core marker still classifies an eligible native copy.
			if instant(in.Marker.SettledAt) == "" {
				return PreparedOrigin{}, ErrClassificationUnavailable
			}
			assessment = AssessCodexMarker(*in.Marker)
		} else if next.hasMarker() {
			return PreparedOrigin{}, ErrClassificationUnavailable
		}
		for _, c := range next.copies {
			gens = append(gens, generation(c.Source))
		}
		assessment.TaskChild = in.TaskChild
		o, evidence := deriveOrigin(originKey{target.Client, id}, gens, assessment, previous, in.Evidence, observedAt.UTC().Format(time.RFC3339Nano))
		if previous != o {
			d.Origins, d.Evidence = []store.SessionOrigin{o}, evidence
			p.derivation = &d
		}
		return p, nil
	})
}
