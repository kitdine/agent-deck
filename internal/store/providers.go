package store

import (
	"context"
	"database/sql"
	"time"
)

type Provider struct {
	ID            int64           `json:"id"`
	Name          string          `json:"name"`
	Endpoint      string          `json:"endpoint"`
	CredentialRef string          `json:"credential_ref"`
	Multiplier    string          `json:"multiplier"`
	Clients       []ClientMapping `json:"clients"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

type ClientMapping struct {
	Client        string `json:"client"`
	NativeModel   string `json:"native_model"`
	ProviderModel string `json:"provider_model"`
}

type Selection struct {
	ProviderID         int64
	Client             string
	MultiplierSnapshot string
	SelectedAt         time.Time
}

type Operation struct {
	ID                 string    `json:"id"`
	Kind               string    `json:"kind"`
	State              string    `json:"state"`
	ProviderID         *int64    `json:"provider_id"`
	Client             string    `json:"client"`
	StartedAt          time.Time `json:"started_at"`
	UpdatedAt          time.Time `json:"updated_at"`
	RedactedBackupPath string    `json:"redacted_backup_path"`
	ErrorCode          string    `json:"error_code"`
	ConfigFingerprint  string    `json:"config_fingerprint"`
}

func (s *Store) CreateProvider(ctx context.Context, provider Provider) (Provider, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Provider{}, err
	}
	now := time.Now().UTC()
	result, err := tx.ExecContext(ctx, "INSERT INTO providers(name, endpoint, credential_ref, multiplier, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)", provider.Name, provider.Endpoint, provider.CredentialRef, provider.Multiplier, now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	if err != nil {
		_ = tx.Rollback()
		return Provider{}, err
	}
	provider.ID, err = result.LastInsertId()
	if err != nil {
		_ = tx.Rollback()
		return Provider{}, err
	}
	if err := replaceClientMappings(ctx, tx, provider.ID, provider.Clients); err != nil {
		_ = tx.Rollback()
		return Provider{}, err
	}
	if err := tx.Commit(); err != nil {
		return Provider{}, err
	}
	provider.CreatedAt, provider.UpdatedAt = now, now
	return provider, s.secureFiles()
}

func (s *Store) ListProviders(ctx context.Context) ([]Provider, error) {
	rows, err := s.DB.QueryContext(ctx, "SELECT id, name, endpoint, credential_ref, multiplier, created_at, updated_at FROM providers ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var providers []Provider
	for rows.Next() {
		var provider Provider
		var created, updated string
		if err := rows.Scan(&provider.ID, &provider.Name, &provider.Endpoint, &provider.CredentialRef, &provider.Multiplier, &created, &updated); err != nil {
			return nil, err
		}
		provider.CreatedAt, err = time.Parse(time.RFC3339Nano, created)
		if err != nil {
			return nil, err
		}
		provider.UpdatedAt, err = time.Parse(time.RFC3339Nano, updated)
		if err != nil {
			return nil, err
		}
		provider.Clients, err = s.listClientMappings(ctx, provider.ID)
		if err != nil {
			return nil, err
		}
		providers = append(providers, provider)
	}
	return providers, rows.Err()
}

func (s *Store) ProviderByName(ctx context.Context, name string) (Provider, error) {
	providers, err := s.ListProviders(ctx)
	if err != nil {
		return Provider{}, err
	}
	for _, provider := range providers {
		if provider.Name == name {
			return provider, nil
		}
	}
	return Provider{}, sql.ErrNoRows
}

func (s *Store) DeleteProvider(ctx context.Context, name string) error {
	result, err := s.DB.ExecContext(ctx, "DELETE FROM providers WHERE name = ?", name)
	if err != nil {
		return err
	}
	if affected, err := result.RowsAffected(); err != nil {
		return err
	} else if affected == 0 {
		return sql.ErrNoRows
	}
	return s.secureFiles()
}

func (s *Store) UpdateProvider(ctx context.Context, provider Provider) (Provider, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Provider{}, err
	}
	now := time.Now().UTC()
	result, err := tx.ExecContext(ctx, "UPDATE providers SET endpoint = ?, credential_ref = ?, multiplier = ?, updated_at = ? WHERE name = ?", provider.Endpoint, provider.CredentialRef, provider.Multiplier, now.Format(time.RFC3339Nano), provider.Name)
	if err != nil {
		_ = tx.Rollback()
		return Provider{}, err
	}
	if affected, err := result.RowsAffected(); err != nil {
		_ = tx.Rollback()
		return Provider{}, err
	} else if affected == 0 {
		_ = tx.Rollback()
		return Provider{}, sql.ErrNoRows
	}
	if err := tx.QueryRowContext(ctx, "SELECT id, created_at FROM providers WHERE name = ?", provider.Name).Scan(&provider.ID, new(string)); err != nil {
		_ = tx.Rollback()
		return Provider{}, err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM provider_clients WHERE provider_id = ?", provider.ID); err != nil {
		_ = tx.Rollback()
		return Provider{}, err
	}
	if err := replaceClientMappings(ctx, tx, provider.ID, provider.Clients); err != nil {
		_ = tx.Rollback()
		return Provider{}, err
	}
	if err := tx.Commit(); err != nil {
		return Provider{}, err
	}
	provider.UpdatedAt = now
	return provider, s.secureFiles()
}

func (s *Store) RecordSelection(ctx context.Context, selection Selection) error {
	if selection.SelectedAt.IsZero() {
		selection.SelectedAt = time.Now().UTC()
	}
	_, err := s.DB.ExecContext(ctx, "INSERT INTO provider_selections(provider_id, client, multiplier_snapshot, selected_at) VALUES (?, ?, ?, ?)", selection.ProviderID, selection.Client, selection.MultiplierSnapshot, selection.SelectedAt.Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	return s.secureFiles()
}

func (s *Store) CreateOperation(ctx context.Context, operation Operation) error {
	if operation.StartedAt.IsZero() {
		operation.StartedAt = time.Now().UTC()
	}
	if operation.UpdatedAt.IsZero() {
		operation.UpdatedAt = operation.StartedAt
	}
	_, err := s.DB.ExecContext(ctx, "INSERT INTO operations(id, kind, state, provider_id, client, started_at, updated_at, redacted_backup_path, error_code, config_fingerprint) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)", operation.ID, operation.Kind, operation.State, operation.ProviderID, operation.Client, operation.StartedAt.Format(time.RFC3339Nano), operation.UpdatedAt.Format(time.RFC3339Nano), operation.RedactedBackupPath, operation.ErrorCode, operation.ConfigFingerprint)
	if err != nil {
		return err
	}
	return s.secureFiles()
}

func (s *Store) UpdateOperation(ctx context.Context, id, state, errorCode string) error {
	result, err := s.DB.ExecContext(ctx, "UPDATE operations SET state = ?, error_code = ?, updated_at = ? WHERE id = ?", state, errorCode, time.Now().UTC().Format(time.RFC3339Nano), id)
	if err != nil {
		return err
	}
	if affected, err := result.RowsAffected(); err != nil {
		return err
	} else if affected == 0 {
		return sql.ErrNoRows
	}
	return s.secureFiles()
}

func (s *Store) PendingOperations(ctx context.Context) ([]Operation, error) {
	rows, err := s.DB.QueryContext(ctx, "SELECT id, kind, state, provider_id, COALESCE(client, ''), started_at, updated_at, COALESCE(redacted_backup_path, ''), COALESCE(error_code, ''), COALESCE(config_fingerprint, '') FROM operations WHERE state != 'completed' ORDER BY started_at")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	operations := make([]Operation, 0)
	for rows.Next() {
		var operation Operation
		var started, updated string
		if err := rows.Scan(&operation.ID, &operation.Kind, &operation.State, &operation.ProviderID, &operation.Client, &started, &updated, &operation.RedactedBackupPath, &operation.ErrorCode, &operation.ConfigFingerprint); err != nil {
			return nil, err
		}
		var parseErr error
		operation.StartedAt, parseErr = time.Parse(time.RFC3339Nano, started)
		if parseErr != nil {
			return nil, parseErr
		}
		operation.UpdatedAt, parseErr = time.Parse(time.RFC3339Nano, updated)
		if parseErr != nil {
			return nil, parseErr
		}
		operations = append(operations, operation)
	}
	return operations, rows.Err()
}

func replaceClientMappings(ctx context.Context, tx *sql.Tx, providerID int64, mappings []ClientMapping) error {
	for _, mapping := range mappings {
		if _, err := tx.ExecContext(ctx, "INSERT INTO provider_clients(provider_id, client, native_model, provider_model) VALUES (?, ?, ?, ?)", providerID, mapping.Client, mapping.NativeModel, mapping.ProviderModel); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) listClientMappings(ctx context.Context, providerID int64) ([]ClientMapping, error) {
	rows, err := s.DB.QueryContext(ctx, "SELECT client, native_model, COALESCE(provider_model, '') FROM provider_clients WHERE provider_id = ? ORDER BY client, native_model", providerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var mappings []ClientMapping
	for rows.Next() {
		var mapping ClientMapping
		if err := rows.Scan(&mapping.Client, &mapping.NativeModel, &mapping.ProviderModel); err != nil {
			return nil, err
		}
		mappings = append(mappings, mapping)
	}
	return mappings, rows.Err()
}
