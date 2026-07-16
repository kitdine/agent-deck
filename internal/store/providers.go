package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type Provider struct {
	ID            int64                `json:"id"`
	Name          string               `json:"name"`
	Endpoint      string               `json:"endpoint"`
	CredentialRef string               `json:"credential_ref"`
	Multiplier    string               `json:"multiplier"`
	Clients       []ClientMapping      `json:"clients"`
	CreatedAt     time.Time            `json:"created_at"`
	UpdatedAt     time.Time            `json:"updated_at"`
	Credentials   []ProviderCredential `json:"credentials,omitempty"`
}

type ProviderCredential struct {
	ID            int64     `json:"id,omitempty"`
	ProviderID    int64     `json:"-"`
	ProviderName  string    `json:"provider"`
	Name          string    `json:"name"`
	CredentialRef string    `json:"credential_ref"`
	SecretRef     string    `json:"-"`
	Endpoint      string    `json:"endpoint"`
	Multiplier    string    `json:"multiplier"`
	Clients       []string  `json:"clients"`
	SecretPresent bool      `json:"-"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (c ProviderCredential) StorageReference() string {
	return c.CredentialRef
}

type CredentialSecret struct {
	CredentialID int64
	Algorithm    string
	KeyVersion   int
	KeyID        string
	Nonce        []byte
	Ciphertext   []byte
	UpdatedAt    time.Time
}

type ClientMapping struct {
	Client        string `json:"client"`
	NativeModel   string `json:"native_model"`
	ProviderModel string `json:"provider_model"`
}

type Selection struct {
	ProviderID         int64
	Client             string
	ProviderName       string
	EndpointSnapshot   string
	MultiplierSnapshot string
	SelectedAt         time.Time
	CredentialID       *int64
	CredentialName     string
	OperationID        string
}

type ProviderSnapshot struct {
	Name       string
	Endpoint   string
	Multiplier string
	Credential string
	SelectedAt time.Time
	Official   bool
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
	ResourceName       string    `json:"resource_name,omitempty"`
	DetailsJSON        string    `json:"-"`
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

// CreateProviderWithCredential persists the initial provider definition,
// credential metadata, and authenticated ciphertext in one transaction.
func (s *Store) CreateProviderWithCredential(ctx context.Context, provider Provider, credential ProviderCredential, secret CredentialSecret) (Provider, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Provider{}, err
	}
	defer tx.Rollback()
	now := time.Now().UTC()
	result, err := tx.ExecContext(ctx, "INSERT INTO providers(name,endpoint,credential_ref,multiplier,created_at,updated_at) VALUES(?,?,?,?,?,?)", provider.Name, provider.Endpoint, provider.CredentialRef, provider.Multiplier, now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	if err != nil {
		return Provider{}, err
	}
	provider.ID, err = result.LastInsertId()
	if err != nil {
		return Provider{}, err
	}
	if err = replaceClientMappings(ctx, tx, provider.ID, provider.Clients); err != nil {
		return Provider{}, err
	}
	credential.ProviderID = provider.ID
	if credential.SecretRef == "" {
		credential.SecretRef = credential.CredentialRef
	}
	result, err = tx.ExecContext(ctx, "INSERT INTO provider_credentials(provider_id,name,credential_ref,logical_ref,endpoint,multiplier,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?)", credential.ProviderID, credential.Name, credential.SecretRef, credential.CredentialRef, credential.Endpoint, credential.Multiplier, now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	if err != nil {
		return Provider{}, err
	}
	credential.ID, err = result.LastInsertId()
	if err != nil {
		return Provider{}, err
	}
	for _, client := range credential.Clients {
		if _, err = tx.ExecContext(ctx, "INSERT INTO provider_credential_clients(credential_id,client) VALUES(?,?)", credential.ID, client); err != nil {
			return Provider{}, err
		}
	}
	secret.CredentialID = credential.ID
	if err = insertCredentialSecret(ctx, tx, secret, now); err != nil {
		return Provider{}, err
	}
	if err = tx.Commit(); err != nil {
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
		provider.Credentials, err = s.ListProviderCredentials(ctx, provider.Name)
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

func (s *Store) RecordSelection(ctx context.Context, selection Selection) error {
	if selection.SelectedAt.IsZero() {
		selection.SelectedAt = time.Now().UTC()
	}
	if selection.ProviderName == "" && selection.ProviderID != 0 {
		if err := s.DB.QueryRowContext(ctx, "SELECT name,endpoint FROM providers WHERE id=?", selection.ProviderID).Scan(&selection.ProviderName, &selection.EndpointSnapshot); err != nil {
			return err
		}
	}
	_, err := s.DB.ExecContext(ctx, "INSERT INTO provider_selections(provider_id,client,provider_name_snapshot,endpoint_snapshot,multiplier_snapshot,credential_id,credential_name_snapshot,operation_id,selected_at) VALUES (?,?,?,?,?,?,?,?,?)", nullableProviderID(selection.ProviderID), selection.Client, selection.ProviderName, selection.EndpointSnapshot, selection.MultiplierSnapshot, selection.CredentialID, selection.CredentialName, nullableString(selection.OperationID), selection.SelectedAt.Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	return s.secureFiles()
}

// CompleteProviderUse makes the operation and its immutable attribution
// snapshot visible atomically. A failure leaves the previous active selection
// authoritative even when the external client file was already written.
func (s *Store) CompleteProviderUse(ctx context.Context, operationID string, selection Selection) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	completedAt := selection.SelectedAt.UTC()
	if completedAt.IsZero() {
		completedAt = time.Now().UTC()
	}
	result, err := tx.ExecContext(ctx, "UPDATE operations SET state='completed',error_code='',updated_at=? WHERE id=? AND state='external_written'", completedAt.Format(time.RFC3339Nano), operationID)
	if err != nil {
		return err
	}
	if n, err := result.RowsAffected(); err != nil {
		return err
	} else if n != 1 {
		return sql.ErrNoRows
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO provider_selections(provider_id,client,provider_name_snapshot,endpoint_snapshot,multiplier_snapshot,credential_id,credential_name_snapshot,operation_id,selected_at) VALUES(?,?,?,?,?,?,?,?,?)", nullableProviderID(selection.ProviderID), selection.Client, selection.ProviderName, selection.EndpointSnapshot, selection.MultiplierSnapshot, selection.CredentialID, selection.CredentialName, operationID, completedAt.Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	return s.secureFiles()
}

func nullableProviderID(id int64) any {
	if id == 0 {
		return nil
	}
	return id
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func (s *Store) LatestSelectionCredential(ctx context.Context, providerID int64, client string) (string, error) {
	var name string
	err := s.DB.QueryRowContext(ctx, `SELECT credential_name_snapshot FROM provider_selections WHERE provider_id=? AND client=? AND operation_id IN (SELECT id FROM operations WHERE state='completed') ORDER BY selected_at DESC,id DESC LIMIT 1`, providerID, client).Scan(&name)
	if err != nil {
		return "", err
	}
	return name, nil
}

func (s *Store) CreateCredential(ctx context.Context, credential ProviderCredential) (ProviderCredential, error) {
	return s.createCredential(ctx, credential, nil)
}

func (s *Store) CreateCredentialWithSecret(ctx context.Context, credential ProviderCredential, secret CredentialSecret) (ProviderCredential, error) {
	return s.createCredential(ctx, credential, &secret)
}

func (s *Store) createCredential(ctx context.Context, credential ProviderCredential, secret *CredentialSecret) (ProviderCredential, error) {
	now := time.Now().UTC()
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return ProviderCredential{}, err
	}
	defer tx.Rollback()
	if credential.ProviderID == 0 {
		if err = tx.QueryRowContext(ctx, "SELECT id FROM providers WHERE name=?", credential.ProviderName).Scan(&credential.ProviderID); err != nil {
			return ProviderCredential{}, err
		}
	}
	if credential.SecretRef == "" {
		credential.SecretRef = credential.CredentialRef
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO provider_credentials(provider_id,name,credential_ref,logical_ref,endpoint,multiplier,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?)`, credential.ProviderID, credential.Name, credential.SecretRef, credential.CredentialRef, credential.Endpoint, credential.Multiplier, now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	if err != nil {
		return ProviderCredential{}, err
	}
	credential.ID, err = result.LastInsertId()
	if err != nil {
		return ProviderCredential{}, err
	}
	for _, client := range credential.Clients {
		if _, err = tx.ExecContext(ctx, `INSERT INTO provider_credential_clients(credential_id,client) VALUES(?,?)`, credential.ID, client); err != nil {
			return ProviderCredential{}, err
		}
	}
	if secret != nil {
		secret.CredentialID = credential.ID
		if err = insertCredentialSecret(ctx, tx, *secret, now); err != nil {
			return ProviderCredential{}, err
		}
		credential.SecretPresent = true
	}
	if err = syncProviderClients(ctx, tx, credential.ProviderID); err != nil {
		return ProviderCredential{}, err
	}
	if err = tx.Commit(); err != nil {
		return ProviderCredential{}, err
	}
	credential.CreatedAt, credential.UpdatedAt = now, now
	return credential, s.secureFiles()
}

func (s *Store) ListProviderCredentials(ctx context.Context, providerName string) ([]ProviderCredential, error) {
	query := `SELECT pc.id,pc.provider_id,p.name,pc.name,pc.credential_ref,pc.logical_ref,pc.endpoint,pc.multiplier,EXISTS(SELECT 1 FROM credential_secrets cs WHERE cs.credential_id=pc.id),pc.created_at,pc.updated_at FROM provider_credentials pc JOIN providers p ON p.id=pc.provider_id`
	args := []any{}
	if providerName != "" {
		query += ` WHERE p.name=?`
		args = append(args, providerName)
	}
	query += ` ORDER BY p.name,pc.name`
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ProviderCredential
	for rows.Next() {
		var item ProviderCredential
		var created, updated string
		if err = rows.Scan(&item.ID, &item.ProviderID, &item.ProviderName, &item.Name, &item.SecretRef, &item.CredentialRef, &item.Endpoint, &item.Multiplier, &item.SecretPresent, &created, &updated); err != nil {
			return nil, err
		}
		item.CreatedAt, err = time.Parse(time.RFC3339Nano, created)
		if err != nil {
			return nil, err
		}
		item.UpdatedAt, err = time.Parse(time.RFC3339Nano, updated)
		if err != nil {
			return nil, err
		}
		clientRows, queryErr := s.DB.QueryContext(ctx, `SELECT client FROM provider_credential_clients WHERE credential_id=? ORDER BY client`, item.ID)
		if queryErr != nil {
			return nil, queryErr
		}
		for clientRows.Next() {
			var client string
			if err = clientRows.Scan(&client); err != nil {
				clientRows.Close()
				return nil, err
			}
			item.Clients = append(item.Clients, client)
		}
		if err = clientRows.Close(); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) ProviderCredential(ctx context.Context, providerName, name string) (ProviderCredential, error) {
	items, err := s.ListProviderCredentials(ctx, providerName)
	if err != nil {
		return ProviderCredential{}, err
	}
	for _, item := range items {
		if item.Name == name {
			return item, nil
		}
	}
	return ProviderCredential{}, sql.ErrNoRows
}

func (s *Store) UpdateProviderCredential(ctx context.Context, credential ProviderCredential) (ProviderCredential, error) {
	return s.updateProviderCredential(ctx, credential, nil)
}

func (s *Store) UpdateProviderCredentialWithSecret(ctx context.Context, credential ProviderCredential, secret CredentialSecret) (ProviderCredential, error) {
	return s.updateProviderCredential(ctx, credential, &secret)
}

func (s *Store) updateProviderCredential(ctx context.Context, credential ProviderCredential, secret *CredentialSecret) (ProviderCredential, error) {
	now := time.Now().UTC()
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return ProviderCredential{}, err
	}
	defer tx.Rollback()
	if credential.SecretRef == "" {
		credential.SecretRef = credential.CredentialRef
	}
	result, err := tx.ExecContext(ctx, `UPDATE provider_credentials SET credential_ref=?,logical_ref=?,endpoint=?,multiplier=?,updated_at=? WHERE id=?`, credential.SecretRef, credential.CredentialRef, credential.Endpoint, credential.Multiplier, now.Format(time.RFC3339Nano), credential.ID)
	if err != nil {
		return ProviderCredential{}, err
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return ProviderCredential{}, sql.ErrNoRows
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM provider_credential_clients WHERE credential_id=?`, credential.ID); err != nil {
		return ProviderCredential{}, err
	}
	for _, client := range credential.Clients {
		if _, err = tx.ExecContext(ctx, `INSERT INTO provider_credential_clients(credential_id,client) VALUES(?,?)`, credential.ID, client); err != nil {
			return ProviderCredential{}, err
		}
	}
	if secret != nil {
		secret.CredentialID = credential.ID
		if err = upsertCredentialSecret(ctx, tx, *secret, now); err != nil {
			return ProviderCredential{}, err
		}
		credential.SecretPresent = true
	}
	if err = syncProviderClients(ctx, tx, credential.ProviderID); err != nil {
		return ProviderCredential{}, err
	}
	if err = tx.Commit(); err != nil {
		return ProviderCredential{}, err
	}
	credential.UpdatedAt = now
	return credential, s.secureFiles()
}

func (s *Store) CredentialSecret(ctx context.Context, credentialID int64) (CredentialSecret, error) {
	var secret CredentialSecret
	var updated string
	err := s.DB.QueryRowContext(ctx, `SELECT credential_id,algorithm,key_version,key_id,nonce,ciphertext,updated_at FROM credential_secrets WHERE credential_id=?`, credentialID).Scan(&secret.CredentialID, &secret.Algorithm, &secret.KeyVersion, &secret.KeyID, &secret.Nonce, &secret.Ciphertext, &updated)
	if err != nil {
		return CredentialSecret{}, err
	}
	secret.UpdatedAt, err = time.Parse(time.RFC3339Nano, updated)
	return secret, err
}

func (s *Store) CredentialSecretKeyIDs(ctx context.Context) ([]string, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT DISTINCT key_id FROM credential_secrets ORDER BY key_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var keyIDs []string
	for rows.Next() {
		var keyID string
		if err = rows.Scan(&keyID); err != nil {
			return nil, err
		}
		keyIDs = append(keyIDs, keyID)
	}
	return keyIDs, rows.Err()
}

// ReplaceCredentialSecrets atomically replaces every credential ciphertext.
// It is used by portable restore after sealing plaintext for the target machine.
func (s *Store) ReplaceCredentialSecrets(ctx context.Context, secrets []CredentialSecret) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `DELETE FROM credential_secrets`); err != nil {
		return err
	}
	now := time.Now().UTC()
	seen := make(map[int64]bool, len(secrets))
	for _, secret := range secrets {
		if secret.CredentialID == 0 || seen[secret.CredentialID] {
			return errors.New("invalid credential secret ownership")
		}
		seen[secret.CredentialID] = true
		if err = insertCredentialSecret(ctx, tx, secret, now); err != nil {
			return err
		}
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	return s.secureFiles()
}

func insertCredentialSecret(ctx context.Context, tx *sql.Tx, secret CredentialSecret, now time.Time) error {
	if secret.UpdatedAt.IsZero() {
		secret.UpdatedAt = now
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO credential_secrets(credential_id,algorithm,key_version,key_id,nonce,ciphertext,updated_at) VALUES(?,?,?,?,?,?,?)`, secret.CredentialID, secret.Algorithm, secret.KeyVersion, secret.KeyID, secret.Nonce, secret.Ciphertext, secret.UpdatedAt.UTC().Format(time.RFC3339Nano))
	return err
}

func upsertCredentialSecret(ctx context.Context, tx *sql.Tx, secret CredentialSecret, now time.Time) error {
	if secret.UpdatedAt.IsZero() {
		secret.UpdatedAt = now
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO credential_secrets(credential_id,algorithm,key_version,key_id,nonce,ciphertext,updated_at) VALUES(?,?,?,?,?,?,?) ON CONFLICT(credential_id) DO UPDATE SET algorithm=excluded.algorithm,key_version=excluded.key_version,key_id=excluded.key_id,nonce=excluded.nonce,ciphertext=excluded.ciphertext,updated_at=excluded.updated_at`, secret.CredentialID, secret.Algorithm, secret.KeyVersion, secret.KeyID, secret.Nonce, secret.Ciphertext, secret.UpdatedAt.UTC().Format(time.RFC3339Nano))
	return err
}

func (s *Store) DeleteProviderCredential(ctx context.Context, providerName, name string) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var providerID int64
	if err = tx.QueryRowContext(ctx, `SELECT id FROM providers WHERE name=?`, providerName).Scan(&providerID); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM provider_credentials WHERE provider_id=? AND name=?`, providerID, name)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	if err = syncProviderClients(ctx, tx, providerID); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	return s.secureFiles()
}

func (s *Store) CurrentProviderSnapshot(ctx context.Context, client string) (ProviderSnapshot, error) {
	return s.providerSnapshot(ctx, client, nil)
}

func (s *Store) ProviderSnapshotAt(ctx context.Context, client string, at time.Time) (ProviderSnapshot, error) {
	return s.providerSnapshot(ctx, client, &at)
}

func (s *Store) providerSnapshot(ctx context.Context, client string, at *time.Time) (ProviderSnapshot, error) {
	operation, err := s.latestCompletedProviderOperation(ctx, client, at)
	if err == nil {
		if snapshot, selectionErr := s.providerSelectionForOperation(ctx, operation.id); selectionErr == nil {
			return snapshot, nil
		} else if !errors.Is(selectionErr, sql.ErrNoRows) {
			return ProviderSnapshot{}, selectionErr
		}
		// Compatibility for pre-v7 completed operations, which did not have an
		// operation-linked immutable selection row.
		if !operation.providerID.Valid {
			return ProviderSnapshot{Name: "official", Multiplier: "1", SelectedAt: operation.updatedAt, Official: true}, nil
		}
		providerID := operation.providerID.Int64
		snapshot, selectionErr := s.latestProviderSelection(ctx, client, &providerID, &operation.startedAt, &operation.updatedAt)
		if selectionErr == nil {
			return snapshot, nil
		}
		if !errors.Is(selectionErr, sql.ErrNoRows) {
			return ProviderSnapshot{}, selectionErr
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return ProviderSnapshot{}, err
	}

	return s.latestProviderSelection(ctx, client, nil, nil, at)
}

func (s *Store) providerSelectionForOperation(ctx context.Context, operationID string) (ProviderSnapshot, error) {
	var snapshot ProviderSnapshot
	var selected string
	err := s.DB.QueryRowContext(ctx, `SELECT provider_name_snapshot,endpoint_snapshot,multiplier_snapshot,credential_name_snapshot,selected_at FROM provider_selections WHERE operation_id=?`, operationID).Scan(&snapshot.Name, &snapshot.Endpoint, &snapshot.Multiplier, &snapshot.Credential, &selected)
	if err != nil {
		return ProviderSnapshot{}, err
	}
	snapshot.SelectedAt, err = time.Parse(time.RFC3339Nano, selected)
	snapshot.Official = snapshot.Name == "official"
	return snapshot, err
}

type completedProviderOperation struct {
	id         string
	providerID sql.NullInt64
	startedAt  time.Time
	updatedAt  time.Time
}

func (s *Store) latestCompletedProviderOperation(ctx context.Context, client string, at *time.Time) (completedProviderOperation, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id,provider_id,started_at,updated_at FROM operations WHERE kind='provider.use' AND state='completed' AND client=?`, client)
	if err != nil {
		return completedProviderOperation{}, err
	}
	defer rows.Close()

	var latest *completedProviderOperation
	for rows.Next() {
		var candidate completedProviderOperation
		var startedText, updatedText string
		if err := rows.Scan(&candidate.id, &candidate.providerID, &startedText, &updatedText); err != nil {
			return completedProviderOperation{}, err
		}
		candidate.startedAt, err = time.Parse(time.RFC3339Nano, startedText)
		if err != nil {
			return completedProviderOperation{}, err
		}
		candidate.updatedAt, err = time.Parse(time.RFC3339Nano, updatedText)
		if err != nil {
			return completedProviderOperation{}, err
		}
		if at != nil && candidate.updatedAt.After(at.UTC()) {
			continue
		}
		if latest == nil || candidate.updatedAt.After(latest.updatedAt) || candidate.updatedAt.Equal(latest.updatedAt) && candidate.id > latest.id {
			copy := candidate
			latest = &copy
		}
	}
	if err := rows.Err(); err != nil {
		return completedProviderOperation{}, err
	}
	if latest == nil {
		return completedProviderOperation{}, sql.ErrNoRows
	}
	return *latest, nil
}

func (s *Store) latestProviderSelection(ctx context.Context, client string, providerID *int64, notBefore, notAfter *time.Time) (ProviderSnapshot, error) {
	query := `SELECT ps.id,ps.provider_name_snapshot,ps.endpoint_snapshot,ps.multiplier_snapshot,ps.credential_name_snapshot,ps.selected_at FROM provider_selections ps LEFT JOIN operations o ON o.id=ps.operation_id WHERE ps.client=? AND (ps.operation_id IS NULL OR o.state='completed')`
	args := []any{client}
	if providerID != nil {
		query += ` AND ps.provider_id=?`
		args = append(args, *providerID)
	}
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return ProviderSnapshot{}, err
	}
	defer rows.Close()

	var latest *ProviderSnapshot
	var latestID int64
	for rows.Next() {
		var candidate ProviderSnapshot
		var id int64
		var selectedText string
		if err := rows.Scan(&id, &candidate.Name, &candidate.Endpoint, &candidate.Multiplier, &candidate.Credential, &selectedText); err != nil {
			return ProviderSnapshot{}, err
		}
		candidate.SelectedAt, err = time.Parse(time.RFC3339Nano, selectedText)
		if err != nil {
			return ProviderSnapshot{}, err
		}
		if notBefore != nil && candidate.SelectedAt.Before(notBefore.UTC()) || notAfter != nil && candidate.SelectedAt.After(notAfter.UTC()) {
			continue
		}
		if latest == nil || candidate.SelectedAt.After(latest.SelectedAt) || candidate.SelectedAt.Equal(latest.SelectedAt) && id > latestID {
			candidate.Official = candidate.Name == "official"
			copy := candidate
			latest = &copy
			latestID = id
		}
	}
	if err := rows.Err(); err != nil {
		return ProviderSnapshot{}, err
	}
	if latest == nil {
		return ProviderSnapshot{}, sql.ErrNoRows
	}
	return *latest, nil
}

func (s *Store) CreateOperation(ctx context.Context, operation Operation) error {
	if operation.StartedAt.IsZero() {
		operation.StartedAt = time.Now().UTC()
	}
	if operation.UpdatedAt.IsZero() {
		operation.UpdatedAt = operation.StartedAt
	}
	if operation.DetailsJSON == "" {
		operation.DetailsJSON = "{}"
	}
	_, err := s.DB.ExecContext(ctx, "INSERT INTO operations(id,kind,state,provider_id,client,started_at,updated_at,redacted_backup_path,error_code,config_fingerprint,resource_name,details_json) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)", operation.ID, operation.Kind, operation.State, operation.ProviderID, operation.Client, operation.StartedAt.Format(time.RFC3339Nano), operation.UpdatedAt.Format(time.RFC3339Nano), operation.RedactedBackupPath, operation.ErrorCode, operation.ConfigFingerprint, operation.ResourceName, operation.DetailsJSON)
	if err != nil {
		return err
	}
	return s.secureFiles()
}

func (s *Store) UpdateOperation(ctx context.Context, id, state, errorCode string) error {
	return s.UpdateOperationDetails(ctx, id, state, errorCode, "")
}

// UpdateOperationDetails persists a journal transition and optional recovery
// details in one database statement. An empty details value preserves the
// existing details_json payload.
func (s *Store) UpdateOperationDetails(ctx context.Context, id, state, errorCode, details string) error {
	query := "UPDATE operations SET state = ?, error_code = ?, updated_at = ? WHERE id = ?"
	args := []any{state, errorCode, time.Now().UTC().Format(time.RFC3339Nano), id}
	if details != "" {
		query = "UPDATE operations SET state = ?, error_code = ?, details_json = ?, updated_at = ? WHERE id = ?"
		args = []any{state, errorCode, details, time.Now().UTC().Format(time.RFC3339Nano), id}
	}
	result, err := s.DB.ExecContext(ctx, query, args...)
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
	rows, err := s.DB.QueryContext(ctx, "SELECT id,kind,state,provider_id,COALESCE(client,''),started_at,updated_at,COALESCE(redacted_backup_path,''),COALESCE(error_code,''),COALESCE(config_fingerprint,''),resource_name,details_json FROM operations WHERE state != 'completed' ORDER BY started_at")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	operations := make([]Operation, 0)
	for rows.Next() {
		var operation Operation
		var started, updated string
		if err := rows.Scan(&operation.ID, &operation.Kind, &operation.State, &operation.ProviderID, &operation.Client, &started, &updated, &operation.RedactedBackupPath, &operation.ErrorCode, &operation.ConfigFingerprint, &operation.ResourceName, &operation.DetailsJSON); err != nil {
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

func syncProviderClients(ctx context.Context, tx *sql.Tx, providerID int64) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM provider_clients WHERE provider_id=? AND client NOT IN (SELECT DISTINCT pcc.client FROM provider_credentials pc JOIN provider_credential_clients pcc ON pcc.credential_id=pc.id WHERE pc.provider_id=?)`, providerID, providerID); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO provider_clients(provider_id,client,native_model,provider_model) SELECT ?,bound.client,'',NULL FROM (SELECT DISTINCT pcc.client FROM provider_credentials pc JOIN provider_credential_clients pcc ON pcc.credential_id=pc.id WHERE pc.provider_id=?) AS bound WHERE NOT EXISTS (SELECT 1 FROM provider_clients current WHERE current.provider_id=? AND current.client=bound.client)`, providerID, providerID, providerID)
	return err
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
