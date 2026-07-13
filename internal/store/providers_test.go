package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

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

func TestProviderHistoryPreventsDeletion(t *testing.T) {
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
	if err := s.DeleteProvider(ctx, created.Name); err == nil {
		t.Fatal("DeleteProvider succeeded despite selection history")
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

func TestUpdateProviderReplacesMappings(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.CreateProvider(ctx, Provider{Name: "example", Endpoint: "https://one.example", CredentialRef: "ref", Multiplier: "1", Clients: []ClientMapping{{Client: "codex"}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateProvider(ctx, Provider{Name: "example", Endpoint: "https://two.example", CredentialRef: "ref", Multiplier: "2", Clients: []ClientMapping{{Client: "claude"}}}); err != nil {
		t.Fatal(err)
	}
	got, err := s.ProviderByName(ctx, "example")
	if err != nil {
		t.Fatal(err)
	}
	if got.Endpoint != "https://two.example" || got.Multiplier != "2" || len(got.Clients) != 1 || got.Clients[0].Client != "claude" {
		t.Fatalf("provider = %#v", got)
	}
}
