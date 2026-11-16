package installation

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/platform"
)

// TestNativeOwnershipFixture is a test-binary-only bridge for native XCTest.
// It never falls back to the operator's account or production helper.
func TestNativeOwnershipFixture(t *testing.T) {
	root := os.Getenv("AGENTDECK_NATIVE_FIXTURE_ROOT")
	if root == "" {
		t.Skip("native fixture driver is only enabled by the isolated harness")
	}
	physical, err := filepath.EvalSymlinks(root)
	if err != nil || physical != filepath.Clean(root) || !strings.HasPrefix(physical, "/private/tmp/agentdeck-macos-xctest.") {
		t.Fatal("unsafe native fixture root")
	}
	home := filepath.Join(physical, "account")
	helper := filepath.Join(physical, "AgentDeck.app", "Contents", "Helpers", "agentdeck")
	for _, path := range []string{home, helper} {
		actual, err := filepath.EvalSymlinks(path)
		if err != nil || actual != path {
			t.Fatalf("unsafe fixture path %s: %v", path, err)
		}
	}
	r := &Resolver{accountHome: func() (string, error) { return home, nil }, executable: func() (string, error) { return helper, nil }}
	action := os.Getenv("AGENTDECK_NATIVE_FIXTURE_ACTION")
	if action == "delete" {
		if err := os.Remove(receiptPath(home)); err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		fmt.Println("NATIVE_RECEIPT_DELETED")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	lock, err := platform.AcquireOwnershipLock(ctx, home, true)
	if err != nil {
		if action == "probe" && (errors.Is(err, context.DeadlineExceeded)) {
			fmt.Println("NATIVE_LOCK_BUSY")
			return
		}
		t.Fatal(err)
	}
	defer lock.Release()
	switch action {
	case "probe":
		fmt.Println("NATIVE_LOCK_ACQUIRED")
	case "register", "mismatch":
		target := helper
		if action == "mismatch" {
			target = filepath.Join(physical, "other-helper")
			if err := os.WriteFile(target, []byte("fixture"), 0700); err != nil {
				t.Fatal(err)
			}
		}
		receipt, err := r.WriteReceipt(lock, target, "local_app")
		if err != nil {
			t.Fatal(err)
		}
		fmt.Printf("NATIVE_RECEIPT %s %s %s\n", receipt.InstallationID, receipt.Executable, filepath.Join(home, "Library", "Application Support", "AgentDeck", "ownership.lock"))
	default:
		t.Fatal("unsupported fixture action")
	}
}
