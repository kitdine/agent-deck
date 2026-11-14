package claudemem

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/sessionorigin/adapters/claudemem/claudememtest"
)

const (
	observerID = "11111111-1111-4111-8111-111111111111" // SDK observer: exact memory_session_id
	userID     = "22222222-2222-4222-8222-222222222222" // user work: only content_session_id
	otherID    = "33333333-3333-4333-8333-333333333333" // not present at all
)

type fixture struct {
	dir    string
	path   string
	writer *sql.DB
}

// newFixture builds the faithful 15.2.1 fixture: the observer row's memory ID
// is the Claude canonical ID of the observer transcript and its content ID is
// the user's session; a synthetic provider memory ID is never a Claude ID.
func newFixture(t *testing.T, version int, mutate ...string) *fixture {
	t.Helper()
	f := claudememtest.New(t, version, []claudememtest.Mapping{
		{RowID: 7, ContentID: userID, MemoryID: observerID},
		{RowID: 8, ContentID: otherID, MemoryID: "gemini-" + otherID + "-1700000000"},
	}, mutate...)
	return &fixture{dir: f.Dir, path: f.Path, writer: f.Writer}
}

func reviewedHome(t *testing.T) string {
	t.Helper()
	return claudememtest.Home(t, claudememtest.ReviewedInstall())
}

func fileDigest(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

func traced(t *testing.T) *[]string {
	t.Helper()
	statements := &[]string{}
	queryTrace = func(statement string) { *statements = append(*statements, statement) }
	t.Cleanup(func() { queryTrace = nil })
	return statements
}

func TestReadMapsOnlyExactMemorySessionIDs(t *testing.T) {
	f := newFixture(t, SupportedSchemaVersion)
	before := map[string]string{}
	for _, name := range []string{f.path, f.path + "-wal"} {
		before[name] = fileDigest(t, name)
	}
	statements := traced(t)
	synthetic := "gemini-" + otherID + "-1700000000"
	result := Read(context.Background(), reviewedHome(t), f.dir, []string{observerID, userID, otherID, synthetic, "not-a-uuid"})
	if result.Outcome != OutcomeOK || result.Reason != "" || result.ValidatedVersion != "15.2.1/sdk-schema-68" {
		t.Fatalf("result = %+v", result)
	}
	if len(result.Mapped) != 1 || result.Mapped[observerID] != 7 {
		t.Fatalf("only the exact memory ID may map, got %v", result.Mapped)
	}
	for _, id := range []string{observerID, userID, otherID, synthetic, "not-a-uuid"} {
		if !result.Checked[id] {
			t.Fatalf("%s not checked: %v", id, result.Checked)
		}
	}
	// The source database and its WAL are byte-for-byte unchanged.
	for name, digest := range before {
		if got := fileDigest(t, name); got != digest {
			t.Fatalf("%s changed by a read-only adapter", filepath.Base(name))
		}
	}
	// The query surface is the declared allowlist: identifier and version
	// columns only, never SELECT *, content or other tables.
	allowed := []string{querySchemaVersion, queryColumns, queryIndexes, queryIndexColumns, queryMappingPrefix}
	for _, statement := range *statements {
		ok := false
		for _, prefix := range allowed {
			ok = ok || strings.HasPrefix(statement, prefix)
		}
		lower := strings.ToLower(statement)
		for _, forbidden := range []string{"*", "content_session_id", "platform_source", "project", "user_prompt", "observations", "tool_uses", "prompt_text"} {
			if strings.Contains(lower, forbidden) {
				t.Fatalf("statement reads forbidden %q: %s", forbidden, statement)
			}
		}
		if !ok {
			t.Fatalf("statement outside the allowlist: %s", statement)
		}
	}
	// Only canonical UUIDs reach the mapping query.
	mapping := (*statements)[len(*statements)-1]
	if strings.Count(mapping, "?") != 3 {
		t.Fatalf("non-UUID candidates were queried: %s", mapping)
	}
}

func TestReadBatchesCandidatesAndSkipsAnEmptySet(t *testing.T) {
	f := newFixture(t, SupportedSchemaVersion)
	for _, n := range []int{0, 1, 200, 201} {
		statements := traced(t)
		ids := make([]string, n)
		for i := range ids {
			ids[i] = fmt.Sprintf("%08x-0000-4000-8000-%012x", i, i)
		}
		result := Read(context.Background(), reviewedHome(t), f.dir, ids)
		if result.Outcome != OutcomeOK || len(result.Checked) != n {
			t.Fatalf("n=%d result=%+v", n, result)
		}
		batches := 0
		for _, statement := range *statements {
			if strings.HasPrefix(statement, queryMappingPrefix) {
				batches++
				if params := strings.Count(statement, "?"); params > BatchSize {
					t.Fatalf("batch of %d", params)
				}
			}
		}
		if want := (n + BatchSize - 1) / BatchSize; batches != want {
			t.Fatalf("n=%d batches=%d want %d", n, batches, want)
		}
	}
}

func TestReadRejectsUnsupportedVersionsAndShapes(t *testing.T) {
	cases := map[string]*fixture{
		"newer version": newFixture(t, SupportedSchemaVersion+1),
		"older version": newFixture(t, SupportedSchemaVersion-1),
		"no unique memory index": newFixture(t, SupportedSchemaVersion,
			`CREATE TABLE sdk_sessions_new (id INTEGER PRIMARY KEY, memory_session_id TEXT)`,
			`DROP TABLE sdk_sessions`, `ALTER TABLE sdk_sessions_new RENAME TO sdk_sessions`,
			`CREATE INDEX idx_memory ON sdk_sessions(memory_session_id)`),
		"no schema_versions": newFixture(t, SupportedSchemaVersion, `DROP TABLE schema_versions`),
	}
	for name, f := range cases {
		result := Read(context.Background(), reviewedHome(t), f.dir, []string{observerID})
		if result.Outcome != OutcomeUnsupported || len(result.Mapped) != 0 || len(result.Checked) != 0 {
			t.Fatalf("%s: %+v", name, result)
		}
	}
	// A directory that only pretends to be a claude-mem root.
	fake := t.TempDir()
	if err := os.WriteFile(filepath.Join(fake, DatabaseName), []byte("not a database at all, just text"), 0o600); err != nil {
		t.Fatal(err)
	}
	if result := Read(context.Background(), reviewedHome(t), fake, []string{observerID}); result.Outcome != OutcomeUnsupported || len(result.Mapped) != 0 {
		t.Fatalf("pseudo root: %+v", result)
	}
}

func TestReadNeverCreatesOrModifiesSourceFiles(t *testing.T) {
	// No database: unavailable, and nothing is created.
	empty := t.TempDir()
	if result := Read(context.Background(), reviewedHome(t), empty, []string{observerID}); result.Outcome != OutcomeUnavailable || result.Reason != ReasonDatabaseMissing {
		t.Fatalf("missing database: %+v", result)
	}
	if entries, _ := os.ReadDir(empty); len(entries) != 0 {
		t.Fatalf("adapter created %v", entries)
	}
	// Relative or unclean roots are never resolved.
	for _, root := range []string{"relative/dir", "/tmp/../tmp"} {
		if result := Read(context.Background(), reviewedHome(t), root, nil); result.Outcome != OutcomeUnavailable || result.Reason != ReasonRootInvalid {
			t.Fatalf("%s: %+v", root, result)
		}
	}

	// A WAL database whose sidecars are gone cannot be read without creating
	// them, so the adapter refuses instead.
	f := newFixture(t, SupportedSchemaVersion)
	if err := f.writer.Close(); err != nil { // checkpoints and removes the sidecars
		t.Fatal(err)
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		if _, err := os.Stat(f.path + suffix); !os.IsNotExist(err) {
			t.Fatalf("sidecar %s still present: %v", suffix, err)
		}
	}
	digest := fileDigest(t, f.path)
	if result := Read(context.Background(), reviewedHome(t), f.dir, []string{observerID}); result.Outcome != OutcomeUnavailable || result.Reason != ReasonReadOnlyUnmet {
		t.Fatalf("sidecar-less WAL database: %+v", result)
	}
	for _, suffix := range []string{"-wal", "-shm", "-journal"} {
		if _, err := os.Stat(f.path + suffix); !os.IsNotExist(err) {
			t.Fatalf("adapter created %s: %v", suffix, err)
		}
	}
	if fileDigest(t, f.path) != digest {
		t.Fatal("database changed")
	}
}

func TestReadReportsPermissionDenied(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores file permissions")
	}
	f := newFixture(t, SupportedSchemaVersion)
	if err := os.Chmod(f.path, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(f.path, 0o600) })
	result := Read(context.Background(), reviewedHome(t), f.dir, []string{observerID})
	if result.Outcome != OutcomeUnavailable || result.Reason != ReasonPermissionDenied {
		t.Fatalf("result = %+v", result)
	}
	info, err := os.Stat(f.path)
	if err != nil || info.Mode().Perm() != 0 {
		t.Fatalf("adapter changed permissions: %v %v", info.Mode(), err)
	}
}

func TestReadBoundsSlowReads(t *testing.T) {
	f := newFixture(t, SupportedSchemaVersion)
	ids := make([]string, 0, 401)
	ids = append(ids, observerID)
	for i := 0; i < 400; i++ {
		ids = append(ids, fmt.Sprintf("%08x-0000-4000-8000-%012x", i, i))
	}
	// Every mapping read after the first stalls past its per-read deadline.
	mappings := 0
	queryTrace = func(statement string) {
		if strings.HasPrefix(statement, queryMappingPrefix) {
			mappings++
		}
	}
	readDelay = func(ctx context.Context) error {
		if mappings < 2 {
			return nil
		}
		<-ctx.Done()
		return ctx.Err()
	}
	t.Cleanup(func() { readDelay, queryTrace = nil, nil })
	started := time.Now()
	result := Read(context.Background(), reviewedHome(t), f.dir, ids)
	if elapsed := time.Since(started); elapsed > RoundTimeout {
		t.Fatalf("read took %v", elapsed)
	}
	if result.Outcome != OutcomePartial || result.Reason != ReasonTimeout {
		t.Fatalf("result = %+v", result)
	}
	if result.Mapped[observerID] != 7 || len(result.Checked) != BatchSize {
		t.Fatalf("first batch must keep its result and the rest stay unchecked: mapped=%v checked=%d", result.Mapped, len(result.Checked))
	}
	// A stall before any mapping read leaves nothing usable.
	mappings = 2
	if result := Read(context.Background(), reviewedHome(t), f.dir, ids); result.Outcome != OutcomeUnavailable || len(result.Checked) != 0 {
		t.Fatalf("result = %+v", result)
	}
}

// ASC-OAR-R1-F1: the schema-68 database alone does not prove the observer's
// purpose semantics; only the confirmed reviewed package version is adapted.
func TestReadRejectsUnconfirmedPluginVersion(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	f := newFixture(t, SupportedSchemaVersion)
	reviewed := claudememtest.ReviewedInstall()
	with := func(change func(*claudememtest.Install)) claudememtest.Install {
		install := reviewed
		change(&install)
		return install
	}
	upgraded := with(func(i *claudememtest.Install) { i.Version, i.Package = "15.3.0", "15.3.0" })
	cases := map[string]struct {
		home    string
		outcome string
		reason  string
	}{
		"no catalog":                   {t.TempDir(), OutcomeUnsupported, ReasonPluginVersion},
		"relative home":                {"home", OutcomeUnavailable, ReasonPluginUnreadable},
		"unknown newer version":        {claudememtest.Home(t, with(func(i *claudememtest.Install) { i.Version, i.Package = "15.9.0", "15.9.0" })), OutcomeUnsupported, ReasonPluginVersion},
		"older version":                {claudememtest.Home(t, with(func(i *claudememtest.Install) { i.Version, i.Package = "15.2.0", "15.2.0" })), OutcomeUnsupported, ReasonPluginVersion},
		"schema unchanged, upgraded":   {claudememtest.Home(t, upgraded), OutcomeUnsupported, ReasonPluginVersion},
		"catalog and package differ":   {claudememtest.Home(t, with(func(i *claudememtest.Install) { i.Package = "15.3.0" })), OutcomeUnsupported, ReasonPluginVersion},
		"other package name":           {claudememtest.Home(t, with(func(i *claudememtest.Install) { i.PackageName = "claude-mem-fork" })), OutcomeUnsupported, ReasonPluginVersion},
		"no package manifest":          {claudememtest.Home(t, with(func(i *claudememtest.Install) { i.Package = "" })), OutcomeUnsupported, ReasonPluginVersion},
		"other marketplace":            {claudememtest.Home(t, with(func(i *claudememtest.Install) { i.ID = "claude-mem@fork" })), OutcomeUnsupported, ReasonPluginVersion},
		"project scope only":           {claudememtest.Home(t, with(func(i *claudememtest.Install) { i.Scope = "project" })), OutcomeUnsupported, ReasonPluginVersion},
		"ambiguous: two user installs": {claudememtest.Home(t, reviewed, upgraded), OutcomeUnsupported, ReasonPluginVersion},
	}
	for name, c := range cases {
		statements := traced(t)
		result := Read(context.Background(), c.home, f.dir, []string{observerID})
		if result.Outcome != c.outcome || result.Reason != c.reason || len(result.Mapped) != 0 || len(result.Checked) != 0 || result.ValidatedVersion != "" {
			t.Fatalf("%s: unconfirmed plugin version accepted: %+v", name, result)
		}
		if len(*statements) != 0 {
			t.Fatalf("%s: database queried before the plugin was confirmed: %v", name, *statements)
		}
	}
	// An unreadable catalog is unavailable, not unsupported.
	broken := claudememtest.Home(t, reviewed)
	if err := os.WriteFile(filepath.Join(broken, ".claude", "plugins", "installed_plugins.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if result := Read(context.Background(), broken, f.dir, []string{observerID}); result.Outcome != OutcomeUnavailable || result.Reason != ReasonPluginUnreadable || len(result.Mapped) != 0 {
		t.Fatalf("unreadable catalog: %+v", result)
	}
	// The reviewed version on the same database is adapted.
	if result := Read(context.Background(), claudememtest.Home(t, reviewed), f.dir, []string{observerID}); result.Outcome != OutcomeOK || result.Mapped[observerID] != 7 {
		t.Fatalf("reviewed version: %+v", result)
	}
}

// ASC-OAR-R1-F1 (Round 2): no install of claude-mem may be filtered out
// before confirmation. Nothing ties an install to the configured data root,
// so any second install, whatever its scope, version or marketplace, leaves
// the data's producer unproven.
func TestReadRejectsMixedPluginInstalls(t *testing.T) {
	f := newFixture(t, SupportedSchemaVersion)
	reviewed := claudememtest.ReviewedInstall()
	with := func(change func(*claudememtest.Install)) claudememtest.Install {
		install := reviewed
		change(&install)
		return install
	}
	project := t.TempDir()
	cases := map[string][]claudememtest.Install{
		"user 15.2.1 + project 15.9.0 (review counterexample)": {reviewed, with(func(i *claudememtest.Install) {
			i.Scope, i.ProjectPath, i.Version, i.Package = "project", project, "15.9.0", "15.9.0"
		})},
		"user 15.2.1 + project 15.2.1": {reviewed, with(func(i *claudememtest.Install) { i.Scope, i.ProjectPath = "project", project })},
		"user 15.2.1 + local 15.3.0": {reviewed, with(func(i *claudememtest.Install) {
			i.Scope, i.ProjectPath, i.Version, i.Package = "local", project, "15.3.0", "15.3.0"
		})},
		"user 15.2.1 + other marketplace 15.2.1": {reviewed, with(func(i *claudememtest.Install) { i.ID = "claude-mem@fork" })},
		"user 15.2.1 + unknown scope 15.9.0": {reviewed, with(func(i *claudememtest.Install) {
			i.Scope, i.Version, i.Package = "managed", "15.9.0", "15.9.0"
		})},
	}
	for name, installs := range cases {
		statements := traced(t)
		result := Read(context.Background(), claudememtest.Home(t, installs...), f.dir, []string{observerID})
		if result.Outcome != OutcomeUnsupported || result.Reason != ReasonPluginVersion || len(result.Mapped) != 0 || result.ValidatedVersion != "" {
			t.Fatalf("%s: mixed installs accepted: %+v", name, result)
		}
		if len(*statements) != 0 {
			t.Fatalf("%s: database queried: %v", name, *statements)
		}
	}
	// An unrelated plugin beside the reviewed install does not matter.
	other := claudememtest.Install{ID: "claude-mem-extras@thedotmack", Scope: "user", Version: "1.0.0"}
	if result := Read(context.Background(), claudememtest.Home(t, reviewed, other), f.dir, []string{observerID}); result.Outcome != OutcomeOK || result.Mapped[observerID] != 7 {
		t.Fatalf("unrelated plugin blocked the reviewed install: %+v", result)
	}
}
