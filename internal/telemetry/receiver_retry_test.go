package telemetry

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"modernc.org/sqlite"
)

var normalizationFaultSequence atomic.Uint64

func TestServeRetriesFailedNormalizationWithoutAnotherDelivery(t *testing.T) {
	var attempts atomic.Int32
	var fail atomic.Bool
	fail.Store(true)
	attempted := make(chan struct{}, 1)
	function := fmt.Sprintf("normalization_fault_%d", normalizationFaultSequence.Add(1))
	if err := sqlite.RegisterScalarFunction(function, 0, func(*sqlite.FunctionContext, []driver.Value) (driver.Value, error) {
		attempts.Add(1)
		if fail.Load() {
			select {
			case attempted <- struct{}{}:
			default:
			}
			return nil, errors.New("temporary normalization failure")
		}
		return nil, nil
	}); err != nil {
		t.Fatal(err)
	}

	database, root := openStore(t)
	home := t.TempDir()
	writeCodexConfig(t, home, "[features]\nmemories = true\n", 0600)
	if err := SaveConfig(root, Config{Enabled: true, Port: freePort(t), RetentionDays: 7}); err != nil {
		t.Fatal(err)
	}
	if _, err := database.DB.Exec("CREATE TRIGGER normalization_fault BEFORE INSERT ON telemetry_observations BEGIN SELECT " + function + "(); END"); err != nil {
		t.Fatal(err)
	}
	initialCursor, err := database.TelemetryCursorPosition(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	var clock atomic.Int64
	clock.Store(receivedAt.UnixNano())
	var clockReads atomic.Int32
	var inventories atomic.Int32
	initialSettled := make(chan struct{})
	ready := make(chan string, 1)
	published := make(chan struct{}, 4)
	done := make(chan error, 1)
	stopped := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		defer close(stopped)
		done <- Serve(ctx, ServeOptions{
			StateRoot: root, Home: home, Store: database,
			Now: func() time.Time {
				if clockReads.Add(1) == 3 {
					close(initialSettled)
				}
				return time.Unix(0, clock.Load()).UTC()
			},
			SettleEvery: 10 * time.Millisecond,
			Ready:       func(addr string) { ready <- addr },
			Published:   func() { published <- struct{}{} },
			Inventory: func(ctx context.Context) error {
				inventories.Add(1)
				if err := database.AcceptInventory(ctx, "retried-delivery", 1, nil, receivedAt.Add(time.Second)); err != nil {
					return err
				}
				return database.FinishInventory(ctx, "retried-delivery", "complete")
			},
		})
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-stopped:
			select {
			case err := <-done:
				if err != nil {
					t.Error(err)
				}
			default:
			}
		case <-time.After(10 * time.Second):
			t.Error("receiver did not stop")
		}
	})
	var addr string
	select {
	case addr = <-ready:
	case err := <-done:
		t.Fatalf("receiver before ready: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("receiver did not start")
	}
	// Ensure the initial drain has finished before the single HTTP delivery.
	select {
	case <-initialSettled:
	case <-time.After(10 * time.Second):
		t.Fatal("receiver did not finish initial recovery")
	}
	token, err := ReadToken(root)
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodPost, "http://"+addr+LogsPath,
		strings.NewReader(exportBody(codexResource, completed("retried-background", "1000", "10"))))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(TokenHeader, token)
	response, err := (&http.Client{Timeout: 5 * time.Second}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, response.Body)
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("receipt status=%d", response.StatusCode)
	}
	select {
	case <-attempted:
	case <-time.After(10 * time.Second):
		t.Fatal("normalization did not encounter the injected fault")
	}
	// Remove the fault without sending another request or restarting the listener.
	fail.Store(false)
	waitPublication := func() {
		t.Helper()
		select {
		case <-published:
		case <-time.After(10 * time.Second):
			t.Fatal("acknowledged journal record did not recover and publish")
		}
	}
	waitPublication()
	assertOwnership(t, database, "retried-background", ownership{states: map[string]int{"pending": 1}})
	cursor, err := database.TelemetryCursorPosition(context.Background())
	if err != nil || cursor == initialCursor {
		t.Fatalf("cursor did not advance after retry: %+v %v", cursor, err)
	}
	clock.Store(receivedAt.Add(SettleAfter + time.Second).UnixNano())
	waitPublication()
	assertOwnership(t, database, "retried-background", ownership{otelEvents: 1, otelTokens: 1010, usageSessions: 1, markers: 1, states: map[string]int{"otel_owned": 1}})
	if attempts.Load() != 2 || inventories.Load() != 1 {
		t.Fatalf("normalization attempts=%d inventories=%d, want one failure, one successful retry and one inventory", attempts.Load(), inventories.Load())
	}
}
