//go:build !unix

package quota

import "os"

func ownedByRootOrCurrentUser(os.FileInfo) bool { return false }
