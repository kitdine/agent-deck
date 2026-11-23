package ingest

import (
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCJSICodexPrefixGeneration(t *testing.T) {
	head := `{"type":"session_meta","payload":{"id":"` + cjsiC + `"}}` + "\n"
	// More than one hash block, so an early rewrite need not touch the tail.
	prefix := head + strings.Repeat(`{"type":"synthetic"}`+"\n", 3600)
	suffix := `{"type":"response_item"`
	input := prefix + suffix
	end := int64(len(input))
	open := func(t *testing.T, contents string) (*os.File, string) {
		t.Helper()
		path := filepath.Join(t.TempDir(), cjsiCanonical(cjsiC))
		file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = file.Close() })
		if _, err := file.WriteString(contents); err != nil {
			t.Fatal(err)
		}
		return file, path
	}
	visit := func(CodexRecord, *CodexVisible) error { return nil }
	capture := func(t *testing.T, file *os.File, path string) CodexPrefixProof {
		t.Helper()
		proof, err := CaptureCodexPrefix(t.Context(), file, path, end, 256, false, visit)
		if err != nil {
			t.Fatalf("capture: %v", err)
		}
		return proof
	}

	t.Run("exact parse digest ignores later bytes and keeps partial boundary", func(t *testing.T) {
		extra := "}\n"
		file, path := open(t, input+extra)
		var last CodexRecord
		if _, err := file.Seek(1, io.SeekStart); err != nil {
			t.Fatal(err)
		}
		proof, err := CaptureCodexPrefix(t.Context(), file, path, end, 256, false, func(record CodexRecord, _ *CodexVisible) error {
			last = record
			return nil
		})
		if err != nil || proof.digest != sha256.Sum256([]byte(input)) ||
			proof.Progress() != (CodexReadProgress{ReadEnd: end, CompleteEnd: int64(len(prefix)), Partial: true}) ||
			last.Span.End != int64(len(prefix)) {
			t.Fatalf("capture extent, digest or replay boundary differed: %v, %+v", err, proof.Progress())
		}
		if off, err := file.Seek(0, io.SeekCurrent); err != nil || off != 1 {
			t.Fatalf("capture changed caller offset: %d, %v", off, err)
		}
		if err := proof.Revalidate(t.Context()); err != nil {
			t.Fatal(err)
		}
		if _, err := file.WriteAt([]byte("!"), end+1); err != nil {
			t.Fatal(err)
		}
		if err := proof.Revalidate(t.Context()); err != nil {
			t.Fatalf("rewrite outside extent invalidated prefix: %v", err)
		}
	})

	t.Run("append completes old suffix outside captured extent", func(t *testing.T) {
		file, path := open(t, input)
		proof := capture(t, file, path)
		if _, err := file.WriteAt([]byte("}\n"), end); err != nil {
			t.Fatal(err)
		}
		if err := proof.Revalidate(t.Context()); err != nil {
			t.Fatalf("stable append invalidated original prefix: %v", err)
		}
		if !proof.Progress().Partial || proof.Progress().CompleteEnd != int64(len(prefix)) ||
			proof.digest != sha256.Sum256([]byte(input)) {
			t.Fatal("revalidation promoted the original partial suffix")
		}
	})

	for _, change := range []string{"rename", "identical replacement", "truncate", "early overwrite with restored mtime", "partial suffix overwrite"} {
		t.Run(change, func(t *testing.T) {
			file, path := open(t, input)
			fixedMtime := time.Unix(1700000000, 0)
			if change == "early overwrite with restored mtime" {
				if err := os.Chtimes(path, fixedMtime, fixedMtime); err != nil {
					t.Fatal(err)
				}
			}
			proof := capture(t, file, path)
			var err error
			switch change {
			case "rename", "identical replacement":
				err = os.Rename(path, filepath.Join(filepath.Dir(path), "moved.jsonl"))
				if err == nil && change == "identical replacement" {
					err = os.WriteFile(path, []byte(input), 0600)
				}
			case "truncate":
				err = file.Truncate(end - 1)
			case "early overwrite with restored mtime":
				_, err = file.WriteAt([]byte("x"), 2)
				if err == nil {
					err = os.Chtimes(path, fixedMtime, fixedMtime)
				}
				info, statErr := file.Stat()
				if statErr != nil || info.Size() != end || !info.ModTime().Equal(fixedMtime) {
					t.Fatal("early rewrite fixture did not preserve size and mtime")
				}
				tail := make([]byte, 4096)
				if _, readErr := file.ReadAt(tail, end-int64(len(tail))); readErr != nil || string(tail) != input[len(input)-len(tail):] {
					t.Fatal("early rewrite fixture unexpectedly changed tail")
				}
			case "partial suffix overwrite":
				_, err = file.WriteAt([]byte("x"), end-1)
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := proof.Revalidate(t.Context()); !errors.Is(err, ErrSourceChanged) {
				t.Fatalf("changed source passed revalidation: %v", err)
			}
		})
	}

	t.Run("no proof on cancellation callback or I/O failure", func(t *testing.T) {
		file, path := open(t, input)
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		rejected := errors.New("synthetic private callback detail")
		for _, tc := range []struct {
			name  string
			ctx   context.Context
			file  *os.File
			end   int64
			visit func(CodexRecord, *CodexVisible) error
			want  error
		}{
			{"callback", t.Context(), file, end, func(CodexRecord, *CodexVisible) error { return rejected }, rejected},
			{"canceled", ctx, file, end, func(CodexRecord, *CodexVisible) error { cancel(); return nil }, context.Canceled},
			{"short", t.Context(), file, end + 1, visit, ErrSourceChanged},
		} {
			t.Run(tc.name, func(t *testing.T) {
				proof, err := CaptureCodexPrefix(tc.ctx, tc.file, path, tc.end, 256, false, tc.visit)
				if !errors.Is(err, tc.want) || proof != (CodexPrefixProof{}) ||
					strings.Contains(err.Error(), path) || strings.Contains(err.Error(), rejected.Error()) {
					t.Fatal("failed capture minted a proof or leaked diagnostic details")
				}
			})
		}
		writeOnly, err := os.OpenFile(path, os.O_WRONLY, 0600)
		if err != nil {
			t.Fatal(err)
		}
		defer writeOnly.Close()
		proof, err := CaptureCodexPrefix(t.Context(), writeOnly, path, end, 256, false, visit)
		if proof != (CodexPrefixProof{}) || err != ErrSourceChanged {
			t.Fatal("unreadable descriptor minted a proof or exposed a raw error")
		}
		valid := capture(t, file, path)
		if err := valid.Revalidate(ctx); !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled revalidation returned %v", err)
		}
		if err := (CodexPrefixProof{}).Revalidate(t.Context()); !errors.Is(err, ErrSourceChanged) {
			t.Fatal("zero proof passed revalidation")
		}
	})

	t.Run("changes within capture wall discard provisional batch", func(t *testing.T) {
		for _, change := range []string{"grow", "shrink", "rewrite"} {
			t.Run(change, func(t *testing.T) {
				file, path := open(t, input)
				calls := 0
				proof, err := CaptureCodexPrefix(t.Context(), file, path, end, 256, false, func(CodexRecord, *CodexVisible) error {
					calls++
					if calls != 1 {
						return nil
					}
					switch change {
					case "grow":
						_, err := file.WriteAt([]byte("!"), end)
						return err
					case "shrink":
						return file.Truncate(int64(len(head)))
					default:
						if _, err := file.WriteAt([]byte("!"), 0); err != nil {
							return err
						}
						changed := time.Unix(1700000000, 0)
						return os.Chtimes(path, changed, changed)
					}
				})
				if !errors.Is(err, ErrSourceChanged) || calls == 0 || proof != (CodexPrefixProof{}) {
					t.Fatalf("changed capture authorized a batch: calls=%d, error=%v", calls, err)
				}
			})
		}
	})
}

func TestCJSICapturedReductionBindsOneCaptureAttempt(t *testing.T) {
	input := `{"type":"session_meta","payload":{"id":"` + cjsiC + `"}}` + "\n"
	path := filepath.Join(t.TempDir(), cjsiCanonical(cjsiC))
	if err := os.WriteFile(path, []byte(input), 0600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	a, err := CaptureCodexReduction(t.Context(), file, path, int64(len(input)), len(input))
	if err != nil || a.Source().State != CodexSourceVerified || !a.MatchesCapture(a.Capture()) {
		t.Fatalf("expected verified bound reduction: %v", err)
	}
	b, err := CaptureCodexReduction(t.Context(), file, path, int64(len(input)), 1)
	if err != nil || b.Source().State != CodexSourceUnknown || !b.MatchesCapture(b.Capture()) {
		t.Fatalf("expected a distinct capture with an oversized head: %v", err)
	}
	if a.Capture().Progress() != b.Capture().Progress() || a.MatchesCapture(b.Capture()) || b.MatchesCapture(a.Capture()) {
		t.Fatal("same descriptor and extent were mistaken for the same capture")
	}
	failed, err := CaptureCodexReduction(t.Context(), file, path, int64(len(input)+1), len(input))
	if !errors.Is(err, ErrSourceChanged) || failed.MatchesCapture(failed.Capture()) {
		t.Fatal("a failed capture yielded a bound reduction")
	}
}
