package desktop

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
)

func scanLockCheck(t *testing.T, health HealthSnapshot) HealthCheck {
	t.Helper()
	for _, check := range health.Checks {
		if check.Name == "scan_lock" {
			return check
		}
	}
	t.Fatalf("health has no scan_lock row: %+v", health.Checks)
	return HealthCheck{}
}

// foreignScanLock writes a scan.lock owned by another live process, as a
// concurrent scan worker leaves it, and returns a function that drains it.
func foreignScanLock(t *testing.T, root string) func() {
	t.Helper()
	owner := exec.Command("/bin/sleep", "60")
	if err := owner.Start(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "scan.lock")
	token := fmt.Sprintf("v1:%d:%032x", owner.Process.Pid, 1)
	if err := os.WriteFile(path, []byte(token), 0o600); err != nil {
		t.Fatal(err)
	}
	drained := false
	drain := func() {
		if drained {
			return
		}
		drained = true
		_ = os.Remove(path)
		_ = owner.Process.Kill()
		_ = owner.Wait()
	}
	t.Cleanup(drain)
	return drain
}

func servingFileDigest(t *testing.T, root string) [32]byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, servingFilename))
	if err != nil {
		t.Fatal(err)
	}
	return sha256.Sum256(raw)
}

// A scan worker publishes while it holds scan.lock. Its own lock is not
// contention with any other owner, so the publication must not tell the App
// to wait for a scan to finish: that warning would persist after the worker
// exits, because no later data change replaces the publication.
func TestServingPublicationIgnoresItsPublishersOwnScanLock(t *testing.T) {
	s, core, _ := newServingFixture(t)
	seedServingUsage(t, s, core)
	ctx := context.Background()
	baseline, err := s.PublishServingSnapshot(ctx, Request{WireVersion: 1, RecentLimit: DefaultRecentLimit}, false)
	if err != nil {
		t.Fatal(err)
	}
	lock, err := store.AcquireScanLock(ctx, s.StateRoot, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	published, err := s.PublishServingSnapshot(ctx, Request{WireVersion: 1, RecentLimit: DefaultRecentLimit}, false)
	if releaseErr := lock.Release(); err == nil {
		err = releaseErr
	}
	if err != nil {
		t.Fatal(err)
	}
	if check := scanLockCheck(t, published.Payload.Data.Health); check.Status != "ok" || check.Code != "" || check.Reason != nil {
		t.Fatalf("publisher reported its own scan lock: %+v", check)
	}
	if !reflect.DeepEqual(published.Payload.Data.Health, baseline.Payload.Data.Health) {
		t.Fatalf("health under the publisher's own lock = %+v, want the unlocked health %+v", published.Payload.Data.Health, baseline.Payload.Data.Health)
	}
}

// Lock rows describe who holds a lock now, not when the data was published.
// Reading a persisted publication re-evaluates them: a real owner that is
// still running stays visible, a drained owner no longer is (including a
// publication written by an older build), a newly started owner appears,
// and the persisted file is never rewritten by a read.
func TestServingReadReEvaluatesLockRowsOfAPersistedPublication(t *testing.T) {
	s, core, _ := newServingFixture(t)
	seedServingUsage(t, s, core)
	ctx := context.Background()
	request := Request{WireVersion: 1, RecentLimit: DefaultRecentLimit}

	drain := foreignScanLock(t, s.StateRoot)
	published, err := s.PublishServingSnapshot(ctx, request, false)
	if err != nil {
		t.Fatal(err)
	}
	if check := scanLockCheck(t, published.Payload.Data.Health); check.Status != "warning" || check.Code != string(store.LockReasonLive) {
		t.Fatalf("a running foreign worker was not reported: %+v", check)
	}
	live, err := s.ReadOrBootstrapServingSnapshot(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if check := scanLockCheck(t, live.Payload.Data.Health); check.Status != "warning" || check.Code != string(store.LockReasonLive) {
		t.Fatalf("an active foreign lock disappeared on read: %+v", check)
	}

	drain()
	persisted := servingFileDigest(t, s.StateRoot)
	drained, err := s.ReadOrBootstrapServingSnapshot(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if drained.Header.PublicationID != published.Header.PublicationID {
		t.Fatalf("read replaced the publication: %s, want %s", drained.Header.PublicationID, published.Header.PublicationID)
	}
	fresh, err := s.Build(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(drained.Payload.Data.Health, fresh.Snapshot.Health) {
		t.Fatalf("drained publication health = %+v, want current health %+v", drained.Payload.Data.Health, fresh.Snapshot.Health)
	}
	if servingFileDigest(t, s.StateRoot) != persisted {
		t.Fatal("reading a publication rewrote the persisted file")
	}

	foreignScanLock(t, s.StateRoot)
	started, err := s.ReadOrBootstrapServingSnapshot(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	check := scanLockCheck(t, started.Payload.Data.Health)
	if check.Status != "warning" || check.Code != string(store.LockReasonLive) || started.Payload.Data.Health.Healthy ||
		started.Payload.Data.Health.Problems != fresh.Snapshot.Health.Problems+1 || started.Payload.Data.Health.Warnings != fresh.Snapshot.Health.Warnings+1 {
		t.Fatalf("a newly started owner is not reported with consistent counts: %+v / %+v", check, started.Payload.Data.Health)
	}
}

func TestRefreshLockHealthPreservesUnrepresentedAndNonLockHealth(t *testing.T) {
	s, _, _ := newServingFixture(t)

	// 1. Available health with nonzero summary counts but empty/nil checks must not
	// be overwritten as healthy/0.
	emptyChecks := HealthSnapshot{
		Available: true,
		Status:    "unhealthy",
		Healthy:   false,
		Problems:  2,
		Warnings:  1,
		Errors:    1,
		Checks:    []HealthCheck{},
	}
	gotEmpty := emptyChecks
	s.refreshLockHealth(&gotEmpty)
	if !reflect.DeepEqual(gotEmpty, emptyChecks) {
		t.Fatalf("empty checks mutated health = %+v, want %+v", gotEmpty, emptyChecks)
	}

	nilChecks := HealthSnapshot{
		Available: true,
		Status:    "degraded",
		Healthy:   false,
		Problems:  1,
		Warnings:  1,
		Errors:    0,
		Checks:    nil,
	}
	gotNil := nilChecks
	s.refreshLockHealth(&gotNil)
	if !reflect.DeepEqual(gotNil, nilChecks) {
		t.Fatalf("nil checks mutated health = %+v, want %+v", gotNil, nilChecks)
	}

	// 2. Non-lock errors must remain unhealthy when represented lock rows change.
	reason := string(store.LockReasonLive)
	nonLockAndLock := HealthSnapshot{
		Available: true,
		Status:    "unhealthy",
		Healthy:   false,
		Problems:  2,
		Warnings:  1,
		Errors:    1,
		Checks: []HealthCheck{
			{Name: "database", Status: "error", Code: "database_unreadable"},
			{Name: "scan_lock", Status: "warning", Code: reason, Reason: &reason},
		},
	}
	s.refreshLockHealth(&nonLockAndLock)
	if nonLockAndLock.Healthy || nonLockAndLock.Status != "unhealthy" ||
		nonLockAndLock.Problems != 1 || nonLockAndLock.Warnings != 0 || nonLockAndLock.Errors != 1 {
		t.Fatalf("cleared lock alongside non-lock error = %+v, want unhealthy with 1 error", nonLockAndLock)
	}
	if nonLockAndLock.Checks[1].Status != "ok" {
		t.Fatalf("scan_lock row was not refreshed to ok: %+v", nonLockAndLock.Checks[1])
	}

	// Adding a foreign lock while non-lock error is present keeps unhealthy and
	// increments warnings/problems accurately.
	drain := foreignScanLock(t, s.StateRoot)
	s.refreshLockHealth(&nonLockAndLock)
	if nonLockAndLock.Healthy || nonLockAndLock.Status != "unhealthy" ||
		nonLockAndLock.Problems != 2 || nonLockAndLock.Warnings != 1 || nonLockAndLock.Errors != 1 {
		t.Fatalf("foreign lock alongside non-lock error = %+v, want unhealthy/2/1/1", nonLockAndLock)
	}
	if nonLockAndLock.Checks[1].Status != "warning" || nonLockAndLock.Checks[1].Code != reason {
		t.Fatalf("foreign scan_lock row = %+v, want warning/%s", nonLockAndLock.Checks[1], reason)
	}
	drain()

	// 3. Sparse checks with unrepresented error counts must preserve unrepresented counts
	// when the represented lock row clears.
	sparseWithUnrepresented := HealthSnapshot{
		Available: true,
		Status:    "unhealthy",
		Healthy:   false,
		Problems:  2,
		Warnings:  1,
		Errors:    1,
		Checks: []HealthCheck{
			{Name: "scan_lock", Status: "warning", Code: reason, Reason: &reason},
		},
	}
	s.refreshLockHealth(&sparseWithUnrepresented)
	if sparseWithUnrepresented.Healthy || sparseWithUnrepresented.Status != "unhealthy" ||
		sparseWithUnrepresented.Problems != 1 || sparseWithUnrepresented.Warnings != 0 || sparseWithUnrepresented.Errors != 1 {
		t.Fatalf("sparse checks lost unrepresented error: %+v", sparseWithUnrepresented)
	}

	// 4. Unknown non-ok check row must never become healthy when lock clears.
	unknownRow := HealthSnapshot{
		Available: true,
		Status:    "degraded",
		Healthy:   false,
		Problems:  1,
		Warnings:  1,
		Errors:    0,
		Checks: []HealthCheck{
			{Name: "unknown_check", Status: "unknown"},
			{Name: "scan_lock", Status: "warning", Code: reason, Reason: &reason},
		},
	}
	s.refreshLockHealth(&unknownRow)
	if unknownRow.Healthy || unknownRow.Status == "healthy" {
		t.Fatalf("unknown non-ok row fabricated healthy status: %+v", unknownRow)
	}
}
