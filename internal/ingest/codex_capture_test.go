package ingest

import (
	"bufio"
	"context"
	"errors"
	"io"
	"slices"
	"strings"
	"testing"
)

type cjsiCaptureReader func([]byte) (int, error)

func (read cjsiCaptureReader) Read(p []byte) (int, error) { return read(p) }

func TestCJSIBoundedCodexCapture(t *testing.T) {
	message := `{"type":"response_item","payload":{"type":"message","role":"user","content":"synthetic"}}` + "\n"
	t.Run("line budget and exclusive capture end", func(t *testing.T) {
		for _, tc := range []struct {
			name  string
			limit int
			kind  CodexRecordKind
		}{
			{"below", len(message) + 1, CodexMessage},
			{"exact", len(message), CodexMessage},
			{"above by newline", len(message) - 1, CodexOversize},
			{"one byte budget", 1, CodexOversize},
		} {
			t.Run(tc.name, func(t *testing.T) {
				const start = 50
				end := int64(start + len(message))
				input := strings.NewReader(message + message)
				remaining := len(message)
				reader := cjsiCaptureReader(func(p []byte) (int, error) {
					if len(p) > remaining || len(p) > tc.limit {
						t.Fatal("read request exceeded capture or buffer bounds")
					}
					n, err := input.Read(p)
					remaining -= n
					return n, err
				})
				calls := 0
				progress, err := ReadCodexJSONL(t.Context(), reader, CodexCaptureBounds{start, end, tc.limit}, true, func(record CodexRecord, visible *CodexVisible) error {
					calls++
					span := CodexSpan{start, end}
					if record.Span != span || record.Kind != tc.kind {
						t.Fatal("line kind or absolute span changed")
					}
					if tc.kind == CodexOversize {
						if record != (CodexRecord{Span: span, Kind: CodexOversize}) || visible != nil {
							t.Fatal("oversize gap retained parsed or visible material")
						}
					} else if visible == nil || visible.Text != "synthetic" || visible.Span != span {
						t.Fatal("bounded capture changed ordinary lexing")
					}
					return nil
				})
				if err != nil || progress != (CodexReadProgress{ReadEnd: end, CompleteEnd: end}) || calls != 1 ||
					input.Len() != len(message) {
					t.Fatalf("wrong capture result: %+v, calls=%d, error=%v", progress, calls, err)
				}
			})
		}
	})

	t.Run("line spanning several reads at the exact budget", func(t *testing.T) {
		line := strings.TrimSuffix(message, "\n") + strings.Repeat(" ", 9000) + "\n"
		input := line + message
		var spans []CodexSpan
		progress, err := ReadCodexJSONL(t.Context(), strings.NewReader(input), CodexCaptureBounds{
			End: int64(len(input)), MaxLineBytes: len(line),
		}, true, func(record CodexRecord, visible *CodexVisible) error {
			if record.Kind != CodexMessage || visible == nil || visible.Text != "synthetic" {
				t.Fatal("multi-read line at the budget did not lex normally")
			}
			spans = append(spans, record.Span)
			return nil
		})
		if err != nil || progress.CompleteEnd != int64(len(input)) || progress.Partial ||
			!slices.Equal(spans, []CodexSpan{{0, int64(len(line))}, {int64(len(line)), int64(len(input))}}) {
			t.Fatalf("wrong multi-read line spans: %v, progress=%+v, error=%v", spans, progress, err)
		}
	})

	head := `{"type":"session_meta","payload":{"id":"` + cjsiC + `"}}` + "\n"
	const limit = 256
	long := strings.Repeat("synthetic-oversize ", 900) + "\n"
	capture := func(t *testing.T, input string, start, end int64) ([]CodexRecord, CodexReadProgress) {
		t.Helper()
		var records []CodexRecord
		progress, err := ReadCodexJSONL(t.Context(), strings.NewReader(input), CodexCaptureBounds{start, end, limit}, false, func(record CodexRecord, visible *CodexVisible) error {
			if visible != nil {
				t.Fatal("proof capture emitted visible text")
			}
			records = append(records, record)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		return records, progress
	}
	t.Run("multi-buffer gap only after newline and incremental replay", func(t *testing.T) {
		input := head + long + message
		cut := int64(len(head) + len(long) - 1)
		records, progress := capture(t, input, 0, cut)
		if len(records) != 1 || progress != (CodexReadProgress{cut, int64(len(head)), true}) {
			t.Fatalf("cut published a gap or moved checkpoint: %+v", progress)
		}
		rest, end := capture(t, input[progress.CompleteEnd:], progress.CompleteEnd, int64(len(input)))
		gap := CodexRecord{Span: CodexSpan{int64(len(head)), cut + 1}, Kind: CodexOversize}
		if len(rest) != 2 || rest[0] != gap || rest[1].Kind != CodexMessage || rest[1].Span.Start != cut+1 || end.Partial {
			t.Fatal("resumed gap or the following short record lost its exact span")
		}
		cold, coldEnd := capture(t, input, 0, int64(len(input)))
		if !slices.Equal(append(records, rest...), cold) || end != coldEnd {
			t.Fatal("incremental replay differed from full capture")
		}
		result := cjsiReduce(t, cjsiCanonical(cjsiC), cold, coldEnd)
		if result.Source.ThreadID != cjsiC || result.Records[2].Local() || result.Checkpoint.ReplayFrom != coldEnd.CompleteEnd {
			t.Fatal("an oversize gap carried a bare local scope or held a closed span")
		}
	})
	t.Run("oversize head cannot be bypassed", func(t *testing.T) {
		oversizeHead := strings.TrimSuffix(head, "\n") + strings.Repeat(" ", 4097) + "\n"
		input := oversizeHead + head
		records, p := capture(t, input, 0, int64(len(input)))
		result := cjsiReduce(t, cjsiCanonical(cjsiC), records, p)
		if len(records) != 2 || records[0].Kind != CodexOversize || result.Source.State != CodexSourceUnknown ||
			result.Checkpoint.ReplayFrom != 0 {
			t.Fatal("later head established proof after an oversize opening")
		}
	})
	t.Run("middle gap closes tool and model scope", func(t *testing.T) {
		turn := `{"type":"turn_context","payload":{"turn_id":"turn","model":"model"}}` + "\n"
		start := `{"type":"response_item","payload":{"type":"function_call","call_id":"call"}}` + "\n"
		finish := `{"type":"response_item","payload":{"type":"function_call_output","call_id":"call"}}` + "\n"
		request := `{"type":"token_usage_record","payload":{"thread_id":"` + cjsiC + `","turn_id":"turn","response_id":"response","usage":{"input_tokens":2,"output_tokens":1}}}` + "\n"
		input := head + turn + start + long + finish + request
		records, p := capture(t, input, 0, int64(len(input)))
		got := cjsiReduce(t, cjsiCanonical(cjsiC), records, p)
		if !got.Records[2].Local() || got.Records[2].Model != "model" || got.Records[3].Local() ||
			got.Records[4].Local() || got.Records[4].ToolStart != nil ||
			got.Records[5].Execution != CodexExplicit || got.Records[5].Model != "" {
			t.Fatal("oversize gap corrupted prior proof, bridged scope, or hid an independent request")
		}
		if total, valid := got.Records[5].Record.Usage.Total(); !valid || total != 3 {
			t.Fatal("independent request lost its count")
		}
		conflicted := cjsiReduce(t, cjsiCanonical(cjsiP), records, p)
		if conflicted.Source.State != CodexSourceConflict || conflicted.Checkpoint.ReplayFrom != p.CompleteEnd ||
			slices.ContainsFunc(conflicted.Records, CodexResolvedRecord.Local) {
			t.Fatal("oversize gap changed filename conflict handling")
		}
	})

	t.Run("callback rejection and cancellation preserve accepted bytes", func(t *testing.T) {
		stop := errors.New("synthetic callback rejection")
		input := head + message + message
		end := int64(len(input))
		for _, reject := range []bool{true, false} {
			ctx, cancel := context.WithCancel(t.Context())
			calls := 0
			progress, err := ReadCodexJSONL(ctx, strings.NewReader(input), CodexCaptureBounds{End: end, MaxLineBytes: 4096}, false, func(CodexRecord, *CodexVisible) error {
				calls++
				if !reject {
					cancel()
				}
				if calls == 2 {
					return stop
				}
				return nil
			})
			cancel()
			wantErr, wantRead, wantCalls := error(context.Canceled), int64(len(head)), 1
			if reject {
				wantErr, wantRead, wantCalls = stop, int64(len(head)+len(message)), 2
			}
			if !errors.Is(err, wantErr) || calls != wantCalls ||
				progress != (CodexReadProgress{ReadEnd: wantRead, CompleteEnd: int64(len(head))}) {
				t.Fatalf("wrong stop result: %+v, calls=%d, error=%v", progress, calls, err)
			}
		}
	})
	t.Run("I/O failure preserves accepted line and partial progress", func(t *testing.T) {
		failure := errors.New("synthetic read failure")
		input := head + "{"
		reader := cjsiCaptureReader(func(p []byte) (int, error) {
			return copy(p, input), failure
		})
		calls := 0
		progress, err := ReadCodexJSONL(t.Context(), reader, CodexCaptureBounds{End: 4096, MaxLineBytes: 4096}, false, func(CodexRecord, *CodexVisible) error {
			calls++
			return nil
		})
		if !errors.Is(err, failure) || calls != 1 ||
			progress != (CodexReadProgress{int64(len(input)), int64(len(head)), true}) {
			t.Fatalf("wrong I/O failure result: %+v, calls=%d, error=%v", progress, calls, err)
		}
	})
	t.Run("cancellation between oversized chunks", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		input := strings.NewReader(long)
		reads := 0
		reader := cjsiCaptureReader(func(p []byte) (int, error) {
			reads++
			if reads == 3 {
				cancel()
			}
			return input.Read(p)
		})
		progress, err := ReadCodexJSONL(ctx, reader, CodexCaptureBounds{End: int64(len(long)), MaxLineBytes: limit}, false, func(CodexRecord, *CodexVisible) error {
			t.Fatal("canceled oversized capture emitted a gap")
			return nil
		})
		if !errors.Is(err, context.Canceled) || reads != 3 ||
			progress != (CodexReadProgress{ReadEnd: 2 * limit, Partial: true}) {
			t.Fatalf("wrong progress on cancellation: %+v, reads=%d, error=%v", progress, reads, err)
		}
	})
	t.Run("invalid and empty bounds do not read", func(t *testing.T) {
		reader := cjsiCaptureReader(func([]byte) (int, error) {
			t.Fatal("invalid or empty bounds requested bytes")
			return 0, io.EOF
		})
		for _, bounds := range []CodexCaptureBounds{{-1, 1, limit}, {2, 1, limit}, {0, 1, 0}} {
			if _, err := ReadCodexJSONL(t.Context(), reader, bounds, false, nil); !errors.Is(err, errCodexRange) {
				t.Fatalf("invalid bounds returned %v", err)
			}
		}
		if progress, err := ReadCodexJSONL(t.Context(), reader, CodexCaptureBounds{2, 2, limit}, false, nil); err != nil ||
			progress != (CodexReadProgress{ReadEnd: 2, CompleteEnd: 2}) {
			t.Fatalf("empty and valid capture returned %+v, %v", progress, err)
		}
	})
	t.Run("generic reader still returns complete long records", func(t *testing.T) {
		reader := bufio.NewReaderSize(strings.NewReader(long+message), 32)
		for _, want := range []string{long, message} {
			got, err := readRecordLine(reader)
			if err != nil || string(got) != want {
				t.Fatal("generic reader stopped returning the whole line")
			}
		}
	})
}
