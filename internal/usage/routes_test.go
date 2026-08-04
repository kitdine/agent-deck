package usage

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
)

func TestSessionRoutesAreIdempotentButKeepLaterReturnBoundary(t *testing.T) {
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
	route := SessionRoute{Client: "codex", SessionID: "session", HookEvent: "SessionStart", Source: "resume"}
	if err := service.RecordSessionRoute(ctx, route); err != nil {
		t.Fatal(err)
	}
	if err := service.RecordSessionRoute(ctx, route); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO provider_selections(provider_id,client,provider_name_snapshot,endpoint_snapshot,multiplier_snapshot,selected_at) VALUES(2,'codex','b','x','2','2026-08-04T00:00:02Z')`); err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Second)
	if err := service.RecordSessionRoute(ctx, route); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO provider_selections(provider_id,client,provider_name_snapshot,endpoint_snapshot,multiplier_snapshot,selected_at) VALUES(1,'codex','official','x','1','2026-08-04T00:00:04Z')`); err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Second)
	if err := service.RecordSessionRoute(ctx, route); err != nil {
		t.Fatal(err)
	}
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

	ready := make(chan struct{}, 2)
	release := make(chan struct{})
	barrier := func() {
		ready <- struct{}{}
		<-release
	}
	now := time.Date(2026, 8, 4, 0, 0, 1, 0, time.UTC)
	services := []*Service{New(first, ""), New(second, "")}
	for _, service := range services {
		service.Now = func() time.Time { return now }
		service.beforeSessionRouteWrite = barrier
	}
	route := SessionRoute{Client: "codex", SessionID: "session", HookEvent: "SessionStart", Source: "resume"}
	errs := make(chan error, 2)
	var writers sync.WaitGroup
	writers.Add(2)
	for _, service := range services {
		go func(service *Service) {
			defer writers.Done()
			errs <- service.RecordSessionRoute(ctx, route)
		}(service)
	}
	<-ready
	<-ready
	close(release)
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
	route := SessionRoute{Client: "codex", SessionID: "session", HookEvent: "SessionStart", Source: "resume"}
	if err := service.RecordSessionRoute(ctx, route); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO provider_selections(provider_id,client,provider_name_snapshot,endpoint_snapshot,multiplier_snapshot,via_wrapper,selected_at) VALUES(1,'codex','official','x','1',1,'2026-08-04T00:00:02Z')`); err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Second)
	if err := service.RecordSessionRoute(ctx, route); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO provider_selections(provider_id,client,provider_name_snapshot,endpoint_snapshot,multiplier_snapshot,via_wrapper,selected_at) VALUES(1,'codex','official','x','1',0,'2026-08-04T00:00:04Z')`); err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Second)
	if err := service.RecordSessionRoute(ctx, route); err != nil {
		t.Fatal(err)
	}
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

func TestClaudeConfigChangeRecordsMatchedOrUnknownRoute(t *testing.T) {
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
	if err := service.RecordClaudeConfigChange(ctx, "session", snapshot, true); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Second)
	if err := service.RecordClaudeConfigChange(ctx, "session", store.ProviderSnapshot{}, false); err != nil {
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
