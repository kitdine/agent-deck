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
	// Schema 31's, 32's and 33's published format and canonical fact layout
	// are unchanged by migrations 32 to 34. Exercise each old header against
	// the upgraded state reader.
	for _, schema := range []int{31, 32, 33} {
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
	}
	for _, schema := range []int{30, 35} {
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
	if compatibleServingCoreSchema(32, 31) || compatibleServingCoreSchema(33, 32) || compatibleServingCoreSchema(34, 33) || compatibleServingCoreSchema(31, 35) || compatibleServingCoreSchema(33, 35) {
		t.Fatal("schema exception widened to downgrade or future migration")
	}
}
