//go:build !unix

package perftrace

import (
	"errors"
	"os"
)

var errNoTraceLock = errors.New("perftrace: trace completion lock is unsupported on this platform")

func lockTrace(*os.File) error { return errNoTraceLock }

// Released cannot prove completion without the producer lock.
func Released(string) (bool, error) { return false, errNoTraceLock }
