package usage

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/kitdine/agent-deck/internal/ingest"
	ledger "github.com/kitdine/agent-deck/internal/usage/cjsiledger"
)

type CJSIUsageWorkset struct {
	Round, Attempt int64
	Selected       ingest.CodexSelectedSource
}
type CJSIUsageOpenWorkset func(context.Context) (CJSIUsageWorkset, ingest.CodexWorksetNext, error)
type CJSIUsagePreparedSource struct {
	entry   InventoryEntry
	binding CJSIUsageWorkset
	source  ledger.Source
}

// The real reader returns the identity of the physical sealed attempt. The
// freshly decoded metadata is authenticated by ingest against that reader.
type sealedUsageWorkset struct {
	binding CJSIUsageWorkset
	open    CJSIUsageOpenWorkset
}

func (sealedUsageWorkset) Abort() {}
func (sealedUsageWorkset) StageCodexRecord(ctx context.Context, _ ingest.CodexRecord, _ ingest.CodexCheckpointEvidence) error {
	return ctx.Err()
}
func (s sealedUsageWorkset) OpenCodexWorkset(ctx context.Context) (ingest.CodexWorksetNext, error) {
	b, next, err := s.open(ctx)
	if err != nil {
		return nil, err
	}
	if b != s.binding || next == nil {
		return nil, ledger.ErrProposalMismatch
	}
	return next, nil
}
func PrepareCJSIUsageCandidateSource(ctx context.Context, entry InventoryEntry, f *os.File, selectSource func(context.Context) (CJSIUsageWorkset, error), open CJSIUsageOpenWorkset, limits ingest.CodexWorksetLimits, maxMeters int, byteBudget int64) (CJSIUsagePreparedSource, error) {
	if byteBudget <= 0 {
		return CJSIUsagePreparedSource{}, ledger.ErrProposalMismatch
	}
	r, err := beginUsageReplay(ctx, entry, f, selectSource, open, limits, maxMeters)
	if err != nil {
		return CJSIUsagePreparedSource{}, err
	}
	defer r.Close()
	for {
		source, _, done, err := r.Step(ctx, byteBudget)
		if err != nil || done {
			return source, err
		}
	}

}

type CJSIUsageCandidate struct {
	runs        []ledger.RunRange
	checkpoints []CJSIUsageCheckpoint
	committed   []ledger.CommittedCall
	thread      string
	proposal    ledger.Proposal
	worksets    []CJSIUsageWorkset
	old         []ledger.OldRow
}

func (c CJSIUsageCandidate) Proposal() ledger.Proposal { return c.proposal }
func PrepareCJSIUsageCandidate(ctx context.Context, in ledger.ReplayInput, sources []CJSIUsagePreparedSource) (CJSIUsageCandidate, error) {
	c := CJSIUsageCandidate{runs: slices.Clone(in.History.Runs), committed: slices.Clone(in.History.Committed), thread: in.Thread, old: slices.Clone(in.History.Rows)}
	if len(in.Sources) != 0 {
		return CJSIUsageCandidate{}, ledger.ErrProposalMismatch
	}
	for _, s := range sources {
		progress, identity, anchor := s.source.Checkpoint()
		c.checkpoints = append(c.checkpoints, CJSIUsageCheckpoint{Entry: s.entry, Workset: s.binding, Progress: progress, HeadAt: identity.HeadTime, Birth: identity.Birth, Anchor: anchor})
		in.Sources = append(in.Sources, s.source)
		c.worksets = append(c.worksets, s.binding)
	}
	slices.SortFunc(c.worksets, func(a, b CJSIUsageWorkset) int { return strings.Compare(a.Selected.Path, b.Selected.Path) })
	var err error
	c.proposal, err = ledger.Replay(ctx, in)
	if err != nil {
		return CJSIUsageCandidate{}, err
	}
	return c, nil
}

func ReadCJSIOldUsage(ctx context.Context, q CJSIUsageDraftReader, key string) (ledger.OldImage, error) {
	var i ledger.OldImage
	err := q.QueryRowContext(ctx, `SELECT e.event_key,e.client,e.session_id,e.event_id,e.event_at,e.model,e.source_path,e.source_offset,e.input_tokens,e.output_tokens,e.cached_input_tokens,e.cache_write_tokens,e.cache_read_tokens,e.cache_creation_tokens,e.cache_write_5m_tokens,e.cache_write_1h_tokens,e.turn_index,e.run_id,b.run_id
 FROM main.usage_events e LEFT JOIN main.usage_run_bindings b USING(event_key) WHERE e.event_key=?`, key).Scan(&i.EventKey, &i.Client, &i.SessionID, &i.EventID, &i.At, &i.Model, &i.Path, &i.Offset, &i.Input, &i.Output, &i.Cached, &i.Write, &i.CacheRead, &i.CacheCreation, &i.Write5m, &i.Write1h, &i.TurnIndex, &i.RunID, &i.Binding)
	return i, err
}

func legacyUsageDecoder(path string, limit int) ledger.LegacyDecoder {
	state := parseState{codexCumulative: map[string]map[string]int64{}}
	return func(raw []byte, offset int64) (ledger.OldImage, bool, error) {
		var v map[string]any
		if json.Unmarshal(raw, &v) != nil {
			return ledger.OldImage{}, false, nil
		}
		e, ok := parse("codex", v, &state, path, offset)
		state.signals = nil
		if len(state.codexCumulative) > limit {
			return ledger.OldImage{}, false, ingest.ErrCodexWorksetBudget
		}
		if !ok || e.Key == "" {
			return ledger.OldImage{}, false, nil
		}
		at, err := time.Parse(time.RFC3339Nano, e.EventAt)
		if err != nil {
			return ledger.OldImage{}, false, nil
		}
		return ledger.OldImage{EventKey: e.Key, Client: e.Client, SessionID: e.SessionID, EventID: e.EventID, At: at.UTC().Format(time.RFC3339Nano), Model: e.Model, Path: e.SourcePath, Offset: offset, Input: e.Tokens["input_tokens"], Cached: e.Tokens["cached_input_tokens"], Output: e.Tokens["output_tokens"], Write: e.Tokens["cache_write_tokens"], TurnIndex: sql.NullInt64{Int64: int64(e.TurnIndex), Valid: e.TurnIndex > 0}}, true, nil
	}
}

// Re-check cheap physical-generation walls after waiting for the publication
// transaction. The captured byte digest was checked outside that transaction.
func usageSourceWall(path string) (InventoryEntry, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return InventoryEntry{}, err
	}
	if !info.Mode().IsRegular() {
		return InventoryEntry{}, errUsageSourceChanged
	}
	id, changed, stable := ingest.FileGeneration(info)
	return InventoryEntry{Client: "codex", Path: path, Identity: id, Size: info.Size(), ModifiedAt: info.ModTime().UnixNano(), ChangedAt: changed, Stable: stable}, nil
}
func (c CJSIUsageCandidate) validateCurrent(ctx context.Context) error {
	for _, p := range c.checkpoints {
		if err := ctx.Err(); err != nil {
			return err
		}
		live, err := usageSourceWall(p.Entry.Path)
		if err != nil {
			return err
		}
		if !live.Stable || live != p.Entry {
			return errUsageSourceChanged
		}
	}
	return ctx.Err()
}
