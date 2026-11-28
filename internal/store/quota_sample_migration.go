package store

import (
	"context"
	"database/sql"
	_ "embed"
)

//go:embed quota_session_samples.sql
var quotaSessionSamplesSQL string

func applyQuotaSessionSamples(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, quotaSessionSamplesSQL)
	return err
}
