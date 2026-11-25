package doctor

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
)

func TestCJSIHealth(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	exec := func(query string) {
		t.Helper()
		if _, err := tx.ExecContext(ctx, query); err != nil {
			t.Fatal(err)
		}
	}
	exec(`CREATE TABLE cjsi_publication_head(singleton,slot_ticket,slot_attempt,slot_seal,session_ticket,
		session_attempt,session_seal,session_revision,core_epoch,index_epoch,protection_revision,coverage_revision);
		CREATE TABLE cjsi_publication_attempts(ticket,attempt,state,core_epoch,index_epoch,diff_digest,
		slot_seal,session_seal,session_revision,slot_size,slot_format,slot_identity,slot_digest,protection_revision,
		coverage_revision,built_at,valid_until);
		CREATE TABLE cjsi_scope_overlays(masked);
		CREATE TABLE cjsi_usage_groups(ticket,attempt,thread);
		CREATE TABLE cjsi_usage_selection(ticket,attempt,thread);
		INSERT INTO cjsi_publication_head VALUES(1,0,0,NULL,0,0,NULL,0,7,8,0,0);
		INSERT INTO cjsi_publication_attempts(ticket,attempt,state,core_epoch,index_epoch)
		VALUES(2,1,'draft',7,8)`)
	extraTables := strings.Fields(`cjsi_layout_journal cjsi_usage_sources cjsi_usage_members
		cjsi_usage_intervals cjsi_usage_interval_dirty_ranges cjsi_usage_references cjsi_old_preconditions cjsi_checkpoint_attempts
		cjsi_checkpoint_records cjsi_takeovers cjsi_receipt_observations cjsi_receipt_events cjsi_receipt_sessions cjsi_receipt_markers`)
	for _, table := range extraTables {
		exec("CREATE TABLE " + table + "(fixture)")
	}
	stamp := store.CJSIIndexStamp{CoreEpoch: 7, IndexEpoch: 8}
	check := func(want time.Time, wantErr error) {
		t.Helper()
		got, err := selectedCJSICoreSlot(ctx, tx, stamp)
		if !got.Equal(want) || !errors.Is(err, wantErr) {
			t.Fatalf("selected slot = %v, err=%v, want %v and %v", got, err, want, wantErr)
		}
	}
	reject := func(name, change string) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			exec("SAVEPOINT invalid_health")
			defer exec("ROLLBACK TO invalid_health; RELEASE invalid_health")
			exec(change)
			got, err := selectedCJSICoreSlot(ctx, tx, stamp)
			if !got.IsZero() || !errors.Is(err, store.ErrCJSIIndexChanged) {
				t.Fatalf("expected incompatible selection, got %v and %v", got, err)
			}
		})
	}
	check(time.Time{}, nil)
	stamp.Ticket = 2
	exec(`UPDATE cjsi_publication_head SET slot_ticket=2,slot_attempt=1,slot_seal=zeroblob(32),
		session_ticket=2,session_attempt=1,session_seal=zeroblob(32),session_revision=3`)
	check(time.Time{}, store.ErrCJSIIndexChanged)
	exec(`UPDATE cjsi_publication_attempts SET state='opened',diff_digest=zeroblob(32),
		slot_seal=zeroblob(32),session_seal=zeroblob(32),session_revision=3,slot_size=10,slot_format=1,
		slot_identity=zeroblob(16),slot_digest=zeroblob(32),protection_revision=0,coverage_revision=0,
		built_at='2026-01-02T10:00:00Z',valid_until='2026-01-02T10:10:00Z'`)
	check(time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC), nil)
	for _, table := range append(extraTables, "cjsi_publication_head", "cjsi_publication_attempts",
		"cjsi_scope_overlays", "cjsi_usage_groups", "cjsi_usage_selection") {
		reject("missing/"+table, "DROP TABLE "+table)
	}
	reject("legacy_trigger", `CREATE TRIGGER Telemetry_JSONL_Retracts_OTel AFTER INSERT ON cjsi_usage_selection BEGIN SELECT 1; END`)
	reject("inverted_slot_interval", "UPDATE cjsi_publication_attempts SET valid_until='2026-01-02T09:59:59Z'")
	exec("INSERT INTO cjsi_usage_groups VALUES(2,1,'thread'); INSERT INTO cjsi_usage_selection VALUES(2,1,'thread')")
	check(time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC), nil)
	reject("dangling_usage_selection", "DELETE FROM cjsi_usage_groups")
	reject("usage_selection_from_other_epoch", `INSERT INTO cjsi_publication_attempts(ticket,attempt,state,core_epoch)
		VALUES(1,1,'opened',9); INSERT INTO cjsi_usage_groups VALUES(1,1,'thread'); UPDATE cjsi_usage_selection SET ticket=1`)
	reject("usage_selection_after_head", `INSERT INTO cjsi_publication_attempts(ticket,attempt,state,core_epoch)
		VALUES(3,1,'opened',7); INSERT INTO cjsi_usage_groups VALUES(3,1,'thread'); UPDATE cjsi_usage_selection SET ticket=3`)
	exec("INSERT INTO cjsi_scope_overlays VALUES(1)")
	check(time.Time{}, nil)
	exec("UPDATE cjsi_publication_attempts SET slot_seal=randomblob(32)")
	check(time.Time{}, store.ErrCJSIIndexChanged)
}

func TestCJSIR1OperationalFailuresUseExistingDatabaseCodes(t *testing.T) {
	for _, tc := range []struct {
		err        error
		name, code string
	}{
		{store.ErrStateBusy, "database", "database_unreadable"},
		{fs.ErrPermission, "database", "database_unreadable"},
		{store.ErrCJSIIndexChanged, "schema", "schema_incompatible"},
	} {
		got := cjsiDatabaseFailure(fmt.Errorf("synthetic hidden detail: %w", tc.err))
		if got.Code != tc.code || got.Name != tc.name || got.Status != "error" {
			t.Errorf("diagnostic for %v = %+v", tc.err, got)
		}
	}
}
