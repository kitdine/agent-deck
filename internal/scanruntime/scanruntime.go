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
	"sync"
	"time"

	"github.com/kitdine/agent-deck/internal/ingest"
	"github.com/kitdine/agent-deck/internal/platform"
	"github.com/kitdine/agent-deck/internal/session"
	"github.com/kitdine/agent-deck/internal/store"
	"github.com/kitdine/agent-deck/internal/usage"
)

const (
	// ProtocolVersion prevents a client from treating an incompatible worker as
	// an accepted scan.  It is intentionally small and private to this runtime.
	ProtocolVersion = 1

	defaultStartupTimeout = 5 * time.Second
	defaultIdleTimeout    = time.Second
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
	Error      string         `json:"error,omitempty"`
	DurationMS int64          `json:"duration_ms"`
}

type SessionResult struct {
	State      string             `json:"state"`
	Scan       session.ScanResult `json:"scan,omitempty"`
	ErrorCode  string             `json:"error_code,omitempty"`
	Error      string             `json:"error,omitempty"`
	DurationMS int64              `json:"duration_ms"`
}

// StageProfile records the work actually performed by this round.  It is a
// compact diagnostic record for the Task 1 baseline; it is not a user-facing
// progress stream.
type StageProfile struct {
	DiscoveryMS int64 `json:"discovery_ms"`
	UsageMS     int64 `json:"usage_ms"`
	SessionMS   int64 `json:"session_ms"`
	TotalMS     int64 `json:"total_ms"`
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
			return &DomainError{Domain: "usage", Cause: r.Usage.Error}
		}
	case ScopeSession:
		if r.Session.State != "completed" {
			return &DomainError{Domain: "session", Cause: r.Session.Error}
		}
	default:
		if r.Usage.State != "completed" {
			return &DomainError{Domain: "usage", Cause: r.Usage.Error}
		}
		if r.Session.State != "completed" {
			return &DomainError{Domain: "session", Cause: r.Session.Error}
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
}

type wireRequest struct {
	Protocol int    `json:"protocol"`
	Request  string `json:"request"`
	StateID  string `json:"state_id"`
	Home     string `json:"home"`
	Scope    Scope  `json:"scope"`
}

type wireResponse struct {
	Type     string  `json:"type"`
	WorkerID string  `json:"worker_id,omitempty"`
	RoundID  string  `json:"round_id,omitempty"`
	Result   *Result `json:"result,omitempty"`
	Error    string  `json:"error,omitempty"`
}

// Request joins a covered running round or starts one finite follow-up round.
// Client cancellation only stops this wait; it never cancels the worker's
// globally accepted scan.
func (c Client) Request(ctx context.Context, scope Scope) (Result, error) {
	if !scope.valid() {
		return Result{}, ErrInvalidScope
	}
	stateRoot, stateID, err := prepareStateRoot(c.StateRoot)
	if err != nil {
		return Result{}, err
	}
	if c.ForceLocal {
		return localRequest(ctx, stateRoot, c.Home, scope)
	}
	endpoint, err := socketPath(stateID)
	if err != nil {
		return Result{}, err
	}
	requestID := c.RequestID
	if requestID == "" {
		requestID = newID()
	}
	request := wireRequest{Protocol: ProtocolVersion, Request: requestID, StateID: stateID, Home: c.Home, Scope: scope}
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
	last, err := decodeResponse(ctx, conn, decoder)
	if err != nil {
		return Result{}, err
	}
	if last.Type != "terminal" || last.Result == nil || last.WorkerID != first.WorkerID {
		if last.Error != "" {
			return Result{}, errors.New(last.Error)
		}
		return Result{}, errors.New("scan worker returned an invalid terminal response")
	}
	return *last.Result, nil
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
	endpoint, err := socketPath(stateID)
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
	result, err := round.wait(s.ctx, request.Scope)
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
	if s.journal != nil {
		receipt := scanReceipt{
			ID: request.Request, StateID: request.StateID, Home: request.Home, Scope: request.Scope,
			RoundID: round.id, Observation: round.observation,
		}
		if err = s.journal.accept(receipt); err != nil {
			s.rollbackSelectedRoundLocked(round, start)
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
	if s.round.covers(scope) {
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
	if s.journal != nil {
		if err := s.journal.terminal(round.id, result); err != nil {
			return err
		}
	}
	delete(s.rounds, round.id)
	if s.round == round && s.pending != nil {
		next := s.pending
		s.pending = nil
		s.round = next
	}
	s.lastActivity = time.Now()
	return nil
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

	done        chan struct{}
	usageDone   chan struct{}
	sessionDone chan struct{}
	once        sync.Once
	usageOnce   sync.Once
	sessionOnce sync.Once
	mu          sync.RWMutex
	result      Result
	terminalErr error
}

type roundRunner func(context.Context, string, string, string, bool) Result

func newRound(home string) *round {
	return newRoundWithID(home, newID(), 0)
}

func newRoundWithID(home, id string, observation uint64) *round {
	return &round{
		id:          id,
		home:        home,
		observation: observation,
		done:        make(chan struct{}),
		usageDone:   make(chan struct{}),
		sessionDone: make(chan struct{}),
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
		if run == nil {
			r.executeProduction(ctx, stateRoot, lockHeld)
		} else {
			r.setResult(run(ctx, stateRoot, r.home, r.id, lockHeld))
		}
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
	if discoverErr != nil {
		r.setUsage(failedUsage(discoverErr, 0))
		r.setSession(failedSession(discoverErr, 0))
		return
	}

	core, coreErr := store.Open(ctx, stateRoot)
	if coreErr == nil {
		defer core.Close()
	}
	sessions, sessionsErr := store.OpenSessions(ctx, stateRoot)
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
	if usagePlanErr != nil || sessionPlanErr != nil {
		if usagePlanErr == nil {
			usagePlanErr = sessionPlanErr
		}
		if sessionPlanErr == nil {
			sessionPlanErr = usagePlanErr
		}
		r.setUsage(failedUsage(usagePlanErr, 0))
		r.setSession(failedSession(sessionPlanErr, 0))
		return
	}
	coordinator.Start(ctx)
	var wait sync.WaitGroup
	wait.Add(2)
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
	go func() {
		defer wait.Done()
		defer coordinator.ConsumerDone(ingest.ConsumerSession)
		started := time.Now()
		scan, err := session.ScanWithOptions(ctx, sessions.DB, r.home, session.ScanOptions{PreparedSources: sources, Coordinator: coordinator})
		duration := time.Since(started).Milliseconds()
		if err != nil {
			r.setSession(failedSession(err, duration))
			return
		}
		r.setSession(SessionResult{State: "completed", Scan: scan, DurationMS: duration})
	}()
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

func (r *round) setDiscovery(duration int64) {
	r.mu.Lock()
	r.result.Stages.DiscoveryMS = duration
	r.mu.Unlock()
}

func (r *round) setUsage(result UsageResult) {
	r.mu.Lock()
	r.result.Usage = result
	r.result.Stages.UsageMS = result.DurationMS
	r.mu.Unlock()
	r.usageOnce.Do(func() { close(r.usageDone) })
}

func (r *round) setSession(result SessionResult) {
	r.mu.Lock()
	r.result.Session = result
	r.result.Stages.SessionMS = result.DurationMS
	r.mu.Unlock()
	r.sessionOnce.Do(func() { close(r.sessionDone) })
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

func (r *round) wait(ctx context.Context, scope Scope) (Result, error) {
	var done <-chan struct{}
	switch scope {
	case ScopeUsage:
		done = r.usageDone
	case ScopeSession:
		done = r.sessionDone
	case ScopeBoth:
		done = r.done
	default:
		return Result{}, ErrInvalidScope
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
	return UsageResult{State: "failed", ErrorCode: failureCode(err), Error: err.Error(), DurationMS: duration}
}

func failedSession(err error, duration int64) SessionResult {
	return SessionResult{State: "failed", ErrorCode: failureCode(err), Error: err.Error(), DurationMS: duration}
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

func localRequest(ctx context.Context, stateRoot, home string, scope Scope) (Result, error) {
	localRounds.Lock()
	round := localRounds.byState[stateRoot]
	if round == nil || round.completed() {
		round = newRound(home)
		localRounds.byState[stateRoot] = round
		go round.execute(context.Background(), stateRoot, false)
	}
	localRounds.Unlock()
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
	digest := sha256.Sum256([]byte(resolved))
	return resolved, hex.EncodeToString(digest[:]), nil
}

func socketPath(stateID string) (string, error) {
	parent := filepath.Join(scanRuntimeBase(), "agentdeck-scan")
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

func scanRuntimeBase() string {
	if runtime.GOOS != "windows" {
		// /tmp is intentionally used instead of os.TempDir(): macOS commonly
		// gives applications a long per-user TMPDIR that exceeds the Unix socket
		// pathname ceiling before the state identity can be represented.
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
