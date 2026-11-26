package desktop

// Migration 32 adds only derived partitions and invalidation markers,
// migration 33 only telemetry observations that no canonical fact reads, and
// migration 34 only session-origin preferences, overrides and derivations that
// no existing publication field reads. Migration 35 adds a source thread
// identity column and rebuilds the derived classification table. A saved
// schema-31 to -34 publication still describes the same canonical facts. The
// fenced layout in 37 has its own serving identity. The surrounding identity
// checks still need to match before retaining any publication.
func compatibleServingCoreSchema(stored, current int) bool {
	if stored == current {
		return true
	}
	switch current {
	case 32:
		return stored == 31
	case 33:
		return stored == 31 || stored == 32
	case 34:
		return stored == 31 || stored == 32 || stored == 33
	case 35:
		return stored >= 31 && stored <= 34
	case 36:
		return stored >= 31 && stored <= 35
	case 37:
		return stored >= 31 && stored <= 36
	}
	return false
}
