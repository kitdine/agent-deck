package store

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/ingest"
)

const cjsiWorksetChildID = "00000000-0000-4000-8000-000000000002"

var cjsiIntegrationLimits = ingest.CodexWorksetLimits{SegmentRecords: 16, Turns: 8}

type cjsiProvisionalProofs struct {
	records  []ingest.CodexResolvedRecord
	segments []ingest.CodexSegmentProof
	aborts   int
	onRecord func(context.Context) error
}

func (p *cjsiProvisionalProofs) StageCodexProof(ctx context.Context, record ingest.CodexResolvedRecord) error {
	p.records = append(p.records, record)
	if p.onRecord != nil {
		return p.onRecord(ctx)
	}
	return nil
}

func (p *cjsiProvisionalProofs) StageCodexSegment(_ context.Context, segment ingest.CodexSegmentProof) error {
	p.segments = append(p.segments, segment)
	return nil
}

func (p *cjsiProvisionalProofs) Abort() {
	p.aborts++
	p.records, p.segments = nil, nil
}

func cjsiWorksetIntegrationInput(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("../ingest/testdata/cjsi_child.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func cjsiFinishIntegrationReplay(t *testing.T, file *os.File, w *cjsiCheckpointWorkset, resume *ingest.CodexCheckpointResume) (*ingest.CodexCheckpointReplay, ingest.CodexCheckpointStep) {
	t.Helper()
	b := w.binding
	replay, err := ingest.BeginCodexCheckpointWorkset(t.Context(), file, file.Name(), b.CaptureEnd, b.Version, resume, w)
	if err != nil {
		t.Fatal(err)
	}
	defer replay.Abort()
	const budget = 97
	var readEnd int64
	for {
		step, err := replay.Step(t.Context(), budget)
		if err != nil || step.Progress.ReadEnd-readEnd > budget {
			t.Fatalf("checkpoint Step = %+v, %v", step, err)
		}
		if step.Done {
			return replay, step
		}
		readEnd = step.Progress.ReadEnd
	}
}

func cjsiCheckSQLiteReduction(t *testing.T, file *os.File, b cjsiCheckpointBinding, replay *ingest.CodexCheckpointReplay, proofs *cjsiProvisionalProofs) {
	t.Helper()
	var records []ingest.CodexRecord
	capture, err := ingest.CaptureCodexPrefix(t.Context(), file, file.Name(), b.CaptureEnd, b.Version.MaxLineBytes, false,
		func(record ingest.CodexRecord, _ *ingest.CodexVisible) error {
			records = append(records, record)
			return nil
		})
	if err != nil {
		t.Fatal(err)
	}
	evidence := capture.Evidence()
	want, err := ingest.ReduceCodexProof(file.Name(), records, evidence.Progress)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	got, err := replay.ReduceWorkset(ctx, cjsiIntegrationLimits, proofs)
	if err != nil {
		t.Fatal(err)
	}
	actual := ingest.CodexReduction{Source: got.Source(), Checkpoint: got.Checkpoint(), Records: proofs.records, Segments: proofs.segments}
	if !reflect.DeepEqual(actual, want) || len(proofs.segments) == 0 || proofs.aborts != 0 {
		t.Fatalf("SQLite reduction differs from fresh lexical reduction: source=%+v checkpoint=%+v records=%d segments=%d aborts=%d",
			got.Source(), got.Checkpoint(), len(proofs.records), len(proofs.segments), proofs.aborts)
	}
	evidence.Version = b.Version
	selected := ingest.CodexSelectedSource{Path: file.Name(), ThreadID: cjsiWorksetChildID, Epoch: 11, Evidence: evidence}
	if err := got.VerifySelectedSource(ctx, selected, nil); !errors.Is(err, ingest.ErrSourceChanged) {
		t.Fatalf("versioned selection without an authority returned %v", err)
	}
	current := func(context.Context) (ingest.CodexSelectedSource, error) { return selected, nil }
	if err := got.VerifySelectedSource(ctx, selected, current); err != nil {
		t.Fatalf("selected source did not verify: %v", err)
	}
	stale := selected
	stale.Epoch--
	if err := got.VerifySelectedSource(ctx, stale, current); !errors.Is(err, ingest.ErrSourceChanged) {
		t.Fatalf("stale authority epoch returned %v", err)
	}
}

func TestCJSICheckpointSQLiteReduction(t *testing.T) {
	input := cjsiWorksetIntegrationInput(t)
	cut := strings.LastIndex(input, `{"type":"response_item","payload":{"type":"function_call_output"`) + 17
	db, dbPath, file, b := newCJSIWorksetFixture(t, input[:cut])
	b.Version.MaxLineBytes = 4096
	first, err := beginCJSICheckpointWorkset(t.Context(), db, b)
	if err != nil {
		t.Fatal(err)
	}
	replay, step := cjsiFinishIntegrationReplay(t, file, first, nil)
	if !step.Progress.Partial {
		t.Fatal("fixture must stop at a partial line")
	}
	if _, err := first.OpenCodexWorkset(t.Context()); !errors.Is(err, errCJSICheckpointWorkset) {
		t.Fatalf("terminal Step without SQLite seal opened a pass: %v", err)
	}
	if err := first.Retire(t.Context()); !errors.Is(err, errCJSICheckpointWorkset) {
		t.Fatalf("sealed retirement accepted a draft: %v", err)
	}
	wrongStep := step
	wrongStep.Prefix.Version.Parser++
	if err := first.Seal(t.Context(), wrongStep); !errors.Is(err, errCJSICheckpointWorkset) {
		t.Fatalf("wrong seal accepted: %v", err)
	}
	if err := first.Seal(t.Context(), step); err != nil {
		t.Fatal(err)
	}
	// SQL in a proof callback must be able to use the only connection,
	// including when the reducer still has records left to read.
	db.SetMaxOpenConns(1)
	cjsiCheckSQLiteReduction(t, file, b, replay, &cjsiProvisionalProofs{
		onRecord: func(ctx context.Context) error {
			var count int
			return db.QueryRowContext(ctx, "SELECT count(*) FROM cjsi_checkpoint_attempts").Scan(&count)
		},
	})

	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db = openCJSIWorksetFixture(t, dbPath)
	for _, change := range []func(*cjsiCheckpointBinding){
		func(b *cjsiCheckpointBinding) { b.Round++ },
		func(b *cjsiCheckpointBinding) { b.Attempt++ },
		func(b *cjsiCheckpointBinding) { b.Identity = "0:1" },
		func(b *cjsiCheckpointBinding) { b.Version.Generation++ },
		func(b *cjsiCheckpointBinding) { b.Version.Parser++ },
		func(b *cjsiCheckpointBinding) { b.Version.MaxLineBytes++ },
		func(b *cjsiCheckpointBinding) { b.CaptureEnd++ },
	} {
		wrong := &cjsiCheckpointWorkset{db: db, binding: b}
		change(&wrong.binding)
		if _, err := wrong.OpenCodexWorkset(t.Context()); !errors.Is(err, errCJSICheckpointWorkset) {
			t.Fatalf("read accepted wrong binding %+v: %v", wrong.binding, err)
		}
		if err := wrong.Retire(t.Context()); !errors.Is(err, errCJSICheckpointWorkset) {
			t.Fatalf("retirement accepted wrong binding %+v: %v", wrong.binding, err)
		}
	}
	// Resume checks the old spans and digests and stages fresh lexical fields.
	// A valid but altered old model must not appear in the fresh attempt.
	result, err := db.Exec(`UPDATE cjsi_checkpoint_records SET metadata=replace(metadata,'child-model','stale-model')
		WHERE round_id=? AND attempt_id=? AND instr(metadata,'child-model')>0`, b.Round, b.Attempt)
	if err := cjsiCheckpointRequireRow(result, err); err != nil {
		t.Fatal(err)
	}
	resume, err := openCJSICheckpointResume(t.Context(), db, b)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteAt([]byte(input[cut:]), int64(cut)); err != nil {
		t.Fatal(err)
	}
	fresh := b
	fresh.Attempt++
	fresh.CaptureEnd = int64(len(input))
	second, err := beginCJSICheckpointWorkset(t.Context(), db, fresh)
	if err != nil {
		t.Fatal(err)
	}
	replay, step = cjsiFinishIntegrationReplay(t, file, second, resume)
	if err := second.Seal(t.Context(), step); err != nil {
		t.Fatal(err)
	}
	cjsiCheckSQLiteReduction(t, file, fresh, replay, &cjsiProvisionalProofs{})

	ctx := t.Context()
	abandoned, err := second.OpenCodexWorkset(ctx)
	if err != nil {
		t.Fatal(err)
	}
	record, evidence, err := abandoned(ctx)
	if err != nil {
		t.Fatal(err)
	}
	next, err := second.OpenCodexWorkset(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if again, proof, err := next(ctx); err != nil || again != record || proof != evidence || again.Span.Start != 0 {
		t.Fatalf("new pass did not restart after abandoned cursor: %v", err)
	}

	third := fresh
	third.Attempt++
	draft, err := beginCJSICheckpointWorkset(ctx, db, third)
	if err != nil {
		t.Fatal(err)
	}
	_, draftStep := cjsiFinishIntegrationReplay(t, file, draft, nil)
	for range 2 {
		if err := second.Retire(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := next(ctx); !errors.Is(err, errCJSICheckpointWorkset) {
		t.Fatalf("existing pass did not observe its retired seal: %v", err)
	}
	if _, err := second.OpenCodexWorkset(ctx); !errors.Is(err, errCJSICheckpointWorkset) {
		t.Fatalf("retired attempt remained readable: %v", err)
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM cjsi_checkpoint_records WHERE round_id=? AND attempt_id=?`,
		fresh.Round, fresh.Attempt).Scan(&count); err != nil || count != 0 {
		t.Fatalf("retired records = %d, %v", count, err)
	}
	if _, err := beginCJSICheckpointWorkset(ctx, db, fresh); err == nil {
		t.Fatal("retired attempt key was reusable")
	}
	prior := &cjsiCheckpointWorkset{db: db, binding: b}
	if err := draft.Seal(ctx, draftStep); err != nil {
		t.Fatalf("retirement removed sibling draft: %v", err)
	}
	for _, sibling := range []*cjsiCheckpointWorkset{prior, draft} {
		read, err := sibling.OpenCodexWorkset(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if got, proof, err := read(ctx); err != nil || got != record || proof != evidence {
			t.Fatalf("retirement affected sibling attempt %d: %v", sibling.binding.Attempt, err)
		}
	}
}

func TestCJSICheckpointSQLiteReductionFailures(t *testing.T) {
	input := cjsiWorksetIntegrationInput(t)
	db, _, file, b := newCJSIWorksetFixture(t, input)
	db.SetMaxOpenConns(1)
	b.Version.MaxLineBytes = 4096
	draft, err := beginCJSICheckpointWorkset(t.Context(), db, b)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := ingest.BeginCodexCheckpointWorkset(t.Context(), file, file.Name(), b.CaptureEnd, b.Version, nil, draft)
	if err != nil {
		t.Fatal(err)
	}
	defer replay.Abort()
	if _, err := replay.Step(t.Context(), int64(strings.IndexByte(input, '\n')+1)); err != nil {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := replay.Step(canceled, b.CaptureEnd); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled Step returned %v", err)
	}
	if _, err := draft.OpenCodexWorkset(t.Context()); !errors.Is(err, errCJSICheckpointWorkset) {
		t.Fatalf("canceled draft opened for reduction: %v", err)
	}
	var state string
	var count int
	if err := db.QueryRow(`SELECT state,(SELECT count(*) FROM cjsi_checkpoint_records)
		FROM cjsi_checkpoint_attempts`).Scan(&state, &count); err != nil || state != "aborted" || count != 0 {
		t.Fatalf("canceled attempt state=%q records=%d: %v", state, count, err)
	}

	b.Attempt++
	workset, err := beginCJSICheckpointWorkset(t.Context(), db, b)
	if err != nil {
		t.Fatal(err)
	}
	replay, step := cjsiFinishIntegrationReplay(t, file, workset, nil)
	if err := workset.Seal(t.Context(), step); err != nil {
		t.Fatal(err)
	}

	// Cancel during the output pass after a provisional proof was staged.
	ctx, stop := context.WithTimeout(t.Context(), 5*time.Second)
	defer stop()
	staged := 0
	proofs := &cjsiProvisionalProofs{onRecord: func(context.Context) error {
		staged++
		stop()
		return nil
	}}
	if got, err := replay.ReduceWorkset(ctx, cjsiIntegrationLimits, proofs); !errors.Is(err, context.Canceled) ||
		got != (ingest.CodexWorksetReduction{}) || staged != 1 || proofs.aborts != 1 || len(proofs.records)+len(proofs.segments) != 0 {
		t.Fatalf("canceled reduction returned %v with %d staged and %d aborts", err, staged, proofs.aborts)
	}
	// Retry against the same sealed attempt and single connection.
	cjsiCheckSQLiteReduction(t, file, b, replay, &cjsiProvisionalProofs{})

	// Corruption can be structurally valid. Keeping the row's byte digest
	// intact cannot authenticate changed lexical fields.
	result, err := db.Exec(`UPDATE cjsi_checkpoint_records SET metadata=replace(metadata,'child-model','stale-model')
		WHERE round_id=? AND attempt_id=? AND instr(metadata,'child-model')>0`, b.Round, b.Attempt)
	if err := cjsiCheckpointRequireRow(result, err); err != nil {
		t.Fatal(err)
	}
	proofs = &cjsiProvisionalProofs{}
	if got, err := replay.ReduceWorkset(t.Context(), cjsiIntegrationLimits, proofs); !errors.Is(err, ingest.ErrSourceChanged) ||
		got != (ingest.CodexWorksetReduction{}) || proofs.aborts != 1 || len(proofs.records)+len(proofs.segments) != 0 {
		t.Fatalf("corrupt reduction returned %v with %d aborts", err, proofs.aborts)
	}
	if err := workset.Retire(t.Context()); err != nil {
		t.Fatal(err)
	}
}
