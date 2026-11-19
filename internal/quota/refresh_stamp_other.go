//go:build !linux && !darwin

package quota

func captureMonotonic() (int64, bool) { return 0, false }
func captureBootID() (string, bool)   { return "", false }
