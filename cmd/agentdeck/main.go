package main

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/kitdine/agent-deck/internal/backup"
	"github.com/kitdine/agent-deck/internal/buildinfo"
	"github.com/kitdine/agent-deck/internal/credentialvault"
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
var machineIdentity credentialvault.MachineIdentity = platform.MachineIdentity
var newCredentialVault = func(stateRoot string) provider.CredentialVault {
	return credentialvault.New(stateRoot, machineIdentity)
}
var credentialIsTerminal = func(file *os.File) bool { return term.IsTerminal(int(file.Fd())) }
var credentialReadPassword = term.ReadPassword

type commandOptions struct {
	stateDir string
	format   string
	quiet    bool
	noColor  bool
	stdin    io.Reader
	stdout   io.Writer
	stderr   io.Writer
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
	command, err := executeCommand(args, stdin, stdout, stderr)
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
	case errors.Is(err, backup.ErrDestinationExists):
		return backup.ErrDestinationExists.Error()
	case errors.Is(err, credentialvault.ErrKeyMissing):
		return credentialvault.ErrKeyMissing.Error()
	case errors.Is(err, credentialvault.ErrKeyPermissions):
		return credentialvault.ErrKeyPermissions.Error()
	case errors.Is(err, credentialvault.ErrKeyMachineMismatch):
		return credentialvault.ErrKeyMachineMismatch.Error()
	case errors.Is(err, credentialvault.ErrKeyVersionUnsupported):
		return credentialvault.ErrKeyVersionUnsupported.Error()
	case errors.Is(err, credentialvault.ErrCiphertextInvalid):
		return credentialvault.ErrCiphertextInvalid.Error()
	case errors.Is(err, credentialvault.ErrMachineIdentityMissing):
		return credentialvault.ErrMachineIdentityMissing.Error()
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
	return errors.As(err, &target) || errors.Is(err, provider.ErrInvalidProvider) || errors.Is(err, provider.ErrInvalidMultiplier)
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
	_, err := executeCommand(args, stdin, stdout, io.Discard)
	return err
}

func executeCommand(args []string, stdin io.Reader, stdout, stderr io.Writer) (*cobra.Command, error) {
	root := newRootCommandWithError(stdin, stdout, stderr)
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
	return newRootCommandWithError(stdin, stdout, io.Discard)
}

func newRootCommandWithError(stdin io.Reader, stdout, stderr io.Writer) *cobra.Command {
	opts := &commandOptions{format: "text", stdin: stdin, stdout: stdout, stderr: stderr}
	showVersion := false
	root := &cobra.Command{
		Use:           "agentdeck",
		Short:         "Manage local AI provider, usage, and session data",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(command *cobra.Command, _ []string) error {
			if err := opts.validateFormat(); err != nil {
				return err
			}
			if opts.format == "ndjson" && command.Name() != "watch" {
				return &inputError{err: fmt.Errorf("ndjson format is supported only by watch")}
			}
			return nil
		},
		Args: exactArgs(0),
		RunE: func(command *cobra.Command, _ []string) error {
			if showVersion {
				return writeBuildIdentity(opts)
			}
			return command.Help()
		},
	}
	root.SetIn(stdin)
	root.SetOut(stdout)
	root.SetErr(stderr)
	flags := root.PersistentFlags()
	flags.StringVar(&opts.stateDir, "state-dir", "", "AgentDeck state directory")
	flags.StringVar(&opts.format, "format", "text", "Output format: text, json, or ndjson for watch")
	flags.BoolVar(&opts.noColor, "no-color", false, "Disable color output")
	flags.BoolVar(&opts.quiet, "quiet", false, "Suppress non-essential output")
	root.Flags().BoolVar(&showVersion, "version", false, "Print build identity")
	root.CompletionOptions.DisableDefaultCmd = true
	root.AddCommand(newProviderCommand(opts), newCredentialCommand(opts), newUsageCommand(opts), newPriceCommand(opts), newSessionCommand(opts), newExtensionCommand(opts), newWatchCommand(opts), newBackupCommand(opts), newDoctorCommand(opts), newRunCommand(opts), newVersionCommand(opts), newCompletionCommand(opts))
	applyHelpCatalog(root)
	wrapArgumentValidators(root)
	return root
}

type helpEntry struct {
	short   string
	long    string
	example string
}

func argumentHelp(summary, arguments string) string {
	return summary + "\n\nArguments:\n" + arguments
}

func applyHelpCatalog(root *cobra.Command) {
	entries := map[string]helpEntry{
		"provider list":    {short: "List provider definitions"},
		"provider current": {short: "Show current provider selections"},
		"provider status":  {short: "Show provider and credential readiness", long: argumentHelp("Show active selection and credential readiness.", "  name  Optional provider filter."), example: "  agentdeck provider status aigocode"},
		"provider recover": {short: "Inspect and recover interrupted provider switches"},
		"provider show": {
			short:   "Show one provider",
			long:    argumentHelp("Show one provider definition without accessing provider credentials.", "  name  Provider name returned by 'agentdeck provider list'."),
			example: "  agentdeck provider show aigocode",
		},
		"provider add": {
			short: "Create a provider or add one of its credentials",
			long:  argumentHelp("Create a provider or add a named credential when the provider already exists. --credential is the short name; AgentDeck generates <provider>-<credential>-ref. Codex-bound endpoints may include a final /v1, which is removed from the stored base and added back in Codex configuration.", "  name  Provider name. Credential endpoint, multiplier, and clients are flags."),
			example: "  agentdeck provider add aigocode --credential default --endpoint https://api.example.com --clients claude\n" +
				"  agentdeck provider add aigocode --credential codex --endpoint https://api.example.com/v1 --clients codex --multiplier 0.4",
		},
		"provider update": {
			short:   "Update provider credential metadata",
			long:    argumentHelp("Update endpoint, multiplier, or client bindings for one provider credential. A sole credential is inferred.", "  name  Existing provider name."),
			example: "  agentdeck provider update aigocode --credential codex --multiplier 1.2",
		},
		"provider remove": {
			short:   "Remove a provider and its credential",
			long:    argumentHelp("Remove a provider definition, credential metadata, and encrypted credential ciphertext in one SQLite transaction.", "  name  Existing provider name."),
			example: "  agentdeck provider remove aigocode",
		},
		"provider use": {
			short:   "Switch a client to a provider",
			long:    argumentHelp("Switch a client to a provider and named credential, inferring unique choices. Codex official sets [model_providers.custom].name to official and removes the custom base URL and bearer token.", "  name  Existing provider name."),
			example: "  agentdeck provider use aigocode --client codex --credential work",
		},
		"credential list": {short: "List credential readiness without revealing values", long: argumentHelp("List named credential metadata and readiness.", "  provider  Optional provider filter."), example: "  agentdeck credential list aigocode --client codex"},
		"credential show": {short: "Show one named credential", long: argumentHelp("Show non-secret metadata and readiness for a named credential.", "  provider  Existing provider name."), example: "  agentdeck credential show aigocode --credential work"},
		"credential add": {
			short:   "Store a provider credential",
			long:    argumentHelp("Prompt without terminal echo and add credential-owned endpoint, multiplier, and client metadata through the shared provider-add service.", "  provider  Existing provider name."),
			example: "  agentdeck credential add aigocode --credential work --endpoint https://api.example.com --clients codex,claude",
		},
		"credential update": {
			short:   "Update or rotate a provider credential",
			long:    argumentHelp("Update endpoint, multiplier, bindings, or rotate a named credential without revealing its value.", "  provider  Existing provider name."),
			example: "  agentdeck credential update aigocode --credential work --rotate",
		},
		"credential remove": {
			short:   "Delete a provider credential",
			long:    argumentHelp("Delete named credential metadata and encrypted ciphertext in one SQLite transaction.", "  provider  Existing provider name."),
			example: "  agentdeck credential remove aigocode --credential work",
		},
		"session scan":        {short: "Incrementally scan local client sessions"},
		"session list":        {short: "List indexed sessions"},
		"session rebuild":     {short: "Rebuild the purgeable session index"},
		"session purge-index": {short: "Delete only the rebuildable session index"},
		"session search": {
			short:   "Search indexed session text",
			long:    argumentHelp("Search the local, separately purgeable session index.", "  query  Search text; quote it when it contains spaces."),
			example: "  agentdeck session search \"provider timeout\"",
		},
		"session show": {
			short:   "Show one indexed session",
			long:    argumentHelp("Show indexed metadata and approved visible text; use --client when the ID is ambiguous.", "  session-id  Session identifier returned by session list or search."),
			example: "  agentdeck session show 019abc123 --client codex",
		},
		"session exclude": {
			short:   "Exclude a session source from indexing",
			long:    "Persist an exclusion for future session scans and rebuilds. Both --kind and --value are required.",
			example: "  agentdeck session exclude --kind client --value claude\n  agentdeck session exclude --kind path --value /private/project",
		},
		"extension scan":   {short: "Scan native Codex and Claude extensions"},
		"extension list":   {short: "List discovered extensions"},
		"extension doctor": {short: "Diagnose extension drift and duplicates"},
		"extension show": {
			short:   "Show one extension",
			long:    argumentHelp("Show one discovered native extension and its diagnostics.", "  id  Extension ID returned by extension list."),
			example: "  agentdeck extension show codex:skill:example",
		},
		"extension adopt": {
			short:   "Mark an extension as managed",
			long:    argumentHelp("Adopt an existing native extension into AgentDeck management metadata.", "  id  Extension ID returned by extension list."),
			example: "  agentdeck extension adopt codex:skill:example",
		},
		"extension release": {
			short:   "Release an extension from management",
			long:    argumentHelp("Remove AgentDeck management metadata without deleting the native extension.", "  id  Managed extension ID."),
			example: "  agentdeck extension release codex:skill:example",
		},
		"extension enable": {
			short:   "Request extension enablement",
			long:    argumentHelp("Request enablement for an extension when its native contract supports mutation.", "  id  Extension ID returned by extension list."),
			example: "  agentdeck extension enable codex:skill:example",
		},
		"extension disable": {
			short:   "Request extension disablement",
			long:    argumentHelp("Request disablement for an extension when its native contract supports mutation.", "  id  Extension ID returned by extension list."),
			example: "  agentdeck extension disable codex:skill:example",
		},
		"backup list": {short: "List portable AgentDeck backups"},
		"backup create": {
			short:   "Create an encrypted portable backup",
			long:    argumentHelp("Create an encrypted .adb archive without overwriting an existing file.", "  path  Optional destination. Defaults to the AgentDeck backup directory."),
			example: "  agentdeck backup create\n  agentdeck backup create /secure/agentdeck.adb",
		},
		"backup inspect": {
			short:   "Inspect an encrypted backup manifest",
			long:    argumentHelp("Authenticate an archive and print its manifest without restoring it.", "  path  Existing .adb archive path."),
			example: "  agentdeck backup inspect /secure/agentdeck.adb",
		},
		"backup restore": {
			short:   "Restore an encrypted portable backup",
			long:    argumentHelp("Restore an authenticated archive into an empty AgentDeck state directory.", "  path  Existing .adb archive path."),
			example: "  agentdeck --state-dir /new/state backup restore /secure/agentdeck.adb",
		},
		"usage scan":     {short: "Incrementally import local usage events"},
		"usage summary":  {short: "Summarize local usage and cost"},
		"usage sessions": {short: "List session-level usage and cost"},
		"usage diagnose": {short: "Diagnose usage attribution and source coverage"},
		"usage rebuild":  {short: "Rebuild usage metadata from local sources"},
		"price history":  {short: "List price catalog provenance history"},
		"price status":   {short: "Show active price catalog provenance"},
		"price update":   {short: "Download the latest LiteLLM price catalog"},
		"price override": {short: "Apply a local structured price override"},
		"run": {
			short:   "Run Codex or Claude with usage attribution",
			long:    argumentHelp("Run a supported client and attribute the resulting local usage events.", "  codex|claude       Client executable to launch.\n  client arguments   Arguments passed unchanged after the required -- separator."),
			example: "  agentdeck run codex -- --help\n  agentdeck run claude -- --version",
		},
		"completion": {short: "Generate shell completion", long: argumentHelp("Generate completion for a supported shell.", "  bash|fish|zsh  Supported shell."), example: "  agentdeck completion zsh"},
	}
	for path, entry := range entries {
		command, _, err := root.Find(strings.Fields(path))
		if err != nil {
			panic(fmt.Sprintf("help catalog command %q: %v", path, err))
		}
		command.Short = entry.short
		if entry.long != "" {
			command.Long = entry.long
		}
		if entry.example != "" {
			command.Example = entry.example
		}
	}
}

func newVersionCommand(opts *commandOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print build identity",
		Args:  exactArgs(0),
		RunE: func(_ *cobra.Command, _ []string) error {
			return writeBuildIdentity(opts)
		},
	}
}

func newCompletionCommand(opts *commandOptions) *cobra.Command {
	return &cobra.Command{Use: "completion <bash|fish|zsh>", Short: "Generate shell completion", Args: exactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return command.Root().GenBashCompletion(opts.stdout)
		case "fish":
			return command.Root().GenFishCompletion(opts.stdout, true)
		case "zsh":
			return command.Root().GenZshCompletion(opts.stdout)
		default:
			return &inputError{err: fmt.Errorf("unsupported shell %q", args[0])}
		}
	}}
}

func writeBuildIdentity(opts *commandOptions) error {
	identity := buildinfo.Current()
	if opts.format == "json" {
		return writeResult(opts.stdout, opts.format, "version", identity)
	}
	_, err := fmt.Fprintf(opts.stdout, "Release Version: %s\nGit Commit Hash: %s\nGit Branch: %s\nGo Version: %s\nUTC Build Time: %s\n", identity.Version, identity.Commit, identity.Branch, identity.GoVersion, identity.BuildTime)
	return err
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
	if err := o.validateFormat(); err != nil {
		return "", err
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

func (o *commandOptions) validateFormat() error {
	if o.format != "text" && o.format != "json" && o.format != "ndjson" {
		return &inputError{err: fmt.Errorf("invalid format %q", o.format)}
	}
	return nil
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
			database, stateDir, err := opts.openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer database.Close()
			data, err := run(cmd.Context(), provider.Service{Store: database, Vault: newCredentialVault(stateDir), StateRoot: stateDir}, args)
			if err != nil {
				return err
			}
			return writeResult(opts.stdout, opts.format, commandOutputName(cmd), data, opts.quiet)
		}
	}
	var configPath, useClient, useCredential string
	use := &cobra.Command{Use: "use <name>", Args: cobra.ExactArgs(1), RunE: withService(func(ctx context.Context, s provider.Service, args []string) (any, error) {
		client := provider.Client(useClient)
		if args[0] == provider.OfficialProviderName {
			if useCredential != "" {
				return nil, &inputError{err: fmt.Errorf("official does not accept credentials")}
			}
			if client == "" {
				client = provider.ClientCodex
			}
		}
		if client == "" {
			result, err := s.Show(ctx, args[0])
			if err != nil {
				return nil, err
			}
			clients := map[string]bool{}
			for _, mapping := range result.Definition.Clients {
				clients[mapping.Client] = true
			}
			if len(clients) != 1 {
				return nil, &inputError{err: fmt.Errorf("--client is required when provider supports multiple clients")}
			}
			for value := range clients {
				client = provider.Client(value)
			}
		}
		if configPath == "" {
			home, err := userHomeDir()
			if err != nil {
				return nil, err
			}
			s.Home = home
		}
		err := s.UseCredential(ctx, args[0], client, useCredential, configPath, "")
		return withTextResource(nil, args[0]), err
	})}
	use.Flags().StringVar(&configPath, "config-path", "", "Override the automatically resolved client configuration path")
	use.Flags().StringVar(&useClient, "client", "", "Client to switch: codex or claude")
	use.Flags().StringVar(&useCredential, "credential", "", "Credential shorthand, not the generated reference")
	var endpoint, clientsValue, multiplierValue, credentialName string
	add := &cobra.Command{Use: "add <name>", Args: cobra.ExactArgs(1), RunE: withService(func(ctx context.Context, s provider.Service, args []string) (any, error) {
		clients, err := parseClients(clientsValue)
		if err != nil {
			return nil, err
		}
		name, err := provider.NormalizeCredentialName(credentialName)
		if err != nil {
			return nil, err
		}
		definition := provider.Definition{Name: args[0], Endpoint: endpoint, Clients: clients, Multiplier: multiplierValue}
		plan, err := s.PlanProviderCredential(ctx, definition, name)
		if err != nil {
			return nil, err
		}
		if plan.Noop {
			shown, showErr := s.Show(ctx, args[0])
			return shown.Definition, showErr
		}
		value, err := readCredential(opts.stdin, opts.stderr, plan.Reference)
		if err != nil {
			return nil, err
		}
		if _, err = s.AddProviderWithCredential(ctx, definition, name, value); err != nil {
			return nil, err
		}
		shown, err := s.Show(ctx, args[0])
		return shown.Definition, err
	})}
	add.Flags().StringVar(&endpoint, "endpoint", "", "Credential base endpoint without userinfo, query, or fragment; Codex-bound final /v1 is normalized")
	add.Flags().StringVar(&clientsValue, "clients", "", "Comma-separated credential client bindings")
	add.Flags().StringVar(&multiplierValue, "multiplier", "1", "Credential cost multiplier")
	add.Flags().StringVar(&credentialName, "credential", "default", "Credential shorthand, not a reference")
	_ = add.MarkFlagRequired("endpoint")
	_ = add.MarkFlagRequired("clients")
	var updateEndpoint, updateClients, updateMultiplier, updateCredential string
	var update *cobra.Command
	update = &cobra.Command{Use: "update <name>", Args: cobra.ExactArgs(1), RunE: withService(func(ctx context.Context, s provider.Service, args []string) (any, error) {
		var ep, mult *string
		var clients []provider.Client
		if update.Flags().Changed("endpoint") {
			ep = &updateEndpoint
		}
		if update.Flags().Changed("multiplier") {
			mult = &updateMultiplier
		}
		if update.Flags().Changed("clients") {
			var err error
			clients, err = parseClients(updateClients)
			if err != nil {
				return nil, err
			}
		}
		if ep == nil && mult == nil && clients == nil {
			return nil, &inputError{err: fmt.Errorf("at least one update flag is required")}
		}
		if _, err := s.UpdateDefinition(ctx, args[0], updateCredential, ep, clients, mult); err != nil {
			return nil, err
		}
		shown, err := s.Show(ctx, args[0])
		return shown.Definition, err
	})}
	update.Flags().StringVar(&updateEndpoint, "endpoint", "", "Replacement endpoint without userinfo, query, or fragment")
	update.Flags().StringVar(&updateClients, "clients", "", "Replacement clients")
	update.Flags().StringVar(&updateMultiplier, "multiplier", "", "Replacement multiplier")
	update.Flags().StringVar(&updateCredential, "credential", "", "Credential shorthand, not a reference; inferred when unique")
	remove := &cobra.Command{Use: "remove <name>", Args: cobra.ExactArgs(1), RunE: withService(func(ctx context.Context, s provider.Service, args []string) (any, error) {
		return withTextResource(nil, args[0]), s.RemoveProvider(ctx, args[0])
	})}
	providerCmd.AddCommand(
		&cobra.Command{Use: "list", Args: cobra.NoArgs, RunE: withService(func(ctx context.Context, s provider.Service, _ []string) (any, error) { return s.List(ctx) })},
		&cobra.Command{Use: "current", Args: cobra.NoArgs, RunE: withService(func(ctx context.Context, s provider.Service, _ []string) (any, error) { return s.Current(ctx) })},
		&cobra.Command{Use: "status [name]", Args: cobra.MaximumNArgs(1), RunE: withService(func(ctx context.Context, s provider.Service, args []string) (any, error) {
			values, err := s.Status(ctx)
			if err != nil || len(args) == 0 {
				return values, err
			}
			for _, v := range values {
				if v.Definition.Name == args[0] {
					return v, nil
				}
			}
			return nil, sql.ErrNoRows
		})},
		&cobra.Command{Use: "show <name>", Args: cobra.ExactArgs(1), RunE: withService(func(ctx context.Context, s provider.Service, args []string) (any, error) {
			return s.Show(ctx, args[0])
		})},
		add, update, remove,
		use,
		&cobra.Command{Use: "recover", Args: cobra.NoArgs, RunE: withService(func(ctx context.Context, s provider.Service, _ []string) (any, error) { return s.Recover(ctx) })},
	)
	return providerCmd
}

func parseClients(value string) ([]provider.Client, error) {
	parts := strings.Split(value, ",")
	if strings.TrimSpace(value) == "" {
		return nil, &inputError{err: fmt.Errorf("clients are required")}
	}
	out := make([]provider.Client, 0, len(parts))
	seen := map[provider.Client]bool{}
	for _, part := range parts {
		client := provider.Client(strings.TrimSpace(part))
		if client != provider.ClientCodex && client != provider.ClientClaude || seen[client] {
			return nil, &inputError{err: fmt.Errorf("invalid client %q", part)}
		}
		seen[client] = true
		out = append(out, client)
	}
	return out, nil
}
func newCredentialCommand(opts *commandOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "credential", Short: "Manage named provider credentials"}
	withService := func(run func(context.Context, provider.Service, []string) (any, error)) func(*cobra.Command, []string) error {
		return func(command *cobra.Command, args []string) error {
			database, stateDir, err := opts.openStore(command.Context())
			if err != nil {
				return err
			}
			defer database.Close()
			data, err := run(command.Context(), provider.Service{Store: database, Vault: newCredentialVault(stateDir), StateRoot: stateDir}, args)
			if err != nil {
				return err
			}
			return writeResult(opts.stdout, opts.format, commandOutputName(command), data, opts.quiet)
		}
	}
	var listClient string
	list := &cobra.Command{Use: "list [provider]", Args: cobra.MaximumNArgs(1), RunE: withService(func(ctx context.Context, s provider.Service, args []string) (any, error) {
		if err := validateOptionalClient(listClient); err != nil {
			return nil, err
		}
		name := ""
		if len(args) > 0 {
			name = args[0]
		}
		return s.ListCredentials(ctx, name, listClient)
	})}
	list.Flags().StringVar(&listClient, "client", "", "Filter by client")
	var showName string
	show := &cobra.Command{Use: "show <provider>", Args: cobra.ExactArgs(1), RunE: withService(func(ctx context.Context, s provider.Service, args []string) (any, error) {
		return s.ShowCredential(ctx, args[0], showName)
	})}
	show.Flags().StringVar(&showName, "credential", "default", "Credential shorthand, not the generated reference")
	var addName, addClients, addEndpoint, addMultiplier string
	add := &cobra.Command{Use: "add <provider>", Args: cobra.ExactArgs(1), RunE: withService(func(ctx context.Context, s provider.Service, args []string) (any, error) {
		clients, err := parseClients(addClients)
		if err != nil {
			return nil, err
		}
		name, err := provider.NormalizeCredentialName(addName)
		if err != nil {
			return nil, err
		}
		definition := provider.Definition{Name: args[0], Endpoint: addEndpoint, Multiplier: addMultiplier, Clients: clients}
		plan, err := s.PlanProviderCredential(ctx, definition, name)
		if err != nil {
			return nil, err
		}
		if !plan.ProviderExists {
			return nil, fmt.Errorf("%w: provider does not exist", provider.ErrInvalidProvider)
		}
		if plan.Noop {
			return s.ShowCredential(ctx, args[0], name)
		}
		value, err := readCredential(opts.stdin, opts.stderr, plan.Reference)
		if err != nil {
			return nil, err
		}
		return s.AddCredential(ctx, args[0], name, addEndpoint, addMultiplier, clients, value)
	})}
	add.Flags().StringVar(&addName, "credential", "default", "Credential shorthand, not a reference")
	add.Flags().StringVar(&addEndpoint, "endpoint", "", "Credential base endpoint without userinfo, query, or fragment; Codex-bound final /v1 is normalized")
	add.Flags().StringVar(&addClients, "clients", "", "Comma-separated credential client bindings")
	add.Flags().StringVar(&addMultiplier, "multiplier", "1", "Credential cost multiplier")
	_ = add.MarkFlagRequired("endpoint")
	_ = add.MarkFlagRequired("clients")
	var updateName, updateClients, updateEndpoint, updateMultiplier string
	var rotate bool
	var update *cobra.Command
	update = &cobra.Command{Use: "update <provider>", Args: cobra.ExactArgs(1), RunE: withService(func(ctx context.Context, s provider.Service, args []string) (any, error) {
		var clients []provider.Client
		var err error
		if update.Flags().Changed("clients") {
			clients, err = parseClients(updateClients)
			if err != nil {
				return nil, err
			}
		}
		var value *string
		if rotate {
			item, showErr := s.ShowCredential(ctx, args[0], updateName)
			if showErr != nil {
				return nil, showErr
			}
			read, readErr := readCredential(opts.stdin, opts.stderr, item.Reference)
			if readErr != nil {
				return nil, readErr
			}
			value = &read
		}
		var ep, multiplier *string
		if update.Flags().Changed("endpoint") {
			ep = &updateEndpoint
		}
		if update.Flags().Changed("multiplier") {
			multiplier = &updateMultiplier
		}
		if clients == nil && value == nil && ep == nil && multiplier == nil {
			return nil, &inputError{err: fmt.Errorf("--endpoint, --multiplier, --clients, or --rotate is required")}
		}
		result, err := s.UpdateNamedCredential(ctx, args[0], updateName, ep, clients, multiplier, value)
		return withTextResource(result, args[0]+"/"+updateName), err
	})}
	update.Flags().StringVar(&updateName, "credential", "default", "Credential shorthand, not the generated reference")
	update.Flags().StringVar(&updateEndpoint, "endpoint", "", "Replacement credential base endpoint without userinfo, query, or fragment")
	update.Flags().StringVar(&updateClients, "clients", "", "Replacement client bindings")
	update.Flags().StringVar(&updateMultiplier, "multiplier", "", "Replacement credential multiplier")
	update.Flags().BoolVar(&rotate, "rotate", false, "Securely prompt for a replacement credential value")
	var removeName string
	remove := &cobra.Command{Use: "remove <provider>", Args: cobra.ExactArgs(1), RunE: withService(func(ctx context.Context, s provider.Service, args []string) (any, error) {
		return withTextResource(nil, args[0]+"/"+removeName), s.RemoveNamedCredential(ctx, args[0], removeName)
	})}
	remove.Flags().StringVar(&removeName, "credential", "default", "Credential shorthand, not the generated reference")
	cmd.AddCommand(list, show, add, update, remove)
	return cmd
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
			if command.Name() == "scan" || command.Name() == "rebuild" {
				fingerprint, fingerprintErr := watch.FingerprintRoots(sessionWatchRoots(home)...)
				if fingerprintErr != nil {
					return fingerprintErr
				}
				if releaseErr := lock.Release(); releaseErr != nil {
					return releaseErr
				}
				core, _, openErr := opts.openStore(command.Context())
				if openErr != nil {
					return openErr
				}
				if setErr := core.SetSetting(command.Context(), "watch.fingerprint.session", fingerprint); setErr != nil {
					core.Close()
					return setErr
				}
				if closeErr := core.Close(); closeErr != nil {
					return closeErr
				}
			}
			return writeResult(opts.stdout, opts.format, commandOutputName(command), data, opts.quiet)
		}
	}
	var listClient, searchClient, showClient, excludeKind, excludeValue string
	list := &cobra.Command{Use: "list", Args: cobra.NoArgs, RunE: withSessions(func(ctx context.Context, s *store.Store, _ string, _ []string) (any, error) {
		if err := validateOptionalClient(listClient); err != nil {
			return nil, err
		}
		items, err := session.List(ctx, s.DB)
		if err != nil {
			return nil, err
		}
		if listClient == "" {
			return items, nil
		}
		out := items[:0]
		for _, item := range items {
			if item.Client == listClient {
				out = append(out, item)
			}
		}
		return out, nil
	})}
	list.Flags().StringVar(&listClient, "client", "", "Filter by client")
	search := &cobra.Command{Use: "search <query>", Args: cobra.ExactArgs(1), RunE: withSessions(func(ctx context.Context, s *store.Store, _ string, args []string) (any, error) {
		if err := validateOptionalClient(searchClient); err != nil {
			return nil, err
		}
		items, err := session.Search(ctx, s.DB, args[0])
		if err != nil {
			return nil, err
		}
		if searchClient == "" {
			return items, nil
		}
		out := items[:0]
		for _, item := range items {
			if item.Client == searchClient {
				out = append(out, item)
			}
		}
		return out, nil
	})}
	search.Flags().StringVar(&searchClient, "client", "", "Filter by client")
	show := &cobra.Command{Use: "show <session-id>", Args: cobra.ExactArgs(1), RunE: withSessions(func(ctx context.Context, s *store.Store, _ string, args []string) (any, error) {
		if err := validateOptionalClient(showClient); err != nil {
			return nil, err
		}
		client := showClient
		if client == "" {
			items, err := session.List(ctx, s.DB)
			if err != nil {
				return nil, err
			}
			for _, item := range items {
				if item.SessionID == args[0] {
					if client != "" && client != item.Client {
						return nil, &inputError{err: fmt.Errorf("session id is ambiguous; use --client")}
					}
					client = item.Client
				}
			}
			if client == "" {
				return nil, sql.ErrNoRows
			}
		}
		return session.Show(ctx, s.DB, client, args[0])
	})}
	show.Flags().StringVar(&showClient, "client", "", "Session client")
	exclude := &cobra.Command{Use: "exclude", Args: cobra.NoArgs, RunE: withSessions(func(ctx context.Context, s *store.Store, _ string, _ []string) (any, error) {
		err := session.Exclude(ctx, s.DB, excludeKind, excludeValue)
		return withTextResource(map[string]any{"excluded": err == nil}, excludeKind+":"+excludeValue), err
	})}
	exclude.Flags().StringVar(&excludeKind, "kind", "", "Exclusion kind")
	exclude.Flags().StringVar(&excludeValue, "value", "", "Exact exclusion value")
	_ = exclude.MarkFlagRequired("kind")
	_ = exclude.MarkFlagRequired("value")
	cmd.AddCommand(
		&cobra.Command{Use: "scan", Args: cobra.NoArgs, RunE: withSessions(func(ctx context.Context, s *store.Store, home string, _ []string) (any, error) {
			return session.Scan(ctx, s.DB, home)
		})},
		list, search, show, exclude,
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
		defer func() {
			if lock != nil {
				_ = lock.Release()
			}
		}()
		if err = purgeSessionIndex(cmd.Context(), stateDir, store.OpenWithLockHeld, os.Remove); err != nil {
			return err
		}
		return writeResult(opts.stdout, opts.format, "session.purge-index", map[string]any{"purged": true}, opts.quiet)
	}}
}

func purgeSessionIndex(ctx context.Context, stateDir string, openCore func(context.Context, string) (*store.Store, error), remove func(string) error) error {
	core, err := openCore(ctx, stateDir)
	if err != nil {
		return err
	}
	if err = core.DeleteSetting(ctx, "watch.fingerprint.session"); err != nil {
		return errors.Join(err, core.Close())
	}
	if err = core.Close(); err != nil {
		return err
	}
	for _, suffix := range []string{"", "-wal", "-shm", "-journal"} {
		if err = remove(filepath.Join(stateDir, "sessions.sqlite3") + suffix); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func validateOptionalClient(client string) error {
	if client == "" || client == string(provider.ClientCodex) || client == string(provider.ClientClaude) {
		return nil
	}
	return &inputError{err: fmt.Errorf("invalid client %q", client)}
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
			if command.Name() == "scan" {
				fingerprint, fingerprintErr := watch.FingerprintRoots(extensionWatchRoots(home, workdir)...)
				if fingerprintErr != nil {
					return fingerprintErr
				}
				if setErr := database.SetSetting(command.Context(), "watch.fingerprint.extension", fingerprint); setErr != nil {
					return setErr
				}
			}
			return writeResult(opts.stdout, opts.format, commandOutputName(command), data, opts.quiet)
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
			return withTextResource(nil, args[0]), extension.Release(ctx, s, args[0])
		})},
		&cobra.Command{Use: "enable <id>", Args: exactArgs(1), RunE: withExtensions(func(ctx context.Context, s *store.Store, _, _ string, args []string) (any, error) {
			return withTextResource(nil, args[0]), extension.SetEnabled(ctx, s, args[0], true)
		})},
		&cobra.Command{Use: "disable <id>", Args: exactArgs(1), RunE: withExtensions(func(ctx context.Context, s *store.Store, _, _ string, args []string) (any, error) {
			return withTextResource(nil, args[0]), extension.SetEnabled(ctx, s, args[0], false)
		})},
	)
	return cmd
}

func newWatchCommand(opts *commandOptions) *cobra.Command {
	var interval time.Duration
	var domainsValue string
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
		sessionRoots := sessionWatchRoots(home)
		extensionRoots := extensionWatchRoots(home, workdir)
		fingerprint := func(roots []string) func(context.Context) (string, error) {
			return func(context.Context) (string, error) { return watch.FingerprintRoots(roots...) }
		}
		initial, err := loadWatchFingerprints(command.Context(), stateDir)
		if err != nil {
			return err
		}
		var database, sessions *store.Store
		openCore := func(ctx context.Context) error {
			if database != nil {
				return nil
			}
			database, err = store.Open(ctx, stateDir)
			return err
		}
		openSessions := func(ctx context.Context) error {
			if sessions != nil {
				return nil
			}
			sessions, err = store.OpenSessions(ctx, stateDir)
			return err
		}
		defer func() {
			if sessions != nil {
				_ = sessions.Close()
			}
			if database != nil {
				_ = database.Close()
			}
		}()
		requested := map[string]bool{}
		for _, domain := range strings.Split(domainsValue, ",") {
			domain = strings.TrimSpace(domain)
			if domain != "usage" && domain != "session" && domain != "extension" {
				return &inputError{err: fmt.Errorf("invalid watch domain %q", domain)}
			}
			requested[domain] = true
		}
		filtered := watch.SourceSet{}
		var usageInventory usage.Inventory
		if requested["usage"] {
			filtered = append(filtered, watch.Source{Domain: "usage", Snapshot: func(ctx context.Context) (string, error) {
				if err := openCore(ctx); err != nil {
					return "", err
				}
				usageInventory, err = usage.New(database, home).Inventory(ctx)
				return usageInventory.Fingerprint, err
			}, Scan: func(ctx context.Context) (int, error) {
				result, scanErr := usage.New(database, home).ScanInventory(ctx, usageInventory)
				return result["imported"] + result["updated"], scanErr
			}})
		}
		if requested["session"] {
			filtered = append(filtered, watch.Source{Domain: "session", Snapshot: fingerprint(sessionRoots), Scan: func(ctx context.Context) (int, error) {
				if err := openSessions(ctx); err != nil {
					return 0, err
				}
				result, scanErr := session.Scan(ctx, sessions.DB, home)
				return result.Documents, scanErr
			}})
		}
		if requested["extension"] {
			filtered = append(filtered, watch.Source{Domain: "extension", Snapshot: fingerprint(extensionRoots), Scan: func(ctx context.Context) (int, error) {
				if err := openCore(ctx); err != nil {
					return 0, err
				}
				result, scanErr := extension.Scan(ctx, database, home, workdir)
				return result.Found, scanErr
			}})
		}
		service := watch.Service{
			InitialFingerprints: initial,
			Sources:             filtered,
			Lock: func(ctx context.Context) (func() error, error) {
				lock, err := store.AcquireScanLock(ctx, stateDir, 0)
				if err != nil {
					return nil, err
				}
				return lock.Release, nil
			},
			PersistFingerprint: func(ctx context.Context, domain, value string) error {
				if err := openCore(ctx); err != nil {
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
	command.Flags().StringVar(&domainsValue, "domains", "usage,session,extension", "Comma-separated domains to watch")
	return command
}

func sessionWatchRoots(home string) []string {
	return []string{filepath.Join(home, ".codex", "sessions"), filepath.Join(home, ".codex", "archived_sessions"), filepath.Join(home, ".claude", "projects")}
}
func extensionWatchRoots(home, workdir string) []string {
	return []string{filepath.Join(home, ".codex", "config.toml"), filepath.Join(home, ".codex", "skills"), filepath.Join(home, ".codex", "plugins", "cache"), filepath.Join(home, ".claude.json"), filepath.Join(home, ".claude", "skills"), filepath.Join(home, ".claude", "plugins", "installed_plugins.json"), filepath.Join(workdir, ".codex", "config.toml"), filepath.Join(workdir, ".codex", "skills"), filepath.Join(workdir, ".codex", "plugins"), filepath.Join(workdir, ".claude", "skills"), filepath.Join(workdir, ".mcp.json")}
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
		manifest, err := (backup.Service{Core: database, StateRoot: stateDir, Vault: newCredentialVault(stateDir), Version: buildinfo.Version}).Create(command.Context(), destination, passphrase, includeSessions)
		if err != nil {
			return err
		}
		return writeResult(opts.stdout, opts.format, "backup.create", map[string]any{"path": destination, "manifest": manifest}, opts.quiet)
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
		manifest, err := backup.Restore(command.Context(), args[0], target, passphrase, machineIdentity)
		if err != nil {
			return err
		}
		return writeResult(opts.stdout, opts.format, "backup.restore", withTextResource(manifest, args[0]), opts.quiet)
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
		report, err := (doctor.Service{StateRoot: stateDir, Home: home, Workdir: workdir, Vault: newCredentialVault(stateDir)}).Check(command.Context(), full)
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
			return writeEnvelope(opts.stdout, opts.format, commandOutputName(command), data, partial, warnings, opts.quiet)
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
	)
	return cmd
}

func newPriceCommand(opts *commandOptions) *cobra.Command {
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
			return writeEnvelope(opts.stdout, opts.format, commandOutputName(command), data, partial, warnings, opts.quiet)
		}
	}
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
		commit, _ := update.Flags().GetString("commit")
		data, err := s.UpdateLiteLLM(ctx, commit, usage.PriceHTTPClient())
		return data, false, nil, err
	})}
	update.PreRunE = func(command *cobra.Command, _ []string) error {
		commit, _ := command.Flags().GetString("commit")
		if err := usage.ValidateLiteLLMCommit(commit); err != nil {
			return &inputError{err: err}
		}
		return nil
	}
	update.Flags().String("commit", "", "Optional pinned LiteLLM commit SHA")
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

var runClientProcesses func(string) ([]int, error)

func newRunCommand(opts *commandOptions) *cobra.Command {
	return &cobra.Command{Use: "run <codex|claude> -- <client arguments>", Args: func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 || cmd.ArgsLenAtDash() != 1 {
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
		service.ClientProcesses = runClientProcesses
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

func writeResult(w io.Writer, format, command string, data any, quiet ...bool) error {
	data, resource := unwrapTextResource(data)
	if format == "json" {
		return json.NewEncoder(w).Encode(output.New(command, data, time.Now()))
	}
	if format == "ndjson" {
		return &inputError{err: fmt.Errorf("ndjson format is supported only by watch")}
	}
	if isMutationCommand(command) {
		if len(quiet) > 0 && quiet[0] {
			return nil
		}
		return renderMutationText(w, command, mutationResource(data, resource))
	}
	if data != nil {
		return renderCommandText(w, command, data)
	}
	return nil
}
func writeEnvelope(w io.Writer, format, command string, data any, partial bool, warnings []string, quiet ...bool) error {
	data, resource := unwrapTextResource(data)
	if format == "json" {
		if warnings == nil {
			warnings = []string{}
		}
		envelope := output.New(command, data, time.Now())
		envelope.Partial, envelope.Warnings = partial, warnings
		return json.NewEncoder(w).Encode(envelope)
	}
	if isMutationCommand(command) {
		if len(quiet) > 0 && quiet[0] {
			return nil
		}
		if command != "usage.scan" && command != "usage.rebuild" {
			return renderMutationText(w, command, mutationResource(data, resource))
		}
	}
	return renderUsageText(w, command, data)
}

type textResource struct {
	data     any
	resource string
}

func withTextResource(data any, resource string) textResource {
	return textResource{data: data, resource: resource}
}

func unwrapTextResource(data any) (any, string) {
	if result, ok := data.(textResource); ok {
		return result.data, result.resource
	}
	return data, ""
}

func mutationResource(data any, explicit string) string {
	if explicit != "" {
		return explicit
	}
	switch value := data.(type) {
	case store.Provider:
		return value.Name
	case store.ProviderCredential:
		return strings.Trim(value.ProviderName+"/"+value.Name, "/")
	case provider.Credential:
		return value.Name
	case extension.DTO:
		return value.ID
	case map[string]any:
		if path, ok := value["path"].(string); ok {
			return path
		}
	}
	return ""
}

func renderMutationText(w io.Writer, command, resource string) error {
	if resource == "" {
		_, err := fmt.Fprintf(w, "Completed %s.\n", command)
		return err
	}
	_, err := fmt.Fprintf(w, "Completed %s for %q.\n", command, resource)
	return err
}

func isMutationCommand(command string) bool {
	switch command {
	case "provider.add", "provider.update", "provider.remove", "provider.use",
		"credential.add", "credential.update", "credential.remove",
		"usage.scan", "usage.rebuild", "price.update", "price.override",
		"session.scan", "session.exclude", "session.rebuild", "session.purge-index",
		"extension.scan", "extension.adopt", "extension.release", "extension.enable", "extension.disable",
		"backup.create", "backup.restore":
		return true
	default:
		return false
	}
}

func renderCommandText(w io.Writer, command string, data any) error {
	switch command {
	case "provider.list":
		value, ok := data.([]provider.DefinitionResult)
		if !ok {
			return fmt.Errorf("unexpected provider.list result %T", data)
		}
		if len(value) == 0 {
			_, err := fmt.Fprintln(w, "No providers.")
			return err
		}
		rows := make([][]string, 0, len(value))
		for _, item := range value {
			definition := item.Definition
			kind := "custom"
			if definition.BuiltIn {
				kind = "built-in"
			}
			clients := make([]string, 0, len(definition.Clients))
			for _, client := range definition.Clients {
				clients = append(clients, client.Client)
			}
			rows = append(rows, []string{definition.Name, kind, strings.Join(clients, ","), strconv.Itoa(definition.CredentialCount)})
		}
		return output.WriteASCIITable(w, []string{"NAME", "TYPE", "CLIENTS", "CREDENTIALS"}, rows)
	case "provider.show":
		value, ok := data.(provider.DefinitionResult)
		if !ok {
			return fmt.Errorf("unexpected provider.show result %T", data)
		}
		return renderProviderDetail(w, value.Definition)
	case "provider.status":
		switch value := data.(type) {
		case []provider.Status:
			return renderProviderStatuses(w, value)
		case provider.Status:
			return renderProviderStatus(w, value)
		default:
			return fmt.Errorf("unexpected provider.status result %T", data)
		}
	case "provider.current":
		value, ok := data.([]provider.CurrentSelection)
		if !ok {
			return fmt.Errorf("unexpected provider.current result %T", data)
		}
		if len(value) == 0 {
			_, err := fmt.Fprintln(w, "No current provider selections.")
			return err
		}
		rows := make([][]string, 0, len(value))
		for _, item := range value {
			credential := item.Credential
			if credential == "" {
				credential = "-"
			}
			rows = append(rows, []string{item.Client, item.Provider, credential, item.SelectedAt})
		}
		return output.WriteASCIITable(w, []string{"CLIENT", "PROVIDER", "CREDENTIAL", "SELECTED AT"}, rows)
	case "provider.recover":
		value, ok := data.([]store.Operation)
		if !ok {
			return fmt.Errorf("unexpected provider.recover result %T", data)
		}
		if len(value) == 0 {
			_, err := fmt.Fprintln(w, "No pending provider operations.")
			return err
		}
		rows := make([][]string, 0, len(value))
		for _, item := range value {
			rows = append(rows, []string{item.ID, item.Kind, item.State, item.ResourceName, item.Client, item.ErrorCode})
		}
		return output.WriteASCIITable(w, []string{"ID", "KIND", "STATE", "RESOURCE", "CLIENT", "ERROR"}, rows)
	case "credential.list":
		value, ok := data.([]provider.Credential)
		if !ok {
			return fmt.Errorf("unexpected credential.list result %T", data)
		}
		if len(value) == 0 {
			_, err := fmt.Fprintln(w, "No credentials.")
			return err
		}
		rows := make([][]string, 0, len(value))
		for _, item := range value {
			rows = append(rows, []string{item.Provider, item.Name, item.Reference, item.Endpoint, item.Multiplier, strings.Join(item.Clients, ","), strconv.FormatBool(item.Present)})
		}
		return output.WriteASCIITable(w, []string{"PROVIDER", "NAME", "REFERENCE", "ENDPOINT", "MULTIPLIER", "CLIENTS", "READY"}, rows)
	case "credential.show":
		value, ok := data.(provider.Credential)
		if !ok {
			return fmt.Errorf("unexpected credential.show result %T", data)
		}
		_, err := fmt.Fprintf(w, "provider: %s\nname: %s\nreference: %s\nendpoint: %s\nmultiplier: %s\nclients: %s\nready: %t\n", value.Provider, value.Name, value.Reference, value.Endpoint, value.Multiplier, textList(value.Clients), value.Present)
		return err
	case "session.list":
		value, ok := data.([]session.Metadata)
		if !ok {
			return fmt.Errorf("unexpected session.list result %T", data)
		}
		return renderSessionMetadata(w, value)
	case "session.search":
		value, ok := data.([]session.Document)
		if !ok {
			return fmt.Errorf("unexpected session.search result %T", data)
		}
		return renderSessionDocuments(w, value)
	case "session.show":
		value, ok := data.(session.Result)
		if !ok {
			return fmt.Errorf("unexpected session.show result %T", data)
		}
		if _, err := fmt.Fprintf(w, "client: %s\nsession: %s\nproject: %s\nmodel: %s\nfirst: %s\nlast: %s\n", value.Client, value.SessionID, value.Project, value.Model, value.FirstAt, value.LastAt); err != nil {
			return err
		}
		return renderSessionDocuments(w, value.Documents)
	case "extension.list":
		value, ok := data.([]extension.DTO)
		if !ok {
			return fmt.Errorf("unexpected extension.list result %T", data)
		}
		return renderExtensions(w, value)
	case "extension.show":
		value, ok := data.(extension.DTO)
		if !ok {
			return fmt.Errorf("unexpected extension.show result %T", data)
		}
		return renderExtensionDetail(w, value)
	case "extension.doctor":
		value, ok := data.(extension.DoctorReport)
		if !ok {
			return fmt.Errorf("unexpected extension.doctor result %T", data)
		}
		_, err := fmt.Fprintf(w, "diagnostics: %s\nmissing paths: %s\nduplicate ids: %s\ndrifted ids: %s\nmanagement anomalies: %s\n", textList(value.Diagnostics), textList(value.MissingPaths), textList(value.DuplicateIDs), textList(value.DriftedIDs), textList(value.ManagementAnomalies))
		return err
	case "backup.list":
		value, ok := data.([]backup.FileInfo)
		if !ok {
			return fmt.Errorf("unexpected backup.list result %T", data)
		}
		if len(value) == 0 {
			_, err := fmt.Fprintln(w, "No portable backups.")
			return err
		}
		rows := make([][]string, 0, len(value))
		for _, item := range value {
			rows = append(rows, []string{item.Path, strconv.FormatInt(item.Size, 10), item.ModifiedAt.Format(time.RFC3339Nano)})
		}
		return output.WriteASCIITable(w, []string{"PATH", "SIZE", "MODIFIED"}, rows)
	case "backup.inspect":
		value, ok := data.(backup.Manifest)
		if !ok {
			return fmt.Errorf("unexpected backup.inspect result %T", data)
		}
		return renderBackupManifest(w, value)
	case "doctor":
		value, ok := data.(doctor.Report)
		if !ok {
			return fmt.Errorf("unexpected doctor result %T", data)
		}
		return renderDoctorText(w, value)
	case "price.history":
		value, ok := data.([]map[string]string)
		if !ok {
			return fmt.Errorf("unexpected price.history result %T", data)
		}
		return renderPriceHistory(w, value)
	case "price.status":
		value, ok := data.(map[string]any)
		if !ok {
			return fmt.Errorf("unexpected price.status result %T", data)
		}
		return renderPriceStatus(w, value)
	default:
		return fmt.Errorf("no text renderer for %s (%T)", command, data)
	}
}

func renderProviderDetail(w io.Writer, value provider.Provider) error {
	kind := "custom"
	if value.BuiltIn {
		kind = "built-in"
	}
	clients := make([]string, 0, len(value.Clients))
	for _, mapping := range value.Clients {
		clients = append(clients, mapping.Client)
	}
	if _, err := fmt.Fprintf(w, "name: %s\ntype: %s\nclients: %s\ncredentials: %d\n", value.Name, kind, textList(clients), value.CredentialCount); err != nil {
		return err
	}
	if value.Authentication != "" {
		_, err := fmt.Fprintf(w, "authentication: %s\n", value.Authentication)
		return err
	}
	return nil
}

func renderProviderStatuses(w io.Writer, values []provider.Status) error {
	if len(values) == 0 {
		_, err := fmt.Fprintln(w, "No providers.")
		return err
	}
	rows := make([][]string, 0, len(values))
	for _, value := range values {
		kind, ready := "custom", value.Ready
		if value.Definition.BuiltIn {
			kind, ready = "built-in", true
		}
		codexActive, claudeActive := "-", "-"
		for _, selection := range value.Active {
			credential := selection.Credential
			if credential == "" {
				credential = "-"
			}
			switch selection.Client {
			case string(provider.ClientCodex):
				codexActive = credential
			case string(provider.ClientClaude):
				claudeActive = credential
			}
		}
		rows = append(rows, []string{
			value.Definition.Name,
			kind,
			strconv.Itoa(len(value.Credentials)),
			strconv.FormatBool(ready),
			codexActive,
			claudeActive,
		})
	}
	return output.WriteASCIITable(w, []string{"NAME", "TYPE", "CREDENTIALS", "READY", "CODEX ACTIVE", "CLAUDE ACTIVE"}, rows)
}

func renderProviderStatus(w io.Writer, value provider.Status) error {
	if err := renderProviderDetail(w, value.Definition); err != nil {
		return err
	}
	if value.Definition.BuiltIn {
		if _, err := fmt.Fprintln(w, "credential readiness: not applicable"); err != nil {
			return err
		}
	} else if len(value.Credentials) == 0 {
		if _, err := fmt.Fprintln(w, "credentials: none\nready: false"); err != nil {
			return err
		}
	} else if _, err := fmt.Fprintf(w, "credentials: %d\nready: %t\n", len(value.Credentials), value.Ready); err != nil {
		return err
	}
	rows := make([][]string, 0, 2)
	for _, client := range []provider.Client{provider.ClientCodex, provider.ClientClaude} {
		active, credential, selectedAt := "false", "-", "-"
		for _, selection := range value.Active {
			if selection.Client != string(client) {
				continue
			}
			active, selectedAt = "true", selection.SelectedAt
			if selection.Credential != "" {
				credential = selection.Credential
			}
		}
		rows = append(rows, []string{string(client), active, credential, selectedAt})
	}
	return output.WriteASCIITable(w, []string{"CLIENT", "ACTIVE", "CREDENTIAL", "SELECTED AT"}, rows)
}

func renderSessionMetadata(w io.Writer, values []session.Metadata) error {
	if len(values) == 0 {
		_, err := fmt.Fprintln(w, "No sessions.")
		return err
	}
	rows := make([][]string, 0, len(values))
	for _, value := range values {
		rows = append(rows, []string{value.Client, value.SessionID, value.Project, value.Model, value.FirstAt, value.LastAt})
	}
	return output.WriteASCIITable(w, []string{"CLIENT", "SESSION", "PROJECT", "MODEL", "FIRST", "LAST"}, rows)
}

func renderSessionDocuments(w io.Writer, values []session.Document) error {
	if len(values) == 0 {
		_, err := fmt.Fprintln(w, "No session documents.")
		return err
	}
	rows := make([][]string, 0, len(values))
	for _, value := range values {
		rows = append(rows, []string{value.Client, value.SessionID, value.Kind, oneLine(value.Text)})
	}
	return output.WriteASCIITable(w, []string{"CLIENT", "SESSION", "KIND", "TEXT"}, rows)
}

func renderExtensions(w io.Writer, values []extension.DTO) error {
	if len(values) == 0 {
		_, err := fmt.Fprintln(w, "No extensions.")
		return err
	}
	rows := make([][]string, 0, len(values))
	for _, value := range values {
		rows = append(rows, []string{value.ID, value.Client, value.Kind, value.Scope, value.Version, value.Enabled, strconv.FormatBool(value.Managed), strconv.FormatBool(value.Drift)})
	}
	return output.WriteASCIITable(w, []string{"ID", "CLIENT", "KIND", "SCOPE", "VERSION", "ENABLED", "MANAGED", "DRIFT"}, rows)
}

func renderExtensionDetail(w io.Writer, value extension.DTO) error {
	_, err := fmt.Fprintf(w, "id: %s\nclient: %s\nkind: %s\nscope: %s\nnative id: %s\nsource path: %s\nversion: %s\nenabled: %s\ncapabilities: %s\ndiagnostics: %s\nmanaged: %t\ndrift: %t\n", value.ID, value.Client, value.Kind, value.Scope, value.NativeID, value.SourcePath, value.Version, value.Enabled, textList(value.Capabilities), textList(value.Diagnostics), value.Managed, value.Drift)
	return err
}

func renderBackupManifest(w io.Writer, value backup.Manifest) error {
	_, err := fmt.Fprintf(w, "schema version: %d\nagentdeck version: %s\ncreated: %s\nsource platform: %s\nincluded: %s\nentries: %d\n", value.SchemaVersion, value.AgentDeckVersion, value.CreatedAt.Format(time.RFC3339Nano), value.SourcePlatform, textList(value.Included), len(value.Entries))
	return err
}

func renderPriceHistory(w io.Writer, values []map[string]string) error {
	if len(values) == 0 {
		_, err := fmt.Fprintln(w, "No price catalogs.")
		return err
	}
	rows := make([][]string, 0, len(values))
	for _, value := range values {
		rows = append(rows, []string{value["version"], value["source_kind"], value["effective_from"], value["source_url"], value["content_sha256"]})
	}
	return output.WriteASCIITable(w, []string{"VERSION", "SOURCE", "EFFECTIVE", "URL", "SHA256"}, rows)
}

func renderPriceStatus(w io.Writer, value map[string]any) error {
	available, _ := value["available"].(bool)
	if !available {
		_, err := fmt.Fprintln(w, "No price catalog is available.")
		return err
	}
	_, err := fmt.Fprintf(w, "available: true\nversion: %v\nsource: %v\neffective: %v\ncommit: %v\nsha256: %v\n", value["version"], value["source_kind"], value["effective_from"], value["commit_sha"], value["content_sha256"])
	return err
}

func renderDoctorText(w io.Writer, report doctor.Report) error {
	if _, err := fmt.Fprintf(w, "status: %s\nmode: %s\nwarnings: %d\nerrors: %d\n", report.Status, report.Mode, report.Warnings, report.Errors); err != nil {
		return err
	}
	for _, check := range report.Checks {
		if _, err := fmt.Fprintf(w, "%s: %s", check.Name, check.Status); err != nil {
			return err
		}
		if check.Code != "" {
			if _, err := fmt.Fprintf(w, " (%s)", check.Code); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}
	return nil
}
func renderUsageText(w io.Writer, command string, data any) error {
	if values, ok := data.(map[string]int); ok && (command == "usage.scan" || command == "usage.rebuild") {
		_, err := fmt.Fprintf(w, "files: %d\nimported: %d\nupdated: %d\nignored non-usage: %d\nunsupported usage: %d\nmalformed: %d\nsource resets: %d\n", values["files"], values["imported"], values["updated"], values["ignored_non_usage"], values["unsupported_usage"], values["malformed"], values["source_resets"])
		return err
	}
	if values, ok := data.(map[string]any); ok && command == "usage.diagnose" {
		for _, key := range []string{"files", "events", "sessions", "exact_runs"} {
			if _, err := fmt.Fprintf(w, "%s: %v\n", key, values[key]); err != nil {
				return err
			}
		}
		return nil
	}
	switch v := data.(type) {
	case usage.Summary:
		_, err := fmt.Fprintf(w, "events: %d\ntokens: %s\ncatalog base cost: %s\nprovider cost: %s\nwarnings: %s\nunpriced: %s\n", v.Counts["events"], formatTokens(v.Tokens), optionalCost(v.CatalogBaseCost), optionalCost(v.ProviderCost), textList(v.Warnings), textList(v.Unpriced))
		return err
	case []usage.SessionSummary:
		if len(v) == 0 {
			_, err := fmt.Fprintln(w, "No usage sessions.")
			return err
		}
		rows := make([][]string, 0, len(v))
		for _, x := range v {
			rows = append(rows, []string{x.Client, x.SessionID, x.FirstAt, x.LastAt, formatTokens(x.Tokens), optionalCost(x.CatalogBaseCost), optionalCost(x.ProviderCost), textList(x.Warnings), textList(x.Unpriced)})
		}
		return output.WriteASCIITable(w, []string{"CLIENT", "SESSION", "FIRST", "LAST", "TOKENS", "BASE COST", "PROVIDER COST", "WARNINGS", "UNPRICED"}, rows)
	default:
		return fmt.Errorf("no usage text renderer for %s (%T)", command, data)
	}
}

func optionalCost(value *string) string {
	if value == nil {
		return "unavailable"
	}
	return *value
}

func formatTokens(values map[string]int64) string {
	parts := make([]string, 0, len(values))
	for _, name := range []string{"input_tokens", "cached_input_tokens", "output_tokens", "cache_read_tokens", "cache_creation_tokens", "cache_write_5m_tokens", "cache_write_1h_tokens"} {
		if value, found := values[name]; found {
			parts = append(parts, fmt.Sprintf("%s=%d", name, value))
		}
	}
	return textList(parts)
}

func textList(values []string) string {
	if len(values) == 0 {
		return "none"
	}
	return strings.Join(values, ",")
}

func textValue(value string) string {
	if value == "" {
		return "none"
	}
	return value
}

func oneLine(value string) string {
	return strings.Join(strings.Fields(value), " ")
}
func readCredential(reader io.Reader, prompt io.Writer, reference string) (string, error) {
	if strings.TrimSpace(reference) == "" || strings.ContainsAny(reference, "\r\n") {
		return "", &inputError{err: fmt.Errorf("invalid credential reference")}
	}
	var value string
	if file, ok := reader.(*os.File); ok && credentialIsTerminal(file) {
		if _, err := fmt.Fprintf(prompt, "Credential for %s: ", reference); err != nil {
			return "", err
		}
		contents, err := credentialReadPassword(int(file.Fd()))
		if _, promptErr := fmt.Fprintln(prompt); err == nil && promptErr != nil {
			err = promptErr
		}
		if err != nil {
			return "", err
		}
		value = strings.TrimRight(string(contents), "\r\n")
	} else {
		line, err := bufio.NewReader(reader).ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return "", err
		}
		value = strings.TrimRight(line, "\r\n")
	}
	if strings.TrimSpace(value) == "" {
		return "", &inputError{err: fmt.Errorf("credential is empty")}
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
