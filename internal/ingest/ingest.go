// Package ingest discovers and decodes transcript inputs once for all index consumers.
package ingest

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	DefaultBudget       = int64(64 << 20)
	decodedWeightFactor = int64(64)
	ConsumerUsage       = "usage"
	ConsumerSession     = "session"
)

var ErrSourceChanged = errors.New("shared ingestion source changed")
var ErrPlanIncomplete = errors.New("shared ingestion plan is incomplete")
var ErrPlanMismatch = errors.New("shared ingestion plan does not cover requested source")

type Source struct {
	Client     string
	Path       string
	Priority   int
	Identity   string
	Size       int64
	ModifiedAt int64
	ChangedAt  int64
	Stable     bool
}

type EventKind string

const (
	EventContext  EventKind = "context"
	EventUsage    EventKind = "usage"
	EventTool     EventKind = "tool"
	EventDocument EventKind = "document"
	EventOther    EventKind = "other"
)

type Record struct {
	Offset, End int64
	Sequence    uint64
	Kind        EventKind
	Value       map[string]any
	Malformed   bool
}

type Batch struct {
	Records []Record
}

type Stream struct {
	Source  Source
	Batches <-chan Batch
	entry   *streamEntry
}

func (s Stream) Wait(ctx context.Context) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.entry.finished:
		return append([]byte(nil), s.entry.tail...), s.entry.err
	}
}

type sourceFile interface {
	io.Reader
	io.Closer
	Stat() (os.FileInfo, error)
}

type Options struct {
	Metrics      *Metrics
	Workers      int
	Budget       int64
	Open         func(string) (sourceFile, error)
	RequirePlans bool
}

// ReadRange is a validated byte interval needed by one domain. The coordinator
// reads the union only after both domains seal their finite source plans.
type ReadRange struct {
	Start int64
	End   int64
}

type discoveryRoot struct {
	client   string
	path     string
	priority int
}

func Discover(home string) ([]Source, error) {
	roots := []discoveryRoot{
		{client: "codex", path: filepath.Join(home, ".codex", "archived_sessions"), priority: 0},
		{client: "claude", path: filepath.Join(home, ".claude", "projects"), priority: 0},
		{client: "codex", path: filepath.Join(home, ".codex", "sessions"), priority: 1},
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	jobs, results := make(chan discoveryRoot), make(chan Source)
	errorsFound := make(chan error, 1)
	var wait sync.WaitGroup
	for worker := 0; worker < DynamicWorkers(len(roots)); worker++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for root := range jobs {
				err := filepath.WalkDir(root.path, func(path string, entry fs.DirEntry, walkErr error) error {
					if walkErr != nil {
						return walkErr
					}
					if err := ctx.Err(); err != nil {
						return err
					}
					if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
						return nil
					}
					info, err := os.Stat(path)
					if err != nil {
						return err
					}
					identity, changedAt, stable := fileGeneration(info)
					source := Source{Client: root.client, Path: filepath.Clean(path), Priority: root.priority, Identity: identity, Size: info.Size(), ModifiedAt: info.ModTime().UnixNano(), ChangedAt: changedAt, Stable: stable}
					select {
					case results <- source:
						return nil
					case <-ctx.Done():
						return ctx.Err()
					}
				})
				if err != nil && !errors.Is(err, fs.ErrNotExist) && !errors.Is(err, context.Canceled) {
					select {
					case errorsFound <- err:
						cancel()
					default:
					}
				}
			}
		}()
	}
	go func() {
		defer close(jobs)
		for _, root := range roots {
			select {
			case jobs <- root:
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() { wait.Wait(); close(results) }()
	sources := make([]Source, 0)
	for source := range results {
		sources = append(sources, source)
	}
	select {
	case err := <-errorsFound:
		return nil, err
	default:
	}
	sort.Slice(sources, func(i, j int) bool {
		if sources[i].Priority != sources[j].Priority {
			return sources[i].Priority < sources[j].Priority
		}
		return sources[i].Path < sources[j].Path
	})
	return sources, nil
}

func DynamicWorkers(tasks int) int {
	if tasks <= 0 {
		return 0
	}
	workers := runtime.NumCPU()
	if workers < 2 {
		workers = 2
	}
	if workers > 32 {
		workers = 32
	}
	if workers > tasks {
		workers = tasks
	}
	return workers
}

func FileIdentity(info os.FileInfo) string {
	identity, _, _ := fileGeneration(info)
	return identity
}

// FileGeneration returns the strongest portable metadata tuple available to
// the shared planner. Stable is false when the platform cannot expose the
// change-time component required to skip a content anchor read safely.
func FileGeneration(info os.FileInfo) (identity string, changedAt int64, stable bool) {
	return fileGeneration(info)
}

type streamEntry struct {
	source           Source
	channels         map[string]chan Batch
	skipCh           map[string]chan struct{}
	skipped          map[string]bool
	allSkipped       chan struct{}
	allSkippedClosed bool
	finished         chan struct{}
	tail             []byte
	err              error
	weight           int64
	released         bool
	planned          map[string]ReadRange
	planningComplete map[string]bool
	planErr          error
}

type Coordinator struct {
	metrics   *Metrics
	mu        sync.RWMutex
	sources   []Source
	entries   map[string]*streamEntry
	active    map[string]bool
	doneCh    map[string]chan struct{}
	used      int64
	budget    int64
	workers   int
	open      func(string) (sourceFile, error)
	wake      chan struct{}
	startOnce sync.Once
	cancel    context.CancelFunc
	done      chan struct{}
	planning  bool
	sealed    map[string]bool
	planReady chan struct{}
	planOnce  sync.Once
}

func NewCoordinator(sources []Source, options Options) *Coordinator {
	workers := options.Workers
	if workers <= 0 {
		workers = DynamicWorkers(len(sources))
	}
	budget := options.Budget
	if budget <= 0 {
		budget = DefaultBudget
	}
	open := options.Open
	if open == nil {
		open = func(path string) (sourceFile, error) { return os.Open(path) }
	}
	ordered := append([]Source(nil), sources...)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].Priority != ordered[j].Priority {
			return ordered[i].Priority < ordered[j].Priority
		}
		return ordered[i].Path < ordered[j].Path
	})
	entries := make(map[string]*streamEntry, len(ordered))
	for _, source := range ordered {
		entries[filepath.Clean(source.Path)] = &streamEntry{
			source:   source,
			channels: map[string]chan Batch{ConsumerUsage: make(chan Batch), ConsumerSession: make(chan Batch)},
			skipCh:   map[string]chan struct{}{ConsumerUsage: make(chan struct{}), ConsumerSession: make(chan struct{})},
			skipped:  map[string]bool{}, planned: map[string]ReadRange{}, planningComplete: map[string]bool{}, allSkipped: make(chan struct{}), finished: make(chan struct{}),
		}
	}
	return &Coordinator{metrics: options.Metrics, sources: ordered, entries: entries, active: map[string]bool{ConsumerUsage: true, ConsumerSession: true}, doneCh: map[string]chan struct{}{ConsumerUsage: make(chan struct{}), ConsumerSession: make(chan struct{})}, budget: budget, workers: workers, open: open, wake: make(chan struct{}, 1), done: make(chan struct{}), planning: options.RequirePlans, sealed: map[string]bool{}, planReady: make(chan struct{})}
}

func (c *Coordinator) Start(parent context.Context) {
	c.startOnce.Do(func() {
		ctx, cancel := context.WithCancel(parent)
		c.cancel = cancel
		jobs := make(chan admission)
		var wait sync.WaitGroup
		for worker := 0; worker < c.workers; worker++ {
			wait.Add(1)
			go func() {
				defer wait.Done()
				for item := range jobs {
					tail, err := c.read(ctx, item.source, item.read)
					c.finish(item.source.Path, tail, err)
				}
			}()
		}
		go c.schedule(ctx, jobs)
		go func() { wait.Wait(); close(c.done) }()
	})
}

type admission struct {
	source Source
	read   ReadRange
}

func (c *Coordinator) schedule(ctx context.Context, jobs chan<- admission) {
	defer close(jobs)
	if c.planning {
		select {
		case <-ctx.Done():
			return
		case <-c.planReady:
		}
	}
	for _, source := range c.sources {
		entry, read, need, planErr := c.admissionFor(source.Path)
		if planErr != nil {
			c.finish(source.Path, nil, planErr)
			continue
		}
		allSkipped := entry.allSkipped
		select {
		case <-allSkipped:
			c.finish(source.Path, nil, nil)
			continue
		default:
		}
		if !need || !source.Stable {
			c.finish(source.Path, nil, nil)
			continue
		}
		weight := (read.End - read.Start) * decodedWeightFactor
		if weight <= 0 {
			weight = 1
		}
		if err := c.acquire(ctx, source.Path, weight); err != nil {
			c.finish(source.Path, nil, err)
			return
		}
		select {
		case jobs <- admission{source: source, read: read}:
		case <-ctx.Done():
			c.finish(source.Path, nil, ctx.Err())
			return
		}
	}
}

// Plan records one domain's validated range before source body admission. Both
// domains must either Plan or Skip every discovered source before Seal releases
// the scheduler when RequirePlans is enabled.
func (c *Coordinator) Plan(path, consumer string, read ReadRange) error {
	if consumer != ConsumerUsage && consumer != ConsumerSession {
		return ErrPlanMismatch
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.planning {
		return nil
	}
	if c.sealed[consumer] {
		return ErrPlanIncomplete
	}
	entry := c.entries[filepath.Clean(path)]
	if entry == nil || entry.skipped[consumer] || read.Start < 0 || read.End < read.Start || read.End > entry.source.Size {
		return ErrPlanMismatch
	}
	if existing, found := entry.planned[consumer]; found && existing != read {
		return ErrPlanMismatch
	}
	entry.planned[consumer] = read
	entry.planningComplete[consumer] = true
	return nil
}

// Seal makes one domain's finite source plan immutable. A source read cannot
// begin until both domain seals are present.
func (c *Coordinator) Seal(consumer string) error {
	if consumer != ConsumerUsage && consumer != ConsumerSession {
		return ErrPlanMismatch
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.planning {
		return nil
	}
	if c.sealed[consumer] {
		return nil
	}
	for _, entry := range c.entries {
		if entry.planErr != nil || (!entry.skipped[consumer] && !entry.planningComplete[consumer]) {
			return ErrPlanIncomplete
		}
	}
	c.sealed[consumer] = true
	if c.sealed[ConsumerUsage] && c.sealed[ConsumerSession] {
		c.planOnce.Do(func() { close(c.planReady) })
	}
	return nil
}

// AbandonPlan removes one failed consumer's partial plan and seals that domain
// as skipped. The other consumer can still read and publish its independent
// result instead of inheriting the planning failure.
func (c *Coordinator) AbandonPlan(consumer string) error {
	if consumer != ConsumerUsage && consumer != ConsumerSession {
		return ErrPlanMismatch
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.planning || c.sealed[consumer] {
		return nil
	}
	for _, entry := range c.entries {
		delete(entry.planned, consumer)
		entry.planningComplete[consumer] = true
		if !entry.skipped[consumer] {
			entry.skipped[consumer] = true
			close(entry.skipCh[consumer])
		}
		if !entry.allSkippedClosed && entry.skipped[ConsumerUsage] && entry.skipped[ConsumerSession] {
			entry.allSkippedClosed = true
			close(entry.allSkipped)
		}
	}
	c.sealed[consumer] = true
	if c.sealed[ConsumerUsage] && c.sealed[ConsumerSession] {
		c.planOnce.Do(func() { close(c.planReady) })
	}
	return nil
}

func (c *Coordinator) admissionFor(path string) (*streamEntry, ReadRange, bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry := c.entries[filepath.Clean(path)]
	if entry == nil {
		return nil, ReadRange{}, false, ErrPlanMismatch
	}
	if entry.planErr != nil {
		return entry, ReadRange{}, false, entry.planErr
	}
	if !c.planning {
		return entry, ReadRange{Start: 0, End: entry.source.Size}, true, nil
	}
	if !c.sealed[ConsumerUsage] || !c.sealed[ConsumerSession] {
		return entry, ReadRange{}, false, ErrPlanIncomplete
	}
	need := false
	read := ReadRange{Start: entry.source.Size, End: 0}
	for _, consumer := range []string{ConsumerUsage, ConsumerSession} {
		if entry.skipped[consumer] {
			continue
		}
		planned, found := entry.planned[consumer]
		if !found {
			return entry, ReadRange{}, false, ErrPlanIncomplete
		}
		if !need || planned.Start < read.Start {
			read.Start = planned.Start
		}
		if planned.End > read.End {
			read.End = planned.End
		}
		need = true
	}
	return entry, read, need, nil
}

func (c *Coordinator) acquire(ctx context.Context, path string, weight int64) error {
	for {
		c.mu.Lock()
		entry := c.entries[filepath.Clean(path)]
		allowed, reserved := c.used+weight <= c.budget, weight
		if weight > c.budget {
			allowed, reserved = c.used == 0, c.budget
		}
		if allowed {
			entry.weight = reserved
			c.used += reserved
			c.mu.Unlock()
			return nil
		}
		c.mu.Unlock()
		var waitStart time.Time
		if c.metrics != nil {
			waitStart = time.Now()
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-c.wake:
		}
		if c.metrics != nil {
			c.metrics.budgetNS.Add(time.Since(waitStart).Nanoseconds())
		}
	}
}

func (c *Coordinator) Stream(ctx context.Context, path, consumer string) (Stream, bool, error) {
	c.mu.RLock()
	entry := c.entries[filepath.Clean(path)]
	active := c.active[consumer]
	planning := c.planning
	sealed := c.sealed[consumer]
	planned := entry != nil && entry.planningComplete[consumer]
	skipped := entry != nil && entry.skipped[consumer]
	planErr := error(nil)
	if entry != nil {
		planErr = entry.planErr
	}
	c.mu.RUnlock()
	if planErr != nil {
		return Stream{}, false, planErr
	}
	if planning && (!sealed || !planned || skipped) {
		return Stream{}, false, ErrPlanMismatch
	}
	if !planning {
		c.Start(ctx)
	}
	if entry == nil || !active || !entry.source.Stable {
		return Stream{}, false, nil
	}
	return Stream{Source: entry.source, Batches: entry.channels[consumer], entry: entry}, true, nil
}

func (c *Coordinator) Skip(path, consumer string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry := c.entries[filepath.Clean(path)]
	if entry == nil || entry.skipped[consumer] {
		return
	}
	if c.planning && entry.planningComplete[consumer] && !entry.skipped[consumer] {
		entry.planErr = ErrPlanMismatch
		return
	}
	entry.skipped[consumer] = true
	if c.planning {
		entry.planningComplete[consumer] = true
	}
	close(entry.skipCh[consumer])
	if !entry.allSkippedClosed && entry.skipped[ConsumerUsage] && entry.skipped[ConsumerSession] {
		entry.allSkippedClosed = true
		close(entry.allSkipped)
	}
}

func (c *Coordinator) Snapshot(ctx context.Context, path, consumer string) (Snapshot, bool, error) {
	stream, shared, err := c.Stream(ctx, path, consumer)
	if err != nil || !shared {
		return Snapshot{}, shared, err
	}
	records := make([]Record, 0)
	for batch := range stream.Batches {
		records = append(records, batch.Records...)
	}
	tail, err := stream.Wait(ctx)
	return Snapshot{Source: stream.Source, Records: records, Tail: tail}, true, err
}

type Snapshot struct {
	Source  Source
	Records []Record
	Tail    []byte
}

func (c *Coordinator) read(ctx context.Context, source Source, readRange ReadRange) ([]byte, error) {
	file, err := c.open(source.Path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	if readRange.Start < 0 || readRange.End < readRange.Start || readRange.End > source.Size {
		return nil, ErrPlanMismatch
	}
	readerAt, ok := file.(io.ReaderAt)
	if !ok {
		return nil, ErrPlanMismatch
	}
	input := io.Reader(io.NewSectionReader(readerAt, readRange.Start, readRange.End-readRange.Start))
	if c.metrics != nil {
		start := time.Now()
		active := c.metrics.active.Add(1)
		for peak := c.metrics.peak.Load(); active > peak && !c.metrics.peak.CompareAndSwap(peak, active); peak = c.metrics.peak.Load() {
		}
		defer func() { c.metrics.sourceNS.Add(time.Since(start).Nanoseconds()); c.metrics.active.Add(-1) }()
		input = measuredInput{Reader: input, metrics: c.metrics}
	}
	// Large transcripts otherwise issue tens of thousands of small reads.
	// Bound each active reader to 1 MiB; small files use only their own size.
	reader := bufio.NewReaderSize(input, int(min(int64(1<<20), max(int64(4096), readRange.End-readRange.Start))))
	var tail []byte
	offset := readRange.Start
	var sequence uint64
	batch := Batch{Records: make([]Record, 0, 256)}
	batchBytes := 0
	flush := func() error {
		if len(batch.Records) == 0 {
			return nil
		}
		if err := c.publish(ctx, batch, source.Path); err != nil {
			return err
		}
		batch = Batch{Records: make([]Record, 0, 256)}
		batchBytes = 0
		return nil
	}
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		c.mu.RLock()
		entry := c.entries[filepath.Clean(source.Path)]
		allSkipped := entry.allSkipped
		c.mu.RUnlock()
		select {
		case <-allSkipped:
			return nil, nil
		default:
		}
		line, readErr := readRecordLine(reader)
		if len(line) > 0 {
			if line[len(line)-1] != '\n' {
				tail = append([]byte(nil), line...)
			} else {
				sequence++
				end := offset + int64(len(line))
				var value map[string]any
				var decodeStart time.Time
				if c.metrics != nil {
					decodeStart = time.Now()
				}
				decodeErr := json.Unmarshal(line[:len(line)-1], &value)
				if c.metrics != nil {
					c.metrics.decodeNS.Add(time.Since(decodeStart).Nanoseconds())
				}
				batch.Records = append(batch.Records, Record{Offset: offset, End: end, Sequence: sequence, Kind: classify(value), Value: value, Malformed: decodeErr != nil})
				batchBytes += len(line)
				if len(batch.Records) >= 256 || batchBytes >= 1<<20 {
					if err := flush(); err != nil {
						return nil, err
					}
				}
				offset = end
			}
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return nil, readErr
		}
	}
	if err := flush(); err != nil {
		return nil, err
	}
	latest, err := file.Stat()
	if err != nil {
		return nil, err
	}
	identity, _, stable := fileGeneration(latest)
	unchanged := latest.Size() == source.Size && SameGeneration(source, latest)
	appendOnlyGrowth := latest.Size() > source.Size && source.Stable && stable && identity == source.Identity && latest.Size() >= readRange.End
	if !unchanged && !appendOnlyGrowth {
		return nil, ErrSourceChanged
	}
	return tail, nil
}

func (c *Coordinator) publish(ctx context.Context, batch Batch, path string) error {
	c.mu.RLock()
	entry := c.entries[filepath.Clean(path)]
	active := make(map[string]bool, len(c.active))
	for consumer := range c.active {
		active[consumer] = true
	}
	doneChannels := c.doneCh
	c.mu.RUnlock()
	for _, consumer := range []string{ConsumerUsage, ConsumerSession} {
		if !active[consumer] {
			continue
		}
		var sendStart time.Time
		if c.metrics != nil {
			sendStart = time.Now()
		}
		select {
		case entry.channels[consumer] <- batch:
		case <-entry.skipCh[consumer]:
		case <-doneChannels[consumer]:
		case <-ctx.Done():
			return ctx.Err()
		}
		if c.metrics != nil {
			elapsed := time.Since(sendStart).Nanoseconds()
			if consumer == ConsumerUsage {
				c.metrics.usageNS.Add(elapsed)
			} else {
				c.metrics.sessionNS.Add(elapsed)
			}
		}
	}
	return nil
}

func (c *Coordinator) finish(path string, tail []byte, err error) {
	c.mu.Lock()
	entry := c.entries[filepath.Clean(path)]
	if entry != nil {
		entry.tail, entry.err = tail, err
		for _, channel := range entry.channels {
			close(channel)
		}
		close(entry.finished)
		c.used -= entry.weight
		entry.weight = 0
	}
	c.mu.Unlock()
	select {
	case c.wake <- struct{}{}:
	default:
	}
}

func (c *Coordinator) ConsumerDone(consumer string) {
	c.mu.Lock()
	if c.active[consumer] {
		delete(c.active, consumer)
		close(c.doneCh[consumer])
	}
	last := len(c.active) == 0
	cancel := c.cancel
	c.mu.Unlock()
	if last && cancel != nil {
		cancel()
	}
}

func classify(value map[string]any) EventKind {
	if value == nil {
		return EventOther
	}
	typeName, _ := value["type"].(string)
	switch typeName {
	case "session_meta", "turn_context":
		return EventContext
	case "visible_user_prompt", "visible_assistant_final", "user", "assistant":
		return EventDocument
	case "event_msg":
		return EventUsage
	case "response_item":
		return EventTool
	default:
		return EventOther
	}
}

func SameGeneration(source Source, info os.FileInfo) bool {
	identity, changedAt, stable := fileGeneration(info)
	return source.Stable && stable && identity == source.Identity && info.Size() == source.Size && info.ModTime().UnixNano() == source.ModifiedAt && changedAt == source.ChangedAt
}

func Validate(source Source) error {
	info, err := os.Stat(source.Path)
	if err != nil {
		return err
	}
	if !SameGeneration(source, info) {
		return ErrSourceChanged
	}
	return nil
}

// ValidateCapturedRange permits same-identity growth after discovery while
// requiring the complete captured byte interval to remain present.
func ValidateCapturedRange(source Source, end int64) error {
	info, err := os.Stat(source.Path)
	if err != nil {
		return err
	}
	identity, _, stable := fileGeneration(info)
	if !source.Stable || !stable || identity != source.Identity || end < 0 || end > source.Size || info.Size() < end {
		return ErrSourceChanged
	}
	return nil
}

func fileGeneration(info os.FileInfo) (string, int64, bool) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return info.Name(), 0, false
	}
	return fmt.Sprintf("%d:%d", stat.Dev, stat.Ino), statChangeTime(stat), true
}

func (c *Coordinator) Close() {
	if c.cancel != nil {
		c.cancel()
		<-c.done
	}
}
