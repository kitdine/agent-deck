import Foundation
import Observation
import ServiceManagement

enum MenuBarValueMode: String, CaseIterable, Identifiable, Sendable {
	case cost
	case tokens
	case icon

	var id: String { rawValue }

	var label: String {
		switch self {
		case .cost: t(DesktopCopy.settingsMenuBarValueCost)
		case .tokens: t(DesktopCopy.settingsMenuBarValueTokens)
		case .icon: t(DesktopCopy.settingsMenuBarValueIcon)
		}
	}
}

/// C9's configured background probe cadence. Fixed to three values rather
/// than a free interval so every stored value is always one this type can
/// represent — an unrecognized persisted value falls back to `.fiveMinutes`
/// (the requirements.md default) rather than a partially-valid custom
/// duration.
enum QuotaProbeInterval: Int, CaseIterable, Identifiable, Sendable {
	case fiveMinutes = 5
	case fifteenMinutes = 15
	case thirtyMinutes = 30

	var id: Int { rawValue }

	var minutes: Int { rawValue }
}

enum MenuBarScopeMode: String, CaseIterable, Identifiable, Sendable {
	case allClients
	case followPanel

	var id: String { rawValue }

	var label: String {
		switch self {
		case .allClients: t(DesktopCopy.settingsMenuBarScopeAll)
		case .followPanel: t(DesktopCopy.settingsMenuBarScopeFollow)
		}
	}
}

/// What `SMAppService` currently reports, not what was requested. The settings
/// window renders this value; it never renders the toggle's own intent.
enum LoginItemState: Equatable, Sendable {
	case disabled
	case enabled
	case requiresApproval
	case refused

	var isOn: Bool {
		self == .enabled || self == .requiresApproval
	}
}

/// Reads and writes `SMAppService.mainApp`. Injected so tests exercise refusal
/// and approval without touching the real login-item database.
protocol LoginItemRegistering: AnyObject {
	func register() throws
	func unregister() throws
	var status: SMAppService.Status { get }
}

final class SystemLoginItemRegistrar: LoginItemRegistering {
	func register() throws { try SMAppService.mainApp.register() }
	func unregister() throws { try SMAppService.mainApp.unregister() }
	var status: SMAppService.Status { SMAppService.mainApp.status }
}

@MainActor
@Observable
final class DesktopPreferences {
	private enum Key {
		static let periodicRefresh = "desktop.periodicRefresh"
		static let menuBarValue = "desktop.menuBarValue"
		static let menuBarScope = "desktop.menuBarScope"
		static let quotaProbeEnabled = "quota.probeEnabled"
		static let quotaProbeInterval = "quota.probeIntervalMinutes"
		static let quotaAlertsEnabled = "quota.alertsEnabled"
	}

	private let defaults: UserDefaults
	@ObservationIgnored private let registrar: any LoginItemRegistering

	/// Off by default: it is background work the user did not ask for.
	var periodicRefreshEnabled: Bool {
		didSet { defaults.set(periodicRefreshEnabled, forKey: Key.periodicRefresh) }
	}

	var menuBarValue: MenuBarValueMode {
		didSet { defaults.set(menuBarValue.rawValue, forKey: Key.menuBarValue) }
	}

	var menuBarScope: MenuBarScopeMode {
		didSet { defaults.set(menuBarScope.rawValue, forKey: Key.menuBarScope) }
	}

	/// Off by default (requirements.md clause 1): subscription-quota's outer
	/// gate (architecture.md C1) — with this off, no client is ever probed
	/// regardless of provider.
	///
	/// architecture.md C9 additionally requires that turning this off
	/// synchronously unregisters an installed status-line route and clears
	/// its consent flag, as part of the same user action. That transition —
	/// and the settings-quota consent flag it touches — needs a CLI surface
	/// this preference alone cannot reach; per the gate-and-schedule task's
	/// own recorded scope note in tasks.md, it is deferred to task 6
	/// (wire-and-cli), which adds that surface. This property is
	/// control-path storage only, per this task's boundary with task 7
	/// ("task 4 owns control-path behavior and task 7 owns presentation").
	var quotaProbeEnabled: Bool {
		didSet { defaults.set(quotaProbeEnabled, forKey: Key.quotaProbeEnabled) }
	}

	/// The background probe cadence (architecture.md C9). Defaults to the
	/// shortest option, matching requirements.md's stated default of 5m.
	var quotaProbeInterval: QuotaProbeInterval {
		didSet { defaults.set(quotaProbeInterval.rawValue, forKey: Key.quotaProbeInterval) }
	}

	/// Mirrors `quota.Settings.AlertsEnabled` for the same reason `reading`/
	/// `interval` are mirrored above: the alert-capable background schedule
	/// (Codex PR #5 tenth review, P1) must be able to gate itself
	/// synchronously at launch, before `QuotaSettingsController.load()`'s
	/// async round trip to core state has ever resolved.
	var quotaAlertsEnabled: Bool {
		didSet { defaults.set(quotaAlertsEnabled, forKey: Key.quotaAlertsEnabled) }
	}

	private(set) var loginItem: LoginItemState

	init(defaults: UserDefaults = .standard, registrar: any LoginItemRegistering = SystemLoginItemRegistrar()) {
		self.defaults = defaults
		self.registrar = registrar
		periodicRefreshEnabled = defaults.bool(forKey: Key.periodicRefresh)
		quotaProbeEnabled = defaults.bool(forKey: Key.quotaProbeEnabled)
		quotaProbeInterval = QuotaProbeInterval(rawValue: defaults.integer(forKey: Key.quotaProbeInterval)) ?? .fiveMinutes
		quotaAlertsEnabled = defaults.bool(forKey: Key.quotaAlertsEnabled)
		menuBarValue = MenuBarValueMode(rawValue: defaults.string(forKey: Key.menuBarValue) ?? "") ?? .cost
		menuBarScope = MenuBarScopeMode(rawValue: defaults.string(forKey: Key.menuBarScope) ?? "") ?? .allClients
		loginItem = DesktopPreferences.state(from: registrar.status)
	}

	/// Idempotent in both directions, and the result is read back from the
	/// service. A refusal leaves the switch at the real status and states why.
	func setLoginItem(enabled: Bool) {
		do {
			if enabled {
				try registrar.register()
			} else {
				try registrar.unregister()
			}
			loginItem = DesktopPreferences.state(from: registrar.status)
		} catch {
			let observed = DesktopPreferences.state(from: registrar.status)
			loginItem = observed == .enabled || observed == .requiresApproval ? observed : .refused
		}
	}

	private static func state(from status: SMAppService.Status) -> LoginItemState {
		switch status {
		case .enabled: .enabled
		case .requiresApproval: .requiresApproval
		default: .disabled
		}
	}
}
