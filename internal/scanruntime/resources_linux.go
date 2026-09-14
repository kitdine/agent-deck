//go:build linux

package scanruntime

import (
	"syscall"
	"time"
)

func currentProcessResources() processResources {
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil {
		return processResources{}
	}
	cpu := time.Duration(usage.Utime.Sec+usage.Stime.Sec)*time.Second +
		time.Duration(usage.Utime.Usec+usage.Stime.Usec)*time.Microsecond
	return processResources{cpuTime: cpu, peakRSS: usage.Maxrss * 1024}
}
