package quota

import (
	"context"
	"encoding/binary"
	"errors"
	"os"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

const (
	deliveryName      = "quota-delivery.lock"
	deliveryPending   = ".quota-delivery.lock.pending"
	deliveryAttr      = "user.agentdeck.quota-delivery-lock"
	deliveryOpenFlags = unix.O_RDONLY | unix.O_NOFOLLOW | unix.O_NONBLOCK | unix.O_CLOEXEC
)

type deliveryInode [16]byte

func deliveryID(st *unix.Stat_t) (id deliveryInode) {
	binary.BigEndian.PutUint64(id[:8], uint64(st.Dev))
	binary.BigEndian.PutUint64(id[8:], uint64(st.Ino))
	return
}

func deliveryBinding(dir int) (id deliveryInode, present bool, err error) {
	n, err := unix.Fgetxattr(dir, deliveryAttr, id[:])
	// Darwin returns ENOATTR and Linux returns ENODATA. Other xattr errors
	// (including unsupported filesystems and oversized values) fail closed.
	var errno syscall.Errno
	if errors.Is(err, unix.ENODATA) || (errors.As(err, &errno) && unix.ErrnoName(errno) == "ENOATTR") {
		return id, false, nil
	}
	if err != nil || n != len(id) {
		return id, false, ErrQuotaDeliveryUnavailable
	}
	return id, true, nil
}

func deliveryEntry(dir int, name string) (st unix.Stat_t, present bool, err error) {
	err = unix.Fstatat(dir, name, &st, unix.AT_SYMLINK_NOFOLLOW)
	if errors.Is(err, unix.ENOENT) {
		return st, false, nil
	}
	if err != nil {
		return st, false, ErrQuotaDeliveryUnavailable
	}
	return st, true, nil
}

func requireDeliveryAbsent(dir int, name string) error {
	_, exists, err := deliveryEntry(dir, name)
	if err != nil || exists {
		return ErrQuotaDeliveryUnavailable
	}
	return nil
}

func privateDeliveryInode(st *unix.Stat_t, links int) bool {
	return st.Mode&unix.S_IFMT == unix.S_IFREG && st.Uid == uint32(os.Geteuid()) &&
		st.Mode&07777 == 0600 && uint64(st.Nlink) == uint64(links)
}

func checkDeliveryEntry(dir int, name string, id deliveryInode, links int) error {
	named, exists, err := deliveryEntry(dir, name)
	if err != nil || !exists || !privateDeliveryInode(&named, links) || deliveryID(&named) != id {
		return ErrQuotaDeliveryUnavailable
	}
	return nil
}

func checkDeliveryFile(dir, fd int, name string, id deliveryInode, links int) error {
	if err := checkDeliveryEntry(dir, name, id, links); err != nil {
		return err
	}
	var held unix.Stat_t
	if unix.Fstat(fd, &held) != nil || !privateDeliveryInode(&held, links) || deliveryID(&held) != id {
		return ErrQuotaDeliveryUnavailable
	}
	return nil
}

type deliverySetup struct {
	guard *QuotaDeliveryGuard
	ctx   context.Context
	until time.Time
	dir   int
}

func (a deliverySetup) budget() error {
	if err := a.ctx.Err(); err != nil {
		return err
	}
	now := time.Now()
	if d, ok := a.ctx.Deadline(); ok && !now.Before(d) {
		return context.DeadlineExceeded
	}
	if !now.Before(a.until) {
		return ErrQuotaDeliveryBusy
	}
	return nil
}

func (a deliverySetup) next() error {
	if err := a.budget(); err != nil {
		return err
	}
	if _, err := a.guard.checkDirectory(a.dir); err != nil {
		return err
	}
	return a.budget()
}

func (a deliverySetup) sync(fd int) error {
	if err := a.next(); err != nil {
		return err
	}
	if err := a.guard.syncFD(fd); err != nil {
		return ErrQuotaDeliveryUnavailable
	}
	return nil
}

// prepare is called only by reconciliation under the cycle's refresh and
// ownership leases. A surviving official name or epoch excludes first use.
// The bound pending inode is the sole recoverable publication source.
func (a deliverySetup) prepare() error {
	if err := a.next(); err != nil {
		return err
	}
	id, bound, err := deliveryBinding(a.dir)
	if err != nil {
		return err
	}
	official, live, err := deliveryEntry(a.dir, deliveryName)
	if err != nil {
		return err
	}
	stage, pending, err := deliveryEntry(a.dir, deliveryPending)
	if err != nil {
		return err
	}
	if live && (!bound || deliveryID(&official) != id) {
		return ErrQuotaDeliveryUnavailable
	}
	if live && !pending {
		if !privateDeliveryInode(&official, 1) {
			return ErrQuotaDeliveryUnavailable
		}
		// A previous unlink may have completed just before its directory sync.
		return a.sync(a.dir)
	}
	_, epoch, err := deliveryEntry(a.dir, quotaEpochName)
	if err != nil || epoch || (bound && !pending) {
		return ErrQuotaDeliveryUnavailable
	}
	links := 1
	if live {
		links = 2
	}
	if pending && (!privateDeliveryInode(&stage, links) || (bound && deliveryID(&stage) != id)) {
		return ErrQuotaDeliveryUnavailable
	}
	flags := deliveryOpenFlags
	if !pending {
		flags = unix.O_RDWR | unix.O_NOFOLLOW | unix.O_NONBLOCK | unix.O_CLOEXEC | unix.O_CREAT | unix.O_EXCL
	}
	if err := a.next(); err != nil {
		return err
	}
	fd, err := unix.Openat(a.dir, deliveryPending, flags, 0600)
	if err != nil {
		return ErrQuotaDeliveryUnavailable
	}
	defer unix.Close(fd)
	if !bound {
		var st unix.Stat_t
		if unix.Fstat(fd, &st) != nil {
			return ErrQuotaDeliveryUnavailable
		}
		id = deliveryID(&st)
		if pending && id != deliveryID(&stage) {
			return ErrQuotaDeliveryUnavailable
		}
		if err := checkDeliveryFile(a.dir, fd, deliveryPending, id, 1); err != nil {
			return err
		}
		if err := a.sync(fd); err != nil {
			return err
		}
		if err := a.sync(a.dir); err != nil {
			return err
		}
		if err := a.next(); err != nil {
			return err
		}
		if err := checkDeliveryFile(a.dir, fd, deliveryPending, id, 1); err != nil {
			return err
		}
		if err := requireDeliveryAbsent(a.dir, deliveryName); err != nil {
			return err
		}
		if err := requireDeliveryAbsent(a.dir, quotaEpochName); err != nil {
			return err
		}
		if err := a.budget(); err != nil {
			return err
		}
		if unix.Fsetxattr(a.dir, deliveryAttr, id[:], unix.XATTR_CREATE) != nil {
			return ErrQuotaDeliveryUnavailable
		}
	}
	verify := func(n int) error {
		if err := a.next(); err != nil {
			return err
		}
		got, ok, err := deliveryBinding(a.dir)
		if err != nil || !ok || got != id {
			return ErrQuotaDeliveryUnavailable
		}
		if err := requireDeliveryAbsent(a.dir, quotaEpochName); err != nil {
			return err
		}
		return checkDeliveryFile(a.dir, fd, deliveryPending, id, n)
	}
	if err := verify(links); err != nil {
		return err
	}
	// Also required on resumption after XATTR_CREATE returned but fsync failed.
	if err := a.sync(a.dir); err != nil {
		return err
	}
	if !live {
		if err := verify(1); err != nil {
			return err
		}
		if err := a.budget(); err != nil {
			return err
		}
		if unix.Linkat(a.dir, deliveryPending, a.dir, deliveryName, 0) != nil {
			return ErrQuotaDeliveryUnavailable
		}
	}
	if err := verify(2); err != nil {
		return err
	}
	if err := checkDeliveryFile(a.dir, fd, deliveryName, id, 2); err != nil {
		return err
	}
	// Make the second name durable before dropping the first.
	if err := a.sync(a.dir); err != nil {
		return err
	}
	if err := verify(2); err != nil {
		return err
	}
	if err := checkDeliveryFile(a.dir, fd, deliveryName, id, 2); err != nil {
		return err
	}
	if err := a.budget(); err != nil {
		return err
	}
	if unix.Unlinkat(a.dir, deliveryPending, 0) != nil {
		return ErrQuotaDeliveryUnavailable
	}
	return a.sync(a.dir)
}

type heldDeliveryGuard struct {
	dir, file int
	rootID    string
	inode     deliveryInode
}

func (g *QuotaDeliveryGuard) verify(lock heldDeliveryGuard) error {
	id, err := g.checkDirectory(lock.dir)
	if err != nil || id != lock.rootID {
		return ErrQuotaDeliveryUnavailable
	}
	bound, ok, err := deliveryBinding(lock.dir)
	if err != nil || !ok || bound != lock.inode {
		return ErrQuotaDeliveryUnavailable
	}
	if err := requireDeliveryAbsent(lock.dir, deliveryPending); err != nil {
		return err
	}
	return checkDeliveryFile(lock.dir, lock.file, deliveryName, bound, 1)
}
