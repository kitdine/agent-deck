package ingest

import (
	"context"
	"crypto/sha256"
	"io"
	"os"
	"path/filepath"
)

type codexFileStamp struct {
	identity           string
	size, mtime, ctime int64
}

// CodexPrefixProof is an in-process proof of the exact bytes in [0, ReadEnd).
// The digest includes any incomplete suffix, but only CompleteEnd is a replay
// boundary. It says nothing about lexical coverage or record ownership.
type CodexPrefixProof struct {
	// Distinguish separate captures even when the bytes and descriptor match.
	attempt      *byte
	file         *os.File
	path         string
	identity     string
	digest       [sha256.Size]byte
	progress     CodexReadProgress
	maxLineBytes int
	// Populated only by verified checkpoint replay.
	checkpointVersion CodexCheckpointVersion
}

func (p CodexPrefixProof) Progress() CodexReadProgress {
	return p.progress
}

type codexCaptureVisitError struct{ cause error }

func (e codexCaptureVisitError) Error() string { return "Codex capture callback failed" }
func (e codexCaptureVisitError) Unwrap() error { return e.cause }

// CaptureCodexPrefix reads a fixed prefix from a caller-owned regular file.
// path must be the absolute plaintext source name approved by the caller's
// inventory. The descriptor stays open and its stream offset is unchanged.
// The callback may build a provisional batch, which must be discarded on
// error. Malformed records and oversize gaps remain lexical coverage issues.
func CaptureCodexPrefix(ctx context.Context, file *os.File, path string, end int64, maxLineBytes int, includeVisible bool, visit func(CodexRecord, *CodexVisible) error) (CodexPrefixProof, error) {
	if end < 0 || maxLineBytes <= 0 || visit == nil {
		return CodexPrefixProof{}, errCodexRange
	}
	if err := ctx.Err(); err != nil {
		return CodexPrefixProof{}, err
	}
	if !filepath.IsAbs(path) || filepath.Ext(path) != ".jsonl" {
		return CodexPrefixProof{}, ErrSourceChanged
	}
	before, err := inspectCodexFile(file, path, end)
	if err != nil {
		return CodexPrefixProof{}, err
	}

	digest := sha256.New()
	reader := io.TeeReader(io.NewSectionReader(file, 0, end), digest)
	progress, err := ReadCodexJSONL(ctx, reader, CodexCaptureBounds{End: end, MaxLineBytes: maxLineBytes}, includeVisible,
		func(record CodexRecord, visible *CodexVisible) error {
			if err := visit(record, visible); err != nil {
				return codexCaptureVisitError{cause: err}
			}
			return nil
		})
	if err != nil {
		if _, ok := err.(codexCaptureVisitError); ok {
			return CodexPrefixProof{}, err
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return CodexPrefixProof{}, ctxErr
		}
		return CodexPrefixProof{}, ErrSourceChanged
	}
	if progress.ReadEnd != end {
		return CodexPrefixProof{}, ErrSourceChanged
	}
	if err := verifyCodexFileWall(ctx, file, path, end, before); err != nil {
		return CodexPrefixProof{}, err
	}
	proof := CodexPrefixProof{attempt: new(byte), file: file, path: path, identity: before.identity, progress: progress, maxLineBytes: maxLineBytes}
	copy(proof.digest[:], digest.Sum(nil))
	return proof, nil
}

// CaptureCodexReduction binds the reduction to the records emitted by this
// successful capture. It does not collect visible text. The caller still
// owns the descriptor, source selection and final revalidation.
func CaptureCodexReduction(ctx context.Context, file *os.File, path string, end int64, maxLineBytes int) (CodexCapturedReduction, error) {
	var records []CodexRecord
	proof, err := CaptureCodexPrefix(ctx, file, path, end, maxLineBytes, false, func(record CodexRecord, _ *CodexVisible) error {
		records = append(records, record)
		return nil
	})
	if err != nil {
		return CodexCapturedReduction{}, err
	}
	return reduceCapturedCodex(ctx, proof, records)
}

func reduceCapturedCodex(ctx context.Context, proof CodexPrefixProof, records []CodexRecord) (CodexCapturedReduction, error) {
	reduction, err := ReduceCodexProof(proof.path, records, proof.Progress())
	if err != nil {
		return CodexCapturedReduction{}, err
	}
	if err := ctx.Err(); err != nil {
		return CodexCapturedReduction{}, err
	}
	return CodexCapturedReduction{capture: proof, reduction: reduction}, nil
}

// Revalidate compares the original extent using the same descriptor and name.
// A stable append beyond ReadEnd is allowed. Revalidate immediately before
// future publication; a successful check is not a lock against external writes
// after the final observation.
func (p CodexPrefixProof) Revalidate(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	end := p.progress.ReadEnd
	before, err := inspectCodexFile(p.file, p.path, end)
	if err != nil || before.identity != p.identity {
		return ErrSourceChanged
	}
	reader := io.NewSectionReader(p.file, 0, end)
	digest := sha256.New()
	var block [32 << 10]byte
	for remaining := end; remaining > 0; {
		if err := ctx.Err(); err != nil {
			return err
		}
		n := min(remaining, int64(len(block)))
		if _, err := io.ReadFull(reader, block[:n]); err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			return ErrSourceChanged
		}
		_, _ = digest.Write(block[:n])
		remaining -= n
	}
	if err := verifyCodexFileWall(ctx, p.file, p.path, end, before); err != nil {
		return err
	}
	if [sha256.Size]byte(digest.Sum(nil)) != p.digest {
		return ErrSourceChanged
	}
	return nil
}

func verifyCodexFileWall(ctx context.Context, file *os.File, path string, end int64, before codexFileStamp) error {
	after, err := inspectCodexFile(file, path, end)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	if err != nil || after != before {
		return ErrSourceChanged
	}
	return nil
}

func inspectCodexFile(file *os.File, path string, end int64) (codexFileStamp, error) {
	if file == nil {
		return codexFileStamp{}, ErrSourceChanged
	}
	opened, err := file.Stat()
	if err != nil {
		return codexFileStamp{}, ErrSourceChanged
	}
	named, err := os.Lstat(path)
	if err != nil || !opened.Mode().IsRegular() || !named.Mode().IsRegular() || opened.Size() < end {
		return codexFileStamp{}, ErrSourceChanged
	}
	id, changedAt, stable := FileGeneration(opened)
	nameID, nameChangedAt, nameStable := FileGeneration(named)
	if !stable || !nameStable || changedAt == 0 || id != nameID || changedAt != nameChangedAt ||
		opened.Size() != named.Size() || !opened.ModTime().Equal(named.ModTime()) {
		return codexFileStamp{}, ErrSourceChanged
	}
	return codexFileStamp{id, opened.Size(), opened.ModTime().UnixNano(), changedAt}, nil
}
