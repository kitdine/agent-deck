package scanruntime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
)

const scanWorkerHelperEnv = "AGENTDECK_SCANRUNTIME_TEST_WORKER"
const scanWorkerStateEnv = "AGENTDECK_SCANRUNTIME_TEST_STATE"

func TestScanRuntimeWorkerHelper(t *testing.T) {
	if os.Getenv(scanWorkerHelperEnv) != "1" {
		return
	}
	if err := Serve(context.Background(), os.Getenv(scanWorkerStateEnv)); err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}

func TestAcceptedRoundContinuesAfterWaiterDetachesAndUsesFiniteFollowUp(t *testing.T) {
	release := make(chan struct{})
	started := make(chan string, 2)
	var mu sync.Mutex
	calls := 0
	server := &server{
		ctx:       context.Background(),
		stateRoot: t.TempDir(),
		run: func(_ context.Context, _ string, _ string, roundID string, _ bool) Result {
			mu.Lock()
			calls++
			mu.Unlock()
			started <- roundID
			<-release
			return Result{RoundID: roundID, Usage: UsageResult{State: "completed"}, Session: SessionResult{State: "completed"}}
		},
	}

	first, err := server.startOrJoin("home")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := server.startOrJoin("different-home"); err == nil {
		t.Fatal("running round accepted incompatible source-root configuration")
	}
	second, err := server.startOrJoin("home")
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("covered requests must join one running round")
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("round did not start")
	}

	waiter, cancel := context.WithCancel(context.Background())
	cancel()
	select {
	case <-waiter.Done():
	case <-time.After(time.Second):
		t.Fatal("waiter did not detach")
	}
	select {
	case <-first.done:
		t.Fatal("detaching a waiter cancelled accepted work")
	default:
	}

	close(release)
	select {
	case <-first.done:
	case <-time.After(time.Second):
		t.Fatal("accepted round did not finish")
	}

	followUp, err := server.startOrJoin("home")
	if err != nil {
		t.Fatal(err)
	}
	if followUp == first {
		t.Fatal("request after terminal observation must start a finite follow-up round")
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("follow-up round did not start")
	}
	select {
	case <-followUp.done:
	case <-time.After(time.Second):
		t.Fatal("follow-up round did not finish")
	}
	mu.Lock()
	defer mu.Unlock()
	if calls != 2 {
		t.Fatalf("round executions = %d, want 2", calls)
	}
}

func TestRoundKeepsHealthyDomainWhenOtherStoreCannotOpen(t *testing.T) {
	for _, test := range []struct {
		name        string
		failCore    bool
		wantUsage   string
		wantSession string
	}{
		{name: "session store unavailable", wantUsage: "completed", wantSession: "failed"},
		{name: "core store unavailable", failCore: true, wantUsage: "failed", wantSession: "completed"},
	} {
		t.Run(test.name, func(t *testing.T) {
			round := newRound(t.TempDir())
			if test.failCore {
				round.openCore = func(context.Context, string) (*store.Store, error) {
					return nil, errors.New("synthetic core open failure")
				}
			} else {
				round.openSessions = func(context.Context, string) (*store.Store, error) {
					return nil, errors.New("synthetic session open failure")
				}
			}
			round.executeProduction(context.Background(), t.TempDir(), false)
			if round.result.Usage.State != test.wantUsage || round.result.Session.State != test.wantSession {
				t.Fatalf("result usage=%#v session=%#v", round.result.Usage, round.result.Session)
			}
		})
	}
}

func TestLateScopeQueuesOneFollowUpBeforeOtherDomainCompletes(t *testing.T) {
	state := t.TempDir()
	journal, err := openReceiptJournal(state)
	if err != nil {
		t.Fatal(err)
	}
	first := newRoundWithID("home", "round-1", 1)
	first.setResult(Result{RoundID: first.id, Session: SessionResult{State: "processing"}})
	first.setUsage(UsageResult{State: "completed"})
	release := make(chan struct{})
	server := &server{
		ctx:             context.Background(),
		stateRoot:       state,
		stateID:         "state",
		journal:         journal,
		round:           first,
		rounds:          map[string]*round{first.id: first},
		nextObservation: 1,
		run: func(_ context.Context, _ string, _ string, id string, _ bool) Result {
			<-release
			return Result{RoundID: id, Usage: UsageResult{State: "completed"}, Session: SessionResult{State: "completed"}}
		},
	}
	firstRequest := wireRequest{Protocol: ProtocolVersion, Request: "late-usage-1", StateID: "state", Home: "home", Scope: ScopeUsage}
	secondRequest := wireRequest{Protocol: ProtocolVersion, Request: "late-usage-2", StateID: "state", Home: "home", Scope: ScopeUsage}
	late, terminal, err := server.accept(firstRequest)
	if err != nil || terminal != nil {
		t.Fatalf("first late request round=%#v terminal=%#v err=%v", late, terminal, err)
	}
	if late == first || late.observation != 2 {
		t.Fatalf("late request round=%#v, want finite observation 2 after round 1", late)
	}
	again, terminal, err := server.accept(secondRequest)
	if err != nil || terminal != nil {
		t.Fatalf("second late request round=%#v terminal=%#v err=%v", again, terminal, err)
	}
	if again != late {
		t.Fatal("continuous late requests created more than one follow-up observation")
	}
	select {
	case <-late.done:
		t.Fatal("follow-up started before the current session observation terminated")
	default:
	}
	first.setSession(SessionResult{State: "completed"})
	first.complete()
	close(release)
	select {
	case <-late.done:
	case <-time.After(time.Second):
		t.Fatal("finite follow-up did not run after the current observation")
	}
}

func TestTerminalReceiptFailureStillAdvancesRoundQueue(t *testing.T) {
	state := t.TempDir()
	current := newRoundWithID("home", "round-1", 1)
	pending := newRoundWithID("home", "round-2", 2)
	pending.predecessor = current
	journal := &receiptJournal{
		path: filepath.Join(state, "missing", receiptJournalFilename),
		entries: map[string]scanReceipt{
			"current": {ID: "current", StateID: "state", Home: "home", Scope: ScopeBoth, RoundID: current.id, State: receiptAccepted},
			"pending": {ID: "pending", StateID: "state", Home: "home", Scope: ScopeBoth, RoundID: pending.id, State: receiptAccepted},
		},
	}
	server := &server{
		ctx:       context.Background(),
		stateRoot: state,
		stateID:   "state",
		journal:   journal,
		round:     current,
		pending:   pending,
		rounds:    map[string]*round{current.id: current, pending.id: pending},
		run: func(_ context.Context, _ string, _ string, id string, _ bool) Result {
			return Result{RoundID: id, Usage: UsageResult{State: "completed"}, Session: SessionResult{State: "completed"}}
		},
	}
	server.launchRoundLocked(current)
	server.launchRoundLocked(pending)
	select {
	case <-pending.done:
	case <-time.After(time.Second):
		t.Fatal("pending round did not complete after terminal receipt failure")
	}
	server.mu.Lock()
	defer server.mu.Unlock()
	if server.pending != nil || server.round != pending {
		t.Fatalf("queue current=%p pending=%p, want promoted round %p and no pending round", server.round, server.pending, pending)
	}
	if len(server.rounds) != 0 {
		t.Fatalf("completed rounds retained after receipt failures: %#v", server.rounds)
	}
	if current.terminalErr == nil || pending.terminalErr == nil {
		t.Fatalf("terminal errors current=%v pending=%v, want both journal failures", current.terminalErr, pending.terminalErr)
	}
}

func TestRequestAfterInventoryObservationQueuesFollowUp(t *testing.T) {
	current := newRoundWithID("home", "round-1", 1)
	current.inventoryOnce.Do(func() { close(current.inventoryObserved) })
	server := &server{round: current, rounds: map[string]*round{current.id: current}, nextObservation: 1}
	next, start, err := server.selectRoundLocked("home", ScopeUsage)
	if err != nil || !start || next == current || next.observation != 2 {
		t.Fatalf("follow-up=%#v start=%t err=%v", next, start, err)
	}
}

func TestReceiptJournalPersistsAcceptanceAndReplaysTerminalResult(t *testing.T) {
	state := t.TempDir()
	journal, err := openReceiptJournal(state)
	if err != nil {
		t.Fatal(err)
	}
	release := make(chan struct{})
	server := &server{
		ctx:       context.Background(),
		stateRoot: state,
		stateID:   "state",
		journal:   journal,
		rounds:    map[string]*round{},
		run: func(_ context.Context, _ string, _ string, id string, _ bool) Result {
			<-release
			return Result{RoundID: id, Usage: UsageResult{State: "completed"}, Session: SessionResult{State: "completed"}}
		},
	}
	request := wireRequest{Protocol: ProtocolVersion, Request: "stable-request", StateID: "state", Home: "home", Scope: ScopeUsage}
	round, terminal, err := server.accept(request)
	if err != nil || terminal != nil {
		t.Fatalf("accept round=%#v terminal=%#v err=%v", round, terminal, err)
	}
	reopened, err := openReceiptJournal(state)
	if err != nil {
		t.Fatal(err)
	}
	receipt, found := reopened.get(request.Request)
	if !found || receipt.State != receiptAccepted || receipt.RoundID != round.id {
		t.Fatalf("persisted receipt=%#v found=%t", receipt, found)
	}

	result := Result{RoundID: round.id, Usage: UsageResult{State: "completed"}, Session: SessionResult{State: "processing"}}
	if err = server.terminalReceipt(request.Request, result); err != nil {
		t.Fatal(err)
	}
	duplicate, replay, err := server.accept(request)
	if err != nil || duplicate == nil || replay == nil {
		t.Fatalf("duplicate round=%#v replay=%#v err=%v", duplicate, replay, err)
	}
	if replay.RoundID != result.RoundID || replay.Usage.State != "completed" || replay.Session.State != "processing" {
		t.Fatalf("replayed terminal result=%#v", replay)
	}
	close(release)
	select {
	case <-round.done:
	case <-time.After(time.Second):
		t.Fatal("accepted round did not finish before receipt test cleanup")
	}
}

func TestReceiptJournalRetainsExpiredIdentityAfterRetention(t *testing.T) {
	state := t.TempDir()
	journal, err := openReceiptJournal(state)
	if err != nil {
		t.Fatal(err)
	}
	request := wireRequest{Protocol: ProtocolVersion, Request: "expired-request", StateID: "state", Home: "home", Scope: ScopeUsage}
	if err = journal.accept(scanReceipt{ID: request.Request, StateID: request.StateID, Home: request.Home, Scope: request.Scope, RoundID: "round", Observation: 1}); err != nil {
		t.Fatal(err)
	}
	if err = journal.terminal("round", Result{RoundID: "round", Usage: UsageResult{State: "completed"}, Session: SessionResult{State: "completed"}}); err != nil {
		t.Fatal(err)
	}
	receipt, found := journal.get(request.Request)
	if !found || receipt.TerminalAt == nil {
		t.Fatalf("terminal receipt=%#v found=%t", receipt, found)
	}
	if err = journal.prune(receipt.TerminalAt.Add(receiptRetention + time.Nanosecond)); err != nil {
		t.Fatal(err)
	}
	reopened, err := openReceiptJournal(state)
	if err != nil {
		t.Fatal(err)
	}
	expired, found := reopened.get(request.Request)
	if !found || expired.State != receiptExpired || expired.Result != nil {
		t.Fatalf("expired receipt=%#v found=%t", expired, found)
	}
	server := &server{stateID: request.StateID, journal: reopened}
	if _, _, err = server.accept(request); err == nil || err.Error() != "scan request receipt expired; re-evaluate required" {
		t.Fatalf("expired replay error=%v", err)
	}
}

func TestReceiptJournalCapacityRetainsExpiredIdentity(t *testing.T) {
	state := t.TempDir()
	journal, err := openReceiptJournal(state)
	if err != nil {
		t.Fatal(err)
	}
	acceptedAt := time.Now().UTC()
	for index := 0; index < maxReceipts; index++ {
		id := fmt.Sprintf("terminal-%03d", index)
		terminalAt := acceptedAt.Add(time.Duration(index) * time.Nanosecond)
		result := Result{RoundID: id, Usage: UsageResult{State: "completed"}, Session: SessionResult{State: "completed"}}
		journal.entries[id] = scanReceipt{ID: id, StateID: "state", Home: "home", Scope: ScopeUsage, RoundID: id, Observation: uint64(index + 1), State: receiptTerminal, RoundTerminal: true, AcceptedAt: terminalAt, TerminalAt: &terminalAt, Result: &result}
	}
	if err = journal.accept(scanReceipt{ID: "new-request", StateID: "state", Home: "home", Scope: ScopeUsage, RoundID: "new-round", Observation: maxReceipts + 1}); err != nil {
		t.Fatal(err)
	}
	reopened, err := openReceiptJournal(state)
	if err != nil {
		t.Fatal(err)
	}
	expired, found := reopened.get("terminal-000")
	if !found || expired.State != receiptExpired || expired.Result != nil {
		t.Fatalf("capacity-evicted receipt=%#v found=%t", expired, found)
	}
	server := &server{stateID: "state", journal: reopened}
	request := wireRequest{Protocol: ProtocolVersion, Request: "terminal-000", StateID: "state", Home: "home", Scope: ScopeUsage}
	if _, _, err = server.accept(request); err == nil || err.Error() != "scan request receipt expired; re-evaluate required" {
		t.Fatalf("capacity replay error=%v", err)
	}
	if got, limit := len(reopened.entries), maxReceipts+maxExpiredReceipts; got > limit {
		t.Fatalf("bounded receipt identity count=%d, want at most %d", got, limit)
	}
}

func TestRecoveredAcceptedReceiptReplansAndCompletes(t *testing.T) {
	state := t.TempDir()
	journal, err := openReceiptJournal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err = journal.accept(scanReceipt{ID: "recover-me", StateID: "state", Home: "home", Scope: ScopeBoth, RoundID: "persisted-round", Observation: 7}); err != nil {
		t.Fatal(err)
	}
	reopened, err := openReceiptJournal(state)
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan string, 1)
	server := &server{
		ctx:       context.Background(),
		stateRoot: state,
		stateID:   "state",
		journal:   reopened,
		rounds:    map[string]*round{},
		run: func(_ context.Context, _ string, _ string, id string, _ bool) Result {
			started <- id
			return Result{RoundID: id, Usage: UsageResult{State: "completed"}, Session: SessionResult{State: "completed"}}
		},
	}
	if err = server.recoverReceipts(); err != nil {
		t.Fatal(err)
	}
	select {
	case id := <-started:
		if id != "persisted-round" {
			t.Fatalf("recovered round id=%q", id)
		}
	case <-time.After(time.Second):
		t.Fatal("accepted receipt was not replanned after worker restart")
	}
	select {
	case <-server.round.done:
	case <-time.After(time.Second):
		t.Fatal("recovered receipt round did not terminate")
	}
	final, found := reopened.get("recover-me")
	if !found || final.State != receiptTerminal || final.Result == nil {
		t.Fatalf("recovered receipt=%#v found=%t", final, found)
	}
}

func TestScopedTerminalReceiptRetainsGlobalRecoveryObligation(t *testing.T) {
	journal, err := openReceiptJournal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err = journal.accept(scanReceipt{ID: "usage-only", StateID: "state", Home: "home", Scope: ScopeUsage, RoundID: "round", Observation: 1}); err != nil {
		t.Fatal(err)
	}
	partial := Result{RoundID: "round", Usage: UsageResult{State: "completed"}, Session: SessionResult{State: "processing"}}
	if err = journal.terminalReceipt("usage-only", partial); err != nil {
		t.Fatal(err)
	}
	if pending := journal.recoverable(); len(pending) != 1 || pending[0].ID != "usage-only" {
		t.Fatalf("scoped terminal receipt lost global obligation: %#v", pending)
	}
	complete := Result{RoundID: "round", Usage: UsageResult{State: "completed"}, Session: SessionResult{State: "completed"}}
	if err = journal.terminal("round", complete); err != nil {
		t.Fatal(err)
	}
	if pending := journal.recoverable(); len(pending) != 0 {
		t.Fatalf("completed global round remained recoverable: %#v", pending)
	}
}

func TestScopedOutcomeDoesNotAdoptOtherDomainFailure(t *testing.T) {
	result := Result{
		Usage:   UsageResult{State: "completed"},
		Session: SessionResult{State: "failed", Error: "session parser rejected source"},
	}
	if err := result.ErrorFor(ScopeUsage); err != nil {
		t.Fatalf("usage-scoped result = %v, want success", err)
	}
	if err := result.ErrorFor(ScopeSession); err == nil {
		t.Fatal("session-scoped result unexpectedly succeeded")
	}
	if err := result.ErrorFor(ScopeBoth); err == nil {
		t.Fatal("unscoped result unexpectedly succeeded")
	}
}

func TestUnixWorkerServesAClientRound(t *testing.T) {
	state := t.TempDir()
	home := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	served := make(chan error, 1)
	go func() { served <- Serve(ctx, state) }()
	_, stateID, err := prepareStateRoot(state)
	if err != nil {
		t.Fatal(err)
	}
	endpoint, err := socketPath(stateID)
	if err != nil {
		t.Fatal(err)
	}
	ready := time.NewTimer(time.Second)
	defer ready.Stop()
	for {
		if _, statErr := os.Stat(endpoint); statErr == nil {
			break
		}
		select {
		case serveErr := <-served:
			t.Fatalf("Serve before ready: %v", serveErr)
		case <-ready.C:
			t.Fatal("worker socket did not become ready")
		case <-time.After(10 * time.Millisecond):
		}
	}
	contents, err := os.ReadFile(filepath.Join(filepath.Dir(endpoint), "owner.json"))
	if err != nil {
		t.Fatalf("read worker owner metadata: %v", err)
	}
	var owner ownerMetadata
	if err = json.Unmarshal(contents, &owner); err != nil {
		t.Fatalf("decode worker owner metadata: %v", err)
	}
	if owner.StateID != stateID || owner.Nonce == "" || owner.Protocol != ProtocolVersion || owner.PID <= 0 {
		t.Fatalf("worker owner metadata = %#v", owner)
	}

	requestCtx, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()
	client := Client{
		StateRoot: state,
		Home:      home,
		RequestID: "stable-unix-request",
		Launch:    func(context.Context, string) error { return nil },
	}
	result, err := client.Request(requestCtx, ScopeBoth)
	if err != nil {
		t.Fatalf("worker request: %v", err)
	}
	if result.RoundID == "" || result.Usage.State != "completed" || result.Session.State != "completed" {
		t.Fatalf("worker result = %#v", result)
	}
	cache, err := os.Stat(filepath.Join(state, "desktop-derived-cache.json"))
	if err != nil || cache.Mode().Perm() != 0o600 {
		t.Fatalf("worker derived cache=%#v err=%v", cache, err)
	}
	replayed, err := client.Request(requestCtx, ScopeBoth)
	if err != nil || replayed.RoundID != result.RoundID {
		t.Fatalf("stable request replay=%#v err=%v, want round %q", replayed, err, result.RoundID)
	}
	cancel()
	select {
	case err := <-served:
		if err != nil {
			t.Fatalf("Serve: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not stop after context cancellation")
	}
}

func TestPrepareStateRootRecanonicalizesNewDirectoryBelowSymlink(t *testing.T) {
	root := t.TempDir()
	realParent := filepath.Join(root, "real")
	if err := os.MkdirAll(realParent, 0o700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "alias")
	if err := os.Symlink(realParent, alias); err != nil {
		t.Fatal(err)
	}
	state := filepath.Join(alias, "new-state")
	firstRoot, firstID, err := prepareStateRoot(state)
	if err != nil {
		t.Fatal(err)
	}
	secondRoot, secondID, err := prepareStateRoot(state)
	if err != nil {
		t.Fatal(err)
	}
	wantParent, err := filepath.EvalSymlinks(realParent)
	if err != nil {
		t.Fatal(err)
	}
	wantRoot := filepath.Join(wantParent, "new-state")
	if firstRoot != wantRoot || secondRoot != wantRoot || firstID != secondID {
		t.Fatalf("first=(%q,%q) second=(%q,%q), want stable canonical root %q", firstRoot, firstID, secondRoot, secondID, wantRoot)
	}
}

func TestDarwinRuntimeParentUsesProtectedPerUserTemporaryDirectory(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS runtime path contract")
	}
	parent := scanRuntimeParent()
	if parent != filepath.Join(os.TempDir(), "ad-s") {
		t.Fatalf("runtime parent=%q", parent)
	}
	if endpoint := filepath.Join(parent, strings.Repeat("f", 24), "w"); len(endpoint) >= 104 {
		t.Fatalf("runtime endpoint exceeds macOS sockaddr_un limit: %d %q", len(endpoint), endpoint)
	}
}

func TestClientLaunchesDetachedWorkerHelper(t *testing.T) {
	state := t.TempDir()
	home := t.TempDir()
	var worker *exec.Cmd
	requestCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var progress []Progress
	result, err := (Client{
		StateRoot: state,
		Home:      home,
		Launch: func(_ context.Context, root string) error {
			worker = exec.Command(os.Args[0], "-test.run=^TestScanRuntimeWorkerHelper$")
			worker.Env = append(os.Environ(), scanWorkerHelperEnv+"=1", scanWorkerStateEnv+"="+root)
			worker.Stdin = nil
			worker.Stdout = io.Discard
			worker.Stderr = io.Discard
			return worker.Start()
		},
	}).RequestWithProgress(requestCtx, ScopeBoth, func(value Progress) {
		progress = append(progress, value)
	})
	if err != nil {
		t.Fatalf("detached worker request: %v", err)
	}
	if result.RoundID == "" || result.Usage.State != "completed" || result.Session.State != "completed" {
		t.Fatalf("detached worker result = %#v", result)
	}
	if len(progress) == 0 || progress[len(progress)-1].Stage != "completed" {
		t.Fatalf("detached worker progress=%#v", progress)
	}
	for index := 1; index < len(progress); index++ {
		if progress[index].Sequence <= progress[index-1].Sequence {
			t.Fatalf("progress sequence=%#v", progress)
		}
	}
	if worker == nil {
		t.Fatal("worker launch was not attempted")
	}
	if err := worker.Wait(); err != nil {
		t.Fatalf("worker exit: %v", err)
	}
}
