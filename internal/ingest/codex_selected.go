package ingest

import (
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// CodexPrefixEvidence describes the selected bytes, not a capture attempt.
// The caller obtains it from a verified proof or an authoritative selection.
type CodexPrefixEvidence struct {
	Identity     string
	Progress     CodexReadProgress
	Digest       [sha256.Size]byte
	MaxLineBytes int
	Version      CodexCheckpointVersion
}

func (p CodexPrefixProof) Evidence() CodexPrefixEvidence {
	return CodexPrefixEvidence{Identity: p.identity, Progress: p.progress, Digest: p.digest,
		MaxLineBytes: p.maxLineBytes, Version: p.checkpointVersion}
}

// CodexSelectedSource binds the selected name, thread and exact prefix.
// If Epoch or the evidence version is set, the caller must supply a
// current-selection reader.
type CodexSelectedSource struct {
	Path, ThreadID string
	Epoch          int64
	Evidence       CodexPrefixEvidence
}

// CodexCurrentSelection reads the caller's authoritative selection, not a
// selection assembled from the file being read.
type CodexCurrentSelection func(context.Context) (CodexSelectedSource, error)

type CodexSelectedLimits struct {
	MaxBytes     int64
	MaxLineBytes int
	MaxRecords   int
}

func (s CodexSelectedSource) valid() bool {
	e, p := s.Evidence, s.Evidence.Progress
	v := e.Version
	return filepath.IsAbs(s.Path) && filepath.Ext(s.Path) == ".jsonl" && s.ThreadID != "" &&
		s.Epoch >= 0 && e.Identity != "" && e.Digest != ([sha256.Size]byte{}) && e.MaxLineBytes > 0 &&
		p.CompleteEnd > 0 && p.ReadEnd >= p.CompleteEnd && p.Partial == (p.ReadEnd > p.CompleteEnd) &&
		(v == (CodexCheckpointVersion{}) || v.Generation > 0 && v.Parser > 0 && v.MaxLineBytes == e.MaxLineBytes)
}

func checkCodexSelection(ctx context.Context, selected CodexSelectedSource, current CodexCurrentSelection) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if current == nil {
		if selected.Epoch != 0 || selected.Evidence.Version != (CodexCheckpointVersion{}) {
			return ErrSourceChanged
		}
		return nil
	}
	actual, err := current(ctx)
	if err != nil {
		return err
	}
	if actual != selected {
		return ErrSourceChanged
	}
	return ctx.Err()
}

func (p CodexPrefixProof) matchesSelectedEvidence(expected CodexPrefixEvidence) bool {
	actual := p.Evidence()
	// One-shot captures prove bytes and line policy. The selector verifies
	// the declared version separately; only a replay can prove that version.
	if actual.Version == (CodexCheckpointVersion{}) {
		expected.Version = CodexCheckpointVersion{}
	}
	return actual == expected
}

func verifySelectedCodexCapture(ctx context.Context, selected CodexSelectedSource, capture CodexPrefixProof, source CodexSourceProof, current CodexCurrentSelection) error {
	if !selected.valid() || capture.attempt == nil || capture.path != selected.Path ||
		!capture.matchesSelectedEvidence(selected.Evidence) || source.State != CodexSourceVerified || source.ThreadID != selected.ThreadID {
		return ErrSourceChanged
	}
	stamp, err := inspectCodexFile(capture.file, selected.Path, selected.Evidence.Progress.ReadEnd)
	if err != nil {
		return err
	}
	if err := capture.Revalidate(ctx); err != nil {
		return err
	}
	if err := checkCodexSelection(ctx, selected, current); err != nil {
		return err
	}
	return verifyCodexFileWall(ctx, capture.file, selected.Path, selected.Evidence.Progress.ReadEnd, stamp)
}

// VerifySelectedCodexReduction verifies an existing captured reduction before
// an owner accepts its staged output. A proof from another attempt is invalid
// even when it describes identical bytes.
func VerifySelectedCodexReduction(ctx context.Context, selected CodexSelectedSource, capture CodexPrefixProof, reduction CodexCapturedReduction, current CodexCurrentSelection) error {
	if !reduction.MatchesCapture(capture) {
		return ErrSourceChanged
	}
	return verifySelectedCodexCapture(ctx, selected, capture, reduction.Source(), current)
}

// VerifySelectedMaterial checks a separate transient document capture against
// the bytes of this sealed workset and the current selection. Only the replay
// proves a checkpoint version. On error a joint consumer must discard both
// batches. On success it still must join material by local record and segment.
func (r CodexWorksetReduction) VerifySelectedMaterial(ctx context.Context, selected CodexSelectedSource, material CodexPrefixProof, current CodexCurrentSelection) error {
	expected := r.capture.Evidence()
	expected.Version = CodexCheckpointVersion{}
	if material.attempt == nil || r.capture.path != material.path || material.Evidence() != expected {
		return ErrSourceChanged
	}
	return r.VerifySelectedSource(ctx, selected, current)
}

// VerifySelectedSource rechecks the exact captured range before the owner
// accepts a workset reduction. The owner discards staged proofs on failure and
// remains responsible for any final publication CAS.
func (r CodexWorksetReduction) VerifySelectedSource(ctx context.Context, selected CodexSelectedSource, current CodexCurrentSelection) error {
	return verifySelectedCodexCapture(ctx, selected, r.capture, r.source, current)
}

// VisitSelectedCodexReduction opens and closes the selected file. It stages
// only a bounded, private reduction of the approved extent. visit must defer
// exposing its result until this function succeeds: the file and selection
// are checked again after the visit. This check does not pin a future publish.
func VisitSelectedCodexReduction(ctx context.Context, selected CodexSelectedSource, current CodexCurrentSelection, limits CodexSelectedLimits, visit func(CodexCapturedReduction) error) error {
	if !selected.valid() {
		return ErrSourceChanged
	}
	if limits.MaxBytes <= 0 || limits.MaxLineBytes <= 0 || limits.MaxRecords <= 0 || visit == nil {
		return errCodexRange
	}
	if selected.Evidence.Progress.ReadEnd > limits.MaxBytes {
		return ErrCodexWorksetBudget
	}
	if selected.Evidence.MaxLineBytes != limits.MaxLineBytes {
		return ErrSourceChanged
	}
	if err := checkCodexSelection(ctx, selected, current); err != nil {
		return err
	}

	fd, err := unix.Open(selected.Path, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return ErrSourceChanged
	}
	file := os.NewFile(uintptr(fd), "selected-codex-source")
	defer file.Close()
	end := selected.Evidence.Progress.ReadEnd
	stamp, err := inspectCodexFile(file, selected.Path, end)
	if err != nil || stamp.identity != selected.Evidence.Identity {
		return ErrSourceChanged
	}
	var records []CodexRecord
	proof, err := CaptureCodexPrefix(ctx, file, selected.Path, end, limits.MaxLineBytes, false, func(record CodexRecord, _ *CodexVisible) error {
		if len(records) == limits.MaxRecords {
			return ErrCodexReplayRecordBudget
		}
		records = append(records, record)
		return nil
	})
	if err != nil {
		return err
	}
	if !proof.matchesSelectedEvidence(selected.Evidence) {
		return ErrSourceChanged
	}
	reduction, err := reduceCapturedCodex(ctx, proof, records)
	if err != nil {
		return err
	}
	if source := reduction.Source(); source.State != CodexSourceVerified || source.ThreadID != selected.ThreadID {
		return ErrSourceChanged
	}
	if err := visit(reduction); err != nil {
		return err
	}
	return VerifySelectedCodexReduction(ctx, selected, proof, reduction, current)
}
