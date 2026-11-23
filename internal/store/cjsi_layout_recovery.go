package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	cjsiSnapshotName = ".cjsi-layout-legacy.sqlite3"
	cjsiPreparedName = ".cjsi-layout-private.sqlite3"
	cjsiFenceStage   = ".cjsi-layout-fence"
)

type cjsiInstallPhase uint8

const (
	cjsiFenceInstalling cjsiInstallPhase = iota + 1
	cjsiLegacyCaptured
	cjsiFenceDurable
	cjsiPrivatePrepared
	cjsiLayoutReady
)

// Persist this verification metadata in the core journal. The initial row
// and schema-ahead must commit together. No path or SQLite content is stored.
type cjsiLayoutJournal struct {
	phase    cjsiInstallPhase
	root     cjsiFileIdentity
	legacy   cjsiLayoutFile
	snapshot cjsiLayoutFile
	fence    cjsiFileIdentity
	prepared cjsiLayoutFile
	epoch    int64
	seal     cjsiIndexSeal
}

func (j cjsiLayoutJournal) valid() bool {
	captured := j.phase >= cjsiLegacyCaptured
	fenced := j.phase >= cjsiFenceDurable
	prepared := j.phase >= cjsiPrivatePrepared && j.legacy.present()
	return j.phase >= cjsiFenceInstalling && j.phase <= cjsiLayoutReady &&
		j.root.inode != 0 && j.legacy.valid() && (!j.legacy.present() || j.legacy.size > 0) &&
		(captured || j.legacy == (cjsiLayoutFile{})) &&
		j.snapshot.valid() && j.snapshot.present() == (captured && j.legacy.present()) &&
		(!j.snapshot.present() || j.snapshot.size > 0) &&
		(fenced && j.fence.inode != 0 || !fenced && j.fence == (cjsiFileIdentity{})) &&
		(j.phase != cjsiPrivatePrepared || prepared) &&
		j.prepared.valid() && j.prepared.present() == prepared && (!prepared || j.prepared.size > 0) &&
		(prepared && j.epoch > 0 && j.seal.valid() ||
			!prepared && j.epoch == 0 && j.seal == (cjsiIndexSeal{}))
}

func (j cjsiLayoutJournal) selection() cjsiLayoutSelection {
	if j.phase != cjsiLayoutReady {
		return cjsiLayoutSelection{phase: cjsiLayoutStaging}
	}
	if !j.legacy.present() {
		return cjsiLayoutSelection{phase: cjsiLayoutNeverBuilt}
	}
	return cjsiLayoutSelection{
		phase: cjsiLayoutSelected, locator: cjsiIndexPrivate,
		identity: j.prepared.identity, epoch: j.epoch, seal: j.seal,
	}
}

type cjsiLayoutInstall struct {
	// Load the committed journal from the schema-ahead core. Advance must
	// compare the whole previous record and durably commit the next one.
	load    func(context.Context) (cjsiLayoutJournal, error)
	advance func(context.Context, cjsiLayoutJournal, cjsiLayoutJournal) error

	// Upgrade the disposable private copy, commit its initial seal, and
	// close all handles to it before returning. The resolver reads the seal
	// independently. These adapters belong to the numbered migration.
	prepare  func(context.Context, string) (int64, cjsiIndexSeal, error)
	readSeal cjsiReadIndexSeal
}

// resumeCJSILayoutInstall runs under the existing maintenance then state lock.
// The caller must stop managed legacy clients and drain their SQLite handles
// before every invocation. Neither lock can revoke an old process's open FD.
// Layout readiness is installation metadata. It does not publish a ticket.
func resumeCJSILayoutInstall(ctx context.Context, root string, install cjsiLayoutInstall) error {
	if !filepath.IsAbs(root) || install.load == nil || install.advance == nil {
		return errCJSILayoutInvalid
	}
	j, err := install.load(ctx)
	if err != nil {
		return err
	}
	if !j.valid() {
		return errCJSILayoutInvalid
	}
	snapshot := filepath.Join(root, cjsiSnapshotName)
	staging := filepath.Join(root, cjsiPreparedName)
	final := filepath.Join(root, cjsiPrivateIndexName)
	legacy := filepath.Join(root, cjsiLegacyIndexName)

	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := cjsiCheckInstallRoot(root, j.root); err != nil {
			return err
		}
		if j.phase >= cjsiFenceDurable {
			fence, err := validateCJSIFence(root)
			if err != nil || fence != j.fence {
				return errCJSIFenceInvalid
			}
		}
		if j.phase == cjsiLayoutReady {
			_, err := resolveCJSILayout(ctx, root, j.selection(), install.readSeal)
			return err
		}
		if j.snapshot.present() && j.phase <= cjsiFenceDurable {
			if err := cjsiVerifyLayoutFile(ctx, snapshot, j.snapshot); err != nil {
				return fmt.Errorf("verify captured legacy index: %w", err)
			}
		}
		if j.phase != cjsiPrivatePrepared {
			if err := cjsiRequireCleanIndexPath(final); err != nil {
				return fmt.Errorf("unexpected private index before installation: %w", err)
			}
		}

		next := j
		switch j.phase {
		case cjsiFenceInstalling:
			next.legacy, next.snapshot, err = cjsiCaptureLegacy(ctx, root, legacy, snapshot)
			next.phase = cjsiLegacyCaptured
		case cjsiLegacyCaptured:
			next.fence, err = cjsiInstallFence(ctx, root, legacy, j)
			next.phase = cjsiFenceDurable
		case cjsiFenceDurable:
			if j.legacy.present() {
				next.prepared, next.epoch, next.seal, err = cjsiPreparePrivate(ctx, root, snapshot, staging, j, install)
				next.phase = cjsiPrivatePrepared
				break
			}
			next.phase = cjsiLayoutReady
		case cjsiPrivatePrepared:
			err = cjsiInstallPrivate(ctx, root, staging, final, j)
			next.phase = cjsiLayoutReady
		}
		if err != nil {
			return fmt.Errorf("resume session index layout at phase %d: %w", j.phase, err)
		}
		if err := cjsiCheckInstallRoot(root, j.root); err != nil {
			return err
		}
		if next.phase == cjsiLayoutReady {
			if _, err := resolveCJSILayout(ctx, root, next.selection(), install.readSeal); err != nil {
				return err
			}
			if err := cjsiCheckInstallRoot(root, j.root); err != nil {
				return err
			}
			if fence, err := validateCJSIFence(root); err != nil || fence != j.fence {
				return errCJSIFenceInvalid
			}
		}
		if !next.valid() {
			return errCJSILayoutInvalid
		}
		if err := install.advance(ctx, j, next); err != nil {
			return err
		}
		j = next
		if j.phase == cjsiLayoutReady {
			return nil
		}
	}
}

func cjsiCaptureLegacy(ctx context.Context, root, legacy, snapshot string) (cjsiLayoutFile, cjsiLayoutFile, error) {
	files, err := cjsiReadLegacyFiles(ctx, legacy)
	if err != nil {
		return cjsiLayoutFile{}, cjsiLayoutFile{}, err
	}
	// Before the capture commit, any files at the reserved snapshot name
	// are incomplete. After that commit the snapshot is immutable.
	if err := cjsiRemoveIndexFiles(snapshot); err != nil {
		return cjsiLayoutFile{}, cjsiLayoutFile{}, err
	}
	var image cjsiLayoutFile
	if files.main.present() {
		if err := cjsiBackupLayoutIndex(ctx, legacy, snapshot); err != nil {
			return cjsiLayoutFile{}, cjsiLayoutFile{}, err
		}
		if err := cjsiCheckSnapshot(ctx, snapshot); err != nil {
			return cjsiLayoutFile{}, cjsiLayoutFile{}, err
		}
		image, err = cjsiReadLayoutFile(ctx, snapshot, true)
		if err != nil {
			return cjsiLayoutFile{}, cjsiLayoutFile{}, err
		}
	}
	current, err := cjsiReadLegacyFiles(ctx, legacy)
	if err != nil || current != files {
		return cjsiLayoutFile{}, cjsiLayoutFile{}, errCJSILayoutInvalid
	}
	if files.main.present() {
		// A surviving WAL can mask the replacement main file. Checkpoint
		// the guarded legacy database while it still has its original name.
		// A restart before the capture commit can snapshot it again.
		if err := cjsiCheckpointLegacy(ctx, legacy); err != nil {
			return cjsiLayoutFile{}, cjsiLayoutFile{}, err
		}
		if err := cjsiSyncLayoutPath(legacy, false); err != nil {
			return cjsiLayoutFile{}, cjsiLayoutFile{}, err
		}
		current, err = cjsiReadLegacyFiles(ctx, legacy)
		if err != nil || current.wal.present() || current.main.identity != files.main.identity {
			return cjsiLayoutFile{}, cjsiLayoutFile{}, errCJSILayoutInvalid
		}
	}
	return current.main, image, cjsiSyncLayoutPath(root, true)
}

func cjsiInstallFence(ctx context.Context, root, legacy string, j cjsiLayoutJournal) (cjsiFileIdentity, error) {
	fence, err := validateCJSIFence(root)
	if err != nil {
		current, err := cjsiReadLegacyFiles(ctx, legacy)
		if err != nil || current.main != j.legacy || current.wal.present() {
			return cjsiFileIdentity{}, errCJSILayoutInvalid
		}
		tmp := filepath.Join(root, cjsiFenceStage)
		if err := cjsiRemoveLayoutFile(tmp); err != nil {
			return cjsiFileIdentity{}, err
		}
		if err := cjsiWriteFence(tmp); err != nil {
			return cjsiFileIdentity{}, err
		}
		if err := cjsiCheckInstallRoot(root, j.root); err != nil {
			return cjsiFileIdentity{}, err
		}
		if err := os.Rename(tmp, legacy); err != nil {
			return cjsiFileIdentity{}, err
		}
		fence, err = validateCJSIFence(root)
		if err != nil {
			return cjsiFileIdentity{}, err
		}
	}
	if err := cjsiCheckLegacySidecars(legacy, j.legacy.present()); err != nil {
		return cjsiFileIdentity{}, err
	}
	if err := cjsiCheckEmptyLayoutSidecar(legacy + "-wal"); err != nil {
		return cjsiFileIdentity{}, err
	}
	if err := cjsiProbeFence(ctx, legacy); err != nil {
		return cjsiFileIdentity{}, err
	}
	if current, err := validateCJSIFence(root); err != nil || current != fence {
		return cjsiFileIdentity{}, errCJSIFenceInvalid
	}
	if err := cjsiRemoveLayoutSidecars(legacy, false); err != nil {
		return cjsiFileIdentity{}, err
	}
	if err := cjsiSyncLayoutPath(legacy, false); err != nil {
		return cjsiFileIdentity{}, err
	}
	return fence, cjsiSyncLayoutPath(root, true)
}

func cjsiPreparePrivate(ctx context.Context, root, snapshot, staging string, j cjsiLayoutJournal, install cjsiLayoutInstall) (cjsiLayoutFile, int64, cjsiIndexSeal, error) {
	if install.prepare == nil || install.readSeal == nil {
		return cjsiLayoutFile{}, 0, cjsiIndexSeal{}, errCJSILayoutInvalid
	}
	if err := cjsiRemoveIndexFiles(staging); err != nil {
		return cjsiLayoutFile{}, 0, cjsiIndexSeal{}, err
	}
	if err := cjsiBackupLayoutIndex(ctx, snapshot, staging); err != nil {
		return cjsiLayoutFile{}, 0, cjsiIndexSeal{}, err
	}
	epoch, seal, err := install.prepare(ctx, staging)
	if err != nil {
		return cjsiLayoutFile{}, 0, cjsiIndexSeal{}, err
	}
	if epoch <= 0 || !seal.valid() {
		return cjsiLayoutFile{}, 0, cjsiIndexSeal{}, errCJSILayoutInvalid
	}
	if err := cjsiCheckInstallRoot(root, j.root); err != nil {
		return cjsiLayoutFile{}, 0, cjsiIndexSeal{}, err
	}
	if err := cjsiSealPrivateFile(ctx, staging, epoch, seal, install.readSeal); err != nil {
		return cjsiLayoutFile{}, 0, cjsiIndexSeal{}, err
	}
	image, err := cjsiReadLayoutFile(ctx, staging, true)
	if err != nil {
		return cjsiLayoutFile{}, 0, cjsiIndexSeal{}, err
	}
	return image, epoch, seal, cjsiSyncLayoutPath(root, true)
}

func cjsiInstallPrivate(ctx context.Context, root, staging, final string, j cjsiLayoutJournal) error {
	if _, err := os.Lstat(final); errors.Is(err, os.ErrNotExist) {
		if err := cjsiRequireCleanIndexPath(final); err != nil {
			return err
		}
		if err := cjsiVerifyLayoutFile(ctx, staging, j.prepared); err != nil {
			return err
		}
		// The held maintenance and state locks serialize managed access
		// to the destination. Rename never goes through the legacy name.
		if err := os.Rename(staging, final); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	if err := cjsiRequireCleanIndexPath(staging); err != nil {
		return err
	}
	if err := cjsiVerifyLayoutFile(ctx, final, j.prepared); err != nil {
		return err
	}
	return cjsiSyncLayoutPath(root, true)
}
