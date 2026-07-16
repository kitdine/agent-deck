package store

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRealV6FixtureMigratesCredentialOwnedMetadataToV9WithoutSecrets(t *testing.T) {
	ctx := context.Background()
	state := filepath.Join(t.TempDir(), "state")
	if err := os.MkdirAll(state, 0o700); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", filepath.Join(state, "agentdeck.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	fixture, err := os.ReadFile(filepath.Join("testdata", "agentdeck-v6.sql"))
	if err != nil {
		db.Close()
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, string(fixture)); err != nil {
		db.Close()
		t.Fatal(err)
	}
	// A selection outside every provider.use window is a genuine pre-journal
	// legacy row and must retain the NULL operation compatibility fallback.
	if _, err = db.ExecContext(ctx, "INSERT INTO provider_selections(provider_id,client,multiplier_snapshot,selected_at) VALUES(7,'codex','0.8','2026-06-30T00:00:00Z')"); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}

	migrated, err := Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	defer migrated.Close()
	credential, err := migrated.ProviderCredential(ctx, "legacy", "default")
	if err != nil || credential.CredentialRef != "legacy-default-ref" || credential.SecretRef != "legacy-ref" || credential.SecretPresent || credential.Endpoint != "https://legacy.example" || credential.Multiplier != "1.500000000000" || strings.Join(credential.Clients, ",") != "claude,codex" {
		t.Fatalf("migrated credential = %#v, %v", credential, err)
	}
	if version, versionErr := migrated.SchemaVersion(ctx); versionErr != nil || version != CurrentSchemaVersion {
		t.Fatalf("schema version = %d, %v", version, versionErr)
	}
	var secretCount int
	if err = migrated.DB.QueryRowContext(ctx, `SELECT count(*) FROM credential_secrets`).Scan(&secretCount); err != nil || secretCount != 0 {
		t.Fatalf("migrated credential secrets = %d, %v", secretCount, err)
	}
	var linked, duplicates, nullFallbacks, interruptedSelections int
	if err = migrated.DB.QueryRowContext(ctx, "SELECT count(*) FROM provider_selections WHERE operation_id IN ('previous-use','legacy-use')").Scan(&linked); err != nil {
		t.Fatal(err)
	}
	if err = migrated.DB.QueryRowContext(ctx, "SELECT count(*) FROM (SELECT operation_id FROM provider_selections WHERE operation_id IS NOT NULL GROUP BY operation_id HAVING count(*)>1)").Scan(&duplicates); err != nil {
		t.Fatal(err)
	}
	if err = migrated.DB.QueryRowContext(ctx, "SELECT count(*) FROM provider_selections WHERE operation_id IS NULL").Scan(&nullFallbacks); err != nil {
		t.Fatal(err)
	}
	if err = migrated.DB.QueryRowContext(ctx, "SELECT count(*) FROM provider_selections WHERE multiplier_snapshot='9.9'").Scan(&interruptedSelections); err != nil {
		t.Fatal(err)
	}
	if linked != 2 || duplicates != 0 || nullFallbacks != 1 || interruptedSelections != 0 {
		t.Fatalf("operation links = %d, duplicates = %d, NULL fallbacks = %d, interrupted selections = %d", linked, duplicates, nullFallbacks, interruptedSelections)
	}
	assertSnapshot := func(name string, got ProviderSnapshot, gotErr error, multiplier string) {
		t.Helper()
		if gotErr != nil || got.Name != "legacy" || got.Endpoint != "https://legacy.example" || got.Multiplier != multiplier || got.Credential != "default" || got.Official {
			t.Fatalf("%s snapshot = %#v, %v", name, got, gotErr)
		}
	}
	timeline := []struct {
		name       string
		at         time.Time
		multiplier string
	}{
		{name: "before interrupted switch", at: time.Date(2026, 7, 2, 0, 0, 2, 0, time.UTC), multiplier: "1.2"},
		{name: "after interrupted switch", at: time.Date(2026, 7, 2, 0, 0, 5, 0, time.UTC), multiplier: "1.2"},
		{name: "after later completed switch", at: time.Date(2026, 7, 2, 0, 0, 8, 0, time.UTC), multiplier: "1.5"},
	}
	for _, check := range timeline {
		got, gotErr := migrated.ProviderSnapshotAt(ctx, "codex", check.at)
		assertSnapshot(check.name, got, gotErr, check.multiplier)
	}
	snapshot, err := migrated.CurrentProviderSnapshot(ctx, "codex")
	assertSnapshot("current", snapshot, err, "1.5")
	if err = migrated.DeleteProvider(ctx, "legacy"); err != nil {
		t.Fatal(err)
	}
	for _, check := range timeline {
		got, gotErr := migrated.ProviderSnapshotAt(ctx, "codex", check.at)
		assertSnapshot(check.name+" after deletion", got, gotErr, check.multiplier)
	}
	snapshot, err = migrated.CurrentProviderSnapshot(ctx, "codex")
	assertSnapshot("current after deletion", snapshot, err, "1.5")
}

func TestV6MigrationCanonicalizesCredentialMetadataByClient(t *testing.T) {
	fixture, err := os.ReadFile(filepath.Join("testdata", "agentdeck-v6.sql"))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name         string
		endpoint     string
		clients      []string
		wantEndpoint string
	}{
		{name: "codex base", endpoint: "https://legacy.example/api/", clients: []string{"codex"}, wantEndpoint: "https://legacy.example/api"},
		{name: "codex v1", endpoint: "https://legacy.example/api/v1/", clients: []string{"codex"}, wantEndpoint: "https://legacy.example/api"},
		{name: "shared v1", endpoint: "https://legacy.example/v1", clients: []string{"claude", "codex"}, wantEndpoint: "https://legacy.example"},
		{name: "claude v1", endpoint: "https://legacy.example/api/v1/", clients: []string{"claude"}, wantEndpoint: "https://legacy.example/api/v1"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			state := filepath.Join(t.TempDir(), "state")
			if err := os.MkdirAll(state, 0o700); err != nil {
				t.Fatal(err)
			}
			db, err := sql.Open("sqlite", filepath.Join(state, "agentdeck.sqlite3"))
			if err != nil {
				t.Fatal(err)
			}
			if _, err = db.ExecContext(ctx, string(fixture)); err != nil {
				db.Close()
				t.Fatal(err)
			}
			if _, err = db.ExecContext(ctx, "UPDATE providers SET name=?,endpoint=? WHERE id=7; DELETE FROM provider_clients WHERE provider_id=7", "Legacy", test.endpoint); err != nil {
				db.Close()
				t.Fatal(err)
			}
			for _, client := range test.clients {
				if _, err = db.ExecContext(ctx, "INSERT INTO provider_clients(provider_id,client,native_model,provider_model) VALUES(7,?,'',NULL)", client); err != nil {
					db.Close()
					t.Fatal(err)
				}
			}
			if err = db.Close(); err != nil {
				t.Fatal(err)
			}

			migrated, err := Open(ctx, state)
			if err != nil {
				t.Fatal(err)
			}
			defer migrated.Close()
			credential, err := migrated.ProviderCredential(ctx, "Legacy", "default")
			if err != nil || credential.CredentialRef != "legacy-default-ref" || credential.Endpoint != test.wantEndpoint || credential.Multiplier != "1.500000000000" {
				t.Fatalf("migrated credential = %#v, %v", credential, err)
			}
		})
	}
}

func TestV6MigrationDropsSelectionsFromIncompleteProviderUseWindows(t *testing.T) {
	fixture, err := os.ReadFile(filepath.Join("testdata", "agentdeck-v6.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for _, state := range []string{"prepared", "external_written", "database_committed", "failed"} {
		t.Run(state, func(t *testing.T) {
			ctx := context.Background()
			stateRoot := filepath.Join(t.TempDir(), "state")
			if err := os.MkdirAll(stateRoot, 0o700); err != nil {
				t.Fatal(err)
			}
			db, err := sql.Open("sqlite", filepath.Join(stateRoot, "agentdeck.sqlite3"))
			if err != nil {
				t.Fatal(err)
			}
			seed := strings.Replace(string(fixture), "'external_written',7,'codex'", "'"+state+"',7,'codex'", 1)
			if _, err = db.ExecContext(ctx, seed); err != nil {
				db.Close()
				t.Fatal(err)
			}
			if err = db.Close(); err != nil {
				t.Fatal(err)
			}

			migrated, err := Open(ctx, stateRoot)
			if err != nil {
				t.Fatal(err)
			}
			defer migrated.Close()
			var interruptedSelections int
			if err = migrated.DB.QueryRowContext(ctx, "SELECT count(*) FROM provider_selections WHERE multiplier_snapshot='9.9'").Scan(&interruptedSelections); err != nil {
				t.Fatal(err)
			}
			if interruptedSelections != 0 {
				t.Fatalf("interrupted selections = %d, want 0", interruptedSelections)
			}
		})
	}
}

func TestProviderPersistenceNeverStoresCredentialValue(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	created, err := s.CreateProvider(ctx, Provider{Name: "example", Endpoint: "https://provider.example/v1", CredentialRef: "agentdeck:provider:example", Multiplier: "1.25", Clients: []ClientMapping{{Client: "codex", NativeModel: "gpt-test", ProviderModel: "provider-test"}, {Client: "claude"}}})
	if err != nil {
		t.Fatal(err)
	}
	providers, err := s.ListProviders(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(providers) != 1 || providers[0].CredentialRef != created.CredentialRef || len(providers[0].Clients) != 2 {
		t.Fatalf("providers = %#v", providers)
	}
	var sqlText string
	if err := s.DB.QueryRowContext(ctx, "SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'providers'").Scan(&sqlText); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(sqlText), "token") || strings.Contains(strings.ToLower(sqlText), "secret") || strings.Contains(strings.ToLower(sqlText), "value") {
		t.Fatalf("providers schema can hold credentials: %s", sqlText)
	}
	if err := s.DeleteProvider(ctx, created.Name); err != nil {
		t.Fatal(err)
	}
	var mappings int
	if err := s.DB.QueryRowContext(ctx, "SELECT count(*) FROM provider_clients").Scan(&mappings); err != nil {
		t.Fatal(err)
	}
	if mappings != 0 {
		t.Fatalf("mappings = %d, want 0", mappings)
	}
}

func TestProviderCredentialAndCiphertextShareOneTransactionAndCascade(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	provider, err := s.CreateProviderWithCredential(ctx,
		Provider{Name: "atomic", Endpoint: "https://atomic.example", CredentialRef: "atomic-default-ref", Multiplier: "1", Clients: []ClientMapping{{Client: "codex"}}},
		ProviderCredential{Name: "default", CredentialRef: "atomic-default-ref", Endpoint: "https://atomic.example", Multiplier: "1", Clients: []string{"codex"}},
		CredentialSecret{Algorithm: "aes-256-gcm", KeyVersion: 1, KeyID: "key", Nonce: []byte("123456789012"), Ciphertext: []byte("sealed")},
	)
	if err != nil {
		t.Fatal(err)
	}
	credential, err := s.ProviderCredential(ctx, provider.Name, "default")
	if err != nil || !credential.SecretPresent {
		t.Fatalf("credential = %#v, %v", credential, err)
	}
	if err = s.DeleteProvider(ctx, provider.Name); err != nil {
		t.Fatal(err)
	}
	var secrets int
	if err = s.DB.QueryRowContext(ctx, `SELECT count(*) FROM credential_secrets`).Scan(&secrets); err != nil || secrets != 0 {
		t.Fatalf("credential secrets after cascade = %d, %v", secrets, err)
	}

	if _, err = s.Exec(ctx, `CREATE TRIGGER reject_secret BEFORE INSERT ON credential_secrets BEGIN SELECT RAISE(FAIL,'reject secret'); END`); err != nil {
		t.Fatal(err)
	}
	_, err = s.CreateProviderWithCredential(ctx,
		Provider{Name: "rollback", Endpoint: "https://rollback.example", CredentialRef: "rollback-default-ref", Multiplier: "1", Clients: []ClientMapping{{Client: "codex"}}},
		ProviderCredential{Name: "default", CredentialRef: "rollback-default-ref", Endpoint: "https://rollback.example", Multiplier: "1", Clients: []string{"codex"}},
		CredentialSecret{Algorithm: "aes-256-gcm", KeyVersion: 1, KeyID: "key", Nonce: []byte("123456789012"), Ciphertext: []byte("sealed")},
	)
	if err == nil {
		t.Fatal("provider creation succeeded")
	}
	var providers int
	if err = s.DB.QueryRowContext(ctx, `SELECT count(*) FROM providers WHERE name='rollback'`).Scan(&providers); err != nil || providers != 0 {
		t.Fatalf("provider metadata after rollback = %d, %v", providers, err)
	}
}

func TestProviderHistorySurvivesDefinitionDeletion(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	created, err := s.CreateProvider(ctx, Provider{Name: "example", Endpoint: "https://provider.example/v1", CredentialRef: "agentdeck:provider:example", Multiplier: "1", Clients: []ClientMapping{{Client: "codex"}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RecordSelection(ctx, Selection{ProviderID: created.ID, Client: "codex", MultiplierSnapshot: "1", SelectedAt: time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteProvider(ctx, created.Name); err != nil {
		t.Fatal(err)
	}
	snapshot, err := s.CurrentProviderSnapshot(ctx, "codex")
	if err != nil || snapshot.Name != "example" || snapshot.Multiplier != "1" || snapshot.Endpoint != "https://provider.example/v1" {
		t.Fatalf("historical snapshot = %#v, %v", snapshot, err)
	}
}

func TestProviderSnapshotTracksBearerOfficialBearerOperations(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	created, err := s.CreateProvider(ctx, Provider{Name: "bearer", Endpoint: "https://provider.example", CredentialRef: "ref", Multiplier: "2", Clients: []ClientMapping{{Client: "codex"}}})
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	providerID := created.ID
	if err := s.CreateOperation(ctx, Operation{ID: "bearer-one", Kind: "provider.use", State: "completed", ProviderID: &providerID, Client: "codex", StartedAt: base, UpdatedAt: base.Add(time.Second)}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordSelection(ctx, Selection{ProviderID: created.ID, Client: "codex", MultiplierSnapshot: "2", SelectedAt: base.Add(500 * time.Millisecond)}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateOperation(ctx, Operation{ID: "official", Kind: "provider.use", State: "completed", Client: "codex", StartedAt: base.Add(2 * time.Second), UpdatedAt: base.Add(3 * time.Second)}); err != nil {
		t.Fatal(err)
	}

	current, err := s.CurrentProviderSnapshot(ctx, "codex")
	if err != nil || !current.Official || current.Name != "official" || current.Multiplier != "1" {
		t.Fatalf("current official snapshot = %#v, %v", current, err)
	}
	historical, err := s.ProviderSnapshotAt(ctx, "codex", base.Add(time.Second))
	if err != nil || historical.Official || historical.Name != "bearer" || historical.Multiplier != "2" {
		t.Fatalf("historical bearer snapshot = %#v, %v", historical, err)
	}
	duringSwitch, err := s.ProviderSnapshotAt(ctx, "codex", base.Add(2500*time.Millisecond))
	if err != nil || duringSwitch.Official || duringSwitch.Name != "bearer" {
		t.Fatalf("snapshot during official switch = %#v, %v", duringSwitch, err)
	}
	historicalOfficial, err := s.ProviderSnapshotAt(ctx, "codex", base.Add(3*time.Second))
	if err != nil || !historicalOfficial.Official || historicalOfficial.Name != "official" || historicalOfficial.Multiplier != "1" {
		t.Fatalf("historical official snapshot = %#v, %v", historicalOfficial, err)
	}

	if err := s.CreateOperation(ctx, Operation{ID: "bearer-two", Kind: "provider.use", State: "completed", ProviderID: &providerID, Client: "codex", StartedAt: base.Add(4 * time.Second), UpdatedAt: base.Add(5 * time.Second)}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordSelection(ctx, Selection{ProviderID: created.ID, Client: "codex", MultiplierSnapshot: "3", SelectedAt: base.Add(4500 * time.Millisecond)}); err != nil {
		t.Fatal(err)
	}
	current, err = s.CurrentProviderSnapshot(ctx, "codex")
	if err != nil || current.Official || current.Name != "bearer" || current.Multiplier != "3" {
		t.Fatalf("current bearer snapshot = %#v, %v", current, err)
	}
}

func TestProviderSnapshotComparesParsedTimesAcrossRFC3339NanoPrecision(t *testing.T) {
	for _, test := range []struct {
		name          string
		withOperation bool
	}{
		{name: "completed operation", withOperation: true},
		{name: "selection fallback"},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			s, err := Open(ctx, filepath.Join(t.TempDir(), "state"))
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()

			base := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
			oldProvider, err := s.CreateProvider(ctx, Provider{Name: "old", Endpoint: "https://old.example", CredentialRef: "old-ref", Multiplier: "2", Clients: []ClientMapping{{Client: "codex"}}})
			if err != nil {
				t.Fatal(err)
			}
			if err := s.RecordSelection(ctx, Selection{ProviderID: oldProvider.ID, Client: "codex", MultiplierSnapshot: "2", SelectedAt: base.Add(-time.Second)}); err != nil {
				t.Fatal(err)
			}

			if test.withOperation {
				if err := s.CreateOperation(ctx, Operation{ID: "official", Kind: "provider.use", State: "completed", Client: "codex", StartedAt: base, UpdatedAt: base.Add(500 * time.Millisecond)}); err != nil {
					t.Fatal(err)
				}
			} else {
				newProvider, createErr := s.CreateProvider(ctx, Provider{Name: "new", Endpoint: "https://new.example", CredentialRef: "new-ref", Multiplier: "3", Clients: []ClientMapping{{Client: "codex"}}})
				if createErr != nil {
					t.Fatal(createErr)
				}
				if err := s.RecordSelection(ctx, Selection{ProviderID: newProvider.ID, Client: "codex", MultiplierSnapshot: "3", SelectedAt: base.Add(500 * time.Millisecond)}); err != nil {
					t.Fatal(err)
				}
			}

			before, err := s.ProviderSnapshotAt(ctx, "codex", base)
			if err != nil || before.Official || before.Name != "old" || before.Multiplier != "2" {
				t.Fatalf("snapshot before fractional boundary = %#v, %v", before, err)
			}
			after, err := s.ProviderSnapshotAt(ctx, "codex", base.Add(500*time.Millisecond))
			if err != nil {
				t.Fatal(err)
			}
			if test.withOperation {
				if !after.Official || after.Name != "official" || after.Multiplier != "1" {
					t.Fatalf("snapshot at operation completion = %#v", after)
				}
			} else if after.Official || after.Name != "new" || after.Multiplier != "3" {
				t.Fatalf("snapshot at selection boundary = %#v", after)
			}
		})
	}
}

func TestDeleteMissingProvider(t *testing.T) {
	s, err := Open(context.Background(), filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.DeleteProvider(context.Background(), "missing"); err != sql.ErrNoRows {
		t.Fatalf("DeleteProvider error = %v", err)
	}
}
