// Package scanruntime owns one finite, detached scan round per state root.
//
// The public clients in cmd/agentdeck use it instead of creating independent
// usage and session scanners.  Ingest remains an implementation detail of a
// round: it can share a source body between the two domain consumers, but it
// cannot decide client lifetime, election, or what a foreground request waits
// for.
package scanruntime

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/kitdine/agent-deck/internal/desktop"
	"github.com/kitdine/agent-deck/internal/ingest"
	"github.com/kitdine/agent-deck/internal/platform"
	"github.com/kitdine/agent-deck/internal/session"
	"github.com/kitdine/agent-deck/internal/store"
	"github.com/kitdine/agent-deck/internal/usage"
)

const (
	// ProtocolVersion prevents a client from treating an incompatible worker as
	// an accepted scan.  It is intentionally small and private to this runtime.
	ProtocolVersion = 2

	defaultStartupTimeout = 5 * time.Second
	defaultIdleTimeout    = time.Second
	runtimeParentLocator  = ".scan-runtime-parent"
)

// Scope selects the terminal outcome a caller waits for.  It never changes the
// round's obligation to scan both domains.
type Scope string

const (
	ScopeBoth    Scope = "both"
	ScopeUsage   Scope = "usage"
	ScopeSession Scope = "session"
)

var ErrInvalidScope = errors.New("invalid scan scope")

func (s Scope) valid() bool {
	return s == ScopeBoth || s == ScopeUsage || s == ScopeSession
}

// UsageResult and SessionResult keep each domain's durable result independent.
// A failure in one must not erase the committed outcome of the other.
type UsageResult struct {
	State      string         `json:"state"`
	Changes    map[string]int `json:"changes,omitempty"`
	ErrorCode  string         `json:"error_code,omitempty"`
	Error      string         `json:"-"`
	DurationMS int64          `json:"duration_ms"`
}

type SessionResult struct {
	State      string             `json:"state"`
	Scan       session.ScanResult `json:"scan,omitempty"`
	ErrorCode  string             `json:"error_code,omitempty"`
	Error      string             `json:"-"`
	DurationMS int64              `json:"duration_ms"`
}

// StageProfile records the work actually performed by this round.  It is a
// compact diagnostic record for the Task 1 baseline; it is not a user-facing
// progress stream.
type StageProfile struct {
	DiscoveryMS        int64 `json:"discovery_ms"`
	UsageMS            int64 `json:"usage_ms"`
	SessionMS          int64 `json:"session_ms"`
	DerivedCacheMS     int64 `json:"derived_cache_ms"`
	TotalMS            int64 `json:"total_ms"`
	WorkerCPUTimeMS    int64 `json:"worker_cpu_time_ms"`
	WorkerPeakRSSBytes int64 `json:"worker_peak_rss_bytes"`
}

type processResources struct {
	cpuTime time.Duration
	peakRSS int64
}

// DomainProgress contains only aggregate committed work. It is safe to expose
// to CLI and App subscribers because it never contains source paths or content.
type DomainProgress struct {
	State     string `json:"state"`
	Committed int    `json:"committed"`
	Total     int    `json:"total"`
	Skipped   int    `json:"skipped"`
}

// Progress is the versioned, bounded subscriber view of one scan round.
type Progress struct {
	Sequence uint64         `json:"sequence"`
	Stage    string         `json:"stage"`
	Usage    DomainProgress `json:"usage"`
	Session  DomainProgress `json:"session"`
}

// Result is the finite observation returned by a round.
type Result struct {
	RoundID     string        `json:"round_id"`
	StartedAt   time.Time     `json:"started_at"`
	CompletedAt time.Time     `json:"completed_at"`
	Usage       UsageResult   `json:"usage"`
	Session     SessionResult `json:"session"`
	Stages      StageProfile  `json:"stages"`
}

// DomainError reports the domain selected by a legacy adapter or a scoped
// request without pretending that the other domain has the same outcome.
type DomainError struct {
	Domain string
	Code   string
	Cause  string
}

func (e *DomainError) Error() string {
	if e.Cause == "" {
		return e.Domain + " scan failed"
	}
	return e.Domain + " scan failed: " + e.Cause
}

// ErrorFor translates the scope's terminal result back into the legacy command
// contract.  An unscoped caller observes both failures; a scoped caller does
// not retroactively fail when the other domain finishes later with an error.
func (r Result) ErrorFor(scope Scope) error {
	if !scope.valid() {
		return ErrInvalidScope
	}
	switch scope {
	case ScopeUsage:
		if r.Usage.State != "completed" {
			return &DomainError{Domain: "usage", Code: r.Usage.ErrorCode}
		}
	case ScopeSession:
		if r.Session.State != "completed" {
			return &DomainError{Domain: "session", Code: r.Session.ErrorCode}
		}
	default:
		if r.Usage.State != "completed" {
			return &DomainError{Domain: "usage", Code: r.Usage.ErrorCode}
		}
		if r.Session.State != "completed" {
			return &DomainError{Domain: "session", Code: r.Session.ErrorCode}
		}
	}
	return nil
}

// Client requests a round from an existing worker or elects one by launching
// the hidden scan-worker command.  Launch is injectable for subprocess tests.
type Client struct {
	StateRoot string
	Home      string
	// RequestID is stable across a client retry. An empty value asks the client
	// to allocate a new local request identity.
	RequestID  string
	Executable string
	Launch     func(context.Context, string) error

	// ForceLocal is intended only for tests and controlled in-process callers.
	// The normal CLI path always uses the detached worker when it is executable.
	ForceLocal bool
	// Now is carried only by ForceLocal test callers so a test's scan and
	// snapshot observe one business clock. Detached workers use their own clock.
	Now func() time.Time
}

type wireRequest struct {
	Protocol int    `json:"protocol"`
	Request  string `json:"request"`
	StateID  string `json:"state_id"`
	Home     string `json:"home"`
	Scope    Scope  `json:"scope"`
	Events   bool   `json:"events,omitempty"`
	CacheNow string `json:"cache_now,omitempty"`
}

type wireResponse struct {
	Type     string    `json:"type"`
	WorkerID string    `json:"worker_id,omitempty"`
	RoundID  string    `json:"round_id,omitempty"`
	Result   *Result   `json:"result,omitempty"`
	Progress *Progress `json:"progress,omitempty"`
	Error    string    `json:"error,omitempty"`
}

// Request joins a covered running round or starts one finite follow-up round.
// Client cancellation only stops this wait; it never cancels the worker's
// globally accepted scan.
func (c Client) Request(ctx context.Context, scope Scope) (Result, error) {
	return c.request(ctx, scope, nil)
}

// RequestWithProgress delivers coalesced aggregate progress while preserving
// Request's final result and subscriber-only cancellation semantics.
func (c Client) RequestWithProgress(ctx context.Context, scope Scope, onProgress func(Progress)) (Result, error) {
	return c.request(ctx, scope, onProgress)
}

func (c Client) request(ctx context.Context, scope Scope, onProgress func(Progress)) (Result, error) {
	if !scope.valid() {
		return Result{}, ErrInvalidScope
	}
	stateRoot, stateID, err := prepareStateRoot(c.StateRoot)
	if err != nil {
		return Result{}, err
	}
	if c.ForceLocal {
		return localRequest(ctx, stateRoot, c.Home, scope, c.Now, onProgress)
	}
	endpoint, err := socketPath(stateRoot, stateID)
	if err != nil {
		return Result{}, err
	}
	requestID := c.RequestID
	if requestID == "" {
		requestID = newID()
	}
	request := wireRequest{Protocol: ProtocolVersion, Request: requestID, StateID: stateID, Home: c.Home, Scope: scope, Events: onProgress != nil}
	if c.Executable != "" && c.Now != nil {
		request.CacheNow = c.Now().UTC().Format(time.RFC3339Nano)
	}
	conn, err := c.connectOrLaunch(ctx, endpoint, stateRoot)
	if err != nil {
		return Result{}, err
	}
	defer conn.Close()
	if err = json.NewEncoder(conn).Encode(request); err != nil {
		return Result{}, err
	}
	decoder := json.NewDecoder(io.LimitReader(conn, 8<<20))
	first, err := decodeResponse(ctx, conn, decoder)
	if err != nil {
		return Result{}, err
	}
	if first.Type != "accepted" || first.WorkerID == "" {
		if first.Error != "" {
			return Result{}, errors.New(first.Error)
		}
		return Result{}, errors.New("scan worker did not acknowledge request")
	}
	for {
		response, decodeErr := decodeResponse(ctx, conn, decoder)
		if decodeErr != nil {
			return Result{}, decodeErr
		}
		if response.WorkerID != first.WorkerID {
			return Result{}, errors.New("scan worker response changed identity")
		}
		switch response.Type {
		case "progress":
			if response.Progress == nil || onProgress == nil {
				return Result{}, errors.New("scan worker returned an invalid progress response")
			}
			onProgress(*response.Progress)
		case "terminal":
			if response.Result == nil {
				return Result{}, errors.New("scan worker returned an invalid terminal response")
			}
			return *response.Result, nil
		default:
			if response.Error != "" {
				return Result{}, errors.New(response.Error)
			}
			return Result{}, errors.New("scan worker returned an unknown response")
		}
	}
}

func (c Client) connectOrLaunch(ctx context.Context, endpoint, stateRoot string) (net.Conn, error) {
	if conn, err := dial(endpoint); err == nil {
		return conn, nil
	}
	if c.Launch != nil {
		if err := c.Launch(ctx, stateRoot); err != nil {
			return nil, err
		}
	} else if err := launchWorker(ctx, c.Executable, stateRoot); err != nil {
		return nil, err
	}
	startupCtx, cancel := context.WithTimeout(ctx, defaultStartupTimeout)
	defer cancel()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		if conn, err := dial(endpoint); err == nil {
			return conn, nil
		}
		select {
		case <-startupCtx.Done():
			return nil, fmt.Errorf("scan worker did not become ready: %w", startupCtx.Err())
		case <-ticker.C:
		}
	}
}

func dial(endpoint string) (net.Conn, error) {
	return net.DialTimeout("unix", endpoint, 150*time.Millisecond)
}

func launchWorker(ctx context.Context, executable, stateRoot string) error {
	if executable == "" {
		var err error
		executable, err = os.Executable()
		if err != nil {
			return err
		}
	}
	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(context.Background(), executable, "--state-dir", stateRoot, "scan-worker")
	cmd.Stdin = devNull
	cmd.Stdout = devNull
	cmd.Stderr = devNull
	detachWorkerProcess(cmd)
	if err = cmd.Start(); err != nil {
		_ = devNull.Close()
		return err
	}
	_ = devNull.Close()
	// Reap only the detached helper process.  This goroutine does not own the
	// round and cannot cancel it when the client context ends.
	go func() { _ = cmd.Wait() }()
	return nil
}

func decodeResponse(ctx context.Context, conn net.Conn, decoder *json.Decoder) (wireResponse, error) {
	type decoded struct {
		response wireResponse
		err      error
	}
	result := make(chan decoded, 1)
	go func() {
		var response wireResponse
		result <- decoded{response: response, err: decoder.Decode(&response)}
	}()
	select {
	case <-ctx.Done():
		_ = conn.Close()
		return wireResponse{}, ctx.Err()
	case value := <-result:
		return value.response, value.err
	}
}

// Serve runs the hidden detached worker.  It holds the scan lock for its whole
// lifetime, which makes socket cleanup and election authority inseparable.
func Serve(ctx context.Context, stateRoot string) error {
	stateRoot, stateID, err := prepareStateRoot(stateRoot)
	if err != nil {
		return err
	}
	lock, err := store.AcquireScanLock(ctx, stateRoot, 0)
	if err != nil {
		return err
	}
	defer lock.Release()
	journal, err := openReceiptJournal(stateRoot)
	if err != nil {
		return err
	}
	endpoint, err := socketPath(stateRoot, stateID)
	if err != nil {
		return err
	}
	if err = removeSocket(endpoint); err != nil {
		return err
	}
	nonce := newID()
	metadataPath := filepath.Join(filepath.Dir(endpoint), "owner.json")
	if err = writeOwnerMetadata(metadataPath, stateID, nonce); err != nil {
		return err
	}
	defer os.Remove(metadataPath)
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: endpoint, Net: "unix"})
	if err != nil {
		return err
	}
	defer func() {
		_ = listener.Close()
		_ = os.Remove(endpoint)
	}()
	if err = os.Chmod(endpoint, 0o600); err != nil {
		return err
	}
	worker := &server{
		ctx:          ctx,
		stateRoot:    stateRoot,
		stateID:      stateID,
		nonce:        nonce,
		journal:      journal,
		rounds:       map[string]*round{},
		startedAt:    time.Now(),
		lastActivity: time.Now(),
	}
	if err = worker.recoverReceipts(); err != nil {
		return err
	}
	for {
		if err = listener.SetDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
			return err
		}
		conn, acceptErr := listener.AcceptUnix()
		if acceptErr != nil {
			if errors.Is(acceptErr, net.ErrClosed) {
				return nil
			}
			if netErr, ok := acceptErr.(net.Error); ok && netErr.Timeout() {
				if ctx.Err() != nil {
					return nil
				}
				if worker.idle() {
					return nil
				}
				continue
			}
			return acceptErr
		}
		go worker.handle(conn)
	}
}

type server struct {
	ctx       context.Context
	stateRoot string
	stateID   string
	nonce     string
	run       roundRunner
	journal   *receiptJournal

	mu              sync.Mutex
	round           *round
	pending         *round
	rounds          map[string]*round
	nextObservation uint64
	startedAt       time.Time
	lastActivity    time.Time
}

func (s *server) handle(conn *net.UnixConn) {
	defer conn.Close()
	decoder := json.NewDecoder(io.LimitReader(conn, 64<<10))
	var request wireRequest
	if err := decoder.Decode(&request); err != nil {
		return
	}
	encoder := json.NewEncoder(conn)
	if request.Protocol != ProtocolVersion || request.Request == "" || request.StateID != s.stateID || !request.Scope.valid() || request.Home == "" {
		_ = encoder.Encode(wireResponse{Type: "rejected", Error: "incompatible scan worker request"})
		return
	}
	round, terminal, err := s.accept(request)
	if err != nil {
		_ = encoder.Encode(wireResponse{Type: "rejected", Error: err.Error()})
		return
	}
	if err := encoder.Encode(wireResponse{Type: "accepted", WorkerID: s.nonce, RoundID: round.id}); err != nil {
		return
	}
	if terminal != nil {
		_ = encoder.Encode(wireResponse{Type: "terminal", WorkerID: s.nonce, RoundID: round.id, Result: terminal})
		return
	}
	var result Result
	if request.Events {
		result, err = round.waitWithProgress(s.ctx, request.Scope, func(progress Progress) error {
			return encoder.Encode(wireResponse{Type: "progress", WorkerID: s.nonce, RoundID: round.id, Progress: &progress})
		})
	} else {
		result, err = round.wait(s.ctx, request.Scope)
	}
	if err != nil {
		return
	}
	if err = s.terminalReceipt(request.Request, result); err != nil {
		return
	}
	_ = encoder.Encode(wireResponse{Type: "terminal", WorkerID: s.nonce, RoundID: round.id, Result: &result})
}

func (s *server) startOrJoin(home string) (*round, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	round, start, err := s.selectRoundLocked(home, ScopeBoth)
	if err != nil {
		return nil, err
	}
	if start {
		s.launchRoundLocked(round)
	}
	s.lastActivity = time.Now()
	return round, nil
}

// accept persists a request receipt before the caller sees accepted. A replay
// returns the original terminal result or attaches to the same recovered round.
func (s *server) accept(request wireRequest) (*round, *Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.journal != nil {
		if receipt, found := s.journal.get(request.Request); found {
			if receipt.StateID != request.StateID || receipt.Home != request.Home || receipt.Scope != request.Scope {
				return nil, nil, errors.New("scan request id conflicts with a different requirement")
			}
			if receipt.State == receiptTerminal && receipt.Result != nil {
				result := *receipt.Result
				return newRoundWithID(receipt.Home, receipt.RoundID, receipt.Observation), &result, nil
			}
			if receipt.State != receiptAccepted {
				return nil, nil, errors.New("scan request receipt expired; re-evaluate required")
			}
			if round := s.rounds[receipt.RoundID]; round != nil {
				return round, nil, nil
			}
			return nil, nil, errors.New("accepted scan request has no recoverable round")
		}
	}
	round, start, err := s.selectRoundLocked(request.Home, request.Scope)
	if err != nil {
		return nil, nil, err
	}
	if request.CacheNow != "" {
		cacheNow, parseErr := time.Parse(time.RFC3339Nano, request.CacheNow)
		if parseErr != nil {
			s.rollbackSelectedRoundLocked(round, start)
			return nil, nil, errors.New("invalid scan worker cache clock")
		}
		if !start && round.cacheNow == nil {
			return nil, nil, errors.New("incompatible scan worker cache clock")
		}
		if round.cacheNow != nil && !round.cacheNow().Equal(cacheNow) {
			return nil, nil, errors.New("incompatible scan worker cache clock")
		}
		round.cacheNow = func() time.Time { return cacheNow }
	}
	if s.journal != nil {
		receipt := scanReceipt{
			ID: request.Request, StateID: request.StateID, Home: request.Home, Scope: request.Scope,
			RoundID: round.id, Observation: round.observation,
		}
		if err = s.journal.accept(receipt); err != nil {
			if receiptWasInstalled(err) {
				if start {
					s.launchRoundLocked(round)
				}
				s.lastActivity = time.Now()
			} else {
				s.rollbackSelectedRoundLocked(round, start)
			}
			return nil, nil, err
		}
	}
	if start {
		s.launchRoundLocked(round)
	}
	s.lastActivity = time.Now()
	return round, nil, nil
}

func (s *server) selectRoundLocked(home string, scope Scope) (*round, bool, error) {
	if s.round == nil {
		round := s.newRoundLocked(home, nil)
		s.round = round
		return round, true, nil
	}
	if !s.round.completed() && s.round.home != home {
		return nil, false, errors.New("incompatible scan worker configuration")
	}
	if !s.round.observedInventory() && s.round.covers(scope) {
		return s.round, false, nil
	}
	if s.pending != nil {
		if s.pending.home != home {
			return nil, false, errors.New("incompatible scan worker configuration")
		}
		return s.pending, false, nil
	}
	if s.round.completed() {
		round := s.newRoundLocked(home, nil)
		s.round = round
		return round, true, nil
	}
	followUp := s.newRoundLocked(home, s.round)
	s.pending = followUp
	return followUp, true, nil
}

func (s *server) newRoundLocked(home string, predecessor *round) *round {
	s.nextObservation++
	candidate := newRoundWithID(home, newID(), s.nextObservation)
	candidate.predecessor = predecessor
	if s.rounds == nil {
		s.rounds = map[string]*round{}
	}
	s.rounds[candidate.id] = candidate
	return candidate
}

func (s *server) rollbackSelectedRoundLocked(round *round, started bool) {
	if !started {
		return
	}
	delete(s.rounds, round.id)
	if s.pending == round {
		s.pending = nil
		return
	}
	if s.round == round {
		s.round = nil
	}
}

func (s *server) launchRoundLocked(round *round) {
	round.onTerminal = func(result Result) error { return s.completeRound(round, result) }
	go func() {
		if round.predecessor != nil {
			<-round.predecessor.done
		}
		round.executeWith(s.ctx, s.stateRoot, true, s.run)
	}()
}

func (s *server) completeRound(round *round, result Result) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var journalErr error
	if s.journal != nil {
		journalErr = s.journal.terminal(round.id, result)
	}
	delete(s.rounds, round.id)
	if s.round == round && s.pending != nil {
		next := s.pending
		s.pending = nil
		s.round = next
	}
	s.lastActivity = time.Now()
	return journalErr
}

func (s *server) terminalReceipt(id string, result Result) error {
	if s.journal == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.journal.terminalReceipt(id, result)
}

func (s *server) recoverReceipts() error {
	if s.journal == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.rounds == nil {
		s.rounds = map[string]*round{}
	}
	if err := s.journal.prune(time.Now().UTC()); err != nil {
		return err
	}
	var previous *round
	seen := map[string]bool{}
	for _, receipt := range s.journal.recoverable() {
		if receipt.StateID != s.stateID {
			return errors.New("scan receipt journal belongs to another state root")
		}
		if seen[receipt.RoundID] {
			continue
		}
		seen[receipt.RoundID] = true
		if previous != nil && receipt.Home != previous.home {
			return errors.New("incompatible unfinished scan receipt configurations")
		}
		if previous != nil && s.pending != nil {
			return errors.New("too many unfinished scan receipt rounds")
		}
		round := newRoundWithID(receipt.Home, receipt.RoundID, receipt.Observation)
		round.predecessor = previous
		s.rounds[round.id] = round
		if previous == nil {
			s.round = round
		} else {
			s.pending = round
		}
		if receipt.Observation > s.nextObservation {
			s.nextObservation = receipt.Observation
		}
		previous = round
	}
	if s.round != nil {
		s.launchRoundLocked(s.round)
	}
	if s.pending != nil {
		s.launchRoundLocked(s.pending)
	}
	return nil
}

func (s *server) idle() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if (s.round != nil && !s.round.completed()) || s.pending != nil {
		return false
	}
	return time.Since(s.lastActivity) >= defaultIdleTimeout
}

type round struct {
	id          string
	home        string
	observation uint64
	predecessor *round
	onTerminal  func(Result) error

	done              chan struct{}
	usageDone         chan struct{}
	sessionDone       chan struct{}
	once              sync.Once
	usageOnce         sync.Once
	sessionOnce       sync.Once
	mu                sync.RWMutex
	result            Result
	progress          Progress
	progressSeq       uint64
	terminalErr       error
	cacheNow          func() time.Time
	openCore          func(context.Context, string) (*store.Store, error)
	openSessions      func(context.Context, string) (*store.Store, error)
	afterDiscovery    func([]ingest.Source)
	inventoryObserved chan struct{}
	inventoryOnce     sync.Once
}

type roundRunner func(context.Context, string, string, string, bool) Result

func newRound(home string) *round {
	return newRoundWithID(home, newID(), 0)
}

func newRoundWithID(home, id string, observation uint64) *round {
	return &round{
		id:                id,
		home:              home,
		observation:       observation,
		done:              make(chan struct{}),
		usageDone:         make(chan struct{}),
		sessionDone:       make(chan struct{}),
		inventoryObserved: make(chan struct{}),
		progress: Progress{
			Stage:   "waiting",
			Usage:   DomainProgress{State: "pending"},
			Session: DomainProgress{State: "pending"},
		},
	}
}

func (r *round) observedInventory() bool {
	select {
	case <-r.inventoryObserved:
		return true
	default:
		return false
	}
}

func (r *round) completed() bool {
	select {
	case <-r.done:
		return true
	default:
		return false
	}
}

func (r *round) execute(ctx context.Context, stateRoot string, lockHeld bool) {
	r.executeWith(ctx, stateRoot, lockHeld, nil)
}

func (r *round) executeWith(ctx context.Context, stateRoot string, lockHeld bool, run roundRunner) {
	r.once.Do(func() {
		resourcesBefore := currentProcessResources()
		if run == nil {
			r.executeProduction(ctx, stateRoot, lockHeld)
		} else {
			r.setResult(run(ctx, stateRoot, r.home, r.id, lockHeld))
		}
		r.setProcessResources(resourcesBefore, currentProcessResources())
		r.complete()
	})
}

func (r *round) executeProduction(ctx context.Context, stateRoot string, lockHeld bool) {
	started := time.Now().UTC()
	r.setResult(Result{
		RoundID: r.id, StartedAt: started,
		Usage:   UsageResult{State: "pending"},
		Session: SessionResult{State: "pending"},
	})
	r.setProgressStage("checking")
	if !lockHeld {
		lock, err := store.AcquireScanLock(ctx, stateRoot, 5*time.Second)
		if err != nil {
			r.setUsage(failedUsage(err, 0))
			r.setSession(failedSession(err, 0))
			return
		}
		defer lock.Release()
	}
	discoverStarted := time.Now()
	sources, discoverErr := ingest.Discover(r.home)
	r.setDiscovery(time.Since(discoverStarted).Milliseconds())
	r.inventoryOnce.Do(func() { close(r.inventoryObserved) })
	if discoverErr != nil {
		r.setUsage(failedUsage(discoverErr, 0))
		r.setSession(failedSession(discoverErr, 0))
		return
	}
	if r.afterDiscovery != nil {
		r.afterDiscovery(sources)
	}

	openCore := r.openCore
	if openCore == nil {
		openCore = store.Open
	}
	openSessions := r.openSessions
	if openSessions == nil {
		openSessions = store.OpenSessions
	}
	core, coreErr := openCore(ctx, stateRoot)
	if coreErr == nil {
		defer core.Close()
	}
	sessions, sessionsErr := openSessions(ctx, stateRoot)
	if sessionsErr == nil {
		defer sessions.Close()
	}

	coordinator := ingest.NewCoordinator(sources, ingest.Options{RequirePlans: true})
	defer coordinator.Close()
	var usageService *usage.Service
	var usageInventory usage.Inventory
	usagePlanErr := coreErr
	if coreErr == nil {
		usageService = usage.New(core, r.home)
		usageService.PreparedSources = sources
		usageService.Coordinator = coordinator
		usageInventory, usagePlanErr = usageService.PlanForCoordinator(ctx, coordinator)
	} else {
		usagePlanErr = sealSkippedSources(coordinator, sources, ingest.ConsumerUsage)
		if usagePlanErr == nil {
			usagePlanErr = coreErr
		}
	}
	sessionPlanErr := sessionsErr
	if sessionsErr == nil {
		sessionPlanErr = session.PlanForCoordinator(ctx, sessions.DB, r.home, sources, coordinator)
	} else {
		sessionPlanErr = sealSkippedSources(coordinator, sources, ingest.ConsumerSession)
		if sessionPlanErr == nil {
			sessionPlanErr = sessionsErr
		}
	}
	if usagePlanErr != nil && sessionPlanErr != nil {
		r.setUsage(failedUsage(usagePlanErr, 0))
		r.setSession(failedSession(sessionPlanErr, 0))
		return
	}
	if usagePlanErr != nil {
		if err := coordinator.AbandonPlan(ingest.ConsumerUsage); err != nil {
			r.setUsage(failedUsage(usagePlanErr, 0))
			r.setSession(failedSession(err, 0))
			return
		}
		r.setUsage(failedUsage(usagePlanErr, 0))
		coordinator.ConsumerDone(ingest.ConsumerUsage)
	}
	if sessionPlanErr != nil {
		if err := coordinator.AbandonPlan(ingest.ConsumerSession); err != nil {
			r.setUsage(failedUsage(err, 0))
			r.setSession(failedSession(sessionPlanErr, 0))
			return
		}
		r.setSession(failedSession(sessionPlanErr, 0))
		coordinator.ConsumerDone(ingest.ConsumerSession)
	}
	r.setProgressStage("importing")
	coordinator.Start(ctx)
	var wait sync.WaitGroup
	wait.Add(1)
	if usagePlanErr == nil {
		usageService.Progress = usageRoundProgress{round: r}
		wait.Add(1)
		go func() {
			defer wait.Done()
			defer coordinator.ConsumerDone(ingest.ConsumerUsage)
			started := time.Now()
			changes, err := usageService.ScanInventory(ctx, usageInventory)
			duration := time.Since(started).Milliseconds()
			if err != nil {
				r.setUsage(failedUsage(err, duration))
				return
			}
			r.setUsage(UsageResult{State: "completed", Changes: changes, DurationMS: duration})
		}()
	}
	go func() {
		defer wait.Done()
		<-r.usageDone
		<-r.sessionDone
		r.mu.Lock()
		usageResult := r.result.Usage
		r.mu.Unlock()
		if usageResult.State != "completed" {
			return
		}
		// Cache publication is best effort and remains independent from the
		// durable usage outcome. Scope-specific callers have already observed
		// their terminal domain result before this derived work completes.
		started := time.Now()
		r.setProgressStage("statistics")
		_ = (desktop.Service{StateRoot: stateRoot, Home: r.home, Now: r.cacheNow}).PublishDerivedSnapshotCache(ctx, desktop.WireVersion)
		r.setDerivedCache(time.Since(started).Milliseconds())
	}()
	if sessionPlanErr == nil {
		wait.Add(1)
		go func() {
			defer wait.Done()
			defer coordinator.ConsumerDone(ingest.ConsumerSession)
			started := time.Now()
			scan, err := session.ScanWithOptions(ctx, sessions.DB, r.home, session.ScanOptions{PreparedSources: sources, Coordinator: coordinator, Progress: sessionRoundProgress{round: r}})
			if err != nil {
				r.setSession(failedSession(err, time.Since(started).Milliseconds()))
				return
			}
			epoch, err := sessions.SessionIndexEpoch(ctx)
			if err != nil {
				r.setSession(failedSession(err, time.Since(started).Milliseconds()))
				return
			}
			if coreErr == nil {
				checkpoint := fmt.Sprintf("v1:%d:%s", epoch, ingest.FingerprintSources(sources))
				// This core setting only enables watch fast-skip. Session rows,
				// cursors and completion markers are committed in the independent
				// session store, so publication failure must not rewrite that
				// successful domain outcome.
				_ = core.SetSetting(ctx, "watch.fingerprint.session", checkpoint)
			}
			duration := time.Since(started).Milliseconds()
			r.setSession(SessionResult{State: "completed", Scan: scan, DurationMS: duration})
		}()
	}
	wait.Wait()
}

func sealSkippedSources(coordinator *ingest.Coordinator, sources []ingest.Source, consumer string) error {
	for _, source := range sources {
		coordinator.Skip(source.Path, consumer)
	}
	return coordinator.Seal(consumer)
}

func (r *round) setResult(result Result) {
	r.mu.Lock()
	r.result = result
	r.mu.Unlock()
}

type usageRoundProgress struct{ round *round }

func (p usageRoundProgress) Start() {}
func (p usageRoundProgress) Stop()  {}
func (p usageRoundProgress) Update(value usage.ScanProgress) {
	p.round.setDomainProgress(true, DomainProgress{State: "processing", Committed: value.Processed, Total: value.Total})
}

type sessionRoundProgress struct{ round *round }

func (p sessionRoundProgress) Start() {}
func (p sessionRoundProgress) Stop()  {}
func (p sessionRoundProgress) Update(value session.ScanProgress) {
	p.round.setDomainProgress(false, DomainProgress{State: "processing", Committed: value.Processed, Total: value.Total, Skipped: value.Skipped})
}

func (r *round) setProgressStage(stage string) {
	r.mu.Lock()
	if r.progress.Stage != stage {
		r.progress.Stage = stage
		r.progressSeq++
		r.progress.Sequence = r.progressSeq
	}
	r.mu.Unlock()
}

func (r *round) setDomainProgress(usageDomain bool, progress DomainProgress) {
	r.mu.Lock()
	if usageDomain {
		r.progress.Usage = progress
	} else {
		r.progress.Session = progress
	}
	r.progressSeq++
	r.progress.Sequence = r.progressSeq
	r.mu.Unlock()
}

func (r *round) setDiscovery(duration int64) {
	r.mu.Lock()
	r.result.Stages.DiscoveryMS = duration
	r.mu.Unlock()
}

func (r *round) setUsage(result UsageResult) {
	r.mu.Lock()
	r.result.Usage = result
	r.result.Stages.UsageMS = result.DurationMS
	r.progress.Usage.State = result.State
	r.progressSeq++
	r.progress.Sequence = r.progressSeq
	r.mu.Unlock()
	r.usageOnce.Do(func() { close(r.usageDone) })
}

func (r *round) setSession(result SessionResult) {
	r.mu.Lock()
	r.result.Session = result
	r.result.Stages.SessionMS = result.DurationMS
	r.progress.Session.State = result.State
	r.progressSeq++
	r.progress.Sequence = r.progressSeq
	r.mu.Unlock()
	r.sessionOnce.Do(func() { close(r.sessionDone) })
}

func (r *round) setDerivedCache(duration int64) {
	r.mu.Lock()
	r.result.Stages.DerivedCacheMS = duration
	r.mu.Unlock()
}

func (r *round) setProcessResources(before, after processResources) {
	r.mu.Lock()
	if after.cpuTime >= before.cpuTime {
		r.result.Stages.WorkerCPUTimeMS = (after.cpuTime - before.cpuTime).Milliseconds()
	}
	r.result.Stages.WorkerPeakRSSBytes = after.peakRSS
	r.mu.Unlock()
}

func (r *round) complete() {
	r.usageOnce.Do(func() { close(r.usageDone) })
	r.sessionOnce.Do(func() { close(r.sessionDone) })
	r.mu.Lock()
	r.result.CompletedAt = time.Now().UTC()
	if !r.result.StartedAt.IsZero() {
		r.result.Stages.TotalMS = r.result.CompletedAt.Sub(r.result.StartedAt).Milliseconds()
	}
	result := r.result
	r.progress.Stage = "completed"
	r.progressSeq++
	r.progress.Sequence = r.progressSeq
	r.mu.Unlock()
	if r.onTerminal != nil {
		if err := r.onTerminal(result); err != nil {
			r.mu.Lock()
			r.terminalErr = err
			r.mu.Unlock()
		}
	}
	close(r.done)
}

func (r *round) snapshot() Result {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.result
}

func (r *round) progressSnapshot() Progress {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.progress
}

func (r *round) scopeDone(scope Scope) (<-chan struct{}, error) {
	switch scope {
	case ScopeUsage:
		return r.usageDone, nil
	case ScopeSession:
		return r.sessionDone, nil
	case ScopeBoth:
		return r.done, nil
	default:
		return nil, ErrInvalidScope
	}
}

func (r *round) wait(ctx context.Context, scope Scope) (Result, error) {
	done, err := r.scopeDone(scope)
	if err != nil {
		return Result{}, err
	}
	select {
	case <-ctx.Done():
		return Result{}, ctx.Err()
	case <-done:
		result := r.snapshot()
		if scope == ScopeBoth {
			r.mu.RLock()
			err := r.terminalErr
			r.mu.RUnlock()
			if err != nil {
				return Result{}, err
			}
		}
		return result, nil
	}
}

func (r *round) waitWithProgress(ctx context.Context, scope Scope, emit func(Progress) error) (Result, error) {
	done, err := r.scopeDone(scope)
	if err != nil {
		return Result{}, err
	}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	var lastSequence uint64
	hasEmitted := false
	emitLatest := func() error {
		progress := r.progressSnapshot()
		if hasEmitted && progress.Sequence == lastSequence {
			return nil
		}
		if err := emit(progress); err != nil {
			return err
		}
		lastSequence = progress.Sequence
		hasEmitted = true
		return nil
	}
	if err := emitLatest(); err != nil {
		return Result{}, err
	}
	for {
		select {
		case <-ctx.Done():
			return Result{}, ctx.Err()
		case <-ticker.C:
			if err := emitLatest(); err != nil {
				return Result{}, err
			}
		case <-done:
			if err := emitLatest(); err != nil {
				return Result{}, err
			}
			return r.wait(ctx, scope)
		}
	}
}

func (r *round) covers(scope Scope) bool {
	switch scope {
	case ScopeUsage:
		select {
		case <-r.usageDone:
			return false
		default:
			return true
		}
	case ScopeSession:
		select {
		case <-r.sessionDone:
			return false
		default:
			return true
		}
	case ScopeBoth:
		select {
		case <-r.usageDone:
			return false
		default:
		}
		select {
		case <-r.sessionDone:
			return false
		default:
			return true
		}
	default:
		return false
	}
}

func failedUsage(err error, duration int64) UsageResult {
	return UsageResult{State: "failed", ErrorCode: failureCode(err), DurationMS: duration}
}

func failedSession(err error, duration int64) SessionResult {
	return SessionResult{State: "failed", ErrorCode: failureCode(err), DurationMS: duration}
}

func failureCode(err error) string {
	var stateErr *store.Error
	if errors.As(err, &stateErr) && stateErr.Code != "" {
		return stateErr.Code
	}
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "deadline_exceeded"
	case errors.Is(err, context.Canceled):
		return "cancelled"
	default:
		return "scan_failed"
	}
}

// WithMaintenance waits behind the same worker election lock before a rebuild
// or watcher mutation.  It deliberately does not join a running round because
// maintenance has different source semantics; it only prevents a second live
// scanner from racing the worker.
func WithMaintenance(ctx context.Context, stateRoot string, timeout time.Duration, run func(context.Context) error) error {
	release, err := AcquireMaintenance(ctx, stateRoot, timeout)
	if err != nil {
		return err
	}
	defer release()
	return run(ctx)
}

// AcquireMaintenance returns the documented barrier used by rebuild and watch
// paths.  Callers acquire it before their own state lock so a worker holding
// scan.lock can finish rather than deadlocking behind a waiting maintenance
// command that already owns state.lock.
func AcquireMaintenance(ctx context.Context, stateRoot string, timeout time.Duration) (func() error, error) {
	stateRoot, _, err := prepareStateRoot(stateRoot)
	if err != nil {
		return nil, err
	}
	lock, err := store.AcquireScanLock(ctx, stateRoot, timeout)
	if err != nil {
		return nil, err
	}
	return lock.Release, nil
}

var localRounds = struct {
	sync.Mutex
	byState map[string]*round
}{byState: map[string]*round{}}

func localRequest(ctx context.Context, stateRoot, home string, scope Scope, now func() time.Time, onProgress func(Progress)) (Result, error) {
	localRounds.Lock()
	round := localRounds.byState[stateRoot]
	if round == nil || round.completed() {
		round = newRound(home)
		round.cacheNow = now
		localRounds.byState[stateRoot] = round
		go round.execute(context.Background(), stateRoot, false)
	}
	localRounds.Unlock()
	if onProgress != nil {
		return round.waitWithProgress(ctx, scope, func(progress Progress) error {
			onProgress(progress)
			return nil
		})
	}
	return round.wait(ctx, scope)
}

func prepareStateRoot(stateRoot string) (string, string, error) {
	if stateRoot == "" {
		return "", "", errors.New("scan state root is required")
	}
	resolved, err := filepath.Abs(stateRoot)
	if err != nil {
		return "", "", err
	}
	if evaluated, evalErr := filepath.EvalSymlinks(resolved); evalErr == nil {
		resolved = evaluated
	}
	resolved = filepath.Clean(resolved)
	if err = platform.EnsureStateRoot(resolved); err != nil {
		return "", "", err
	}
	evaluated, err := filepath.EvalSymlinks(resolved)
	if err != nil {
		return "", "", err
	}
	resolved = filepath.Clean(evaluated)
	digest := sha256.Sum256([]byte(resolved))
	return resolved, hex.EncodeToString(digest[:]), nil
}

func socketPath(stateRoot, stateID string) (string, error) {
	parent, err := scanRuntimeParent(stateRoot)
	if err != nil {
		return "", err
	}
	if err := secureRuntimeDirectory(parent); err != nil {
		return "", err
	}
	shortID := stateID
	if len(shortID) > 24 {
		shortID = shortID[:24]
	}
	directory := filepath.Join(parent, shortID)
	if err := secureRuntimeDirectory(directory); err != nil {
		return "", err
	}
	return filepath.Join(directory, "w"), nil
}

func scanRuntimeParent(stateRoot string) (string, error) {
	if runtime.GOOS == "darwin" {
		// launchd supplies each macOS account a protected per-user temporary
		// directory. A compact child keeps the Unix socket below sockaddr_un's
		// path limit without exposing a predictable /tmp directory to other users.
		parent := filepath.Join(os.TempDir(), "ad-s")
		if len(filepath.Join(parent, strings.Repeat("f", 24), "w")) < 104 {
			return parent, nil
		}
		return compactDarwinRuntimeParent(stateRoot)
	}
	return filepath.Join(scanRuntimeBase(), fmt.Sprintf("agentdeck-scan-%d", os.Geteuid())), nil
}

// compactDarwinRuntimeParent stores only a random basename in the private
// state root. The corresponding /tmp directory is established atomically, so
// another account cannot predict and pre-create the fallback while independent
// client and worker processes still resolve the same short path.
func compactDarwinRuntimeParent(stateRoot string) (string, error) {
	locator := filepath.Join(stateRoot, runtimeParentLocator)
	for attempt := 0; attempt < 8; attempt++ {
		if parent, found, err := readRuntimeParentLocator(locator); err != nil {
			return "", err
		} else if found {
			if err = secureRuntimeDirectory(parent); err == nil {
				return parent, nil
			}
			if removeErr := os.Remove(locator); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				return "", removeErr
			}
		}

		bytes := make([]byte, 16)
		if _, err := rand.Read(bytes); err != nil {
			return "", err
		}
		name := fmt.Sprintf("ad-s-%d-%s", os.Geteuid(), hex.EncodeToString(bytes))
		parent := filepath.Join("/tmp", name)
		if err := os.Mkdir(parent, 0o700); err != nil {
			if errors.Is(err, os.ErrExist) {
				continue
			}
			return "", err
		}
		installed, err := installRuntimeParentLocator(stateRoot, locator, name)
		if err != nil {
			_ = os.Remove(parent)
			return "", err
		}
		if installed {
			return parent, nil
		}
		_ = os.Remove(parent)
	}
	return "", errors.New("could not establish private scan runtime directory")
}

func readRuntimeParentLocator(path string) (string, bool, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() > 128 {
		return "", false, errors.New("scan runtime parent locator is invalid")
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return "", false, err
	}
	name := strings.TrimSpace(string(contents))
	prefix := fmt.Sprintf("ad-s-%d-", os.Geteuid())
	if filepath.Base(name) != name || !strings.HasPrefix(name, prefix) || len(name) != len(prefix)+32 {
		return "", false, errors.New("scan runtime parent locator is invalid")
	}
	if _, err = hex.DecodeString(strings.TrimPrefix(name, prefix)); err != nil {
		return "", false, errors.New("scan runtime parent locator is invalid")
	}
	return filepath.Join("/tmp", name), true, nil
}

func installRuntimeParentLocator(stateRoot, locator, name string) (bool, error) {
	temporary, err := os.CreateTemp(stateRoot, ".scan-runtime-parent-*")
	if err != nil {
		return false, err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err = temporary.Chmod(platform.FileMode); err == nil {
		_, err = temporary.WriteString(name + "\n")
	}
	if err == nil {
		err = temporary.Sync()
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return false, err
	}
	if err = os.Link(temporaryPath, locator); errors.Is(err, os.ErrExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func scanRuntimeBase() string {
	if runtime.GOOS != "windows" {
		return "/tmp"
	}
	return os.TempDir()
}

func secureRuntimeDirectory(path string) error {
	if err := os.Mkdir(path, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("scan runtime path is not an owned directory: %s", path)
	}
	if stat, ok := info.Sys().(*syscall.Stat_t); ok && uint32(stat.Uid) != uint32(os.Geteuid()) {
		return fmt.Errorf("scan runtime path is not owned by the current user: %s", path)
	}
	return os.Chmod(path, 0o700)
}

func removeSocket(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("scan runtime path is not a socket: %s", path)
	}
	return os.Remove(path)
}

type ownerMetadata struct {
	StateID  string `json:"state_id"`
	Nonce    string `json:"nonce"`
	Protocol int    `json:"protocol"`
	PID      int    `json:"pid"`
}

func writeOwnerMetadata(path, stateID, nonce string) error {
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("scan runtime owner metadata is not a regular file: %s", path)
		}
		if err = os.Remove(path); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	contents, err := json.Marshal(ownerMetadata{StateID: stateID, Nonce: nonce, Protocol: ProtocolVersion, PID: os.Getpid()})
	if err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err = file.Write(append(contents, '\n')); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return nil
}

func newID() string {
	bytes := make([]byte, 12)
	if _, err := rand.Read(bytes); err == nil {
		return hex.EncodeToString(bytes)
	}
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
