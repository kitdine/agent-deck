package store

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/kitdine/agent-deck/internal/platform"
)

// openV6Fixture replays the pre-wrapper fixture through the full migration
// sequence, which is how a database written before this field existed reaches
// the current schema.
func openV6Fixture(t *testing.T) *Store {
	t.Helper()
	ctx := context.Background()
	state := filepath.Join(t.TempDir(), "state")
	if err := os.MkdirAll(state, platform.DirectoryMode); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", filepath.Join(state, "agentdeck.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	fixture, err := os.ReadFile(filepath.Join("testdata", "agentdeck-v6.sql"))
	if err != nil {
		db.Close()
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, string(fixture)); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	migrated, err := Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { migrated.Close() })
	return migrated
}

// TestV16MigrationAddsWrapperKindLeavingExistingWrappersUndeclared is the
// acceptance case for an existing database: every provider it already held must
// come back with no declared protocol, which the service layer reads as the
// default. A migration that defaulted the column to a concrete value would opt
// existing wrappers into a protocol their owner never chose.
func TestV16MigrationAddsWrapperKindLeavingExistingWrappersUndeclared(t *testing.T) {
	ctx := context.Background()
	migrated := openV6Fixture(t)

	if version, err := migrated.SchemaVersion(ctx); err != nil || version != CurrentSchemaVersion {
		t.Fatalf("schema version = %d, %v", version, err)
	}
	providers, err := migrated.ListProviders(ctx)
	if err != nil || len(providers) != 1 {
		t.Fatalf("providers = %#v, %v", providers, err)
	}
	if providers[0].Name != "legacy" || providers[0].WrapperKind != "" || providers[0].WrapperURL != "" {
		t.Fatalf("migrated provider = %#v, want no wrapper and no declared kind", providers[0])
	}
	// The v15 fields the same rows carry must survive the added column.
	snapshot, err := migrated.CurrentProviderSnapshot(ctx, "codex")
	if err != nil || snapshot.ViaWrapper || snapshot.Name != "legacy" || snapshot.Endpoint != "https://legacy.example" {
		t.Fatalf("selection after migration = %#v, %v", snapshot, err)
	}
	if kind, err := migrated.OfficialWrapperKind(ctx); err != nil || kind != "" {
		t.Fatalf("official wrapper kind after migration = %q, %v", kind, err)
	}
}

// TestSetProviderWrapperRoundTripsKindAndClearsItWithTheURL pins that the
// protocol is stored beside the URL it describes and cannot outlive it.
func TestSetProviderWrapperRoundTripsKindAndClearsItWithTheURL(t *testing.T) {
	ctx := context.Background()
	migrated := openV6Fixture(t)

	updated, err := migrated.SetProviderWrapper(ctx, "legacy", "https://proxy.example", "headroom")
	if err != nil || updated.WrapperURL != "https://proxy.example" || updated.WrapperKind != "headroom" {
		t.Fatalf("SetProviderWrapper = %#v, %v", updated, err)
	}
	reread, err := migrated.ProviderByName(ctx, "legacy")
	if err != nil || reread.WrapperKind != "headroom" {
		t.Fatalf("reread kind = %#v, %v", reread, err)
	}

	cleared, err := migrated.SetProviderWrapper(ctx, "legacy", "", "")
	if err != nil || cleared.WrapperURL != "" || cleared.WrapperKind != "" {
		t.Fatalf("cleared wrapper = %#v, %v", cleared, err)
	}
}

// TestSetOfficialWrapperURLClearsKindWithTheURL covers the built-in provider,
// whose wrapper lives in the settings table and therefore clears through two
// deletes rather than one row update — the path where a protocol could most
// easily be orphaned.
func TestSetOfficialWrapperURLClearsKindWithTheURL(t *testing.T) {
	ctx := context.Background()
	migrated := openV6Fixture(t)

	if err := migrated.SetOfficialWrapperURL(ctx, "https://proxy.example", "headroom"); err != nil {
		t.Fatal(err)
	}
	if kind, err := migrated.OfficialWrapperKind(ctx); err != nil || kind != "headroom" {
		t.Fatalf("official kind after set = %q, %v", kind, err)
	}

	if err := migrated.SetOfficialWrapperURL(ctx, "", ""); err != nil {
		t.Fatal(err)
	}
	if kind, err := migrated.OfficialWrapperKind(ctx); err != nil || kind != "" {
		t.Fatalf("official kind after clear = %q, %v", kind, err)
	}
	if url, err := migrated.OfficialWrapperURL(ctx); err != nil || url != "" {
		t.Fatalf("official url after clear = %q, %v", url, err)
	}
}

// TestSetOfficialWrapperURLReplacingAKindDropsThePreviousOne pins that
// re-declaring a wrapper without a protocol removes the old declaration rather
// than leaving the previous one attached to a new URL. This is the shape the CLI
// actually produces: the service stores the default as absence, so an omitted
// --kind arrives here as "".
func TestSetOfficialWrapperURLReplacingAKindDropsThePreviousOne(t *testing.T) {
	ctx := context.Background()
	migrated := openV6Fixture(t)

	if err := migrated.SetOfficialWrapperURL(ctx, "https://first.example", "headroom"); err != nil {
		t.Fatal(err)
	}
	if err := migrated.SetOfficialWrapperURL(ctx, "https://second.example", ""); err != nil {
		t.Fatal(err)
	}

	if kind, err := migrated.OfficialWrapperKind(ctx); err != nil || kind != "" {
		t.Fatalf("official kind after undeclared re-set = %q, %v", kind, err)
	}
}

// TestOfficialWrapperWriteLeavesNoOrphanedDeclaration pins the invariant the
// transaction exists for: the two settings rows the built-in provider needs are
// never observable in a state where a declaration outlives the URL it describes.
// Every reachable combination is checked against the settings table directly,
// because the row pair is the only place that state could appear.
func TestOfficialWrapperWriteLeavesNoOrphanedDeclaration(t *testing.T) {
	ctx := context.Background()
	migrated := openV6Fixture(t)

	for _, test := range []struct {
		name     string
		url      string
		kind     string
		wantURL  string
		wantKind string
	}{
		{name: "declared", url: "https://a.example", kind: "headroom", wantURL: "https://a.example", wantKind: "headroom"},
		{name: "undeclared replaces declared", url: "https://b.example", kind: "", wantURL: "https://b.example", wantKind: ""},
		{name: "declared again", url: "https://c.example", kind: "headroom", wantURL: "https://c.example", wantKind: "headroom"},
		{name: "cleared", url: "", kind: "", wantURL: "", wantKind: ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := migrated.SetOfficialWrapperURL(ctx, test.url, test.kind); err != nil {
				t.Fatal(err)
			}
			url, err := migrated.OfficialWrapperURL(ctx)
			if err != nil {
				t.Fatal(err)
			}
			kind, err := migrated.OfficialWrapperKind(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if url != test.wantURL || kind != test.wantKind {
				t.Fatalf("url = %q, kind = %q, want %q and %q", url, kind, test.wantURL, test.wantKind)
			}
			if url == "" && kind != "" {
				t.Fatalf("declaration %q outlived its URL", kind)
			}
		})
	}
}
