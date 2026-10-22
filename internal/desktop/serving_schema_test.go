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
	// Schema 31's published format and canonical fact layout are unchanged by
	// migration 32. Exercise the old header against the upgraded state reader.
	full.Header.Generation.CoreSchema = 31
	bytes, err := encodeServingSnapshot(full)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeServingSnapshot(s.StateRoot, bytes); err != nil {
		t.Fatal(err)
	}
	old, err := s.ReadServingSnapshot(ctx)
	if err != nil || old.Header.PublicationID != full.Header.PublicationID || old.Header.Freshness != "stale" {
		t.Fatalf("compatible publication lost or redated: %v", err)
	}
	for _, schema := range []int{30, 33} {
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
	if compatibleServingCoreSchema(32, 31) || compatibleServingCoreSchema(31, 33) {
		t.Fatal("schema exception widened to downgrade or future migration")
	}
}
