package quota

import "golang.org/x/sys/unix"

func captureMonotonic() (int64, bool) {
	var ts unix.Timespec
	err := unix.ClockGettime(unix.CLOCK_MONOTONIC, &ts)
	return ts.Nano(), err == nil
}

func captureBootID() (string, bool) {
	b, e := unix.Sysctl("kern.bootsessionuuid")
	return b, e == nil && b != ""
}
