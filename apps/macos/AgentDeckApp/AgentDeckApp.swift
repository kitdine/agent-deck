import AgentDeckShared
import AppKit
import SwiftUI
import WidgetKit

@main
enum AgentDeckMain {
	static let widgetReloadArgument = "--reload-widget-timelines"
	static let widgetKinds = [
		"com.kitdine.agentdeck.widget.magnitude",
		"com.kitdine.agentdeck.widget.composition",
		"com.kitdine.agentdeck.widget.trust",
		"com.kitdine.agentdeck.widget.rhythm",
	]

	static func main() {
		if Array(CommandLine.arguments.dropFirst()) == [widgetReloadArgument] {
			for kind in widgetKinds {
				WidgetCenter.shared.reloadTimelines(ofKind: kind)
			}
			RunLoop.current.run(until: Date().addingTimeInterval(1))
			return
		}
		let application = NSApplication.shared
		let delegate = AgentDeckApplicationDelegate()
		application.delegate = delegate
		application.setActivationPolicy(.accessory)
		application.run()
	}
}

@MainActor
final class AgentDeckApplicationDelegate: NSObject, NSApplicationDelegate {
	private let preferences: DesktopPreferences
	private let refreshCoordinator: DesktopRefreshCoordinator
	private let switchController: SwitchController
	private let model: MenuBarViewModel
	private let quotaSettings: QuotaSettingsController
	private let settingsController: SettingsWindowController
	private var itemController: MenuBarItemController?
	private var periodicRefresh: Task<Void, Never>?
	private var acceptanceWindow: NSWindow?

	override init() {
		var runner = EmbeddedHelperRunner()
		var snapshotStore = AppGroupSnapshotStore()
		var defaults: UserDefaults = .standard
		// ~/.claude/settings.json's real location; overridden below to the
		// same isolated home the acceptance harness already points the
		// embedded helper's own subprocess environment at, so the
		// status-line consent preview (a direct, read-only local file read —
		// see QuotaSettingsController) never reads or reasons about a real
		// user's file just because the harness is running.
		var claudeSettingsURL = FileManager.default.homeDirectoryForCurrentUser
			.appendingPathComponent(".claude", isDirectory: true)
			.appendingPathComponent("settings.json", isDirectory: false)
		#if DEBUG
		let processEnvironment = ProcessInfo.processInfo.environment
		// AgentDeckAppTests is a hosted XCTest target: starting the test bundle
		// starts this application delegate, including its initial helper refresh.
		// Refuse to construct a production-home runner when a test command forgot
		// to pass TEST_RUNNER_AGENTDECK_TEST_HOME (Xcode strips TEST_RUNNER_ for
		// the launched test host). A mistaken test must fail before it can open or
		// migrate the operator's real database.
		if processEnvironment["XCTestConfigurationFilePath"] != nil,
			processEnvironment["AGENTDECK_TEST_HOME"] == nil
		{
			preconditionFailure("Hosted AgentDeck tests require an isolated AGENTDECK_TEST_HOME")
		}
		// The acceptance harness runs the app against an isolated home so the
		// manual checklist never reads real AgentDeck or client state.
		if let rawTestHome = processEnvironment["AGENTDECK_TEST_HOME"] {
			let testHome = URL(fileURLWithPath: rawTestHome, isDirectory: true).standardizedFileURL
			let accepted = testHome.path.hasPrefix("/tmp/agentdeck-menubar-acceptance.")
				|| testHome.path.hasPrefix("/private/tmp/agentdeck-menubar-acceptance.")
			precondition(accepted, "AGENTDECK_TEST_HOME must be an isolated AgentDeck acceptance directory")
			runner = EmbeddedHelperRunner(
				appBundleURL: Bundle.main.bundleURL,
				environment: [
					"HOME": testHome.path,
					"LANG": "en_US_POSIX",
					"LC_ALL": "en_US_POSIX",
					"PATH": "/usr/bin:/bin",
				]
			)
			snapshotStore = AppGroupSnapshotStore(
				directoryURL: testHome.appendingPathComponent("app-group", isDirectory: true)
			)
			defaults = UserDefaults(suiteName: "com.kitdine.agentdeck.acceptance") ?? .standard
			defaults.setVolatileDomain([:], forName: "com.kitdine.agentdeck.acceptance")
			claudeSettingsURL = testHome.appendingPathComponent(".claude", isDirectory: true)
				.appendingPathComponent("settings.json", isDirectory: false)
		}
		#endif
		let preferences = DesktopPreferences(defaults: defaults)
		let notifications = SystemUserNotifications()
		let coordinator = DesktopRefreshCoordinator(
			host: DesktopHost(runner: runner),
			quotaRefresher: runner,
			alertDeliverer: QuotaAlertNotifier(permission: notifications, poster: notifications),
			snapshotStore: snapshotStore
		)
		let switchController = SwitchController(transport: runner, refreshCoordinator: coordinator)
		let quotaSettings = QuotaSettingsController(
			preferences: preferences,
			transport: runner,
			claudeSettingsURL: claudeSettingsURL,
			notifications: notifications
		)
		self.preferences = preferences
		refreshCoordinator = coordinator
		self.switchController = switchController
		self.quotaSettings = quotaSettings
		model = MenuBarViewModel(
			coordinator: coordinator,
			switchController: switchController,
			preferences: preferences
		)
		settingsController = SettingsWindowController(preferences: preferences, quotaSettings: quotaSettings)
		super.init()
	}

	func applicationDidFinishLaunching(_ notification: Notification) {
		installMainMenu()
		itemController = MenuBarItemController(model: model) { [weak self] in
			self?.settingsController.show()
		}
		refreshCoordinator.startInitialRefresh()
		startPeriodicRefresh()
		#if DEBUG
		presentAcceptanceWindowIfRequested()
		#endif
	}

	/// An accessory application still needs a main menu for `⌘,` and `⌘Q` to
	/// reach anything while the popover is key.
	private func installMainMenu() {
		let mainMenu = NSMenu()
		let appItem = NSMenuItem()
		let appMenu = NSMenu()
		appMenu.addItem(withTitle: t(DesktopCopy.menuAbout), action: #selector(NSApplication.orderFrontStandardAboutPanel(_:)), keyEquivalent: "")
		appMenu.addItem(.separator())
		let settings = NSMenuItem(title: t(DesktopCopy.menuSettings), action: #selector(openSettings), keyEquivalent: ",")
		settings.target = self
		appMenu.addItem(settings)
		appMenu.addItem(.separator())
		appMenu.addItem(withTitle: t(DesktopCopy.menuQuit), action: #selector(NSApplication.terminate(_:)), keyEquivalent: "q")
		appItem.submenu = appMenu
		mainMenu.addItem(appItem)
		NSApp.mainMenu = mainMenu
	}

	@objc private func openSettings() {
		settingsController.show()
	}

	/// Opt-in and off by default. The cadence comes from the snapshot's
	/// `next_refresh_at`; a due time missed while the app was suspended
	/// refreshes once when it comes back rather than replaying every interval.
	private func startPeriodicRefresh() {
		periodicRefresh = Task { [weak self] in
			while !Task.isCancelled {
				try? await Task.sleep(for: .seconds(30))
				guard let self, self.preferences.periodicRefreshEnabled else { continue }
				guard let snapshot = self.refreshCoordinator.latestSnapshot?.data,
					let due = DesktopFormat.timestamp(snapshot.nextRefreshAt)
				else { continue }
				guard due <= Date() else { continue }
				await self.refreshCoordinator.refresh(manualQuota: false)
			}
		}
	}

	#if DEBUG
	private func presentAcceptanceWindowIfRequested() {
		guard ProcessInfo.processInfo.environment["AGENTDECK_TEST_WINDOW"] == "1" else { return }
		let hosting = NSHostingController(rootView: MenuBarSurfaceView(model: model))
		let window = NSWindow(contentViewController: hosting)
		window.title = t(DesktopCopy.appName)
		window.styleMask = [.titled, .closable]
		window.setContentSize(hosting.view.fittingSize)
		window.center()
		acceptanceWindow = window
		NSApp.setActivationPolicy(.regular)
		NSApp.activate(ignoringOtherApps: true)
		window.makeKeyAndOrderFront(nil)
		if ProcessInfo.processInfo.environment["AGENTDECK_TEST_SETTINGS_WINDOW"] == "1" {
			settingsController.show()
		}
	}
	#endif
}
