package provider

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/jobshen/agentdeck/internal/platform"
	"github.com/jobshen/agentdeck/internal/store"
)

func TestRecoverMarksPreparedOperationFailed(t *testing.T) {
	ctx := context.Background()
	database, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	service := Service{Store: database, Secrets: platform.NewMemorySecretStore()}
	if err := database.CreateOperation(ctx, store.Operation{ID: "prepared", Kind: "provider.use", State: "prepared"}); err != nil {
		t.Fatal(err)
	}
	operations, err := service.Recover(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(operations) != 1 || operations[0].State != "failed" || operations[0].ErrorCode != "interrupted_before_external_write" {
		t.Fatalf("operations = %#v", operations)
	}
}
