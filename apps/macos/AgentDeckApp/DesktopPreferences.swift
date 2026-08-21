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

	private(set) var loginItem: LoginItemState

	init(defaults: UserDefaults = .standard, registrar: any LoginItemRegistering = SystemLoginItemRegistrar()) {
		self.defaults = defaults
		self.registrar = registrar
		periodicRefreshEnabled = defaults.bool(forKey: Key.periodicRefresh)
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
