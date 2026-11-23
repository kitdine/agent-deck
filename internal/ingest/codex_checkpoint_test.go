package ingest

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

type cjsiCheckpointBatch struct {
	records  []CodexRecord
	evidence []CodexCheckpointEvidence
	aborts   int
	before   func() error
}

func (b *cjsiCheckpointBatch) StageCodexRecord(_ context.Context, record CodexRecord, evidence CodexCheckpointEvidence) error {
	if b.before != nil {
		if err := b.before(); err != nil {
			return err
		}
	}
	b.records = append(b.records, record)
	b.evidence = append(b.evidence, evidence)
	return nil
}

func (b *cjsiCheckpointBatch) Abort() {
	b.aborts++
	b.records, b.evidence = nil, nil
}

func cjsiCheckpointFile(t *testing.T, input string) (*os.File, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), cjsiCanonical(cjsiC))
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = file.Close() })
	if _, err := file.WriteString(input); err != nil {
		t.Fatal(err)
	}
	return file, path
}

func cjsiCheckpointInput(prefix CodexCheckpointPrefix, evidence []CodexCheckpointEvidence) *CodexCheckpointResume {
	cursor := 0
	return &CodexCheckpointResume{Prefix: prefix, Next: func(context.Context) (CodexCheckpointEvidence, error) {
		if cursor == len(evidence) {
			return CodexCheckpointEvidence{}, io.EOF
		}
		next := evidence[cursor]
		cursor++
		return next, nil
	}}
}

func cjsiRunCheckpoint(t *testing.T, file *os.File, path string, end, budget int64, version CodexCheckpointVersion, resume *CodexCheckpointResume) (CodexCheckpointStep, *cjsiCheckpointBatch) {
	t.Helper()
	batch := &cjsiCheckpointBatch{}
	replay, err := BeginCodexCheckpointReplay(t.Context(), file, path, end, version, resume, batch)
	if err != nil {
		t.Fatal(err)
	}
	defer replay.Abort()
	var readEnd int64
	for {
		step, err := replay.Step(t.Context(), budget)
		if err != nil {
			t.Fatal(err)
		}
		if step.Progress.ReadEnd != readEnd+min(end-readEnd, budget) || cap(replay.line) > version.MaxLineBytes {
			t.Fatal("replay violated its byte or retained line budget")
		}
		if step.Done {
			if replay.line != nil || replay.digest != nil || batch.aborts != 0 {
				t.Fatal("successful replay retained a partial line or aborted")
			}
			return step, batch
		}
		if step.Prefix != (CodexCheckpointPrefix{}) || step.Progress.ReadEnd >= end {
			t.Fatal("pending step authorized a checkpoint prematurely")
		}
		readEnd = step.Progress.ReadEnd
	}
}

func TestCJSIBoundedCheckpointReplay(t *testing.T) {
	child, err := os.ReadFile("testdata/cjsi_child.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	long := `{"type":"synthetic","ignored":"` + strings.Repeat("long-sentinel", 420) + `"}` + "\n"
	partial := `{"type":"synthetic","ignored":"partial-sentinel"`
	initial := string(child) + long + partial
	file, path := cjsiCheckpointFile(t, initial)
	appendBytes := "}\n" + `{"type":"synthetic"}` + "\n"
	version := CodexCheckpointVersion{Generation: 1, Parser: 2, MaxLineBytes: 2048}

	first, prior := cjsiRunCheckpoint(t, file, path, int64(len(initial)), 73, version, nil)
	lastComplete := int64(len(initial) - len(partial))
	if !first.Progress.Partial || first.Progress.CompleteEnd != lastComplete ||
		first.Prefix.CompleteEnd != lastComplete || prior.records[len(prior.records)-1].Kind != CodexOversize {
		t.Fatal("oversize or incomplete suffix changed the complete prefix")
	}
	for _, evidence := range prior.evidence {
		if evidence.Digest != sha256.Sum256([]byte(initial[evidence.Span.Start:evidence.Span.End])) {
			t.Fatal("evidence did not cover exactly one complete line")
		}
	}
	final := initial + appendBytes
	if _, err := file.WriteAt([]byte(appendBytes+`{"type":"outside-capture"}`+"\n"), int64(len(initial))); err != nil {
		t.Fatal(err)
	}
	step, resumed := cjsiRunCheckpoint(t, file, path, int64(len(final)), 83, version, cjsiCheckpointInput(first.Prefix, prior.evidence))
	if step.Progress.Partial || step.Progress.CompleteEnd != int64(len(final)) ||
		len(resumed.records) != len(prior.records)+2 || !slices.Equal(resumed.evidence[:len(prior.evidence)], prior.evidence) {
		t.Fatal("verified resume lost a complete record or read beyond the fixed extent")
	}

	var cold []CodexRecord
	progress, err := ReadCodexJSONL(t.Context(), strings.NewReader(final), CodexCaptureBounds{End: int64(len(final)), MaxLineBytes: version.MaxLineBytes}, false, func(record CodexRecord, visible *CodexVisible) error {
		if visible != nil {
			t.Fatal("proof read returned text")
		}
		cold = append(cold, record)
		return nil
	})
	if err != nil || progress != step.Progress || !slices.Equal(cold, resumed.records) {
		t.Fatal("bounded replay differs from a full capture")
	}
	got := cjsiReduce(t, path, resumed.records, step.Progress)
	want := cjsiReduce(t, path, cold, progress)
	if !reflect.DeepEqual(got, want) || got.Source.ThreadID != cjsiC {
		t.Fatal("bounded input changed whole-prefix reduction")
	}

	encoded, err := json.Marshal([]any{step, resumed.records, resumed.evidence})
	if err != nil || strings.Contains(string(encoded), "sentinel") || strings.Contains(string(encoded), "local-user-synthetic") {
		t.Fatal("staged metadata or checkpoint retained source text")
	}
}

func TestCJSICheckpointRejectsUnverifiedResume(t *testing.T) {
	const input = "{\"type\":\"first\"}\n{\"type\":\"second\"}\n"
	version := CodexCheckpointVersion{Generation: 1, Parser: 2, MaxLineBytes: 2048}
	for _, name := range []string{"inode", "parser", "generation", "line policy", "truncate", "changed bytes", "overlap", "hole", "extra", "premature EOF", "incomplete boundary"} {
		t.Run(name, func(t *testing.T) {
			file, path := cjsiCheckpointFile(t, input)
			first, prior := cjsiRunCheckpoint(t, file, path, int64(len(input)), 15, version, nil)
			evidence := slices.Clone(prior.evidence)
			end := int64(len(input))
			beginReject := false
			switch name {
			case "inode":
				// Identical bytes in a different regular file are insufficient.
				file, path = cjsiCheckpointFile(t, input)
				beginReject = true
			case "parser":
				first.Prefix.Version.Parser++
				beginReject = true
			case "generation":
				first.Prefix.Version.Generation++
				beginReject = true
			case "line policy":
				first.Prefix.Version.MaxLineBytes++
				beginReject = true
			case "truncate":
				if err := file.Truncate(end - 1); err != nil {
					t.Fatal(err)
				}
				beginReject = true
			case "changed bytes":
				info, err := file.Stat()
				if err != nil {
					t.Fatal(err)
				}
				if _, err := file.WriteAt([]byte("x"), 10); err != nil {
					t.Fatal(err)
				}
				if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
					t.Fatal(err)
				}
			case "overlap":
				evidence[1].Span.Start--
			case "hole":
				evidence[1].Span.Start++
			case "extra":
				evidence = append(evidence, evidence[1])
			case "premature EOF":
				evidence = evidence[:1]
			case "incomplete boundary":
				first.Prefix.CompleteEnd--
			}
			batch := &cjsiCheckpointBatch{}
			replay, err := BeginCodexCheckpointReplay(t.Context(), file, path, end, version, cjsiCheckpointInput(first.Prefix, evidence), batch)
			if beginReject {
				if !errors.Is(err, ErrSourceChanged) || replay != nil || batch.aborts != 0 {
					t.Fatal("invalid header accepted or caller's sink ownership changed")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			step, err := replay.Step(t.Context(), end)
			if !errors.Is(err, ErrSourceChanged) || step.Done || step.Prefix != (CodexCheckpointPrefix{}) ||
				batch.aborts != 1 || len(batch.records) != 0 {
				t.Fatal("unverified evidence escaped the provisional batch")
			}
		})
	}
}

func TestCJSICheckpointAbortsIncompleteBatch(t *testing.T) {
	const input = "{\"type\":\"first\"}\n{\"type\":\"last\"}\n"
	firstEnd := int64(strings.IndexByte(input, '\n') + 1)
	version := CodexCheckpointVersion{Generation: 1, Parser: 2, MaxLineBytes: 64}
	for _, name := range []string{"canceled", "sink error", "file changed between steps", "file appended between steps", "file changed during stage", "invalid budget", "abandoned"} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			file, path := cjsiCheckpointFile(t, input)
			batch := &cjsiCheckpointBatch{}
			replay, err := BeginCodexCheckpointReplay(ctx, file, path, int64(len(input)), version, nil, batch)
			if err != nil {
				t.Fatal(err)
			}
			first, err := replay.Step(ctx, firstEnd+1)
			if err != nil || first.Done || first.Progress.CompleteEnd != firstEnd ||
				first.Progress.ReadEnd != firstEnd+1 || !first.Progress.Partial || len(batch.records) != 1 {
				t.Fatal("initial step did not stage one row and stop at a half line")
			}
			partialLine := replay.line
			budget := int64(len(input))
			var expect error
			changeFile := func() error {
				if _, err := file.WriteAt([]byte(" "), 0); err != nil {
					return err
				}
				at := time.Unix(1700000000, 0)
				return os.Chtimes(path, at, at)
			}
			switch name {
			case "canceled":
				cancel()
				expect = context.Canceled
			case "sink error":
				expect = errors.New("synthetic sink failure")
				batch.before = func() error { return expect }
			case "file changed between steps":
				if err := changeFile(); err != nil {
					t.Fatal(err)
				}
				expect = ErrSourceChanged
			case "file appended between steps":
				if _, err := file.WriteAt([]byte("{}\n"), int64(len(input))); err != nil {
					t.Fatal(err)
				}
				expect = ErrSourceChanged
			case "file changed during stage":
				batch.before = changeFile
				expect = ErrSourceChanged
			case "invalid budget":
				budget = 0
				expect = errCodexRange
			case "abandoned":
				replay.Abort()
				expect = errCodexReplayClosed
			}
			step, err := replay.Step(ctx, budget)
			replay.Abort()
			if !errors.Is(err, expect) || step.Done || step.Prefix != (CodexCheckpointPrefix{}) ||
				batch.aborts != 1 || len(batch.records) != 0 || replay.line != nil || replay.digest != nil {
				t.Fatal("failed replay did not discard the entire provisional batch")
			}
			if name == "abandoned" && (len(partialLine) != 1 || partialLine[0] != 0) {
				t.Fatal("failed replay retained buffered partial input")
			}
		})
	}
}
