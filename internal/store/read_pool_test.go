package store

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func poolFixture(t *testing.T, path string, value int) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE fixture(value INTEGER); INSERT INTO fixture VALUES (?)`, value); err != nil {
		t.Fatal(err)
	}
}

func TestReadPoolReusesInitializedConnectionsAndKeepsLeasesIndependent(t *testing.T) {
	ctx := context.Background()
	root, writer, _ := servingTestStores(t)
	first, err := OpenReadOnly(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	second, err := OpenReadOnly(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if first.DB != second.DB {
		t.Fatal("request created another database pool")
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	if err := writer.SetSetting(ctx, "pool-live", "changed"); err != nil {
		t.Fatal(err)
	}
	var value string
	if err := second.DB.QueryRow(`SELECT value FROM settings WHERE key='pool-live'`).Scan(&value); err != nil || value != "changed" {
		t.Fatalf("cached values or closed another lease: %q %v", value, err)
	}
	if _, err := second.DB.Exec(`DELETE FROM settings`); err == nil {
		t.Fatal("read pool permitted a write")
	}
	if err := second.Close(); err != nil {
		t.Fatal(err)
	}
	third, err := OpenReadOnly(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	defer third.Close()
	if third.DB != first.DB {
		t.Fatal("last request close destroyed the process pool")
	}
}

func TestServingReaderReusesPrimedPoolsWithIndependentDeadlines(t *testing.T) {
	root, _, _ := servingTestStores(t)
	first, err := OpenServingGenerationReader(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := first.Read(); err != nil {
		t.Fatal(err)
	}
	first.Close()
	started := time.Now()
	second, err := OpenServingGenerationReader(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	if _, err := second.Read(); err != nil {
		t.Fatal(err)
	}
	t.Logf("primed core/session identity read: %s", time.Since(started))
	if first.core != second.core || first.sessions != second.sessions {
		t.Fatal("identity request reopened initialized connections")
	}
	if first.ctx.Err() == nil || second.ctx.Err() != nil {
		t.Fatal("one reader's cancellation affected another lease")
	}
}

func TestReadPoolWriterClosePreservesMainDatabaseCopies(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writer, err := Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.SetSetting(ctx, "checkpoint-pool", "committed"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	main, err := os.ReadFile(filepath.Join(root, "agentdeck.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	copyRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(copyRoot, "agentdeck.sqlite3"), main, 0600); err != nil {
		t.Fatal(err)
	}
	copied, err := OpenReadOnly(ctx, copyRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer copied.Close()
	var value string
	if err := copied.DB.QueryRow(`SELECT value FROM settings WHERE key='checkpoint-pool'`).Scan(&value); err != nil || value != "committed" {
		t.Fatalf("main database omitted a committed WAL write: %q %v", value, err)
	}
}

func TestReadPoolRetiresReplacedFilesWithoutClosingActiveOldReaders(t *testing.T) {
	ctx := context.Background()
	p := &readPoolCache{limit: 2}
	t.Cleanup(func() { _ = p.close() })
	path := filepath.Join(t.TempDir(), "fixture.sqlite3")
	poolFixture(t, path, 1)
	open := func() (*sql.DB, error) { return sql.Open("sqlite", "file:"+path+"?mode=ro") }
	old, releaseOld, err := p.acquire(ctx, path, "test", open)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(path, path+".old"); err != nil {
		t.Fatal(err)
	}
	poolFixture(t, path, 2)
	current, releaseCurrent, err := p.acquire(ctx, path, "test", open)
	if err != nil {
		t.Fatal(err)
	}
	defer releaseCurrent()
	if current == old {
		t.Fatal("reused a connection to the replaced database")
	}
	for db, want := range map[*sql.DB]int{old: 1, current: 2} {
		var value int
		if err := db.QueryRow(`SELECT value FROM fixture`).Scan(&value); err != nil || value != want {
			t.Fatalf("value=%d want=%d err=%v", value, want, err)
		}
	}
	if err := releaseOld(); err != nil {
		t.Fatal(err)
	}
	if err := old.Ping(); err == nil {
		t.Fatal("retired pool remained open after final lease")
	}
	if err := current.Ping(); err != nil {
		t.Fatal(err)
	}
}

func TestReadPoolCoalescesColdOpensAndCancelsOnlyTheWaitingLease(t *testing.T) {
	p := &readPoolCache{limit: 2}
	t.Cleanup(func() { _ = p.close() })
	path := filepath.Join(t.TempDir(), "fixture.sqlite3")
	poolFixture(t, path, 1)
	entered, proceed := make(chan struct{}), make(chan struct{})
	var opens atomic.Int32
	open := func() (*sql.DB, error) {
		if opens.Add(1) == 1 {
			close(entered)
		}
		<-proceed
		return sql.Open("sqlite", "file:"+path+"?mode=ro")
	}
	owner := make(chan error, 1)
	go func() {
		_, release, err := p.acquire(context.Background(), path, "test", open)
		if err == nil {
			err = release()
		}
		owner <- err
	}()
	<-entered
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, _, err := p.acquire(ctx, path, "test", open); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("wait ignored deadline: %v", err)
	}
	close(proceed)
	if err := <-owner; err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	for range 10 {
		group.Add(1)
		go func() {
			defer group.Done()
			db, release, err := p.acquire(context.Background(), path, "test", open)
			if err != nil {
				t.Error(err)
				return
			}
			defer release()
			if err := db.Ping(); err != nil {
				t.Error(err)
			}
		}()
	}
	group.Wait()
	if opens.Load() != 1 {
		t.Fatalf("cold opens=%d", opens.Load())
	}
}

func TestReadPoolCapacityEvictsIdlePoolsAndDefersActiveRetirement(t *testing.T) {
	p := &readPoolCache{limit: 1}
	t.Cleanup(func() { _ = p.close() })
	root := t.TempDir()
	acquire := func(name string) (*sql.DB, func() error) {
		path := filepath.Join(root, name)
		poolFixture(t, path, 1)
		db, release, err := p.acquire(context.Background(), path, "test", func() (*sql.DB, error) { return sql.Open("sqlite", "file:"+path+"?mode=ro") })
		if err != nil {
			t.Fatal(err)
		}
		return db, release
	}
	first, releaseFirst := acquire("first")
	second, releaseSecond := acquire("second")
	if len(p.entries) != 1 {
		t.Fatal("active leases grew the cache")
	}
	if err := releaseSecond(); err != nil {
		t.Fatal(err)
	}
	if err := second.Ping(); err == nil {
		t.Fatal("uncached pool leaked")
	}
	if err := first.Ping(); err != nil {
		t.Fatal(err)
	}
	if err := releaseFirst(); err != nil {
		t.Fatal(err)
	}
	third, releaseThird := acquire("third")
	defer releaseThird()
	if err := first.Ping(); err == nil {
		t.Fatal("idle pool was not evicted")
	}
	if err := p.close(); err != nil {
		t.Fatal(err)
	}
	if err := third.Ping(); err != nil {
		t.Fatal("shutdown closed a live lease")
	}
	if err := releaseThird(); err != nil {
		t.Fatal(err)
	}
	if err := third.Ping(); err == nil {
		t.Fatal("final retired lease leaked")
	}
}

type poolWaitContext struct {
	context.Context
	entered chan struct{}
	once    sync.Once
}

func (c *poolWaitContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.entered) })
	return c.Context.Done()
}

func TestReadPoolInitializerCancellationDoesNotPoisonOtherWaiters(t *testing.T) {
	p := &readPoolCache{limit: 1}
	t.Cleanup(func() { _ = p.close() })
	path := filepath.Join(t.TempDir(), "fixture.sqlite3")
	poolFixture(t, path, 1)
	entered, proceed := make(chan struct{}), make(chan struct{})
	var opens atomic.Int32
	open := func() (*sql.DB, error) {
		if opens.Add(1) == 1 {
			close(entered)
			<-proceed
		}
		return sql.Open("sqlite", "file:"+path+"?mode=ro")
	}
	ownerContext, cancel := context.WithCancel(context.Background())
	defer cancel()
	owner := make(chan error, 1)
	go func() { _, _, err := p.acquire(ownerContext, path, "test", open); owner <- err }()
	<-entered
	waitContext := &poolWaitContext{Context: context.Background(), entered: make(chan struct{})}
	waiter := make(chan error, 1)
	go func() {
		_, release, err := p.acquire(waitContext, path, "test", open)
		if err == nil {
			err = release()
		}
		waiter <- err
	}()
	<-waitContext.entered
	cancel()
	close(proceed)
	if err := <-owner; !errors.Is(err, context.Canceled) {
		t.Fatalf("owner error=%v", err)
	}
	if err := <-waiter; err != nil {
		t.Fatalf("another request's cancellation poisoned waiter: %v", err)
	}
	if opens.Load() != 2 {
		t.Fatalf("initializations=%d", opens.Load())
	}
}
