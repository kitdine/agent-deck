import AppKit
import SwiftUI
import WidgetKit
import XCTest

final class WidgetPresentationTests: XCTestCase {
	func testAllTwelveKindSizeConfigurationsUseThePrototypeComposition() throws {
		let snapshot = try widgetFixture("snapshot-complete")
		let families: [WidgetFamily] = [.systemSmall, .systemMedium, .systemLarge]
		var configurations = 0

		for kind in AgentDeckWidgetKind.allCases {
			for family in families {
				XCTAssertEqual(
					WidgetLayoutContract.sections(kind, family: family),
					expectedSections(kind, family: family),
					"\(kind) \(family) must follow the matching Widgets.jsx size branch"
				)
				let entry = AgentDeckWidgetEntry(
					date: try XCTUnwrap(WidgetTimelinePolicy.date(snapshot.generatedAt)), snapshot: snapshot,
					kind: kind, client: .all, period: .today, isPlaceholder: false
				)
				XCTAssertEqual(WidgetSurfaceModel(entry: entry, now: entry.date).surface, .data)
				configurations += 1
			}
		}

		XCTAssertEqual(configurations, 12)
		XCTAssertEqual(
			[.systemSmall, .systemMedium, .systemLarge].map(WidgetLayoutContract.bucketCount),
			[7, 20, 90]
		)
		XCTAssertEqual(WidgetLayoutContract.canvas(.systemLarge).width, WidgetLayoutContract.canvas(.systemMedium).width)
		XCTAssertGreaterThan(WidgetLayoutContract.canvas(.systemLarge).height, WidgetLayoutContract.canvas(.systemMedium).height * 2)
	}

	func testPrototypeNumberFormattingDropsFalsePrecision() {
		XCTAssertEqual(WidgetFormat.share("100.00"), "100%")
		XCTAssertEqual(WidgetFormat.share("59.24"), "59.2%")
		XCTAssertEqual(WidgetFormat.share(nil), "—")
		XCTAssertEqual(WidgetFormat.percentage(59, total: 100), "59.00")
		let symbols = DateFormatter().standaloneWeekdaySymbols!
		XCTAssertEqual(WidgetFormat.weekday("monday"), symbols[1])
		XCTAssertEqual(WidgetFormat.weekday("sunday"), symbols[0])
	}

	func testLargestDynamicTypeDegradesToTheNextFamily() {
		XCTAssertEqual(
			WidgetLayoutContract.presentationFamily(.systemLarge, dynamicTypeSize: .accessibility5),
			.systemMedium
		)
		XCTAssertEqual(
			WidgetLayoutContract.presentationFamily(.systemMedium, dynamicTypeSize: .accessibility5),
			.systemSmall
		)
		XCTAssertEqual(
			WidgetLayoutContract.presentationFamily(.systemSmall, dynamicTypeSize: .accessibility5),
			.systemSmall
		)
		XCTAssertEqual(
			WidgetLayoutContract.presentationFamily(.systemLarge, dynamicTypeSize: .xxLarge),
			.systemLarge
		)
	}

	@MainActor
	func testPrototypeAlignedDarkRenderingsAreAttached() throws {
		let snapshot = try snapshotWithPricedToday(widgetFixture("snapshot-complete"), client: "all")
		let now = try XCTUnwrap(WidgetTimelinePolicy.date(snapshot.generatedAt))
		for kind in AgentDeckWidgetKind.allCases {
			for family in [WidgetFamily.systemSmall, .systemMedium, .systemLarge] {
				let entry = AgentDeckWidgetEntry(
					date: now,
					snapshot: snapshot,
					kind: kind,
					client: .all,
					period: prototypePeriod(kind: kind, family: family),
					isPlaceholder: false
				)
				let size = WidgetLayoutContract.canvas(family)
				let view = AgentDeckWidgetView(entry: entry, familyOverride: family)
					.environment(\.colorScheme, .dark)
					.frame(width: size.width, height: size.height)
					.background(Color(red: 0.071, green: 0.082, blue: 0.110))
				let png = try renderedViewPNG(view, size: NSSize(width: size.width, height: size.height))
				XCTAssertGreaterThan(png.count, 1_000)
				let attachment = XCTAttachment(data: png, uniformTypeIdentifier: "public.png")
				attachment.name = "Widget prototype alignment — \(kind.rawValue) \(familyName(family))"
				attachment.lifetime = XCTAttachment.Lifetime.keepAlways
				add(attachment)
			}
		}
	}

	@MainActor
	func testLargeBottomAnchorsAreAttached() throws {
		let incomplete = try widgetFixture("snapshot-complete")
		let priced = try snapshotWithPricedToday(incomplete, client: "all")
		let size = WidgetLayoutContract.canvas(.systemLarge)

		for (kind, snapshot) in [(AgentDeckWidgetKind.composition, priced), (.trust, incomplete)] {
			let now = try XCTUnwrap(WidgetTimelinePolicy.date(snapshot.generatedAt))
			let entry = AgentDeckWidgetEntry(
				date: now,
				snapshot: snapshot,
				kind: kind,
				client: .all,
				period: prototypePeriod(kind: kind, family: .systemLarge),
				isPlaceholder: false
			)
			let view = AgentDeckWidgetView(entry: entry, familyOverride: .systemLarge)
				.environment(\.colorScheme, .dark)
				.frame(width: size.width, height: size.height)
				.background(Color(red: 0.071, green: 0.082, blue: 0.110))
			let png = try renderedViewPNG(view, size: NSSize(width: size.width, height: size.height))
			XCTAssertGreaterThan(png.count, 1_000)
			let attachment = XCTAttachment(data: png, uniformTypeIdentifier: "public.png")
			attachment.name = "Widget bottom anchor — \(kind.rawValue) large"
			attachment.lifetime = XCTAttachment.Lifetime.keepAlways
			add(attachment)
		}
	}

	@MainActor
	func testLargestDynamicTypeRenderingsAreAttached() throws {
		let snapshot = try snapshotWithPricedToday(widgetFixture("snapshot-complete"), client: "all")
		let now = try XCTUnwrap(WidgetTimelinePolicy.date(snapshot.generatedAt))
		for kind in AgentDeckWidgetKind.allCases {
			for family in [WidgetFamily.systemSmall, .systemMedium, .systemLarge] {
				let entry = AgentDeckWidgetEntry(
					date: now,
					snapshot: snapshot,
					kind: kind,
					client: .all,
					period: prototypePeriod(kind: kind, family: family),
					isPlaceholder: false
				)
				let size = WidgetLayoutContract.canvas(family)
				let view = AgentDeckWidgetView(entry: entry, familyOverride: family)
					.environment(\.dynamicTypeSize, .accessibility5)
					.environment(\.colorScheme, .light)
					.frame(width: size.width, height: size.height)
					.background(Color.white)
				let png = try renderedViewPNG(view, size: NSSize(width: size.width, height: size.height))
				XCTAssertGreaterThan(png.count, 1_000)
				let attachment = XCTAttachment(data: png, uniformTypeIdentifier: "public.png")
				attachment.name = "Widget largest Dynamic Type — \(kind.rawValue) \(familyName(family))"
				attachment.lifetime = XCTAttachment.Lifetime.keepAlways
				add(attachment)
			}
		}
	}

	@MainActor
	func testGalleryPlaceholderRenderingsContainNoSnapshotValues() throws {
		for kind in AgentDeckWidgetKind.allCases {
			for family in [WidgetFamily.systemSmall, .systemMedium, .systemLarge] {
				let entry = AgentDeckWidgetEntry(
					date: Date(timeIntervalSince1970: 0),
					snapshot: nil,
					kind: kind,
					client: .all,
					period: .today,
					isPlaceholder: true
				)
				XCTAssertNil(entry.snapshot)
				let size = WidgetLayoutContract.canvas(family)
				let view = AgentDeckWidgetView(entry: entry, familyOverride: family)
					.environment(\.colorScheme, .light)
					.frame(width: size.width, height: size.height)
					.background(Color.white)
				let png = try renderedViewPNG(view, size: NSSize(width: size.width, height: size.height))
				XCTAssertGreaterThan(png.count, 1_000)
				let attachment = XCTAttachment(data: png, uniformTypeIdentifier: "public.png")
				attachment.name = "Widget gallery placeholder — \(kind.rawValue) \(familyName(family))"
				attachment.lifetime = XCTAttachment.Lifetime.keepAlways
				add(attachment)
			}
		}
	}

	func testChartsAndDerivedFactsReadTheProjectionSeries() throws {
		let snapshot = try widgetFixture("snapshot-complete")
		let generated = try XCTUnwrap(WidgetTimelinePolicy.date(snapshot.generatedAt))
		let entry = AgentDeckWidgetEntry(
			date: generated, snapshot: snapshot, kind: .magnitude,
			client: .all, period: .thirtyDays, isPlaceholder: false
		)
		let model = WidgetSurfaceModel(entry: entry, now: generated)
		let expected = try XCTUnwrap(model.scope).daily.items.map { WidgetFormat.chartValue($0.value) }

		XCTAssertEqual(model.chartValues, expected)
		XCTAssertEqual(model.period?.period, "30d")
	}

	func testAccessibilityDescriptorsNameMetricsAndSummarizeCharts() throws {
		let snapshot = try widgetFixture("snapshot-complete")
		let generated = try XCTUnwrap(WidgetTimelinePolicy.date(snapshot.generatedAt))
		let entry = AgentDeckWidgetEntry(
			date: generated, snapshot: snapshot, kind: .magnitude,
			client: .all, period: .thirtyDays, isPlaceholder: false
		)
		let scope = try XCTUnwrap(WidgetSurfaceModel(entry: entry, now: generated).scope)
		let trend = WidgetAccessibility.trend(items: scope.daily.items)

		XCTAssertEqual(trend.label, WidgetCopy.text("Usage trend"))
		XCTAssertTrue(trend.value.contains(WidgetCopy.text("Date range")))
		XCTAssertTrue(trend.value.contains(WidgetCopy.text("Peak")))
		XCTAssertTrue(trend.value.contains(WidgetCopy.text("Trend")))

		let hourGrid = WidgetAccessibility.hourGrid(scope.rhythm)
		XCTAssertEqual(hourGrid.label, WidgetCopy.text("Activity by hour"))
		XCTAssertTrue(hourGrid.value.contains(WidgetCopy.text("Date range")))
		XCTAssertTrue(hourGrid.value.contains(WidgetCopy.text("Peak")))

		let provider = WidgetAccessibility.metric(label: "Official", values: ["$1.25", "88.3%"])
		XCTAssertEqual(provider.label, "Official", "the provider name must be announced before its values")
		XCTAssertEqual(provider.value, "$1.25, 88.3%")
	}

	func testAccessibilityTrendDirectionUsesTheRenderedSeriesEndpoints() {
		XCTAssertEqual(WidgetAccessibility.trendDirection([1, 2]), WidgetCopy.text("Rising"))
		XCTAssertEqual(WidgetAccessibility.trendDirection([2, 1]), WidgetCopy.text("Falling"))
		XCTAssertEqual(WidgetAccessibility.trendDirection([2, 2]), WidgetCopy.text("Steady"))
	}

	func testEmptyIsPerWidgetRatherThanProjectionWide() throws {
		let snapshot = try snapshotWithEmptyToday(widgetFixture("snapshot-complete"), client: "all")
		let now = try XCTUnwrap(WidgetTimelinePolicy.date(snapshot.generatedAt))
		let magnitude = AgentDeckWidgetEntry(
			date: now, snapshot: snapshot, kind: .magnitude,
			client: .all, period: .today, isPlaceholder: false
		)
		let rhythm = AgentDeckWidgetEntry(
			date: now, snapshot: snapshot, kind: .rhythm,
			client: .all, period: .thirtyDays, isPlaceholder: false
		)

		XCTAssertTrue(WidgetSurfaceModel(entry: magnitude, now: now).qualifiers.contains(.empty))
		XCTAssertFalse(WidgetSurfaceModel(entry: rhythm, now: now).qualifiers.contains(.empty))
	}

	func testTrustKeepsTheUnattributedAmountWhenPricingIsIncomplete() throws {
		let snapshot = try widgetFixture("snapshot-complete")
		let now = try XCTUnwrap(WidgetTimelinePolicy.date(snapshot.generatedAt))
		let entry = AgentDeckWidgetEntry(
			date: now, snapshot: snapshot, kind: .trust,
			client: .all, period: .today, isPlaceholder: false
		)
		let model = WidgetSurfaceModel(entry: entry, now: now)

		XCTAssertEqual(model.trustTiers.map(\.quality), ["determinable", "inferred", "unattributed"])
		let unattributed = try XCTUnwrap(model.trustTiers.first { $0.quality == "unattributed" })
		XCTAssertEqual(unattributed.amount, .tokens("1.8k"))
		XCTAssertTrue(unattributed.costIncomplete)
		XCTAssertNil(unattributed.share)
		XCTAssertEqual(model.trustTiers.first { $0.quality == "inferred" }?.amount, .cost("$0.00"))

		let headline = try XCTUnwrap(model.trustHeadline)
		XCTAssertEqual(headline.quality, "determinable")
		XCTAssertNil(headline.share, "an unavailable share must not become the small headline")

		let pricing = try XCTUnwrap(model.trustPricing)
		XCTAssertTrue(pricing.incomplete)
		XCTAssertEqual(pricing.coverage, "0.00")
		XCTAssertEqual(pricing.unpricedIdentifiers, ["claude/claude-sonnet-5", "codex/gpt-5"])
		XCTAssertTrue(model.trustCostIncomplete, "every size must state that cost is incomplete")
		XCTAssertFalse(model.qualifiers.contains(.empty), "1800 unattributed tokens are not an empty period")
	}

	func testTrustUsesTheDeterminableShareOnlyWhileItIsMeaningful() throws {
		let snapshot = try snapshotWithPricedToday(widgetFixture("snapshot-complete"), client: "all")
		let now = try XCTUnwrap(WidgetTimelinePolicy.date(snapshot.generatedAt))
		let entry = AgentDeckWidgetEntry(
			date: now, snapshot: snapshot, kind: .trust,
			client: .all, period: .today, isPlaceholder: false
		)
		let model = WidgetSurfaceModel(entry: entry, now: now)
		let headline = try XCTUnwrap(model.trustHeadline)

		XCTAssertEqual(headline.share, "62.50")
		XCTAssertEqual(headline.amount, .cost("$1.25"))
		XCTAssertFalse(headline.costIncomplete)
		XCTAssertEqual(model.trustTiers.first { $0.quality == "unattributed" }?.amount, .cost("$0.25"))
		XCTAssertFalse(model.trustCostIncomplete)
		XCTAssertEqual(model.trustPricing?.coverage, "100.00")
	}

	private func expectedSections(_ kind: AgentDeckWidgetKind, family: WidgetFamily) -> [String] {
		switch (kind, family) {
		case (.magnitude, .systemSmall): ["headline", "volume", "mini-bars"]
		case (.magnitude, .systemMedium): ["periods", "mini-bars", "date-axis"]
		case (.magnitude, .systemLarge): ["headline", "volume", "periods", "area", "date-axis", "statistics"]
		case (.composition, .systemSmall): ["eyebrow", "top-model", "headline-share", "share-track", "volume"]
		case (.composition, .systemMedium): ["models", "share-tracks"]
		case (.composition, .systemLarge): ["models", "share-tracks", "token-mix", "clients"]
		case (.trust, .systemSmall): ["eyebrow", "headline", "share-track", "support"]
		case (.trust, .systemMedium): ["quality", "share-tracks", "coverage"]
		case (.trust, .systemLarge): ["eyebrow", "headline", "quality", "providers", "pricing-summary"]
		case (.rhythm, .systemSmall): ["eyebrow", "active-days", "busiest"]
		case (.rhythm, .systemMedium): ["hour-axis", "legend", "hour-grid"]
		case (.rhythm, .systemLarge): ["legend", "hour-axis", "hour-grid", "daily-grid", "day-statistics"]
		default: []
		}
	}

	private func prototypePeriod(kind: AgentDeckWidgetKind, family: WidgetFamily) -> WidgetPeriod {
		guard kind == .magnitude || kind == .composition else {
			return kind == .rhythm ? .thirtyDays : .today
		}
		return family == .systemLarge ? .thirtyDays : .today
	}

	private func familyName(_ family: WidgetFamily) -> String {
		switch family {
		case .systemSmall: "small"
		case .systemMedium: "medium"
		case .systemLarge: "large"
		default: "other"
		}
	}

	@MainActor
	private func renderedViewPNG<Content: View>(_ content: Content, size: NSSize) throws -> Data {
		let hosting = NSHostingView(rootView: content)
		hosting.frame = NSRect(origin: .zero, size: size)
		hosting.layoutSubtreeIfNeeded()
		hosting.displayIfNeeded()
		let representation = try XCTUnwrap(hosting.bitmapImageRepForCachingDisplay(in: hosting.bounds))
		hosting.cacheDisplay(in: hosting.bounds, to: representation)
		return try XCTUnwrap(representation.representation(using: .png, properties: [:]))
	}
}
