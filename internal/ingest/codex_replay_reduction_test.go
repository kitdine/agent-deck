package ingest

import (
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
)

func cjsiBoundReplay(t *testing.T, file *os.File, path string, end int64, version CodexCheckpointVersion, resume *CodexCheckpointResume) (CodexCapturedReduction, *cjsiCheckpointBatch) {
	t.Helper()
	batch := &cjsiCheckpointBatch{}
	replay, err := BeginCodexCheckpointReduction(t.Context(), file, path, end, version, resume, batch, 32)
	if err != nil {
		t.Fatal(err)
	}
	defer replay.Abort()
	const budget = 71
	var previousEnd int64
	for {
		step, err := replay.Step(t.Context(), budget)
		if err != nil {
			t.Fatal(err)
		}
		if step.Progress.ReadEnd != previousEnd+min(budget, end-previousEnd) {
			t.Fatal("bound reduction changed the Step byte budget")
		}
		previousEnd = step.Progress.ReadEnd
		result, available := replay.CapturedReduction()
		if available != step.Done {
			t.Fatal("proof became available before terminal verification")
		}
		if step.Done {
			prefix, bound := result.ReplayPrefix()
			if !bound || prefix != step.Prefix || result.Capture().Progress() != step.Progress ||
				!result.MatchesCapture(result.Capture()) || batch.aborts != 0 {
				t.Fatal("terminal result lost its exact capture binding")
			}
			return result, batch
		}
	}
}

func TestCJSIVerifiedReplayReduction(t *testing.T) {
	data, err := os.ReadFile("testdata/cjsi_child.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	golden := string(data)
	toolEnd := strings.LastIndex(golden, `{"type":"response_item","payload":{"type":"function_call_output"`)
	segmentStart := strings.Index(golden, `{"type":"turn_context","payload":{"turn_id":"child-turn"`)
	if toolEnd < 0 || segmentStart < 0 {
		t.Fatal("golden fixture lacks child segment or tool output")
	}
	cut := toolEnd + 17 // split the tool output before its first complete newline
	file, path := cjsiCheckpointFile(t, golden[:cut])
	version := CodexCheckpointVersion{Generation: 2, Parser: 3, MaxLineBytes: 4096}
	before, prior := cjsiBoundReplay(t, file, path, int64(cut), version, nil)
	ck := before.Checkpoint()
	if !ck.Partial || ck.ReadEnd != int64(cut) || ck.CompleteEnd != int64(toolEnd) ||
		ck.ReplayFrom != int64(segmentStart) ||
		before.Capture().digest != sha256.Sum256(data[:cut]) {
		t.Fatalf("unexpected partial replay checkpoint: %+v", ck)
	}
	if before.reduction.Records[9].Local() || len(before.reduction.Records) != 10 {
		t.Fatal("an unterminated completion or unanchored reply escaped")
	}
	if _, err := file.WriteAt(data[cut:], int64(cut)); err != nil {
		t.Fatal(err)
	}
	if err := before.Capture().Revalidate(t.Context()); err != nil {
		t.Fatalf("append invalidated earlier bounded capture: %v", err)
	}
	// Persisted metadata is not an input to the resumed reduction.
	prior.records[5].Model = "untrusted-previous-model"
	oldPrefix, _ := before.ReplayPrefix()
	after, _ := cjsiBoundReplay(t, file, path, int64(len(data)), version, cjsiCheckpointInput(oldPrefix, prior.evidence))
	cold, err := CaptureCodexReduction(t.Context(), file, path, int64(len(data)), version.MaxLineBytes)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after.reduction, cold.reduction) {
		t.Fatal("resumed reduction differed from cold re-lexing")
	}
	final := after.Checkpoint()
	records := after.reduction.Records
	if final.Partial || final.CompleteEnd != int64(len(data)) || final.ReplayFrom != final.CompleteEnd ||
		records[9].Execution != CodexRightAnchored || records[9].Model != "child-model" ||
		records[10].ToolStart == nil || *records[10].ToolStart != records[7].Record.Span {
		t.Fatal("fresh attempt did not resolve the pending right anchor and tool")
	}
	if after.Capture().digest != sha256.Sum256(data) ||
		after.MatchesCapture(before.Capture()) || after.MatchesCapture(cold.Capture()) {
		t.Fatal("resumed proof did not bind the exact new capture attempt")
	}
	if err := after.Capture().Revalidate(t.Context()); err != nil {
		t.Fatal(err)
	}

	if _, err := file.WriteAt([]byte(" "), 0); err != nil {
		t.Fatal(err)
	}
	batch := &cjsiCheckpointBatch{}
	replay, err := BeginCodexCheckpointReduction(t.Context(), file, path, int64(len(data)), version, cjsiCheckpointInput(oldPrefix, prior.evidence), batch, 32)
	if err != nil {
		t.Fatal(err)
	}
	defer replay.Abort()
	step, err := replay.Step(t.Context(), int64(len(data)))
	if _, ok := replay.CapturedReduction(); !errors.Is(err, ErrSourceChanged) ||
		step.Done || ok || len(batch.records) != 0 || batch.aborts != 1 {
		t.Fatal("changed prefix yielded a bound reduction or retained provisional facts")
	}
}

func TestCJSIVerifiedReplayReductionAbort(t *testing.T) {
	const line = "{\"type\":\"synthetic\"}\n"
	for _, mode := range []string{"record limit", "canceled", "sink failure"} {
		t.Run(mode, func(t *testing.T) {
			file, path := cjsiCheckpointFile(t, line+line)
			version := CodexCheckpointVersion{Generation: 1, Parser: 1, MaxLineBytes: 64}
			batch := &cjsiCheckpointBatch{}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			limit := 2
			if mode == "record limit" {
				limit = 1
			}
			replay, err := BeginCodexCheckpointReduction(ctx, file, path, 2*int64(len(line)), version, nil, batch, limit)
			if err != nil {
				t.Fatal(err)
			}
			defer replay.Abort()
			if step, err := replay.Step(ctx, int64(len(line))); err != nil || step.Done || len(batch.records) != 1 {
				t.Fatal("first step did not stage exactly one row")
			}
			var expect error
			switch mode {
			case "record limit":
				expect = ErrCodexReplayRecordBudget
			case "canceled":
				expect = context.Canceled
				cancel()
			case "sink failure":
				expect = errors.New("synthetic sink failure")
				batch.before = func() error { return expect }
			}
			step, err := replay.Step(ctx, int64(len(line)))
			if _, ok := replay.CapturedReduction(); !errors.Is(err, expect) || step.Done || ok ||
				batch.aborts != 1 || len(batch.records) != 0 || replay.binding != nil {
				t.Fatal("failed replay exposed a reduction or retained the provisional batch")
			}
		})
	}
}

func TestCJSIVerifiedReplayReductionRejectsUnverifiedTerminalLine(t *testing.T) {
	const line = "{\"type\":\"synthetic\"}\n"
	file, path := cjsiCheckpointFile(t, line+line)
	end := int64(2 * len(line))
	version := CodexCheckpointVersion{Generation: 1, Parser: 1, MaxLineBytes: 64}
	before, prior := cjsiBoundReplay(t, file, path, end, version, nil)
	prefix, ok := before.ReplayPrefix()
	if !ok || prefix.CompleteEnd != end {
		t.Fatal("initial capture did not verify both complete lines")
	}
	if _, err := file.WriteAt([]byte(" "), end-1); err != nil {
		t.Fatal(err)
	}

	batch := &cjsiCheckpointBatch{}
	replay, err := BeginCodexCheckpointReduction(t.Context(), file, path, end, version, cjsiCheckpointInput(prefix, prior.evidence), batch, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer replay.Abort()
	budget := int64(len(line))
	if step, err := replay.Step(t.Context(), budget); err != nil || step.Done || len(batch.records) != 1 {
		t.Fatal("first step did not stage the verified first line")
	}
	step, err := replay.Step(t.Context(), budget)
	_, captured := replay.CapturedReduction()
	if !errors.Is(err, ErrSourceChanged) || step.Done || captured || batch.aborts != 1 || len(batch.records) != 0 {
		t.Fatalf("terminal prefix claim did not fail closed: done=%t captured=%t aborts=%d rows=%d err=%v",
			step.Done, captured, batch.aborts, len(batch.records), err)
	}
}
