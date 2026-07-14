package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"
)

var ErrExtensionNotFound = errors.New("extension_not_found")

type Extension struct {
	ID, Client, Kind, Scope, NativeID, SourcePath, Version string
	Enabled                                                string
	Managed                                                bool
	AdoptedFingerprint                                     string
	Capabilities, Diagnostics                              []string
	Fingerprint                                            string
}

func (s *Store) ReplaceExtensions(ctx context.Context, values []Extension) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		caps, err := json.Marshal(value.Capabilities)
		if err != nil {
			return err
		}
		diagnostics, err := json.Marshal(value.Diagnostics)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO extensions(id,client,kind,scope,native_id,source_path,version,enabled,capabilities_json,diagnostics_json,fingerprint,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET client=excluded.client,kind=excluded.kind,scope=excluded.scope,native_id=excluded.native_id,source_path=excluded.source_path,version=excluded.version,enabled=excluded.enabled,capabilities_json=excluded.capabilities_json,diagnostics_json=excluded.diagnostics_json,fingerprint=excluded.fingerprint,updated_at=excluded.updated_at WHERE extensions.client IS NOT excluded.client OR extensions.kind IS NOT excluded.kind OR extensions.scope IS NOT excluded.scope OR extensions.native_id IS NOT excluded.native_id OR extensions.source_path IS NOT excluded.source_path OR extensions.version IS NOT excluded.version OR extensions.enabled IS NOT excluded.enabled OR extensions.capabilities_json IS NOT excluded.capabilities_json OR extensions.diagnostics_json IS NOT excluded.diagnostics_json OR extensions.fingerprint IS NOT excluded.fingerprint`, value.ID, value.Client, value.Kind, value.Scope, value.NativeID, value.SourcePath, value.Version, value.Enabled, caps, diagnostics, value.Fingerprint, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			return err
		}
		seen[value.ID] = true
	}
	rows, err := tx.QueryContext(ctx, "SELECT id FROM extensions")
	if err != nil {
		return err
	}
	var stale []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		if !seen[id] {
			stale = append(stale, id)
		}
	}
	if err = rows.Close(); err != nil {
		return err
	}
	for _, id := range stale {
		if _, err = tx.ExecContext(ctx, "DELETE FROM extensions WHERE id=?", id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) ListExtensions(ctx context.Context) ([]Extension, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT e.id,e.client,e.kind,e.scope,e.native_id,e.source_path,e.version,e.enabled,e.capabilities_json,e.diagnostics_json,e.fingerprint,m.extension_id IS NOT NULL,COALESCE(m.fingerprint,'') FROM extensions e LEFT JOIN extension_management m ON m.extension_id=e.id ORDER BY e.client,e.kind,e.scope,e.native_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var values []Extension
	for rows.Next() {
		var v Extension
		var caps, diagnostics []byte
		if err = rows.Scan(&v.ID, &v.Client, &v.Kind, &v.Scope, &v.NativeID, &v.SourcePath, &v.Version, &v.Enabled, &caps, &diagnostics, &v.Fingerprint, &v.Managed, &v.AdoptedFingerprint); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(caps, &v.Capabilities); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(diagnostics, &v.Diagnostics); err != nil {
			return nil, err
		}
		values = append(values, v)
	}
	return values, rows.Err()
}

func (s *Store) ExtensionByID(ctx context.Context, id string) (Extension, error) {
	values, err := s.ListExtensions(ctx)
	if err != nil {
		return Extension{}, err
	}
	for _, value := range values {
		if value.ID == id {
			return value, nil
		}
	}
	return Extension{}, fmt.Errorf("%w: %s", ErrExtensionNotFound, id)
}

func (s *Store) AdoptExtension(ctx context.Context, id string) (Extension, error) {
	v, err := s.ExtensionByID(ctx, id)
	if err != nil {
		return Extension{}, err
	}
	if _, err = s.DB.ExecContext(ctx, `INSERT INTO extension_management(extension_id,fingerprint,adopted_at) VALUES(?,?,?) ON CONFLICT(extension_id) DO UPDATE SET fingerprint=excluded.fingerprint,adopted_at=excluded.adopted_at`, id, v.Fingerprint, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return Extension{}, err
	}
	v.Managed = true
	v.AdoptedFingerprint = v.Fingerprint
	return v, nil
}

func (s *Store) ReleaseExtension(ctx context.Context, id string) error {
	result, err := s.DB.ExecContext(ctx, "DELETE FROM extension_management WHERE extension_id=?", id)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("%w: %s", ErrExtensionNotFound, id)
	}
	return nil
}

func SortedCapabilities(values []string) []string {
	out := append([]string(nil), values...)
	sort.Strings(out)
	return out
}
