package ingest

import (
	"context"
	"errors"
	"math/rand/v2"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestOrderedWritesInIndexOrderWhileReducingAhead(t *testing.T) {
	var active, peak atomic.Int64
	var written []int
	err := Ordered(context.Background(), 200, OrderedOptions{Workers: 4, Window: 8},
		func(index int) Reduced[int] {
			n := active.Add(1)
			for p := peak.Load(); n > p && !peak.CompareAndSwap(p, n); p = peak.Load() {
			}
			time.Sleep(time.Duration(rand.IntN(200)) * time.Microsecond)
			active.Add(-1)
			return Reduced[int]{Value: index * 10}
		},
		func(index, value int, _ bool) (bool, error) {
			if value != index*10 {
				t.Fatalf("index %d got %d", index, value)
			}
			written = append(written, index)
			return true, nil
		},
		func() error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	for i, index := range written {
		if i != index {
			t.Fatalf("written out of order: %v", written[:i+1])
		}
	}
	if len(written) != 200 || peak.Load() < 2 || peak.Load() > 4 {
		t.Fatalf("written=%d peak workers=%d", len(written), peak.Load())
	}
}

func TestOrderedStopsAtFirstErrorAfterFlushingEarlierResults(t *testing.T) {
	failure := errors.New("reduce failed")
	var written []int
	flushed := 0
	err := Ordered(context.Background(), 50, OrderedOptions{Workers: 4, Window: 8},
		func(index int) Reduced[int] {
			if index == 20 {
				return Reduced[int]{Err: failure}
			}
			return Reduced[int]{Value: index}
		},
		func(index, _ int, _ bool) (bool, error) {
			written = append(written, index)
			return false, nil
		},
		func() error { flushed++; return nil })
	if !errors.Is(err, failure) || len(written) != 20 || written[19] != 19 || flushed != 1 {
		t.Fatalf("err=%v written=%v flushed=%d", err, written, flushed)
	}
}

// Results are reserved in the shared budget and released only on commit. A
// budget smaller than one result must still let the writer's head proceed.
func TestOrderedBudgetAdmitsHeadAndReleasesOnCommit(t *testing.T) {
	coordinator := NewCoordinator(nil, Options{Budget: 10})
	var maxUsed int64
	commits := 0
	err := Ordered(context.Background(), 40, OrderedOptions{Workers: 4, Window: 8, Budget: coordinator},
		func(index int) Reduced[int] { return Reduced[int]{Value: index, Weight: 25} },
		func(index, _ int, nextReady bool) (bool, error) {
			coordinator.mu.Lock()
			maxUsed = max(maxUsed, coordinator.used)
			coordinator.mu.Unlock()
			if index%3 == 2 || !nextReady {
				commits++
				return true, nil
			}
			return false, nil
		},
		func() error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	coordinator.mu.Lock()
	used := coordinator.used
	coordinator.mu.Unlock()
	if used != 0 || commits == 0 {
		t.Fatalf("budget leaked: used=%d commits=%d", used, commits)
	}
	// At most the uncommitted head group may exceed the budget.
	if maxUsed > 3*25 {
		t.Fatalf("results reserved past the head group: %d", maxUsed)
	}
}

// After a failure, reductions still running may need budget for their own
// source reads. Reserved results that will never be written must not hold it.
func TestOrderedFailureReleasesReservationsForRunningReductions(t *testing.T) {
	coordinator := NewCoordinator(nil, Options{Budget: 100})
	failure := errors.New("reduce failed")
	used := func() int64 {
		coordinator.mu.Lock()
		defer coordinator.mu.Unlock()
		return coordinator.used
	}
	done := make(chan error, 1)
	go func() {
		done <- Ordered(context.Background(), 12, OrderedOptions{Workers: 4, Window: 8, Budget: coordinator},
			func(index int) Reduced[int] {
				switch {
				case index == 2:
					// Fail only once later results hold the budget.
					for used() < 75 {
						time.Sleep(time.Millisecond)
					}
					return Reduced[int]{Err: failure}
				case index > 5:
					// A read admitted only when the budget drains.
					for used() != 0 {
						time.Sleep(time.Millisecond)
					}
				}
				return Reduced[int]{Value: index, Weight: 25}
			},
			func(int, int, bool) (bool, error) { return true, nil },
			func() error { return nil })
	}()
	select {
	case err := <-done:
		if !errors.Is(err, failure) {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatalf("reductions after a failure waited on reservations that were never written: used=%d", used())
	}
	if got := used(); got != 0 {
		t.Fatalf("budget leaked after failure: %d", got)
	}
}

// ownership records, per index, how a successfully reduced value ended.
type ownership struct {
	mu        sync.Mutex
	reduced   map[int]bool
	written   map[int]int
	discarded map[int]int
}

func newOwnership() *ownership {
	return &ownership{reduced: map[int]bool{}, written: map[int]int{}, discarded: map[int]int{}}
}

func (o *ownership) result(index int, weight int64) Reduced[int] {
	o.mu.Lock()
	o.reduced[index] = true
	o.mu.Unlock()
	return Reduced[int]{Value: index, Weight: weight, Discard: func() {
		o.mu.Lock()
		o.discarded[index]++
		o.mu.Unlock()
	}}
}

func (o *ownership) write(index int) {
	o.mu.Lock()
	o.written[index]++
	o.mu.Unlock()
}

func (o *ownership) discards() int {
	o.mu.Lock()
	defer o.mu.Unlock()
	return len(o.discarded)
}

// check requires every reduced value to be written or discarded exactly once,
// and reports how many were discarded.
func (o *ownership) check(t *testing.T) int {
	t.Helper()
	o.mu.Lock()
	defer o.mu.Unlock()
	for index := range o.reduced {
		if o.written[index]+o.discarded[index] != 1 {
			t.Fatalf("index %d written %d discarded %d times", index, o.written[index], o.discarded[index])
		}
	}
	for index := range o.discarded {
		if !o.reduced[index] {
			t.Fatalf("index %d discarded without a reduced value", index)
		}
	}
	return len(o.discarded)
}

// After a failure, results already delivered and results whose reduce ends
// later are never written; each releases what it owns exactly once.
func TestOrderedDiscardsUnwrittenResultsOnce(t *testing.T) {
	failure := errors.New("reduce failed")
	owned := newOwnership()
	delivered := make(chan struct{})
	var ready atomic.Int64
	err := Ordered(context.Background(), 12, OrderedOptions{Workers: 4, Window: 8},
		func(index int) Reduced[int] {
			switch {
			case index == 2:
				// Fail once later results are queued for the writer.
				<-delivered
				return Reduced[int]{Err: failure}
			case index > 6:
				// Still reducing when the writer fails.
				time.Sleep(50 * time.Millisecond)
			case index > 2:
				if ready.Add(1) == 4 {
					close(delivered)
				}
			}
			return owned.result(index, 0)
		},
		func(index, _ int, _ bool) (bool, error) {
			owned.write(index)
			return true, nil
		},
		func() error { return nil })
	if !errors.Is(err, failure) {
		t.Fatal(err)
	}
	if discarded := owned.check(t); discarded < 5 {
		t.Fatalf("only %d results discarded", discarded)
	}
}

// Without a budget a weighted result that finishes only after the failure is
// still discarded once, and nothing is released.
func TestOrderedDiscardsLateWeightedResultWithoutBudget(t *testing.T) {
	failure := errors.New("reduce failed")
	started, resume := make(chan struct{}), make(chan struct{})
	var discards atomic.Int64
	err := Ordered(context.Background(), 2, OrderedOptions{Workers: 2, Abort: func() { close(resume) }},
		func(index int) Reduced[int] {
			if index == 0 {
				<-started
				return Reduced[int]{Err: failure}
			}
			close(started)
			// Finishes only once the writer has stopped.
			<-resume
			return Reduced[int]{Value: index, Weight: 1, Discard: func() { discards.Add(1) }}
		},
		func(int, int, bool) (bool, error) {
			t.Error("a result after the failure was written")
			return true, nil
		},
		func() error { return nil })
	if !errors.Is(err, failure) || discards.Load() != 1 {
		t.Fatalf("err=%v discards=%d", err, discards.Load())
	}
}

// A result whose reservation fails is never written and is discarded.
func TestOrderedDiscardsResultWhoseReservationFails(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	coordinator := NewCoordinator(nil, Options{Budget: 10})
	owned := newOwnership()
	headWriting := make(chan struct{})
	err := Ordered(ctx, 4, OrderedOptions{Workers: 4, Window: 4, Budget: coordinator},
		func(index int) Reduced[int] {
			if index > 0 {
				// Reserve only behind the head's reservation, which an empty
				// budget would otherwise not require.
				<-headWriting
			}
			return owned.result(index, 25)
		},
		func(index, _ int, _ bool) (bool, error) {
			owned.write(index)
			if index == 0 {
				// While this uncommitted head holds the budget, no other
				// result is admitted; cancelling fails every reservation.
				close(headWriting)
				cancel()
				deadline := time.Now().Add(10 * time.Second)
				for owned.discards() < 3 && time.Now().Before(deadline) {
					time.Sleep(time.Millisecond)
				}
			}
			return false, nil
		},
		func() error { return nil })
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if discarded := owned.check(t); discarded != 3 {
		t.Fatalf("discarded %d results, want 3", discarded)
	}
	coordinator.mu.Lock()
	used := coordinator.used
	coordinator.mu.Unlock()
	if used != 0 {
		t.Fatalf("budget leaked: %d", used)
	}
}

// Reductions may wait on input only the failed domain would have consumed,
// such as a shared reader blocked on the failed source. Abort must end that
// wait before Ordered waits for them, without an outside cancellation.
func TestOrderedAbortEndsReductionsWaitingOnFailedInput(t *testing.T) {
	failure := errors.New("reduce failed")
	input := make(chan struct{})
	var aborts atomic.Int64
	done := make(chan error, 1)
	go func() {
		done <- Ordered(context.Background(), 4, OrderedOptions{Workers: 4, Abort: func() {
			aborts.Add(1)
			close(input)
		}},
			func(index int) Reduced[int] {
				if index == 0 {
					return Reduced[int]{Err: failure}
				}
				<-input
				return Reduced[int]{Value: index}
			},
			func(int, int, bool) (bool, error) { return true, nil },
			func() error { return nil })
	}()
	select {
	case err := <-done:
		if !errors.Is(err, failure) || aborts.Load() != 1 {
			t.Fatalf("err=%v aborts=%d", err, aborts.Load())
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Ordered waited for reductions blocked on the failed domain's input")
	}
}
