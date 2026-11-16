package scanruntime

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/desktop"
	"github.com/kitdine/agent-deck/internal/store"
	"github.com/kitdine/agent-deck/internal/usage"
)

func TestUsagePublicationSurvivesSessionFailure(t *testing.T) {
	root := t.TempDir()
	index, err := store.OpenSessions(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	index.Close()
	r := newRound(t.TempDir())
	r.openSessions = func(context.Context, string) (*store.Store, error) {
		return nil, errors.New("session index unavailable")
	}
	publications := 0
	r.publishFullServing = func(ctx context.Context, stateRoot string) (desktop.ServingSnapshot, error) {
		publications++
		return r.desktopService(stateRoot).PublishServingSnapshot(ctx, desktop.Request{WireVersion: desktop.WireVersion, RecentLimit: desktop.DefaultRecentLimit}, false)
	}
	r.executeProduction(context.Background(), root, false)
	if r.result.Usage.State != "completed" || r.result.Session.State != "failed" {
		t.Fatalf("domain results: usage=%+v session=%+v", r.result.Usage, r.result.Session)
	}
	if publications != 1 {
		t.Fatalf("successful usage publications=%d", publications)
	}
	if r.publication == nil || !r.publication.Payload.Data.Usage.Available {
		t.Fatal("successful usage has no available serving publication")
	}
}

func TestIdentityPendingStillClassifiesReadableSessions(t *testing.T) {
	ctx := context.Background()
	home, root := t.TempDir(), t.TempDir()
	core, err := store.Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	defer core.Close()
	index, err := store.OpenSessions(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	defer index.Close()
	missing := filepath.Join(home, ".codex", "sessions", "unreadable-copy.jsonl")
	if _, err := core.DB.Exec(`INSERT INTO usage_source_files(path,identity,size,cursor,prefix_hash) VALUES(?,'old',0,0,'')`, missing); err != nil {
		t.Fatal(err)
	}
	if _, err := core.DB.Exec(`INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,output_tokens,source_path,source_offset)
 VALUES('pending','codex','parent','pending','2026-11-15T12:00:00Z','m',10,1,?,0)`, missing); err != nil {
		t.Fatal(err)
	}
	if _, err := index.DB.Exec(`INSERT INTO session_sources(source_path,identity,cursor,size,modified_at,prefix_hash,priority,parser_version,scanned_at)
 VALUES('readable.jsonl','current',0,0,0,'prefix',1,1,'2026-11-15T12:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if _, err := index.DB.Exec(`INSERT INTO session_metadata(source_path,client,session_id,project,model,parser_version,first_at,last_at)
 VALUES('readable.jsonl','claude','work-new','project','m',1,'2026-11-15T12:00:00Z','2026-11-15T12:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	r := newRound(home)
	if err := r.settleAndClassify(ctx, root, core, index, usage.New(core, home)); !errors.Is(err, store.ErrTelemetrySourceIdentityPending) {
		t.Fatalf("lost numerical pending result: %v", err)
	}
	var automatic, reason string
	if err := core.DB.QueryRow(`SELECT automatic,reason_code FROM session_origins WHERE client='claude' AND session_id='work-new'`).Scan(&automatic, &reason); err != nil || automatic != "work" || reason != "default_work" {
		t.Fatalf("readable session not classified: %s %s %v", automatic, reason, err)
	}
}

func TestIdentityIncompleteInventoryCannotReportCompleted(t *testing.T) {
	ctx := context.Background()
	home := t.TempDir()
	db, err := store.Open(ctx, filepath.Join(home, "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	path := filepath.Join(home, ".codex", "sessions", "unreadable-copy.jsonl")
	for _, q := range []string{
		`INSERT INTO usage_source_files(path,identity,size,cursor,prefix_hash) VALUES('` + path + `','missing',0,0,'')`,
		`INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,output_tokens,source_path,source_offset) VALUES('e','codex','parent','e','2026-10-01T00:00:00Z','gpt-5',10,1,'` + path + `',0)`,
	} {
		if _, err = db.DB.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	if err = db.AcceptInventory(ctx, "identity", 1, nil, time.Now()); err != nil {
		t.Fatal(err)
	}
	err = finishUsageInventory(ctx, db, usage.New(db, home), "identity")
	if !errors.Is(err, store.ErrTelemetrySourceIdentityPending) {
		t.Fatalf("inventory %v", err)
	}
	finishDomainInventory(db, "identity", err)
	var state string
	if err = db.DB.QueryRow(`SELECT state FROM inventory_runs WHERE inventory_id='identity'`).Scan(&state); err != nil || state != "partial" {
		t.Fatalf("inventory state %q %v", state, err)
	}
	if got := failedUsage(store.ErrTelemetrySourceIdentityPending, 0); got.State == "completed" || got.ErrorCode != "source_identity_pending" {
		t.Fatalf("result %+v", got)
	}
	// Normal source cleanup permits the next inventory without a user setting.
	if _, err = db.DB.Exec(`DELETE FROM usage_events; DELETE FROM usage_source_files`); err != nil {
		t.Fatal(err)
	}
	if err = db.AcceptInventory(ctx, "recovery", 1, nil, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err = finishUsageInventory(ctx, db, usage.New(db, home), "recovery"); err != nil {
		t.Fatal(err)
	}
	if err = db.DB.QueryRow(`SELECT state FROM inventory_runs WHERE inventory_id='recovery'`).Scan(&state); err != nil || state != "complete" {
		t.Fatalf("recovery %q %v", state, err)
	}
}
