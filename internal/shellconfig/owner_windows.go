//go:build windows

package shellconfig

import "io/fs"

// Windows ownership validation requires an access-token lookup. AgentDeck's
// shell integration is currently supported on Unix shells only, so Windows
// builds retain the portable API without enabling a mutation path.
func ownedByCurrentUser(fs.FileInfo) bool {
	return false
}
