package desktop

// Migration 32 adds only derived partitions and invalidation markers. A saved
// schema-31 publication still describes the same canonical facts. Keep this
// exception directional and explicit; unknown schema transitions stay invalid.
func compatibleServingCoreSchema(stored, current int) bool {
	return stored == current || (stored == 31 && current == 32)
}
