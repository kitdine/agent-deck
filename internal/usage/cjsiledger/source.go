package cjsiledger

import (
	"context"
	"crypto/sha256"
	"golang.org/x/sys/unix"
	"io"
	"os"

	"github.com/kitdine/agent-deck/internal/ingest"
)

// Source is the bounded, metering-only output of one C1 workset acceptance.
// No lexical observation, header, caller-assembled record or partial batch
// can be passed to Replay in place of this value.
type Source struct {
	checkpoint ingest.CodexCheckpoint
	identity   ingest.CodexSourceProof
	anchor     [32]byte
	stamp      SourceStamp
	thread     string
	meters     []meter
	gaps       Gap
	old        []legacyFact
}

func SelectionStamp(s ingest.CodexSelectedSource) SourceStamp {
	return SourceStamp{sha256.Sum256([]byte(s.Path)), s.Epoch, s.Evidence}
}

type meter struct {
	span        ingest.CodexSpan
	epochStart  int64
	modelFrom   int64
	turn, model string
	turnIndex   int
	record      ingest.CodexRecord
	legacyKey   string
	prefixHash  [32]byte // through this complete record, from byte zero
	epochHash   [32]byte // through the last hard source boundary
}

type sourceSink struct {
	Source
	limit      int
	model      string
	modelFrom  int64
	epochStart int64
}

func (s *sourceSink) Abort() { s.Source = Source{} }
func (*sourceSink) StageCodexSegment(ctx context.Context, _ ingest.CodexSegmentProof) error {
	return ctx.Err()
}

func (s *sourceSink) StageCodexProof(ctx context.Context, rr ingest.CodexResolvedRecord) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r := rr.Record
	// A local model applies only while every intervening record has the
	// same proved context. Returning to M after N starts a new run of M.
	model := ""
	if rr.Local() && rr.ThreadID == s.thread && rr.Context != nil && rr.Context.Start <= r.Span.Start {
		model = rr.Model
	}
	if model != s.model {
		s.model = model
		s.modelFrom = r.Span.Start
	}
	switch r.Kind {
	case ingest.CodexMalformed, ingest.CodexOversize, ingest.CodexBoundary:
		s.gaps |= IncompletePrefix
		s.epochStart = r.Span.End
		return nil
	case ingest.CodexSessionMeta:
		s.epochStart = r.Span.End
	}
	if rr.Execution == ingest.CodexForeign || r.BadIdentity || r.BadThread {
		s.epochStart = r.Span.End
	}
	if r.Kind != ingest.CodexRequestUsage && r.Kind != ingest.CodexLegacyUsage {
		return nil
	}
	if !rr.Local() || rr.ThreadID != s.thread {
		if rr.Execution != ingest.CodexForeign {
			s.gaps |= IncompletePrefix
		}
		s.epochStart = r.Span.End
		return nil
	}
	if len(s.meters) >= s.limit {
		return ingest.ErrCodexWorksetBudget
	}
	// Retain only meter fields, never even transient tool/document metadata.
	m := ingest.CodexRecord{Kind: r.Kind, At: r.At, ResponseID: r.ResponseID,
		Usage: r.Usage, ThreadTotal: r.ThreadTotal, TurnTotal: r.TurnTotal,
		LegacyLast: r.LegacyLast, LegacyTotal: r.LegacyTotal}
	modelFrom := s.modelFrom
	if s.model == "" {
		modelFrom = r.Span.End
	}
	s.meters = append(s.meters, meter{span: r.Span, epochStart: s.epochStart, turn: rr.TurnID, turnIndex: rr.TurnIndex, model: rr.Model, modelFrom: modelFrom, record: m})
	return nil
}

// StageSource consumes a terminal, sealed replay. It owns its proof sink,
// not the workset, descriptor or selection. The caller retires the workset
// after readers finish. On any error the return has no reusable facts.
func StageSource(ctx context.Context, selected ingest.CodexSelectedSource, current ingest.CodexCurrentSelection,
	replay *ingest.CodexCheckpointReplay, limits ingest.CodexWorksetLimits, maxMeters int) (Source, error) {
	f, err := os.OpenFile(selected.Path, os.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return Source{}, err
	}
	defer f.Close()
	return stage(ctx, selected, current, replay, limits, maxMeters, f, nil)
}

// StageWithLegacy also verifies compatibility keys and already-published v8
// observations against exactly the same prefix. Its private frozen v8
// decoder cannot manufacture new local execution or primary counts.
func StageWithLegacy(ctx context.Context, selected ingest.CodexSelectedSource, current ingest.CodexCurrentSelection,
	replay *ingest.CodexCheckpointReplay, limits ingest.CodexWorksetLimits, maxMeters int, file io.ReaderAt, decode LegacyDecoder) (Source, error) {
	if file == nil {
		return Source{}, ErrProposalMismatch
	}
	return stage(ctx, selected, current, replay, limits, maxMeters, file, decode)
}

func stage(ctx context.Context, selected ingest.CodexSelectedSource, current ingest.CodexCurrentSelection,
	replay *ingest.CodexCheckpointReplay, limits ingest.CodexWorksetLimits, maxMeters int, file io.ReaderAt, decode LegacyDecoder) (Source, error) {
	if current == nil || replay == nil || selected.Epoch <= 0 || selected.Evidence.Version.Parser != Parser || maxMeters <= 0 {
		return Source{}, ErrProposalMismatch
	}
	s := sourceSink{Source: Source{thread: selected.ThreadID}, limit: maxMeters}
	r, err := replay.ReduceWorkset(ctx, limits, &s)
	if err != nil {
		return Source{}, err
	}
	if r.Capture().Evidence() != selected.Evidence {
		return Source{}, ingest.ErrSourceChanged
	}
	s.stamp = SelectionStamp(selected)
	s.checkpoint = r.Checkpoint()
	s.identity = r.Source()
	if file != nil {
		if err := s.readLegacy(ctx, file, decode); err != nil {
			return Source{}, err
		}
	}
	if err := r.VerifySelectedSource(ctx, selected, current); err != nil {
		return Source{}, err
	}
	if p := r.Checkpoint(); p.Partial || p.ReplayFrom < p.CompleteEnd {
		s.gaps |= IncompletePrefix
	}
	return s.Source, nil
}

func (s Source) Checkpoint() (ingest.CodexCheckpoint, ingest.CodexSourceProof, [32]byte) {
	return s.checkpoint, s.identity, s.anchor
}
