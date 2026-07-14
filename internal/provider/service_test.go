package provider

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kitdine/agent-deck/internal/platform"
	"github.com/kitdine/agent-deck/internal/store"
)

func TestServiceKeepsCredentialsOutOfStore(t *testing.T) {
	ctx := context.Background()
	database, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	secrets := platform.NewMemorySecretStore()
	service := Service{Store: database, Secrets: secrets}
	created, err := service.Add(ctx, Definition{Name: "example", Endpoint: "https://provider.example/v1", Clients: []Client{ClientCodex}, CredentialRef: "agentdeck:provider:example", Multiplier: "1.5"}, "synthetic-secret")
	if err != nil {
		t.Fatal(err)
	}
	statuses, err := service.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(statuses) != 1 || !statuses[0].Credential.Present || statuses[0].Definition.ID != created.ID {
		t.Fatalf("statuses = %#v", statuses)
	}
	if err := service.RemoveCredential(ctx, "agentdeck:provider:example"); err != nil {
		t.Fatal(err)
	}
	statuses, err = service.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if statuses[0].Credential.Present {
		t.Fatal("credential remains present")
	}
}

func TestUseWritesOnlyExplicitTemporaryConfigPath(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	database, err := store.Open(ctx, filepath.Join(root, "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	service := Service{Store: database, Secrets: platform.NewMemorySecretStore()}
	if _, err := service.Add(ctx, Definition{Name: "example", Endpoint: "https://provider.example", Clients: []Client{ClientCodex}, CredentialRef: "agentdeck:provider:example"}, "synthetic-secret"); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(root, "config.toml")
	if err := os.WriteFile(config, []byte("model = 'keep'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := service.Use(ctx, "example", ClientCodex, config, filepath.Join(root, "backup.toml")); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(config)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(contents), "provider.example/v1") {
		t.Fatalf("config = %s", contents)
	}
}

func TestUseRejectsUnsupportedClient(t *testing.T) {
	ctx := context.Background()
	database, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	service := Service{Store: database, Secrets: platform.NewMemorySecretStore()}
	if _, err := service.Add(ctx, Definition{Name: "example", Endpoint: "https://provider.example", Clients: []Client{ClientCodex}, CredentialRef: "ref"}, "synthetic-secret"); err != nil {
		t.Fatal(err)
	}
	if err := service.Use(ctx, "example", ClientClaude, filepath.Join(t.TempDir(), "settings.json"), filepath.Join(t.TempDir(), "backup.json")); err == nil {
		t.Fatal("Use succeeded for unsupported client")
	}
}

func TestEditReferenceRemovesOldCredential(t *testing.T) {
	ctx := context.Background()
	database, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	secrets := platform.NewMemorySecretStore()
	service := Service{Store: database, Secrets: secrets}
	if _, err := service.Add(ctx, Definition{Name: "example", Endpoint: "https://provider.example", Clients: []Client{ClientCodex}, CredentialRef: "old"}, "old-secret"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Edit(ctx, Definition{Name: "example", Endpoint: "https://provider.example", Clients: []Client{ClientCodex}, CredentialRef: "new"}, "new-secret"); err != nil {
		t.Fatal(err)
	}
	if exists, _ := secrets.Exists(ctx, "old"); exists {
		t.Fatal("old credential remains")
	}
	if exists, _ := secrets.Exists(ctx, "new"); !exists {
		t.Fatal("new credential missing")
	}
}
