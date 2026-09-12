//go:build darwin

package ingest

import "syscall"

func statChangeTime(stat *syscall.Stat_t) int64 {
	return stat.Ctimespec.Sec*1_000_000_000 + int64(stat.Ctimespec.Nsec)
}
