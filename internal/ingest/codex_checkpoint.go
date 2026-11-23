package ingest

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"hash"
	"io"
	"os"
	"path/filepath"
)

// CodexCheckpointVersion is assigned by the caller for this source and parser.
// The line policy is included because it changes the resulting observations.
type CodexCheckpointVersion struct {
	Generation   int64
	Parser       int
	MaxLineBytes int
}

// CodexCheckpointPrefix describes contiguous complete-line evidence starting
// at zero. It is an input claim until re-read and checked against the source.
type CodexCheckpointPrefix struct {
	Version     CodexCheckpointVersion
	Identity    string
	CompleteEnd int64
}

type CodexCheckpointEvidence struct {
	Span   CodexSpan
	Digest [sha256.Size]byte
}

type CodexCheckpointResume struct {
	Prefix CodexCheckpointPrefix
	// Next supplies complete lines in order and returns io.EOF exactly
	// at the end of the claimed prefix.
	Next func(context.Context) (CodexCheckpointEvidence, error)
}

// CodexCheckpointSink receives provisional observations, including gap facts.
// On failure or abandonment Abort must discard the whole attempted batch.
// The caller owns promotion after successful replay and later source checks.
type CodexCheckpointSink interface {
	StageCodexRecord(context.Context, CodexRecord, CodexCheckpointEvidence) error
	Abort()
}

type CodexCheckpointStep struct {
	// ReadEnd counts consumed bytes. CompleteEnd advances only after the sink
	// accepts a complete record. Error results are progress diagnostics only.
	Progress CodexReadProgress
	// Done means the fixed capture extent has been verified. Partial may
	// still be true. Prefix is usable only when Done is true. ReplayFrom
	// and publication remain decisions of the reducer and its callers.
	Done   bool
	Prefix CodexCheckpointPrefix
}

var errCodexReplayClosed = errors.New("Codex checkpoint replay closed")

// CodexCheckpointReplay re-lexes the source from zero in bounded steps.
// It never reads persisted parsed records. A partial line is held only in
// transient memory, up to MaxLineBytes, until completion or discard. This
// value and its sink are owned by a single caller, not concurrent workers.
type CodexCheckpointReplay struct {
	file    *os.File
	path    string
	stamp   codexFileStamp
	end     int64
	version CodexCheckpointVersion
	resume  CodexCheckpointResume
	sink    CodexCheckpointSink
	binding *codexReplayReduction
	workset *codexWorksetCapture

	progress CodexReadProgress
	line     []byte
	digest   hash.Hash
	oversize bool
	done     bool
	aborted  bool
}

// BeginCodexCheckpointReplay validates the caller's binding and takes
// ownership of the sink on success. file remains caller-owned. Restarting
// from a persisted checkpoint means re-verifying its prefix from byte zero.
// Defer replay.Abort after a successful Begin to clean up an abandoned
// replay, including one interrupted by a callback panic.
func BeginCodexCheckpointReplay(ctx context.Context, file *os.File, path string, end int64, version CodexCheckpointVersion, resume *CodexCheckpointResume, sink CodexCheckpointSink) (*CodexCheckpointReplay, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if end < 0 || version.Parser <= 0 || version.Generation <= 0 || version.MaxLineBytes <= 0 || sink == nil {
		return nil, errCodexRange
	}
	if !filepath.IsAbs(path) || filepath.Ext(path) != ".jsonl" {
		return nil, ErrSourceChanged
	}
	stamp, err := inspectCodexFile(file, path, end)
	if err != nil {
		return nil, err
	}
	replay := &CodexCheckpointReplay{file: file, path: path, stamp: stamp, end: end, version: version, sink: sink, digest: sha256.New()}
	if resume != nil {
		prefix := resume.Prefix
		if prefix.Version != version || prefix.Identity != stamp.identity ||
			prefix.CompleteEnd < 0 || prefix.CompleteEnd > end || resume.Next == nil {
			return nil, ErrSourceChanged
		}
		replay.resume = *resume
	}
	return replay, nil
}

// Abort abandons an unfinished replay. It is safe to call after Step fails.
func (r *CodexCheckpointReplay) Abort() {
	if r.sink == nil || r.done || r.aborted {
		return
	}
	r.aborted = true
	clear(r.line)
	r.line = nil
	r.digest = nil
	r.resume = CodexCheckpointResume{}
	r.binding = nil
	r.workset = nil
	r.sink.Abort()
}

// Step reads at most byteBudget source bytes, including verification of the
// old prefix. Oversized lines consume budget but retain only their digest and
// range. A changed source or an adapter failure aborts the whole batch.
// The file must stay unchanged across steps. Appends beyond the capture end
// also abort this attempt and require a fresh replay.
func (r *CodexCheckpointReplay) Step(ctx context.Context, byteBudget int64) (result CodexCheckpointStep, err error) {
	if r.sink == nil || r.aborted || r.done {
		return CodexCheckpointStep{}, errCodexReplayClosed
	}
	defer func() {
		result.Progress = r.progress
		if err != nil {
			r.Abort()
		}
	}()
	if byteBudget <= 0 {
		return result, errCodexRange
	}
	if err := verifyCodexFileWall(ctx, r.file, r.path, r.end, r.stamp); err != nil {
		return result, err
	}
	if err := r.finishResume(ctx); err != nil {
		return result, err
	}

	reader := io.NewSectionReader(r.file, r.progress.ReadEnd, min(byteBudget, r.end-r.progress.ReadEnd))
	var block [4096]byte
	for remaining := reader.Size(); remaining > 0; {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		n := min(remaining, int64(len(block)))
		if _, err := io.ReadFull(reader, block[:n]); err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return result, ctxErr
			}
			return result, ErrSourceChanged
		}
		remaining -= n
		if err := r.consume(ctx, block[:n]); err != nil {
			return result, err
		}
	}
	var reduction CodexCapturedReduction
	finished := r.progress.ReadEnd == r.end
	if finished {
		if r.resume.Next != nil {
			return result, ErrSourceChanged
		}
		if r.binding != nil {
			reduction, err = r.reduce(ctx)
			if err != nil {
				return result, err
			}
		}
	}
	if err := verifyCodexFileWall(ctx, r.file, r.path, r.end, r.stamp); err != nil {
		return result, err
	}
	if finished {
		if r.binding != nil {
			r.binding.result = reduction
		}
		if r.workset != nil {
			r.sealWorksetCapture()
		}
		clear(r.line)
		r.line = nil
		r.digest = nil
		r.done = true
		result.Done = true
		result.Prefix = CodexCheckpointPrefix{Version: r.version, Identity: r.stamp.identity, CompleteEnd: r.progress.CompleteEnd}
	}
	return result, nil
}

func (r *CodexCheckpointReplay) consume(ctx context.Context, data []byte) error {
	for len(data) != 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		newline := bytes.IndexByte(data, '\n')
		n := len(data)
		if newline >= 0 {
			n = newline + 1
		}
		fragment := data[:n]
		data = data[n:]
		_, _ = r.digest.Write(fragment)
		if r.binding != nil {
			_, _ = r.binding.digest.Write(fragment)
		}
		if r.workset != nil {
			_, _ = r.workset.bytes.Write(fragment)
		}
		r.progress.ReadEnd += int64(n)
		r.progress.Partial = newline < 0
		if !r.oversize && n > r.version.MaxLineBytes-len(r.line) {
			clear(r.line)
			r.line, r.oversize = nil, true
		}
		if !r.oversize {
			r.line = appendCodexFragment(r.line, fragment, r.version.MaxLineBytes)
		}
		if newline >= 0 {
			if err := r.stageLine(ctx); err != nil {
				return err
			}
			clear(r.line)
			r.line, r.oversize = r.line[:0], false
			r.digest.Reset()
		}
	}
	return nil
}

func (r *CodexCheckpointReplay) stageLine(ctx context.Context) error {
	span := CodexSpan{Start: r.progress.CompleteEnd, End: r.progress.ReadEnd}
	evidence := CodexCheckpointEvidence{Span: span}
	r.digest.Sum(evidence.Digest[:0])
	if r.resume.Next != nil {
		old, err := r.resume.Next(ctx)
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if err != nil || old != evidence || span.End > r.resume.Prefix.CompleteEnd {
			return ErrSourceChanged
		}
	}

	record := CodexRecord{Span: span, Kind: CodexOversize}
	if !r.oversize {
		record, _ = lexCodex(r.line, span, false)
	}
	if r.binding != nil && len(r.binding.records) == r.binding.limit {
		return ErrCodexReplayRecordBudget
	}
	if r.workset != nil {
		if err := hashCodexWorksetRecord(r.workset.facts, record, evidence); err != nil {
			return err
		}
	}
	if err := r.sink.StageCodexRecord(ctx, record, evidence); err != nil {
		return codexCaptureVisitError{cause: err}
	}
	if r.binding != nil {
		r.binding.records = append(r.binding.records, record)
	}
	r.progress.CompleteEnd = span.End
	return r.finishResume(ctx)
}

func (r *CodexCheckpointReplay) finishResume(ctx context.Context) error {
	if r.resume.Next == nil || r.progress.CompleteEnd != r.resume.Prefix.CompleteEnd {
		return nil
	}
	_, err := r.resume.Next(ctx)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	if err != io.EOF {
		return ErrSourceChanged
	}
	r.resume = CodexCheckpointResume{}
	return nil
}
