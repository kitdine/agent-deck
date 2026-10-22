package p07measure_test

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	. "github.com/kitdine/agent-deck/scripts/p07measure"
)

func TestAttributionPreservesFullBoundaryAndCaller(t *testing.T) {
	var output bytes.Buffer
	Start(&output)
	defer Start(nil)
	db, err := Open("sqlite", filepath.Join(t.TempDir(), "sessions.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer CloseDB(db)
	db.SetMaxOpenConns(1)
	held, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	go func() { time.Sleep(20 * time.Millisecond); held.Close() }()
	tx, err := BeginTx("shared-begin-helper", db, context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(12 * time.Millisecond)
	if err := Commit(tx); err != nil {
		t.Fatal(err)
	}
	var r Record
	decoder := json.NewDecoder(bytes.NewReader(output.Bytes()))
	for decoder.More() {
		var next Record
		if err := decoder.Decode(&next); err != nil {
			t.Fatal(err)
		}
		if next.Event == "end" {
			r = next
			break
		}
	}
	if !r.PhasesComplete || r.BeginCallNS < int64(15*time.Millisecond) || r.BodyNS < int64(10*time.Millisecond) || r.BeginCallNS+r.BodyNS+r.TerminalNS != r.DurationNS {
		t.Fatalf("bad attribution: %+v", r)
	}
	if !strings.Contains(strings.Join(r.Caller, "/"), "TestAttributionPreservesFullBoundaryAndCaller") {
		t.Fatalf("caller missing: %+v", r.Caller)
	}
	if r.BeginReturnNS-r.BeginNS != r.BeginCallNS || r.EndNS-r.TerminalStartNS != r.TerminalNS {
		t.Fatal("phase clocks do not cover full boundary")
	}
}
