import AppKit
import SwiftUI
import Vision
import XCTest
@testable import AgentDeck
@testable import AgentDeckShared

private final class ClientFilterBoundsCapture: @unchecked Sendable {
	private let lock = NSLock()
	private var stored: [String: CGRect] = [:]
	func update(_ frames: [String: CGRect]) { lock.withLock { stored = frames } }
	var frames: [String: CGRect] { lock.withLock { stored } }
}

@MainActor
final class MenuBarChromeTests: XCTestCase {
	private static var retainedFocusWindows = [NSWindow]()
	func testStandaloneReloadIncludesEveryWidgetKind() {
		XCTAssertEqual(AgentDeckMain.widgetKinds, [
			"com.kitdine.agentdeck.widget.magnitude",
			"com.kitdine.agentdeck.widget.composition",
			"com.kitdine.agentdeck.widget.trust",
			"com.kitdine.agentdeck.widget.rhythm",
			"com.kitdine.agentdeck.widget.quota",
		])
	}

	func testScanProgressRendersAtNativeWidthsInBothLanguages() async throws {
		let oldWidth = ProcessInfo.processInfo.environment["AGENTDECK_TEST_WIDTH"]
		let oldLocale = ProcessInfo.processInfo.environment["AGENTDECK_TEST_LOCALE"]
		defer {
			if let oldWidth { setenv("AGENTDECK_TEST_WIDTH", oldWidth, 1) } else { unsetenv("AGENTDECK_TEST_WIDTH") }
			if let oldLocale { setenv("AGENTDECK_TEST_LOCALE", oldLocale, 1) } else { unsetenv("AGENTDECK_TEST_LOCALE") }
		}
		for language in ["en", "zh-Hans"] {
			setenv("AGENTDECK_TEST_LOCALE", language, 1)
			for width in [280, 420] {
				setenv("AGENTDECK_TEST_WIDTH", String(width), 1)
				let host = StubDesktopHost(behavior: .suspendedEnvelope(WireFixture.envelope()))
				let model = await makeModel(host: host)
				let refresh = Task { await model.coordinator.refresh() }
				while host.refreshCount == 0 || model.coordinator.scanProgress?.stage != .importing {
					await Task.yield()
				}
				XCTAssertEqual(model.scanProgressStageText, t(DesktopCopy.scanImporting))
				XCTAssertNotNil(model.scanProgressCountsText)
				let view = MenuBarSurfaceView(model: model).environment(\.dynamicTypeSize, .accessibility3)
				let hosting = NSHostingView(rootView: view)
				hosting.frame = NSRect(x: 0, y: 0, width: CGFloat(width), height: 760)
				hosting.layoutSubtreeIfNeeded()
				hosting.displayIfNeeded()
				XCTAssertLessThanOrEqual(hosting.fittingSize.width, CGFloat(width) + 1)
				let png = try renderedViewPNG(hosting)
				XCTAssertGreaterThan(png.count, 4_000)
				add(renderingAttachment(png, named: "Scan progress — \(language) — \(width) — accessibility3"))
				host.resume()
				await refresh.value
			}
		}
	}

	func testRefreshFailureAndFirstUseRenderAtNativeWidthsInBothLanguagesAndThemes() async throws {
		let oldWidth = ProcessInfo.processInfo.environment["AGENTDECK_TEST_WIDTH"]
		let oldLocale = ProcessInfo.processInfo.environment["AGENTDECK_TEST_LOCALE"]
		defer {
			if let oldWidth { setenv("AGENTDECK_TEST_WIDTH", oldWidth, 1) } else { unsetenv("AGENTDECK_TEST_WIDTH") }
			if let oldLocale { setenv("AGENTDECK_TEST_LOCALE", oldLocale, 1) } else { unsetenv("AGENTDECK_TEST_LOCALE") }
		}
		for (language, scheme) in [("en", ColorScheme.light), ("zh-Hans", .dark)] {
			setenv("AGENTDECK_TEST_LOCALE", language, 1)
			for width in [280, 420] {
				setenv("AGENTDECK_TEST_WIDTH", String(width), 1)
				let retainedHost = StubDesktopHost(behavior: .envelope(WireFixture.envelope()))
				let retained = await makeModel(host: retainedHost)
				await retained.coordinator.refresh()
				retainedHost.behavior = .failure(HelperExecutionError.timedOut)
				await retained.coordinator.refresh()
				let first = await makeModel(host: StubDesktopHost(behavior: .failure(HelperExecutionError.timedOut)))
				await first.coordinator.refresh()

				for (name, model) in [("retained", retained), ("first", first)] {
					let view = MenuBarSurfaceView(model: model).preferredColorScheme(scheme)
					let hosting = NSHostingView(rootView: view)
					hosting.frame = NSRect(x: 0, y: 0, width: CGFloat(width), height: 760)
					hosting.layoutSubtreeIfNeeded()
					hosting.displayIfNeeded()
					XCTAssertLessThanOrEqual(hosting.fittingSize.width, CGFloat(width) + 1)
					let png = try renderedViewPNG(hosting)
					XCTAssertGreaterThan(png.count, 4_000)
					add(renderingAttachment(png, named: "Refresh \(name) — \(language) — \(width)"))
				}
			}
		}
	}

	func testRefreshControlIdentitySurvivesErrorRunningSuccessAndFailure() async throws {
		let host = StubDesktopHost(behavior: .failure(HelperExecutionError.timedOut))
		let model = await makeModel(host: host)
		await model.coordinator.refresh()
		let capture = RefreshControlIdentityCapture()
		let view = RefreshControlIdentityProbe(
			content: MenuBarSurfaceView(model: model),
			capture: capture
		)
		let hosting = NSHostingView(rootView: view)
		hosting.frame = NSRect(x: 0, y: 0, width: 420, height: 760)
		let window = NSWindow(contentRect: hosting.frame, styleMask: [.titled], backing: .buffered, defer: false)
		let container = NSView(frame: hosting.frame)
		container.addSubview(hosting)
		let otherButton = NSButton(title: "Other", target: nil, action: nil)
		otherButton.frame = NSRect(x: 0, y: 0, width: 80, height: 24)
		container.addSubview(otherButton)
		window.contentView = container
		window.makeKeyAndOrderFront(nil)
		Self.retainedFocusWindows.append(window)

		func renderState() throws -> UUID {
			hosting.layoutSubtreeIfNeeded()
			hosting.displayIfNeeded()
			RunLoop.current.run(until: Date().addingTimeInterval(0.02))
			hosting.layoutSubtreeIfNeeded()
			return try XCTUnwrap(capture.identity)
		}

		_ = try renderState()
		let button = try XCTUnwrap(findRefreshButton(in: hosting))
		XCTAssertEqual(button.keyEquivalent, "r")
		XCTAssertEqual(button.keyEquivalentModifierMask, NSEvent.ModifierFlags.command)
		XCTAssertTrue(window.makeFirstResponder(button))
		XCTAssertTrue(window.firstResponder === button, "Retry must receive actual AppKit first-responder focus")
		let errorIdentity = try renderState()
		host.behavior = .suspendedEnvelope(WireFixture.envelope())
		let success = Task { await model.coordinator.refresh() }
		while host.refreshCount < 2 { await Task.yield() }
		XCTAssertEqual(model.refreshActionState, .running)
		let runningIdentity = try renderState()
		XCTAssertEqual(runningIdentity, errorIdentity)
		XCTAssertTrue(findRefreshButton(in: hosting) === button, "running must keep the same native control")
		host.resume()
		await success.value
		XCTAssertEqual(model.refreshActionState, .succeeded)
		let successIdentity = try renderState()
		XCTAssertEqual(successIdentity, errorIdentity)
		XCTAssertTrue(window.firstResponder === button, "focus must return when the control becomes enabled after success")
		XCTAssertTrue(window.makeFirstResponder(otherButton))
		model.selectedPanel = .usage
		_ = try renderState()
		XCTAssertTrue(window.firstResponder === otherButton, "an unrelated enabled-state update must not consume a stale restore request")
		XCTAssertTrue(window.makeFirstResponder(button))

		host.behavior = .suspendedFailure(HelperExecutionError.timedOut)
		let failure = Task { await model.coordinator.refresh() }
		while host.refreshCount < 3 { await Task.yield() }
		XCTAssertEqual(model.refreshActionState, .running)
		XCTAssertEqual(try renderState(), errorIdentity)
		host.resume()
		await failure.value
		XCTAssertEqual(model.refreshActionState, .failed)
		let failureIdentity = try renderState()
		XCTAssertEqual(failureIdentity, errorIdentity)
		XCTAssertTrue(window.firstResponder === button, "focus must remain on Retry after failure")
	}

	private func findRefreshButton(in view: NSView) -> NSButton? {
		if let button = view as? NSButton,
			button.identifier?.rawValue.hasPrefix("menubar.refresh.") == true
		{
			return button
		}
		for child in view.subviews {
			if let result = findRefreshButton(in: child) { return result }
		}
		return nil
	}

	func testFixedHeightPopoverPinsHeaderToTheTopEdge() async throws {
		let model = await makeModel(host: StubDesktopHost(behavior: .envelope(WireFixture.envelope())))
		await model.coordinator.refresh()
		let controller = NSHostingController(rootView: MenuBarSurfaceView(model: model, height: 760))
		controller.view.frame = NSRect(x: 0, y: 0, width: 420, height: 760)
		controller.view.layoutSubtreeIfNeeded()
		controller.view.displayIfNeeded()
		let button = try XCTUnwrap(findRefreshButton(in: controller.view))
		let frame = controller.view.convert(button.bounds, from: button)
		XCTAssertLessThan(frame.minY, 40, "the header was displaced away from the top edge: \(frame)")
		XCTAssertLessThanOrEqual(frame.height, MenuBarGeometry.rowMinimumHeight + 1, "the header absorbed the popover's spare height: \(frame)")
		XCTAssertLessThanOrEqual(frame.width, 96, "the refresh control expanded across the header instead of hugging its icon and title: \(frame)")
	}

	func testQuotaPanelKeepsTheSharedClientHeroAndPeriodContext() async throws {
		let model = await makeModel(host: StubDesktopHost(behavior: .envelope(WireFixture.envelope())))
		await model.coordinator.refresh()
		model.selectedPanel = .quota
		let capture = SharedUsageContextPresenceCapture()
		let view = SharedUsageContextPresenceProbe(
			content: MenuBarSurfaceView(model: model, height: 760),
			capture: capture
		)
		let controller = NSHostingController(rootView: view)
		controller.view.frame = NSRect(x: 0, y: 0, width: 420, height: 760)
		controller.view.layoutSubtreeIfNeeded()
		controller.view.displayIfNeeded()
		for _ in 0..<10 {
			await Task.yield()
			controller.view.layoutSubtreeIfNeeded()
		}
		XCTAssertTrue(capture.isPresent, "quota must retain the prototype's shared client, hero, and period context above the panel switcher")
	}

	func testMenuBarPopoverActivationClearsInitialResponderAcrossQuotaInsertion() async throws {
		let host = StubDesktopHost(behavior: .envelope(WireFixture.envelope()))
		let model = await makeModel(host: host)
		model.selectedPanel = .quota
		let hosting = NSHostingView(rootView: MenuBarSurfaceView(model: model, height: 760))
		hosting.frame = NSRect(x: 0, y: 0, width: 420, height: 760)
		let container = NSView(frame: hosting.frame)
		container.addSubview(hosting)
		let sentinel = NSButton(title: "Initial responder", target: nil, action: nil)
		sentinel.frame = NSRect(x: 0, y: 0, width: 120, height: 24)
		container.addSubview(sentinel)
		let window = NSWindow(contentRect: hosting.frame, styleMask: [.titled], backing: .buffered, defer: false)
		window.contentView = container
		window.makeKeyAndOrderFront(nil)
		Self.retainedFocusWindows.append(window)

		XCTAssertTrue(window.makeFirstResponder(sentinel))
		MenuBarItemController.activatePopoverWindow(window)
		XCTAssertTrue(window.firstResponder === window)

		await model.coordinator.refresh()
		hosting.layoutSubtreeIfNeeded()
		hosting.displayIfNeeded()
		XCTAssertTrue(window.firstResponder === window, "inserting quota rows after refresh must not claim initial focus")
		XCTAssertTrue(window.childWindows?.filter(\.isVisible).isEmpty ?? true)
	}

	func testInitialPopoverPreparationWaitsForDataAndRunsOnce() async throws {
		let host = StubDesktopHost(behavior: .suspendedEnvelope(WireFixture.envelope()))
		let model = await makeModel(host: host)
		let controller = MenuBarItemController(model: model, openSettings: {})
		XCTAssertFalse(controller.isInitialPresentationPrepared)
		let refresh = Task { await model.coordinator.refresh() }
		while host.refreshCount == 0 { await Task.yield() }
		XCTAssertEqual(model.surface, .loadingSurface)
		XCTAssertFalse(controller.isInitialPresentationPrepared)
		host.resume()
		await refresh.value
		let clock = ContinuousClock()
		let deadline = clock.now.advanced(by: .seconds(2))
		while !controller.isInitialPresentationPrepared, clock.now < deadline {
			try await Task.sleep(for: .milliseconds(10))
		}
		XCTAssertTrue(controller.isInitialPresentationPrepared)
		host.behavior = .envelope(WireFixture.envelope())
		await model.coordinator.refresh()
		await Task.yield()
		XCTAssertEqual(controller.presentationPreparationCount, 1)
		XCTAssertFalse(controller.popoverIsShown)
	}

	func testInitialPopoverPreparationDoesNotShowAWindowOrMoveFocus() async throws {
		let model = await makeModel(host: StubDesktopHost(behavior: .envelope(WireFixture.envelope())))
		let controller = MenuBarItemController(model: model, openSettings: {})
		XCTAssertNil(controller.presentationPreparationWindow)
		let window = NSWindow(contentRect: NSRect(x: 0, y: 0, width: 120, height: 40),
			styleMask: [.titled], backing: .buffered, defer: false)
		let sentinel = NSButton(title: "Keep focus", target: nil, action: nil)
		window.contentView = sentinel
		window.makeKeyAndOrderFront(nil)
		Self.retainedFocusWindows.append(window)
		XCTAssertTrue(window.makeFirstResponder(sentinel))
		let keyWindowBefore = NSApp.keyWindow
		await model.coordinator.refresh()
		let clock = ContinuousClock()
		let deadline = clock.now.advanced(by: .seconds(2))
		while !controller.isInitialPresentationPrepared, clock.now < deadline {
			try await Task.sleep(for: .milliseconds(10))
		}
		XCTAssertTrue(controller.isInitialPresentationPrepared)
		XCTAssertFalse(controller.popoverIsShown)
		XCTAssertTrue(window.firstResponder === sentinel)
		XCTAssertTrue(NSApp.keyWindow === keyWindowBefore)
		XCTAssertEqual(controller.presentationPreparationCount, 1)
		// Status-bar scenes from other controllers can connect or retire during
		// the await. Inspect this controller's actual preparation window.
		let preparationWindow = try XCTUnwrap(controller.presentationPreparationWindow)
		XCTAssertFalse(preparationWindow.isVisible)
		XCTAssertFalse(preparationWindow.isKeyWindow)
		XCTAssertFalse(preparationWindow.contentView?.subviews.isEmpty ?? true,
			"the unchanged hosting hierarchy must be attached before first presentation")
		XCTAssertEqual(model.selectedClient, "all")
		XCTAssertEqual(model.selectedPeriod, "today")
		XCTAssertEqual(model.selectedPanel, .quota)
	}

	func testWindowWarmUpRunsOnceWithoutShowingAWindowOrMovingFocus() async throws {
		let model = await makeModel(host: StubDesktopHost(behavior: .envelope(WireFixture.envelope())))
		let controller = MenuBarItemController(model: model, openSettings: {})
		controller.warmsWindowAfterPreparation = true
		let window = NSWindow(contentRect: NSRect(x: 0, y: 0, width: 120, height: 40),
			styleMask: [.titled], backing: .buffered, defer: false)
		let sentinel = NSButton(title: "Keep focus", target: nil, action: nil)
		window.contentView = sentinel
		window.makeKeyAndOrderFront(nil)
		Self.retainedFocusWindows.append(window)
		XCTAssertTrue(window.makeFirstResponder(sentinel))
		let visibleWindowNumbers = Self.visibleContentWindowNumbers()
		let keyBefore = NSApp.keyWindow
		await model.coordinator.refresh()
		let clock = ContinuousClock()
		let deadline = clock.now.advanced(by: .seconds(5))
		while !controller.isWindowWarmed, clock.now < deadline {
			try await Task.sleep(for: .milliseconds(10))
		}
		XCTAssertTrue(controller.isWindowWarmed)
		XCTAssertFalse(controller.popoverIsShown)
		XCTAssertTrue(NSApp.keyWindow === keyBefore, "the warm-up must give the key window back")
		XCTAssertTrue(window.firstResponder === sentinel)
		XCTAssertEqual(Self.visibleContentWindowNumbers(), visibleWindowNumbers)
		XCTAssertEqual(controller.presentationPreparationCount, 1)
	}

	// The status item adds its own bar window on each extra display once it has
	// content; only content windows tell whether the warm-up showed something.
	private static func visibleContentWindowNumbers() -> Set<Int> {
		Set(NSApp.windows.filter { $0.isVisible && String(describing: type(of: $0)) != "NSStatusBarWindow" }.map(\.windowNumber))
	}

	func testInitialPopoverPreparationUsesAlreadyReadyMemoryWithoutRefresh() async throws {
		let host = StubDesktopHost(behavior: .envelope(WireFixture.envelope()))
		let model = await makeModel(host: host)
		await model.coordinator.refresh()
		let refreshCount = host.refreshCount
		let controller = MenuBarItemController(model: model, openSettings: {})
		XCTAssertTrue(controller.isInitialPresentationPrepared)
		XCTAssertEqual(host.refreshCount, refreshCount)
	}

	func testPreparedPopoverControllerDoesNotRetainItself() async throws {
		let model = await makeModel(host: StubDesktopHost(behavior: .envelope(WireFixture.envelope())))
		await model.coordinator.refresh()
		var controller: MenuBarItemController? = MenuBarItemController(model: model, openSettings: {})
		XCTAssertTrue(controller?.isInitialPresentationPrepared == true)
		weak var released = controller
		controller = nil
		XCTAssertNil(released, "unshown preparation must not create a controller retention cycle")
	}

	func testFirstOpenRetiresPreparationWindowWithoutRemovingPopoverContent() async throws {
		let host = StubDesktopHost(behavior: .envelope(WireFixture.envelope()))
		let views = WireFixture.fixtureScopeViews()
		let reader = StubSessionScopeReader { selection in
			guard let view = views[selection] else { return .failure(.malformedOutput) }
			return .view(view)
		}
		let model = await makeModel(host: host, sessionScopeReader: reader)
		await model.coordinator.refresh()
		model.selectedPanel = .sessions
		@MainActor func waitForScope(_ scope: DesktopSessionScopeV1) async throws {
			let clock = ContinuousClock()
			let deadline = clock.now.advanced(by: .seconds(2))
			while (model.sessionScope.accepted?.selection?.scope != scope || model.sessionScope.request != .idle), clock.now < deadline {
				try await Task.sleep(for: .milliseconds(5))
			}
			XCTAssertEqual(model.sessionScope.accepted?.selection?.scope, scope)
			XCTAssertEqual(model.sessionScope.request, .idle)
		}
		let existingWindows = Set(NSApp.windows.map(ObjectIdentifier.init))
		let controller = MenuBarItemController(model: model, openSettings: {})
		defer { controller.closePopover() }
		let preparationWindow = try XCTUnwrap(NSApp.windows.first {
			!existingWindows.contains(ObjectIdentifier($0)) && !$0.isVisible
				&& $0.contentView?.subviews.isEmpty == false
		})
		let preparedView = try XCTUnwrap(preparationWindow.contentView?.subviews.first)
		let closed = expectation(description: "preparation window retired")
		let observer = NotificationCenter.default.addObserver(
			forName: NSWindow.willCloseNotification, object: preparationWindow, queue: .main
		) { _ in closed.fulfill() }
		defer { NotificationCenter.default.removeObserver(observer) }
		let item = NSStatusBar.system.statusItem(withLength: NSStatusItem.variableLength)
		defer { NSStatusBar.system.removeStatusItem(item) }
		let button = try XCTUnwrap(item.button)
		button.title = "Fixture"
		button.target = controller
		button.action = NSSelectorFromString("handleClick:")
		let anchor = NSWindow(contentRect: NSRect(x: 100, y: 100, width: 160, height: 50),
			styleMask: [.titled], backing: .buffered, defer: false)
		anchor.isReleasedWhenClosed = false
		anchor.contentView = button
		anchor.makeKeyAndOrderFront(nil)
		defer { anchor.close() }
		XCTAssertTrue(button.window?.isVisible == true, "fixture anchor must be visible before opening")
		XCTAssertGreaterThan(button.bounds.width, 0, "fixture anchor must have completed layout")
		button.performClick(nil)
		let result = await XCTWaiter.fulfillment(of: [closed], timeout: 2)
		XCTAssertEqual(result, .completed)
		let visibleWindow = try XCTUnwrap(preparedView.window)
		XCTAssertFalse(visibleWindow === preparationWindow)
		XCTAssertTrue(visibleWindow.isVisible, "teardown must not remove the transferred hosting view")
		XCTAssertTrue(visibleWindow.firstResponder === visibleWindow)
		try await waitForScope(.work)
		model.chooseSessionScope(.all)
		try await waitForScope(.all)

		// A new confirmed default does not replace this open's temporary choice.
		var nextView = WireFixture.sessionScopeView(scope: "auxiliary",
			defaults: WireFixture.confirmedDefault("auxiliary", revision: 2), sessions: 0,
			projects: 0, knownDurationMS: 0, unknownCount: 0, recent: [])
		nextView["generated_at"] = "2026-08-13T10:01:00Z"
		nextView["publication_id"] = "native-reopen-default-2"
		host.behavior = .envelope(WireFixture.envelope(sessionScope: .custom(nextView), generatedAt: "2026-08-13T10:01:00Z"))
		await model.coordinator.refresh()
		let defaultClock = ContinuousClock()
		let defaultDeadline = defaultClock.now.advanced(by: .seconds(2))
		while model.sessionScopeField.view?.default.value != .auxiliary, defaultClock.now < defaultDeadline {
			try await Task.sleep(for: .milliseconds(5))
		}
		XCTAssertEqual(model.sessionScopeField.view?.default.value, .auxiliary)
		XCTAssertEqual(model.sessionScopeField.view?.default.revision, 2)
		try await waitForScope(.all)
		XCTAssertEqual(model.sessionScope.requestedScope, .all)

		// Exercise the actual NSPopover close/show path twice, without key events.
		for _ in 0 ..< 2 {
			controller.closePopover()
			XCTAssertFalse(controller.popoverIsShown)
			button.performClick(nil)
			XCTAssertTrue(controller.popoverIsShown)
			try await waitForScope(.auxiliary)
			XCTAssertEqual(model.sessionScope.requestedScope, .auxiliary)
			XCTAssertTrue(preparedView.window?.isVisible == true)
			XCTAssertEqual(controller.presentationPreparationCount, 1)
		}
	}

	func testQuotaTabIsHiddenWhenReadingIsOffAndFallsBackToUsage() async throws {
		let base = WireFixture.envelope()
		let readingOff = base.replacingSubscription(
			DesktopSubscriptionSnapshotV1(available: true, reading: false, clients: [])
		)
		let host = StubDesktopHost(behavior: .envelope(readingOff))
		let model = await makeModel(host: host)
		await model.coordinator.refresh()

		XCTAssertFalse(model.panelTabs.contains { $0.id == .quota })
		XCTAssertEqual(model.presentedPanel, .usage)

		host.behavior = .envelope(base.replacingSubscription(
			DesktopSubscriptionSnapshotV1(available: true, reading: true, clients: [])
		))
		await model.coordinator.refresh()
		XCTAssertTrue(model.panelTabs.contains { $0.id == .quota })
		XCTAssertEqual(model.presentedPanel, .quota, "the remembered quota selection should return when reading is re-enabled")
	}

	func testSchemaHealthProseRendersExpandedAndCollapsed() async throws {
		let model = await makeModel(host: StubDesktopHost(behavior: .envelope(WireFixture.schemaSignal(refusals: true))))
		await model.coordinator.refresh()
		let source = try XCTUnwrap(model.healthDetail.rows.first)
		let directory = URL(fileURLWithPath: "/private/tmp/agentdeck-schema-presentation")
		try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
		for language in ["en", "zh-Hans"] {
			let bundle = try XCTUnwrap(Bundle(path: XCTUnwrap(Bundle.main.path(forResource: language, ofType: "lproj"))))
			let cause = String(format: bundle.localizedString(forKey: DesktopCopy.schemaSignalCause, value: nil, table: nil), arguments: [Int64(99), Int64(23)])
			let row = HealthCheckRow(id: source.id, name: source.name,
				status: bundle.localizedString(forKey: DesktopCopy.healthStatusFailed, value: nil, table: nil),
				severity: source.severity, recovery: source.recovery, code: source.code,
				count: source.count, supportedCount: source.supportedCount, cause: cause,
				recoveryProse: bundle.localizedString(forKey: DesktopCopy.schemaSignalRecovery, value: nil, table: nil))
			XCTAssertNil(row.recovery, "schema prose must not offer a command-copy button")
			let expanded = NSHostingView(rootView: HealthCheckRowView(row: row).frame(width: 396).padding(12))
			let collapsed = NSHostingView(rootView: HealthCheckRowView(row: row, initiallyExpanded: false).frame(width: 396).padding(12))
			XCTAssertGreaterThan(expanded.fittingSize.height, collapsed.fittingSize.height)
			for (name, expanded) in [("expanded", true), ("collapsed", false)] {
				let content = HealthCheckRowView(row: row, initiallyExpanded: expanded).padding(12)
					.foregroundStyle(Color.black).background(Color.white).environment(\.colorScheme, .light)
				let png = try renderedViewPNG(content, size: NSSize(width: 420, height: 150))
				try png.write(to: directory.appendingPathComponent("health-\(language)-\(name).png"))
				add(renderingAttachment(png, named: "Schema Health — \(language) — \(name)"))
			}
		}
	}

	func testHealthRecoveryActionFitsNarrowAndWidePopoverInBothLanguages() throws {
		let oldLocale = ProcessInfo.processInfo.environment["AGENTDECK_TEST_LOCALE"]
		defer {
			if let oldLocale { setenv("AGENTDECK_TEST_LOCALE", oldLocale, 1) }
			else { unsetenv("AGENTDECK_TEST_LOCALE") }
		}
		for language in ["en", "zh-Hans"] {
			setenv("AGENTDECK_TEST_LOCALE", language, 1)
			let row = HealthCheckRow(
				id: "extension.stale", name: t(DesktopCopy.healthExtensions),
				status: t(DesktopCopy.healthStatusWarning), severity: .warning,
				recovery: nil, code: "extension_stale_inventory", count: 2, supportedCount: nil,
				cause: t(DesktopCopy.healthCauseKeys["extension_stale_inventory"]!),
				recoveryProse: t(DesktopCopy.healthNextKeys["extension_stale_inventory"]!),
				reasonLabel: t(DesktopCopy.healthReasonKeys["extension_stale_inventory"]!),
				effect: t(DesktopCopy.healthEffectStale),
				actionLabel: t(DesktopCopy.healthCopySync),
				actionContent: "agentdeck extension scan"
			)
			for width in [280, 420] {
				let view = HealthCheckRowView(row: row)
					.frame(width: CGFloat(width))
					.foregroundStyle(Color.black)
					.background(Color.white)
					.environment(\.colorScheme, .light)
				let hosting = NSHostingView(rootView: view)
				hosting.frame = NSRect(x: 0, y: 0, width: CGFloat(width), height: 400)
				hosting.layoutSubtreeIfNeeded()
				XCTAssertLessThanOrEqual(hosting.fittingSize.width, CGFloat(width) + 1)
				let button = try XCTUnwrap(findHealthCopyButton(in: hosting, rowID: row.id))
				if width == 280 {
					XCTAssertGreaterThan(button.frame.width, 200, "the stacked copy target fills the narrow row")
				} else {
					XCTAssertLessThan(button.frame.width, 200, "the command and copy target share the wide row")
				}
				let png = try renderedViewPNG(hosting)
				XCTAssertGreaterThan(png.count, 2_000)
				add(renderingAttachment(png, named: "Health recovery — \(language) — \(width) pt"))
			}
		}
	}

	func testHealthRecoveryCopyRetainsNativeFocusAndAccessibleFeedback() async throws {
		let health: [String: Any] = [
			"available": true, "status": "warning", "healthy": false,
			"problems": 1, "warnings": 1, "errors": 0,
			"checks": [[
				"name": "extensions", "status": "warning", "resource": "extension_inventory",
				"reason": "extension_stale_inventory", "action_kind": "synchronize_inventory",
				"recovery_command": "agentdeck extension scan",
			]],
		]
		let model = await makeModel(host: StubDesktopHost(behavior: .envelope(WireFixture.envelope(health: health))))
		await model.coordinator.refresh()
		let row = try XCTUnwrap(model.healthDetail.rows.first)
		let hosting = NSHostingView(rootView: HealthCheckRowView(row: row, model: model).frame(width: 420))
		hosting.frame = NSRect(x: 0, y: 0, width: 420, height: 300)
		let window = NSWindow(contentRect: hosting.frame, styleMask: [.titled], backing: .buffered, defer: false)
		window.contentView = hosting
		window.makeKeyAndOrderFront(nil)
		Self.retainedFocusWindows.append(window)
		hosting.layoutSubtreeIfNeeded()
		let button = try XCTUnwrap(findHealthCopyButton(in: hosting, rowID: row.id))
		XCTAssertEqual(button.accessibilityLabel(), t(DesktopCopy.healthCopySync))
		XCTAssertTrue(window.makeFirstResponder(button))
		button.performClick(nil)
		try await Task.sleep(for: .milliseconds(20))
		hosting.layoutSubtreeIfNeeded()
		XCTAssertEqual(model.copiedHealthRowID, row.id)
		XCTAssertTrue(findHealthCopyButton(in: hosting, rowID: row.id) === button)
		XCTAssertTrue(window.firstResponder === button)
		XCTAssertEqual(button.title, t(DesktopCopy.healthCopied))
		XCTAssertEqual(button.accessibilityValue() as? String, t(DesktopCopy.healthCopied))
		try await Task.sleep(for: .milliseconds(1_700))
		hosting.layoutSubtreeIfNeeded()
		XCTAssertTrue(window.firstResponder === button)
		XCTAssertEqual(button.title, t(DesktopCopy.healthCopySync))
	}

	private func findHealthCopyButton(in view: NSView, rowID: String) -> NSButton? {
		if let button = view as? NSButton,
			button.identifier?.rawValue == "health.copy.\(rowID)",
			!button.isHidden, button.frame.width > 0
		{
			return button
		}
		for child in view.subviews {
			if let button = findHealthCopyButton(in: child, rowID: rowID) { return button }
		}
		return nil
	}

	func testPopoverHeightUsesTheStatusItemScreensVisibleFrame() {
		let shorterSecondaryDisplay = MenuBarGeometry.height(visibleFrameHeight: 600)
		let tallerMainDisplay = MenuBarGeometry.height(visibleFrameHeight: 1_200)

		XCTAssertEqual(shorterSecondaryDisplay, 528)
		XCTAssertEqual(tallerMainDisplay, MenuBarGeometry.maximumHeight)
		XCTAssertLessThan(shorterSecondaryDisplay, tallerMainDisplay)
	}

	func testTrendInteractionPrioritizesPinHoverAndKeyboardFocus() {
		let ids = ["00", "01", "02"]
		var interaction = TrendChartInteraction()

		XCTAssertNil(interaction.activeID(bucketIDs: ids, focused: false))
		interaction.setHover("01", inside: true)
		XCTAssertEqual(interaction.activeID(bucketIDs: ids, focused: false), "01")

		interaction.togglePin("02")
		interaction.setHover("01", inside: false)
		XCTAssertEqual(interaction.activeID(bucketIDs: ids, focused: false), "02")

		interaction.togglePin("02")
		interaction.move(by: 1, bucketCount: ids.count)
		XCTAssertEqual(interaction.activeID(bucketIDs: ids, focused: true), "01")
		interaction.move(by: 20, bucketCount: ids.count)
		XCTAssertEqual(interaction.activeID(bucketIDs: ids, focused: true), "02")
		XCTAssertEqual(TrendChartInteraction.heightFraction(magnitude: 0.48, maximum: 0.48), 1)
		XCTAssertEqual(TrendChartInteraction.heightFraction(magnitude: 0.24, maximum: 0.48), 0.5)
	}

	func testHourlyAxisRejectsPartialOrNonHourlyBucketIdentities() {
		XCTAssertNil(TrendChartInteraction.hourlyAxis(bucketIDs: ["hour.0", "hour.2"]))
		XCTAssertNil(TrendChartInteraction.hourlyAxis(bucketIDs: ["2026-08-20"]))
		XCTAssertEqual(
			TrendChartInteraction.hourlyAxis(bucketIDs: (0 ..< 24).map { "hour.\($0)" }),
			TrendChartAxis(ticks: ["00", "06", "12", "18", "24"])
		)
	}

	func testBreakdownPaletteFollowsPrototypeIdentityAndTokenRoles() {
		XCTAssertEqual(BreakdownPalette.modelTone(label: "gpt-5.6-sol", fallbackIndex: 3), .series(0))
		XCTAssertEqual(BreakdownPalette.modelTone(label: "claude-opus-5", fallbackIndex: 0), .series(1))
		XCTAssertEqual(BreakdownPalette.modelTone(label: "codex-auto-review", fallbackIndex: 0), .series(2))
		XCTAssertEqual(BreakdownPalette.modelTone(label: "gpt-5.5", fallbackIndex: 0), .series(3))
		XCTAssertEqual(BreakdownPalette.tokenTone(id: "input"), .series(0))
		XCTAssertEqual(BreakdownPalette.tokenTone(id: "output"), .series(1))
		XCTAssertEqual(BreakdownPalette.tokenTone(id: "cache-read"), .series(2))
		XCTAssertEqual(BreakdownPalette.tokenTone(id: "cache-write"), .warning)
	}

	func testBreakdownPanelViewShowsDistinguishableModelClientLabelsAndAccessibility() async throws {
		let longModel = "gpt-6-luna-enterprise-reasoning-preview-2026-11-17-long-name-variant"
		let allScope = WireFixture.scope(
			client: "all",
			tokensByPeriod: ["today": 1_000, "7d": 1_000, "30d": 1_000],
			modelsByPeriod: [
				"today": [
					.init(client: "codex", model: longModel, tokens: 600, share: "60.00"),
					.init(client: "claude", model: longModel, tokens: 400, share: "40.00"),
				],
			]
		)
		let codexScope = WireFixture.scope(
			client: "codex",
			tokensByPeriod: ["today": 600, "7d": 600, "30d": 600],
			modelsByPeriod: [
				"today": [.init(client: nil, model: longModel, tokens: 600, share: "100.00")],
			]
		)
		let claudeScope = WireFixture.scope(
			client: "claude",
			tokensByPeriod: ["today": 400, "7d": 400, "30d": 400],
			modelsByPeriod: [
				"today": [.init(client: nil, model: longModel, tokens: 400, share: "100.00")],
			]
		)
		let host = StubDesktopHost(behavior: .envelope(WireFixture.envelope(scopes: [allScope, codexScope, claudeScope])))
		let model = await makeModel(host: host)
		await model.coordinator.refresh()

		func ocrLines(for view: some View, width: CGFloat) throws -> [String] {
			let size = NSSize(width: width, height: 340)
			let hosting = NSHostingView(rootView: view.frame(width: width).background(Color.black))
			hosting.frame = NSRect(origin: .zero, size: size)
			hosting.layoutSubtreeIfNeeded()
			hosting.displayIfNeeded()
			let scale = 2
			let bitmap = try XCTUnwrap(NSBitmapImageRep(
				bitmapDataPlanes: nil,
				pixelsWide: Int(size.width) * scale,
				pixelsHigh: Int(size.height) * scale,
				bitsPerSample: 8,
				samplesPerPixel: 4,
				hasAlpha: true,
				isPlanar: false,
				colorSpaceName: .deviceRGB,
				bytesPerRow: 0,
				bitsPerPixel: 0
			))
			bitmap.size = size
			hosting.cacheDisplay(in: hosting.bounds, to: bitmap)
			let cgImage = try XCTUnwrap(bitmap.cgImage)
			let request = VNRecognizeTextRequest()
			request.recognitionLevel = .accurate
			request.usesLanguageCorrection = false
			let handler = VNImageRequestHandler(cgImage: cgImage, options: [:])
			try handler.perform([request])
			return (request.results ?? []).compactMap { $0.topCandidates(1).first?.string }
		}

		model.selectedClient = "all"
		model.selectedPeriod = "today"
		let allPanel = BreakdownPanelModel(
			available: model.breakdownPanel.available,
			models: model.breakdownPanel.models,
			modelsEmptyCopy: model.breakdownPanel.modelsEmptyCopy,
			tokenMix: model.breakdownPanel.tokenMix,
			clientRows: []
		)
		let allLines = try ocrLines(
			for: BreakdownPanelView(panel: allPanel)
				.environment(\.colorScheme, .dark)
				.environment(\.dynamicTypeSize, .accessibility3),
			width: 280
		)
		XCTAssertTrue(
			allLines.contains { $0.contains("gpt-6-lu") && ($0.contains("variant") || $0.contains("varlant")) && $0.contains("Codex") },
			"expected Codex-tagged model row with client suffix preserved in \(allLines)"
		)
		XCTAssertTrue(
			allLines.contains { $0.contains("gpt-6-lu") && ($0.contains("variant") || $0.contains("varlant")) && $0.contains("Claude") },
			"expected Claude-tagged model row with client suffix preserved in \(allLines)"
		)

		model.selectedClient = "codex"
		let codexPanel = BreakdownPanelModel(
			available: model.breakdownPanel.available,
			models: model.breakdownPanel.models,
			modelsEmptyCopy: model.breakdownPanel.modelsEmptyCopy,
			tokenMix: model.breakdownPanel.tokenMix,
			clientRows: []
		)
		let codexLines = try ocrLines(
			for: BreakdownPanelView(panel: codexPanel).environment(\.colorScheme, .dark),
			width: 280
		)
		XCTAssertTrue(codexLines.contains { $0.contains("gpt-6-luna") && !$0.contains("Codex") }, "single-client should omit suffix: \(codexLines)")
	}

	func testRhythmHoverClearsOnlyTheCellThatActuallyExited() {
		var hover = RhythmHoverState()
		hover.setHour("tue.09", inside: true)
		XCTAssertEqual(hover.hourCellID, "tue.09")
		hover.setHour("mon.08", inside: false)
		XCTAssertEqual(hover.hourCellID, "tue.09")
		hover.setHour("tue.09", inside: false)
		XCTAssertNil(hover.hourCellID)

		hover.setCalendar("calendar.2026-08-20", inside: true)
		XCTAssertEqual(hover.calendarBucketID, "calendar.2026-08-20")
		hover.setCalendar("calendar.2026-08-20", inside: false)
		XCTAssertNil(hover.calendarBucketID)
	}

	func testWorkSignalNavigationKeepsOneExpandedCategoryAndReturnsToItsOpeningCard() {
		var navigation = WorkSignalNavigationState()
		navigation.open(.activity)
		XCTAssertEqual(navigation.detail, .activity)

		navigation.setActivity("coding", expanded: true)
		XCTAssertEqual(navigation.expandedActivityKind, "coding")
		navigation.setActivity("debugging", expanded: true)
		XCTAssertEqual(navigation.expandedActivityKind, "debugging")
		navigation.setActivity("debugging", expanded: false)
		XCTAssertNil(navigation.expandedActivityKind)

		XCTAssertEqual(navigation.close(), .activity)
		XCTAssertNil(navigation.detail)
	}

	func testWorkSignalFormattingKeepsMeasuredZeroDistinctFromUnavailable() {
		XCTAssertEqual(DesktopFormat.workSignalCount(nil), "—")
		XCTAssertNotEqual(DesktopFormat.workSignalCount(0), "—")
		XCTAssertEqual(DesktopFormat.workSignalDecimal(nil), "—")
		XCTAssertNotEqual(DesktopFormat.workSignalDecimal(0), "—")
		XCTAssertEqual(DesktopFormat.workSignalDuration(nil), "—")
		XCTAssertNotEqual(DesktopFormat.workSignalDuration(0), "—")
		XCTAssertEqual(DesktopFormat.workSignalPercent(nil), "—")
		XCTAssertEqual(DesktopFormat.workSignalCost(nil), "—")
	}

	func testStatusItemGlyphRendersNormalAndBadgedAcceptanceImages() throws {
		XCTAssertNotNil(MenuBarItemController.glyph(badged: false), "the built app must load its raw status-item resource")
		let source = try XCTUnwrap(NSImage(contentsOf: menuBarIconURL))
		let normal = try XCTUnwrap(MenuBarItemController.glyph(badged: false, base: source))
		let badged = try XCTUnwrap(MenuBarItemController.glyph(badged: true, base: source))
		let normalRendering = try renderAt2x(normal)
		let badgedRendering = try renderAt2x(badged)

		XCTAssertEqual(normal.size, NSSize(width: 18, height: 18))
		XCTAssertEqual(badged.size, normal.size)
		XCTAssertGreaterThanOrEqual(alphaBounds(in: normalRendering.bitmap).width, 30)
		XCTAssertGreaterThanOrEqual(
			alphaBounds(in: normalRendering.bitmap).height,
			28,
			"the approved AD template must retain its visible silhouette at 18pt"
		)
		XCTAssertNotEqual(normalRendering.png, badgedRendering.png)
		XCTAssertGreaterThanOrEqual(
			clearedAlphaPixelCount(from: normalRendering.bitmap, to: badgedRendering.bitmap),
			24,
			"the badge needs a transparent halo so its template silhouette remains visible over the base mark"
		)

		add(renderingAttachment(normalRendering.png, named: "AgentDeck status item — normal @2x"))
		add(renderingAttachment(badgedRendering.png, named: "AgentDeck status item — badged @2x"))
	}

	private var menuBarIconURL: URL {
		URL(fileURLWithPath: #filePath)
			.deletingLastPathComponent()
			.deletingLastPathComponent()
			.appendingPathComponent("AgentDeckApp/Assets.xcassets/AgentDeckMenuBarIcon.imageset/AgentDeckMenuBarIcon@2x.png")
	}

	func testStandardAboutPanelMetadataAndApplicationIconArePresent() throws {
		let info = try XCTUnwrap(Bundle.main.infoDictionary)
		XCTAssertEqual(info["CFBundleDisplayName"] as? String, "AgentDeck")
		XCTAssertEqual(info["CFBundleIdentifier"] as? String, Bundle.main.bundleIdentifier)
		XCTAssertTrue(Bundle.main.bundleIdentifier?.hasPrefix("com.kitdine.agentdeck") == true)
		XCTAssertFalse(try XCTUnwrap(info["CFBundleShortVersionString"] as? String).isEmpty)
		XCTAssertFalse(try XCTUnwrap(info["CFBundleVersion"] as? String).isEmpty)
		XCTAssertFalse(try XCTUnwrap(info["NSHumanReadableCopyright"] as? String).isEmpty)
		XCTAssertEqual(info["CFBundleIconName"] as? String, "AppIcon")
		XCTAssertNotNil(NSImage(named: NSImage.applicationIconName))

		let existingWindows = Set(NSApp.windows.map(ObjectIdentifier.init))
		let options = MenuBarItemController.aboutPanelOptions()
		XCTAssertNotNil(options[.applicationIcon] as? NSImage)
		NSApp.orderFrontStandardAboutPanel(options: options)
		let about = try XCTUnwrap(NSApp.windows.first { !existingWindows.contains(ObjectIdentifier($0)) })
		defer { about.close() }
		XCTAssertTrue(about.isVisible)
		let content = try XCTUnwrap(about.contentView)
		let aboutPNG = try renderedViewPNG(content)
		XCTAssertGreaterThan(aboutPNG.count, 5_000)
		add(renderingAttachment(aboutPNG, named: "AgentDeck About — built bundle candidate @2x"))
	}

	func testWorkSignalNarrowCapturedAndLegacyDetailRenderingsAreAttached() async throws {
		let capturedHost = StubDesktopHost(behavior: .envelope(WireFixture.envelope()))
		let capturedModel = await makeModel(host: capturedHost)
		await capturedModel.coordinator.refresh()
		let legacyHost = StubDesktopHost(behavior: .envelope(WireFixture.envelope(includeWorkSignals: false)))
		let legacyModel = await makeModel(host: legacyHost)
		await legacyModel.coordinator.refresh()

		let summaryDark = SessionsPanelView(panel: capturedModel.sessionsPanel)
			.environment(\.colorScheme, .dark)
			.tint(DesktopVisualTheme.accent)
			.foregroundStyle(DesktopVisualTheme.text)
			.background(DesktopVisualTheme.background)
		let activityLight = SessionsPanelView(
			panel: capturedModel.sessionsPanel,
			initialNavigation: WorkSignalNavigationState(detail: .activity, expandedActivityKind: "coding")
		)
		.environment(\.colorScheme, .light)
		.tint(DesktopVisualTheme.accent)
		.foregroundStyle(DesktopVisualTheme.text)
		.background(DesktopVisualTheme.background)
		let activityDark = SessionsPanelView(
			panel: capturedModel.sessionsPanel,
			initialNavigation: WorkSignalNavigationState(detail: .activity, expandedActivityKind: "coding")
		)
		.environment(\.colorScheme, .dark)
		.tint(DesktopVisualTheme.accent)
		.foregroundStyle(DesktopVisualTheme.text)
		.background(DesktopVisualTheme.background)
		let workflowLight = SessionsPanelView(
			panel: capturedModel.sessionsPanel,
			initialNavigation: WorkSignalNavigationState(detail: .workflow)
		)
		.environment(\.colorScheme, .light)
		.tint(DesktopVisualTheme.accent)
		.foregroundStyle(DesktopVisualTheme.text)
		.background(DesktopVisualTheme.background)
		let workflowDark = SessionsPanelView(
			panel: capturedModel.sessionsPanel,
			initialNavigation: WorkSignalNavigationState(detail: .workflow)
		)
		.environment(\.colorScheme, .dark)
		.tint(DesktopVisualTheme.accent)
		.foregroundStyle(DesktopVisualTheme.text)
		.background(DesktopVisualTheme.background)
		let toolingLight = SessionsPanelView(
			panel: capturedModel.sessionsPanel,
			initialNavigation: WorkSignalNavigationState(detail: .tooling)
		)
		.environment(\.colorScheme, .light)
		.tint(DesktopVisualTheme.accent)
		.foregroundStyle(DesktopVisualTheme.text)
		.background(DesktopVisualTheme.background)
		let toolingDark = SessionsPanelView(
			panel: capturedModel.sessionsPanel,
			initialNavigation: WorkSignalNavigationState(detail: .tooling)
		)
		.environment(\.colorScheme, .dark)
		.tint(DesktopVisualTheme.accent)
		.foregroundStyle(DesktopVisualTheme.text)
		.background(DesktopVisualTheme.background)
		let legacyLight = SessionsPanelView(
			panel: legacyModel.sessionsPanel,
			initialNavigation: WorkSignalNavigationState(detail: .activity)
		)
		.environment(\.colorScheme, .light)
		.tint(DesktopVisualTheme.accent)
		.foregroundStyle(DesktopVisualTheme.text)
		.background(DesktopVisualTheme.background)

		let renderings: [(String, Data)] = try [
			("summary-dark", renderedViewPNG(summaryDark, size: NSSize(width: 256, height: 620))),
			("activity-light", renderedViewPNG(activityLight, size: NSSize(width: 256, height: 620))),
			("activity-dark", renderedViewPNG(activityDark, size: NSSize(width: 256, height: 620))),
			("workflow-light", renderedViewPNG(workflowLight, size: NSSize(width: 256, height: 420))),
			("workflow-dark", renderedViewPNG(workflowDark, size: NSSize(width: 256, height: 420))),
			("tooling-light", renderedViewPNG(toolingLight, size: NSSize(width: 256, height: 420))),
			("tooling-dark", renderedViewPNG(toolingDark, size: NSSize(width: 256, height: 420))),
			("legacy-light", renderedViewPNG(legacyLight, size: NSSize(width: 256, height: 220))),
		]
		for (name, png) in renderings {
			XCTAssertGreaterThan(png.count, 4_000, "\(name) should contain rendered surface content")
			add(renderingAttachment(png, named: "Work signals narrow — \(name) — \(DesktopLocale.current.identifier) @2x"))
		}
	}

	/// Native renderings of the final session-scope surface: every state that
	/// changes layout, at the regular and narrow Popover widths, in both
	/// themes. Each must lay out real content, and each is attached for review.
	func testSessionScopeStatesRenderAtBothWidthsAndThemes() async throws {
		let override: [String: Any] = ["value": "work", "binding_status": "matched", "binding_limited": false, "applied": true]
		let rows = [
			WireFixture.scopeRecent(id: "task-01", project: "agent-deck", effective: "work", override: override, revision: 2),
			WireFixture.scopeRecent(id: "child-1", project: "a-project-name-long-enough-to-need-truncation", reason: "task_child_unconfirmed"),
			WireFixture.scopeRecent(id: "old-01", project: "research", first: NSNull(), last: NSNull()),
		]
		let ready = await makeModel(host: StubDesktopHost(behavior: .envelope(WireFixture.envelope(sessionScope: .custom(
			WireFixture.sessionScopeView(sessions: 3, projects: 2, knownDurationMS: 2_700_000, missing: 1, unknownCount: 2, recent: rows))))))
		await ready.coordinator.refresh()
		let views = WireFixture.fixtureScopeViews()
		let failingReader = StubSessionScopeReader { $0.scope == .auxiliary ? .failure(.nonZeroExit(1)) : .view(views[$0]!) }
		let failed = await makeModel(host: StubDesktopHost(behavior: .envelope(WireFixture.envelope())), sessionScopeReader: failingReader)
		await failed.coordinator.refresh()
		failed.beginSessionScopePresentation()
		failed.chooseSessionScope(.auxiliary)
		await failed.synchronizeSessionScope()
		let legacy = await makeModel(host: StubDesktopHost(behavior: .envelope(WireFixture.envelope(sessionScope: .absent))))
		await legacy.coordinator.refresh()
		let unconfirmed = await makeModel(host: StubDesktopHost(behavior: .envelope(WireFixture.envelope(sessionScope: .custom(
			WireFixture.unavailableSessionScopeView(reason: "default_unconfirmed", defaults: WireFixture.unconfirmedDefault, selection: nil))))))
		await unconfirmed.coordinator.refresh()
		XCTAssertEqual(ready.sessionsPanel.scope.state, .ready)
		XCTAssertNotNil(failed.sessionsPanel.scope.retryLabel)
		XCTAssertEqual(legacy.sessionsPanel.scope.state, .legacy)
		XCTAssertEqual(unconfirmed.sessionsPanel.scope.state, .defaultUnknown)

		let states: [(String, SessionsPanelModel, SessionScopeRow?, CGFloat, Int)] = [
			("ready", ready.sessionsPanel, nil, 900, 12_000),
			("detail", ready.sessionsPanel, ready.sessionsPanel.scope.rows.first, 760, 10_000),
			("failed", failed.sessionsPanel, nil, 900, 10_000),
			("legacy", legacy.sessionsPanel, nil, 200, 3_000),
			("default-unknown", unconfirmed.sessionsPanel, nil, 200, 3_000),
		]
		for (name, panel, detail, height, minimum) in states {
			for width in [CGFloat(280), 420] {
				for scheme in [ColorScheme.light, .dark] {
					let view = SessionsPanelView(panel: panel, initialOriginDetail: detail)
						.padding(MenuBarGeometry.padding)
						.environment(\.colorScheme, scheme)
						.tint(DesktopVisualTheme.accent)
						.foregroundStyle(DesktopVisualTheme.text)
						.background(DesktopVisualTheme.background)
					let png = try renderedViewPNG(view, size: NSSize(width: width, height: height))
					XCTAssertGreaterThan(png.count, minimum, "\(name) \(Int(width)) \(scheme)")
					add(renderingAttachment(png, named: "Session scope — \(name) — \(Int(width))pt — \(scheme) @2x"))
				}
			}
		}
	}

	/// ASC-DM-R3-F1: the scope note, now naming both data domains, stays within
	/// the native widths in both shipped languages. The selection lists no
	/// session yet carries work signals — the state the note explains.
	func testClientFiltersKeepEveryTargetInsideTheAvailableWidth() async throws {
		for width: CGFloat in [280, 420] {
			let available = width - 2 * MenuBarGeometry.padding
			for language in ["en", "zh-Hans"] {
				for largeValues in [false, true] {
					for largeType in [false, true] {
						let values = largeValues ? ["≈US$12,345.67", "≈US$6,234.56", "≈US$6,111.11"]
							: language == "en" ? ["$1.44", "$1.44", "$0.10"] : ["US$1.44", "US$1.44", "US$0.10"]
						let options = [
							FilterOption(id: "all", label: language == "en" ? "All" : "全部", value: values[0]),
							FilterOption(id: "codex", label: "Codex", value: values[1]),
							FilterOption(id: "claude", label: "Claude", value: values[2]),
						]
						let capture = ClientFilterBoundsCapture()
						let root = MenuBarClientFilters(options: options, selectedID: "all", select: { _ in })
							.environment(\.dynamicTypeSize, largeType ? .accessibility3 : .large)
							.frame(width: available)
							.onPreferenceChange(ClientFilterFramesPreferenceKey.self) { capture.update($0) }
						let hosting = NSHostingView(rootView: root)
						hosting.frame = NSRect(x: 0, y: 0, width: available, height: 240)
						hosting.layoutSubtreeIfNeeded()
						_ = try renderedViewPNG(hosting)
						let clock = ContinuousClock()
						let deadline = clock.now.advanced(by: .seconds(2))
						while capture.frames.count < options.count, clock.now < deadline {
							try await Task.sleep(for: .milliseconds(5))
							hosting.layoutSubtreeIfNeeded()
						}
						let label = "\(language), \(width)pt, large values \(largeValues), large type \(largeType)"
						let frames = capture.frames
						XCTAssertEqual(Set(frames.keys), Set(options.map(\.id)), label)
						for option in options {
							guard let frame = frames[option.id] else { continue }
							XCTAssertGreaterThanOrEqual(frame.minX, -0.5, "\(option.id): \(label)")
							XCTAssertLessThanOrEqual(frame.maxX, available + 0.5, "\(option.id): \(label)")
							XCTAssertGreaterThan(frame.width, 0, label)
							XCTAssertGreaterThanOrEqual(frame.height, MenuBarGeometry.rowMinimumHeight, label)
						}
					}
				}
			}
		}
	}

	func testSessionScopeNoteFitsNativeWidthsInBothLanguages() async throws {
		let oldWidth = ProcessInfo.processInfo.environment["AGENTDECK_TEST_WIDTH"]
		let oldLocale = ProcessInfo.processInfo.environment["AGENTDECK_TEST_LOCALE"]
		defer {
			if let oldWidth { setenv("AGENTDECK_TEST_WIDTH", oldWidth, 1) } else { unsetenv("AGENTDECK_TEST_WIDTH") }
			if let oldLocale { setenv("AGENTDECK_TEST_LOCALE", oldLocale, 1) } else { unsetenv("AGENTDECK_TEST_LOCALE") }
		}
		let view = WireFixture.sessionScopeView(sessions: 0, projects: 0, knownDurationMS: 0, unknownCount: 0, recent: [])
		for language in ["en", "zh-Hans"] {
			setenv("AGENTDECK_TEST_LOCALE", language, 1)
			for width in [280, 420] {
				setenv("AGENTDECK_TEST_WIDTH", String(width), 1)
				let model = await makeModel(host: StubDesktopHost(behavior: .envelope(WireFixture.envelope(sessionScope: .custom(view)))))
				await model.coordinator.refresh()
				model.selectedPanel = .sessions
				XCTAssertEqual(model.sessionsPanel.scope.state, .ready, language)
				XCTAssertTrue(model.sessionsPanel.scope.rows.isEmpty, language)
				XCTAssertEqual(model.sessionsPanel.workSignals.state, .captured, language)
				let hosting = NSHostingView(rootView: MenuBarSurfaceView(model: model))
				hosting.frame = NSRect(x: 0, y: 0, width: CGFloat(width), height: 900)
				hosting.layoutSubtreeIfNeeded()
				hosting.displayIfNeeded()
				XCTAssertLessThanOrEqual(hosting.fittingSize.width, CGFloat(width) + 1, "\(language) \(width)")
				let png = try renderedViewPNG(hosting)
				XCTAssertGreaterThan(png.count, 8_000, "\(language) \(width)")
				add(renderingAttachment(png, named: "Session scope note — \(language) — \(width)pt"))
			}
		}
	}

	func testApprovedDarkPopoverAndSettingsRenderingsAreAttached() async throws {
		let host = StubDesktopHost(behavior: .envelope(WireFixture.envelope(health: WireFixture.warningHealth)))
		let model = await makeModel(host: host)
		await model.coordinator.refresh()
		let multiTargetHost = StubDesktopHost(
			behavior: .envelope(WireFixture.envelope(candidates: [WireFixture.multiTargetCandidate]))
		)
		let multiTargetModel = await makeModel(host: multiTargetHost)
		await multiTargetModel.coordinator.refresh()

		let popover = MenuBarSurfaceView(model: model, height: MenuBarGeometry.maximumHeight)
			.environment(\.colorScheme, .dark)
		let settings = SettingsPaneView(pane: .general, preferences: model.preferences, quotaSettings: makeQuotaSettingsController(preferences: model.preferences), telemetrySettings: makeTelemetrySettingsController(), scrolls: false)
			.environment(\.colorScheme, .dark)
		let providers = ProviderMenuView(model: multiTargetModel, dismiss: {})
			.environment(\.colorScheme, .dark)
		let providerTargets = ProviderMenuView(
			model: multiTargetModel,
			dismiss: {},
			selectedRowID: "codex:aigocode"
		)
			.environment(\.colorScheme, .dark)
		let hoveredRhythmCell = try XCTUnwrap(model.rhythmBlock.cells.max { $0.tokens < $1.tokens })
		let hoveredCalendarBucket = try XCTUnwrap(model.rhythmBlock.calendar.first)
		let rhythmHover = RhythmBlockView(
			block: model.rhythmBlock,
			initialHover: RhythmHoverState(
				hourCellID: hoveredRhythmCell.id,
				calendarBucketID: hoveredCalendarBucket.id
			)
		)
		.environment(\.colorScheme, .dark)
		let popoverPNG = try renderedViewPNG(popover, size: NSSize(width: 420, height: 760))
		let rhythmHoverPNG = try renderedViewPNG(rhythmHover, size: NSSize(width: 396, height: 520))
		model.selectedPanel = .breakdown
		let breakdownPNG = try renderedViewPNG(
			MenuBarSurfaceView(model: model, height: MenuBarGeometry.maximumHeight).environment(\.colorScheme, .dark),
			size: NSSize(width: 420, height: 760)
		)
		model.selectedPanel = .sessions
		let sessionsPNG = try renderedViewPNG(
			MenuBarSurfaceView(model: model, height: MenuBarGeometry.maximumHeight).environment(\.colorScheme, .dark),
			size: NSSize(width: 420, height: 760)
		)
		model.selectedPanel = .usage
		let settingsPNG = try renderedViewPNG(settings, size: NSSize(width: 460, height: 400))
		let providersPNG = try renderedViewPNG(providers, size: NSSize(width: 250, height: 260))
		let providerTargetsPNG = try renderedViewPNG(providerTargets, size: NSSize(width: 250, height: 260))

		XCTAssertGreaterThan(popoverPNG.count, 10_000)
		XCTAssertGreaterThan(rhythmHoverPNG.count, 8_000)
		XCTAssertGreaterThan(breakdownPNG.count, 10_000)
		XCTAssertGreaterThan(sessionsPNG.count, 10_000)
		XCTAssertGreaterThan(settingsPNG.count, 5_000)
		XCTAssertGreaterThan(providersPNG.count, 3_000)
		XCTAssertGreaterThan(providerTargetsPNG.count, 2_000)
		add(renderingAttachment(popoverPNG, named: "AgentDeck popover — approved dark candidate @2x"))
		add(renderingAttachment(rhythmHoverPNG, named: "AgentDeck rhythm — visible hover readout @2x"))
		add(renderingAttachment(breakdownPNG, named: "AgentDeck breakdown — prototype palette candidate @2x"))
		add(renderingAttachment(sessionsPNG, named: "AgentDeck sessions — producer duration candidate @2x"))
		add(renderingAttachment(settingsPNG, named: "AgentDeck settings — approved dark candidate @2x"))
		add(renderingAttachment(providersPNG, named: "AgentDeck providers — bounded grouped candidate @2x"))
		add(renderingAttachment(providerTargetsPNG, named: "AgentDeck provider targets — bounded second level @2x"))
	}

	func testLightAttributionAndSessionsRenderingsAreAttached() async throws {
		let host = StubDesktopHost(behavior: .envelope(WireFixture.envelope(health: WireFixture.warningHealth)))
		let model = await makeModel(host: host)
		await model.coordinator.refresh()

		model.selectedPanel = .attribution
		let attributionPNG = try renderedViewPNG(
			MenuBarSurfaceView(model: model, height: MenuBarGeometry.maximumHeight).environment(\.colorScheme, .light),
			size: NSSize(width: 420, height: 760)
		)
		model.selectedPanel = .sessions
		let sessionsPNG = try renderedViewPNG(
			MenuBarSurfaceView(model: model, height: MenuBarGeometry.maximumHeight).environment(\.colorScheme, .light),
			size: NSSize(width: 420, height: 760)
		)

		XCTAssertGreaterThan(attributionPNG.count, 10_000)
		XCTAssertGreaterThan(sessionsPNG.count, 10_000)
		add(renderingAttachment(attributionPNG, named: "AgentDeck attribution — light visual-contract candidate @2x"))
		add(renderingAttachment(sessionsPNG, named: "AgentDeck sessions — light visual-contract candidate @2x"))
	}

	private func renderedViewPNG<Content: View>(_ content: Content, size: NSSize) throws -> Data {
		let hosting = NSHostingView(rootView: content)
		hosting.frame = NSRect(origin: .zero, size: size)
		return try renderedViewPNG(hosting)
	}

	private func renderedViewPNG(_ view: NSView) throws -> Data {
		view.layoutSubtreeIfNeeded()
		view.displayIfNeeded()
		let representation = try XCTUnwrap(view.bitmapImageRepForCachingDisplay(in: view.bounds))
		view.cacheDisplay(in: view.bounds, to: representation)
		return try XCTUnwrap(representation.representation(using: .png, properties: [:]))
	}

	private func renderAt2x(_ image: NSImage) throws -> (bitmap: NSBitmapImageRep, png: Data) {
		let width = Int(image.size.width * 2)
		let height = Int(image.size.height * 2)
		let bitmap = try XCTUnwrap(NSBitmapImageRep(
			bitmapDataPlanes: nil,
			pixelsWide: width,
			pixelsHigh: height,
			bitsPerSample: 8,
			samplesPerPixel: 4,
			hasAlpha: true,
			isPlanar: false,
			colorSpaceName: .deviceRGB,
			bytesPerRow: 0,
			bitsPerPixel: 0
		))
		bitmap.size = image.size
		let context = try XCTUnwrap(NSGraphicsContext(bitmapImageRep: bitmap))
		NSGraphicsContext.saveGraphicsState()
		NSGraphicsContext.current = context
		context.imageInterpolation = .high
		image.draw(in: NSRect(origin: .zero, size: image.size))
		NSGraphicsContext.restoreGraphicsState()

		let png = try XCTUnwrap(bitmap.representation(using: .png, properties: [:]))
		return (bitmap, png)
	}

	private func alphaBounds(in bitmap: NSBitmapImageRep) -> NSRect {
		var minimumX = bitmap.pixelsWide
		var minimumY = bitmap.pixelsHigh
		var maximumX = -1
		var maximumY = -1
		for y in 0 ..< bitmap.pixelsHigh {
			for x in 0 ..< bitmap.pixelsWide {
				guard (bitmap.colorAt(x: x, y: y)?.alphaComponent ?? 0) > 0.1 else { continue }
				minimumX = min(minimumX, x)
				minimumY = min(minimumY, y)
				maximumX = max(maximumX, x)
				maximumY = max(maximumY, y)
			}
		}
		guard maximumX >= minimumX, maximumY >= minimumY else { return .zero }
		return NSRect(
			x: minimumX,
			y: minimumY,
			width: maximumX - minimumX + 1,
			height: maximumY - minimumY + 1
		)
	}

	private func clearedAlphaPixelCount(from normal: NSBitmapImageRep, to badged: NSBitmapImageRep) -> Int {
		var count = 0
		for y in 0 ..< normal.pixelsHigh {
			for x in 0 ..< normal.pixelsWide {
				let normalAlpha = normal.colorAt(x: x, y: y)?.alphaComponent ?? 0
				let badgedAlpha = badged.colorAt(x: x, y: y)?.alphaComponent ?? 0
				if normalAlpha > 0.5, badgedAlpha < 0.1 {
					count += 1
				}
			}
		}
		return count
	}

	private func renderingAttachment(_ data: Data, named name: String) -> XCTAttachment {
		let attachment = XCTAttachment(data: data, uniformTypeIdentifier: "public.png")
		attachment.name = name
		attachment.lifetime = .keepAlways
		return attachment
	}

	// Codex PR #5 P1: a client with retained windows after a probe stopped
	// succeeding rendered only its source, dropping observed_at and stale
	// entirely -- ux/menubar-quota.md's header requires both ("freshness is
	// never implied").
	func testQuotaCardHeaderShowsAgeAndStaleMarkerBesideRetainedWindows() throws {
		func client(observedAt: String?, stale: Bool) throws -> DesktopSubscriptionClientV1 {
			let json: [String: Any] = [
				"client": "claude",
				"applicable": true,
				"source": "claude_statusline",
				"observed_at": observedAt as Any,
				"stale": stale,
				"attribution_confirmed": true,
				"windows": [],
			]
			let data = try JSONSerialization.data(withJSONObject: json)
			return try JSONDecoder().decode(DesktopSubscriptionClientV1.self, from: data)
		}

		let panel = QuotaPanelView(clients: [])
		let statusLineLabel = t(DesktopCopy.quotaSourceClaudeStatusLine)
		let fresh = try client(observedAt: "2026-09-10T09:58:00Z", stale: false)
		let freshCaption = panel.headerCaption(fresh, source: .claudeStatusLine)
		XCTAssertTrue(freshCaption.hasPrefix(statusLineLabel + " · "), "caption = \(freshCaption)")
		XCTAssertFalse(freshCaption.contains(t(DesktopCopy.quotaStale)), "caption = \(freshCaption)")

		let stale = try client(observedAt: "2026-09-10T08:00:00Z", stale: true)
		let staleCaption = panel.headerCaption(stale, source: .claudeStatusLine)
		XCTAssertTrue(staleCaption.contains(t(DesktopCopy.quotaStale)), "caption = \(staleCaption)")
		XCTAssertNotEqual(staleCaption, statusLineLabel, "must not collapse back to source alone once stale")

		let never = try client(observedAt: nil, stale: false)
		let neverCaption = panel.headerCaption(never, source: .claudeStatusLine)
		XCTAssertEqual(neverCaption, statusLineLabel, "no observed_at means no age clause to show")
	}

	// Codex PR #5 second review, P2: a Codex limit's primary and secondary
	// windows share the same vendor label, so the label alone cannot tell a
	// 5-hour row from a 7-day row for the same limit.
	func testQuotaWindowLabelAppendsTheSpanRatherThanReplacingItWithTheVendorLabel() throws {
		let panel = QuotaPanelView(clients: [])
		let labeled = try JSONDecoder().decode(
			DesktopSubscriptionWindowV1.self,
			from: JSONSerialization.data(withJSONObject: [
				"key": "codex", "label": "GPT-5.3-Codex-Spark", "window_minutes": 300,
				"window_minutes_reason": NSNull(), "used_percent": 12, "resets_at": NSNull(),
			])
		)
		XCTAssertEqual(panel.windowLabel(labeled), "GPT-5.3-Codex-Spark · " + t(DesktopCopy.quotaWindow5h))

		let unlabeled = try JSONDecoder().decode(
			DesktopSubscriptionWindowV1.self,
			from: JSONSerialization.data(withJSONObject: [
				"key": "five_hour", "label": NSNull(), "window_minutes": 300,
				"window_minutes_reason": NSNull(), "used_percent": 22, "resets_at": NSNull(),
			])
		)
		XCTAssertEqual(panel.windowLabel(unlabeled), t(DesktopCopy.quotaWindow5h))
	}

	// Codex PR #5 seventh review, P2: a partial status-line refresh can
	// leave one window older and prose-derived while the card header names
	// only the client's newest source/age; the row itself must carry each
	// window's own age and source instead of implying it shares the
	// header's freshness and provenance.
	func testWindowProvenanceCaptionCarriesEachWindowsOwnAgeAndSource() throws {
		let panel = QuotaPanelView(clients: [])
		func window(source: String?, observedAt: String?) throws -> DesktopSubscriptionWindowV1 {
			try JSONDecoder().decode(
				DesktopSubscriptionWindowV1.self,
				from: JSONSerialization.data(withJSONObject: [
					"key": "five_hour", "label": NSNull(), "window_minutes": 300,
					"window_minutes_reason": NSNull(), "used_percent": 22, "resets_at": NSNull(),
					"observed_at": observedAt as Any, "source": source as Any,
				])
			)
		}

		let both = try window(source: "claude_statusline", observedAt: "2026-09-10T09:58:00Z")
		let caption = try XCTUnwrap(panel.windowProvenanceCaption(both))
		XCTAssertTrue(caption.contains(t(DesktopCopy.quotaSourceClaudeStatusLine)), "caption = \(caption)")

		let neither = try window(source: nil, observedAt: nil)
		XCTAssertNil(panel.windowProvenanceCaption(neither))
	}

	// Codex PR #5 ninth review, P2: a flat "%.0f%%" rounded 89.6 up to a
	// displayed "90%" that had not actually crossed the 90% threshold the
	// progress bar's tint and the CLI's own threshold both still correctly
	// evaluate against the raw value.
	func testQuotaPercentTextPreservesPrecisionAtThresholds() {
		let panel = QuotaPanelView(clients: [])
		XCTAssertEqual(panel.quotaPercentText(64), "64%")
		XCTAssertEqual(panel.quotaPercentText(89.6), "89.6%")
		XCTAssertEqual(panel.quotaPercentText(74.6), "74.6%")
		XCTAssertEqual(panel.quotaPercentText(90), "90%")
	}

	// Codex PR #5 eleventh review, P2: a window with no resets_at used to
	// render nothing for it, unlike every other absent field on the card.
	func testWindowResetLabelRendersTheReasonWhenResetsAtIsAbsent() throws {
		let panel = QuotaPanelView(clients: [])
		func window(resetsAt: Any, reason: Any) throws -> DesktopSubscriptionWindowV1 {
			try JSONDecoder().decode(
				DesktopSubscriptionWindowV1.self,
				from: JSONSerialization.data(withJSONObject: [
					"key": "codex", "label": NSNull(), "window_minutes": 300,
					"window_minutes_reason": NSNull(), "used_percent": 40, "resets_at": resetsAt,
					"resets_at_reason": reason,
				])
			)
		}

		let missing = try window(resetsAt: NSNull(), reason: "not_reported")
		XCTAssertEqual(panel.windowResetLabel(missing), t(DesktopCopy.quotaReasonNotReported))

		let present = try window(resetsAt: "2026-09-18T05:00:00Z", reason: NSNull())
		XCTAssertNotNil(panel.windowResetLabel(present, now: Date(timeIntervalSince1970: 0)))
		XCTAssertNotEqual(panel.windowResetLabel(present, now: Date(timeIntervalSince1970: 0)), t(DesktopCopy.quotaReasonNotReported))
	}

	func testQuotaCardShowsMissingPlanAndOnlyTheUsefulResetRemainingCount() throws {
		let panel = QuotaPanelView(clients: [])
		let client = try JSONDecoder().decode(
			DesktopSubscriptionClientV1.self,
			from: JSONSerialization.data(withJSONObject: [
				"client": "codex", "applicable": true, "applicable_reason": NSNull(),
				"source": "codex_app_server", "observed_at": "2026-09-10T09:58:00Z",
				"stale": false, "attribution_confirmed": true,
				"plan": NSNull(), "plan_reason": "not_reported", "windows": [],
				"tightest_window_key": NSNull(), "reset_allowance": NSNull(),
				"reset_allowance_reason": NSNull(), "observed_reset_at": NSNull(), "failure": NSNull(),
			])
		)
		XCTAssertEqual(panel.planUnavailableLabel(client), t(DesktopCopy.quotaReasonNotReported))

		// Codex PR #5 ninth review, P2: this used to require client ==
		// "codex", so an applicable Claude client with figures -- whose
		// planReason is always .notReported (C6: Claude never reports a
		// plan) -- silently dropped the required row.
		let claudeClient = try JSONDecoder().decode(
			DesktopSubscriptionClientV1.self,
			from: JSONSerialization.data(withJSONObject: [
				"client": "claude", "applicable": true, "applicable_reason": NSNull(),
				"source": "claude_statusline", "observed_at": "2026-09-10T09:58:00Z",
				"stale": false, "attribution_confirmed": true,
				"plan": NSNull(), "plan_reason": "not_reported", "windows": [],
				"tightest_window_key": NSNull(), "reset_allowance": NSNull(),
				"reset_allowance_reason": "not_reported", "observed_reset_at": NSNull(), "failure": NSNull(),
			])
		)
		XCTAssertEqual(panel.planUnavailableLabel(claudeClient), t(DesktopCopy.quotaReasonNotReported))

		let allowance = try JSONDecoder().decode(
			DesktopResetAllowanceV1.self,
			from: JSONSerialization.data(withJSONObject: [
				"remaining": 3, "remaining_reason": NSNull(),
				"total": NSNull(), "total_reason": "not_reported", "credits": [],
			])
		)
		let summary = panel.allowanceSummary(allowance)
		XCTAssertEqual(summary, t(DesktopCopy.quotaLeft, Int64(3)))
	}

	// Codex PR #5 fifth review, P2: a successful probe that legitimately
	// returns no windows (Codex's explicit-empty `rateLimitsByLimitId: {}`)
	// has a current observed_at and no failure; the card must report that as
	// not_reported, not collapse it into never_probed as if nothing had run.
	func testQuotaCardReportsSuccessfulEmptyResponseAsNotReportedRatherThanNeverProbed() throws {
		func client(observedAt: String?) throws -> DesktopSubscriptionClientV1 {
			let json: [String: Any] = [
				"client": "codex", "applicable": true, "applicable_reason": NSNull(),
				"source": observedAt == nil ? NSNull() : "codex_app_server",
				"observed_at": observedAt as Any, "stale": false, "attribution_confirmed": true,
				"plan": NSNull(), "plan_reason": NSNull(), "windows": [],
				"tightest_window_key": NSNull(), "reset_allowance": NSNull(),
				"reset_allowance_reason": NSNull(), "observed_reset_at": NSNull(), "failure": NSNull(),
			]
			let data = try JSONSerialization.data(withJSONObject: json)
			return try JSONDecoder().decode(DesktopSubscriptionClientV1.self, from: data)
		}

		let panel = QuotaPanelView(clients: [])
		let probedEmpty = try client(observedAt: "2026-09-18T02:00:00Z")
		XCTAssertEqual(panel.primaryReason(probedEmpty), .notReported, "a successful, empty observation must not read as never probed")

		let neverProbed = try client(observedAt: nil)
		XCTAssertEqual(panel.primaryReason(neverProbed), .neverProbed, "no observed_at at all is still never probed")
	}
}

@MainActor
private final class RefreshControlIdentityCapture {
	var identity: UUID?
}

@MainActor
private final class SharedUsageContextPresenceCapture {
	var isPresent = false
}

private struct SharedUsageContextPresenceProbe<Content: View>: View {
	let content: Content
	let capture: SharedUsageContextPresenceCapture

	var body: some View {
		content
			.onPreferenceChange(SharedUsageContextPresencePreferenceKey.self) { capture.isPresent = $0 }
	}
}

private struct RefreshControlIdentityProbe<Content: View>: View {
	let content: Content
	let capture: RefreshControlIdentityCapture

	var body: some View {
		content
			.onPreferenceChange(RefreshControlIdentityPreferenceKey.self) { capture.identity = $0 }
	}
}

@MainActor
final class SchemaSignalAcceptanceTests: XCTestCase {
	private static func fixtureHome(in environment: [String: String]) throws -> String {
		guard environment["AGENTDECK_TEST_SCHEMA_ACCEPTANCE"] == "1" else {
			throw XCTSkip("Schema acceptance matrix is opt-in; skipping is not manual acceptance evidence")
		}
		guard let home = environment["AGENTDECK_TEST_HOME"],
			home.hasPrefix("/private/tmp/agentdeck-menubar-acceptance.")
		else {
			throw XCTSkip("Explicit schema acceptance requires an isolated Home configured before App startup")
		}
		return home
	}

	func testSchemaSignalMatrixEntryIsExplicitAndIsolated() throws {
		let home = "/private/tmp/agentdeck-menubar-acceptance.fixture"
		let skipped: [[String: String]] = [
			[:], ["AGENTDECK_TEST_HOME": home],
			["AGENTDECK_TEST_SCHEMA_ACCEPTANCE": "1"],
			["AGENTDECK_TEST_SCHEMA_ACCEPTANCE": "1", "AGENTDECK_TEST_HOME": "/tmp/not-an-acceptance-home"],
			["AGENTDECK_TEST_SCHEMA_ACCEPTANCE": "0", "AGENTDECK_TEST_HOME": home],
		]
		for environment in skipped {
			XCTAssertThrowsError(try Self.fixtureHome(in: environment)) { error in
				XCTAssertTrue(error is XCTSkip, "unconfigured entry must skip, not fail")
			}
		}
		XCTAssertEqual(try Self.fixtureHome(in: ["AGENTDECK_TEST_SCHEMA_ACCEPTANCE": "1", "AGENTDECK_TEST_HOME": home]), home)
	}

	func testSchemaSignalNativeMatrix() async throws {
		_ = try Self.fixtureHome(in: ProcessInfo.processInfo.environment)
		let oldWidth = ProcessInfo.processInfo.environment["AGENTDECK_TEST_WIDTH"]
		let oldLocale = ProcessInfo.processInfo.environment["AGENTDECK_TEST_LOCALE"]
		defer {
			if let oldWidth { setenv("AGENTDECK_TEST_WIDTH", oldWidth, 1) } else { unsetenv("AGENTDECK_TEST_WIDTH") }
			if let oldLocale { setenv("AGENTDECK_TEST_LOCALE", oldLocale, 1) } else { unsetenv("AGENTDECK_TEST_LOCALE") }
		}
		let repository = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
		let fixture = try Data(contentsOf: repository.appendingPathComponent("desktop/fixtures/v1/snapshot-schema-ahead.json"))
		let envelope = try decodeDesktopWireEnvelopeV1(fixture)
		let directory = URL(fileURLWithPath: "/private/tmp/agentdeck-schema-acceptance-native")
		try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
		for language in ["en", "zh-Hans"] {
			setenv("AGENTDECK_TEST_LOCALE", language, 1)
			for width in [280, 420] {
				setenv("AGENTDECK_TEST_WIDTH", String(width), 1)
				XCTAssertEqual(MenuBarGeometry.width, CGFloat(width))
				let host = StubDesktopHost(behavior: .envelope(envelope))
				let model = await makeModel(host: host)
				await model.coordinator.refresh()
				XCTAssertTrue(model.hasSchemaSignal)
				XCTAssertTrue(model.notices.contains { $0.id == "schema" && $0.opensHealthDetail })
				XCTAssertEqual(model.footer.routesText, t(DesktopCopy.schemaSignalFooter))
				for mode in MenuBarValueMode.allCases {
					model.preferences.menuBarValue = mode
					XCTAssertTrue(model.menuBarBadged, "schema badge must survive \(mode)")
					XCTAssertEqual(model.menuBarAccessibilityLabel, t(DesktopCopy.badgedSchemaSignal))
				}
				for large in [false, true] {
					for health in [false, true] {
						model.showsHealthDetail = health
						let label = "\(language)-\(width)-\(large ? "large" : "standard")-\(health ? "health" : "surface")"
						let view = MenuBarSurfaceView(model: model)
							.environment(\.dynamicTypeSize, large ? .accessibility3 : .large)
						let hosting = NSHostingView(rootView: view)
						hosting.frame = NSRect(x: 0, y: 0, width: CGFloat(width), height: 760)
						hosting.layoutSubtreeIfNeeded()
						hosting.displayIfNeeded()
						XCTAssertLessThanOrEqual(hosting.fittingSize.width, CGFloat(width) + 1)
						let bitmap = try XCTUnwrap(hosting.bitmapImageRepForCachingDisplay(in: hosting.bounds))
						hosting.cacheDisplay(in: hosting.bounds, to: bitmap)
						let png = try XCTUnwrap(bitmap.representation(using: .png, properties: [:]))
						try png.write(to: directory.appendingPathComponent(label + ".png"))
						let attachment = XCTAttachment(data: png, uniformTypeIdentifier: "public.png")
						attachment.name = "Schema acceptance \(label)"
						attachment.lifetime = .keepAlways
						add(attachment)
					}
				}
				host.behavior = .envelope(WireFixture.envelope())
				await model.coordinator.refresh()
				XCTAssertFalse(model.hasSchemaSignal)
				XCTAssertFalse(model.menuBarBadged)
				XCTAssertFalse(model.notices.contains { $0.id == "schema" })
			}
		}
		// Rendering and model-driven navigation do not assert real VoiceOver
		// speech order or disclosure operation; those remain manual acceptance.
	}
}

@MainActor
private final class QuotaPanelWidthCapture {
	var width: CGFloat = 0
}

// subscription-account-identity task 5 (`menubar-subscription-identity`):
// ux/menubar.md M1–M11 over the producer's additive wire. Pure presentation
// assertions pin selection, wording and spoken order; the hosted renderings
// pin the 280 pt and normal-width layout. Real VoiceOver speech, SF font
// truncation on a live menu-bar window and Dynamic Type remain task 7's
// native acceptance (ux/menubar.md §9).
@MainActor
extension MenuBarChromeTests {
	private static let quotaISO: ISO8601DateFormatter = {
		let formatter = ISO8601DateFormatter()
		formatter.formatOptions = [.withInternetDateTime]
		return formatter
	}()

	private func quotaStamp(_ offset: TimeInterval, from now: Date) -> String {
		Self.quotaISO.string(from: now.addingTimeInterval(offset))
	}

	private func withQuotaLocale<T>(_ language: String, _ body: () throws -> T) rethrows -> T {
		let old = ProcessInfo.processInfo.environment["AGENTDECK_TEST_LOCALE"]
		setenv("AGENTDECK_TEST_LOCALE", language, 1)
		defer { if let old { setenv("AGENTDECK_TEST_LOCALE", old, 1) } else { unsetenv("AGENTDECK_TEST_LOCALE") } }
		return try body()
	}

	private func planJSON(_ state: String, name: String? = nil, short: String? = nil, source: String, raw: [(String, String)] = [], reason: String? = nil) -> [String: Any] {
		[
			"state": state, "name": name as Any, "short_name": short as Any,
			"basis": (state == "known" ? "mapping" : state == "raw_only" ? "raw" : nil) as String? as Any,
			"source": source, "raw": raw.map { ["field": $0.0, "value": $0.1] }, "reason": reason as Any,
		]
	}

	/// One identity object as the producer emits it: every key present.
	private func identityJSON(
		state: String,
		freshness: String,
		currentUntil: String? = nil,
		observedAt: String? = nil,
		attemptedAt: String? = nil,
		unknownReason: String? = nil,
		email: String? = nil,
		orgKind: String? = nil,
		orgName: String? = nil,
		plan: [String: Any]? = nil,
		association: String? = nil,
		source: String
	) -> [String: Any] {
		[
			"state": state, "unknown_reason": unknownReason as Any, "freshness": freshness,
			"current_until": currentUntil as Any, "observed_at": observedAt as Any,
			"attempted_at": (attemptedAt ?? observedAt) as Any, "failure": unknownReason == "never_probed" ? NSNull() : unknownReason as Any,
			"failure_at": NSNull(), "source": source, "email_masked": email as Any,
			"organization_kind": orgKind as Any, "organization_name": orgName as Any,
			"plan_display": plan as Any, "quota_association": association as Any,
		]
	}

	/// A quota client; `extra` overrides or adds keys. Leaving a key out of
	/// `extra` and `omit` reproduces the current producer; `omit` removes keys
	/// to reproduce a legacy producer.
	private func quotaClient(
		_ client: String, extra: [String: Any] = [:], omit: Set<String> = [],
		resetsAt: String = "2026-11-19T14:00:00Z"
	) throws -> DesktopSubscriptionClientV1 {
		var json: [String: Any] = [
			"client": client, "applicable": true, "applicable_reason": NSNull(),
			"source": client == "codex" ? "codex_app_server" : "claude_statusline",
			"observed_at": "2026-11-19T10:00:00Z", "stale": false,
			"attribution_confirmed": client == "codex",
			"plan": client == "codex" ? "prolite" : NSNull(), "plan_reason": client == "codex" ? NSNull() : "not_reported",
			"windows": [[
				"key": client == "codex" ? "codex" : "five_hour", "label": NSNull(), "window_minutes": 300,
				"window_minutes_reason": NSNull(), "used_percent": 89, "resets_at": resetsAt,
			]],
			"tightest_window_key": client == "codex" ? "codex" : "five_hour",
			"reset_allowance": NSNull(), "reset_allowance_reason": "not_reported",
			"observed_reset_at": NSNull(), "failure": NSNull(),
			"identity": NSNull(), "quota_context": NSNull(), "failure_detail": NSNull(),
			"plan_display": client == "codex"
				? planJSON("known", name: "ChatGPT Pro 100", short: "Pro 100", source: "codex_rate_limits", raw: [("planType", "prolite")])
				: NSNull(),
		]
		for (key, value) in extra { json[key] = value }
		for key in omit { json.removeValue(forKey: key) }
		return try JSONDecoder().decode(DesktopSubscriptionClientV1.self, from: JSONSerialization.data(withJSONObject: json))
	}

	private func quotaAccessibilityLabels(_ element: Any) -> [String] {
		// SwiftUI does not build an NSHostingView's accessibility tree until an
		// accessibility query triggers it, and in some test hosts (observed on
		// CI) a direct traversal returns no children. Enabling
		// AXEnhancedUserInterface on NSApp forces the tree to be built.
		let getter = Selector(("accessibilityEnhancedUserInterfaceAttribute"))
		let setter = Selector(("accessibilitySetEnhancedUserInterfaceAttribute:"))
		let app = NSApplication.shared
		XCTAssertTrue(app.responds(to: setter), "AXEnhancedUserInterface setter is missing; the accessibility tree may not be built")
		let wasEnhanced = (app.responds(to: getter)
			? (app.perform(getter)?.takeUnretainedValue() as? NSNumber)?.boolValue
			: nil) ?? false
		if !wasEnhanced, app.responds(to: setter) {
			app.perform(setter, with: NSNumber(value: true))
		}
		defer {
			if !wasEnhanced, app.responds(to: setter) {
				app.perform(setter, with: NSNumber(value: false))
			}
		}
		return collectQuotaAccessibilityLabels(element, depth: 0)
	}

	private func collectQuotaAccessibilityLabels(_ element: Any, depth: Int) -> [String] {
		guard depth < 40, let object = element as? NSObject else { return [] }
		var labels: [String] = []
		if object.responds(to: #selector(NSAccessibilityProtocol.accessibilityLabel)),
			let label = object.perform(#selector(NSAccessibilityProtocol.accessibilityLabel))?.takeUnretainedValue() as? String {
			labels.append(label)
		}
		if object.responds(to: #selector(NSAccessibilityProtocol.accessibilityChildren)),
			let children = object.perform(#selector(NSAccessibilityProtocol.accessibilityChildren))?.takeUnretainedValue() as? [Any] {
			for child in children { labels += collectQuotaAccessibilityLabels(child, depth: depth + 1) }
		}
		return labels
	}

	private func renderedQuotaText(_ hosting: NSView, language: String) throws -> [String] {
		// Rasterize at a fixed 3x scale rather than the host screen's backing
		// scale; at 1x (as on CI) small caption and CJK glyphs lose detail and
		// cause Vision OCR misreads.
		let scale = 3
		let bitmap = try XCTUnwrap(NSBitmapImageRep(
			bitmapDataPlanes: nil,
			pixelsWide: max(1, Int(ceil(hosting.bounds.width)) * scale),
			pixelsHigh: max(1, Int(ceil(hosting.bounds.height)) * scale),
			bitsPerSample: 8,
			samplesPerPixel: 4,
			hasAlpha: true,
			isPlanar: false,
			colorSpaceName: .deviceRGB,
			bytesPerRow: 0,
			bitsPerPixel: 0
		))
		bitmap.size = hosting.bounds.size
		hosting.cacheDisplay(in: hosting.bounds, to: bitmap)
		let request = VNRecognizeTextRequest()
		request.recognitionLevel = .accurate
		request.usesLanguageCorrection = false
		if language == "zh-Hans" { request.recognitionLanguages = ["zh-Hans", "en-US"] }
		try VNImageRequestHandler(cgImage: try XCTUnwrap(bitmap.cgImage), options: [:]).perform([request])
		let lines = (request.results ?? []).compactMap { $0.topCandidates(1).first?.string }
		guard language == "zh-Hans" else { return lines }
		// Vision can return the traditional form of a simplified glyph.
		let simplified = StringTransform("Traditional-Simplified")
		return lines.map { $0.applyingTransform(simplified, reverse: false) ?? $0 }
	}

	/// Shared expiry: the same open card relabels a current sign-in as the
	/// last known one at `current_until`, in visible text and in the spoken
	/// identity element, without a new snapshot.
	func testOpenQuotaCardRelabelsExpiredSignIn() async throws {
		let now = Date()
		let stamp = quotaStamp(4, from: now)
		let deadline = try XCTUnwrap(DesktopFormat.timestamp(stamp))
		let client = try quotaClient("claude", extra: ["identity": identityJSON(
			state: "identified", freshness: "current", currentUntil: stamp,
			observedAt: quotaStamp(-60, from: now), email: "c***e@expiry.example.test",
			source: "claude_auth_status"
		)])
		let language = QuotaIdentityPunctuation.isChinese ? "zh-Hans" : "en"
		let current = "Claude " + t(DesktopCopy.quotaIdentityTitle)
		let lastKnown = "Claude " + t(DesktopCopy.quotaIdentityLastKnown)
		let hosting = NSHostingView(rootView: QuotaPanelView(clients: [client])
			.frame(width: 396)
			.frame(width: 420, height: 440, alignment: .top)
			.background(Color.white)
			.environment(\.colorScheme, .light))
		let bounds = NSRect(x: 0, y: 0, width: 420, height: 440)
		let window = NSWindow(contentRect: bounds, styleMask: [.titled], backing: .buffered, defer: false)
		window.isReleasedWhenClosed = false
		window.contentView = hosting
		window.orderFront(nil)
		defer { window.close() }
		hosting.layoutSubtreeIfNeeded()
		try await Task.sleep(for: .milliseconds(100))

		XCTAssertLessThan(Date(), deadline)
		XCTAssertTrue(quotaAccessibilityLabels(hosting).contains { $0.hasPrefix(current) })
		XCTAssertTrue(try renderedQuotaText(hosting, language: language).joined(separator: " ").contains(t(DesktopCopy.quotaIdentityTitle)))

		let limit = deadline.addingTimeInterval(2)
		var labels = quotaAccessibilityLabels(hosting)
		while Date() < limit && !labels.contains(where: { $0.hasPrefix(lastKnown) }) {
			try await Task.sleep(for: .milliseconds(50))
			labels = quotaAccessibilityLabels(hosting)
		}
		XCTAssertGreaterThanOrEqual(Date(), deadline)
		XCTAssertTrue(labels.contains { $0.hasPrefix(lastKnown) }, "\(labels)")
		XCTAssertFalse(labels.contains { $0.hasPrefix(current) }, "\(labels)")
		let text = try renderedQuotaText(hosting, language: language).joined(separator: " ")
		XCTAssertTrue(text.contains(t(DesktopCopy.quotaIdentityLastKnown)), text)
		XCTAssertFalse(text.contains(t(DesktopCopy.quotaIdentityTitle)), text)
	}

	func testOpenQuotaCardWithdrawsExpiredAssociation() async throws {
		let now = Date()
		let stamp = quotaStamp(4, from: now)
		let deadline = try XCTUnwrap(DesktopFormat.timestamp(stamp))
		let client = try quotaClient("codex", extra: ["identity": identityJSON(
			state: "identified", freshness: "current", currentUntil: stamp,
			observedAt: quotaStamp(-60, from: now), email: "a***e@expiry.example.test",
			association: "confirmed", source: "codex_account_read"
		)])
		let language = QuotaIdentityPunctuation.isChinese ? "zh-Hans" : "en"
		let confirmed = t(DesktopCopy.quotaIdentityAssociationConfirmed)
		let lastKnown = t(DesktopCopy.quotaIdentityLastKnown)
		let unconfirmed = t(DesktopCopy.quotaIdentityAssociationUnconfirmed)
		let hosting = NSHostingView(rootView: QuotaPanelView(clients: [client])
			.frame(width: 396)
			.frame(width: 420, height: 440, alignment: .top)
			.background(Color.white)
			.environment(\.colorScheme, .light))
		let bounds = NSRect(x: 0, y: 0, width: 420, height: 440)
		let window = NSWindow(contentRect: bounds, styleMask: [.titled], backing: .buffered, defer: false)
		window.isReleasedWhenClosed = false
		window.contentView = hosting
		window.orderFront(nil)
		defer { window.close() }
		hosting.layoutSubtreeIfNeeded()
		try await Task.sleep(for: .milliseconds(100))

		XCTAssertLessThan(Date(), deadline)
		XCTAssertTrue(quotaAccessibilityLabels(hosting).contains { $0.contains(confirmed) })
		XCTAssertTrue(try renderedQuotaText(hosting, language: language).joined(separator: " ").contains(confirmed))

		let limit = deadline.addingTimeInterval(2)
		var labels = quotaAccessibilityLabels(hosting)
		while Date() < limit && !labels.contains(where: { $0.contains(lastKnown) && $0.contains(unconfirmed) }) {
			try await Task.sleep(for: .milliseconds(50))
			labels = quotaAccessibilityLabels(hosting)
		}
		XCTAssertGreaterThanOrEqual(Date(), deadline)
		XCTAssertTrue(labels.contains { $0.contains(lastKnown) && $0.contains(unconfirmed) }, "\(labels)")
		XCTAssertFalse(labels.contains { $0.contains(confirmed) }, "\(labels)")
		let text = try renderedQuotaText(hosting, language: language).joined(separator: " ")
		XCTAssertTrue(text.contains(lastKnown), text)
		XCTAssertTrue(text.contains(unconfirmed), text)
		XCTAssertFalse(text.contains(confirmed), text)
	}

	func testWideRawQuotaPlanLeavesSourceAndReadTimeVisibleAt280Points() throws {
		try withQuotaLocale("en") {
			let now = Date()
			let observedAt = quotaStamp(-320, from: now)
			let resetsAt = quotaStamp(-6 * 3600 - 600, from: now)
			let readAge = DesktopFormat.relative(observedAt)
			XCTAssertNotEqual(readAge, DesktopFormat.relative(resetsAt))
			let caption = t(DesktopCopy.quotaSourceCodexAppServer) + " " + t(DesktopCopy.quotaObservedAt, readAge)

			func captionWords(_ text: String) -> String {
				let words = text.lowercased().split { !$0.isLetter && !$0.isNumber }
				return " " + words.joined(separator: " ") + " "
			}

			let raw = String(repeating: "W", count: 16)
			let client = try quotaClient("codex", extra: [
				"observed_at": observedAt,
				"plan_display": planJSON("raw_only", name: raw, short: raw, source: "codex_rate_limits", raw: [("planType", raw)]),
			], resetsAt: resetsAt)
			let pro = try quotaClient("codex", extra: ["observed_at": observedAt], resetsAt: resetsAt)
			for (card, isRaw) in [(client, true), (pro, false)] {
				let hosting = NSHostingView(rootView: QuotaPanelView(clients: [card])
					.frame(width: 256)
					.frame(width: 280, height: 600, alignment: .top)
					.background(Color.white)
					.environment(\.colorScheme, .light))
				hosting.frame = NSRect(x: 0, y: 0, width: 280, height: 600)
				hosting.layoutSubtreeIfNeeded()
				hosting.displayIfNeeded()
				let lines = try renderedQuotaText(hosting, language: "en")
				let text = lines.joined(separator: " ")
				XCTAssertTrue(captionWords(text).contains(captionWords(caption)), "Expected \(caption) in \(lines)")
				if isRaw {
					XCTAssertTrue(text.uppercased().contains("WWW"), "\(lines)")
					XCTAssertFalse(text.uppercased().contains(raw), "\(lines)")
				} else {
					XCTAssertTrue(lines.contains { $0.contains("Pro 100") }, "\(lines)")
				}
			}
		}
	}

	func testQuotaIdentityHidesOnlyWhenNeitherSideHasProbed() throws {
		try withQuotaLocale("en") {
			let identity = identityJSON(
				state: "unknown", freshness: "last_read", unknownReason: "never_probed", source: "codex_account_read"
			)
			let neverProbed = try quotaClient("codex", extra: [
				"identity": identity, "failure": "never_probed", "observed_at": NSNull(),
				"windows": [], "tightest_window_key": NSNull(),
			])
			XCTAssertNil(QuotaIdentityPresentation.make(neverProbed, now: Date()))
			let quotaAlreadyRead = try quotaClient("codex", extra: ["identity": identity])
			XCTAssertEqual(QuotaIdentityPresentation.make(quotaAlreadyRead, now: Date())?.meta, "Identity not read yet")
		}
	}

	func testQuotaIdentityCurrentAndLastKnownSignInInBothLanguages() throws {
		let now = Date()
		let expected: [String: (title: String, lastKnown: String, unconfirmed: String, confirmed: String, hidden: String)] = [
			"en": ("Signed in", "Last known sign-in", "Not confirmed as the account behind this quota", "Same account as this quota", "partly hidden"),
			"zh-Hans": ("当前登录", "上次识别的登录", "与此卡额度的账号关联未确认", "与此卡额度为同一账号", "部分隐藏"),
		]
		for (language, copy) in expected {
			try withQuotaLocale(language) {
				let current = try quotaClient("codex", extra: ["identity": identityJSON(
					state: "identified", freshness: "current", currentUntil: quotaStamp(600, from: now),
					observedAt: quotaStamp(-60, from: now), email: "a***e@example.test",
					orgKind: "workspace", orgName: "Acme Research", association: "unconfirmed", source: "codex_account_read"
				)])
				let presented = try XCTUnwrap(QuotaIdentityPresentation.make(current, now: now))
				XCTAssertEqual(presented.lead, .email(label: copy.title, masked: "a***e@example.test", lastKnown: false), language)
				XCTAssertTrue(presented.isCurrent)
				XCTAssertFalse(presented.metaAdjacent)
				XCTAssertEqual(presented.association, copy.unconfirmed, language)
				XCTAssertEqual(presented.organization, t(DesktopCopy.quotaIdentityWorkspace) + QuotaIdentityPunctuation.colon + "Acme Research")
				XCTAssertTrue(presented.accessibilityLabel.hasPrefix("Codex " + copy.title), presented.accessibilityLabel)
				XCTAssertTrue(presented.accessibilityLabel.contains("a \(copy.hidden) e@example.test"), presented.accessibilityLabel)
				XCTAssertFalse(presented.accessibilityLabel.contains("***"), "the spoken name must not read the mask as asterisks")
				XCTAssertTrue(presented.accessibilityLabel.hasSuffix(QuotaIdentityPunctuation.semicolon + copy.unconfirmed), presented.accessibilityLabel)

				// A confirmed association is only stated while the identity is
				// still current at the rendering instant.
				let confirmed = try quotaClient("codex", extra: ["identity": identityJSON(
					state: "identified", freshness: "current", currentUntil: quotaStamp(600, from: now),
					observedAt: quotaStamp(-60, from: now), email: "a***e@example.test", association: "confirmed", source: "codex_account_read"
				)])
				XCTAssertEqual(QuotaIdentityPresentation.make(confirmed, now: now)?.association, copy.confirmed, language)
				let expiredRender = try XCTUnwrap(QuotaIdentityPresentation.make(confirmed, now: now.addingTimeInterval(601)))
				XCTAssertEqual(expiredRender.association, copy.unconfirmed, "an expired current_until must withdraw the confirmation")
				XCTAssertEqual(expiredRender.lead, .email(label: copy.lastKnown, masked: "a***e@example.test", lastKnown: true))
				XCTAssertTrue(expiredRender.metaAdjacent)
				XCTAssertFalse(expiredRender.isCurrent)

				// A producer-side last_read keeps the old mailbox dimmed with its
				// original read time and actual reason adjacent, before row b.
				let lastRead = try quotaClient("codex", extra: ["identity": identityJSON(
					state: "unknown", freshness: "last_read", observedAt: quotaStamp(-3 * 3600, from: now),
					unknownReason: "probe_failed", email: "a***e@example.test", orgKind: "workspace", orgName: "Acme Research",
					source: "codex_account_read"
				)])
				let last = try XCTUnwrap(QuotaIdentityPresentation.make(lastRead, now: now))
				XCTAssertEqual(last.lead, .email(label: copy.lastKnown, masked: "a***e@example.test", lastKnown: true))
				let relative = DesktopFormat.relative(quotaStamp(-3 * 3600, from: now))
				XCTAssertEqual(last.meta, t(DesktopCopy.quotaIdentityLastRead, relative) + " · " + t(DesktopCopy.quotaIdentityReasonProbeFailed))
				XCTAssertTrue(last.metaAdjacent)
				let spoken = last.accessibilityLabel
				let timeIndex = try XCTUnwrap(spoken.range(of: try XCTUnwrap(last.meta))?.lowerBound)
				let orgIndex = try XCTUnwrap(spoken.range(of: try XCTUnwrap(last.organization))?.lowerBound)
				XCTAssertLessThan(timeIndex, orgIndex, "the last-known read time is spoken right after the mailbox: \(spoken)")
			}
		}
		XCTAssertEqual(withQuotaLocale("zh-Hans") { t(DesktopCopy.quotaIdentityLastRead, "3小时前") }, "上次读取 · 3小时前")
		XCTAssertEqual(withQuotaLocale("en") { t(DesktopCopy.quotaIdentityLastRead, "3 hours ago") }, "last read 3 hours ago")
	}

	func testQuotaIdentityWithoutMailboxSpeaksTheStateItself() throws {
		let now = Date()
		try withQuotaLocale("en") {
			let signedOut = try quotaClient("claude", extra: ["identity": identityJSON(
				state: "signed_out", freshness: "current", currentUntil: quotaStamp(600, from: now),
				observedAt: quotaStamp(-30, from: now), email: "stale***@example.test", source: "claude_auth_status"
			)])
			let out = try XCTUnwrap(QuotaIdentityPresentation.make(signedOut, now: now))
			XCTAssertEqual(out.lead, .status("Not signed in to a subscription"), "a signed-out state never shows a mailbox")
			XCTAssertNil(out.association, "Claude never states an account association")
			XCTAssertNil(out.organization)
			XCTAssertTrue(out.accessibilityLabel.hasPrefix("Claude Not signed in to a subscription"), out.accessibilityLabel)
			XCTAssertFalse(out.accessibilityLabel.contains("stale"))

			let apiKey = try quotaClient("claude", extra: ["identity": identityJSON(
				state: "non_subscription", freshness: "last_read", observedAt: quotaStamp(-7200, from: now), source: "claude_auth_status"
			)])
			let key = try XCTUnwrap(QuotaIdentityPresentation.make(apiKey, now: now))
			XCTAssertEqual(key.lead, .status("Last read · Not a subscription sign-in"))
			XCTAssertTrue(key.metaAdjacent)
			XCTAssertTrue(try XCTUnwrap(key.meta).hasPrefix("last read "))

			let codexSignedOut = try quotaClient("codex", extra: ["identity": identityJSON(
				state: "signed_out", freshness: "current", currentUntil: quotaStamp(600, from: now),
				observedAt: quotaStamp(-30, from: now), source: "codex_account_read"
			)])
			let codexOut = try XCTUnwrap(QuotaIdentityPresentation.make(codexSignedOut, now: now))
			XCTAssertEqual(codexOut.lead, .status("Not signed in to a subscription"))
			XCTAssertNil(codexOut.association, "no mailbox, no association row")
			XCTAssertNil(QuotaPanelView(clients: []).quotaDiscardedLabel(codexSignedOut), "Codex sign-out never discards quota (D3)")

			let never = try quotaClient("codex", extra: ["identity": identityJSON(
				state: "unknown", freshness: "last_read", unknownReason: "never_probed", source: "codex_account_read"
			)])
			let unknown = try XCTUnwrap(QuotaIdentityPresentation.make(never, now: now))
			XCTAssertEqual(unknown.lead, .status("Current identity unknown"))
			XCTAssertEqual(unknown.meta, "Identity not read yet")
			XCTAssertEqual(unknown.accessibilityLabel, "Codex Current identity unknown, Identity not read yet")

			let incomparable = try quotaClient("claude", extra: ["identity": identityJSON(
				state: "unknown", freshness: "last_read", unknownReason: "incomparable", source: "claude_auth_status"
			)])
			XCTAssertEqual(QuotaIdentityPresentation.make(incomparable, now: now)?.meta, "Identity fields incomplete")
		}
		try withQuotaLocale("zh-Hans") {
			let apiKey = try quotaClient("claude", extra: ["identity": identityJSON(
				state: "non_subscription", freshness: "current", currentUntil: quotaStamp(600, from: now),
				observedAt: quotaStamp(-30, from: now), source: "claude_auth_status"
			)])
			XCTAssertEqual(QuotaIdentityPresentation.make(apiKey, now: now)?.lead, .status("当前不是订阅登录"))
		}
	}

	func testQuotaIdentityIsHiddenWhenReadingIsOffNotOfficialOrGated() throws {
		let identity = identityJSON(
			state: "identified", freshness: "current", currentUntil: "2099-01-01T00:00:00Z",
			observedAt: "2026-11-19T10:00:00Z", email: "a***e@example.test", source: "codex_account_read"
		)
		let now = Date()
		XCTAssertNil(QuotaIdentityPresentation.make(try quotaClient("codex", extra: ["identity": NSNull()]), now: now), "gate: identity null")
		XCTAssertNil(QuotaIdentityPresentation.make(try quotaClient("codex", omit: ["identity"]), now: now), "legacy producer: no identity key")
		XCTAssertNil(QuotaIdentityPresentation.make(try quotaClient("codex", extra: ["identity": identity, "failure": "probe_disabled", "windows": [], "tightest_window_key": NSNull()]), now: now), "reading off hides any retained identity")
		XCTAssertNil(QuotaIdentityPresentation.make(try quotaClient("codex", extra: ["identity": identity, "applicable": false, "applicable_reason": "not_official", "windows": [], "tightest_window_key": NSNull()]), now: now), "a non-official provider hides any retained identity")
		XCTAssertNotNil(QuotaIdentityPresentation.make(try quotaClient("codex", extra: ["identity": identity]), now: now))
	}

	func testQuotaPlanPillConflictAndLegacyFallback() throws {
		let panel = QuotaPanelView(clients: [])
		let now = Date()
		try withQuotaLocale("en") {
			// M1/Pro 100: the pill shows the compact name; help and the
			// accessibility name carry the full name and the safe raw value.
			let codex = try quotaClient("codex")
			let pill = try XCTUnwrap(panel.planPill(codex))
			XCTAssertEqual(pill.text, "Pro 100")
			XCTAssertEqual(pill.help, "ChatGPT Pro 100 (raw value prolite)")
			XCTAssertNil(panel.planUnavailableLabel(codex))

			// Legacy producer: only a missing `plan_display` key falls back.
			let legacy = try quotaClient("codex", omit: ["plan_display", "identity", "quota_context", "failure_detail"])
			XCTAssertEqual(panel.planPill(legacy)?.text, "prolite")
			let legacyMissing = try quotaClient("codex", extra: ["plan": NSNull(), "plan_reason": "not_reported"], omit: ["plan_display"])
			XCTAssertNil(panel.planPill(legacyMissing))
			XCTAssertEqual(panel.planUnavailableLabel(legacyMissing), t(DesktopCopy.quotaReasonNotReported))

			// Explicit null never reads the legacy `plan`.
			let claude = try quotaClient("claude", extra: ["plan": "should-not-show"])
			XCTAssertNil(panel.planPill(claude))
			XCTAssertEqual(panel.planUnavailableLabel(claude), "not reported by quota source")

			let unsafe = try quotaClient("codex", extra: ["plan": NSNull(), "plan_display": planJSON("missing", source: "codex_rate_limits", reason: "unsafe_value")])
			XCTAssertNil(panel.planPill(unsafe))
			XCTAssertEqual(panel.planUnavailableLabel(unsafe), "plan value cannot be safely displayed")
			let unreported = try quotaClient("codex", extra: ["plan": NSNull(), "plan_display": planJSON("missing", source: "codex_rate_limits", reason: "not_reported")])
			XCTAssertEqual(panel.planUnavailableLabel(unreported), "not reported by quota source")

			let rawOnly = try quotaClient("codex", extra: ["plan_display": planJSON("raw_only", name: "max", short: "max", source: "codex_rate_limits", raw: [("planType", "max")])])
			XCTAssertEqual(panel.planPill(rawOnly)?.text, "max", "an unknown safe raw value is shown as itself")

			let long = String(repeating: "W", count: 64)
			let longPill = try XCTUnwrap(panel.planPill(try quotaClient("codex", extra: ["plan_display": planJSON("known", name: long, short: long, source: "codex_rate_limits")])))
			XCTAssertEqual(longPill.text, long)

			// Claude's identity-side conflict: row b names the disagreement and
			// help lists both raw values; there is still no pill.
			let conflict = try quotaClient("claude", extra: ["identity": identityJSON(
				state: "identified", freshness: "current", currentUntil: quotaStamp(600, from: now),
				observedAt: quotaStamp(-60, from: now), email: "c***e@example.test", orgKind: "organization", orgName: "Acme",
				plan: planJSON("conflict", source: "claude_auth_status", raw: [("planTier", "pro"), ("subscriptionType", "max")]),
				source: "claude_auth_status"
			)])
			let presented = try XCTUnwrap(QuotaIdentityPresentation.make(conflict, now: now))
			XCTAssertEqual(presented.plan, "Sign-in plan: Plan sources disagree")
			XCTAssertTrue(presented.planConflict)
			XCTAssertEqual(presented.planHelp, "Plan sources disagree (planTier: raw value pro · subscriptionType: raw value max)")
			XCTAssertEqual(presented.organization, "Organization: Acme")

			// Identity plan equal to the quota pill is not repeated; a different
			// one is shown with its prefix.
			let same = try quotaClient("codex", extra: ["identity": identityJSON(
				state: "identified", freshness: "current", currentUntil: quotaStamp(600, from: now), observedAt: quotaStamp(-60, from: now),
				email: "a***e@example.test",
				plan: planJSON("known", name: "ChatGPT Pro 100", short: "Pro 100", source: "codex_account_read", raw: [("planType", "prolite")]),
				association: "unconfirmed", source: "codex_account_read"
			)])
			XCTAssertNil(QuotaIdentityPresentation.make(same, now: now)?.plan)
			let claudePro = try quotaClient("claude", extra: ["identity": identityJSON(
				state: "identified", freshness: "current", currentUntil: quotaStamp(600, from: now), observedAt: quotaStamp(-60, from: now),
				email: "c***e@example.test", plan: planJSON("known", name: "Claude Pro", short: "Pro", source: "claude_auth_status", raw: [("planTier", "pro")]),
				source: "claude_auth_status"
			)])
			let claudeIdentity = try XCTUnwrap(QuotaIdentityPresentation.make(claudePro, now: now))
			XCTAssertEqual(claudeIdentity.plan, "Sign-in plan: Pro")
			XCTAssertTrue(claudeIdentity.accessibilityLabel.contains("Sign-in plan: Claude Pro"), claudeIdentity.accessibilityLabel)
		}
		try withQuotaLocale("zh-Hans") {
			XCTAssertEqual(panel.planPill(try quotaClient("codex"))?.help, "ChatGPT Pro 100（原值 prolite）")
			XCTAssertEqual(panel.planUnavailableLabel(try quotaClient("claude")), "额度来源不提供套餐")
		}
	}

	func testQuotaDiscardedRowAndParseDetailSelection() throws {
		let panel = QuotaPanelView(clients: [])
		let emptied: [String: Any] = ["windows": [], "tightest_window_key": NSNull(), "source": NSNull(), "observed_at": NSNull()]
		let expected: [String: [String: String]] = [
			"en": [
				"context_changed": "Sign-in context changed; earlier quota discarded",
				"signed_out": "Signed out of the subscription; earlier quota discarded",
				"non_subscription": "Switched to a non-subscription sign-in; earlier quota discarded",
				"identity_key_reset": "Cannot compare with the earlier sign-in; earlier quota discarded",
				"from_a_newer_producer": "Sign-in context changed; earlier quota discarded",
			],
			"zh-Hans": [
				"context_changed": "登录上下文已变化，旧额度已失效",
				"signed_out": "已退出订阅登录，旧额度已失效",
				"non_subscription": "已切换为非订阅登录，旧额度已失效",
				"identity_key_reset": "无法与之前的登录比较，旧额度已失效",
			],
		]
		for (language, reasons) in expected {
			try withQuotaLocale(language) {
				XCTAssertEqual(t(DesktopCopy.quotaDiscarded), language == "en" ? "Discarded" : "已失效")
				for (reason, sentence) in reasons {
					var extra = emptied
					extra["quota_context"] = ["invalidated_at": "2026-11-19T10:00:00Z", "reason": reason]
					let client = try quotaClient("claude", extra: extra)
					XCTAssertEqual(panel.quotaDiscardedLabel(client), sentence, "\(language) \(reason)")
					XCTAssertNil(panel.parseDetailLabel(client))
				}
			}
		}
		// The first new usage after invalidation clears quota_context: windows
		// return and Claude's attribution stays unconfirmed.
		let recovered = try quotaClient("claude")
		XCTAssertNil(panel.quotaDiscardedLabel(recovered))
		XCTAssertFalse(recovered.attributionConfirmed)

		try withQuotaLocale("en") {
			for (detail, text) in [("missing_line", "expected usage line missing"), ("shape_invalid", "usage line shape or value invalid"), ("timezone_unresolved", "reset time or time zone unresolved")] {
				var extra = emptied
				extra["failure"] = "parse_failed"
				extra["failure_detail"] = detail
				XCTAssertEqual(panel.parseDetailLabel(try quotaClient("claude", extra: extra)), text)
			}
			var legacy = emptied
			legacy["failure"] = "parse_failed"
			XCTAssertNil(panel.parseDetailLabel(try quotaClient("claude", extra: legacy, omit: ["failure_detail"])), "an old producer shows only the original wording")
			var probe = emptied
			probe["failure"] = "probe_failed"
			probe["failure_detail"] = "missing_line"
			XCTAssertNil(panel.parseDetailLabel(try quotaClient("claude", extra: probe)))
		}
	}

	/// Quota and identity carry independent times and provenance: the header
	/// keeps the quota source and age, the identity area its own read time.
	func testQuotaAndIdentityTimesStayIndependent() throws {
		let now = Date()
		try withQuotaLocale("en") {
			let quotaObserved = quotaStamp(-5 * 60, from: now)
			let identityObserved = quotaStamp(-2 * 3600, from: now)
			let identityAttempted = quotaStamp(-30, from: now)
			let client = try quotaClient("claude", extra: [
				"observed_at": quotaObserved,
				"identity": identityJSON(
					state: "identified", freshness: "current", currentUntil: quotaStamp(600, from: now),
					observedAt: identityObserved, attemptedAt: identityAttempted,
					email: "c***e@example.test", source: "claude_auth_status"
				),
			])
			let caption = QuotaPanelView(clients: []).headerCaption(client, source: .claudeStatusLine)
			XCTAssertEqual(caption, t(DesktopCopy.quotaSourceClaudeStatusLine) + " · " + t(DesktopCopy.quotaObservedAt, DesktopFormat.relative(quotaObserved)))
			let identity = try XCTUnwrap(QuotaIdentityPresentation.make(client, now: now))
			XCTAssertEqual(identity.meta, t(DesktopCopy.quotaObservedAt, DesktopFormat.relative(identityObserved)))
			XCTAssertNotEqual(identity.meta, t(DesktopCopy.quotaObservedAt, DesktopFormat.relative(identityAttempted)))
			XCTAssertNotEqual(identity.meta.map { caption.contains($0) }, true)
		}
	}

	/// Hosted rendering at 280 pt and normal width, in both languages and
	/// themes. Panel A carries the long plan, long mailbox and organization
	/// (M3/M11) beside Claude's discarded window with a plan conflict (M4/M6);
	/// panel B carries `Pro 100` with a current, confirmed Codex sign-in (M2)
	/// beside a recovered Claude card (M9). The laid-out panel width proves no
	/// long value widens the card, OCR proves `Pro 100` stays one token and the
	/// long values are truncated, and the AX tree proves each identity area is
	/// one element named by the presentation's spoken text.
	func testQuotaIdentityCardsRenderAtNativeWidthsInBothLanguagesAndThemes() throws {
		let now = Date()
		let long = String(repeating: "W", count: 64)
		let longMailbox = "averyveryverylongmailboxname***e@subdomain.example.org"
		func panels() throws -> [(name: String, cards: [DesktopSubscriptionClientV1])] {
			[
				("long-and-discarded", [
					try quotaClient("codex", extra: [
						"plan_display": planJSON("known", name: long, short: long, source: "codex_rate_limits", raw: [("planType", "w")]),
						"identity": identityJSON(
							state: "unknown", freshness: "last_read", observedAt: quotaStamp(-3 * 3600, from: now), unknownReason: "probe_failed",
							email: longMailbox, orgKind: "workspace",
							orgName: "Acme Research Workspace With A Long Display Name", association: "unconfirmed", source: "codex_account_read"
						),
					]),
					try quotaClient("claude", extra: [
						"windows": [], "tightest_window_key": NSNull(),
						"quota_context": ["invalidated_at": quotaStamp(-60, from: now), "reason": "context_changed"],
						"identity": identityJSON(
							state: "identified", freshness: "current", currentUntil: quotaStamp(600, from: now), observedAt: quotaStamp(-60, from: now),
							email: "c***e@example.test", orgKind: "organization", orgName: "Acme",
							plan: planJSON("conflict", source: "claude_auth_status", raw: [("planTier", "pro"), ("subscriptionType", "max")]),
							source: "claude_auth_status"
						),
					]),
				]),
				("pro100-and-recovered", [
					try quotaClient("codex", extra: ["identity": identityJSON(
						state: "identified", freshness: "current", currentUntil: quotaStamp(600, from: now), observedAt: quotaStamp(-60, from: now),
						email: "a***e@example.test", orgKind: "workspace", orgName: "Acme",
						plan: planJSON("known", name: "ChatGPT Pro 100", short: "Pro 100", source: "codex_account_read", raw: [("planType", "prolite")]),
						association: "confirmed", source: "codex_account_read"
					)]),
					try quotaClient("claude", extra: ["identity": identityJSON(
						state: "identified", freshness: "current", currentUntil: quotaStamp(600, from: now), observedAt: quotaStamp(-60, from: now),
						email: "c***e@example.test", plan: planJSON("known", name: "Claude Pro", short: "Pro", source: "claude_auth_status", raw: [("planTier", "pro")]),
						source: "claude_auth_status"
					)]),
				]),
			]
		}

		for (language, scheme) in [("en", ColorScheme.light), ("zh-Hans", .dark), ("en", .dark), ("zh-Hans", .light)] {
			try withQuotaLocale(language) {
				for panel in try panels() {
					for width in [280, 420] {
						let inner = CGFloat(width - 24)
						let measured = QuotaPanelWidthCapture()
						let view = QuotaPanelView(clients: panel.cards)
							.onGeometryChange(for: CGFloat.self, of: { $0.size.width }) { measured.width = $0 }
							.frame(width: inner)
							.padding(.horizontal, 12)
							.frame(width: CGFloat(width), height: 900, alignment: .top)
							.background(scheme == .dark ? Color.black : Color.white)
							.environment(\.colorScheme, scheme)
						let hosting = NSHostingView(rootView: view)
						hosting.frame = NSRect(x: 0, y: 0, width: width, height: 900)
						hosting.layoutSubtreeIfNeeded()
						hosting.displayIfNeeded()
						RunLoop.current.run(until: Date().addingTimeInterval(0.02))
						let label = "\(panel.name) \(language) \(width) \(scheme == .dark ? "dark" : "light")"
						XCTAssertGreaterThan(measured.width, 0, label)
						XCTAssertLessThanOrEqual(measured.width, inner + 1, "\(label): a long plan, mailbox or organization must not widen the card")

						let png = try renderedViewPNG(hosting)
						XCTAssertGreaterThan(png.count, 8_000)
						add(renderingAttachment(png, named: "Quota identity — \(label)"))

						let lines = try renderedQuotaText(hosting, language: language)
						let joined = lines.joined(separator: "\n")
						XCTAssertTrue(joined.contains("Codex") && joined.contains("Claude"), "\(label): \(lines)")
						if panel.name == "pro100-and-recovered" {
							XCTAssertTrue(joined.contains("example.test"), "\(label): \(lines)")
							XCTAssertTrue(lines.contains { $0.contains("Pro 100") }, "\(label): `Pro 100` must render on one line: \(lines)")
							XCTAssertTrue(joined.contains(language == "en" ? "Same account as this quota" : "与此卡额度为同一账号"), "\(label): \(lines)")
						} else {
							XCTAssertTrue(lines.contains { line in
								guard let start = line.range(of: "avery", options: .caseInsensitive) else { return false }
								return line[start.upperBound...].contains(".org")
							},
								"\(label): middle truncation keeps the same mailbox's start and top-level domain: \(lines)")
							XCTAssertFalse(joined.contains(long), "\(label): the 64-character plan must be truncated")
							XCTAssertTrue(joined.contains(language == "en" ? "Discarded" : "已失效"), "\(label): \(lines)")
							if width == 280 {
								XCTAssertFalse(joined.contains(longMailbox), "\(label): the long mailbox is truncated")
							}
						}

						let labels = quotaAccessibilityLabels(hosting)
						for card in panel.cards {
							guard let spoken = QuotaIdentityPresentation.make(card, now: Date())?.accessibilityLabel else { continue }
							XCTAssertTrue(labels.contains(spoken), "\(label): identity area is one AX element named \(spoken); found \(labels)")
							XCTAssertFalse(labels.contains { $0 != spoken && $0.contains("***") }, "\(label): no AX element reads the mask as asterisks")
						}
					}
				}
			}
		}
	}
}
