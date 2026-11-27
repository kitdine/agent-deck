package quota

import (
	"os"
	"strings"

	"golang.org/x/sys/unix"
)

func captureMonotonic() (int64, bool) {
	var ts unix.Timespec
	err := unix.ClockGettime(unix.CLOCK_BOOTTIME, &ts)
	return ts.Nano(), err == nil
}

func captureBootID() (string, bool) {
	b, e := os.ReadFile("/proc/sys/kernel/random/boot_id")
	id := strings.TrimSpace(string(b))
	return id, e == nil && id != ""
}
