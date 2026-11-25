package scanruntime

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"time"

	"github.com/kitdine/agent-deck/internal/ingest"
	"github.com/kitdine/agent-deck/internal/session"
	"github.com/kitdine/agent-deck/internal/sessionorigin"
	"github.com/kitdine/agent-deck/internal/store"
	"github.com/kitdine/agent-deck/internal/usage"
	"github.com/kitdine/agent-deck/internal/usage/cjsiledger"
)

// CJSIPublicationGroup combines a prepared ledger and its independently
// registered old selection with the OTel owner and optional binding projection.
type CJSIPublicationGroup struct {
	Usage       usage.CJSIUsageCandidate
	Draft       usage.CJSIUsageDraft
	OTel        *store.CJSIOTelChange
	Receipts    store.CJSIOTelReceiptStore
	RestoreOTel bool
}

func (g CJSIPublicationGroup) apply(ctx context.Context, tx *sql.Tx) error {
	b := g.Draft.Stamp.Basis
	if _, err := tx.ExecContext(ctx, `INSERT INTO cjsi_usage_selection(thread,ticket,attempt) VALUES(?,?,?)
 ON CONFLICT(thread) DO UPDATE SET ticket=excluded.ticket,attempt=excluded.attempt`, g.Draft.Thread, b.Ticket, b.Attempt); err != nil {
		return err
	}
	requests := func(t *sql.Tx) error {
		return g.Usage.ApplySelectedRequests(ctx, t, g.Draft.Stamp, g.Draft.ExpectedOldRows)
	}
	switch {
	case g.RestoreOTel:
		if g.OTel == nil {
			return store.ErrTelemetrySourceIdentityPending
		}
		if err := g.OTel.Apply(ctx, tx, g.Receipts, requests); err != nil {
			return err
		}
	default:
		if g.OTel != nil {
			if err := g.OTel.Apply(ctx, tx, g.Receipts, nil); err != nil {
				return err
			}
		}
		if err := requests(tx); err != nil {
			return err
		}
	}
	return nil
}

// CJSIProposedView binds the renderer's SQLite baseline, sealed index and
// affected ledger proposals to one immutable delta. Groups are the same
// prepared owners used by the final transaction.
type CJSIProposedView struct {
	Core                *sql.Tx
	Index               store.CJSIIndexSnapshot
	Groups              []CJSIPublicationGroup
	Origins             []sessionorigin.PreparedOrigin
	Digest              [32]byte
	FrozenDiff          [32]byte // exact attempts.diff_digest, distinct from the domain change digest
	Generation          store.ServingGeneration
	Slot                [16]byte
	BuiltAt, ValidUntil time.Time
}
type CJSIServingBuilder func(context.Context, CJSIProposedView) (func() ([]byte, error), error)

// CJSIPublicationPlan is prepared against Draft.Parent by the source and
// serving owners. Build borrows exactly one proposed session/core view. Its
// returned encoder owns detached values; no SQL handle survives the callback.
type CJSIPublicationPlan struct {
	alreadyOpened       bool
	PrepareOrigins      CJSIOriginPlanner
	Draft               store.CJSIPublicationDraft
	Index               *session.CJSIIndexCandidate
	Groups              []CJSIPublicationGroup
	Changes             store.CJSICombinedChanges
	Build               CJSIServingBuilder
	BuiltAt, ValidUntil time.Time
}

func (p CJSIPublicationPlan) Publish(ctx context.Context) error {
	if p.alreadyOpened {
		return nil
	}
	if p.Index == nil || p.Build == nil {
		return store.ErrTelemetrySourceIdentityPending
	}
	c, err := p.stageUsage(ctx)
	if err != nil {
		return err
	}
	list := p.Index.Withdrawals()
	removed := make([]store.CJSISourceWithdrawal, len(list))
	for i, r := range list {
		removed[i] = store.CJSISourceWithdrawal{Path: r.Path, Identity: r.Identity}
	}
	if err := p.Draft.StageIndex(ctx, p.Index, c.SessionRevision, removed); err != nil {
		return err
	}
	var origins []sessionorigin.PreparedOrigin
	if p.PrepareOrigins != nil {
		err := p.Draft.WithStagedViews(ctx, p.Index.Seal(), c.SessionRevision, c.CoverageRevision, removed, func(tx *sql.Tx, old, next store.CJSIIndexSnapshot) error {
			if err := c.Validate(ctx, tx); err != nil {
				return err
			}
			var e error
			origins, e = p.PrepareOrigins(ctx, tx, old, next)
			return e
		})
		if err != nil {
			return err
		}
		apply := c.ApplySelected
		c.ApplySelected = func(ctx context.Context, tx *sql.Tx) error {
			if e := apply(ctx, tx); e != nil {
				return e
			}
			for _, origin := range origins {
				if e := origin.ApplyTx(ctx, tx); e != nil {
					return e
				}
			}
			return nil
		}
	}
	frozen, err := p.Draft.FreezePublication(ctx, p.Index.Seal(), removed, c)
	if err != nil {
		return err
	}
	var encode func() ([]byte, error)
	err = p.Draft.WithProposedView(ctx, p.Index.Seal(), c.SessionRevision, c.CoverageRevision, removed, func(tx *sql.Tx, v store.CJSIIndexSnapshot) error {
		if err := c.Validate(ctx, tx); err != nil {
			return err
		}
		var err error
		encode, err = p.Build(ctx, CJSIProposedView{Core: tx, Index: v, Groups: slices.Clone(p.Groups), Origins: slices.Clone(origins), Digest: c.Digest, FrozenDiff: frozen.DiffDigest(), Generation: p.Draft.ServingGenerationFor(v.Selection(), c.UsageRevision), Slot: p.Draft.SlotIdentity(), BuiltAt: p.BuiltAt, ValidUntil: p.ValidUntil})
		return err
	})
	if err != nil {
		return err
	}
	if encode == nil {
		return store.ErrTelemetrySourceIdentityPending
	}
	data, err := encode()
	if err != nil {
		return err
	}
	return frozen.Open(ctx, data, p.BuiltAt, p.ValidUntil)
}

// CJSIRoundPlanner is supplied at process startup by the managed command
// wiring. Components never merge plans across a process or a worker election.
type CJSIRoundRequest struct {
	Root     store.CJSIRuntimeRoot
	Home     string
	Sources  []ingest.Source
	Election store.CJSIRuntimeElection
}
type CJSIRoundPlanner interface {
	Combined(context.Context, CJSIRoundRequest) (CJSIPublicationPlan, store.CJSIRuntimeResult, error)
	UsageOnly(context.Context, CJSIRoundRequest) (store.CJSIRuntimeResult, error)
}

func dispatchCJSIRound(ctx context.Context, planner CJSIRoundPlanner, scope Scope, req CJSIRoundRequest) (store.CJSIRuntimeResult, error) {
	var zero store.CJSIRuntimeResult
	if planner == nil {
		return zero, store.ErrTelemetrySourceIdentityPending
	}
	var out store.CJSIRuntimeResult
	var err error
	if scope == ScopeUsage {
		out, err = planner.UsageOnly(ctx, req)
	} else {
		var p CJSIPublicationPlan
		p, out, err = planner.Combined(ctx, req)
		if err == nil {
			err = p.Publish(ctx)
		}
	}
	if err != nil {
		if errors.Is(err, cjsiledger.ErrProposalMismatch) || errors.Is(err, ingest.ErrSourceChanged) {
			err = errors.Join(store.ErrTelemetrySourceIdentityPending, err)
		}
		return zero, store.PublicationOutcome(err)
	}
	return out, nil
}

// CJSISourcePreparation contains the verified capture and the original
// source-index metadata. MovedFrom must be the independently proved old
// pathname of this same physical source.
type CJSISourcePreparation struct {
	Source              ingest.Source
	Index               session.CJSISourceInput
	Current             ingest.CodexCurrentSelection
	Limits              ingest.CodexWorksetLimits
	MaxBytes, StepBytes int64
	MaxMeters           int
}
type CJSIPreparedSource struct {
	Usage   usage.CJSIUsagePreparedSource
	Index   session.CJSIPreparedSource
	Workset usage.CJSIUsageWorkset
}

func PreparePublicationSource(ctx context.Context, draft store.CJSIPublicationDraft, input CJSISourcePreparation, scope Scope) (CJSIPreparedSource, error) {
	var prepared CJSIPreparedSource
	if input.Current == nil || !scope.valid() {
		return prepared, store.ErrTelemetrySourceIdentityPending
	}
	src := input.Source
	err := draft.WithSourceWorkset(ctx, src, input.Index.Selected, input.StepBytes, func(work store.CJSICapturedWorkset) error {
		binding := usage.CJSIUsageWorkset{Round: work.Round, Attempt: work.Attempt, Selected: input.Index.Selected}
		current := func(ctx context.Context) (usage.CJSIUsageWorkset, error) {
			s, e := input.Current(ctx)
			if e != nil {
				return usage.CJSIUsageWorkset{}, e
			}
			if s != binding.Selected {
				return usage.CJSIUsageWorkset{}, store.ErrTelemetrySourceIdentityPending
			}
			return binding, nil
		}
		open := func(ctx context.Context) (usage.CJSIUsageWorkset, ingest.CodexWorksetNext, error) {
			r, e := work.Open(ctx)
			return binding, r, e
		}
		entry := usage.InventoryEntry{Client: src.Client, Path: src.Path, Identity: src.Identity, Size: src.Size, ModifiedAt: src.ModifiedAt, ChangedAt: src.ChangedAt, Stable: src.Stable}
		var e error
		prepared.Usage, e = usage.PrepareCJSIUsageCandidateSource(ctx, entry, work.File, current, open, input.Limits, input.MaxMeters, input.StepBytes)
		if e != nil {
			return e
		}
		prepared.Workset = binding
		if scope != ScopeUsage {
			in := input.Index
			in.Round, in.WorksetAttempt = work.Round, work.Attempt
			prepared.Index, e = session.PrepareCJSISource(ctx, work.File, work.Replay, input.Limits, input.MaxBytes, in, input.Current)
		}
		return e
	})
	if err != nil {
		return CJSIPreparedSource{}, err
	}
	return prepared, nil
}
