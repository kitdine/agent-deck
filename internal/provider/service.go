package provider

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/kitdine/agent-deck/internal/platform"
	"github.com/kitdine/agent-deck/internal/store"
)

type Service struct {
	Store   *store.Store
	Secrets platform.SecretStore
}

type Credential struct {
	Reference string `json:"reference"`
	Present   bool   `json:"present"`
}

type Status struct {
	Definition store.Provider `json:"definition"`
	Credential Credential     `json:"credential"`
}

// ConfigDrift counts selected clients whose native endpoint no longer matches
// the recorded provider. It does not expose native configuration values.
func (s Service) ConfigDrift(ctx context.Context, home string) (int, error) {
	rows, err := s.Store.DB.QueryContext(ctx, `SELECT ps.client,p.endpoint FROM provider_selections ps JOIN providers p ON p.id=ps.provider_id WHERE ps.id IN (SELECT MAX(id) FROM provider_selections GROUP BY client) ORDER BY ps.client`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	drift := 0
	for rows.Next() {
		var client, endpoint string
		if err = rows.Scan(&client, &endpoint); err != nil {
			return 0, err
		}
		path := map[string]string{
			string(ClientCodex):  filepath.Join(home, ".codex", "config.toml"),
			string(ClientClaude): filepath.Join(home, ".claude", "settings.json"),
		}[client]
		matches, checkErr := ConfigMatchesEndpoint(Client(client), path, endpoint)
		if checkErr != nil || !matches {
			drift++
		}
	}
	return drift, rows.Err()
}

func (s Service) Add(ctx context.Context, definition Definition, credential string) (store.Provider, error) {
	validated, err := Validate(definition)
	if err != nil {
		return store.Provider{}, err
	}
	credential = strings.TrimSpace(credential)
	if credential == "" {
		return store.Provider{}, fmt.Errorf("%w: credential", ErrInvalidProvider)
	}
	if err := s.Secrets.Put(ctx, validated.CredentialRef, credential); err != nil {
		return store.Provider{}, err
	}
	created, err := s.Store.CreateProvider(ctx, store.Provider{Name: validated.Name, Endpoint: validated.Endpoint, CredentialRef: validated.CredentialRef, Multiplier: validated.Multiplier, Clients: mappings(validated)})
	if err != nil {
		_ = s.Secrets.Delete(ctx, validated.CredentialRef)
		return store.Provider{}, err
	}
	return created, nil
}

func (s Service) List(ctx context.Context) ([]Status, error) {
	providers, err := s.Store.ListProviders(ctx)
	if err != nil {
		return nil, err
	}
	statuses := make([]Status, 0, len(providers))
	for _, definition := range providers {
		present, err := s.Secrets.Exists(ctx, definition.CredentialRef)
		if err != nil {
			return nil, err
		}
		statuses = append(statuses, Status{Definition: definition, Credential: Credential{Reference: definition.CredentialRef, Present: present}})
	}
	return statuses, nil
}

func (s Service) UpdateCredential(ctx context.Context, reference, credential string) error {
	if strings.TrimSpace(reference) == "" || strings.TrimSpace(credential) == "" {
		return fmt.Errorf("%w: credential", ErrInvalidProvider)
	}
	return s.Secrets.Put(ctx, reference, credential)
}

func (s Service) Edit(ctx context.Context, definition Definition, credential string) (store.Provider, error) {
	validated, err := Validate(definition)
	if err != nil {
		return store.Provider{}, err
	}
	existing, err := s.Store.ProviderByName(ctx, validated.Name)
	if err != nil {
		return store.Provider{}, err
	}
	if existing.CredentialRef != validated.CredentialRef && credential == "" {
		return store.Provider{}, fmt.Errorf("%w: credential required when changing reference", ErrInvalidProvider)
	}
	if credential != "" {
		if err := s.UpdateCredential(ctx, validated.CredentialRef, credential); err != nil {
			return store.Provider{}, err
		}
	}
	updated, err := s.Store.UpdateProvider(ctx, store.Provider{Name: validated.Name, Endpoint: validated.Endpoint, CredentialRef: validated.CredentialRef, Multiplier: validated.Multiplier, Clients: mappings(validated)})
	if err != nil {
		return store.Provider{}, err
	}
	if existing.CredentialRef != validated.CredentialRef {
		if err := s.Secrets.Delete(ctx, existing.CredentialRef); err != nil {
			return store.Provider{}, err
		}
	}
	return updated, nil
}

func (s Service) RemoveCredential(ctx context.Context, reference string) error {
	return s.Secrets.Delete(ctx, reference)
}

func (s Service) Remove(ctx context.Context, name, credentialRef string) error {
	if _, err := s.Secrets.Get(ctx, credentialRef); err != nil {
		return err
	}
	if err := s.Store.DeleteProvider(ctx, name); err != nil {
		return err
	}
	return s.Secrets.Delete(ctx, credentialRef)
}

func (s Service) Use(ctx context.Context, name string, client Client, configPath, backupPath string) error {
	definition, err := s.Store.ProviderByName(ctx, name)
	if err != nil {
		return err
	}
	supported := false
	for _, mapping := range definition.Clients {
		if mapping.Client == string(client) {
			supported = true
			break
		}
	}
	if !supported {
		return fmt.Errorf("%w: provider does not support client", ErrInvalidProvider)
	}
	credential, err := s.Secrets.Get(ctx, definition.CredentialRef)
	if err != nil {
		return err
	}
	opID, err := operationID()
	if err != nil {
		return err
	}
	fingerprint, err := ConfigFingerprint(configPath)
	if err != nil {
		return err
	}
	if err := WriteRedactedBackup(client, configPath, backupPath); err != nil {
		return err
	}
	if err := s.Store.CreateOperation(ctx, store.Operation{ID: opID, Kind: "provider.use", State: "prepared", ProviderID: &definition.ID, Client: string(client), RedactedBackupPath: backupPath, ConfigFingerprint: fingerprint}); err != nil {
		return err
	}
	currentFingerprint, err := ConfigFingerprint(configPath)
	if err != nil {
		return err
	}
	if currentFingerprint != fingerprint {
		_ = s.Store.UpdateOperation(ctx, opID, "failed", "config_fingerprint_conflict")
		return fmt.Errorf("config_fingerprint_conflict")
	}
	config := ClientConfig{Name: definition.Name, Endpoint: definition.Endpoint, Credential: credential}
	if client == ClientCodex {
		err = WriteCodexConfig(configPath, config)
	} else if client == ClientClaude {
		err = WriteClaudeConfig(configPath, config)
	} else {
		err = fmt.Errorf("%w: client", ErrInvalidProvider)
	}
	if err != nil {
		_ = s.Store.UpdateOperation(ctx, opID, "failed", "config_write_failed")
		return err
	}
	if err := s.Store.UpdateOperation(ctx, opID, "external_written", ""); err != nil {
		return err
	}
	if err := s.Store.RecordSelection(ctx, store.Selection{ProviderID: definition.ID, Client: string(client), MultiplierSnapshot: definition.Multiplier}); err != nil {
		_ = s.Store.UpdateOperation(ctx, opID, "failed", "selection_write_failed")
		return err
	}
	if err := s.Store.UpdateOperation(ctx, opID, "database_committed", ""); err != nil {
		return err
	}
	return s.Store.UpdateOperation(ctx, opID, "completed", "")
}

// Recover marks operations that never reached an external write as failed.
// Operations that changed a client file remain visible for explicit operator
// recovery because the backup intentionally excludes credential values.
func (s Service) Recover(ctx context.Context) ([]store.Operation, error) {
	operations, err := s.Store.PendingOperations(ctx)
	if err != nil {
		return nil, err
	}
	for _, operation := range operations {
		if operation.State == "prepared" {
			if err := s.Store.UpdateOperation(ctx, operation.ID, "failed", "interrupted_before_external_write"); err != nil {
				return nil, err
			}
		}
	}
	return s.Store.PendingOperations(ctx)
}

func operationID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func mappings(definition Definition) []store.ClientMapping {
	mappings := make([]store.ClientMapping, 0, len(definition.Clients))
	for _, client := range definition.Clients {
		mappings = append(mappings, store.ClientMapping{Client: string(client)})
	}
	return mappings
}
