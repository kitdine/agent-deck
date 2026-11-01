//go:build darwin

package perftrace

import (
	"syscall"

	"golang.org/x/sys/unix"
)

// ClockName is the clock shared with the App's PerfTrace and NSEvent timestamps.
const ClockName = "CLOCK_UPTIME_RAW"

// Now returns CLOCK_UPTIME_RAW in nanoseconds.
func Now() int64 {
	var ts unix.Timespec
	if err := unix.ClockGettime(unix.CLOCK_UPTIME_RAW, &ts); err != nil {
		return 0
	}
	return ts.Nano()
}

type resources struct{ maxRSSBytes, userNS, sysNS int64 }

func lifetimeResources() resources {
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil {
		return resources{}
	}
	// darwin reports ru_maxrss in bytes.
	return resources{maxRSSBytes: usage.Maxrss, userNS: usage.Utime.Nano(), sysNS: usage.Stime.Nano()}
}
