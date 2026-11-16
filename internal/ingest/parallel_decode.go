package ingest

import (
	"context"
	"encoding/json"
	"time"
)

// parallelDecodeMinBytes selects ranges whose decoded weight fills the whole
// default budget. Such a source is admitted alone, so decoding it on the reader
// goroutine leaves the domain consumers waiting on a single core.
const parallelDecodeMinBytes = DefaultBudget / decodedWeightFactor

const (
	parallelDecodeWorkers = 4
	// parallelDecodeWindow bounds batches decoded ahead of publication. All of
	// them belong to the admitted range, whose weight is already reserved.
	parallelDecodeWindow = 2 * parallelDecodeWorkers
)

// pendingBatch holds copied complete lines whose batch boundaries were chosen
// exactly as the sequential reader chooses them.
type pendingBatch struct {
	lines   [][]byte
	offsets []int64
	first   uint64
	bytes   int
	done    chan Batch
}

func (c *Coordinator) decodePending(p *pendingBatch) Batch {
	records := make([]Record, len(p.lines), 256)
	for i, line := range p.lines {
		var value map[string]any
		var decodeStart time.Time
		if c.metrics != nil {
			decodeStart = time.Now()
		}
		decodeErr := json.Unmarshal(line[:len(line)-1], &value)
		if c.metrics != nil {
			c.metrics.decodeNS.Add(time.Since(decodeStart).Nanoseconds())
			c.metrics.records.Add(1)
		}
		records[i] = Record{Offset: p.offsets[i], End: p.offsets[i] + int64(len(line)), Sequence: p.first + uint64(i), Kind: classify(value), Value: value, Malformed: decodeErr != nil}
	}
	return Batch{Records: records}
}

// orderedDecoder decodes batches on a bounded worker pool and publishes them in
// submission order from the reader goroutine. Publication still blocks on the
// consumers; only decoding of the next few batches proceeds meanwhile.
type orderedDecoder struct {
	c      *Coordinator
	ctx    context.Context
	path   string
	jobs   chan *pendingBatch
	window []*pendingBatch
}

func (c *Coordinator) newOrderedDecoder(ctx context.Context, path string) *orderedDecoder {
	d := &orderedDecoder{c: c, ctx: ctx, path: path, jobs: make(chan *pendingBatch, parallelDecodeWindow)}
	for range parallelDecodeWorkers {
		go func() {
			for p := range d.jobs {
				p.done <- c.decodePending(p)
			}
		}()
	}
	return d
}

func (d *orderedDecoder) submit(p *pendingBatch) error {
	p.done = make(chan Batch, 1)
	d.jobs <- p
	d.window = append(d.window, p)
	if len(d.window) >= parallelDecodeWindow {
		return d.publishHead()
	}
	return nil
}

func (d *orderedDecoder) publishHead() error {
	head := d.window[0]
	d.window[0] = nil
	d.window = d.window[1:]
	return d.c.publish(d.ctx, <-head.done, d.path)
}

func (d *orderedDecoder) drain() error {
	for len(d.window) > 0 {
		if err := d.publishHead(); err != nil {
			return err
		}
	}
	return nil
}

// close stops the workers. Abandoned batches finish into buffered channels.
func (d *orderedDecoder) close() { close(d.jobs) }
