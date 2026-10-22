package usage

import (
	"sync"
	"testing"
)

func TestScanProfileConcurrentAggregationAndSnapshotIsolation(t *testing.T) {
	var profile ScanProfile
	var workers sync.WaitGroup
	for i := 0; i < 100; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			defer profile.start("source_commit")()
		}()
	}
	workers.Wait()
	report := profile.Report()
	value := report["source_commit"]
	if value.Calls != 100 || value.TotalMS < value.MaxMS || value.MaxMS < 0 {
		t.Fatalf("lost or invalid timings: %+v", value)
	}
	delete(report, "source_commit")
	if profile.Report()["source_commit"].Calls != 100 {
		t.Fatal("caller mutated the live profile")
	}
	var disabled *ScanProfile
	disabled.start("disabled")()
	if disabled.Report() != nil {
		t.Fatal("disabled profile allocated a report")
	}
}
