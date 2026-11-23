package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
)

// cjsiSessionRoute names the session index location that the committed core
// state of one guarded physical root authorizes.
type cjsiSessionRoute uint8

const (
	cjsiRouteNone cjsiSessionRoute = iota
	// The core predates the layout and records none of it. The legacy name
	// keeps its existing opener, admission and SQLite validation.
	cjsiRouteLegacy
	// The layout is ready over a root that never had an index. This route
	// authorizes no database at either name.
	cjsiRouteAbsent
	// The ready layout selected a private index whose identity, epoch and
	// seal matched the record read from the same core snapshot.
	cjsiRoutePrivate
)

// An opener rechecks the selection against its own read or write boundary.
type cjsiSessionLocation struct {
	route     cjsiSessionRoute
	path      string
	selection cjsiLayoutSelection
}

// Only the managed, drained resume advances an unfinished installation.
var errCJSILayoutPending = fmt.Errorf("%w: Codex session index layout installation pending", ErrStateBusy)

// locateCJSISessionIndex resolves the session index of an already guarded
// physical root from the caller's core transaction. The schema version and
// the layout journal come from that one snapshot, so a concurrent migration
// is either wholly visible or absent. Below the layout version the legacy
// name stays authoritative. At the layout version the journal is required,
// and no failure falls back to the legacy name. A later version is ahead of
// this binary until it learns that version's layout records.
//
// Verifying a selected index opens it under root admission, which a draining
// manager waits for or refuses. The pooled verification reader is idle on
// return, and the managed resume retires it with its install read pools.
func locateCJSISessionIndex(ctx context.Context, root string, core *sql.Tx) (cjsiSessionLocation, error) {
	if err := ctx.Err(); err != nil {
		return cjsiSessionLocation{}, err
	}
	if core == nil || !filepath.IsAbs(root) {
		return cjsiSessionLocation{}, errCJSILayoutInvalid
	}
	var version int
	if err := core.QueryRowContext(ctx, `SELECT version FROM main.schema_metadata WHERE singleton=1`).Scan(&version); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			err = errors.Join(ErrUnknownSchema, err)
		}
		return cjsiSessionLocation{}, err
	}
	var journaled bool
	if err := core.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM main.sqlite_master
		WHERE type='table' AND name='cjsi_layout_journal')`).Scan(&journaled); err != nil {
		return cjsiSessionLocation{}, err
	}
	switch {
	case version > cjsiLayoutCoreSchemaVersion:
		return cjsiSessionLocation{}, &SchemaAhead{Stored: version, Supported: cjsiLayoutCoreSchemaVersion}
	case version < cjsiLayoutCoreSchemaVersion && !journaled:
		return cjsiSessionLocation{route: cjsiRouteLegacy, path: filepath.Join(root, cjsiLegacyIndexName)}, nil
	case version < cjsiLayoutCoreSchemaVersion || !journaled:
		// The journal and the layout version commit in one transaction.
		return cjsiSessionLocation{}, errCJSILayoutInvalid
	}
	j, err := cjsiReadLayoutJournal(ctx, core)
	if err != nil {
		return cjsiSessionLocation{}, err
	}
	if err := cjsiCheckInstallRoot(root, j.root); err != nil {
		return cjsiSessionLocation{}, err
	}
	if j.phase != cjsiLayoutReady {
		return cjsiSessionLocation{}, errCJSILayoutPending
	}
	// As in recovery, an unrecorded fence fails before anything can open
	// the private index. The resolution below rechecks for replacements.
	if err := cjsiCheckRecordedFence(root, j.fence); err != nil {
		return cjsiSessionLocation{}, err
	}
	selection := j.selection()
	if selection.phase != cjsiLayoutSelected {
		return locateCJSIReadyLayout(ctx, root, j, selection)
	}
	if selection.seal.version != cjsiPrivateLayoutVersion {
		return cjsiSessionLocation{}, errCJSILayoutInvalid
	}
	pin, err := admitLegacy(ctx, root)
	if err != nil {
		return cjsiSessionLocation{}, err
	}
	location, err := cjsiSessionLocation{}, errCJSILayoutInvalid
	if (cjsiFileIdentity{pin.root.dev, pin.root.ino}) == j.root {
		location, err = locateCJSIReadyLayout(ctx, root, j, selection)
	}
	if err = errors.Join(err, pin.close()); err != nil {
		return cjsiSessionLocation{}, err
	}
	return location, nil
}

func locateCJSIReadyLayout(ctx context.Context, root string, j cjsiLayoutJournal, selection cjsiLayoutSelection) (cjsiSessionLocation, error) {
	resolution, err := resolveCJSILayout(ctx, root, selection, readCJSIProductionSeal)
	if err != nil {
		return cjsiSessionLocation{}, err
	}
	if err := cjsiCheckRecordedFence(root, j.fence); err != nil {
		return cjsiSessionLocation{}, err
	}
	if err := cjsiCheckInstallRoot(root, j.root); err != nil {
		return cjsiSessionLocation{}, err
	}
	switch {
	case selection.phase == cjsiLayoutNeverBuilt && resolution == (cjsiLayoutResolution{phase: cjsiLayoutNeverBuilt}):
		return cjsiSessionLocation{route: cjsiRouteAbsent}, nil
	case selection.phase == cjsiLayoutSelected && resolution.phase == cjsiLayoutSelected && resolution.path != "":
		return cjsiSessionLocation{route: cjsiRoutePrivate, path: resolution.path, selection: selection}, nil
	}
	return cjsiSessionLocation{}, errCJSILayoutInvalid
}

// The resolver checks fence content. The journal binds its identity.
func cjsiCheckRecordedFence(root string, recorded cjsiFileIdentity) error {
	if fence, err := validateCJSIFence(root); err != nil || fence != recorded {
		return errCJSIFenceInvalid
	}
	return nil
}
