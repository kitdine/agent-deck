package store

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kitdine/agent-deck/internal/perftrace"
)

func TestWritableStoreTracesWrites(t *testing.T) {
	ctx := context.Background()
	out := filepath.Join(t.TempDir(), "perf")
	if err := os.Mkdir(out, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(out, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv(perftrace.EnvOut, out)
	if !perftrace.Start(perftrace.RoleCLI) {
		t.Fatal("tracing did not start")
	}
	t.Cleanup(func() { _ = perftrace.Close() })

	db, err := Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.DB.ExecContext(ctx, "CREATE TABLE perf_probe (v INTEGER)"); err != nil {
		t.Fatal(err)
	}
	explicitCommit(t, ctx, db)
	explicitRollback(t, ctx, db)
	statementTransaction(t, ctx, db)
	if err := perftrace.Close(); err != nil {
		t.Fatal(err)
	}

	got := map[string]string{}
	file, err := os.Open(filepath.Join(out, "cli-000001.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var record struct {
			Stage string `json:"stage"`
			Dur   *int64 `json:"dur"`
			Err   string `json:"err"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(record.Stage, "tx.") {
			if record.Dur == nil || *record.Dur < 0 {
				t.Fatalf("%s has no duration", record.Stage)
			}
			got[record.Stage] = record.Err
		}
		if record.Stage == "trace.invalid" {
			t.Fatalf("store produced a rejected marker: %s", scanner.Text())
		}
	}
	for stage, code := range map[string]string{
		"tx.core.store_testwritablestoretraceswrites": "",
		"tx.core.store_explicitcommit":                "",
		"tx.core.store_explicitrollback":              "rollback",
		"tx.core.store_statementtransaction":          "",
	} {
		gotCode, ok := got[stage]
		if !ok {
			t.Fatalf("missing %s in %v", stage, got)
		}
		if gotCode != code {
			t.Fatalf("%s err = %q, want %q", stage, gotCode, code)
		}
	}
}

func explicitCommit(t *testing.T, ctx context.Context, db *Store) {
	tx, err := db.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO perf_probe VALUES (1)"); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
}

func explicitRollback(t *testing.T, ctx context.Context, db *Store) {
	tx, err := db.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO perf_probe VALUES (2)"); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
}

// statementTransaction mirrors the BEGIN IMMEDIATE pattern on a pinned conn.
func statementTransaction(t *testing.T, ctx context.Context, db *Store) {
	conn, err := db.DB.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	for _, statement := range []string{"BEGIN IMMEDIATE", "INSERT INTO perf_probe VALUES (3)", "COMMIT"} {
		if _, err := conn.ExecContext(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
}

func TestWritableStoreIsUntracedWhenRecordingIsOff(t *testing.T) {
	t.Setenv(perftrace.EnvOut, "")
	db, err := openWritableSQL(":memory:", "core")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.Raw(func(raw any) error {
		if _, ok := raw.(*tracedConn); ok {
			t.Fatal("recording off must use the driver connection directly")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestIdentifierLabel(t *testing.T) {
	for in, want := range map[string]string{
		"usage.(*Service).scanFileMode.func2": "usage_service_scanfilemode_func2",
		"store.AcceptInventory":               "store_acceptinventory",
		"9lives":                              "f9lives",
		"":                                    "unknown",
	} {
		if got := identifierLabel(in); got != want {
			t.Fatalf("identifierLabel(%q) = %q, want %q", in, got, want)
		}
	}
	long := identifierLabel("scanruntime.(*round).acceptDomainInventoryForAllConfiguredSourcesAndPublish.func12")
	if len(long) > 48 || long == "" || !strings.HasSuffix(long, "func12") {
		t.Fatalf("long label = %q", long)
	}
}
