package watch

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
)

func TestPollSkipsUnchangedSourcesWithoutScanning(t *testing.T) {
	scans := 0
	service := Service{
		Sources: SourceSet{{Domain: "usage", Snapshot: func(context.Context) (string, error) { return "same", nil }, Scan: func(context.Context) (int, error) { scans++; return 2, nil }}},
		Lock:    func(context.Context) (func() error, error) { return func() error { return nil }, nil },
		Now:     func() time.Time { return time.Unix(1, 0) },
	}
	first, err := service.Poll(context.Background())
	if err != nil || len(first) != 1 || scans != 1 {
		t.Fatalf("first Poll = %#v, scans=%d, err=%v", first, scans, err)
	}
	second, err := service.Poll(context.Background())
	if err != nil || len(second) != 0 || scans != 1 {
		t.Fatalf("unchanged Poll = %#v, scans=%d, err=%v", second, scans, err)
	}
}

func TestPollUsesPersistedFingerprintsAfterRestartWithoutWriting(t *testing.T) {
	scans, persists := 0, 0
	service := Service{
		InitialFingerprints: map[string]string{"extension": "stable"},
		Sources:             SourceSet{{Domain: "extension", Snapshot: func(context.Context) (string, error) { return "stable", nil }, Scan: func(context.Context) (int, error) { scans++; return 0, nil }}},
		Lock: func(context.Context) (func() error, error) {
			t.Fatal("unchanged source acquired scan lock")
			return nil, nil
		},
		PersistFingerprint: func(context.Context, string, string) error { persists++; return nil },
	}
	events, err := service.Poll(context.Background())
	if err != nil || len(events) != 0 || scans != 0 || persists != 0 {
		t.Fatalf("restart Poll = %#v scans=%d persists=%d err=%v", events, scans, persists, err)
	}
}

func TestPollReportsBusyWithoutScanning(t *testing.T) {
	scans := 0
	service := Service{
		Sources: SourceSet{{Domain: "session", Snapshot: func(context.Context) (string, error) { return "changed", nil }, Scan: func(context.Context) (int, error) { scans++; return 0, nil }}},
		Lock:    func(context.Context) (func() error, error) { return nil, store.ErrStateBusy },
	}
	events, err := service.Poll(context.Background())
	if err != nil || scans != 0 || len(events) != 1 || !events[0].Skipped || events[0].Reason != "state_busy" || events[0].SchemaVersion != 1 {
		t.Fatalf("Poll = %#v, scans=%d, err=%v", events, scans, err)
	}
}

func TestFailedScanDoesNotAdvanceFingerprint(t *testing.T) {
	attempts := 0
	service := Service{
		Sources: SourceSet{{Domain: "extension", Snapshot: func(context.Context) (string, error) { return "changed", nil }, Scan: func(context.Context) (int, error) { attempts++; return 0, errors.New("scan failed") }}},
		Lock:    func(context.Context) (func() error, error) { return func() error { return nil }, nil },
	}
	for range 2 {
		if _, err := service.Poll(context.Background()); err == nil {
			t.Fatal("Poll succeeded")
		}
	}
	if attempts != 2 {
		t.Fatalf("scan attempts = %d, want 2", attempts)
	}
}

func TestFingerprintRootsChangesWithMetadata(t *testing.T) {
	root := t.TempDir()
	before, err := FingerprintRoots(root)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "source.jsonl")
	if err = os.WriteFile(path, []byte("synthetic"), 0o600); err != nil {
		t.Fatal(err)
	}
	after, err := FingerprintRoots(root)
	if err != nil {
		t.Fatal(err)
	}
	if before == after {
		t.Fatal("fingerprint did not change")
	}
}

func TestScanLockDoesNotBlockStateLock(t *testing.T) {
	root := t.TempDir()
	scan, err := store.AcquireScanLock(t.Context(), root, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer scan.Release()
	state, err := store.AcquireLock(t.Context(), root, 0)
	if err != nil {
		t.Fatalf("scan lock blocked state lock: %v", err)
	}
	if err = state.Release(); err != nil {
		t.Fatal(err)
	}
	if _, err = store.AcquireScanLock(t.Context(), root, 0); !errors.Is(err, store.ErrStateBusy) {
		t.Fatalf("second scan lock error = %v", err)
	}
}
