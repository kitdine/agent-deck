package sessionorigin

import (
	"context"
	"database/sql/driver"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"modernc.org/sqlite"

	"github.com/kitdine/agent-deck/internal/store"
)

// identityQueryLatency is how long one armed serving-identity query takes. It
// stands for a slow process: the startup identity budget is 200ms.
const identityQueryLatency = 300 * time.Millisecond

const identityLatencyFunction = "agentdeck_test_identity_latency"

var (
	identityLatencyInstall sync.Once
	identityLatencyMu      sync.Mutex
	identityLatencyCores   = map[string]bool{}
	identityLatencyArmed   atomic.Bool
	identityLatencyHits    atomic.Int32
	// identityLatencyEntered, when set, runs inside the delayed identity
	// query's SQL callback before it sleeps: the query has started.
	identityLatencyEntered atomic.Pointer[func()]
)

// installIdentityLatency registers, once per test process, a driver hook that
// gives every identity-profile connection (mode=ro, busy_timeout 20) to a
// registered core database a TEMP VIEW shadowing serving_generation. The view
// is per connection and never touches the database file; it calls a function
// that sleeps only while armed and disarms itself after one sleep. Ordinary
// read-only and writable connections are never changed.
func installIdentityLatency(t *testing.T) {
	t.Helper()
	identityLatencyInstall.Do(func() {
		if err := sqlite.RegisterScalarFunction(identityLatencyFunction, 0, func(*sqlite.FunctionContext, []driver.Value) (driver.Value, error) {
			if identityLatencyArmed.CompareAndSwap(true, false) {
				identityLatencyHits.Add(1)
				if entered := identityLatencyEntered.Load(); entered != nil {
					(*entered)()
				}
				time.Sleep(identityQueryLatency)
			}
			return int64(0), nil
		}); err != nil {
			t.Fatal(err)
		}
		sqlite.RegisterConnectionHook(func(conn sqlite.ExecQuerierContext, dsn string) error {
			parsed, err := url.Parse(dsn)
			if err != nil || !strings.Contains(parsed.RawQuery, "mode=ro") || !strings.Contains(parsed.RawQuery, "busy_timeout(20)") {
				return nil
			}
			identityLatencyMu.Lock()
			registered := identityLatencyCores[parsed.Path]
			identityLatencyMu.Unlock()
			if !registered {
				return nil
			}
			_, err = conn.ExecContext(context.Background(), `CREATE TEMP VIEW serving_generation AS SELECT singleton, schema_version, revision + `+identityLatencyFunction+`() AS revision FROM main.serving_generation`, nil)
			return err
		})
	})
}

// latencyStores opens isolated stores whose core identity connections carry
// the latency view. The core path is registered before the stores open
// because opening already prepares the identity connection.
func latencyStores(t *testing.T) stores {
	t.Helper()
	installIdentityLatency(t)
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(parent, "state")
	core := filepath.Join(root, "agentdeck.sqlite3")
	identityLatencyMu.Lock()
	identityLatencyCores[core] = true
	identityLatencyMu.Unlock()
	t.Cleanup(func() {
		identityLatencyArmed.Store(false)
		identityLatencyEntered.Store(nil)
		identityLatencyMu.Lock()
		delete(identityLatencyCores, core)
		identityLatencyMu.Unlock()
	})
	coreStore, err := store.Open(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { coreStore.Close() })
	sessions, err := store.OpenSessions(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sessions.Close() })
	if _, err := os.Stat(core); err != nil {
		t.Fatal(err)
	}
	s := stores{root: root, core: coreStore, sessions: sessions}
	s.indexCopy(t, "claude", "c-1", "/c/c-1.jsonl", "c-1", "p1", "/w/alpha", "2026-10-29T08:00:00Z", "2026-10-29T09:00:00Z", 1)
	s.indexCopy(t, "codex", "x-1", "/x/x-1.jsonl", "x-1", "p2", "/w/beta", "2026-10-29T06:00:00Z", "2026-10-29T06:45:00Z", 1)
	return s
}

// armIdentityLatencyAtVerify delays the first identity query of each
// verification, and only that: the selection's own reads have finished.
func armIdentityLatencyAtVerify(t *testing.T, also func()) *int {
	t.Helper()
	t.Cleanup(func() { beforeSelectionVerify = func(int) {} })
	verifications := 0
	identityLatencyHits.Store(0)
	beforeSelectionVerify = func(int) {
		verifications++
		identityLatencyArmed.Store(true)
		if also != nil {
			also()
		}
	}
	return &verifications
}

// A healthy state whose serving-identity read is slower than the startup
// identity budget still verifies through Select's default (Verify == nil)
// path, and answers exactly what an undelayed selection answers.
func TestSelectDefaultVerifyOutlastsIdentityBudgetOnHealthyState(t *testing.T) {
	s := latencyStores(t)
	verifications := armIdentityLatencyAtVerify(t, nil)
	start := time.Now()
	slow, err := s.reader().Select(context.Background(), Selection{Scope: ScopeAll}, nil)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("healthy state reported unavailable after %v: %v", elapsed, err)
	}
	if *verifications != 1 || identityLatencyHits.Load() != 1 {
		t.Fatalf("latency not applied inside the default verification: verifications=%d delays=%d", *verifications, identityLatencyHits.Load())
	}
	if elapsed < identityQueryLatency {
		t.Fatalf("verification did not wait for the delayed identity read: %v", elapsed)
	}

	beforeSelectionVerify = func(int) {}
	fast, err := s.reader().Select(context.Background(), Selection{Scope: ScopeAll}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if identityLatencyHits.Load() != 1 {
		t.Fatal("undelayed selection was delayed")
	}
	if !reflect.DeepEqual(slow, fast) {
		t.Fatalf("delayed selection differs:\n%+v\n%+v", slow, fast)
	}
	if got := keysOf(slow.Sessions); !reflect.DeepEqual(got, []string{"claude/c-1", "codex/x-1"}) {
		t.Fatalf("selected sessions = %v", got)
	}
}

// Caller cancellation during the delayed verification still ends Select as
// generation unavailable; the verification budget does not outlast it.
func TestSelectDefaultVerifyHonorsCallerCancellation(t *testing.T) {
	s := latencyStores(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Cancel only once the delayed identity query is running, however late
	// the scheduler gets the verification there.
	cancelInside := func() { cancel() }
	identityLatencyEntered.Store(&cancelInside)
	armIdentityLatencyAtVerify(t, nil)
	start := time.Now()
	_, err := s.reader().Select(ctx, Selection{Scope: ScopeAll}, nil)
	if !errors.Is(err, ErrGenerationUnavailable) || !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatalf("caller cancellation during verification = %v (ctx %v)", err, ctx.Err())
	}
	if identityLatencyHits.Load() != 1 {
		t.Fatalf("cancellation not exercised inside the default verification: delays=%d", identityLatencyHits.Load())
	}
	// The SQL function itself cannot observe cancellation; nothing after it
	// may wait for the verification budget.
	if elapsed := time.Since(start); elapsed > identityQueryLatency+time.Second {
		t.Fatalf("cancellation outlasted by verification: %v", elapsed)
	}
}
