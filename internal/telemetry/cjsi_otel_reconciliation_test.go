package telemetry

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
)

// upgradeTestStoreToCJSI37 applies the CJSI schema-37 core facts & receipt
// tables and drops the legacy trigger in a test-isolated store without widening
// production migration APIs.
func upgradeTestStoreToCJSI37(t *testing.T, db *store.Store, root string) {
	t.Helper()
	if _, err := db.DB.Exec(`
DROP TRIGGER IF EXISTS telemetry_jsonl_retracts_otel;
CREATE TABLE cjsi_scope_overlays (
    domain TEXT NOT NULL CHECK(domain IN ('usage','session','origin','serving')),
    scope_kind TEXT NOT NULL CHECK(scope_kind IN ('thread','source','group','partition','orphan')),
    scope_key BLOB NOT NULL CHECK(length(scope_key)=32), window_key BLOB NOT NULL CHECK(length(window_key)=32),
    lower_at TEXT, upper_at TEXT, pending INTEGER NOT NULL CHECK(pending IN (0,1)),
    partial INTEGER NOT NULL CHECK(partial IN (0,1)), masked INTEGER NOT NULL CHECK(masked IN (0,1)),
    coverage_revision INTEGER NOT NULL CHECK(coverage_revision>=0), protection_revision INTEGER NOT NULL CHECK(protection_revision>=0),
    PRIMARY KEY(domain,scope_kind,scope_key,window_key)
) STRICT, WITHOUT ROWID;
CREATE TABLE cjsi_publication_attempts (
    ticket INTEGER NOT NULL CHECK(ticket>0), attempt INTEGER NOT NULL CHECK(attempt>0),
    base_ticket INTEGER NOT NULL CHECK(base_ticket>=0 AND base_ticket<ticket),
    core_epoch INTEGER NOT NULL CHECK(core_epoch>0), index_epoch INTEGER NOT NULL CHECK(index_epoch>=0),
    session_revision INTEGER NOT NULL, usage_revision INTEGER NOT NULL, user_revision INTEGER NOT NULL,
    route_revision INTEGER NOT NULL, price_revision INTEGER NOT NULL,
    protection_revision INTEGER NOT NULL, coverage_revision INTEGER NOT NULL,
    input_digest BLOB NOT NULL CHECK(length(input_digest)=32),
    diff_digest BLOB CHECK(length(diff_digest)=32), session_seal BLOB CHECK(length(session_seal)=32),
    slot_seal BLOB CHECK(length(slot_seal)=32), slot_digest BLOB CHECK(length(slot_digest)=32),
    slot_size INTEGER CHECK(slot_size>=0), slot_format INTEGER CHECK(slot_format>0),
    slot_identity BLOB CHECK(length(slot_identity)=16), built_at TEXT, valid_until TEXT,
    accept_until TEXT NOT NULL, state TEXT NOT NULL CHECK(state IN ('draft','sealed','opened','retired')),
    PRIMARY KEY(ticket,attempt)
) STRICT, WITHOUT ROWID;
CREATE TABLE cjsi_usage_groups (
    ticket INTEGER NOT NULL, attempt INTEGER NOT NULL, thread TEXT NOT NULL CHECK(length(thread)>0),
    input_digest BLOB NOT NULL CHECK(length(input_digest)=32), diff_digest BLOB NOT NULL CHECK(length(diff_digest)=32),
    state INTEGER NOT NULL CHECK(state BETWEEN 0 AND 3), gaps INTEGER NOT NULL CHECK(gaps>=0),
    pending INTEGER NOT NULL CHECK(pending IN (0,1)), partial INTEGER NOT NULL CHECK(partial IN (0,1)),
    unresolved_overlap INTEGER NOT NULL CHECK(unresolved_overlap IN (0,1)),
    PRIMARY KEY(ticket,attempt,thread),
    FOREIGN KEY(ticket,attempt) REFERENCES cjsi_publication_attempts(ticket,attempt) ON DELETE CASCADE
) STRICT, WITHOUT ROWID;
CREATE TABLE cjsi_usage_selection (
    thread TEXT PRIMARY KEY, ticket INTEGER NOT NULL, attempt INTEGER NOT NULL,
    FOREIGN KEY(ticket,attempt,thread) REFERENCES cjsi_usage_groups(ticket,attempt,thread)
) STRICT, WITHOUT ROWID;
CREATE TABLE cjsi_usage_sources (
    ticket INTEGER NOT NULL, attempt INTEGER NOT NULL, thread TEXT NOT NULL,
    name_digest BLOB NOT NULL CHECK(length(name_digest)=32), selection_epoch INTEGER NOT NULL CHECK(selection_epoch>0),
    workset_round INTEGER NOT NULL CHECK(workset_round>0), workset_attempt INTEGER NOT NULL CHECK(workset_attempt>0),
    identity TEXT NOT NULL, generation INTEGER NOT NULL CHECK(generation>0),
    replay_parser INTEGER NOT NULL CHECK(replay_parser>0), max_line_bytes INTEGER NOT NULL CHECK(max_line_bytes>0),
    read_end INTEGER NOT NULL, complete_end INTEGER NOT NULL CHECK(complete_end BETWEEN 0 AND read_end),
    prefix_digest BLOB NOT NULL CHECK(length(prefix_digest)=32),
    PRIMARY KEY(ticket,attempt,thread,name_digest),
    FOREIGN KEY(ticket,attempt,thread) REFERENCES cjsi_usage_groups ON DELETE CASCADE
) STRICT, WITHOUT ROWID;
CREATE TABLE cjsi_takeovers (
    client TEXT NOT NULL CHECK(client='codex'), thread TEXT NOT NULL, period INTEGER NOT NULL CHECK(period>0),
    opened_ticket INTEGER NOT NULL CHECK(opened_ticket>0), closed_ticket INTEGER CHECK(closed_ticket>=opened_ticket),
    attempt INTEGER NOT NULL CHECK(attempt>0), receipt_version INTEGER NOT NULL CHECK(receipt_version=1),
    safe_old_side INTEGER NOT NULL CHECK(safe_old_side IN (0,1)), proof_digest BLOB NOT NULL CHECK(length(proof_digest)=32),
    PRIMARY KEY(client,thread,period),
    FOREIGN KEY(opened_ticket,attempt,thread) REFERENCES cjsi_usage_groups(ticket,attempt,thread)
) STRICT, WITHOUT ROWID;
UPDATE schema_metadata SET version = 37 WHERE singleton = 1;
`); err != nil {
		t.Fatal(err)
	}
	// Write a non-SQLite fence file at the legacy sessions.sqlite3 path so any
	// illegal attempt to open the legacy index fails immediately.
	if err := os.WriteFile(filepath.Join(root, "sessions.sqlite3"), []byte("AGENTDECK-CJSI-FENCE-TEST"+strings.Repeat("\x00", 4096)), 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestCJSIOTelReconciliation verifies both legacy schema-36 coexistence and
// CJSI schema-37 reconciliation through Reconciler.Drain/Import/Settle,
// ProjectSessionIndex, SettleAndProject, and Serve.
func TestCJSIOTelReconciliation(t *testing.T) {
	t.Run("legacy schema36 coexistence", func(t *testing.T) {
		const thread = "00000000-0000-4000-abcd-000000000002"
		db, _ := openStore(t)
		now := receivedAt.Add(SettleAfter)
		r := Reconciler{Store: db, Now: func() time.Time { return now }}
		records, err := Decode([]byte(exportBody(codexResource, completed(thread, "10", "1"))))
		if err != nil {
			t.Fatal(err)
		}
		if _, err = r.Import(t.Context(), records, receivedAt); err != nil {
			t.Fatal(err)
		}
		recordInventory(t, db, "same-time", receivedAt)
		if _, err = r.Settle(t.Context()); err != nil {
			t.Fatal(err)
		}
		assertOwnership(t, db, thread, ownership{states: map[string]int{"pending": 1}})
		recordInventory(t, db, "later", receivedAt.Add(time.Second))
		if _, err = r.Settle(t.Context()); err != nil {
			t.Fatal(err)
		}
		assertOwnership(t, db, thread, ownership{otelEvents: 1, otelTokens: 11, usageSessions: 1, markers: 1, states: map[string]int{"otel_owned": 1}})
		if _, err = r.Import(t.Context(), records, now); err != nil {
			t.Fatal(err)
		}
		if result, err := r.Settle(t.Context()); err != nil || result.Changed {
			t.Fatalf("unchanged settlement = %+v, %v", result, err)
		}
		// Legacy schema 36 retains its trigger until migration 37 drops it.
		insertJSONLEvent(t, db, thread, "legacy-call")
		assertOwnership(t, db, thread, ownership{jsonlEvents: 1, usageSessions: 1, states: map[string]int{"jsonl_owned": 1}})
	})

	t.Run("cjsi schema37 reconciler and projection gates", func(t *testing.T) {
		const (
			threadP = "00000000-0000-4000-abcd-000000000011"
			threadC = "00000000-0000-4000-abcd-000000000012"
			threadS = "00000000-0000-4000-abcd-000000000013"
		)
		db, root := openStore(t)
		legacyIndex, err := store.OpenSessions(t.Context(), t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		defer legacyIndex.Close()
		upgradeTestStoreToCJSI37(t, db, root)

		// Header C / rollout filename P conflict only constrains P and C, never S.
		conflictPath := fmt.Sprintf("/synthetic/rollout-2026-10-29T10-00-00-%s.jsonl", threadP)
		if _, err := db.DB.Exec(`INSERT INTO usage_source_files(path,identity,size,cursor,prefix_hash,thread_id) VALUES(?,'id-cp',10,10,'h',?)`, conflictPath, threadC); err != nil {
			t.Fatal(err)
		}
		now := receivedAt.Add(SettleAfter)
		var projectedCalls int
		r := &Reconciler{
			Store: db,
			Now:   func() time.Time { return now },
			SessionsChanged: func(context.Context) error {
				projectedCalls++
				return nil
			},
		}
		records, err := Decode([]byte(exportBody(codexResource,
			completed(threadC, "10", "1"),
			completed(threadS, "12", "1"),
		)))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := r.Import(t.Context(), records, receivedAt); err != nil {
			t.Fatal(err)
		}
		recordInventory(t, db, "later", receivedAt.Add(time.Second))
		res, err := r.Settle(t.Context())
		if !errors.Is(err, store.ErrTelemetrySourceIdentityPending) {
			t.Fatalf("expected pending error for conflicted C, got %+v %v", res, err)
		}
		if res.Owned != 1 || !res.Changed || projectedCalls != 1 {
			t.Fatalf("S should settle and trigger projection callback despite C conflict: res=%+v calls=%d", res, projectedCalls)
		}
		assertOwnership(t, db, threadS, ownership{otelEvents: 1, otelTokens: 13, usageSessions: 1, markers: 1, states: map[string]int{"otel_owned": 1}})
		assertOwnership(t, db, threadC, ownership{states: map[string]int{"pending": 1}})

		// Raw INSERT on schema 37 must NOT silently retract S's OTel ownership.
		insertJSONLEvent(t, db, threadS, "raw-jsonl-s")
		assertOwnership(t, db, threadS, ownership{otelEvents: 1, otelTokens: 13, jsonlEvents: 1, usageSessions: 1, markers: 1, states: map[string]int{"otel_owned": 1}})
		if _, err := db.DB.Exec(`DELETE FROM usage_events WHERE event_key='raw-jsonl-s'`); err != nil {
			t.Fatal(err)
		}

		// Direct physical session-index projection and SettleAndProject bypass fail closed on schema 37.
		if err := ProjectSessionIndex(t.Context(), db, legacyIndex); !errors.Is(err, store.ErrCJSIOTelAuthorityRequired) {
			t.Fatalf("ProjectSessionIndex bypassed CJSI gate: %v", err)
		}
		if _, err := SettleAndProject(t.Context(), db, legacyIndex, now, nil); !errors.Is(err, store.ErrCJSIOTelAuthorityRequired) {
			t.Fatalf("SettleAndProject bypassed CJSI gate: %v", err)
		}
		// Usage-only SettleAndProject (index == nil) works without touching session index.
		if _, err := SettleAndProject(t.Context(), db, nil, now, nil); !errors.Is(err, store.ErrTelemetrySourceIdentityPending) {
			t.Fatalf("SettleAndProject(nil index) = %v", err)
		}
	})

	t.Run("cjsi schema37 receiver drain and settle without legacy index open", func(t *testing.T) {
		const threadS = "00000000-0000-4000-abcd-000000000023"
		db, root := openStore(t)
		upgradeTestStoreToCJSI37(t, db, root)
		if err := SaveConfig(root, Config{Enabled: true, Port: freePort(t), RetentionDays: 7}); err != nil {
			t.Fatal(err)
		}
		var clock atomic.Int64
		clock.Store(receivedAt.UnixNano())
		var customProjections atomic.Int32
		ready, published := make(chan string, 1), make(chan struct{}, 4)
		done := make(chan error, 1)
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		go func() {
			done <- Serve(ctx, ServeOptions{
				StateRoot: root, Home: t.TempDir(), Store: db,
				Now: func() time.Time { return time.Unix(0, clock.Load()).UTC() }, SettleEvery: 10 * time.Millisecond,
				Ready: func(addr string) { ready <- addr }, Published: func() { published <- struct{}{} },
				ProjectSessions: func(context.Context) error {
					customProjections.Add(1)
					return nil
				},
				Inventory: func(ctx context.Context) error {
					if err := db.AcceptInventory(ctx, "cjsi-after", 1, nil, receivedAt.Add(time.Second)); err != nil {
						return err
					}
					return db.FinishInventory(ctx, "cjsi-after", "complete")
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
		req, _ := http.NewRequest(http.MethodPost, "http://"+addr+LogsPath, strings.NewReader(exportBody(codexResource, completed(threadS, "12", "1"))))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(TokenHeader, token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status=%d", resp.StatusCode)
		}
		// Wait for inventory completion publication signal, then advance clock past SettleAfter.
		select {
		case <-published:
		case <-time.After(5 * time.Second):
			t.Fatal("inventory publication signal not received")
		}
		clock.Store(receivedAt.Add(SettleAfter + time.Second).UnixNano())
		select {
		case <-published:
		case <-time.After(5 * time.Second):
			t.Fatal("settlement publication signal not received")
		}
		assertOwnership(t, db, threadS, ownership{otelEvents: 1, otelTokens: 13, usageSessions: 1, markers: 1, states: map[string]int{"otel_owned": 1}})
		if customProjections.Load() == 0 {
			t.Fatal("custom ProjectSessions hook was not invoked on CJSI settlement")
		}
	})
}
