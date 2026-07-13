package platform

import (
	"context"
	"errors"
	"testing"
)

func TestMemorySecretStore(t *testing.T) {
	store := NewMemorySecretStore()
	ctx := context.Background()
	if err := store.Put(ctx, "agentdeck:provider:example", "synthetic-secret"); err != nil {
		t.Fatal(err)
	}
	if exists, err := store.Exists(ctx, "agentdeck:provider:example"); err != nil || !exists {
		t.Fatalf("Exists = %v, %v", exists, err)
	}
	if got, err := store.Get(ctx, "agentdeck:provider:example"); err != nil || got != "synthetic-secret" {
		t.Fatalf("Get = %q, %v", got, err)
	}
	if err := store.Delete(ctx, "agentdeck:provider:example"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(ctx, "agentdeck:provider:example"); !errors.Is(err, ErrSecretNotFound) {
		t.Fatalf("Get error = %v", err)
	}
}
