package credentialvault

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/kitdine/agent-deck/internal/platform"
)

// syntheticSeedA and syntheticSeedB are fixed, non-secret 32-byte sequences
// standing in for a credential-key seed so tests can assert exact key-file
// preservation without depending on the package's real random source.
var (
	syntheticSeedA = bytes.Repeat([]byte{0xA1}, 32)
	syntheticSeedB = bytes.Repeat([]byte{0xB2}, 32)
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

func TestInitializeNewDoesNotOverwriteExistingKey(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	vault := New(root, fixedMachine("machine-a"))
	vault.random = bytes.NewReader(syntheticSeedA)

	created, err := vault.InitializeNew(ctx)
	if !created || err != nil {
		t.Fatalf("first InitializeNew() = %v, %v", created, err)
	}
	info, err := os.Stat(vault.KeyPath())
	if err != nil || info.Mode().Perm() != platform.FileMode {
		t.Fatalf("credential key mode = %v, %v", info.Mode(), err)
	}
	original, err := os.ReadFile(vault.KeyPath())
	if err != nil {
		t.Fatal(err)
	}

	vault.random = bytes.NewReader(syntheticSeedB)
	created, err = vault.InitializeNew(ctx)
	if created {
		t.Fatal("InitializeNew() reported created = true for an already-initialized key")
	}
	if !errors.Is(err, fs.ErrExist) {
		t.Fatalf("InitializeNew() error = %v, want fs.ErrExist", err)
	}
	if strings.Contains(err.Error(), hex.EncodeToString(syntheticSeedB)) || strings.Contains(err.Error(), fmt.Sprintf("%v", syntheticSeedB)) {
		t.Fatal("InitializeNew() error exposed the rejected replacement key material")
	}

	after, err := os.ReadFile(vault.KeyPath())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(original, after) {
		t.Fatal("existing credential key contents changed after a repeat InitializeNew call")
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("state root contains %d entries after a rejected InitializeNew, want 1", len(entries))
	}
}

func TestInitializeNewFailsWithoutUsableKey(t *testing.T) {
	ctx := context.Background()

	t.Run("random source failure", func(t *testing.T) {
		root := t.TempDir()
		vault := New(root, fixedMachine("machine-a"))
		vault.random = errorReader{err: errors.New("synthetic-random-failure")}

		created, err := vault.InitializeNew(ctx)
		if created {
			t.Fatal("InitializeNew() reported created = true after a random-source failure")
		}
		if err == nil {
			t.Fatal("InitializeNew() error = nil, want a random-source failure")
		}
		if _, statErr := os.Stat(vault.KeyPath()); !os.IsNotExist(statErr) {
			t.Fatalf("credential key was created after a random-source failure: %v", statErr)
		}
	})

	t.Run("missing machine identity", func(t *testing.T) {
		root := t.TempDir()
		vault := New(root, nil)
		vault.random = bytes.NewReader(syntheticSeedA)

		created, err := vault.InitializeNew(ctx)
		if !created {
			t.Fatalf("InitializeNew() created = false, want true once the key file is written")
		}
		if !errors.Is(err, ErrMachineIdentityMissing) {
			t.Fatalf("InitializeNew() error = %v, want ErrMachineIdentityMissing", err)
		}
		info, statErr := os.Stat(vault.KeyPath())
		if statErr != nil {
			t.Fatalf("created = true but no key file exists: %v", statErr)
		}
		if info.Mode().Perm() != platform.FileMode {
			t.Fatalf("credential key mode = %v, want %v", info.Mode().Perm(), platform.FileMode)
		}
		if _, sealErr := vault.Seal(ctx, "provider-default-ref", "synthetic-secret"); !errors.Is(sealErr, ErrMachineIdentityMissing) {
			t.Fatalf("Seal() on the same vault = %v, want the key to stay unusable", sealErr)
		}
	})

	t.Run("machine identity error", func(t *testing.T) {
		root := t.TempDir()
		identityErr := errors.New("synthetic-identity-failure")
		vault := New(root, func(context.Context) (string, error) { return "", identityErr })
		vault.random = bytes.NewReader(syntheticSeedA)

		created, err := vault.InitializeNew(ctx)
		if !created {
			t.Fatalf("InitializeNew() created = false, want true once the key file is written")
		}
		if !errors.Is(err, ErrMachineIdentityMissing) {
			t.Fatalf("InitializeNew() error = %v, want it to wrap ErrMachineIdentityMissing", err)
		}
		info, statErr := os.Stat(vault.KeyPath())
		if statErr != nil {
			t.Fatalf("created = true but no key file exists: %v", statErr)
		}
		if info.Mode().Perm() != platform.FileMode {
			t.Fatalf("credential key mode = %v, want %v", info.Mode().Perm(), platform.FileMode)
		}
		if strings.Contains(err.Error(), hex.EncodeToString(syntheticSeedA)) || strings.Contains(err.Error(), fmt.Sprintf("%v", syntheticSeedA)) {
			t.Fatal("InitializeNew() error exposed credential key material")
		}
	})
}

func TestInitializeNewPreservesCreatedKeyForRecovery(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	identityErr := errors.New("synthetic-identity-failure")
	failing := New(root, func(context.Context) (string, error) { return "", identityErr })
	failing.random = bytes.NewReader(syntheticSeedA)

	created, err := failing.InitializeNew(ctx)
	if !created {
		t.Fatalf("InitializeNew() created = false, want true once the key file is written")
	}
	if !errors.Is(err, ErrMachineIdentityMissing) {
		t.Fatalf("InitializeNew() error = %v, want it to wrap ErrMachineIdentityMissing", err)
	}

	info, statErr := os.Stat(failing.KeyPath())
	if statErr != nil {
		t.Fatalf("credential key missing after a post-creation failure: %v", statErr)
	}
	if info.Mode().Perm() != platform.FileMode {
		t.Fatalf("credential key mode = %v, want %v", info.Mode().Perm(), platform.FileMode)
	}
	beforeRecovery, err := os.ReadFile(failing.KeyPath())
	if err != nil {
		t.Fatal(err)
	}

	// A later vault on the same state root, with working machine identity,
	// must recover using the already-created key rather than needing (or
	// silently triggering) a fresh one.
	recovered := New(root, fixedMachine("machine-a"))
	sealed, err := recovered.Seal(ctx, "provider-default-ref", "synthetic-secret")
	if err != nil {
		t.Fatalf("Seal() during recovery = %v, want the preserved key to be usable", err)
	}
	if _, err = recovered.Open(ctx, "provider-default-ref", sealed); err != nil {
		t.Fatalf("Open() during recovery = %v", err)
	}

	afterRecovery, err := os.ReadFile(recovered.KeyPath())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(beforeRecovery, afterRecovery) {
		t.Fatal("credential key contents changed while recovering from a post-creation failure")
	}

	// Seal/Open round-trips on their own would pass for any key, whether or
	// not it came from the preserved seed. Bind the recovered derivation to
	// that specific seed: under the identical machine identity, a key rooted
	// in a different seed must produce a different KeyID. If derivation ever
	// stopped depending on the persisted seed, this comparison would collapse.
	recoveredKeyID, err := recovered.InspectKey(ctx)
	if err != nil {
		t.Fatalf("InspectKey() during recovery = %v", err)
	}
	otherRoot := t.TempDir()
	differentlySeeded := New(otherRoot, fixedMachine("machine-a"))
	differentlySeeded.random = bytes.NewReader(syntheticSeedB)
	if _, err := differentlySeeded.InitializeNew(ctx); err != nil {
		t.Fatalf("InitializeNew() for the comparison vault = %v", err)
	}
	otherKeyID, err := differentlySeeded.InspectKey(ctx)
	if err != nil {
		t.Fatalf("InspectKey() for the comparison vault = %v", err)
	}
	if recoveredKeyID == otherKeyID {
		t.Fatal("recovered key ID matches a differently-seeded key under the same machine identity")
	}
}

type errorReader struct{ err error }

func (r errorReader) Read([]byte) (int, error) { return 0, r.err }

func fixedMachine(value string) MachineIdentity {
	return func(context.Context) (string, error) { return value, nil }
}
