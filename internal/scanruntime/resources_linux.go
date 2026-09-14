//go:build linux

package scanruntime

import (
	"os/exec"
	"syscall"
	"time"
)

func detachWorkerProcess(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true} }

func currentProcessResources() processResources {
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil {
		return processResources{}
	}
	cpu := time.Duration(usage.Utime.Sec+usage.Stime.Sec)*time.Second +
		time.Duration(usage.Utime.Usec+usage.Stime.Usec)*time.Microsecond
	return processResources{cpuTime: cpu, peakRSS: usage.Maxrss * 1024}
}
