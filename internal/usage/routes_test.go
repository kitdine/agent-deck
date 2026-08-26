package usage

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
)

func TestRecordHookDeliverySessionStartAdvancesIdempotentRoute(t *testing.T) {
	ctx := context.Background()
	database, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err = database.Exec(ctx, `INSERT INTO providers(id,name,endpoint,credential_ref,multiplier,created_at,updated_at) VALUES(1,'official','x','', '1','2026-08-04T00:00:00Z','2026-08-04T00:00:00Z'),(2,'b','x','r','2','2026-08-04T00:00:00Z','2026-08-04T00:00:00Z'); INSERT INTO provider_selections(provider_id,client,provider_name_snapshot,endpoint_snapshot,multiplier_snapshot,selected_at) VALUES(1,'codex','official','x','1','2026-08-04T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	service := New(database, "")
	now := time.Date(2026, 8, 4, 0, 0, 1, 0, time.UTC)
	service.Now = func() time.Time { return now }
	deliverID := 0
	deliver := func() {
		t.Helper()
		deliverID++
		snapshot, snapErr := database.CurrentProviderSnapshot(ctx, "codex")
		if snapErr != nil {
			t.Fatal(snapErr)
		}
		delivery := HookDelivery{Client: "codex", SessionID: "session", HookEvent: "SessionStart", Source: "resume", DeliveryID: fmt.Sprintf("d%d", deliverID), HasSelection: true, Selection: snapshot}
		if err := service.RecordHookDelivery(ctx, delivery); err != nil {
			t.Fatal(err)
		}
	}
	deliver()
	deliver()
	if _, err = database.Exec(ctx, `INSERT INTO provider_selections(provider_id,client,provider_name_snapshot,endpoint_snapshot,multiplier_snapshot,selected_at) VALUES(2,'codex','b','x','2','2026-08-04T00:00:02Z')`); err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Second)
	deliver()
	if _, err = database.Exec(ctx, `INSERT INTO provider_selections(provider_id,client,provider_name_snapshot,endpoint_snapshot,multiplier_snapshot,selected_at) VALUES(1,'codex','official','x','1','2026-08-04T00:00:04Z')`); err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Second)
	deliver()

	rows, err := database.DB.QueryContext(ctx, `SELECT provider FROM usage_session_routes ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var provider string
		if err := rows.Scan(&provider); err != nil {
			t.Fatal(err)
		}
		got = append(got, provider)
	}
	if len(got) != 3 || got[0] != "official" || got[1] != "b" || got[2] != "official" {
		t.Fatalf("routes = %#v", got)
	}

	var observationCount int
	if err := database.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM usage_session_observations WHERE client='codex' AND session_id='session'`).Scan(&observationCount); err != nil {
		t.Fatal(err)
	}
	if observationCount != 4 {
		t.Fatalf("observations = %d, want 4 (one per delivered SessionStart, including the consecutive-identical no-op)", observationCount)
	}
	var advanceCount int
	if err := database.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM usage_session_observations WHERE route_effect='advance'`).Scan(&advanceCount); err != nil {
		t.Fatal(err)
	}
	if advanceCount != 4 {
		t.Fatalf("advance observations = %d, want 4", advanceCount)
	}
}

func TestSecondManagedRunDowngradesBothRuns(t *testing.T) {
	ctx := context.Background()
	database, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err = database.Exec(ctx, `INSERT INTO providers(id,name,endpoint,credential_ref,multiplier,created_at,updated_at) VALUES(1,'p','x','r','1','2026-08-04T00:00:00Z','2026-08-04T00:00:00Z'); INSERT INTO provider_selections(provider_id,client,provider_name_snapshot,endpoint_snapshot,multiplier_snapshot,selected_at) VALUES(1,'codex','p','x','1','2026-08-04T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	service := New(database, "")
	clientProcessCalls := 0
	service.ClientProcesses = func(string) ([]int, error) {
		clientProcessCalls++
		if clientProcessCalls == 1 {
			return []int{101}, nil
		}
		return []int{101, 102}, nil
	}
	service.ProcessAlive = func(pid int) bool { return pid == 101 || pid == 102 }
	if _, _, err = service.StartRun(ctx, "codex", 101); err != nil {
		t.Fatal(err)
	}
	if _, _, err = service.StartRun(ctx, "codex", 102); err != nil {
		t.Fatal(err)
	}
	var exact int
	var rows int
	if err = database.DB.QueryRowContext(ctx, `SELECT COUNT(*),SUM(exact) FROM usage_runs WHERE ended_at IS NULL`).Scan(&rows, &exact); err != nil {
		t.Fatal(err)
	}
	if rows != 2 || exact != 0 {
		t.Fatalf("open runs=%d exact sum=%d, want 2/0", rows, exact)
	}
}

func TestConcurrentDuplicateSessionRoutesInsertOneBoundary(t *testing.T) {
	ctx := context.Background()
	state := filepath.Join(t.TempDir(), "state")
	first, err := store.Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	if _, err = first.Exec(ctx, `
		INSERT INTO providers(id,name,endpoint,credential_ref,multiplier,created_at,updated_at)
		VALUES(1,'official','x','','1','2026-08-04T00:00:00Z','2026-08-04T00:00:00Z');
		INSERT INTO provider_selections(provider_id,client,provider_name_snapshot,endpoint_snapshot,multiplier_snapshot,selected_at)
		VALUES(1,'codex','official','x','1','2026-08-04T00:00:00Z');
	`); err != nil {
		t.Fatal(err)
	}
	second, err := store.Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()

	// The whole accepted-Hook operation is one SQLite transaction, so two
	// concurrent deliveries are serialized by the store's single-writer lock
	// rather than by an application-level barrier: the second transaction
	// only starts its writes once the first has committed, and its own
	// WHERE NOT EXISTS route guard then sees the row the first already wrote.
	now := time.Date(2026, 8, 4, 0, 0, 1, 0, time.UTC)
	services := []*Service{New(first, ""), New(second, "")}
	deliveryIDs := []string{"delivery-a", "delivery-b"}
	for _, service := range services {
		service.Now = func() time.Time { return now }
	}
	snapshot, err := first.CurrentProviderSnapshot(ctx, "codex")
	if err != nil {
		t.Fatal(err)
	}
	errs := make(chan error, 2)
	var writers sync.WaitGroup
	writers.Add(2)
	for i, service := range services {
		go func(service *Service, deliveryID string) {
			defer writers.Done()
			delivery := HookDelivery{Client: "codex", SessionID: "session", HookEvent: "SessionStart", Source: "resume", DeliveryID: deliveryID, HasSelection: true, Selection: snapshot}
			errs <- service.RecordHookDelivery(ctx, delivery)
		}(service, deliveryIDs[i])
	}
	writers.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err := first.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM usage_session_routes`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("concurrent duplicate routes = %d, want 1", count)
	}
	var observations int
	if err := first.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM usage_session_observations`).Scan(&observations); err != nil {
		t.Fatal(err)
	}
	if observations != 2 {
		t.Fatalf("concurrent distinct-delivery observations = %d, want 2", observations)
	}
}

func TestSessionRoutesKeepWrapperOnlyChanges(t *testing.T) {
	ctx := context.Background()
	database, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err = database.Exec(ctx, `
		INSERT INTO providers(id,name,endpoint,credential_ref,multiplier,created_at,updated_at)
		VALUES(1,'official','x','','1','2026-08-04T00:00:00Z','2026-08-04T00:00:00Z');
		INSERT INTO provider_selections(provider_id,client,provider_name_snapshot,endpoint_snapshot,multiplier_snapshot,via_wrapper,selected_at)
		VALUES(1,'codex','official','x','1',0,'2026-08-04T00:00:00Z');
	`); err != nil {
		t.Fatal(err)
	}
	service := New(database, "")
	now := time.Date(2026, 8, 4, 0, 0, 1, 0, time.UTC)
	service.Now = func() time.Time { return now }
	deliverID := 0
	deliver := func() {
		t.Helper()
		deliverID++
		snapshot, snapErr := database.CurrentProviderSnapshot(ctx, "codex")
		if snapErr != nil {
			t.Fatal(snapErr)
		}
		delivery := HookDelivery{Client: "codex", SessionID: "session", HookEvent: "SessionStart", Source: "resume", DeliveryID: fmt.Sprintf("w%d", deliverID), HasSelection: true, Selection: snapshot}
		if err := service.RecordHookDelivery(ctx, delivery); err != nil {
			t.Fatal(err)
		}
	}
	deliver()
	if _, err = database.Exec(ctx, `INSERT INTO provider_selections(provider_id,client,provider_name_snapshot,endpoint_snapshot,multiplier_snapshot,via_wrapper,selected_at) VALUES(1,'codex','official','x','1',1,'2026-08-04T00:00:02Z')`); err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Second)
	deliver()
	if _, err = database.Exec(ctx, `INSERT INTO provider_selections(provider_id,client,provider_name_snapshot,endpoint_snapshot,multiplier_snapshot,via_wrapper,selected_at) VALUES(1,'codex','official','x','1',0,'2026-08-04T00:00:04Z')`); err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Second)
	deliver()
	rows, err := database.DB.QueryContext(ctx, `SELECT via_wrapper FROM usage_session_routes ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []int
	for rows.Next() {
		var viaWrapper int
		if err := rows.Scan(&viaWrapper); err != nil {
			t.Fatal(err)
		}
		got = append(got, viaWrapper)
	}
	if len(got) != 3 || got[0] != 0 || got[1] != 1 || got[2] != 0 {
		t.Fatalf("wrapper route sequence = %#v, want [0 1 0]", got)
	}
}

func TestRecordHookDeliveryConfigChangeRecordsMatchedOrUnknownRoute(t *testing.T) {
	ctx := context.Background()
	database, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	service := New(database, "")
	now := time.Date(2026, 8, 4, 0, 0, 1, 0, time.UTC)
	service.Now = func() time.Time { return now }
	snapshot := store.ProviderSnapshot{Name: "custom", Multiplier: "2", ViaWrapper: true}
	matched := true
	if err := service.RecordHookDelivery(ctx, HookDelivery{Client: "claude", SessionID: "session", HookEvent: "ConfigChange", Source: "user_settings", DeliveryID: "c1", ConfigMatched: &matched, HasSelection: true, Selection: snapshot}); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Second)
	mismatched := false
	if err := service.RecordHookDelivery(ctx, HookDelivery{Client: "claude", SessionID: "session", HookEvent: "ConfigChange", Source: "user_settings", DeliveryID: "c2", ConfigMatched: &mismatched}); err != nil {
		t.Fatal(err)
	}
	rows, err := database.DB.QueryContext(ctx, `SELECT provider,multiplier,via_wrapper,hook_event,source,quality FROM usage_session_routes ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	type route struct {
		provider, multiplier, hookEvent, source, quality string
		viaWrapper                                       int
	}
	var got []route
	for rows.Next() {
		var item route
		if err := rows.Scan(&item.provider, &item.multiplier, &item.viaWrapper, &item.hookEvent, &item.source, &item.quality); err != nil {
			t.Fatal(err)
		}
		got = append(got, item)
	}
	if len(got) != 2 || got[0] != (route{"custom", "2", "ConfigChange", "user_settings", "estimated", 1}) || got[1] != (route{"unknown", "1", "ConfigChange", "user_settings", "estimated", 0}) {
		t.Fatalf("config-change routes = %#v", got)
	}
	var configMatched1, configMatched2 sql.NullInt64
	if err := database.DB.QueryRowContext(ctx, `SELECT config_matched FROM usage_session_observations WHERE delivery_id='c1'`).Scan(&configMatched1); err != nil {
		t.Fatal(err)
	}
	if err := database.DB.QueryRowContext(ctx, `SELECT config_matched FROM usage_session_observations WHERE delivery_id='c2'`).Scan(&configMatched2); err != nil {
		t.Fatal(err)
	}
	if !configMatched1.Valid || configMatched1.Int64 != 1 || !configMatched2.Valid || configMatched2.Int64 != 0 {
		t.Fatalf("config_matched = (%v,%v), want (1,0)", configMatched1, configMatched2)
	}
}

func TestRecordHookDeliverySessionEndAndCompactWriteObservationOnlyNoRoute(t *testing.T) {
	ctx := context.Background()
	database, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	service := New(database, "")
	now := time.Date(2026, 8, 4, 0, 0, 1, 0, time.UTC)
	service.Now = func() time.Time { return now }
	if err := service.RecordHookDelivery(ctx, HookDelivery{Client: "claude", SessionID: "session", HookEvent: "SessionEnd", Source: "", DeliveryID: "e1"}); err != nil {
		t.Fatal(err)
	}
	if err := service.RecordHookDelivery(ctx, HookDelivery{Client: "codex", SessionID: "session", HookEvent: "SessionStart", Source: "compact", DeliveryID: "e2", HasSelection: true, Selection: store.ProviderSnapshot{Name: "official", Multiplier: "1"}}); err != nil {
		t.Fatal(err)
	}
	var routeCount int
	if err := database.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM usage_session_routes`).Scan(&routeCount); err != nil {
		t.Fatal(err)
	}
	if routeCount != 0 {
		t.Fatalf("routes = %d, want 0", routeCount)
	}
	rows, err := database.DB.QueryContext(ctx, `SELECT route_effect FROM usage_session_observations ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var effects []string
	for rows.Next() {
		var effect string
		if err := rows.Scan(&effect); err != nil {
			t.Fatal(err)
		}
		effects = append(effects, effect)
	}
	if len(effects) != 2 || effects[0] != "none" || effects[1] != "none" {
		t.Fatalf("route effects = %#v, want [none none]", effects)
	}
}

func TestRecordHookDeliveryWholeOperationRetryIsANoOpAcrossBothStreams(t *testing.T) {
	ctx := context.Background()
	database, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	service := New(database, "")
	now := time.Date(2026, 8, 4, 0, 0, 1, 0, time.UTC)
	service.Now = func() time.Time { return now }
	delivery := HookDelivery{Client: "codex", SessionID: "session", HookEvent: "SessionStart", Source: "resume", DeliveryID: "retry-1", HasSelection: true, Selection: store.ProviderSnapshot{Name: "official", Multiplier: "1"}}
	for i := 0; i < 3; i++ {
		if err := service.RecordHookDelivery(ctx, delivery); err != nil {
			t.Fatal(err)
		}
	}
	var observations, routes int
	if err := database.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM usage_session_observations WHERE delivery_id='retry-1'`).Scan(&observations); err != nil {
		t.Fatal(err)
	}
	if err := database.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM usage_session_routes`).Scan(&routes); err != nil {
		t.Fatal(err)
	}
	if observations != 1 {
		t.Fatalf("observations for retried delivery = %d, want 1", observations)
	}
	if routes != 1 {
		t.Fatalf("routes after retried delivery = %d, want 1", routes)
	}
}

func TestRecordHookDeliveryRouteWriteFailureRollsBackBothStreams(t *testing.T) {
	ctx := context.Background()
	database, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	// Force the route write (the step after the observation insert) to fail,
	// so a successful RecordHookDelivery call cannot exist without exercising
	// this path: the trigger only fires once the observation insert already
	// affected one row.
	if _, err = database.Exec(ctx, `CREATE TRIGGER force_route_insert_failure BEFORE INSERT ON usage_session_routes BEGIN SELECT RAISE(ABORT, 'forced route insert failure'); END;`); err != nil {
		t.Fatal(err)
	}
	service := New(database, "")
	now := time.Date(2026, 8, 4, 0, 0, 1, 0, time.UTC)
	service.Now = func() time.Time { return now }
	delivery := HookDelivery{Client: "codex", SessionID: "session", HookEvent: "SessionStart", Source: "resume", DeliveryID: "fail-1", HasSelection: true, Selection: store.ProviderSnapshot{Name: "official", Multiplier: "1"}}
	if err := service.RecordHookDelivery(ctx, delivery); err == nil {
		t.Fatal("expected the route write failure to propagate")
	}
	var observations, routes int
	if err := database.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM usage_session_observations`).Scan(&observations); err != nil {
		t.Fatal(err)
	}
	if err := database.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM usage_session_routes`).Scan(&routes); err != nil {
		t.Fatal(err)
	}
	if observations != 0 || routes != 0 {
		t.Fatalf("after a route-write failure, observations=%d routes=%d, want 0/0 (both streams rolled back)", observations, routes)
	}

	// The failed transaction must be fully closed, not left open holding the
	// write lock: a later delivery, once the trigger is removed, commits
	// normally.
	if _, err = database.Exec(ctx, `DROP TRIGGER force_route_insert_failure`); err != nil {
		t.Fatal(err)
	}
	delivery.DeliveryID = "fail-1-retry"
	if err := service.RecordHookDelivery(ctx, delivery); err != nil {
		t.Fatal(err)
	}
	if err := database.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM usage_session_observations`).Scan(&observations); err != nil {
		t.Fatal(err)
	}
	if err := database.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM usage_session_routes`).Scan(&routes); err != nil {
		t.Fatal(err)
	}
	if observations != 1 || routes != 1 {
		t.Fatalf("after the trigger is removed, observations=%d routes=%d, want 1/1", observations, routes)
	}
}

func TestReadPriceResolverUsesSessionRouteOnlyAfterBoundary(t *testing.T) {
	ctx := context.Background()
	database, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err = database.Exec(ctx, `
		INSERT INTO providers(id,name,endpoint,credential_ref,multiplier,created_at,updated_at)
		VALUES(1,'official','x','','1','2026-08-04T00:00:00Z','2026-08-04T00:00:00Z');
		INSERT INTO provider_selections(provider_id,client,provider_name_snapshot,endpoint_snapshot,multiplier_snapshot,selected_at)
		VALUES(1,'codex','official','x','1','2026-08-04T00:00:00Z');
		INSERT INTO usage_session_routes(client,session_id,observed_at,provider,multiplier,via_wrapper,hook_event,source,quality,semantic_key)
		VALUES('codex','session','2026-08-04T00:00:02Z','b','2',1,'SessionStart','resume','estimated','route-b');
	`); err != nil {
		t.Fatal(err)
	}
	service := New(database, "")
	resolver, err := service.loadReadPriceResolver(ctx, time.Date(2026, 8, 4, 0, 0, 4, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	before, err := resolver.priceForEvent(storedEvent{Event: Event{Client: "codex", SessionID: "session", EventAt: "2026-08-04T00:00:01Z", Model: "fixture"}})
	if err != nil {
		t.Fatal(err)
	}
	after, err := resolver.priceForEvent(storedEvent{Event: Event{Client: "codex", SessionID: "session", EventAt: "2026-08-04T00:00:03Z", Model: "fixture"}})
	if err != nil {
		t.Fatal(err)
	}
	if before.provider != "official" || before.multiplier != "1" || before.quality != "estimated" || before.viaWrapper {
		t.Fatalf("before boundary = %#v", before)
	}
	if after.provider != "b" || after.multiplier != "2" || after.quality != "estimated" || !after.viaWrapper {
		t.Fatalf("after boundary = %#v", after)
	}
}
