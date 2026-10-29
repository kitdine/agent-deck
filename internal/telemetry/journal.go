package telemetry

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/kitdine/agent-deck/internal/platform"
	"github.com/kitdine/agent-deck/internal/store"
)

const (
	// DefaultSegmentBytes is the largest journal segment.
	DefaultSegmentBytes = 8 << 20
	// DefaultJournalCap is the total journal size limit.
	DefaultJournalCap = 256 << 20
)

// ErrJournalFull reports that a record could not be persisted without
// deleting unprocessed data. The receiver answers 503 and records a gap.
var ErrJournalFull = errors.New("telemetry journal is full")

// Journal is the private append-only record of acknowledged deliveries.
// Appends are serialized across processes with an exclusive file lock.
type Journal struct {
	Dir          string
	SegmentBytes int64
	Cap          int64
	Retention    time.Duration
	// Processed reports the first unprocessed position; cleanup never
	// removes a segment at or after it.
	Processed func() (store.TelemetryCursor, error)
	Now       func() time.Time

	mu sync.Mutex
}

type segment struct {
	index int64
	path  string
	size  int64
	mod   time.Time
}

func segmentName(index int64) string { return fmt.Sprintf("seg-%016d.jsonl", index) }

func (j *Journal) segments() ([]segment, error) {
	entries, err := os.ReadDir(j.Dir)
	if err != nil {
		return nil, err
	}
	var result []segment
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "seg-") || !strings.HasSuffix(name, ".jsonl") {
			continue
		}
		index, err := strconv.ParseInt(strings.TrimSuffix(strings.TrimPrefix(name, "seg-"), ".jsonl"), 10, 64)
		if err != nil {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		result = append(result, segment{index: index, path: filepath.Join(j.Dir, name), size: info.Size(), mod: info.ModTime()})
	}
	sort.Slice(result, func(a, b int) bool { return result[a].index < result[b].index })
	return result, nil
}

// Append durably writes records as one batch: it returns only after the data
// and, for a new segment, its directory entry are fsync'd. A batch is never
// split across segments.
func (j *Journal) Append(records []Record) error {
	if len(records) == 0 {
		return nil
	}
	var batch bytes.Buffer
	for _, record := range records {
		line, err := json.Marshal(record)
		if err != nil {
			return err
		}
		batch.Write(line)
		batch.WriteByte('\n')
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	unlock, err := j.lock()
	if err != nil {
		return err
	}
	defer unlock()
	segments, err := j.segments()
	if err != nil {
		return err
	}
	if n := len(segments); n > 0 {
		// A crash or failed write can leave an unacknowledged partial line.
		// Terminating it keeps every new batch on a record boundary, so the
		// fragment reads as one unreadable line instead of swallowing the
		// next acknowledged record or blocking the reader at a rotation.
		terminated, err := terminateTornTail(segments[n-1])
		if err != nil {
			return err
		}
		segments[n-1].size += terminated
	}
	size := int64(batch.Len())
	if total(segments)+size > j.Cap {
		if segments, err = j.cleanupLocked(segments, true); err != nil {
			return err
		}
		if total(segments)+size > j.Cap {
			return ErrJournalFull
		}
	}
	index := int64(1)
	created := true
	if n := len(segments); n > 0 {
		last := segments[n-1]
		index = last.index
		created = false
		if last.size > 0 && last.size+size > j.SegmentBytes {
			index++
			created = true
		}
	}
	path := filepath.Join(j.Dir, segmentName(index))
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, platform.FileMode)
	if err != nil {
		return err
	}
	if _, err := file.Write(batch.Bytes()); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if created {
		return syncDir(j.Dir)
	}
	return nil
}

// Cleanup removes processed segments older than the retention period.
func (j *Journal) Cleanup() error {
	j.mu.Lock()
	defer j.mu.Unlock()
	unlock, err := j.lock()
	if err != nil {
		return err
	}
	defer unlock()
	segments, err := j.segments()
	if err != nil {
		return err
	}
	_, err = j.cleanupLocked(segments, false)
	return err
}

// cleanupLocked removes fully processed segments: those past retention, and
// with overCap also the oldest processed segments regardless of age.
// Unprocessed data is never removed.
func (j *Journal) cleanupLocked(segments []segment, overCap bool) ([]segment, error) {
	cursor, err := j.Processed()
	if err != nil {
		return segments, err
	}
	now := j.Now()
	var kept []segment
	removed := false
	for _, s := range segments {
		processed := s.index < cursor.Segment
		if processed && (overCap || now.Sub(s.mod) >= j.Retention) {
			if err := os.Remove(s.path); err != nil && !errors.Is(err, os.ErrNotExist) {
				return segments, err
			}
			removed = true
			continue
		}
		kept = append(kept, s)
	}
	if removed {
		if err := syncDir(j.Dir); err != nil {
			return kept, err
		}
	}
	return kept, nil
}

// ReadFrom returns complete records after cursor, up to limit, and the
// position after the last returned record. A partially written final line is
// left for the next read.
func (j *Journal) ReadFrom(cursor store.TelemetryCursor, limit int) ([]Record, store.TelemetryCursor, error) {
	segments, err := j.segments()
	if err != nil {
		return nil, cursor, err
	}
	var records []Record
	next := cursor
	for _, s := range segments {
		if s.index < next.Segment {
			continue
		}
		if s.index > next.Segment {
			// Reached only after the cursor's segment was read to its end
			// (or never existed): appends never leave a partial batch
			// behind in a segment that a later one follows.
			next = store.TelemetryCursor{Segment: s.index, Offset: 0}
		}
		file, err := os.Open(s.path)
		if err != nil {
			return nil, cursor, err
		}
		if _, err := file.Seek(next.Offset, io.SeekStart); err != nil {
			file.Close()
			return nil, cursor, err
		}
		reader := bufio.NewReader(file)
		for len(records) < limit {
			line, err := reader.ReadBytes('\n')
			if err == io.EOF {
				break
			}
			if err != nil {
				file.Close()
				return nil, cursor, err
			}
			var record Record
			if err := json.Unmarshal(line, &record); err != nil {
				record = Record{Invalid: "journal_line_unreadable"}
			}
			records = append(records, record)
			next.Offset += int64(len(line))
		}
		file.Close()
		if len(records) >= limit || next.Offset < s.size {
			break
		}
	}
	return records, next, nil
}

// terminateTornTail durably appends a line feed when the segment does not end
// on one and reports the bytes added. The caller holds the journal lock, so
// no acknowledged write can be in progress.
func terminateTornTail(s segment) (int64, error) {
	if s.size == 0 {
		return 0, nil
	}
	file, err := os.OpenFile(s.path, os.O_RDWR|os.O_APPEND, platform.FileMode)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	last := make([]byte, 1)
	if _, err := file.ReadAt(last, s.size-1); err != nil {
		return 0, err
	}
	if last[0] == '\n' {
		return 0, nil
	}
	if _, err := file.Write([]byte{'\n'}); err != nil {
		return 0, err
	}
	if err := file.Sync(); err != nil {
		return 0, err
	}
	return 1, nil
}

func total(segments []segment) int64 {
	var sum int64
	for _, s := range segments {
		sum += s.size
	}
	return sum
}

func (j *Journal) lock() (func(), error) {
	file, err := os.OpenFile(filepath.Join(j.Dir, ".lock"), os.O_RDWR|os.O_CREATE, platform.FileMode)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		file.Close()
		return nil, err
	}
	return func() {
		_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		file.Close()
	}, nil
}

func syncDir(dir string) error {
	handle, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer handle.Close()
	return handle.Sync()
}
