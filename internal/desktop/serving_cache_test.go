package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/doctor"
	"github.com/kitdine/agent-deck/internal/store"
	"github.com/kitdine/agent-deck/internal/usage"
)

func newServingFixture(t *testing.T) (Service, *store.Store, *store.Store) {
	t.Helper()
	ctx := context.Background()
	home := t.TempDir()
	root := filepath.Join(home, ".agentdeck")
	core, err := store.Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { core.Close() })
	sessions, err := store.OpenSessions(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sessions.Close() })
	now := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	return Service{StateRoot: root, Home: home, Now: func() time.Time { return now }, Location: time.UTC,
		RegistrationProbe: func(context.Context, bool) doctor.RegistrationDetails { return doctor.RegistrationDetails{} }}, core, sessions
}

func seedServingUsage(t *testing.T, s Service, core *store.Store) {
	t.Helper()
	path := filepath.Join(s.Home, ".codex", "sessions", "fixture.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	lines := []string{
		`{"type":"session_meta","payload":{"id":"serving-fixture"}}`,
		`{"type":"turn_context","payload":{"turn_id":"serving-turn","model":"gpt-5"}}`,
		`{"timestamp":"2026-10-12T10:00:00Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":21,"output_tokens":7,"total_tokens":28}}}}`,
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	service := usage.New(core, s.Home)
	if _, err := service.Scan(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestServingSnapshotReadOnlyBootstrapAndTodayOracle(t *testing.T) {
	s, core, _ := newServingFixture(t)
	seedServingUsage(t, s, core)
	ctx := context.Background()
	generation, err := store.ReadServingGeneration(ctx, s.StateRoot)
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := s.ReadOrBootstrapServingSnapshot(ctx, Request{WireVersion: 1, RecentLimit: DefaultRecentLimit})
	if err != nil {
		t.Fatal(err)
	}
	if !prepared.Payload.Data.Usage.Available || prepared.Payload.Data.Sessions.Available || prepared.Header.SessionPrepared || !prepared.Payload.Partial {
		t.Fatalf("bootstrap availability is not truthful: %+v", prepared.Header)
	}
	if _, err = os.Stat(filepath.Join(s.StateRoot, servingFilename)); !os.IsNotExist(err) {
		t.Fatalf("readonly bootstrap wrote a cache: %v", err)
	}
	after, err := store.ReadServingGeneration(ctx, s.StateRoot)
	if err != nil || generation != after {
		t.Fatalf("readonly bootstrap wrote state: %v", err)
	}
	full, err := usage.New(core, s.Home).Presentation(ctx, s.now(), s.location())
	if err != nil {
		t.Fatal(err)
	}
	today := prepared.Payload.Data.Usage.Presentation
	for index, scope := range today.Scopes {
		if len(scope.Periods.Items) > 0 && !reflect.DeepEqual(scope.Periods.Items[0], full.Scopes[index].Periods.Items[0]) {
			t.Fatalf("today differs from full oracle for %s", scope.Client)
		}
		if len(scope.Periods.Items) > 1 || scope.Rhythm.Available {
			t.Fatalf("bootstrap fabricated historical windows for %s", scope.Client)
		}
	}
}

func TestServingSnapshotRetainsRevisionStaleDataAndRejectsEpoch(t *testing.T) {
	s, core, sessions := newServingFixture(t)
	seedServingUsage(t, s, core)
	ctx := context.Background()
	before, err := s.PublishServingSnapshot(ctx, Request{WireVersion: 1, RecentLimit: DefaultRecentLimit}, true)
	if err != nil {
		t.Fatal(err)
	}
	if err = core.SetSetting(ctx, "changed", "yes"); err != nil {
		t.Fatal(err)
	}
	stale, err := s.ReadServingSnapshot(ctx)
	if err != nil || stale.Header.PublicationID != before.Header.PublicationID || stale.Header.GeneratedAt != before.Header.GeneratedAt || stale.Header.Freshness != "stale" {
		t.Fatalf("dirty revision discarded or redated valid data: %+v %v", stale.Header, err)
	}
	if _, err = store.MintSessionIndexEpoch(ctx, sessions.DB); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ReadServingSnapshot(ctx); !errors.Is(err, ErrServingInvalid) {
		t.Fatalf("foreign session epoch accepted: %v", err)
	}
}

func TestServingSnapshotDoesNotReplaceFullPublicationWithPartial(t *testing.T) {
	s, core, _ := newServingFixture(t)
	seedServingUsage(t, s, core)
	ctx := context.Background()
	full, err := s.PublishServingSnapshot(ctx, Request{WireVersion: 1, RecentLimit: DefaultRecentLimit}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !full.Header.SessionPrepared {
		t.Fatal("fixture did not prepare sessions")
	}
	partial, err := s.PublishServingSnapshot(ctx, Request{WireVersion: 1, RecentLimit: DefaultRecentLimit}, true)
	if err != nil || partial.Header.SessionPrepared {
		t.Fatalf("partial publication failed: %v", err)
	}
	restored, err := s.ReadServingSnapshot(ctx)
	if err != nil || restored.Header.PublicationID != full.Header.PublicationID {
		t.Fatalf("partial erased last full publication: %v", err)
	}
}

func TestServingSnapshotPrivacyCorruptionPermissionAndLimit(t *testing.T) {
	s, core, _ := newServingFixture(t)
	seedServingUsage(t, s, core)
	ctx := context.Background()
	prepared, err := s.PublishServingSnapshot(ctx, Request{WireVersion: 1, RecentLimit: DefaultRecentLimit}, true)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(s.StateRoot, servingFilename)
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var document servingDocument
	if err = json.Unmarshal(original, &document); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(document.Body), "serving-turn") || strings.Contains(string(document.Body), ".jsonl") {
		t.Fatal("source context leaked into serving payload")
	}
	if err = os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ReadServingSnapshot(ctx); !errors.Is(err, ErrServingInvalid) {
		t.Fatalf("public cache accepted: %v", err)
	}
	if err = os.Chmod(path, 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, []byte(`{"format_version":1,"body":"e30=","checksum":"bad"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ReadServingSnapshot(ctx); !errors.Is(err, ErrServingInvalid) {
		t.Fatalf("corrupt cache accepted: %v", err)
	}
	if err = os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	prepared.Payload.Warnings = []string{strings.Repeat("x", servingMaxBytes)}
	if _, err = encodeServingSnapshot(prepared); !errors.Is(err, ErrServingTooLarge) {
		t.Fatalf("oversize payload accepted: %v", err)
	}
	if err = writeServingSnapshot(s.StateRoot, make([]byte, servingMaxBytes+1)); !errors.Is(err, ErrServingTooLarge) {
		t.Fatalf("oversize write accepted: %v", err)
	}
	remaining, err := os.ReadFile(path)
	if err != nil || string(remaining) != string(original) {
		t.Fatal("failed publication removed last valid file")
	}
}

func TestServingSnapshotDateTimezoneAndMissingState(t *testing.T) {
	s, _, _ := newServingFixture(t)
	ctx := context.Background()
	if _, err := s.PublishServingSnapshot(ctx, Request{WireVersion: 1, RecentLimit: DefaultRecentLimit}, true); err != nil {
		t.Fatal(err)
	}
	next := s
	next.Now = func() time.Time { return s.now().AddDate(0, 0, 1) }
	if _, err := next.ReadServingSnapshot(ctx); !errors.Is(err, ErrServingInvalid) {
		t.Fatalf("yesterday relabelled today: %v", err)
	}
	zone, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	next = s
	next.Location = zone
	if _, err = next.ReadServingSnapshot(ctx); !errors.Is(err, ErrServingInvalid) {
		t.Fatalf("wrong timezone accepted: %v", err)
	}
	next.StateRoot = filepath.Join(t.TempDir(), "absent")
	if _, err = next.ReadOrBootstrapServingSnapshot(ctx, Request{WireVersion: 1, RecentLimit: DefaultRecentLimit}); err == nil {
		t.Fatal("missing identity accepted")
	}
	if _, err = os.Stat(next.StateRoot); !os.IsNotExist(err) {
		t.Fatal("readonly path created state")
	}
}

func TestServingSnapshotRechecksIdentityAfterRead(t *testing.T) {
	s, core, _ := newServingFixture(t)
	ctx := context.Background()
	if _, err := s.PublishServingSnapshot(ctx, Request{WireVersion: 1, RecentLimit: DefaultRecentLimit}, true); err != nil {
		t.Fatal(err)
	}
	reader := s
	reader.Now = func() time.Time {
		if err := core.MintDerivedSnapshotEpoch(ctx); err != nil {
			t.Fatal(err)
		}
		return s.now()
	}
	if _, err := reader.ReadServingSnapshot(ctx); !errors.Is(err, ErrServingInvalid) {
		t.Fatalf("identity changed during file decode was accepted: %v", err)
	}
}
