package scanruntime

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/credentialvault"
	"github.com/kitdine/agent-deck/internal/desktop"
	"github.com/kitdine/agent-deck/internal/provider"
	"github.com/kitdine/agent-deck/internal/store"
)

func TestServingPublicationsUseTheCredentialVault(t *testing.T) {
	for _, serving := range []bool{false, true} {
		t.Run(map[bool]string{false: "ordinary", true: "serving"}[serving], func(t *testing.T) {
			ctx := context.Background()
			home := t.TempDir()
			root := filepath.Join(home, ".agentdeck")
			database, err := store.Open(ctx, root)
			if err != nil {
				t.Fatal(err)
			}
			vault := credentialvault.New(root, func(context.Context) (string, error) { return "synthetic-machine", nil })
			_, err = (provider.Service{Store: database, Vault: vault}).Add(ctx, provider.Definition{Name: "synthetic", Endpoint: "https://example.invalid", Clients: []provider.Client{provider.ClientCodex}}, "synthetic-secret")
			database.Close()
			if err != nil {
				t.Fatal(err)
			}
			now := time.Now()
			writeServingSource(t, home, "today", now, 10)
			for pass := 0; pass < 2; pass++ {
				r := newRound(home)
				r.serving = serving
				r.servingVault = vault
				r.cacheNow = func() time.Time { return now }
				r.execute(ctx, root, false)
				result, err := r.wait(ctx, ScopeBoth)
				if err != nil || result.Usage.State != "completed" || result.Session.State != "completed" {
					t.Fatalf("pass %d round=%+v err=%v", pass, result, err)
				}
				publication, err := (desktop.Service{StateRoot: root, Home: home}).ReadServingSnapshot(ctx)
				if err != nil {
					t.Fatal(err)
				}
				found := false
				for _, check := range publication.Payload.Data.Health.Checks {
					if check.Name == "provider_credential_key" {
						found = true
						if check.Status != "ok" {
							t.Fatalf("pass %d false credential health: %+v", pass, check)
						}
					}
				}
				if !found {
					t.Fatalf("pass %d did not check stored credentials", pass)
				}
			}
		})
	}
}
