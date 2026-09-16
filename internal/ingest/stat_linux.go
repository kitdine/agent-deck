//go:build linux

package ingest

import "syscall"

func statChangeTime(stat *syscall.Stat_t) int64 {
	return stat.Ctim.Sec*1_000_000_000 + int64(stat.Ctim.Nsec)
}
