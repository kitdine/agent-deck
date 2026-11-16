package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"

	"github.com/kitdine/agent-deck/internal/store"
)

// The helper emits the current core schema in every serving publication.
// A native consumer left behind rejects all real data, even if its synthetic
// fixtures and the Go schema migration tests pass independently.
func TestNativeServingReaderSupportsCurrentCoreSchema(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "apps", "macos", "AgentDeckShared", "DesktopWire.swift"))
	if err != nil {
		t.Fatal(err)
	}
	match := regexp.MustCompile(`maximumSupportedCoreSchema\s*=\s*(\d+)`).FindSubmatch(source)
	if len(match) != 2 {
		t.Fatal("native serving schema limit not found")
	}
	supported, err := strconv.Atoi(string(match[1]))
	if err != nil || supported != store.CurrentSchemaVersion {
		t.Fatalf("native serving reader supports schema %d; helper emits %d", supported, store.CurrentSchemaVersion)
	}
}
