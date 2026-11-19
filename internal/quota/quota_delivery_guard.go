package quota

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/kitdine/agent-deck/internal/platform"
	"golang.org/x/sys/unix"
	sqlite3 "modernc.org/sqlite/lib"
)

const (
	deliveryGuardWait = time.Second
	quotaEpochName    = "quota-delivery.epoch"
	quotaEpochMagic   = "AGENTDECK-QDG-1\n"
)

var (
	ErrQuotaDeliveryBusy        = errors.New("quota delivery guard busy")
	ErrQuotaDeliveryUnavailable = errors.New("quota delivery guard unavailable")
)

// QuotaDeliveryGuard is a reconciled refresh-cycle capability. The caller
// retains its ownership session and refresh lock throughout the cycle. No
// directory descriptor is retained between reconciliation and the transition.
type QuotaDeliveryGuard struct {
	cycle  RefreshCycle
	root   string
	id     string
	inode  deliveryInode
	syncFD func(int) error
}

// StateRootID is the decimal dev:ino of the verified physical state directory.
func (g *QuotaDeliveryGuard) StateRootID() string { return g.id }

// ReconcileQuotaDelivery derives the root from this cycle's open core, not
// from caller-supplied alert or path data. SQLite's committed value wins even
// if the previous process died after syncing the mirror but before COMMIT.
func (s *Store) ReconcileQuotaDelivery(ctx context.Context, cycle RefreshCycle) (*QuotaDeliveryGuard, error) {
	until := time.Now().Add(deliveryGuardWait)
	lookup, cancel := context.WithDeadline(ctx, until)
	defer cancel()
	if err := s.checkCycle(cycle); err != nil {
		return nil, ErrQuotaDeliveryUnavailable
	}
	var path string
	if err := s.db.QueryRowContext(lookup, `SELECT file FROM pragma_database_list WHERE name='main'`).Scan(&path); err != nil || !filepath.IsAbs(path) {
		if e := (deliverySetup{ctx: ctx, until: until}).budget(); e != nil {
			return nil, e
		}
		return nil, ErrQuotaDeliveryUnavailable
	}
	g := &QuotaDeliveryGuard{cycle: cycle, root: filepath.Dir(path), syncFD: unix.Fsync}
	err := g.acquire(ctx, until, true, func(lock heldDeliveryGuard) error {
		state, err := s.CommittedClaudeContext(ctx, cycle)
		if err != nil {
			return ErrQuotaDeliveryUnavailable
		}
		if current, ok := readQuotaEpoch(lock.dir); !ok || current != state.Generation {
			if err := g.publish(lock, state.Generation); err != nil {
				return err
			}
		}
		if err := g.verify(lock); err != nil {
			return err
		}
		g.id = lock.rootID
		g.inode = lock.inode
		return nil
	})
	if err != nil {
		return nil, err
	}
	return g, nil
}

// ApplyClaudeIdentity acquires the bound guard file before entering the frozen
// immediate transaction and releases it only after its outcome is known or
// reported unresolved. Never compensate an uncertain commit here.
func (g *QuotaDeliveryGuard) ApplyClaudeIdentity(ctx context.Context, obs IdentityObservation, base, until time.Time) (ClaudeIdentityResult, error) {
	result, err, _ := g.applyClaudeIdentity(ctx, obs, base, until)
	return result, err
}

// Preserve the public write outcome and distinguish a delivery failure from
// SQLite contention after entering the guard. Only the former needs the
// partial-alert diagnostic.
func (g *QuotaDeliveryGuard) applyClaudeIdentity(ctx context.Context, obs IdentityObservation, base, until time.Time) (ClaudeIdentityResult, error, bool) {
	result := ClaudeIdentityResult{Outcome: WriteUnresolved}
	if g == nil || g.cycle.owner == nil || g.id == "" {
		return result, ErrQuotaDeliveryUnavailable, false
	}
	entered, mirrorFailed := false, false
	err := g.locked(ctx, func(lock heldDeliveryGuard) error {
		entered = true
		var err error
		result, err = g.cycle.owner.ApplyClaudeIdentity(ctx, g.cycle, obs, base, until, func(ctx context.Context, next Generation) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			err := g.publish(lock, next)
			mirrorFailed = err != nil
			return err
		})
		if err != nil && !errors.Is(err, ErrQuotaDeliveryUnavailable) {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			var coded interface{ Code() int }
			if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &coded) &&
				(coded.Code()&255 == sqlite3.SQLITE_BUSY || coded.Code()&255 == sqlite3.SQLITE_LOCKED)) {
				return ErrQuotaDeliveryBusy
			}
			return ErrQuotaDeliveryUnavailable
		}
		return err
	})
	return result, err, mirrorFailed || (!entered && (errors.Is(err, ErrQuotaDeliveryBusy) || errors.Is(err, ErrQuotaDeliveryUnavailable)))
}

func (g *QuotaDeliveryGuard) locked(ctx context.Context, body func(heldDeliveryGuard) error) error {
	return g.acquire(ctx, time.Now().Add(deliveryGuardWait), false, body)
}

func (g *QuotaDeliveryGuard) acquire(ctx context.Context, until time.Time, initialize bool, body func(heldDeliveryGuard) error) error {
	a := deliverySetup{guard: g, ctx: ctx, until: until}
	if err := a.budget(); err != nil {
		return err
	}
	if initialize && g.cycle.owner == nil {
		return ErrQuotaDeliveryUnavailable
	}
	if g.cycle.owner != nil {
		if err := g.cycle.owner.checkCycle(g.cycle); err != nil {
			return ErrQuotaDeliveryUnavailable
		}
		if err := a.budget(); err != nil {
			return err
		}
	}
	root, err := platform.GuardStateRoot(g.root)
	if err != nil || root != g.root {
		return ErrQuotaDeliveryUnavailable
	}
	if err = a.budget(); err != nil {
		return err
	}
	dir, err := unix.Open(root, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return ErrQuotaDeliveryUnavailable
	}
	defer unix.Close(dir)
	a.dir = dir
	if initialize {
		if err = a.prepare(); err != nil {
			return err
		}
	}
	if err = a.next(); err != nil {
		return err
	}
	rootID, err := g.checkDirectory(dir)
	if err != nil {
		return err
	}
	inode, ok, err := deliveryBinding(dir)
	if err != nil || !ok || (g.inode != (deliveryInode{}) && g.inode != inode) {
		return ErrQuotaDeliveryUnavailable
	}
	if err := checkDeliveryEntry(dir, deliveryName, inode, 1); err != nil {
		return err
	}
	if err = a.budget(); err != nil {
		return err
	}
	file, err := unix.Openat(dir, deliveryName, deliveryOpenFlags, 0)
	if err != nil {
		return ErrQuotaDeliveryUnavailable
	}
	defer unix.Close(file)
	held := heldDeliveryGuard{dir, file, rootID, inode}
	for {
		if err = a.budget(); err != nil {
			return err
		}
		err = unix.Flock(file, unix.LOCK_EX|unix.LOCK_NB)
		if err == nil {
			break
		}
		if err != unix.EWOULDBLOCK && err != unix.EINTR {
			return ErrQuotaDeliveryUnavailable
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(min(10*time.Millisecond, time.Until(until))):
		}
	}
	if err = a.budget(); err != nil {
		return err
	}
	if err = g.verify(held); err != nil {
		return err
	}
	if err = a.budget(); err != nil {
		return err
	}
	return body(held)
}

// verifyAlertGuard checks the cycle's bound inode for an output proof. A
// valid guard may be held by the App, so this inspection takes no flock.
func (g *QuotaDeliveryGuard) verifyAlertGuard(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	root, err := platform.GuardStateRoot(g.root)
	if err != nil || root != g.root {
		return ErrQuotaDeliveryUnavailable
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	dir, err := unix.Open(root, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return ErrQuotaDeliveryUnavailable
	}
	defer unix.Close(dir)
	if err := checkDeliveryEntry(dir, deliveryName, g.inode, 1); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	file, err := unix.Openat(dir, deliveryName, deliveryOpenFlags, 0)
	if err != nil {
		return ErrQuotaDeliveryUnavailable
	}
	defer unix.Close(file)
	if err := g.verify(heldDeliveryGuard{dir: dir, file: file, rootID: g.id, inode: g.inode}); err != nil {
		return err
	}
	return ctx.Err()
}

func (g *QuotaDeliveryGuard) checkDirectory(fd int) (string, error) {
	root, err := platform.GuardStateRoot(g.root)
	var st, path unix.Stat_t
	if err != nil || root != g.root || unix.Fstat(fd, &st) != nil || unix.Lstat(root, &path) != nil ||
		st.Dev != path.Dev || st.Ino != path.Ino || st.Mode&unix.S_IFMT != unix.S_IFDIR ||
		st.Uid != uint32(os.Geteuid()) || st.Mode&07777 != 0700 {
		return "", ErrQuotaDeliveryUnavailable
	}
	id := fmt.Sprintf("%d:%d", uint64(st.Dev), st.Ino)
	if g.id != "" && g.id != id {
		return "", ErrQuotaDeliveryUnavailable
	}
	return id, nil
}

func readQuotaEpoch(dir int) (Generation, bool) {
	fd, err := unix.Openat(dir, quotaEpochName, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_CLOEXEC|unix.O_NONBLOCK, 0)
	if err != nil {
		return 0, false
	}
	defer unix.Close(fd)
	var st unix.Stat_t
	var image [len(quotaEpochMagic) + 8]byte
	if unix.Fstat(fd, &st) != nil || st.Uid != uint32(os.Geteuid()) || st.Mode&unix.S_IFMT != unix.S_IFREG || st.Mode&07777 != 0600 || st.Nlink != 1 || st.Size != int64(len(image)) {
		return 0, false
	}
	n, err := unix.Pread(fd, image[:], 0)
	value := binary.BigEndian.Uint64(image[len(quotaEpochMagic):])
	if err != nil || n != len(image) || string(image[:len(quotaEpochMagic)]) != quotaEpochMagic || value > math.MaxInt64 {
		return 0, false
	}
	return Generation(value), true
}

// Publish a complete private inode, rather than following or overwriting an
// invalid target. Sync both data and the containing directory before COMMIT.
func (g *QuotaDeliveryGuard) publish(lock heldDeliveryGuard, next Generation) error {
	dir := lock.dir
	if err := g.verify(lock); err != nil || next < 0 {
		return ErrQuotaDeliveryUnavailable
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return ErrQuotaDeliveryUnavailable
	}
	tmp := "." + quotaEpochName + "-" + hex.EncodeToString(nonce[:])
	fd, err := unix.Openat(dir, tmp, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0600)
	if err != nil {
		return ErrQuotaDeliveryUnavailable
	}
	defer unix.Unlinkat(dir, tmp, 0)
	var image [len(quotaEpochMagic) + 8]byte
	copy(image[:], quotaEpochMagic)
	binary.BigEndian.PutUint64(image[len(quotaEpochMagic):], uint64(next))
	err = unix.Fchmod(fd, 0600)
	if err == nil {
		var n int
		n, err = unix.Write(fd, image[:])
		if n != len(image) {
			err = ErrQuotaDeliveryUnavailable
		}
	}
	if err == nil {
		err = g.syncFD(fd)
	}
	closeErr := unix.Close(fd)
	if err != nil || closeErr != nil || g.verify(lock) != nil || unix.Renameat(dir, tmp, dir, quotaEpochName) != nil || g.syncFD(dir) != nil {
		return ErrQuotaDeliveryUnavailable
	}
	return g.verify(lock)
}
