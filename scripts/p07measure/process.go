package p07measure

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var processStartOnce, processEndOnce sync.Once

// Process barriers exist only in the private overlay. No SQL or timed operation
// is delayed by them: start precedes main; end follows its final output/cleanup.
// The collector acknowledges each phase only after a complete fixed-tick frame
// contains this PID with its pinned birth identity. A missing observer fails closed.
func processBarrier(phase string) {
	dir := os.Getenv("AGENTDECK_P07_BARRIER_DIR")
	if dir == "" {
		return
	}
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		panic("p07: private process barrier directory required")
	}
	name := filepath.Join(dir, fmt.Sprintf("%d.%s", os.Getpid(), phase))
	f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		panic("p07: exclusive process barrier unavailable")
	}
	if err = f.Close(); err != nil {
		panic("p07: process barrier close failed")
	}
	deadline := time.Now().Add(30 * time.Second)
	for {
		if _, err := os.Stat(name + ".ack"); err == nil {
			return
		} else if !os.IsNotExist(err) || time.Now().After(deadline) {
			panic("p07: final sample acknowledgement missing")
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func ProcessStart() { processStartOnce.Do(func() { processBarrier("start") }) }
func ProcessEnd()   { processEndOnce.Do(func() { processBarrier("end") }) }
func Exit(code int) { ProcessEnd(); os.Exit(code) }
