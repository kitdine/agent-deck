package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/kitdine/agent-deck/internal/output"
	"github.com/kitdine/agent-deck/internal/platform"
	"github.com/kitdine/agent-deck/internal/provider"
	"github.com/kitdine/agent-deck/internal/session"
	"github.com/kitdine/agent-deck/internal/store"
	"github.com/kitdine/agent-deck/internal/usage"
)

var userHomeDir = os.UserHomeDir

type commandOptions struct {
	stateDir string
	format   string
	quiet    bool
	noColor  bool
	stdin    io.Reader
	stdout   io.Writer
}

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stdin io.Reader, stdout io.Writer) error {
	root := newRootCommand(stdin, stdout)
	root.SetArgs(args)
	return root.Execute()
}

func newRootCommand(stdin io.Reader, stdout io.Writer) *cobra.Command {
	opts := &commandOptions{format: "text", stdin: stdin, stdout: stdout}
	root := &cobra.Command{
		Use:           "agentdeck",
		Short:         "Manage local AI provider, usage, and session data",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetIn(stdin)
	root.SetOut(stdout)
	root.SetErr(os.Stderr)
	flags := root.PersistentFlags()
	flags.StringVar(&opts.stateDir, "state-dir", "", "AgentDeck state directory")
	flags.StringVar(&opts.format, "format", "text", "Output format: text or json")
	flags.BoolVar(&opts.noColor, "no-color", false, "Disable color output")
	flags.BoolVar(&opts.quiet, "quiet", false, "Suppress non-essential output")
	root.AddCommand(newProviderCommand(opts), newUsageCommand(opts), newSessionCommand(opts), newRunCommand(opts))
	return root
}

func (o *commandOptions) stateRoot() (string, error) {
	if o.format != "text" && o.format != "json" {
		return "", fmt.Errorf("invalid format %q", o.format)
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
			data, err := run(cmd.Context(), provider.Service{Store: database, Secrets: platform.NewKeychainSecretStore("com.agentdeck.provider")}, args)
			if err != nil {
				return err
			}
			return writeResult(opts.stdout, opts.format, "provider."+cmd.Name(), data)
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
			return writeResult(opts.stdout, opts.format, "session."+command.Name(), data)
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
			return writeEnvelope(opts.stdout, opts.format, "usage."+command.Name(), data, partial, warnings)
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
		data, err := s.UpdateLiteLLM(ctx, url, commit, nil)
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
	return &cobra.Command{Use: "run <codex|claude> -- <client arguments>", Args: cobra.MinimumNArgs(2), DisableFlagParsing: true, RunE: func(cmd *cobra.Command, args []string) error {
		if (args[0] != "codex" && args[0] != "claude") || args[1] != "--" {
			return fmt.Errorf("usage: agentdeck run <codex|claude> -- <client arguments>")
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
		child := exec.CommandContext(cmd.Context(), args[0], args[2:]...)
		child.Stdin, child.Stdout, child.Stderr = os.Stdin, opts.stdout, os.Stderr
		if err = child.Start(); err != nil {
			_ = service.EndRun(cmd.Context(), runID, args[0], start)
			return err
		}
		if err = service.SetRunPID(cmd.Context(), runID, child.Process.Pid); err != nil {
			return err
		}
		waitErr := child.Wait()
		if home, homeErr := userHomeDir(); homeErr == nil {
			service.Home = home
			if _, err = service.Scan(cmd.Context()); err != nil {
				return err
			}
		}
		if err = service.EndRun(cmd.Context(), runID, args[0], start); err != nil {
			return err
		}
		exact, reason, err := service.RunStatus(cmd.Context(), runID)
		if err != nil {
			return err
		}
		if opts.format == "json" {
			return writeResult(opts.stdout, opts.format, "run."+args[0], map[string]any{"run_id": runID, "exact": exact, "attribution": map[bool]string{true: "exact", false: "estimated"}[exact], "reason": reason})
		}
		return waitErr
	}}
}

func writeResult(w io.Writer, format, command string, data any) error {
	if format == "json" {
		return json.NewEncoder(w).Encode(output.New(command, data, time.Now()))
	}
	if data != nil {
		return json.NewEncoder(w).Encode(data)
	}
	return nil
}
func writeEnvelope(w io.Writer, format, command string, data any, partial bool, warnings []string) error {
	if format == "json" {
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
