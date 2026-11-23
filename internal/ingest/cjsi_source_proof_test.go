package ingest

import (
	"context"
	"errors"
	"io"
	"os"
	"slices"
	"strings"
	"testing"
)

func TestCJSINativeSourceProof(t *testing.T) {
	raw, err := os.ReadFile("testdata/cjsi_child.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	golden := string(raw)
	t.Run("golden right anchor and complete-line checkpoint", func(t *testing.T) {
		cjsiGoldenProof(t, golden)
	})
	t.Run("source identity and relation evidence", func(t *testing.T) {
		cjsiIdentityProof(t, golden)
	})
	t.Run("ambiguous reused turn and hard boundary", cjsiAmbiguityProof)
	t.Run("threadless reply after request requires right anchor", cjsiNamedTurnBoundaryProof)
	t.Run("replay starts at the open pending segment", cjsiPendingReplayProof)
	t.Run("event identifiers reject paths", cjsiEventIDProof)
	t.Run("legacy and new token presence", cjsiCountsProof)
	t.Run("visible text whitelist", cjsiWhitelistProof)
	t.Run("malformed header cannot corroborate a filename", func(t *testing.T) {
		request := `{"type":"token_usage_record","payload":{"thread_id":"` + cjsiC + `","turn_id":"local"}}` + "\n"
		got := cjsiSource(t, cjsiCanonical(cjsiC), `{"type":"session_meta"}`+"\n"+request)
		if got.Source.State != CodexSourceUnknown || slices.ContainsFunc(got.Records, CodexResolvedRecord.Local) {
			t.Fatal("header without an object payload supplied source identity")
		}
	})
	t.Run("unidentified later header breaks ordinary attribution", func(t *testing.T) {
		head := `{"type":"session_meta","payload":{"id":"` + cjsiC + `"}}` + "\n"
		tool := `{"type":"response_item","payload":{"type":"function_call","call_id":"call"}}` + "\n"
		request := `{"type":"token_usage_record","payload":{"thread_id":"` + cjsiC + `","turn_id":"t"}}` + "\n"
		for _, boundary := range []string{`{"type":"session_meta","payload":{}}`, `{"type":"session_meta","payload":{"id":"invalid"}}`, "{broken"} {
			data := head + tool + boundary + "\n" + tool + request
			got := cjsiSource(t, cjsiCanonical(cjsiC), data)
			file, path := cjsiCheckpointFile(t, data)
			version := CodexCheckpointVersion{Generation: 1, Parser: 9, MaxLineBytes: 512}
			replay, _, _ := cjsiVerifiedWorkset(t, file, path, int64(len(data)), 31, version, nil)
			staged := &cjsiWorksetProofs{}
			_, err := replay.ReduceWorkset(t.Context(), CodexWorksetLimits{Turns: 2, SegmentRecords: 3}, staged)
			if err != nil || !slices.EqualFunc(got.Records, staged.records, func(a, b CodexResolvedRecord) bool { return a.Execution == b.Execution }) {
				t.Fatalf("workset and cold ownership differ across %q: %v", boundary, err)
			}
			if !got.Records[1].Local() || got.Records[3].Local() || !got.Records[4].Local() {
				t.Fatalf("independent facts and unresolved tool after %q: %v %v %v", boundary, got.Records[1].Execution, got.Records[3].Execution, got.Records[4].Execution)
			}
		}
	})
	t.Run("failed seal at end of evidence", cjsiFailedSealAtEOF)
	t.Run("shared selected prefix for usage and documents", func(t *testing.T) {
		cjsiSharedPrefix(t, golden)
	})
}

func cjsiSharedPrefix(t *testing.T, golden string) {
	const partial = "{\"partial\":"
	input := golden + partial
	file, path := cjsiCheckpointFile(t, input)
	version := CodexCheckpointVersion{Generation: 1, Parser: 9, MaxLineBytes: 4096}
	captureMaterial := func() (CodexPrefixProof, map[CodexSpan]CodexVisible) {
		t.Helper()
		material := make(map[CodexSpan]CodexVisible)
		capture, err := CaptureCodexPrefix(t.Context(), file, path, int64(len(input)), version.MaxLineBytes, true, func(_ CodexRecord, visible *CodexVisible) error {
			if visible != nil {
				material[visible.Span] = *visible
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		return capture, material
	}
	original, material := captureMaterial()
	selected := CodexSelectedSource{Path: path, ThreadID: cjsiC, Epoch: 7, Evidence: original.Evidence()}
	selected.Evidence.Version = version
	current := func(context.Context) (CodexSelectedSource, error) { return selected, nil }

	// Capture different material with identical inode, line spans and lengths.
	pos := strings.Index(input, "local-final-synthetic")
	if _, err := file.WriteAt([]byte("other-final-synthetic"), int64(pos)); err != nil {
		t.Fatal(err)
	}
	replaced, replacementText := captureMaterial()
	if _, err := file.WriteAt([]byte("local-final-synthetic"), int64(pos)); err != nil {
		t.Fatal(err)
	}

	replay, _, _ := cjsiVerifiedWorkset(t, file, path, int64(len(input)), 127, version, nil)
	staged := &cjsiWorksetProofs{}
	reduction, err := replay.ReduceWorkset(t.Context(), CodexWorksetLimits{SegmentRecords: 16, Turns: 4}, staged)
	if err != nil {
		t.Fatal(err)
	}
	checkpoint := reduction.Checkpoint()
	if !checkpoint.Partial || checkpoint.CompleteEnd != int64(len(golden)) || checkpoint.ReadEnd != int64(len(input)) {
		t.Fatalf("wrong complete and observed boundaries: %+v", checkpoint)
	}
	// Both test consumers receive the same staged rows. Expose their batch
	// only after the current selection and independent material agree.
	accept := func(capture CodexPrefixProof, docs map[CodexSpan]CodexVisible) (requests, intervals []int64, documents []string, err error) {
		if err = reduction.VerifySelectedMaterial(t.Context(), selected, capture, current); err != nil {
			return
		}
		for _, row := range staged.records {
			if !row.Local() || row.ThreadID != selected.ThreadID {
				continue
			}
			if row.Record.Kind == CodexRequestUsage {
				request, hasRequest := row.Record.Usage.Total()
				turn, hasTurn := row.Record.TurnTotal.Total()
				thread, hasThread := row.Record.ThreadTotal.Total()
				if !hasRequest || !hasTurn || !hasThread {
					t.Fatal("request or cumulative evidence was incomplete")
				}
				requests = append(requests, request)
				intervals = append(intervals, turn, thread)
			}
			doc, ok := docs[row.Record.Span]
			if ok && doc.Kind == row.Record.VisibleKind && (doc.Kind == CodexUserPrompt || doc.Kind == CodexAssistantFinal) &&
				slices.ContainsFunc(staged.segments, func(s CodexSegmentProof) bool {
					return s.ThreadID == row.ThreadID && s.TurnID == row.TurnID && s.Span.Start <= doc.Span.Start && doc.Span.End <= s.Span.End
				}) {
				documents = append(documents, doc.Text)
			}
		}
		return
	}
	requests, intervals, documents, err := accept(original, material)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(requests, []int64{35}) || !slices.Equal(intervals, []int64{35, 115}) ||
		!slices.Equal(documents, []string{"local-user-synthetic", "local-final-synthetic"}) {
		t.Fatalf("requests=%v cumulative=%v documents=%v", requests, intervals, documents)
	}
	requests, intervals, documents, err = accept(replaced, replacementText)
	if !errors.Is(err, ErrSourceChanged) || len(requests)+len(intervals)+len(documents) != 0 {
		t.Fatalf("same-length material replacement returned requests=%v cumulative=%v documents=%v err=%v", requests, intervals, documents, err)
	}
	// The material still matches the file, but the current authority moved.
	stale := selected
	stale.Epoch++
	current = func(context.Context) (CodexSelectedSource, error) { return stale, nil }
	requests, intervals, documents, err = accept(original, material)
	if !errors.Is(err, ErrSourceChanged) || len(requests)+len(intervals)+len(documents) != 0 {
		t.Fatalf("stale selection returned requests=%v cumulative=%v documents=%v err=%v", requests, intervals, documents, err)
	}
}

// A cursor can report that it reached the end but could not validate its
// attempt there. Neither a resume nor a workset pass may accept that result.
func cjsiFailedSealAtEOF(t *testing.T) {
	data := `{"type":"session_meta","payload":{"id":"` + cjsiC + `"}}` + "\n"
	version := CodexCheckpointVersion{Generation: 1, Parser: 9, MaxLineBytes: 512}
	file, path := cjsiCheckpointFile(t, data)
	work, rows, step := cjsiVerifiedWorkset(t, file, path, int64(len(data)), 32, version, nil)
	failedSeal := errors.New("seal check failed")
	terminal := errors.Join(io.EOF, failedSeal)
	staged := &cjsiWorksetProofs{stage: func() { rows.terminalErr = terminal }}
	proof, err := work.ReduceWorkset(t.Context(), CodexWorksetLimits{Turns: 1, SegmentRecords: 1}, staged)
	if !errors.Is(err, failedSeal) || proof != (CodexWorksetReduction{}) || staged.aborts != 1 || staged.count != 0 {
		t.Fatalf("workset accepted a failed final seal: err=%v aborts=%d retained=%d", err, staged.aborts, staged.count)
	}

	resume := cjsiCheckpointInput(step.Prefix, rows.evidence)
	next := resume.Next
	resume.Next = func(ctx context.Context) (CodexCheckpointEvidence, error) {
		evidence, err := next(ctx)
		if err == io.EOF {
			return evidence, terminal
		}
		return evidence, err
	}
	batch := &cjsiCheckpointBatch{}
	retry, err := BeginCodexCheckpointReplay(t.Context(), file, path, int64(len(data)), version, resume, batch)
	if err != nil {
		t.Fatal(err)
	}
	defer retry.Abort()
	end, err := retry.Step(t.Context(), int64(len(data)))
	if !errors.Is(err, ErrSourceChanged) || end.Done || batch.aborts != 1 || len(batch.records) != 0 {
		t.Fatalf("resume accepted a failed final seal: err=%v done=%t aborts=%d retained=%d", err, end.Done, batch.aborts, len(batch.records))
	}
}
