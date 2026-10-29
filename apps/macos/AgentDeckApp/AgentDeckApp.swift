import AgentDeckShared
import AppKit
import SwiftUI
import WidgetKit

#if DEBUG || AGENTDECK_PERF
enum DebugTestIsolationError: Error, Equatable {
	case unsafeHome
}

func debugTestHome(environment: [String: String]) throws -> URL? {
	if let rawTestHome = environment["AGENTDECK_TEST_HOME"] {
		let testHome = URL(fileURLWithPath: rawTestHome, isDirectory: true).standardizedFileURL
		let acceptedPrefixes = [
			"/tmp/agentdeck-menubar-acceptance.",
			"/private/tmp/agentdeck-menubar-acceptance.",
			"/tmp/agentdeck-macos-xctest.",
			"/private/tmp/agentdeck-macos-xctest.",
			"/tmp/agentdeck-perf.",
			"/private/tmp/agentdeck-perf.",
		]
		guard acceptedPrefixes.contains(where: testHome.path.hasPrefix) else {
			throw DebugTestIsolationError.unsafeHome
		}
		return testHome
	}
	return nil
}

func debugAutomaticRefreshEnabled(environment: [String: String]) -> Bool {
	environment["XCTestConfigurationFilePath"] == nil
}

/// What `AgentDeckApplicationDelegate.init()` builds its real-HOME-touching
/// objects (the embedded helper runner, its snapshot store/defaults, and the
/// `~/.claude/settings.json` URL `QuotaSettingsController.load()` reads from
/// Settings' `onAppear`) from, resolved once as a pure function so a hosted
/// test's fail-closed behavior is directly testable without constructing the
/// real delegate. `.real` is reachable only when `XCTestConfigurationFilePath`
/// is absent (docs/fixes/xctest-state-isolation.md); a hosted run that never
/// resolves an isolated home must not silently fall through to it merely
/// because automatic refresh is separately disabled.
enum DebugHomeResolution: Equatable {
	case real
	case isolated(URL)
	case unsafeHome
	case missingForHostedTest
}

func resolveDebugHome(environment: [String: String]) -> DebugHomeResolution {
	do {
		if let testHome = try debugTestHome(environment: environment) {
			return .isolated(testHome)
		}
	} catch {
		return .unsafeHome
	}
	return debugAutomaticRefreshEnabled(environment: environment) ? .real : .missingForHostedTest
}
#endif

@main
enum AgentDeckMain {
	static let widgetReloadArgument = "--reload-widget-timelines"
	static let widgetKinds = [
		"com.kitdine.agentdeck.widget.magnitude",
		"com.kitdine.agentdeck.widget.composition",
		"com.kitdine.agentdeck.widget.trust",
		"com.kitdine.agentdeck.widget.rhythm",
		"com.kitdine.agentdeck.widget.quota",
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
	private let periodicDriver: DesktopRefreshPeriodicDriver
	private let automaticRefreshEnabled: Bool
	private var itemController: MenuBarItemController?
	private var quotaSchedule: Task<Void, Never>?
	private var wakeObserver: NSObjectProtocol?
	private var acceptanceWindow: NSWindow?
	private let sourceHome: URL
	private var sourceMonitor: DesktopSourceChangeMonitor?
	private var automaticUpdates: DesktopAutomaticUpdates?
	#if AGENTDECK_PERF
	private var perfDriver: PerfDriver?
	#endif

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
		var automaticRefreshEnabled = true
		var sourceHome = FileManager.default.homeDirectoryForCurrentUser
		#if DEBUG || AGENTDECK_PERF
		// AgentDeckAppTests is a hosted XCTest target: starting the test bundle
		// starts this application delegate, including its initial helper refresh.
		// resolveDebugHome is the single fail-closed decision for every
		// real-HOME-touching object built below (docs/fixes/xctest-state-isolation.md
		// covers automatic refresh; QuotaSettingsController.load(), reachable
		// from Settings' onAppear independent of automatic refresh, needed the
		// same guard). A hosted test that never resolves an isolated home --
		// a command that forgot to pass TEST_RUNNER_AGENTDECK_TEST_HOME (Xcode
		// strips TEST_RUNNER_ for the launched test host), or supplied an
		// unsafe prefix -- must fail before the delegate can read or reason
		// about real state.
		automaticRefreshEnabled = debugAutomaticRefreshEnabled(environment: ProcessInfo.processInfo.environment)
		switch resolveDebugHome(environment: ProcessInfo.processInfo.environment) {
		case .real:
			#if AGENTDECK_PERF
			// A measurement build never reads or writes real AgentDeck state.
			preconditionFailure("Perf builds require an isolated AGENTDECK_TEST_HOME")
			#else
			break
			#endif
		case .isolated(let testHome):
			sourceHome = testHome
			var isolatedEnvironment = [
				"HOME": testHome.path, "LANG": "en_US_POSIX", "LC_ALL": "en_US_POSIX", "PATH": "/usr/bin:/bin",
			]
			if PerfTrace.isEnabled {
				// App and helpers run in the batch's timezone (anchor date).
				isolatedEnvironment["TZ"] = ProcessInfo.processInfo.environment["TZ"] ?? "America/New_York"
				isolatedEnvironment["TMPDIR"] = ProcessInfo.processInfo.environment["TMPDIR"] ?? NSTemporaryDirectory()
			}
			runner = EmbeddedHelperRunner(
				appBundleURL: Bundle.main.bundleURL,
				environment: isolatedEnvironment
			)
			snapshotStore = AppGroupSnapshotStore(
				directoryURL: testHome.appendingPathComponent("app-group", isDirectory: true)
			)
			defaults = UserDefaults(suiteName: "com.kitdine.agentdeck.acceptance") ?? .standard
			defaults.setVolatileDomain([:], forName: "com.kitdine.agentdeck.acceptance")
			claudeSettingsURL = testHome.appendingPathComponent(".claude", isDirectory: true)
				.appendingPathComponent("settings.json", isDirectory: false)
		case .missingForHostedTest:
			preconditionFailure("Hosted AgentDeck tests require an isolated AGENTDECK_TEST_HOME")
		case .unsafeHome:
			preconditionFailure("AgentDeck test harness requires a safe temporary home")
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
		let periodicDriver = DesktopRefreshPeriodicDriver { [weak coordinator] in
			await coordinator?.requestQuotaRefresh(manual: false)
		}
		let switchController = SwitchController(transport: runner, refreshCoordinator: coordinator)
		let quotaSettings = QuotaSettingsController(
			preferences: preferences,
			transport: runner,
			claudeSettingsURL: claudeSettingsURL,
			notifications: notifications,
			refreshQuotaSnapshot: { [weak coordinator] in
				await coordinator?.refresh(manualQuota: false)
			}
		)
		self.preferences = preferences
		self.periodicDriver = periodicDriver
		self.automaticRefreshEnabled = automaticRefreshEnabled
		self.sourceHome = sourceHome
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
		if PerfTrace.isEnabled {
			PerfTrace.mark(.processStart, at: PerfTrace.processStartUptime())
			PerfTrace.mark(.appReady)
		}
		#if !DEBUG && !AGENTDECK_PERF
		// Homebrew's install-step sandbox cannot contact PlugInKit. Register only
		// this signed host's own nested extension, after normal App launch.
		let extensionURL = Bundle.main.bundleURL.appendingPathComponent("Contents/PlugIns/AgentDeckWidget.appex")
		if FileManager.default.fileExists(atPath: extensionURL.path) {
			Task.detached(priority: .utility) {
				let process = Process()
				process.executableURL = URL(fileURLWithPath: "/usr/bin/pluginkit")
				process.arguments = ["-a", extensionURL.path]
				do {
					try process.run()
					DispatchQueue.global().asyncAfter(deadline: .now() + 5) {
						if process.isRunning { process.terminate() }
					}
					process.waitUntilExit()
					guard process.terminationStatus == 0 else {
						NSLog("AgentDeck Widget registration failed: status %d", process.terminationStatus)
						return
					}
					WidgetTimelineReloader.live.reload(kinds: Set(AppGroupWidgetKind.allCases))
				} catch {
					NSLog("AgentDeck Widget registration failed: %@", String(describing: error))
				}
			}
		}
		#endif
		installMainMenu()
		itemController = MenuBarItemController(model: model) { [weak self] in
			self?.settingsController.show()
		}
		itemController?.warmsWindowAfterPreparation = true
		if automaticRefreshEnabled {
			let monitor = DesktopSourceChangeMonitor(home: sourceHome) { [weak refreshCoordinator] in
				Task { @MainActor in await refreshCoordinator?.requestFullRefresh(trigger: .sourceChange) }
			}
			let updates = DesktopAutomaticUpdates(
				enabled: preferences.automaticUpdateEnabled,
				startWatching: { [weak monitor] in monitor?.start() ?? false },
				stopWatching: { [weak monitor] in monitor?.stop() },
				request: { [weak refreshCoordinator] trigger in
					Task { @MainActor in await refreshCoordinator?.requestFullRefresh(trigger: trigger) }
				}
			)
			sourceMonitor = monitor
			automaticUpdates = updates
			preferences.automaticUpdateDidChange = { [weak updates] enabled in
				updates?.setEnabled(enabled)
			}
			refreshCoordinator.startInitialRefresh()
			// Restore first: watching and quota work start after it, like the
			// startup refresh (architecture, startup order).
			Task { @MainActor [weak self] in
				await self?.refreshCoordinator.waitForStartupSettled()
				guard let self else { return }
				self.automaticUpdates?.start()
				self.startBackgroundSchedules()
			}
		}
		#if DEBUG
		presentAcceptanceWindowIfRequested()
		#endif
		#if AGENTDECK_PERF
		if let itemController {
			perfDriver = PerfDriver(model: model, itemController: itemController)
			perfDriver?.start()
		}
		#endif
	}

		func applicationWillTerminate(_ notification: Notification) {
			preferences.automaticUpdateDidChange = nil
			automaticUpdates?.stop()
			refreshCoordinator.stopAppWork()
			PerfTrace.flush()
		quotaSchedule?.cancel()
		periodicDriver.cancel()
		if let wakeObserver {
			NSWorkspace.shared.notificationCenter.removeObserver(wakeObserver)
		}
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

	/// Wake asks automatic updates for one finite reconciliation (a no-op
	/// while they are off); data never refreshes on a period.
	///
	/// Codex PR #5 tenth review, P1: quota alert evaluation must not depend
	/// on the local-data update preference -- a user can turn on quota reading
	/// and alerts while leaving automatic updates off, and still expects
	/// threshold/reset notifications. The quota tick runs on its own schedule,
	/// gated on the mirrored reading preference so it does not wait on
	/// `QuotaSettingsController.load()`'s async round trip either.
	private func startBackgroundSchedules() {
		wakeObserver = NSWorkspace.shared.notificationCenter.addObserver(
			forName: NSWorkspace.didWakeNotification,
			object: nil,
			queue: .main
		) { [weak self] _ in
			Task { @MainActor in self?.automaticUpdates?.didWake() }
		}
		quotaSchedule = Task { [weak self] in
			while !Task.isCancelled {
				try? await Task.sleep(for: DesktopRefreshPeriodicDriver.evaluatorInterval)
				guard let self else { continue }
				// Codex PR #5 twelfth review, P1: this tick is this app's only
				// recurring caller into the quota probe itself, not merely alert
				// evaluation -- gating it on alerts too meant a user with reading
				// on but alerts off (both defaults: alerts starts off) never got a
				// single background probe after startup. The helper already no-ops
				// alert delivery on its own when alerts are disabled; gate this
				// call on reading alone.
				await self.periodicDriver.tick(quotaEnabled: self.preferences.quotaProbeEnabled)
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
