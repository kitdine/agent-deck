// Package shellconfig owns persistent AgentDeck shell integration blocks.
package shellconfig

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type Shell string

const (
	ShellBash Shell = "bash"
	ShellFish Shell = "fish"
	ShellZsh  Shell = "zsh"
)

const (
	managedVersion = 2
	startMarker    = "# >>> agentdeck shell integration >>>"
	endMarker      = "# <<< agentdeck shell integration <<<"
)

type Invocation struct {
	Shell Shell
	Login bool
}

type Environment struct {
	Home          string
	ZDOTDir       string
	XDGConfigHome string
	StateRoot     string
	Invocation    Invocation
}

type Request struct {
	Shell Shell
	RC    string
}

type Outcome string

const (
	OutcomeConfigured Outcome = "configured"
	OutcomeUnchanged  Outcome = "unchanged"
	OutcomeRemoved    Outcome = "removed"
	OutcomeAbsent     Outcome = "absent"
	OutcomeSkipped    Outcome = "skipped"
	OutcomeFailed     Outcome = "failed"
)

type Result struct {
	Shell   Shell   `json:"shell"`
	Path    string  `json:"path"`
	Outcome Outcome `json:"outcome"`
	Error   string  `json:"error,omitempty"`
}

type Summary struct {
	Results []Result `json:"results"`
}

func (s Summary) HasFailures() bool {
	for _, result := range s.Results {
		if result.Outcome == OutcomeFailed {
			return true
		}
	}
	return false
}

type temporaryFile interface {
	io.Writer
	Name() string
	Chmod(fs.FileMode) error
	Sync() error
	Close() error
}

type syncCloser interface {
	Sync() error
	Close() error
}

type fileOperations struct {
	lstat      func(string) (fs.FileInfo, error)
	readFile   func(string) ([]byte, error)
	mkdirAll   func(string, fs.FileMode) error
	createTemp func(string, string) (temporaryFile, error)
	rename     func(string, string) error
	remove     func(string) error
	openDir    func(string) (syncCloser, error)
	sameFile   func(fs.FileInfo, fs.FileInfo) bool
}

func osFileOperations() fileOperations {
	return fileOperations{
		lstat:    os.Lstat,
		readFile: os.ReadFile,
		mkdirAll: os.MkdirAll,
		createTemp: func(dir, pattern string) (temporaryFile, error) {
			return os.CreateTemp(dir, pattern)
		},
		rename: os.Rename,
		remove: os.Remove,
		openDir: func(path string) (syncCloser, error) {
			return os.Open(path)
		},
		sameFile: os.SameFile,
	}
}

type Manager struct {
	environment Environment
	files       fileOperations
	ownsFile    func(fs.FileInfo) bool
}

func New(environment Environment) *Manager {
	return &Manager{
		environment: environment,
		files:       osFileOperations(),
		ownsFile:    ownedByCurrentUser,
	}
}

func (m *Manager) Setup(request Request) (Summary, error) {
	targets, err := m.targets(request, true)
	if err != nil {
		return Summary{}, err
	}
	summary := Summary{Results: make([]Result, 0, len(targets))}
	for _, target := range targets {
		if !target.selected {
			summary.Results = append(summary.Results, Result{
				Shell:   target.shell,
				Path:    target.path,
				Outcome: OutcomeSkipped,
			})
			continue
		}
		outcome, setupErr := m.setupFile(target.path, target.shell)
		result := Result{Shell: target.shell, Path: target.path, Outcome: outcome}
		if setupErr != nil {
			result.Outcome = OutcomeFailed
			result.Error = setupErr.Error()
		}
		summary.Results = append(summary.Results, result)
	}
	return summary, nil
}

// SetupIfUnconfigured installs every selected target as one operation only
// when none already carries the current managed block. All replacement and
// rollback work stays inside the editor so callers never compensate with raw
// startup-file operations.
func (m *Manager) SetupIfUnconfigured(request Request) (Summary, error) {
	targets, err := m.targets(request, true)
	if err != nil {
		return Summary{}, err
	}
	summary := Summary{Results: make([]Result, 0, len(targets))}
	changes := make([]*preparedSetupChange, 0, len(targets))
	currentBlockPresent := false
	preparationFailed := false
	for _, target := range targets {
		result := Result{Shell: target.shell, Path: target.path}
		if !target.selected {
			result.Outcome = OutcomeSkipped
			summary.Results = append(summary.Results, result)
			continue
		}
		change, outcome, prepareErr := m.prepareSetupChange(target, len(summary.Results))
		result.Outcome = outcome
		if prepareErr != nil {
			result.Outcome = OutcomeFailed
			result.Error = prepareErr.Error()
			preparationFailed = true
		} else if outcome == OutcomeUnchanged {
			currentBlockPresent = true
		} else {
			changes = append(changes, change)
		}
		summary.Results = append(summary.Results, result)
	}
	defer m.cleanupPreparedSetup(changes)

	if preparationFailed || currentBlockPresent {
		for _, change := range changes {
			summary.Results[change.resultIndex].Outcome = OutcomeSkipped
		}
		return summary, nil
	}
	for _, change := range changes {
		if err := m.targetUnchanged(change.path, change.expected); err != nil {
			transactionErr := fmt.Errorf("verify shell startup transaction: %w", err)
			m.failPreparedSetup(&summary, changes, transactionErr)
			return summary, transactionErr
		}
	}

	committed := make([]*preparedSetupChange, 0, len(changes))
	for _, change := range changes {
		if err := m.files.rename(change.temporaryPath, change.path); err != nil {
			transactionErr := fmt.Errorf("replace shell startup file: %w", err)
			rollbackErr := m.rollbackPreparedSetup(committed)
			combined := errors.Join(transactionErr, rollbackErr)
			m.failPreparedSetup(&summary, changes, combined)
			return summary, combined
		}
		installed, inspectErr := m.files.lstat(change.path)
		if inspectErr != nil {
			transactionErr := fmt.Errorf("inspect replaced shell startup file: %w", inspectErr)
			rollbackErr := m.rollbackPreparedSetup(committed)
			combined := errors.Join(transactionErr, rollbackErr)
			m.failPreparedSetup(&summary, changes, combined)
			return summary, combined
		}
		if !sameSnapshot(m.files.sameFile, change.replacement, installed) {
			transactionErr := errors.New(
				"preserve concurrent shell startup change: replacement changed during transaction commit",
			)
			rollbackErr := m.rollbackPreparedSetup(committed)
			combined := errors.Join(transactionErr, rollbackErr)
			m.failPreparedSetup(&summary, changes, combined)
			return summary, combined
		}
		change.installed = installed
		committed = append(committed, change)
		if err := m.syncDirectory(filepath.Dir(change.path)); err != nil {
			transactionErr := fmt.Errorf("finalize shell startup file: %w", err)
			rollbackErr := m.rollbackPreparedSetup(committed)
			combined := errors.Join(transactionErr, rollbackErr)
			m.failPreparedSetup(&summary, changes, combined)
			return summary, combined
		}
	}
	return summary, nil
}

type preparedSetupChange struct {
	resultIndex    int
	shell          Shell
	path           string
	original       []byte
	updated        []byte
	expected       fs.FileInfo
	replacement    fs.FileInfo
	installed      fs.FileInfo
	temporaryPath  string
	backupPath     string
	restored       bool
	rollbackFailed bool
}

func (m *Manager) prepareSetupChange(target target, resultIndex int) (*preparedSetupChange, Outcome, error) {
	if err := m.files.mkdirAll(filepath.Dir(target.path), 0o700); err != nil {
		return nil, OutcomeFailed, fmt.Errorf("create shell startup directory: %w", err)
	}
	contents, info, err := m.readStartup(target.path)
	if errors.Is(err, fs.ErrNotExist) {
		contents, info = nil, nil
	} else if err != nil {
		return nil, OutcomeFailed, err
	}
	block := inspectManagedBlock(contents)
	if block.state == blockInvalid {
		return nil, OutcomeFailed, block.err
	}
	if block.state == blockValid {
		if err := m.validateManagedBlock(block, target.shell); err != nil {
			return nil, OutcomeFailed, err
		}
		if block.version == managedVersion && bytes.Equal(block.body, m.managedBody(target.shell)) {
			return nil, OutcomeUnchanged, nil
		}
		contents = replaceBytes(
			contents,
			block.start,
			block.end,
			m.buildManagedBlock(target.shell, block.separatorAdded),
		)
	} else {
		separatorAdded := len(contents) > 0 && contents[len(contents)-1] != '\n'
		updated := append([]byte(nil), contents...)
		if separatorAdded {
			updated = append(updated, '\n')
		}
		contents = append(updated, m.buildManagedBlock(target.shell, separatorAdded)...)
	}

	mode := fs.FileMode(0o600)
	if info != nil {
		mode = info.Mode().Perm()
	}
	change := &preparedSetupChange{
		resultIndex: resultIndex,
		shell:       target.shell,
		path:        target.path,
		original:    nil,
		updated:     contents,
		expected:    info,
	}
	if info != nil {
		original, readInfo, readErr := m.readStartup(target.path)
		if readErr != nil {
			return nil, OutcomeFailed, readErr
		}
		if !sameSnapshot(m.files.sameFile, info, readInfo) {
			return nil, OutcomeFailed, errors.New("shell startup file changed during transaction preparation")
		}
		change.original = original
	}
	change.temporaryPath, err = m.writeTemporary(
		filepath.Dir(target.path),
		".agentdeck-shell-*",
		change.updated,
		mode,
	)
	if err != nil {
		return nil, OutcomeFailed, err
	}
	change.replacement, err = m.files.lstat(change.temporaryPath)
	if err != nil {
		_ = m.files.remove(change.temporaryPath)
		return nil, OutcomeFailed, fmt.Errorf("inspect prepared shell startup replacement: %w", err)
	}
	if info != nil {
		change.backupPath, err = m.writeTemporary(
			filepath.Dir(target.path),
			".agentdeck-shell-backup-*",
			change.original,
			mode,
		)
		if err != nil {
			_ = m.files.remove(change.temporaryPath)
			return nil, OutcomeFailed, fmt.Errorf("prepare shell startup rollback: %w", err)
		}
	}
	return change, OutcomeConfigured, nil
}

func (m *Manager) cleanupPreparedSetup(changes []*preparedSetupChange) {
	for _, change := range changes {
		if change.rollbackFailed && !change.restored {
			continue
		}
		if change.temporaryPath != "" {
			_ = m.files.remove(change.temporaryPath)
		}
		if change.backupPath != "" {
			_ = m.files.remove(change.backupPath)
		}
	}
}

func (m *Manager) failPreparedSetup(summary *Summary, changes []*preparedSetupChange, err error) {
	for _, change := range changes {
		result := &summary.Results[change.resultIndex]
		result.Outcome = OutcomeFailed
		result.Error = err.Error()
	}
}

func (m *Manager) rollbackPreparedSetup(changes []*preparedSetupChange) error {
	var rollbackErr error
	for index := len(changes) - 1; index >= 0; index-- {
		change := changes[index]
		if err := m.restorePreparedSetup(change); err != nil {
			change.rollbackFailed = true
			rollbackErr = errors.Join(
				rollbackErr,
				fmt.Errorf("rollback shell startup file %s: %w", change.path, err),
			)
		}
	}
	return rollbackErr
}

func (m *Manager) restorePreparedSetup(change *preparedSetupChange) error {
	installed := change.installed
	if installed == nil {
		contents, info, err := m.readStartup(change.path)
		if err != nil {
			return err
		}
		if !bytes.Equal(contents, change.updated) {
			return errors.New("shell startup file changed before rollback")
		}
		installed = info
	}
	if err := m.targetUnchanged(change.path, installed); err != nil {
		return fmt.Errorf("preserve concurrent shell startup change: %w", err)
	}
	directory := filepath.Dir(change.path)
	if change.expected == nil {
		if change.temporaryPath == "" {
			return errors.New("rollback path for missing shell startup file is unavailable")
		}
		if err := m.files.rename(change.path, change.temporaryPath); err != nil {
			if unchangedErr := m.targetUnchanged(change.path, installed); unchangedErr != nil {
				return fmt.Errorf("preserve concurrent shell startup change: %w", unchangedErr)
			}
			return fmt.Errorf("restore missing shell startup file: %w", err)
		}
		change.restored = true
		if err := m.syncDirectory(directory); err != nil {
			return err
		}
		if err := m.files.remove(change.temporaryPath); err != nil {
			return fmt.Errorf("remove rolled-back shell startup temporary file: %w", err)
		}
		change.temporaryPath = ""
	} else if err := m.files.rename(change.backupPath, change.path); err != nil {
		if unchangedErr := m.targetUnchanged(change.path, installed); unchangedErr != nil {
			return fmt.Errorf("preserve concurrent shell startup change: %w", unchangedErr)
		}
		latest, inspectErr := m.files.lstat(change.path)
		if inspectErr != nil {
			return errors.Join(err, inspectErr)
		}
		if replaceErr := m.atomicReplace(change.path, change.original, change.updated, latest); replaceErr != nil {
			return errors.Join(err, replaceErr)
		}
		change.restored = true
		return errors.Join(err, m.syncDirectory(directory))
	} else {
		change.backupPath = ""
		change.restored = true
	}
	return m.syncDirectory(directory)
}

func (m *Manager) Remove(request Request) (Summary, error) {
	targets, err := m.targets(request, false)
	if err != nil {
		return Summary{}, err
	}
	summary := Summary{Results: make([]Result, 0, len(targets))}
	for _, target := range targets {
		if !target.selected {
			summary.Results = append(summary.Results, Result{
				Shell:   target.shell,
				Path:    target.path,
				Outcome: OutcomeSkipped,
			})
			continue
		}
		outcome, removeErr := m.removeFile(target.path, target.shell)
		result := Result{Shell: target.shell, Path: target.path, Outcome: outcome}
		if removeErr != nil {
			result.Outcome = OutcomeFailed
			result.Error = removeErr.Error()
		}
		summary.Results = append(summary.Results, result)
	}
	return summary, nil
}

type target struct {
	shell    Shell
	path     string
	selected bool
}

func (m *Manager) targets(request Request, includeInvoking bool) ([]target, error) {
	if request.Shell != "" {
		if !supportedShell(request.Shell) {
			return nil, fmt.Errorf("unsupported shell %q", request.Shell)
		}
		path, err := m.explicitPath(request)
		if err != nil {
			return nil, err
		}
		return []target{{shell: request.Shell, path: path, selected: true}}, nil
	}
	if request.RC != "" {
		return nil, errors.New("--rc requires exactly one shell")
	}
	if includeInvoking && !supportedShell(m.environment.Invocation.Shell) {
		return nil, errors.New("unable to detect invoking shell")
	}

	zshPath, err := m.defaultPath(ShellZsh, false)
	if err != nil {
		return nil, err
	}
	fishPath, err := m.defaultPath(ShellFish, false)
	if err != nil {
		return nil, err
	}
	bashProfile, err := m.cleanPath(filepath.Join(m.environment.Home, ".bash_profile"))
	if err != nil {
		return nil, err
	}
	bashRC, err := m.cleanPath(filepath.Join(m.environment.Home, ".bashrc"))
	if err != nil {
		return nil, err
	}

	targets := []target{
		{shell: ShellZsh, path: zshPath, selected: m.pathInUse(zshPath, ShellZsh, includeInvoking)},
		{shell: ShellFish, path: fishPath, selected: m.pathInUse(fishPath, ShellFish, includeInvoking)},
	}
	if !includeInvoking {
		return append(targets,
			target{shell: ShellBash, path: bashProfile, selected: m.pathExistsOrUnreadable(bashProfile)},
			target{shell: ShellBash, path: bashRC, selected: m.pathExistsOrUnreadable(bashRC)},
		), nil
	}

	bashTargets := make([]target, 0, 2)
	for _, path := range []string{bashProfile, bashRC} {
		selected := m.pathExistsOrUnreadable(path)
		if m.environment.Invocation.Shell == ShellBash {
			invokingPath := bashRC
			if m.environment.Invocation.Login {
				invokingPath = bashProfile
			}
			selected = selected || path == invokingPath
		}
		if selected {
			bashTargets = append(bashTargets, target{shell: ShellBash, path: path, selected: true})
		}
	}
	if len(bashTargets) == 0 {
		bashTargets = append(bashTargets, target{shell: ShellBash, path: bashRC})
	}
	return append(targets, bashTargets...), nil
}

func (m *Manager) pathInUse(path string, shell Shell, includeInvoking bool) bool {
	return m.pathExistsOrUnreadable(path) ||
		(includeInvoking && m.environment.Invocation.Shell == shell)
}

func (m *Manager) pathExistsOrUnreadable(path string) bool {
	_, err := m.files.lstat(path)
	return err == nil || !errors.Is(err, fs.ErrNotExist)
}

func (m *Manager) explicitPath(request Request) (string, error) {
	if request.RC != "" {
		return m.cleanPath(request.RC)
	}
	login := request.Shell == ShellBash &&
		m.environment.Invocation.Shell == ShellBash &&
		m.environment.Invocation.Login
	return m.defaultPath(request.Shell, login)
}

func (m *Manager) defaultPath(shell Shell, bashLogin bool) (string, error) {
	var path string
	switch shell {
	case ShellZsh:
		root := m.environment.ZDOTDir
		if root == "" {
			root = m.environment.Home
		}
		path = filepath.Join(root, ".zshrc")
	case ShellFish:
		root := m.environment.XDGConfigHome
		if root == "" {
			root = filepath.Join(m.environment.Home, ".config")
		}
		path = filepath.Join(root, "fish", "config.fish")
	case ShellBash:
		name := ".bashrc"
		if bashLogin {
			name = ".bash_profile"
		}
		path = filepath.Join(m.environment.Home, name)
	default:
		return "", fmt.Errorf("unsupported shell %q", shell)
	}
	return m.cleanPath(path)
}

func (m *Manager) cleanPath(path string) (string, error) {
	if path == "" {
		return "", errors.New("shell startup path is empty")
	}
	if strings.ContainsAny(path, "\r\n\x00") {
		return "", errors.New("shell startup path contains an unsafe character")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve shell startup path: %w", err)
	}
	return filepath.Clean(absolute), nil
}

func supportedShell(shell Shell) bool {
	return shell == ShellBash || shell == ShellFish || shell == ShellZsh
}

type blockState int

const (
	blockAbsent blockState = iota
	blockValid
	blockInvalid
)

type managedBlock struct {
	state          blockState
	start          int
	end            int
	version        int
	separatorAdded bool
	body           []byte
	modified       bool
	err            error
}

func (m *Manager) setupFile(path string, shell Shell) (Outcome, error) {
	if err := m.files.mkdirAll(filepath.Dir(path), 0o700); err != nil {
		return OutcomeFailed, fmt.Errorf("create shell startup directory: %w", err)
	}
	contents, info, err := m.readStartup(path)
	if errors.Is(err, fs.ErrNotExist) {
		contents, info = nil, nil
	} else if err != nil {
		return OutcomeFailed, err
	}
	block := inspectManagedBlock(contents)
	if block.state == blockInvalid {
		return OutcomeFailed, block.err
	}
	if block.state == blockValid {
		if err := m.validateManagedBlock(block, shell); err != nil {
			return OutcomeFailed, err
		}
		if block.version == managedVersion && bytes.Equal(block.body, m.managedBody(shell)) {
			return OutcomeUnchanged, nil
		}
		replacement := m.buildManagedBlock(shell, block.separatorAdded)
		updated := replaceBytes(contents, block.start, block.end, replacement)
		if err := m.atomicReplace(path, updated, contents, info); err != nil {
			return OutcomeFailed, err
		}
		return OutcomeConfigured, nil
	}

	separatorAdded := len(contents) > 0 && contents[len(contents)-1] != '\n'
	updated := append([]byte(nil), contents...)
	if separatorAdded {
		updated = append(updated, '\n')
	}
	updated = append(updated, m.buildManagedBlock(shell, separatorAdded)...)
	if err := m.atomicReplace(path, updated, contents, info); err != nil {
		return OutcomeFailed, err
	}
	return OutcomeConfigured, nil
}

func (m *Manager) removeFile(path string, shell Shell) (Outcome, error) {
	contents, info, err := m.readStartup(path)
	if errors.Is(err, fs.ErrNotExist) {
		return OutcomeAbsent, nil
	}
	if err != nil {
		return OutcomeFailed, err
	}
	block := inspectManagedBlock(contents)
	if block.state == blockAbsent {
		return OutcomeAbsent, nil
	}
	if block.state == blockInvalid {
		return OutcomeFailed, block.err
	}
	if err := m.validateManagedBlock(block, shell); err != nil {
		return OutcomeFailed, err
	}
	start := block.start
	if block.separatorAdded && block.end == len(contents) {
		if start == 0 || contents[start-1] != '\n' {
			return OutcomeFailed, errors.New("managed shell block separator is invalid")
		}
		start--
	}
	updated := replaceBytes(contents, start, block.end, nil)
	if err := m.atomicReplace(path, updated, contents, info); err != nil {
		return OutcomeFailed, err
	}
	return OutcomeRemoved, nil
}

func (m *Manager) readStartup(path string) ([]byte, fs.FileInfo, error) {
	info, err := m.files.lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil, fs.ErrNotExist
	}
	if err != nil {
		return nil, nil, fmt.Errorf("inspect shell startup file: %w", err)
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		return nil, nil, errors.New("shell startup file is a symlink")
	}
	if !info.Mode().IsRegular() {
		return nil, nil, errors.New("shell startup file is not a regular file")
	}
	if !m.ownsFile(info) {
		return nil, nil, errors.New("shell startup file is not owned by the current user")
	}
	contents, err := m.files.readFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("read shell startup file: %w", err)
	}
	latest, err := m.files.lstat(path)
	if err != nil {
		return nil, nil, fmt.Errorf("reinspect shell startup file: %w", err)
	}
	if !sameSnapshot(m.files.sameFile, info, latest) {
		return nil, nil, errors.New("shell startup file changed while it was read")
	}
	return contents, info, nil
}

func sameSnapshot(sameFile func(fs.FileInfo, fs.FileInfo) bool, before, after fs.FileInfo) bool {
	return sameFile(before, after) &&
		before.Mode() == after.Mode() &&
		before.Size() == after.Size() &&
		before.ModTime() == after.ModTime()
}

func (m *Manager) atomicReplace(path string, contents, original []byte, expected fs.FileInfo) error {
	mode := fs.FileMode(0o600)
	if expected != nil {
		mode = expected.Mode().Perm()
	}

	directoryPath := filepath.Dir(path)
	temporaryPath, err := m.writeTemporary(directoryPath, ".agentdeck-shell-*", contents, mode)
	if err != nil {
		return err
	}
	defer m.files.remove(temporaryPath)

	backupPath := ""
	if expected != nil {
		backupPath, err = m.writeTemporary(directoryPath, ".agentdeck-shell-backup-*", original, mode)
		if err != nil {
			return fmt.Errorf("prepare shell startup rollback: %w", err)
		}
		defer m.files.remove(backupPath)
	}
	if err := m.targetUnchanged(path, expected); err != nil {
		return err
	}
	if err := m.files.rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace shell startup file: %w", err)
	}
	if err := m.syncDirectory(directoryPath); err != nil {
		rollbackErr := m.rollbackReplace(path, directoryPath, backupPath, expected != nil)
		if rollbackErr != nil {
			return errors.Join(
				fmt.Errorf("finalize shell startup file: %w", err),
				fmt.Errorf("rollback shell startup file: %w", rollbackErr),
			)
		}
		return fmt.Errorf("finalize shell startup file: %w", err)
	}
	return nil
}

func (m *Manager) writeTemporary(directory, pattern string, contents []byte, mode fs.FileMode) (string, error) {
	temporary, err := m.files.createTemp(directory, pattern)
	if err != nil {
		return "", fmt.Errorf("create temporary shell startup file: %w", err)
	}
	temporaryPath := temporary.Name()
	succeeded := false
	defer func() {
		if !succeeded {
			_ = temporary.Close()
			_ = m.files.remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(mode); err != nil {
		return "", fmt.Errorf("set temporary shell startup mode: %w", err)
	}
	written, err := temporary.Write(contents)
	if err != nil {
		return "", fmt.Errorf("write temporary shell startup file: %w", err)
	}
	if written != len(contents) {
		return "", fmt.Errorf("write temporary shell startup file: %w", io.ErrShortWrite)
	}
	if err := temporary.Sync(); err != nil {
		return "", fmt.Errorf("flush temporary shell startup file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return "", fmt.Errorf("close temporary shell startup file: %w", err)
	}
	succeeded = true
	return temporaryPath, nil
}

func (m *Manager) syncDirectory(path string) error {
	directory, err := m.files.openDir(path)
	if err != nil {
		return fmt.Errorf("open shell startup directory: %w", err)
	}
	syncErr := directory.Sync()
	closeErr := directory.Close()
	if syncErr != nil {
		syncErr = fmt.Errorf("flush shell startup directory: %w", syncErr)
	}
	if closeErr != nil {
		closeErr = fmt.Errorf("close shell startup directory: %w", closeErr)
	}
	return errors.Join(syncErr, closeErr)
}

func (m *Manager) rollbackReplace(path, directory, backupPath string, existed bool) error {
	var restoreErr error
	if existed {
		restoreErr = m.files.rename(backupPath, path)
	} else {
		restoreErr = m.files.remove(path)
	}
	if restoreErr != nil {
		return restoreErr
	}
	return m.syncDirectory(directory)
}

func (m *Manager) targetUnchanged(path string, expected fs.FileInfo) error {
	latest, err := m.files.lstat(path)
	if expected == nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("reinspect shell startup file: %w", err)
		}
		return errors.New("shell startup file appeared during update")
	}
	if err != nil {
		return fmt.Errorf("reinspect shell startup file: %w", err)
	}
	if !sameSnapshot(m.files.sameFile, expected, latest) {
		return errors.New("shell startup file changed during update")
	}
	return nil
}

type lineSpan struct {
	start int
	end   int
	text  string
}

func inspectManagedBlock(contents []byte) managedBlock {
	lines := lineSpans(contents)
	var starts, ends []lineSpan
	for _, line := range lines {
		switch line.text {
		case startMarker:
			starts = append(starts, line)
		case endMarker:
			ends = append(ends, line)
		}
	}
	if len(starts) == 0 && len(ends) == 0 {
		return managedBlock{state: blockAbsent}
	}
	if len(starts) != 1 || len(ends) != 1 || starts[0].start >= ends[0].start {
		return invalidBlock("managed shell block markers are missing, duplicated, or out of order")
	}
	metadata := contents[starts[0].end:ends[0].start]
	hashEnd := bytes.IndexByte(metadata, '\n')
	if hashEnd < 0 {
		return invalidBlock("managed shell block hash record is missing")
	}
	hashLine := string(metadata[:hashEnd])
	const hashPrefix = "# managed-hash: "
	if !strings.HasPrefix(hashLine, hashPrefix) {
		return invalidBlock("managed shell block hash record is invalid")
	}
	expectedHash := strings.TrimPrefix(hashLine, hashPrefix)
	if len(expectedHash) != sha256.Size*2 {
		return invalidBlock("managed shell block hash record is invalid")
	}
	if _, err := hex.DecodeString(expectedHash); err != nil {
		return invalidBlock("managed shell block hash record is invalid")
	}
	payload := metadata[hashEnd+1:]
	actualHash := sha256.Sum256(payload)
	if !strings.EqualFold(expectedHash, hex.EncodeToString(actualHash[:])) {
		return managedBlock{
			state:    blockInvalid,
			modified: true,
			err:      errors.New("managed shell block was modified"),
		}
	}
	versionLine, payload := takeLine(payload)
	separatorLine, body := takeLine(payload)
	const versionPrefix = "# managed-version: "
	if !strings.HasPrefix(versionLine, versionPrefix) {
		return invalidBlock("managed shell block version record is invalid")
	}
	version, err := strconv.Atoi(strings.TrimPrefix(versionLine, versionPrefix))
	if err != nil || version < 1 || version > managedVersion {
		return invalidBlock("managed shell block version is unsupported")
	}
	const separatorPrefix = "# managed-separator-added: "
	if !strings.HasPrefix(separatorLine, separatorPrefix) {
		return invalidBlock("managed shell block separator record is invalid")
	}
	separator, err := strconv.ParseBool(strings.TrimPrefix(separatorLine, separatorPrefix))
	if err != nil {
		return invalidBlock("managed shell block separator record is invalid")
	}
	if separator && (starts[0].start == 0 || contents[starts[0].start-1] != '\n') {
		return invalidBlock("managed shell block separator is invalid")
	}
	return managedBlock{
		state:          blockValid,
		start:          starts[0].start,
		end:            ends[0].end,
		version:        version,
		separatorAdded: separator,
		body:           body,
	}
}

func invalidBlock(message string) managedBlock {
	return managedBlock{state: blockInvalid, err: errors.New(message)}
}

func (m *Manager) validateManagedBlock(block managedBlock, shell Shell) error {
	valid := false
	switch block.version {
	case 1:
		valid = bytes.Equal(block.body, legacyManagedBody(shell))
	case managedVersion:
		valid = validManagedBodyForShell(block.body, shell)
	}
	if !valid {
		return errors.New("managed shell block content does not match its shell")
	}
	return nil
}

func takeLine(contents []byte) (string, []byte) {
	end := bytes.IndexByte(contents, '\n')
	if end < 0 {
		return string(contents), nil
	}
	return string(contents[:end]), contents[end+1:]
}

func lineSpans(contents []byte) []lineSpan {
	if len(contents) == 0 {
		return nil
	}
	lines := make([]lineSpan, 0, bytes.Count(contents, []byte{'\n'})+1)
	for start := 0; start < len(contents); {
		next := bytes.IndexByte(contents[start:], '\n')
		end := len(contents)
		textEnd := end
		if next >= 0 {
			textEnd = start + next
			end = textEnd + 1
		}
		lines = append(lines, lineSpan{
			start: start,
			end:   end,
			text:  string(contents[start:textEnd]),
		})
		start = end
	}
	return lines
}

func (m *Manager) buildManagedBlock(shell Shell, separatorAdded bool) []byte {
	payload := []byte(fmt.Sprintf(
		"# managed-version: %d\n# managed-separator-added: %t\n%s",
		managedVersion,
		separatorAdded,
		m.managedBody(shell),
	))
	hash := sha256.Sum256(payload)
	return []byte(fmt.Sprintf(
		"%s\n# managed-hash: %s\n%s%s\n",
		startMarker,
		hex.EncodeToString(hash[:]),
		payload,
		endMarker,
	))
}

func (m *Manager) managedBody(shell Shell) []byte {
	agentdeck := "command agentdeck"
	if m.environment.StateRoot != "" {
		agentdeck += " --state-dir " + quoteShellArgument(m.environment.StateRoot)
	}
	return managedBodyForCommand(shell, agentdeck)
}

func legacyManagedBody(shell Shell) []byte {
	return managedBodyForCommand(shell, "command agentdeck")
}

func managedBodyForCommand(shell Shell, agentdeck string) []byte {
	if shell == ShellFish {
		return []byte("if type -q agentdeck\n" +
			"    " + agentdeck + " shell-init fish | source\n" +
			"end\n")
	}
	return []byte(fmt.Sprintf(
		"command -v agentdeck >/dev/null 2>&1 && eval \"$(%s shell-init %s)\"\n",
		agentdeck,
		shell,
	))
}

func validManagedBodyForShell(body []byte, shell Shell) bool {
	text := string(body)
	var prefix, suffix string
	if shell == ShellFish {
		prefix = "if type -q agentdeck\n    "
		suffix = " shell-init fish | source\nend\n"
	} else {
		prefix = "command -v agentdeck >/dev/null 2>&1 && eval \"$("
		suffix = fmt.Sprintf(" shell-init %s)\"\n", shell)
	}
	if !strings.HasPrefix(text, prefix) || !strings.HasSuffix(text, suffix) {
		return false
	}
	command := strings.TrimSuffix(strings.TrimPrefix(text, prefix), suffix)
	if command == "command agentdeck" {
		return true
	}
	const statePrefix = "command agentdeck --state-dir "
	if !strings.HasPrefix(command, statePrefix) {
		return false
	}
	quoted := strings.TrimPrefix(command, statePrefix)
	if len(quoted) < 2 || quoted[0] != '\'' || quoted[len(quoted)-1] != '\'' {
		return false
	}
	inner := quoted[1 : len(quoted)-1]
	decoded := strings.ReplaceAll(inner, "'\"'\"'", "'")
	return quoteShellArgument(decoded) == quoted
}

func quoteShellArgument(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func replaceBytes(contents []byte, start, end int, replacement []byte) []byte {
	updated := make([]byte, 0, len(contents)-(end-start)+len(replacement))
	updated = append(updated, contents[:start]...)
	updated = append(updated, replacement...)
	updated = append(updated, contents[end:]...)
	return updated
}

type processLookup func(int) (command string, parent int, err error)

func DetectInvokingShell() (Invocation, error) {
	return detectInvokingShell(os.Getppid(), lookupProcess)
}

func detectInvokingShell(parent int, lookup processLookup) (Invocation, error) {
	for count := 0; parent > 1 && count < 16; count++ {
		command, next, err := lookup(parent)
		if err != nil {
			return Invocation{}, fmt.Errorf("inspect parent process %d: %w", parent, err)
		}
		rawName := filepath.Base(strings.TrimSpace(command))
		name := strings.TrimPrefix(rawName, "-")
		shell := Shell(name)
		if supportedShell(shell) {
			return Invocation{Shell: shell, Login: shell == ShellBash && rawName == "-bash"}, nil
		}
		parent = next
	}
	return Invocation{}, errors.New("unable to detect invoking shell")
}

func lookupProcess(pid int) (string, int, error) {
	commandOutput, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "comm=").Output()
	if err != nil {
		return "", 0, err
	}
	parentOutput, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "ppid=").Output()
	if err != nil {
		return "", 0, err
	}
	parent, err := strconv.Atoi(strings.TrimSpace(string(parentOutput)))
	if err != nil {
		return "", 0, err
	}
	return strings.TrimSpace(string(commandOutput)), parent, nil
}
