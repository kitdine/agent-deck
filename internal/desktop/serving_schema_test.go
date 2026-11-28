package desktop

import (
	"context"
	"errors"
	"testing"

	"github.com/kitdine/agent-deck/internal/store"
)

func TestServingCoreSchemaCompatibility(t *testing.T) {
	for current := 30; current <= 37; current++ {
		for stored := 30; stored <= 37; stored++ {
			want := stored == current && current <= 36 || current >= 32 && current <= 35 && stored >= 31 && stored < current
			if got := compatibleServingCoreSchema(stored, current); got != want {
				t.Errorf("stored %d, current %d: compatible=%v, want %v", stored, current, got, want)
			}
		}
	}
}

func TestServingPricingFactsSchemaRejectsAndRebuildsRetainedPublication(t *testing.T) {
	s, core, _ := newServingFixture(t)
	seedServingUsage(t, s, core)
	ctx := context.Background()
	request := Request{WireVersion: 1, RecentLimit: DefaultRecentLimit}
	full, err := s.PublishServingSnapshot(ctx, request, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, schema := range []int{30, 31, 32, 33, 34, 35, 37} {
		full.Header.Generation.CoreSchema = schema
		data, err := encodeServingSnapshot(full)
		if err != nil {
			t.Fatal(err)
		}
		if err := writeServingSnapshot(s.StateRoot, data); err != nil {
			t.Fatal(err)
		}
		if _, err := s.ReadServingSnapshot(ctx); !errors.Is(err, ErrServingInvalid) {
			t.Fatalf("stored schema %d, live schema %d: %v", schema, store.CurrentSchemaVersion, err)
		}
	}
	bootstrap, err := s.ReadOrBootstrapServingSnapshot(ctx, request)
	if err != nil || bootstrap.Header.Generation.CoreSchema != 36 || bootstrap.Header.PublicationID == full.Header.PublicationID {
		t.Fatalf("existing bootstrap used old publication: %v", err)
	}
	rebuilt, err := s.PublishServingSnapshot(ctx, request, false)
	if err != nil {
		t.Fatal(err)
	}
	read, err := s.ReadServingSnapshot(ctx)
	if err != nil || read.Header.Generation.CoreSchema != 36 || read.Header.PublicationID != rebuilt.Header.PublicationID {
		t.Fatalf("rebuilt current publication was not readable: schema %d, err %v", read.Header.Generation.CoreSchema, err)
	}
	if _, err := core.DB.ExecContext(ctx, `UPDATE schema_metadata SET version=37`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReadServingSnapshot(ctx); !errors.Is(err, store.ErrSchemaAhead) {
		t.Fatalf("live schema ahead: %v", err)
	}
}
