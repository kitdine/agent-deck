package credentialvault

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/kitdine/agent-deck/internal/platform"
)

func TestVaultRoundTripAndPrivateKey(t *testing.T) {
	root := t.TempDir()
	vault := New(root, fixedMachine("machine-a"))
	sealed, err := vault.Seal(context.Background(), "provider-default-ref", "synthetic-secret")
	if err != nil {
		t.Fatal(err)
	}
	value, err := vault.Open(context.Background(), "provider-default-ref", sealed)
	if err != nil || value != "synthetic-secret" {
		t.Fatalf("Open() = %q, %v", value, err)
	}
	info, err := os.Stat(vault.KeyPath())
	if err != nil || info.Mode().Perm() != platform.FileMode {
		t.Fatalf("credential key mode = %v, %v", info.Mode(), err)
	}
	keyContents, err := os.ReadFile(vault.KeyPath())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(keyContents), "synthetic-secret") || strings.Contains(string(sealed.Ciphertext), "synthetic-secret") {
		t.Fatal("credential plaintext persisted")
	}
}

func TestVaultFailsClosed(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	vault := New(root, fixedMachine("machine-a"))
	sealed, err := vault.Seal(ctx, "provider-default-ref", "synthetic-secret")
	if err != nil {
		t.Fatal(err)
	}

	t.Run("machine mismatch", func(t *testing.T) {
		_, err := New(root, fixedMachine("machine-b")).Open(ctx, "provider-default-ref", sealed)
		if !errors.Is(err, ErrKeyMachineMismatch) {
			t.Fatalf("Open error = %v", err)
		}
	})
	t.Run("associated data mismatch", func(t *testing.T) {
		_, err := vault.Open(ctx, "other-default-ref", sealed)
		if !errors.Is(err, ErrCiphertextInvalid) {
			t.Fatalf("Open error = %v", err)
		}
	})
	t.Run("ciphertext tamper", func(t *testing.T) {
		tampered := sealed
		tampered.Ciphertext = append([]byte(nil), sealed.Ciphertext...)
		tampered.Ciphertext[0] ^= 0xff
		_, err := vault.Open(ctx, "provider-default-ref", tampered)
		if !errors.Is(err, ErrCiphertextInvalid) {
			t.Fatalf("Open error = %v", err)
		}
	})
	t.Run("missing key", func(t *testing.T) {
		missing := New(t.TempDir(), fixedMachine("machine-a"))
		_, err := missing.Open(ctx, "provider-default-ref", sealed)
		if !errors.Is(err, ErrKeyMissing) {
			t.Fatalf("Open error = %v", err)
		}
		if _, statErr := os.Stat(missing.KeyPath()); !os.IsNotExist(statErr) {
			t.Fatalf("missing key was created: %v", statErr)
		}
		if _, err = missing.SealExisting(ctx, "provider-default-ref", "replacement"); !errors.Is(err, ErrKeyMissing) {
			t.Fatalf("SealExisting error = %v", err)
		}
		if _, statErr := os.Stat(missing.KeyPath()); !os.IsNotExist(statErr) {
			t.Fatalf("SealExisting created missing key: %v", statErr)
		}
	})
	t.Run("permissive key", func(t *testing.T) {
		if err := os.Chmod(vault.KeyPath(), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(vault.KeyPath(), platform.FileMode) })
		_, err := vault.Open(ctx, "provider-default-ref", sealed)
		if !errors.Is(err, ErrKeyPermissions) {
			t.Fatalf("Open error = %v", err)
		}
	})
	t.Run("symlink key", func(t *testing.T) {
		root := t.TempDir()
		target := root + "/seed"
		if err := os.WriteFile(target, []byte("synthetic"), platform.FileMode); err != nil {
			t.Fatal(err)
		}
		linked := New(root, fixedMachine("machine-a"))
		if err := os.Symlink(target, linked.KeyPath()); err != nil {
			t.Fatal(err)
		}
		if _, err := linked.InspectKey(ctx); !errors.Is(err, ErrKeyPermissions) {
			t.Fatalf("InspectKey symlink error = %v", err)
		}
	})
}

func TestVaultConcurrentInitializationUsesOneKey(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	vaults := []*Vault{New(root, fixedMachine("machine-a")), New(root, fixedMachine("machine-a"))}
	sealed := make([]Sealed, len(vaults))
	errs := make([]error, len(vaults))
	var wait sync.WaitGroup
	for i := range vaults {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			sealed[index], errs[index] = vaults[index].Seal(ctx, "provider-"+string(rune('a'+index))+"-ref", "secret")
		}(i)
	}
	wait.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("Seal %d error = %v", i, err)
		}
	}
	if sealed[0].KeyID != sealed[1].KeyID {
		t.Fatalf("concurrent key IDs = %q, %q", sealed[0].KeyID, sealed[1].KeyID)
	}
}

func fixedMachine(value string) MachineIdentity {
	return func(context.Context) (string, error) { return value, nil }
}
