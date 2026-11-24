package cjsiledger

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"

	"github.com/kitdine/agent-deck/internal/ingest"
)

// LegacyDecoder runs the original v8 parser only to identify previously
// published rows. Its output never decides execution identity or new usage.
type LegacyDecoder func(line []byte, start int64) (OldImage, bool, error)

func (s *sourceSink) readLegacy(ctx context.Context, f io.ReaderAt, decode LegacyDecoder) error {
	e := s.stamp.Prefix
	h := sha256.New()
	offsets := map[int64][32]byte{0: sha256.Sum256(nil)}
	byStart := map[int64]int{}
	for i, m := range s.meters {
		offsets[m.epochStart] = [32]byte{}
		byStart[m.span.Start] = i
	}
	reader := bufio.NewReaderSize(io.NewSectionReader(f, 0, e.Progress.ReadEnd), min(e.MaxLineBytes, 64<<10))
	tailHash := sha256.New()
	tailStart := max(int64(0), e.Progress.CompleteEnd-4096)
	var end, start int64
	var line []byte
	var oversize bool
	for end < e.Progress.ReadEnd {
		if err := ctx.Err(); err != nil {
			return err
		}
		b, err := reader.ReadSlice('\n')
		if err != nil && !errors.Is(err, bufio.ErrBufferFull) && !errors.Is(err, io.EOF) {
			return err
		}
		lo, hi := max(end, tailStart), min(end+int64(len(b)), e.Progress.CompleteEnd)
		if hi > lo {
			_, _ = tailHash.Write(b[lo-end : hi-end])
		}
		end += int64(len(b))
		_, _ = h.Write(b)
		if len(b) == 0 {
			return io.ErrUnexpectedEOF
		}
		if !oversize && len(line)+len(b) <= e.MaxLineBytes {
			line = append(line, b...)
		} else {
			oversize = true
		}
		if b[len(b)-1] != '\n' {
			continue
		}
		var through [32]byte
		copy(through[:], h.Sum(nil))
		if _, ok := offsets[end]; ok {
			offsets[end] = through
		}
		i, meter := byStart[start]
		if meter {
			s.meters[i].prefixHash = through
		}
		if oversize {
			decode = nil
		} // The previous parser context beyond this line is unknown.
		if decode != nil {
			image, ok, err := decode(line, start)
			if err != nil {
				return err
			}
			if ok {
				if len(s.old) >= s.limit {
					return ingest.ErrCodexWorksetBudget
				}
				s.old = append(s.old, legacyFact{image, ingest.CodexSpan{Start: start, End: end}, through})
				if meter && s.meters[i].record.Kind == ingest.CodexLegacyUsage {
					s.meters[i].legacyKey = image.EventKey
				}
			}
		}
		start = end
		line = line[:0]
		oversize = false
	}
	copy(s.anchor[:], tailHash.Sum(nil))
	if !bytes.Equal(e.Digest[:], h.Sum(nil)) {
		return ingest.ErrSourceChanged
	}
	for i := range s.meters {
		s.meters[i].epochHash = offsets[s.meters[i].epochStart]
	}
	return nil
}
