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
	"time"

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

func TestVaultRejectsMalformedPayloadBeforeCreatingOrUsingKey(t *testing.T) {
	ctx := context.Background()

	t.Run("empty reference before key creation", func(t *testing.T) {
		vault := New(t.TempDir(), fixedMachine("machine-a"))
		_, err := vault.Seal(ctx, "", "")
		if err != ErrCredentialReferenceEmpty {
			t.Fatalf("Seal() error = %v, want ErrCredentialReferenceEmpty", err)
		}
		if _, statErr := os.Stat(vault.KeyPath()); !os.IsNotExist(statErr) {
			t.Fatalf("Seal() created a key for an empty reference: %v", statErr)
		}
	})

	t.Run("SealExisting empty reference before missing key", func(t *testing.T) {
		vault := New(t.TempDir(), fixedMachine("machine-a"))
		_, err := vault.SealExisting(ctx, "", "")
		if err != ErrCredentialReferenceEmpty {
			t.Fatalf("SealExisting() error = %v, want ErrCredentialReferenceEmpty", err)
		}
		if _, statErr := os.Stat(vault.KeyPath()); !os.IsNotExist(statErr) {
			t.Fatalf("SealExisting() created a key for an empty reference: %v", statErr)
		}
	})

	t.Run("unsupported metadata before key creation", func(t *testing.T) {
		cases := []struct {
			name   string
			sealed Sealed
		}{
			{
				name: "algorithm",
				sealed: Sealed{
					Algorithm:  "unsupported",
					KeyVersion: KeyVersion,
				},
			},
			{
				name: "key version",
				sealed: Sealed{
					Algorithm:  AlgorithmAES256GCM,
					KeyVersion: KeyVersion + 1,
				},
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				vault := New(t.TempDir(), fixedMachine("machine-a"))
				value, err := vault.Open(ctx, "synthetic-reference", tc.sealed)
				if err != ErrKeyVersionUnsupported {
					t.Fatalf("Open() error = %v, want ErrKeyVersionUnsupported", err)
				}
				if value != "" {
					t.Fatal("Open() returned data for unsupported payload metadata")
				}
				if _, statErr := os.Stat(vault.KeyPath()); !os.IsNotExist(statErr) {
					t.Fatalf("Open() created a key for unsupported payload metadata: %v", statErr)
				}
			})
		}
	})

	root := t.TempDir()
	vault := New(root, fixedMachine("machine-a"))
	vault.random = bytes.NewReader(syntheticSeedA)
	if created, err := vault.InitializeNew(ctx); !created || err != nil {
		t.Fatalf("InitializeNew() = %v, %v", created, err)
	}
	keyID, err := vault.InspectKey(ctx)
	if err != nil {
		t.Fatal(err)
	}
	keyBefore, err := os.ReadFile(vault.KeyPath())
	if err != nil {
		t.Fatal(err)
	}

	invalidCiphertext := []byte{0xD3, 0x41, 0x7E, 0x02}
	validNonceLength := bytes.Repeat([]byte{0x5C}, 12)
	cases := []struct {
		name   string
		sealed Sealed
		err    error
	}{
		{
			name: "empty key ID",
			sealed: Sealed{
				Algorithm: AlgorithmAES256GCM, KeyVersion: KeyVersion,
				Nonce: validNonceLength, Ciphertext: invalidCiphertext,
			},
			err: ErrKeyMachineMismatch,
		},
		{
			name: "wrong key ID",
			sealed: Sealed{
				Algorithm: AlgorithmAES256GCM, KeyVersion: KeyVersion, KeyID: keyID + "-other",
				Nonce: validNonceLength, Ciphertext: invalidCiphertext,
			},
			err: ErrKeyMachineMismatch,
		},
		{
			name: "short nonce",
			sealed: Sealed{
				Algorithm: AlgorithmAES256GCM, KeyVersion: KeyVersion, KeyID: keyID,
				Nonce: validNonceLength[:len(validNonceLength)-1], Ciphertext: invalidCiphertext,
			},
			err: ErrCiphertextInvalid,
		},
		{
			name: "long nonce",
			sealed: Sealed{
				Algorithm: AlgorithmAES256GCM, KeyVersion: KeyVersion, KeyID: keyID,
				Nonce: append(append([]byte(nil), validNonceLength...), 0x01), Ciphertext: invalidCiphertext,
			},
			err: ErrCiphertextInvalid,
		},
		{
			name: "truncated ciphertext",
			sealed: Sealed{
				Algorithm: AlgorithmAES256GCM, KeyVersion: KeyVersion, KeyID: keyID,
				Nonce: validNonceLength, Ciphertext: invalidCiphertext[:1],
			},
			err: ErrCiphertextInvalid,
		},
		{
			name: "nil ciphertext",
			sealed: Sealed{
				Algorithm: AlgorithmAES256GCM, KeyVersion: KeyVersion, KeyID: keyID,
				Nonce: validNonceLength, Ciphertext: nil,
			},
			err: ErrCiphertextInvalid,
		},
		{
			name: "zero-length ciphertext",
			sealed: Sealed{
				Algorithm: AlgorithmAES256GCM, KeyVersion: KeyVersion, KeyID: keyID,
				Nonce: validNonceLength, Ciphertext: []byte{},
			},
			err: ErrCiphertextInvalid,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			value, err := vault.Open(ctx, "synthetic-reference", tc.sealed)
			if err != tc.err {
				t.Fatalf("Open() error = %v, want %v", err, tc.err)
			}
			if value != "" {
				t.Fatal("Open() returned data for an unauthenticated payload")
			}
			assertKeyBytesUnchanged(t, vault.KeyPath(), keyBefore)
		})
	}
}

func TestVaultRejectsMalformedKeyFilesWithoutReplacement(t *testing.T) {
	ctx := context.Background()
	valid := append(append([]byte(nil), []byte(keyMagic)...), byte(KeyVersion))
	valid = append(valid, syntheticSeedA...)

	badMagic := append([]byte(nil), valid...)
	badMagic[0] ^= 0xff
	badVersion := append([]byte(nil), valid...)
	badVersion[len(keyMagic)] = byte(KeyVersion + 1)
	oversized := append(append([]byte(nil), valid...), 0xC4)
	cases := []struct {
		name     string
		contents []byte
	}{
		{name: "truncated key", contents: valid[:len(valid)-1]},
		{name: "oversized key", contents: oversized},
		{name: "bad magic", contents: badMagic},
		{name: "unsupported key version", contents: badVersion},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			vault := New(root, fixedMachine("machine-a"))
			if err := os.WriteFile(vault.KeyPath(), tc.contents, platform.FileMode); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(vault.KeyPath())
			if err != nil {
				t.Fatal(err)
			}

			_, err = vault.Seal(ctx, "synthetic-reference", "")
			if err != ErrKeyVersionUnsupported {
				t.Fatalf("Seal() error = %v, want ErrKeyVersionUnsupported", err)
			}
			assertKeyBytesUnchanged(t, vault.KeyPath(), before)
		})
	}
}

func assertKeyBytesUnchanged(t *testing.T, path string, before []byte) {
	t.Helper()
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("credential key bytes changed after a rejected operation")
	}
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

func TestVaultSyncsKeyDirectoryAfterLink(t *testing.T) {
	root := t.TempDir()
	vault := New(root, fixedMachine("machine-a"))
	var calls int
	vault.syncStateRoot = func(path string) error {
		calls++
		if path != root {
			t.Errorf("sync state root = %q, want %q", path, root)
		}
		if _, err := os.Stat(vault.KeyPath()); err != nil {
			t.Errorf("directory sync before credential key link: %v", err)
		}
		return nil
	}

	if _, err := vault.Seal(context.Background(), "provider-default-ref", "synthetic-secret"); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("directory sync calls = %d, want 1", calls)
	}
}

func TestVaultConcurrentSealWaitsForDirectorySync(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	winner := New(root, fixedMachine("machine-a"))
	loser := New(root, fixedMachine("machine-a"))
	winnerSyncStarted := make(chan struct{})
	releaseWinnerSync := make(chan struct{})
	loserSyncStarted := make(chan struct{})
	releaseLoserSync := make(chan struct{})
	release := func(done chan struct{}) {
		select {
		case <-done:
		default:
			close(done)
		}
	}
	t.Cleanup(func() {
		release(releaseWinnerSync)
		release(releaseLoserSync)
	})

	winner.syncStateRoot = func(string) error {
		close(winnerSyncStarted)
		<-releaseWinnerSync
		return nil
	}
	loser.syncStateRoot = func(string) error {
		close(loserSyncStarted)
		<-releaseLoserSync
		return nil
	}
	winnerResult := make(chan error, 1)
	go func() {
		_, err := winner.Seal(ctx, "winner-ref", "winner-secret")
		winnerResult <- err
	}()
	select {
	case <-winnerSyncStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("winner did not reach directory sync")
	}

	loserResult := make(chan error, 1)
	go func() {
		_, err := loser.Seal(ctx, "loser-ref", "loser-secret")
		loserResult <- err
	}()
	select {
	case err := <-loserResult:
		t.Fatalf("loser Seal() returned before directory sync: %v", err)
	case <-loserSyncStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("loser did not reach directory sync")
	}

	release(releaseLoserSync)
	select {
	case err := <-loserResult:
		if err != nil {
			t.Fatalf("loser Seal() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("loser Seal() did not return after directory sync")
	}
	release(releaseWinnerSync)
	select {
	case err := <-winnerResult:
		if err != nil {
			t.Fatalf("winner Seal() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("winner Seal() did not return after directory sync")
	}
}

func TestVaultDirectorySyncFailurePreservesRecoverableKey(t *testing.T) {
	ctx := context.Background()
	syncErr := errors.New("synthetic-directory-sync-failure")

	tests := []struct {
		name   string
		create func(*Vault) error
	}{
		{
			name: "create seed exclusive",
			create: func(vault *Vault) error {
				_, err := vault.createSeedExclusive()
				return err
			},
		},
		{
			name: "create seed",
			create: func(vault *Vault) error {
				_, err := vault.createSeed()
				return err
			},
		},
		{
			name: "initialize new",
			create: func(vault *Vault) error {
				_, err := vault.InitializeNew(ctx)
				return err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			vault := New(root, fixedMachine("machine-a"))
			vault.random = bytes.NewReader(syntheticSeedA)
			vault.syncStateRoot = func(string) error { return syncErr }

			if err := test.create(vault); !errors.Is(err, syncErr) {
				t.Fatalf("create error = %v, want it wrap %v", err, syncErr)
			}
			info, err := os.Stat(vault.KeyPath())
			if err != nil {
				t.Fatalf("credential key missing after directory sync failure: %v", err)
			}
			if info.Mode().Perm() != platform.FileMode {
				t.Fatalf("credential key mode = %v, want %v", info.Mode().Perm(), platform.FileMode)
			}

			_, wantKeyID, err := vault.deriveKey(ctx, syntheticSeedA)
			if err != nil {
				t.Fatal(err)
			}
			recovered := New(root, fixedMachine("machine-a"))
			gotKeyID, err := recovered.InspectKey(ctx)
			if err != nil {
				t.Fatalf("InspectKey() after directory sync failure = %v", err)
			}
			if gotKeyID != wantKeyID {
				t.Fatalf("recovered key ID = %q, want %q", gotKeyID, wantKeyID)
			}
		})
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
