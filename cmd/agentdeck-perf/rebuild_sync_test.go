package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRebuildWaitsForAppReadiness(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	exited := make(chan struct{})
	done := make(chan error, 1)
	go func() { done <- waitRebuildRequest(ctx, dir, exited) }()
	select {
	case err := <-done:
		t.Fatalf("rebuild released before App readiness: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	if err := os.WriteFile(filepath.Join(dir, "rebuild-request"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	close(exited)
	if err := waitRebuildRequest(ctx, t.TempDir(), exited); err == nil {
		t.Fatal("App exit did not end readiness wait")
	}
	cancel()
	if err := waitRebuildRequest(ctx, t.TempDir(), make(chan struct{})); err == nil {
		t.Fatal("cancelled readiness wait succeeded")
	}
}

func TestRebuildReadyRequiresSuccessfulProcessStart(t *testing.T) {
	dir := t.TempDir()
	trace := t.TempDir()
	cfg := runConfig{Agentdeck: filepath.Join(dir, "missing"), TZ: "UTC", Batch: &batchState{Root: dir, Home: t.TempDir()}}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if got := runRebuild(ctx, cfg, trace); got.Error == "" {
		t.Fatal("missing executable succeeded")
	}
	if _, err := os.Stat(filepath.Join(trace, "rebuild-ready")); !os.IsNotExist(err) {
		t.Fatalf("failed start released driver: %v", err)
	}
	cfg.Agentdeck = filepath.Join(dir, "fake-cli")
	if err := os.WriteFile(cfg.Agentdeck, []byte("#!/bin/sh\nwhile [ ! -f \"$AGENTDECK_PERF_OUT/rebuild-ready\" ]; do sleep 0.01; done\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	got := runRebuild(ctx, cfg, trace)
	if got.Error != "" || got.ExitCode != 0 || got.EndNS <= got.StartNS {
		t.Fatalf("rebuild handshake failed: %+v", got)
	}
}
