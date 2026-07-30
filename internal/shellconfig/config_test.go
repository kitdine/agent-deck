package shellconfig

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetupIsIdempotentAndRemoveRestoresUnrelatedBytesAndMode(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".zshrc")
	original := []byte("export USER_SETTING=kept")
	if err := os.WriteFile(path, original, 0o640); err != nil {
		t.Fatal(err)
	}
	manager := New(Environment{Home: home, Invocation: Invocation{Shell: ShellZsh}})
	request := Request{Shell: ShellZsh, RC: path}

	first, err := manager.Setup(request)
	if err != nil {
		t.Fatal(err)
	}
	assertSingleOutcome(t, first, OutcomeConfigured)
	configured, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(configured, append(append([]byte(nil), original...), '\n')) {
		t.Fatalf("setup changed unrelated prefix:\n%s", configured)
	}
	if !bytes.Contains(configured, []byte(startMarker)) || !bytes.Contains(configured, []byte(endMarker)) {
		t.Fatalf("setup did not install managed block:\n%s", configured)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o640 {
		t.Fatalf("setup mode = %o, want 640", got)
	}

	second, err := manager.Setup(request)
	if err != nil {
		t.Fatal(err)
	}
	assertSingleOutcome(t, second, OutcomeUnchanged)
	afterSecond, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(afterSecond, configured) {
		t.Fatal("idempotent setup rewrote the startup file")
	}

	removed, err := manager.Remove(request)
	if err != nil {
		t.Fatal(err)
	}
	assertSingleOutcome(t, removed, OutcomeRemoved)
	restored, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(restored, original) {
		t.Fatalf("remove restored %q, want %q", restored, original)
	}
	info, err = os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o640 {
		t.Fatalf("remove mode = %o, want 640", got)
	}
}

func TestSetupUpgradesCompatibleManagedBlocksToRequestedStateRoot(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(t *testing.T, path string)
	}{
		{
			name: "version one",
			prepare: func(t *testing.T, path string) {
				t.Helper()
				payload := []byte(fmt.Sprintf(
					"# managed-version: 1\n# managed-separator-added: false\n%s",
					legacyManagedBody(ShellZsh),
				))
				hash := sha256.Sum256(payload)
				block := []byte(fmt.Sprintf(
					"%s\n# managed-hash: %s\n%s%s\n",
					startMarker,
					hex.EncodeToString(hash[:]),
					payload,
					endMarker,
				))
				if err := os.WriteFile(path, block, 0o600); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "different version two state root",
			prepare: func(t *testing.T, path string) {
				t.Helper()
				old := New(Environment{
					Home:      filepath.Dir(path),
					StateRoot: filepath.Join(filepath.Dir(path), "old-state"),
					Invocation: Invocation{
						Shell: ShellZsh,
					},
				})
				summary, err := old.Setup(Request{Shell: ShellZsh, RC: path})
				if err != nil {
					t.Fatal(err)
				}
				assertSingleOutcome(t, summary, OutcomeConfigured)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			path := filepath.Join(home, ".zshrc")
			test.prepare(t, path)
			stateRoot := filepath.Join(home, "state'custom")
			manager := New(Environment{
				Home:      home,
				StateRoot: stateRoot,
				Invocation: Invocation{
					Shell: ShellZsh,
				},
			})

			summary, err := manager.Setup(Request{Shell: ShellZsh, RC: path})
			if err != nil {
				t.Fatal(err)
			}
			assertSingleOutcome(t, summary, OutcomeConfigured)
			contents, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(contents, manager.buildManagedBlock(ShellZsh, false)) {
				t.Fatalf("upgraded managed block does not bind requested state root:\n%s", contents)
			}

			summary, err = manager.Setup(Request{Shell: ShellZsh, RC: path})
			if err != nil {
				t.Fatal(err)
			}
			assertSingleOutcome(t, summary, OutcomeUnchanged)
		})
	}
}

func TestSetupCreatesMissingStartupFileAndParentsPrivately(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".config", "fish", "config.fish")
	manager := New(Environment{Home: home, Invocation: Invocation{Shell: ShellFish}})
	summary, err := manager.Setup(Request{Shell: ShellFish, RC: path})
	if err != nil {
		t.Fatal(err)
	}
	assertSingleOutcome(t, summary, OutcomeConfigured)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("new startup file mode = %o, want 600", got)
	}
}

func TestSetupRefusesInvalidManagedBlocksWithoutChangingFiles(t *testing.T) {
	home := t.TempDir()
	manager := New(Environment{Home: home, Invocation: Invocation{Shell: ShellZsh}})
	valid := append([]byte("before\n"), manager.buildManagedBlock(ShellZsh, false)...)
	tests := map[string][]byte{
		"duplicate marker": append(append([]byte(nil), valid...), []byte(startMarker+"\n")...),
		"truncated":        bytes.Replace(valid, []byte(endMarker), nil, 1),
		"modified body":    bytes.Replace(valid, []byte("shell-init zsh"), []byte("shell-init bash"), 1),
		"invalid hash":     bytes.Replace(valid, []byte("# managed-hash: "), []byte("# managed-hash: z"), 1),
	}
	for name, contents := range tests {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(home, strings.ReplaceAll(name, " ", "-"))
			if err := os.WriteFile(path, contents, 0o600); err != nil {
				t.Fatal(err)
			}
			summary, err := manager.Setup(Request{Shell: ShellZsh, RC: path})
			if err != nil {
				t.Fatal(err)
			}
			assertSingleOutcome(t, summary, OutcomeFailed)
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(after, contents) {
				t.Fatal("refused setup changed startup file")
			}
			summary, err = manager.Remove(Request{Shell: ShellZsh, RC: path})
			if err != nil {
				t.Fatal(err)
			}
			assertSingleOutcome(t, summary, OutcomeFailed)
			after, err = os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(after, contents) {
				t.Fatal("refused remove changed startup file")
			}
		})
	}
}

func TestSetupRefusesSymlinkAndWrongOwnership(t *testing.T) {
	home := t.TempDir()
	target := filepath.Join(home, "target")
	if err := os.WriteFile(target, []byte("keep\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(home, ".zshrc")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	manager := New(Environment{Home: home, Invocation: Invocation{Shell: ShellZsh}})
	summary, err := manager.Setup(Request{Shell: ShellZsh, RC: link})
	if err != nil {
		t.Fatal(err)
	}
	assertSingleOutcome(t, summary, OutcomeFailed)
	contents, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "keep\n" {
		t.Fatalf("symlink target changed to %q", contents)
	}

	ownedPath := filepath.Join(home, "owned")
	if err := os.WriteFile(ownedPath, []byte("keep\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	manager = New(Environment{Home: home, Invocation: Invocation{Shell: ShellZsh}})
	manager.ownsFile = func(fs.FileInfo) bool { return false }
	summary, err = manager.Setup(Request{Shell: ShellZsh, RC: ownedPath})
	if err != nil {
		t.Fatal(err)
	}
	assertSingleOutcome(t, summary, OutcomeFailed)
}

func TestAtomicFailuresLeaveOriginalFileAndNoTemporaryArtifact(t *testing.T) {
	tests := []struct {
		name  string
		alter func(*Manager)
	}{
		{
			name: "write",
			alter: func(manager *Manager) {
				create := manager.files.createTemp
				manager.files.createTemp = func(dir, pattern string) (temporaryFile, error) {
					file, err := create(dir, pattern)
					if err != nil {
						return nil, err
					}
					return &failingTemporaryFile{temporaryFile: file, writeErr: errors.New("synthetic write failure")}, nil
				}
			},
		},
		{
			name: "sync",
			alter: func(manager *Manager) {
				create := manager.files.createTemp
				manager.files.createTemp = func(dir, pattern string) (temporaryFile, error) {
					file, err := create(dir, pattern)
					if err != nil {
						return nil, err
					}
					return &failingTemporaryFile{temporaryFile: file, syncErr: errors.New("synthetic sync failure")}, nil
				}
			},
		},
		{
			name: "rename",
			alter: func(manager *Manager) {
				manager.files.rename = func(string, string) error {
					return errors.New("synthetic rename failure")
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			path := filepath.Join(home, ".zshrc")
			original := []byte("keep\n")
			if err := os.WriteFile(path, original, 0o600); err != nil {
				t.Fatal(err)
			}
			manager := New(Environment{Home: home, Invocation: Invocation{Shell: ShellZsh}})
			test.alter(manager)
			summary, err := manager.Setup(Request{Shell: ShellZsh, RC: path})
			if err != nil {
				t.Fatal(err)
			}
			assertSingleOutcome(t, summary, OutcomeFailed)
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(after, original) {
				t.Fatalf("failed %s changed original file", test.name)
			}
			temporary, err := filepath.Glob(filepath.Join(home, ".agentdeck-shell-*"))
			if err != nil {
				t.Fatal(err)
			}
			if len(temporary) != 0 {
				t.Fatalf("failed %s left temporary files: %v", test.name, temporary)
			}
		})
	}
}

type failingTemporaryFile struct {
	temporaryFile
	writeErr error
	syncErr  error
}

func (f *failingTemporaryFile) Write(contents []byte) (int, error) {
	if f.writeErr != nil {
		return 0, f.writeErr
	}
	return f.temporaryFile.Write(contents)
}

func (f *failingTemporaryFile) Sync() error {
	if f.syncErr != nil {
		return f.syncErr
	}
	return f.temporaryFile.Sync()
}

func TestPostRenameFailuresRestoreOriginalStateAndRemoveTemporaryArtifacts(t *testing.T) {
	tests := []struct {
		name  string
		alter func(*Manager)
	}{
		{
			name: "open directory",
			alter: func(manager *Manager) {
				manager.files.openDir = func(string) (syncCloser, error) {
					return nil, errors.New("synthetic open directory failure")
				}
			},
		},
		{
			name: "sync directory",
			alter: func(manager *Manager) {
				open := manager.files.openDir
				manager.files.openDir = func(path string) (syncCloser, error) {
					directory, err := open(path)
					if err != nil {
						return nil, err
					}
					return &failingSyncCloser{
						syncCloser: directory,
						syncErr:    errors.New("synthetic directory sync failure"),
					}, nil
				}
			},
		},
	}

	for _, test := range tests {
		for _, existing := range []bool{true, false} {
			state := "missing"
			if existing {
				state = "existing"
			}
			t.Run(test.name+"/"+state, func(t *testing.T) {
				home := t.TempDir()
				path := filepath.Join(home, ".zshrc")
				original := []byte("keep\n")
				if existing {
					if err := os.WriteFile(path, original, 0o640); err != nil {
						t.Fatal(err)
					}
				}

				manager := New(Environment{Home: home, Invocation: Invocation{Shell: ShellZsh}})
				test.alter(manager)
				summary, err := manager.Setup(Request{Shell: ShellZsh, RC: path})
				if err != nil {
					t.Fatal(err)
				}
				assertSingleOutcome(t, summary, OutcomeFailed)

				after, err := os.ReadFile(path)
				if existing {
					if err != nil {
						t.Fatal(err)
					}
					if !bytes.Equal(after, original) {
						t.Fatalf("post-rename failure left bytes %q, want %q", after, original)
					}
				} else if !errors.Is(err, fs.ErrNotExist) {
					t.Fatalf("post-rename failure left missing target present: bytes=%q err=%v", after, err)
				}

				temporary, err := filepath.Glob(filepath.Join(home, ".agentdeck-shell-*"))
				if err != nil {
					t.Fatal(err)
				}
				if len(temporary) != 0 {
					t.Fatalf("post-rename failure left temporary files: %v", temporary)
				}
			})
		}
	}
}

type failingSyncCloser struct {
	syncCloser
	syncErr error
}

func (f *failingSyncCloser) Sync() error {
	return f.syncErr
}

func TestDefaultPathsAndParentShellDetectionMatchInstallerRules(t *testing.T) {
	home := t.TempDir()
	manager := New(Environment{
		Home:          home,
		ZDOTDir:       filepath.Join(home, "zdot"),
		XDGConfigHome: filepath.Join(home, "xdg"),
	})
	tests := []struct {
		shell Shell
		login bool
		want  string
	}{
		{shell: ShellZsh, want: filepath.Join(home, "zdot", ".zshrc")},
		{shell: ShellFish, want: filepath.Join(home, "xdg", "fish", "config.fish")},
		{shell: ShellBash, want: filepath.Join(home, ".bashrc")},
		{shell: ShellBash, login: true, want: filepath.Join(home, ".bash_profile")},
	}
	for _, test := range tests {
		got, err := manager.defaultPath(test.shell, test.login)
		if err != nil {
			t.Fatal(err)
		}
		if got != test.want {
			t.Errorf("defaultPath(%s, %t) = %q, want %q", test.shell, test.login, got, test.want)
		}
	}
	fallback := New(Environment{Home: home})
	for shell, want := range map[Shell]string{
		ShellZsh:  filepath.Join(home, ".zshrc"),
		ShellFish: filepath.Join(home, ".config", "fish", "config.fish"),
	} {
		got, err := fallback.defaultPath(shell, false)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("fallback defaultPath(%s) = %q, want %q", shell, got, want)
		}
	}

	processes := map[int]struct {
		command string
		parent  int
	}{
		50: {command: "/usr/bin/env", parent: 40},
		40: {command: "-bash", parent: 1},
	}
	invocation, err := detectInvokingShell(50, func(pid int) (string, int, error) {
		process, ok := processes[pid]
		if !ok {
			return "", 0, fs.ErrNotExist
		}
		return process.command, process.parent, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if invocation != (Invocation{Shell: ShellBash, Login: true}) {
		t.Fatalf("invocation = %#v", invocation)
	}
}

func TestNoArgumentSetupCreatesOnlyInvokingAndExistingShellFiles(t *testing.T) {
	home := t.TempDir()
	manager := New(Environment{Home: home, Invocation: Invocation{Shell: ShellZsh}})
	summary, err := manager.Setup(Request{})
	if err != nil {
		t.Fatal(err)
	}
	if len(summary.Results) != 3 {
		t.Fatalf("results = %#v", summary.Results)
	}
	for _, result := range summary.Results {
		want := OutcomeSkipped
		if result.Shell == ShellZsh {
			want = OutcomeConfigured
		}
		if result.Outcome != want {
			t.Errorf("%s outcome = %s, want %s", result.Shell, result.Outcome, want)
		}
	}
	if _, err := os.Stat(filepath.Join(home, ".zshrc")); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		filepath.Join(home, ".config", "fish", "config.fish"),
		filepath.Join(home, ".bashrc"),
		filepath.Join(home, ".bash_profile"),
	} {
		if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("unused startup file %s exists: %v", path, err)
		}
	}
}

func TestUnsafeCustomStartupPathIsRejectedBeforeMutation(t *testing.T) {
	home := t.TempDir()
	manager := New(Environment{Home: home, Invocation: Invocation{Shell: ShellZsh}})
	summary, err := manager.Setup(Request{Shell: ShellZsh, RC: filepath.Join(home, "bad\npath")})
	if err == nil || len(summary.Results) != 0 {
		t.Fatalf("Setup() = %#v, %v; want request error", summary, err)
	}
}

func TestMultiShellSetupContinuesAfterOneRefusal(t *testing.T) {
	home := t.TempDir()
	zshPath := filepath.Join(home, ".zshrc")
	if err := os.WriteFile(zshPath, []byte("zsh\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	fishPath := filepath.Join(home, ".config", "fish", "config.fish")
	if err := os.MkdirAll(filepath.Dir(fishPath), 0o700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(home, "fish-target")
	if err := os.WriteFile(target, []byte("fish\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, fishPath); err != nil {
		t.Fatal(err)
	}
	manager := New(Environment{Home: home, Invocation: Invocation{Shell: ShellBash}})
	summary, err := manager.Setup(Request{})
	if err != nil {
		t.Fatal(err)
	}
	outcomes := map[Shell]map[Outcome]int{}
	for _, result := range summary.Results {
		if outcomes[result.Shell] == nil {
			outcomes[result.Shell] = map[Outcome]int{}
		}
		outcomes[result.Shell][result.Outcome]++
	}
	if outcomes[ShellZsh][OutcomeConfigured] != 1 ||
		outcomes[ShellFish][OutcomeFailed] != 1 ||
		outcomes[ShellBash][OutcomeConfigured] != 1 ||
		!summary.HasFailures() {
		t.Fatalf("multi-shell outcomes = %#v", summary.Results)
	}
	for _, path := range []string{zshPath, filepath.Join(home, ".bashrc")} {
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(contents, []byte(startMarker)) {
			t.Fatalf("%s was not configured:\n%s", path, contents)
		}
	}
	contents, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "fish\n" {
		t.Fatalf("refused fish target changed to %q", contents)
	}
}

func TestSetupIfUnconfiguredPreparesEveryTargetBeforeCommit(t *testing.T) {
	home := t.TempDir()
	zshPath := filepath.Join(home, ".zshrc")
	bashPath := filepath.Join(home, ".bashrc")
	if err := os.WriteFile(zshPath, []byte("zsh-original\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bashPath, []byte("bash-original\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	manager := New(Environment{Home: home, Invocation: Invocation{Shell: ShellBash}})
	create := manager.files.createTemp
	replacements := 0
	manager.files.createTemp = func(dir, pattern string) (temporaryFile, error) {
		if pattern == ".agentdeck-shell-*" {
			replacements++
			if replacements == 2 {
				return nil, errors.New("synthetic later target preparation failure")
			}
		}
		return create(dir, pattern)
	}

	summary, err := manager.SetupIfUnconfigured(Request{})
	if err != nil {
		t.Fatalf("SetupIfUnconfigured returned request error: %v", err)
	}
	if !summary.HasFailures() {
		t.Fatalf("SetupIfUnconfigured summary = %#v, want failure", summary.Results)
	}
	for path, want := range map[string]string{
		zshPath:  "zsh-original\n",
		bashPath: "bash-original\n",
	} {
		contents, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if string(contents) != want {
			t.Fatalf("%s = %q, want %q", path, contents, want)
		}
	}
	assertNoShellTemporaryArtifacts(t, home)
}

func TestSetupIfUnconfiguredRollsBackCommittedMissingTarget(t *testing.T) {
	home := t.TempDir()
	zshPath := filepath.Join(home, ".zshrc")
	fishPath := filepath.Join(home, ".config", "fish", "config.fish")
	if err := os.MkdirAll(filepath.Dir(fishPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fishPath, []byte("fish-original\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	manager := New(Environment{Home: home, Invocation: Invocation{Shell: ShellZsh}})
	rename := manager.files.rename
	manager.files.rename = func(source, destination string) error {
		if destination == fishPath {
			return errors.New("synthetic later target commit failure")
		}
		return rename(source, destination)
	}

	summary, err := manager.SetupIfUnconfigured(Request{})
	if err == nil || !summary.HasFailures() {
		t.Fatalf("SetupIfUnconfigured = %#v, %v; want transaction failure", summary.Results, err)
	}
	if _, statErr := os.Lstat(zshPath); !errors.Is(statErr, fs.ErrNotExist) {
		t.Fatalf("missing zsh target not restored: %v", statErr)
	}
	contents, readErr := os.ReadFile(fishPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(contents) != "fish-original\n" {
		t.Fatalf("fish target = %q, want original", contents)
	}
	assertNoShellTemporaryArtifacts(t, home)
}

func TestSetupIfUnconfiguredReportsRollbackCleanupFailureAfterRestoringTargets(t *testing.T) {
	home := t.TempDir()
	zshPath := filepath.Join(home, ".zshrc")
	fishPath := filepath.Join(home, ".config", "fish", "config.fish")
	if err := os.MkdirAll(filepath.Dir(fishPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fishPath, []byte("fish-original\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	manager := New(Environment{Home: home, Invocation: Invocation{Shell: ShellZsh}})
	rename := manager.files.rename
	manager.files.rename = func(source, destination string) error {
		if destination == fishPath {
			return errors.New("synthetic later target commit failure")
		}
		return rename(source, destination)
	}
	remove := manager.files.remove
	rollbackAttempts := 0
	manager.files.remove = func(path string) error {
		if strings.Contains(filepath.Base(path), ".agentdeck-shell-") {
			rollbackAttempts++
			if rollbackAttempts == 1 {
				return errors.New("synthetic rollback cleanup failure")
			}
		}
		return remove(path)
	}

	summary, err := manager.SetupIfUnconfigured(Request{})
	if err == nil || !summary.HasFailures() ||
		!strings.Contains(err.Error(), "synthetic rollback cleanup failure") {
		t.Fatalf("SetupIfUnconfigured = %#v, %v; want surfaced rollback failure", summary.Results, err)
	}
	if rollbackAttempts == 0 {
		t.Fatal("rollback cleanup failure was not exercised")
	}
	if _, statErr := os.Lstat(zshPath); !errors.Is(statErr, fs.ErrNotExist) {
		t.Fatalf("missing zsh target was not restored before cleanup failure: %v", statErr)
	}
	assertNoShellTemporaryArtifacts(t, home)
}

func TestSetupIfUnconfiguredReportsRollbackReplacementFailureAfterFallbackRestore(t *testing.T) {
	home := t.TempDir()
	zshPath := filepath.Join(home, ".zshrc")
	fishPath := filepath.Join(home, ".config", "fish", "config.fish")
	const zshOriginal = "zsh-original\n"
	if err := os.WriteFile(zshPath, []byte(zshOriginal), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(fishPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fishPath, []byte("fish-original\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	manager := New(Environment{Home: home, Invocation: Invocation{Shell: ShellZsh}})
	rename := manager.files.rename
	manager.files.rename = func(source, destination string) error {
		if destination == fishPath {
			return errors.New("synthetic later target commit failure")
		}
		if destination == zshPath &&
			strings.HasPrefix(filepath.Base(source), ".agentdeck-shell-backup-") {
			return errors.New("synthetic rollback replacement failure")
		}
		return rename(source, destination)
	}

	summary, err := manager.SetupIfUnconfigured(Request{})
	if err == nil || !summary.HasFailures() ||
		!strings.Contains(err.Error(), "synthetic rollback replacement failure") {
		t.Fatalf("SetupIfUnconfigured = %#v, %v; want surfaced rollback replacement failure", summary.Results, err)
	}
	contents, readErr := os.ReadFile(zshPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(contents) != zshOriginal {
		t.Fatalf("zsh target = %q, want fallback-restored original", contents)
	}
	assertNoShellTemporaryArtifacts(t, home)
}

func TestSetupIfUnconfiguredPreservesConcurrentReplacement(t *testing.T) {
	home := t.TempDir()
	zshPath := filepath.Join(home, ".zshrc")
	fishPath := filepath.Join(home, ".config", "fish", "config.fish")
	if err := os.MkdirAll(filepath.Dir(fishPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fishPath, []byte("fish-original\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	manager := New(Environment{Home: home, Invocation: Invocation{Shell: ShellZsh}})
	rename := manager.files.rename
	manager.files.rename = func(source, destination string) error {
		if destination == zshPath {
			if err := rename(source, destination); err != nil {
				return err
			}
			replacement := filepath.Join(home, "concurrent-zshrc")
			if err := os.WriteFile(replacement, []byte("concurrent-user-change\n"), 0o600); err != nil {
				return err
			}
			return os.Rename(replacement, destination)
		}
		if destination == fishPath {
			return errors.New("synthetic later target commit failure")
		}
		return rename(source, destination)
	}

	summary, err := manager.SetupIfUnconfigured(Request{})
	if err == nil || !summary.HasFailures() ||
		!strings.Contains(err.Error(), "preserve concurrent shell startup change") {
		t.Fatalf("SetupIfUnconfigured = %#v, %v; want preserved concurrent change", summary.Results, err)
	}
	contents, readErr := os.ReadFile(zshPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(contents) != "concurrent-user-change\n" {
		t.Fatalf("zsh target = %q, want concurrent replacement", contents)
	}
	fishContents, readErr := os.ReadFile(fishPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(fishContents) != "fish-original\n" {
		t.Fatalf("fish target = %q, want original", fishContents)
	}
	assertNoShellTemporaryArtifacts(t, home)
}

func assertNoShellTemporaryArtifacts(t *testing.T, root string) {
	t.Helper()
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if strings.HasPrefix(entry.Name(), ".agentdeck-shell-") {
			t.Errorf("temporary shell artifact remains: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestNoArgumentRemoveClearsEveryConfiguredDefaultFile(t *testing.T) {
	home := t.TempDir()
	manager := New(Environment{Home: home})
	for _, request := range []Request{
		{Shell: ShellZsh},
		{Shell: ShellFish},
		{Shell: ShellBash, RC: filepath.Join(home, ".bash_profile")},
		{Shell: ShellBash, RC: filepath.Join(home, ".bashrc")},
	} {
		summary, err := manager.Setup(request)
		if err != nil {
			t.Fatal(err)
		}
		assertSingleOutcome(t, summary, OutcomeConfigured)
	}
	summary, err := manager.Remove(Request{})
	if err != nil {
		t.Fatal(err)
	}
	if summary.HasFailures() {
		t.Fatalf("remove failures: %#v", summary.Results)
	}
	removed := 0
	for _, result := range summary.Results {
		if result.Outcome == OutcomeRemoved {
			removed++
		}
	}
	if removed != 4 {
		t.Fatalf("removed %d files, want 4: %#v", removed, summary.Results)
	}
}

func TestCompletionAndIntegrationBlocksRemainByteIndependent(t *testing.T) {
	const completion = "# >>> agentdeck completion >>>\nsource /tmp/agentdeck-completion\n# <<< agentdeck completion <<<\n"
	t.Run("completion before integration", func(t *testing.T) {
		home := t.TempDir()
		path := filepath.Join(home, ".zshrc")
		original := []byte("before\n" + completion)
		if err := os.WriteFile(path, original, 0o600); err != nil {
			t.Fatal(err)
		}
		manager := New(Environment{Home: home, Invocation: Invocation{Shell: ShellZsh}})
		if _, err := manager.Setup(Request{Shell: ShellZsh, RC: path}); err != nil {
			t.Fatal(err)
		}
		if _, err := manager.Remove(Request{Shell: ShellZsh, RC: path}); err != nil {
			t.Fatal(err)
		}
		assertFileBytes(t, path, original)
	})
	t.Run("completion after integration", func(t *testing.T) {
		home := t.TempDir()
		path := filepath.Join(home, ".zshrc")
		original := []byte("before\n")
		if err := os.WriteFile(path, original, 0o600); err != nil {
			t.Fatal(err)
		}
		manager := New(Environment{Home: home, Invocation: Invocation{Shell: ShellZsh}})
		if _, err := manager.Setup(Request{Shell: ShellZsh, RC: path}); err != nil {
			t.Fatal(err)
		}
		file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.WriteString(completion); err != nil {
			file.Close()
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		if _, err := manager.Setup(Request{Shell: ShellZsh, RC: path}); err != nil {
			t.Fatal(err)
		}
		if _, err := manager.Remove(Request{Shell: ShellZsh, RC: path}); err != nil {
			t.Fatal(err)
		}
		assertFileBytes(t, path, append(original, []byte(completion)...))
	})
}

func TestManagedBlockPresenceGuardIsSilentOnlyWhenAgentDeckIsAbsent(t *testing.T) {
	for _, shell := range []Shell{ShellBash, ShellFish, ShellZsh} {
		t.Run(string(shell), func(t *testing.T) {
			shellPath, err := exec.LookPath(string(shell))
			if err != nil {
				t.Skipf("%s is not installed", shell)
			}
			home := t.TempDir()
			rc := filepath.Join(home, "rc")
			manager := New(Environment{Home: home, Invocation: Invocation{Shell: shell}})
			if _, err := manager.Setup(Request{Shell: shell, RC: rc}); err != nil {
				t.Fatal(err)
			}
			bin := filepath.Join(home, "bin")
			if err := os.Mkdir(bin, 0o700); err != nil {
				t.Fatal(err)
			}
			writeExecutable(t, filepath.Join(bin, "codex"), "#!/bin/sh\nprintf 'real-client\\n'\n")

			stdout, stderr, err := sourceAndRun(shellPath, shell, rc, bin)
			if err != nil {
				t.Fatalf("source without agentdeck: %v\nstderr=%s", err, stderr)
			}
			if stdout != "real-client\n" || stderr != "" {
				t.Fatalf("absent agentdeck stdout=%q stderr=%q", stdout, stderr)
			}

			writeExecutable(t, filepath.Join(bin, "agentdeck"), "#!/bin/sh\nprintf 'agentdeck exploded\\n' >&2\nexit 23\n")
			stdout, stderr, err = sourceAndRun(shellPath, shell, rc, bin)
			if err != nil {
				t.Fatalf("source with failing agentdeck: %v\nstderr=%s", err, stderr)
			}
			if stdout != "real-client\n" || !strings.Contains(stderr, "agentdeck exploded") {
				t.Fatalf("failing agentdeck stdout=%q stderr=%q", stdout, stderr)
			}
		})
	}
}

func sourceAndRun(shellPath string, shell Shell, rc, path string) (string, string, error) {
	var command *exec.Cmd
	switch shell {
	case ShellFish:
		command = exec.Command(shellPath, "--no-config", "-c", "source $argv[1]; codex", rc)
	case ShellZsh:
		command = exec.Command(shellPath, "-f", "-c", `source "$1"; codex`, "_", rc)
	default:
		command = exec.Command(shellPath, "--noprofile", "--norc", "-c", `source "$1"; codex`, "_", rc)
	}
	command.Env = []string{"HOME=" + filepath.Dir(rc), "PATH=" + path}
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	return stdout.String(), stderr.String(), err
}

func writeExecutable(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o700); err != nil {
		t.Fatal(err)
	}
}

func assertSingleOutcome(t *testing.T, summary Summary, want Outcome) {
	t.Helper()
	if len(summary.Results) != 1 || summary.Results[0].Outcome != want {
		t.Fatalf("results = %#v, want one %s", summary.Results, want)
	}
}

func assertFileBytes(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s = %q, want %q", path, got, want)
	}
}
