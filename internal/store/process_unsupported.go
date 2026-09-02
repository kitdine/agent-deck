//go:build !darwin

package store

import "os"

func lockProcessAlive(int) (alive, known bool) {
	return false, false
}

func tryLockReclaimFile(*os.File) (bool, error) {
	return true, nil
}
