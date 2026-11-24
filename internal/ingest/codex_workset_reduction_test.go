package ingest

import (
	"context"
	"errors"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"
)

type cjsiReplayWorkset struct {
	cjsiCheckpointBatch
	sealed      bool
	passes      int
	terminalErr error
	alter       func(pass, index int, record *CodexRecord) error
}

var errCJSIWorksetUnsealed = errors.New("workset is not sealed")

func (w *cjsiReplayWorkset) OpenCodexWorkset(context.Context) (CodexWorksetNext, error) {
	if !w.sealed {
		return nil, errCJSIWorksetUnsealed
	}
	w.passes++
	pass, index := w.passes, 0
	return func(context.Context) (CodexRecord, CodexCheckpointEvidence, error) {
		if index == len(w.records) {
			if w.terminalErr != nil {
				return CodexRecord{}, CodexCheckpointEvidence{}, w.terminalErr
			}
			return CodexRecord{}, CodexCheckpointEvidence{}, io.EOF
		}
		record, evidence := w.records[index], w.evidence[index]
		if w.alter != nil {
			if err := w.alter(pass, index, &record); err != nil {
				return CodexRecord{}, CodexCheckpointEvidence{}, err
			}
		}
		index++
		return record, evidence, nil
	}, nil
}

type cjsiWorksetProofs struct {
	records  []CodexResolvedRecord
	segments []CodexSegmentProof
	count    int
	discard  bool
	aborts   int
	stage    func()
}

func (s *cjsiWorksetProofs) StageCodexProof(_ context.Context, record CodexResolvedRecord) error {
	s.count++
	if !s.discard {
		s.records = append(s.records, record)
	}
	if s.stage != nil {
		s.stage()
	}
	return nil
}

func (s *cjsiWorksetProofs) StageCodexSegment(_ context.Context, segment CodexSegmentProof) error {
	if !s.discard {
		s.segments = append(s.segments, segment)
	}
	return nil
}

func (s *cjsiWorksetProofs) Abort() {
	s.aborts++
	s.records, s.segments, s.count = nil, nil, 0
}

func cjsiVerifiedWorkset(t *testing.T, file *os.File, path string, end, budget int64, version CodexCheckpointVersion, resume *CodexCheckpointResume) (*CodexCheckpointReplay, *cjsiReplayWorkset, CodexCheckpointStep) {
	t.Helper()
	w := &cjsiReplayWorkset{}
	r, err := BeginCodexCheckpointWorkset(t.Context(), file, path, end, version, resume, w)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Abort()
	var offset int64
	for {
		step, err := r.Step(t.Context(), budget)
		if err != nil || step.Progress.ReadEnd-offset > budget {
			t.Fatalf("workset replay failed or exceeded step budget: %v", err)
		}
		offset = step.Progress.ReadEnd
		if step.Done {
			w.sealed = true
			return r, w, step
		}
	}
}

func TestCJSICodexWorksetReductionParity(t *testing.T) {
	data, err := os.ReadFile("testdata/cjsi_child.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	golden := string(data)
	cut := strings.LastIndex(golden, `{"type":"response_item","payload":{"type":"function_call_output"`) + 17
	file, path := cjsiCheckpointFile(t, golden[:cut])
	version := CodexCheckpointVersion{Generation: 2, Parser: 3, MaxLineBytes: 4096}
	limits := CodexWorksetLimits{SegmentRecords: 12, Turns: 4}

	check := func(r *CodexCheckpointReplay, input string) CodexWorksetReduction {
		t.Helper()
		proofs := &cjsiWorksetProofs{}
		result, err := r.ReduceWorkset(t.Context(), limits, proofs)
		if err != nil {
			t.Fatal(err)
		}
		records, _, progress := cjsiRead(t, input, 0, false)
		want := cjsiReduce(t, path, records, progress)
		got := CodexReduction{Source: result.Source(), Checkpoint: result.Checkpoint(), Records: proofs.records, Segments: proofs.segments}
		if !reflect.DeepEqual(got, want) {
			t.Fatal("segmented workset changed the cold reduction")
		}
		if err := result.Capture().Revalidate(t.Context()); err != nil {
			t.Fatal(err)
		}
		return result
	}

	before, w, step := cjsiVerifiedWorkset(t, file, path, int64(cut), 71, version, nil)
	partial := check(before, golden[:cut]).Checkpoint()
	if !partial.Partial || partial.ReplayFrom >= partial.CompleteEnd {
		t.Fatal("partial workset lost the earliest pending segment")
	}
	if _, err := file.WriteAt(data[cut:], int64(cut)); err != nil {
		t.Fatal(err)
	}
	after, fresh, final := cjsiVerifiedWorkset(t, file, path, int64(len(golden)), 71, version, cjsiCheckpointInput(step.Prefix, w.evidence))
	if ck := check(after, golden).Checkpoint(); ck.Partial || ck.ReplayFrom != ck.CompleteEnd {
		t.Fatal("right anchor and tool completion did not settle the open segment")
	}

	late := `{"type":"turn_context","payload":{"turn_id":"child-turn","model":"foreign-model"}}` + "\n"
	if _, err := file.WriteAt([]byte(late), int64(len(golden))); err != nil {
		t.Fatal(err)
	}
	combined := golden + late
	replayed, lateWorkset, _ := cjsiVerifiedWorkset(t, file, path, int64(len(combined)), 71, version, cjsiCheckpointInput(final.Prefix, fresh.evidence))
	check(replayed, combined)
	proofs := &cjsiWorksetProofs{}
	if _, err := replayed.ReduceWorkset(t.Context(), limits, proofs); err != nil || proofs.records[9].Local() {
		t.Fatal("late disproof did not remove the earlier bare reply")
	}
	if lateWorkset.passes != 6 {
		t.Fatalf("expected three verified passes per reduction, got %d", lateWorkset.passes)
	}
}

func TestCJSICodexWorksetReductionHighVolume(t *testing.T) {
	const groups = 1200
	head := `{"type":"session_meta","payload":{"id":"` + cjsiC + `"}}` + "\n"
	group := `{"type":"turn_context","payload":{"turn_id":"same","model":"synthetic"}}` + "\n" +
		`{"type":"response_item","payload":{"type":"function_call","call_id":"repeat"}}` + "\n" +
		`{"type":"response_item","payload":{"type":"function_call_output","call_id":"repeat"}}` + "\n"
	input := head + strings.Repeat(group, groups)
	file, path := cjsiCheckpointFile(t, input)
	version := CodexCheckpointVersion{Generation: 1, Parser: 1, MaxLineBytes: 256}
	replay, workset, _ := cjsiVerifiedWorkset(t, file, path, int64(len(input)), 997, version, nil)
	proofs := &cjsiWorksetProofs{discard: true}
	result, err := replay.ReduceWorkset(t.Context(), CodexWorksetLimits{SegmentRecords: 3, Turns: 1}, proofs)
	if err != nil || proofs.count != groups*3+1 || len(proofs.records) != 0 || workset.passes != 3 ||
		result.Checkpoint().CompleteEnd != int64(len(input)) || replay.binding != nil {
		t.Fatalf("high-volume workset did not reduce within three-record/one-turn limits: count=%d, error=%v", proofs.count, err)
	}
	if err := result.Capture().Revalidate(t.Context()); err != nil {
		t.Fatal(err)
	}
	proofs = &cjsiWorksetProofs{}
	if _, err := replay.ReduceWorkset(t.Context(), CodexWorksetLimits{SegmentRecords: 2, Turns: 1}, proofs); !errors.Is(err, ErrCodexWorksetBudget) ||
		proofs.count != 0 || proofs.aborts != 1 {
		t.Fatal("segment limit admitted or exposed an oversized segment")
	}
}

func TestCJSICodexWorksetReductionRejectsUnverifiedOutput(t *testing.T) {
	const line = `{"type":"turn_context","payload":{"turn_id":"other"}}` + "\n"
	head := `{"type":"session_meta","payload":{"id":"` + cjsiC + `"}}` + "\n"
	input := head + line + strings.Replace(line, "other", "next", 1)
	version := CodexCheckpointVersion{Generation: 1, Parser: 1, MaxLineBytes: 256}
	for _, name := range []string{"missing row", "changed late metadata", "turn limit", "canceled output", "changed source", "unsealed attempt"} {
		t.Run(name, func(t *testing.T) {
			file, path := cjsiCheckpointFile(t, input)
			replay, workset, _ := cjsiVerifiedWorkset(t, file, path, int64(len(input)), 75, version, nil)
			proofs := &cjsiWorksetProofs{}
			limits := CodexWorksetLimits{SegmentRecords: 4, Turns: 4}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			expected := ErrSourceChanged
			switch name {
			case "missing row":
				workset.alter = func(_, index int, _ *CodexRecord) error {
					if index == 2 {
						return io.EOF
					}
					return nil
				}
			case "changed late metadata":
				workset.alter = func(pass, index int, record *CodexRecord) error {
					if pass == 3 && index == 2 {
						record.Model = "not-from-source"
					}
					return nil
				}
			case "turn limit":
				limits.Turns = 1
				expected = ErrCodexWorksetBudget
			case "canceled output":
				proofs.stage = cancel
				expected = context.Canceled
			case "changed source":
				if err := file.Truncate(int64(len(input) - 1)); err != nil {
					t.Fatal(err)
				}
			case "unsealed attempt":
				workset.sealed = false
				expected = errCJSIWorksetUnsealed
			}
			got, err := replay.ReduceWorkset(ctx, limits, proofs)
			if !errors.Is(err, expected) || got != (CodexWorksetReduction{}) || proofs.count != 0 ||
				len(proofs.records) != 0 || len(proofs.segments) != 0 || proofs.aborts != 1 {
				t.Fatalf("invalid input retained provisional output: %v", err)
			}
		})
	}
}

func TestCJSICodexWorksetRetiredBeforeEOF(t *testing.T) {
	input := `{"type":"session_meta","payload":{"id":"` + cjsiC + `"}}` + "\n" +
		`{"type":"turn_context","payload":{"turn_id":"local-turn"}}` + "\n" +
		`{"type":"event_msg","payload":{"type":"task_complete","turn_id":"local-turn"}}` + "\n"
	file, path := cjsiCheckpointFile(t, input)
	version := CodexCheckpointVersion{Generation: 2, Parser: 3, MaxLineBytes: 256}
	replay, workset, _ := cjsiVerifiedWorkset(t, file, path, int64(len(input)), 71, version, nil)
	retired := errors.New("sealed attempt retired")
	proofs := &cjsiWorksetProofs{}
	var staged int
	proofs.stage = func() {
		staged++
		if staged == 3 {
			// All row bytes and lexical facts now match. The reader must
			// still confirm its seal when the reducer asks for EOF.
			workset.terminalErr = retired
		}
	}
	got, err := replay.ReduceWorkset(t.Context(), CodexWorksetLimits{SegmentRecords: 4, Turns: 2}, proofs)
	if staged != 3 || !errors.Is(err, retired) || got != (CodexWorksetReduction{}) ||
		proofs.aborts != 1 || len(proofs.records) != 0 || len(proofs.segments) != 0 {
		t.Fatalf("staged=%d err=%v reduction=%+v aborts=%d retained records=%d segments=%d",
			staged, err, got, proofs.aborts, len(proofs.records), len(proofs.segments))
	}
	t.Logf("retired reader after %d staged records: aborts=%d retained records=%d segments=%d",
		staged, proofs.aborts, len(proofs.records), len(proofs.segments))
}

type failingWorksetProofSink struct {
	cjsiWorksetProofs
	err error
}

func (s *failingWorksetProofSink) StageCodexProof(context.Context, CodexResolvedRecord) error {
	return s.err
}

func TestCJSIUnverifiedWorksetSink(t *testing.T) {
	const input = `{"type":"turn_context","payload":{"turn_id":"unverified"}}` + "\n"
	file, path := cjsiCheckpointFile(t, input)
	version := CodexCheckpointVersion{Generation: 1, Parser: 1, MaxLineBytes: 256}
	replay, _, _ := cjsiVerifiedWorkset(t, file, path, int64(len(input)), 64, version, nil)
	limits := CodexWorksetLimits{SegmentRecords: 2, Turns: 1}
	proofs := &cjsiWorksetProofs{}
	result, err := replay.ReduceWorkset(t.Context(), limits, proofs)
	if err != nil || result.Source().State == CodexSourceVerified || len(proofs.records) != 1 || proofs.records[0].Local() {
		t.Fatalf("unverified proof setup: source=%v, records=%v, err=%v", result.Source(), proofs.records, err)
	}
	sink := &failingWorksetProofSink{err: errors.New("private sink detail")}
	got, err := replay.ReduceWorkset(t.Context(), limits, sink)
	var wrapped codexCaptureVisitError
	if !errors.As(err, &wrapped) || !errors.Is(err, sink.err) || strings.Contains(err.Error(), sink.err.Error()) || sink.aborts != 1 || got != (CodexWorksetReduction{}) {
		t.Fatalf("unverified sink failure: err=%v, aborts=%d, result=%v", err, sink.aborts, got)
	}
}

func TestCJSIWorksetSinkOwnsProofSpans(t *testing.T) {
	data, err := os.ReadFile("testdata/cjsi_child.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	file, path := cjsiCheckpointFile(t, string(data))
	version := CodexCheckpointVersion{Generation: 1, Parser: 1, MaxLineBytes: 4096}
	replay, _, _ := cjsiVerifiedWorkset(t, file, path, int64(len(data)), 128, version, nil)
	proofs := &cjsiWorksetProofs{}
	if _, err := replay.ReduceWorkset(t.Context(), CodexWorksetLimits{SegmentRecords: 12, Turns: 4}, proofs); err != nil {
		t.Fatal(err)
	}
	// Several records refer to the same source span. Editing the sink's
	// copy of one proof must leave every other staged proof intact.
	seen := make(map[*CodexSpan]bool)
	for _, r := range proofs.records {
		for _, span := range []*CodexSpan{r.Context, r.RightAnchor, r.ToolStart} {
			if span != nil {
				if seen[span] {
					t.Fatalf("sink proofs share a mutable span %+v", *span)
				}
				seen[span] = true
			}
		}
	}
	if len(seen) < 2 {
		t.Fatal("fixture did not emit multiple proof spans")
	}
}
