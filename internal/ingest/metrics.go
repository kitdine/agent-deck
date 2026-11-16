package ingest

import (
	"io"
	"sync/atomic"
	"time"

	"github.com/kitdine/agent-deck/internal/perftrace"
)

// Metrics is enabled with measurement recording. Worker durations overlap
// and are not elapsed wall time.
type Metrics struct {
	active, peak, sourceNS, readNS, decodeNS, budgetNS, usageNS, sessionNS, bytes, reads, records atomic.Int64
}

// Counts returns the integer ingest counters recorded with scan.end. P08
// compares read_bytes and decoded_records exactly across repeated runs.
func (m *Metrics) Counts() []perftrace.Count {
	return []perftrace.Count{
		{Key: "read_bytes", Value: m.bytes.Load()},
		{Key: "read_calls", Value: m.reads.Load()},
		{Key: "decoded_records", Value: m.records.Load()},
		{Key: "peak_open_readers", Value: m.peak.Load()},
		{Key: "source_ns", Value: m.sourceNS.Load()},
		{Key: "read_ns", Value: m.readNS.Load()},
		{Key: "decode_ns", Value: m.decodeNS.Load()},
		{Key: "budget_wait_ns", Value: m.budgetNS.Load()},
		{Key: "usage_send_ns", Value: m.usageNS.Load()},
		{Key: "session_send_ns", Value: m.sessionNS.Load()},
	}
}

type measuredInput struct {
	io.Reader
	metrics *Metrics
}

func (r measuredInput) Read(p []byte) (int, error) {
	start := time.Now()
	n, err := r.Reader.Read(p)
	r.metrics.readNS.Add(time.Since(start).Nanoseconds())
	r.metrics.reads.Add(1)
	r.metrics.bytes.Add(int64(n))
	return n, err
}
