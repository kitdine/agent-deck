import AgentDeckShared
import AppKit
import Observation
import SwiftUI

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
	private(set) var presentationPreparationCount = 0
	var popoverIsShown: Bool { popover.isShown }
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
		if popover.isShown {
			closePopover()
		} else {
			hasPresentedPopover = true
			PerfTrace.mark(.popoverShowBegin)
			if PerfTrace.isEnabled { PerfFrameProbe.shared.rearm() }
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
			// Show/activation only; the open ends at the data frame's frame.vsync.
			PerfTrace.mark(.popoverShowEnd)
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
	}

	static func activatePopoverWindow(_ window: NSWindow?) {
		window?.makeKey()
		window?.makeFirstResponder(nil)
	}

	func closePopover() {
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
