//go:build windows

package usagehook

import "io/fs"

// Windows ownership validation requires an access-token lookup. The hook
// mutation path is currently Unix-oriented, so portable builds fail closed.
func ownedByCurrentUser(fs.FileInfo) bool { return false }
