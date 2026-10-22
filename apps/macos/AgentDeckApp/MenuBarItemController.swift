import AgentDeckShared
import AppKit
import Observation
import SwiftUI
#if DEBUG
import Vision
#endif

/// A display link retains its target. Keep preparation independent of the
/// controller's lifetime; this link is never added to a run loop.
private final class PopoverPreparationDisplayLinkTarget: NSObject {
	@objc func tick(_ link: CADisplayLink) {}
}

/// The status item, its popover, and its own menu. `MenuBarExtra` cannot carry
/// a right-click menu, and the contract puts the application's exits there
/// rather than in the popover, so the item is managed directly.
@MainActor
final class MenuBarItemController: NSObject, NSMenuDelegate {
	private let model: MenuBarViewModel
	let statusItem: NSStatusItem
	private let popover = NSPopover()
	private let openSettings: () -> Void
	private var preparedHeight = MenuBarGeometry.maximumHeight
	private var hasPresentedPopover = false
	private var presentationPreparationWindow: NSWindow?
	private var presentationPreparationDisplayLink: CADisplayLink?
	private(set) var isInitialPresentationPrepared = false
	private(set) var dateInvalidationTimer: Timer?
	private let dateNotificationCenter: NotificationCenter
	private let wakeNotificationCenter: NotificationCenter
	private var dateObservers = [NSObjectProtocol]()
	private var wakeObserver: NSObjectProtocol?

	isolated deinit {
		dateInvalidationTimer?.invalidate()
		for observer in dateObservers { dateNotificationCenter.removeObserver(observer) }
		if let wakeObserver { wakeNotificationCenter.removeObserver(wakeObserver) }
		presentationPreparationDisplayLink?.invalidate()
		presentationPreparationWindow?.close()
	}

	init(model: MenuBarViewModel, openSettings: @escaping () -> Void,
		dateNotificationCenter: NotificationCenter = .default,
		wakeNotificationCenter: NotificationCenter = NSWorkspace.shared.notificationCenter) {
		self.model = model
		self.openSettings = openSettings
		self.dateNotificationCenter = dateNotificationCenter
		self.wakeNotificationCenter = wakeNotificationCenter
		statusItem = NSStatusBar.system.statusItem(withLength: NSStatusItem.variableLength)
		super.init()

		popover.behavior = .transient
		popover.animates = false
		popover.contentViewController = NSHostingController(rootView: MenuBarSurfaceView(model: model))

		if let button = statusItem.button {
			button.target = self
			button.action = #selector(handleClick(_:))
			button.sendAction(on: [.leftMouseUp, .rightMouseUp])
			button.imagePosition = .imageLeading
		}
		observeDateChanges()
		render()
		observe()
		prepareInitialPresentationIfReady()
	}

	/// Re-renders on any change the item reads. There is no animation, ever,
	/// including during a refresh.
	private func observe() {
		withObservationTracking {
			_ = model.menuBarText
			_ = model.menuBarBadged
			_ = model.menuBarAccessibilityLabel
			_ = model.surface
			_ = model.coordinator.servingHeader
		} onChange: { [weak self] in
			Task { @MainActor in
				self?.render()
				self?.observe()
				self?.prepareInitialPresentationIfReady()
			}
		}
	}

	private func render() {
		scheduleDateInvalidation()
		guard let button = statusItem.button else { return }
		button.image = MenuBarItemController.glyph(badged: model.menuBarBadged)
		button.image?.isTemplate = true
		button.title = model.menuBarText ?? ""
		button.setAccessibilityLabel(model.menuBarAccessibilityLabel)
	}

	private func observeDateChanges() {
		for name in [Notification.Name.NSCalendarDayChanged, .NSSystemClockDidChange, .NSSystemTimeZoneDidChange] {
			dateObservers.append(dateNotificationCenter.addObserver(forName: name, object: nil, queue: .main) { [weak self] _ in
				Task { @MainActor [weak self] in self?.dateDidChange() }
			})
		}
		wakeObserver = wakeNotificationCenter.addObserver(forName: NSWorkspace.didWakeNotification, object: nil, queue: .main) { [weak self] _ in
			Task { @MainActor [weak self] in self?.dateDidChange() }
		}
	}

	private func scheduleDateInvalidation() {
		dateInvalidationTimer?.invalidate()
		let timer = Timer(timeInterval: model.nextDateInvalidationDelay, repeats: false) { [weak self] _ in
			Task { @MainActor [weak self] in self?.dateDidChange() }
		}
		dateInvalidationTimer = timer
		// Fire while a popover/menu is open too, independently of refresh/quota.
		RunLoop.main.add(timer, forMode: .common)
	}

	private func dateDidChange() {
		model.invalidateDate()
		render()
	}

	/// The base glyph is the monochrome template asset, so macOS owns light,
	/// dark, and highlight appearance. The badge is composited only for the two
	/// states that mean the data cannot be trusted.
	static func glyph(badged: Bool, base suppliedBase: NSImage? = nil) -> NSImage? {
		guard let base = suppliedBase ?? bundledMenuBarIcon() else { return nil }
		let size = NSSize(width: 18, height: 18)
		guard badged else {
			let resized = NSImage(size: size)
			resized.lockFocus()
			base.draw(in: NSRect(origin: .zero, size: size))
			resized.unlockFocus()
			resized.isTemplate = true
			return resized
		}
		let composed = NSImage(size: size)
		composed.lockFocus()
		base.draw(in: NSRect(origin: .zero, size: size))
		if let badge = NSImage(systemSymbolName: "exclamationmark.triangle.fill", accessibilityDescription: nil) {
			NSGraphicsContext.saveGraphicsState()
			NSGraphicsContext.current?.compositingOperation = .clear
			NSBezierPath(ovalIn: NSRect(x: 8, y: -1, width: 11, height: 11)).fill()
			NSGraphicsContext.restoreGraphicsState()
			badge.draw(in: NSRect(x: 9.5, y: 0.5, width: 8, height: 8))
		}
		composed.unlockFocus()
		composed.isTemplate = true
		return composed
	}

	private static func bundledMenuBarIcon() -> NSImage? {
		bundledImage(named: "AgentDeckMenuBarIcon")
	}

	static func aboutPanelOptions() -> [NSApplication.AboutPanelOptionKey: Any] {
		guard let icon = bundledImage(named: "AgentDeckAppIcon") else { return [:] }
		return [.applicationIcon: icon]
	}

	private static func bundledImage(named name: String) -> NSImage? {
		guard let url = Bundle.main.url(forResource: name, withExtension: "png") else {
			return nil
		}
		return NSImage(contentsOf: url)
	}

	@objc private func handleClick(_ sender: NSStatusBarButton) {
		let event = NSApp.currentEvent
		let wantsMenu = event?.type == .rightMouseUp || (event?.clickCount ?? 1) >= 2
		if wantsMenu {
			closePopover()
			presentMenu()
			return
		}
		togglePopover(sender)
	}

	private func togglePopover(_ sender: NSStatusBarButton) {
		if popover.isShown {
			closePopover()
		} else {
			hasPresentedPopover = true
			let openInterval = DesktopPerformanceInterval()
			configurePopoverGeometry(for: sender)
			popover.show(relativeTo: sender.bounds, of: sender, preferredEdge: .minY)
			// The popover has adopted the same hosting view. Its preparation
			// window was never ordered onscreen and no longer owns the content.
			let preparationWindow = presentationPreparationWindow
			presentationPreparationWindow = nil
			if let preparationWindow {
				// Retiring an unshown window is not needed to display the popover.
				// Keep AppKit teardown out of this synchronous input action.
				Task { @MainActor in preparationWindow.close() }
			}
			MenuBarItemController.activatePopoverWindow(popover.contentViewController?.view.window)
			// This ends at show/activation, not at the first usable data frame.
			DesktopLogger.recordPerformance(openInterval.sample(.popoverOpen))
		}
	}

	private func configurePopoverGeometry(for button: NSStatusBarButton) {
		let visibleFrameHeight = button.window?.screen?.visibleFrame.height
			?? MenuBarGeometry.maximumHeight + MenuBarGeometry.screenMargin
		let height = MenuBarGeometry.height(visibleFrameHeight: visibleFrameHeight)
		// Keep the same hosting hierarchy and native control identity. Geometry
		// changes update its root, not the controller or focus ownership.
		if height != preparedHeight,
			let hosting = popover.contentViewController as? NSHostingController<MenuBarSurfaceView> {
			hosting.rootView = MenuBarSurfaceView(model: model, height: height)
			preparedHeight = height
		}
		popover.contentSize = NSSize(width: MenuBarGeometry.width, height: height)
	}

	private func prepareInitialPresentationIfReady() {
		guard !isInitialPresentationPrepared, !hasPresentedPopover, !popover.isShown,
			model.surface == .dataSurface, let button = statusItem.button,
			let hosting = popover.contentViewController else { return }
		let preparation = DesktopPerformanceInterval()
		// AppKit creates a display link during the first visible layout. Resolve
		// its screen resources now, without scheduling callbacks or animation.
		presentationPreparationDisplayLink = (button.window?.screen ?? NSScreen.main)?.displayLink(
			target: PopoverPreparationDisplayLinkTarget(),
			selector: #selector(PopoverPreparationDisplayLinkTarget.tick(_:)))
		configurePopoverGeometry(for: button)
		// Detached layout cannot prepare window-dependent SwiftUI traits or
		// AppKit's first display-link setup. Give the existing view a window
		// environment without ordering it onscreen or requesting focus.
		let preparationWindow = NSWindow(contentRect: NSRect(origin: .zero, size: popover.contentSize),
			styleMask: [.borderless], backing: .buffered, defer: false, screen: button.window?.screen)
		preparationWindow.isReleasedWhenClosed = false
		preparationWindow.contentView?.addSubview(hosting.view)
		presentationPreparationWindow = preparationWindow
		hosting.view.frame = NSRect(origin: .zero, size: popover.contentSize)
		hosting.view.layoutSubtreeIfNeeded()
		hosting.view.displayIfNeeded()
		preparationWindow.displayIfNeeded()
		isInitialPresentationPrepared = true
		// Preparation is real startup/main-actor work, recorded separately from
		// clicks. Never show a window, request focus, or start a helper here.
		DesktopLogger.recordPerformance(preparation.sample(.popoverPreparation))
	}

	static func activatePopoverWindow(_ window: NSWindow?) {
		window?.makeKey()
		window?.makeFirstResponder(nil)
	}

	func closePopover() {
		popover.performClose(nil)
	}

	#if DEBUG
	static let baselineClickStimulus = "App-local posted leftMouseDown/leftMouseUp at status button; activation after initial routing; not global physical input"

	/// Standalone process baseline. The normal delegate/coordinator/helper run;
	/// hosted XCTest cannot stand in for this launch. Bitmaps and OCR stay in memory.
	func captureNativeBaseline(home: URL, processEntryUptime: UInt64) async {
		struct Frame: Codable {
			let panel: String
			let elapsedNanoseconds: UInt64
			let valid: Bool
		}
		struct Result: Codable {
			let version: Int
			let clickStimulus: String
			let launchToDataNanoseconds: UInt64?
			let initialDataPresent: Bool
			let latestDataChanged: Bool
			let frames: [Frame]
			let stages: [DesktopPerformanceSample]
			let droppedStages: Int
			let helperFailures: [DesktopPerformanceFailure]
			let stageAggregates: [DesktopPerformanceAggregate]
			let nativeEnvironment: [String: Bool]
			let initialClickRouted: Bool
			let readyClickRequired: Bool
			let failure: String?
		}
		let initialDataPresent = model.hero != nil
		var frames = [Frame]()
		var launchToData: UInt64?
		var latestDataChanged = false
		var failure: String?
		var refreshFailure: String?
		var initialClickRouted = true
		var readyClickRequired = false
		do {
			guard let button = statusItem.button else { throw BaselineFailure.noStatusItem }
			// Wait for the status window before posting this App's mouse events.
			let activationDeadline = DispatchTime.now().uptimeNanoseconds + 2_000_000_000
			while button.window == nil {
				guard DispatchTime.now().uptimeNanoseconds < activationDeadline else {
					throw BaselineFailure.statusWindowUnavailable
				}
				try await Task.sleep(for: .milliseconds(10))
			}
			// First click uses the status item's normal action, including the empty surface.
			do { try await baselineClick(button) }
			catch BaselineFailure.clickNotRouted { initialClickRouted = false }
			// Accessory applications need a keyable window before activation.
			NSApp.activate(ignoringOtherApps: true)
			MenuBarItemController.activatePopoverWindow(popover.contentViewController?.view.window)
			let deadline = DispatchTime.now().uptimeNanoseconds + 150_000_000_000
			while model.coordinator.latestSnapshot == nil {
				guard DispatchTime.now().uptimeNanoseconds < deadline else { throw BaselineFailure.dataTimeout }
				if case let .degraded(_, issue) = model.coordinator.state {
					if case let .helper(error) = issue { refreshFailure = DesktopLogPolicy.helperFailureEvent(error) }
					else { refreshFailure = "invalid_wire_or_unavailable" }
					throw BaselineFailure.refreshFailed
				}
				try await Task.sleep(for: .milliseconds(10))
			}
			guard let hero = model.hero, let snapshot = model.coordinator.latestSnapshot,
				snapshot.data.usage.available, snapshot.data.sessions.available else { throw BaselineFailure.noData }
			// A ready-state click is a different, explicitly recorded stimulus.
			// Do not relabel a lost loading click as a successful first click.
			if !popover.isShown {
				readyClickRequired = true
				try await baselineClick(button)
			}
			let firstFrame = try baselineFrame()
			guard try baselineFrameIsValid(firstFrame, hero: hero) else { throw BaselineFailure.numericFrameMissing }
			launchToData = firstFrame.end - processEntryUptime
			frames.append(Frame(panel: "quota", elapsedNanoseconds: launchToData!, valid: true))
			for panel in MenuBarPanel.allCases {
				closePopover()
				model.selectedPanel = panel
				let start = DispatchTime.now().uptimeNanoseconds
				try await baselineClick(button)
				let frame = try baselineFrame()
				let valid = try baselineFrameIsValid(frame, hero: hero)
				frames.append(Frame(panel: panel.rawValue, elapsedNanoseconds: frame.end - start, valid: valid))
				guard valid else { throw BaselineFailure.numericFrameMissing }
			}
			// A deterministic append is prepared by the external harness. Only its
			// isolated source is changed; compare old and new frames under one process.
			let delta = home.appendingPathComponent("baseline-delta.jsonl")
			if FileManager.default.fileExists(atPath: delta.path) {
				model.selectedPanel = .quota
				let before = model.coordinator.latestSnapshot
				let input = home.appendingPathComponent(".codex/sessions/baseline-today.jsonl")
				let handle = try FileHandle(forWritingTo: input)
				try handle.seekToEnd()
				try handle.write(contentsOf: Data(contentsOf: delta))
				try handle.close()
				let start = DispatchTime.now().uptimeNanoseconds
				let update = Task { await model.coordinator.refresh(manualQuota: false) }
				await Task.yield()
				let old = try baselineFrame()
				await update.value
				guard let current = model.coordinator.latestSnapshot, let newHero = model.hero else { throw BaselineFailure.noData }
				latestDataChanged = current.data.usage.presentation != before?.data.usage.presentation && newHero.tokens != hero.tokens
				let new = try baselineFrame()
				// Retain both bitmaps/endpoints before any synchronous Vision work.
				// Validation can now neither delay refresh nor enter update_latest.
				let oldValid = try baselineFrameIsValid(old, hero: hero)
				let newValid = try baselineFrameIsValid(new, hero: newHero)
				frames.append(Frame(panel: "update_previous", elapsedNanoseconds: old.end - start, valid: oldValid))
				frames.append(Frame(panel: "update_latest", elapsedNanoseconds: new.end - start, valid: newValid && latestDataChanged))
				guard oldValid, newValid, latestDataChanged else { throw BaselineFailure.updateMissing }
			}
		} catch let error as BaselineFailure {
			failure = refreshFailure ?? error.rawValue
		} catch {
			failure = "isolated_harness_io_or_render_failure"
		}
		let trace = DesktopPerformanceTrace.finish()
		let result = Result(version: 1, clickStimulus: Self.baselineClickStimulus,
			launchToDataNanoseconds: launchToData, initialDataPresent: initialDataPresent,
			latestDataChanged: latestDataChanged, frames: frames, stages: trace.samples, droppedStages: trace.dropped,
				helperFailures: trace.failures, stageAggregates: trace.aggregates, nativeEnvironment: ["hasScreens": !NSScreen.screens.isEmpty,
					"statusButtonHasWindow": statusItem.button?.window != nil, "appIsActive": NSApp.isActive],
				initialClickRouted: initialClickRouted, readyClickRequired: readyClickRequired, failure: failure)
		do {
			let destination = home.appendingPathComponent("baseline-result.json")
			try JSONEncoder().encode(result).write(to: destination, options: [.atomic])
			try FileManager.default.setAttributes([.posixPermissions: 0o600], ofItemAtPath: destination.path)
		} catch {
			// Never include the path, localized exception or helper contents.
			NSLog("AgentDeck isolated baseline result write failed")
		}
		closePopover()
	}

	private enum BaselineFailure: String, Error {
		case noStatusItem = "no_status_item", dataTimeout = "data_timeout", refreshFailed = "refresh_failed"
		case noData = "no_usable_data", numericFrameMissing = "numeric_frame_missing", updateMissing = "update_missing"
		case popoverInvisible = "popover_invisible", bitmapMissing = "bitmap_missing"
		case activationUnavailable = "native_activation_unavailable"
		case statusWindowUnavailable = "native_status_window_unavailable"
		case clickNotRouted = "native_mouse_event_not_routed"
	}

	private func baselineClick(_ button: NSStatusBarButton) async throws {
		guard let window = button.window else { throw BaselineFailure.statusWindowUnavailable }
		let point = button.convert(NSPoint(x: button.bounds.midX, y: button.bounds.midY), to: nil)
		for type in [NSEvent.EventType.leftMouseDown, .leftMouseUp] {
			guard let event = NSEvent.mouseEvent(with: type, location: point, modifierFlags: [],
				timestamp: ProcessInfo.processInfo.systemUptime, windowNumber: window.windowNumber,
				context: nil, eventNumber: 1, clickCount: 1, pressure: 1) else { throw BaselineFailure.clickNotRouted }
			NSApp.postEvent(event, atStart: false)
		}
		let deadline = DispatchTime.now().uptimeNanoseconds + 2_000_000_000
		while !popover.isShown {
			guard DispatchTime.now().uptimeNanoseconds < deadline else { throw BaselineFailure.clickNotRouted }
			try await Task.sleep(for: .milliseconds(10))
		}
	}

	private func baselineFrame() throws -> (end: UInt64, image: CGImage) {
		guard popover.isShown, let view = popover.contentViewController?.view,
			let window = view.window, window.isVisible, window.windowNumber > 0 else { throw BaselineFailure.popoverInvisible }
		view.layoutSubtreeIfNeeded()
		window.displayIfNeeded()
		view.displayIfNeeded()
		guard let bitmap = view.bitmapImageRepForCachingDisplay(in: view.bounds) else { throw BaselineFailure.bitmapMissing }
		view.cacheDisplay(in: view.bounds, to: bitmap)
		let end = DispatchTime.now().uptimeNanoseconds
		guard let image = bitmap.cgImage else { throw BaselineFailure.bitmapMissing }
		return (end, image)
	}

	private func baselineFrameIsValid(_ frame: (end: UInt64, image: CGImage), hero: MenuBarHero) throws -> Bool {
		let request = VNRecognizeTextRequest()
		request.recognitionLevel = .accurate
		request.recognitionLanguages = ["en-US"]
		try VNImageRequestHandler(cgImage: frame.image).perform([request])
		let recognized = (request.results ?? []).compactMap { $0.topCandidates(1).first?.string }.joined(separator: " ")
		func normalized(_ text: String) -> String { text.lowercased().filter { $0.isNumber || $0.isLetter } }
		let text = normalized(recognized)
		return text.contains(normalized(hero.tokens)) && text.contains(normalized(hero.amount))
	}
	#endif

	/// Menu-bar value, Settings, About, and Quit. The popover carries no exits.
	private func presentMenu() {
		let menu = NSMenu()
		let shows = NSMenuItem(title: t(DesktopCopy.menuShows), action: nil, keyEquivalent: "")
		let showsMenu = NSMenu()
		for mode in MenuBarValueMode.allCases {
			let item = NSMenuItem(title: mode.label, action: #selector(selectValueMode(_:)), keyEquivalent: "")
			item.target = self
			item.representedObject = mode.rawValue
			item.state = model.preferences.menuBarValue == mode ? .on : .off
			showsMenu.addItem(item)
		}
		shows.submenu = showsMenu
		menu.addItem(shows)
		menu.addItem(.separator())
		let settings = NSMenuItem(title: t(DesktopCopy.menuSettings), action: #selector(showSettings), keyEquivalent: ",")
		settings.target = self
		menu.addItem(settings)
		let about = NSMenuItem(title: t(DesktopCopy.menuAbout), action: #selector(showAbout), keyEquivalent: "")
		about.target = self
		menu.addItem(about)
		menu.addItem(.separator())
		let quit = NSMenuItem(title: t(DesktopCopy.menuQuit), action: #selector(quit), keyEquivalent: "q")
		quit.target = self
		menu.addItem(quit)

		statusItem.menu = menu
		statusItem.button?.performClick(nil)
		statusItem.menu = nil
	}

	@objc private func selectValueMode(_ sender: NSMenuItem) {
		guard let raw = sender.representedObject as? String, let mode = MenuBarValueMode(rawValue: raw) else { return }
		model.preferences.menuBarValue = mode
	}

	@objc private func showSettings() {
		openSettings()
	}

	@objc private func showAbout() {
		NSApp.activate(ignoringOtherApps: true)
		NSApp.orderFrontStandardAboutPanel(options: MenuBarItemController.aboutPanelOptions())
	}

	@objc private func quit() {
		NSApp.terminate(nil)
	}
}

/// Escape closes the settings window from anywhere in it, and focus opens on
/// the window rather than on its close control.
final class SettingsWindow: NSWindow {
	override func cancelOperation(_ sender: Any?) {
		performClose(sender)
	}
}

@MainActor
final class FittingSizeHostingController<Content: View>: NSHostingController<Content> {
	var fittingSizeDidChange: ((NSSize) -> Void)?

	override func viewDidLayout() {
		super.viewDidLayout()
		fittingSizeDidChange?(view.fittingSize)
	}
}

@MainActor
final class SettingsWindowController {
	private(set) var window: SettingsWindow?
	private let preferences: DesktopPreferences
	private let quotaSettings: QuotaSettingsController

	init(preferences: DesktopPreferences, quotaSettings: QuotaSettingsController) {
		self.preferences = preferences
		self.quotaSettings = quotaSettings
	}

	func show() {
		if window == nil {
			let hosting = FittingSizeHostingController(
				rootView: SettingsWindowView(preferences: preferences, quotaSettings: quotaSettings)
			)
			let created = SettingsWindow(contentViewController: hosting)
			created.title = t(DesktopCopy.settingsTitle)
			created.styleMask = [.titled, .closable]
			created.isReleasedWhenClosed = false
			created.setContentSize(hosting.view.fittingSize)
			hosting.fittingSizeDidChange = { [weak created] size in
				guard let created,
					abs(created.contentLayoutRect.width - size.width) > 0.5
						|| abs(created.contentLayoutRect.height - size.height) > 0.5
				else { return }
				created.setContentSize(size)
			}
			created.center()
			window = created
		}
		NSApp.activate(ignoringOtherApps: true)
		window?.makeKeyAndOrderFront(nil)
		window?.makeFirstResponder(nil)
		let quotaSettings = quotaSettings
		Task { await quotaSettings.refreshNotificationPermission() }
	}
}
