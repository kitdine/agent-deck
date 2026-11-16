//go:build unix

package perftrace

import (
	"errors"
	"os"
	"syscall"
)

// lockTrace holds an exclusive advisory lock on the trace for the producer's
// lifetime. The kernel releases it only when the file is closed, which Close
// does after every record, including any tail written after
// resource.lifetime, has been flushed and synced.
func lockTrace(file *os.File) error {
	return syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
}

// Released reports whether no producer holds the trace at path any longer.
// Together with a resource.lifetime record it is the completion barrier: a
// file that is still empty when it reports true was never locked, so callers
// must also require the lifetime record.
func Released(path string) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer file.Close()
	err = syscall.Flock(int(file.Fd()), syscall.LOCK_SH|syscall.LOCK_NB)
	if errors.Is(err, syscall.EWOULDBLOCK) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
}
