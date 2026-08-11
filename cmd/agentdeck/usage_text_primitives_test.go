package main

import "testing"

func TestUsageTextPrimitivesPreserveTracksAndSectionTitles(t *testing.T) {
	primitives := usageTextPrimitives{width: 12}
	if got, want := primitives.barTrack(2, 4, ""), "██··"; got != want {
		t.Fatalf("bar track = %q, want %q", got, want)
	}
	if got, want := primitives.sectionTitle("SECTION", 12, ""), "SECTION ────"; got != want {
		t.Fatalf("section title = %q, want %q", got, want)
	}
}
