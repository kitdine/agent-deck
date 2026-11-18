package store

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/platform"
)

func servingTestStores(t *testing.T) (string, *Store, *Store) {
	t.Helper()
	ctx := context.Background()
	root := filepath.Join(t.TempDir(), "state")
	core, err := Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { core.Close() })
	sessions, err := OpenSessions(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sessions.Close() })
	return root, core, sessions
}

func TestServingGenerationReadOnlyAndDirtyRevision(t *testing.T) {
	ctx := context.Background()
	root, core, _ := servingTestStores(t)
	before, err := ReadServingGeneration(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"first", "second"} {
		if err = core.SetSetting(ctx, "serving-test", value); err != nil {
			t.Fatal(err)
		}
	}
	after, err := ReadServingGeneration(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if after.StateIdentity != before.StateIdentity || after.CoreEpoch != before.CoreEpoch || after.UsageRevision <= before.UsageRevision+1 {
		t.Fatalf("revision did not advance for each dirty write: before=%+v after=%+v", before, after)
	}
	again, err := ReadServingGeneration(ctx, root)
	if err != nil || again != after {
		t.Fatalf("read mutated generation: %+v %v", again, err)
	}
	if err = core.MintDerivedSnapshotEpoch(ctx); err != nil {
		t.Fatal(err)
	}
	restored, err := ReadServingGeneration(ctx, root)
	if err != nil || restored.CoreEpoch == after.CoreEpoch {
		t.Fatalf("restore identity did not change: %+v %v", restored, err)
	}
}

func TestServingGenerationRejectsMissingAndUnsafeState(t *testing.T) {
	ctx := context.Background()
	missing := filepath.Join(t.TempDir(), "missing")
	if _, err := ReadServingGeneration(ctx, missing); err == nil {
		t.Fatal("missing state accepted")
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatalf("read created state: %v", err)
	}
	root, _, _ := servingTestStores(t)
	path := filepath.Join(root, "agentdeck.sqlite3")
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadServingGeneration(ctx, root); err == nil {
		t.Fatal("public database accepted")
	}
	if err := os.Chmod(path, 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "linked-state")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	// State-root aliases are resolved by the ownership guard. SQLite files
	// themselves retain their independent private, no-symlink checks.
	direct, err := ReadServingGeneration(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	aliased, err := ReadServingGeneration(ctx, link)
	if err != nil || aliased != direct {
		t.Fatalf("physical state identity = %+v, %v; want %+v", aliased, err, direct)
	}
}

func TestSourceCoverageCommitRollbackAndFiniteInventory(t *testing.T) {
	ctx := context.Background()
	_, core, _ := servingTestStores(t)
	coverage, err := core.ReadSourceCoverage(ctx)
	if err != nil || coverage.State != "unknown" {
		t.Fatalf("new DB fabricated coverage: %+v %v", coverage, err)
	}
	generation, err := core.DerivedSnapshotGeneration(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = core.AcceptInventory(ctx, "finite", generation.Epoch, []InventorySource{{Path: "source", Identity: "identity", End: 123}}, time.Now()); err != nil {
		t.Fatal(err)
	}
	coverage, err = core.ReadSourceCoverage(ctx)
	if err != nil || coverage.State != "update_pending" {
		t.Fatalf("pending acceptance lost: %+v %v", coverage, err)
	}
	tx, err := core.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = CommitSourceCoverage(ctx, tx, "finite", "source", "identity", 123, 123); err != nil {
		t.Fatal(err)
	}
	tx.Rollback()
	if err = core.FinishInventory(ctx, "finite", "complete"); err == nil {
		t.Fatal("rolled back source claimed complete")
	}
	tx, err = core.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = CommitSourceCoverage(ctx, tx, "finite", "source", "wrong", 123, 123); err == nil {
		t.Fatal("wrong source identity accepted")
	}
	if err = CommitSourceCoverage(ctx, tx, "finite", "source", "identity", 123, 123); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	coverage, err = core.ReadSourceCoverage(ctx)
	if err != nil || coverage.State != "partial" || coverage.Committed != 1 {
		t.Fatalf("source commit prematurely completed finite scan: %+v %v", coverage, err)
	}
	if err = core.FinishInventory(ctx, "finite", "complete"); err != nil {
		t.Fatal(err)
	}
	coverage, err = core.ReadSourceCoverage(ctx)
	if err != nil || coverage.State != "complete" {
		t.Fatalf("complete finite inventory lost: %+v %v", coverage, err)
	}
	if err = core.AcceptInventory(ctx, "followup", generation.Epoch, []InventorySource{{Path: "source", Identity: "identity", End: 456}}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err = core.FinishInventory(ctx, "followup", "paused"); err != nil {
		t.Fatal(err)
	}
	coverage, err = core.ReadSourceCoverage(ctx)
	if err != nil || coverage.State != "partial" || coverage.InventoryID != "followup" {
		t.Fatalf("paused history erased or fabricated coverage: %+v %v", coverage, err)
	}
}

func TestServingGenerationReaderRechecksValuesAndFileIdentity(t *testing.T) {
	root, core, _ := servingTestStores(t)
	ctx := context.Background()
	reader, err := OpenServingGenerationReader(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	before, err := reader.Read()
	if err != nil {
		t.Fatal(err)
	}
	if err := core.SetSetting(ctx, "reader-recheck", "changed"); err != nil {
		t.Fatal(err)
	}
	after, err := reader.Read()
	if err != nil {
		t.Fatal(err)
	}
	if after.UsageRevision <= before.UsageRevision {
		t.Fatal("reader cached generation values")
	}
	if err := core.MintDerivedSnapshotEpoch(ctx); err != nil {
		t.Fatal(err)
	}
	epoch, err := reader.Read()
	if err != nil || epoch.CoreEpoch == after.CoreEpoch {
		t.Fatalf("cached epoch: %+v %v", epoch, err)
	}
	path := filepath.Join(root, "sessions.sqlite3")
	if err := os.Rename(path, path+".old"); err != nil {
		t.Fatal(err)
	}
	defer os.Rename(path+".old", path)
	if err := os.WriteFile(path, []byte("replacement"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := reader.Read(); err == nil {
		t.Fatal("open connection hid replaced database")
	}
}

func TestServingGenerationReaderBudgetAndCancellation(t *testing.T) {
	root, _, _ := servingTestStores(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	start := time.Now()
	reader, err := OpenServingGenerationReader(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	deadline, ok := reader.ctx.Deadline()
	if !ok || deadline.Before(start.Add(200*time.Millisecond)) || deadline.After(time.Now().Add(200*time.Millisecond)) {
		t.Fatal("identity budget increased")
	}
	if _, err := reader.Read(); err != nil {
		t.Fatal(err)
	}
	next, _ := reader.ctx.Deadline()
	if !next.Equal(deadline) {
		t.Fatal("read renewed deadline")
	}
	cancel()
	if _, err := reader.Read(); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel ignored: %v", err)
	}
}

// holdIdentityConnection occupies the process identity pool's only connection
// to the core database until the returned release runs. Readers on that pool
// then wait inside the serving reader exactly as they do when the process is
// slow, without depending on machine load. The returned pool counts those
// waits, so a test can see a reader enter one.
func holdIdentityConnection(t *testing.T, root string) (*sql.DB, func()) {
	t.Helper()
	guarded, err := platform.GuardStateRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	db, release, err := acquireReadOnlySQL(context.Background(), filepath.Join(guarded, "agentdeck.sqlite3"), "identity")
	if err != nil {
		t.Fatal(err)
	}
	conn, err := db.Conn(context.Background())
	if err != nil {
		_ = release()
		t.Fatal(err)
	}
	var once sync.Once
	free := func() {
		once.Do(func() {
			_ = conn.Close()
			_ = release()
		})
	}
	t.Cleanup(free)
	return db, free
}

// awaitIdentityWaiter returns once a reader has started waiting for the held
// identity connection: the pool counts a wait when it begins, so this is
// ordered after the reader entered the wait however the scheduler runs.
func awaitIdentityWaiter(t *testing.T, db *sql.DB, before int64) {
	t.Helper()
	guard := time.Now().Add(time.Minute) // hang guard only
	for db.Stats().WaitCount <= before {
		if time.Now().After(guard) {
			t.Fatal("no reader waited for the identity connection")
		}
		time.Sleep(time.Millisecond)
	}
}

type servingVerification struct {
	generation ServingGeneration
	err        error
	elapsed    time.Duration
}

// startVerification runs VerifyServingGeneration in the background; its
// elapsed time starts when the call does.
func startVerification(ctx context.Context, root string) <-chan servingVerification {
	done := make(chan servingVerification, 1)
	go func() {
		start := time.Now()
		generation, err := VerifyServingGeneration(ctx, root)
		done <- servingVerification{generation, err, time.Since(start)}
	}()
	return done
}

func awaitVerification(t *testing.T, done <-chan servingVerification) servingVerification {
	t.Helper()
	select {
	case result := <-done:
		return result
	case <-time.After(time.Minute):
		t.Fatal("verification did not return") // hang guard only
		return servingVerification{}
	}
}

func TestServingGenerationVerifyOutlastsIdentityBudgetOnHealthyState(t *testing.T) {
	root, _, _ := servingTestStores(t)
	ctx := context.Background()
	want, err := ReadServingGeneration(ctx, root)
	if err != nil {
		t.Fatal(err)
	}

	// The startup identity check keeps its budget: a connection held past it
	// still makes identity unknown there.
	db, free := holdIdentityConnection(t, root)
	if _, err := ReadServingGeneration(ctx, root); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("identity check outlived its budget: %v", err)
	}

	// Verification waits for the same connection past that budget. It is
	// released only after the verification is seen waiting and has run longer
	// than the identity budget, so a slow scheduler only lengthens the hold.
	waits := db.Stats().WaitCount
	start := time.Now()
	done := startVerification(ctx, root)
	awaitIdentityWaiter(t, db, waits)
	time.Sleep(time.Until(start.Add(servingIdentityBudget + 100*time.Millisecond)))
	free()
	got := awaitVerification(t, done)
	if got.err != nil {
		t.Fatalf("healthy state reported unavailable after %v: %v", got.elapsed, got.err)
	}
	if got.elapsed <= servingIdentityBudget {
		t.Fatalf("verification did not wait for the held connection: %v", got.elapsed)
	}
	if got.generation != want {
		t.Fatalf("verified generation = %+v, want %+v", got.generation, want)
	}
}

// Without a caller deadline, a verification whose identity connection stays
// busy ends at VerifyServingGeneration's own budget: the ordinary read-only
// busy timeout, never longer and never the startup identity budget.
func TestServingGenerationVerifyEndsAtItsOwnBudget(t *testing.T) {
	root, _, _ := servingTestStores(t)
	db, _ := holdIdentityConnection(t, root)
	waits := db.Stats().WaitCount
	got := awaitVerification(t, startVerification(context.Background(), root))
	if !errors.Is(got.err, context.DeadlineExceeded) {
		t.Fatalf("held identity connection = %v after %v", got.err, got.elapsed)
	}
	if db.Stats().WaitCount <= waits {
		t.Fatal("verification never waited for the identity connection")
	}
	// The deadline cannot fire early; returning takes far less than the slack.
	const slack = 2 * time.Second
	if got.elapsed < readOnlyBusyTimeout || got.elapsed > readOnlyBusyTimeout+slack {
		t.Fatalf("verification budget ended after %v, want %v", got.elapsed, readOnlyBusyTimeout)
	}
}

func TestServingGenerationVerifyBoundedByCallerAndBudget(t *testing.T) {
	root, _, _ := servingTestStores(t)
	db, _ := holdIdentityConnection(t, root)

	// A caller deadline far shorter than the budget ends the wait: the
	// budget never extends it.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	got := awaitVerification(t, startVerification(ctx, root))
	if !errors.Is(got.err, context.DeadlineExceeded) || ctx.Err() == nil {
		t.Fatalf("caller deadline ignored: %v", got.err)
	}
	if got.elapsed >= readOnlyBusyTimeout {
		t.Fatalf("caller deadline extended to %v", got.elapsed)
	}

	// Cancellation while the verification waits ends it as cancelled.
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	waits := db.Stats().WaitCount
	done := startVerification(ctx, root)
	awaitIdentityWaiter(t, db, waits)
	cancel()
	got = awaitVerification(t, done)
	if !errors.Is(got.err, context.Canceled) {
		t.Fatalf("caller cancellation ignored: %v", got.err)
	}
	if got.elapsed >= readOnlyBusyTimeout {
		t.Fatalf("cancellation observed after %v", got.elapsed)
	}
}

func TestServingGenerationVerifyKeepsGenuineFailures(t *testing.T) {
	ctx := context.Background()
	missing := filepath.Join(t.TempDir(), "missing")
	if _, err := VerifyServingGeneration(ctx, missing); err == nil {
		t.Fatal("missing state verified")
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatalf("verification created state: %v", err)
	}

	root, core, sessionStore := servingTestStores(t)
	path := filepath.Join(root, "agentdeck.sqlite3")
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyServingGeneration(ctx, root); err == nil {
		t.Fatal("public database verified")
	}
	if err := os.Chmod(path, 0600); err != nil {
		t.Fatal(err)
	}

	if _, err := core.DB.Exec(`UPDATE schema_metadata SET version=?`, CurrentSchemaVersion+1); err != nil {
		t.Fatal(err)
	}
	var ahead *SchemaAhead
	if _, err := VerifyServingGeneration(ctx, root); !errors.As(err, &ahead) {
		t.Fatalf("newer core verified: %v", err)
	}
	if _, err := core.DB.Exec(`UPDATE schema_metadata SET version=?`, CurrentSchemaVersion); err != nil {
		t.Fatal(err)
	}

	if _, err := sessionStore.DB.Exec(`UPDATE serving_generation SET schema_version=schema_version+1`); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyServingGeneration(ctx, root); err == nil || err.Error() != "invalid serving generation" {
		t.Fatalf("invalid generation verified: %v", err)
	}

	// Move the WAL with the database: it still holds the old pages.
	sessions := filepath.Join(root, "sessions.sqlite3")
	for _, suffix := range []string{"", "-wal", "-shm"} {
		if err := os.Rename(sessions+suffix, sessions+suffix+".old"); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(sessions, []byte("not a database"), 0600); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if _, err := VerifyServingGeneration(ctx, root); err == nil {
		t.Fatal("corrupt session index verified")
	}
	if elapsed := time.Since(start); elapsed > servingIdentityBudget {
		t.Fatalf("corrupt state waited for the budget: %v", elapsed)
	}
}
