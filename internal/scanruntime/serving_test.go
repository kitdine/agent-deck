package scanruntime

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/desktop"
	"github.com/kitdine/agent-deck/internal/ingest"
	"github.com/kitdine/agent-deck/internal/store"
)

func writeServingSource(t *testing.T, home, name string, at time.Time, tokens int) string {
	t.Helper()
	path := filepath.Join(home, ".codex", "sessions", name, "fixture.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	text := fmt.Sprintf(`{"type":"session_meta","payload":{"id":%q}}
{"type":"turn_context","payload":{"turn_id":"turn","model":"gpt-5"}}
{"timestamp":%q,"type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":%d,"output_tokens":1,"total_tokens":%d}}}}
`, name, at.UTC().Format(time.RFC3339Nano), tokens, tokens+1)
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestServingFirstBatchPublishesBeforeHistoryAndRetainsFullInventory(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, ".agentdeck")
	root, _, err := prepareStateRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	writeServingSource(t, home, now.UTC().Format("2006/01/02"), now.Add(-time.Minute), 27)
	history := writeServingSource(t, home, "2000/01/01", now.AddDate(0, 0, -20), 99)
	old := now.AddDate(-1, 0, 0)
	if err := os.Chtimes(history, old, old); err != nil {
		t.Fatal(err)
	}
	r := newRound(home)
	r.serving = true
	r.cacheNow = func() time.Time { return now }
	blocked, release := make(chan struct{}), make(chan struct{})
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	t.Cleanup(func() {
		cancel()
		select {
		case <-r.done:
		case <-time.After(2 * time.Second):
			t.Error("serving round did not stop")
		}
	})
	r.beforeServingBatch = func(index int) {
		if index == 1 {
			close(blocked)
			select {
			case <-release:
			case <-ctx.Done():
			}
		}
	}
	go r.execute(ctx, root, false)
	select {
	case <-blocked:
	case <-r.done:
		result, _ := r.wait(ctx, ScopeBoth)
		t.Fatalf("round terminated before first publication: %+v", result)
	case <-ctx.Done():
		t.Fatal("priority batch did not finish")
	}
	select {
	case <-r.done:
		t.Fatal("global round terminated before history")
	default:
	}
	prepared, err := (desktop.Service{StateRoot: root, Home: home, Now: r.cacheNow}).ReadServingSnapshot(ctx)
	if err != nil {
		close(release)
		t.Fatal(err)
	}
	if prepared.Header.UsageCoverage.Sources != 2 || prepared.Header.UsageCoverage.Committed != 1 || prepared.Header.UsageCoverage.State != "partial" {
		close(release)
		t.Fatalf("priority hints dropped or completed history: %+v", prepared.Header.UsageCoverage)
	}
	today := prepared.Payload.Data.Usage.Presentation.Scopes[0].Periods.Items[0]
	if today.Totals.Tokens != 28 || prepared.Payload.Data.Sessions.Available {
		close(release)
		t.Fatalf("wrong first batch: %+v", today.Totals)
	}
	close(release)
	result, err := r.wait(ctx, ScopeBoth)
	if err != nil || result.Usage.State != "completed" || result.Session.State != "completed" {
		t.Fatalf("finite history failed: %+v %v", result, err)
	}
	core, err := store.OpenReadOnly(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	defer core.Close()
	coverage, err := core.ReadSourceCoverage(ctx)
	if err != nil || coverage.State != "complete" || coverage.Committed != 2 {
		t.Fatalf("history coverage not committed: %+v %v", coverage, err)
	}
}

func TestServingSourcePriorityHintsNeverDropUnknownArchivedOrOldActiveSources(t *testing.T) {
	now := time.Now()
	sources := []ingest.Source{
		{Path: "old-active", ModifiedAt: now.UnixNano()},
		{Path: "unknown-flat", ModifiedAt: 1},
		{Path: "/.codex/archived_sessions/unknown", ModifiedAt: 1},
	}
	batches := servingSourceBatches(sources, now)
	if len(batches) != 2 || len(batches[0]) != 1 || batches[0][0].Path != "old-active" || len(batches[1]) != 2 {
		t.Fatalf("priority filtered inventory: %+v", batches)
	}
}

func TestServingMetadataOnlyNoopProvesTodayZeroBeforeGlobalDerivation(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, ".agentdeck")
	now := time.Now()
	oldSource := writeServingSource(t, home, "2000/01/01", now.AddDate(0, 0, -20), 99)
	old := now.AddDate(-1, 0, 0)
	if err := os.Chtimes(oldSource, old, old); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	client := Client{StateRoot: root, Home: home, ForceLocal: true, Now: func() time.Time { return now }}
	if result, err := client.Request(ctx, ScopeBoth); err != nil || result.Usage.State != "completed" || result.Session.State != "completed" {
		t.Fatalf("seed failed: %+v %v", result, err)
	}
	// Remove only this test's generated full serving cache to exercise no-cache.
	if err := os.Remove(filepath.Join(root, "desktop-serving-snapshot.json")); err != nil {
		t.Fatal(err)
	}
	r := newRound(home)
	r.serving = true
	r.cacheNow = client.Now
	batchCount := 0
	r.beforeServingBatch = func(int) { batchCount++ }
	r.execute(ctx, root, false)
	result, err := r.wait(ctx, ScopeBoth)
	if err != nil || result.Usage.State != "completed" || result.Session.State != "completed" {
		t.Fatalf("no-op failed: %+v %v", result, err)
	}
	if batchCount != 1 {
		t.Fatalf("metadata-only scan split into %d history batches", batchCount)
	}
	prepared, err := (desktop.Service{StateRoot: root, Home: home, Now: client.Now}).ReadServingSnapshot(ctx)
	if err != nil || !prepared.Payload.Data.Usage.Available || prepared.Header.UsageCoverage.State != "complete" {
		t.Fatalf("proved zero unavailable: %+v %v", prepared.Header, err)
	}
	if tokens := prepared.Payload.Data.Usage.Presentation.Scopes[0].Periods.Items[0].Totals.Tokens; tokens != 0 {
		t.Fatalf("historical tokens relabelled today: %d", tokens)
	}
}

func TestServingOwnerConnectionCountsAndCLIObligation(t *testing.T) {
	for _, cli := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		r := newRound(t.TempDir())
		r.cancelWork = cancel
		r.registerOwner("app-request", true)
		r.registerOwner("app-request", true)
		if cli {
			r.registerOwner("cli-request", false)
		}
		r.releaseAppOwner("app-request")
		if ctx.Err() != nil {
			t.Fatal("closing one replay connection cancelled another owner")
		}
		r.releaseAppOwner("app-request")
		if cli && ctx.Err() != nil {
			t.Fatal("App exit cancelled accepted CLI obligation")
		}
		if !cli && ctx.Err() == nil {
			t.Fatal("App-only work survived its last owner")
		}
		cancel()
	}
}

func TestServingResponseSizeBoundIsPerMessageNotLifetime(t *testing.T) {
	readerConn, writerConn := net.Pipe()
	defer readerConn.Close()
	defer writerConn.Close()
	writeDone := make(chan error, 1)
	go func() {
		encoder := json.NewEncoder(writerConn)
		for range 2 {
			if err := encoder.Encode(wireResponse{Type: "publication", Error: strings.Repeat("x", 4<<20)}); err != nil {
				writeDone <- err
				return
			}
		}
		writeDone <- nil
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	reader := bufio.NewReader(readerConn)
	for range 2 {
		message, err := decodeServingResponse(ctx, readerConn, reader)
		if err != nil || len(message.Error) != 4<<20 {
			t.Fatalf("bounded individual message failed: %v", err)
		}
	}
	if err := <-writeDone; err != nil {
		t.Fatal(err)
	}
}

// Exercise real production discovery/commit/publication with both public
// admission paths. Gates make the request order deterministic, not timing based.
func TestServingJoinedRoundPublishesBeforeHistory(t *testing.T) {
	for _, local := range []bool{false, true} {
		for _, appFirst := range []bool{false, true} {
			t.Run(fmt.Sprintf("local=%t/appFirst=%t", local, appFirst), func(t *testing.T) {
				home := t.TempDir()
				root, _, err := prepareStateRoot(filepath.Join(home, ".agentdeck"))
				if err != nil {
					t.Fatal(err)
				}
				now := time.Now()
				writeServingSource(t, home, now.UTC().Format("2006/01/02"), now.Add(-time.Minute), 27)
				history := writeServingSource(t, home, "2000/01/01", now.AddDate(0, 0, -20), 99)
				old := now.AddDate(-1, 0, 0)
				if err := os.Chtimes(history, old, old); err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				appCtx, cancelApp := context.WithCancel(ctx)
				defer cancelApp()
				r := newRound(home)
				workCtx, stopWork := context.WithCancel(ctx)
				defer stopWork()
				r.cancelWork = stopWork
				r.cacheNow = func() time.Time { return now }
				discovered, admit := make(chan struct{}), make(chan struct{})
				historyBlocked, release := make(chan struct{}), make(chan struct{})
				r.beforeDiscovery = func() {
					close(discovered)
					select {
					case <-admit:
					case <-ctx.Done():
					}
				}
				r.beforeServingBatch = func(index int) {
					if index == 1 {
						close(historyBlocked)
						select {
						case <-release:
						case <-ctx.Done():
						}
					}
				}
				server := &server{ctx: ctx, stateRoot: root, round: r, rounds: map[string]*round{r.id: r}}
				if local {
					localRounds.Lock()
					localRounds.byState[root] = r
					localRounds.Unlock()
				}
				appDone := make(chan struct{})
				cliDone := make(chan Result, 1)
				published := make(chan struct{}, 1)
				join := func(app bool) {
					if !local {
						id := "cli"
						if app {
							id = "app"
						}
						joined, _, err := server.accept(wireRequest{Request: id, Home: home, Scope: ScopeBoth, AppOwned: app, Serving: app})
						if err != nil || joined != r {
							t.Fatalf("join: %v same=%t", err, joined == r)
						}
						return
					}
					ready := make(chan struct{})
					var once sync.Once
					progress := func(Progress) { once.Do(func() { close(ready) }) }
					go func() {
						if app {
							_, _ = localServingRequest(appCtx, root, home, ScopeBoth, r.cacheNow, progress, true, func(desktop.ServingSnapshot) {
								select {
								case published <- struct{}{}:
								default:
								}
							})
							close(appDone)
						} else {
							result, _ := localRequest(ctx, root, home, ScopeBoth, r.cacheNow, progress)
							cliDone <- result
						}
					}()
					select {
					case <-ready:
					case <-ctx.Done():
						t.Fatal("local request not admitted")
					}
				}
				join(appFirst)
				go r.execute(workCtx, root, false)
				t.Cleanup(func() {
					cancel()
					select {
					case <-r.done:
					case <-time.After(time.Second):
						t.Error("round leaked")
					}
					if local {
						localRounds.Lock()
						delete(localRounds.byState, root)
						localRounds.Unlock()
					}
				})
				<-discovered
				join(!appFirst)
				close(admit)
				select {
				case <-historyBlocked:
				case <-r.done:
					t.Fatalf("no first checkpoint: %+v", r.snapshot())
				case <-ctx.Done():
					t.Fatal("checkpoint timed out")
				}
				r.mu.RLock()
				publication := r.publication
				r.mu.RUnlock()
				if publication == nil || publication.Header.UsageCoverage.Committed != 1 || publication.Header.UsageCoverage.Sources != 2 {
					t.Fatal("first committed publication missing or history omitted")
				}
				if got := publication.Payload.Data.Usage.Presentation.Scopes[0].Periods.Items[0].Totals.Tokens; got != 28 {
					t.Fatalf("first tokens=%d", got)
				}
				if !r.covers(ScopeBoth) {
					t.Fatal("CLI terminal before history")
				}
				if local {
					select {
					case <-published:
					case <-ctx.Done():
						t.Fatal("subscriber did not receive checkpoint")
					}
					cancelApp()
					select {
					case <-appDone:
					case <-ctx.Done():
						t.Fatal("App did not detach")
					}
				} else {
					r.releaseAppOwner("app")
				}
				if workCtx.Err() != nil {
					t.Fatal("App exit cancelled CLI")
				}
				close(release)
				result, err := r.wait(ctx, ScopeBoth)
				if err != nil || result.Usage.State != "completed" || result.Session.State != "completed" {
					t.Fatalf("CLI incomplete: %+v %v", result, err)
				}
				if local {
					select {
					case result := <-cliDone:
						if result.Usage.State != "completed" || result.Session.State != "completed" {
							t.Fatal("local CLI incomplete")
						}
					case <-ctx.Done():
						t.Fatal("CLI wait blocked")
					}
				}
				core, err := store.OpenReadOnly(ctx, root)
				if err != nil {
					t.Fatal(err)
				}
				defer core.Close()
				coverage, err := core.ReadSourceCoverage(ctx)
				if err != nil || coverage.State != "complete" || coverage.Committed != 2 {
					t.Fatalf("coverage=%+v %v", coverage, err)
				}
			})
		}
	}
}

func TestServingAdmissionSealsCapabilityAndUpgradesPendingRound(t *testing.T) {
	r := newRound("home")
	r.inventoryOnce.Do(func() { close(r.inventoryObserved) })
	s := &server{round: r, rounds: map[string]*round{r.id: r}}
	pending, start, err := s.selectRoundLocked("home", ScopeBoth)
	if err != nil || !start || pending == r {
		t.Fatalf("pending: %v %t", err, start)
	}
	joined, start, err := s.selectRoundLocked("home", ScopeBoth, true)
	if err != nil || start || joined != pending || !pending.serving || r.serving {
		t.Fatal("serving capability not bound to unobserved pending round")
	}
}

func TestLocalServingRejectsIncompatibleSealedRound(t *testing.T) {
	root := t.TempDir()
	r := newRound(root)
	r.inventoryOnce.Do(func() { close(r.inventoryObserved) })
	r.registerOwner("cli", false)
	localRounds.Lock()
	localRounds.byState[root] = r
	localRounds.Unlock()
	defer func() { localRounds.Lock(); delete(localRounds.byState, root); localRounds.Unlock() }()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := localServingRequest(ctx, root, root, ScopeBoth, nil, nil, true, func(desktop.ServingSnapshot) { t.Error("unexpected publication") })
	if err == nil || errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected explicit capability rejection: %v", err)
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.serving || !r.cliObligation || len(r.appOwners) != 0 {
		t.Fatal("rejection changed accepted round")
	}
}

func TestServingEmptyInventoryReconcilesLikeOrdinaryScan(t *testing.T) {
	for _, seeded := range []bool{false, true} {
		for _, serving := range []bool{false, true} {
			t.Run(fmt.Sprintf("seeded=%t/serving=%t", seeded, serving), func(t *testing.T) {
				home := t.TempDir()
				root, _, err := prepareStateRoot(filepath.Join(home, ".agentdeck"))
				if err != nil {
					t.Fatal(err)
				}
				now := time.Now()
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				run := func(mode bool) *round {
					r := newRound(home)
					r.serving = mode
					r.cacheNow = func() time.Time { return now }
					r.execute(ctx, root, false)
					result, err := r.wait(ctx, ScopeBoth)
					if err != nil || result.Usage.State != "completed" || result.Session.State != "completed" {
						t.Fatalf("round=%+v err=%v", result, err)
					}
					return r
				}
				if seeded {
					path := writeServingSource(t, home, now.UTC().Format("2006/01/02"), now.Add(-time.Minute), 27)
					first := run(true)
					if first.publication == nil || first.publication.Payload.Data.Usage.Presentation.Scopes[0].Periods.Items[0].Totals.Tokens != 28 {
						t.Fatal("seed not published")
					}
					if err := os.Remove(path); err != nil {
						t.Fatal(err)
					}
				}
				r := run(serving)
				db, err := store.OpenReadOnly(ctx, root)
				if err != nil {
					t.Fatal(err)
				}
				defer db.Close()
				for _, table := range []string{"usage_events", "usage_source_files"} {
					var count int
					if err := db.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&count); err != nil {
						t.Fatal(err)
					}
					if count != 0 {
						t.Fatalf("%s=%d after empty scan", table, count)
					}
				}
				coverage, err := db.ReadSourceCoverage(ctx)
				if err != nil || coverage.State != "complete" || coverage.Sources != 0 || coverage.Committed != 0 {
					t.Fatalf("coverage=%+v %v", coverage, err)
				}
				// This regression checks publication contents, not the reader's
				// separate 200ms identity budget under race instrumentation.
				contents, err := os.ReadFile(filepath.Join(root, "desktop-serving-snapshot.json"))
				if err != nil {
					t.Fatal(err)
				}
				var document struct {
					Body     []byte `json:"body"`
					Checksum string `json:"checksum"`
				}
				if err := json.Unmarshal(contents, &document); err != nil {
					t.Fatal(err)
				}
				sum := sha256.Sum256(document.Body)
				if document.Checksum != hex.EncodeToString(sum[:]) {
					t.Fatal("publication checksum mismatch")
				}
				var publication desktop.ServingSnapshot
				if err := json.Unmarshal(document.Body, &publication); err != nil {
					t.Fatal(err)
				}
				emitted, marshalErr := json.Marshal(r.publication)
				if marshalErr != nil || r.publication == nil || !bytes.Equal(document.Body, emitted) {
					t.Fatalf("disk and emitted publication differ: emitted_nil=%t marshal_error=%v persisted_usage=%+v", r.publication == nil, marshalErr, publication.Header.UsageCoverage)
				}
				if !publication.Payload.Data.Usage.Available || publication.Header.UsageCoverage.State != "complete" || publication.Header.UsageCoverage.Sources != 0 {
					t.Fatalf("publication coverage=%+v", publication.Header)
				}
				for _, scope := range publication.Payload.Data.Usage.Presentation.Scopes {
					for _, period := range scope.Periods.Items {
						if period.Totals.Tokens != 0 || period.Totals.Events != 0 {
							t.Fatalf("stale totals=%+v", period.Totals)
						}
					}
				}
			})
		}
	}
}
