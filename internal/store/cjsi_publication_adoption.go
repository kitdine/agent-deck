package store

import (
	"context"
	"crypto/sha256"
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

// The production assembler independently reconstructed the same reservation,
// parent, staged delta and bytes. A crash may have installed that exact inode
// before directory sync or before seal registration. Only under the existing
// root worker lease, authenticate the entry and establish both syncs again.
// A mismatch never unlinks, overwrites or accepts another worker's artifact.
func persistReservedCJSISlot(ctx context.Context, root *os.File, info os.FileInfo, id [16]byte, data []byte) (CJSISlotReceipt, error) {
	return writeCJSIServingSlot(ctx, root, info, id, data, func(dir int, id [16]byte, contents []byte) (cjsiDurableSlot, error) {
		slot, err := persistCJSIServingSlotAt(dir, id, contents, unix.Fsync)
		if !errors.Is(err, os.ErrExist) {
			return slot, err
		}
		name := cjsiServingSlotName(id)
		size := int64(len(contents))
		digest := sha256.Sum256(contents)
		fd, e := unix.Openat(dir, name, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
		if e != nil {
			return cjsiDurableSlot{}, e
		}
		var st unix.Stat_t
		e = unix.Fstat(fd, &st)
		if e == nil && !privateCJSISlotFile(&st, size) {
			e = errCJSISlotAdmission
		}
		if e == nil {
			e = verifyCJSIServingSlot(dir, name, &st, size, digest)
		}
		if e == nil {
			e = unix.Fsync(fd)
		}
		if e = errors.Join(e, unix.Close(fd)); e != nil {
			return cjsiDurableSlot{}, e
		}
		if e = unix.Fsync(dir); e != nil {
			return cjsiDurableSlot{}, e
		}
		if e = verifyCJSIServingSlot(dir, name, &st, size, digest); e != nil {
			return cjsiDurableSlot{}, e
		}
		return cjsiDurableSlot{identity: id, digest: digest, size: size}, nil
	})
}
