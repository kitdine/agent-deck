package doctor

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
)

// A scan worker that acquired its lock under one spelling of the state root
// and diagnoses it under another, on a case-insensitive volume, still holds
// the same directory entry: it is not waiting for another scan.
func TestLockCheckRecognizesTheHoldersLockUnderAnotherSpelling(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "CaseRoot")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	folded := filepath.Join(parent, "caseroot")
	rootInfo, err := os.Stat(root)
	if err != nil {
		t.Fatal(err)
	}
	if foldedInfo, err := os.Stat(folded); err != nil || !os.SameFile(rootInfo, foldedInfo) {
		t.Skipf("temporary volume at %s is case-sensitive; a differently cased path names another entry there", parent)
	}
	lock, err := store.AcquireScanLock(context.Background(), root, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = lock.Release() })
	alive := func(int) (bool, bool) { return true, true }
	for _, stateRoot := range []string{root, folded} {
		check, ok := (Service{StateRoot: stateRoot, LockProcessCheck: alive}).LockCheck("scan_lock")
		if !ok || check != (Check{Name: "scan_lock", Status: "ok"}) {
			t.Errorf("LockCheck via %s = %+v, %v; want ok", stateRoot, check, ok)
		}
	}

	// A copy of the held token at another entry is still another owner.
	token, err := os.ReadFile(filepath.Join(root, "scan.lock"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "state.lock"), token, 0o600); err != nil {
		t.Fatal(err)
	}
	check, _ := (Service{StateRoot: folded, LockProcessCheck: alive}).LockCheck("state_lock")
	if check.Status != "warning" || check.Code != string(store.LockReasonLive) {
		t.Fatalf("copied token on state.lock = %+v, want lock_live warning", check)
	}
}
