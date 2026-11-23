package quota

import (
	"context"
	"os"

	"github.com/kitdine/agent-deck/internal/platform"
	"github.com/kitdine/agent-deck/internal/store"
	"golang.org/x/sys/unix"
)

// RefreshCore pins the physical directory across store.Open and the entire
// refresh. Ownership is the existing process-wide platform session.
type RefreshCore struct {
	core  *store.Store
	root  string
	dir   *os.File
	id    string
	store *Store
}

func OpenRefreshCore(ctx context.Context, path string) (*RefreshCore, error) {
	root, err := platform.GuardStateRoot(path)
	if err != nil {
		return nil, err
	}
	if err := platform.EnsureStateRoot(root); err != nil {
		return nil, err
	}
	fd, err := unix.Open(root, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, ErrQuotaDeliveryUnavailable
	}
	dir := os.NewFile(uintptr(fd), "quota root")
	check := QuotaDeliveryGuard{root: root}
	id, err := check.checkDirectory(fd)
	if err != nil {
		dir.Close()
		return nil, err
	}
	core, err := store.Open(ctx, root)
	if err != nil {
		dir.Close()
		return nil, err
	}
	check.id = id
	if _, err = check.checkDirectory(fd); err != nil {
		core.Close()
		dir.Close()
		return nil, err
	}
	return &RefreshCore{core: core, store: NewWritableStore(core), root: root, dir: dir, id: id}, nil
}
func (r *RefreshCore) Core() *store.Store { return r.core }
func (r *RefreshCore) Store() *Store      { return r.store }
func (r *RefreshCore) Root() string       { return r.root }
func (r *RefreshCore) Close()             { r.core.Close(); r.dir.Close() }
