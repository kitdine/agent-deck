package ingest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// sequentialBatches is the reference batching and decoding of complete lines.
func sequentialBatches(data []byte) ([]Batch, []byte) {
	var batches []Batch
	batch, batchBytes, offset, sequence := Batch{Records: make([]Record, 0, 256)}, 0, int64(0), uint64(0)
	for len(data) > 0 {
		index := bytes.IndexByte(data, '\n')
		if index < 0 {
			if len(batch.Records) > 0 {
				batches = append(batches, batch)
			}
			return batches, append([]byte(nil), data...)
		}
		line := data[:index+1]
		sequence++
		var value map[string]any
		malformed := json.Unmarshal(line[:len(line)-1], &value) != nil
		batch.Records = append(batch.Records, Record{Offset: offset, End: offset + int64(len(line)), Sequence: sequence, Kind: classify(value), Value: value, Malformed: malformed})
		batchBytes += len(line)
		offset += int64(len(line))
		if len(batch.Records) >= 256 || batchBytes >= 1<<20 {
			batches = append(batches, batch)
			batch, batchBytes = Batch{Records: make([]Record, 0, 256)}, 0
		}
		data = data[index+1:]
	}
	if len(batch.Records) > 0 {
		batches = append(batches, batch)
	}
	return batches, nil
}

func TestParallelDecodeKeepsSequentialBatchesRecordsAndTail(t *testing.T) {
	var content strings.Builder
	for index := 0; content.Len() < int(5*parallelDecodeMinBytes); index++ {
		switch {
		case index%97 == 0:
			content.WriteString("{not json\n")
		case index%2003 == 0:
			// Longer than the reader buffer and than one batch byte limit.
			fmt.Fprintf(&content, "{\"type\":\"user\",\"text\":%q}\n", strings.Repeat("x", 1<<20+17))
		case index%3 == 0:
			fmt.Fprintf(&content, "{\"type\":\"event_msg\",\"n\":%d}\n", index)
		default:
			fmt.Fprintf(&content, "{\"type\":\"response_item\",\"text\":%q}\n", strings.Repeat("y", index%700))
		}
	}
	content.WriteString(`{"partial":`)
	data := []byte(content.String())
	path := filepath.Join(t.TempDir(), "large.jsonl")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	source := testSource(t, path)
	if source.Size < parallelDecodeMinBytes {
		t.Fatalf("fixture %d bytes does not select parallel decoding", source.Size)
	}
	want, wantTail := sequentialBatches(data)
	full := 0
	for _, batch := range want {
		if len(batch.Records) == 256 {
			full++
		}
	}
	if full < 2*parallelDecodeWindow || len(want) == full {
		t.Fatalf("fixture batches=%d full=%d do not cover both boundaries beyond the window", len(want), full)
	}
	c := NewCoordinator([]Source{source}, Options{})
	defer c.Close()
	type result struct {
		batches []Batch
		tail    []byte
		err     error
	}
	results := make(map[string]chan result)
	for _, consumer := range []string{ConsumerUsage, ConsumerSession} {
		results[consumer] = make(chan result, 1)
		go func() {
			stream, shared, err := c.Stream(context.Background(), path, consumer)
			if err != nil || !shared {
				results[consumer] <- result{err: fmt.Errorf("shared=%t err=%v", shared, err)}
				return
			}
			var got []Batch
			for batch := range stream.Batches {
				// Slow consumers make decoding run ahead of publication.
				time.Sleep(time.Millisecond)
				got = append(got, batch)
			}
			tail, err := stream.Wait(context.Background())
			if err == nil {
				err = stream.ValidateCapturedRange()
			}
			results[consumer] <- result{batches: got, tail: tail, err: err}
		}()
	}
	for _, consumer := range []string{ConsumerUsage, ConsumerSession} {
		got := <-results[consumer]
		if got.err != nil {
			t.Fatalf("%s: %v", consumer, got.err)
		}
		if len(got.batches) != len(want) {
			t.Fatalf("%s batches=%d want %d", consumer, len(got.batches), len(want))
		}
		for index := range want {
			if !reflect.DeepEqual(got.batches[index].Records, want[index].Records) {
				t.Fatalf("%s batch %d differs from sequential decoding", consumer, index)
			}
		}
		if !bytes.Equal(got.tail, wantTail) {
			t.Fatalf("%s tail=%q want %q", consumer, got.tail, wantTail)
		}
	}
}

func TestParallelDecodeStopsOnCancellationWithoutLeakingPublication(t *testing.T) {
	line := `{"type":"user","text":"` + strings.Repeat("z", 1000) + `"}` + "\n"
	path := filepath.Join(t.TempDir(), "large.jsonl")
	if err := os.WriteFile(path, []byte(strings.Repeat(line, int(2*parallelDecodeMinBytes)/len(line))), 0o600); err != nil {
		t.Fatal(err)
	}
	c := NewCoordinator([]Source{testSource(t, path)}, Options{})
	ctx, cancel := context.WithCancel(context.Background())
	c.Start(ctx)
	stream, shared, err := c.Stream(ctx, path, ConsumerUsage)
	if err != nil || !shared {
		t.Fatalf("shared=%t err=%v", shared, err)
	}
	if _, ok := <-stream.Batches; !ok {
		t.Fatal("first batch missing")
	}
	cancel()
	select {
	case <-c.done:
	case <-time.After(5 * time.Second):
		t.Fatal("reader did not stop after cancellation")
	}
	for range stream.Batches {
	}
	if _, err = stream.Wait(context.Background()); !errors.Is(err, context.Canceled) {
		t.Fatalf("stream error=%v, want cancellation", err)
	}
}
