package installation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/kitdine/agent-deck/internal/platform"
	"golang.org/x/sys/unix"
)

// Install refusal reasons share the install_claim_refused error code. The
// unregister protocol reuses the receipt reasons for manifests it cannot trust.
const (
	ReasonNotCurrentFormulaKeg = "not_current_formula_keg"
	ReasonOwnedByOther         = "owned_by_other"
	ReasonOwnerRunning         = "owner_running"
	ReasonReceiptMissing       = "receipt_missing"
	ReasonReceiptInvalid       = "receipt_invalid"
)

// Unregister outcomes. Each is a successful exit; failures are errors.
const (
	OutcomeRemoved           = "removed"
	OutcomeUnregistered      = "unregistered"
	OutcomeSkippedSuperseded = "skipped_superseded"
	OutcomeAbsent            = "absent"
	// OutcomeSkippedUnverified is the authorized-repair tolerance: ownership
	// could not be proven, so nothing was removed and the receipt is unchanged.
	OutcomeSkippedUnverified = "skipped_unverified"
)

const (
	maxManifestBytes = 4 * 1024
	bundleHelper     = "Contents/Helpers/agentdeck"
)

// InstallError is a refusal the user can act on. It is path-free like the
// state refusal: callers render only the reason.
type InstallError struct {
	Reason string
	Err    error
	// Detail is an optional path-free marker that installers key hints on.
	Detail string
}

func (e *InstallError) Error() string {
	if e.Err != nil {
		return e.Reason + ": " + e.Err.Error()
	}
	return e.Reason
}

func (e *InstallError) Unwrap() error { return e.Err }

func refused(reason string, err error) error { return &InstallError{Reason: reason, Err: err} }

// installHook lets tests pause or fail at named protocol points while the
// exclusive lock is held. It is nil in production builds.
var installHook func(point string) error

func runInstallHook(point string) error {
	if installHook == nil {
		return nil
	}
	return installHook(point)
}

// pluginkitTool is fixed in production. Acceptance builds may point it at a
// recording stub so tests never touch the account's real registrations.
var pluginkitTool = "/usr/bin/pluginkit"

// deregisterWidgets removes a bundle's extension registrations. It runs only
// after the receipt matched, inside the same exclusive lock as the removal.
var deregisterWidgets = func(root string) error {
	extensions, err := filepath.Glob(filepath.Join(root, "Contents", "PlugIns", "*.appex"))
	if err != nil || len(extensions) == 0 {
		return err
	}
	if _, err := os.Stat(pluginkitTool); err != nil {
		return nil
	}
	for _, extension := range extensions {
		if output, err := exec.Command(pluginkitTool, "-r", extension).CombinedOutput(); err != nil {
			return fmt.Errorf("pluginkit -r %s: %w: %s", extension, err, bytes.TrimSpace(output))
		}
	}
	return nil
}

// CleanupManifest fixes which installation a later cleanup may remove. The
// installer keeps it outside the final path until every cleanup call is done.
type CleanupManifest struct {
	InstallationID string `json:"installation_id"`
	Channel        string `json:"channel"`
	Executable     string `json:"executable"`
	RemoveRoot     string `json:"remove_root"`
}

func validChannel(channel string) bool {
	switch channel {
	case "homebrew_cask", "homebrew_formula", "make", "local_app":
		return true
	}
	return false
}

func cleanAbsolute(path string) bool {
	return path != "" && filepath.IsAbs(path) && filepath.Clean(path) == path
}

// removeRootFor accepts only the executable itself or the App bundle that
// contains it as Contents/Helpers/agentdeck; never an arbitrary ancestor.
func removeRootFor(executable, root string) bool {
	if root == executable {
		return true
	}
	return strings.HasSuffix(root, ".app") && filepath.Join(root, bundleHelper) == executable
}

func (m CleanupManifest) validate() error {
	if !installationIDPattern.MatchString(m.InstallationID) || !validChannel(m.Channel) || !cleanAbsolute(m.Executable) || !cleanAbsolute(m.RemoveRoot) || !removeRootFor(m.Executable, m.RemoveRoot) {
		return errors.New("invalid cleanup manifest")
	}
	return nil
}

func writeManifestExclusive(path string, manifest CleanupManifest) error {
	data, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	fd, err := unix.Open(path, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, uint32(platform.FileMode))
	if err != nil {
		return fmt.Errorf("create cleanup manifest: %w", err)
	}
	file := os.NewFile(uintptr(fd), filepath.Base(path))
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return err
	}
	return syncDirectory(filepath.Dir(path))
}

func readManifest(path string) (CleanupManifest, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return CleanupManifest{}, err
	}
	file := os.NewFile(uintptr(fd), filepath.Base(path))
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return CleanupManifest{}, err
	}
	if !info.Mode().IsRegular() || info.Size() > maxManifestBytes {
		return CleanupManifest{}, errors.New("invalid cleanup manifest file")
	}
	data, err := io.ReadAll(io.LimitReader(file, maxManifestBytes+1))
	if err != nil {
		return CleanupManifest{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var manifest CleanupManifest
	if err := decoder.Decode(&manifest); err != nil {
		return CleanupManifest{}, err
	}
	if decoder.More() {
		return CleanupManifest{}, errors.New("trailing cleanup manifest data")
	}
	return manifest, manifest.validate()
}

func (r *Resolver) exclusiveLock(ctx context.Context, home string) (*platform.OwnershipLock, error) {
	acquire := r.acquireLock
	if acquire == nil {
		acquire = platform.AcquireOwnershipLock
	}
	lock, err := acquire(ctx, home, true)
	if err != nil {
		var busy *platform.OwnershipLockError
		if errors.As(err, &busy) {
			return nil, refused(ReasonOwnerRunning, nil)
		}
		return nil, err
	}
	return lock, nil
}

// currentOwner reports the live receipt, or ok=false when no receipt can name
// an owner: missing, invalid, or stale because its executable is gone.
func currentOwner(home string) (Receipt, bool) {
	receipt, reason := ownerOrReason(home)
	return receipt, reason == ""
}

// ownerOrReason is currentOwner plus why there is no owner: a missing or
// stale receipt is receipt_missing, an unreadable one is receipt_invalid.
func ownerOrReason(home string) (Receipt, string) {
	receipt, err := readReceipt(home)
	if errors.Is(err, fs.ErrNotExist) {
		return Receipt{}, ReasonReceiptMissing
	}
	if err != nil {
		return Receipt{}, ReasonReceiptInvalid
	}
	if _, err := os.Lstat(receipt.Executable); errors.Is(err, fs.ErrNotExist) {
		return Receipt{}, ReasonReceiptMissing
	}
	return receipt, ""
}

// RegisterRequest is the hidden install register command's input.
type RegisterRequest struct {
	Channel         string
	Executable      string
	Source          string
	CleanupManifest string
	Takeover        bool
	// RequireUnowned, AuthorizedRepair and ExpectedInstallation are the cask
	// protocol. RequireUnowned publishes only when no valid owner exists and
	// the target is absent. AuthorizedRepair additionally replaces an unowned
	// existing target; with ExpectedInstallation it instead replaces exactly
	// the cask installation that still carries that ID, channel and path.
	RequireUnowned       bool
	AuthorizedRepair     bool
	ExpectedInstallation string
	// expectedSet records that --expected-installation was passed at all, so
	// an empty value is refused rather than read as "no binding".
	expectedSet bool
}

// BindExpectedInstallation marks the request as an identity-bound repair.
// The value is validated by Register; an empty or malformed ID is refused.
func (r *RegisterRequest) BindExpectedInstallation(id string) {
	r.ExpectedInstallation, r.expectedSet = id, true
}

type RegisterResult struct {
	Channel        string
	InstallationID string
	Takeover       bool
	// Residual names a replaced copy that publication could not delete. The
	// installation is complete; the copy is the installer's staging residue.
	Residual string
}

type publication struct {
	executable, source, root string
	bundle, replaced         bool
	sourceHelper             os.FileInfo
}

func preparePublication(request RegisterRequest) (publication, error) {
	p := publication{executable: request.Executable, source: request.Source, root: request.Executable}
	info, err := os.Lstat(request.Source)
	if err != nil {
		return p, fmt.Errorf("staged source: %w", err)
	}
	switch {
	case info.IsDir():
		p.bundle = true
		p.root = filepath.Dir(filepath.Dir(filepath.Dir(request.Executable)))
		if filepath.Join(p.root, bundleHelper) != request.Executable || !strings.HasSuffix(p.root, ".app") {
			return p, errors.New("an App bundle source requires an executable at <App>.app/Contents/Helpers/agentdeck")
		}
		helper, err := os.Lstat(filepath.Join(request.Source, bundleHelper))
		if err != nil || !helper.Mode().IsRegular() {
			return p, errors.New("staged App bundle has no regular Contents/Helpers/agentdeck")
		}
		p.sourceHelper = helper
	case info.Mode().IsRegular():
		p.sourceHelper = info
	default:
		return p, errors.New("staged source must be a regular file or an App bundle directory")
	}
	if filepath.Dir(request.Source) != filepath.Dir(p.root) || request.Source == p.root {
		return p, errors.New("staged source must be a sibling of the target in the same directory")
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(p.root))
	if err != nil || parent != filepath.Dir(p.root) {
		return p, errors.New("target directory must be a physical absolute path")
	}
	if rel, err := filepath.Rel(p.root, request.CleanupManifest); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return p, errors.New("cleanup manifest must be outside the installed object")
	}
	if existing, err := os.Lstat(p.root); err == nil {
		if p.bundle && (!existing.IsDir() || existing.Mode()&os.ModeSymlink != 0) {
			return p, errors.New("existing target is not an App bundle directory")
		}
		if !p.bundle && existing.IsDir() {
			return p, errors.New("existing target is a directory")
		}
		p.replaced = true
	} else if !errors.Is(err, fs.ErrNotExist) {
		return p, err
	}
	return p, nil
}

// publish moves the staged object onto the target. An existing target is
// swapped atomically so a failure before the receipt can swap it back.
func (p publication) publish() error {
	if p.replaced {
		return unix.RenamexNp(p.source, p.root, unix.RENAME_SWAP)
	}
	return unix.RenamexNp(p.source, p.root, unix.RENAME_EXCL)
}

func (p publication) unpublish() error {
	if p.replaced {
		return unix.RenamexNp(p.source, p.root, unix.RENAME_SWAP)
	}
	return unix.RenamexNp(p.root, p.source, unix.RENAME_EXCL)
}

func (p publication) verify() error {
	physical, err := filepath.EvalSymlinks(p.executable)
	if err != nil {
		return err
	}
	if physical != p.executable {
		return errors.New("published executable is not a physical path")
	}
	info, err := os.Lstat(p.executable)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || !os.SameFile(info, p.sourceHelper) {
		return errors.New("published executable is not the staged program")
	}
	return nil
}

// Register publishes and records one installation inside a single continuously
// held exclusive ownership lock. It never accesses the state directory.
func (r *Resolver) Register(ctx context.Context, request RegisterRequest) (RegisterResult, error) {
	if !validChannel(request.Channel) || !cleanAbsolute(request.Executable) {
		return RegisterResult{}, errors.New("register requires a valid --channel and an absolute clean --executable")
	}
	if (request.Source == "") != (request.CleanupManifest == "") {
		return RegisterResult{}, errors.New("--source and --cleanup-manifest must be used together")
	}
	if request.Source != "" && (!cleanAbsolute(request.Source) || !cleanAbsolute(request.CleanupManifest)) {
		return RegisterResult{}, errors.New("--source and --cleanup-manifest must be absolute clean paths")
	}
	if request.RequireUnowned && (request.Source == "" || request.Takeover) {
		return RegisterResult{}, errors.New("--require-unowned needs --source and cannot be combined with --takeover")
	}
	if request.AuthorizedRepair && !request.RequireUnowned {
		return RegisterResult{}, errors.New("--authorized-repair is only valid with --require-unowned")
	}
	if request.expectedSet && (!request.AuthorizedRepair || !installationIDPattern.MatchString(request.ExpectedInstallation)) {
		return RegisterResult{}, errors.New("--expected-installation needs --authorized-repair and a valid installation ID")
	}
	home, err := r.AccountHome()
	if err != nil {
		return RegisterResult{}, err
	}
	if request.Source == "" {
		self, err := r.self()
		if err != nil || self != request.Executable {
			return RegisterResult{}, errors.New("register without --source must be run by the program it registers")
		}
	}
	if err := runInstallHook("register_unlocked"); err != nil {
		return RegisterResult{}, err
	}
	lock, err := r.exclusiveLock(ctx, home)
	if err != nil {
		return RegisterResult{}, err
	}
	defer lock.Release()
	var result RegisterResult
	err = lock.WithExclusive(home, func() error {
		if err := runInstallHook("register_locked"); err != nil {
			return err
		}
		takeover := false
		owner, unownedReason := ownerOrReason(home)
		owned := unownedReason == ""
		// boundMatch is the one valid owner an identity-bound repair may
		// replace: every recorded value equals the value captured beforehand.
		boundMatch := owned && request.expectedSet && owner.InstallationID == request.ExpectedInstallation && owner.Channel == request.Channel && owner.Executable == request.Executable
		switch {
		case request.RequireUnowned:
			if owned && !boundMatch {
				return refused(ReasonOwnedByOther, nil)
			}
		case owned && !(owner.Executable == request.Executable && owner.Channel == request.Channel):
			if !request.Takeover {
				return refused(ReasonOwnedByOther, nil)
			}
			takeover = true
		}
		id, err := newInstallationID()
		if err != nil {
			return err
		}
		result = RegisterResult{Channel: request.Channel, InstallationID: id, Takeover: takeover}
		if request.Source == "" {
			_, _, err = r.writeReceiptLocked(home, request.Executable, request.Channel, id)
			return err
		}
		p, err := preparePublication(request)
		if err != nil {
			return err
		}
		if request.RequireUnowned && p.replaced && !boundMatch {
			// Unowned but occupied. Only an unbound authorized repair may
			// replace it; a bound repair that cannot prove its identity never
			// falls back to this branch.
			if !request.AuthorizedRepair || request.expectedSet {
				return refused(unownedReason, nil)
			}
		}
		if boundMatch && !p.replaced {
			return refused(ReasonOwnedByOther, nil)
		}
		manifest := CleanupManifest{InstallationID: id, Channel: request.Channel, Executable: request.Executable, RemoveRoot: p.root}
		if err := writeManifestExclusive(request.CleanupManifest, manifest); err != nil {
			return err
		}
		if err := runInstallHook("register_manifest"); err != nil {
			return err
		}
		if err := p.publish(); err != nil {
			return fmt.Errorf("publish %s: %w", p.root, err)
		}
		committed := false
		err = runInstallHook("register_published")
		if err == nil {
			err = p.verify()
		}
		if err == nil {
			_, committed, err = r.writeReceiptLocked(home, request.Executable, request.Channel, id)
		}
		if err != nil {
			if !committed {
				if restoreErr := p.unpublish(); restoreErr != nil {
					return errors.Join(err, fmt.Errorf("restore %s: %w", p.root, restoreErr))
				}
			}
			return err
		}
		if p.replaced {
			if err := os.RemoveAll(p.source); err != nil {
				result.Residual = p.source
			}
		}
		return runInstallHook("register_recorded")
	})
	if err != nil {
		return RegisterResult{}, err
	}
	return result, nil
}

// UnregisterRequest is the hidden install unregister command's input. Every
// value is the installer's own record; none is refreshed from the receipt.
type UnregisterRequest struct {
	Channel              string
	Executable           string
	ExpectedInstallation string
	CleanupManifest      string
	Remove               bool
	// AuthorizedRepair tolerates unprovable ownership without removing
	// anything. With an empty CleanupManifest it is the identity-bound check
	// for missing cleanup materials: ExpectedInstallation is then the ID the
	// user captured from the installed helper, and must still own the path.
	AuthorizedRepair bool
	// RequireIdentity carries the user's captured identity through cleanup with
	// complete materials. It validates that identity under the removal lock.
	RequireIdentity bool
}

type UnregisterResult struct {
	Outcome string
}

func objectExists(path string) (bool, error) {
	_, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

// Unregister removes exactly the installation named by the cleanup manifest,
// and only while the receipt still names that same installation.
func (r *Resolver) Unregister(ctx context.Context, request UnregisterRequest) (UnregisterResult, error) {
	if request.RequireIdentity && !request.AuthorizedRepair {
		return UnregisterResult{}, errors.New("--require-identity needs --authorized-repair")
	}
	checkOnly := request.AuthorizedRepair && request.CleanupManifest == ""
	bound := request.RequireIdentity || checkOnly
	if !validChannel(request.Channel) || !cleanAbsolute(request.Executable) || !installationIDPattern.MatchString(request.ExpectedInstallation) || (!checkOnly && !cleanAbsolute(request.CleanupManifest)) {
		return UnregisterResult{}, errors.New("unregister requires --channel, an absolute --executable, --expected-installation and an absolute --cleanup-manifest")
	}
	home, err := r.AccountHome()
	if err != nil {
		return UnregisterResult{}, err
	}
	lock, err := r.exclusiveLock(ctx, home)
	if err != nil {
		return UnregisterResult{}, err
	}
	defer lock.Release()
	var result UnregisterResult
	err = lock.WithExclusive(home, func() error {
		if err := runInstallHook("unregister_locked"); err != nil {
			return err
		}
		if bound {
			outcome, err := boundRepairCheck(home, request)
			if err != nil || checkOnly || outcome == OutcomeAbsent {
				result.Outcome = outcome
				return err
			}
		}
		manifest, err := readManifest(request.CleanupManifest)
		if err != nil && request.AuthorizedRepair {
			result.Outcome = OutcomeSkippedUnverified
			return nil
		}
		if errors.Is(err, fs.ErrNotExist) {
			return refused(ReasonReceiptMissing, errors.New("cleanup manifest is missing"))
		}
		if err != nil {
			return refused(ReasonReceiptInvalid, err)
		}
		if manifest.InstallationID != request.ExpectedInstallation || manifest.Channel != request.Channel || manifest.Executable != request.Executable {
			return refused(ReasonReceiptInvalid, errors.New("cleanup manifest does not match the requested installation"))
		}
		receipt, receiptErr := readReceipt(home)
		if receiptErr != nil {
			exists, err := objectExists(manifest.RemoveRoot)
			if err != nil {
				return err
			}
			if !exists {
				result.Outcome = OutcomeAbsent
				return nil
			}
			if request.AuthorizedRepair {
				result.Outcome = OutcomeSkippedUnverified
				return nil
			}
			if errors.Is(receiptErr, fs.ErrNotExist) {
				return refused(ReasonReceiptMissing, nil)
			}
			return refused(ReasonReceiptInvalid, receiptErr)
		}
		if receipt.InstallationID != manifest.InstallationID || receipt.Channel != manifest.Channel || receipt.Executable != manifest.Executable {
			result.Outcome = OutcomeSkippedSuperseded
			return nil
		}
		if !request.Remove {
			if err := removeReceiptLocked(home); err != nil {
				return err
			}
			result.Outcome = OutcomeUnregistered
			return nil
		}
		info, err := os.Lstat(manifest.RemoveRoot)
		exists := err == nil
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		bundle := manifest.RemoveRoot != manifest.Executable
		if exists {
			if info.Mode()&os.ModeSymlink != 0 || (bundle && !info.IsDir()) || (!bundle && !info.Mode().IsRegular()) {
				return fmt.Errorf("refusing to remove %s: it is not the recorded %s", manifest.RemoveRoot, map[bool]string{true: "App bundle", false: "regular file"}[bundle])
			}
			if bundle {
				if err := deregisterWidgets(manifest.RemoveRoot); err != nil {
					return err
				}
			}
		}
		if err := removeReceiptLocked(home); err != nil {
			return err
		}
		if err := runInstallHook("unregister_revoked"); err != nil {
			return err
		}
		if exists {
			if err := os.RemoveAll(manifest.RemoveRoot); err != nil {
				return fmt.Errorf("remove %s: %w", manifest.RemoveRoot, err)
			}
		}
		result.Outcome = OutcomeRemoved
		return nil
	})
	if err != nil {
		return UnregisterResult{}, err
	}
	return result, nil
}

// boundRepairCheck runs under the exclusive lock when an identity-bound
// repair has no usable cleanup materials. It never removes anything: the
// original installation must still be the owner for the tolerance to apply,
// and any successor or unprovable state is a refusal, not a skip.
func boundRepairCheck(home string, request UnregisterRequest) (string, error) {
	receipt, err := readReceipt(home)
	if err != nil {
		exists, statErr := objectExists(request.Executable)
		if statErr != nil {
			return "", statErr
		}
		if !exists {
			if bundle := filepath.Dir(filepath.Dir(filepath.Dir(request.Executable))); removeRootFor(request.Executable, bundle) {
				if exists, statErr = objectExists(bundle); statErr != nil {
					return "", statErr
				}
			}
		}
		switch {
		case !exists && errors.Is(err, fs.ErrNotExist):
			return OutcomeAbsent, nil
		case errors.Is(err, fs.ErrNotExist):
			return "", refused(ReasonReceiptMissing, nil)
		}
		return "", refused(ReasonReceiptInvalid, err)
	}
	if receipt.InstallationID != request.ExpectedInstallation || receipt.Channel != request.Channel || receipt.Executable != request.Executable {
		return "", refused(ReasonOwnedByOther, nil)
	}
	if err := verifyInstalledExecutable(receipt.Executable); err != nil {
		return "", refused(ReasonReceiptMissing, err)
	}
	return OutcomeSkippedUnverified, nil
}

func verifyInstalledExecutable(executable string) error {
	physical, err := filepath.EvalSymlinks(executable)
	if err != nil {
		return err
	}
	info, err := os.Lstat(executable)
	if err != nil {
		return err
	}
	if physical != executable || !info.Mode().IsRegular() {
		return errors.New("recorded executable is not a physical regular file")
	}
	return nil
}

// RepairIdentityRequest is the hidden install repair-identity command's
// input: the fixed App helper path of the cask the user selected.
type RepairIdentityRequest struct {
	Channel    string
	Executable string
}

// RepairIdentity reports the installation ID of the running cask helper's own
// valid installation. It holds the exclusive lock, writes nothing, and reads
// neither cleanup manifests, retained helpers, locators nor attempt indexes.
func (r *Resolver) RepairIdentity(ctx context.Context, request RepairIdentityRequest) (string, error) {
	if request.Channel != "homebrew_cask" || !cleanAbsolute(request.Executable) {
		return "", errors.New("repair-identity requires --channel homebrew_cask and an absolute clean --executable")
	}
	home, err := r.AccountHome()
	if err != nil {
		return "", err
	}
	self, err := r.self()
	if err != nil {
		return "", err
	}
	lock, err := r.exclusiveLock(ctx, home)
	if err != nil {
		return "", err
	}
	defer lock.Release()
	var id string
	err = lock.WithExclusive(home, func() error {
		if err := runInstallHook("repair_identity_locked"); err != nil {
			return err
		}
		receipt, err := readReceipt(home)
		if errors.Is(err, fs.ErrNotExist) {
			return refused(ReasonReceiptMissing, nil)
		}
		if err != nil {
			return refused(ReasonReceiptInvalid, err)
		}
		if receipt.Channel != request.Channel || receipt.Executable != request.Executable || self != request.Executable {
			return refused(ReasonOwnedByOther, nil)
		}
		if err := verifyInstalledExecutable(receipt.Executable); err != nil {
			return refused(ReasonReceiptMissing, err)
		}
		id = receipt.InstallationID
		return nil
	})
	if err != nil {
		return "", err
	}
	return id, nil
}

type ClaimResult struct {
	Channel  string
	Takeover bool
}

// Claim lets the current formula keg take ownership. It writes only the
// receipt, registering the keg's physical path rather than the opt link.
func (r *Resolver) Claim(ctx context.Context, takeover bool) (ClaimResult, error) {
	self, err := r.self()
	if err != nil {
		return ClaimResult{}, refused(ReasonNotCurrentFormulaKeg, err)
	}
	keg, ok := parseFormulaKeg(self)
	if !ok {
		return ClaimResult{}, refused(ReasonNotCurrentFormulaKeg, nil)
	}
	if opt, err := filepath.EvalSymlinks(filepath.Join(keg.prefix, "opt", keg.formula)); err != nil || opt != keg.root {
		return ClaimResult{}, refused(ReasonNotCurrentFormulaKeg, nil)
	}
	home, err := r.AccountHome()
	if err != nil {
		return ClaimResult{}, err
	}
	lock, err := r.exclusiveLock(ctx, home)
	if err != nil {
		return ClaimResult{}, err
	}
	defer lock.Release()
	result := ClaimResult{Channel: "homebrew_formula"}
	err = lock.WithExclusive(home, func() error {
		if err := runInstallHook("claim_locked"); err != nil {
			return err
		}
		if owner, ok := currentOwner(home); ok && owner.Executable != self {
			if !takeover {
				return refused(ReasonOwnedByOther, nil)
			}
			result.Takeover = true
		}
		id, err := newInstallationID()
		if err != nil {
			return err
		}
		_, _, err = r.writeReceiptLocked(home, self, "homebrew_formula", id)
		return err
	})
	if err != nil {
		return ClaimResult{}, err
	}
	return result, nil
}
