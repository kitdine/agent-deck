package desktop

import (
	"context"
	"database/sql/driver"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"modernc.org/sqlite"

	"github.com/kitdine/agent-deck/internal/platform"
	"github.com/kitdine/agent-deck/internal/sessionorigin"
)

const identitySpinFunction = "agentdeck_test_identity_spin"

var (
	identitySpinInstall sync.Once
	// identitySpinCore is the core database whose identity reads spin; empty
	// means none does.
	identitySpinCore atomic.Value
	identitySpinHits atomic.Int64
)

// installIdentitySpin registers, once per test process, a driver hook that
// gives every identity-profile connection (mode=ro, busy_timeout 20) to a core
// database a TEMP VIEW shadowing serving_generation. The view is per
// connection and never touches the database file. Its revision column runs a
// recursive query that keeps going while identitySpinCore names that
// connection's database: a serving-identity read that is slow for as long as
// it is allowed to run. SQLite checks for interruption between steps, so the
// read still ends at its context's deadline, exactly as a read slowed by a
// loaded machine does. Ordinary read-only and writable connections are never
// changed, so the selection's own reads stay fast.
func installIdentitySpin(t *testing.T) {
	t.Helper()
	identitySpinInstall.Do(func() {
		identitySpinCore.Store("")
		if err := sqlite.RegisterScalarFunction(identitySpinFunction, 1, func(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			path, _ := args[0].(string)
			if path != "" && path == identitySpinCore.Load().(string) {
				identitySpinHits.Add(1)
				return int64(1), nil
			}
			return int64(0), nil
		}); err != nil {
			t.Fatal(err)
		}
		sqlite.RegisterConnectionHook(func(conn sqlite.ExecQuerierContext, dsn string) error {
			parsed, err := url.Parse(dsn)
			if err != nil || !strings.Contains(parsed.RawQuery, "mode=ro") || !strings.Contains(parsed.RawQuery, "busy_timeout(20)") ||
				filepath.Base(parsed.Path) != "agentdeck.sqlite3" {
				return nil
			}
			literal := "'" + strings.ReplaceAll(parsed.Path, "'", "''") + "'"
			_, err = conn.ExecContext(context.Background(), `CREATE TEMP VIEW serving_generation AS SELECT singleton, schema_version, revision + (
				WITH RECURSIVE spin(step) AS (SELECT 0 UNION ALL SELECT step + 1 FROM spin WHERE `+identitySpinFunction+`(`+literal+`))
				SELECT count(*) * 0 FROM spin) AS revision FROM main.serving_generation`, nil)
			return err
		})
	})
}

// A batch whose first view verifies normally and whose later verifications
// stay slow answers well inside the App helper's 30s deadline: every
// verification of the batch shares one waiting budget. The first view keeps
// its verified answer; every view the budget did not cover is unavailable with
// generation_unavailable, as a single view whose verification fails is.
func TestSessionScopeBatchSharesOneVerificationBudget(t *testing.T) {
	installIdentitySpin(t)
	root := widgetBatchState(t)
	guarded, err := platform.GuardStateRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	core := filepath.Join(guarded, "agentdeck.sqlite3")
	t.Cleanup(func() {
		identitySpinCore.Store("")
		sessionScopeBatchObserver = nil
	})
	identitySpinHits.Store(0)
	sessionScopeBatchObserver = func(index int) {
		if index == 1 {
			identitySpinCore.Store(core)
		}
	}
	s := scopeService(t, root)

	type answer struct {
		batch SessionScopeBatch
		err   error
	}
	done := make(chan answer, 1)
	start := time.Now()
	go func() {
		batch, err := s.SessionScopeBatch(context.Background(), SessionScopeRequest{Scope: "work", RecentLimit: 1, AsOf: fixtureNow})
		done <- answer{batch, err}
	}()
	var got answer
	select {
	case got = <-done:
	case <-time.After(2 * time.Minute):
		t.Fatal("batch did not return") // hang guard only
	}
	elapsed := time.Since(start)
	if got.err != nil || len(got.batch.Views) != 9 {
		t.Fatalf("batch: %d views, %v", len(got.batch.Views), got.err)
	}
	if identitySpinHits.Load() == 0 {
		t.Fatal("no verification was slowed")
	}
	// The helper is killed at 30s. The batch's verifications wait at most one
	// shared 5s budget; the rest is the nine views' own fast reads.
	if limit := 10 * time.Second; elapsed > limit {
		t.Fatalf("batch with slow verifications took %v, want at most %v", elapsed, limit)
	}
	first := got.batch.Views[0]
	if !first.Available || first.GenerationID == nil || first.Counts == nil || first.Reason != nil {
		t.Fatalf("verified first view lost: %+v", first)
	}
	for index, view := range got.batch.Views[1:] {
		if view.Available || view.Counts != nil || view.GenerationID != nil || view.Reason == nil ||
			*view.Reason != sessionorigin.PublicationReasonGenerationUnavailable || view.Selection == nil {
			t.Fatalf("view %d after the budget = %+v", index+1, view)
		}
	}
}
