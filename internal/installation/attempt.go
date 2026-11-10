package installation

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/kitdine/agent-deck/internal/platform"
	"golang.org/x/sys/unix"
)

// A Homebrew cask keeps the materials of every install attempt outside the
// final App, the Caskroom and Homebrew's backup and purge roots. Each attempt
// gets a fresh private directory that is never reused or overwritten:
//
//	<account>/Library/Application Support/AgentDeck/installers/homebrew_cask/attempts/<attempt ID>/
//	    attempt.json   immutable index record: token, version, target
//	    helper         retained copy of the program that prepared the attempt
//	    manifest.json  cleanup manifest, written by register inside the lock
//	    cleaned        marker created after a successful conditional cleanup
//
// The cask's staged directory holds only a locator naming the attempt. It
// never stores a path, so replacing it cannot redirect what gets executed.
const (
	AttemptLocatorName  = ".agentdeck-attempt"
	attemptRecordName   = "attempt.json"
	attemptHelperName   = "helper"
	attemptManifestName = "manifest.json"
	attemptCleanedName  = "cleaned"
	caskChannel         = "homebrew_cask"

	// MaterialsUnavailableMarker prefixes the detail of a cleanup that could
	// not run because the attempt's own materials are gone while the
	// installation itself is still valid. Installers key their identity-bound
	// repair hint on it.
	MaterialsUnavailableMarker = "cleanup_materials_unavailable"
)

var (
	caskTokenPattern   = regexp.MustCompile(`^[a-z0-9][a-z0-9@._+-]{0,127}$`)
	caskVersionPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+,:-]{0,127}$`)
)

// attemptRecord is both the external index entry and the locator payload.
type attemptRecord struct {
	AttemptID  string `json:"attempt_id"`
	Token      string `json:"token"`
	Version    string `json:"version"`
	Executable string `json:"executable"`
}

func (a attemptRecord) validate() error {
	if !installationIDPattern.MatchString(a.AttemptID) || !caskTokenPattern.MatchString(a.Token) || !caskVersionPattern.MatchString(a.Version) || !cleanAbsolute(a.Executable) || filepath.Base(a.Executable) != "agentdeck" {
		return errors.New("invalid attempt record")
	}
	return nil
}

// AttemptRequest identifies one cask installation as the cask declares it.
type AttemptRequest struct {
	AttemptID  string
	Token      string
	Version    string
	Executable string
	Locator    string
}

func (q AttemptRequest) record() attemptRecord {
	return attemptRecord{AttemptID: q.AttemptID, Token: q.Token, Version: q.Version, Executable: q.Executable}
}

func (q AttemptRequest) validateLocator() error {
	if !cleanAbsolute(q.Locator) || filepath.Base(q.Locator) != AttemptLocatorName {
		return fmt.Errorf("--locator must be an absolute clean path named %s", AttemptLocatorName)
	}
	return nil
}

// AttemptMaterials are the helper-resolved paths of one attempt. Installers
// use these values verbatim and derive no AgentDeck path themselves.
type AttemptMaterials struct {
	AttemptID string
	Directory string
	Helper    string
	Manifest  string
}

func attemptsRoot(home string) string {
	return filepath.Join(home, "Library", "Application Support", "AgentDeck", "installers", caskChannel, "attempts")
}

func materialsFor(home, id string) AttemptMaterials {
	dir := filepath.Join(attemptsRoot(home), id)
	return AttemptMaterials{AttemptID: id, Directory: dir, Helper: filepath.Join(dir, attemptHelperName), Manifest: filepath.Join(dir, attemptManifestName)}
}

// ensurePrivateDirectory creates path and its missing ancestors below base as
// 0700 directories and refuses any existing component that is not a real
// directory, so a planted symlink can never relocate the materials.
func ensurePrivateDirectory(base, path string) error {
	rel, err := filepath.Rel(base, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return errors.New("private directory escapes the account directory")
	}
	current := base
	for _, part := range strings.Split(rel, string(os.PathSeparator)) {
		current = filepath.Join(current, part)
		if err := os.Mkdir(current, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
			return err
		}
		info, err := os.Lstat(current)
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return fmt.Errorf("%s is not a directory", current)
		}
	}
	return nil
}

func writeJSONExclusive(path string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return writeFileExclusive(path, append(data, '\n'), platform.FileMode)
}

func writeFileExclusive(path string, data []byte, mode os.FileMode) error {
	fd, err := unix.Open(path, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, uint32(mode))
	if err != nil {
		return fmt.Errorf("create %s: %w", filepath.Base(path), err)
	}
	file := os.NewFile(uintptr(fd), filepath.Base(path))
	_, err = file.Write(data)
	if err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(path)
		return err
	}
	return syncDirectory(filepath.Dir(path))
}

func readAttemptRecord(path string) (attemptRecord, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return attemptRecord{}, err
	}
	file := os.NewFile(uintptr(fd), filepath.Base(path))
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return attemptRecord{}, err
	}
	if !info.Mode().IsRegular() || info.Size() > maxManifestBytes {
		return attemptRecord{}, errors.New("invalid attempt record file")
	}
	data, err := io.ReadAll(io.LimitReader(file, maxManifestBytes+1))
	if err != nil {
		return attemptRecord{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var record attemptRecord
	if err := decoder.Decode(&record); err != nil {
		return attemptRecord{}, err
	}
	if decoder.More() {
		return attemptRecord{}, errors.New("trailing attempt record data")
	}
	return record, record.validate()
}

// retainHelper copies the running program into the attempt directory and
// checks the copy against what was read, so a later cleanup never depends on
// the installed App's helper being intact.
func retainHelper(self, target string) error {
	data, err := os.ReadFile(self)
	if err != nil {
		return err
	}
	if err := writeFileExclusive(target, data, 0o700); err != nil {
		return err
	}
	copied, err := os.ReadFile(target)
	if err != nil {
		return err
	}
	if sha256.Sum256(copied) != sha256.Sum256(data) {
		_ = os.Remove(target)
		return errors.New("retained helper does not match the staged program")
	}
	return nil
}

// PrepareAttempt creates the private materials of a new cask attempt and then
// points the staged directory's locator at it. It runs before register, so
// the materials are on disk before anything is published.
func (r *Resolver) PrepareAttempt(request AttemptRequest) (AttemptMaterials, error) {
	record := request.record()
	if err := record.validate(); err != nil {
		return AttemptMaterials{}, errors.New("attempt prepare requires a 32-hex --attempt, a cask --token and --version, and an absolute --executable")
	}
	if err := request.validateLocator(); err != nil {
		return AttemptMaterials{}, err
	}
	home, err := r.AccountHome()
	if err != nil {
		return AttemptMaterials{}, err
	}
	self, err := r.self()
	if err != nil {
		return AttemptMaterials{}, err
	}
	if err := ensurePrivateDirectory(home, attemptsRoot(home)); err != nil {
		return AttemptMaterials{}, fmt.Errorf("attempt materials root: %w", err)
	}
	materials := materialsFor(home, record.AttemptID)
	if err := os.Mkdir(materials.Directory, 0o700); err != nil {
		return AttemptMaterials{}, fmt.Errorf("create attempt directory: %w", err)
	}
	if err := runInstallHook("attempt_directory"); err != nil {
		return AttemptMaterials{}, err
	}
	if err := retainHelper(self, materials.Helper); err != nil {
		return AttemptMaterials{}, fmt.Errorf("retain cleanup helper: %w", err)
	}
	if err := writeJSONExclusive(filepath.Join(materials.Directory, attemptRecordName), record); err != nil {
		return AttemptMaterials{}, err
	}
	if err := syncDirectory(attemptsRoot(home)); err != nil {
		return AttemptMaterials{}, err
	}
	if err := writeLocator(request.Locator, record); err != nil {
		return AttemptMaterials{}, fmt.Errorf("write attempt locator: %w", err)
	}
	return materials, nil
}

// writeLocator replaces the locator atomically. Replacing it never touches the
// previous attempt, which stays addressable by its immutable ID.
func writeLocator(path string, record attemptRecord) error {
	dir := filepath.Dir(path)
	info, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("locator directory is not a directory")
	}
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(dir, AttemptLocatorName+".*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	if err := file.Chmod(platform.FileMode); err != nil {
		return err
	}
	if _, err := file.Write(append(data, '\n')); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(file.Name(), path); err != nil {
		return err
	}
	return syncDirectory(dir)
}

// CompleteAttempt records that publication finished. The marker is advisory:
// cleanup decisions never read it, they compare the manifest with the receipt.
func (r *Resolver) CompleteAttempt(request AttemptRequest) error {
	if !installationIDPattern.MatchString(request.AttemptID) {
		return errors.New("attempt complete requires a 32-hex --attempt")
	}
	home, err := r.AccountHome()
	if err != nil {
		return err
	}
	if err := runInstallHook("attempt_complete"); err != nil {
		return err
	}
	materials := materialsFor(home, request.AttemptID)
	if _, err := readAttemptRecord(filepath.Join(materials.Directory, attemptRecordName)); err != nil {
		return fmt.Errorf("attempt record: %w", err)
	}
	return markAttempt(materials, "published")
}

func markAttempt(materials AttemptMaterials, name string) error {
	err := writeFileExclusive(filepath.Join(materials.Directory, name), []byte(materials.AttemptID+"\n"), platform.FileMode)
	if errors.Is(err, fs.ErrExist) {
		return nil
	}
	return err
}

// CleanupRequest asks for the conditional cleanup of one cask attempt. With
// AttemptID set it is a rollback of exactly that attempt and never carries
// repair authority; otherwise the locator names the installation to remove.
type CleanupRequest struct {
	AttemptRequest
	AuthorizedRepair     bool
	ExpectedInstallation string
	expectedSet          bool
}

// BindExpectedInstallation marks the cleanup as part of an identity-bound
// repair. The ID is the one the user captured; it is never refreshed.
func (q *CleanupRequest) BindExpectedInstallation(id string) {
	q.ExpectedInstallation, q.expectedSet = id, true
}

// prepareHelperCommand is nil in production. Acceptance builds use it so a
// test process can start its own binary as the helper.
var prepareHelperCommand func(*exec.Cmd)

// runHelper executes one retained or installed helper. Tests replace it.
var runHelper = func(ctx context.Context, program string, args ...string) (stdout, stderr []byte, err error) {
	var out, errOut bytes.Buffer
	command := exec.CommandContext(ctx, program, args...)
	if prepareHelperCommand != nil {
		prepareHelperCommand(command)
	}
	command.Stdout, command.Stderr = &out, &errOut
	err = command.Run()
	return out.Bytes(), errOut.Bytes(), err
}

var refusalReasonPattern = regexp.MustCompile(`(?m)^\s*reason: ([a-z_]+)\s*$`)

// helperUnregister runs install unregister in another program and maps its
// text contract back to an outcome or the refusal it reported.
func helperUnregister(ctx context.Context, program string, args ...string) (string, error) {
	stdout, stderr, err := runHelper(ctx, program, append([]string{"install", "unregister"}, args...)...)
	if err != nil {
		if match := refusalReasonPattern.FindSubmatch(stderr); match != nil {
			return "", refused(string(match[1]), nil)
		}
		return "", fmt.Errorf("cleanup helper failed: %w: %s", err, bytes.TrimSpace(stderr))
	}
	text := string(stdout)
	for _, outcome := range []string{OutcomeSkippedSuperseded, OutcomeSkippedUnverified, OutcomeUnregistered, OutcomeRemoved, OutcomeAbsent} {
		if strings.HasPrefix(text, strings.ReplaceAll(outcome, "_", " ")+" ") {
			return outcome, nil
		}
	}
	return "", fmt.Errorf("cleanup helper reported an unknown result: %s", bytes.TrimSpace(stdout))
}

func usableHelper(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o100 != 0
}

// resolveAttempt finds the attempt named by the request and reports which of
// its materials are usable. A missing piece is a fact, not an error.
func resolveAttempt(home string, request CleanupRequest) (materials AttemptMaterials, manifest CleanupManifest, problem string) {
	id := request.AttemptID
	if id == "" {
		located, err := readAttemptRecord(request.Locator)
		if err != nil {
			return AttemptMaterials{}, CleanupManifest{}, "attempt locator"
		}
		if located.Token != request.Token || located.Version != request.Version || located.Executable != request.Executable {
			return AttemptMaterials{}, CleanupManifest{}, "attempt locator"
		}
		id = located.AttemptID
	}
	materials = materialsFor(home, id)
	if root, err := filepath.EvalSymlinks(materials.Directory); err != nil || root != materials.Directory {
		return materials, CleanupManifest{}, "attempt directory"
	}
	record, err := readAttemptRecord(filepath.Join(materials.Directory, attemptRecordName))
	if err != nil || record != (attemptRecord{AttemptID: id, Token: request.Token, Version: request.Version, Executable: request.Executable}) {
		return materials, CleanupManifest{}, "attempt record"
	}
	manifest, err = readManifest(materials.Manifest)
	if err != nil || manifest.Channel != caskChannel || manifest.Executable != request.Executable {
		return materials, CleanupManifest{}, "cleanup manifest"
	}
	if !usableHelper(materials.Helper) {
		return materials, manifest, "retained helper"
	}
	return materials, manifest, ""
}

// CleanupAttempt removes a cask installation only through the retained
// helper's conditional unregister. It deletes nothing itself, and reports
// unusable materials instead of falling back to a path-based removal.
func (r *Resolver) CleanupAttempt(ctx context.Context, request CleanupRequest) (string, error) {
	rollback := request.AttemptID != ""
	if !caskTokenPattern.MatchString(request.Token) || !caskVersionPattern.MatchString(request.Version) || !cleanAbsolute(request.Executable) || filepath.Base(request.Executable) != "agentdeck" {
		return "", errors.New("attempt cleanup requires a cask --token and --version and an absolute --executable")
	}
	if rollback {
		if !installationIDPattern.MatchString(request.AttemptID) || request.AuthorizedRepair || request.expectedSet {
			return "", errors.New("a rollback names one 32-hex --attempt and never carries repair authority")
		}
	} else if err := request.validateLocator(); err != nil {
		return "", err
	}
	if request.expectedSet && (!request.AuthorizedRepair || !installationIDPattern.MatchString(request.ExpectedInstallation)) {
		return "", errors.New("--expected-installation needs --authorized-repair and a valid installation ID")
	}
	home, err := r.AccountHome()
	if err != nil {
		return "", err
	}
	materials, manifest, problem := resolveAttempt(home, request)
	base := []string{"--channel", caskChannel, "--executable", request.Executable}
	if problem == "" {
		if request.expectedSet && manifest.InstallationID != request.ExpectedInstallation {
			// The locator now names a successor's complete materials. A bound
			// repair never cleans up an installation it did not select.
			return "", refused(ReasonOwnedByOther, nil)
		}
		args := append(base, "--expected-installation", manifest.InstallationID, "--cleanup-manifest", materials.Manifest, "--remove")
		if request.AuthorizedRepair {
			args = append(args, "--authorized-repair")
		}
		if request.expectedSet {
			args = append(args, "--require-identity")
		}
		outcome, err := helperUnregister(ctx, materials.Helper, args...)
		if err != nil {
			return "", err
		}
		if request.expectedSet && outcome == OutcomeSkippedSuperseded {
			return "", refused(ReasonOwnedByOther, nil)
		}
		if outcome == OutcomeRemoved || outcome == OutcomeAbsent {
			if err := markAttempt(materials, attemptCleanedName); err != nil {
				return "", fmt.Errorf("installation removed, but attempt %s could not be marked cleaned: %w", materials.AttemptID, err)
			}
		}
		return outcome, nil
	}
	switch {
	case rollback:
		return "", fmt.Errorf("attempt %s was not cleaned up: its %s is unavailable; nothing was removed", request.AttemptID, problem)
	case request.expectedSet:
		// Identity-bound repair with missing materials: the helper checks the
		// captured identity inside the lock and removes nothing.
		program := request.Executable
		if materials.Helper != "" && usableHelper(materials.Helper) {
			program = materials.Helper
		}
		return helperUnregister(ctx, program, append(base, "--expected-installation", request.ExpectedInstallation, "--authorized-repair", "--require-identity")...)
	case request.AuthorizedRepair:
		return OutcomeSkippedUnverified, nil
	}
	return "", r.materialsRefusal(ctx, home, request, problem)
}

// materialsRefusal classifies a cleanup that cannot run. A still-valid cask
// installation at the target is reported with MaterialsUnavailableMarker so
// the installer offers the identity-bound repair; otherwise the receipt
// reason selects the unowned repair.
func (r *Resolver) materialsRefusal(ctx context.Context, home string, request CleanupRequest, problem string) error {
	lock, err := r.exclusiveLock(ctx, home)
	if err != nil {
		return err
	}
	defer lock.Release()
	return lock.WithExclusive(home, func() error {
		receipt, reason := ownerOrReason(home)
		if reason != "" {
			return refused(reason, fmt.Errorf("the %s is unavailable", problem))
		}
		if receipt.Channel == caskChannel && receipt.Executable == request.Executable {
			return &InstallError{Reason: ReasonReceiptInvalid, Detail: MaterialsUnavailableMarker, Err: fmt.Errorf("%s: the %s is unavailable", MaterialsUnavailableMarker, problem)}
		}
		return refused(ReasonOwnedByOther, nil)
	})
}
