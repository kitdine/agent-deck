package p07measure

import (
	"fmt"
	"golang.org/x/sys/unix"
)

func childIdentity(pid int) (string, error) {
	info, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
	if err != nil {
		return "", err
	}
	if int(info.Proc.P_pid) != pid {
		return "", fmt.Errorf("child identity unavailable")
	}
	return fmt.Sprintf("%d:%d", info.Proc.P_starttime.Sec, info.Proc.P_starttime.Usec), nil
}

func childClock() int64 {
	var value unix.Timespec
	if err := unix.ClockGettime(unix.CLOCK_UPTIME_RAW, &value); err != nil {
		panic("p07: child clock unavailable")
	}
	return value.Nano()
}
