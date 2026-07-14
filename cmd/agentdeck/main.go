package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/kitdine/agent-deck/internal/backup"
	"github.com/kitdine/agent-deck/internal/doctor"
	"github.com/kitdine/agent-deck/internal/extension"
	"github.com/kitdine/agent-deck/internal/output"
	"github.com/kitdine/agent-deck/internal/platform"
	"github.com/kitdine/agent-deck/internal/provider"
	"github.com/kitdine/agent-deck/internal/session"
	"github.com/kitdine/agent-deck/internal/store"
	"github.com/kitdine/agent-deck/internal/usage"
	"github.com/kitdine/agent-deck/internal/watch"
)

var userHomeDir = os.UserHomeDir
var newSecretStore = func() platform.SecretStore { return platform.NewKeychainSecretStore("com.agentdeck.provider") }

type commandOptions struct {
	stateDir string
	format   string
	quiet    bool
	noColor  bool
	stdin    io.Reader
	stdout   io.Writer
}

type inputError struct {
	err error
}

func (e *inputError) Error() string {
	return e.err.Error()
}

func (e *inputError) Unwrap() error {
	return e.err
}

func main() {
	os.Exit(execute(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func execute(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	command, err := executeCommand(args, stdin, stdout)
	if err == nil {
		return 0
	}
	if jsonOutputRequested(args) {
		_ = json.NewEncoder(stderr).Encode(output.NewError(automationCommandName(command), errorCode(err), err.Error(), time.Now()))
	} else {
		_, _ = fmt.Fprintln(stderr, err)
	}
	return errorExitCode(err)
}

func jsonOutputRequested(args []string) bool {
	for index, arg := range args {
		if arg == "--format=json" || arg == "--format=ndjson" {
			return true
		}
		if arg == "--format" && index+1 < len(args) && (args[index+1] == "json" || args[index+1] == "ndjson") {
			return true
		}
	}
	return false
}

func automationCommandName(command *cobra.Command) string {
	return commandOutputName(command)
}

func commandOutputName(command *cobra.Command) string {
	if command == nil {
		return "agentdeck"
	}
	path := strings.TrimPrefix(command.CommandPath(), "agentdeck")
	path = strings.TrimSpace(path)
	if path == "" {
		return "agentdeck"
	}
	return strings.ReplaceAll(path, " ", ".")
}

func errorCode(err error) string {
	switch {
	case errors.Is(err, extension.ErrReadOnly):
		return extension.ErrReadOnly.Error()
	case errors.Is(err, store.ErrExtensionNotFound):
		return store.ErrExtensionNotFound.Error()
	case errors.Is(err, backup.ErrInvalidArchive):
		return backup.ErrInvalidArchive.Error()
	case errors.Is(err, backup.ErrTargetNotEmpty):
		return backup.ErrTargetNotEmpty.Error()
	case errors.Is(err, backup.ErrSecretConflict):
		return backup.ErrSecretConflict.Error()
	case errors.Is(err, backup.ErrDestinationExists):
		return backup.ErrDestinationExists.Error()
	case errors.Is(err, store.ErrStateBusy):
		return store.ErrStateBusy.Code
	case isInputError(err):
		return "invalid_argument"
	default:
		return "runtime_error"
	}
}

func errorExitCode(err error) int {
	if isInputError(err) {
		return 2
	}
	return 1
}

func isInputError(err error) bool {
	var target *inputError
	return errors.As(err, &target)
}

func exactArgs(count int) cobra.PositionalArgs {
	return func(_ *cobra.Command, args []string) error {
		if len(args) != count {
			return &inputError{err: fmt.Errorf("accepts %d arg(s), received %d", count, len(args))}
		}
		return nil
	}
}

func rangeArgs(minimum, maximum int) cobra.PositionalArgs {
	return func(_ *cobra.Command, args []string) error {
		if len(args) < minimum || len(args) > maximum {
			return &inputError{err: fmt.Errorf("accepts between %d and %d arg(s), received %d", minimum, maximum, len(args))}
		}
		return nil
	}
}

func run(args []string, stdin io.Reader, stdout io.Writer) error {
	_, err := executeCommand(args, stdin, stdout)
	return err
}

func executeCommand(args []string, stdin io.Reader, stdout io.Writer) (*cobra.Command, error) {
	root := newRootCommand(stdin, stdout)
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error { return &inputError{err: err} })
	root.SetArgs(args)
	command, err := root.ExecuteC()
	if err != nil && !isInputError(err) && isCobraSyntaxError(err) {
		err = &inputError{err: err}
	}
	return command, err
}

func isCobraSyntaxError(err error) bool {
	message := err.Error()
	return strings.HasPrefix(message, "unknown command ") ||
		strings.HasPrefix(message, "required flag(s) ") ||
		strings.HasPrefix(message, "unknown flag ") ||
		strings.Contains(message, "flag needs an argument") ||
		strings.HasPrefix(message, "invalid argument ")
}

func newRootCommand(stdin io.Reader, stdout io.Writer) *cobra.Command {
	opts := &commandOptions{format: "text", stdin: stdin, stdout: stdout}
	root := &cobra.Command{
		Use:           "agentdeck",
		Short:         "Manage local AI provider, usage, and session data",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(command *cobra.Command, _ []string) error {
			if opts.format == "ndjson" && command.Name() != "watch" {
				return &inputError{err: fmt.Errorf("ndjson format is supported only by watch")}
			}
			return nil
		},
	}
	root.SetIn(stdin)
	root.SetOut(stdout)
	root.SetErr(os.Stderr)
	flags := root.PersistentFlags()
	flags.StringVar(&opts.stateDir, "state-dir", "", "AgentDeck state directory")
	flags.StringVar(&opts.format, "format", "text", "Output format: text, json, or ndjson for watch")
	flags.BoolVar(&opts.noColor, "no-color", false, "Disable color output")
	flags.BoolVar(&opts.quiet, "quiet", false, "Suppress non-essential output")
	root.AddCommand(newProviderCommand(opts), newUsageCommand(opts), newSessionCommand(opts), newExtensionCommand(opts), newWatchCommand(opts), newBackupCommand(opts), newDoctorCommand(opts), newRunCommand(opts))
	wrapArgumentValidators(root)
	return root
}

func wrapArgumentValidators(command *cobra.Command) {
	if command.Args != nil {
		validate := command.Args
		command.Args = func(cmd *cobra.Command, args []string) error {
			err := validate(cmd, args)
			if err != nil && !isInputError(err) {
				return &inputError{err: err}
			}
			return err
		}
	}
	for _, child := range command.Commands() {
		wrapArgumentValidators(child)
	}
}

func (o *commandOptions) stateRoot() (string, error) {
	if o.format != "text" && o.format != "json" && o.format != "ndjson" {
		return "", &inputError{err: fmt.Errorf("invalid format %q", o.format)}
	}
	if o.stateDir != "" {
		return o.stateDir, nil
	}
	home, err := userHomeDir()
	if err != nil {
		return "", err
	}
	return platform.StateRoot("", home), nil
}

func (o *commandOptions) openStore(ctx context.Context) (*store.Store, string, error) {
	stateDir, err := o.stateRoot()
	if err != nil {
		return nil, "", err
	}
	database, err := store.Open(ctx, stateDir)
	return database, stateDir, err
}

func newProviderCommand(opts *commandOptions) *cobra.Command {
	providerCmd := &cobra.Command{Use: "provider", Short: "Manage providers"}
	withService := func(run func(context.Context, provider.Service, []string) (any, error)) func(*cobra.Command, []string) error {
		return func(cmd *cobra.Command, args []string) error {
			database, _, err := opts.openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer database.Close()
			data, err := run(cmd.Context(), provider.Service{Store: database, Secrets: newSecretStore()}, args)
			if err != nil {
				return err
			}
			return writeResult(opts.stdout, opts.format, commandOutputName(cmd), data)
		}
	}
	providerCmd.AddCommand(
		&cobra.Command{Use: "list", Args: cobra.NoArgs, RunE: withService(func(ctx context.Context, s provider.Service, _ []string) (any, error) { return s.List(ctx) })},
		&cobra.Command{Use: "status", Args: cobra.NoArgs, RunE: withService(func(ctx context.Context, s provider.Service, _ []string) (any, error) { return s.List(ctx) })},
		&cobra.Command{Use: "show <name>", Args: cobra.ExactArgs(1), RunE: withService(func(ctx context.Context, s provider.Service, args []string) (any, error) {
			statuses, err := s.List(ctx)
			if err != nil {
				return nil, err
			}
			for _, status := range statuses {
				if status.Definition.Name == args[0] {
					return status, nil
				}
			}
			return nil, fmt.Errorf("provider not found")
		})},
		&cobra.Command{Use: "add <name> <endpoint> <credential-ref> <multiplier> <codex|claude>", Args: cobra.ExactArgs(5), RunE: withService(func(ctx context.Context, s provider.Service, args []string) (any, error) {
			credential, err := readCredential(opts.stdin)
			if err != nil {
				return nil, err
			}
			return s.Add(ctx, provider.Definition{Name: args[0], Endpoint: args[1], CredentialRef: args[2], Multiplier: args[3], Clients: []provider.Client{provider.Client(args[4])}}, credential)
		})},
		&cobra.Command{Use: "edit <name> <endpoint> <credential-ref> <multiplier> <codex|claude>", Args: cobra.ExactArgs(5), RunE: withService(func(ctx context.Context, s provider.Service, args []string) (any, error) {
			return s.Edit(ctx, provider.Definition{Name: args[0], Endpoint: args[1], CredentialRef: args[2], Multiplier: args[3], Clients: []provider.Client{provider.Client(args[4])}}, "")
		})},
		&cobra.Command{Use: "remove <name> <credential-ref>", Args: cobra.ExactArgs(2), RunE: withService(func(ctx context.Context, s provider.Service, args []string) (any, error) {
			return nil, s.Remove(ctx, args[0], args[1])
		})},
		&cobra.Command{Use: "use <name> <codex|claude> <config-path> <redacted-backup-path>", Args: cobra.ExactArgs(4), RunE: withService(func(ctx context.Context, s provider.Service, args []string) (any, error) {
			return nil, s.Use(ctx, args[0], provider.Client(args[1]), args[2], args[3])
		})},
		&cobra.Command{Use: "recover", Args: cobra.NoArgs, RunE: withService(func(ctx context.Context, s provider.Service, _ []string) (any, error) { return s.Recover(ctx) })},
	)
	credentials := &cobra.Command{Use: "credential", Short: "Manage provider credentials"}
	credentials.AddCommand(
		&cobra.Command{Use: "list", Args: cobra.NoArgs, RunE: withService(func(ctx context.Context, s provider.Service, _ []string) (any, error) { return s.List(ctx) })},
		&cobra.Command{Use: "add <reference>", Args: cobra.ExactArgs(1), RunE: withService(func(ctx context.Context, s provider.Service, args []string) (any, error) {
			value, err := readCredential(opts.stdin)
			if err != nil {
				return nil, err
			}
			return nil, s.UpdateCredential(ctx, args[0], value)
		})},
		&cobra.Command{Use: "update <reference>", Args: cobra.ExactArgs(1), RunE: withService(func(ctx context.Context, s provider.Service, args []string) (any, error) {
			value, err := readCredential(opts.stdin)
			if err != nil {
				return nil, err
			}
			return nil, s.UpdateCredential(ctx, args[0], value)
		})},
		&cobra.Command{Use: "remove <reference>", Args: cobra.ExactArgs(1), RunE: withService(func(ctx context.Context, s provider.Service, args []string) (any, error) {
			return nil, s.RemoveCredential(ctx, args[0])
		})},
	)
	providerCmd.AddCommand(credentials)
	return providerCmd
}

func newSessionCommand(opts *commandOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "session", Short: "Search local sessions"}
	withSessions := func(run func(context.Context, *store.Store, string, []string) (any, error)) func(*cobra.Command, []string) error {
		return func(command *cobra.Command, args []string) error {
			stateDir, err := opts.stateRoot()
			if err != nil {
				return err
			}
			if err = platform.EnsureStateRoot(stateDir); err != nil {
				return err
			}
			lock, err := store.AcquireLock(command.Context(), stateDir, 5*time.Second)
			if err != nil {
				return err
			}
			defer lock.Release()
			sessions, err := store.OpenSessions(command.Context(), stateDir)
			if err != nil {
				return err
			}
			defer sessions.Close()
			home, err := userHomeDir()
			if err != nil {
				return err
			}
			data, err := run(command.Context(), sessions, home, args)
			if err != nil {
				return err
			}
			return writeResult(opts.stdout, opts.format, commandOutputName(command), data)
		}
	}
	cmd.AddCommand(
		&cobra.Command{Use: "scan", Args: cobra.NoArgs, RunE: withSessions(func(ctx context.Context, s *store.Store, home string, _ []string) (any, error) {
			return session.Scan(ctx, s.DB, home)
		})},
		&cobra.Command{Use: "list", Args: cobra.NoArgs, RunE: withSessions(func(ctx context.Context, s *store.Store, _ string, _ []string) (any, error) {
			return session.List(ctx, s.DB)
		})},
		&cobra.Command{Use: "search <query>", Args: cobra.ExactArgs(1), RunE: withSessions(func(ctx context.Context, s *store.Store, _ string, args []string) (any, error) {
			return session.Search(ctx, s.DB, args[0])
		})},
		&cobra.Command{Use: "show <codex|claude> <session-id>", Args: cobra.ExactArgs(2), RunE: withSessions(func(ctx context.Context, s *store.Store, _ string, args []string) (any, error) {
			return session.Show(ctx, s.DB, args[0], args[1])
		})},
		&cobra.Command{Use: "exclude <project|path|session|client> <value>", Args: cobra.ExactArgs(2), RunE: withSessions(func(ctx context.Context, s *store.Store, _ string, args []string) (any, error) {
			err := session.Exclude(ctx, s.DB, args[0], args[1])
			return map[string]any{"excluded": err == nil}, err
		})},
		&cobra.Command{Use: "rebuild", Args: cobra.NoArgs, RunE: withSessions(func(ctx context.Context, s *store.Store, home string, _ []string) (any, error) {
			return session.Rebuild(ctx, s.DB, home)
		})},
		newSessionPurgeCommand(opts),
	)
	return cmd
}

func newSessionPurgeCommand(opts *commandOptions) *cobra.Command {
	return &cobra.Command{Use: "purge-index", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		stateDir, err := opts.stateRoot()
		if err != nil {
			return err
		}
		if err = platform.EnsureStateRoot(stateDir); err != nil {
			return err
		}
		lock, err := store.AcquireLock(cmd.Context(), stateDir, 5*time.Second)
		if err != nil {
			return err
		}
		defer lock.Release()
		for _, suffix := range []string{"", "-wal", "-shm", "-journal"} {
			if err = os.Remove(filepath.Join(stateDir, "sessions.sqlite3") + suffix); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
		return writeResult(opts.stdout, opts.format, "session.purge-index", map[string]any{"purged": true})
	}}
}

func newExtensionCommand(opts *commandOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "extension", Short: "Inspect native extensions"}
	withExtensions := func(run func(context.Context, *store.Store, string, string, []string) (any, error)) func(*cobra.Command, []string) error {
		return func(command *cobra.Command, args []string) error {
			database, _, err := opts.openStore(command.Context())
			if err != nil {
				return err
			}
			defer database.Close()
			home, err := userHomeDir()
			if err != nil {
				return err
			}
			workdir, err := os.Getwd()
			if err != nil {
				return err
			}
			data, err := run(command.Context(), database, home, workdir, args)
			if err != nil {
				return err
			}
			return writeResult(opts.stdout, opts.format, commandOutputName(command), data)
		}
	}
	cmd.AddCommand(
		&cobra.Command{Use: "scan", Args: exactArgs(0), RunE: withExtensions(func(ctx context.Context, s *store.Store, home, workdir string, _ []string) (any, error) {
			return extension.Scan(ctx, s, home, workdir)
		})},
		&cobra.Command{Use: "list", Args: exactArgs(0), RunE: withExtensions(func(ctx context.Context, s *store.Store, _, _ string, _ []string) (any, error) {
			return extension.List(ctx, s)
		})},
		&cobra.Command{Use: "show <id>", Args: exactArgs(1), RunE: withExtensions(func(ctx context.Context, s *store.Store, _, _ string, args []string) (any, error) {
			return extension.Show(ctx, s, args[0])
		})},
		&cobra.Command{Use: "doctor", Args: exactArgs(0), RunE: withExtensions(func(ctx context.Context, s *store.Store, home, workdir string, _ []string) (any, error) {
			return extension.Doctor(ctx, s, home, workdir)
		})},
		&cobra.Command{Use: "adopt <id>", Args: exactArgs(1), RunE: withExtensions(func(ctx context.Context, s *store.Store, _, _ string, args []string) (any, error) {
			return extension.Adopt(ctx, s, args[0])
		})},
		&cobra.Command{Use: "release <id>", Args: exactArgs(1), RunE: withExtensions(func(ctx context.Context, s *store.Store, _, _ string, args []string) (any, error) {
			return nil, extension.Release(ctx, s, args[0])
		})},
		&cobra.Command{Use: "enable <id>", Args: exactArgs(1), RunE: withExtensions(func(ctx context.Context, s *store.Store, _, _ string, args []string) (any, error) {
			return nil, extension.SetEnabled(ctx, s, args[0], true)
		})},
		&cobra.Command{Use: "disable <id>", Args: exactArgs(1), RunE: withExtensions(func(ctx context.Context, s *store.Store, _, _ string, args []string) (any, error) {
			return nil, extension.SetEnabled(ctx, s, args[0], false)
		})},
	)
	return cmd
}

func newWatchCommand(opts *commandOptions) *cobra.Command {
	var interval time.Duration
	command := &cobra.Command{Use: "watch", Short: "Watch local sources in the foreground", Args: exactArgs(0), RunE: func(command *cobra.Command, _ []string) error {
		if opts.format == "json" {
			return &inputError{err: fmt.Errorf("watch requires text or ndjson format")}
		}
		stateDir, err := opts.stateRoot()
		if err != nil {
			return err
		}
		home, err := userHomeDir()
		if err != nil {
			return err
		}
		workdir, err := os.Getwd()
		if err != nil {
			return err
		}
		sessionRoots := []string{filepath.Join(home, ".codex", "sessions"), filepath.Join(home, ".codex", "archived_sessions"), filepath.Join(home, ".claude", "projects")}
		extensionRoots := []string{
			filepath.Join(home, ".codex", "config.toml"), filepath.Join(home, ".codex", "skills"), filepath.Join(home, ".codex", "plugins", "cache"),
			filepath.Join(home, ".claude.json"), filepath.Join(home, ".claude", "skills"), filepath.Join(home, ".claude", "plugins", "installed_plugins.json"),
			filepath.Join(workdir, ".codex", "config.toml"), filepath.Join(workdir, ".codex", "skills"), filepath.Join(workdir, ".codex", "plugins"),
			filepath.Join(workdir, ".claude", "skills"), filepath.Join(workdir, ".mcp.json"),
		}
		fingerprint := func(roots []string) func(context.Context) (string, error) {
			return func(context.Context) (string, error) { return watch.FingerprintRoots(roots...) }
		}
		initial, err := loadWatchFingerprints(command.Context(), stateDir)
		if err != nil {
			return err
		}
		var database, sessions *store.Store
		openForScan := func(ctx context.Context) error {
			if database != nil {
				return nil
			}
			database, err = store.Open(ctx, stateDir)
			if err != nil {
				return err
			}
			sessions, err = store.OpenSessions(ctx, stateDir)
			if err != nil {
				closeErr := database.Close()
				database = nil
				if closeErr != nil {
					return errors.Join(err, closeErr)
				}
				return err
			}
			return nil
		}
		defer func() {
			if sessions != nil {
				_ = sessions.Close()
			}
			if database != nil {
				_ = database.Close()
			}
		}()
		service := watch.Service{
			InitialFingerprints: initial,
			Sources: watch.SourceSet{
				{Domain: "usage", Snapshot: fingerprint(sessionRoots), Scan: func(ctx context.Context) (int, error) {
					if err := openForScan(ctx); err != nil {
						return 0, err
					}
					usageService := usage.New(database, home)
					result, err := usageService.Scan(ctx)
					return result["imported"] + result["replaced"], err
				}},
				{Domain: "session", Snapshot: fingerprint(sessionRoots), Scan: func(ctx context.Context) (int, error) {
					if err := openForScan(ctx); err != nil {
						return 0, err
					}
					result, err := session.Scan(ctx, sessions.DB, home)
					return result.Documents, err
				}},
				{Domain: "extension", Snapshot: fingerprint(extensionRoots), Scan: func(ctx context.Context) (int, error) {
					if err := openForScan(ctx); err != nil {
						return 0, err
					}
					result, err := extension.Scan(ctx, database, home, workdir)
					return result.Found, err
				}},
			},
			Lock: func(ctx context.Context) (func() error, error) {
				lock, err := store.AcquireScanLock(ctx, stateDir, 0)
				if err != nil {
					return nil, err
				}
				return lock.Release, nil
			},
			PersistFingerprint: func(ctx context.Context, domain, value string) error {
				if err := openForScan(ctx); err != nil {
					return err
				}
				return database.SetSetting(ctx, "watch.fingerprint."+domain, value)
			},
		}
		ctx, stop := signal.NotifyContext(command.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		encoder := json.NewEncoder(opts.stdout)
		return service.Run(ctx, interval, func(event watch.Event) error {
			if opts.format == "ndjson" {
				return encoder.Encode(event)
			}
			_, err := fmt.Fprintf(opts.stdout, "%s domain=%s changes=%d skipped=%t reason=%s\n", event.Type, event.Domain, event.Changes, event.Skipped, event.Reason)
			return err
		})
	}}
	command.Flags().DurationVar(&interval, "interval", time.Minute, "Polling interval")
	return command
}

func loadWatchFingerprints(ctx context.Context, stateDir string) (map[string]string, error) {
	path := filepath.Join(stateDir, "agentdeck.sqlite3")
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return map[string]string{}, nil
	} else if err != nil {
		return nil, err
	}
	database, err := store.OpenReadOnly(ctx, stateDir)
	if err != nil {
		return nil, err
	}
	defer database.Close()
	fingerprints := make(map[string]string)
	for _, domain := range []string{"usage", "session", "extension"} {
		if value, found, settingErr := database.Setting(ctx, "watch.fingerprint."+domain); settingErr != nil {
			return nil, settingErr
		} else if found {
			fingerprints[domain] = value
		}
	}
	return fingerprints, nil
}

func newBackupCommand(opts *commandOptions) *cobra.Command {
	command := &cobra.Command{Use: "backup", Short: "Manage encrypted portable backups"}
	var includeSessions bool
	create := &cobra.Command{Use: "create [path]", Args: rangeArgs(0, 1), RunE: func(command *cobra.Command, args []string) error {
		database, stateDir, err := opts.openStore(command.Context())
		if err != nil {
			return err
		}
		defer database.Close()
		passphrase, err := readPassphrase(opts.stdin)
		if err != nil {
			return err
		}
		destination := ""
		if len(args) == 1 {
			destination = args[0]
		} else {
			destination = filepath.Join(stateDir, "backups", "portable", time.Now().UTC().Format("20060102T150405Z")+".adb")
		}
		manifest, err := (backup.Service{Core: database, StateRoot: stateDir, Secrets: newSecretStore(), Version: "dev"}).Create(command.Context(), destination, passphrase, includeSessions)
		if err != nil {
			return err
		}
		return writeResult(opts.stdout, opts.format, "backup.create", map[string]any{"path": destination, "manifest": manifest})
	}}
	create.Flags().BoolVar(&includeSessions, "include-sessions", false, "Include the rebuildable session index")
	list := &cobra.Command{Use: "list", Args: exactArgs(0), RunE: func(command *cobra.Command, _ []string) error {
		stateDir, err := opts.stateRoot()
		if err != nil {
			return err
		}
		values, err := backup.List(filepath.Join(stateDir, "backups", "portable"))
		if err != nil {
			return err
		}
		return writeResult(opts.stdout, opts.format, "backup.list", values)
	}}
	inspect := &cobra.Command{Use: "inspect <path>", Args: exactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		passphrase, err := readPassphrase(opts.stdin)
		if err != nil {
			return err
		}
		manifest, err := (backup.Service{}).Inspect(args[0], passphrase)
		if err != nil {
			return err
		}
		return writeResult(opts.stdout, opts.format, "backup.inspect", manifest)
	}}
	restore := &cobra.Command{Use: "restore <path>", Args: exactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		target, err := opts.stateRoot()
		if err != nil {
			return err
		}
		passphrase, err := readPassphrase(opts.stdin)
		if err != nil {
			return err
		}
		manifest, err := backup.Restore(command.Context(), args[0], target, passphrase, newSecretStore())
		if err != nil {
			return err
		}
		return writeResult(opts.stdout, opts.format, "backup.restore", manifest)
	}}
	command.AddCommand(create, list, inspect, restore)
	return command
}

func newDoctorCommand(opts *commandOptions) *cobra.Command {
	var full bool
	command := &cobra.Command{Use: "doctor", Short: "Run read-only diagnostics", Args: exactArgs(0), RunE: func(command *cobra.Command, _ []string) error {
		stateDir, err := opts.stateRoot()
		if err != nil {
			return err
		}
		home, err := userHomeDir()
		if err != nil {
			return err
		}
		workdir, err := os.Getwd()
		if err != nil {
			return err
		}
		report, err := (doctor.Service{StateRoot: stateDir, Home: home, Workdir: workdir, Secrets: newSecretStore()}).Check(command.Context(), full)
		if err != nil {
			return err
		}
		return writeResult(opts.stdout, opts.format, "doctor", report)
	}}
	command.Flags().BoolVar(&full, "full", false, "Run full integrity and source checks")
	return command
}

func newUsageCommand(opts *commandOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "usage", Short: "Inspect usage and pricing"}
	withUsage := func(run func(context.Context, *usage.Service, *store.Store, []string) (any, bool, []string, error)) func(*cobra.Command, []string) error {
		return func(command *cobra.Command, args []string) error {
			database, _, err := opts.openStore(command.Context())
			if err != nil {
				return err
			}
			defer database.Close()
			home, err := userHomeDir()
			if err != nil {
				return err
			}
			data, partial, warnings, err := run(command.Context(), usage.New(database, home), database, args)
			if err != nil {
				return err
			}
			return writeEnvelope(opts.stdout, opts.format, commandOutputName(command), data, partial, warnings)
		}
	}
	cmd.AddCommand(
		&cobra.Command{Use: "scan", Args: cobra.NoArgs, RunE: withUsage(func(ctx context.Context, s *usage.Service, _ *store.Store, _ []string) (any, bool, []string, error) {
			data, err := s.Scan(ctx)
			return data, false, nil, err
		})},
		&cobra.Command{Use: "summary", Args: cobra.NoArgs, RunE: withUsage(func(ctx context.Context, s *usage.Service, _ *store.Store, _ []string) (any, bool, []string, error) {
			_, scanErr := s.Scan(ctx)
			data, err := s.Summary(ctx)
			return data, scanErr != nil, map[bool][]string{true: {"scan_incomplete"}}[scanErr != nil], err
		})},
		&cobra.Command{Use: "sessions", Args: cobra.NoArgs, RunE: withUsage(func(ctx context.Context, s *usage.Service, _ *store.Store, _ []string) (any, bool, []string, error) {
			data, err := s.Sessions(ctx)
			return data, false, nil, err
		})},
		&cobra.Command{Use: "diagnose", Args: cobra.NoArgs, RunE: withUsage(func(ctx context.Context, s *usage.Service, _ *store.Store, _ []string) (any, bool, []string, error) {
			data, err := s.Diagnose(ctx)
			return data, false, nil, err
		})},
		&cobra.Command{Use: "rebuild", Args: cobra.NoArgs, RunE: withUsage(func(ctx context.Context, s *usage.Service, database *store.Store, _ []string) (any, bool, []string, error) {
			if _, err := database.Exec(ctx, "DELETE FROM usage_events; DELETE FROM usage_sessions; DELETE FROM usage_source_files"); err != nil {
				return nil, false, nil, err
			}
			data, err := s.Scan(ctx)
			return data, false, nil, err
		})},
		newPriceCommand(opts, withUsage),
	)
	return cmd
}

func newPriceCommand(opts *commandOptions, withUsage func(func(context.Context, *usage.Service, *store.Store, []string) (any, bool, []string, error)) func(*cobra.Command, []string) error) *cobra.Command {
	price := &cobra.Command{Use: "price", Short: "Manage price catalogs"}
	price.AddCommand(
		&cobra.Command{Use: "history", Args: cobra.NoArgs, RunE: withUsage(func(ctx context.Context, s *usage.Service, _ *store.Store, _ []string) (any, bool, []string, error) {
			data, err := s.PriceHistory(ctx)
			return data, false, nil, err
		})},
		&cobra.Command{Use: "status", Args: cobra.NoArgs, RunE: withUsage(func(ctx context.Context, s *usage.Service, _ *store.Store, _ []string) (any, bool, []string, error) {
			data, err := s.PriceStatus(ctx)
			return data, false, nil, err
		})},
	)
	var update *cobra.Command
	update = &cobra.Command{Use: "update", Args: cobra.NoArgs, RunE: withUsage(func(ctx context.Context, s *usage.Service, _ *store.Store, _ []string) (any, bool, []string, error) {
		url, _ := update.Flags().GetString("url")
		commit, _ := update.Flags().GetString("commit")
		data, err := s.UpdateLiteLLM(ctx, url, commit, usage.PriceHTTPClient())
		return data, false, nil, err
	})}
	update.Flags().String("url", "", "Pinned LiteLLM catalog URL")
	update.Flags().String("commit", "", "Pinned LiteLLM commit SHA")
	_ = update.MarkFlagRequired("url")
	_ = update.MarkFlagRequired("commit")
	price.AddCommand(update)
	var override *cobra.Command
	override = &cobra.Command{Use: "override", Args: cobra.NoArgs, RunE: withUsage(func(ctx context.Context, s *usage.Service, _ *store.Store, _ []string) (any, bool, []string, error) {
		file, _ := override.Flags().GetString("file")
		contents, err := os.ReadFile(file)
		if err != nil {
			return nil, false, nil, err
		}
		var values []usage.OfficialOverride
		if err = json.Unmarshal(contents, &values); err != nil {
			return nil, false, nil, err
		}
		err = s.ImportOfficialOverrides(ctx, values)
		return map[string]any{"overrides": len(values)}, false, nil, err
	})}
	override.Flags().String("file", "", "Official component override JSON")
	_ = override.MarkFlagRequired("file")
	price.AddCommand(override)
	return price
}

func newRunCommand(opts *commandOptions) *cobra.Command {
	return &cobra.Command{Use: "run <codex|claude> -- <client arguments>", Args: func(cmd *cobra.Command, args []string) error {
		if len(args) < 2 || cmd.ArgsLenAtDash() != 1 {
			return &inputError{err: fmt.Errorf("usage: agentdeck run <codex|claude> -- <client arguments>")}
		}
		return nil
	}, RunE: func(cmd *cobra.Command, args []string) error {
		if args[0] != "codex" && args[0] != "claude" {
			return &inputError{err: fmt.Errorf("usage: agentdeck run <codex|claude> -- <client arguments>")}
		}
		database, _, err := opts.openStore(cmd.Context())
		if err != nil {
			return err
		}
		defer database.Close()
		service := usage.New(database, "")
		runID, start, err := service.StartRun(cmd.Context(), args[0], 0)
		if err != nil {
			return err
		}
		child := exec.CommandContext(cmd.Context(), args[0], args[1:]...)
		child.Stdin, child.Stdout, child.Stderr = os.Stdin, opts.stdout, os.Stderr
		if err = child.Start(); err != nil {
			finishErr := service.FailRun(context.WithoutCancel(cmd.Context()), runID, "client_start_failed")
			return errors.Join(err, finishErr)
		}
		pidErr := service.SetRunPID(cmd.Context(), runID, child.Process.Pid)
		waitErr := child.Wait()
		cleanupCtx := context.WithoutCancel(cmd.Context())
		if lifecycleErr := errors.Join(pidErr, cmd.Context().Err()); lifecycleErr != nil {
			finishErr := service.FailRun(cleanupCtx, runID, "wrapper_cleanup_failed")
			return errors.Join(lifecycleErr, waitErr, finishErr)
		}
		home, homeErr := userHomeDir()
		if homeErr != nil {
			finishErr := service.FailRun(cleanupCtx, runID, "wrapper_cleanup_failed")
			return errors.Join(homeErr, waitErr, finishErr)
		}
		service.Home = home
		if _, err = service.Scan(cleanupCtx); err != nil {
			finishErr := service.FailRun(cleanupCtx, runID, "wrapper_cleanup_failed")
			return errors.Join(err, waitErr, finishErr)
		}
		if err = service.EndRun(cleanupCtx, runID, args[0], start); err != nil {
			finishErr := service.FailRun(cleanupCtx, runID, "wrapper_cleanup_failed")
			return errors.Join(err, waitErr, finishErr)
		}
		if waitErr != nil {
			return fmt.Errorf("%s exited: %w", args[0], waitErr)
		}
		exact, reason, err := service.RunStatus(cmd.Context(), runID)
		if err != nil {
			return err
		}
		if opts.format == "json" {
			return writeResult(opts.stdout, opts.format, "run."+args[0], map[string]any{"run_id": runID, "exact": exact, "attribution": map[bool]string{true: "exact", false: "estimated"}[exact], "reason": reason})
		}
		return nil
	}}
}

func writeResult(w io.Writer, format, command string, data any) error {
	if format == "json" {
		return json.NewEncoder(w).Encode(output.New(command, data, time.Now()))
	}
	if format == "ndjson" {
		return &inputError{err: fmt.Errorf("ndjson format is supported only by watch")}
	}
	if data != nil {
		return json.NewEncoder(w).Encode(data)
	}
	return nil
}
func writeEnvelope(w io.Writer, format, command string, data any, partial bool, warnings []string) error {
	if format == "json" {
		if warnings == nil {
			warnings = []string{}
		}
		envelope := output.New(command, data, time.Now())
		envelope.Partial, envelope.Warnings = partial, warnings
		return json.NewEncoder(w).Encode(envelope)
	}
	return renderUsageText(w, command, data)
}
func renderUsageText(w io.Writer, command string, data any) error {
	switch v := data.(type) {
	case usage.Summary:
		_, err := fmt.Fprintf(w, "events: %d\ntokens: %v\ncatalog base cost: %v\nprovider cost: %v\nwarnings: %v\nunpriced: %v\n", v.Counts["events"], v.Tokens, v.CatalogBaseCost, v.ProviderCost, v.Warnings, v.Unpriced)
		return err
	case []usage.SessionSummary:
		for _, x := range v {
			if _, err := fmt.Fprintf(w, "%s %s %s..%s tokens=%v base=%v provider=%v warnings=%v unpriced=%v\n", x.Client, x.SessionID, x.FirstAt, x.LastAt, x.Tokens, x.CatalogBaseCost, x.ProviderCost, x.Warnings, x.Unpriced); err != nil {
				return err
			}
		}
		return nil
	default:
		return json.NewEncoder(w).Encode(data)
	}
}
func readCredential(reader io.Reader) (string, error) {
	if file, ok := reader.(*os.File); ok {
		info, err := file.Stat()
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeCharDevice != 0 {
			return "", fmt.Errorf("credential must be supplied through non-interactive stdin")
		}
	}
	contents, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	value := strings.TrimSpace(string(contents))
	if value == "" {
		return "", fmt.Errorf("credential is empty")
	}
	return value, nil
}

func readPassphrase(reader io.Reader) (string, error) {
	if file, ok := reader.(*os.File); ok {
		info, err := file.Stat()
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeCharDevice != 0 {
			value, err := term.ReadPassword(int(file.Fd()))
			if err != nil {
				return "", err
			}
			passphrase := strings.TrimRight(string(value), "\r\n")
			if passphrase == "" {
				return "", &inputError{err: fmt.Errorf("passphrase is empty")}
			}
			return passphrase, nil
		}
	}
	value, err := bufio.NewReader(reader).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	passphrase := strings.TrimRight(value, "\r\n")
	if passphrase == "" {
		return "", &inputError{err: fmt.Errorf("passphrase is empty")}
	}
	return passphrase, nil
}
