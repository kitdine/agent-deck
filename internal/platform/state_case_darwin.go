//go:build darwin

package platform

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

// Darwin sys/unistd.h: _PC_CASE_SENSITIVE.
const pcCaseSensitive = 11

// DirectoryIgnoresASCIICase reports whether the open directory is known to
// resolve ASCII case variants to the same entry, even if its path has moved.
func DirectoryIgnoresASCIICase(dir *os.File) bool {
	conn, err := dir.SyscallConn()
	if err != nil {
		return false
	}
	var insensitive bool
	err = conn.Control(func(fd uintptr) {
		value, err := unix.Fpathconf(int(fd), pcCaseSensitive)
		insensitive = err == nil && value == 0
	})
	return err == nil && insensitive
}

func filesystemCaseSensitive(directory string) (bool, error) {
	value, err := unix.Pathconf(directory, pcCaseSensitive)
	if err != nil {
		return false, err
	}
	if value != 0 && value != 1 {
		return false, errors.New("state_path_unresolved: unknown filesystem case policy")
	}
	return value == 1, nil
}

func filesystemNormalizationSensitive(directory string) (bool, error) {
	var volume unix.Statfs_t
	if err := unix.Statfs(directory, &volume); err != nil {
		return false, err
	}
	// Both APFS variants use normalization-insensitive name lookup. pathconf's
	// case flag does not answer this separate question. Other local/network
	// filesystems may use different rules; do not guess from their case flag.
	if unix.ByteSliceToString(volume.Fstypename[:]) == "apfs" {
		return false, nil
	}
	return false, errors.New("state_path_unresolved: filesystem normalization policy unavailable")
}
