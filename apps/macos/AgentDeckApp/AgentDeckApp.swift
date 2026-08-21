import AgentDeckShared
import AppKit
import SwiftUI

@main
enum AgentDeckMain {
	static func main() {
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
	private let settingsController: SettingsWindowController
	private var itemController: MenuBarItemController?
	private var periodicRefresh: Task<Void, Never>?
	private var acceptanceWindow: NSWindow?

	override init() {
		var runner = EmbeddedHelperRunner()
		var snapshotStore = AppGroupSnapshotStore()
		var defaults: UserDefaults = .standard
		#if DEBUG
		// The acceptance harness runs the app against an isolated home so the
		// manual checklist never reads real AgentDeck or client state.
		if let rawTestHome = ProcessInfo.processInfo.environment["AGENTDECK_TEST_HOME"] {
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
		}
		#endif
		let preferences = DesktopPreferences(defaults: defaults)
		let coordinator = DesktopRefreshCoordinator(host: DesktopHost(runner: runner), snapshotStore: snapshotStore)
		let switchController = SwitchController(transport: runner, refreshCoordinator: coordinator)
		self.preferences = preferences
		refreshCoordinator = coordinator
		self.switchController = switchController
		model = MenuBarViewModel(
			coordinator: coordinator,
			switchController: switchController,
			preferences: preferences
		)
		settingsController = SettingsWindowController(preferences: preferences)
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
				await self.refreshCoordinator.refresh()
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
	}
	#endif
}
