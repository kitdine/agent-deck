package desktop

// Migration 32 adds only derived partitions and invalidation markers, and
// migration 33 only telemetry observations that no canonical fact reads. A
// saved schema-31 or schema-32 publication still describes the same canonical
// facts. Keep these exceptions directional and explicit; unknown schema
// transitions stay invalid.
func compatibleServingCoreSchema(stored, current int) bool {
	if stored == current {
		return true
	}
	switch current {
	case 32:
		return stored == 31
	case 33:
		return stored == 31 || stored == 32
	}
	return false
}
