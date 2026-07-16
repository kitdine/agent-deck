// Package credentialvault owns machine-bound authenticated credential encryption.
package credentialvault

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"golang.org/x/crypto/hkdf"

	"github.com/kitdine/agent-deck/internal/platform"
)

const (
	AlgorithmAES256GCM = "aes-256-gcm"
	KeyVersion         = 1
	keyFilename        = "credential.key"
	keyMagic           = "AGENTDECK-CREDENTIAL-KEY\n"
	seedSize           = 32
)

var (
	ErrKeyMissing               = errors.New("credential_key_missing")
	ErrKeyPermissions           = errors.New("credential_key_permissions")
	ErrKeyVersionUnsupported    = errors.New("credential_key_version_unsupported")
	ErrKeyMachineMismatch       = errors.New("credential_key_machine_mismatch")
	ErrCiphertextInvalid        = errors.New("credential_ciphertext_invalid")
	ErrMachineIdentityMissing   = errors.New("machine_identity_unavailable")
	ErrCredentialReferenceEmpty = errors.New("credential_reference_empty")
)

// MachineIdentity returns one stable, non-secret machine identifier.
type MachineIdentity func(context.Context) (string, error)

// Sealed is the complete authenticated-encryption payload persisted by SQLite.
type Sealed struct {
	Algorithm  string
	KeyVersion int
	KeyID      string
	Nonce      []byte
	Ciphertext []byte
}

// Vault lazily loads or initializes the state-root credential key.
type Vault struct {
	stateRoot       string
	machineIdentity MachineIdentity
	random          io.Reader
}

func New(stateRoot string, machineIdentity MachineIdentity) *Vault {
	return &Vault{stateRoot: stateRoot, machineIdentity: machineIdentity, random: rand.Reader}
}

func (v *Vault) KeyPath() string { return filepath.Join(v.stateRoot, keyFilename) }

func (v *Vault) Seal(ctx context.Context, reference, value string) (Sealed, error) {
	return v.seal(ctx, reference, value, true)
}

// SealExisting encrypts with an already-established key and never creates a
// replacement key file. Callers use it whenever SQLite already owns ciphertext.
func (v *Vault) SealExisting(ctx context.Context, reference, value string) (Sealed, error) {
	return v.seal(ctx, reference, value, false)
}

func (v *Vault) seal(ctx context.Context, reference, value string, create bool) (Sealed, error) {
	if reference == "" {
		return Sealed{}, ErrCredentialReferenceEmpty
	}
	key, keyID, err := v.key(ctx, create)
	if err != nil {
		return Sealed{}, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return Sealed{}, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return Sealed{}, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err = io.ReadFull(v.random, nonce); err != nil {
		return Sealed{}, fmt.Errorf("credential nonce: %w", err)
	}
	return Sealed{
		Algorithm:  AlgorithmAES256GCM,
		KeyVersion: KeyVersion,
		KeyID:      keyID,
		Nonce:      nonce,
		Ciphertext: aead.Seal(nil, nonce, []byte(value), associatedData(reference)),
	}, nil
}

func (v *Vault) Open(ctx context.Context, reference string, sealed Sealed) (string, error) {
	if sealed.Algorithm != AlgorithmAES256GCM || sealed.KeyVersion != KeyVersion {
		return "", ErrKeyVersionUnsupported
	}
	key, keyID, err := v.key(ctx, false)
	if err != nil {
		return "", err
	}
	if sealed.KeyID == "" || sealed.KeyID != keyID {
		return "", ErrKeyMachineMismatch
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(sealed.Nonce) != aead.NonceSize() {
		return "", ErrCiphertextInvalid
	}
	plaintext, err := aead.Open(nil, sealed.Nonce, sealed.Ciphertext, associatedData(reference))
	if err != nil {
		return "", ErrCiphertextInvalid
	}
	return string(plaintext), nil
}

func (v *Vault) InspectKey(ctx context.Context) (string, error) {
	_, keyID, err := v.key(ctx, false)
	return keyID, err
}

// InitializeNew atomically creates a fresh credential key and reports whether
// this caller owns the new file even when machine-key derivation later fails.
// It never reuses an existing key.
func (v *Vault) InitializeNew(ctx context.Context) (bool, error) {
	seed, err := v.createSeedExclusive()
	if err != nil {
		return false, err
	}
	_, _, err = v.deriveKey(ctx, seed)
	return true, err
}

func (v *Vault) key(ctx context.Context, create bool) ([]byte, string, error) {
	seed, err := v.loadSeed()
	if errors.Is(err, fs.ErrNotExist) && create {
		seed, err = v.createSeed()
	}
	if errors.Is(err, fs.ErrNotExist) {
		return nil, "", ErrKeyMissing
	}
	if err != nil {
		return nil, "", err
	}
	return v.deriveKey(ctx, seed)
}

func (v *Vault) deriveKey(ctx context.Context, seed []byte) ([]byte, string, error) {
	if v.machineIdentity == nil {
		return nil, "", ErrMachineIdentityMissing
	}
	machineID, err := v.machineIdentity(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", ErrMachineIdentityMissing, err)
	}
	if machineID == "" {
		return nil, "", ErrMachineIdentityMissing
	}
	reader := hkdf.New(sha256.New, seed, []byte(machineID), []byte("agentdeck/credential-key/v1"))
	key := make([]byte, 32)
	if _, err = io.ReadFull(reader, key); err != nil {
		return nil, "", err
	}
	digest := sha256.Sum256(key)
	return key, hex.EncodeToString(digest[:16]), nil
}

func (v *Vault) loadSeed() ([]byte, error) {
	info, err := os.Lstat(v.KeyPath())
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != platform.FileMode {
		return nil, ErrKeyPermissions
	}
	contents, err := os.ReadFile(v.KeyPath())
	if err != nil {
		return nil, err
	}
	if len(contents) != len(keyMagic)+1+seedSize || string(contents[:len(keyMagic)]) != keyMagic || int(contents[len(keyMagic)]) != KeyVersion {
		return nil, ErrKeyVersionUnsupported
	}
	seed := make([]byte, seedSize)
	copy(seed, contents[len(keyMagic)+1:])
	return seed, nil
}

func (v *Vault) createSeed() ([]byte, error) {
	seed, err := v.createSeedExclusive()
	if errors.Is(err, fs.ErrExist) {
		return v.loadSeed()
	}
	return seed, err
}

func (v *Vault) createSeedExclusive() ([]byte, error) {
	if err := platform.EnsureStateRoot(v.stateRoot); err != nil {
		return nil, err
	}
	seed := make([]byte, seedSize)
	if _, err := io.ReadFull(v.random, seed); err != nil {
		return nil, fmt.Errorf("credential key seed: %w", err)
	}
	contents := make([]byte, 0, len(keyMagic)+1+len(seed))
	contents = append(contents, keyMagic...)
	contents = append(contents, byte(KeyVersion))
	contents = append(contents, seed...)

	temporary, err := os.CreateTemp(v.stateRoot, ".credential.key-*")
	if err != nil {
		return nil, err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err = temporary.Chmod(platform.FileMode); err == nil {
		_, err = temporary.Write(contents)
	}
	if err == nil {
		err = temporary.Sync()
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return nil, err
	}
	err = os.Link(temporaryName, v.KeyPath())
	if err != nil {
		return nil, err
	}
	return seed, nil
}

func associatedData(reference string) []byte {
	return []byte("agentdeck/credential/v1\x00" + reference)
}
