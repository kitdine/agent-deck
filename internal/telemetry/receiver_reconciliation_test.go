package telemetry

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
)

func TestServeReconcilesNewBackgroundUsageAndPublishesConflictRetractions(t *testing.T) {
	database, root := openStore(t)
	if err := SaveConfig(root, Config{Enabled: true, Port: freePort(t), RetentionDays: 7}); err != nil {
		t.Fatal(err)
	}
	var clock atomic.Int64
	clock.Store(receivedAt.UnixNano())
	var inventories atomic.Int32
	ready, published := make(chan string, 1), make(chan struct{}, 4)
	done := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		done <- Serve(ctx, ServeOptions{
			StateRoot: root, Home: t.TempDir(), Store: database,
			Now: func() time.Time { return time.Unix(0, clock.Load()).UTC() }, SettleEvery: 10 * time.Millisecond,
			Ready: func(addr string) { ready <- addr }, Published: func() { published <- struct{}{} },
			Inventory: func(ctx context.Context) error {
				inventories.Add(1)
				if err := database.AcceptInventory(ctx, "after-receipt", 1, nil, receivedAt.Add(time.Second)); err != nil {
					return err
				}
				if err := database.FinishInventory(ctx, "after-receipt", "complete"); err != nil {
					return err
				}
				return nil
			},
		})
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(5 * time.Second):
			t.Error("receiver did not stop")
		}
	})
	var addr string
	select {
	case addr = <-ready:
	case err := <-done:
		t.Fatal(err)
	case <-time.After(5 * time.Second):
		t.Fatal("receiver did not start")
	}
	token, err := ReadToken(root)
	if err != nil {
		t.Fatal(err)
	}
	post := func(output string) {
		t.Helper()
		request, _ := http.NewRequest(http.MethodPost, "http://"+addr+LogsPath, strings.NewReader(exportBody(codexResource, completed("finite-background", "1000", output))))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set(TokenHeader, token)
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Fatalf("receipt status %d", response.StatusCode)
		}
	}
	waitPublication := func() {
		t.Helper()
		select {
		case <-published:
		case <-time.After(5 * time.Second):
			t.Fatal("counted change was not published")
		}
	}
	post("10")
	// Inventory completion requests a publication before the grace period.
	// Advance the business clock only after observing that acknowledgement,
	// so it cannot race the settlement ticker and leave a queued notification.
	waitPublication()
	assertOwnership(t, database, "finite-background", ownership{states: map[string]int{"pending": 1}})
	clock.Store(receivedAt.Add(SettleAfter + time.Second).UnixNano())
	waitPublication()
	assertOwnership(t, database, "finite-background", ownership{otelEvents: 1, otelTokens: 1010, usageSessions: 1, markers: 1, states: map[string]int{"otel_owned": 1}})
	if needed, err := database.TelemetryNeedsInventory(ctx); err != nil || needed || inventories.Load() != 1 {
		t.Fatalf("finite reconciliation needed=%v calls=%d err=%v", needed, inventories.Load(), err)
	}
	index, err := store.OpenSessions(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	defer index.Close()
	var projected int
	if err := index.DB.QueryRow(`SELECT count(*) FROM session_sources WHERE substr(source_path,1,5)='otel:'`).Scan(&projected); err != nil || projected != 1 {
		t.Fatalf("background projection=%d err=%v", projected, err)
	}
	post("11")
	waitPublication()
	assertOwnership(t, database, "finite-background", ownership{states: map[string]int{"conflict": 2}})
	if err := index.DB.QueryRow(`SELECT count(*) FROM session_sources WHERE substr(source_path,1,5)='otel:'`).Scan(&projected); err != nil || projected != 0 {
		t.Fatalf("retracted session still projected=%d err=%v", projected, err)
	}
}

func TestReconcilerRetriesNormalizationProjectionAfterFailure(t *testing.T) {
	database, _ := openStore(t)
	ownedConversation(t, database, "conflict-projection")
	journal := &Journal{Dir: t.TempDir(), SegmentBytes: DefaultSegmentBytes, Cap: DefaultJournalCap, Now: func() time.Time { return receivedAt }}
	records, err := Decode([]byte(exportBody(codexResource, completed("conflict-projection", "1000", "11"))))
	if err != nil {
		t.Fatal(err)
	}
	if err := journal.Append(records); err != nil {
		t.Fatal(err)
	}
	calls := 0
	r := &Reconciler{Store: database, Journal: journal, Now: func() time.Time { return receivedAt.Add(time.Hour) }, Batch: 10,
		SessionsChanged: func(context.Context) error {
			calls++
			if calls == 1 {
				return errors.New("synthetic projection failure")
			}
			return nil
		}}
	if result, err := r.Drain(context.Background()); err != nil || !result.Changed {
		t.Fatalf("drain retraction=%+v err=%v", result, err)
	}
	if _, err := r.Settle(context.Background()); err == nil {
		t.Fatal("projection failure hidden")
	}
	if result, err := r.Settle(context.Background()); err != nil || !result.Changed || calls != 2 {
		t.Fatalf("projection not retried: %+v calls=%d err=%v", result, calls, err)
	}
	if result, err := r.Settle(context.Background()); err != nil || result.Changed || calls != 2 {
		t.Fatalf("successful projection repeated: %+v calls=%d err=%v", result, calls, err)
	}
}
