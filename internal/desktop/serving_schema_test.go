package desktop

import (
	"context"
	"errors"
	"testing"
)

func TestServingDerivedOnlySchemaUpgradeKeepsSavedPublicationReadable(t *testing.T) {
	s, core, _ := newServingFixture(t)
	seedServingUsage(t, s, core)
	ctx := context.Background()
	full, err := s.PublishServingSnapshot(ctx, Request{WireVersion: 1, RecentLimit: DefaultRecentLimit}, false)
	if err != nil {
		t.Fatal(err)
	}
	// Older headers retain their usage and session publication. The quota
	// segment itself must come from the migrated core.
	unsafePlan := "legacy-unsafe@example.test"
	full.Payload.Data.Subscription = SubscriptionSnapshot{
		Available: true, Reading: true, Clients: []SubscriptionClient{{Client: "codex", Plan: &unsafePlan}},
	}
	for _, schema := range []int{31, 32, 33, 34, 35} {
		full.Header.Generation.CoreSchema = schema
		bytes, err := encodeServingSnapshot(full)
		if err != nil {
			t.Fatal(err)
		}
		if err := writeServingSnapshot(s.StateRoot, bytes); err != nil {
			t.Fatal(err)
		}
		old, err := s.ReadServingSnapshot(ctx)
		if err != nil || old.Header.PublicationID != full.Header.PublicationID || old.Header.Freshness != "stale" {
			t.Fatalf("compatible schema %d publication lost or redated: %v", schema, err)
		}
		for _, client := range old.Payload.Data.Subscription.Clients {
			if client.Plan != nil && *client.Plan == unsafePlan {
				t.Fatalf("schema %d reused rejected cached plan", schema)
			}
		}
	}
	for _, schema := range []int{30, 37} {
		full.Header.Generation.CoreSchema = schema
		bytes, err := encodeServingSnapshot(full)
		if err != nil {
			t.Fatal(err)
		}
		if err := writeServingSnapshot(s.StateRoot, bytes); err != nil {
			t.Fatal(err)
		}
		if _, err := s.ReadServingSnapshot(ctx); !errors.Is(err, ErrServingInvalid) {
			t.Fatalf("unknown schema %d accepted: %v", schema, err)
		}
	}
	if compatibleServingCoreSchema(32, 31) || compatibleServingCoreSchema(33, 32) || compatibleServingCoreSchema(34, 33) || compatibleServingCoreSchema(35, 34) || compatibleServingCoreSchema(30, 35) || compatibleServingCoreSchema(30, 36) || compatibleServingCoreSchema(36, 35) {
		t.Fatal("schema exception widened to downgrade or future migration")
	}
}

func TestServingSchema37Compatibility(t *testing.T) {
	for stored := 30; stored <= 38; stored++ {
		if got, want := compatibleServingCoreSchema(stored, 37), stored >= 31 && stored <= 37; got != want {
			t.Errorf("schema %d → 37 compatibility = %t, want %t", stored, got, want)
		}
	}
	if compatibleServingCoreSchema(37, 36) {
		t.Fatal("schema 37 accepted by schema 36")
	}
}
