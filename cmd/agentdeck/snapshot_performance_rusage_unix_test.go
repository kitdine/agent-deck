//go:build darwin || linux

package main

import (
	"os"
	"runtime"
	"syscall"
)

func snapshotPerformancePeakRSS(state *os.ProcessState) int64 {
	usage, ok := state.SysUsage().(*syscall.Rusage)
	if !ok || usage == nil {
		return 0
	}
	value := int64(usage.Maxrss)
	if runtime.GOOS == "linux" {
		value *= 1024
	}
	return value
}
