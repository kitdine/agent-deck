package usage

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/sys/unix"

	"github.com/kitdine/agent-deck/internal/ingest"
	ledger "github.com/kitdine/agent-deck/internal/usage/cjsiledger"
)

// CJSIUsageSourceReplay authenticates a fixed, already sealed Task 2 workset.
// A coordinator can keep this handle across scan budgets. After restart it
// reopens the same durable attempt and reauthenticates its complete prefix.
type CJSIUsageSourceReplay struct {
	mu           sync.Mutex
	wall         InventoryEntry
	workset      CJSIUsageWorkset
	selectSource func(context.Context) (CJSIUsageWorkset, error)
	replay       *ingest.CodexCheckpointReplay
	file         *os.File
	ownFile      bool
	limits       ingest.CodexWorksetLimits
	maxMeters    int
	read         int64
	complete     *CJSIUsagePreparedSource
	closed       bool
}

func BeginCJSIUsageSourceReplay(ctx context.Context, entry InventoryEntry, selectSource func(context.Context) (CJSIUsageWorkset, error), open CJSIUsageOpenWorkset, limits ingest.CodexWorksetLimits, maxMeters int) (*CJSIUsageSourceReplay, error) {
	f, err := os.OpenFile(entry.Path, os.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	r, err := beginUsageReplay(ctx, entry, f, selectSource, open, limits, maxMeters)
	if err != nil {
		return nil, errors.Join(err, f.Close())
	}
	r.ownFile = true
	return r, nil
}
func beginUsageReplay(ctx context.Context, entry InventoryEntry, f *os.File, selectSource func(context.Context) (CJSIUsageWorkset, error), open CJSIUsageOpenWorkset, limits ingest.CodexWorksetLimits, maxMeters int) (*CJSIUsageSourceReplay, error) {
	if selectSource == nil || open == nil || f == nil || maxMeters <= 0 || limits.Turns <= 0 || limits.SegmentRecords <= 0 {
		return nil, ledger.ErrProposalMismatch
	}
	b, err := selectSource(ctx)
	if err != nil {
		return nil, err
	}
	s := b.Selected
	if b.Round <= 0 || b.Attempt <= 0 || entry.Client != "codex" || filepath.Ext(entry.Path) != ".jsonl" || entry.Path != s.Path || entry.Identity != s.Evidence.Identity || entry.Size != s.Evidence.Progress.ReadEnd || s.Evidence.Version.Parser != ledger.Parser {
		return nil, ledger.ErrProposalMismatch
	}
	wall, err := usageSourceWall(entry.Path)
	if err != nil {
		return nil, err
	}
	if wall.Identity != entry.Identity || wall.Size != entry.Size || !wall.Stable || entry.Stable && wall != entry {
		return nil, errUsageSourceChanged
	}
	replay, err := ingest.BeginCodexCheckpointWorkset(ctx, f, entry.Path, entry.Size, s.Evidence.Version, nil, sealedUsageWorkset{b, open})
	if err != nil {
		return nil, err
	}
	return &CJSIUsageSourceReplay{wall: wall, workset: b, selectSource: selectSource, replay: replay, file: f, limits: limits, maxMeters: maxMeters}, nil
}
func (r *CJSIUsageSourceReplay) closeFile() error {
	if r.replay != nil {
		r.replay.Abort()
		r.replay = nil
	}
	if r.ownFile && r.file != nil {
		f := r.file
		r.file = nil
		return f.Close()
	}
	return nil
}
func (r *CJSIUsageSourceReplay) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed = true
	r.complete = nil
	return r.closeFile()
}

// Step returns a zero candidate until the whole prefix, the sealed metadata
// passes and the original generation have been checked. Reusing a completed
// source costs no capture bytes but still checks the physical wall/selection.
func (r *CJSIUsageSourceReplay) Step(ctx context.Context, budget int64) (source CJSIUsagePreparedSource, consumed int64, done bool, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return source, 0, false, ledger.ErrProposalMismatch
	}
	defer func() {
		if err != nil {
			r.closed = true
			err = errors.Join(err, r.closeFile())
		}
	}()
	if err = ctx.Err(); err != nil {
		return
	}
	current, err := r.selectSource(ctx)
	if err != nil {
		return
	}
	if current != r.workset {
		return source, 0, false, ledger.ErrProposalMismatch
	}
	wall, err := usageSourceWall(r.wall.Path)
	if err != nil {
		return
	}
	if wall != r.wall {
		return source, 0, false, errUsageSourceChanged
	}
	if r.complete != nil {
		return *r.complete, 0, true, nil
	}
	if budget <= 0 {
		return source, 0, false, nil
	}
	step, err := r.replay.Step(ctx, budget)
	if err != nil {
		return
	}
	consumed = step.Progress.ReadEnd - r.read
	r.read = step.Progress.ReadEnd
	if !step.Done {
		return source, consumed, false, nil
	}
	reduced, err := ledger.StageWithLegacy(ctx, r.workset.Selected, func(ctx context.Context) (ingest.CodexSelectedSource, error) {
		b, err := r.selectSource(ctx)
		if err == nil && b != r.workset {
			err = ledger.ErrProposalMismatch
		}
		return b.Selected, err
	}, r.replay, r.limits, r.maxMeters, r.file, legacyUsageDecoder(r.wall.Path, r.maxMeters))
	if err != nil {
		return
	}
	wall, err = usageSourceWall(r.wall.Path)
	if err != nil {
		return
	}
	if wall != r.wall {
		return source, consumed, false, errUsageSourceChanged
	}
	if err = r.closeFile(); err != nil {
		return
	}
	source = CJSIUsagePreparedSource{entry: r.wall, binding: r.workset, source: reduced}
	r.complete = &source
	return source, consumed, true, nil
}
