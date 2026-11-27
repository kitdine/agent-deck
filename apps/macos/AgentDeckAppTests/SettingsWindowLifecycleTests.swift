import AppKit
import SwiftUI
import XCTest
@testable import AgentDeck
@testable import AgentDeckShared

/// A host that lays SwiftUI out on 1x pixels whatever display the test runs
/// on: a page fitted to its rounded-up natural height then measures a point
/// taller than its window, as on a non-Retina display.
private final class OneXWindow: NSWindow {
	override var backingScaleFactor: CGFloat { 1 }
}

@MainActor
private final class ReportCounter {
	var count = 0
}

/// A Settings page shows whole or scrolls by its natural height alone, so a
/// point of pixel alignment between its fitted and laid-out height never
/// makes it switch between the two on every layout pass.
@MainActor
final class SettingsPaneLayoutStabilityTests: XCTestCase {
	private func pump(_ window: NSWindow, turns: Int) {
		for _ in 0 ..< turns {
			window.contentView?.layoutSubtreeIfNeeded()
			RunLoop.main.run(until: Date().addingTimeInterval(0.01))
		}
	}

	private func containsScrollView(_ view: NSView) -> Bool {
		view is NSScrollView || view.subviews.contains(where: containsScrollView)
	}

	private func scrollView(in view: NSView) -> NSScrollView? {
		if let scroll = view as? NSScrollView { return scroll }
		for subview in view.subviews {
			if let found = scrollView(in: subview) { return found }
		}
		return nil
	}

	func testAPageAPointTallerThanItsFittedWindowOnA1xDisplaySettles() async throws {
		let preferences = DesktopPreferences(defaults: isolatedDefaults(), registrar: StubLoginItemRegistrar())
		let quota = makeQuotaSettingsController(preferences: preferences)
		await quota.load()
		let telemetry = makeTelemetrySettingsController()
		await telemetry.load()
		for pane in [SettingsPane.general, .liveUsage, .quota] {
			let reports = ReportCounter()
			let hosting = NSHostingController(rootView: AnyView(SettingsPaneView(
				pane: pane, preferences: preferences, quotaSettings: quota, telemetrySettings: telemetry,
				onNaturalHeight: { _ in reports.count += 1 }
			).environment(\.displayScale, 1)))
			hosting.sizingOptions = []
			// The window controller's fit: the natural height rounded up.
			let fitted = hosting.sizeThatFits(in: NSSize(width: 460, height: CGFloat.greatestFiniteMagnitude)).height.rounded(.up)
			let window = OneXWindow(
				contentRect: NSRect(x: 0, y: 0, width: 460, height: fitted), styleMask: [.borderless], backing: .buffered, defer: false
			)
			window.isReleasedWhenClosed = false
			window.contentViewController = hosting
			window.setContentSize(NSSize(width: 460, height: fitted))
			pump(window, turns: 10)
			let settled = reports.count
			pump(window, turns: 40)
			let more = reports.count - settled
			let scrolled = containsScrollView(hosting.view)
			window.contentViewController = nil
			window.close()
			XCTAssertGreaterThan(settled, 0, "\(pane): the page reported its height")
			XCTAssertEqual(more, 0, "\(pane): the page is laid out again on every pass (\(more) more height reports)")
			XCTAssertFalse(scrolled, "\(pane): a page shorter than the screen never shows inside the scroll view")
		}
	}

	func testAShrinkingScreenMakesAWholePageScrollAndAGrowingOneUndoesIt() async throws {
		let preferences = DesktopPreferences(defaults: isolatedDefaults(), registrar: StubLoginItemRegistrar())
		let quota = makeQuotaSettingsController(preferences: preferences)
		await quota.load()
		let telemetry = makeTelemetrySettingsController()
		await telemetry.load()
		let limit = ScreenLimit()
		let reports = ReportCounter()
		let hosting = NSHostingController(rootView: SettingsPaneView(
			pane: .general, preferences: preferences, quotaSettings: quota, telemetrySettings: telemetry,
			onNaturalHeight: { _ in reports.count += 1 }, screenLimit: { limit.value }
		))
		hosting.sizingOptions = []
		let window = NSWindow(
			contentRect: NSRect(x: 0, y: 0, width: 460, height: 400), styleMask: [.borderless], backing: .buffered, defer: false
		)
		window.isReleasedWhenClosed = false
		window.contentViewController = hosting
		pump(window, turns: 10)
		XCTAssertFalse(containsScrollView(hosting.view), "a page shorter than the screen shows whole")
		let before = reports.count

		limit.value = 120
		NotificationCenter.default.post(name: NSApplication.didChangeScreenParametersNotification, object: NSApp)
		pump(window, turns: 10)
		XCTAssertTrue(containsScrollView(hosting.view), "after the usable screen shrinks below the page, the page scrolls")
		XCTAssertGreaterThan(reports.count, before, "the window is asked to refit when the decision flips")

		// The screen shrinks again while the page already scrolls: the scroll
		// view takes the new limit and the window is asked to refit.
		window.setContentSize(NSSize(width: 460, height: 400))
		let whileScrolling = reports.count
		limit.value = 90
		NotificationCenter.default.post(name: NSApplication.didChangeScreenParametersNotification, object: NSApp)
		pump(window, turns: 10)
		XCTAssertGreaterThan(reports.count, whileScrolling, "a new limit while scrolling asks the window to refit")
		let scrollHeight = scrollView(in: hosting.view)?.frame.height ?? .infinity
		XCTAssertLessThanOrEqual(scrollHeight, 90.5, "the scroll view follows the smaller limit (\(scrollHeight))")

		limit.value = 2000
		NotificationCenter.default.post(name: NSApplication.didChangeScreenParametersNotification, object: NSApp)
		pump(window, turns: 10)
		XCTAssertFalse(containsScrollView(hosting.view), "after the screen grows again, the page shows whole")
		window.contentViewController = nil
		window.close()
	}
}

@MainActor
private final class ScreenLimit {
	var value: CGFloat = 2000
}

/// The Settings window's own lifecycle: a page scrolls only past the screen
/// limit with hysteresis; a window off screen is not resized until it shows
/// again; closing takes the window down completely, with or without the
/// sheet, and the next open builds it again as before.
@MainActor
final class SettingsWindowLifecycleTests: XCTestCase {
	private var opened: [SettingsWindowController] = []

	override func tearDown() async throws {
		for controller in opened { controller.tearDown() }
		opened = []
		try await super.tearDown()
	}

	private func makeWindow(
		origin: SessionOriginSettingsController? = nil,
		quotaSettings: QuotaSettingsController? = nil,
		preferences: DesktopPreferences? = nil
	) -> SettingsWindowController {
		let preferences = preferences ?? DesktopPreferences(defaults: isolatedDefaults(), registrar: StubLoginItemRegistrar())
		let controller = SettingsWindowController(
			preferences: preferences, quotaSettings: quotaSettings ?? makeQuotaSettingsController(preferences: preferences),
			telemetrySettings: makeTelemetrySettingsController(), sessionOrigin: origin
		)
		opened.append(controller)
		return controller
	}

	private func makeOrigin() -> SessionOriginSettingsController {
		SessionOriginSettingsController(transport: StubOriginTransport(), publish: { .failed }, newRequestID: { UUID().uuidString })
	}

	private func settle(_ window: NSWindow?) {
		for _ in 0 ..< 8 {
			window?.contentView?.layoutSubtreeIfNeeded()
			RunLoop.main.run(until: Date().addingTimeInterval(0.03))
		}
	}

	/// Runs the run loop until `released` holds or two seconds pass.
	private func waitForRelease(_ released: () -> Bool) {
		let deadline = Date().addingTimeInterval(2)
		while !released(), Date() < deadline {
			RunLoop.main.run(until: Date().addingTimeInterval(0.02))
		}
	}

	func testAPageScrollsOnlyWellPastTheLimitAndStopsOnlyWellBelowIt() {
		let limit: CGFloat = 600
		for scrolling in [false, true] {
			for offset: CGFloat in [-1, 0, 1] {
				XCTAssertEqual(
					SettingsPaneView.scrollsPage(height: limit + offset, limit: limit, scrolling: scrolling), scrolling,
					"a point either side of the limit keeps the page as it is (scrolling: \(scrolling), offset: \(offset))"
				)
			}
		}
		XCTAssertTrue(SettingsPaneView.scrollsPage(height: limit + 40, limit: limit, scrolling: false), "a page well past the limit scrolls")
		XCTAssertFalse(SettingsPaneView.scrollsPage(height: limit - 40, limit: limit, scrolling: true), "a page well below the limit shows whole")
		XCTAssertFalse(SettingsPaneView.scrollsPage(height: 465, limit: SettingsPaneView.maximumHeight, scrolling: false))
	}

	func testClosingReleasesTheWindowAndItsPages() throws {
		let controller = makeWindow(origin: makeOrigin())
		weak var window: SettingsWindow?
		weak var tabs: SettingsTabViewController?
		var pages: [WeakController] = []
		autoreleasepool {
			controller.show(pane: .quota)
			settle(controller.window)
			window = controller.window
			tabs = controller.tabs
			pages = controller.panes.map { WeakController(controller.paneController($0)) }
			window?.performClose(nil)
		}
		XCTAssertNil(controller.window, "closing takes the window down")
		XCTAssertNil(controller.tabs)
		XCTAssertTrue(controller.panes.allSatisfy { controller.paneController($0) == nil })
		waitForRelease { window == nil && tabs == nil && pages.allSatisfy { $0.value == nil } }
		XCTAssertNil(window, "the closed window is released")
		XCTAssertNil(tabs)
		XCTAssertTrue(pages.allSatisfy { $0.value == nil }, "every page's hosting controller is released")
	}

	func testClosingAfterTheSheetReleasesTheWindow() throws {
		for closesSheetFirst in [true, false] {
			let origin = makeOrigin()
			let controller = makeWindow(origin: origin)
			weak var window: SettingsWindow?
			weak var sheet: NSWindow?
			autoreleasepool {
				controller.show(pane: .sessions)
				origin.openManage()
				settle(controller.window)
				window = controller.window
				sheet = controller.window?.attachedSheet
			}
			XCTAssertNotNil(sheet, "the manual-classification sheet is presented")
			autoreleasepool {
				if closesSheetFirst {
					origin.closeManage()
					settle(window)
					XCTAssertNil(window?.attachedSheet)
					window?.performClose(nil)
				} else {
					controller.tearDown()
				}
			}
			XCTAssertNil(controller.window)
			XCTAssertFalse(origin.isManaging, "the sheet does not come back with the next open")
			waitForRelease { window == nil }
			XCTAssertNil(window, "the window is released after the sheet was shown (sheet closed first: \(closesSheetFirst))")
		}
	}

	/// Something outside the controller may keep the closed window object (the
	/// sheet's presentation did); it still holds none of the pages.
	func testAClosedWindowThatIsStillReferencedHoldsNoContent() throws {
		let controller = makeWindow(origin: makeOrigin())
		var held: SettingsWindow?
		weak var tabs: SettingsTabViewController?
		var pages: [WeakController] = []
		// Everything AppKit autoreleases while the window is shown and closed
		// goes with this pool; only `held` keeps the window object.
		autoreleasepool {
			controller.show(pane: .quota)
			settle(controller.window)
			held = controller.window
			tabs = controller.tabs
			pages = controller.panes.map { WeakController(controller.paneController($0)) }
			held?.performClose(nil)
		}
		settle(nil)
		let window = try XCTUnwrap(held)
		XCTAssertFalse(window.isVisible)
		XCTAssertNil(window.contentViewController, "the closed window no longer holds the pages")
		XCTAssertEqual(window.contentView?.subviews.count ?? 0, 0, "the closed window shows no page view")
		XCTAssertNil(window.toolbar, "the closed window no longer holds the page toolbar")
		XCTAssertNil(window.onClose)
		waitForRelease { tabs == nil && pages.allSatisfy { $0.value == nil } }
		XCTAssertNil(tabs, "the tab controller is released with the window still referenced")
		XCTAssertTrue(pages.allSatisfy { $0.value == nil }, "every page is released with the window still referenced")
	}

	/// ux/settings.md 窗口分页: the next open after a close comes back on the
	/// page shown last, fitted to that page, and the sheet opens on it again.
	func testReopeningAfterACloseRebuildsTheWindowAsBefore() throws {
		let origin = makeOrigin()
		let controller = makeWindow(origin: origin)
		controller.show(pane: .quota)
		let first = try XCTUnwrap(controller.window)
		settle(first)
		let firstHeight = first.contentView?.frame.height ?? 0
		first.performClose(nil)

		controller.show()
		let second = try XCTUnwrap(controller.window)
		settle(second)
		XCTAssertFalse(second === first, "the closed window was taken down")
		XCTAssertTrue(second.isVisible)
		XCTAssertEqual(controller.selectedPane, .quota, "a reopened window comes back to the page shown last")
		XCTAssertEqual(controller.tabs?.selectedTabViewItemIndex, 3)
		XCTAssertEqual(second.title, SettingsPane.quota.title)
		XCTAssertEqual(controller.tabs?.tabViewItems.map { $0.identifier as? String }, ["general", "sessions", "liveUsage", "quota"])
		XCTAssertEqual(second.contentView?.frame.height ?? 0, firstHeight, accuracy: 1, "the same page fits the same height")
		XCTAssertEqual(second.contentView?.frame.width ?? 0, 460, accuracy: 0.5)

		controller.select(.sessions)
		origin.openManage()
		settle(second)
		XCTAssertNotNil(second.attachedSheet, "the sheet opens on the rebuilt window")
		controller.select(.quota)
		XCTAssertEqual(controller.selectedPane, .sessions, "no other page while the sheet is open")
		origin.closeManage()
		settle(second)
		XCTAssertNil(second.attachedSheet)
	}

	/// A hidden window is still closed when it is taken down, explicitly or
	/// by releasing its controller: the sheet does not come back and the
	/// lookup draft is discarded.
	func testTakingDownAHiddenWindowDiscardsItsDrafts() throws {
		for releasesController in [false, true] {
			let origin = makeOrigin()
			let preferences = DesktopPreferences(defaults: isolatedDefaults(), registrar: StubLoginItemRegistrar())
			var controller: SettingsWindowController? = SettingsWindowController(
				preferences: preferences, quotaSettings: makeQuotaSettingsController(preferences: preferences),
				telemetrySettings: makeTelemetrySettingsController(), sessionOrigin: origin
			)
			weak var window: SettingsWindow?
			autoreleasepool {
				controller?.show(pane: .sessions)
				origin.openManage()
				origin.setLookupID("draft-id")
				settle(controller?.window)
				window = controller?.window
				window?.orderOut(nil)
			}
			let label = releasesController ? "released controller" : "tearDown()"
			XCTAssertNotNil(window)
			XCTAssertEqual(window?.isVisible, false, "\(label): the window is hidden, not closed")
			XCTAssertTrue(origin.isManaging)
			XCTAssertEqual(origin.lookupID, "draft-id")
			autoreleasepool {
				if releasesController {
					controller = nil
				} else {
					controller?.tearDown()
				}
			}
			waitForRelease { !origin.isManaging && window == nil }
			XCTAssertFalse(origin.isManaging, "\(label): the sheet does not come back with the next open")
			XCTAssertEqual(origin.lookupID, "", "\(label): the lookup draft is discarded")
			XCTAssertNil(window, "\(label): the hidden window is released")
		}
	}

	/// The Quota page shown, then hidden while its denied-permission warning
	/// appears. Every resize the controller asks for is recorded.
	private func quotaPageThatGrowsWhileHidden() async throws -> (
		controller: SettingsWindowController, window: SettingsWindow, resizes: ResizeRecorder, grown: CGFloat
	) {
		let preferences = DesktopPreferences(defaults: isolatedDefaults(), registrar: StubLoginItemRegistrar())
		let transport = StubQuotaSettingsTransport(settings: DesktopQuotaSettingsValuesV1(
			reading: true, interval: .fiveMinutes, alerts: false, thresholds: [75, 90], resetNotice: false, statusline: false
		))
		let quotaSettings = makeQuotaSettingsController(
			preferences: preferences, transport: transport, notifications: StubNotificationPermission(granted: false)
		)
		let controller = makeWindow(quotaSettings: quotaSettings, preferences: preferences)
		controller.show(pane: .quota)
		let window = try XCTUnwrap(controller.window)
		await quotaSettings.load()
		settle(window)
		let before = window.contentView?.frame.height ?? 0
		let resizes = ResizeRecorder()
		let resizeWindow = controller.resizeWindow
		controller.resizeWindow = { window, frame, animate in
			resizes.frames.append(frame)
			resizeWindow(window, frame, animate)
		}

		window.orderOut(nil)
		XCTAssertFalse(window.isVisible)
		await quotaSettings.setAlerts(true)
		XCTAssertNotNil(quotaSettings.alertsRow, "the denied-permission warning row appears")
		settle(window)
		let grown = try XCTUnwrap(controller.naturalSize(of: .quota)).height.rounded(.up)
		XCTAssertGreaterThan(grown, before + 1, "the page grew while off screen")
		XCTAssertEqual(resizes.frames, [], "a window off screen is not resized")
		XCTAssertEqual(window.contentView?.frame.height ?? 0, before, accuracy: 0.5)
		return (controller, window, resizes, grown)
	}

	/// A page that grows while the window is off screen leaves the window
	/// alone; showing the window again fits the grown page.
	func testAPageThatGrowsWhileTheWindowIsOffScreenFitsWhenItShowsAgain() async throws {
		let (controller, window, resizes, grown) = try await quotaPageThatGrowsWhileHidden()
		controller.show()
		settle(window)
		XCTAssertTrue(window === controller.window)
		XCTAssertFalse(resizes.frames.isEmpty, "showing again fits the page")
		XCTAssertEqual(window.contentView?.frame.height ?? 0, grown, accuracy: 1, "the window fits the grown page")
	}

	/// Unhiding the App puts the window back on screen without `show()`; the
	/// window's occlusion change then fits the page that grew meanwhile.
	func testAPageThatGrowsWhileTheAppIsHiddenFitsWhenTheWindowIsVisibleAgain() async throws {
		let (controller, window, resizes, grown) = try await quotaPageThatGrowsWhileHidden()
		window.orderFront(nil)
		XCTAssertTrue(window.isVisible)
		// The system reports the change asynchronously; posting it here makes
		// the test independent of when (or whether) the host's WindowServer does.
		NotificationCenter.default.post(name: NSWindow.didChangeOcclusionStateNotification, object: window)
		settle(window)
		XCTAssertFalse(resizes.frames.isEmpty, "becoming visible again fits the page")
		XCTAssertEqual(window.contentView?.frame.height ?? 0, grown, accuracy: 1, "the window fits the grown page")

		// A taken-down window's notification reaches nothing.
		controller.tearDown()
		resizes.frames = []
		NotificationCenter.default.post(name: NSWindow.didChangeOcclusionStateNotification, object: window)
		settle(nil)
		XCTAssertEqual(resizes.frames, [])
	}
}

@MainActor
private final class ResizeRecorder {
	var frames: [NSRect] = []
}

@MainActor
private struct WeakController {
	weak var value: NSViewController?
	init(_ value: NSViewController?) { self.value = value }
}
