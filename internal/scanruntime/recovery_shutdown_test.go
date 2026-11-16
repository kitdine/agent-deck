package scanruntime

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/platform"
	"github.com/kitdine/agent-deck/internal/store"
)

func TestServePartialRecoveryFailureReturnsAndReleasesLocks(t *testing.T) {
	for _, failure := range []string{"invalid-state", "app-terminal-write"} {
		t.Run(failure, func(t *testing.T) {
			account, err := platform.AccountHome()
			if err != nil {
				t.Fatal(err)
			}
			if err := platform.ReleaseStateOwnership(); err != nil {
				t.Fatal(err)
			}
			platform.SetInstallationResolver(&admissionOwnershipResolver{account: account})
			t.Cleanup(func() {
				_ = platform.ReleaseStateOwnership()
				platform.SetInstallationResolver(isolatedStateTestResolver{account})
			})
			root, stateID, err := prepareStateRoot(filepath.Join(account, ".agentdeck", "recovery-"+newID()))
			if err != nil {
				t.Fatal(err)
			}
			journal, err := openReceiptJournal(root)
			if err != nil {
				t.Fatal(err)
			}
			first := scanReceipt{ID: "first", StateID: stateID, Home: account, Scope: ScopeBoth, RoundID: "cli-first", Observation: 1}
			if err := journal.accept(first); err != nil {
				t.Fatal(err)
			}
			second := scanReceipt{ID: "second", StateID: stateID, Home: account, Scope: ScopeBoth, RoundID: "second-round", Observation: 2}
			want := "another state root"
			if failure == "invalid-state" {
				second.StateID = "different-state"
			} else {
				second.AppOwned = true
				want = "injected recovery sync failure"
			}
			if err := journal.accept(second); err != nil {
				t.Fatal(err)
			}
			if failure == "app-terminal-write" {
				original := syncReceiptDirectory
				syncReceiptDirectory = func(string) error { return errors.New(want) }
				t.Cleanup(func() { syncReceiptDirectory = original })
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- Serve(ctx, root) }()
			select {
			case err := <-done:
				if err == nil || !strings.Contains(err.Error(), want) {
					t.Fatalf("Serve error = %v, want %s", err, want)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("Serve hung waiting for a recovered round that was never launched")
			}
			scan, err := store.AcquireScanLock(context.Background(), root, 0)
			if err != nil {
				t.Fatalf("scan lock retained: %v", err)
			}
			if err := scan.Release(); err != nil {
				t.Fatal(err)
			}
			// The caller ends the process-wide ownership session only after Serve has
			// returned and closed all workers, just as process exit does for the CLI.
			if err := platform.ReleaseStateOwnership(); err != nil {
				t.Fatal(err)
			}
			lock, err := platform.AcquireOwnershipLock(context.Background(), account, true)
			if err != nil {
				t.Fatalf("ownership lock retained: %v", err)
			}
			if err := lock.Release(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestShutdownWaitsForLaunchedRoundAfterCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started, cancelled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	s := &server{ctx: ctx, stateRoot: t.TempDir(), run: func(ctx context.Context, _, _, id string, _ bool) Result {
		close(started)
		<-ctx.Done()
		close(cancelled)
		<-release
		return Result{RoundID: id, Usage: failedUsage(ctx.Err(), 0), Session: failedSession(ctx.Err(), 0)}
	}}
	r, err := s.startOrJoin(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	<-started
	cancel()
	<-cancelled
	waited := make(chan struct{})
	go func() { s.waitForLaunchedRounds(); close(waited) }()
	select {
	case <-waited:
		t.Fatal("shutdown returned while launched work still held resources")
	case <-time.After(30 * time.Millisecond):
	}
	close(release)
	select {
	case <-waited:
	case <-time.After(time.Second):
		t.Fatal("shutdown did not return after launched work ended")
	}
	select {
	case <-r.done:
	default:
		t.Fatal("shutdown did not wait for round completion")
	}
}
