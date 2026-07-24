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

func TestPollAlwaysReleasesLock(t *testing.T) {
	scanErr := errors.New("scan failed")
	persistErr := errors.New("persist failed")
	laterScanErr := errors.New("later scan failed")
	tests := []struct {
		name             string
		sourceCount      int
		persist          bool
		scanErrorAt      int
		persistErrorAt   int
		wantErr          error
		wantEvents       int
		wantScans        int
		wantPersistCalls int
	}{
		{name: "success without fingerprint persistence", sourceCount: 1, scanErrorAt: -1, persistErrorAt: -1, wantEvents: 1, wantScans: 1},
		{name: "success with fingerprint persistence", sourceCount: 1, persist: true, scanErrorAt: -1, persistErrorAt: -1, wantEvents: 1, wantScans: 1, wantPersistCalls: 1},
		{name: "two source success", sourceCount: 2, persist: true, scanErrorAt: -1, persistErrorAt: -1, wantEvents: 2, wantScans: 2, wantPersistCalls: 2},
		{name: "scan error", sourceCount: 1, persist: true, scanErrorAt: 0, persistErrorAt: -1, wantErr: scanErr, wantScans: 1},
		{name: "fingerprint persistence error", sourceCount: 1, persist: true, scanErrorAt: -1, persistErrorAt: 0, wantErr: persistErr, wantScans: 1, wantPersistCalls: 1},
		{name: "later source failure", sourceCount: 2, persist: true, scanErrorAt: 1, persistErrorAt: -1, wantErr: laterScanErr, wantScans: 2, wantPersistCalls: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			lock := newWatchTestLock(t, nil)
			scans, persists := 0, 0
			sources := make(SourceSet, test.sourceCount)
			for index := range sources {
				index := index
				domain := "source-" + string(rune('a'+index))
				sources[index] = Source{
					Domain:   domain,
					Snapshot: func(context.Context) (string, error) { return domain + "-changed", nil },
					Scan: func(context.Context) (int, error) {
						lock.assertHeld("Scan")
						scans++
						if index == test.scanErrorAt {
							if index == 0 {
								return 0, scanErr
							}
							return 0, laterScanErr
						}
						return 1, nil
					},
				}
			}
			service := Service{
				Sources: sources,
				Lock:    lock.acquire,
			}
			if test.persist {
				service.PersistFingerprint = func(context.Context, string, string) error {
					lock.assertHeld("PersistFingerprint")
					call := persists
					persists++
					if call == test.persistErrorAt {
						return persistErr
					}
					return nil
				}
			}

			events, err := service.Poll(context.Background())
			if test.wantErr != nil {
				if !errors.Is(err, test.wantErr) {
					t.Fatalf("Poll error = %v, want %v", err, test.wantErr)
				}
				if events != nil {
					t.Fatalf("Poll events = %#v, want nil on failure", events)
				}
			} else if err != nil || len(events) != test.wantEvents {
				t.Fatalf("Poll = %#v, %v, want %d successful events", events, err, test.wantEvents)
			}
			if scans != test.wantScans || persists != test.wantPersistCalls {
				t.Fatalf("calls: scans=%d persists=%d, want %d and %d", scans, persists, test.wantScans, test.wantPersistCalls)
			}
			lock.assertLifecycle()
		})
	}
}

func TestPollPreservesReleaseError(t *testing.T) {
	scanErr := errors.New("scan failed")
	persistErr := errors.New("persist failed")
	releaseErr := errors.New("release failed")
	tests := []struct {
		name           string
		scanErr        error
		persistErr     error
		operationalErr error
		wantPersists   int
	}{
		{name: "otherwise successful work and release error", wantPersists: 1},
		{name: "scan and release errors", scanErr: scanErr, operationalErr: scanErr},
		{name: "persistence and release errors", persistErr: persistErr, operationalErr: persistErr, wantPersists: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			lock := newWatchTestLock(t, releaseErr)
			scans, persists := 0, 0
			service := Service{
				Sources: SourceSet{{
					Domain:   "session",
					Snapshot: func(context.Context) (string, error) { return "changed", nil },
					Scan: func(context.Context) (int, error) {
						lock.assertHeld("Scan")
						scans++
						return 1, test.scanErr
					},
				}},
				Lock: lock.acquire,
				PersistFingerprint: func(context.Context, string, string) error {
					lock.assertHeld("PersistFingerprint")
					persists++
					return test.persistErr
				},
			}

			events, err := service.Poll(context.Background())
			if events != nil {
				t.Fatalf("Poll events = %#v, want nil on combined failure", events)
			}
			if !errors.Is(err, releaseErr) {
				t.Fatalf("Poll error = %v, want release error", err)
			}
			if test.operationalErr != nil && !errors.Is(err, test.operationalErr) {
				t.Fatalf("Poll error = %v, want operational error %v", err, test.operationalErr)
			}
			if scans != 1 || persists != test.wantPersists {
				t.Fatalf("calls: scans=%d persists=%d, want 1 and %d", scans, persists, test.wantPersists)
			}
			lock.assertLifecycle()
		})
	}
}

func TestRunStopsPromptlyOnCancellationWithoutAnotherPollOrEmit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	snapshots, scans, emits := 0, 0, 0
	service := Service{
		Sources: SourceSet{{
			Domain: "session",
			Snapshot: func(context.Context) (string, error) {
				snapshots++
				return "changed", nil
			},
			Scan: func(context.Context) (int, error) {
				scans++
				return 1, nil
			},
		}},
		Lock: func(context.Context) (func() error, error) {
			return func() error { return nil }, nil
		},
	}
	emitted := make(chan Event, 2)
	result := make(chan error, 1)
	go func() {
		result <- service.Run(ctx, time.Hour, func(event Event) error {
			emits++
			emitted <- event
			return nil
		})
	}()

	select {
	case event := <-emitted:
		if event.Type != "scan_completed" || event.Domain != "session" {
			t.Fatalf("initial event = %#v", event)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not emit the initial event")
	}
	cancel()
	select {
	case err := <-result:
		if err != nil {
			t.Fatalf("Run cancellation error = %v, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not stop promptly after cancellation")
	}
	if snapshots != 1 || scans != 1 || emits != 1 {
		t.Fatalf("calls after cancellation: snapshots=%d scans=%d emits=%d, want 1 each", snapshots, scans, emits)
	}
	select {
	case event := <-emitted:
		t.Fatalf("unexpected event after cancellation: %#v", event)
	default:
	}
}

func TestRunStopsOnEmitError(t *testing.T) {
	emitErr := errors.New("emit failed")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	snapshots, scans, emits := 0, 0, 0
	service := Service{
		Sources: SourceSet{{
			Domain: "usage",
			Snapshot: func(context.Context) (string, error) {
				snapshots++
				return "changed", nil
			},
			Scan: func(context.Context) (int, error) {
				scans++
				return 1, nil
			},
		}},
		Lock: func(context.Context) (func() error, error) {
			return func() error { return nil }, nil
		},
	}
	result := make(chan error, 1)
	go func() {
		result <- service.Run(ctx, time.Hour, func(Event) error {
			emits++
			return emitErr
		})
	}()

	select {
	case err := <-result:
		if err != emitErr {
			t.Fatalf("Run error = %v, want emit error", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not stop promptly after emit failure")
	}
	if snapshots != 1 || scans != 1 || emits != 1 {
		t.Fatalf("calls after emit failure: snapshots=%d scans=%d emits=%d, want 1 each", snapshots, scans, emits)
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

func TestFingerprintRootsTracksDisappearanceAndReappearance(t *testing.T) {
	root := filepath.Join(t.TempDir(), "watched")
	missing, err := FingerprintRoots(root)
	if err != nil {
		t.Fatal(err)
	}
	missingAgain, err := FingerprintRoots(root)
	if err != nil {
		t.Fatal(err)
	}
	if missing == "" || missingAgain != missing {
		t.Fatalf("missing fingerprints = %q, %q, want equal non-empty values", missing, missingAgain)
	}

	if err = os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "source.jsonl")
	if err = os.WriteFile(path, []byte("synthetic"), 0o600); err != nil {
		t.Fatal(err)
	}
	present, err := FingerprintRoots(root)
	if err != nil {
		t.Fatal(err)
	}
	if present == missing {
		t.Fatalf("present fingerprint = missing fingerprint %q", present)
	}

	if err = os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(root); err != nil {
		t.Fatal(err)
	}
	disappeared, err := FingerprintRoots(root)
	if err != nil {
		t.Fatal(err)
	}
	if disappeared != missing {
		t.Fatalf("disappeared fingerprint = %q, want original missing fingerprint %q", disappeared, missing)
	}
}

func TestFingerprintRootsHandlesBrokenAndCyclicSkillLinks(t *testing.T) {
	root := t.TempDir()
	link := filepath.Join(root, "skill-link")
	if err := os.Symlink(filepath.Join(root, "missing"), link); err != nil {
		t.Fatal(err)
	}
	broken, err := FingerprintRoots(root)
	if err != nil || broken == "" {
		t.Fatalf("broken link fingerprint = %q, %v", broken, err)
	}
	if err = os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink(link, link); err != nil {
		t.Fatal(err)
	}
	cyclic, err := FingerprintRoots(root)
	if err != nil || cyclic == "" {
		t.Fatalf("cyclic link fingerprint = %q, %v", cyclic, err)
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

type watchTestLock struct {
	t          *testing.T
	acquired   int
	held       bool
	released   int
	releaseErr error
}

func newWatchTestLock(t *testing.T, releaseErr error) *watchTestLock {
	t.Helper()
	return &watchTestLock{t: t, releaseErr: releaseErr}
}

func (l *watchTestLock) acquire(context.Context) (func() error, error) {
	l.t.Helper()
	if l.held {
		l.t.Fatal("lock acquired while already held")
	}
	l.acquired++
	l.held = true
	return func() error {
		l.t.Helper()
		if !l.held {
			l.t.Fatal("lock released while not held")
		}
		l.released++
		l.held = false
		return l.releaseErr
	}, nil
}

func (l *watchTestLock) assertHeld(operation string) {
	l.t.Helper()
	if !l.held {
		l.t.Fatalf("%s called without the scan lock held", operation)
	}
}

func (l *watchTestLock) assertLifecycle() {
	l.t.Helper()
	if l.acquired != 1 || l.released != 1 || l.held {
		l.t.Fatalf("lock state: acquired=%d held=%t released=%d, want 1 false 1", l.acquired, l.held, l.released)
	}
}
