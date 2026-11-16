package main

import (
	"bytes"
	"context"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
	"github.com/kitdine/agent-deck/internal/telemetry"
)

func TestMockOTelEventsAreDeterministicMeteredAndNeverAProducer(t *testing.T) {
	if !bytes.Equal(mockEvent(7, 3), mockEvent(7, 3)) || bytes.Equal(mockEvent(7, 3), mockEvent(7, 4)) {
		t.Fatal("mock events are not a deterministic function of seed and sequence")
	}
	records, err := telemetry.Decode(mockEvent(7, 3))
	if err != nil || len(records) != 1 {
		t.Fatalf("decode: %v %d", err, len(records))
	}
	if r := records[0]; !r.Metered() || r.Invalid != "" || r.Producer != mockProducer || r.ResponseID == "" {
		t.Fatalf("mock record = %+v", r)
	}
}

func TestMockOTelProducerDeliversToTheReceiver(t *testing.T) {
	root := filepath.Join(t.TempDir(), "state")
	database, err := store.Open(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	if _, err := sendOTel(root, 1, 1, 1, 0); err == nil {
		t.Fatal("sent to a disabled receiver")
	}
	if err := telemetry.SaveConfig(root, telemetry.Config{Enabled: true, Port: port, RetentionDays: 7}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	ready := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- telemetry.Serve(ctx, telemetry.ServeOptions{StateRoot: root, Owner: "foreground", Store: database, Ready: func(string) { close(ready) }})
	}()
	select {
	case <-ready:
	case err := <-done:
		t.Fatal(err)
	}
	result, err := sendOTel(root, 1, 1, 5, 0)
	if err != nil || result.Accepted != 5 || result.Sent != 5 {
		t.Fatalf("send = %+v %v", result, err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		summary, _ := database.TelemetrySummary(context.Background())
		// The mock is never a supported producer: it stays capture_only.
		if summary.CaptureOnly == 5 && summary.OTelOwned == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("observations not normalized: %+v", summary)
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
