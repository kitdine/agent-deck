//go:build !darwin

package platform

import "errors"

func filesystemCaseSensitive(string) (bool, error) {
	// Existing entrances use portable OS object identities. An ambiguous alias
	// of a missing entrance needs a filesystem capability we cannot assume.
	return false, errors.New("state_path_unresolved: filesystem case policy unavailable")
}

func filesystemNormalizationSensitive(string) (bool, error) {
	return false, errors.New("state_path_unresolved: filesystem normalization policy unavailable")
}
