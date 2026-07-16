package provider

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/kitdine/agent-deck/internal/credentialvault"
	"github.com/kitdine/agent-deck/internal/providermeta"
	"github.com/kitdine/agent-deck/internal/store"
)

type Service struct {
	Store     *store.Store
	Vault     CredentialVault
	Home      string
	StateRoot string
}

type CredentialVault interface {
	Seal(context.Context, string, string) (credentialvault.Sealed, error)
	SealExisting(context.Context, string, string) (credentialvault.Sealed, error)
	Open(context.Context, string, credentialvault.Sealed) (string, error)
	InspectKey(context.Context) (string, error)
}

type Credential struct {
	Provider   string   `json:"provider"`
	Name       string   `json:"name"`
	Reference  string   `json:"reference"`
	Endpoint   string   `json:"endpoint"`
	Multiplier string   `json:"multiplier"`
	Clients    []string `json:"clients"`
	Present    bool     `json:"present"`
}

type CredentialAddPlan struct {
	Definition     Definition
	CredentialName string
	Reference      string
	ProviderExists bool
	Noop           bool
	Provider       store.Provider
}

type Provider struct {
	ID              int64                 `json:"id,omitempty"`
	Name            string                `json:"name"`
	Clients         []store.ClientMapping `json:"clients"`
	CredentialCount int                   `json:"credential_count"`
	CreatedAt       *time.Time            `json:"created_at,omitempty"`
	UpdatedAt       *time.Time            `json:"updated_at,omitempty"`
	BuiltIn         bool                  `json:"built_in,omitempty"`
	Authentication  string                `json:"authentication,omitempty"`
}

type DefinitionResult struct {
	Definition Provider `json:"definition"`
}

type Status struct {
	Definition  Provider          `json:"definition"`
	Credentials []Credential      `json:"credentials,omitempty"`
	Ready       bool              `json:"ready"`
	Active      []ActiveSelection `json:"active,omitempty"`
}

type ActiveSelection struct {
	Client     string `json:"client"`
	Credential string `json:"credential,omitempty"`
}

const OfficialProviderName = "official"

// ConfigDrift counts selected clients whose native endpoint no longer matches
// the recorded provider. It does not expose native configuration values.
func (s Service) ConfigDrift(ctx context.Context, home string) (int, error) {
	drift := 0
	for _, client := range []Client{ClientCodex, ClientClaude} {
		snapshot, err := s.Store.CurrentProviderSnapshot(ctx, string(client))
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return 0, err
		}
		path, pathErr := defaultConfigPath(home, client)
		if pathErr != nil {
			drift++
			continue
		}
		var matches bool
		var checkErr error
		if snapshot.Official {
			matches, checkErr = ConfigMatchesOfficialCodex(path)
		} else {
			matches, checkErr = ConfigMatchesEndpoint(client, path, snapshot.Endpoint)
		}
		if checkErr != nil || !matches {
			drift++
		}
	}
	return drift, nil
}

func (s Service) Add(ctx context.Context, definition Definition, credential string) (store.Provider, error) {
	return s.AddProvider(ctx, definition, "default", credential)
}

func (s Service) AddProvider(ctx context.Context, definition Definition, credentialName, credential string) (store.Provider, error) {
	return s.AddProviderWithCredential(ctx, definition, credentialName, credential)
}

func (s Service) PlanProviderCredential(ctx context.Context, definition Definition, credentialName string) (CredentialAddPlan, error) {
	if strings.EqualFold(strings.TrimSpace(definition.Name), OfficialProviderName) {
		return CredentialAddPlan{}, fmt.Errorf("%w: %q is a built-in provider", ErrInvalidProvider, OfficialProviderName)
	}
	name, err := NormalizeCredentialName(credentialName)
	if err != nil {
		return CredentialAddPlan{}, err
	}
	reference, err := CredentialReference(definition.Name, name)
	if err != nil {
		return CredentialAddPlan{}, err
	}
	definition.CredentialRef = reference
	validated, err := Validate(definition)
	if err != nil {
		return CredentialAddPlan{}, err
	}
	plan := CredentialAddPlan{Definition: validated, CredentialName: name, Reference: reference}
	existing, lookupErr := s.Store.ProviderByName(ctx, validated.Name)
	if lookupErr != nil && !errors.Is(lookupErr, sql.ErrNoRows) {
		return CredentialAddPlan{}, lookupErr
	}
	if errors.Is(lookupErr, sql.ErrNoRows) {
		if err = s.ensureCredentialReferenceAvailable(ctx, reference); err != nil {
			return CredentialAddPlan{}, err
		}
		return plan, nil
	}
	plan.ProviderExists = true
	plan.Provider = existing
	item, credentialErr := s.Store.ProviderCredential(ctx, validated.Name, name)
	if credentialErr == nil {
		if item.CredentialRef != reference || item.Endpoint != validated.Endpoint || item.Multiplier != validated.Multiplier || !sameClients(item.Clients, validated.Clients) {
			return CredentialAddPlan{}, fmt.Errorf("%w: credential already exists with different metadata; use credential update", ErrInvalidProvider)
		}
		if !item.SecretPresent {
			return CredentialAddPlan{}, fmt.Errorf("%w: credential secret is missing; use credential update --rotate", ErrInvalidProvider)
		}
		plan.Noop = true
		return plan, nil
	}
	if !errors.Is(credentialErr, sql.ErrNoRows) {
		return CredentialAddPlan{}, credentialErr
	}
	if err = s.ensureCredentialReferenceAvailable(ctx, reference); err != nil {
		return CredentialAddPlan{}, err
	}
	return plan, nil
}

func (s Service) ensureCredentialReferenceAvailable(ctx context.Context, reference string) error {
	var conflicts int
	if err := s.Store.DB.QueryRowContext(ctx, `SELECT count(*) FROM provider_credentials WHERE logical_ref=? OR credential_ref=?`, reference, reference).Scan(&conflicts); err != nil {
		return err
	}
	if conflicts != 0 {
		return fmt.Errorf("%w: credential reference already exists", ErrInvalidProvider)
	}
	return nil
}

func sameClients(stored []string, requested []Client) bool {
	if len(stored) != len(requested) {
		return false
	}
	seen := make(map[string]bool, len(stored))
	for _, client := range stored {
		seen[client] = true
	}
	for _, client := range requested {
		if !seen[string(client)] {
			return false
		}
	}
	return true
}

func (s Service) AddProviderWithCredential(ctx context.Context, definition Definition, credentialName, credential string) (store.Provider, error) {
	plan, err := s.PlanProviderCredential(ctx, definition, credentialName)
	if err != nil {
		return store.Provider{}, err
	}
	if plan.Noop {
		return plan.Provider, nil
	}
	credential = strings.TrimSpace(credential)
	if credential == "" {
		return store.Provider{}, fmt.Errorf("%w: credential", ErrInvalidProvider)
	}
	clients := make([]string, 0, len(plan.Definition.Clients))
	for _, client := range plan.Definition.Clients {
		clients = append(clients, string(client))
	}
	credentialPlan := credentialPlan{providerName: plan.Definition.Name, name: plan.CredentialName, reference: plan.Reference, endpoint: plan.Definition.Endpoint, multiplier: plan.Definition.Multiplier, clients: clients, value: credential}
	err = s.createCredential(ctx, credentialPlan, func(item store.ProviderCredential, secret store.CredentialSecret) error {
		if plan.ProviderExists {
			_, err = s.Store.CreateCredentialWithSecret(ctx, item, secret)
			return err
		}
		_, err = s.Store.CreateProviderWithCredential(ctx, store.Provider{Name: plan.Definition.Name, Endpoint: plan.Definition.Endpoint, CredentialRef: plan.Reference, Multiplier: plan.Definition.Multiplier, Clients: mappings(plan.Definition)}, item, secret)
		return err
	})
	if err != nil {
		return store.Provider{}, err
	}
	return s.Store.ProviderByName(ctx, plan.Definition.Name)
}

func (s Service) UpdateDefinition(ctx context.Context, name, credentialName string, endpoint *string, clients []Client, multiplierValue *string) (store.Provider, error) {
	if name == OfficialProviderName {
		return store.Provider{}, fmt.Errorf("%w: %q is a built-in provider", ErrInvalidProvider, name)
	}
	resolved, err := s.ResolveCredentialName(ctx, name, credentialName)
	if err != nil {
		return store.Provider{}, err
	}
	if _, err = s.UpdateNamedCredential(ctx, name, resolved, endpoint, clients, multiplierValue, nil); err != nil {
		return store.Provider{}, err
	}
	return s.Store.ProviderByName(ctx, name)
}

func (s Service) ResolveCredentialName(ctx context.Context, providerName, credentialName string) (string, error) {
	if credentialName != "" {
		name, err := NormalizeCredentialName(credentialName)
		if err != nil {
			return "", err
		}
		if _, err = s.Store.ProviderCredential(ctx, providerName, name); err != nil {
			return "", err
		}
		return name, nil
	}
	credentials, err := s.Store.ListProviderCredentials(ctx, providerName)
	if err != nil {
		return "", err
	}
	if len(credentials) != 1 {
		return "", fmt.Errorf("%w: --credential is required when provider has multiple credentials", ErrInvalidProvider)
	}
	return credentials[0].Name, nil
}

func (s Service) RemoveProvider(ctx context.Context, name string) error {
	if name == OfficialProviderName {
		return fmt.Errorf("%w: %q is a built-in provider", ErrInvalidProvider, name)
	}
	return s.Store.DeleteProvider(ctx, name)
}

type providerUseDetails struct {
	ConfigPath string `json:"config_path"`
}

func NormalizeCredentialName(value string) (string, error) {
	name, err := providermeta.NormalizeCredentialName(value)
	if err != nil {
		return "", fmt.Errorf("%w: credential name", ErrInvalidProvider)
	}
	return name, nil
}

func CredentialReference(providerName, credentialName string) (string, error) {
	reference, err := providermeta.CredentialReference(providerName, credentialName)
	if err != nil {
		return "", fmt.Errorf("%w: credential reference", ErrInvalidProvider)
	}
	return reference, nil
}

type credentialPlan struct {
	providerName string
	name         string
	reference    string
	endpoint     string
	multiplier   string
	clients      []string
	value        string
}

func (s Service) createCredential(ctx context.Context, plan credentialPlan, persist func(store.ProviderCredential, store.CredentialSecret) error) error {
	if s.Vault == nil {
		return errors.New("credential vault is unavailable")
	}
	sealed, err := s.sealCredential(ctx, plan.reference, plan.value)
	if err != nil {
		return err
	}
	item := store.ProviderCredential{ProviderName: plan.providerName, Name: plan.name, CredentialRef: plan.reference, SecretRef: plan.reference, Endpoint: plan.endpoint, Multiplier: plan.multiplier, Clients: plan.clients}
	return persist(item, secretRecord(sealed))
}

func (s Service) AddCredential(ctx context.Context, providerName, name, endpoint, multiplier string, clients []Client, value string) (store.ProviderCredential, error) {
	definition := Definition{Name: providerName, Endpoint: endpoint, Multiplier: multiplier, Clients: clients}
	plan, err := s.PlanProviderCredential(ctx, definition, name)
	if err != nil {
		return store.ProviderCredential{}, err
	}
	if !plan.ProviderExists {
		return store.ProviderCredential{}, sql.ErrNoRows
	}
	if !plan.Noop {
		if _, err = s.AddProviderWithCredential(ctx, definition, name, value); err != nil {
			return store.ProviderCredential{}, err
		}
	}
	return s.Store.ProviderCredential(ctx, providerName, plan.CredentialName)
}

func (s Service) ListCredentials(ctx context.Context, providerName, client string) ([]Credential, error) {
	items, err := s.Store.ListProviderCredentials(ctx, providerName)
	if err != nil {
		return nil, err
	}
	out := make([]Credential, 0, len(items))
	for _, item := range items {
		if client != "" {
			found := false
			for _, binding := range item.Clients {
				if binding == client {
					found = true
				}
			}
			if !found {
				continue
			}
		}
		out = append(out, credentialDTO(item, item.SecretPresent))
	}
	return out, nil
}

func (s Service) ShowCredential(ctx context.Context, providerName, name string) (Credential, error) {
	item, err := s.Store.ProviderCredential(ctx, providerName, name)
	if err != nil {
		return Credential{}, err
	}
	return credentialDTO(item, item.SecretPresent), nil
}

func credentialDTO(item store.ProviderCredential, present bool) Credential {
	return Credential{Provider: item.ProviderName, Name: item.Name, Reference: item.CredentialRef, Endpoint: item.Endpoint, Multiplier: item.Multiplier, Clients: item.Clients, Present: present}
}

func (s Service) UpdateNamedCredential(ctx context.Context, providerName, name string, endpoint *string, clients []Client, multiplierValue *string, value *string) (Credential, error) {
	item, err := s.Store.ProviderCredential(ctx, providerName, name)
	if err != nil {
		return Credential{}, err
	}
	definition := Definition{Name: providerName, Endpoint: item.Endpoint, Multiplier: item.Multiplier, CredentialRef: item.CredentialRef}
	for _, client := range item.Clients {
		definition.Clients = append(definition.Clients, Client(client))
	}
	if endpoint != nil {
		definition.Endpoint = *endpoint
	}
	if clients != nil {
		definition.Clients = append([]Client(nil), clients...)
	}
	if multiplierValue != nil {
		definition.Multiplier = *multiplierValue
	}
	validated, err := Validate(definition)
	if err != nil {
		return Credential{}, err
	}
	item.Endpoint = validated.Endpoint
	item.Multiplier = validated.Multiplier
	item.Clients = item.Clients[:0]
	for _, client := range validated.Clients {
		item.Clients = append(item.Clients, string(client))
	}
	if value != nil && strings.TrimSpace(*value) == "" {
		return Credential{}, fmt.Errorf("%w: credential", ErrInvalidProvider)
	}
	if endpoint != nil || clients != nil || multiplierValue != nil || value != nil {
		if value == nil {
			_, err = s.Store.UpdateProviderCredential(ctx, item)
		} else {
			if s.Vault == nil {
				return Credential{}, errors.New("credential vault is unavailable")
			}
			sealed, sealErr := s.sealCredential(ctx, item.CredentialRef, *value)
			if sealErr != nil {
				return Credential{}, sealErr
			}
			_, err = s.Store.UpdateProviderCredentialWithSecret(ctx, item, secretRecord(sealed))
		}
		if err != nil {
			return Credential{}, err
		}
	}
	return s.ShowCredential(ctx, providerName, name)
}

func (s Service) RemoveNamedCredential(ctx context.Context, providerName, name string) error {
	return s.Store.DeleteProviderCredential(ctx, providerName, name)
}

func (s Service) List(ctx context.Context) ([]DefinitionResult, error) {
	stored, err := s.Store.ListProviders(ctx)
	if err != nil {
		return nil, err
	}
	providers := make([]DefinitionResult, 0, len(stored)+1)
	providers = append(providers, DefinitionResult{Definition: officialProvider()})
	for _, definition := range stored {
		providers = append(providers, DefinitionResult{Definition: storedProvider(definition)})
	}
	return providers, nil
}

func (s Service) Show(ctx context.Context, name string) (DefinitionResult, error) {
	if name == OfficialProviderName {
		return DefinitionResult{Definition: officialProvider()}, nil
	}
	definition, err := s.Store.ProviderByName(ctx, name)
	if err != nil {
		return DefinitionResult{}, err
	}
	return DefinitionResult{Definition: storedProvider(definition)}, nil
}

func (s Service) Status(ctx context.Context) ([]Status, error) {
	providers, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	statuses := make([]Status, 0, len(providers))
	for _, result := range providers {
		definition := result.Definition
		status := Status{Definition: definition, Ready: definition.BuiltIn}
		if !definition.BuiltIn {
			credentials, err := s.ListCredentials(ctx, definition.Name, "")
			if err != nil {
				return nil, err
			}
			status.Credentials = credentials
			status.Ready = len(credentials) > 0
			for _, credential := range credentials {
				status.Ready = status.Ready && credential.Present
			}
		}
		for _, client := range []Client{ClientCodex, ClientClaude} {
			snapshot, snapshotErr := s.Store.CurrentProviderSnapshot(ctx, string(client))
			if errors.Is(snapshotErr, sql.ErrNoRows) {
				continue
			}
			if snapshotErr != nil {
				return nil, snapshotErr
			}
			if snapshot.Name != definition.Name {
				continue
			}
			active := ActiveSelection{Client: string(client)}
			if !definition.BuiltIn {
				active.Credential = snapshot.Credential
			}
			status.Active = append(status.Active, active)
		}
		statuses = append(statuses, status)
	}
	return statuses, nil
}

func (s Service) Use(ctx context.Context, name string, client Client, configPath, backupPath string) error {
	return s.UseCredential(ctx, name, client, "", configPath, backupPath)
}

func (s Service) UseCredential(ctx context.Context, name string, client Client, credentialName, configPath, backupPath string) error {
	var (
		definition             *store.Provider
		credential             string
		credentialID           *int64
		selectedCredentialName string
		selectedCredential     *store.ProviderCredential
		err                    error
	)
	if client != ClientCodex && client != ClientClaude {
		return fmt.Errorf("%w: client", ErrInvalidProvider)
	}
	if name == OfficialProviderName {
		if client != ClientCodex {
			return fmt.Errorf("%w: provider does not support client", ErrInvalidProvider)
		}
	} else {
		stored, lookupErr := s.Store.ProviderByName(ctx, name)
		if lookupErr != nil {
			return lookupErr
		}
		definition = &stored
		supported := false
		for _, mapping := range stored.Clients {
			if mapping.Client == string(client) {
				supported = true
				break
			}
		}
		if !supported {
			return fmt.Errorf("%w: provider does not support client", ErrInvalidProvider)
		}
		credentials, credentialErr := s.Store.ListProviderCredentials(ctx, stored.Name)
		if credentialErr != nil {
			return credentialErr
		}
		var selected *store.ProviderCredential
		for i := range credentials {
			applies := false
			for _, binding := range credentials[i].Clients {
				if binding == string(client) {
					applies = true
				}
			}
			if !applies {
				continue
			}
			if credentialName == "" {
				if selected != nil {
					return fmt.Errorf("%w: credential is ambiguous", ErrInvalidProvider)
				}
				selected = &credentials[i]
			} else if credentials[i].Name == credentialName {
				selected = &credentials[i]
			}
		}
		if selected == nil {
			return fmt.Errorf("%w: credential", ErrInvalidProvider)
		}
		if s.Vault == nil {
			return errors.New("credential vault is unavailable")
		}
		secret, secretErr := s.Store.CredentialSecret(ctx, selected.ID)
		if secretErr != nil {
			return secretErr
		}
		credential, err = s.Vault.Open(ctx, selected.CredentialRef, sealedRecord(secret))
		if err != nil {
			return err
		}
		definition.CredentialRef = selected.CredentialRef
		credentialID = &selected.ID
		selectedCredentialName = selected.Name
		selectedCredential = selected
	}
	opID, err := operationID()
	if err != nil {
		return err
	}
	if configPath == "" {
		configPath, err = defaultConfigPath(s.Home, client)
		if err != nil {
			return err
		}
	}
	if backupPath == "" {
		backupPath, err = managedBackupPath(s.StateRoot, client, opID)
		if err != nil {
			return err
		}
	}
	fingerprint, err := ConfigFingerprint(configPath)
	if err != nil {
		return err
	}
	if err := WriteRedactedBackup(client, configPath, backupPath); err != nil {
		return err
	}
	var providerID *int64
	if definition != nil {
		providerID = &definition.ID
	}
	useDetails, err := json.Marshal(providerUseDetails{ConfigPath: configPath})
	if err != nil {
		return err
	}
	if err := s.Store.CreateOperation(ctx, store.Operation{ID: opID, Kind: "provider.use", State: "prepared", ProviderID: providerID, Client: string(client), RedactedBackupPath: backupPath, ConfigFingerprint: fingerprint, ResourceName: name, DetailsJSON: string(useDetails)}); err != nil {
		return err
	}
	currentFingerprint, err := ConfigFingerprint(configPath)
	if err != nil {
		return err
	}
	if currentFingerprint != fingerprint {
		return s.failOperation(ctx, opID, "config_fingerprint_conflict", fmt.Errorf("config_fingerprint_conflict"))
	}
	if name == OfficialProviderName {
		err = WriteOfficialCodexConfig(configPath)
	} else if client == ClientCodex {
		config := ClientConfig{Name: definition.Name, Endpoint: selectedCredential.Endpoint, Credential: credential}
		err = WriteCodexConfig(configPath, config)
	} else if client == ClientClaude {
		config := ClientConfig{Name: definition.Name, Endpoint: selectedCredential.Endpoint, Credential: credential}
		err = WriteClaudeConfig(configPath, config)
	} else {
		err = fmt.Errorf("%w: client", ErrInvalidProvider)
	}
	if err != nil {
		return s.failOperation(ctx, opID, "config_write_failed", err)
	}
	if err := s.Store.UpdateOperation(ctx, opID, "external_written", ""); err != nil {
		return s.failOperation(ctx, opID, "external_written_transition_failed", err)
	}
	selection := store.Selection{Client: string(client), ProviderName: name, MultiplierSnapshot: "1", CredentialID: credentialID, CredentialName: selectedCredentialName, OperationID: opID}
	if definition != nil {
		selection.ProviderID = definition.ID
		selection.EndpointSnapshot = selectedCredential.Endpoint
		selection.MultiplierSnapshot = selectedCredential.Multiplier
	}
	if err := s.Store.CompleteProviderUse(ctx, opID, selection); err != nil {
		return s.failOperation(ctx, opID, "selection_commit_failed", err)
	}
	return nil
}

func (s Service) failOperation(ctx context.Context, operationID, code string, cause error) error {
	if err := s.Store.UpdateOperation(ctx, operationID, "failed", code); err != nil {
		return errors.Join(cause, fmt.Errorf("record operation failure: %w", err))
	}
	return cause
}

func defaultConfigPath(home string, client Client) (string, error) {
	if home == "" {
		return "", fmt.Errorf("%w: home directory is unavailable", ErrInvalidProvider)
	}
	switch client {
	case ClientCodex:
		return filepath.Join(home, ".codex", "config.toml"), nil
	case ClientClaude:
		return filepath.Join(home, ".claude", "settings.json"), nil
	default:
		return "", fmt.Errorf("%w: client", ErrInvalidProvider)
	}
}

func managedBackupPath(stateRoot string, client Client, operationID string) (string, error) {
	if stateRoot == "" {
		return "", fmt.Errorf("%w: state directory is unavailable", ErrInvalidProvider)
	}
	extension := ""
	switch client {
	case ClientCodex:
		extension = ".redacted.toml"
	case ClientClaude:
		extension = ".redacted.json"
	default:
		return "", fmt.Errorf("%w: client", ErrInvalidProvider)
	}
	return filepath.Join(stateRoot, "client-backups", string(client), operationID+extension), nil
}

// Recover resolves restart-safe provider removals and diagnoses interrupted
// provider uses. Operations that changed a client file remain visible because
// the redacted backup intentionally excludes credential values.
func (s Service) Recover(ctx context.Context) ([]store.Operation, error) {
	operations, err := s.Store.PendingOperations(ctx)
	if err != nil {
		return nil, err
	}
	for _, operation := range operations {
		if operation.Kind == "provider.use" && operation.State == "prepared" {
			code := DiagnoseProviderUseOperation(operation)
			if err := s.Store.UpdateOperation(ctx, operation.ID, "failed", code); err != nil {
				return nil, err
			}
		}
	}
	return s.Store.PendingOperations(ctx)
}

// DiagnoseProviderUseOperation classifies a pending provider.use operation
// without mutating its journal or client configuration.
func DiagnoseProviderUseOperation(operation store.Operation) string {
	if operation.Kind != "provider.use" {
		return ""
	}
	if operation.ErrorCode != "" {
		return operation.ErrorCode
	}
	switch operation.State {
	case "external_written":
		return "interrupted_after_external_write"
	case "prepared":
		var details providerUseDetails
		if json.Unmarshal([]byte(operation.DetailsJSON), &details) != nil || details.ConfigPath == "" {
			return "interrupted_before_external_write"
		}
		current, err := ConfigFingerprint(details.ConfigPath)
		if err != nil || current != operation.ConfigFingerprint {
			return "interrupted_after_external_write"
		}
		return "interrupted_before_external_write"
	case "failed":
		return "provider_use_failed"
	default:
		return ""
	}
}

func operationID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func secretRecord(sealed credentialvault.Sealed) store.CredentialSecret {
	return store.CredentialSecret{Algorithm: sealed.Algorithm, KeyVersion: sealed.KeyVersion, KeyID: sealed.KeyID, Nonce: sealed.Nonce, Ciphertext: sealed.Ciphertext}
}

func sealedRecord(secret store.CredentialSecret) credentialvault.Sealed {
	return credentialvault.Sealed{Algorithm: secret.Algorithm, KeyVersion: secret.KeyVersion, KeyID: secret.KeyID, Nonce: secret.Nonce, Ciphertext: secret.Ciphertext}
}

func (s Service) sealCredential(ctx context.Context, reference, value string) (credentialvault.Sealed, error) {
	keyIDs, err := s.Store.CredentialSecretKeyIDs(ctx)
	if err != nil {
		return credentialvault.Sealed{}, err
	}
	if len(keyIDs) == 0 {
		return s.Vault.Seal(ctx, reference, value)
	}
	keyID, err := s.Vault.InspectKey(ctx)
	if err != nil {
		return credentialvault.Sealed{}, err
	}
	if len(keyIDs) != 1 || keyIDs[0] == "" || keyIDs[0] != keyID {
		return credentialvault.Sealed{}, credentialvault.ErrKeyMachineMismatch
	}
	return s.Vault.SealExisting(ctx, reference, value)
}

func mappings(definition Definition) []store.ClientMapping {
	mappings := make([]store.ClientMapping, 0, len(definition.Clients))
	for _, client := range definition.Clients {
		mappings = append(mappings, store.ClientMapping{Client: string(client)})
	}
	return mappings
}

func officialProvider() Provider {
	return Provider{
		Name:           OfficialProviderName,
		Clients:        []store.ClientMapping{{Client: string(ClientCodex)}},
		BuiltIn:        true,
		Authentication: "codex_existing_login",
	}
}

func storedProvider(definition store.Provider) Provider {
	createdAt, updatedAt := definition.CreatedAt, definition.UpdatedAt
	return Provider{
		ID:              definition.ID,
		Name:            definition.Name,
		Clients:         definition.Clients,
		CredentialCount: len(definition.Credentials),
		CreatedAt:       &createdAt,
		UpdatedAt:       &updatedAt,
	}
}
