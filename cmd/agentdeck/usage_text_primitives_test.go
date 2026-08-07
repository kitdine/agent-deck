package main

import "testing"

func TestUsageTextPrimitivesPreserveTracksAndResponsiveColumns(t *testing.T) {
	primitives := usageTextPrimitives{width: 12}
	if got, want := primitives.barTrack(2, 4, ""), "██░░"; got != want {
		t.Fatalf("bar track = %q, want %q", got, want)
	}
	if got, want := primitives.sectionTitle("SECTION", 12, ""), "SECTION ────"; got != want {
		t.Fatalf("section title = %q, want %q", got, want)
	}
	if usageResponsiveTableFits(70, 4, 30, 40) {
		t.Fatal("undersized responsive table unexpectedly fit")
	}
	if !usageResponsiveTableFits(74, 4, 30, 40) {
		t.Fatal("minimum responsive table did not fit")
	}
	left, right := usageResponsiveTableWidths(100, 4, 40)
	if left != 56 || right != 40 {
		t.Fatalf("responsive widths = %d,%d, want 56,40", left, right)
	}
}
