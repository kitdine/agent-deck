package p07measure

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

// The parent observes birth after Start and before Wait can reap the child.
// Wait-return evidence includes nonzero/signal exits; no child is delayed,
// excluded, renamed or assigned synthetic RSS. All observation work is retained.
type childRecord struct {
	PID           int    `json:"pid"`
	Birth         string `json:"birth"`
	ParentPID     int    `json:"parent_pid"`
	ParentBirth   string `json:"parent_birth"`
	Name          string `json:"name"`
	BeforeStartNS int64  `json:"before_start_ns"`
	ObservedNS    int64  `json:"observed_ns"`
	Event         string `json:"event"`
}

var children = struct {
	sync.Mutex
	active map[*exec.Cmd]childRecord
}{active: map[*exec.Cmd]childRecord{}}

func childDirectory() string { return os.Getenv("AGENTDECK_P07_BARRIER_DIR") }

func recordChild(value childRecord) {
	dir := childDirectory()
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		panic("p07: private child ledger directory required")
	}
	data, err := json.Marshal(value)
	if err != nil {
		panic("p07: child record encoding failed")
	}
	name := filepath.Join(dir, fmt.Sprintf("%d-%d-%d.%s.json", value.ParentPID, value.PID, value.BeforeStartNS, value.Event))
	f, err := os.CreateTemp(dir, ".child-pending-")
	if err != nil {
		panic("p07: child ledger unavailable")
	}
	if _, err = f.Write(data); err != nil {
		panic("p07: child ledger write failed")
	}
	if err = f.Close(); err != nil {
		panic("p07: child ledger close failed")
	}
	if err = os.Rename(f.Name(), name); err != nil {
		panic("p07: child ledger publish failed")
	}
}

func CommandStart(_ string, c *exec.Cmd) error {
	if childDirectory() == "" {
		return c.Start()
	}
	before := childClock()
	if err := c.Start(); err != nil {
		return err
	}
	birth, err := childIdentity(c.Process.Pid)
	if err != nil {
		panic("p07: unobserved child birth")
	}
	parentBirth, err := childIdentity(os.Getpid())
	if err != nil {
		panic("p07: unobserved parent birth")
	}
	value := childRecord{PID: c.Process.Pid, Birth: birth, ParentPID: os.Getpid(), ParentBirth: parentBirth, Name: filepath.Base(c.Path), BeforeStartNS: before, ObservedNS: childClock(), Event: "child_start"}
	children.Lock()
	children.active[c] = value
	children.Unlock()
	recordChild(value)
	return nil
}

func CommandWait(_ string, c *exec.Cmd) error {
	err := c.Wait()
	at := int64(0)
	if childDirectory() != "" {
		at = childClock()
	}
	children.Lock()
	value, ok := children.active[c]
	if ok && c.ProcessState != nil {
		delete(children.active, c)
	}
	children.Unlock()
	if ok && c.ProcessState != nil {
		value.Event = "child_exit"
		value.ObservedNS = at
		recordChild(value)
	}
	return err
}

func CommandRun(site string, c *exec.Cmd) error {
	if childDirectory() == "" {
		return c.Run()
	}
	if err := CommandStart(site, c); err != nil {
		return err
	}
	return CommandWait(site, c)
}

// Output and CombinedOutput retain os/exec's buffer/error behavior. The only
// substitution is the observed Start/Wait pair in Run. The bounded stderr saver
// below follows Go's os/exec implementation (Go Authors, BSD-3-Clause).
func CommandOutput(site string, c *exec.Cmd) ([]byte, error) {
	if childDirectory() == "" {
		return c.Output()
	}
	if c.Stdout != nil {
		return nil, errors.New("exec: Stdout already set")
	}
	var stdout bytes.Buffer
	c.Stdout = &stdout
	capture := c.Stderr == nil
	if capture {
		c.Stderr = &childStderr{}
	}
	err := CommandRun(site, c)
	if err != nil && capture {
		if ee, ok := err.(*exec.ExitError); ok {
			ee.Stderr = c.Stderr.(*childStderr).Bytes()
		}
	}
	return stdout.Bytes(), err
}

func CommandCombinedOutput(site string, c *exec.Cmd) ([]byte, error) {
	if childDirectory() == "" {
		return c.CombinedOutput()
	}
	if c.Stdout != nil {
		return nil, errors.New("exec: Stdout already set")
	}
	if c.Stderr != nil {
		return nil, errors.New("exec: Stderr already set")
	}
	var output bytes.Buffer
	c.Stdout = &output
	c.Stderr = &output
	err := CommandRun(site, c)
	return output.Bytes(), err
}

type childStderr struct {
	prefix, suffix []byte
	skipped        int64
}

func (w *childStderr) Write(data []byte) (int, error) {
	n := len(data)
	const limit = 32 << 10
	if len(w.prefix) < limit {
		take := min(limit-len(w.prefix), len(data))
		w.prefix = append(w.prefix, data[:take]...)
		data = data[take:]
	}
	if len(data) > 0 {
		if excess := len(w.suffix) + len(data) - limit; excess > 0 {
			w.skipped += int64(excess)
		}
		if len(data) >= limit {
			w.suffix = append(w.suffix[:0], data[len(data)-limit:]...)
		} else {
			keep := min(len(w.suffix), limit-len(data))
			copy(w.suffix, w.suffix[len(w.suffix)-keep:])
			w.suffix = w.suffix[:keep]
			w.suffix = append(w.suffix, data...)
		}
	}
	return n, nil
}
func (w *childStderr) Bytes() []byte {
	if w.skipped == 0 {
		return append(append([]byte(nil), w.prefix...), w.suffix...)
	}
	return []byte(string(w.prefix) + fmt.Sprintf("\n... omitting %d bytes ...\n", w.skipped) + string(w.suffix))
}
