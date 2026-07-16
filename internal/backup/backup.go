// Package backup creates and restores encrypted portable AgentDeck archives.
package backup

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"filippo.io/age"

	"github.com/kitdine/agent-deck/internal/credentialvault"
	"github.com/kitdine/agent-deck/internal/platform"
	"github.com/kitdine/agent-deck/internal/store"
)

const (
	ManifestSchemaVersion = 1
	manifestName          = "manifest.json"
	coreName              = "agentdeck.sqlite3"
	credentialsName       = "credentials.json"
	sessionsName          = "sessions.sqlite3"
)

var (
	ErrInvalidArchive    = errors.New("invalid_backup")
	ErrTargetNotEmpty    = errors.New("restore_target_not_empty")
	ErrDestinationExists = errors.New("backup_exists")

	initializeRestoreCredentialKey = func(ctx context.Context, vault *credentialvault.Vault) (bool, error) {
		return vault.InitializeNew(ctx)
	}
)

type Entry struct {
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type Manifest struct {
	SchemaVersion    int            `json:"schema_version"`
	AgentDeckVersion string         `json:"agentdeck_version"`
	CreatedAt        time.Time      `json:"created_at"`
	SourcePlatform   string         `json:"source_platform"`
	DatabaseSchemas  map[string]int `json:"database_schemas"`
	Included         []string       `json:"included"`
	Entries          []Entry        `json:"entries"`
}

type FileInfo struct {
	Path       string    `json:"path"`
	Size       int64     `json:"size"`
	ModifiedAt time.Time `json:"modified_at"`
}

type Credential struct {
	Reference string `json:"reference"`
	Value     string `json:"value"`
}

type Service struct {
	Core              *store.Store
	StateRoot         string
	Vault             CredentialVault
	Version           string
	Now               func() time.Time
	AfterCoreSnapshot func()
}

type CredentialVault interface {
	Seal(context.Context, string, string) (credentialvault.Sealed, error)
	Open(context.Context, string, credentialvault.Sealed) (string, error)
}

func (s Service) Create(ctx context.Context, destination, passphrase string, includeSessions bool) (Manifest, error) {
	if s.Core == nil || s.Vault == nil || passphrase == "" {
		return Manifest{}, fmt.Errorf("%w: missing backup input", ErrInvalidArchive)
	}
	staging, err := os.MkdirTemp("", "agentdeck-backup-")
	if err != nil {
		return Manifest{}, err
	}
	defer os.RemoveAll(staging)
	if err = os.Chmod(staging, platform.DirectoryMode); err != nil {
		return Manifest{}, err
	}
	coreSnapshot := filepath.Join(staging, coreName)
	if err = s.Core.Backup(ctx, coreSnapshot); err != nil {
		return Manifest{}, fmt.Errorf("snapshot core database: %w", err)
	}
	if s.AfterCoreSnapshot != nil {
		s.AfterCoreSnapshot()
	}

	entries := make(map[string][]byte)
	if entries[coreName], err = os.ReadFile(coreSnapshot); err != nil {
		return Manifest{}, err
	}
	snapshot, err := store.OpenReadOnly(ctx, staging)
	if err != nil {
		return Manifest{}, fmt.Errorf("open core snapshot: %w", err)
	}
	credentials, credentialErr := s.credentials(ctx, snapshot)
	version, versionErr := snapshot.SchemaVersion(ctx)
	closeErr := snapshot.Close()
	if credentialErr != nil {
		return Manifest{}, credentialErr
	}
	if versionErr != nil {
		return Manifest{}, versionErr
	}
	if closeErr != nil {
		return Manifest{}, closeErr
	}
	entries[credentialsName] = credentials
	if includeSessions {
		source := filepath.Join(s.StateRoot, sessionsName)
		if _, statErr := os.Stat(source); statErr == nil {
			snapshot := filepath.Join(staging, sessionsName)
			if err = store.BackupSQLiteFile(ctx, source, snapshot); err != nil {
				return Manifest{}, fmt.Errorf("snapshot session database: %w", err)
			}
			if entries[sessionsName], err = os.ReadFile(snapshot); err != nil {
				return Manifest{}, err
			}
			if err = validateSessionSnapshot(ctx, entries[sessionsName], sessionSnapshotSchemaVersion); err != nil {
				return Manifest{}, fmt.Errorf("snapshot session database: %w", err)
			}
		} else if !errors.Is(statErr, fs.ErrNotExist) {
			return Manifest{}, statErr
		}
	}

	manifest := Manifest{
		SchemaVersion:    ManifestSchemaVersion,
		AgentDeckVersion: s.Version,
		CreatedAt:        s.now(),
		SourcePlatform:   runtime.GOOS + "/" + runtime.GOARCH,
		DatabaseSchemas:  map[string]int{coreName: version},
		Included:         sortedKeys(entries),
	}
	if _, included := entries[sessionsName]; included {
		manifest.DatabaseSchemas[sessionsName] = sessionSnapshotSchemaVersion
	}
	for _, name := range manifest.Included {
		digest := sha256.Sum256(entries[name])
		manifest.Entries = append(manifest.Entries, Entry{Name: name, Size: int64(len(entries[name])), SHA256: hex.EncodeToString(digest[:])})
	}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		return Manifest{}, err
	}
	entries[manifestName] = manifestBytes
	if err = writeEncrypted(destination, passphrase, entries, manifest.CreatedAt); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func (s Service) credentials(ctx context.Context, core *store.Store) ([]byte, error) {
	providers, err := core.ListProviders(ctx)
	if err != nil {
		return nil, err
	}
	credentials := make([]Credential, 0)
	for _, provider := range providers {
		for _, item := range provider.Credentials {
			if !item.SecretPresent {
				continue
			}
			secret, secretErr := core.CredentialSecret(ctx, item.ID)
			if secretErr != nil {
				return nil, secretErr
			}
			value, openErr := s.Vault.Open(ctx, item.CredentialRef, credentialvault.Sealed{Algorithm: secret.Algorithm, KeyVersion: secret.KeyVersion, KeyID: secret.KeyID, Nonce: secret.Nonce, Ciphertext: secret.Ciphertext})
			if openErr != nil {
				return nil, fmt.Errorf("read credential reference: %w", openErr)
			}
			credentials = append(credentials, Credential{Reference: item.CredentialRef, Value: value})
		}
	}
	sort.Slice(credentials, func(i, j int) bool { return credentials[i].Reference < credentials[j].Reference })
	return json.Marshal(credentials)
}

func (s Service) Inspect(path, passphrase string) (Manifest, error) {
	manifest, _, err := readEncrypted(path, passphrase)
	return manifest, err
}

func List(directory string) ([]FileInfo, error) {
	entries, err := os.ReadDir(directory)
	if errors.Is(err, fs.ErrNotExist) {
		return []FileInfo{}, nil
	}
	if err != nil {
		return nil, err
	}
	result := make([]FileInfo, 0)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".adb" {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		result = append(result, FileInfo{Path: filepath.Join(directory, entry.Name()), Size: info.Size(), ModifiedAt: info.ModTime().UTC()})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Path < result[j].Path })
	return result, nil
}

const sessionSnapshotSchemaVersion = 1

func Restore(ctx context.Context, archivePath, targetRoot, passphrase string, machineIdentity credentialvault.MachineIdentity) (manifest Manifest, err error) {
	if machineIdentity == nil || passphrase == "" {
		return Manifest{}, fmt.Errorf("%w: missing restore input", ErrInvalidArchive)
	}
	manifest, entries, err := readEncrypted(archivePath, passphrase)
	if err != nil {
		return Manifest{}, err
	}
	var credentials []Credential
	if err = json.Unmarshal(entries[credentialsName], &credentials); err != nil {
		return Manifest{}, fmt.Errorf("%w: credentials", ErrInvalidArchive)
	}
	referenceOwners, err := validateCoreSnapshot(ctx, entries[coreName], manifest.DatabaseSchemas[coreName])
	if err != nil {
		return Manifest{}, err
	}
	if err = validateCredentials(credentials, sortedReferenceOwners(referenceOwners)); err != nil {
		return Manifest{}, err
	}
	if sessionData, included := entries[sessionsName]; included {
		if err = validateSessionSnapshot(ctx, sessionData, manifest.DatabaseSchemas[sessionsName]); err != nil {
			return Manifest{}, err
		}
	}

	createdRoot, originalMode, err := reserveEmptyTarget(targetRoot)
	if err != nil {
		return Manifest{}, err
	}
	var lock *store.Lock
	ownedPaths := make([]string, 0, 8)
	defer func() {
		failed := err != nil
		var rollbackErrs []error
		if failed {
			for index := len(ownedPaths) - 1; index >= 0; index-- {
				if removeErr := os.Remove(ownedPaths[index]); removeErr != nil && !errors.Is(removeErr, fs.ErrNotExist) {
					rollbackErrs = append(rollbackErrs, fmt.Errorf("rollback file %q: %w", ownedPaths[index], removeErr))
				}
			}
			if !createdRoot && originalMode.Perm() != platform.DirectoryMode {
				if chmodErr := os.Chmod(targetRoot, originalMode.Perm()); chmodErr != nil {
					rollbackErrs = append(rollbackErrs, fmt.Errorf("rollback state root permissions: %w", chmodErr))
				}
			}
		}
		if lock != nil {
			if releaseErr := lock.Release(); releaseErr != nil {
				rollbackErrs = append(rollbackErrs, fmt.Errorf("release restore state lock: %w", releaseErr))
			}
		}
		if failed && createdRoot {
			if removeErr := removeEmptyRestoreRoot(targetRoot); removeErr != nil {
				rollbackErrs = append(rollbackErrs, fmt.Errorf("rollback state root: %w", removeErr))
			}
		}
		if len(rollbackErrs) > 0 {
			err = errors.Join(append([]error{err}, rollbackErrs...)...)
		}
	}()
	if !createdRoot && originalMode.Perm() != platform.DirectoryMode {
		if err = os.Chmod(targetRoot, platform.DirectoryMode); err != nil {
			return Manifest{}, err
		}
	}
	lock, err = store.AcquireLock(ctx, targetRoot, 5*time.Second)
	if err != nil {
		return Manifest{}, err
	}
	if err = validateReservedTarget(targetRoot); err != nil {
		return Manifest{}, err
	}
	for _, name := range []string{coreName, sessionsName} {
		data, ok := entries[name]
		if !ok {
			continue
		}
		path := filepath.Join(targetRoot, name)
		created, writeErr := writeNewPrivateFile(path, data)
		if created {
			ownedPaths = append(ownedPaths, path)
		}
		if writeErr != nil {
			err = writeErr
			return Manifest{}, err
		}
	}
	corePath := filepath.Join(targetRoot, coreName)
	for _, suffix := range []string{"-wal", "-shm", "-journal"} {
		path := corePath + suffix
		created, reserveErr := writeNewPrivateFile(path, nil)
		if created {
			ownedPaths = append(ownedPaths, path)
		}
		if reserveErr != nil {
			err = reserveErr
			return Manifest{}, err
		}
	}
	database, openErr := store.OpenWithLockHeld(ctx, targetRoot)
	if openErr != nil {
		err = openErr
		return Manifest{}, err
	}
	vault := credentialvault.New(targetRoot, machineIdentity)
	if len(credentials) > 0 {
		keyCreated, keyErr := initializeRestoreCredentialKey(ctx, vault)
		if keyCreated {
			ownedPaths = append(ownedPaths, vault.KeyPath())
		}
		if keyErr != nil {
			_ = database.Close()
			if errors.Is(keyErr, fs.ErrExist) {
				err = fmt.Errorf("%w: credential key already exists", ErrTargetNotEmpty)
			} else {
				err = keyErr
			}
			return Manifest{}, err
		}
	}
	targetOwners := make(map[string]int64)
	targetProviders, listErr := database.ListProviders(ctx)
	if listErr != nil {
		_ = database.Close()
		err = listErr
		return Manifest{}, err
	}
	for _, provider := range targetProviders {
		for _, credential := range provider.Credentials {
			targetOwners[credential.CredentialRef] = credential.ID
		}
	}
	sealed := make([]store.CredentialSecret, 0, len(credentials))
	for _, credential := range credentials {
		credentialID, ok := targetOwners[credential.Reference]
		if !ok {
			_ = database.Close()
			err = fmt.Errorf("%w: credential ownership", ErrInvalidArchive)
			return Manifest{}, err
		}
		payload, sealErr := vault.SealExisting(ctx, credential.Reference, credential.Value)
		if sealErr != nil {
			_ = database.Close()
			err = sealErr
			return Manifest{}, err
		}
		sealed = append(sealed, store.CredentialSecret{CredentialID: credentialID, Algorithm: payload.Algorithm, KeyVersion: payload.KeyVersion, KeyID: payload.KeyID, Nonce: payload.Nonce, Ciphertext: payload.Ciphertext})
	}
	if err = database.ReplaceCredentialSecrets(ctx, sealed); err != nil {
		_ = database.Close()
		return Manifest{}, err
	}
	if err = database.Close(); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func writeEncrypted(destination, passphrase string, entries map[string][]byte, createdAt time.Time) (err error) {
	if err = ensurePrivateParent(filepath.Dir(destination)); err != nil {
		return err
	}
	recipient, err := age.NewScryptRecipient(passphrase)
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(destination), ".agentdeck-*.adb.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer func() {
		_ = temporary.Close()
		if err != nil {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err = temporary.Chmod(platform.FileMode); err != nil {
		return err
	}
	encrypted, err := age.Encrypt(temporary, recipient)
	if err != nil {
		return err
	}
	archive := tar.NewWriter(encrypted)
	ordered := sortedKeys(entries)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i] == manifestName {
			return true
		}
		if ordered[j] == manifestName {
			return false
		}
		return ordered[i] < ordered[j]
	})
	for _, name := range ordered {
		data := entries[name]
		header := &tar.Header{Name: name, Mode: int64(platform.FileMode), Size: int64(len(data)), ModTime: createdAt}
		if err = archive.WriteHeader(header); err != nil {
			return err
		}
		if _, err = archive.Write(data); err != nil {
			return err
		}
	}
	if err = archive.Close(); err != nil {
		return err
	}
	if err = encrypted.Close(); err != nil {
		return err
	}
	if err = temporary.Sync(); err != nil {
		return err
	}
	if err = temporary.Close(); err != nil {
		return err
	}
	if err = os.Link(temporaryPath, destination); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return ErrDestinationExists
		}
		return err
	}
	if err = os.Remove(temporaryPath); err != nil {
		return errors.Join(err, os.Remove(destination))
	}
	return nil
}

func readEncrypted(path, passphrase string) (Manifest, map[string][]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return Manifest{}, nil, err
	}
	defer file.Close()
	identity, err := age.NewScryptIdentity(passphrase)
	if err != nil {
		return Manifest{}, nil, fmt.Errorf("%w: passphrase", ErrInvalidArchive)
	}
	decrypted, err := age.Decrypt(file, identity)
	if err != nil {
		return Manifest{}, nil, fmt.Errorf("%w: authentication", ErrInvalidArchive)
	}
	archive := tar.NewReader(decrypted)
	entries := make(map[string][]byte)
	for {
		header, err := archive.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return Manifest{}, nil, fmt.Errorf("%w: archive", ErrInvalidArchive)
		}
		if header.Typeflag != tar.TypeReg || filepath.Base(header.Name) != header.Name {
			return Manifest{}, nil, fmt.Errorf("%w: entry name", ErrInvalidArchive)
		}
		if _, duplicate := entries[header.Name]; duplicate {
			return Manifest{}, nil, fmt.Errorf("%w: duplicate entry", ErrInvalidArchive)
		}
		data, err := io.ReadAll(archive)
		if err != nil {
			return Manifest{}, nil, fmt.Errorf("%w: entry authentication", ErrInvalidArchive)
		}
		entries[header.Name] = data
	}
	var manifest Manifest
	if err = json.Unmarshal(entries[manifestName], &manifest); err != nil || manifest.SchemaVersion != ManifestSchemaVersion {
		return Manifest{}, nil, fmt.Errorf("%w: manifest", ErrInvalidArchive)
	}
	allowed := map[string]bool{manifestName: true}
	for _, name := range manifest.Included {
		if name != coreName && name != credentialsName && name != sessionsName {
			return Manifest{}, nil, fmt.Errorf("%w: unknown component", ErrInvalidArchive)
		}
		allowed[name] = true
	}
	for name := range entries {
		if !allowed[name] {
			return Manifest{}, nil, fmt.Errorf("%w: unexpected entry", ErrInvalidArchive)
		}
	}
	if entries[coreName] == nil || entries[credentialsName] == nil {
		return Manifest{}, nil, fmt.Errorf("%w: required entry", ErrInvalidArchive)
	}
	if len(manifest.Entries) != len(manifest.Included) {
		return Manifest{}, nil, fmt.Errorf("%w: entry manifest", ErrInvalidArchive)
	}
	seen := make(map[string]bool)
	for _, entry := range manifest.Entries {
		data, ok := entries[entry.Name]
		if !ok || seen[entry.Name] || int64(len(data)) != entry.Size {
			return Manifest{}, nil, fmt.Errorf("%w: entry metadata", ErrInvalidArchive)
		}
		digest := sha256.Sum256(data)
		if !strings.EqualFold(entry.SHA256, hex.EncodeToString(digest[:])) {
			return Manifest{}, nil, fmt.Errorf("%w: entry hash", ErrInvalidArchive)
		}
		seen[entry.Name] = true
	}
	return manifest, entries, nil
}

func validateCoreSnapshot(ctx context.Context, data []byte, expected int) (map[string]int64, error) {
	root, err := os.MkdirTemp("", "agentdeck-restore-validate-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(root)
	if err = os.Chmod(root, platform.DirectoryMode); err != nil {
		return nil, err
	}
	if err = os.WriteFile(filepath.Join(root, coreName), data, platform.FileMode); err != nil {
		return nil, err
	}
	database, err := store.OpenReadOnly(ctx, root)
	if err != nil {
		return nil, fmt.Errorf("%w: core database", ErrInvalidArchive)
	}
	version, err := database.SchemaVersion(ctx)
	if err != nil || version != expected || version > store.CurrentSchemaVersion {
		return nil, fmt.Errorf("%w: database schema", ErrInvalidArchive)
	}
	if err = database.Close(); err != nil {
		return nil, err
	}
	database, err = store.Open(ctx, root)
	if err != nil {
		return nil, fmt.Errorf("%w: core database migration", ErrInvalidArchive)
	}
	defer database.Close()
	providers, err := database.ListProviders(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: provider records", ErrInvalidArchive)
	}
	referenceSet := make(map[string]int64, len(providers))
	for _, provider := range providers {
		for _, credential := range provider.Credentials {
			if credential.SecretPresent {
				referenceSet[credential.CredentialRef] = credential.ID
			}
		}
	}
	return referenceSet, nil
}

func sortedReferenceOwners(values map[string]int64) []string {
	references := make([]string, 0, len(values))
	for reference := range values {
		references = append(references, reference)
	}
	sort.Strings(references)
	return references
}

func validateSessionSnapshot(ctx context.Context, data []byte, expected int) error {
	if expected != sessionSnapshotSchemaVersion {
		return fmt.Errorf("%w: session database schema", ErrInvalidArchive)
	}
	root, err := os.MkdirTemp("", "agentdeck-restore-session-validate-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root)
	if err = os.Chmod(root, platform.DirectoryMode); err != nil {
		return err
	}
	path := filepath.Join(root, sessionsName)
	if err = os.WriteFile(path, data, platform.FileMode); err != nil {
		return err
	}
	database, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		return fmt.Errorf("%w: session database", ErrInvalidArchive)
	}
	defer database.Close()
	var integrity string
	if err = database.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&integrity); err != nil || integrity != "ok" {
		return fmt.Errorf("%w: session database integrity", ErrInvalidArchive)
	}
	for _, table := range []string{"session_sources", "session_metadata", "session_documents"} {
		var count int
		if err = database.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE name=?", table).Scan(&count); err != nil || count != 1 {
			return fmt.Errorf("%w: session database schema", ErrInvalidArchive)
		}
	}
	return nil
}

func validateCredentials(credentials []Credential, references []string) error {
	seen := make(map[string]bool, len(credentials))
	actual := make([]string, 0, len(credentials))
	for _, credential := range credentials {
		if credential.Reference == "" || credential.Value == "" || seen[credential.Reference] {
			return fmt.Errorf("%w: credential entry", ErrInvalidArchive)
		}
		seen[credential.Reference] = true
		actual = append(actual, credential.Reference)
	}
	sort.Strings(actual)
	if len(actual) != len(references) {
		return fmt.Errorf("%w: credential references", ErrInvalidArchive)
	}
	for index := range actual {
		if actual[index] != references[index] {
			return fmt.Errorf("%w: credential references", ErrInvalidArchive)
		}
	}
	return nil
}

func reserveEmptyTarget(path string) (bool, os.FileMode, error) {
	if err := os.Mkdir(path, platform.DirectoryMode); err == nil {
		return true, 0, nil
	} else if !errors.Is(err, fs.ErrExist) {
		return false, 0, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return false, 0, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return false, 0, ErrTargetNotEmpty
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return false, 0, err
	}
	if len(entries) != 0 {
		return false, 0, ErrTargetNotEmpty
	}
	return false, info.Mode(), nil
}

func validateReservedTarget(path string) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Name() != "state.lock" {
			return ErrTargetNotEmpty
		}
	}
	return nil
}

func removeEmptyRestoreRoot(path string) error {
	if err := os.Remove(path); err == nil || errors.Is(err, fs.ErrNotExist) {
		return nil
	} else {
		entries, readErr := os.ReadDir(path)
		if readErr == nil && len(entries) > 0 {
			return nil
		}
		if readErr != nil {
			return errors.Join(err, readErr)
		}
		return err
	}
}

func writeNewPrivateFile(path string, data []byte) (bool, error) {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, platform.FileMode)
	if err != nil {
		return false, err
	}
	defer file.Close()
	if _, err = file.Write(data); err != nil {
		return true, err
	}
	return true, file.Sync()
}

func ensurePrivateParent(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(path, platform.DirectoryMode); err != nil {
		return err
	}
	return os.Chmod(path, platform.DirectoryMode)
}

func sortedKeys(values map[string][]byte) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (s Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

// EncodeManifest is retained as a small deterministic contract helper.
func EncodeManifest(manifest Manifest) ([]byte, error) {
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(manifest); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}
