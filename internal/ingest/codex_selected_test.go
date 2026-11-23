package ingest

import (
	"context"
	"errors"
	"os"
	"testing"
)

func TestCJSISelectedCodexProof(t *testing.T) {
	input := `{"type":"session_meta","payload":{"id":"` + cjsiC + `"}}` + "\n"
	file, path := cjsiCheckpointFile(t, input)
	ctx := t.Context()
	first, err := CaptureCodexReduction(ctx, file, path, int64(len(input)), 256)
	if err != nil {
		t.Fatal(err)
	}
	second, err := CaptureCodexReduction(ctx, file, path, int64(len(input)), 256)
	if err != nil {
		t.Fatal(err)
	}
	selected := CodexSelectedSource{Path: path, ThreadID: cjsiC, Evidence: first.Capture().Evidence(), Epoch: 4}
	current := func(context.Context) (CodexSelectedSource, error) { return selected, nil }
	if err := VerifySelectedCodexReduction(ctx, selected, first.Capture(), first, current); err != nil {
		t.Fatal(err)
	}
	if err := VerifySelectedCodexReduction(ctx, selected, second.Capture(), first, current); !errors.Is(err, ErrSourceChanged) {
		t.Fatal("another attempt could be substituted")
	}
	if err := VerifySelectedCodexReduction(ctx, selected, first.Capture(), first, nil); !errors.Is(err, ErrSourceChanged) {
		t.Fatal("selected epoch was used without a current-selection check")
	}
	alias := path + "-alias.jsonl"
	if err := os.Link(path, alias); err != nil {
		t.Fatal(err)
	}
	aliased := selected
	aliased.Path = alias
	if err := VerifySelectedCodexReduction(ctx, aliased, first.Capture(), first,
		func(context.Context) (CodexSelectedSource, error) { return aliased, nil }); !errors.Is(err, ErrSourceChanged) {
		t.Fatal("same-inode alias could replace the captured name")
	}

	version := CodexCheckpointVersion{Generation: 2, Parser: 3, MaxLineBytes: 256}
	replay, _, _ := cjsiVerifiedWorkset(t, file, path, int64(len(input)), 31, version, nil)
	workset, err := replay.ReduceWorkset(ctx, CodexWorksetLimits{SegmentRecords: 2, Turns: 1}, &cjsiWorksetProofs{})
	if err != nil {
		t.Fatal(err)
	}
	selected.Evidence = workset.Capture().Evidence()
	if err := workset.VerifySelectedSource(ctx, selected, current); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		version CodexCheckpointVersion
	}{
		{"parser", CodexCheckpointVersion{Generation: version.Generation, Parser: version.Parser + 1, MaxLineBytes: version.MaxLineBytes}},
		{"generation", CodexCheckpointVersion{Generation: version.Generation + 1, Parser: version.Parser, MaxLineBytes: version.MaxLineBytes}},
		{"omitted version", CodexCheckpointVersion{}},
	} {
		t.Run("workset rejects "+tc.name, func(t *testing.T) {
			wrong := selected
			wrong.Evidence.Version = tc.version
			// The selector agrees with the candidate; only the workset
			// proof can reject its version.
			agrees := func(context.Context) (CodexSelectedSource, error) { return wrong, nil }
			if err := workset.VerifySelectedSource(t.Context(), wrong, agrees); !errors.Is(err, ErrSourceChanged) {
				t.Fatalf("workset proof accepted wrong version or returned unexpected error: %v", err)
			}
		})
	}
	if _, err := file.WriteAt([]byte("{\"type\":\"synthetic\"}\n"), int64(len(input))); err != nil {
		t.Fatal(err)
	}
	if err := workset.VerifySelectedSource(ctx, selected, current); err != nil {
		t.Fatalf("stable append invalidated selected prefix: %v", err)
	}
}

func TestCJSISelectedCodexVisitDoesNotMintReplayPrefix(t *testing.T) {
	input := `{"type":"session_meta","payload":{"id":"` + cjsiC + `"}}` + "\n"
	file, path := cjsiCheckpointFile(t, input)
	ctx := t.Context()
	proof, err := CaptureCodexPrefix(ctx, file, path, int64(len(input)), 256, false, func(CodexRecord, *CodexVisible) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	selected := CodexSelectedSource{Path: path, ThreadID: cjsiC, Evidence: proof.Evidence()}
	selected.Evidence.Version = CodexCheckpointVersion{Generation: 9, Parser: 777, MaxLineBytes: 256}
	active := selected
	current := func(context.Context) (CodexSelectedSource, error) { return active, nil }
	limits := CodexSelectedLimits{MaxBytes: int64(len(input)), MaxRecords: 1, MaxLineBytes: 256}
	visits := 0
	visit := func(r CodexCapturedReduction) error {
		visits++
		if prefix, ok := r.ReplayPrefix(); ok || prefix != (CodexCheckpointPrefix{}) ||
			r.Capture().Evidence() != proof.Evidence() {
			t.Error("one-shot visit promoted declared version into a replay proof")
		}
		return nil
	}
	if err := VisitSelectedCodexReduction(ctx, selected, current, limits, visit); err != nil || visits != 1 {
		t.Fatalf("authoritative versioned visit did not finish: %v, visits=%d", err, visits)
	}
	if err := VisitSelectedCodexReduction(ctx, selected, nil, limits, visit); !errors.Is(err, ErrSourceChanged) || visits != 1 {
		t.Error("versioned selection did not require its current selector")
	}
	err = VisitSelectedCodexReduction(ctx, selected, current, limits, func(r CodexCapturedReduction) error {
		active.Evidence.Version.Parser++
		return visit(r)
	})
	if !errors.Is(err, ErrSourceChanged) || visits != 2 {
		t.Fatal("visit accepted a version change at its final selector check")
	}
}

func TestCJSISelectedCodexVisitRejectsStaleSelection(t *testing.T) {
	input := `{"type":"session_meta","payload":{"id":"` + cjsiC + `"}}` + "\n"
	for _, tc := range []string{"capacity", "record budget", "epoch changed", "canceled", "path changed during visit", "same size edit during visit"} {
		t.Run(tc, func(t *testing.T) {
			source := input + "{\"type\":\"synthetic\"}\n"
			file, path := cjsiCheckpointFile(t, source)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			p, err := CaptureCodexPrefix(ctx, file, path, int64(len(source)), 256, false, func(CodexRecord, *CodexVisible) error { return nil })
			if err != nil {
				t.Fatal(err)
			}
			selected := CodexSelectedSource{Path: path, ThreadID: cjsiC, Epoch: 4, Evidence: p.Evidence()}
			active := selected
			limits := CodexSelectedLimits{MaxBytes: int64(len(source)), MaxRecords: 3, MaxLineBytes: 256}
			want := ErrSourceChanged
			wantVisits := 1
			switch tc {
			case "capacity":
				limits.MaxBytes--
				want, wantVisits = ErrCodexWorksetBudget, 0
			case "record budget":
				limits.MaxRecords = 1
				want, wantVisits = ErrCodexReplayRecordBudget, 0
			case "canceled":
				want = context.Canceled
			}
			var visits int
			var opened *os.File
			err = VisitSelectedCodexReduction(ctx, selected, func(context.Context) (CodexSelectedSource, error) {
				return active, nil
			}, limits, func(r CodexCapturedReduction) error {
				visits++
				opened = r.Capture().file
				switch tc {
				case "epoch changed":
					active.Epoch++
				case "canceled":
					cancel()
				case "path changed during visit":
					if err := os.Rename(path, path+".old"); err != nil {
						t.Fatal(err)
					}
				case "same size edit during visit":
					if _, err := file.WriteAt([]byte("S"), int64(len(input)+len("{\"type\":\""))); err != nil {
						t.Fatal(err)
					}
				}
				return nil
			})
			if !errors.Is(err, want) || visits != wantVisits {
				t.Fatalf("incorrect acceptance or visit count: err=%v visits=%d", err, visits)
			}
			if opened != nil {
				if _, err := opened.Stat(); !errors.Is(err, os.ErrClosed) {
					t.Fatal("failed read left the selected descriptor open")
				}
			}
		})
	}
}
