package usage

import (
	"context"
	"database/sql"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
	ledger "github.com/kitdine/agent-deck/internal/usage/cjsiledger"
)

// CJSIUsageReaders pins the effective selection for the complete invocation.
// The scoped reducer owns interval allocation, pricing and distinct sessions.
type CJSIUsageReaders interface {
	Summary(context.Context, time.Time, time.Time) (Summary, error)
	Stats(context.Context, StatsOptions) (StatsReport, error)
	Sessions(context.Context) ([]SessionSummary, error)
	Diagnose(context.Context) (map[string]any, error)
}

// The coordinator owns source handles across budgets and closes them on
// abandonment. Selection and Step never open the session index.
type CJSIUsageGroupPlan struct {
	Input      ledger.ReplayInput
	Sources    []*CJSIUsageSourceReplay
	ByteBudget int64
	Draft      CJSIUsageSelectDraft
}

// The coordinator selects the whole overlap group, including other sources
// and old contributions. Publish runs its final dependency CAS in the caller's
// transaction and invokes the supplied request writer only for the selected
// JSONL side with the independently registered full old selection. Checkpoint, public membership and coverage share that transaction.
// SelectGroup may return the pending sentinel after saving bounded progress.
type CJSIUsageInventory struct {
	Fingerprint string
	// Replay contains only source names with unresolved work, even when their
	// filesystem generation and most recent public checkpoint still match.
	Replay map[string]bool
}
type CJSIUsagePublication interface {
	PrepareSourceIdentities(context.Context) (bool, error)
	SelectGroup(context.Context, InventoryEntry) (CJSIUsageGroupPlan, error)
	Publish(context.Context, *sql.Tx, InventoryEntry, CJSIUsageCandidate, func([]ledger.OldRow) error) (pending bool, err error)
	UnchangedCoverage(context.Context, *sql.Tx, InventoryEntry, string) error
	ProtectMissing(context.Context, *sql.Tx, string) (bool, error)
	InventoryState(context.Context, []InventoryEntry) (CJSIUsageInventory, error)
}

func managedUsage(ctx context.Context, q usageQueryer) (bool, error) {
	var v int
	err := q.QueryRowContext(ctx, "SELECT version FROM main.schema_metadata").Scan(&v)
	if err == nil && v > 37 {
		return false, store.ErrSchemaAhead
	}
	return v == 37, err
}
func (s *Service) usageReaders(ctx context.Context) (CJSIUsageReaders, error) {
	if !s.cjsiManaged {
		return nil, nil
	}
	if s.CJSIReaders == nil {
		return nil, store.ErrTelemetrySourceIdentityPending
	}
	return s.CJSIReaders, nil
}
func (s *Service) prepareManagedUsage(ctx context.Context, e InventoryEntry) (CJSIUsageCandidate, error) {
	if s.CJSIPublication == nil {
		return CJSIUsageCandidate{}, store.ErrTelemetrySourceIdentityPending
	}
	plan, err := s.CJSIPublication.SelectGroup(ctx, e)
	if err != nil {
		return CJSIUsageCandidate{}, err
	}
	sources := make([]CJSIUsagePreparedSource, 0, len(plan.Sources))
	selectedEntry := false
	remaining := plan.ByteBudget
	if len(plan.Sources) > 0 && remaining <= 0 {
		return CJSIUsageCandidate{}, ledger.ErrProposalMismatch
	}
	for _, r := range plan.Sources {
		if r == nil {
			return CJSIUsageCandidate{}, ledger.ErrProposalMismatch
		}
		source, used, done, err := r.Step(ctx, remaining)
		if err != nil {
			return CJSIUsageCandidate{}, err
		}
		remaining -= used
		if !done {
			return CJSIUsageCandidate{}, store.ErrTelemetrySourceIdentityPending
		}
		sources = append(sources, source)
		selectedEntry = selectedEntry || source.entry == e
	}
	// A quarantined selection may have no accepted source; its assessment can
	// convey a gap, but never a positive request or a takeover.
	if !selectedEntry && (len(sources) != 0 || plan.Input.Hints&(ledger.UnverifiedSource|ledger.IdentityConflict) == 0) {
		return CJSIUsageCandidate{}, ledger.ErrProposalMismatch
	}
	c, err := PrepareCJSIUsageCandidate(ctx, plan.Input, sources)
	if err != nil {
		return CJSIUsageCandidate{}, err
	}
	if err = c.Stage(ctx, s.Store.DB, plan.Draft); err != nil {
		return CJSIUsageCandidate{}, err
	}
	return c, nil
}

func (s *Service) protectMissingUsage(ctx context.Context, tx *sql.Tx, path string) (bool, error) {
	if !s.cjsiManaged {
		return false, nil
	}
	if s.CJSIPublication != nil {
		return s.CJSIPublication.ProtectMissing(ctx, tx, path)
	}
	if s.sourceClient(store.UnassessedSource{Path: path}) == "claude" {
		return false, nil
	}
	return true, store.ErrTelemetrySourceIdentityPending
}

// NewCJSIUsage binds a core opened under the layout manager's lease. All
// ordinary store.Open variants enforce the old schema ceiling. The lease
// pins the schema for the lifetime of this handle, including read-only calls.
func NewCJSIUsage(ctx context.Context, core *store.Store, home string, readers CJSIUsageReaders, publication CJSIUsagePublication) (*Service, error) {
	if core == nil || core.DB == nil {
		return nil, ledger.ErrProposalMismatch
	}
	managed, err := managedUsage(ctx, core.DB)
	if err != nil {
		return nil, err
	}
	if !managed {
		return nil, ledger.ErrProposalMismatch
	}
	s := New(core, home)
	s.cjsiManaged = true
	s.CJSIReaders = readers
	s.CJSIPublication = publication
	return s, nil
}
