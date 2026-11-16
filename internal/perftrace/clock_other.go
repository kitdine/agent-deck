//go:build !darwin

package perftrace

import "time"

// ClockName marks traces from platforms outside the measurement contract.
const ClockName = "process_monotonic"

var processStart = time.Now()

// Now returns a process-local monotonic time; only darwin traces are comparable
// across processes.
func Now() int64 { return int64(time.Since(processStart)) }

type resources struct{ maxRSSBytes, userNS, sysNS int64 }

func lifetimeResources() resources { return resources{} }
