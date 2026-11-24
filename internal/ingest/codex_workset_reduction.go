package ingest

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"hash"
	"io"
	"os"
)

// CodexCheckpointWorkset couples the provisional writer with its own sealed
// attempt reader. OpenCodexWorkset starts at zero on each call; the adapter
// checks the attempt's seal and owns any reader resources.
type CodexCheckpointWorkset interface {
	CodexCheckpointSink
	OpenCodexWorkset(context.Context) (CodexWorksetNext, error)
}

// CodexWorksetNext returns io.EOF only after confirming the sealed end.
// Any other error, including a failed final seal check, invalidates the pass.
type CodexWorksetNext func(context.Context) (CodexRecord, CodexCheckpointEvidence, error)

// CodexWorksetProofSink must keep all staged proofs provisional until
// ReduceWorkset succeeds. Abort discards the entire attempted reduction.
type CodexWorksetProofSink interface {
	StageCodexProof(context.Context, CodexResolvedRecord) error
	StageCodexSegment(context.Context, CodexSegmentProof) error
	Abort()
}

type CodexWorksetLimits struct {
	SegmentRecords int
	Turns          int
}

var ErrCodexWorksetBudget = errors.New("Codex workset reduction budget exhausted")

type codexWorksetCapture struct {
	reader       CodexCheckpointWorkset
	bytes, facts hash.Hash
	capture      CodexPrefixProof
	factDigest   [sha256.Size]byte
}

// BeginCodexCheckpointWorkset authenticates the staged lexical metadata while
// replay reads the source. Only fresh records contribute to the digest;
// persisted resume evidence cannot supply lexical fields. Like checkpoint
// replay, this mode retains no complete-prefix slice and requires defer Abort.
func BeginCodexCheckpointWorkset(ctx context.Context, file *os.File, path string, end int64, version CodexCheckpointVersion, resume *CodexCheckpointResume, workset CodexCheckpointWorkset) (*CodexCheckpointReplay, error) {
	replay, err := BeginCodexCheckpointReplay(ctx, file, path, end, version, resume, workset)
	if err != nil {
		return nil, err
	}
	replay.workset = &codexWorksetCapture{reader: workset, bytes: sha256.New(), facts: sha256.New()}
	return replay, nil
}

func (r *CodexCheckpointReplay) sealWorksetCapture() {
	w := r.workset
	w.capture = CodexPrefixProof{attempt: new(byte), file: r.file, path: r.path, identity: r.stamp.identity,
		progress: r.progress, maxLineBytes: r.version.MaxLineBytes, checkpointVersion: r.version}
	w.bytes.Sum(w.capture.digest[:0])
	w.facts.Sum(w.factDigest[:0])
	w.bytes, w.facts = nil, nil
}

func hashCodexWorksetRecord(h hash.Hash, record CodexRecord, evidence CodexCheckpointEvidence) error {
	for _, field := range [...]string{record.HeadID, record.LegacyID, record.ThreadID, record.TurnID,
		record.RootTurnID, record.ResponseID, record.ParentID, record.ForkedFromID, record.Model, record.CallID, record.Tool} {
		if len(field) > 128 {
			return ErrSourceChanged
		}
	}
	// This encoding is only compared within one in-process replay attempt.
	// The adapter continues to own its separate persisted representation.
	return json.NewEncoder(h).Encode(struct {
		Record   CodexRecord
		Evidence CodexCheckpointEvidence
	}{record, evidence})
}

// CodexWorksetReduction supplies the source and checkpoint of staged output
// without keeping the staged rows in memory or authorizing publication.
type CodexWorksetReduction struct {
	capture    CodexPrefixProof
	source     CodexSourceProof
	checkpoint CodexCheckpoint
}

func (r CodexWorksetReduction) Capture() CodexPrefixProof { return r.capture }
func (r CodexWorksetReduction) Source() CodexSourceProof  { return r.source }
func (r CodexWorksetReduction) Checkpoint() CodexCheckpoint {
	return r.checkpoint
}

// ReduceWorkset verifies each ordered pass against what this replay staged.
// Source identity and global turn ambiguity settle before the output pass,
// so late disproof can affect earlier segments. Memory is proportional to
// one segment and the distinct turn keys, both subject to explicit limits.
// The caller still owns source selection and final Capture().Revalidate.
func (r *CodexCheckpointReplay) ReduceWorkset(ctx context.Context, limits CodexWorksetLimits, sink CodexWorksetProofSink) (CodexWorksetReduction, error) {
	if sink == nil {
		return CodexWorksetReduction{}, errCodexRange
	}
	complete := false
	defer func() {
		if !complete {
			sink.Abort()
		}
	}()
	if !r.done || r.workset == nil || limits.SegmentRecords <= 0 || limits.Turns <= 0 {
		return CodexWorksetReduction{}, errCodexRange
	}
	if err := verifyCodexFileWall(ctx, r.file, r.path, r.end, r.stamp); err != nil {
		return CodexWorksetReduction{}, err
	}
	sourceScan := newCodexSourceScan(r.path)
	if err := r.scanWorkset(ctx, func(record CodexRecord) error {
		sourceScan.observe(record)
		return nil
	}); err != nil {
		return CodexWorksetReduction{}, err
	}
	source := sourceScan.proof()
	p := r.progress
	checkpoint := CodexCheckpoint{p.ReadEnd, p.CompleteEnd, p.CompleteEnd, p.Partial}
	if source.State != CodexSourceVerified {
		if source.State == CodexSourceUnknown {
			checkpoint.ReplayFrom = 0
		}
		if err := r.scanWorkset(ctx, func(record CodexRecord) error {
			return stageCodexWorksetProof(ctx, sink, CodexResolvedRecord{Record: record})
		}); err != nil {
			return CodexWorksetReduction{}, err
		}
	} else {
		var err error
		source, checkpoint, err = r.reduceWorksetSegments(ctx, limits, sink, source, checkpoint)
		if err != nil {
			return CodexWorksetReduction{}, err
		}
	}
	if err := verifyCodexFileWall(ctx, r.file, r.path, r.end, r.stamp); err != nil {
		return CodexWorksetReduction{}, err
	}
	complete = true
	return CodexWorksetReduction{capture: r.workset.capture, source: source, checkpoint: checkpoint}, nil
}

func stageCodexWorksetProof(ctx context.Context, sink CodexWorksetProofSink, record CodexResolvedRecord) error {
	if err := sink.StageCodexProof(ctx, copyCodexResolvedRecord(record)); err != nil {
		return codexCaptureVisitError{cause: err}
	}
	return nil
}

func (r *CodexCheckpointReplay) scanWorkset(ctx context.Context, visit func(CodexRecord) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	next, err := r.workset.reader.OpenCodexWorkset(ctx)
	if err != nil {
		return codexCaptureVisitError{cause: err}
	}
	if next == nil {
		return ErrSourceChanged
	}
	digest := sha256.New()
	var end int64
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		record, evidence, err := next(ctx)
		if err == io.EOF {
			break
		}
		if err != nil {
			return codexCaptureVisitError{cause: err}
		}
		if record.Span != evidence.Span || record.Span.Start != end ||
			record.Span.End <= end || record.Span.End > r.progress.CompleteEnd {
			return ErrSourceChanged
		}
		if err := hashCodexWorksetRecord(digest, record, evidence); err != nil {
			return err
		}
		if err := visit(record); err != nil {
			return err
		}
		end = record.Span.End
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if end != r.progress.CompleteEnd || [sha256.Size]byte(digest.Sum(nil)) != r.workset.factDigest {
		return ErrSourceChanged
	}
	return nil
}

func (r *CodexCheckpointReplay) reduceWorksetSegments(ctx context.Context, limits CodexWorksetLimits, sink CodexWorksetProofSink, source CodexSourceProof, checkpoint CodexCheckpoint) (CodexSourceProof, CodexCheckpoint, error) {
	uses := make(map[string]codexTurnUse)
	err := r.scanWorksetSegments(ctx, source.ThreadID, limits.SegmentRecords, func(records []CodexRecord, seg codexSegment) error {
		for _, record := range records {
			noteCodexTurn(uses, record, source.ThreadID)
			if len(uses) > limits.Turns {
				return ErrCodexWorksetBudget
			}
		}
		noteCodexBareStart(uses, records[0], seg)
		if len(uses) > limits.Turns {
			return ErrCodexWorksetBudget
		}
		return nil
	})
	if err != nil {
		return CodexSourceProof{}, CodexCheckpoint{}, err
	}
	turns := make(map[string]int)
	err = r.scanWorksetSegments(ctx, source.ThreadID, limits.SegmentRecords, func(records []CodexRecord, seg codexSegment) error {
		piece := CodexReduction{Source: source, Checkpoint: checkpoint, Records: make([]CodexResolvedRecord, len(records))}
		for i, record := range records {
			piece.Records[i].Record = record
		}
		piece.resolveSegment(seg, uses)
		piece.numberTurns(turns)
		source, checkpoint = piece.Source, piece.Checkpoint
		for _, record := range piece.Records {
			if err := ctx.Err(); err != nil {
				return err
			}
			if err := stageCodexWorksetProof(ctx, sink, record); err != nil {
				return err
			}
		}
		if err := sink.StageCodexSegment(ctx, copyCodexSegmentProof(piece.Segments[0])); err != nil {
			return codexCaptureVisitError{cause: err}
		}
		return nil
	})
	return source, checkpoint, err
}

func (r *CodexCheckpointReplay) scanWorksetSegments(ctx context.Context, thread string, limit int, visit func([]CodexRecord, codexSegment) error) error {
	var records []CodexRecord
	segment := codexSegment{context: -1}
	afterIdentityGap := false
	flush := func() error {
		if len(records) != 0 {
			segment.end = len(records)
			if err := visit(records, segment); err != nil {
				return err
			}
			clear(records)
			records = records[:0]
		}
		segment = codexSegment{context: -1, afterIdentityGap: afterIdentityGap}
		return nil
	}
	if err := r.scanWorkset(ctx, func(record CodexRecord) error {
		fence := codexSegmentFence(record, thread)
		if fence || segment.startsBefore(record) {
			if err := flush(); err != nil {
				return err
			}
		}
		if len(records) == limit {
			return ErrCodexWorksetBudget
		}
		if record.Kind == CodexTurnContext && !fence {
			segment.context = len(records)
		}
		if record.TurnID != "" && !fence {
			segment.turn = record.TurnID
		}
		records = append(records, record)
		if fence || record.Kind == CodexTurnEnd {
			afterIdentityGap = afterIdentityGap || record.losesCodexIdentity()
			return flush()
		}
		return nil
	}); err != nil {
		return err
	}
	segment.open = true
	return flush()
}
