package store

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

type cjsiDurableSlot struct {
	identity [16]byte
	digest   [sha256.Size]byte
	size     int64
}

func cjsiServingSlotName(identity [16]byte) string {
	return ".cjsi-serving-slot-" + hex.EncodeToString(identity[:])
}

// The caller admits and pins the physical root, reserves identity for exactly
// one ticket/attempt, and owns the immutable prepared bytes until return. Run
// outside root locks and SQLite snapshots. A result still needs a live attempt
// and an unexpired window before the coordinator may register its seal.
func persistCJSIServingSlot(root string, identity [16]byte, contents []byte) (cjsiDurableSlot, error) {
	return persistCJSIServingSlotWithSync(root, identity, contents, unix.Fsync)
}

func persistCJSIServingSlotWithSync(root string, identity [16]byte, contents []byte, syncFD func(int) error) (cjsiDurableSlot, error) {
	if !filepath.IsAbs(root) || filepath.Clean(root) != root {
		return cjsiDurableSlot{}, errCJSISlotAdmission
	}
	if err := validateCJSISlotInput(identity, contents); err != nil {
		return cjsiDurableSlot{}, err
	}
	dir, err := openCJSISlotRoot(root)
	if err != nil {
		return cjsiDurableSlot{}, err
	}
	defer unix.Close(dir)
	return persistCJSIServingSlotAt(dir, identity, contents, syncFD)
}

func validateCJSISlotInput(identity [16]byte, contents []byte) error {
	if identity == ([16]byte{}) || len(contents) == 0 {
		return errCJSISlotAdmission
	}
	if len(contents) > cjsiMaxSlotBytes {
		return errCJSISlotTooLarge
	}
	return nil
}

// dir is the borrowed, verified directory. The caller owns its lifetime.
func persistCJSIServingSlotAt(dir int, identity [16]byte, contents []byte, syncFD func(int) error) (cjsiDurableSlot, error) {
	name := cjsiServingSlotName(identity)
	// Refuse a known collision before creating a staging file. Rename
	// still enforces exclusive installation if the destination changes.
	var existing unix.Stat_t
	if err := unix.Fstatat(dir, name, &existing, unix.AT_SYMLINK_NOFOLLOW); err == nil {
		return cjsiDurableSlot{}, unix.EEXIST
	} else if err != unix.ENOENT {
		return cjsiDurableSlot{}, err
	}

	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return cjsiDurableSlot{}, err
	}
	temp := ".cjsi-serving-tmp-" + hex.EncodeToString(nonce[:])
	fd, err := unix.Openat(dir, temp, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0600)
	if err != nil {
		return cjsiDurableSlot{}, err
	}
	file := os.NewFile(uintptr(fd), temp)
	// Close the owned descriptor on failure. A name-based unlink cannot
	// guarantee the inode is still ours, so leave unsealed entries for
	// coordinated reclamation.
	defer file.Close()
	var created unix.Stat_t
	if err := unix.Fstat(fd, &created); err != nil {
		return cjsiDurableSlot{}, err
	}
	if !privateCJSISlotFile(&created, 0) {
		return cjsiDurableSlot{}, errCJSISlotAdmission
	}
	digest := sha256.Sum256(contents)
	size := int64(len(contents))
	if _, err := file.Write(contents); err != nil { // os.File.Write reports short writes as errors.
		return cjsiDurableSlot{}, err
	}
	if err := syncFD(fd); err != nil {
		return cjsiDurableSlot{}, err
	}
	if err := file.Close(); err != nil {
		return cjsiDurableSlot{}, err
	}
	var staged unix.Stat_t
	if err := unix.Fstatat(dir, temp, &staged, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return cjsiDurableSlot{}, err
	}
	if !privateCJSISlotFile(&staged, size) || staged.Dev != created.Dev || staged.Ino != created.Ino {
		return cjsiDurableSlot{}, errCJSISlotAdmission
	}
	if err := unix.RenameatxNp(dir, temp, dir, name, unix.RENAME_EXCL); err != nil {
		return cjsiDurableSlot{}, err
	}
	if err := syncFD(dir); err != nil {
		return cjsiDurableSlot{}, err
	}
	if err := verifyCJSIServingSlot(dir, name, &created, size, digest); err != nil {
		return cjsiDurableSlot{}, err
	}
	return cjsiDurableSlot{identity: identity, digest: digest, size: size}, nil
}

func privateCJSISlotFile(st *unix.Stat_t, size int64) bool {
	return st.Mode&unix.S_IFMT == unix.S_IFREG && st.Mode&07777 == 0600 &&
		st.Uid == uint32(os.Getuid()) && st.Nlink == 1 && st.Size == size
}

func verifyCJSIServingSlot(dir int, name string, created *unix.Stat_t, size int64, digest [sha256.Size]byte) error {
	fd, err := unix.Openat(dir, name, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	file := os.NewFile(uintptr(fd), name)
	defer file.Close()
	var st unix.Stat_t
	if err := unix.Fstat(fd, &st); err != nil {
		return err
	}
	if !privateCJSISlotFile(&st, size) || st.Dev != created.Dev || st.Ino != created.Ino {
		return errCJSISlotAdmission
	}
	hash := sha256.New()
	n, err := io.Copy(hash, io.LimitReader(file, size+1))
	if err != nil {
		return err
	}
	if n != size || !bytes.Equal(hash.Sum(nil), digest[:]) {
		return errCJSISlotAdmission
	}
	return nil
}

const cjsiMaxSlotBytes = 8 << 20

var errCJSISlotTooLarge = errors.New("CJSI serving slot exceeds size limit")

func openCJSISlotRoot(path string) (int, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return -1, err
	}
	var st unix.Stat_t
	if err = unix.Fstat(fd, &st); err != nil || st.Mode&07777 != 0700 || st.Uid != uint32(os.Getuid()) {
		unix.Close(fd)
		return -1, errors.Join(errCJSISlotAdmission, err)
	}
	return fd, nil
}
