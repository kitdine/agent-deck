import AgentDeckShared
import Foundation
import Observation

/// The subscription-quota settings group's three-choice alert threshold
/// control (ux/settings-quota.md: "Three choices — 75 %, 90 %, both").
/// Modeled as a case set rather than a raw `[Double]` so the segmented
/// control below binds to exactly the three combinations `ParseAlertThresholds`
/// accepts, with no state the CLI would reject.
enum QuotaAlertThresholdChoice: CaseIterable, Identifiable, Sendable {
	case seventyFive
	case ninety
	case both

	var id: Self { self }

	var values: [Double] {
		switch self {
		case .seventyFive: [75]
		case .ninety: [90]
		case .both: [75, 90]
		}
	}

	/// `.both` is the fallback for a stored value this UI's own three choices
	/// cannot represent, which is unreachable through this app (the CLI's own
	/// parser accepts only these three combinations) but keeps a value read
	/// from core state total rather than partial.
	init(values: [Double]) {
		let sorted = Set(values)
		if sorted == [75] {
			self = .seventyFive
		} else if sorted == [90] {
			self = .ninety
		} else {
			self = .both
		}
	}
}

/// The Claude settings-window preview of what `enable` would chain
/// (ux/settings-quota.md: "The chained command is read from the user's
/// current configuration and shown before they consent"). Read-only, and
/// never the source AgentDeck itself writes from — that stays entirely on
/// the Go side (task 3's `usagehook` package), which alone knows the prior
/// value across a write. A malformed or absent file reads as "no command
/// configured" rather than an error: the field this feeds only ever renders
/// a command or its absence, never a parse failure.
private struct ClaudeStatusLinePreview: Decodable {
	struct StatusLine: Decodable {
		let command: String?
	}

	let statusLine: StatusLine?

	enum CodingKeys: String, CodingKey {
		case statusLine
	}
}

func readChainedStatusLineCommand(claudeSettingsURL: URL) -> String? {
	guard let data = try? Data(contentsOf: claudeSettingsURL),
		let decoded = try? JSONDecoder().decode(ClaudeStatusLinePreview.self, from: data),
		let command = decoded.statusLine?.command,
		!command.isEmpty
	else {
		return nil
	}
	return command
}

/// Drives the subscription-quota settings group (ux/settings-quota.md).
/// Reading and interval are also mirrored into `DesktopPreferences` so the
/// app's own periodic-refresh scheduler — a synchronous, no-I/O check per
/// tick — never waits on this controller's async round trip to core state;
/// this controller's own `settings` is what the window renders, and it is
/// always what core state, not the local mirror, last confirmed.
///
/// Alerts, thresholds, reset notice, and status-line consent have no local
/// mirror: core state (`internal/quota/settings.go`, task 6's operator
/// decision 1) is their only home, read and written entirely through
/// `desktop quota-settings` / `desktop quota-statusline`.
@MainActor
@Observable
final class QuotaSettingsController {
	private let preferences: DesktopPreferences
	private let transport: any QuotaSettingsTransport
	private let claudeSettingsURL: URL
	private let notifications: any NotificationPermissionChecking

	private(set) var settings: DesktopQuotaSettingsValuesV1?
	private(set) var chainedStatusLineCommand: String?
	/// Set only by a write this controller made, and cleared by the next
	/// successful one — a transient failure explains the write that
	/// triggered it, not every future glance at the window.
	private(set) var settingsRow: SettingsRowStatus?
	private(set) var statuslineRow: SettingsRowStatus?
	/// True while alerts are on but AgentDeck may not post notifications
	/// (ux/settings-quota.md: "When notifications are not allowed"). The alerts
	/// setting itself stays on.
	private(set) var notificationsDenied = false
	@ObservationIgnored private var isApplyingSettings = false
	@ObservationIgnored private var desiredSettings: DesktopQuotaSettingsDesiredV1?
	@ObservationIgnored private var pendingSettings: DesktopQuotaSettingsDesiredV1?
	@ObservationIgnored private var isApplyingStatusline = false

	init(
		preferences: DesktopPreferences,
		transport: any QuotaSettingsTransport,
		claudeSettingsURL: URL,
		notifications: any NotificationPermissionChecking
	) {
		self.preferences = preferences
		self.transport = transport
		self.claudeSettingsURL = claudeSettingsURL
		self.notifications = notifications
	}

	/// The alerts field's warning row. Empty with alerts off, whatever the
	/// permission is.
	var alertsRow: SettingsRowStatus? {
		guard notificationsDenied, settings?.alerts == true else { return nil }
		return SettingsRowStatus(text: t(DesktopCopy.settingsQuotaAlertsNotificationsDenied), severity: .warning)
	}

	/// Reads core state once (the window's `onAppear`); a preference change
	/// afterward always goes through `applySettings`/`applyStatusline`, whose
	/// own responses are the next source of truth, so this never needs to
	/// re-poll on a timer.
	func load() async {
		chainedStatusLineCommand = readChainedStatusLineCommand(claudeSettingsURL: claudeSettingsURL)
		guard case let .decoded(result) = await transport.loadQuotaSettings() else {
			return
		}
		adopt(result.settings)
		await refreshNotificationPermission()
	}

	/// Re-reads the permission without prompting — on load and whenever the
	/// window becomes active again, so returning from System Settings clears
	/// the row without a restart.
	func refreshNotificationPermission() async {
		guard settings?.alerts == true else {
			notificationsDenied = false
			return
		}
		notificationsDenied = await !notifications.authorizationGranted()
	}

	func setReading(_ on: Bool) async {
		preferences.quotaProbeEnabled = on
		var desired = currentDesired()
		desired.reading = on
		stage(desired)
		await applySettings(desired)
	}

	func setInterval(_ interval: QuotaProbeInterval) async {
		preferences.quotaProbeInterval = interval
		var desired = currentDesired()
		desired.interval = DesktopQuotaIntervalV1(interval)
		stage(desired)
		await applySettings(desired)
	}

	/// Turning alerts on is when notification permission is requested; a
	/// refusal keeps the setting on and shows the warning row instead.
	func setAlerts(_ on: Bool) async {
		var desired = currentDesired()
		desired.alerts = on
		stage(desired)
		await applySettings(desired)
		if on {
			notificationsDenied = await !notifications.requestAuthorization()
		} else {
			notificationsDenied = false
		}
	}

	func setThresholds(_ choice: QuotaAlertThresholdChoice) async {
		var desired = currentDesired()
		desired.thresholds = choice.values
		stage(desired)
		await applySettings(desired)
	}

	func setResetNotice(_ on: Bool) async {
		var desired = currentDesired()
		desired.resetNotice = on
		stage(desired)
		await applySettings(desired)
	}

	func setStatuslineConsent(_ on: Bool) async {
		guard !isApplyingStatusline else { return }
		isApplyingStatusline = true
		defer { isApplyingStatusline = false }
		guard case let .decoded(result) = await transport.setQuotaStatusLine(enabled: on) else {
			statuslineRow = SettingsRowStatus(text: t(DesktopCopy.settingsQuotaWriteFailed), severity: .error)
			return
		}
		applyStatuslineResult(consent: result.consent, outcome: result.result)
	}

	/// Builds the write payload. `reading`/`interval` always come from the
	/// local preference mirror — `setReading`/`setInterval` update it
	/// synchronously before this is ever called, so it is never stale the way
	/// re-reading `settings` (last confirmed by the *previous* round trip)
	/// would be. `alerts`/`thresholds`/`resetNotice` have no local mirror
	/// (core state is their only home), so they come from `settings` and
	/// default off/empty — matching `requirements.md`'s default-off contract —
	/// only when core state has not been read yet at all.
	private func currentDesired() -> DesktopQuotaSettingsDesiredV1 {
		if let desiredSettings { return desiredSettings }
		return DesktopQuotaSettingsDesiredV1(
			reading: preferences.quotaProbeEnabled,
			interval: DesktopQuotaIntervalV1(preferences.quotaProbeInterval),
			alerts: settings?.alerts ?? false,
			thresholds: settings?.thresholds ?? [],
			resetNotice: settings?.resetNotice ?? false
		)
	}

	private func applySettings() async {
		await applySettings(currentDesired())
	}

	private func applySettings(_ desired: DesktopQuotaSettingsDesiredV1) async {
		pendingSettings = desired
		guard !isApplyingSettings else { return }
		isApplyingSettings = true
		defer { isApplyingSettings = false }
		while let request = pendingSettings {
			pendingSettings = nil
			guard case let .decoded(result) = await transport.applyQuotaSettings(request) else {
				settingsRow = SettingsRowStatus(text: t(DesktopCopy.settingsQuotaWriteFailed), severity: .error)
				continue
			}
			settingsRow = nil
			if let restore = result.statuslineRestore {
				applyStatuslineResult(consent: result.settings.statusline, outcome: restore)
			} else {
				statuslineRow = nil
			}
			// A newer user intent queued while this request was in flight owns
			// presentation. Only the last response is allowed to replace it.
			if pendingSettings == nil {
				adopt(result.settings)
			}
		}
	}

	private func stage(_ desired: DesktopQuotaSettingsDesiredV1) {
		desiredSettings = desired
		settings = DesktopQuotaSettingsValuesV1(
			reading: desired.reading, interval: desired.interval, alerts: desired.alerts,
			thresholds: desired.thresholds, resetNotice: desired.resetNotice,
			statusline: settings?.statusline ?? false
		)
	}

	private func applyStatuslineResult(consent: Bool, outcome: DesktopUsageHookResultV1) {
		switch outcome.outcome {
		case .configured, .unchanged, .removed, .absent, .skipped:
			statuslineRow = nil
		case .restoreIncomplete:
			statuslineRow = SettingsRowStatus(text: t(DesktopCopy.settingsQuotaStatuslineRestoreIncomplete), severity: .warning)
		case .failed, .unknown:
			statuslineRow = SettingsRowStatus(text: t(DesktopCopy.settingsQuotaStatuslineWriteRefused), severity: .error)
		}
		if let settings {
			self.settings = DesktopQuotaSettingsValuesV1(
				reading: settings.reading, interval: settings.interval, alerts: settings.alerts,
				thresholds: settings.thresholds, resetNotice: settings.resetNotice, statusline: consent
			)
		}
	}

	/// The single point that keeps every read of core state — the initial
	/// load and every write's own response — in the same shape, and the
	/// local preference mirror in step with what core state just confirmed
	/// (not merely with what this controller asked for).
	private func adopt(_ values: DesktopQuotaSettingsValuesV1) {
		settings = values
		desiredSettings = DesktopQuotaSettingsDesiredV1(
			reading: values.reading, interval: values.interval, alerts: values.alerts,
			thresholds: values.thresholds, resetNotice: values.resetNotice
		)
		preferences.quotaProbeEnabled = values.reading
		preferences.quotaProbeInterval = QuotaProbeInterval(values.interval)
	}
}

extension DesktopQuotaIntervalV1 {
	init(_ interval: QuotaProbeInterval) {
		switch interval {
		case .fiveMinutes: self = .fiveMinutes
		case .fifteenMinutes: self = .fifteenMinutes
		case .thirtyMinutes: self = .thirtyMinutes
		}
	}
}

extension QuotaProbeInterval {
	init(_ interval: DesktopQuotaIntervalV1) {
		switch interval {
		case .fiveMinutes: self = .fiveMinutes
		case .fifteenMinutes: self = .fifteenMinutes
		case .thirtyMinutes: self = .thirtyMinutes
		}
	}
}
