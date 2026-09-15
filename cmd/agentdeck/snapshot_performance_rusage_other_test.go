//go:build !darwin && !linux

package main

import "os"

func snapshotPerformancePeakRSS(*os.ProcessState) int64 { return 0 }
