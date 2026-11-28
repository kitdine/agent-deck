package store

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestPricingFactsMigrationFromReleasedSchemaRollsBackAndRetries(t *testing.T) {
	const schemaBeforePricing = QuotaIdentitySchemaVersion
	ctx := context.Background()
	root := t.TempDir()
	path := filepath.Join(root, "agentdeck.sqlite3")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = migrate(ctx, db, migrations[:schemaBeforePricing]); err != nil {
		t.Fatal(err)
	}
	ownershipExec(t, db, `INSERT INTO price_catalogs VALUES('legacy','official','https://openai.example/catalog','','hash','2026-01-01T00:00:00Z','2026-01-01T00:00:00Z','USD',1);
INSERT INTO model_prices VALUES('legacy','gpt-5','openai','2026-01-01T00:00:00Z','{"input":"1"}','[]');
INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,output_tokens,source_path,source_offset) VALUES('legacy','codex','s','turn','2026-11-18T00:00:00Z','gpt-5',10,3,'fixture',0);
UPDATE usage_derived_partitions SET dirty=0,active_revision=revision,payload='legacy';
UPDATE quota_cycle SET seq=4;
INSERT INTO quota_identities(client,attempt_cycle_seq) VALUES('codex',4);
INSERT INTO quota_envelopes_v2(client,cycle_seq,plan) VALUES('codex',4,'prolite');
CREATE TABLE usage_request_facts(failure_fixture INTEGER);`)

	if err = migrate(ctx, db, migrations); err == nil || !strings.Contains(err.Error(), fmt.Sprintf("migration %d", PricingFactsSchemaVersion)) {
		t.Fatalf("expected migration rollback, got %v", err)
	}
	if version, err := schemaVersion(ctx, db); err != nil || version != schemaBeforePricing {
		t.Fatalf("schema after failed transaction: %d, %v", version, err)
	}
	if count := ownershipCount(t, db, `SELECT count(*) FROM sqlite_schema WHERE name='price_rule_sets'`); count != 0 {
		t.Fatal("failed migration left its first table behind")
	}
	ownershipExec(t, db, `DROP TABLE usage_request_facts`)
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	upgraded, err := Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	defer upgraded.Close()
	if version, err := upgraded.SchemaVersion(ctx); err != nil || version != CurrentSchemaVersion {
		t.Fatalf("retried version: %d, %v", version, err)
	}
	for _, query := range []string{
		`SELECT count(*) FROM price_catalogs WHERE version='legacy' AND schema_version=1`,
		`SELECT count(*) FROM model_prices WHERE catalog_version='legacy' AND prices_json='{"input":"1"}'`,
		`SELECT count(*) FROM usage_events WHERE event_key='legacy' AND input_tokens=10 AND output_tokens=3`,
		`SELECT count(*) FROM usage_derived_partitions WHERE day='2026-11-18' AND dirty=1 AND revision=active_revision+1 AND payload='legacy'`,
		`SELECT count(*) FROM quota_identities WHERE client='codex' AND attempt_cycle_seq=4`,
		`SELECT count(*) FROM quota_envelopes_v2 WHERE client='codex' AND cycle_seq=4 AND plan='prolite'`,
		`SELECT count(*) FROM sqlite_schema WHERE type='table' AND name='quota_session_samples'`,
	} {
		if n := ownershipCount(t, upgraded.DB, query); n != 1 {
			t.Fatalf("%s = %d, want 1", query, n)
		}
	}
	if n := ownershipCount(t, upgraded.DB, `SELECT (SELECT count(*) FROM price_rule_sets)+(SELECT count(*) FROM usage_request_facts)`); n != 0 {
		t.Fatal("migration invented legacy rules or request facts")
	}
	before := ownershipCount(t, upgraded.DB, `SELECT revision FROM serving_generation WHERE singleton=1`)
	ownershipExec(t, upgraded.DB, `UPDATE usage_derived_partitions SET dirty=0;
INSERT INTO usage_request_facts VALUES('legacy','{}')`)
	if n := ownershipCount(t, upgraded.DB, `SELECT revision FROM serving_generation WHERE singleton=1`); n != before+1 {
		t.Fatal("fact-only change did not advance serving revision")
	}
	if n := ownershipCount(t, upgraded.DB, `SELECT dirty FROM usage_derived_partitions WHERE day='2026-11-18'`); n != 1 {
		t.Fatal("fact-only change did not invalidate the event's partition")
	}
}
