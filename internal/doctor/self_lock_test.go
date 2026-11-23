package doctor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
)

// A lock held by the diagnosing process itself — a scan worker computing the
// health of its own publication — is not contention with another owner. A
// live lock of any other process remains lock_live with its retry action.
func TestLockRowsDistinguishTheCheckingProcessFromOtherOwners(t *testing.T) {
	ctx := context.Background()
	state := filepath.Join(t.TempDir(), "state")
	database, err := store.Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	if err = database.Close(); err != nil {
		t.Fatal(err)
	}
	rows := func(t *testing.T) map[string]Check {
		t.Helper()
		report, err := (Service{RegistrationProbe: deterministicRegistration, StateRoot: state, Home: t.TempDir(), Workdir: t.TempDir(),
			LockProcessCheck: func(int) (bool, bool) { return true, true }}).Check(ctx, false)
		if err != nil {
			t.Fatal(err)
		}
		found := map[string]Check{}
		for _, check := range report.Checks {
			if check.Name == "state_lock" || check.Name == "scan_lock" {
				found[check.Name] = check
			}
		}
		return found
	}

	scan, err := store.AcquireScanLock(ctx, state, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !store.LockOwnedByCurrentProcess(filepath.Join(state, "scan.lock")) {
		t.Fatal("real acquired scan.lock not recognized as owned by current process")
	}
	own := rows(t)
	if got := own["scan_lock"]; got != (Check{Name: "scan_lock", Status: "ok"}) {
		t.Fatalf("own held scan lock = %+v, want ok", got)
	}
	if got := own["state_lock"]; got != (Check{Name: "state_lock", Status: "ok"}) {
		t.Fatalf("absent state lock = %+v", got)
	}

	// Copying the genuinely held token to another lock name or another state root
	// must not make those copies appear owned by this process.
	heldToken, err := os.ReadFile(filepath.Join(state, "scan.lock"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(state, "state.lock"), heldToken, 0o600); err != nil {
		t.Fatal(err)
	}
	otherRoot := filepath.Join(t.TempDir(), "other")
	if err := os.MkdirAll(otherRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(otherRoot, "scan.lock"), heldToken, 0o600); err != nil {
		t.Fatal(err)
	}
	copied := rows(t)
	if got := copied["scan_lock"]; got != (Check{Name: "scan_lock", Status: "ok"}) {
		t.Fatalf("original held scan lock = %+v, want ok", got)
	}
	wantStateCopied := Check{Name: "state_lock", Status: "warning", Code: "lock_live", Resource: "state", Reason: "lock_live", ActionKind: "retry"}
	if got := copied["state_lock"]; got != wantStateCopied {
		t.Fatalf("copied token on state_lock = %+v, want %+v", got, wantStateCopied)
	}
	if store.LockOwnedByCurrentProcess(filepath.Join(otherRoot, "scan.lock")) {
		t.Fatal("copied token in another root reported as owned by current process")
	}
	if err := os.Remove(filepath.Join(state, "state.lock")); err != nil {
		t.Fatal(err)
	}

	if err := scan.Release(); err != nil {
		t.Fatal(err)
	}
	if store.LockOwnedByCurrentProcess(filepath.Join(state, "scan.lock")) {
		t.Fatal("released scan.lock still reported as owned by current process")
	}

	// An unregistered same-PID token (forged or from PID reuse) blocks
	// acquisition and must be reported as live contention, never ok.
	for _, name := range []string{"state.lock", "scan.lock"} {
		token := fmt.Sprintf("v1:%d:%032x", os.Getpid(), 7)
		if err := os.WriteFile(filepath.Join(state, name), []byte(token), 0o600); err != nil {
			t.Fatal(err)
		}
		if store.LockOwnedByCurrentProcess(filepath.Join(state, name)) {
			t.Fatalf("unregistered same-PID %s reported as owned by current process", name)
		}
	}
	unregistered := rows(t)
	for name, resource := range map[string]string{"state_lock": "state", "scan_lock": "scan"} {
		want := Check{Name: name, Status: "warning", Code: "lock_live", Resource: resource, Reason: "lock_live", ActionKind: "retry"}
		if unregistered[name] != want {
			t.Fatalf("%s with unregistered same-PID token = %+v, want %+v", name, unregistered[name], want)
		}
	}

	for _, name := range []string{"state.lock", "scan.lock"} {
		token := fmt.Sprintf("v1:%d:%032x", os.Getpid()+100000, 7)
		if err := os.WriteFile(filepath.Join(state, name), []byte(token), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	other := rows(t)
	for name, resource := range map[string]string{"state_lock": "state", "scan_lock": "scan"} {
		want := Check{Name: name, Status: "warning", Code: "lock_live", Resource: resource, Reason: "lock_live", ActionKind: "retry"}
		if other[name] != want {
			t.Fatalf("%s of another live owner = %+v, want %+v", name, other[name], want)
		}
	}
}
