import AppKit
import SwiftUI
import XCTest
@testable import AgentDeck
@testable import AgentDeckShared

@MainActor
final class MenuBarChromeTests: XCTestCase {
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
		XCTAssertEqual(info["CFBundleIdentifier"] as? String, "com.kitdine.agentdeck")
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
		let settings = SettingsWindowView(preferences: model.preferences)
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
}
