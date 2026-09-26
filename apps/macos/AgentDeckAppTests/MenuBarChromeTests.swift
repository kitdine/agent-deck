import AppKit
import SwiftUI
import XCTest
@testable import AgentDeck
@testable import AgentDeckShared

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
			"the prototype robot is wider than it is tall and must not be stretched into a square"
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
		let settings = SettingsWindowView(preferences: model.preferences, quotaSettings: makeQuotaSettingsController(preferences: model.preferences))
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
		let settingsPNG = try renderedViewPNG(settings, size: NSSize(width: 460, height: 310))
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

	func testQuotaCardShowsMissingPlanAndResetTotalReasons() throws {
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
		XCTAssertTrue(summary.contains(t(DesktopCopy.quotaLeft, Int64(3))))
		XCTAssertTrue(summary.contains(t(DesktopCopy.quotaTotalReason, t(DesktopCopy.quotaReasonNotReported))))
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
