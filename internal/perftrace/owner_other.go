//go:build !unix

package perftrace

import "os"

func ownedByCurrentUser(os.FileInfo) bool { return false }
