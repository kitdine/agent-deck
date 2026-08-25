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
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/kitdine/agent-deck/internal/activity"
	"github.com/kitdine/agent-deck/internal/backup"
	"github.com/kitdine/agent-deck/internal/buildinfo"
	"github.com/kitdine/agent-deck/internal/credentialvault"
	"github.com/kitdine/agent-deck/internal/desktop"
	"github.com/kitdine/agent-deck/internal/doctor"
	"github.com/kitdine/agent-deck/internal/errdefs"
	"github.com/kitdine/agent-deck/internal/extension"
	"github.com/kitdine/agent-deck/internal/output"
	"github.com/kitdine/agent-deck/internal/platform"
	"github.com/kitdine/agent-deck/internal/provider"
	"github.com/kitdine/agent-deck/internal/session"
	"github.com/kitdine/agent-deck/internal/shellconfig"
	"github.com/kitdine/agent-deck/internal/store"
	"github.com/kitdine/agent-deck/internal/usage"
	"github.com/kitdine/agent-deck/internal/usagehook"
	"github.com/kitdine/agent-deck/internal/watch"
)

var userHomeDir = os.UserHomeDir
var sleepForHookReconciliation = time.Sleep
var detectInvokingShell = shellconfig.DetectInvokingShell
var newShellConfigManager = shellconfig.New
var parentProcessID = os.Getppid

// displayLocation is the zone human-readable output uses. Usage reports also
// resolve their local dates and buckets in it. It is a seam for the same reason
// userHomeDir is: tests need a fixed frame, and the machine zone cannot be
// pinned any other way once the process is running, because TZ is read only on
// the first resolution of time.Local.
var displayLocation = func() *time.Location { return time.Local }
var machineIdentity credentialvault.MachineIdentity = platform.MachineIdentity
var newCredentialVault = func(stateRoot string) provider.CredentialVault {
	return credentialvault.New(stateRoot, machineIdentity)
}
var credentialIsTerminal = func(file *os.File) bool { return term.IsTerminal(int(file.Fd())) }
var credentialReadPassword = term.ReadPassword
var usageProgressIsTerminal = func(w io.Writer) bool {
	file, ok := w.(*os.File)
	return ok && term.IsTerminal(int(file.Fd()))
}
var shellSetupIsTerminal = func(w io.Writer) bool {
	file, ok := w.(*os.File)
	return ok && term.IsTerminal(int(file.Fd()))
}
var newUsageProgress = func(stderr io.Writer, quiet bool) usage.ScanProgressReporter {
	return newUsageProgressOutput(stderr, quiet, usageProgressIsTerminal(stderr))
}

var newSessionProgress = func(stderr io.Writer, quiet bool) session.ScanProgressReporter {
	return newSessionProgressOutput(stderr, quiet, usageProgressIsTerminal(stderr))
}

type sessionUsageContextKey struct{}

func sessionUsageFromContext(ctx context.Context) (*usage.Service, bool) {
	service, ok := ctx.Value(sessionUsageContextKey{}).(*usage.Service)
	return service, ok
}

type commandOptions struct {
	stateDir string
	format   string
	quiet    bool
	verbose  bool
	noColor  bool
	stdin    io.Reader
	stdout   io.Writer
	stderr   io.Writer
}

type inputError struct {
	err error
}

const (
	usageProgressInitialDelay = time.Second
	usageProgressRefresh      = 250 * time.Millisecond
)

type usageProgressTimer interface {
	Chan() <-chan time.Time
	Stop()
}

type usageProgressTicker interface {
	Chan() <-chan time.Time
	Stop()
}

type usageProgressClock interface {
	NewTimer(time.Duration) usageProgressTimer
	NewTicker(time.Duration) usageProgressTicker
}

type realUsageProgressClock struct{}

type realUsageProgressTimer struct{ *time.Timer }

func (timer realUsageProgressTimer) Chan() <-chan time.Time { return timer.C }
func (timer realUsageProgressTimer) Stop()                  { timer.Timer.Stop() }

type realUsageProgressTicker struct{ *time.Ticker }

func (ticker realUsageProgressTicker) Chan() <-chan time.Time { return ticker.C }
func (ticker realUsageProgressTicker) Stop()                  { ticker.Ticker.Stop() }

func (realUsageProgressClock) NewTimer(delay time.Duration) usageProgressTimer {
	return realUsageProgressTimer{Timer: time.NewTimer(delay)}
}

func (realUsageProgressClock) NewTicker(interval time.Duration) usageProgressTicker {
	return realUsageProgressTicker{Ticker: time.NewTicker(interval)}
}

type usageProgressOutput struct {
	stderr   io.Writer
	quiet    bool
	terminal bool
	clock    usageProgressClock

	mu              sync.Mutex
	progress        usage.ScanProgress
	sessionProgress session.ScanProgress
	sessionMode     bool
	emitted         bool
	done            chan struct{}
	wg              sync.WaitGroup
}

type sessionProgressOutput struct{ *usageProgressOutput }

func newUsageProgressOutput(stderr io.Writer, quiet, terminal bool) *usageProgressOutput {
	return newUsageProgressOutputWithClock(stderr, quiet, terminal, realUsageProgressClock{})
}

func newUsageProgressOutputWithClock(stderr io.Writer, quiet, terminal bool, clock usageProgressClock) *usageProgressOutput {
	return &usageProgressOutput{stderr: stderr, quiet: quiet, terminal: terminal, clock: clock}
}

func newSessionProgressOutput(stderr io.Writer, quiet, terminal bool) *sessionProgressOutput {
	return newSessionProgressOutputWithClock(stderr, quiet, terminal, realUsageProgressClock{})
}

func newSessionProgressOutputWithClock(stderr io.Writer, quiet, terminal bool, clock usageProgressClock) *sessionProgressOutput {
	return &sessionProgressOutput{usageProgressOutput: newUsageProgressOutputWithClock(stderr, quiet, terminal, clock)}
}

func (p *usageProgressOutput) Start() {
	if p.quiet {
		return
	}
	p.mu.Lock()
	if p.done != nil {
		p.mu.Unlock()
		return
	}
	p.done = make(chan struct{})
	done := p.done
	timer := p.clock.NewTimer(usageProgressInitialDelay)
	p.wg.Add(1)
	p.mu.Unlock()
	go func() {
		defer p.wg.Done()
		defer timer.Stop()
		select {
		case <-done:
			return
		case <-timer.Chan():
		}
		p.writeProgress()
		ticker := p.clock.NewTicker(usageProgressRefresh)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.Chan():
				p.writeProgress()
			}
		}
	}()
}

func (p *usageProgressOutput) Update(progress usage.ScanProgress) {
	p.mu.Lock()
	p.progress = progress
	p.mu.Unlock()
}

func (p *sessionProgressOutput) Update(progress session.ScanProgress) {
	p.mu.Lock()
	p.sessionMode = true
	p.sessionProgress = progress
	p.mu.Unlock()
}

func (p *usageProgressOutput) Stop() {
	if p.quiet {
		return
	}
	p.mu.Lock()
	done := p.done
	if done == nil {
		p.mu.Unlock()
		return
	}
	p.done = nil
	close(done)
	p.mu.Unlock()
	p.wg.Wait()
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.emitted {
		return
	}
	p.writeProgressLocked()
	if p.terminal {
		_, _ = io.WriteString(p.stderr, "\n")
	}
}

func (p *usageProgressOutput) writeProgress() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.writeProgressLocked()
}

func (p *usageProgressOutput) writeProgressLocked() {
	if p.sessionMode {
		if p.sessionProgress.Total == 0 {
			return
		}
		message := fmt.Sprintf("session scan: %d/%d source files, %d documents, %d skipped", p.sessionProgress.Processed, p.sessionProgress.Total, p.sessionProgress.Documents, p.sessionProgress.Skipped)
		p.writeProgressMessageLocked(message)
		return
	}
	if p.progress.Total == 0 {
		return
	}
	message := fmt.Sprintf("usage scan: %d/%d source files", p.progress.Processed, p.progress.Total)
	if p.progress.Reason != "" {
		message += " (" + p.progress.Reason + ")"
	}
	p.writeProgressMessageLocked(message)
}

func (p *usageProgressOutput) writeProgressMessageLocked(message string) {
	if p.terminal {
		_, _ = fmt.Fprintf(p.stderr, "\r\x1b[2K%s", message)
	} else {
		_, _ = fmt.Fprintf(p.stderr, "%s\n", message)
	}
	p.emitted = true
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
	var notFound *errdefs.NotFound
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
	case errors.As(err, &notFound):
		return notFound.Code
	case errors.Is(err, store.ErrStateBusy):
		return store.ErrStateBusy.Code
	case errors.Is(err, desktop.ErrUnsupportedWireVersion):
		return desktop.ErrUnsupportedWireVersion.Error()
	case errors.Is(err, desktop.ErrInvalidRecentLimit):
		return desktop.ErrInvalidRecentLimit.Error()
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

func sessionShowNotFound(ctx context.Context, opts *commandOptions, client, sessionID string) error {
	message := fmt.Sprintf("no session %q is known", sessionID)
	stateRoot, stateErr := opts.stateRoot()
	if stateErr == nil {
		// Ordinary read-only mode must observe current WAL-backed usage rows. SQLite
		// may materialize owner-only WAL/SHM sidecars, but it does not create the
		// core database or change committed database contents.
		core, err := store.OpenReadOnly(ctx, stateRoot)
		if err != nil {
			return errdefs.NewNotFound(session.CodeSessionNotFound, message, sql.ErrNoRows)
		}
		defer core.Close()
		query := "SELECT EXISTS(SELECT 1 FROM usage_sessions WHERE session_id=?)"
		args := []any{sessionID}
		if client != "" {
			query = "SELECT EXISTS(SELECT 1 FROM usage_sessions WHERE client=? AND session_id=?)"
			args = []any{client, sessionID}
		}
		var exists bool
		if err = core.DB.QueryRowContext(ctx, query, args...).Scan(&exists); err == nil && exists {
			message = fmt.Sprintf("session %q is missing from the session index; run agentdeck session scan", sessionID)
		}
	}
	return errdefs.NewNotFound(session.CodeSessionNotFound, message, sql.ErrNoRows)
}

func isInputError(err error) bool {
	var target *inputError
	return errors.As(err, &target) || errors.Is(err, provider.ErrInvalidProvider) || errors.Is(err, provider.ErrInvalidMultiplier) || errors.Is(err, desktop.ErrUnsupportedWireVersion) || errors.Is(err, desktop.ErrInvalidRecentLimit)
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
		strings.Contains(message, "if any flags in the group") ||
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
	flags.BoolVar(&opts.verbose, "verbose", false, "Include technical provenance in text output")
	root.Flags().BoolVar(&showVersion, "version", false, "Print build identity")
	root.CompletionOptions.DisableDefaultCmd = true
	root.AddCommand(newProviderCommand(opts), newCredentialCommand(opts), newUsageCommand(opts), newPriceCommand(opts), newSessionCommand(opts), newExtensionCommand(opts), newWatchCommand(opts), newBackupCommand(opts), newDoctorCommand(opts), newDesktopCommand(opts), newStateCommand(opts), newRunCommand(opts), newVersionCommand(opts), newCompletionCommand(opts), newShellCommand(opts), newShellInitCommand(opts))
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
			short: "Switch a client to a provider",
			long:  argumentHelp("Switch a client to a provider and named credential, inferring unique choices. Codex official sets [model_providers.custom].name to official and removes the custom base URL and bearer token. --via writes the provider's configured wrapper URL as the endpoint for this switch only; the credential written is unchanged either way, and the effective route is reported on stderr. A Codex switch advises starting a new or restarting the running session so the file change is applied. A Claude switch advises restarting running sessions because it reads settings live, and, when selecting official, reports any credential source AgentDeck does not own that overrides the selection.", "  name  Existing provider name, or official for the built-in provider."),
			example: "  agentdeck provider use aigocode --client codex --credential work\n" +
				"  agentdeck provider use aigocode --client codex --via\n" +
				"  agentdeck provider use official --client claude",
		},
		"provider set-wrapper": {
			short: "Set or clear a provider's wrapper URL",
			long:  argumentHelp("Store one wrapper URL per provider, including official. The URL is normalized like a Codex-bound credential endpoint, so a final /v1 is removed; Codex adds it back and Claude uses the base unchanged. Storing a wrapper never routes a switch by itself: only 'provider use --via' writes it. Exactly one of --url and --clear is required. --kind declares which protocol the wrapper speaks; it describes the stored URL, so it cannot be combined with --clear.", "  name  Existing provider name, or official for the built-in provider."),
			example: "  agentdeck provider set-wrapper aigocode --url https://127.0.0.1:8788\n" +
				"  agentdeck provider set-wrapper aigocode --url https://127.0.0.1:8788 --kind headroom\n" +
				"  agentdeck provider set-wrapper aigocode --clear",
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
			long:    argumentHelp("Show indexed metadata and approved visible text; --activity reads only safe tool metadata from the selected source on demand.", "  session-id  Session identifier returned by session list or search."),
			example: "  agentdeck session show 019abc123 --client codex --activity",
		},
		"session exclude": {
			short:   "Exclude a session source from indexing",
			long:    "Persist an exclusion for future session scans and rebuilds. Both --kind and --value are required.",
			example: "  agentdeck session exclude --kind client --value claude\n  agentdeck session exclude --kind path --value /private/project",
		},
		"extension scan":   {short: "Scan native Codex and Claude extensions"},
		"state migrate":    {short: "Migrate the local AgentDeck state schema"},
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
		"usage scan":        {short: "Incrementally import local usage events"},
		"usage summary":     {short: "Summarize local usage and cost", long: argumentHelp("Summarize all history or a local calendar period.", "  period  Optional daily, weekly, or monthly shortcut."), example: "  agentdeck usage summary weekly"},
		"usage stats":       {short: "Show usage, cache hits, models, and activity", long: "Show responsive usage analytics. Use --model with --activity for safe model-level tool summaries.", example: "  agentdeck usage stats --model gpt-5.4 --activity"},
		"usage sessions":    {short: "List session-level usage and cost"},
		"usage diagnose":    {short: "Diagnose usage attribution and source coverage"},
		"usage rebuild":     {short: "Rebuild usage metadata from local sources"},
		"usage hook setup":  {short: "Install runtime attribution hooks", long: argumentHelp("Install only AgentDeck-owned command hooks into Codex and Claude configuration. Existing unrelated hooks are preserved; Codex may require a manual trust approval through /hooks.", " --client codex|claude|all Client configuration to update."), example: " agentdeck usage hook setup\n agentdeck usage hook setup --client codex"},
		"usage hook status": {short: "Inspect runtime attribution hooks", long: argumentHelp("Report managed hook configuration state and client-side activation or trust guidance without changing configuration.", " --client codex|claude|all Client configuration to inspect."), example: " agentdeck usage hook status"},
		"usage hook remove": {short: "Remove runtime attribution hooks", long: argumentHelp("Remove only byte-equivalent AgentDeck-owned command hooks and preserve unrelated configuration.", " --client codex|claude|all Client configuration to update."), example: " agentdeck usage hook remove --client claude"},
		"price history":     {short: "List price catalog provenance history"},
		"price list":        {short: "List current effective model prices", long: argumentHelp("List component-wise merged prices in USD per one million tokens.", "  model  Optional exact model filter."), example: "  agentdeck price list gpt-5.6-sol"},
		"price status":      {short: "Show active price catalog provenance"},
		"price update":      {short: "Download the latest LiteLLM price catalog"},
		"price override":    {short: "Apply a local structured price override"},
		"run": {
			short:   "Run Codex or Claude with usage attribution",
			long:    argumentHelp("Run a supported client and attribute the resulting local usage events.", "  codex|claude       Client executable to launch.\n  client arguments   Arguments passed unchanged after the required -- separator."),
			example: "  agentdeck run codex -- --help\n  agentdeck run claude -- --version",
		},
		"completion": {short: "Generate shell completion", long: argumentHelp("Generate completion for a supported shell.", "  bash|fish|zsh  Supported shell."), example: "  agentdeck completion zsh"},
		"shell-init": {
			short: "Print shell wrapper functions to stdout; does not install or activate them",
			long: argumentHelp(
				"Print sourceable codex and claude wrapper functions to stdout. Running this command alone changes no shell state and writes no file. The wrappers inject attribution only for an eligible Headroom route; otherwise they invoke the real client unchanged. Use 'agentdeck shell setup' for persistent installation.",
				"  bash|fish|zsh  Supported shell.",
			),
			example: "  eval \"$(agentdeck shell-init bash)\"\n" +
				"  agentdeck shell-init fish | source\n" +
				"  eval \"$(agentdeck shell-init zsh)\"",
		},
		"shell env": {
			short:   "Resolve one client's project attribution environment",
			long:    argumentHelp("Print the current project attribution value for an eligible Headroom route. Ineligible or unreadable state produces no output so shell wrappers fail open.", "  codex|claude  Supported client."),
			example: "  agentdeck shell env codex\n  agentdeck shell env claude",
		},
		"shell setup": {
			short:   "Install project attribution shell integration",
			long:    argumentHelp("Configure project attribution wrappers persistently. With no shell selection, every shell in use is covered by default.", "  bash|fish|zsh  Optional shell; equivalent to --shell."),
			example: "  agentdeck shell setup\n  agentdeck shell setup zsh\n  agentdeck shell setup --shell fish",
		},
		"shell status": {
			short:   "Inspect project attribution shell integration",
			long:    argumentHelp("Inspect persistent configuration, current-session activation, and route eligibility. With no shell selection, every shell in use is covered by default.", "  bash|fish|zsh  Optional shell; equivalent to --shell."),
			example: "  agentdeck shell status\n  agentdeck shell status --shell zsh",
		},
		"shell remove": {
			short:   "Remove project attribution shell integration",
			long:    argumentHelp("Remove AgentDeck-owned project attribution blocks. With no shell selection, every configured shell is covered by default.", "  bash|fish|zsh  Optional shell; equivalent to --shell."),
			example: "  agentdeck shell remove\n  agentdeck shell remove fish",
		},
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

type shellTarget struct {
	shell string
	rc    string
}

const shellLifecycleSurfaceOnlyAnnotation = "agentdeck.shell-lifecycle-surface-only"
const humanInteractiveSurfaceOnlyAnnotation = "agentdeck.human-interactive-surface-only"
const shellSetupDeclinedSetting = "shell.setup.declined"

func newShellCommand(opts *commandOptions) *cobra.Command {
	command := &cobra.Command{
		Use:   "shell",
		Short: "Manage project attribution shell integration",
		Long: "Set up, inspect, or remove project attribution shell integration. " +
			"Setup, status, and remove cover every shell in use by default.",
		Args: exactArgs(0),
	}
	command.AddCommand(
		newShellLifecycleCommand(opts, "setup"),
		newShellLifecycleCommand(opts, "status"),
		newShellLifecycleCommand(opts, "remove"),
		&cobra.Command{
			Use:  "env <codex|claude>",
			Args: exactArgs(1),
			RunE: func(command *cobra.Command, args []string) error {
				return writeProjectEnvironment(command.Context(), opts, provider.Client(args[0]))
			},
		},
	)
	return command
}

func newShellLifecycleCommand(opts *commandOptions, operation string) *cobra.Command {
	var shellFlag string
	var rc string
	var target shellTarget
	command := &cobra.Command{
		Use:         operation + " [bash|fish|zsh]",
		Args:        rangeArgs(0, 1),
		Annotations: map[string]string{shellLifecycleSurfaceOnlyAnnotation: "true"},
		PreRunE: func(_ *cobra.Command, args []string) error {
			resolved, err := resolveShellTarget(args, shellFlag, rc)
			if err != nil {
				return err
			}
			if opts.format != "text" && opts.format != "json" {
				return &inputError{err: fmt.Errorf("shell %s supports only text or json format", operation)}
			}
			target = resolved
			return err
		},
		RunE: func(command *cobra.Command, _ []string) error {
			return runShellLifecycle(command.Context(), opts, operation, target)
		},
	}
	command.Flags().StringVar(&shellFlag, "shell", "", "Restrict the operation to one shell")
	command.Flags().StringVar(&rc, "rc", "", "Use a non-default startup file for one shell")
	return command
}

func runShellLifecycle(ctx context.Context, opts *commandOptions, operation string, target shellTarget) error {
	home, err := userHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}
	shellStateRoot := opts.shellStateRoot()
	invocation := shellconfig.Invocation{}
	needsInvocation := ((operation == "setup" || operation == "status") && target.shell == "") ||
		(target.shell == string(shellconfig.ShellBash) && target.rc == "")
	shouldDetectInvocation := operation == "status" || needsInvocation
	if shouldDetectInvocation {
		detected, detectErr := detectInvokingShell()
		if detectErr != nil {
			if needsInvocation {
				return detectErr
			}
		} else {
			invocation = detected
		}
	}
	manager := newShellConfigManager(shellconfig.Environment{
		Home:          home,
		ZDOTDir:       os.Getenv("ZDOTDIR"),
		XDGConfigHome: os.Getenv("XDG_CONFIG_HOME"),
		StateRoot:     shellStateRoot,
		Invocation:    invocation,
	})
	request := shellconfig.Request{Shell: shellconfig.Shell(target.shell), RC: target.rc}
	var summary shellconfig.Summary
	switch operation {
	case "setup":
		if err := setShellSetupDeclined(ctx, opts, false); err != nil {
			return err
		}
		summary, err = manager.Setup(request)
	case "status":
		return runShellStatus(ctx, opts, manager, request, invocation, target)
	case "remove":
		if err := setShellSetupDeclined(ctx, opts, true); err != nil {
			return err
		}
		summary, err = manager.Remove(request)
	default:
		return fmt.Errorf("shell %s is not available in this build", operation)
	}
	if err != nil {
		return err
	}
	if err := writeShellLifecycleSummary(opts, operation, summary); err != nil {
		return err
	}
	if summary.HasFailures() {
		return fmt.Errorf("shell %s failed for one or more startup files", operation)
	}
	return nil
}

func setShellSetupDeclined(ctx context.Context, opts *commandOptions, declined bool) error {
	database, _, err := opts.openStore(ctx)
	if err != nil {
		return err
	}
	var preferenceErr error
	if declined {
		preferenceErr = database.SetSetting(ctx, shellSetupDeclinedSetting, "1")
	} else {
		preferenceErr = database.DeleteSetting(ctx, shellSetupDeclinedSetting)
	}
	return errors.Join(preferenceErr, database.Close())
}

type shellEligibilityStatus struct {
	Client provider.Client                  `json:"client"`
	Reason provider.ProjectRouteEligibility `json:"reason"`
	Error  string                           `json:"error,omitempty"`
}

type shellGateStatus struct {
	Required   bool   `json:"required"`
	Present    bool   `json:"present"`
	Consistent bool   `json:"consistent"`
	Error      string `json:"error,omitempty"`
}

type shellStatusOutput struct {
	Shells      []shellconfig.StatusResult `json:"shells"`
	Eligibility []shellEligibilityStatus   `json:"eligibility"`
	Gate        shellGateStatus            `json:"negative_gate"`
}

func runShellStatus(
	ctx context.Context,
	opts *commandOptions,
	manager *shellconfig.Manager,
	request shellconfig.Request,
	invocation shellconfig.Invocation,
	target shellTarget,
) error {
	markers := map[shellconfig.Shell]string{}
	for _, shell := range []shellconfig.Shell{
		shellconfig.ShellBash,
		shellconfig.ShellFish,
		shellconfig.ShellZsh,
	} {
		markers[shell] = os.Getenv(shellconfig.ActivationMarkerName(shell))
	}
	status, err := manager.Status(request, markers, parentProcessID())
	if err != nil {
		return err
	}
	output := shellStatusOutput{
		Shells:      status.Results,
		Eligibility: shellEligibility(ctx, opts),
		Gate:        shellAttributionGate(ctx, opts),
	}
	if opts.format == "json" {
		return writeResult(opts.stdout, opts.format, "shell.status", output)
	}
	return writeShellStatusText(opts, output, invocation, target)
}

func shellAttributionGate(ctx context.Context, opts *commandOptions) shellGateStatus {
	stateDir, err := opts.stateRoot()
	if err != nil {
		return shellGateStatus{Error: err.Error()}
	}
	database, err := store.OpenReadOnly(ctx, stateDir)
	if err != nil {
		return shellGateStatus{Error: err.Error()}
	}
	defer database.Close()

	status, err := (provider.Service{
		Store:     database,
		StateRoot: stateDir,
	}).ProjectAttributionGateStatus(ctx)
	result := shellGateStatus{
		Required:   status.Required,
		Present:    status.Present,
		Consistent: status.Consistent,
	}
	if err != nil {
		result.Error = err.Error()
	}
	return result
}

func shellEligibility(ctx context.Context, opts *commandOptions) []shellEligibilityStatus {
	results := []shellEligibilityStatus{
		{Client: provider.ClientCodex},
		{Client: provider.ClientClaude},
	}
	stateDir, err := opts.stateRoot()
	if err != nil {
		return undeterminedShellEligibility(results, err)
	}
	database, err := store.OpenReadOnly(ctx, stateDir)
	if err != nil {
		return undeterminedShellEligibility(results, err)
	}
	defer database.Close()

	service := provider.Service{Store: database}
	for index := range results {
		reason, eligibilityErr := service.ProjectRouteEligibility(ctx, results[index].Client)
		results[index].Reason = reason
		if eligibilityErr != nil {
			results[index].Error = eligibilityErr.Error()
		}
	}
	return results
}

func undeterminedShellEligibility(results []shellEligibilityStatus, err error) []shellEligibilityStatus {
	for index := range results {
		results[index].Reason = provider.ProjectRouteUndetermined
		results[index].Error = err.Error()
	}
	return results
}

func writeShellStatusText(
	opts *commandOptions,
	status shellStatusOutput,
	invocation shellconfig.Invocation,
	target shellTarget,
) error {
	if opts.quiet {
		for _, result := range status.Shells {
			if result.Error != "" {
				if _, err := fmt.Fprintf(opts.stdout, "%s: %s\n", result.Shell, result.Error); err != nil {
					return err
				}
			}
		}
		for _, eligibility := range status.Eligibility {
			if eligibility.Error != "" {
				if _, err := fmt.Fprintf(opts.stdout, "%s: %s\n", eligibility.Client, eligibility.Error); err != nil {
					return err
				}
			}
		}
		return nil
	}

	for _, result := range status.Shells {
		activation := string(result.Activation)
		if result.Activation == shellconfig.ActivationInherited {
			activation = "inactive (marker inherited from an ancestor shell)"
		}
		if _, err := fmt.Fprintf(
			opts.stdout,
			"%s  %s  %s  session: %s",
			result.Shell,
			result.Path,
			result.Configuration,
			activation,
		); err != nil {
			return err
		}
		if result.Error != "" {
			if _, err := fmt.Fprintf(opts.stdout, " (%s)", result.Error); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(opts.stdout); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprint(opts.stdout, "Attribution: "); err != nil {
		return err
	}
	for index, eligibility := range status.Eligibility {
		if index > 0 {
			if _, err := fmt.Fprint(opts.stdout, "; "); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(
			opts.stdout,
			"%s %s",
			eligibility.Client,
			shellEligibilityDescription(eligibility),
		); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(opts.stdout); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(
		opts.stdout,
		"Attribution gate: %s\n",
		shellGateDescription(status.Gate),
	); err != nil {
		return err
	}

	nextShell := shellconfig.Shell(target.shell)
	if nextShell == "" {
		nextShell = invocation.Shell
	}
	for _, result := range status.Shells {
		if result.Shell == nextShell && result.Activation == shellconfig.ActivationActive {
			return nil
		}
	}
	if nextShell == shellconfig.ShellFish {
		_, err := fmt.Fprintln(opts.stdout, "Next action: agentdeck shell-init fish | source")
		return err
	}
	if nextShell == shellconfig.ShellBash || nextShell == shellconfig.ShellZsh {
		_, err := fmt.Fprintf(opts.stdout, "Next action: eval \"$(agentdeck shell-init %s)\"\n", nextShell)
		return err
	}
	return nil
}

func shellGateDescription(status shellGateStatus) string {
	if status.Error != "" {
		return "undetermined (" + status.Error + ")"
	}
	if status.Consistent {
		if status.Present {
			return "ready"
		}
		return "not required"
	}
	if status.Required {
		return "missing (eligible route exists; rerun the intended provider use command)"
	}
	return "stale (no eligible route remains; rerun the intended provider use command)"
}

func shellEligibilityDescription(status shellEligibilityStatus) string {
	switch status.Reason {
	case provider.ProjectRouteEligible:
		return "eligible"
	case provider.ProjectRouteNoWrapper:
		return "has no wrapper route"
	case provider.ProjectRouteWrapperNotHeadroom:
		return "wrapper is not declared headroom"
	case provider.ProjectRouteEndpointDrifted:
		return "wrapper endpoint has drifted"
	case provider.ProjectRouteUndetermined:
		if status.Error != "" {
			return "undetermined (" + status.Error + ")"
		}
		return "undetermined"
	default:
		return string(status.Reason)
	}
}

func writeShellLifecycleSummary(opts *commandOptions, operation string, summary shellconfig.Summary) error {
	if opts.format == "json" {
		return writeResult(opts.stdout, opts.format, "shell."+operation, summary)
	}
	deactivationPrinted := map[shellconfig.Shell]bool{}
	for _, result := range summary.Results {
		if opts.quiet && result.Outcome != shellconfig.OutcomeFailed {
			continue
		}
		switch result.Outcome {
		case shellconfig.OutcomeSkipped:
			if _, err := fmt.Fprintf(opts.stdout, "%s: skipped %s (no startup file and not the invoking shell)\n", result.Shell, result.Path); err != nil {
				return err
			}
		case shellconfig.OutcomeFailed:
			if _, err := fmt.Fprintf(opts.stdout, "%s: failed %s: %s\n", result.Shell, result.Path, result.Error); err != nil {
				return err
			}
		default:
			if _, err := fmt.Fprintf(opts.stdout, "%s: %s %s\n", result.Shell, result.Outcome, result.Path); err != nil {
				return err
			}
		}
		if operation == "remove" &&
			(result.Outcome == shellconfig.OutcomeRemoved ||
				result.Outcome == shellconfig.OutcomeAbsent) &&
			!deactivationPrinted[result.Shell] {
			command, ok := shellDeactivationCommand(result.Shell)
			if ok {
				if _, err := fmt.Fprintf(
					opts.stdout,
					"Current session deactivation for %s: %s\n",
					result.Shell,
					command,
				); err != nil {
					return err
				}
				deactivationPrinted[result.Shell] = true
			}
		}
	}
	return nil
}

func shellDeactivationCommand(shell shellconfig.Shell) (string, bool) {
	switch shell {
	case shellconfig.ShellBash:
		return "if declare -F codex >/dev/null; then unset -f codex; fi; " +
			"if declare -F claude >/dev/null; then unset -f claude; fi", true
	case shellconfig.ShellFish:
		return "if functions -q codex; functions --erase codex; end; " +
			"if functions -q claude; functions --erase claude; end", true
	case shellconfig.ShellZsh:
		return "if (( $+functions[codex] )); then unfunction codex; fi; " +
			"if (( $+functions[claude] )); then unfunction claude; fi", true
	default:
		return "", false
	}
}

func resolveShellTarget(args []string, shellFlag, rc string) (shellTarget, error) {
	if len(args) > 1 {
		return shellTarget{}, &inputError{err: fmt.Errorf("accepts between 0 and 1 arg(s), received %d", len(args))}
	}
	if len(args) == 1 && shellFlag != "" {
		return shellTarget{}, &inputError{err: fmt.Errorf("specify a shell either positionally or with --shell, not both")}
	}
	shell := shellFlag
	if len(args) == 1 {
		shell = args[0]
	}
	if shell != "" && shell != "bash" && shell != "fish" && shell != "zsh" {
		return shellTarget{}, &inputError{err: fmt.Errorf("unsupported shell %q", shell)}
	}
	if rc != "" && shell == "" {
		return shellTarget{}, &inputError{err: fmt.Errorf("--rc requires exactly one shell, specified positionally or with --shell")}
	}
	return shellTarget{shell: shell, rc: rc}, nil
}

func requireTextFormat(opts *commandOptions, command string) error {
	if opts.format != "text" {
		return &inputError{err: fmt.Errorf("%s requires text format", command)}
	}
	return nil
}

func newShellInitCommand(opts *commandOptions) *cobra.Command {
	var projectEnvironment string
	command := &cobra.Command{
		Use:    "shell-init <bash|fish|zsh>",
		Short:  "Print shell wrapper functions to stdout; does not install or activate them",
		Hidden: true,
		Args: func(command *cobra.Command, args []string) error {
			// The hidden resolver is shell-independent; only public script
			// generation takes a shell positional argument.
			if projectEnvironment != "" {
				return exactArgs(0)(command, args)
			}
			return exactArgs(1)(command, args)
		},
		RunE: func(command *cobra.Command, args []string) error {
			if projectEnvironment != "" {
				return writeProjectEnvironment(command.Context(), opts, provider.Client(projectEnvironment))
			}
			shell := args[0]
			if shell != "bash" && shell != "fish" && shell != "zsh" {
				return &inputError{err: fmt.Errorf("unsupported shell %q", shell)}
			}
			if err := requireTextFormat(opts, "shell-init"); err != nil {
				return err
			}
			stateDir, err := opts.stateRoot()
			if err != nil {
				return err
			}
			_, err = io.WriteString(
				opts.stdout,
				shellInitScript(
					shell,
					opts.shellStateRoot(),
					provider.ProjectAttributionGatePath(stateDir),
				),
			)
			return err
		},
	}
	command.Flags().StringVar(&projectEnvironment, "project-environment", "", "Resolve one client's project environment")
	_ = command.Flags().MarkHidden("project-environment")
	return command
}

func writeProjectEnvironment(ctx context.Context, opts *commandOptions, client provider.Client) error {
	if client != provider.ClientCodex && client != provider.ClientClaude {
		return &inputError{err: fmt.Errorf("unsupported client %q", client)}
	}
	if err := requireTextFormat(opts, "shell project environment resolver"); err != nil {
		return err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil
	}
	stateDir, err := opts.stateRoot()
	if err != nil {
		return nil
	}
	database, err := store.OpenReadOnly(ctx, stateDir)
	if err != nil {
		return nil
	}
	defer database.Close()
	service := provider.Service{Store: database}
	environment, changed := service.RunProjectEnvironment(ctx, client, cwd, os.Environ())
	if !changed {
		return nil
	}
	name := provider.HeadroomProjectEnvironment
	if client == provider.ClientClaude {
		name = provider.ClaudeCustomHeadersEnvironment
	}
	value, found := environmentVariableValue(environment, name)
	if !found {
		return nil
	}
	_, err = io.WriteString(opts.stdout, value)
	return err
}

func environmentVariableValue(environment []string, name string) (string, bool) {
	prefix := name + "="
	for i := len(environment) - 1; i >= 0; i-- {
		if strings.HasPrefix(environment[i], prefix) {
			return strings.TrimPrefix(environment[i], prefix), true
		}
	}
	return "", false
}

func shellInitScript(shell, stateRoot, gatePath string) string {
	marker := shellconfig.ActivationMarkerName(shellconfig.Shell(shell))
	quotedGatePath := shellQuote(gatePath)
	agentdeck := "command agentdeck"
	if stateRoot != "" {
		agentdeck += " --state-dir " + shellQuote(stateRoot)
	}
	if shell == "fish" {
		return fmt.Sprintf(`# Generated by agentdeck shell-init fish.
# Requires fish 3.4 or newer.
set -gx %s $fish_pid
function codex
    set -l _agentdeck_project_value
    set -l _agentdeck_project_status 1
    if type -q agentdeck; and test -f %s
        set _agentdeck_project_value "$(%s shell-init --project-environment codex 2>/dev/null)"
        set _agentdeck_project_status $status
    end
    if test $_agentdeck_project_status -eq 0; and test -n "$_agentdeck_project_value"
        env HEADROOM_PROJECT="$_agentdeck_project_value" codex $argv
    else
        command codex $argv
    end
end

function claude
    set -l _agentdeck_project_value
    set -l _agentdeck_project_status 1
    if type -q agentdeck; and test -f %s
        set _agentdeck_project_value "$(%s shell-init --project-environment claude 2>/dev/null)"
        set _agentdeck_project_status $status
    end
    if test $_agentdeck_project_status -eq 0; and test -n "$_agentdeck_project_value"
        env ANTHROPIC_CUSTOM_HEADERS="$_agentdeck_project_value" claude $argv
    else
        command claude $argv
    end
end
`, marker, quotedGatePath, agentdeck, quotedGatePath, agentdeck)
	}
	return fmt.Sprintf(`# Generated by agentdeck shell-init %s.
export %s="$$"
codex() {
    local _agentdeck_project_value
    if command -v agentdeck >/dev/null 2>&1 && [ -f %s ] && _agentdeck_project_value="$(%s shell-init --project-environment codex 2>/dev/null)" && [ -n "$_agentdeck_project_value" ]; then
        HEADROOM_PROJECT="$_agentdeck_project_value" command codex "$@"
    else
        command codex "$@"
    fi
}

claude() {
    local _agentdeck_project_value
    if command -v agentdeck >/dev/null 2>&1 && [ -f %s ] && _agentdeck_project_value="$(%s shell-init --project-environment claude 2>/dev/null)" && [ -n "$_agentdeck_project_value" ]; then
        ANTHROPIC_CUSTOM_HEADERS="$_agentdeck_project_value" command claude "$@"
    else
        command claude "$@"
    fi
}
`, shell, marker, quotedGatePath, agentdeck, quotedGatePath, agentdeck)
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

func (o *commandOptions) shellStateRoot() string {
	return o.stateDir
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
	var useVia, noShellSetup bool
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
		previousEligibility, _ := s.ProjectRouteEligibility(ctx, client)
		if err := s.UseCredential(ctx, args[0], client, useCredential, configPath, "", useVia); err != nil {
			return nil, err
		}
		reportEffectiveRoute(ctx, s, opts, client)
		reportSwitchAdvisories(s, opts, client, args[0], configPath)
		configured := maybeSetupShellIntegration(ctx, opts, s, useVia, noShellSetup)
		if !configured {
			reportRouteChangeAttributionGuidance(ctx, opts, s, client, previousEligibility)
		}
		return withTextResource(nil, args[0]), nil
	})}
	use.Flags().StringVar(&configPath, "config-path", "", "Override the automatically resolved client configuration path")
	use.Flags().StringVar(&useClient, "client", "", "Client to switch: codex or claude")
	use.Flags().StringVar(&useCredential, "credential", "", "Credential shorthand, not the generated reference")
	use.Flags().BoolVar(&useVia, "via", false, "Route this switch through the provider's configured wrapper URL")
	use.Flags().BoolVar(&noShellSetup, "no-shell-setup", false, "Do not configure shell integration for this switch")
	var wrapperURL, wrapperKind string
	var clearWrapper bool
	var setWrapper *cobra.Command
	setWrapper = &cobra.Command{Use: "set-wrapper <name>", Args: cobra.ExactArgs(1), RunE: withService(func(ctx context.Context, s provider.Service, args []string) (any, error) {
		urlGiven := setWrapper.Flags().Changed("url")
		if urlGiven == clearWrapper {
			if clearWrapper {
				return nil, &inputError{err: fmt.Errorf("--url and --clear are mutually exclusive")}
			}
			return nil, &inputError{err: fmt.Errorf("--url or --clear is required")}
		}
		// --kind describes the URL being stored, so it has nothing to describe
		// while clearing. Rejecting the combination is clearer than silently
		// dropping a flag the user typed.
		if clearWrapper && setWrapper.Flags().Changed("kind") {
			return nil, &inputError{err: fmt.Errorf("--kind cannot be combined with --clear")}
		}
		result, dropped, err := s.SetWrapper(ctx, args[0], wrapperURL, wrapperKind, clearWrapper)
		if err != nil {
			return nil, err
		}
		reportDroppedWrapperKind(opts, dropped)
		if result.Definition.WrapperKind == provider.WrapperKindHeadroom {
			reportProjectAttributionGuidance(opts, provider.ProjectAttributionAdvisory)
		}
		return withTextResource(result.Definition, args[0]), nil
	})}
	setWrapper.Flags().StringVar(&wrapperURL, "url", "", "Wrapper base URL; a final /v1 is removed like a Codex-bound endpoint")
	setWrapper.Flags().BoolVar(&clearWrapper, "clear", false, "Remove the provider's wrapper URL")
	setWrapper.Flags().StringVar(&wrapperKind, "kind", "", "Protocol the wrapper speaks: plain (default) or headroom")
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
		use, setWrapper,
		&cobra.Command{Use: "recover", Args: cobra.NoArgs, RunE: withService(func(ctx context.Context, s provider.Service, _ []string) (any, error) { return s.Recover(ctx) })},
	)
	return providerCmd
}

// reportEffectiveRoute prints the route a completed switch actually wrote to
// stderr, because --via and a direct switch are indistinguishable in the
// client file once written. It reads the recorded selection rather than the
// requested flag so the line describes what was written, not what was asked
// for. The line is informational: it never changes the exit status, never
// enters the JSON envelope on stdout, and a read-back failure drops the line
// instead of failing a switch that already succeeded.
func reportEffectiveRoute(ctx context.Context, s provider.Service, opts *commandOptions, client provider.Client) {
	if opts.quiet || opts.stderr == nil {
		return
	}
	selections, err := s.Current(ctx)
	if err != nil {
		return
	}
	for _, selection := range selections {
		if selection.Client != string(client) {
			continue
		}
		route := "direct"
		if selection.ViaWrapper {
			route = "via wrapper"
		}
		if selection.Endpoint == "" {
			_, _ = fmt.Fprintf(opts.stderr, "effective route: %s %s, no endpoint written\n", selection.Client, route)
			return
		}
		_, _ = fmt.Fprintf(opts.stderr, "effective route: %s %s, endpoint %s\n", selection.Client, route, selection.Endpoint)
		return
	}
}

// reportSwitchAdvisories prints the informational notes a completed switch
// carries to stderr, under the same rules as the effective-route line: never
// on stdout, never in the JSON envelope, never affecting the exit status, and
// suppressed by --quiet.
func reportSwitchAdvisories(s provider.Service, opts *commandOptions, client provider.Client, name, configPath string) {
	if opts.quiet || opts.stderr == nil {
		return
	}
	for _, advisory := range s.SwitchAdvisories(client, name, configPath) {
		_, _ = fmt.Fprintf(opts.stderr, "advisory: %s\n", advisory)
	}
}

func reportProjectAttributionGuidance(opts *commandOptions, advisory string) {
	if advisory == "" || opts.quiet || opts.stderr == nil {
		return
	}
	_, _ = fmt.Fprintf(opts.stderr, "advisory: %s\n", advisory)
}

func maybeSetupShellIntegration(
	ctx context.Context,
	opts *commandOptions,
	s provider.Service,
	via, suppressed bool,
) bool {
	if !via ||
		suppressed ||
		opts.quiet ||
		opts.format != "text" ||
		opts.stderr == nil ||
		!shellSetupIsTerminal(opts.stderr) {
		return false
	}
	if _, found, err := s.Store.Setting(ctx, shellSetupDeclinedSetting); err != nil || found {
		return false
	}
	eligible := false
	for _, client := range []provider.Client{provider.ClientCodex, provider.ClientClaude} {
		status, err := s.ProjectRouteEligibility(ctx, client)
		if err == nil && status == provider.ProjectRouteEligible {
			eligible = true
			break
		}
	}
	if !eligible {
		return false
	}

	home, err := userHomeDir()
	if err != nil {
		return false
	}
	shellStateRoot := opts.shellStateRoot()
	invocation, err := detectInvokingShell()
	if err != nil {
		return false
	}
	manager := newShellConfigManager(shellconfig.Environment{
		Home:          home,
		ZDOTDir:       os.Getenv("ZDOTDIR"),
		XDGConfigHome: os.Getenv("XDG_CONFIG_HOME"),
		StateRoot:     shellStateRoot,
		Invocation:    invocation,
	})
	summary, err := manager.SetupIfUnconfigured(shellconfig.Request{})
	if err != nil || summary.HasFailures() {
		return false
	}
	if !summaryHasOutcome(summary, shellconfig.OutcomeConfigured) {
		return false
	}

	automatic := *opts
	automatic.format = "text"
	automatic.quiet = false
	automatic.stdout = opts.stderr
	configured := shellconfig.Summary{Results: make([]shellconfig.Result, 0, len(summary.Results))}
	for _, result := range summary.Results {
		if result.Outcome == shellconfig.OutcomeConfigured {
			configured.Results = append(configured.Results, result)
		}
	}
	_ = writeShellLifecycleSummary(&automatic, "setup", configured)
	_, _ = fmt.Fprintf(
		opts.stderr,
		"new shell sessions are covered; activate current %s session: %s\n",
		invocation.Shell,
		shellActivationCommand(invocation.Shell, shellStateRoot),
	)
	return true
}

func summaryHasOutcome(summary shellconfig.Summary, outcome shellconfig.Outcome) bool {
	for _, result := range summary.Results {
		if result.Outcome == outcome {
			return true
		}
	}
	return false
}

func reportRouteChangeAttributionGuidance(
	ctx context.Context,
	opts *commandOptions,
	s provider.Service,
	client provider.Client,
	previous provider.ProjectRouteEligibility,
) {
	if opts.quiet || opts.stderr == nil {
		return
	}
	current, err := s.ProjectRouteEligibility(ctx, client)
	if err != nil {
		return
	}
	configured, invocation, shellStateRoot := shellIntegrationConfigured(opts)
	advisory := ""
	switch {
	case current == provider.ProjectRouteEligible && configured:
		advisory = fmt.Sprintf(
			"project attribution is in effect; configured shell startup files carry it in new sessions; activate this %s session: %s; see %s",
			invocation.Shell,
			shellActivationCommand(invocation.Shell, shellStateRoot),
			provider.ProjectAttributionGuideURL,
		)
	case current == provider.ProjectRouteEligible:
		advisory = "project attribution needs one-time shell integration setup: agentdeck shell setup; see " +
			provider.ProjectAttributionGuideURL
	case previous == provider.ProjectRouteEligible && configured:
		advisory = "managed shell wrappers remain installed but stop injecting project attribution now; see " +
			provider.ProjectAttributionGuideURL
	}
	if advisory == "" {
		return
	}
	_, _ = fmt.Fprintf(opts.stderr, "advisory: %s\n", advisory)
}

func shellIntegrationConfigured(opts *commandOptions) (bool, shellconfig.Invocation, string) {
	home, err := userHomeDir()
	if err != nil {
		return false, shellconfig.Invocation{}, ""
	}
	shellStateRoot := opts.shellStateRoot()
	invocation, err := detectInvokingShell()
	if err != nil {
		return false, shellconfig.Invocation{}, shellStateRoot
	}
	manager := newShellConfigManager(shellconfig.Environment{
		Home:          home,
		ZDOTDir:       os.Getenv("ZDOTDIR"),
		XDGConfigHome: os.Getenv("XDG_CONFIG_HOME"),
		StateRoot:     shellStateRoot,
		Invocation:    invocation,
	})
	status, err := manager.Status(shellconfig.Request{}, nil, parentProcessID())
	if err != nil {
		return false, invocation, shellStateRoot
	}
	configured := false
	for _, result := range status.Results {
		if result.Error != "" {
			return false, invocation, shellStateRoot
		}
		configured = configured || result.Configuration == shellconfig.ConfigurationConfigured
	}
	return configured, invocation, shellStateRoot
}

func shellActivationCommand(shell shellconfig.Shell, stateRoot string) string {
	agentdeck := "agentdeck"
	if stateRoot != "" {
		agentdeck += " --state-dir " + shellQuote(stateRoot)
	}
	if shell == shellconfig.ShellFish {
		return agentdeck + " shell-init fish | source"
	}
	return fmt.Sprintf(`eval "$(%s shell-init %s)"`, agentdeck, shell)
}

// reportDroppedWrapperKind names a non-default wrapper declaration that a
// set-wrapper call replaced. set-wrapper sets the whole wrapper rather than
// patching it, so omitting --kind legitimately returns the declaration to the
// default — but a user who only meant to move a proxy to a new address would
// otherwise get no sign that attribution stopped. It follows the same rules as
// every other advisory: stderr only, never in the JSON envelope, no effect on
// the exit status, and suppressed by --quiet.
func reportDroppedWrapperKind(opts *commandOptions, dropped string) {
	if dropped == "" || opts.quiet || opts.stderr == nil {
		return
	}
	_, _ = fmt.Fprintf(opts.stderr, "advisory: wrapper kind reset to %s (was %s); pass --kind %s to keep it\n", provider.WrapperKindPlain, dropped, dropped)
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

type sessionListPage struct {
	Sessions    []session.Metadata            `json:"sessions"`
	Pagination  map[string]session.Pagination `json:"pagination,omitempty"`
	NextCommand string                        `json:"-"`
}
type sessionSearchPage struct {
	Documents   []session.Document            `json:"documents"`
	Pagination  map[string]session.Pagination `json:"pagination"`
	NextCommand string                        `json:"-"`
}

type sessionShowPage struct {
	session.Result
	Usage             *usage.SessionSummary         `json:"usage,omitempty"`
	Invocations       []usage.SessionInvocation     `json:"invocations,omitempty"`
	Pagination        map[string]session.Pagination `json:"pagination,omitempty"`
	NextCommand       string                        `json:"-"`
	ActivityRequested bool                          `json:"-"`
	ActivityWarning   string                        `json:"activity_warning,omitempty"`
	SourceStale       bool                          `json:"stale,omitempty"`
}

type sessionViewerCompleted struct{}

func newSessionCommand(opts *commandOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "session",
		Short:       "Search local sessions",
		Annotations: map[string]string{humanInteractiveSurfaceOnlyAnnotation: "true"},
	}
	var showTokens, showInteractive, sessionInteractive bool
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
			lockReleased := false
			defer func() {
				if !lockReleased {
					_ = lock.Release()
				}
			}()
			sessions, err := store.OpenSessions(command.Context(), stateDir)
			if err != nil {
				return err
			}
			defer sessions.Close()
			home, err := userHomeDir()
			if err != nil {
				return err
			}
			if (command.Name() == "show" && (showTokens || showInteractive)) || (command == cmd && sessionInteractive) {
				core, err := store.OpenWithLockHeld(command.Context(), stateDir)
				if err != nil {
					return err
				}
				defer core.Close()
				command.SetContext(context.WithValue(command.Context(), sessionUsageContextKey{}, usage.New(core, home)))
			}
			if (command.Name() == "show" && showInteractive) || (command == cmd && sessionInteractive) {
				if releaseErr := lock.Release(); releaseErr != nil {
					return releaseErr
				}
				lockReleased = true
			}
			data, err := run(command.Context(), sessions, home, args)
			if err != nil {
				return err
			}
			if _, ok := data.(sessionViewerCompleted); ok {
				return nil
			}
			if command.Name() == "scan" || command.Name() == "rebuild" {
				fingerprint, fingerprintErr := watch.FingerprintRoots(sessionWatchRoots(home)...)
				if fingerprintErr != nil {
					return fingerprintErr
				}
				if releaseErr := lock.Release(); releaseErr != nil {
					return releaseErr
				}
				lockReleased = true
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
	cmd.Args = cobra.NoArgs
	cmd.RunE = func(command *cobra.Command, args []string) error {
		if !sessionInteractive {
			return command.Help()
		}
		return withSessions(func(ctx context.Context, s *store.Store, _ string, _ []string) (any, error) {
			if opts.format != "text" {
				return nil, errors.New("session --interactive requires text format")
			}
			input, inputOK := opts.stdin.(*os.File)
			output, outputOK := opts.stdout.(*os.File)
			if !inputOK || !outputOK {
				return nil, errors.New("session --interactive requires TTY stdin and stdout")
			}
			items, err := session.List(ctx, s.DB)
			if err != nil {
				return nil, err
			}
			service, _ := sessionUsageFromContext(ctx)
			err = runSessionBrowser(ctx, input, output, items, func(metadata session.Metadata) sessionViewerLoad {
				return newSessionViewerLoad(ctx, s, metadata, service)
			}, opts.noColor)
			if err != nil {
				return nil, err
			}
			return sessionViewerCompleted{}, nil
		})(command, args)
	}
	cmd.Flags().BoolVar(&sessionInteractive, "interactive", false, "Browse indexed sessions in the read-only viewer")
	var listClient, searchClient, showClient, excludeKind, excludeValue string
	var listPage, listLimit, searchPage, searchLimit, showPage, showLimit int
	var listAll, searchAll, showAll bool
	var showActivity bool
	var list, search *cobra.Command
	list = &cobra.Command{Use: "list", Args: cobra.NoArgs, RunE: withSessions(func(ctx context.Context, s *store.Store, _ string, _ []string) (any, error) {
		if err := validateOptionalClient(listClient); err != nil {
			return nil, err
		}
		items, err := session.List(ctx, s.DB)
		if err != nil {
			return nil, err
		}
		out := items[:0]
		for _, item := range items {
			if listClient == "" || item.Client == listClient {
				out = append(out, item)
			}
		}
		explicit := list.Flags().Changed("page") || list.Flags().Changed("limit") || list.Flags().Changed("all")
		if opts.format == "json" && !explicit {
			return out, nil
		}
		paged, pagination, pageErr := session.Paginate(out, listPage, listLimit, listAll)
		if pageErr != nil {
			return nil, &inputError{err: pageErr}
		}
		return sessionListPage{Sessions: paged, Pagination: map[string]session.Pagination{"sessions": pagination}, NextCommand: sessionNextCommand(opts.stateDir, "list", listClient, "", false, false, pagination)}, nil
	})}
	list.Flags().StringVar(&listClient, "client", "", "Filter by client")
	list.Flags().IntVar(&listPage, "page", 1, "Page number")
	list.Flags().IntVar(&listLimit, "limit", 20, "Rows per page")
	list.Flags().BoolVar(&listAll, "all", false, "Show all rows")
	list.MarkFlagsMutuallyExclusive("all", "page")
	list.MarkFlagsMutuallyExclusive("all", "limit")
	search = &cobra.Command{Use: "search <query>", Args: cobra.ExactArgs(1), RunE: withSessions(func(ctx context.Context, s *store.Store, _ string, args []string) (any, error) {
		if err := validateOptionalClient(searchClient); err != nil {
			return nil, err
		}
		items, err := session.Search(ctx, s.DB, args[0])
		if err != nil {
			return nil, err
		}
		out := items
		if searchClient != "" {
			out = items[:0]
			for _, item := range items {
				if item.Client == searchClient {
					out = append(out, item)
				}
			}
		}
		explicit := search.Flags().Changed("page") || search.Flags().Changed("limit") || search.Flags().Changed("all")
		if opts.format == "json" && !explicit {
			return out, nil
		}
		paged, pagination, pageErr := session.Paginate(out, searchPage, searchLimit, searchAll)
		if pageErr != nil {
			return nil, &inputError{err: pageErr}
		}
		return sessionSearchPage{Documents: paged, Pagination: map[string]session.Pagination{"search": pagination}, NextCommand: sessionNextCommand(opts.stateDir, "search", searchClient, args[0], false, false, pagination)}, nil
	})}
	search.Flags().StringVar(&searchClient, "client", "", "Filter by client")
	search.Flags().IntVar(&searchPage, "page", 1, "Page number")
	search.Flags().IntVar(&searchLimit, "limit", 20, "Rows per page")
	search.Flags().BoolVar(&searchAll, "all", false, "Show all rows")
	search.MarkFlagsMutuallyExclusive("all", "page")
	search.MarkFlagsMutuallyExclusive("all", "limit")
	var show *cobra.Command
	show = &cobra.Command{Use: "show <session-id>", Args: cobra.ExactArgs(1), RunE: withSessions(func(ctx context.Context, s *store.Store, _ string, args []string) (any, error) {
		if showInteractive {
			if opts.format != "text" {
				return nil, &inputError{err: errors.New("--interactive requires text output")}
			}
			if _, ok := opts.stdin.(*os.File); !ok {
				return nil, &inputError{err: errors.New("--interactive requires TTY stdin and stdout")}
			}
			if _, ok := opts.stdout.(*os.File); !ok {
				return nil, &inputError{err: errors.New("--interactive requires TTY stdin and stdout")}
			}
		}
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
				return nil, sessionShowNotFound(ctx, opts, "", args[0])
			}
		}
		metadata, err := session.ShowMetadata(ctx, s.DB, client, args[0])
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, sessionShowNotFound(ctx, opts, client, args[0])
			}
			return nil, err
		}
		if showInteractive {
			input := opts.stdin.(*os.File)
			output := opts.stdout.(*os.File)
			service, _ := sessionUsageFromContext(ctx)
			if err := runSessionViewer(ctx, input, output, newSessionViewerLoad(ctx, s, metadata, service), opts.noColor); err != nil {
				return nil, err
			}
			return sessionViewerCompleted{}, nil
		}
		pagingExplicit := show.Flags().Changed("page") || show.Flags().Changed("limit") || show.Flags().Changed("all")
		documents, documentsPagination, pageErr := session.DocumentsPage(ctx, s.DB, metadata, showPage, showLimit, showAll || (opts.format == "json" && !pagingExplicit))
		if pageErr != nil {
			return nil, &inputError{err: pageErr}
		}
		result := session.Result{Metadata: metadata, Documents: documents}
		_, sourceErr := os.Stat(metadata.SourcePath)
		sourceStale := errors.Is(sourceErr, os.ErrNotExist)
		activityWarning := ""
		var activityPagination session.Pagination
		if showActivity {
			activityPage, activityErr := activity.ReadDetailsPage(metadata.SourcePath, client, args[0], showPage, showLimit, showAll || (opts.format == "json" && !pagingExplicit))
			if activityErr != nil {
				activityWarning = "Activity source is unavailable."
				if sourceStale {
					activityWarning = "Activity source is stale."
				}
			} else {
				result.Activity = activityPage.Details
				result.ActivitySummary = session.ActivitySummaryFromSourceSummary(activityPage.Summary)
				activityPagination = session.Pagination{Page: activityPage.Page, Limit: activityPage.Limit, Total: activityPage.Total, Shown: activityPage.Shown, HasMore: activityPage.HasMore, NextPage: activityPage.NextPage}
			}
		}
		var sessionUsage *usage.SessionSummary
		var invocations []usage.SessionInvocation
		var invocationPagination session.Pagination
		if showTokens {
			service, ok := sessionUsageFromContext(ctx)
			if !ok {
				return nil, errors.New("session usage service unavailable")
			}
			summary, usageErr := service.SessionUsageSummary(ctx, client, args[0])
			if usageErr != nil {
				return nil, usageErr
			}
			var page usage.InvocationPagination
			pagingExplicit := show.Flags().Changed("page") || show.Flags().Changed("limit") || show.Flags().Changed("all")
			invocations, page, usageErr = service.SessionInvocations(ctx, client, args[0], showPage, showLimit, showAll || (opts.format == "json" && !pagingExplicit))
			if usageErr != nil {
				return nil, &inputError{err: usageErr}
			}
			sessionUsage = &summary
			invocationPagination = session.Pagination{Page: page.Page, Limit: page.Limit, Total: page.Total, Shown: page.Shown, HasMore: page.HasMore, NextPage: page.NextPage}
		}
		if opts.format == "json" && !pagingExplicit {
			if showTokens || (showActivity && activityWarning != "") || sourceStale {
				return sessionShowPage{Result: result, Usage: sessionUsage, Invocations: invocations, ActivityRequested: showActivity, ActivityWarning: activityWarning, SourceStale: sourceStale}, nil
			}
			return result, nil
		}
		pagination := map[string]session.Pagination{"documents": documentsPagination}
		nextPagination := documentsPagination
		if showActivity {
			pagination["activity"] = activityPagination
			if activityPagination.HasMore {
				nextPagination = activityPagination
			}
		}
		if showTokens {
			pagination["invocations"] = invocationPagination
			if invocationPagination.HasMore {
				nextPagination = invocationPagination
			}
		}
		return sessionShowPage{Result: result, Usage: sessionUsage, Invocations: invocations, Pagination: pagination, NextCommand: sessionNextCommand(opts.stateDir, "show", client, args[0], showActivity, showTokens, nextPagination), ActivityRequested: showActivity, ActivityWarning: activityWarning, SourceStale: sourceStale}, nil
	})}
	show.Flags().StringVar(&showClient, "client", "", "Session client")
	show.Flags().BoolVar(&showActivity, "activity", false, "Show safe tool activity metadata from the source log")
	show.Flags().BoolVar(&showTokens, "tokens", false, "Show normalized usage events and pricing")
	show.Flags().BoolVar(&showInteractive, "interactive", false, "Open the read-only interactive session viewer")
	show.Flags().IntVar(&showPage, "page", 1, "Page number")
	show.Flags().IntVar(&showLimit, "limit", 20, "Rows per page")
	show.Flags().BoolVar(&showAll, "all", false, "Show all rows")
	show.MarkFlagsMutuallyExclusive("all", "page")
	show.MarkFlagsMutuallyExclusive("all", "limit")
	show.MarkFlagsMutuallyExclusive("interactive", "page")
	show.MarkFlagsMutuallyExclusive("interactive", "limit")
	show.MarkFlagsMutuallyExclusive("interactive", "all")
	show.MarkFlagsMutuallyExclusive("interactive", "activity")
	show.MarkFlagsMutuallyExclusive("interactive", "tokens")
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
			return session.ScanWithOptions(ctx, s.DB, home, session.ScanOptions{Progress: newSessionProgress(opts.stderr, opts.quiet)})
		})},
		list, search, show, exclude,
		&cobra.Command{Use: "rebuild", Args: cobra.NoArgs, RunE: withSessions(func(ctx context.Context, s *store.Store, home string, _ []string) (any, error) {
			return session.RebuildWithOptions(ctx, s.DB, home, session.ScanOptions{Progress: newSessionProgress(opts.stderr, opts.quiet)})
		})},
		newSessionPurgeCommand(opts),
	)
	return cmd
}

func sessionNextCommand(stateDir, action, client, id string, activity, tokens bool, p session.Pagination) string {
	if !p.HasMore {
		return ""
	}
	parts := []string{"agentdeck"}
	if stateDir != "" {
		parts = append(parts, "--state-dir", shellQuote(stateDir))
	}
	parts = append(parts, "session", action)
	if id != "" {
		parts = append(parts, shellQuote(id))
	}
	if client != "" {
		parts = append(parts, "--client", shellQuote(client))
	}
	if activity {
		parts = append(parts, "--activity")
	}
	if tokens {
		parts = append(parts, "--tokens")
	}
	parts = append(parts, "--page", strconv.Itoa(p.NextPage), "--limit", strconv.Itoa(p.Limit))
	return strings.Join(parts, " ")
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
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

func newStateCommand(opts *commandOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "state", Short: "Maintain AgentDeck state"}
	cmd.AddCommand(&cobra.Command{Use: "migrate", Args: exactArgs(0), RunE: func(command *cobra.Command, _ []string) error {
		stateDir, err := opts.stateRoot()
		if err != nil {
			return err
		}
		database, err := store.Open(command.Context(), stateDir)
		if err != nil {
			return err
		}
		defer database.Close()
		return writeResult(opts.stdout, opts.format, "state.migrate", map[string]any{"migrated": true}, opts.quiet)
	}})
	return cmd
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
	var listClient, listKind string
	var list *cobra.Command
	scan := &cobra.Command{Use: "scan", Args: exactArgs(0), RunE: withExtensions(func(ctx context.Context, s *store.Store, home, workdir string, _ []string) (any, error) {
		result, err := extension.Scan(ctx, s, home, workdir)
		if err == nil && opts.verbose {
			result.Roots, result.Workdir = extensionWatchRoots(home, workdir), workdir
		}
		return result, err
	})}
	list = &cobra.Command{Use: "list", Args: exactArgs(0), RunE: withExtensions(func(ctx context.Context, s *store.Store, _, _ string, _ []string) (any, error) {
		if listClient != "" && listClient != "codex" && listClient != "claude" {
			return nil, &inputError{err: fmt.Errorf("invalid extension client %q", listClient)}
		}
		if listKind != "" && listKind != "plugin" && listKind != "mcp" && listKind != "skill" {
			return nil, &inputError{err: fmt.Errorf("invalid extension kind %q", listKind)}
		}
		values, err := extension.List(ctx, s)
		if err != nil {
			return nil, err
		}
		filtered := values[:0]
		for _, value := range values {
			if (listClient == "" || value.Client == listClient) && (listKind == "" || value.Kind == listKind) {
				filtered = append(filtered, value)
			}
		}
		return filtered, nil
	})}
	list.Flags().StringVar(&listClient, "client", "", "Filter inventory by client")
	list.Flags().StringVar(&listKind, "kind", "", "Filter inventory by kind")
	cmd.AddCommand(scan, list,
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
				return result["imported"] + result["updated"] + result["removed"], scanErr
			}})
		}
		if requested["session"] {
			filtered = append(filtered, watch.Source{Domain: "session", Snapshot: fingerprint(sessionRoots), Scan: func(ctx context.Context) (int, error) {
				if err := openSessions(ctx); err != nil {
					return 0, err
				}
				result, scanErr := session.Scan(ctx, sessions.DB, home)
				return result.Documents + result.Removed, scanErr
			}})
		}
		if requested["extension"] {
			filtered = append(filtered, watch.Source{Domain: "extension", Snapshot: fingerprint(extensionRoots), Scan: func(ctx context.Context) (int, error) {
				if err := openCore(ctx); err != nil {
					return 0, err
				}
				result, scanErr := extension.Scan(ctx, database, home, workdir)
				return result.Added + result.Updated + result.Removed, scanErr
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
			return renderWatchText(opts.stdout, event)
		})
	}}
	command.Flags().DurationVar(&interval, "interval", time.Minute, "Polling interval")
	command.Flags().StringVar(&domainsValue, "domains", "usage,session,extension", "Comma-separated domains to watch")
	return command
}

func renderWatchText(w io.Writer, event watch.Event) error {
	at := renderDisplayTimeWithZone(event.GeneratedAt)
	if event.Skipped {
		_, err := fmt.Fprintf(w, "%s Watch scan skipped: %s.\n", at, strings.ReplaceAll(event.Reason, "_", " "))
		return err
	}
	labels := map[string]string{"usage": "usage records", "session": "session documents", "extension": "extension inventory entries"}
	label := labels[event.Domain]
	if label == "" {
		label = event.Domain
	}
	_, err := fmt.Fprintf(w, "%s %s scan completed: %d %s changed.\n", at, strings.Title(event.Domain), event.Changes, label)
	return err
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

func newUsageHookCommand(opts *commandOptions) *cobra.Command {
	command := &cobra.Command{Use: "hook", Short: "Manage runtime attribution hooks", Args: exactArgs(0)}
	command.AddCommand(
		newUsageHookLifecycleCommand(opts, "setup"),
		newUsageHookLifecycleCommand(opts, "status"),
		newUsageHookLifecycleCommand(opts, "remove"),
		newUsageHookEventCommand(opts),
	)
	return command
}

func newUsageHookLifecycleCommand(opts *commandOptions, operation string) *cobra.Command {
	var clientValue string
	var client usagehook.Client
	command := &cobra.Command{
		Use:  operation,
		Args: cobra.NoArgs,
		PreRunE: func(_ *cobra.Command, _ []string) error {
			if opts.format != "text" && opts.format != "json" {
				return &inputError{err: fmt.Errorf("usage hook %s supports only text or json format", operation)}
			}
			parsed, err := usagehook.ParseClient(clientValue)
			if err != nil {
				return &inputError{err: err}
			}
			client = parsed
			return nil
		},
		RunE: func(command *cobra.Command, _ []string) error {
			return runUsageHookLifecycle(command.Context(), opts, operation, client)
		},
	}
	command.Flags().StringVar(&clientValue, "client", "all", "Client configuration: codex, claude, or all")
	return command
}

func newUsageHookEventCommand(opts *commandOptions) *cobra.Command {
	var client usagehook.Client
	return &cobra.Command{
		Use:   "event <codex|claude>",
		Short: "Handle one runtime attribution hook event",
		Long: argumentHelp(
			"Read one bounded Codex or Claude hook event from stdin, remain silent, and fail open for the client.",
			" codex|claude Client that invoked the hook.",
		),
		Example: " agentdeck usage hook event codex\n agentdeck usage hook event claude",
		Hidden:  true,
		Args:    exactArgs(1),
		PreRunE: func(_ *cobra.Command, args []string) error {
			if opts.format != "text" {
				return &inputError{err: errors.New("usage hook event requires text format")}
			}
			parsed, err := usagehook.ParseClient(args[0])
			if err != nil || parsed == usagehook.ClientAll {
				if err == nil {
					err = errors.New("usage hook event requires codex or claude client")
				}
				return &inputError{err: err}
			}
			client = parsed
			return nil
		},
		RunE: func(command *cobra.Command, _ []string) error {
			return runUsageHookEvent(command.Context(), opts, client)
		},
	}
}

func runUsageHookLifecycle(ctx context.Context, opts *commandOptions, operation string, client usagehook.Client) error {
	home, err := userHomeDir()
	if err != nil {
		return err
	}
	command := "agentdeck"
	if opts.stateDir != "" {
		command += " --state-dir " + shellQuote(opts.stateDir)
	}
	manager := usagehook.New(usagehook.Environment{Home: home, AgentDeckCommand: command})
	request := usagehook.Request{Client: client}
	var summary usagehook.Summary
	switch operation {
	case "setup":
		summary, err = manager.Setup(request)
	case "status":
		summary, err = manager.Status(request)
	case "remove":
		summary, err = manager.Remove(request)
	default:
		return fmt.Errorf("usage hook %s not available in build", operation)
	}
	if err != nil {
		return err
	}
	if opts.format == "json" {
		if err := writeResult(opts.stdout, opts.format, "usage.hook."+operation, summary); err != nil {
			return err
		}
	} else if err := writeUsageHookText(opts, operation, summary); err != nil {
		return err
	}
	if summary.HasFailures() {
		return fmt.Errorf("usage hook %s failed for one or more clients", operation)
	}
	return nil
}

func writeUsageHookText(opts *commandOptions, operation string, summary usagehook.Summary) error {
	for _, result := range summary.Results {
		if opts.quiet && operation != "status" && result.Outcome != usagehook.OutcomeFailed {
			continue
		}
		if operation == "status" {
			if _, err := fmt.Fprintf(opts.stdout, "%s %s: %s", result.Client, result.Path, result.Configuration); err != nil {
				return err
			}
		} else if _, err := fmt.Fprintf(opts.stdout, "%s %s: %s", result.Client, result.Path, result.Outcome); err != nil {
			return err
		}
		if result.Error != "" {
			if _, err := fmt.Fprintf(opts.stdout, " (%s)", result.Error); err != nil {
				return err
			}
		}
		if result.Guidance != "" && (!opts.quiet || operation == "status") {
			if _, err := fmt.Fprintf(opts.stdout, "\n  guidance: %s", result.Guidance); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(opts.stdout); err != nil {
			return err
		}
	}
	return nil
}

func runUsageHookEvent(ctx context.Context, opts *commandOptions, client usagehook.Client) error {
	contents, err := io.ReadAll(io.LimitReader(opts.stdin, usagehook.MaxEventBytes+1))
	if err != nil || len(contents) > usagehook.MaxEventBytes {
		// Hook delivery is fail-open: malformed or oversized client input must
		// never prevent the client from starting, resuming, or exiting.
		return nil
	}
	event, parseErr := usagehook.ParseEvent(client, contents)
	if parseErr != nil {
		return nil
	}
	database, _, openErr := opts.openStore(ctx)
	if openErr != nil {
		return nil
	}
	defer database.Close()
	home, homeErr := userHomeDir()
	if homeErr != nil {
		return nil
	}
	if client == usagehook.ClientClaude && event.Name == "ConfigChange" {
		if !managedClaudeConfigChange(home, event) {
			return nil
		}
		_ = reconcileClaudeConfigChange(ctx, database, home, event.SessionID)
		return nil
	}
	if event.Name == "SessionStart" && !validHookTranscript(home, client, event) {
		return nil
	}
	_ = usage.New(database, home).RecordSessionRoute(ctx, usage.SessionRoute{Client: string(client), SessionID: event.SessionID, HookEvent: event.Name, Source: event.Source})
	return nil
}

func managedClaudeConfigChange(home string, event usagehook.Event) bool {
	return event.Source == "user_settings" && filepath.Clean(event.ConfigPath) == filepath.Join(home, ".claude", "settings.json")
}

func reconcileClaudeConfigChange(ctx context.Context, database *store.Store, home, sessionID string) error {
	const attempts = 3
	const delay = 25 * time.Millisecond
	configPath := filepath.Join(home, ".claude", "settings.json")
	service := usage.New(database, home)
	var lastConfigErr error
	inspectedConfig := false
	for attempt := 0; attempt < attempts; attempt++ {
		snapshot, err := database.CurrentProviderSnapshot(ctx, "claude")
		if err == nil {
			matches, matchErr := provider.ClaudeConfigMatchesSnapshot(configPath, snapshot)
			if matchErr != nil {
				lastConfigErr = matchErr
			} else {
				inspectedConfig = true
				lastConfigErr = nil
				if matches {
					return service.RecordClaudeConfigChange(ctx, sessionID, snapshot, true)
				}
			}
		} else if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if attempt+1 < attempts {
			sleepForHookReconciliation(delay)
		}
	}
	if lastConfigErr != nil && !inspectedConfig {
		return lastConfigErr
	}
	return service.RecordClaudeConfigChange(ctx, sessionID, store.ProviderSnapshot{}, false)
}

func validHookTranscript(home string, client usagehook.Client, event usagehook.Event) bool {
	if event.TranscriptPath == "" || !strings.Contains(filepath.Base(event.TranscriptPath), event.SessionID) {
		return false
	}
	root := filepath.Join(home, ".codex", "sessions")
	if client == usagehook.ClientClaude {
		root = filepath.Join(home, ".claude", "projects")
	}
	info, err := os.Lstat(event.TranscriptPath)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return false
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return false
	}
	resolvedPath, err := filepath.EvalSymlinks(event.TranscriptPath)
	if err != nil {
		return false
	}
	relative, err := filepath.Rel(resolvedRoot, resolvedPath)
	return err == nil && relative != "." && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func newUsageCommand(opts *commandOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "usage", Short: "Inspect usage and pricing"}
	var statsInteractive bool
	withUsage := func(run func(context.Context, *usage.Service, *store.Store, []string) (any, bool, []string, error)) func(*cobra.Command, []string) error {
		return func(command *cobra.Command, args []string) error {
			if command.Name() == "stats" && statsInteractive && opts.format != "text" {
				return &inputError{err: errors.New("usage stats --interactive requires text output")}
			}
			if command.Name() == "stats" && statsInteractive {
				if _, _, err := usageStatsInteractiveTerminal(opts.stdin, opts.stdout); err != nil {
					return &inputError{err: err}
				}
			}
			database, _, err := opts.openStore(command.Context())
			if err != nil {
				return err
			}
			defer database.Close()
			home, err := userHomeDir()
			if err != nil {
				return err
			}
			service := usage.New(database, home)
			service.Progress = newUsageProgress(opts.stderr, opts.quiet)
			data, partial, warnings, err := run(command.Context(), service, database, args)
			if err != nil {
				return err
			}
			renderOptions := newUsageTextRenderOptions(opts.stdout, opts.noColor)
			if command.Name() == "stats" && command.Flags().Changed("top") {
				if top, err := command.Flags().GetInt("top"); err == nil {
					renderOptions.top = &top
				}
			}
			if command.Name() == "stats" && statsInteractive {
				report, ok := data.(usage.StatsReport)
				if !ok {
					return errors.New("usage stats interactive report is unavailable")
				}
				input, output, err := usageStatsInteractiveTerminal(opts.stdin, opts.stdout)
				if err != nil {
					return &inputError{err: err}
				}
				return runUsageStatsViewer(command.Context(), input, output, report, warnings, opts.noColor, renderOptions.top)
			}
			return writeUsageEnvelope(opts.stdout, opts.format, commandOutputName(command), data, partial, warnings, opts.quiet, renderOptions)
		}
	}
	var summaryNoScan bool
	summary := &cobra.Command{Use: "summary [daily|weekly|monthly]", Args: cobra.MaximumNArgs(1), RunE: withUsage(func(ctx context.Context, s *usage.Service, _ *store.Store, args []string) (any, bool, []string, error) {
		var scanErr error
		if !summaryNoScan {
			_, scanErr = s.Scan(ctx)
		}
		if len(args) == 0 {
			data, err := s.Summary(ctx)
			return data, scanErr != nil, map[bool][]string{true: {"scan_incomplete"}}[scanErr != nil], err
		}
		period := map[string]string{"daily": "today", "weekly": "week", "monthly": "month"}[args[0]]
		if period == "" {
			return nil, false, nil, &inputError{err: fmt.Errorf("usage summary period must be daily, weekly, or monthly")}
		}
		from, to, err := resolveUsageRange(ctx, s, period, "", "", time.Now(), displayLocation())
		if err != nil {
			return nil, false, nil, err
		}
		data, err := s.SummaryRange(ctx, from, to)
		return data, scanErr != nil, map[bool][]string{true: {"scan_incomplete"}}[scanErr != nil], err
	})}
	summary.Flags().BoolVar(&summaryNoScan, "no-scan", false, "Use stored aggregate without scanning sources")
	var statsPeriod, statsFrom, statsTo, statsGroup, statsMetric, statsClient, statsModel, statsProvider string
	var statsActivity, statsNoScan bool
	var statsTop int
	stats := &cobra.Command{Use: "stats", Args: cobra.NoArgs, RunE: withUsage(func(ctx context.Context, s *usage.Service, _ *store.Store, _ []string) (any, bool, []string, error) {
		if statsClient != "" && statsClient != "codex" && statsClient != "claude" {
			return nil, false, nil, &inputError{err: fmt.Errorf("usage stats client must be codex or claude")}
		}
		if statsMetric != "tokens" && statsMetric != "cost" && statsMetric != "sessions" {
			return nil, false, nil, &inputError{err: fmt.Errorf("usage stats metric must be tokens, cost, or sessions")}
		}
		if statsActivity && statsModel == "" {
			return nil, false, nil, &inputError{err: fmt.Errorf("usage stats --activity requires --model")}
		}
		if statsTop < 0 {
			return nil, false, nil, &inputError{err: fmt.Errorf("usage stats --top must be zero or positive")}
		}
		var scanErr error
		if !statsNoScan {
			_, scanErr = s.Scan(ctx)
		}
		now := time.Now()
		location := displayLocation()
		from, to, err := resolveUsageRange(ctx, s, statsPeriod, statsFrom, statsTo, now, location)
		if err != nil {
			return nil, false, nil, err
		}
		group := statsGroup
		if group == "auto" {
			group = automaticUsageGroup(from, to)
		}
		if group != "hour" && group != "day" && group != "week" && group != "month" {
			return nil, false, nil, &inputError{err: fmt.Errorf("usage stats group-by must be auto, hour, day, week, or month")}
		}
		data, err := s.Stats(ctx, usage.StatsOptions{From: from, To: to, GroupBy: group, Metric: statsMetric, Client: statsClient, Model: statsModel, Provider: statsProvider, Timezone: displayTimezoneName(location, now), Location: location, Activity: statsActivity})
		warnings := map[bool][]string{true: {"scan_incomplete"}}[scanErr != nil]
		warnings = append(warnings, data.Warnings...)
		return data, scanErr != nil, warnings, err
	})}
	stats.Flags().StringVar(&statsPeriod, "period", "7d", "Range: today, 7d, 30d, week, month, 6m, or all")
	stats.Flags().StringVar(&statsFrom, "from", "", "Local start date in YYYY-MM-DD")
	stats.Flags().StringVar(&statsTo, "to", "", "Inclusive local end date in YYYY-MM-DD")
	stats.Flags().StringVar(&statsGroup, "group-by", "auto", "Buckets: auto, hour, day, week, or month")
	stats.Flags().StringVar(&statsMetric, "metric", "tokens", "Trend metric: tokens, cost, or sessions")
	stats.Flags().StringVar(&statsClient, "client", "", "Filter by codex or claude")
	stats.Flags().StringVar(&statsModel, "model", "", "Filter by exact model name")
	stats.Flags().StringVar(&statsProvider, "provider", "", "Open-set exact runtime provider; values are not enumerated; unknown selects unattributed events")
	stats.Flags().BoolVar(&statsActivity, "activity", false, "Show safe activity and tool summaries for the selected model")
	stats.Flags().BoolVar(&statsNoScan, "no-scan", false, "Use stored aggregate without scanning sources")
	stats.Flags().BoolVar(&statsInteractive, "interactive", false, "Open a read-only terminal usage viewer")
	stats.Flags().IntVar(&statsTop, "top", 0, "Text-list cap for MODELS/PROVIDERS/UNPRICED/per-model CACHE/cache sessions: unset keeps each section's default cap, 0 shows every row, N overrides the cap to N. TREND and CLIENTS are unaffected; --format json always has every row.")
	cmd.AddCommand(
		newUsageHookCommand(opts),
		&cobra.Command{Use: "scan", Args: cobra.NoArgs, RunE: withUsage(func(ctx context.Context, s *usage.Service, _ *store.Store, _ []string) (any, bool, []string, error) {
			data, err := s.Scan(ctx)
			return data, false, nil, err
		})},
		summary,
		stats,
		&cobra.Command{Use: "sessions", Args: cobra.NoArgs, RunE: withUsage(func(ctx context.Context, s *usage.Service, _ *store.Store, _ []string) (any, bool, []string, error) {
			data, err := s.Sessions(ctx)
			return data, false, nil, err
		})},
		&cobra.Command{Use: "diagnose", Args: cobra.NoArgs, RunE: withUsage(func(ctx context.Context, s *usage.Service, _ *store.Store, _ []string) (any, bool, []string, error) {
			data, err := s.Diagnose(ctx)
			return data, false, nil, err
		})},
		&cobra.Command{Use: "rebuild", Args: cobra.NoArgs, RunE: withUsage(func(ctx context.Context, s *usage.Service, _ *store.Store, _ []string) (any, bool, []string, error) {
			data, warnings, err := s.Rebuild(ctx)
			return data, len(warnings) > 0, warnings, err
		})},
	)
	return cmd
}

func resolveUsageRange(ctx context.Context, service *usage.Service, period, fromText, toText string, now time.Time, location *time.Location) (time.Time, time.Time, error) {
	if location == nil {
		location = displayLocation()
	}
	now = now.In(location)
	if fromText != "" || toText != "" {
		if fromText == "" || toText == "" {
			return time.Time{}, time.Time{}, &inputError{err: fmt.Errorf("usage stats --from and --to must be provided together")}
		}
		from, err := time.ParseInLocation("2006-01-02", fromText, location)
		if err != nil {
			return time.Time{}, time.Time{}, &inputError{err: fmt.Errorf("invalid --from date %q", fromText)}
		}
		to, err := time.ParseInLocation("2006-01-02", toText, location)
		if err != nil {
			return time.Time{}, time.Time{}, &inputError{err: fmt.Errorf("invalid --to date %q", toText)}
		}
		to = to.AddDate(0, 0, 1)
		if !from.Before(to) {
			return time.Time{}, time.Time{}, &inputError{err: errors.New("usage stats --from must not be after --to")}
		}
		return from, to, nil
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	var from time.Time
	switch period {
	case "today":
		from = today
	case "7d":
		from = today.AddDate(0, 0, -6)
	case "30d":
		from = today.AddDate(0, 0, -29)
	case "week":
		from = today.AddDate(0, 0, -((int(today.Weekday()) + 6) % 7))
	case "month":
		from = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, location)
	case "6m":
		from = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, location).AddDate(0, -5, 0)
	case "all":
		earliest, err := service.EarliestEventAt(ctx)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
		if earliest == nil {
			from = today
		} else {
			from = earliest.In(location)
		}
	default:
		return time.Time{}, time.Time{}, &inputError{err: fmt.Errorf("usage stats period must be today, 7d, 30d, week, month, 6m, or all")}
	}
	to := now
	if !from.Before(to) {
		to = from.AddDate(0, 0, 1)
	}
	return from, to, nil
}

func automaticUsageGroup(from, to time.Time) string {
	days := to.Sub(from).Hours() / 24
	switch {
	case days <= 2:
		return "hour"
	case days <= 90:
		return "day"
	case days <= 366:
		return "week"
	default:
		return "month"
	}
}

type displayTimeValue interface {
	string | time.Time
}

// renderDisplayTime localizes stored instants for human-readable output.
// Unparseable strings stay unchanged so presentation cannot fail a command.
func renderDisplayTime[T displayTimeValue](value T) string {
	var instant time.Time
	switch value := any(value).(type) {
	case string:
		parsed, err := time.Parse(time.RFC3339Nano, value)
		if err != nil {
			return value
		}
		instant = parsed
	case time.Time:
		instant = value
	}
	return instant.In(displayLocation()).Format("2006-01-02 15:04:05")
}

func renderDisplayTimeWithZone[T displayTimeValue](value T) string {
	if text, ok := any(value).(string); ok {
		if _, err := time.Parse(time.RFC3339Nano, text); err != nil {
			return text
		}
	}
	return renderDisplayTime(value) + " " + displayZoneName()
}

func displayTimezoneName(location *time.Location, now time.Time) string {
	if location == nil {
		location = displayLocation()
	}
	if name := location.String(); name != "" && name != "Local" {
		return name
	}
	if value, exists := os.LookupEnv("TZ"); exists {
		value = strings.TrimPrefix(value, ":")
		if value == "" {
			return "UTC"
		}
		if name := timezoneNameFromPath(value); name != "" {
			return name
		}
		if !filepath.IsAbs(value) {
			if _, err := time.LoadLocation(value); err == nil {
				return value
			}
		}
	}
	if resolved, err := filepath.EvalSymlinks("/etc/localtime"); err == nil {
		if name := timezoneNameFromPath(resolved); name != "" {
			return name
		}
	}
	_, offset := now.In(location).Zone()
	sign := '+'
	if offset < 0 {
		sign = '-'
		offset = -offset
	}
	return fmt.Sprintf("UTC%c%02d:%02d", sign, offset/3600, offset%3600/60)
}

func displayZoneName() string {
	location := displayLocation()
	return displayTimezoneName(location, time.Now())
}

func timezoneNameFromPath(path string) string {
	path = filepath.ToSlash(path)
	marker := "/zoneinfo/"
	index := strings.LastIndex(path, marker)
	if index < 0 || index+len(marker) == len(path) {
		return ""
	}
	return path[index+len(marker):]
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
			return writePriceEnvelope(opts.stdout, opts.format, commandOutputName(command), data, partial, warnings, opts.quiet, opts.verbose)
		}
	}
	price := &cobra.Command{Use: "price", Short: "Manage price catalogs"}
	var listProvider string
	list := &cobra.Command{Use: "list [model]", Args: cobra.MaximumNArgs(1), RunE: withUsage(func(ctx context.Context, s *usage.Service, _ *store.Store, args []string) (any, bool, []string, error) {
		if listProvider != "" && listProvider != "openai" && listProvider != "anthropic" {
			return nil, false, nil, &inputError{err: fmt.Errorf("price provider must be openai or anthropic")}
		}
		model := ""
		if len(args) == 1 {
			model = args[0]
		}
		data, err := s.PriceList(ctx, listProvider, model)
		return data, false, nil, err
	})}
	list.Flags().StringVar(&listProvider, "provider", "", "Filter by openai or anthropic")
	price.AddCommand(
		&cobra.Command{Use: "history", Args: cobra.NoArgs, RunE: withUsage(func(ctx context.Context, s *usage.Service, _ *store.Store, _ []string) (any, bool, []string, error) {
			data, err := s.PriceHistory(ctx)
			return data, false, nil, err
		})},
		&cobra.Command{Use: "status", Args: cobra.NoArgs, RunE: withUsage(func(ctx context.Context, s *usage.Service, _ *store.Store, _ []string) (any, bool, []string, error) {
			data, err := s.PriceStatus(ctx)
			return data, false, nil, err
		})},
		list,
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
		if cwd, cwdErr := os.Getwd(); cwdErr == nil {
			projectService := provider.Service{Store: database}
			if environment, changed := projectService.RunProjectEnvironment(cmd.Context(), provider.Client(args[0]), cwd, os.Environ()); changed {
				child.Env = environment
			}
		}
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
	if command == "backup.create" {
		if len(quiet) > 0 && quiet[0] {
			return nil
		}
		return renderBackupCreateText(w, data, mutationResource(data, resource))
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
	quietOutput := len(quiet) > 0 && quiet[0]
	return writeUsageEnvelope(w, format, command, data, partial, warnings, quietOutput, usageTextRenderOptions{})
}

func writeUsageEnvelope(w io.Writer, format, command string, data any, partial bool, warnings []string, quiet bool, renderOptions usageTextRenderOptions) error {
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
		if quiet && !partial && len(warnings) == 0 {
			return nil
		}
		if command != "usage.scan" && command != "usage.rebuild" {
			return renderMutationText(w, command, mutationResource(data, resource))
		}
	}
	if err := renderUsageTextWithOptions(w, command, data, renderOptions); err != nil {
		return err
	}
	if command == "usage.rebuild" && partial && len(warnings) > 0 {
		_, err := fmt.Fprintf(w, "warnings: %s\n", textList(warnings))
		return err
	}
	return nil
}

func writePriceEnvelope(w io.Writer, format, command string, data any, partial bool, warnings []string, quiet, verbose bool) error {
	if format == "json" {
		if warnings == nil {
			warnings = []string{}
		}
		envelope := output.New(command, data, time.Now())
		envelope.Partial, envelope.Warnings = partial, warnings
		return json.NewEncoder(w).Encode(envelope)
	}
	if format == "ndjson" {
		return &inputError{err: fmt.Errorf("ndjson format is supported only by watch")}
	}
	if quiet && (command == "price.update" || command == "price.override") && !partial && len(warnings) == 0 {
		return nil
	}
	if err := renderPriceText(w, command, data, verbose); err != nil {
		return err
	}
	if len(warnings) > 0 {
		_, err := fmt.Fprintf(w, "warnings: %s\n", textList(warnings))
		return err
	}
	return nil
}

func renderPriceText(w io.Writer, command string, data any, verbose bool) error {
	switch command {
	case "price.history":
		values, ok := data.([]usage.PriceCatalog)
		if !ok {
			return fmt.Errorf("unexpected price.history result %T", data)
		}
		return renderPriceHistory(w, values, verbose)
	case "price.status":
		value, ok := data.(map[string]any)
		if !ok {
			return fmt.Errorf("unexpected price.status result %T", data)
		}
		return renderPriceStatus(w, value, verbose)
	case "price.list":
		values, ok := data.([]usage.EffectivePrice)
		if !ok {
			return fmt.Errorf("unexpected price.list result %T", data)
		}
		return renderPriceList(w, values, verbose)
	case "price.update", "price.override":
		value, ok := data.(map[string]any)
		if !ok {
			return fmt.Errorf("unexpected %s result %T", command, data)
		}
		keys := []string{"version", "models", "overrides", "commit_sha", "content_sha256"}
		rows := make([][]string, 0, len(keys))
		for _, key := range keys {
			raw, exists := value[key]
			if !exists {
				continue
			}
			textValue := fmt.Sprint(raw)
			if !verbose && (key == "commit_sha" || key == "content_sha256") {
				textValue = shortDigest(textValue)
			}
			rows = append(rows, []string{strings.ReplaceAll(key, "_", " "), textValue})
		}
		return output.WriteASCIITable(w, []string{"RESULT", "VALUE"}, rows)
	default:
		return fmt.Errorf("no price text renderer for %s (%T)", command, data)
	}
}

func shortDigest(value string) string {
	if len(value) <= 12 {
		return value
	}
	return value[:12]
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
	case "provider.add", "provider.update", "provider.remove", "provider.use", "provider.set-wrapper",
		"credential.add", "credential.update", "credential.remove",
		"usage.scan", "usage.rebuild", "price.update", "price.override",
		"session.scan", "session.exclude", "session.rebuild", "session.purge-index",
		"extension.adopt", "extension.release", "extension.enable", "extension.disable",
		"backup.create", "backup.restore", "state.migrate":
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
			rows = append(rows, []string{definition.Name, kind, strings.Join(clients, ","), strconv.Itoa(definition.CredentialCount), textOrDash(wrapperCell(definition))})
		}
		return output.WriteASCIITable(w, []string{"NAME", "TYPE", "CLIENTS", "CREDENTIALS", "WRAPPER"}, rows)
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
			rows = append(rows, []string{item.Client, item.Provider, credential, renderDisplayTime(item.SelectedAt), routeLabel(item.ViaWrapper), textOrDash(item.Endpoint)})
		}
		return output.WriteASCIITable(w, []string{"CLIENT", "PROVIDER", "CREDENTIAL", fmt.Sprintf("SELECTED AT (%s)", displayZoneName()), "ROUTE", "ENDPOINT"}, rows)
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
		switch value := data.(type) {
		case []session.Metadata:
			return renderSessionMetadata(w, value)
		case sessionListPage:
			if err := renderSessionMetadata(w, value.Sessions); err != nil {
				return err
			}
			return renderPagination(w, value.Pagination["sessions"], value.NextCommand)
		default:
			return fmt.Errorf("unexpected session.list result %T", data)
		}
	case "session.search":
		switch value := data.(type) {
		case []session.Document:
			return renderSessionDocuments(w, value)
		case sessionSearchPage:
			if err := renderSessionDocuments(w, value.Documents); err != nil {
				return err
			}
			return renderPagination(w, value.Pagination["search"], value.NextCommand)
		default:
			return fmt.Errorf("unexpected session.search result %T", data)
		}
	case "session.show":
		value, pagination, nextCommand, ok := session.Result{}, map[string]session.Pagination(nil), "", false
		usageSummary, invocations := (*usage.SessionSummary)(nil), []usage.SessionInvocation(nil)
		activityRequested, activityWarning, sourceStale := false, "", false
		switch typed := data.(type) {
		case session.Result:
			value, ok = typed, true
		case sessionShowPage:
			value, pagination, nextCommand, usageSummary, invocations, activityRequested, activityWarning, sourceStale, ok = typed.Result, typed.Pagination, typed.NextCommand, typed.Usage, typed.Invocations, typed.ActivityRequested, typed.ActivityWarning, typed.SourceStale, true
		}
		if !ok {
			return fmt.Errorf("unexpected session.show result %T", data)
		}
		return renderSessionShowText(w, value, pagination, nextCommand, usageSummary, invocations, activityRequested, activityWarning, sourceStale)
	case "extension.list":
		value, ok := data.([]extension.DTO)
		if !ok {
			return fmt.Errorf("unexpected extension.list result %T", data)
		}
		if _, err := fmt.Fprintln(w, "Inventory from the most recent extension scan (not a live scan). "); err != nil {
			return err
		}
		return renderExtensions(w, value)
	case "extension.scan":
		value, ok := data.(extension.Result)
		if !ok {
			return fmt.Errorf("unexpected extension.scan result %T", data)
		}
		if _, err := fmt.Fprintf(w, "found: %d\nadded: %d\nupdated: %d\nremoved: %d\nunchanged: %d\n", value.Found, value.Added, value.Updated, value.Removed, value.Unchanged); err != nil {
			return err
		}
		keys := make([]string, 0, len(value.Summary))
		for key := range value.Summary {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		rows := make([][]string, 0, len(keys))
		for _, key := range keys {
			rows = append(rows, []string{key, strconv.Itoa(value.Summary[key])})
		}
		if err := output.WriteASCIITable(w, []string{"CLIENT:KIND:SCOPE", "FOUND"}, rows); err != nil {
			return err
		}
		if len(value.Roots) > 0 {
			if _, err := fmt.Fprintf(w, "scan roots: %s\nproject cwd: %s\n", strings.Join(value.Roots, ", "), value.Workdir); err != nil {
				return err
			}
		}
		return nil
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
			rows = append(rows, []string{item.Path, strconv.FormatInt(item.Size, 10), renderDisplayTime(item.ModifiedAt)})
		}
		return output.WriteASCIITable(w, []string{"PATH", "SIZE", fmt.Sprintf("MODIFIED (%s)", displayZoneName())}, rows)
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
		value, ok := data.([]usage.PriceCatalog)
		if !ok {
			return fmt.Errorf("unexpected price.history result %T", data)
		}
		return renderPriceHistory(w, value, false)
	case "price.status":
		value, ok := data.(map[string]any)
		if !ok {
			return fmt.Errorf("unexpected price.status result %T", data)
		}
		return renderPriceStatus(w, value, false)
	default:
		return fmt.Errorf("no text renderer for %s (%T)", command, data)
	}
}

// wrapperCell renders a wrapper URL with its declared protocol appended, and
// the bare URL when none is declared. The protocol is an annotation rather than
// its own column or line so that a wrapper with no declaration — every wrapper
// that predates the field — renders exactly as it did before.
func wrapperCell(value provider.Provider) string {
	if value.WrapperURL == "" || value.WrapperKind == "" {
		return value.WrapperURL
	}
	return value.WrapperURL + " (" + value.WrapperKind + ")"
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
	if value.WrapperURL != "" {
		if _, err := fmt.Fprintf(w, "wrapper: %s\n", wrapperCell(value)); err != nil {
			return err
		}
	}
	if value.Authentication != "" {
		_, err := fmt.Fprintf(w, "authentication: %s\n", value.Authentication)
		return err
	}
	return nil
}

// routeLabel names the route a recorded selection took. The two labels are
// the same words the effective-route line uses on a switch.
func routeLabel(viaWrapper bool) string {
	if viaWrapper {
		return "via wrapper"
	}
	return "direct"
}

func textOrDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
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
		route, endpoint := "-", "-"
		for _, selection := range value.Active {
			if selection.Client != string(client) {
				continue
			}
			active, selectedAt = "true", renderDisplayTime(selection.SelectedAt)
			route, endpoint = routeLabel(selection.ViaWrapper), textOrDash(selection.Endpoint)
			if selection.Credential != "" {
				credential = selection.Credential
			}
		}
		rows = append(rows, []string{string(client), active, credential, selectedAt, route, endpoint})
	}
	return output.WriteASCIITable(w, []string{"CLIENT", "ACTIVE", "CREDENTIAL", fmt.Sprintf("SELECTED AT (%s)", displayZoneName()), "ROUTE", "ENDPOINT"}, rows)
}

func renderSessionMetadata(w io.Writer, values []session.Metadata) error {
	if len(values) == 0 {
		_, err := fmt.Fprintln(w, "No sessions.")
		return err
	}
	rows := make([][]string, 0, len(values))
	for _, value := range values {
		rows = append(rows, []string{value.Client, value.SessionID, value.Project, value.Model, renderDisplayTime(value.FirstAt), renderDisplayTime(value.LastAt)})
	}
	zone := displayZoneName()
	return output.WriteASCIITable(w, []string{"CLIENT", "SESSION", "PROJECT", "MODEL", fmt.Sprintf("FIRST (%s)", zone), fmt.Sprintf("LAST (%s)", zone)}, rows)
}

func renderSessionDocuments(w io.Writer, values []session.Document) error {
	if len(values) == 0 {
		_, err := fmt.Fprintln(w, "No session documents.")
		return err
	}
	rows := make([][]string, 0, len(values))
	for _, value := range values {
		rows = append(rows, []string{value.Client, value.SessionID, renderSessionDocumentTime(value.EventAt), value.Kind, oneLine(value.Text)})
	}
	zone := displayZoneName()
	return output.WriteASCIITable(w, []string{"CLIENT", "SESSION", fmt.Sprintf("EVENT AT (%s)", zone), "KIND", "TEXT"}, rows)
}

func renderSessionDocumentTime(value string) string {
	if _, err := time.Parse(time.RFC3339Nano, value); err != nil {
		return "—"
	}
	return renderDisplayTime(value)
}

func renderPagination(w io.Writer, p session.Pagination, nextCommand string) error {
	first, last := 0, 0
	if p.Shown > 0 {
		first = (p.Page-1)*p.Limit + 1
		last = first + p.Shown - 1
	}
	if _, err := fmt.Fprintf(w, "Showing %d-%d of %d\n", first, last, p.Total); err != nil {
		return err
	}
	if p.HasMore && nextCommand != "" {
		_, err := fmt.Fprintf(w, "Next page: %s\n", nextCommand)
		return err
	}
	return nil
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
	_, err := fmt.Fprintf(w, "schema version: %d\nagentdeck version: %s\ncreated: %s\nsource platform: %s\nincluded: %s\nentries: %d\n", value.SchemaVersion, value.AgentDeckVersion, renderDisplayTimeWithZone(value.CreatedAt), value.SourcePlatform, textList(value.Included), len(value.Entries))
	return err
}

func renderBackupCreateText(w io.Writer, data any, resource string) error {
	value, ok := data.(map[string]any)
	if !ok {
		return fmt.Errorf("unexpected backup.create result %T", data)
	}
	manifest, ok := value["manifest"].(backup.Manifest)
	if !ok {
		return fmt.Errorf("unexpected backup.create manifest %T", value["manifest"])
	}
	if err := renderMutationText(w, "backup.create", resource); err != nil {
		return err
	}
	_, err := fmt.Fprintf(w, "created: %s\n", renderDisplayTimeWithZone(manifest.CreatedAt))
	return err
}

func renderPriceHistory(w io.Writer, values []usage.PriceCatalog, verbose bool) error {
	if len(values) == 0 {
		_, err := fmt.Fprintln(w, "No price catalogs.")
		return err
	}
	rows := make([][]string, 0, len(values))
	for _, value := range values {
		row := []string{value.Version, value.SourceKind, renderDisplayTime(value.EffectiveFrom), strconv.FormatInt(value.Models, 10), strconv.FormatInt(value.Components, 10)}
		if verbose {
			row = append(row, value.CommitSHA, value.ContentSHA256, value.SourceURL)
		}
		rows = append(rows, row)
	}
	headers := []string{"VERSION", "SOURCE", fmt.Sprintf("EFFECTIVE (%s)", displayZoneName()), "MODELS", "COMPONENTS"}
	if verbose {
		headers = append(headers, "COMMIT", "SHA256", "URL")
	}
	return output.WriteASCIITable(w, headers, rows)
}

func renderPriceStatus(w io.Writer, value map[string]any, verbose bool) error {
	available, _ := value["available"].(bool)
	if !available {
		_, err := fmt.Fprintln(w, "No price catalog is available.")
		return err
	}
	if err := output.WriteASCIITable(w, []string{"STATUS", "MODELS", "COMPONENTS"}, [][]string{{"available", fmt.Sprint(value["models"]), fmt.Sprint(value["components"])}}); err != nil {
		return err
	}
	catalogs, _ := value["catalogs"].([]usage.PriceCatalog)
	if len(catalogs) == 0 {
		return nil
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	return renderPriceHistory(w, catalogs, verbose)
}

func renderPriceList(w io.Writer, values []usage.EffectivePrice, verbose bool) error {
	if len(values) == 0 {
		_, err := fmt.Fprintln(w, "No effective prices.")
		return err
	}
	components := []string{"input", "cached_input", "output", "cache_read", "cache_write_5m", "cache_write_1h"}
	rows := make([][]string, 0, len(values))
	var estimated []usage.EffectivePrice
	for _, value := range values {
		if value.PriceKind == usage.GapfillKindEquivalentEstimate {
			estimated = append(estimated, value)
		}
		row := []string{value.Provider, value.Model}
		for _, component := range components {
			price := value.Prices[component]
			if price == "" {
				price = "-"
			}
			row = append(row, price)
		}
		rows = append(rows, row)
	}
	headers := []string{"PROVIDER", "MODEL", "INPUT", "CACHED INPUT", "OUTPUT", "CACHE READ", "WRITE 5M", "WRITE 1H"}
	// The estimate marker gets its own column rather than being appended to
	// the model name: MODEL is the value the user passes back to
	// `price list <model>`, so decorating it hands them a name that does not
	// resolve. The column appears only when something is actually estimated,
	// so an ordinary listing keeps exactly the shape it always had.
	if len(estimated) > 0 {
		headers = append(headers, estimatedPriceMarkerColumn)
		for index, value := range values {
			marker := ""
			if value.PriceKind == usage.GapfillKindEquivalentEstimate {
				marker = estimatedPriceMarker
			}
			rows[index] = append(rows[index], marker)
		}
	}
	if _, err := fmt.Fprintln(w, "USD / 1M tokens"); err != nil {
		return err
	}
	if err := output.WriteASCIITable(w, headers, rows); err != nil {
		return err
	}
	if err := renderEstimatedPriceNotes(w, estimated); err != nil {
		return err
	}
	if !verbose {
		return nil
	}
	if _, err := fmt.Fprintln(w, "\nPROVENANCE"); err != nil {
		return err
	}
	var provenanceRows [][]string
	for _, value := range values {
		keys := make([]string, 0, len(value.Provenance))
		for component := range value.Provenance {
			keys = append(keys, component)
		}
		sort.Strings(keys)
		for _, component := range keys {
			provenance := value.Provenance[component]
			provenanceRows = append(provenanceRows, []string{value.Provider, value.Model, component, provenance.SourceKind, provenance.CatalogVersion, renderDisplayTime(provenance.EffectiveFrom), provenance.CommitSHA, provenance.ContentSHA256, provenance.SourceURL})
		}
	}
	return output.WriteASCIITable(w, []string{"PROVIDER", "MODEL", "COMPONENT", "SOURCE", "VERSION", fmt.Sprintf("EFFECTIVE (%s)", displayZoneName()), "COMMIT", "SHA256", "URL"}, provenanceRows)
}

// estimatedPriceNoteWidth wraps the disclosure paragraph narrower than the
// price table it follows, which already runs past 110 columns with its eight
// fixed columns. A fixed width keeps the note reflowing identically whether or
// not the output is a TTY, so captured price-list output stays comparable.
//
// statsWrap breaks on spaces and never splits a word, so a pricing_note must
// not contain a single token longer than this width or that line will overflow.
// Hard-folding instead would mean changing statsWrap, which the usage stats
// renderer shares, so the constraint is enforced on the data rather than in the
// shared helper: TestEstimatePricingNotesFitTheDisclosureWidth checks every
// shipped note. Keep URLs and other long tokens out of a pricing_note.
const estimatedPriceNoteWidth = 96

// estimatedPriceMarkerColumn is a marker-only column added to the price table
// when a listing contains an estimate; estimatedPriceMarker is what it carries,
// and the note block below the table repeats it.
const (
	estimatedPriceMarkerColumn = "EST"
	estimatedPriceMarker       = "*"
)

// renderEstimatedPriceNotes names, below the table, what each ESTIMATED row is
// estimated from and why. The marker alone would tell a reader the number is
// not a published rate without telling them what it is, which is the part that
// keeps the disclosure honest.
func renderEstimatedPriceNotes(w io.Writer, estimated []usage.EffectivePrice) error {
	if len(estimated) == 0 {
		return nil
	}
	if _, err := fmt.Fprintln(w, "\nESTIMATED PRICES"); err != nil {
		return err
	}
	for _, value := range estimated {
		note := fmt.Sprintf("%s %s (%s): equivalent estimate based on %s. %s", estimatedPriceMarker, value.Model, value.Provider, value.BasisModel, value.PricingNote)
		// statsWrap is a plain word wrapper; the disclosure is a paragraph, and
		// an unwrapped one would be a single several-hundred-column line.
		for _, line := range statsWrap(note, estimatedPriceNoteWidth) {
			if _, err := fmt.Fprintln(w, line); err != nil {
				return err
			}
		}
	}
	return nil
}

func renderDoctorText(w io.Writer, report doctor.Report) error {
	if _, err := fmt.Fprintf(w, "status: %s\nmode: %s\nwarnings: %d\nerrors: %d\n", report.Status, report.Mode, report.Warnings, report.Errors); err != nil {
		return err
	}
	for _, check := range report.Checks {
		if _, err := fmt.Fprintf(w, "%s: %s", check.Name, check.Status); err != nil {
			return err
		}
		details := make([]string, 0, 2)
		if check.Code != "" {
			details = append(details, check.Code)
		}
		if check.Count != 0 {
			details = append(details, "count="+strconv.Itoa(check.Count))
		}
		if len(details) > 0 {
			if _, err := fmt.Fprintf(w, " (%s)", strings.Join(details, "; ")); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
		if check.Recovery != "" {
			if _, err := fmt.Fprintf(w, "  recovery: %s\n", check.Recovery); err != nil {
				return err
			}
		}
	}
	return nil
}
func renderUsageText(w io.Writer, command string, data any) error {
	return renderUsageTextWithOptions(w, command, data, usageTextRenderOptions{})
}

func renderUsageTextWithOptions(w io.Writer, command string, data any, renderOptions usageTextRenderOptions) error {
	if values, ok := data.(map[string]int); ok && (command == "usage.scan" || command == "usage.rebuild") {
		return renderUsageMetricTable(w, "📥 USAGE "+strings.ToUpper(strings.TrimPrefix(command, "usage.")), [][]string{
			{"files", strconv.Itoa(values["files"])},
			{"imported", strconv.Itoa(values["imported"])},
			{"updated", strconv.Itoa(values["updated"])},
			{"ignored non-usage", strconv.Itoa(values["ignored_non_usage"])},
			{"unsupported usage", strconv.Itoa(values["unsupported_usage"])},
			{"malformed", strconv.Itoa(values["malformed"])},
			{"source resets", strconv.Itoa(values["source_resets"])},
		})
	}
	if values, ok := data.(map[string]any); ok && command == "usage.diagnose" {
		rows := make([][]string, 0, 4)
		for _, key := range []string{"files", "events", "sessions", "exact_runs"} {
			rows = append(rows, []string{key, fmt.Sprint(values[key])})
		}
		return renderUsageFamilyMetricTable(w, "🩺 USAGE DIAGNOSTICS", rows, renderOptions)
	}
	switch v := data.(type) {
	case usage.StatsReport:
		return renderUsageStatsWithOptions(w, v, renderOptions)
	case usage.Summary:
		return renderUsageFamilySummary(w, v, renderOptions)
	case []usage.SessionSummary:
		return renderUsageFamilySessions(w, v, renderOptions)
	default:
		return fmt.Errorf("no usage text renderer for %s (%T)", command, data)
	}
}

var usageTokenNames = []struct {
	key   string
	label string
}{
	{key: "input_tokens", label: "input"},
	{key: "cached_input_tokens", label: "cached input"},
	{key: "output_tokens", label: "output"},
	{key: "cache_read_tokens", label: "cache read"},
	{key: "cache_creation_tokens", label: "cache create"},
	{key: "cache_write_5m_tokens", label: "write 5m"},
	{key: "cache_write_1h_tokens", label: "write 1h"},
	{key: "cache_write_tokens", label: "cache write"},
}

func renderUsageMetricTable(w io.Writer, title string, rows [][]string) error {
	if _, err := fmt.Fprintln(w, title); err != nil {
		return err
	}
	return output.WriteASCIITable(w, []string{"METRIC", "VALUE"}, rows)
}

func modelCoverageStatus(model usage.ModelCoverage) string {
	switch {
	case model.UnpricedEvents == 0:
		return "priced"
	case model.PricedEvents == 0:
		return "unpriced"
	default:
		return "partial"
	}
}

func sessionCostText(total, known *string) string {
	if total != nil {
		return *total
	}
	if known != nil {
		return *known + " (partial)"
	}
	return "unavailable"
}

func statsValueText(complete *string, known string) string {
	if complete != nil {
		return *complete
	}
	if known != "" {
		return known + " (partial)"
	}
	return "unavailable"
}

func statsPercentText(complete *string, known string) string {
	if complete != nil {
		return *complete + "%"
	}
	if known != "" {
		return known + "% (partial)"
	}
	return "unavailable"
}

func usageSessionStatus(value usage.SessionSummary) string {
	parts := append([]string{}, value.Warnings...)
	if len(value.Unpriced) > 0 {
		parts = append(parts, "partial cost: "+textList(value.Unpriced))
	}
	return textList(parts)
}

func optionalCost(value *string) string {
	if value == nil {
		return "unavailable"
	}
	return *value
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
