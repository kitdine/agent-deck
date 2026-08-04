package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestV17MigrationAddsSessionRoutesAndAllowsManagedOverlap(t *testing.T) {
	ctx := context.Background()
	database, err := Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	if version, err := database.SchemaVersion(ctx); err != nil || version != CurrentSchemaVersion {
		t.Fatalf("schema version = %d, %v", version, err)
	}
	if _, err := database.Exec(ctx, `INSERT INTO usage_session_routes(client,session_id,observed_at,provider,multiplier,via_wrapper,hook_event,source,quality,semantic_key) VALUES('codex','s','2026-08-03T00:00:00Z','official','1',0,'SessionStart','resume','estimated','route-1')`); err != nil {
		t.Fatalf("insert route: %v", err)
	}
	if _, err := database.Exec(ctx, `INSERT INTO usage_runs(client,provider,multiplier,started_at,exact,ambiguity_reason) VALUES('codex','official','1','2026-08-03T00:00:00Z',0,'managed_client_overlap'),('codex','official','1','2026-08-03T00:00:01Z',0,'managed_client_overlap')`); err != nil {
		t.Fatalf("managed overlap was rejected: %v", err)
	}
}
