import AgentDeckShared
import AppKit
import Observation
import SwiftUI

/// A display link retains its target. Keep preparation independent of the
/// controller's lifetime; this link is never added to a run loop.
private final class PopoverPreparationDisplayLinkTarget: NSObject {
	@objc func tick(_ link: CADisplayLink) {}
}

/// Runs its action on the first display refresh, then stops.
final class FirstRefreshTarget: NSObject {
	private var action: (() -> Void)?

	init(_ action: @escaping () -> Void) {
		self.action = action
	}

	@objc func tick(_ link: CADisplayLink) {
		link.invalidate()
		let action = self.action
		self.action = nil
		action?()
	}
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
	private(set) var presentationPreparationCount = 0
	var popoverIsShown: Bool { popover.isShown }
	/// Show the popover once, invisibly, after the first preparation so its own
	/// window and backing store exist before the first click (architecture,
	/// open path step 2). The App enables it; it never runs otherwise.
	var warmsWindowAfterPreparation = false {
		didSet { if warmsWindowAfterPreparation, isInitialPresentationPrepared { scheduleWindowWarmUp() } }
	}
	private(set) var isWindowWarmed = false
	/// Ends an in-progress warm-up; a click during it finishes it first.
	private var finishWarm: (() -> Void)?
	/// The publication hold of the current open, released by its first frame.
	private var openHold: PublicationHold?
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
		let hosting = NSHostingController(rootView: MenuBarSurfaceView(model: model))
		// The popover's size is set explicitly (configurePopoverGeometry). Without
		// sizing options the controller no longer re-measures the whole surface
		// for its constraints and preferred size on every update.
		hosting.sizingOptions = []
		popover.contentViewController = hosting

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
		// clickCount raises for a non-mouse event, which is current when the
		// action arrives without a click (an accessibility press, or the perf
		// driver's action stimulus).
		let isMouseUp = event?.type == .leftMouseUp || event?.type == .rightMouseUp
		let wantsMenu = event?.type == .rightMouseUp || (isMouseUp && (event?.clickCount ?? 1) >= 2)
		if PerfTrace.isEnabled {
			// Which branch the click takes, as integers only.
			PerfTrace.mark(.inputAction, PerfCount("menu", wantsMenu ? 1 : 0), PerfCount("shown", popover.isShown ? 1 : 0),
				PerfCount("event_type", Int64(event?.type.rawValue ?? 0)))
			PerfFrameProbe.shared.inputActionHandled()
		}
		if wantsMenu {
			closePopover()
			presentMenu()
			return
		}
		togglePopover(sender)
	}

	private func togglePopover(_ sender: NSStatusBarButton) {
		finishWarm?()
		if popover.isShown {
			closePopover()
		} else {
			hasPresentedPopover = true
			PerfTrace.mark(.popoverShowBegin)
			if PerfTrace.isEnabled { PerfFrameProbe.shared.rearm() }
			configurePopoverGeometry(for: sender)
			holdPublicationsUntilFirstFrame()
			popover.show(relativeTo: sender.bounds, of: sender, preferredEdge: .minY)
			retirePreparationWindow()
			MenuBarItemController.activatePopoverWindow(popover.contentViewController?.view.window)
			// Show/activation only; the open ends at the data frame's frame.vsync.
			PerfTrace.mark(.popoverShowEnd)
		}
	}

	/// A publication arriving while the popover opens would lay the whole
	/// surface out again before its first frame; let that frame commit and reach
	/// the display with the data already shown, then install it (at most 300 ms
	/// later).
	private func holdPublicationsUntilFirstFrame() {
		openHold?.release()
		let hold = PublicationHold(model.coordinator)
		openHold = hold
		CATransaction.setCompletionBlock {
			// After the first frame's commit, wait for the refresh that shows it;
			// release on a later main-queue turn so other work of that refresh
			// runs before the new data's layout.
			DispatchQueue.main.async {
				MainActor.assumeIsolated { [weak self] in
					guard let view = self?.popover.contentViewController?.view else { return hold.release() }
					// The display link retains its target until it is invalidated.
					let target = FirstRefreshTarget { DispatchQueue.main.async { MainActor.assumeIsolated { hold.release() } } }
					view.displayLink(target: target, selector: #selector(FirstRefreshTarget.tick(_:))).add(to: .main, forMode: .common)
				}
			}
		}
		DispatchQueue.main.asyncAfter(deadline: .now() + .milliseconds(300)) { MainActor.assumeIsolated { hold.release() } }
	}

	/// The popover has adopted the same hosting view. Its preparation window was
	/// never ordered onscreen and no longer owns the content.
	private func retirePreparationWindow() {
		let preparationWindow = presentationPreparationWindow
		presentationPreparationWindow = nil
		if let preparationWindow {
			// Retiring an unshown window is not needed to display the popover.
			// Keep AppKit teardown out of this synchronous input action.
			Task { @MainActor in preparationWindow.close() }
		}
	}

	/// Shows the same popover once, transparent and ignoring the mouse, without
	/// activation or key, keeps it for one Core Animation commit so AppKit and
	/// the window server create its window, then closes it. It must never draw
	/// a visible frame, change the front app, or leave the key window changed;
	/// popover.warm records them, and whether the popover was key meanwhile.
	private func warmPopoverWindow() {
		guard warmsWindowAfterPreparation, !isWindowWarmed, finishWarm == nil, !hasPresentedPopover, !popover.isShown,
			let button = statusItem.button else { return }
		let keyBefore = NSApp.keyWindow
		let frontBefore = NSWorkspace.shared.frontmostApplication?.processIdentifier
		let began = PerfTrace.now()
		configurePopoverGeometry(for: button)
		popover.show(relativeTo: button.bounds, of: button, preferredEdge: .minY)
		retirePreparationWindow()
		guard let window = popover.contentViewController?.view.window else {
			popover.performClose(nil)
			return
		}
		// Before this pass's commit: no frame of it is ever drawn opaque.
		window.alphaValue = 0
		window.ignoresMouseEvents = true
		var visible: Int64 = 0
		var keyDuring: Int64 = 0
		var keyChanged: Int64 = 0
		var frontChanged: Int64 = 0
		let checkFront = {
			if NSWorkspace.shared.frontmostApplication?.processIdentifier != frontBefore { frontChanged = 1 }
		}
		var observer: CFRunLoopObserver?
		finishWarm = { [weak self] in
			guard let self else { return }
			if let observer { CFRunLoopRemoveObserver(CFRunLoopGetMain(), observer, .commonModes) }
			observer = nil
			self.finishWarm = nil
			// Visibility only while ordered in, before alpha is restored.
			if window.isVisible, window.alphaValue > 0 { visible = 1 }
			// AppKit may make the shown popover the App's key window; give the
			// key back so a window the user was typing in keeps it.
			if NSApp.keyWindow !== keyBefore { keyDuring = 1 }
			checkFront()
			self.popover.performClose(nil)
			window.alphaValue = 1
			window.ignoresMouseEvents = false
			// AppKit settles the key window after the close returns; restore and
			// check it on the next main-queue turn.
			DispatchQueue.main.async { [weak self] in
				MainActor.assumeIsolated {
					if let keyBefore, NSApp.keyWindow !== keyBefore, keyBefore.isVisible { keyBefore.makeKey() }
					if NSApp.keyWindow !== keyBefore { keyChanged = 1 }
					checkFront()
					self?.isWindowWarmed = true
					PerfTrace.mark(.popoverWarm, PerfCount("visible", visible), PerfCount("key_during", keyDuring),
						PerfCount("key_changed", keyChanged), PerfCount("front_changed", frontChanged),
						PerfCount("dur_ns", Int64(PerfTrace.now() - began)))
				}
			}
		}
		// Core Animation commits in a beforeWaiting observer at order 2,000,000;
		// this one runs after the commit that ordered the window in.
		observer = CFRunLoopObserverCreateWithHandler(nil, CFRunLoopActivity.beforeWaiting.rawValue, false, 2_000_001) {
			_, _ in MainActor.assumeIsolated { [weak self] in self?.finishWarm?() }
		}
		CFRunLoopAddObserver(CFRunLoopGetMain(), observer, .commonModes)
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
		presentationPreparationCount += 1
		// Never show a window, request focus, or start a helper here.
		if warmsWindowAfterPreparation { scheduleWindowWarmUp() }
	}

	/// After preparation, on a later idle pass of the main run loop.
	private func scheduleWindowWarmUp() {
		RunLoop.main.perform(inModes: [.default]) { [weak self] in
			MainActor.assumeIsolated { self?.warmPopoverWindow() }
		}
	}

	static func activatePopoverWindow(_ window: NSWindow?) {
		window?.makeKey()
		window?.makeFirstResponder(nil)
	}

	func closePopover() {
		openHold?.release()
		openHold = nil
		popover.performClose(nil)
	}

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
