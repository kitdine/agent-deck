package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/kitdine/agent-deck/internal/output"
	"github.com/kitdine/agent-deck/internal/scanruntime"
	"github.com/kitdine/agent-deck/internal/store"
	"github.com/kitdine/agent-deck/internal/telemetry"
)

const telemetryRedactedToken = "<redacted: run with --reveal-local-token>"

// errTelemetryImportUnsupported is a malformed or unsupported import file;
// like other failed reads it exits 1, not as a usage error.
var errTelemetryImportUnsupported = errors.New("telemetry_import_unsupported")

// saveTelemetryConfig is replaced by tests to inject a receiver save failure.
var saveTelemetryConfig = telemetry.SaveConfig

// telemetryCodexError is a failed Codex configuration outcome; its text says
// what, if anything, was changed.
// It exits 1 with its code as the error code and its text on stderr.
type telemetryCodexError struct {
	code string
	text string
}

func (e *telemetryCodexError) Error() string { return e.code + ": " + e.text }

// telemetryCodexBlockRemains reports a failed enable whose managed block could
// not be taken back; its digest record is kept, so disable removes it.
func telemetryCodexBlockRemains() error {
	return &telemetryCodexError{code: "codex_config_write_failed", text: "~/.codex/config.toml was updated, but the receiver was not enabled and the [otel] block could not be taken back.\nRun agentdeck telemetry disable to remove the block if it is unchanged."}
}

// telemetryServeEventPublication is the line the App-attached listener
// writes on stdout when its owner needs a fresh publication.
const telemetryServeEventPublication = "publication_needed"

func newTelemetryCommand(opts *commandOptions) *cobra.Command {
	command := &cobra.Command{
		Use:   "telemetry",
		Short: "Manage the optional local OTel usage receiver",
		Long: "Manage AgentDeck's optional local OTel receiver. It listens only on 127.0.0.1, keeps only usage, model and identity fields, " +
			"and changes Codex configuration only when you ask for it.",
	}
	command.AddCommand(newTelemetryStatusCommand(opts), newTelemetryEnableCommand(opts), newTelemetryDisableCommand(opts),
		newTelemetryConfigPreviewCommand(opts), newTelemetryImportCommand(opts), newTelemetryServeCommand(opts))
	return command
}

func requireTextOrJSON(opts *commandOptions, command string) error {
	if opts.format != "text" && opts.format != "json" {
		return &inputError{err: fmt.Errorf("%s supports only text or json format", command)}
	}
	return nil
}

func writeTelemetryResult(opts *commandOptions, command string, data any, warnings []string) error {
	envelope := output.New(command, data, time.Now())
	envelope.Warnings = append(envelope.Warnings, warnings...)
	return json.NewEncoder(opts.stdout).Encode(envelope)
}

func newTelemetryStatusCommand(opts *commandOptions) *cobra.Command {
	return &cobra.Command{
		Use:     "status",
		Short:   "Show receiver, listener, metering, coverage and Codex configuration facts",
		Example: "  agentdeck telemetry status\n  agentdeck --format json telemetry status",
		Args:    exactArgs(0),
		RunE: func(command *cobra.Command, _ []string) error {
			if err := requireTextOrJSON(opts, "telemetry status"); err != nil {
				return err
			}
			status, err := readTelemetryStatus(command.Context(), opts)
			if err != nil {
				return err
			}
			if opts.format == "json" {
				return writeTelemetryResult(opts, "telemetry.status", status, status.Warnings)
			}
			return renderTelemetryStatus(opts.stdout, status)
		},
	}
}

func readTelemetryStatus(ctx context.Context, opts *commandOptions) (telemetry.Status, error) {
	stateRoot, err := opts.stateRoot()
	if err != nil {
		return telemetry.Status{}, err
	}
	home, err := userHomeDir()
	if err != nil {
		return telemetry.Status{}, err
	}
	return telemetry.ReadStatus(ctx, stateRoot, home, func() (*store.Store, error) { return store.OpenReadOnly(ctx, stateRoot) })
}

var telemetryReasonText = map[string]string{
	telemetry.ReasonProducerUnsupported: "producer version not verified",
	telemetry.ReasonNoProducerObserved:  "no verified producer event received yet",
}

func renderTelemetryStatus(w io.Writer, status telemetry.Status) error {
	var b strings.Builder
	fmt.Fprintf(&b, "Receiver: %s\n", status.Receiver)
	enabled := status.Receiver == "enabled"
	if enabled {
		switch {
		case status.Listener == "running" && status.EngineMode == "app":
			b.WriteString("Listener: running (App-attached)\n")
		case status.Listener == "running":
			b.WriteString("Listener: running (foreground)\n")
		case status.ListenerReason != nil && *status.ListenerReason == telemetry.ReasonMemoriesOff:
			b.WriteString("Listener: stopped (Codex Memories is off; collection is not needed)\n")
		case status.Listener == "failed" && status.ListenerReason != nil:
			fmt.Fprintf(&b, "Listener: failed (%s)\n", *status.ListenerReason)
		default:
			fmt.Fprintf(&b, "Listener: %s (it runs while the App runs)\n", status.Listener)
		}
		fmt.Fprintf(&b, "Producer configuration: %s\n", status.ProducerConfigured)
	}
	fmt.Fprintf(&b, "Codex Memories: %s\n", status.CodexMemories)
	fmt.Fprintf(&b, "Codex exporter: %s\n", status.CodexExporter)
	if enabled {
		if status.LastReceivedAt == nil {
			b.WriteString("Reception: waiting for first verified event\n")
		} else {
			fmt.Fprintf(&b, "Last valid event: %s\n", renderDisplayTimeWithZoneString(*status.LastReceivedAt))
		}
		b.WriteString("Connection: unknown (independent HTTP requests)\n")
		if status.LastRefusedAt != nil {
			fmt.Fprintf(&b, "Last refused request: %s (%s)\n", renderDisplayTimeWithZoneString(*status.LastRefusedAt), *status.LastRefusal)
		}
		fmt.Fprintf(&b, "Metering: %s\n", status.Metering)
		if status.ReasonCode != nil {
			fmt.Fprintf(&b, "Reason: %s\n", telemetryReasonText[*status.ReasonCode])
		}
		if status.BackgroundSessions > 0 {
			fmt.Fprintf(&b, "Background sessions counted: %d\n", status.BackgroundSessions)
		}
		fmt.Fprintf(&b, "Pending observations: %d\n", status.PendingObservations)
		if status.ConflictObservations > 0 {
			fmt.Fprintf(&b, "Conflicting observations: %d\n", status.ConflictObservations)
		}
		if status.InvalidObservations > 0 {
			fmt.Fprintf(&b, "Invalid observations: %d\n", status.InvalidObservations)
		}
		fmt.Fprintf(&b, "Historical coverage: %s\n", status.HistoricalCoverage)
		if len(status.Gaps) > 0 {
			fmt.Fprintf(&b, "Known gaps: %d\n", len(status.Gaps))
		}
		if status.PendingObservations > 0 {
			b.WriteString("No pending tokens were added to totals.\n")
		}
	}
	for _, warning := range status.Warnings {
		if warning == telemetry.WarningMemoriesNotCounted {
			b.WriteString("Warning: Codex Memories background usage is not counted.\n")
			b.WriteString("Run agentdeck telemetry enable --write-codex-config to count it.\n")
		}
	}
	_, err := io.WriteString(w, b.String())
	return err
}

func renderDisplayTimeWithZoneString(value string) string {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return value
	}
	return renderDisplayTimeWithZone(parsed)
}

type telemetryReceiverResult struct {
	Receiver      string `json:"receiver"`
	Changed       bool   `json:"changed"`
	Port          int    `json:"port"`
	Endpoint      string `json:"endpoint"`
	Listener      string `json:"listener"`
	CodexConfig   string `json:"codex_config"`
	CodexExporter string `json:"codex_exporter"`
}

func newTelemetryEnableCommand(opts *commandOptions) *cobra.Command {
	var port int
	var writeCodex bool
	command := &cobra.Command{
		Use:     "enable",
		Short:   "Enable the AgentDeck receiver; --write-codex-config also adds the managed Codex [otel] block",
		Example: "  agentdeck telemetry enable\n  agentdeck telemetry enable --write-codex-config\n  agentdeck telemetry enable --port 4319",
		Args:    exactArgs(0),
		RunE: func(command *cobra.Command, _ []string) error {
			return setTelemetryEnabled(opts, true, port, command.Flags().Changed("port"), writeCodex)
		},
	}
	command.Flags().IntVar(&port, "port", telemetry.DefaultPort, "Loopback port for the receiver")
	command.Flags().BoolVar(&writeCodex, "write-codex-config", false, "Append AgentDeck's managed [otel] block to ~/.codex/config.toml (your consent)")
	return command
}

func newTelemetryDisableCommand(opts *commandOptions) *cobra.Command {
	return &cobra.Command{
		Use:     "disable",
		Short:   "Disable the receiver and remove AgentDeck's managed Codex block; stored data is kept",
		Example: "  agentdeck telemetry disable",
		Args:    exactArgs(0),
		RunE: func(command *cobra.Command, _ []string) error {
			return setTelemetryEnabled(opts, false, 0, false, false)
		},
	}
}

func setTelemetryEnabled(opts *commandOptions, enabled bool, port int, portSet, writeCodex bool) error {
	name := "telemetry.disable"
	if enabled {
		name = "telemetry.enable"
	}
	if err := requireTextOrJSON(opts, strings.ReplaceAll(name, ".", " ")); err != nil {
		return err
	}
	if portSet && (port < 1 || port > 65535) {
		return &inputError{err: fmt.Errorf("port %d is out of range", port)}
	}
	stateRoot, err := opts.stateRoot()
	if err != nil {
		return err
	}
	home, err := userHomeDir()
	if err != nil {
		return err
	}
	config, err := telemetry.LoadConfig(stateRoot)
	if err != nil {
		return err
	}
	next := config
	next.Enabled = enabled
	if portSet {
		next.Port = port
	}
	codexConfig := "unchanged"
	var warnings []string
	var stderrText string
	if enabled {
		token, err := telemetry.EnsureToken(stateRoot)
		if err != nil {
			return err
		}
		if writeCodex {
			facts := telemetry.ReadCodexFacts(home, next.Port)
			if facts.Exporter != telemetry.ExporterManaged && facts.Exporter != telemetry.ExporterAgentDeck {
				// The Codex write happens first: when it is refused, nothing
				// at all changes, including the receiver preference.
				switch err := telemetry.WriteManagedBlock(home, stateRoot, next.Port, token, time.Now()); {
				case errors.Is(err, telemetry.ErrCodexOTelConflict):
					return &telemetryCodexError{code: "codex_otel_conflict", text: "~/.codex/config.toml already has an [otel] section.\nNothing was changed. Run agentdeck telemetry config-preview and merge it yourself."}
				case errors.Is(err, telemetry.ErrCodexConfigBlockRemains):
					return telemetryCodexBlockRemains()
				case err != nil:
					return &telemetryCodexError{code: "codex_config_write_failed", text: "~/.codex/config.toml could not be updated.\nNothing was changed. Retry, or run agentdeck telemetry config-preview and merge it yourself."}
				}
				codexConfig = "written"
			}
		}
	} else {
		removed, err := telemetry.RemoveManagedBlock(home, stateRoot)
		switch {
		case errors.Is(err, telemetry.ErrCodexConfigRestoreIncomplete):
			codexConfig = "restore_incomplete"
			warnings = append(warnings, "codex_config_restore_incomplete")
			stderrText = "codex_config_restore_incomplete: the [otel] block was changed elsewhere and was left untouched.\n"
		case err != nil:
			return err
		case removed:
			codexConfig = "removed"
		}
	}
	changed := next != config
	if changed {
		if err := saveTelemetryConfig(stateRoot, next); err != nil {
			// The receiver stays off, so a block written for it is taken back.
			if codexConfig == "written" {
				if _, removeErr := telemetry.RemoveManagedBlock(home, stateRoot); removeErr != nil {
					return telemetryCodexBlockRemains()
				}
			}
			return err
		}
	}
	listener, err := telemetry.ReadListenerState(stateRoot)
	if err != nil {
		return err
	}
	facts := telemetry.ReadCodexFacts(home, next.Port)
	result := telemetryReceiverResult{Receiver: map[bool]string{true: "enabled", false: "disabled"}[next.Enabled], Changed: changed,
		Port: next.Port, Endpoint: telemetry.Endpoint(next.Port), Listener: listener.State, CodexConfig: codexConfig, CodexExporter: facts.Exporter}
	if stderrText != "" && opts.format == "text" {
		_, _ = io.WriteString(opts.stderr, stderrText)
	}
	if opts.format == "json" {
		return writeTelemetryResult(opts, name, result, warnings)
	}
	if opts.quiet {
		return nil
	}
	var b strings.Builder
	switch {
	case enabled && !changed:
		b.WriteString("Receiver already enabled.\n")
	case enabled:
		b.WriteString("Receiver enabled.\n")
	case !changed:
		b.WriteString("Receiver already disabled.\n")
	default:
		b.WriteString("Receiver disabled. Stored observations and the journal are kept.\n")
	}
	switch codexConfig {
	case "written":
		b.WriteString("Appended AgentDeck's [otel] block to ~/.codex/config.toml.\nRestart Codex for it to take effect.\n")
	case "removed":
		b.WriteString("Removed AgentDeck's [otel] block from ~/.codex/config.toml.\nRestart Codex to stop exporting.\n")
	}
	if enabled {
		fmt.Fprintf(&b, "Endpoint: %s\n", result.Endpoint)
		if codexConfig == "unchanged" && facts.Exporter != telemetry.ExporterManaged && facts.Exporter != telemetry.ExporterAgentDeck {
			b.WriteString("Codex configuration was not changed. Run agentdeck telemetry enable --write-codex-config, or merge agentdeck telemetry config-preview yourself.\n")
		}
		if facts.CollectionBlocked() {
			b.WriteString("Codex Memories is off; nothing needs collection, so the receiver stays stopped.\n")
		} else if listener.State != "running" {
			b.WriteString("The receiver listens while the App runs.\n")
		}
	}
	_, err = io.WriteString(opts.stdout, b.String())
	return err
}

type telemetryConfigPreview struct {
	Applied     bool   `json:"applied"`
	Endpoint    string `json:"endpoint"`
	TokenHeader string `json:"token_header"`
	TokenShown  bool   `json:"token_shown"`
	Snippet     string `json:"snippet"`
}

func newTelemetryConfigPreviewCommand(opts *commandOptions) *cobra.Command {
	var reveal bool
	command := &cobra.Command{
		Use:   "config-preview",
		Short: "Show a Codex [otel] configuration proposal; nothing is applied",
		Long: "Show a user-level Codex [otel] configuration proposal for review. Nothing is written. Merge it yourself, or run " +
			"agentdeck telemetry enable --write-codex-config. The local token is redacted unless --reveal-local-token is given.",
		Example: "  agentdeck telemetry config-preview\n  agentdeck telemetry config-preview --reveal-local-token",
		Args:    exactArgs(0),
		RunE: func(command *cobra.Command, _ []string) error {
			if err := requireTextOrJSON(opts, "telemetry config-preview"); err != nil {
				return err
			}
			stateRoot, err := opts.stateRoot()
			if err != nil {
				return err
			}
			config, err := telemetry.LoadConfig(stateRoot)
			if err != nil {
				return err
			}
			token := telemetryRedactedToken
			if reveal {
				if token, err = telemetry.EnsureToken(stateRoot); err != nil {
					return err
				}
			}
			preview := telemetryConfigPreview{
				Endpoint: telemetry.Endpoint(config.Port), TokenHeader: telemetry.TokenHeader, TokenShown: reveal,
				Snippet: codexOTelSnippet(telemetry.Endpoint(config.Port), token),
			}
			if opts.format == "json" {
				return writeResult(opts.stdout, opts.format, "telemetry.config-preview", preview)
			}
			var b strings.Builder
			b.WriteString("# Proposed configuration; Codex config was not changed.\n# Merge with your user-level configuration after review.\n")
			b.WriteString(preview.Snippet)
			if !reveal {
				b.WriteString("# Local token is redacted by default.\n")
			}
			if !config.Enabled {
				b.WriteString("# The AgentDeck receiver is disabled; run agentdeck telemetry enable.\n")
			}
			_, err = io.WriteString(opts.stdout, b.String())
			return err
		},
	}
	command.Flags().BoolVar(&reveal, "reveal-local-token", false, "Include the local token so the snippet can be copied")
	return command
}

func codexOTelSnippet(endpoint, token string) string {
	return "[otel]\n" +
		"exporter = { otlp-http = { endpoint = \"" + endpoint + "\", protocol = \"json\", headers = { \"" + telemetry.TokenHeader + "\" = \"" + token + "\" } } }\n" +
		"log_user_prompt = false\n"
}

type telemetryImportResult struct {
	Records     int `json:"records"`
	Inserted    int `json:"inserted"`
	Duplicate   int `json:"duplicate"`
	Pending     int `json:"pending"`
	CaptureOnly int `json:"capture_only"`
	Conflict    int `json:"conflict"`
	Invalid     int `json:"invalid"`
	Unmetered   int `json:"unmetered"`
}

func newTelemetryImportCommand(opts *commandOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "import <otel-json-file>",
		Short: "Import an OTLP JSON logs file through the same validation",
		Long: argumentHelp("Import a real OTLP/HTTP JSON logs export you name explicitly. Records pass the same allowlist, validation, ownership rules and idempotent keys as the receiver. Repeating an import is a no-op.",
			"  otel-json-file  OTLP/HTTP JSON logs export, at most 4 MiB."),
		Example: "  agentdeck telemetry import ./otel-logs.json",
		Args:    exactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if err := requireTextOrJSON(opts, "telemetry import"); err != nil {
				return err
			}
			body, err := readTelemetryImport(args[0])
			if err != nil {
				return err
			}
			records, err := telemetry.Decode(body)
			if err != nil {
				return fmt.Errorf("%w: the file is not a supported OTLP JSON logs export", errTelemetryImportUnsupported)
			}
			database, stateRoot, err := opts.openStore(command.Context())
			if err != nil {
				return err
			}
			defer database.Close()
			reconciler := &telemetry.Reconciler{Store: database, Now: time.Now, SessionsChanged: func(ctx context.Context) error {
				index, err := store.OpenSessions(ctx, stateRoot)
				if err != nil {
					return err
				}
				defer index.Close()
				return telemetry.ProjectSessionIndex(ctx, database, index)
			}}
			applied, err := reconciler.Import(command.Context(), records, time.Now())
			if err != nil {
				return err
			}
			if _, err := reconciler.Settle(command.Context()); err != nil {
				return err
			}
			result := telemetryImportResult{Records: len(records), Inserted: applied.Inserted, Duplicate: applied.Duplicate,
				Pending: applied.Pending, CaptureOnly: applied.CaptureOnly, Conflict: applied.Conflict, Invalid: applied.Invalid, Unmetered: applied.Unmetered}
			partial := applied.Pending > 0 || applied.CaptureOnly > 0 || applied.Conflict > 0
			if opts.format == "json" {
				envelope := output.New("telemetry.import", result, time.Now())
				envelope.Partial = partial
				if partial {
					envelope.Warnings = append(envelope.Warnings, "telemetry_pending_not_counted")
				}
				return json.NewEncoder(opts.stdout).Encode(envelope)
			}
			var b strings.Builder
			if applied.Inserted == 0 {
				fmt.Fprintf(&b, "No new observations (%d already imported).\n", applied.Duplicate)
			} else {
				fmt.Fprintf(&b, "Imported %d observations (%d already present).\n", applied.Inserted, applied.Duplicate)
			}
			fmt.Fprintf(&b, "Pending: %d, capture only: %d, conflicting: %d, invalid: %d, not usage: %d\n", applied.Pending, applied.CaptureOnly, applied.Conflict, applied.Invalid, applied.Unmetered)
			if partial {
				b.WriteString("Pending observations were not added to totals.\n")
			}
			_, err = io.WriteString(opts.stdout, b.String())
			return err
		},
	}
}

func readTelemetryImport(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, &inputError{err: fmt.Errorf("cannot read %s", path)}
	}
	defer file.Close()
	body, err := io.ReadAll(io.LimitReader(file, telemetry.MaxBodyBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > telemetry.MaxBodyBytes {
		return nil, &inputError{err: errors.New("the file exceeds the 4 MiB OTLP request limit")}
	}
	return body, nil
}

// newTelemetryServeCommand runs the listener in the foreground. The App
// starts it with --owner app and holds its stdin; the listener exits when
// that pipe closes, so it never outlives the App. With --owner app it writes
// publication_needed on stdout after counted usage changed.
func newTelemetryServeCommand(opts *commandOptions) *cobra.Command {
	var owner string
	command := &cobra.Command{
		Use:         "serve",
		Short:       "Run the receiver in the foreground until interrupted",
		Hidden:      true,
		Annotations: map[string]string{internalRuntimeSurfaceOnlyAnnotation: "true"},
		Args:        exactArgs(0),
		RunE: func(command *cobra.Command, _ []string) error {
			if owner != "app" && owner != "foreground" {
				return &inputError{err: fmt.Errorf("invalid owner %q", owner)}
			}
			stateRoot, err := opts.stateRoot()
			if err != nil {
				return err
			}
			home, err := userHomeDir()
			if err != nil {
				return err
			}
			ctx, stop := signal.NotifyContext(command.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			var published func()
			if owner == "app" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				defer cancel()
				go func() {
					_, _ = io.Copy(io.Discard, opts.stdin)
					cancel()
				}()
				var mu sync.Mutex
				events := bufio.NewWriter(opts.stdout)
				published = func() {
					mu.Lock()
					defer mu.Unlock()
					_, _ = events.WriteString(telemetryServeEventPublication + "\n")
					_ = events.Flush()
				}
			}
			database, err := openTelemetryStore(ctx, stateRoot)
			if err != nil {
				return err
			}
			defer database.Close()
			err = telemetry.Serve(ctx, telemetry.ServeOptions{
				StateRoot: stateRoot, Home: home, Owner: owner, Store: database, Published: published,
				Inventory: func(ctx context.Context) error {
					result, err := (scanruntime.Client{StateRoot: stateRoot, Home: home, AppOwned: owner == "app", Background: true}).Request(ctx, scanruntime.ScopeBoth)
					if err == nil && (result.Usage.State != "completed" || result.Session.State != "completed") {
						err = errors.New("telemetry inventory reconciliation incomplete")
					}
					return err
				},
			})
			switch {
			case errors.Is(err, telemetry.ErrDisabled):
				return &inputError{err: errors.New("the telemetry receiver is disabled; run agentdeck telemetry enable")}
			case errors.Is(err, telemetry.ErrCollectionNotNeeded):
				// The linkage keeps the receiver stopped; that is not a failure.
				return nil
			}
			return err
		},
	}
	command.Flags().StringVar(&owner, "owner", "foreground", "Listener owner: app or foreground")
	return command
}

// openTelemetryStore opens the core store for the long-lived listener. On
// first launch the App's scan worker can hold the state lock while it creates
// and migrates the database for longer than one lock wait; the listener keeps
// retrying instead of exiting, for at most two minutes or until its owner
// goes away.
func openTelemetryStore(ctx context.Context, stateRoot string) (*store.Store, error) {
	deadline := time.Now().Add(2 * time.Minute)
	for {
		database, err := store.Open(ctx, stateRoot)
		var contention *store.ErrLockContention
		if err == nil || !(errors.As(err, &contention) || errors.Is(err, store.ErrStateBusy)) || time.Now().After(deadline) {
			return database, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Second):
		}
	}
}
