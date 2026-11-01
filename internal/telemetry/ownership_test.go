package telemetry

import (
	"context"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
)

var receivedAt = time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)

func importRecords(t *testing.T, database *store.Store, body string) {
	t.Helper()
	records, err := Decode([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Import(context.Background(), database, records, receivedAt); err != nil {
		t.Fatal(err)
	}
}

func recordInventory(t *testing.T, database *store.Store, id string, captured time.Time) {
	t.Helper()
	if _, err := database.DB.Exec(`INSERT INTO inventory_runs(inventory_id, epoch, captured_at, source_count, state) VALUES(?, 1, ?, 0, 'complete')`, id, captured.UTC().Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
}

func settle(t *testing.T, database *store.Store, now time.Time) store.TelemetrySettleResult {
	t.Helper()
	result, err := database.SettleTelemetry(context.Background(), now, SettleAfter)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

type ownership struct {
	otelEvents, otelTokens, jsonlEvents, usageSessions, markers int
	states                                                      map[string]int
}

func ownershipOf(t *testing.T, database *store.Store, thread string) ownership {
	t.Helper()
	var o ownership
	db := database.DB
	_ = db.QueryRow(`SELECT count(*), coalesce(sum(input_tokens+output_tokens),0) FROM usage_events WHERE session_id=? AND source_path LIKE 'otel:%'`, thread).Scan(&o.otelEvents, &o.otelTokens)
	_ = db.QueryRow(`SELECT count(*) FROM usage_events WHERE session_id=? AND source_path NOT LIKE 'otel:%'`, thread).Scan(&o.jsonlEvents)
	_ = db.QueryRow(`SELECT count(*) FROM usage_sessions WHERE client='codex' AND session_id=?`, thread).Scan(&o.usageSessions)
	_ = db.QueryRow(`SELECT count(*) FROM telemetry_sessions WHERE client='codex' AND session_id=? AND source_kind='codex-otel-background'`, thread).Scan(&o.markers)
	o.states = map[string]int{}
	rows, err := db.Query(`SELECT state, count(*) FROM telemetry_observations WHERE thread_id=? GROUP BY state`, thread)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var state string
		var n int
		_ = rows.Scan(&state, &n)
		o.states[state] = n
	}
	return o
}

func insertJSONLEvent(t *testing.T, database *store.Store, thread, key string) {
	t.Helper()
	if _, err := database.DB.Exec(`INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,output_tokens,source_path,source_offset) VALUES(?,'codex',?,?,'2026-10-29T12:30:00Z','gpt-6-astra',100,5,'/home/.codex/sessions/rollout.jsonl',0)`, key, thread, key); err != nil {
		t.Fatal(err)
	}
}

func TestBackgroundConversationSettlesOnceAndSurvivesRepeatedPasses(t *testing.T) {
	database, _ := openStore(t)
	importRecords(t, database, exportBody(codexResource, completed("bg-1", "1000", "10"), completed("bg-1", "2000", "20")))
	if o := ownershipOf(t, database, "bg-1"); o.states["pending"] != 2 || o.otelEvents != 0 {
		t.Fatalf("supported observations not pending: %+v", o)
	}
	// Too young, and no inventory after receipt: nothing is counted.
	settle(t, database, receivedAt.Add(time.Minute))
	recordInventory(t, database, "before", receivedAt.Add(-time.Minute))
	settle(t, database, receivedAt.Add(time.Hour))
	if o := ownershipOf(t, database, "bg-1"); o.otelEvents != 0 || o.markers != 0 {
		t.Fatalf("counted before settle conditions held: %+v", o)
	}
	recordInventory(t, database, "after", receivedAt.Add(5*time.Minute))
	if result := settle(t, database, receivedAt.Add(SettleAfter)); !result.Changed || result.Owned != 2 {
		t.Fatalf("settle result %+v", result)
	}
	want := ownership{otelEvents: 2, otelTokens: 3030, usageSessions: 1, markers: 1, states: map[string]int{"otel_owned": 2}}
	assertOwnership(t, database, "bg-1", want)
	// Its own usage_sessions, marker and otel: events are not JSONL evidence:
	// any number of further passes leaves exactly one contribution each.
	for pass := 0; pass < 3; pass++ {
		if result := settle(t, database, receivedAt.Add(time.Duration(pass+2)*time.Hour)); result.Changed {
			t.Fatalf("pass %d changed an owned conversation: %+v", pass, result)
		}
	}
	assertOwnership(t, database, "bg-1", want)
	// Replay and re-import are no-ops.
	importRecords(t, database, exportBody(codexResource, completed("bg-1", "1000", "10"), completed("bg-1", "2000", "20")))
	settle(t, database, receivedAt.Add(5*time.Hour))
	assertOwnership(t, database, "bg-1", want)
}

func assertOwnership(t *testing.T, database *store.Store, thread string, want ownership) {
	t.Helper()
	got := ownershipOf(t, database, thread)
	if got.otelEvents != want.otelEvents || got.otelTokens != want.otelTokens || got.jsonlEvents != want.jsonlEvents ||
		got.usageSessions != want.usageSessions || got.markers != want.markers || len(got.states) != len(want.states) {
		t.Fatalf("ownership = %+v, want %+v", got, want)
	}
	for state, n := range want.states {
		if got.states[state] != n {
			t.Fatalf("ownership = %+v, want %+v", got, want)
		}
	}
}

func ownedConversation(t *testing.T, database *store.Store, thread string) {
	t.Helper()
	importRecords(t, database, exportBody(codexResource, completed(thread, "1000", "10")))
	recordInventory(t, database, "inv-"+thread, receivedAt.Add(time.Minute))
	settle(t, database, receivedAt.Add(SettleAfter))
	if o := ownershipOf(t, database, thread); o.otelEvents != 1 || o.markers != 1 {
		t.Fatalf("conversation not owned: %+v", o)
	}
}

func TestJSONLConversationIsNeverCountedFromOTel(t *testing.T) {
	database, _ := openStore(t)
	insertJSONLEvent(t, database, "main-1", "jsonl-1")
	importRecords(t, database, exportBody(codexResource, completed("main-1", "100", "5")))
	recordInventory(t, database, "inv", receivedAt.Add(time.Minute))
	settle(t, database, receivedAt.Add(SettleAfter))
	assertOwnership(t, database, "main-1", ownership{jsonlEvents: 1, states: map[string]int{"jsonl_owned": 1}})
	// A committed rollout file alone, with no usage events yet, is evidence too.
	if _, err := database.DB.Exec(`INSERT INTO usage_source_files(path,identity,size,cursor,prefix_hash,session_id) VALUES('/home/.codex/sessions/rollout-main-2.jsonl','i',0,0,'','main-2')`); err != nil {
		t.Fatal(err)
	}
	importRecords(t, database, exportBody(codexResource, completed("main-2", "100", "5")))
	settle(t, database, receivedAt.Add(SettleAfter))
	assertOwnership(t, database, "main-2", ownership{states: map[string]int{"jsonl_owned": 1}})
}

func TestLateJSONLRetractsOTelInTheSameTransaction(t *testing.T) {
	database, _ := openStore(t)
	ownedConversation(t, database, "late-1")
	tx, err := database.DB.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(`INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,output_tokens,source_path,source_offset) VALUES('jsonl-late','codex','late-1','jsonl-late','2026-10-29T12:30:00Z','gpt-6-astra',100,5,'/home/.codex/sessions/rollout-late.jsonl',0)`); err != nil {
		t.Fatal(err)
	}
	var otel, markers int
	_ = tx.QueryRow(`SELECT count(*) FROM usage_events WHERE session_id='late-1' AND source_path LIKE 'otel:%'`).Scan(&otel)
	_ = tx.QueryRow(`SELECT count(*) FROM telemetry_sessions WHERE session_id='late-1'`).Scan(&markers)
	if otel != 0 || markers != 0 {
		t.Fatalf("JSONL write left OTel contribution inside its transaction: events=%d markers=%d", otel, markers)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	settle(t, database, receivedAt.Add(2*time.Hour))
	assertOwnership(t, database, "late-1", ownership{jsonlEvents: 1, usageSessions: 1, states: map[string]int{"jsonl_owned": 1}})
	var first, last string
	_ = database.DB.QueryRow(`SELECT first_at, last_at FROM usage_sessions WHERE session_id='late-1'`).Scan(&first, &last)
	if first != "2026-10-29T12:30:00Z" || last != first {
		t.Fatalf("session kept OTel-derived times: %s..%s", first, last)
	}
}

func TestLateRolloutFileRetractsOnTheNextSettle(t *testing.T) {
	database, _ := openStore(t)
	ownedConversation(t, database, "late-2")
	if _, err := database.DB.Exec(`INSERT INTO usage_source_files(path,identity,size,cursor,prefix_hash,session_id) VALUES('/home/.codex/sessions/rollout-late-2.jsonl','i',0,0,'','late-2')`); err != nil {
		t.Fatal(err)
	}
	if result := settle(t, database, receivedAt.Add(2*time.Hour)); result.Retracted != 1 {
		t.Fatalf("settle result %+v", result)
	}
	assertOwnership(t, database, "late-2", ownership{states: map[string]int{"jsonl_owned": 1}})
}

func TestRebuildLossIsRestoredExactlyOnce(t *testing.T) {
	database, _ := openStore(t)
	ownedConversation(t, database, "rb-1")
	// A full rebuild or lost facts: the otel: rows and sessions are gone.
	if _, err := database.DB.Exec(`DELETE FROM usage_events WHERE source_path LIKE 'otel:%'; DELETE FROM usage_sessions`); err != nil {
		t.Fatal(err)
	}
	if result := settle(t, database, receivedAt.Add(time.Hour)); result.Restored != 1 {
		t.Fatalf("settle result %+v", result)
	}
	settle(t, database, receivedAt.Add(2*time.Hour))
	assertOwnership(t, database, "rb-1", ownership{otelEvents: 1, otelTokens: 1010, usageSessions: 1, markers: 1, states: map[string]int{"otel_owned": 1}})
}

func TestConflictingCountsAreNeverCounted(t *testing.T) {
	database, _ := openStore(t)
	ownedConversation(t, database, "cf-1")
	// The same conversation and record time with different counts.
	importRecords(t, database, exportBody(codexResource, completed("cf-1", "1000", "11")))
	settle(t, database, receivedAt.Add(time.Hour))
	assertOwnership(t, database, "cf-1", ownership{states: map[string]int{"conflict": 2}})
}

func TestUnsupportedProducerStaysCaptureOnly(t *testing.T) {
	database, _ := openStore(t)
	importRecords(t, database, exportBody([]attr{s("service.name", "codex_exec"), s("service.version", "0.166.0")}, completed("u-1", "1000", "10")))
	importRecords(t, database, exportBody(codexResource, append(completed("u-2", "1000", "10")[:0], s("event.name", "codex.websocket_event"), s("event.kind", "response.completed"), s("conversation.id", "u-2"), i("input_token_count", "5"))))
	recordInventory(t, database, "inv", receivedAt.Add(time.Minute))
	settle(t, database, receivedAt.Add(time.Hour))
	assertOwnership(t, database, "u-1", ownership{states: map[string]int{"capture_only": 1}})
	assertOwnership(t, database, "u-2", ownership{states: map[string]int{"capture_only": 1}})
	if capability, reason := MeteringCapability(true, "", true); capability != CapabilityCaptureOnly || reason != ReasonProducerUnsupported {
		t.Fatalf("capability %s %s", capability, reason)
	}
	if capability, _ := MeteringCapability(true, supportTable[0].Name(), true); capability != CapabilityReconciled {
		t.Fatalf("supported producer capability %s", capability)
	}
}

func TestSourceMarkerQueryIsTheClassificationInterface(t *testing.T) {
	database, _ := openStore(t)
	ownedConversation(t, database, "mk-1")
	insertJSONLEvent(t, database, "main-9", "jsonl-9")
	kinds, err := database.TelemetrySessionSourceKinds(context.Background(), "codex", []string{"mk-1", "main-9", "absent"})
	if err != nil {
		t.Fatal(err)
	}
	if len(kinds) != 1 || kinds["mk-1"] != store.TelemetrySessionSourceKind {
		t.Fatalf("markers = %v", kinds)
	}
}

func TestSupportTableRequiresTheVerifiedOriginator(t *testing.T) {
	database, _ := openStore(t)
	tui := []attr{s("service.name", "codex-app-server"), s("service.version", "0.165.0")}
	withOriginator := func(conversation, originator string) []attr {
		fields := completed(conversation, "1000", "10")
		for i := range fields {
			if fields[i].key == "originator" {
				fields[i] = s("originator", originator)
			}
		}
		return fields
	}
	importRecords(t, database, exportBody(tui, withOriginator("tui-1", "codex-tui")))
	importRecords(t, database, exportBody(tui, withOriginator("desk-1", "Codex Desktop")))
	assertOwnership(t, database, "tui-1", ownership{states: map[string]int{"pending": 1}})
	assertOwnership(t, database, "desk-1", ownership{states: map[string]int{"capture_only": 1}})
}

func TestMeteringFollowsTheLastMeteredEventNotStreamEvents(t *testing.T) {
	database, _ := openStore(t)
	records, err := Decode([]byte(exportBody(codexResource, completed("m-1", "1000", "10"))))
	if err != nil {
		t.Fatal(err)
	}
	startup, _ := Decode([]byte(exportBody(codexResource, []attr{s("event.name", "codex.conversation_starts"), s("conversation.id", "m-1")})))
	for k, batch := range [][]Record{records, startup} {
		observations := []store.TelemetryObservation{Observation(batch[0], "receiver", receivedAt.Add(time.Duration(k)*time.Second))}
		if _, err := database.ApplyTelemetryObservations(context.Background(), observations, nil, nil); err != nil {
			t.Fatal(err)
		}
	}
	summary, _ := database.TelemetrySummary(context.Background())
	if capability, _ := MeteringCapability(true, summary.LastSupportEntry, summary.LastReceivedAt != ""); capability != CapabilityReconciled {
		t.Fatalf("a later unmetered startup event hid a supported producer: %s (%+v)", capability, summary)
	}
}
