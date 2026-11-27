package desktop

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"testing"

	"github.com/kitdine/agent-deck/internal/store"
)

// persistServingBody writes body as the persisted publication with a valid
// checksum, exactly as an older or sparse publisher could have left it.
func persistServingBody(t *testing.T, root string, body []byte) {
	t.Helper()
	sum := sha256.Sum256(body)
	contents, err := json.Marshal(servingDocument{FormatVersion: servingFormatVersion, Body: body, Checksum: hex.EncodeToString(sum[:])})
	if err != nil {
		t.Fatal(err)
	}
	if err := writeServingSnapshot(root, append(contents, '\n')); err != nil {
		t.Fatal(err)
	}
}

// publishWithHealth publishes a real snapshot, then persists it again with
// health replaced, and returns the publication.
func publishWithHealth(t *testing.T, s Service, health HealthSnapshot) ServingSnapshot {
	t.Helper()
	ctx := context.Background()
	published, err := s.PublishServingSnapshot(ctx, Request{WireVersion: 1, RecentLimit: DefaultRecentLimit}, false)
	if err != nil {
		t.Fatal(err)
	}
	published.Payload.Data.Health = health
	body, err := json.Marshal(published)
	if err != nil {
		t.Fatal(err)
	}
	persistServingBody(t, s.StateRoot, body)
	return published
}

func liveScanLockRow() HealthCheck {
	reason, resource, action := string(store.LockReasonLive), "scan", "retry"
	return HealthCheck{Name: "scan_lock", Status: "warning", Code: reason, Reason: &reason, Resource: &resource, ActionKind: &action}
}

func scanLockRows(health HealthSnapshot) []HealthCheck {
	var rows []HealthCheck
	for _, check := range health.Checks {
		if check.Name == "scan_lock" {
			rows = append(rows, check)
		}
	}
	return rows
}

func TestServingReadClearsStaleStatusWithZeroSummary(t *testing.T) {
	s, core, _ := newServingFixture(t)
	seedServingUsage(t, s, core)
	published := publishWithHealth(t, s, HealthSnapshot{
		Available: true, Status: "degraded", Healthy: false,
		Checks: []HealthCheck{liveScanLockRow(), liveScanLockRow()},
	})
	persisted := servingFileDigest(t, s.StateRoot)
	request := Request{WireVersion: 1, RecentLimit: DefaultRecentLimit}
	for read := 1; read <= 2; read++ {
		got, err := s.ReadOrBootstrapServingSnapshot(context.Background(), request)
		if err != nil {
			t.Fatal(err)
		}
		if got.Header.PublicationID != published.Header.PublicationID {
			t.Fatalf("read %d replaced publication %s with %s", read, published.Header.PublicationID, got.Header.PublicationID)
		}
		h := got.Payload.Data.Health
		if len(h.Checks) != 1 || h.Checks[0].Name != "scan_lock" || h.Checks[0].Status != "ok" {
			t.Fatalf("read %d: checks = %+v, want one scan_lock/ok", read, h.Checks)
		}
		if !h.Available || !h.Healthy || h.Status != "healthy" || h.Problems != 0 || h.Warnings != 0 || h.Errors != 0 {
			t.Errorf("read %d with scan_lock/ok: %s healthy=%v %d/%d/%d, want healthy/true 0/0/0",
				read, h.Status, h.Healthy, h.Problems, h.Warnings, h.Errors)
		}
	}
	if servingFileDigest(t, s.StateRoot) != persisted {
		t.Fatal("reading the publication rewrote the persisted snapshot")
	}
}

// A sparse publication can repeat one lock row. The lock is observed and
// accounted once: one copy's contribution is removed when the lock is gone,
// and the summary keeps the warning no row explains.
func TestServingReadAccountsARepeatedLockRowOnce(t *testing.T) {
	s, core, _ := newServingFixture(t)
	seedServingUsage(t, s, core)
	ctx := context.Background()
	request := Request{WireVersion: 1, RecentLimit: DefaultRecentLimit}
	publishWithHealth(t, s, HealthSnapshot{
		Available: true, Status: "degraded", Healthy: false, Problems: 2, Warnings: 2, Errors: 0,
		Checks: []HealthCheck{liveScanLockRow(), liveScanLockRow()},
	})
	persisted := servingFileDigest(t, s.StateRoot)

	for read := 1; read <= 2; read++ {
		got, err := s.ReadOrBootstrapServingSnapshot(ctx, request)
		if err != nil {
			t.Fatal(err)
		}
		health := got.Payload.Data.Health
		if health.Status != "degraded" || health.Healthy || health.Problems != 1 || health.Warnings != 1 || health.Errors != 0 {
			t.Fatalf("read %d: drained repeated lock row = %s healthy=%v %d/%d/%d, want degraded 1/1/0",
				read, health.Status, health.Healthy, health.Problems, health.Warnings, health.Errors)
		}
		if rows := scanLockRows(health); len(rows) != 1 || rows[0].Status != "ok" {
			t.Fatalf("read %d: scan_lock rows = %+v, want one ok row", read, rows)
		}
	}

	// A still-running owner keeps the summary as published: the copy that
	// was counted is replaced by the same live observation, counted once.
	foreignScanLock(t, s.StateRoot)
	for read := 1; read <= 2; read++ {
		got, err := s.ReadOrBootstrapServingSnapshot(ctx, request)
		if err != nil {
			t.Fatal(err)
		}
		health := got.Payload.Data.Health
		if health.Status != "degraded" || health.Problems != 2 || health.Warnings != 2 || health.Errors != 0 {
			t.Fatalf("read %d: live repeated lock row = %s %d/%d/%d, want degraded 2/2/0",
				read, health.Status, health.Problems, health.Warnings, health.Errors)
		}
		if rows := scanLockRows(health); len(rows) != 1 || rows[0].Status != "warning" || rows[0].Code != string(store.LockReasonLive) {
			t.Fatalf("read %d: scan_lock rows = %+v, want one lock_live row", read, rows)
		}
	}
	if servingFileDigest(t, s.StateRoot) != persisted {
		t.Fatal("reading a publication rewrote the persisted file")
	}
}

// The native reader decodes health.checks as a required array. A persisted
// publication whose checks are null or absent is served as an empty array,
// with its summary and its persisted bytes unchanged.
func TestServingReadEmitsAbsentChecksAsAnEmptyArray(t *testing.T) {
	for _, variant := range []string{"null", "absent"} {
		t.Run(variant, func(t *testing.T) {
			s, core, _ := newServingFixture(t)
			seedServingUsage(t, s, core)
			ctx := context.Background()
			published := publishWithHealth(t, s, HealthSnapshot{
				Available: true, Status: "degraded", Healthy: false, Problems: 2, Warnings: 2, Errors: 0, Checks: nil,
			})
			if variant == "absent" {
				var body map[string]any
				raw, err := json.Marshal(published)
				if err != nil {
					t.Fatal(err)
				}
				decoder := json.NewDecoder(bytes.NewReader(raw))
				decoder.UseNumber()
				if err := decoder.Decode(&body); err != nil {
					t.Fatal(err)
				}
				health := body["payload"].(map[string]any)["data"].(map[string]any)["health"].(map[string]any)
				delete(health, "checks")
				if raw, err = json.Marshal(body); err != nil {
					t.Fatal(err)
				}
				persistServingBody(t, s.StateRoot, raw)
			}
			persisted := servingFileDigest(t, s.StateRoot)

			got, err := s.ReadOrBootstrapServingSnapshot(ctx, Request{WireVersion: 1, RecentLimit: DefaultRecentLimit})
			if err != nil {
				t.Fatal(err)
			}
			if got.Header.PublicationID != published.Header.PublicationID {
				t.Fatalf("read replaced the publication: %s, want %s", got.Header.PublicationID, published.Header.PublicationID)
			}
			out, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			var wire struct {
				Payload struct {
					Data struct {
						Health map[string]json.RawMessage `json:"health"`
					} `json:"data"`
				} `json:"payload"`
			}
			if err := json.Unmarshal(out, &wire); err != nil {
				t.Fatal(err)
			}
			if checks := string(wire.Payload.Data.Health["checks"]); checks != "[]" {
				t.Fatalf("served health.checks = %s, want []", checks)
			}
			health := got.Payload.Data.Health
			if !health.Available || health.Status != "degraded" || health.Healthy || health.Problems != 2 || health.Warnings != 2 || health.Errors != 0 {
				t.Fatalf("summary changed: %+v", health)
			}
			if servingFileDigest(t, s.StateRoot) != persisted {
				t.Fatal("reading a publication rewrote the persisted file")
			}
		})
	}
}

func blockedTasksWarningRow() HealthCheck {
	return HealthCheck{Name: "blocked_tasks", Status: "warning", Code: "blocked_tasks_present"}
}

// A visible non-lock warning is a floor of the summary. When an older
// publication's summary is smaller than its rows suggest, removing stale lock
// rows deducts only what remains after that floor, never the visible warning.
func TestServingReadKeepsVisibleNonLockProblemsWhenDrainingRepeatedLockRows(t *testing.T) {
	s, core, _ := newServingFixture(t)
	seedServingUsage(t, s, core)
	ctx := context.Background()
	publishWithHealth(t, s, HealthSnapshot{
		Available: true, Status: "degraded", Healthy: false, Problems: 1, Warnings: 1, Errors: 0,
		Checks: []HealthCheck{blockedTasksWarningRow(), liveScanLockRow(), liveScanLockRow()},
	})
	persisted := servingFileDigest(t, s.StateRoot)

	for read := 1; read <= 2; read++ {
		got, err := s.ReadOrBootstrapServingSnapshot(ctx, Request{WireVersion: 1, RecentLimit: DefaultRecentLimit})
		if err != nil {
			t.Fatal(err)
		}
		health := got.Payload.Data.Health
		if health.Status != "degraded" || health.Healthy || health.Problems != 1 || health.Warnings != 1 || health.Errors != 0 {
			t.Fatalf("read %d: summary = %s healthy=%v %d/%d/%d, want degraded 1/1/0 for the visible blocked_tasks warning",
				read, health.Status, health.Healthy, health.Problems, health.Warnings, health.Errors)
		}
		if len(health.Checks) != 2 || health.Checks[0].Name != "blocked_tasks" || health.Checks[0].Status != "warning" {
			t.Fatalf("read %d: checks = %+v, want blocked_tasks warning kept", read, health.Checks)
		}
		if rows := scanLockRows(health); len(rows) != 1 || rows[0].Status != "ok" {
			t.Fatalf("read %d: scan_lock rows = %+v, want one ok row", read, rows)
		}
	}
	if servingFileDigest(t, s.StateRoot) != persisted {
		t.Fatal("reading a publication rewrote the persisted file")
	}
}

// Accepted summary counts come from persisted data. Near the int limit the
// adjusted counts saturate instead of wrapping negative and reading healthy.
func TestServingReadSaturatesSummaryCountsNearTheIntLimit(t *testing.T) {
	s, core, _ := newServingFixture(t)
	seedServingUsage(t, s, core)
	ctx := context.Background()
	okScanLock := HealthCheck{Name: "scan_lock", Status: "ok"}
	publishWithHealth(t, s, HealthSnapshot{
		Available: true, Status: "unhealthy", Healthy: false, Problems: math.MaxInt, Warnings: math.MaxInt, Errors: math.MaxInt,
		Checks: []HealthCheck{okScanLock},
	})
	foreignScanLock(t, s.StateRoot)

	got, err := s.ReadOrBootstrapServingSnapshot(ctx, Request{WireVersion: 1, RecentLimit: DefaultRecentLimit})
	if err != nil {
		t.Fatal(err)
	}
	health := got.Payload.Data.Health
	if health.Status != "unhealthy" || health.Healthy || health.Problems != math.MaxInt || health.Warnings != math.MaxInt || health.Errors != math.MaxInt {
		t.Fatalf("summary = %s healthy=%v %d/%d/%d, want unhealthy and saturated at MaxInt",
			health.Status, health.Healthy, health.Problems, health.Warnings, health.Errors)
	}
	if rows := scanLockRows(health); len(rows) != 1 || rows[0].Code != string(store.LockReasonLive) {
		t.Fatalf("scan_lock rows = %+v, want one lock_live row", rows)
	}
}

// A negative count in a persisted summary is not a credit against real rows.
func TestServingReadDoesNotLetNegativeSummaryCountsHideLockProblems(t *testing.T) {
	s, core, _ := newServingFixture(t)
	seedServingUsage(t, s, core)
	ctx := context.Background()
	publishWithHealth(t, s, HealthSnapshot{
		Available: true, Status: "healthy", Healthy: true, Problems: -5, Warnings: -5, Errors: 0,
		Checks: []HealthCheck{{Name: "scan_lock", Status: "ok"}},
	})
	foreignScanLock(t, s.StateRoot)

	got, err := s.ReadOrBootstrapServingSnapshot(ctx, Request{WireVersion: 1, RecentLimit: DefaultRecentLimit})
	if err != nil {
		t.Fatal(err)
	}
	health := got.Payload.Data.Health
	if health.Status != "degraded" || health.Healthy || health.Problems != 1 || health.Warnings != 1 || health.Errors != 0 {
		t.Fatalf("summary = %s healthy=%v %d/%d/%d, want degraded 1/1/0",
			health.Status, health.Healthy, health.Problems, health.Warnings, health.Errors)
	}
}
