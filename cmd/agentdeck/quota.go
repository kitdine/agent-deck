package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/kitdine/agent-deck/internal/desktop"
	"github.com/kitdine/agent-deck/internal/quota"
	"github.com/kitdine/agent-deck/internal/store"
	"github.com/kitdine/agent-deck/internal/usagehook"
)

// quotaMaxBackoff bounds C9's geometric backoff for the desktop quota refresh.
const quotaMaxBackoff = time.Hour

// A refresh probes Codex (5s) and Claude (10s) sequentially. A small margin
// lets a second helper process join the serialized refresh boundary without
// ever allowing an older completion to replace a newer process's state.
const quotaRefreshLockTimeout = 20 * time.Second

// Seam so tests never spawn a real client. The helper posts no notification:
// due alerts are returned to the app, which delivers them (C10).
var quotaRefreshService = func(stateRoot, home string) desktop.Service {
	return desktop.Service{StateRoot: stateRoot, Home: home}
}

var quotaExecutable = os.Executable

// Seamed only so the failed-persistence rollback can be exercised without
// corrupting a real SQLite database in a command test.
var saveQuotaStatusLineSettings = quota.SaveSettings

// Seamed only so a command test can assert a no-op quota-settings read never
// calls this, without corrupting a real SQLite database.
var saveQuotaSettings = quota.SaveSettings

func newQuotaCommand(opts *commandOptions) *cobra.Command {
	command := &cobra.Command{
		Use:   "quota",
		Short: "Show subscription quota",
		Long: "Show each client's subscription quota as last recorded: the gate, source, freshness, windows, and reset allowance. " +
			"It reads stored state only, probes no client, and writes nothing.",
		Example: "  agentdeck quota\n  agentdeck --format json quota",
		Args:    exactArgs(0),
		RunE: func(command *cobra.Command, _ []string) error {
			return runQuota(command.Context(), opts)
		},
	}
	command.AddCommand(newQuotaCaptureCommand(opts))
	return command
}

// runQuota is C12. The gate and probe failures are payload states at exit 0;
// only a condition that prevents producing a payload returns an error.
func runQuota(ctx context.Context, opts *commandOptions) error {
	if opts.format != "text" && opts.format != "json" {
		return &inputError{err: errors.New("quota supports only text or json format")}
	}
	subscription, err := loadQuotaSubscription(ctx, opts)
	if err != nil {
		return err
	}
	if opts.format == "json" {
		return writeResult(opts.stdout, opts.format, "quota", subscription)
	}
	return renderQuotaText(opts.stdout, subscription)
}

func loadQuotaSubscription(ctx context.Context, opts *commandOptions) (desktop.SubscriptionSnapshot, error) {
	stateRoot, err := opts.stateRoot()
	if err != nil {
		return desktop.SubscriptionSnapshot{}, err
	}
	home, err := userHomeDir()
	if err != nil {
		return desktop.SubscriptionSnapshot{}, err
	}
	if _, err := os.Stat(filepath.Join(stateRoot, "agentdeck.sqlite3")); errors.Is(err, os.ErrNotExist) {
		// A fresh installation has no state: every setting is its default, so
		// reading is off (requirements.md clause 1).
		return desktop.ReadingOffSubscription(), nil
	}
	core, err := store.OpenReadOnly(ctx, stateRoot)
	if err != nil {
		return desktop.SubscriptionSnapshot{}, err
	}
	defer core.Close()
	return desktop.Service{StateRoot: stateRoot, Home: home}.BuildSubscription(ctx, core, time.Now())
}

func renderQuotaText(w io.Writer, subscription desktop.SubscriptionSnapshot) error {
	var b strings.Builder
	if !subscription.Available {
		b.WriteString("Subscription quota is unavailable.\n")
	}
	for i, client := range subscription.Clients {
		if i > 0 {
			b.WriteString("\n")
		}
		name := quotaClientName(client.Client)
		if !client.Applicable {
			fmt.Fprintf(&b, "%s: not applicable, %s\n", name, quotaReasonPhrase(client.ApplicableReason))
			continue
		}
		if client.Failure != nil && len(client.Windows) == 0 {
			fmt.Fprintf(&b, "%s: no figures, %s", name, quotaReasonPhrase(client.Failure))
			if client.ObservedAt != nil {
				fmt.Fprintf(&b, " (attempted %s)", renderDisplayTimeWithZone(*client.ObservedAt))
			}
			b.WriteString("\n")
			continue
		}
		header := []string{name}
		if client.Plan != nil {
			header = append(header, "plan "+*client.Plan)
		} else if client.PlanReason != nil {
			// Codex PR #5 sixth review, P2: a nil Plan with a PlanReason (e.g.
			// Codex omitting planType, or Claude's always-unsupported plan)
			// must still surface the field as unavailable with its reason,
			// matching the quota contract, instead of silently omitting it.
			header = append(header, "plan "+quotaReasonPhrase(client.PlanReason))
		}
		if client.Source != nil {
			header = append(header, "via "+*client.Source)
		}
		if client.ObservedAt != nil {
			header = append(header, "observed "+renderDisplayTimeWithZone(*client.ObservedAt))
		}
		if client.Stale {
			header = append(header, "stale")
		}
		if client.Failure != nil {
			header = append(header, "last "+quotaReasonPhrase(client.Failure))
		}
		b.WriteString(strings.Join(header, ", ") + "\n")
		if !client.AttributionConfirmed {
			b.WriteString("  account attribution cannot be confirmed\n")
		}
		for _, window := range client.Windows {
			resets := "reset time not reported"
			if window.ResetsAt != nil {
				resets = "resets " + renderDisplayTimeWithZone(*window.ResetsAt)
			}
			marker := ""
			if client.TightestWindowKey != nil && *client.TightestWindowKey == window.Key {
				marker = "  tightest"
			}
			fmt.Fprintf(&b, "  %-32s %s  %s%s", quotaWindowName(window), quotaPercentText(window.UsedPercent), resets, marker)
			// Codex PR #5 sixth review, P2: a partial mixed-age Claude update
			// can leave one window observed well before the client-level
			// timestamp above; print each window's own instant rather than
			// let it be read as observed then too.
			if window.ObservedAt != nil {
				fmt.Fprintf(&b, "  (observed %s)", renderDisplayTimeWithZone(*window.ObservedAt))
			}
			// Codex PR #5 ninth review, P2: the same partial mixed-age
			// update can leave one window from a different route than the
			// header's newest-source label; print each row's own source.
			if window.Source != nil {
				fmt.Fprintf(&b, "  (via %s)", *window.Source)
			}
			b.WriteString("\n")
		}
		if allowance := client.ResetAllowance; allowance != nil {
			var details []string
			if allowance.Remaining != nil {
				details = append(details, fmt.Sprintf("%d remaining", *allowance.Remaining))
			} else if allowance.RemainingReason != nil {
				details = append(details, "remaining "+quotaReasonPhrase(allowance.RemainingReason))
			}
			if allowance.Total != nil {
				details = append(details, fmt.Sprintf("%d total", *allowance.Total))
			} else if allowance.TotalReason != nil {
				details = append(details, "total "+quotaReasonPhrase(allowance.TotalReason))
			}
			if len(details) > 0 {
				fmt.Fprintf(&b, "  reset allowance: %s\n", strings.Join(details, ", "))
			}
		} else if client.ResetAllowanceReason != nil {
			// Codex PR #5 ninth review, P2: Claude's normal success path
			// carries ResetAllowanceReason=not_reported with no allowance
			// struct at all; this branch previously emitted nothing,
			// silently dropping the field instead of reporting it
			// unavailable with its reason like every other field here.
			fmt.Fprintf(&b, "  reset allowance: %s\n", quotaReasonPhrase(client.ResetAllowanceReason))
		}
		if client.ObservedResetAt != nil {
			fmt.Fprintf(&b, "  last observed reset: %s\n", renderDisplayTimeWithZone(*client.ObservedResetAt))
		}
	}
	_, err := io.WriteString(w, b.String())
	return err
}

func quotaClientName(client string) string {
	switch client {
	case "codex":
		return "Codex"
	case "claude":
		return "Claude"
	default:
		return client
	}
}

func quotaReasonPhrase(reason *string) string {
	if reason == nil {
		return "unknown"
	}
	switch quota.Reason(*reason) {
	case quota.ReasonProbeDisabled:
		return "quota reading is off"
	case quota.ReasonNotOfficial:
		return "provider is not official"
	case quota.ReasonNeverProbed:
		return "not yet observed"
	case quota.ReasonProbeFailed:
		return "probe failed"
	case quota.ReasonParseFailed:
		return "output not recognized"
	case quota.ReasonNotConsented:
		return "status-line route not consented"
	case quota.ReasonNotReported:
		return "not reported"
	default:
		return *reason
	}
}

// quotaPercentText renders usedPercent with only as much precision as the
// value actually needs. Codex PR #5 ninth review, P2: a structured Claude
// observation can carry a fractional usedPercent, and a flat "%4.0f%%"
// rounds a value like 89.6 up to a displayed "90%" that has not actually
// crossed the 90% threshold this same text implies.
func quotaPercentText(value float64) string {
	if math.Abs(math.Round(value)-value) < 0.05 {
		return fmt.Sprintf("%4.0f%%", value)
	}
	return fmt.Sprintf("%5.1f%%", value)
}

// quotaWindowName labels a window from its vendor label and length. The
// window key is opaque and never shown (C6).
func quotaWindowName(window desktop.SubscriptionWindow) string {
	length := ""
	if window.WindowMinutes != nil {
		minutes := *window.WindowMinutes
		switch {
		case minutes%1440 == 0:
			length = fmt.Sprintf("%d-day", minutes/1440)
		case minutes%60 == 0:
			length = fmt.Sprintf("%d-hour", minutes/60)
		default:
			length = fmt.Sprintf("%d-minute", minutes)
		}
	}
	switch {
	case window.Label != nil && length != "":
		return fmt.Sprintf("%s (%s)", *window.Label, length)
	case window.Label != nil:
		return *window.Label
	case length != "":
		return length + " window"
	default:
		return "window"
	}
}

// quotaAgentDeckCommand is the command registered in Claude's statusLine,
// built the same way as the usage hook's so RestoreStatusLine recognizes it.
func quotaAgentDeckCommand(opts *commandOptions) string {
	command := "agentdeck"
	if executable, err := quotaExecutable(); err == nil && filepath.Base(executable) == "agentdeck" {
		// Direct-download installs do not create a global `agentdeck` symlink.
		// The running embedded helper is nevertheless executable at this stable
		// bundle path, so register that exact path with shell-safe quoting.
		command = shellQuote(executable)
	}
	if opts.stateDir != "" {
		// Codex PR #5 fifth review, P2: this command is persisted into
		// Claude's settings.json and invoked later from whatever working
		// directory Claude's own session or project happens to have at
		// refresh time, not this process's. A relative --state-dir would
		// then resolve against that different directory and open the wrong
		// (or a nonexistent) state root. filepath.Abs resolves against this
		// process's cwd, while it is still known; an already-absolute path
		// passes through unchanged. A resolution failure falls back to the
		// original value rather than silently dropping --state-dir.
		stateDir := opts.stateDir
		if abs, err := filepath.Abs(stateDir); err == nil {
			stateDir = abs
		}
		command += " --state-dir " + shellQuote(stateDir)
	}
	return command
}

func quotaStatusLineManager(opts *commandOptions, stateRoot string) (*usagehook.Manager, error) {
	home, err := userHomeDir()
	if err != nil {
		return nil, err
	}
	return usagehook.New(usagehook.Environment{Home: home, AgentDeckCommand: quotaAgentDeckCommand(opts), StateDir: stateRoot}), nil
}

type desktopQuotaRefreshResult struct {
	GateReasons map[string]*string  `json:"gate_reasons"`
	Alerts      []desktopQuotaAlert `json:"alerts"`
}

// desktopQuotaAlert is one due notice for the app to post (C10). Threshold is
// present only for a threshold notice; label is the vendor's window label, and
// window_minutes is null when the length is not reported.
type desktopQuotaAlert struct {
	ID            string   `json:"id"`
	Kind          string   `json:"kind"`
	Client        string   `json:"client"`
	Label         string   `json:"label,omitempty"`
	WindowMinutes *int     `json:"window_minutes"`
	UsedPercent   float64  `json:"used_percent"`
	Threshold     *float64 `json:"threshold,omitempty"`
}

func desktopQuotaAlerts(due []quota.DueAlert) []desktopQuotaAlert {
	alerts := make([]desktopQuotaAlert, 0, len(due))
	for _, alert := range due {
		item := desktopQuotaAlert{ID: alert.ID, Kind: string(alert.Kind), Client: string(alert.Client), Label: alert.Label, UsedPercent: alert.UsedPercent}
		if alert.WindowMinutes > 0 {
			minutes := alert.WindowMinutes
			item.WindowMinutes = &minutes
		}
		if alert.Kind == quota.AlertThreshold {
			threshold := alert.Threshold
			item.Threshold = &threshold
		}
		alerts = append(alerts, item)
	}
	return alerts
}

func newDesktopQuotaRefreshCommand(opts *commandOptions) *cobra.Command {
	manual := false
	command := &cobra.Command{
		Use:   "quota-refresh",
		Short: "Probe subscription quota per the stored settings, then evaluate quota alerts",
		Args:  exactArgs(0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if opts.format != "json" {
				return &inputError{err: errors.New("desktop quota-refresh requires --format json")}
			}
			return runDesktopQuotaRefresh(cmd.Context(), opts, manual)
		},
	}
	command.Flags().BoolVar(&manual, "manual", false, "A user-initiated refresh: bypass the quota interval and backoff")
	return command
}

// runDesktopQuotaRefresh runs C9's schedule for both clients and then C10's
// evaluator, both under the stored settings, and returns the due alerts for
// the app to deliver; nothing is recorded as sent here. With reading off
// neither probes nor evaluates anything.
func runDesktopQuotaRefresh(ctx context.Context, opts *commandOptions, manual bool) (err error) {
	core, stateRoot, err := opts.openStore(ctx)
	if err != nil {
		return err
	}
	defer core.Close()
	refreshLock, err := store.AcquireQuotaRefreshLock(ctx, stateRoot, quotaRefreshLockTimeout)
	if err != nil {
		return err
	}
	defer func() {
		if releaseErr := refreshLock.Release(); err == nil && releaseErr != nil {
			err = releaseErr
		}
	}()
	home, err := userHomeDir()
	if err != nil {
		return err
	}
	settings, err := quota.LoadSettings(ctx, core)
	if err != nil {
		return err
	}
	trigger := quota.TriggerBackground
	if manual {
		trigger = quota.TriggerManual
	}
	outcome := quotaRefreshService(stateRoot, home).RefreshQuota(ctx, core, home, trigger, settings.ProbeEnabled, settings.ProbeInterval, quotaMaxBackoff)

	warnings := []string{}
	// outcome's empty reason means this cycle's C1 gate passed for that
	// client; anything else -- not_official, or probe_failed when the gate
	// itself could not be evaluated (Codex PR #5 eleventh review, P2: an
	// unknown gate is not a confirmed official one either) -- means the gate
	// did not affirmatively pass, and its retained windows must not generate
	// alerts until it does.
	officialClients := map[quota.Client]bool{
		quota.ClientCodex:  outcome[quota.ClientCodex] == "",
		quota.ClientClaude: outcome[quota.ClientClaude] == "",
	}
	due, err := quota.DueAlerts(ctx, quota.NewStore(core.DB), settings.ProbeEnabled, officialClients, settings.AlertConfig(), time.Now())
	if err != nil {
		warnings = append(warnings, "quota_alerts_failed")
	}
	result := desktopQuotaRefreshResult{GateReasons: map[string]*string{}, Alerts: desktopQuotaAlerts(due)}
	for client, reason := range outcome {
		var text *string
		if reason != "" {
			value := string(reason)
			text = &value
		}
		result.GateReasons[string(client)] = text
	}
	return writeEnvelope(opts.stdout, opts.format, "desktop.quota-refresh", result, len(warnings) > 0, warnings)
}

type desktopQuotaAlertsAckResult struct {
	Acknowledged int `json:"acknowledged"`
}

func newDesktopQuotaAlertsCommand(opts *commandOptions) *cobra.Command {
	command := &cobra.Command{Use: "quota-alerts", Short: "Acknowledge quota alerts the app delivered"}
	var ids []string
	ack := &cobra.Command{
		Use:   "ack",
		Short: "Record delivered quota alerts so they are not offered again",
		Args:  exactArgs(0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if opts.format != "json" {
				return &inputError{err: errors.New("desktop quota-alerts ack requires --format json")}
			}
			if len(ids) == 0 {
				return &inputError{err: errors.New("desktop quota-alerts ack requires at least one --id")}
			}
			return runDesktopQuotaAlertsAck(cmd.Context(), opts, ids)
		},
	}
	ack.Flags().StringArrayVar(&ids, "id", nil, "The id of a delivered alert, as returned by desktop quota-refresh; repeatable")
	command.AddCommand(ack)
	return command
}

// runDesktopQuotaAlertsAck records the ledger entries for alerts the app
// posted (C10). Every id is validated before any is recorded, so a malformed
// id records nothing.
func runDesktopQuotaAlertsAck(ctx context.Context, opts *commandOptions, ids []string) error {
	if err := quota.ValidateAlertIDs(ids); err != nil {
		return &inputError{err: err}
	}
	core, _, err := opts.openStore(ctx)
	if err != nil {
		return err
	}
	defer core.Close()
	store := quota.NewStore(core.DB)
	now := time.Now()
	for _, id := range ids {
		if err := quota.AcknowledgeAlert(ctx, store, id, now); err != nil {
			return err
		}
	}
	return writeResult(opts.stdout, opts.format, "desktop.quota-alerts.ack", desktopQuotaAlertsAckResult{Acknowledged: len(ids)})
}

type desktopQuotaSettingsView struct {
	Reading     bool      `json:"reading"`
	Interval    string    `json:"interval"`
	Alerts      bool      `json:"alerts"`
	Thresholds  []float64 `json:"thresholds"`
	ResetNotice bool      `json:"reset_notice"`
	StatusLine  bool      `json:"statusline"`
}

func quotaSettingsView(s quota.Settings) desktopQuotaSettingsView {
	return desktopQuotaSettingsView{
		Reading: s.ProbeEnabled, Interval: s.ProbeInterval.String(), Alerts: s.AlertsEnabled,
		Thresholds: s.AlertThresholds, ResetNotice: s.ResetNotice, StatusLine: s.StatusLineConsent,
	}
}

type desktopQuotaSettingsResult struct {
	Settings          desktopQuotaSettingsView `json:"settings"`
	StatusLineRestore *usagehook.Result        `json:"statusline_restore"`
}

func newDesktopQuotaSettingsCommand(opts *commandOptions) *cobra.Command {
	var reading, interval, alerts, thresholds, resetNotice string
	command := &cobra.Command{
		Use:   "quota-settings",
		Short: "Show or change subscription-quota settings",
		Args:  exactArgs(0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if opts.format != "json" {
				return &inputError{err: errors.New("desktop quota-settings requires --format json")}
			}
			flags := cmd.Flags()
			return runDesktopQuotaSettings(cmd.Context(), opts, func(next *quota.Settings) error {
				var err error
				if flags.Changed("reading") {
					if next.ProbeEnabled, err = quota.ParseSwitch(reading); err != nil {
						return err
					}
				}
				if flags.Changed("interval") {
					if next.ProbeInterval, err = quota.ParseProbeInterval(interval); err != nil {
						return err
					}
				}
				if flags.Changed("alerts") {
					if next.AlertsEnabled, err = quota.ParseSwitch(alerts); err != nil {
						return err
					}
				}
				if flags.Changed("thresholds") {
					if next.AlertThresholds, err = quota.ParseAlertThresholds(thresholds); err != nil {
						return err
					}
				}
				if flags.Changed("reset-notice") {
					if next.ResetNotice, err = quota.ParseSwitch(resetNotice); err != nil {
						return err
					}
				}
				return nil
			})
		},
	}
	command.Flags().StringVar(&reading, "reading", "", "Quota reading: on or off")
	command.Flags().StringVar(&interval, "interval", "", "Background probe interval: 5m, 15m, or 30m")
	command.Flags().StringVar(&alerts, "alerts", "", "Quota alerts: on or off")
	command.Flags().StringVar(&thresholds, "thresholds", "", "Alert thresholds: 75, 90, or 75,90")
	command.Flags().StringVar(&resetNotice, "reset-notice", "", "Notify when a window resets: on or off")
	return command
}

// runDesktopQuotaSettings applies requested changes. Turning reading off
// performs C9's transition as part of the same action: it restores an
// installed status-line route and clears the consent flag, and a file that
// changed underneath reports restore_incomplete. Stored observations are not
// touched.
func runDesktopQuotaSettings(ctx context.Context, opts *commandOptions, apply func(*quota.Settings) error) error {
	core, stateRoot, err := opts.openStore(ctx)
	if err != nil {
		return err
	}
	defer core.Close()
	current, err := quota.LoadSettings(ctx, core)
	if err != nil {
		return err
	}
	next := current
	if err := apply(&next); err != nil {
		return &inputError{err: err}
	}
	// Codex PR #5 tenth review, P2: the app loads settings by invoking this
	// command with no mutation flags, so apply is a no-op and next stays
	// equal to the just-read current. Saving unconditionally below turns
	// that load into a read-modify-write: a concurrent helper process's
	// confirmed write landing between this read and the save would be
	// silently undone by this call re-persisting the stale value it read.
	// Nothing changed, so there is nothing to validate or persist.
	if reflect.DeepEqual(next, current) {
		return writeResult(opts.stdout, opts.format, "desktop.quota-settings", desktopQuotaSettingsResult{Settings: quotaSettingsView(current), StatusLineRestore: nil})
	}

	var restore *usagehook.Result
	if current.ProbeEnabled && !next.ProbeEnabled {
		manager, err := quotaStatusLineManager(opts, stateRoot)
		if err != nil {
			return err
		}
		status, err := manager.StatusLineStatus()
		if err != nil {
			return err
		}
		// Restore only a route this state installed. ConfigurationModified
		// means something recognizable as an AgentDeck route is registered
		// but does not exactly match this state's own desired entry --
		// managedStatusLineCommand deliberately ignores --state-dir (for
		// RestoreStatusLine's dead-lock safety elsewhere), so that state is
		// reached just as readily by a *different* state directory's active,
		// consented route as by this state's own drifted one. Without
		// current.StatusLineConsent already true, RestoreStatusLine's
		// modified-managed branch would delete that other state's route
		// outright, with no prior record of its own to restore (Codex PR #5
		// sixth review, P2). A statusLine that is not AgentDeck's command at
		// all belongs to the user and is left alone either way.
		if current.StatusLineConsent || status.Configuration == usagehook.ConfigurationConfigured {
			result, err := manager.RestoreStatusLine()
			if err != nil {
				return err
			}
			restore = &result
		}
		if restore == nil || restore.Outcome != usagehook.OutcomeFailed {
			next.StatusLineConsent = false
		}
	}
	// Codex PR #5 eighth review, P2: on a failed restore, next still carries
	// StatusLineConsent=true with ProbeEnabled=false -- a combination
	// Validate() correctly refuses to persist, but only with its own generic
	// message, masking the more specific and actionable restore failure
	// below. Report that failure directly, before Validate ever sees this
	// intentionally unpersisted intermediate state; nothing here is saved.
	if restore != nil && restore.Outcome == usagehook.OutcomeFailed {
		if err := writeResult(opts.stdout, opts.format, "desktop.quota-settings", desktopQuotaSettingsResult{Settings: quotaSettingsView(current), StatusLineRestore: restore}); err != nil {
			return err
		}
		return fmt.Errorf("quota status-line restore failed: %s", restore.Error)
	}
	if err := next.Validate(); err != nil {
		return &inputError{err: err}
	}
	if err := saveQuotaSettings(ctx, core, next); err != nil {
		// Codex PR #5 twelfth review, P2: SetSettings commits its transaction
		// before running secureFiles (mirroring the statusline command's own
		// rollback exemption above), so this specific error means next is
		// already durably persisted -- only the post-commit permission
		// hardening failed. Returning here without writing a result would
		// make an already-committed write look uncommitted to the app (whose
		// transport reads decoded JSON, not this process's exit status), and
		// skip the settingsRow==nil-gated snapshot refresh that a real
		// reading/interval change is supposed to trigger.
		if !errors.Is(err, store.ErrSettingsSecureFilesFailed) {
			return err
		}
		if writeErr := writeResult(opts.stdout, opts.format, "desktop.quota-settings", desktopQuotaSettingsResult{Settings: quotaSettingsView(next), StatusLineRestore: restore}); writeErr != nil {
			return writeErr
		}
		if opts.stderr != nil {
			fmt.Fprintf(opts.stderr, "advisory: quota settings saved but permission hardening failed: %v\n", err)
		}
		return nil
	}
	if err := writeResult(opts.stdout, opts.format, "desktop.quota-settings", desktopQuotaSettingsResult{Settings: quotaSettingsView(next), StatusLineRestore: restore}); err != nil {
		return err
	}
	if restore != nil && restore.Outcome == usagehook.OutcomeFailed {
		return fmt.Errorf("quota status-line restore failed: %s", restore.Error)
	}
	return nil
}

type desktopQuotaStatusLineResult struct {
	Consent bool             `json:"consent"`
	Result  usagehook.Result `json:"result"`
}

func newDesktopQuotaStatusLineCommand(opts *commandOptions) *cobra.Command {
	command := &cobra.Command{Use: "quota-statusline", Short: "Consent to or withdraw the Claude status-line route"}
	for _, operation := range []string{"enable", "disable"} {
		operation := operation
		command.AddCommand(&cobra.Command{
			Use:   operation,
			Short: "Status-line route: " + operation,
			Args:  exactArgs(0),
			RunE: func(cmd *cobra.Command, _ []string) error {
				if opts.format != "json" {
					return &inputError{err: fmt.Errorf("desktop quota-statusline %s requires --format json", operation)}
				}
				return runDesktopQuotaStatusLine(cmd.Context(), opts, operation)
			},
		})
	}
	return command
}

// runDesktopQuotaStatusLine is the consent switch's control path (C3):
// enabling registers AgentDeck's capture command and requires reading to be
// on (requirements.md clause 14); disabling restores the prior configuration.
func runDesktopQuotaStatusLine(ctx context.Context, opts *commandOptions, operation string) error {
	core, stateRoot, err := opts.openStore(ctx)
	if err != nil {
		return err
	}
	defer core.Close()
	settings, err := quota.LoadSettings(ctx, core)
	if err != nil {
		return err
	}
	manager, err := quotaStatusLineManager(opts, stateRoot)
	if err != nil {
		return err
	}
	var result usagehook.Result
	switch operation {
	case "enable":
		if !settings.ProbeEnabled {
			return &inputError{err: errors.New("the status-line route requires quota reading to be on")}
		}
		if result, err = manager.SetupStatusLine(); err != nil {
			return err
		}
		if result.Outcome == usagehook.OutcomeConfigured || result.Outcome == usagehook.OutcomeUnchanged {
			settings.StatusLineConsent = true
		}
	default:
		if result, err = manager.RestoreStatusLine(); err != nil {
			return err
		}
		// RestoreStatusLine reports a failed removal as Outcome=Failed with no
		// Go error (checked below, after settings persist); the file's actual
		// on-disk state is then unknown, so consent must not be recorded as
		// revoked when AgentDeck's registration may still be active there.
		if result.Outcome != usagehook.OutcomeFailed {
			settings.StatusLineConsent = false
		}
	}
	if err := saveQuotaStatusLineSettings(ctx, core, settings); err != nil {
		// SetupStatusLine has already changed ~/.claude/settings.json. If the
		// consent bit cannot be persisted, undo a route this invocation newly
		// installed so capture cannot run without durable consent.
		//
		// Codex PR #5 ninth review, P2: SetSettings commits its transaction
		// before running secureFiles, so a chmod failure there returns
		// ErrSettingsSecureFilesFailed even though the consent bit is
		// already durably persisted. Rolling back the just-installed route
		// in that case would make the stored StatusLineConsent=true
		// disagree with the route actually left on disk.
		if operation == "enable" && result.Outcome == usagehook.OutcomeConfigured && !errors.Is(err, store.ErrSettingsSecureFilesFailed) {
			rollback, rollbackErr := manager.RestoreStatusLine()
			switch {
			case rollbackErr != nil:
				return errors.Join(err, fmt.Errorf("roll back quota status-line route: %w", rollbackErr))
			case rollback.Outcome == usagehook.OutcomeFailed:
				return errors.Join(err, fmt.Errorf("roll back quota status-line route: %s", rollback.Error))
			}
		}
		// Codex PR #5 tenth review, P2: mirror image of the enable rollback
		// above. RestoreStatusLine may have already removed AgentDeck's route
		// from ~/.claude/settings.json before this persistence failure, but
		// core state still carries the prior (pre-call) StatusLineConsent,
		// which for a disable call means "true" -- so the next load would
		// present capture as enabled while no route is actually installed.
		// Reinstall it so the file agrees with what is (still) persisted.
		if operation == "disable" && result.Outcome != usagehook.OutcomeFailed && !errors.Is(err, store.ErrSettingsSecureFilesFailed) {
			reinstall, reinstallErr := manager.SetupStatusLine()
			switch {
			case reinstallErr != nil:
				return errors.Join(err, fmt.Errorf("restore quota status-line route: %w", reinstallErr))
			case reinstall.Outcome == usagehook.OutcomeFailed:
				return errors.Join(err, fmt.Errorf("restore quota status-line route: %s", reinstall.Error))
			}
		}
		return err
	}
	if err := writeResult(opts.stdout, opts.format, "desktop.quota-statusline."+operation, desktopQuotaStatusLineResult{Consent: settings.StatusLineConsent, Result: result}); err != nil {
		return err
	}
	if result.Outcome == usagehook.OutcomeFailed {
		return fmt.Errorf("quota status-line %s failed: %s", operation, result.Error)
	}
	return nil
}
