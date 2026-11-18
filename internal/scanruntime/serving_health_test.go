package scanruntime

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

// The production round publishes while it holds the state root's scan lock.
// Neither the streamed publication the App installs on Refresh nor the
// persisted file may report that lock as another live scan.
func TestRoundPublicationDoesNotReportItsOwnScanLock(t *testing.T) {
	home := t.TempDir()
	root, _, err := prepareStateRoot(filepath.Join(home, ".agentdeck"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	writeServingSource(t, home, now.UTC().Format("2006/01/02"), now.Add(-time.Minute), 27)
	ctx := context.Background()
	for _, serving := range []bool{true, false} {
		r := newRound(home)
		r.serving = serving
		r.cacheNow = func() time.Time { return now }
		r.executeWith(ctx, root, false, nil)
		if r.result.Usage.State != "completed" || r.result.Session.State != "completed" {
			t.Fatalf("serving=%t usage=%+v session=%+v", serving, r.result.Usage, r.result.Session)
		}
		r.mu.Lock()
		publication := r.publication
		r.mu.Unlock()
		if publication == nil {
			t.Fatalf("serving=%t round published nothing", serving)
		}
		for _, check := range publication.Payload.Data.Health.Checks {
			if check.Name == "scan_lock" && (check.Status != "ok" || check.Code != "") {
				t.Fatalf("serving=%t round publication reports its own scan lock: %+v", serving, check)
			}
		}
	}
}
