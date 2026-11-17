//go:build unix

package quota

import (
	"os"
	"syscall"
)

func ownedByRootOrCurrentUser(info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && (stat.Uid == 0 || int(stat.Uid) == os.Geteuid())
}
