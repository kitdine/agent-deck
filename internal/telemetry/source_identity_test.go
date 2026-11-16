package telemetry

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
)

func TestPreparedRetractionProjectsAndRetries(t *testing.T) {
	db, root := openStore(t)
	ctx := context.Background()
	thread := "019a1234-abcd-7000-8000-000000000001"
	ownedConversation(t, db, thread)
	index, err := store.OpenSessions(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	defer index.Close()
	if err = ProjectSessionIndex(ctx, db, index); err != nil {
		t.Fatal(err)
	}
	prepares, calls := 0, 0
	r := &Reconciler{Store: db, Now: func() time.Time { return receivedAt.Add(time.Hour) }, PrepareSources: func(ctx context.Context) (bool, error) {
		prepares++
		if prepares != 1 {
			return false, nil
		}
		tx, err := db.DB.BeginTx(ctx, nil)
		if err != nil {
			return false, err
		}
		defer tx.Rollback()
		n, err := store.RetractTelemetryForJSONLThreads(ctx, tx, []string{thread}, receivedAt.Add(time.Hour))
		if err != nil {
			return false, err
		}
		return n > 0, tx.Commit()
	}, SessionsChanged: func(ctx context.Context) error {
		calls++
		if calls == 1 {
			return errors.New("projection unavailable")
		}
		return ProjectSessionIndex(ctx, db, index)
	}}
	if _, err = r.Settle(ctx); err == nil {
		t.Fatal("lost projection failure")
	}
	if result, err := r.Settle(ctx); err != nil || !result.Changed || calls != 2 {
		t.Fatalf("retry %+v %v calls=%d", result, err, calls)
	}
	var n int
	if err = index.DB.QueryRow(`SELECT count(*) FROM session_sources WHERE source_path LIKE 'otel:%'`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("stale projection %d %v", n, err)
	}
}

func TestSettleAndProjectReportsPreparationIncomplete(t *testing.T) {
	db, root := openStore(t)
	ctx := context.Background()
	index, err := store.OpenSessions(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	defer index.Close()
	called := false
	changed, err := SettleAndProject(ctx, db, index, receivedAt, func(context.Context) (bool, error) {
		called = true
		return true, store.ErrTelemetrySourceIdentityPending
	})
	if !called || !changed || !errors.Is(err, store.ErrTelemetrySourceIdentityPending) {
		t.Fatalf("called=%v changed=%v err=%v", called, changed, err)
	}
}

func TestCommittedPreparationCancellationRecoversProjection(t *testing.T) {
	for _, entry := range []string{"reconciler retry", "fresh projection entry"} {
		t.Run(entry, func(t *testing.T) {
			db, root := openStore(t)
			thread := "019a1234-abcd-7000-8000-000000000002"
			ownedConversation(t, db, thread)
			index, err := store.OpenSessions(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			defer index.Close()
			if err := ProjectSessionIndex(context.Background(), db, index); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			prepared := false
			prepare := func(ctx context.Context) (bool, error) {
				if prepared {
					return false, nil
				}
				tx, err := db.DB.BeginTx(ctx, nil)
				if err != nil {
					return false, err
				}
				defer tx.Rollback()
				n, err := store.RetractTelemetryForJSONLThreads(ctx, tx, []string{thread}, receivedAt.Add(time.Hour))
				if err != nil {
					return false, err
				}
				if err := tx.Commit(); err != nil {
					return false, err
				}
				prepared = true
				cancel()
				return n > 0, nil
			}
			callbacks := 0
			r := &Reconciler{
				Store: db, Now: func() time.Time { return receivedAt.Add(time.Hour) }, PrepareSources: prepare,
				SessionsChanged: func(ctx context.Context) error {
					callbacks++
					return ProjectSessionIndex(ctx, db, index)
				},
			}
			var changed bool
			if entry == "reconciler retry" {
				var result store.TelemetrySettleResult
				result, err = r.Settle(ctx)
				changed = result.Changed
			} else {
				changed, err = SettleAndProject(ctx, db, index, receivedAt.Add(time.Hour), prepare)
			}
			if !errors.Is(err, context.Canceled) || !changed {
				t.Errorf("committed preparation: changed=%v error=%v", changed, err)
			}
			if entry == "reconciler retry" {
				result, retryErr := r.Settle(context.Background())
				if retryErr != nil || !result.Changed || callbacks != 1 {
					t.Errorf("retry: result=%+v error=%v callbacks=%d", result, retryErr, callbacks)
				}
			} else {
				// The scan/rebuild entry has no reconciler memory. It repairs the
				// persisted stale projection even after preparation is complete.
				if _, err := SettleAndProject(context.Background(), db, index, receivedAt.Add(time.Hour), nil); err != nil {
					t.Fatal(err)
				}
			}
			var remaining int
			if err := index.DB.QueryRow(`SELECT count(*) FROM session_sources WHERE source_path LIKE 'otel:%'`).Scan(&remaining); err != nil || remaining != 0 {
				t.Fatalf("stale projections=%d error=%v", remaining, err)
			}
			if err := db.DB.QueryRow(`SELECT count(*) FROM telemetry_contributions`).Scan(&remaining); err != nil || remaining != 0 {
				t.Fatalf("restored contributions=%d error=%v", remaining, err)
			}
		})
	}
}

func TestReceiverRetriesSourceIdentityAndClearsDiagnostic(t *testing.T) {
	db, root := openStore(t)
	if err := SaveConfig(root, Config{Enabled: true, Port: freePort(t), RetentionDays: 7}); err != nil {
		t.Fatal(err)
	}
	var recover atomic.Bool
	ready := make(chan string, 1)
	done := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(5 * time.Second):
			t.Error("receiver did not stop")
		}
	}()
	go func() {
		done <- Serve(ctx, ServeOptions{StateRoot: root, Home: t.TempDir(), Store: db, SettleEvery: 10 * time.Millisecond, Ready: func(addr string) { ready <- addr }, PrepareSources: func(context.Context) (bool, error) {
			if !recover.Load() {
				return false, store.ErrTelemetrySourceIdentityPending
			}
			return false, nil
		}})
	}()
	select {
	case <-ready:
	case <-time.After(5 * time.Second):
		t.Fatal("receiver did not start")
	}
	wait := func(want string) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			state, err := ReadListenerState(root)
			if err == nil && state.Error == want {
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
		t.Fatalf("listener never reached %q", want)
	}
	wait(ListenerErrorSourceIdentityPending)
	recover.Store(true)
	wait("")
}

func TestReceiverPublishesCommittedRetractionAfterPreparationError(t *testing.T) {
	for _, name := range []string{"preparation error", "preparation and projection errors"} {
		t.Run(name, func(t *testing.T) {
			db, root := openStore(t)
			thread := "019a1234-abcd-7000-8000-000000000003"
			ownedConversation(t, db, thread)
			index, err := store.OpenSessions(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			defer index.Close()
			if err := ProjectSessionIndex(context.Background(), db, index); err != nil {
				t.Fatal(err)
			}
			if name == "preparation and projection errors" {
				if _, err := index.DB.Exec(`CREATE TRIGGER fail_projection BEFORE DELETE ON session_sources
 WHEN substr(OLD.source_path,1,5)='otel:' BEGIN SELECT RAISE(ABORT,'projection unavailable'); END`); err != nil {
					t.Fatal(err)
				}
			}
			if err := SaveConfig(root, Config{Enabled: true, Port: freePort(t), RetentionDays: 7}); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan error, 1)
			defer func() {
				cancel()
				select {
				case err := <-done:
					if err != nil {
						t.Error(err)
					}
				case <-time.After(5 * time.Second):
					t.Error("receiver did not stop")
				}
			}()
			var prepares atomic.Int32
			published := make(chan struct{}, 1)
			home := t.TempDir()
			go func() {
				done <- Serve(ctx, ServeOptions{
					StateRoot: root, Home: home, Store: db, SettleEvery: 10 * time.Millisecond,
					PrepareSources: func(ctx context.Context) (bool, error) {
						switch prepares.Add(1) {
						case 1:
							tx, err := db.DB.BeginTx(ctx, nil)
							if err != nil {
								return false, err
							}
							defer tx.Rollback()
							n, err := store.RetractTelemetryForJSONLThreads(ctx, tx, []string{thread}, receivedAt.Add(time.Hour))
							if err != nil {
								return false, err
							}
							if err := tx.Commit(); err != nil {
								return false, err
							}
							return n > 0, errors.New("later source query failed")
						case 2:
							if _, err := index.DB.ExecContext(ctx, `DROP TRIGGER IF EXISTS fail_projection`); err != nil {
								return false, err
							}
							return false, errors.New("source query still unavailable")
						default:
							return false, nil
						}
					},
					Published: func() {
						select {
						case published <- struct{}{}:
						default:
						}
					},
				})
			}()
			select {
			case <-published:
				if prepares.Load() < 3 {
					t.Fatal("published before ordinary preparation error recovered")
				}
			case <-time.After(5 * time.Second):
				t.Fatalf("committed retraction never published; prepares=%d", prepares.Load())
			}
			var remaining int
			if err := index.DB.QueryRow(`SELECT count(*) FROM session_sources WHERE source_path LIKE 'otel:%'`).Scan(&remaining); err != nil || remaining != 0 {
				t.Fatalf("stale projections=%d error=%v", remaining, err)
			}
			if err := db.DB.QueryRow(`SELECT count(*) FROM telemetry_contributions`).Scan(&remaining); err != nil || remaining != 0 {
				t.Fatalf("remaining contributions=%d error=%v", remaining, err)
			}
		})
	}
}

func TestServeClassificationRetriesBeforePublication(t *testing.T) {
	db, root := openStore(t)
	ownedConversation(t, db, "classification-retry")
	if err := SaveConfig(root, Config{Enabled: true, Port: freePort(t), RetentionDays: 7}); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	failed, published := make(chan struct{}, 1), make(chan struct{}, 1)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- Serve(ctx, ServeOptions{StateRoot: root, Home: t.TempDir(), Store: db, SettleEvery: 10 * time.Millisecond, Now: func() time.Time { return receivedAt.Add(time.Hour) }, ClassifySessions: func(ctx context.Context, index *store.Store) error {
			var n int
			if err := index.DB.QueryRowContext(ctx, `SELECT count(*) FROM session_sources WHERE source_path LIKE 'otel:%'`).Scan(&n); err != nil {
				return err
			}
			if n != 1 {
				return errors.New("classification ran before projection")
			}
			if calls.Add(1) == 1 {
				failed <- struct{}{}
				return errors.New("classification unavailable")
			}
			return nil
		}, Published: func() {
			select {
			case published <- struct{}{}:
			default:
			}
		}})
	}()
	defer func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(5 * time.Second):
			t.Error("receiver did not stop")
		}
	}()
	select {
	case <-failed:
	case <-time.After(5 * time.Second):
		t.Fatal("initial classification not attempted")
	}
	select {
	case <-published:
	case <-time.After(5 * time.Second):
		t.Fatal("classification retry not published")
	}
	if calls.Load() < 2 {
		t.Fatal("failure not retried")
	}
}
