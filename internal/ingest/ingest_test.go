package ingest

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestDiscoverProvidesBothDomainOrdersFromOneInventory(t *testing.T) {
	home := t.TempDir()
	paths := []string{
		filepath.Join(home, ".codex", "sessions", "z.jsonl"),
		filepath.Join(home, ".codex", "archived_sessions", "a.jsonl"),
		filepath.Join(home, ".claude", "projects", "p", "m.jsonl"),
	}
	for _, path := range paths {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("{}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	sources, err := Discover(home)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, len(sources))
	for index, source := range sources {
		got[index] = source.Client + ":" + filepath.Base(source.Path)
	}
	want := []string{"claude:m.jsonl", "codex:a.jsonl", "codex:z.jsonl"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("inventory order=%v want=%v", got, want)
	}
}

func TestDiscoverReturnsPreparedEmptyInventory(t *testing.T) {
	sources, err := Discover(t.TempDir())
	if err != nil || sources == nil || len(sources) != 0 {
		t.Fatalf("sources=%#v err=%v", sources, err)
	}
}

func TestCoordinatorSharesOneStableDecodeAndReleasesItAfterBothConsumers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source.jsonl")
	if err := os.WriteFile(path, []byte("{\"n\":1}\ninvalid\npartial"), 0o600); err != nil {
		t.Fatal(err)
	}
	source := testSource(t, path)
	var opens atomic.Int64
	coordinator := NewCoordinator([]Source{source}, Options{Open: func(path string) (sourceFile, error) {
		opens.Add(1)
		return os.Open(path)
	}})
	first, second := consumePair(t, coordinator, path)
	if opens.Load() != 1 {
		t.Fatalf("source opens=%d", opens.Load())
	}
	if !reflect.DeepEqual(first.Records, second.Records) || len(first.Records) != 2 || !first.Records[1].Malformed || first.Records[0].Sequence != 1 || string(first.Tail) != "partial" {
		t.Fatalf("snapshot=%#v", first)
	}
}

func TestCoordinatorProcessesOversizedSourceExclusivelyOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "large.jsonl")
	if err := os.WriteFile(path, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	source := testSource(t, path)
	var opens atomic.Int64
	coordinator := NewCoordinator([]Source{source}, Options{Budget: 1, Open: func(string) (sourceFile, error) {
		opens.Add(1)
		return os.Open(path)
	}})
	consumePair(t, coordinator, path)
	if opens.Load() != 1 {
		t.Fatalf("oversized source opens=%d", opens.Load())
	}
}

func TestCoordinatorStartUsesConfiguredWorkersAndKeepsBothConsumerReads(t *testing.T) {
	root := t.TempDir()
	var sources []Source
	for index := 0; index < 8; index++ {
		path := filepath.Join(root, string(rune('a'+index))+".jsonl")
		if err := os.WriteFile(path, []byte("{}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		sources = append(sources, testSource(t, path))
	}
	var active, maximum, opens atomic.Int64
	coordinator := NewCoordinator(sources, Options{Workers: 2, Open: func(path string) (sourceFile, error) {
		opens.Add(1)
		current := active.Add(1)
		for observed := maximum.Load(); current > observed && !maximum.CompareAndSwap(observed, current); observed = maximum.Load() {
		}
		time.Sleep(20 * time.Millisecond)
		active.Add(-1)
		return os.Open(path)
	}})
	coordinator.Start(context.Background())
	for _, source := range sources {
		consumePair(t, coordinator, source.Path)
	}
	if maximum.Load() != 2 || opens.Load() != int64(len(sources)) {
		t.Fatalf("maximum=%d opens=%d", maximum.Load(), opens.Load())
	}
}

func TestCoordinatorRejectsChangedSource(t *testing.T) {
	path := filepath.Join(t.TempDir(), "changed.jsonl")
	if err := os.WriteFile(path, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	source := testSource(t, path)
	if err := os.WriteFile(path, []byte("[]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	coordinator := NewCoordinator([]Source{source}, Options{})
	coordinator.ConsumerDone(ConsumerSession)
	if _, shared, err := coordinator.Snapshot(context.Background(), path, ConsumerUsage); !shared || !errors.Is(err, ErrSourceChanged) {
		t.Fatalf("shared=%t err=%v", shared, err)
	}
}

func TestCoordinatorReadsCapturedPrefixAfterAppendOnlyGrowth(t *testing.T) {
	path := filepath.Join(t.TempDir(), "growing.jsonl")
	if err := os.WriteFile(path, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	source := testSource(t, path)
	if err := os.WriteFile(path, []byte("{}\n{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	coordinator := NewCoordinator([]Source{source}, Options{})
	coordinator.ConsumerDone(ConsumerSession)
	snapshot, shared, err := coordinator.Snapshot(context.Background(), path, ConsumerUsage)
	if err != nil || !shared || len(snapshot.Records) != 1 || snapshot.Records[0].End != source.Size {
		t.Fatalf("snapshot=%#v shared=%t err=%v", snapshot, shared, err)
	}
}

func TestCoordinatorConsumerExitReleasesBackpressureForRemainingDomain(t *testing.T) {
	root := t.TempDir()
	var sources []Source
	for index := 0; index < 3; index++ {
		path := filepath.Join(root, string(rune('a'+index))+".jsonl")
		if err := os.WriteFile(path, []byte("{}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		sources = append(sources, testSource(t, path))
	}
	coordinator := NewCoordinator(sources, Options{Budget: sources[0].Size * decodedWeightFactor})
	coordinator.Start(context.Background())
	coordinator.ConsumerDone(ConsumerUsage)
	for _, source := range sources {
		if _, shared, err := coordinator.Snapshot(context.Background(), source.Path, ConsumerSession); err != nil || !shared {
			t.Fatalf("%s shared=%t err=%v", source.Path, shared, err)
		}
	}
}

func TestCoordinatorCancelsProducerWhenAllConsumersExit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source.jsonl")
	if err := os.WriteFile(path, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	coordinator := NewCoordinator([]Source{testSource(t, path)}, Options{})
	coordinator.Start(context.Background())
	coordinator.ConsumerDone(ConsumerUsage)
	coordinator.ConsumerDone(ConsumerSession)
	done := make(chan struct{})
	go func() {
		coordinator.Close()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("coordinator did not stop after all consumers exited")
	}
}

func TestCoordinatorRequiresBothSealedPlansBeforeBodyAdmission(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source.jsonl")
	if err := os.WriteFile(path, []byte("{}\n{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	source := testSource(t, path)
	opened := make(chan struct{}, 1)
	coordinator := NewCoordinator([]Source{source}, Options{
		RequirePlans: true,
		Open: func(name string) (sourceFile, error) {
			opened <- struct{}{}
			return os.Open(name)
		},
	})
	defer coordinator.Close()
	coordinator.Start(context.Background())
	select {
	case <-opened:
		t.Fatal("body read started before either domain sealed its plan")
	case <-time.After(50 * time.Millisecond):
	}
	coordinator.Skip(path, ConsumerUsage)
	if err := coordinator.Seal(ConsumerUsage); err != nil {
		t.Fatal(err)
	}
	select {
	case <-opened:
		t.Fatal("body read started before the session plan sealed")
	case <-time.After(50 * time.Millisecond):
	}
	coordinator.Skip(path, ConsumerSession)
	if err := coordinator.Seal(ConsumerSession); err != nil {
		t.Fatal(err)
	}
	select {
	case <-coordinator.done:
	case <-time.After(time.Second):
		t.Fatal("both-skip plan did not terminate")
	}
	select {
	case <-opened:
		t.Fatal("both-skip plan opened a source body")
	default:
	}
}

func TestCoordinatorReadsOnlyPlannedUnionRange(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source.jsonl")
	first := []byte("{}\n")
	if err := os.WriteFile(path, append(first, []byte("{}\n")...), 0o600); err != nil {
		t.Fatal(err)
	}
	source := testSource(t, path)
	coordinator := NewCoordinator([]Source{source}, Options{RequirePlans: true})
	defer coordinator.Close()
	if err := coordinator.Plan(path, ConsumerUsage, ReadRange{Start: int64(len(first)), End: source.Size}); err != nil {
		t.Fatal(err)
	}
	coordinator.Skip(path, ConsumerSession)
	if err := coordinator.Seal(ConsumerUsage); err != nil {
		t.Fatal(err)
	}
	if err := coordinator.Seal(ConsumerSession); err != nil {
		t.Fatal(err)
	}
	coordinator.Start(context.Background())
	snapshot, shared, err := coordinator.Snapshot(context.Background(), path, ConsumerUsage)
	if err != nil || !shared {
		t.Fatalf("shared=%t err=%v", shared, err)
	}
	if len(snapshot.Records) != 1 || snapshot.Records[0].Offset != int64(len(first)) {
		t.Fatalf("planned append snapshot=%#v", snapshot)
	}
}

func TestCoordinatorAbandonsOnePartialPlanWithoutBlockingOtherConsumer(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source.jsonl")
	if err := os.WriteFile(path, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	source := testSource(t, path)
	coordinator := NewCoordinator([]Source{source}, Options{RequirePlans: true})
	defer coordinator.Close()
	if err := coordinator.Plan(path, ConsumerUsage, ReadRange{End: source.Size}); err != nil {
		t.Fatal(err)
	}
	if err := coordinator.AbandonPlan(ConsumerUsage); err != nil {
		t.Fatal(err)
	}
	if err := coordinator.Plan(path, ConsumerSession, ReadRange{End: source.Size}); err != nil {
		t.Fatal(err)
	}
	if err := coordinator.Seal(ConsumerSession); err != nil {
		t.Fatal(err)
	}
	coordinator.ConsumerDone(ConsumerUsage)
	coordinator.Start(context.Background())
	snapshot, shared, err := coordinator.Snapshot(context.Background(), path, ConsumerSession)
	if err != nil || !shared || len(snapshot.Records) != 1 {
		t.Fatalf("session snapshot=%#v shared=%t err=%v", snapshot, shared, err)
	}
}

func TestDynamicWorkersUsesEnvironmentAndTaskBounds(t *testing.T) {
	if got := DynamicWorkers(1); got != 1 {
		t.Fatalf("one task workers=%d", got)
	}
	if got := DynamicWorkers(0); got != 0 {
		t.Fatalf("zero task workers=%d", got)
	}
	if got := DynamicWorkers(1000); got < 2 || got > 32 {
		t.Fatalf("large task workers=%d", got)
	}
}

func TestDiscoverKeepsExistingJSONLFileSymlinkBehavior(t *testing.T) {
	home := t.TempDir()
	target := filepath.Join(t.TempDir(), "target.jsonl")
	link := filepath.Join(home, ".codex", "sessions", "linked.jsonl")
	if err := os.MkdirAll(filepath.Dir(link), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	sources, err := Discover(home)
	if err != nil || len(sources) != 1 {
		t.Fatalf("sources=%#v err=%v", sources, err)
	}
	targetInfo, _ := os.Stat(target)
	if sources[0].Identity != FileIdentity(targetInfo) || sources[0].Size != targetInfo.Size() {
		t.Fatalf("symlink source=%#v target=%#v", sources[0], targetInfo)
	}
}

func testSource(t *testing.T, path string) Source {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	identity, changedAt, stable := fileGeneration(info)
	return Source{Path: path, Identity: identity, Size: info.Size(), ModifiedAt: info.ModTime().UnixNano(), ChangedAt: changedAt, Stable: stable}
}

func TestReaderAmortizesFileReadsWithoutChangingRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "large.jsonl")
	line := `{"text":"` + strings.Repeat("a", 6000) + `"}` + "\n"
	if err := os.WriteFile(path, []byte(strings.Repeat(line, 100)), 0600); err != nil {
		t.Fatal(err)
	}
	metrics := &Metrics{}
	c := NewCoordinator([]Source{testSource(t, path)}, Options{Metrics: metrics})
	defer c.Close()
	first, second := consumePair(t, c, path)
	c.Close()
	report := metrics.Report(time.Second)
	if report["read_bytes"] != float64(len(line)*100) || report["read_calls"] > 3 || report["peak_open_readers"] != 1 {
		t.Fatalf("invalid read metrics: %v", report)
	}
	if len(first.Records) != 100 || !reflect.DeepEqual(first.Records, second.Records) {
		t.Fatal("reader changed record stream")
	}
}

func consumePair(t *testing.T, coordinator *Coordinator, path string) (Snapshot, Snapshot) {
	t.Helper()
	type result struct {
		snapshot Snapshot
		shared   bool
		err      error
	}
	results := make(chan result, 2)
	for _, consumer := range []string{ConsumerUsage, ConsumerSession} {
		consumer := consumer
		go func() {
			snapshot, shared, err := coordinator.Snapshot(context.Background(), path, consumer)
			results <- result{snapshot: snapshot, shared: shared, err: err}
		}()
	}
	first, second := <-results, <-results
	for _, got := range []result{first, second} {
		if got.err != nil || !got.shared {
			t.Fatalf("%s shared=%t err=%v", path, got.shared, got.err)
		}
	}
	return first.snapshot, second.snapshot
}
