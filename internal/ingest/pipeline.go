package ingest

import (
	"context"
	"strconv"
	"sync"
)

// reduceWorkers is the per-domain reduce worker count. It is a string so a
// candidate build can select 1, 2, 4 or 8 with -ldflags -X; the shipped value
// is the one the full-cycle comparison selected.
var reduceWorkers = "4"

// ReduceWorkers returns the configured per-domain reduce worker count.
func ReduceWorkers() int {
	workers, err := strconv.Atoi(reduceWorkers)
	if err != nil || workers < 1 {
		return 1
	}
	return min(workers, 32)
}

// Reserver accounts reduce results waiting for, or held by, a domain writer in
// the same admission budget as the source reads. The head result, the next one
// the writer needs, is always admitted so the writer can make progress.
type Reserver interface {
	Reserve(ctx context.Context, weight int64, head func() bool) error
	Release(weight int64)
	// Wake re-evaluates waiting reservations after the writer's head moved.
	Wake()
}

// OrderedOptions configures Ordered.
type OrderedOptions struct {
	Workers int
	// Window bounds the sources reduced ahead of the writer; at least Workers.
	Window int
	// Budget, when set, accounts each result's weight until its commit.
	Budget Reserver
	// Abort, when set, is called once after a failure, after the results
	// before it are flushed and before Ordered waits for the running
	// reductions. It must end their waits on input the failed domain no
	// longer consumes, such as a shared reader's sends to it.
	Abort func()
}

// Reduced is one source's reduce outcome.
type Reduced[T any] struct {
	Value  T
	Weight int64
	Err    error
	// Discard, when set, releases what Value owns. Ordered calls it exactly
	// once when a successful result is never passed to write; a value passed
	// to write is owned by write, also when write fails.
	Discard func()
}

func (r Reduced[T]) discard() {
	if r.Err == nil && r.Discard != nil {
		r.Discard()
	}
}

// Ordered reduces items 0..n-1 on a bounded worker pool and hands their
// results to write strictly in index order on the calling goroutine, so all
// cross-source decisions stay in the writer exactly as in a serial loop.
//
// write receives whether the next result is already reduced, so a writer
// that groups several sources into one transaction never holds it open while
// waiting for a reduce. It reports whether its call committed everything
// written so far; the weights of committed results are then released. flush commits any
// remainder and is called before Ordered returns, also after an error, so
// results written before the first failure stay committed as they would be
// serially. A reduce error is returned when the writer reaches its index;
// later results are discarded after their workers finish.
func Ordered[T any](ctx context.Context, n int, options OrderedOptions, reduce func(int) Reduced[T], write func(index int, value T, nextReady bool) (bool, error), flush func() error) error {
	workers := max(1, min(options.Workers, n))
	window := max(workers, options.Window)
	slots := make([]chan Reduced[T], n)
	for i := range slots {
		slots[i] = make(chan Reduced[T], 1)
	}
	var mu sync.Mutex
	cond := sync.NewCond(&mu)
	next, written, stopped := 0, 0, false
	head := func(index int) func() bool {
		return func() bool {
			mu.Lock()
			defer mu.Unlock()
			return index == written || stopped
		}
	}
	var wait sync.WaitGroup
	for range workers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for {
				mu.Lock()
				for !stopped && next < n && next >= written+window {
					cond.Wait()
				}
				if stopped || next >= n {
					mu.Unlock()
					return
				}
				index := next
				next++
				mu.Unlock()
				result := reduce(index)
				if result.Err == nil && options.Budget != nil {
					if err := options.Budget.Reserve(ctx, result.Weight, head(index)); err != nil {
						result.discard()
						result = Reduced[T]{Err: err}
					}
				}
				if result.Err != nil {
					result.Weight = 0
				}
				// After a failure nothing is written, so neither the result
				// nor its reservation may outlive this worker: a still-running
				// worker may need the budget for its source read. Release
				// outside mu, because Reserve evaluates head under the
				// budget's own lock.
				mu.Lock()
				late := stopped
				if !late {
					slots[index] <- result
				}
				mu.Unlock()
				if late {
					result.discard()
					if options.Budget != nil && result.Weight > 0 {
						options.Budget.Release(result.Weight)
					}
				}
			}
		}()
	}
	var held int64
	release := func() {
		if options.Budget != nil && held > 0 {
			options.Budget.Release(held)
		}
		held = 0
	}
	var failure error
	for index := 0; index < n; index++ {
		result := <-slots[index]
		if result.Err != nil {
			failure = result.Err
			break
		}
		held += result.Weight
		nextReady := index+1 < n && len(slots[index+1]) > 0
		committed, err := write(index, result.Value, nextReady)
		if committed {
			release()
		}
		if err != nil {
			failure = err
			break
		}
		mu.Lock()
		written = index + 1
		cond.Broadcast()
		mu.Unlock()
		if options.Budget != nil {
			options.Budget.Wake()
		}
	}
	if err := flush(); failure == nil {
		failure = err
	}
	release()
	if failure != nil {
		// Results reduced past the failure are never written. Release those
		// already delivered; workers still running release their own.
		mu.Lock()
		stopped = true
		cond.Broadcast()
		var delivered []Reduced[T]
		for _, slot := range slots {
			select {
			case result := <-slot:
				delivered = append(delivered, result)
			default:
			}
		}
		mu.Unlock()
		var reserved int64
		for _, result := range delivered {
			reserved += result.Weight
			result.discard()
		}
		if options.Budget != nil {
			options.Budget.Release(reserved)
			options.Budget.Wake()
		}
		if options.Abort != nil {
			options.Abort()
		}
	}
	wait.Wait()
	return failure
}

// Reserve accounts weight bytes of reduce results in the shared admission
// budget. It waits while the budget is full unless head reports that the
// result is the next one its writer needs.
func (c *Coordinator) Reserve(ctx context.Context, weight int64, head func() bool) error {
	if weight <= 0 {
		return nil
	}
	for {
		c.mu.Lock()
		if c.used+weight <= c.budget || c.used == 0 || head() {
			c.used += weight
			c.mu.Unlock()
			return nil
		}
		changed := c.released
		c.mu.Unlock()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-changed:
		}
	}
}

// Release returns reduce result weight reserved with Reserve.
func (c *Coordinator) Release(weight int64) {
	if weight <= 0 {
		return
	}
	c.mu.Lock()
	c.used -= weight
	c.broadcastLocked()
	c.mu.Unlock()
	select {
	case c.wake <- struct{}{}:
	default:
	}
}

// Wake re-evaluates waiting reservations.
func (c *Coordinator) Wake() {
	c.mu.Lock()
	c.broadcastLocked()
	c.mu.Unlock()
}

// broadcastLocked wakes every Reserve waiter; c.mu must be held.
func (c *Coordinator) broadcastLocked() {
	close(c.released)
	c.released = make(chan struct{})
}
