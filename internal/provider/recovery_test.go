package provider

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kitdine/agent-deck/internal/store"
)

func TestRecoverMarksPreparedOperationFailed(t *testing.T) {
	ctx := context.Background()
	database, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	service := Service{Store: database, Vault: testCredentialVault(t)}
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

func TestUseJournalTransitionsDiagnoseActualExternalState(t *testing.T) {
	for _, stage := range []struct {
		name        string
		trigger     string
		wantState   string
		wantCode    string
		failureJoin bool
	}{
		{name: "external_written transition", trigger: `CREATE TRIGGER fail_transition BEFORE UPDATE OF state ON operations WHEN NEW.state='external_written' BEGIN SELECT RAISE(FAIL,'external transition'); END`, wantState: "failed", wantCode: "external_written_transition_failed"},
		{name: "completed transition", trigger: `CREATE TRIGGER fail_transition BEFORE UPDATE OF state ON operations WHEN NEW.state='completed' BEGIN SELECT RAISE(FAIL,'completed transition'); END`, wantState: "failed", wantCode: "selection_commit_failed"},
		{name: "failure recording", trigger: `CREATE TRIGGER fail_transition BEFORE UPDATE OF state ON operations WHEN NEW.state IN ('external_written','failed') BEGIN SELECT RAISE(FAIL,'journal unavailable'); END`, wantState: "prepared", failureJoin: true},
	} {
		t.Run(stage.name, func(t *testing.T) {
			ctx := context.Background()
			root := t.TempDir()
			database, err := store.Open(ctx, filepath.Join(root, "state"))
			if err != nil {
				t.Fatal(err)
			}
			defer database.Close()
			secrets := testCredentialVault(t)
			service := Service{Store: database, Vault: secrets}
			if _, err = service.Add(ctx, Definition{Name: "journal", Endpoint: "https://example.invalid", CredentialRef: "journal-ref", Clients: []Client{ClientCodex}}, "synthetic-secret"); err != nil {
				t.Fatal(err)
			}
			config := filepath.Join(root, "config.toml")
			if err = os.WriteFile(config, []byte("model='keep'\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err = database.Exec(ctx, stage.trigger); err != nil {
				t.Fatal(err)
			}
			err = service.Use(ctx, "journal", ClientCodex, config, filepath.Join(root, "backup.toml"))
			if err == nil {
				t.Fatal("provider use unexpectedly succeeded")
			}
			if stage.failureJoin && !strings.Contains(err.Error(), "record operation failure") {
				t.Fatalf("failure recording error = %v", err)
			}
			contents, readErr := os.ReadFile(config)
			if readErr != nil || !strings.Contains(string(contents), "example.invalid") {
				t.Fatalf("external config = %q, %v", contents, readErr)
			}
			pending, pendingErr := database.PendingOperations(ctx)
			if pendingErr != nil || len(pending) != 1 || pending[0].State != stage.wantState || pending[0].ErrorCode != stage.wantCode {
				t.Fatalf("journal = %#v, %v", pending, pendingErr)
			}
			if stage.failureJoin {
				if _, err = database.Exec(ctx, "DROP TRIGGER fail_transition"); err != nil {
					t.Fatal(err)
				}
				pending, err = service.Recover(ctx)
				if err != nil || len(pending) != 1 || pending[0].State != "failed" || pending[0].ErrorCode != "interrupted_after_external_write" {
					t.Fatalf("recovered journal = %#v, %v", pending, err)
				}
			}
		})
	}
}
