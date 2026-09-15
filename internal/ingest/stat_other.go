//go:build !darwin && !linux

package ingest

import "syscall"

func statChangeTime(*syscall.Stat_t) int64 { return 0 }
