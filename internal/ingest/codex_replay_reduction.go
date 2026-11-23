package ingest

import (
	"context"
	"crypto/sha256"
	"errors"
	"hash"
	"os"
)

var ErrCodexReplayRecordBudget = errors.New("Codex replay reduction record budget exhausted")

type codexReplayReduction struct {
	limit   int
	records []CodexRecord
	digest  hash.Hash
	result  CodexCapturedReduction
}

// BeginCodexCheckpointReduction adds an in-memory full-prefix reduction to a
// budgeted checkpoint replay. At most maxRecords fresh lexical observations,
// including gaps, are retained; hitting the limit aborts the attempted batch.
// For larger worksets, use the streaming replay with a separate reducer.
// As with BeginCodexCheckpointReplay, defer Abort and keep the file open.
func BeginCodexCheckpointReduction(ctx context.Context, file *os.File, path string, end int64, version CodexCheckpointVersion, resume *CodexCheckpointResume, sink CodexCheckpointSink, maxRecords int) (*CodexCheckpointReplay, error) {
	if maxRecords <= 0 {
		return nil, errCodexRange
	}
	replay, err := BeginCodexCheckpointReplay(ctx, file, path, end, version, resume, sink)
	if err != nil {
		return nil, err
	}
	replay.binding = &codexReplayReduction{limit: maxRecords, digest: sha256.New()}
	return replay, nil
}

// CapturedReduction is available only after successful terminal Step in
// reduction mode. No independently supplied facts can be attached here.
// The caller still owns selected-source checks and final revalidation.
func (r *CodexCheckpointReplay) CapturedReduction() (CodexCapturedReduction, bool) {
	if !r.done || r.binding == nil {
		return CodexCapturedReduction{}, false
	}
	return r.binding.result, true
}

func (r *CodexCheckpointReplay) reduce(ctx context.Context) (CodexCapturedReduction, error) {
	binding := r.binding
	proof := CodexPrefixProof{attempt: new(byte), file: r.file, path: r.path, identity: r.stamp.identity,
		progress: r.progress, maxLineBytes: r.version.MaxLineBytes, checkpointVersion: r.version}
	copy(proof.digest[:], binding.digest.Sum(nil))
	result, err := reduceCapturedCodex(ctx, proof, binding.records)
	binding.records, binding.digest = nil, nil
	return result, err
}
