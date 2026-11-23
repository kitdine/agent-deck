package quota

import "time"

// CaptureEntry is sampled before reading stdin. It does not resolve the boot
// identity until after the prior command has finished.
type CaptureEntry struct {
	ObservedAt time.Time
	mono       int64
	valid      bool
}

func BeginStatusLineCapture() CaptureEntry {
	mono, valid := captureMonotonic()
	return CaptureEntry{ObservedAt: time.Now(), mono: mono, valid: valid}
}

func (e CaptureEntry) BootStamp() *BootStamp {
	if !e.valid {
		return nil
	}
	boot, ok := captureBootID()
	if !ok {
		return nil
	}
	return &BootStamp{Boot: boot, Mono: e.mono}
}

func refreshBootStamp() (BootStamp, bool) {
	mono, ok := captureMonotonic()
	if stamp := (CaptureEntry{mono: mono, valid: ok}).BootStamp(); stamp != nil {
		return *stamp, true
	}
	return BootStamp{}, false
}
