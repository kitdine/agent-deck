package scanruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/desktop"
	"github.com/kitdine/agent-deck/internal/platform"
)

func TestOwnershipRefusalSurvivesWorkerResultWire(t *testing.T) {
	for _, reason := range []string{"receipt_other_executable", "executable_unresolved", "ownership_busy"} {
		t.Run(reason, func(t *testing.T) {
			cause := fmt.Errorf("scan round: %w", &platform.ErrStateOwnershipRefused{Resource: "state_root", Reason: reason, Revoked: true})
			encoded, err := json.Marshal(Result{Usage: failedUsage(cause, 0), Session: failedSession(cause, 0)})
			if err != nil {
				t.Fatal(err)
			}
			var decoded Result
			if err = json.Unmarshal(encoded, &decoded); err != nil {
				t.Fatal(err)
			}
			for _, scope := range []Scope{ScopeUsage, ScopeSession, ScopeBoth} {
				var refused *platform.ErrStateOwnershipRefused
				err := decoded.ErrorFor(scope)
				if !errors.As(err, &refused) || refused.Resource != "state_root" || refused.Reason != reason || !refused.Revoked {
					t.Fatalf("scope=%s lost typed refusal on worker wire: %T %v, payload=%s", scope, err, err, encoded)
				}
			}
		})
	}
}

type admissionOwnershipResolver struct {
	account string
	reason  string
	revoked atomic.Bool
}

func (r *admissionOwnershipResolver) AccountHome() (string, error) { return r.account, nil }
func (r *admissionOwnershipResolver) Resolve() platform.InstallationIdentity {
	if r.revoked.Load() {
		return platform.InstallationIdentity{Reason: r.reason}
	}
	return platform.InstallationIdentity{Installed: true, Channel: "make"}
}

func TestWorkerAdmissionOwnershipRefusal(t *testing.T) {
	for _, reason := range []string{"executable_unresolved", "receipt_other_executable", "ownership_busy"} {
		for _, scope := range []Scope{ScopeUsage, ScopeSession, ScopeBoth} {
			for _, serving := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/serving=%t", reason, scope, serving), func(t *testing.T) {
					err := workerAdmissionError(t, scope, serving, reason, false)
					var refused *platform.ErrStateOwnershipRefused
					if !errors.As(err, &refused) || refused.Resource != "state_root" || refused.Reason != reason || !refused.Revoked {
						t.Fatalf("receive-time refusal lost its contract: %T %v, details=%+v", err, err, refused)
					}
				})
			}
		}
	}
}

func TestWorkerAdmissionOwnershipControls(t *testing.T) {
	t.Run("installed-owner-is-accepted", func(t *testing.T) {
		if err := workerAdmissionError(t, ScopeBoth, false, "", false); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("ordinary-rejection-stays-untyped", func(t *testing.T) {
		err := workerAdmissionError(t, ScopeBoth, false, "", true)
		var refused *platform.ErrStateOwnershipRefused
		if err == nil || err.Error() != "incompatible scan worker request" || errors.As(err, &refused) {
			t.Fatalf("ordinary rejection changed: %T %v", err, err)
		}
	})
}

// The listener provides a deterministic barrier after the client's guard and
// before the real worker handler revalidates. No sleep or real account is used.
func workerAdmissionError(t *testing.T, scope Scope, serving bool, reason string, invalidHome bool) error {
	t.Helper()
	originalAccount, err := platform.AccountHome()
	if err != nil {
		t.Fatal(err)
	}
	if err := platform.ReleaseStateOwnership(); err != nil {
		t.Fatal(err)
	}
	// Protected roots are cached for the process account. Reuse TestMain's
	// synthetic account and isolate each request beneath its protected root.
	account := originalAccount
	resolver := &admissionOwnershipResolver{account: account, reason: reason}
	platform.SetInstallationResolver(resolver)
	t.Cleanup(func() {
		if err := platform.ReleaseStateOwnership(); err != nil {
			t.Error(err)
		}
		platform.SetInstallationResolver(isolatedStateTestResolver{account: originalAccount})
	})
	state, stateID, err := prepareStateRoot(filepath.Join(account, ".agentdeck", "admission-"+newID()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(state) })
	journal, err := openReceiptJournal(state)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := journal.save(); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(journal.path)
	if err != nil {
		t.Fatal(err)
	}
	endpoint, err := socketPath(state, stateID)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: endpoint, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = listener.Close()
		_ = os.Remove(filepath.Dir(endpoint))
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := listener.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	var rounds atomic.Int32
	s := &server{
		ctx: ctx, stateRoot: state, stateID: stateID, nonce: "ownership-test",
		journal: journal, rounds: map[string]*round{},
		run: func(_ context.Context, _, _, id string, _ bool) Result {
			rounds.Add(1)
			return Result{RoundID: id, Usage: UsageResult{State: "completed"}, Session: SessionResult{State: "completed"}}
		},
	}
	done := make(chan error, 1)
	go func() {
		conn, err := listener.AcceptUnix()
		if err != nil {
			done <- err
			return
		}
		if reason != "" {
			resolver.revoked.Store(true)
		}
		s.handle(conn)
		done <- nil
	}()
	client := Client{StateRoot: state, Home: account, RequestID: "admission-request"}
	if invalidHome {
		client.Home = ""
	}
	if serving {
		client.OnPublication = func(desktop.ServingSnapshot) { t.Error("rejected request published a snapshot") }
	}
	result, requestErr := client.Request(ctx, scope)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("worker did not finish:", ctx.Err())
	}
	if reason == "" && !invalidHome {
		if rounds.Load() != 1 || result.RoundID == "" || result.ErrorFor(scope) != nil {
			t.Fatalf("positive control did not complete a round: calls=%d result=%+v error=%v", rounds.Load(), result, requestErr)
		}
		return requestErr
	}
	after, err := os.ReadFile(journal.path)
	if err != nil {
		t.Fatal(err)
	}
	if rounds.Load() != 0 || result.RoundID != "" || len(journal.entries) != 0 || !bytes.Equal(before, after) {
		t.Fatalf("rejected request started or persisted work: calls=%d result=%+v receipts=%+v", rounds.Load(), result, journal.entries)
	}
	return requestErr
}
