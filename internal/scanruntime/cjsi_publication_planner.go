package scanruntime

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/kitdine/agent-deck/internal/ingest"
	"github.com/kitdine/agent-deck/internal/session"
	"github.com/kitdine/agent-deck/internal/sessionorigin"
	"github.com/kitdine/agent-deck/internal/store"
	"github.com/kitdine/agent-deck/internal/usage"
	"github.com/kitdine/agent-deck/internal/usage/cjsiledger"
)

// CJSIGroupInputs is the complete registered overlap group, including the
// original preconditions (even for retained rows), selected worksets and
// any OTel restoration evidence. A missing old selection cannot mean empty.
type CJSIGroupInputs struct {
	Thread      string
	History     cjsiledger.History
	Hints       cjsiledger.Gap
	Sources     []CJSISourcePreparation
	Receipts    func(cjsiledger.ProposalStamp) store.CJSIOTelReceiptStore
	RestoreOTel bool
}

// CJSIRoundSelection is supplied by the managed entry point's inventory and
// domain selectors. It contains evidence and reducer inputs, not SQL handles.
type CJSIRoundSelection struct {
	Opened       *store.CJSIOpenReceipt
	VerifyOpened func(*sql.Tx) error
	// Scope keys come from the registered overlap groups. A held selector
	// can return these keys alongside ErrTelemetrySourceIdentityPending.
	CoverageGroups       [][32]byte
	Inputs               store.CJSIPublicationInputs
	Changes              store.CJSICombinedChanges
	Groups               []CJSIGroupInputs
	Removed              []session.CJSISourceRemoval
	Telemetry            *[]session.TelemetrySession
	Result               store.CJSIRuntimeResult
	BuiltAt, AcceptUntil time.Time
	// Derive validity from the original allocation's clock on both a new
	// render and a restart. Interval calculation uses captured day/pricing
	// inputs outside the root locks and the core writer transaction.
	ValidUntil    func(time.Time) time.Time
	Build         CJSIServingBuilder
	OriginChanges []CJSIOriginChange
}
type CJSIOriginPlanner func(context.Context, *sql.Tx, store.CJSIIndexSnapshot, store.CJSIIndexSnapshot) ([]sessionorigin.PreparedOrigin, error)

// CJSIPlanner composes the owned domain APIs for both runtime scopes. The
// entry point supplies one registered selection per request; the core derives,
// stages, verifies and applies its material under the current election.
type CJSIPlanner struct {
	Select func(context.Context, CJSIRoundRequest, Scope) (CJSIRoundSelection, error)
}

func (p CJSIPlanner) prepare(ctx context.Context, req CJSIRoundRequest, scope Scope) (CJSIPublicationPlan, store.CJSIRuntimeResult, error) {
	var plan CJSIPublicationPlan
	var out store.CJSIRuntimeResult
	if p.Select == nil {
		return plan, out, store.ErrTelemetrySourceIdentityPending
	}
	in, err := p.Select(ctx, req, scope)
	if err != nil {
		if errors.Is(err, store.ErrTelemetrySourceIdentityPending) && ctx.Err() == nil {
			for _, key := range in.CoverageGroups {
				if e := req.Root.SetUsagePending(ctx, key, req.Election); e != nil {
					return plan, out, errors.Join(err, e)
				}
			}
		}
		return plan, out, err
	}
	if in.Opened != nil {
		if in.Inputs.Digest != in.Opened.Input || in.Changes.RecheckSources == nil {
			return plan, out, store.ErrTelemetrySourceIdentityPending
		}
		if e := in.Changes.RecheckSources(ctx); e != nil {
			return plan, out, e
		}
		if e := req.Root.ConfirmOpen(ctx, *in.Opened, time.Now(), in.VerifyOpened); e != nil {
			return plan, out, e
		}
		plan.alreadyOpened = true
		return plan, in.Result, nil
	}
	if in.Groups == nil || in.Changes.RecheckSources == nil {
		return plan, out, store.ErrTelemetrySourceIdentityPending
	}
	if err = in.Changes.RecheckSources(ctx); err != nil {
		return plan, out, err
	}
	in.Inputs.BuiltAt = in.BuiltAt
	begin := req.Root.BeginPublication
	if scope == ScopeUsage {
		begin = req.Root.BeginUsagePublication
	} else {
		if in.ValidUntil == nil {
			return plan, out, store.ErrTelemetrySourceIdentityPending
		}
		in.Inputs.SlotValidUntil = in.ValidUntil
	}
	d, err := begin(ctx, in.Inputs, in.AcceptUntil, req.Election)
	if err != nil {
		return plan, out, err
	}
	plan = CJSIPublicationPlan{Draft: d, Changes: in.Changes, Build: in.Build, BuiltAt: d.BuiltAt(), Groups: make([]CJSIPublicationGroup, 0, len(in.Groups))}
	if scope != ScopeUsage {
		plan.ValidUntil = d.SlotValidUntil()
		if in.OriginChanges == nil {
			return plan, out, store.ErrTelemetrySourceIdentityPending
		}
		plan.PrepareOrigins = func(ctx context.Context, tx *sql.Tx, old, next store.CJSIIndexSnapshot) ([]sessionorigin.PreparedOrigin, error) {
			return PreparePublicationOrigins(ctx, tx, old, next, d.BuiltAt(), in.OriginChanges)
		}
	}
	checkWalls := plan.Changes.CheckSourceWalls
	plan.Changes.CheckSourceWalls = func(ctx context.Context) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if checkWalls != nil {
			if e := checkWalls(ctx); e != nil {
				return e
			}
		}
		for _, group := range in.Groups {
			for _, s := range group.Sources {
				if e := ingest.Validate(s.Source); e != nil {
					return errors.Join(store.ErrTelemetrySourceIdentityPending, e)
				}
			}
		}
		return nil
	}
	if len(in.CoverageGroups) > 0 {
		apply := plan.Changes.ApplySelected
		if apply == nil {
			return plan, out, store.ErrTelemetrySourceIdentityPending
		}
		plan.Changes.ApplySelected = func(ctx context.Context, tx *sql.Tx) error {
			if e := apply(ctx, tx); e != nil {
				return e
			}
			for _, key := range in.CoverageGroups {
				if e := store.ClearUsagePendingTx(ctx, tx, key, d.Parent().CoverageRevision, in.Changes.CoverageRevision); e != nil {
					return e
				}
			}
			return nil
		}
	}
	var sources []session.CJSIPreparedSource
	seen := make(map[string]bool)
	groups := slices.Clone(in.Groups)
	slices.SortFunc(groups, func(a, b CJSIGroupInputs) int { return strings.Compare(a.Thread, b.Thread) })
	for i, g := range groups {
		if i > 0 && groups[i-1].Thread == g.Thread {
			return plan, out, store.ErrTelemetrySourceIdentityPending
		}

		if g.History.Rows == nil || g.Thread == "" || g.Receipts == nil {
			return plan, out, store.ErrTelemetrySourceIdentityPending
		}
		var prepared []usage.CJSIUsagePreparedSource
		var works []usage.CJSIUsageWorkset
		var stamps []cjsiledger.SourceStamp
		for _, input := range g.Sources {
			input.Index.ScannedAt = d.BuiltAt().Format(time.RFC3339Nano)
			if seen[input.Source.Path] || input.Index.Selected.ThreadID != g.Thread || !slices.Contains(req.Sources, input.Source) {
				return plan, out, store.ErrTelemetrySourceIdentityPending
			}
			seen[input.Source.Path] = true
			v, e := PreparePublicationSource(ctx, d, input, scope)
			if e != nil {
				return plan, out, e
			}
			prepared = append(prepared, v.Usage)
			works = append(works, v.Workset)
			stamps = append(stamps, cjsiledger.SelectionStamp(v.Workset.Selected))
			if scope != ScopeUsage {
				sources = append(sources, v.Index)
			}
		}
		candidate, e := usage.PrepareCJSIUsageCandidate(ctx, cjsiledger.ReplayInput{Basis: d.Basis(), Thread: g.Thread, Expected: stamps, History: g.History, Hints: g.Hints}, prepared)
		if e != nil {
			return plan, out, e
		}
		group := CJSIPublicationGroup{Usage: candidate, Draft: usage.CJSIUsageDraft{Thread: g.Thread, Stamp: candidate.Proposal().Stamp(), Worksets: works, ExpectedOldRows: slices.Clone(g.History.Rows)}, Receipts: g.Receipts(candidate.Proposal().Stamp()), RestoreOTel: g.RestoreOTel}
		plan.Groups = append(plan.Groups, group)
	}
	err = req.Root.WithPublishedCore(ctx, func(tx *sql.Tx, h store.CJSIIndexStamp) error {
		if h.Ticket != d.Parent().Ticket || h.CoreEpoch != d.Parent().CoreEpoch {
			return store.ErrCJSIIndexChanged
		}
		if e := in.Inputs.Validate(ctx, tx); e != nil {
			return e
		}
		for i := range plan.Groups {
			g := &plan.Groups[i]
			change, e := store.PrepareCJSIOTel(ctx, tx, g.Draft.Thread, g.Usage.Proposal(), g.Receipts)
			if e != nil {
				return e
			}
			g.OTel = change
		}
		if scope == ScopeUsage {
			return nil
		}
		return req.Root.WithIndexSnapshot(ctx, tx, func(v store.CJSIIndexSnapshot) error {
			if v.Selection() != d.Parent() {
				return store.ErrCJSIIndexChanged
			}
			restored, found, e := session.RecoverCJSIIndex(ctx, v, d.Basis().Ticket, d.Basis().Attempt, in.Changes.SessionRevision, sources, in.Removed, in.Telemetry)
			if e != nil {
				return e
			}
			if found {
				plan.Index = restored
				return nil
			}
			plan.Index, e = session.PlanCJSIIndex(ctx, v, d.Basis().Ticket, d.Basis().Attempt, in.Changes.SessionRevision, sources, in.Removed, in.Telemetry)
			return e
		})
	})
	if err != nil {
		return plan, out, err
	}
	return plan, in.Result, nil
}
func (p CJSIPlanner) Combined(ctx context.Context, r CJSIRoundRequest) (CJSIPublicationPlan, store.CJSIRuntimeResult, error) {
	return p.prepare(ctx, r, ScopeBoth)
}
func (p CJSIPlanner) UsageOnly(ctx context.Context, r CJSIRoundRequest) (store.CJSIRuntimeResult, error) {
	var zero store.CJSIRuntimeResult
	plan, out, err := p.prepare(ctx, r, ScopeUsage)
	if err != nil {
		return zero, err
	}
	if plan.alreadyOpened {
		return out, nil
	}
	c, err := plan.stageUsage(ctx)
	if err != nil {
		return zero, err
	}
	if err = plan.Draft.CompleteUsagePublication(ctx, c); err != nil {
		return zero, err
	}
	return out, nil
}

var _ CJSIRoundPlanner = CJSIPlanner{}

func (p CJSIPublicationPlan) stageUsage(ctx context.Context) (store.CJSICombinedChanges, error) {
	c := p.Changes
	if p.Groups == nil || c.Validate == nil || c.ApplySelected == nil {
		return c, store.ErrTelemetrySourceIdentityPending
	}
	c.Groups = make([]cjsiledger.Eligibility, 0, len(p.Groups))
	for _, g := range p.Groups {
		if g.Draft.Stamp.Basis != p.Draft.Basis() || g.Usage.Thread() != g.Draft.Thread {
			return c, store.ErrTelemetrySourceIdentityPending
		}
		v, e := g.Usage.Proposal().Eligibility(g.Draft.Stamp, g.Draft.Thread)
		if e != nil {
			return c, e
		}
		c.Groups = append(c.Groups, v)
	}
	err := p.Draft.StageUsage(ctx, func(db *sql.DB) error {
		for _, g := range p.Groups {
			if e := g.Usage.Stage(ctx, db, func(context.Context, usage.CJSIUsageDraftReader, string) (usage.CJSIUsageDraft, error) {
				return g.Draft, nil
			}); e != nil {
				return e
			}
		}
		return nil
	})
	if err != nil {
		return c, err
	}
	check, coverage := c.Validate, c.ApplySelected
	c.Validate = func(ctx context.Context, tx *sql.Tx) error {
		if err := check(ctx, tx); err != nil {
			return err
		}
		for _, g := range p.Groups {
			if err := g.Usage.VerifyStaged(ctx, tx, g.Draft); err != nil {
				return err
			}
		}
		return nil
	}
	c.ApplySelected = func(ctx context.Context, tx *sql.Tx) error {
		for _, group := range p.Groups {
			if err := group.apply(ctx, tx); err != nil {
				return err
			}
		}
		return coverage(ctx, tx)
	}
	return c, nil
}
