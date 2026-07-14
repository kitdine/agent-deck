package doctor

import (
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/platform"
	"github.com/kitdine/agent-deck/internal/store"
)

func TestCheckMissingStateIsReadOnly(t *testing.T) {
	root := filepath.Join(t.TempDir(), "missing")
	report, err := (Service{StateRoot: root, Secrets: platform.NewMemorySecretStore()}).Check(context.Background(), false)
	if err != nil || report.Healthy || report.Problems != 1 || report.Checks[0].Code != "state_missing" {
		t.Fatalf("Check = %#v, %v", report, err)
	}
	if _, err = os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("doctor created state: %v", err)
	}
}

func TestFullCheckReportsProblemsWithoutChangingDatabases(t *testing.T) {
	ctx := context.Background()
	root := filepath.Join(t.TempDir(), "state")
	database, err := store.Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.CreateProvider(ctx, store.Provider{Name: "missing-secret", Endpoint: "https://example.invalid", CredentialRef: "missing", Multiplier: "1", Clients: []store.ClientMapping{{Client: "codex"}}}); err != nil {
		t.Fatal(err)
	}
	if err = database.CreateOperation(ctx, store.Operation{ID: "pending", Kind: "provider.use", State: "prepared"}); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO price_catalogs(version,source_kind,source_url,content_sha256,imported_at,effective_from,currency,schema_version) VALUES('invalid','unknown','', 'short', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z', 'USD', 1)`); err != nil {
		t.Fatal(err)
	}
	if err = database.Close(); err != nil {
		t.Fatal(err)
	}
	sessions, err := store.OpenSessions(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if err = sessions.Close(); err != nil {
		t.Fatal(err)
	}
	sessions, err = store.OpenSessions(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = sessions.DB.ExecContext(ctx, `INSERT INTO session_sources(source_path,identity,cursor,size,modified_at,prefix_hash,priority,parser_version,scanned_at) VALUES(?,?,?,?,?,?,?,?,?)`, filepath.Join(t.TempDir(), "missing.jsonl"), "synthetic", 0, 0, 0, "", 0, 1, "2026-01-01T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	if err = sessions.Close(); err != nil {
		t.Fatal(err)
	}
	before := fileDigest(t, filepath.Join(root, "agentdeck.sqlite3"))
	report, err := (Service{StateRoot: root, Home: t.TempDir(), Workdir: t.TempDir(), Secrets: platform.NewMemorySecretStore(), Now: func() time.Time { return time.Unix(1000, 0) }}).Check(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	if report.Healthy || !hasCode(report, "pending_operations") || !hasCode(report, "credential_missing") || !hasCode(report, "session_source_unreadable") || !hasCode(report, "price_provenance_invalid") {
		t.Fatalf("report = %#v", report)
	}
	after := fileDigest(t, filepath.Join(root, "agentdeck.sqlite3"))
	if before != after {
		t.Fatal("doctor changed core database")
	}
}

func fileDigest(t *testing.T, path string) [32]byte {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return sha256.Sum256(contents)
}

func hasCode(report Report, code string) bool {
	for _, check := range report.Checks {
		if check.Code == code {
			return true
		}
	}
	return false
}
