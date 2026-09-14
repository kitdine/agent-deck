//go:build !darwin

package quota

// DefaultNotifier has no delivery off macOS. EvaluateAlerts treats a nil
// Notifier as nothing to deliver and evaluates nothing.
func DefaultNotifier() Notifier { return nil }
