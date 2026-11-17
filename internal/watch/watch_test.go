package watch

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
)

func TestPendingScanRetriesWithoutAdvancingItsFingerprint(t *testing.T) {
	attempts, releases := 0, 0
	persisted := map[string]string{}
	service := Service{
		Sources: SourceSet{
			{Domain: "session", Snapshot: func(context.Context) (string, error) { return "same", nil }, Scan: func(context.Context) (int, error) {
				attempts++
				if attempts == 1 {
					return 1, store.ErrTelemetrySourceIdentityPending
				}
				return 0, nil
			}},
			{Domain: "extension", Snapshot: func(context.Context) (string, error) { return "same", nil }, Scan: func(context.Context) (int, error) { return 2, nil }},
		},
		Lock: func(context.Context) (func() error, error) {
			return func() error { releases++; return nil }, nil
		},
		PersistFingerprint: func(_ context.Context, domain, value string) error {
			persisted[domain] = value
			return nil
		},
	}
	events, err := service.Poll(context.Background())
	if err != nil || len(events) != 2 {
		t.Fatalf("pending poll: %#v %v", events, err)
	}
	if events[0].Type != "scan_pending" || events[0].Reason != "source_identity_pending" || events[0].Changes != 1 || events[0].Skipped {
		t.Fatalf("pending event: %+v", events[0])
	}
	if _, advanced := persisted["session"]; advanced || persisted["extension"] != "same" {
		t.Fatalf("fingerprints: %#v", persisted)
	}
	events, err = service.Poll(context.Background())
	if err != nil || len(events) != 1 || events[0].Type != "scan_completed" || attempts != 2 || persisted["session"] != "same" || releases != 2 {
		t.Fatalf("recovery: %#v %v attempts=%d persisted=%v releases=%d", events, err, attempts, persisted, releases)
	}
	if events, err := service.Poll(context.Background()); err != nil || len(events) != 0 || attempts != 2 {
		t.Fatalf("unchanged after recovery: %#v %v attempts=%d", events, err, attempts)
	}
}

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

func TestPollPersistsAndRetainsPostScanFingerprint(t *testing.T) {
	scans := 0
	current := "pre-scan"
	persisted := ""
	service := Service{
		Sources: SourceSet{{
			Domain:   "session",
			Snapshot: func(context.Context) (string, error) { return current, nil },
			Scan: func(context.Context) (int, error) {
				scans++
				current = "post-scan"
				return 1, nil
			},
			PostScanFingerprint: func(_ context.Context, captured string) (string, error) {
				if captured != "pre-scan" {
					t.Fatalf("captured fingerprint=%q, want pre-scan", captured)
				}
				return current, nil
			},
		}},
		Lock: func(context.Context) (func() error, error) { return func() error { return nil }, nil },
		PersistFingerprint: func(_ context.Context, _ string, value string) error {
			persisted = value
			return nil
		},
	}
	if events, err := service.Poll(context.Background()); err != nil || len(events) != 1 {
		t.Fatalf("first Poll=%#v err=%v", events, err)
	}
	if persisted != "post-scan" {
		t.Fatalf("persisted fingerprint=%q, want post-scan", persisted)
	}
	if events, err := service.Poll(context.Background()); err != nil || len(events) != 0 || scans != 1 {
		t.Fatalf("second Poll=%#v scans=%d err=%v", events, scans, err)
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

func TestRunStopsCleanlyOnMidRoundCancellation(t *testing.T) {
	tests := []struct {
		name               string
		cancelIn           string
		returnContextError bool
	}{
		{name: "snapshot returns context error", cancelIn: "snapshot", returnContextError: true},
		{name: "snapshot ignores context", cancelIn: "snapshot", returnContextError: false},
		{name: "lock returns context error", cancelIn: "lock", returnContextError: true},
		{name: "scan returns context error", cancelIn: "scan", returnContextError: true},
		{name: "scan ignores context", cancelIn: "scan", returnContextError: false},
		{name: "post-scan fingerprint returns context error", cancelIn: "post_scan", returnContextError: true},
		{name: "post-scan fingerprint ignores context", cancelIn: "post_scan", returnContextError: false},
		{name: "persist fingerprint returns context error", cancelIn: "persist", returnContextError: true},
		{name: "persist fingerprint ignores context", cancelIn: "persist", returnContextError: false},
		{name: "release after completed round", cancelIn: "release"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			lock := newWatchTestLock(t, nil)
			lockCalls, scans, postScans, persists, emits := 0, 0, 0, 0, 0
			persisted := map[string]string{}

			service := Service{
				Sources: SourceSet{{
					Domain: "session",
					Snapshot: func(ctx context.Context) (string, error) {
						if test.cancelIn == "snapshot" {
							cancel()
							if test.returnContextError {
								return "", ctx.Err()
							}
						}
						return "changed", nil
					},
					Scan: func(ctx context.Context) (int, error) {
						lock.assertHeld("Scan")
						scans++
						if test.cancelIn == "scan" {
							cancel()
							if test.returnContextError {
								return 0, ctx.Err()
							}
						}
						return 1, nil
					},
					PostScanFingerprint: func(ctx context.Context, captured string) (string, error) {
						lock.assertHeld("PostScanFingerprint")
						postScans++
						if test.cancelIn == "post_scan" {
							cancel()
							if test.returnContextError {
								return "", ctx.Err()
							}
						}
						return captured + "-post", nil
					},
				}},
				Lock: func(ctx context.Context) (func() error, error) {
					lockCalls++
					if test.cancelIn == "lock" && lockCalls == 1 {
						cancel()
						return nil, ctx.Err()
					}
					release, err := lock.acquire(ctx)
					if err != nil {
						return nil, err
					}
					return func() error {
						err := release()
						if test.cancelIn == "release" {
							cancel()
						}
						return err
					}, nil
				},
				PersistFingerprint: func(ctx context.Context, domain, value string) error {
					lock.assertHeld("PersistFingerprint")
					persists++
					if test.cancelIn == "persist" {
						cancel()
						if test.returnContextError {
							return ctx.Err()
						}
					}
					persisted[domain] = value
					return nil
				},
			}

			err := service.Run(ctx, time.Hour, func(Event) error {
				emits++
				return nil
			})
			if err != nil {
				t.Fatalf("Run error = %v, want nil on mid-round cancellation", err)
			}
			completed := (test.cancelIn == "persist" && !test.returnContextError) || test.cancelIn == "release"
			wantEmits := 0
			if completed {
				wantEmits = 1
				if persisted["session"] != "changed-post" || service.fingerprints["session"] != "changed-post" {
					t.Fatalf("completed fingerprints = %v / %v, want changed-post", persisted, service.fingerprints)
				}
			}
			if emits != wantEmits {
				t.Fatalf("emits = %d, want %d; completed=%t", emits, wantEmits, completed)
			}
			if test.cancelIn == "snapshot" || test.cancelIn == "lock" {
				if scans != 0 || postScans != 0 || persists != 0 {
					t.Fatalf("unexpected work after %s cancellation: scans=%d postScans=%d persists=%d", test.cancelIn, scans, postScans, persists)
				}
				if lock.acquired != 0 || lock.released != 0 || lock.held {
					t.Fatalf("unexpected lock activity: acquired=%d held=%t released=%d", lock.acquired, lock.held, lock.released)
				}
			} else {
				lock.assertLifecycle()
				if test.cancelIn == "scan" && (postScans != 0 || persists != 0) {
					t.Fatalf("unexpected post-scan/persist after scan cancellation: postScans=%d persists=%d", postScans, persists)
				}
				if test.cancelIn == "post_scan" && persists != 0 {
					t.Fatalf("unexpected persist after post-scan cancellation: persists=%d", persists)
				}
			}
			if !completed {
				if len(persisted) != 0 || len(service.fingerprints) != 0 {
					t.Fatalf("fingerprints updated after cancellation: persisted=%v inMemory=%v", persisted, service.fingerprints)
				}
			}
			// A subsequent uncancelled Poll must still see the source as unacknowledged
			// unless PersistFingerprint already committed on disk before cancellation.
			if !completed {
				events, pollErr := service.Poll(context.Background())
				if pollErr != nil || len(events) != 1 || events[0].Type != "scan_completed" {
					t.Fatalf("follow-up Poll = %#v, %v, want 1 scan_completed event", events, pollErr)
				}
			}
		})
	}
}

func TestRunPreservesErrorsWhenCancelledMidRound(t *testing.T) {
	independentErr := errors.New("independent scan failure")
	releaseErr := errors.New("release lock failure")
	emitErr := errors.New("emit failure")

	t.Run("independent scan error preserved even when ctx cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		lock := newWatchTestLock(t, nil)
		service := Service{
			Sources: SourceSet{{
				Domain:   "session",
				Snapshot: func(context.Context) (string, error) { return "changed", nil },
				Scan: func(context.Context) (int, error) {
					cancel()
					return 0, independentErr
				},
			}},
			Lock: lock.acquire,
		}
		err := service.Run(ctx, time.Hour, func(Event) error {
			t.Fatal("emit should not be called")
			return nil
		})
		if !errors.Is(err, independentErr) {
			t.Fatalf("Run error = %v, want %v", err, independentErr)
		}
		lock.assertLifecycle()
	})

	t.Run("lock release failure preserved on mid-round cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		lock := newWatchTestLock(t, releaseErr)
		service := Service{
			Sources: SourceSet{{
				Domain:   "session",
				Snapshot: func(context.Context) (string, error) { return "changed", nil },
				Scan: func(ctx context.Context) (int, error) {
					cancel()
					return 0, ctx.Err()
				},
			}},
			Lock: lock.acquire,
		}
		err := service.Run(ctx, time.Hour, func(Event) error {
			t.Fatal("emit should not be called")
			return nil
		})
		if !errors.Is(err, releaseErr) {
			t.Fatalf("Run error = %v, want release error %v", err, releaseErr)
		}
		lock.assertLifecycle()
	})

	t.Run("emit error preserved even when ctx cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		lock := newWatchTestLock(t, nil)
		service := Service{
			Sources: SourceSet{{
				Domain:   "usage",
				Snapshot: func(context.Context) (string, error) { return "changed", nil },
				Scan:     func(context.Context) (int, error) { return 1, nil },
			}},
			Lock: lock.acquire,
		}
		err := service.Run(ctx, time.Hour, func(Event) error {
			cancel()
			return emitErr
		})
		if !errors.Is(err, emitErr) {
			t.Fatalf("Run error = %v, want emit error %v", err, emitErr)
		}
		lock.assertLifecycle()
	})
}

func TestRunPreservesWrappedCancellationAndIndependentErrors(t *testing.T) {
	for _, stage := range []string{"snapshot", "scan", "post_scan", "persist"} {
		for _, shape := range []string{"join", "multi_wrap", "wrapped_join"} {
			t.Run(stage+"/"+shape, func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				independent := errors.New("independent operation failure")
				fail := func() error {
					cancel()
					switch shape {
					case "join":
						return errors.Join(independent, ctx.Err())
					case "multi_wrap":
						return fmt.Errorf("%w: %w", independent, ctx.Err())
					default:
						return fmt.Errorf("operation: %w", errors.Join(independent, ctx.Err()))
					}
				}
				lock := newWatchTestLock(t, nil)
				service := Service{
					Sources: SourceSet{{
						Domain: "session",
						Snapshot: func(context.Context) (string, error) {
							if stage == "snapshot" {
								return "", fail()
							}
							return "changed", nil
						},
						Scan: func(context.Context) (int, error) {
							if stage == "scan" {
								return 0, fail()
							}
							return 1, nil
						},
						PostScanFingerprint: func(context.Context, string) (string, error) {
							if stage == "post_scan" {
								return "", fail()
							}
							return "complete", nil
						},
					}},
					Lock: lock.acquire,
					PersistFingerprint: func(context.Context, string, string) error {
						if stage == "persist" {
							return fail()
						}
						return nil
					},
				}
				err := service.Run(ctx, time.Hour, func(Event) error { t.Fatal("unexpected completion event"); return nil })
				if !errors.Is(err, independent) || !errors.Is(err, context.Canceled) {
					t.Fatalf("Run error = %v, want both independent failure and cancellation", err)
				}
				if stage != "snapshot" {
					lock.assertLifecycle()
				}
			})
		}
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
