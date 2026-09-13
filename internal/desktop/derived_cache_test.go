package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
)

func derivedCacheFixtureService(t *testing.T) (Service, string) {
	t.Helper()
	root := fixtureStateRoot(t)
	seedSelections(t, root)
	seedUsage(t, root, []usageSeed{{
		client: "codex", session: "cache-session", at: "2026-08-13T08:00:00Z", model: "gpt-5", input: 1200, output: 240,
		activityKind: "coding", activitySub: "feature", toolKind: "edit", baseName: "main.go", wrote: true,
	}})
	seedSessions(t, root, []sessionSeed{{
		client: "codex", id: "cache-session", project: "/Users/example/private/agent-deck", model: "gpt-5",
		first: "2026-08-13T08:00:00Z", last: "2026-08-13T09:00:00Z",
	}})
	return Service{StateRoot: root, Home: t.TempDir(), Workdir: t.TempDir(), Now: func() time.Time { return fixtureNow }, Location: time.UTC}, root
}

func TestDerivedSnapshotCachePublishesAndReadOnlySnapshotReusesIt(t *testing.T) {
	ctx := context.Background()
	service, root := derivedCacheFixtureService(t)
	if err := service.PublishDerivedSnapshotCache(ctx, WireVersion); err != nil {
		t.Fatal(err)
	}
	path := service.derivedSnapshotCachePath()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("cache mode=%#o", got)
	}
	if directory, err := os.Stat(root); err != nil || directory.Mode().Perm() != 0o700 {
		t.Fatalf("cache directory=%#v err=%v", directory, err)
	}

	core, err := store.OpenReadOnly(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	payload, found := service.loadDerivedSnapshotCache(ctx, core, fixtureNow, WireVersion)
	if closeErr := core.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	if !found || !payload.Presentation.Available || !payload.PriceAvailability.Validated {
		t.Fatalf("cache payload=%#v found=%t", payload, found)
	}
	before := snapshotFileDigest(t, path)
	result, err := service.Build(ctx, Request{WireVersion: WireVersion, RecentLimit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if result.Partial || !result.DerivedCacheHit || !result.Snapshot.Usage.Available || len(result.Snapshot.Sessions.WorkSignals.Activity.Items) == 0 {
		t.Fatalf("cached snapshot=%#v", result)
	}
	if after := snapshotFileDigest(t, path); after != before {
		t.Fatal("read-only snapshot modified derived cache")
	}
}

func TestDerivedSnapshotCacheRejectsChangedGenerationAndCorruption(t *testing.T) {
	ctx := context.Background()
	service, root := derivedCacheFixtureService(t)
	if err := service.PublishDerivedSnapshotCache(ctx, WireVersion); err != nil {
		t.Fatal(err)
	}
	database, err := store.Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.DB.ExecContext(ctx, `INSERT INTO usage_source_files(path,identity,size,cursor,prefix_hash) VALUES('changed-source','changed',0,0,'')`); err != nil {
		database.Close()
		t.Fatal(err)
	}
	if err = database.Close(); err != nil {
		t.Fatal(err)
	}
	core, err := store.OpenReadOnly(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	_, found := service.loadDerivedSnapshotCache(ctx, core, fixtureNow, WireVersion)
	if closeErr := core.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	if found {
		t.Fatal("cache reused after relevant generation changed")
	}
	if err = os.WriteFile(service.derivedSnapshotCachePath(), []byte("not-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := service.Build(ctx, Request{WireVersion: WireVersion, RecentLimit: 5})
	if err != nil || !result.Snapshot.Usage.Available {
		t.Fatalf("corrupt-cache fallback result=%#v err=%v", result, err)
	}
}

func TestDerivedSnapshotCachePublisherRejectsConcurrentGenerationChange(t *testing.T) {
	ctx := context.Background()
	service, _ := derivedCacheFixtureService(t)
	original := beforeDerivedSnapshotCacheGenerationRecheck
	var writeErr error
	beforeDerivedSnapshotCacheGenerationRecheck = func(core *store.Store) {
		_, writeErr = core.DB.ExecContext(ctx, `INSERT INTO usage_source_files(path,identity,size,cursor,prefix_hash) VALUES('concurrent-source','concurrent',0,0,'')`)
	}
	t.Cleanup(func() { beforeDerivedSnapshotCacheGenerationRecheck = original })

	err := service.PublishDerivedSnapshotCache(ctx, WireVersion)
	if writeErr != nil {
		t.Fatal(writeErr)
	}
	if !errors.Is(err, ErrDerivedSnapshotCacheChanged) {
		t.Fatalf("concurrent publication error=%v", err)
	}
	if _, statErr := os.Stat(service.derivedSnapshotCachePath()); !os.IsNotExist(statErr) {
		t.Fatalf("concurrent publication left cache: %v", statErr)
	}
}

func TestDerivedSnapshotCacheRejectsMintedDatabaseEpoch(t *testing.T) {
	ctx := context.Background()
	service, root := derivedCacheFixtureService(t)
	if err := service.PublishDerivedSnapshotCache(ctx, WireVersion); err != nil {
		t.Fatal(err)
	}
	core, err := store.Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if err = core.MintDerivedSnapshotEpoch(ctx); err != nil {
		core.Close()
		t.Fatal(err)
	}
	if err = core.Close(); err != nil {
		t.Fatal(err)
	}
	core, err = store.OpenReadOnly(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	_, found := service.loadDerivedSnapshotCache(ctx, core, fixtureNow, WireVersion)
	if closeErr := core.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	if found {
		t.Fatal("cache accepted a generation copied from an earlier database lifecycle")
	}
}

func TestDerivedSnapshotCacheDoesNotHideSessionAvailability(t *testing.T) {
	ctx := context.Background()
	service, root := derivedCacheFixtureService(t)
	if err := service.PublishDerivedSnapshotCache(ctx, WireVersion); err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{"", "-wal", "-shm"} {
		path := filepath.Join(root, "sessions.sqlite3") + suffix
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
	result, err := service.Build(ctx, Request{WireVersion: WireVersion, RecentLimit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Snapshot.Usage.Available || result.Snapshot.Sessions.Available || !result.Partial {
		t.Fatalf("independent cache/session result=%#v", result)
	}
	if !containsWarning(result.Warnings, "sessions_unavailable") {
		t.Fatalf("missing independent sessions warning=%#v", result.Warnings)
	}
}

func TestDerivedSnapshotCacheMissDoesNotWriteAndRejectsClockRollback(t *testing.T) {
	ctx := context.Background()
	service, root := derivedCacheFixtureService(t)
	miss, err := service.Build(ctx, Request{WireVersion: WireVersion, RecentLimit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if miss.DerivedCacheHit {
		t.Fatal("read-only cache miss reported a hit")
	}
	if _, err := os.Stat(service.derivedSnapshotCachePath()); !os.IsNotExist(err) {
		t.Fatalf("read-only cache miss created a cache: %v", err)
	}
	if err := service.PublishDerivedSnapshotCache(ctx, WireVersion); err != nil {
		t.Fatal(err)
	}
	core, err := store.OpenReadOnly(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	_, found := service.loadDerivedSnapshotCache(ctx, core, fixtureNow.Add(-time.Second), WireVersion)
	if closeErr := core.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	if found {
		t.Fatal("cache accepted a clock rollback before creation")
	}
	core, err = store.OpenReadOnly(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	_, found = service.loadDerivedSnapshotCache(ctx, core, fixtureNow.Add(time.Hour), WireVersion)
	if closeErr := core.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	if found {
		t.Fatal("cache accepted data beyond its next local-hour boundary")
	}
}

func TestDerivedSnapshotCachePublisherCleansOnlyOwnedTemporaryFiles(t *testing.T) {
	ctx := context.Background()
	service, root := derivedCacheFixtureService(t)
	owned := filepath.Join(root, ".desktop-derived-cache-abandoned")
	if err := os.WriteFile(owned, []byte("owned"), 0o600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "not-owned-target")
	if err := os.WriteFile(target, []byte("not-owned"), 0o600); err != nil {
		t.Fatal(err)
	}
	notOwned := filepath.Join(root, ".desktop-derived-cache-not-owned")
	if err := os.Symlink(target, notOwned); err != nil {
		t.Fatal(err)
	}
	if err := service.PublishDerivedSnapshotCache(ctx, WireVersion); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(owned); !os.IsNotExist(err) {
		t.Fatalf("owned temporary remained: %v", err)
	}
	if info, err := os.Lstat(notOwned); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("non-owned symlink was removed or followed: %#v err=%v", info, err)
	}
}

func TestDerivedSnapshotCacheWriteDoesNotRecreateMissingStateRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "deleted-state")
	err := writeDerivedSnapshotCache(filepath.Join(root, derivedSnapshotCacheFilename), []byte("payload"))
	if err == nil || !os.IsNotExist(err) {
		t.Fatalf("write missing state root error=%v", err)
	}
	if _, statErr := os.Stat(root); !os.IsNotExist(statErr) {
		t.Fatalf("cache write recreated missing state root: %v", statErr)
	}
}

func TestDerivedSnapshotCacheRejectsIncompatibleAndUnsafeEntries(t *testing.T) {
	ctx := context.Background()
	service, root := derivedCacheFixtureService(t)
	if err := service.PublishDerivedSnapshotCache(ctx, WireVersion); err != nil {
		t.Fatal(err)
	}
	path := service.derivedSnapshotCachePath()
	document, err := readDerivedSnapshotCache(path)
	if err != nil {
		t.Fatal(err)
	}
	document.Header.UsageParserVersion++
	document.Checksum, err = derivedSnapshotCacheChecksum(document.Header, document.Payload)
	if err != nil {
		t.Fatal(err)
	}
	contents, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, append(contents, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	core, err := store.OpenReadOnly(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	_, found := service.loadDerivedSnapshotCache(ctx, core, fixtureNow, WireVersion)
	if closeErr := core.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	if found {
		t.Fatal("cache accepted a mismatched usage parser version")
	}
	if err = service.PublishDerivedSnapshotCache(ctx, WireVersion); err != nil {
		t.Fatal(err)
	}
	document, err = readDerivedSnapshotCache(path)
	if err != nil {
		t.Fatal(err)
	}
	document.Header.DatabaseIdentity = "restored-database"
	document.Checksum, err = derivedSnapshotCacheChecksum(document.Header, document.Payload)
	if err != nil {
		t.Fatal(err)
	}
	contents, err = json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, append(contents, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	core, err = store.OpenReadOnly(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	_, found = service.loadDerivedSnapshotCache(ctx, core, fixtureNow, WireVersion)
	if closeErr := core.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	if found {
		t.Fatal("cache accepted a different restored database identity")
	}
	if err = service.PublishDerivedSnapshotCache(ctx, WireVersion); err != nil {
		t.Fatal(err)
	}
	service.Location = time.FixedZone("UTC+08", 8*60*60)
	core, err = store.OpenReadOnly(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	_, found = service.loadDerivedSnapshotCache(ctx, core, fixtureNow, WireVersion)
	if closeErr := core.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	if found {
		t.Fatal("cache accepted a different timezone/window identity")
	}
	service.Location = time.UTC
	if err = service.PublishDerivedSnapshotCache(ctx, WireVersion); err != nil {
		t.Fatal(err)
	}
	if err = os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err = readDerivedSnapshotCache(path); err == nil {
		t.Fatal("cache reader accepted a non-private cache file")
	}
	if err = os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, make([]byte, derivedSnapshotCacheMaxBytes+1), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err = readDerivedSnapshotCache(path); !errors.Is(err, ErrDerivedSnapshotCacheTooLarge) {
		t.Fatalf("oversized cache error=%v", err)
	}
}

func containsWarning(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
