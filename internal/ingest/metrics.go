package ingest

import (
	"sync/atomic"
	"time"
)

// Metrics is opt-in. Worker durations overlap and are not elapsed wall time.
type Metrics struct {
	active, peak, sourceNS, readNS, decodeNS, budgetNS, usageNS, sessionNS, bytes, reads atomic.Int64
}

func (m *Metrics) Report(wall time.Duration) map[string]float64 {
	average := float64(0)
	if wall > 0 {
		average = float64(m.sourceNS.Load()) / float64(wall)
	}
	return map[string]float64{
		"peak_open_readers": float64(m.peak.Load()), "average_open_readers": average,
		"read_ms": float64(m.readNS.Load()) / 1e6, "decode_ms": float64(m.decodeNS.Load()) / 1e6,
		"budget_wait_ms": float64(m.budgetNS.Load()) / 1e6,
		"usage_send_ms":  float64(m.usageNS.Load()) / 1e6, "session_send_ms": float64(m.sessionNS.Load()) / 1e6,
		"source_lifetime_ms": float64(m.sourceNS.Load()) / 1e6, "read_bytes": float64(m.bytes.Load()), "read_calls": float64(m.reads.Load()),
	}
}

type measuredReader struct {
	sourceFile
	metrics *Metrics
}

func (r measuredReader) Read(p []byte) (int, error) {
	start := time.Now()
	n, err := r.sourceFile.Read(p)
	r.metrics.readNS.Add(time.Since(start).Nanoseconds())
	r.metrics.reads.Add(1)
	r.metrics.bytes.Add(int64(n))
	return n, err
}
